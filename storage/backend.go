package storage

import (
	"io"

	"github.com/creativeprojects/imap/mailbox"
)

type Backend interface {
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
	AddToHistory(actions ...mailbox.HistoryAction) error
	GetHistory() (*mailbox.History, error)
}
