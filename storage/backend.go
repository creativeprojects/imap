package storage

import (
	"fmt"
	"io"

	"github.com/creativeprojects/imap/cfg"
	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/mdir"
	"github.com/creativeprojects/imap/mem"
	"github.com/creativeprojects/imap/remote"
	"github.com/creativeprojects/imap/storage/local"
)

type Backend interface {
	// DebugLogger sets a logger to send debug information to
	DebugLogger(logger lib.Logger)
	// Delimiter used to construct a path of mailboxes with its children
	Delimiter() string
	// SupportMessageID indicates if the backend support unique IDs (like the IMAP UIDPLUS extension)
	SupportMessageID() bool
	// Close the backend
	Close() error
	CreateMailbox(info mailbox.Info) error
	ListMailbox() ([]mailbox.Info, error)
	DeleteMailbox(info mailbox.Info) error
	// SelectMailbox opens the current mailbox for fetching messages
	SelectMailbox(info mailbox.Info) (*mailbox.Status, error)
	PutMessage(info mailbox.Info, props mailbox.MessageProperties, body io.Reader) (mailbox.MessageID, error)
	// FetchMessages needs a mailbox to be selected first
	FetchMessages(messages chan *mailbox.Message) error
	// UnselectMailbox after fetching messages
	UnselectMailbox() error
}

// verify interface
var (
	_ Backend = &remote.Imap{}
	_ Backend = &local.BoltStore{}
	_ Backend = &mdir.Maildir{}
	_ Backend = &mem.Backend{}
)

func NewBackend(config cfg.Account) (Backend, error) {
	switch config.Type {
	case cfg.IMAP:
		return remote.NewImap(remote.Config{
			ServerURL:           config.ServerURL,
			Username:            config.Username,
			Password:            config.Password,
			SkipTLSVerification: config.SkipTLSVerification,
		})
	case cfg.LOCAL:
		return local.NewBoltStore(config.File)
	case cfg.MAILDIR:
		return mdir.New(config.Root)
	default:
		return nil, fmt.Errorf("unsupported account type %q", config.Type)
	}
}
