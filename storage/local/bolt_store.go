package local

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	bolt "go.etcd.io/bbolt"
)

const (
	metadataBucket  = "metadata"
	mailboxBucket   = "mailbox"
	infoKey         = "info"
	statusKey       = "status"
	historyKey      = "history"
	bodyPrefix      = "body-"
	msgPrefix       = "msg-"
	versionKey      = "version"
	boltFileVersion = 1
)

type BoltStore struct {
	dbFile   string
	db       *bolt.DB
	log      lib.Logger
	selected string
}

func NewBoltStore(filename string) (*BoltStore, error) {
	return NewBoltStoreWithLogger(filename, nil)
}

func NewBoltStoreWithLogger(filename string, logger lib.Logger) (*BoltStore, error) {
	if logger == nil {
		logger = &lib.NoLog{}
	}
	options := bolt.DefaultOptions
	options.Timeout = 10 * time.Second

	err := os.MkdirAll(filepath.Dir(filename), 0700)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", filename, err)
	}

	db, err := bolt.Open(filename, 0600, options)
	if err != nil {
		return nil, err
	}

	return &BoltStore{
		dbFile: filename,
		db:     db,
		log:    logger,
	}, nil
}

func (s *BoltStore) Delimiter() string {
	return "."
}

func (s *BoltStore) SupportMessageID() bool {
	return true
}

func (s *BoltStore) SupportMessageHash() bool {
	return true
}

func (s *BoltStore) Exists() bool {
	_, err := os.Stat(s.dbFile)
	return err == nil
}

func (s *BoltStore) Init() error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(metadataBucket))
		if err != nil {
			return err
		}
		version, err := SerializeInt(boltFileVersion)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(versionKey), version)
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}

// CreateMailbox doesn't return an error if the mailbox already exists
func (s *BoltStore) CreateMailbox(info mailbox.Info) error {
	// Start the transaction.
	tx, err := s.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Setup the mailbox bucket.
	root, err := tx.CreateBucketIfNotExists([]byte(mailboxBucket))
	if err != nil {
		return err
	}

	info = mailbox.ChangeDelimiter(info, s.Delimiter())

	bucket, err := root.CreateBucket([]byte(info.Name))
	if err != nil {
		if errors.Is(err, bolt.ErrBucketExists) {
			// don't return an error when the bucket exists
			return nil
		}
		return err
	}

	err = setMailboxInfo(bucket, info)
	if err != nil {
		return err
	}

	// default status on empty mailbox
	status := mailbox.Status{
		Name:        info.Name,
		UidValidity: lib.NewUID(),
	}
	err = setMailboxStatus(bucket, status)
	if err != nil {
		return err
	}

	// Commit the transaction.
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *BoltStore) ListMailbox() ([]mailbox.Info, error) {
	list := make([]mailbox.Info, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(mailboxBucket))
		if bucket == nil {
			return nil
		}
		err := bucket.ForEach(func(k, v []byte) error {
			// if there's a value it's not a bucket
			if v != nil {
				return nil
			}
			entry := bucket.Bucket(k)
			if entry == nil {
				return nil
			}
			info, err := getMailboxInfo(entry)
			if err != nil {
				return err
			}
			list = append(list, *info)
			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *BoltStore) DeleteMailbox(info mailbox.Info) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(mailboxBucket))
		if bucket == nil {
			return nil
		}
		name := lib.VerifyDelimiter(info.Name, info.Delimiter, s.Delimiter())

		return bucket.DeleteBucket([]byte(name))
	})
}

