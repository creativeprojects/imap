package mdir

import (
	"os"
	"path/filepath"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/emersion/go-maildir"
)

const Delimiter = "."

type Maildir struct {
	root string
}

func New(root string) (*Maildir, error) {
	err := os.MkdirAll(root, 0700)
	if err != nil {
		return nil, err
	}
	return &Maildir{
		root: root,
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

func (m *Maildir) CreateMailbox(info mailbox.Info) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	mailbox := maildir.Dir(filepath.Join(m.root, name))
	err := mailbox.Init()
	if err != nil {
		return err
	}
	return nil
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
	return os.RemoveAll(filepath.Join(m.root, name))
}
