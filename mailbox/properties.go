package mailbox

import "time"

type MessageProperties struct {
	// The message flags.
	Flags []string
	// The date the message was received by the server.
	InternalDate time.Time
	// The message size.
	Size uint32
	// The message Hash (if available)
	Hash []byte
}
