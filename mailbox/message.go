package mailbox

import (
	"io"
	"time"
)

type Message struct {
	// The message sequence number.
	SeqNum uint32
	// The message flags.
	Flags []string
	// The date the message was received by the server.
	InternalDate time.Time
	// The message size.
	Size uint32
	// The message unique identifier.
	Uid MessageID
	// The message body.
	Body io.ReadCloser
}
