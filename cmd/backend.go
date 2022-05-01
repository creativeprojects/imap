package cmd

import (
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/mdir"
	"github.com/creativeprojects/imap/remote"
	"github.com/creativeprojects/imap/store"
)

type Backend interface {
	// Delimiter used to construct a path of mailboxes with its children
	Delimiter() string
	// Close the backend
	Close() error
	CreateMailbox(info mailbox.Info) error
	ListMailbox() ([]mailbox.Info, error)
	DeleteMailbox(info mailbox.Info) error
}

// verify interface
var (
	_ Backend = &remote.Imap{}
	_ Backend = &store.BoltStore{}
	_ Backend = &mdir.Maildir{}
)
