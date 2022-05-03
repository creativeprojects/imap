package mem

import (
	"bytes"
	"io"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
)

const Delimiter = "."

type Backend struct {
	data     map[string]*memMailbox
	log      lib.Logger
	selected string
}

func New() *Backend {
	return &Backend{
		data: make(map[string]*memMailbox),
		log:  &lib.NoLog{},
	}
}

func (m *Backend) Close() error {
	return nil
}

func (m *Backend) DebugLogger(logger lib.Logger) {
	m.log = logger
}

func (s *Backend) Delimiter() string {
	return Delimiter
}

func (s *Backend) SupportMessageID() bool {
	return true
}

func (m *Backend) CreateMailbox(info mailbox.Info) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)

	m.data[name] = &memMailbox{
		uidValidity: lib.NewUID(),
		messages:    make(map[uint32]*memMessage),
	}
	return nil
}

func (m *Backend) ListMailbox() ([]mailbox.Info, error) {
	list := make([]mailbox.Info, len(m.data))
	index := 0
	for name := range m.data {
		list[index] = mailbox.Info{
			Delimiter: Delimiter,
			Name:      name,
		}
		index++
	}
	return list, nil
}

func (m *Backend) DeleteMailbox(info mailbox.Info) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	delete(m.data, name)
	return nil
}

func (m *Backend) SelectMailbox(info mailbox.Info) (*mailbox.Status, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, m.Delimiter())
	mbox, ok := m.data[name]
	if !ok {
		return nil, lib.ErrMailboxNotFound
	}
	m.selected = name
	return &mailbox.Status{
		Name:        name,
		Messages:    uint32(len(mbox.messages)),
		Unseen:      0,
		UidValidity: mbox.uidValidity,
	}, nil
}

func (m *Backend) PutMessage(info mailbox.Info, flags []string, date time.Time, body io.Reader) (mailbox.MessageID, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	_, ok := m.data[name]
	if !ok {
		return mailbox.EmptyMessageID, lib.ErrMailboxNotFound
	}
	buffer := &bytes.Buffer{}
	buffer.ReadFrom(body)
	uid := m.data[name].newMessage(buffer.Bytes(), flags, date)
	return mailbox.NewMessageIDFromUint(uid), nil
}

func (m *Backend) FetchMessages(messages chan *mailbox.Message) error {
	defer close(messages)

	if m.selected == "" {
		return lib.ErrNotSelected
	}

	var count uint32
	for uid, msg := range m.data[m.selected].messages {
		count++
		messages <- &mailbox.Message{
			SeqNum:       count,
			Flags:        msg.flags,
			InternalDate: msg.date,
			Size:         uint32(len(msg.content)),
			Uid:          mailbox.NewMessageIDFromUint(uid),
			Body:         io.NopCloser(bytes.NewReader(msg.content)),
		}
	}

	return nil
}

func (m *Backend) UnselectMailbox() error {
	m.selected = ""
	return nil
}
