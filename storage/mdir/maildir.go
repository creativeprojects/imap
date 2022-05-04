package mdir

import (
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

func (s *Maildir) Delimiter() string {
	return Delimiter
}

func (s *Maildir) SupportMessageID() bool {
	return true
}

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
	return m.setMailboxStatus(name, mailbox.Status{
		Name:        name,
		UidValidity: lib.NewUID(),
	})
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
	return os.RemoveAll(filepath.Join(m.root, name))
}

func (m *Maildir) SelectMailbox(info mailbox.Info) (*mailbox.Status, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, m.Delimiter())
	m.selected = name
	return m.getMailboxStatus(name)
}

func (m *Maildir) PutMessage(info mailbox.Info, props mailbox.MessageProperties, body io.Reader) (mailbox.MessageID, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	mbox := maildir.Dir(filepath.Join(m.root, name))
	key, copied, err := m.createFromStream(mbox, props.Flags, body)
	if err != nil {
		return mailbox.EmptyMessageID, err
	}
	if props.Size > 0 && copied != int64(props.Size) {
		// delete the message
		filename, err := mbox.Filename(key)
		if err == nil {
			_ = os.Remove(filename)
		}
		return mailbox.EmptyMessageID, fmt.Errorf("message body size advertised as %d bytes but read %d bytes from buffer", props.Size, copied)
	}
	m.log.Printf("Message saved: mailbox=%q key=%q size=%d", name, key, copied)

	filename, err := mbox.Filename(key)
	if err == nil {
		_ = os.Chtimes(filename, time.Now(), props.InternalDate)
	}

	status, err := m.getMailboxStatus(name)
	if err != nil {
		return mailbox.EmptyMessageID, err
	}
	status.Messages++
	err = m.setMailboxStatus(name, *status)
	if err != nil {
		return mailbox.EmptyMessageID, err
	}
	return mailbox.NewMessageIDFromString(key), nil
}

func (m *Maildir) createFromStream(mbox maildir.Dir, flags []string, body io.Reader) (string, int64, error) {
	key, writer, err := mbox.Create(toFlags(flags))
	if err != nil {
		return key, 0, err
	}
	defer writer.Close()
	copied, err := io.Copy(writer, body)
	if err != nil {
		return key, copied, err
	}
	return key, copied, nil
}

func (m *Maildir) FetchMessages(messages chan *mailbox.Message) error {
	defer close(messages)

	if m.selected == "" {
		return lib.ErrNotSelected
	}

	name := m.selected
	mbox := maildir.Dir(filepath.Join(m.root, name))
	keys, err := mbox.Keys()
	if err != nil {
		return err
	}

	for _, key := range keys {
		flags, err := mbox.Flags(key)
		if err != nil {
			return err
		}
		filename, err := mbox.Filename(key)
		if err != nil {
			return err
		}
		info, err := os.Stat(filename)
		if err != nil {
			return err
		}
		file, err := mbox.Open(key)
		if err != nil {
			return err
		}
		messages <- &mailbox.Message{
			MessageProperties: mailbox.MessageProperties{
				Flags:        flagsToStrings(flags),
				InternalDate: info.ModTime(),
				Size:         uint32(info.Size()),
			},
			Uid:  mailbox.NewMessageIDFromString(key),
			Body: file,
		}
	}
	return nil
}

func (m *Maildir) UnselectMailbox() error {
	m.selected = ""
	return nil
}

func (m *Maildir) statusFile(name string) string {
	return filepath.Join(m.root, name+".json")
}

func (m *Maildir) infoFile() string {
	return filepath.Join(m.root, ".info.json")
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