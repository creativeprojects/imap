package mailbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

type History struct {
	Actions []HistoryAction
}

type HistoryAction struct {
	SourceAccountTag string
	Date             time.Time
	Action           string
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

func GetHistoryFromFile(filename string) (*History, error) {
	history := &History{}
	file, err := os.Open(filename)
	if err != nil {
		// return nil, fmt.Errorf("cannot open history file: %w", err)
		// return an empty history instead
		return history, nil
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(history)
	if err != nil {
		return nil, fmt.Errorf("error reading history file: %w", err)
	}

	sort.SliceStable(history.Actions, func(i, j int) bool {
		return history.Actions[i].Date.Before(history.Actions[j].Date)
	})
	return history, nil
}

func SaveHistoryToFile(filename string, history *History) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("cannot save history: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(history)
	if err != nil {
		return fmt.Errorf("cannot encode history: %w", err)
	}
	return nil
}

func FindHistoryEntryFromSourceID(history *History, sourceMessageID MessageID) *HistoryEntry {
	if history == nil {
		return nil
	}
	for _, action := range history.Actions {
		for _, entry := range action.Entries {
			if entry.SourceID == sourceMessageID {
				return &entry
			}
		}
	}
	return nil
}

func FindLastAction(history *History) time.Time {
	last := time.Time{}
	if history == nil {
		return last
	}
	if len(history.Actions) == 0 {
		return last
	}
	for _, action := range history.Actions {
		if action.Date.After(last) {
			last = action.Date
		}
	}
	return last
}
