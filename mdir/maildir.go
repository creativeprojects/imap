package mdir

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/emersion/go-maildir"
)

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

func (m *Maildir) CreateMailbox(info mailbox.Info) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, "/")
	mailbox := maildir.Dir(filepath.Join(m.root, name))
	err := mailbox.Init()
	if err != nil {
		return err
	}
	return nil
}

func (m *Maildir) ListMailbox() ([]mailbox.Info, error) {
	return nil, errors.New("not implemented")
}
