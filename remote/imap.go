package remote

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/emersion/go-imap"
	uidplus "github.com/emersion/go-imap-uidplus"
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
	client        *client.Client
	uidplusClient *uidplus.Client
	log           lib.Logger
	delimiter     string
	selected      *mailbox.Status
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

	uidExt := uidplus.NewClient(imapClient)
	supported, err := uidExt.SupportUidPlus()
	if err != nil || supported == false {
		log.Print("IMAP server does NOT support UIDPLUS extension")
		uidExt = nil
	}
	return &Imap{
		client:        imapClient,
		uidplusClient: uidExt,
		log:           log,
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

func (i *Imap) SupportMessageID() bool {
	return i.uidplusClient != nil
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
		Unseen:         status.Unseen,
		UidValidity:    status.UidValidity,
	}
	return i.selected, nil
}

func (i *Imap) PutMessage(info mailbox.Info, flags []string, date time.Time, body io.Reader) (mailbox.MessageID, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, i.Delimiter())
	buffer := &bytes.Buffer{}
	read, err := buffer.ReadFrom(body)
	if err != nil {
		return mailbox.EmptyMessageID, fmt.Errorf("cannot read message body: %w", err)
	}
	i.log.Printf("Message body: read %d bytes", read)
	var uid uint32
	if i.uidplusClient != nil {
		_, uid, err = i.uidplusClient.Append(name, flags, date, buffer)
	} else {
		err = i.client.Append(name, flags, date, buffer)
	}
	if err != nil {
		return mailbox.EmptyMessageID, fmt.Errorf("cannot append new message to IMAP server: %w", err)
	}
	return mailbox.NewMessageIDFromUint(uid), nil
}

func (i *Imap) FetchMessages(messages chan *mailbox.Message) error {
	seqset := new(imap.SeqSet)
	seqset.AddRange(0, i.selected.Messages)

	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchFlags, imap.FetchUid}

	receiver := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- i.client.Fetch(seqset, items, receiver)
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range receiver {
			i.log.Printf("Received IMAP message seq=%d", msg.SeqNum)
			// receive all the messages as they get in
			message := &mailbox.Message{
				SeqNum:       msg.SeqNum,
				Flags:        msg.Flags,
				InternalDate: msg.InternalDate,
				Size:         msg.Size,
				Uid:          mailbox.NewMessageIDFromUint(msg.Uid),
				Body:         io.NopCloser(msg.GetBody(section)),
			}
			// and transfer them to the output
			messages <- message
		}
	}()
	// will return the error from Fetch when it's finished
	err := <-done
	wg.Wait()
	close(messages)
	i.log.Print("All IMAP messages received")
	return err
}

func (i *Imap) UnselectMailbox() error {
	i.selected = nil
	return i.client.Unselect()
}
