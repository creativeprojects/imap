package mailbox

import (
	"io"
	"time"
)

type Message struct {
	// The message sequence number. It must be greater than or equal to 1.
	SeqNum uint32
	// The message flags.
	Flags []string
	// The date the message was received by the server.
	InternalDate time.Time
	// The message size.
	Size uint32
	// The message unique identifier. It must be greater than or equal to 1.
	Uid uint32
	// The message body.
	Body io.Reader
}
