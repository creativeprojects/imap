package remote

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/emersion/go-imap"
	uidplus "github.com/emersion/go-imap-uidplus"
	"github.com/emersion/go-imap/client"
)

type Config struct {
	ServerURL           string
	Username            string
	Password            string
	CacheDir            string
	DebugLogger         lib.Logger
	NoTLS               bool
	SkipTLSVerification bool
}

type Imap struct {
	client        *client.Client
	uidplusClient *uidplus.Client
	log           lib.Logger
	delimiter     string
	selected      *mailbox.Status
	tag           string
	cacheDir      string
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
		tlsConfig := &tls.Config{}
		if cfg.SkipTLSVerification {
			tlsConfig.InsecureSkipVerify = true
		}
		imapClient, err = client.DialTLS(cfg.ServerURL, tlsConfig)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot connect to server %s: %w", cfg.ServerURL, err)
	}
	log.Print("Connected")

	if err := imapClient.Login(cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("authentication failure: %w", err)
	}
	log.Printf("Logged in as %s", cfg.Username)

	if caps, err := imapClient.Capability(); err == nil {
		log.Printf("capabilities: %+v", caps)
	}

	// try to enable UIDPLUS extension
	uidExt := uidplus.NewClient(imapClient)
	supported, err := uidExt.SupportUidPlus()
	if err != nil || supported == false {
		log.Print("IMAP server does NOT support UIDPLUS extension")
		uidExt = nil
	}

	// cache dir
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		wd, _ := os.Getwd()
		cacheDir = filepath.Join(wd, ".cache")
	}

	return &Imap{
		client:        imapClient,
		uidplusClient: uidExt,
		log:           log,
		tag:           mailbox.AccountTag(cfg.ServerURL, cfg.Username),
		cacheDir:      cacheDir,
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

func (i *Imap) SupportMessageHash() bool {
	return false
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
			Delimiter: m.Delimiter,
			Name:      m.Name,
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
		Name:        status.Name,
		Messages:    status.Messages,
		Unseen:      status.Unseen,
		UidValidity: status.UidValidity,
	}
	return i.selected, nil
}

func (i *Imap) PutMessage(info mailbox.Info, props mailbox.MessageProperties, body io.Reader) (mailbox.MessageID, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, i.Delimiter())
	buffer := &bytes.Buffer{}
	read, err := buffer.ReadFrom(body)
	if err != nil {
		return mailbox.EmptyMessageID, fmt.Errorf("cannot read message body: %w", err)
	}
	if props.Size > 0 && read != int64(props.Size) {
		return mailbox.EmptyMessageID, fmt.Errorf("message body size advertised as %d bytes but read %d bytes from buffer", props.Size, read)
	}

	// IMAP server cannot accept the recent flag
	flags := lib.StripRecentFlag(props.Flags)

	var uid uint32
	if i.uidplusClient != nil {
		_, uid, err = i.uidplusClient.Append(name, flags, props.InternalDate, buffer)
	} else {
		err = i.client.Append(name, flags, props.InternalDate, buffer)
	}
	if err != nil {
		return mailbox.EmptyMessageID,
			fmt.Errorf("cannot append new message to IMAP server (mailbox=%q size=%d flags=%v): %w",
				name, read, flags, err,
			)
	}
	i.log.Printf("Message saved: mailbox=%q uid=%v size=%d flags=%v date=%q", name, uid, read, flags, props.InternalDate)

	return mailbox.NewMessageIDFromUint(uid), nil
}

func (i *Imap) FetchMessages(ctx context.Context, since time.Time, messages chan *mailbox.Message) error {
	defer close(messages)

	if i.selected == nil {
		return lib.ErrNotSelected
	}

	var seqset *imap.SeqSet

	if !since.IsZero() {
		// removes a day
		since = lib.SafePadding(since)
		i.log.Printf("searching for emails after %s", since)
		seqNums, err := i.client.Search(&imap.SearchCriteria{Since: since})
		if err != nil {
			i.log.Printf("error filtering emails by date: %s", err)
		}
		if len(seqNums) == 0 {
			// no message
			return nil
		}
		seqset = new(imap.SeqSet)
		seqset.AddNum(seqNums...)
	}
	if seqset == nil {
		// download all messages
		seqset = new(imap.SeqSet)
		seqset.AddRange(1, i.selected.Messages)
	}

	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchFlags, imap.FetchUid, imap.FetchInternalDate}
	i.log.Printf("items: %+v", items)

	receiver := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	// fetch messages in the background
	go func() {
		done <- i.client.Fetch(seqset, items, receiver)
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range receiver {
			i.log.Printf("Received IMAP message seq=%d flags=%+v date=%q", msg.SeqNum, msg.Flags, msg.InternalDate)
			// receive all the messages as they get in
			message := &mailbox.Message{
				MessageProperties: mailbox.MessageProperties{
					Flags:        lib.StripRecentFlag(msg.Flags),
					InternalDate: msg.InternalDate,
					Size:         msg.Size,
				},
				Uid:  mailbox.NewMessageIDFromUint(msg.Uid),
				Body: io.NopCloser(msg.GetBody(section)),
			}
			// and transfer them to the output
			messages <- message
		}
	}()
	// will return the error from Fetch when it's finished
	err := <-done
	wg.Wait()
	i.log.Print("All IMAP messages received")
	return err
}

func (i *Imap) UnselectMailbox() error {
	i.selected = nil
	return i.client.Unselect()
}

func (i *Imap) AddToHistory(info mailbox.Info, actions ...mailbox.HistoryAction) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, i.Delimiter())
	history, err := i.GetHistory(info)
	if err != nil {
		// just create a new file instead of failing
		history = &mailbox.History{
			Actions: make([]mailbox.HistoryAction, 0),
		}
	}
	history.Actions = append(history.Actions, actions...)

	return mailbox.SaveHistoryToFile(i.historyFile(name), history)
}

func (i *Imap) GetHistory(info mailbox.Info) (*mailbox.History, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, i.Delimiter())
	return mailbox.GetHistoryFromFile(i.historyFile(name))
}

func (i *Imap) historyFile(name string) string {
	filename := filepath.Join(i.cacheDir, i.tag)
	_ = os.MkdirAll(filename, 0700)
	return filepath.Join(filename, name+".history.json")
}
