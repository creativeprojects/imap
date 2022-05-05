package mailbox

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type History struct {
	Actions []HistoryAction
}

type HistoryAction struct {
	SourceAccountTag string
	Date             time.Time
	Action           string
	Mailbox          string
	UidValidity      uint32
	Entries          []HistoryEntry
}

type HistoryEntry struct {
	SourceID  MessageID
	MessageID MessageID
}

func AccountTag(serverURL, username string) string {
	hasher := sha256.New()
	hasher.Write([]byte(username))
	hasher.Write([]byte(":"))
	hasher.Write([]byte(serverURL))
	hasher.Write([]byte("\n"))
	return hex.EncodeToString(hasher.Sum(nil))
}

const (
	ActionCopy = "COPY"
)
