package store

import (
	"fmt"
	"os"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	bolt "go.etcd.io/bbolt"
)

const (
	metadataBucket  = "metadata"
	mailboxBucket   = "mailbox"
	infoKey         = "info"
	versionKey      = "version"
	boltFileVersion = 1
)

type BoltStore struct {
	dbFile string
	db     *bolt.DB
}

func NewBoltStore(filename string) (*BoltStore, error) {
	options := bolt.DefaultOptions
	options.Timeout = 10 * time.Second

	db, err := bolt.Open(filename, 0600, options)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %q", filename, err)
	}

	return &BoltStore{
		dbFile: filename,
		db:     db,
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

	bucket, err := root.CreateBucketIfNotExists([]byte(info.Name))
	if err != nil {
		return err
	}

	data, err := SerializeObject(&info)
	if err != nil {
		return err
	}

	err = bucket.Put([]byte(infoKey), data)
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
			if v != nil {
				return nil
			}
			entry := bucket.Bucket(k)
			if entry == nil {
				return nil
			}
			data := entry.Get([]byte(infoKey))
			if data == nil {
				return nil
			}
			info, err := DeserializeObject[mailbox.Info](data)
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

func (s *BoltStore) Backup(filename string) error {
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.CopyFile(filename, 0644)
	})
	if err != nil {
		return err
	}
	return nil
}