func (s *BoltStore) SelectMailbox(info mailbox.Info) (*mailbox.Status, error) {
	var status *mailbox.Status
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, s.Delimiter())

	err := s.db.View(func(tx *bolt.Tx) error {
		var err error

		bucket := tx.Bucket([]byte(mailboxBucket))
		if bucket == nil {
			return lib.ErrMailboxNotFound
		}
		mailboxBucket := bucket.Bucket([]byte(name))
		if mailboxBucket == nil {
			return lib.ErrMailboxNotFound
		}
		status, err = getMailboxStatus(mailboxBucket)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.selected = name
	return status, nil
}

func (s *BoltStore) PutMessage(info mailbox.Info, props mailbox.MessageProperties, body io.Reader) (mailbox.MessageID, error) {
	var messageID mailbox.MessageID
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(mailboxBucket))
		if bucket == nil {
			return lib.ErrMailboxNotFound
		}
		name := lib.VerifyDelimiter(info.Name, info.Delimiter, s.Delimiter())
		mbox := bucket.Bucket([]byte(name))
		if mbox == nil {
			return lib.ErrMailboxNotFound
		}
		status, err := getMailboxStatus(mbox)
		if err != nil {
			return err
		}
		uid, err := mbox.NextSequence()
		if err != nil {
			return fmt.Errorf("cannot get next message ID: %w", err)
		}
		messageID = mailbox.NewMessageIDFromUint(uint32(uid))

		// T reader for hashing
		hasher := sha256.New()
		tee := io.TeeReader(body, hasher)
		buffer := &bytes.Buffer{}

		// compression
		writer := zlib.NewWriter(buffer)
		read, err := io.Copy(writer, tee)
		if err != nil {
			return fmt.Errorf("cannot read message body: %w", err)
		}
		err = writer.Close()
		if err != nil {
			return fmt.Errorf("error closing zlib writer: %w", err)
		}
		if props.Size > 0 && read != int64(props.Size) {
			return fmt.Errorf("message body size advertised as %d bytes but read %d bytes from buffer", props.Size, read)
		}

		msg := buffer.Bytes()
		err = mbox.Put(SerializeUID(bodyPrefix, uid), msg)
		if err != nil {
			return fmt.Errorf("cannot save message body: %w", err)
		}
		s.log.Printf("Message saved: mailbox=%q uid=%d size=%d flags=%+v date=%q", name, uid, read, props.Flags, props.InternalDate)

		props := &msgProps{
			Flags: props.Flags,
			Date:  props.InternalDate,
			Size:  uint32(read),
			Hash:  hasher.Sum(nil),
		}
		err = storeUID(mbox, msgPrefix, uid, props)
		if err != nil {
			return err
		}

		status.Messages++
		return setMailboxStatus(mbox, *status)
	})
	return messageID, err
}

func (s *BoltStore) FetchMessages(ctx context.Context, since time.Time, messages chan *mailbox.Message) error {
	defer close(messages)

	if s.selected == "" {
		return lib.ErrNotSelected
	}
	name := s.selected

	// removes a day
	since = lib.SafePadding(since)

	err := s.db.View(func(tx *bolt.Tx) error {
		var err error

		bucket := tx.Bucket([]byte(mailboxBucket))
		if bucket == nil {
			return lib.ErrMailboxNotFound
		}
		mailboxBucket := bucket.Bucket([]byte(name))
		if mailboxBucket == nil {
			return lib.ErrMailboxNotFound
		}

		err = mailboxBucket.ForEach(func(key, value []byte) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.log.Printf("* Key %q", string(key))
			if bytes.HasPrefix(key, []byte(bodyPrefix)) {
				properties := &msgProps{}
				propsKey := bytes.Replace(key, []byte(bodyPrefix), []byte(msgPrefix), 1)
				propsData := mailboxBucket.Get(propsKey)
				if propsData != nil {
					properties, err = DeserializeObject[msgProps](propsData)
					if err != nil {
						return err
					}
				}
				if !since.IsZero() && properties.Date.Before(since) {
					// skip this message
					return nil
				}
				// uncompress data
				reader, err := zlib.NewReader(bytes.NewReader(value))
				if err != nil {
					return err
				}
				reader.Close()

				channelMessage(
					mailbox.NewMessageIDFromUint(uint32(DeserializeUID(bodyPrefix, key))),
					properties,
					reader,
					messages,
				)
			}
			return nil
		})
		return err
	})
	if err != nil {
		return err
	}
	return nil
}

func channelMessage(uid mailbox.MessageID, properties *msgProps, body io.ReadCloser, to chan *mailbox.Message) {
	to <- &mailbox.Message{
		MessageProperties: mailbox.MessageProperties{
			Flags:        properties.Flags,
			Size:         properties.Size,
			Hash:         properties.Hash,
			InternalDate: properties.Date,
		},
		Uid:  uid,
		Body: body,
	}
}

