package storage

import (
	"fmt"

	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/term"
)

func CopyMessages(backendSource, backendDest Backend, mbox mailbox.Info, pbar Progresser, history *mailbox.History) ([]mailbox.HistoryEntry, error) {
	err := backendDest.CreateMailbox(mbox)
	if err != nil {
		return nil, fmt.Errorf("cannot create mailbox at destination: %w", err)
	}

	entries := make([]mailbox.HistoryEntry, 0)

	receiver := make(chan *mailbox.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- backendSource.FetchMessages(receiver)
	}()

	for msg := range receiver {
		if pbar != nil {
			pbar.Increment()
		}
		if previousEntry := mailbox.FindHistoryEntryFromSourceID(history, msg.Uid); previousEntry != nil {
			// message ID already copied
			continue
		}
		props := mailbox.MessageProperties{
			Flags:        msg.Flags,
			InternalDate: msg.InternalDate,
			Size:         msg.Size,
			Hash:         msg.Hash,
		}
		id, err := backendDest.PutMessage(mbox, props, msg.Body)
		_ = msg.Body.Close()
		if err != nil {
			// display error but keep going
			term.Errorf("error saving message: %s", err)
		}
		entries = append(entries, mailbox.HistoryEntry{
			SourceID:  msg.Uid,
			MessageID: id,
		})
	}
	// wait until all the messages arrived
	err = <-done
	_ = backendSource.UnselectMailbox()
	if err != nil {
		return entries, fmt.Errorf("error loading messages: %w", err)
	}
	return entries, nil
}
