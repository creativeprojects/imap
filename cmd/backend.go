package cmd

import (
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/mdir"
	"github.com/creativeprojects/imap/remote"
	"github.com/creativeprojects/imap/store"
)

type Backend interface {
	Close() error
	CreateMailbox(info mailbox.Info) error
	ListMailbox() ([]mailbox.Info, error)
}

// verify interface
var (
	_ Backend = &remote.Imap{}
	_ Backend = &store.BoltStore{}
	_ Backend = &mdir.Maildir{}
)
