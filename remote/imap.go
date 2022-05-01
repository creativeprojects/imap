package remote

import (
	"errors"
	"fmt"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Config struct {
	ServerURL   string
	Username    string
	Password    string
	DebugLogger Logger
	NoTLS       bool
}

type Imap struct {
	client    *client.Client
	log       Logger
	delimiter string
}

func NewImap(cfg Config) (*Imap, error) {
	log := cfg.DebugLogger
	if log == nil {
		log = &noLog{}
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
	i.client.Select(name, false)
	return nil, nil
}
