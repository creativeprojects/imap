package mem

import (
	"time"

	"github.com/creativeprojects/imap/mailbox"
)

type memMessage struct {
	content []byte
	flags   []string
	date    time.Time
	hash    []byte
}

type memMailbox struct {
	uidValidity uint32
	currentUid  uint32
	messages    map[uint32]*memMessage
	history     []mailbox.HistoryAction
}

func (m *memMailbox) newMessage(content []byte, flags []string, date time.Time, hash []byte) uint32 {
	m.currentUid++
	m.messages[m.currentUid] = &memMessage{
		content: content,
		flags:   flags,
		date:    date,
		hash:    hash,
	}
	return m.currentUid
}
