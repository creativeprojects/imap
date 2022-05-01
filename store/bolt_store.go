package store

import (
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
	versionKey      = "version"
	boltFileVersion = 1
)

type BoltStore struct {
	dbFile string
	db     *bolt.DB
	log    lib.Logger
}

func NewBoltStore(filename string) (*BoltStore, error) {
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
		log:    &lib.NoLog{},
	}, nil
}

func (s *BoltStore) Delimiter() string {
	return "."
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

func (m *BoltStore) DebugLogger(logger lib.Logger) {
	m.log = logger
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
		Name:           info.Name,
		PermanentFlags: []string{"\\*"},
		UidNext:        1,
		UidValidity:    1,
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
	return status, nil
}

func (s *BoltStore) PutMessage(info mailbox.Info, flags []string, date time.Time, body io.Reader) error {
	return s.db.Update(func(tx *bolt.Tx) error {
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
		status.Messages++
		return setMailboxStatus(mbox, *status)
	})
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
