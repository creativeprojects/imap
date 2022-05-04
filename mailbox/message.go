package mailbox

import (
	"io"
)

type Message struct {
	MessageProperties
	// The message unique identifier.
	Uid MessageID
	// The message body.
	Body io.ReadCloser
}
