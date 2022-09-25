package storage

import (
	"context"
	"fmt"

	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/term"
)

func CopyMessages(ctx context.Context, backendSource, backendDest Backend, mbox mailbox.Info, pbar Progresser, history *mailbox.History) ([]mailbox.HistoryEntry, error) {
	err := backendDest.CreateMailbox(mbox)
	if err != nil {
		return nil, fmt.Errorf("cannot create mailbox at destination: %w", err)
	}

	entries := make([]mailbox.HistoryEntry, 0)

	receiver := make(chan *mailbox.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- backendSource.FetchMessages(ctx, receiver)
	}()

	for msg := range receiver {
		if pbar != nil {
			pbar.Increment()
		}
		id, err := copyMessage(ctx, msg, backendDest, mbox, history)
		if err != nil || id == nil {
			// don't save this entry in history
			continue
		}
		entries = append(entries, mailbox.HistoryEntry{
			SourceID:  msg.Uid,
			MessageID: *id,
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

func copyMessage(ctx context.Context, msgSource *mailbox.Message, backendDest Backend, mboxDest mailbox.Info, history *mailbox.History) (*mailbox.MessageID, error) {
	defer msgSource.Body.Close()

	if previousEntry := mailbox.FindHistoryEntryFromSourceID(history, msgSource.Uid); previousEntry != nil {
		// message ID already copied
		return nil, nil
	}
	props := mailbox.MessageProperties{
		Flags:        msgSource.Flags,
		InternalDate: msgSource.InternalDate,
		Size:         msgSource.Size,
		Hash:         msgSource.Hash,
	}
	id, err := backendDest.PutMessage(mboxDest, props, msgSource.Body)
	if err != nil {
		// display error but keep going
		term.Errorf("error saving message: %s", err)
	}
	return &id, err
}
