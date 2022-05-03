package mem

import "time"

type memMessage struct {
	content []byte
	flags   []string
	date    time.Time
}

type memMailbox struct {
	uidValidity uint32
	currentUid  uint32
	messages    map[uint32]*memMessage
}

func (m *memMailbox) newMessage(content []byte, flags []string, date time.Time) uint32 {
	m.currentUid++
	m.messages[m.currentUid] = &memMessage{
		content: content,
		flags:   flags,
		date:    date,
	}
	return m.currentUid
}
