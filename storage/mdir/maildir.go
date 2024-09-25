package mdir

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/emersion/go-maildir"
)

const Delimiter = "."

type Maildir struct {
	root     string
	log      lib.Logger
	selected string
}

func New(root string) (*Maildir, error) {
	return NewWithLogger(root, nil)
}

func NewWithLogger(root string, logger lib.Logger) (*Maildir, error) {
	if runtime.GOOS == "windows" {
		return nil, errors.New("maildir is not supported on Windows")
	}
	if logger == nil {
		logger = &lib.NoLog{}
	}
	err := os.MkdirAll(root, 0700)
	if err != nil {
		return nil, err
	}

	return &Maildir{
		root: root,
		log:  logger,
	}, nil
}

func (m *Maildir) Close() error {
	return nil
}

func (m *Maildir) Root() string {
	return m.root
}

// AccountID is an internal ID used to tag accounts in history
func (s *Maildir) AccountID() string {
	metadata, _ := s.getMetadata()
	if metadata == nil || metadata.AccountID == "" {
		metadata = &AccountMetadata{
			AccountID: lib.RandomTag(s.root),
		}
		_ = s.setMetadata(metadata)
	}
	return metadata.AccountID
}

func (s *Maildir) Delimiter() string {
	return Delimiter
}

func (s *Maildir) SupportMessageID() bool {
	return true
}

func (s *Maildir) SupportMessageHash() bool {
	return false
}

// CreateMailbox doesn't return an error if the mailbox already exists
func (m *Maildir) CreateMailbox(info mailbox.Info) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	dirName := filepath.Join(m.root, name)
	if _, err := os.Stat(dirName); err == nil || errors.Is(err, fs.ErrExist) {
		// mailbox already exists
		return nil
	}
	mbox := maildir.Dir(dirName)
	err := mbox.Init()
	if err != nil {
		return err
	}
	// default status on new mailbox
	err = m.setMailboxStatus(name, mailbox.Status{
		Name:        name,
		UidValidity: lib.NewUID(),
	})
	if err != nil {
		return err
	}
	// and sets an empty history
	return m.AddToHistory(info)
}

func (m *Maildir) ListMailbox() ([]mailbox.Info, error) {
	list := make([]mailbox.Info, 0)
	files, err := os.ReadDir(m.root)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		list = append(list, mailbox.Info{
			Delimiter: ".",
			Name:      file.Name(),
		})
	}
	return list, nil
}

func (m *Maildir) DeleteMailbox(info mailbox.Info) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	_ = os.Remove(m.statusFile(name))
	_ = os.Remove(m.historyFile(name))
	return os.RemoveAll(filepath.Join(m.root, name))
}

func (m *Maildir) SelectMailbox(info mailbox.Info) (*mailbox.Status, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, m.Delimiter())
	if !m.mailboxExists(name) {
		return nil, lib.ErrMailboxNotFound
	}
	m.selected = name
	return m.getMailboxStatus(name)
}

func (m *Maildir) PutMessage(info mailbox.Info, props mailbox.MessageProperties, body io.Reader) (mailbox.MessageID, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	mbox := maildir.Dir(filepath.Join(m.root, name))
	msg, copied, err := m.createFromStream(mbox, props.Flags, body)
	if err != nil {
		return mailbox.EmptyMessageID, err
	}
	if props.Size > 0 && copied != int64(props.Size) {
		// delete the message
		filename := msg.Filename()
		_ = os.Remove(filename)
		return mailbox.EmptyMessageID, fmt.Errorf("message body size advertised as %d bytes but read %d bytes from buffer", props.Size, copied)
	}
	m.log.Printf("Message saved: mailbox=%q key=%q size=%d flags=%v date=%q", name, msg, copied, props.Flags, props.InternalDate)

	filename := msg.Filename()
	_ = os.Chtimes(filename, time.Now(), props.InternalDate)

	status, err := m.getMailboxStatus(name)
	if err != nil {
		return mailbox.EmptyMessageID, err
	}
	status.Messages++
	err = m.setMailboxStatus(name, *status)
	if err != nil {
		return mailbox.EmptyMessageID, err
	}
	return mailbox.NewMessageIDFromString(msg.Key()), nil
}

func (m *Maildir) createFromStream(mbox maildir.Dir, flags []string, body io.Reader) (*maildir.Message, int64, error) {
	msg, writer, err := mbox.Create(toFlags(flags))
	if err != nil {
		return msg, 0, err
	}
	defer writer.Close()
	copied, err := io.Copy(writer, body)
	if err != nil {
		return msg, copied, err
	}
	return msg, copied, nil
}

