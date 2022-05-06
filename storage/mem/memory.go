package mem

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"runtime"
	"sort"
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
	return NewWithLogger(nil)
}

func NewWithLogger(logger lib.Logger) *Backend {
	if logger == nil {
		logger = &lib.NoLog{}
	}
	return &Backend{
		data: make(map[string]*memMailbox),
		log:  logger,
	}
}

func (m *Backend) Close() error {
	m.data = make(map[string]*memMailbox)
	runtime.GC()
	return nil
}

func (s *Backend) Delimiter() string {
	return Delimiter
}

func (s *Backend) SupportMessageID() bool {
	return true
}

func (m *Backend) CreateMailbox(info mailbox.Info) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)

	if _, ok := m.data[name]; ok {
		// already exists
		return nil
	}

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

func (m *Backend) PutMessage(info mailbox.Info, props mailbox.MessageProperties, body io.Reader) (mailbox.MessageID, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	_, ok := m.data[name]
	if !ok {
		return mailbox.EmptyMessageID, lib.ErrMailboxNotFound
	}
	hasher := sha256.New()
	tee := io.TeeReader(body, hasher)
	buffer := &bytes.Buffer{}
	read, err := buffer.ReadFrom(tee)
	if err != nil {
		return mailbox.EmptyMessageID, fmt.Errorf("cannot read message source: %w", err)
	}
	if props.Size > 0 && read != int64(props.Size) {
		return mailbox.EmptyMessageID, fmt.Errorf("message body size advertised as %d bytes but read %d bytes from buffer", props.Size, read)
	}
	uid := m.data[name].newMessage(buffer.Bytes(), props.Flags, props.InternalDate, hasher.Sum(nil))
	return mailbox.NewMessageIDFromUint(uid), nil
}

func (m *Backend) FetchMessages(messages chan *mailbox.Message) error {
	defer close(messages)

	if m.selected == "" {
		return lib.ErrNotSelected
	}

	for uid, msg := range m.data[m.selected].messages {
		messages <- &mailbox.Message{
			MessageProperties: mailbox.MessageProperties{
				Flags:        msg.flags,
				InternalDate: msg.date,
				Size:         uint32(len(msg.content)),
				Hash:         msg.hash,
			},
			Uid:  mailbox.NewMessageIDFromUint(uid),
			Body: io.NopCloser(bytes.NewReader(msg.content)),
		}
	}

	return nil
}

func (m *Backend) UnselectMailbox() error {
	m.selected = ""
	return nil
}

func (m *Backend) AddToHistory(info mailbox.Info, actions ...mailbox.HistoryAction) error {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	_, ok := m.data[name]
	if !ok {
		return lib.ErrMailboxNotFound
	}
	if m.data[name].history == nil {
		m.data[name].history = make([]mailbox.HistoryAction, 0, len(actions))
	}
	m.data[name].history = append(m.data[name].history, actions...)
	return nil
}

func (m *Backend) GetHistory(info mailbox.Info) (*mailbox.History, error) {
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)
	_, ok := m.data[name]
	if !ok {
		return nil, lib.ErrMailboxNotFound
	}
	sort.SliceStable(m.data[name].history, func(i, j int) bool {
		return m.data[name].history[i].Date.Before(m.data[name].history[j].Date)
	})

	return &mailbox.History{
		Actions: m.data[name].history,
	}, nil
}

func (m *Backend) GenerateFakeEmails(info mailbox.Info, count uint32, minSize, maxSize int) {
	_ = m.CreateMailbox(info)
	name := lib.VerifyDelimiter(info.Name, info.Delimiter, Delimiter)

	var i uint32
	for i = 1; i <= count; i++ {
		msg := lib.GenerateEmail("user1@example.com", "user2@example.com", i, minSize, maxSize)
		hasher := sha256.New()
		m.data[name].newMessage(
			msg,
			lib.GenerateFlags(5),
			lib.GenerateDateFrom(time.Date(2010, 1, 1, 12, 0, 0, 0, time.Local)),
			hasher.Sum(msg),
		)
	}
}
