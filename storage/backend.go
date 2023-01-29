package storage

import (
	"context"
	"io"
	"time"

	"github.com/creativeprojects/imap/mailbox"
)

type Backend interface {
	// AccountID is an internal ID used to tag accounts in history
	AccountID() string
	// Delimiter used to construct a path of mailboxes with its children
	Delimiter() string
	// SupportMessageID indicates if the backend support unique IDs (like the IMAP UIDPLUS extension)
	SupportMessageID() bool
	// SupportMessageHash indicates if the backend stores or provides a hash for the messages
	SupportMessageHash() bool
	// Close the backend
	Close() error
	// CreateMailbox doesn't return an error if the mailbox already exists
	CreateMailbox(info mailbox.Info) error
	ListMailbox() ([]mailbox.Info, error)
	DeleteMailbox(info mailbox.Info) error
	// SelectMailbox opens the current mailbox for fetching messages
	SelectMailbox(info mailbox.Info) (*mailbox.Status, error)
	PutMessage(info mailbox.Info, props mailbox.MessageProperties, body io.Reader) (mailbox.MessageID, error)
	// FetchMessages needs a mailbox to be selected first.
	// Use the zero Time to fetch all messages.
	FetchMessages(ctx context.Context, since time.Time, messages chan *mailbox.Message) error
	// LatestDate returns the internal date of the latest message
	LatestDate(ctx context.Context) (time.Time, error)
	// UnselectMailbox after fetching messages
	UnselectMailbox() error
	AddToHistory(info mailbox.Info, actions ...mailbox.HistoryAction) error
	GetHistory(info mailbox.Info) (*mailbox.History, error)
}
