package remote

import (
	"errors"
	"fmt"

	"github.com/creativeprojects/clog"
	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Config struct {
	ServerURL string
	Username  string
	Password  string
	Logger    clog.Logger
}

type Imap struct {
	client *client.Client
	log    clog.Logger
}

func NewImap(cfg Config) (*Imap, error) {
	log := cfg.Logger
	if cfg.ServerURL == "" || cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("missing information from Config object")
	}

	log.Debugf("Connecting to server %s...", cfg.ServerURL)
	imapClient, err := client.DialTLS(cfg.ServerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to server %s: %w", cfg.ServerURL, err)
	}
	log.Debug("Connected")

	if err := imapClient.Login(cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("authentication failure: %w", err)
	}
	log.Debugf("Logged in as %s", cfg.Username)

	return &Imap{
		client: imapClient,
		log:    log,
	}, nil
}

func (i *Imap) Close() error {
	return i.client.Logout()
}

func (i *Imap) ListMailbox() ([]mailbox.Info, error) {
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- i.client.List("", "*", mailboxes)
	}()

	i.log.Debug("Listing mailboxes:")
	info := make([]mailbox.Info, 0, 10)
	for m := range mailboxes {
		i.log.Debugf("* %q: %+v (delimiter = %q)", m.Name, m.Attributes, m.Delimiter)
		info = append(info, mailbox.Info{
			Attributes: m.Attributes,
			Delimiter:  m.Delimiter,
			Name:       m.Name,
		})
	}

	if err := <-done; err != nil {
		return nil, err
	}
	return info, nil
}

func (i *Imap) CreateMailbox(info mailbox.Info) error {
	name := info.Name
	// Let's load the list of existing mailboxes,
	// which will also give us the delimiter used by the server
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
		expectedDelimiter := mailboxes[0].Delimiter
		name = lib.VerifyDelimiter(name, info.Delimiter, expectedDelimiter)
	}

	return i.client.Create(name)
}