// LatestDate returns the internal date of the latest message
func (s *BoltStore) LatestDate(ctx context.Context) (time.Time, error) {
	latest := time.Time{}

	if s.selected == "" {
		return latest, lib.ErrNotSelected
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(mailboxBucket))
		if bucket == nil {
			return lib.ErrMailboxNotFound
		}
		mailboxBucket := bucket.Bucket([]byte(s.selected))
		if mailboxBucket == nil {
			return lib.ErrMailboxNotFound
		}

		// Create a cursor for iteration.
		c := mailboxBucket.Cursor()

		// Iterate over items in reverse sorted key order. This starts
		// from the last key/value pair and updates the key/value variables to
		// the previous key/value on each iteration.
		//
		// The loop finishes at the beginning of the cursor when a nil key
		// is returned.
		for key, value := c.Last(); key != nil; key, value = c.Prev() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.log.Printf("* Key %q", string(key))
			if bytes.HasPrefix(key, []byte(msgPrefix)) {
				if value != nil {
					properties, err := DeserializeObject[msgProps](value)
					if err != nil {
						return err
					}
					if latest.Before(properties.Date) {
						latest = properties.Date
					}
					// we stop at the first message encountered
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		return latest, err
	}

	return latest, nil
}

func (s *BoltStore) UnselectMailbox() error {
	s.selected = ""
	return nil
}

func (s *BoltStore) AddToHistory(info mailbox.Info, actions ...mailbox.HistoryAction) error {
	// Start the transaction.
	tx, err := s.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Setup the mailbox bucket.
	root, err := tx.CreateBucketIfNotExists([]byte(mailboxBucket))
	if err != nil {
		return err
	}

	info = mailbox.ChangeDelimiter(info, s.Delimiter())

	bucket, err := root.CreateBucketIfNotExists([]byte(info.Name))
	if err != nil {
		return err
	}

	history, err := getMailboxHistory(bucket)
	if err != nil {
		history = &mailbox.History{}
	}
	history.Actions = append(history.Actions, actions...)

	err = setMailboxHistory(bucket, *history)
	if err != nil {
		return err
	}

	// Commit the transaction.
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *BoltStore) GetHistory(info mailbox.Info) (*mailbox.History, error) {
	var history *mailbox.History
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, s.Delimiter())

	err := s.db.View(func(tx *bolt.Tx) error {
		var err error

		bucket := tx.Bucket([]byte(mailboxBucket))
		if bucket == nil {
			return lib.ErrMailboxNotFound
		}
		mailboxBucket := bucket.Bucket([]byte(name))
		if mailboxBucket == nil {
			return lib.ErrMailboxNotFound
		}
		history, err = getMailboxHistory(mailboxBucket)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return history, nil
}

func (s *BoltStore) Backup(filename string) error {
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.CopyFile(filename, 0644)
	})
	if err != nil {
		return err
	}
	return nil
}

func setMailboxInfo(bucket *bolt.Bucket, info mailbox.Info) error {
	data, err := SerializeObject(&info)
	if err != nil {
		return err
	}

	err = bucket.Put([]byte(infoKey), data)
	if err != nil {
		return err
	}

	return nil
}

func getMailboxInfo(bucket *bolt.Bucket) (*mailbox.Info, error) {
	data := bucket.Get([]byte(infoKey))
	if data == nil {
		return nil, lib.ErrInfoNotFound
	}
	info, err := DeserializeObject[mailbox.Info](data)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func setMailboxStatus(bucket *bolt.Bucket, status mailbox.Status) error {
	data, err := SerializeObject(&status)
	if err != nil {
		return err
	}

	err = bucket.Put([]byte(statusKey), data)
	if err != nil {
		return err
	}

	return nil
}

func getMailboxStatus(bucket *bolt.Bucket) (*mailbox.Status, error) {
	data := bucket.Get([]byte(statusKey))
	if data == nil {
		return nil, lib.ErrStatusNotFound
	}
	info, err := DeserializeObject[mailbox.Status](data)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func setMailboxHistory(bucket *bolt.Bucket, history mailbox.History) error {
	data, err := SerializeObject(&history)
	if err != nil {
		return err
	}

	err = bucket.Put([]byte(historyKey), data)
	if err != nil {
		return err
	}

	return nil
}

func getMailboxHistory(bucket *bolt.Bucket) (*mailbox.History, error) {
	data := bucket.Get([]byte(historyKey))
	if data == nil {
		// return empty history instead of an error
		return &mailbox.History{}, nil
	}
	history, err := DeserializeObject[mailbox.History](data)
	if err != nil {
		return nil, err
	}
	return history, nil
}

func storeUID[T any](bucket *bolt.Bucket, prefix string, uid uint64, data *T) error {
	serialized, err := SerializeObject(data)
	if err != nil {
		return err
	}
	err = bucket.Put(SerializeUID(prefix, uid), serialized)
	if err != nil {
		return err
	}
	return nil
}