func (m *Maildir) FetchMessages(ctx context.Context, since time.Time, messages chan *mailbox.Message) error {
	defer close(messages)

	if m.selected == "" {
		return lib.ErrNotSelected
	}

	// removes a day
	since = lib.SafePadding(since)

	name := m.selected
	mbox := maildir.Dir(filepath.Join(m.root, name))
	msgs, err := mbox.Messages()
	if err != nil {
		return err
	}

	for _, msg := range msgs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		flags := msg.Flags()
		filename := msg.Filename()
		info, err := os.Stat(filename)
		if err != nil {
			return fmt.Errorf("cannot stat %q: %w", filename, err)
		}
		if !since.IsZero() && info.ModTime().Before(since) {
			// skip this message
			continue
		}
		file, err := msg.Open()
		if err != nil {
			return fmt.Errorf("cannot open key %q: %w", msg, err)
		}
		messages <- &mailbox.Message{
			MessageProperties: mailbox.MessageProperties{
				Flags:        flagsToStrings(flags),
				InternalDate: info.ModTime(),
				Size:         uint32(info.Size()),
			},
			Uid:  mailbox.NewMessageIDFromString(msg.Key()),
			Body: file,
		}
	}
	return nil
}

// LatestDate returns the internal date of the latest message
func (m *Maildir) LatestDate(ctx context.Context) (time.Time, error) {
	latest := time.Time{}

	if m.selected == "" {
		return latest, lib.ErrNotSelected
	}

	mbox := maildir.Dir(filepath.Join(m.root, m.selected))
	msgs, err := mbox.Messages()
	if err != nil {
		return latest, err
	}

	for _, msg := range msgs {
		if ctx.Err() != nil {
			return latest, ctx.Err()
		}
		filename := msg.Filename()
		info, err := os.Stat(filename)
		if err != nil {
			// should we keep going after an error?
			return latest, fmt.Errorf("cannot stat %q: %w", filename, err)
		}
		if latest.Before(info.ModTime()) {
			latest = info.ModTime()
		}
	}

	return latest, nil
}

func (m *Maildir) UnselectMailbox() error {
	m.selected = ""
	return nil
}

func (m *Maildir) AddToHistory(info mailbox.Info, actions ...mailbox.HistoryAction) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	history, err := m.GetHistory(info)
	if err != nil {
		// just create a new file instead of failing
		history = &mailbox.History{
			Actions: make([]mailbox.HistoryAction, 0),
		}
	}
	history.Actions = append(history.Actions, actions...)

	return mailbox.SaveHistoryToFile(m.historyFile(name), history)
}

func (m *Maildir) GetHistory(info mailbox.Info) (*mailbox.History, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	if !m.mailboxExists(name) {
		return nil, lib.ErrMailboxNotFound
	}
	return mailbox.GetHistoryFromFile(m.historyFile(name))
}

func (m *Maildir) mailboxExists(name string) bool {
	stat, err := os.Stat(filepath.Join(m.root, name))
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return stat.IsDir()
}

func (m *Maildir) metadataFile() string {
	return filepath.Join(m.root, ".account.metadata.json")
}

func (m *Maildir) statusFile(name string) string {
	return filepath.Join(m.root, name+".json")
}

func (m *Maildir) historyFile(name string) string {
	return filepath.Join(m.root, name+".history.json")
}

func (m *Maildir) setMailboxStatus(name string, status mailbox.Status) error {
	file, err := os.Create(m.statusFile(name))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(status)
	if err != nil {
		return err
	}

	return nil
}

func (m *Maildir) getMailboxStatus(name string) (*mailbox.Status, error) {
	file, err := os.Open(m.statusFile(name))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", lib.ErrStatusNotFound, err)
	}
	defer file.Close()

	status := &mailbox.Status{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(status)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", lib.ErrStatusNotFound, err)
	}

	return status, nil
}

func (m *Maildir) setMetadata(metadata *AccountMetadata) error {
	file, err := os.Create(m.metadataFile())
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(metadata)
	if err != nil {
		return err
	}

	return nil
}

func (m *Maildir) getMetadata() (*AccountMetadata, error) {
	file, err := os.Open(m.metadataFile())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", lib.ErrStatusNotFound, err)
	}
	defer file.Close()

	metadata := &AccountMetadata{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(metadata)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", lib.ErrStatusNotFound, err)
	}

	return metadata, nil
}
