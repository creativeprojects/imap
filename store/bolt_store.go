package store

import (
	"io/fs"
	"os"
	"time"

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
	dbFile         string
	dbFileMode     fs.FileMode
	defaultOptions *bolt.Options
}

func NewBoltStore(filename string) *BoltStore {
	options := bolt.DefaultOptions
	options.Timeout = 10 * time.Second
	return &BoltStore{
		dbFile:         filename,
		dbFileMode:     0644,
		defaultOptions: options,
	}
}

func (s *BoltStore) Exists() bool {
	_, err := os.Stat(s.dbFile)
	return err == nil
}

func (s *BoltStore) Init() error {
	db, err := bolt.Open(s.dbFile, s.dbFileMode, s.defaultOptions)
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
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

func (s *BoltStore) Close() {
	//
}

func (s *BoltStore) CreateMailbox(info mailbox.Info) error {
	db, err := bolt.Open(s.dbFile, s.dbFileMode, s.defaultOptions)
	if err != nil {
		return err
	}
	defer db.Close()

	// Start the transaction.
	tx, err := db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Setup the mailbox bucket.
	root, err := tx.CreateBucketIfNotExists([]byte(mailboxBucket))
	if err != nil {
		return err
	}

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

func (s *BoltStore) List() ([]mailbox.Info, error) {
	db, err := bolt.Open(s.dbFile, s.dbFileMode, s.defaultOptions)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	list := make([]mailbox.Info, 0)
	err = db.View(func(tx *bolt.Tx) error {
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

func (s *BoltStore) Backup(filename string) error {
	db, err := bolt.Open(s.dbFile, s.dbFileMode, s.defaultOptions)
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		return tx.CopyFile(filename, 0644)
	})
	if err != nil {
		return err
	}
	return nil
}
