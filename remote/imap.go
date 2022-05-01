package remote

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Config struct {
	ServerURL   string
	Username    string
	Password    string
	DebugLogger lib.Logger
	NoTLS       bool
}

type Imap struct {
	client    *client.Client
	log       lib.Logger
	delimiter string
	selected  *mailbox.Status
}

func NewImap(cfg Config) (*Imap, error) {
	log := cfg.DebugLogger
	if log == nil {
		log = &lib.NoLog{}
	}
	if cfg.ServerURL == "" || cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("missing information from Config object")
	}

	var imapClient *client.Client
	var err error
	log.Printf("Connecting to server %s...", cfg.ServerURL)
	if cfg.NoTLS {
		imapClient, err = client.Dial(cfg.ServerURL)
	} else {
		imapClient, err = client.DialTLS(cfg.ServerURL, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot connect to server %s: %w", cfg.ServerURL, err)
	}
	log.Print("Connected")

	if err := imapClient.Login(cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("authentication failure: %w", err)
	}
	log.Printf("Logged in as %s", cfg.Username)

	return &Imap{
		client: imapClient,
		log:    log,
	}, nil
}

func (i *Imap) Close() error {
	i.log.Print("Closing connection")
	return i.client.Logout()
}

func (i *Imap) Delimiter() string {
	if i.delimiter == "" {
		_, _ = i.ListMailbox()
	}
	return i.delimiter
}

func (i *Imap) ListMailbox() ([]mailbox.Info, error) {
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- i.client.List("", "*", mailboxes)
	}()

	i.log.Print("Listing mailboxes:")
	info := make([]mailbox.Info, 0, 10)
	for m := range mailboxes {
		i.log.Printf("* %q: %+v (delimiter = %q)", m.Name, m.Attributes, m.Delimiter)
		info = append(info, mailbox.Info{
			Attributes: m.Attributes,
			Delimiter:  m.Delimiter,
			Name:       m.Name,
		})
		// sets the delimiter (if not already set)
		if i.delimiter == "" {
			i.delimiter = m.Delimiter
		}
	}

	if err := <-done; err != nil {
		return nil, err
	}
	return info, nil
}

func (i *Imap) CreateMailbox(info mailbox.Info) error {
	name := info.Name
	mailboxes, err := i.ListMailbox()
	if err != nil {
		return err
	}
	if len(mailboxes) > 0 {
		for _, mailbox := range mailboxes {
			if mailbox.Name == name {
				// already existing
				return nil
			}
		}
		name = lib.VerifyDelimiter(name, info.Delimiter, i.Delimiter())
	}

	i.log.Printf("Creating mailbox %q using delimiter %q", name, i.Delimiter())
	return i.client.Create(name)
}

func (i *Imap) DeleteMailbox(info mailbox.Info) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, i.Delimiter())
	i.log.Printf("Deleting mailbox %q using delimiter %q", name, i.Delimiter())
	return i.client.Delete(name)
}

func (i *Imap) SelectMailbox(info mailbox.Info) (*mailbox.Status, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, i.Delimiter())
	i.log.Printf("Selecting mailbox %q using delimiter %q", name, i.Delimiter())
	status, err := i.client.Select(name, false)
	if err != nil {
		return nil, err
	}
	i.selected = &mailbox.Status{
		Name:           status.Name,
		Flags:          status.Flags,
		PermanentFlags: status.PermanentFlags,
		Messages:       status.Messages,
		Recent:         status.Recent,
		Unseen:         status.Unseen,
		UnseenSeqNum:   status.UnseenSeqNum,
		UidNext:        status.UidNext,
		UidValidity:    status.UidValidity,
	}
	return i.selected, nil
}

func (i *Imap) PutMessage(info mailbox.Info, flags []string, date time.Time, body io.Reader) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, i.Delimiter())
	buffer := &bytes.Buffer{}
	read, err := buffer.ReadFrom(body)
	if err != nil {
		return fmt.Errorf("cannot read message body: %w", err)
	}
	i.log.Printf("Message body: read %d bytes", read)
	err = i.client.Append(name, flags, date, buffer)
	if err != nil {
		return fmt.Errorf("cannot append new message to IMAP server: %w", err)
	}
	return nil
}

func (i *Imap) FetchMessages(messages chan *mailbox.Message) error {
	seqset := new(imap.SeqSet)
	seqset.AddRange(0, i.selected.Messages)

	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchFlags, imap.FetchUid}

	receiver := make(chan *imap.Message, 10)
	// done := make(chan error, 1)
	// go func() {
	// 	done <- i.client.Fetch(seqset, items, receiver)
	// }()

	go func() {
		for msg := range receiver {
			// receive all the messages as they get in
			message := &mailbox.Message{
				SeqNum:       msg.SeqNum,
				Flags:        msg.Flags,
				InternalDate: msg.InternalDate,
				Size:         msg.Size,
				Uid:          msg.Uid,
				Body:         msg.GetBody(section),
			}
			// and transfer them to the output
			messages <- message
		}
		close(messages)
	}()
	// will return the error from Fetch when it's finished
	// err := <-done
	// send a nil message to signal it's the end
	// messages <- nil
	err := i.client.Fetch(seqset, items, receiver)
	return err
}
