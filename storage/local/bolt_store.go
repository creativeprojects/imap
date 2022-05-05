package local

import (
	"bytes"
	"compress/zlib"
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
		buffer := &bytes.Buffer{}
		// read, err := buffer.ReadFrom(body)
		// \-> use compression instead
		writer := zlib.NewWriter(buffer)
		read, err := io.Copy(writer, body)
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
		err = mbox.Put(SerializeUID(bodyPrefix, uid), buffer.Bytes())
		if err != nil {
			return fmt.Errorf("cannot save message body: %w", err)
		}
		s.log.Printf("Message saved: mailbox=%q uid=%d size=%d flags=%+v", name, uid, read, props.Flags)

		props := &msgProps{
			Flags: props.Flags,
			Date:  props.InternalDate,
			Size:  uint32(read),
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

func (s *BoltStore) FetchMessages(messages chan *mailbox.Message) error {
	defer close(messages)

	if s.selected == "" {
		return lib.ErrNotSelected
	}
	name := s.selected

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
				// uncompress data
				reader, err := zlib.NewReader(bytes.NewReader(value))
				if err != nil {
					return err
				}
				reader.Close()
				messages <- &mailbox.Message{
					MessageProperties: mailbox.MessageProperties{
						Flags:        properties.Flags,
						Size:         properties.Size,
						Hash:         properties.Hash,
						InternalDate: properties.Date,
					},
					Uid: mailbox.NewMessageIDFromUint(uint32(DeserializeUID(bodyPrefix, key))),
					// Body: io.NopCloser(bytes.NewReader(value)),
					Body: reader,
				}
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

func (s *BoltStore) UnselectMailbox() error {
	s.selected = ""
	return nil
}

func (s *BoltStore) AddToHistory(actions ...mailbox.HistoryAction) error {
	return nil
}

func (s *BoltStore) GetHistory() (*mailbox.History, error) {
	return nil, nil
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
