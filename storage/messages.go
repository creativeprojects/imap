package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/creativeprojects/imap/mailbox"
)

func LoadMessageProperties(ctx context.Context, backend Backend, mbox mailbox.Info, pbar Progresser) ([]mailbox.Message, error) {
	messages := make([]mailbox.Message, 0)

	receiver := make(chan *mailbox.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- backend.FetchMessages(ctx, time.Time{}, receiver)
	}()

	for msg := range receiver {
		if pbar != nil {
			pbar.Increment()
		}
		if len(msg.Hash) == 0 {
			// calculate the hash now
			hasher := sha256.New()
			_, err := io.Copy(hasher, msg.Body)
			if err != nil {
				return messages, fmt.Errorf("error reading message %v: %w", msg.Uid.Value(), err)
			}
			msg.Hash = hasher.Sum(nil)
		}
		_ = msg.Body.Close()
		msg.Body = nil
		messages = append(messages, *msg)
	}
	// wait until all the messages arrived
	err := <-done
	_ = backend.UnselectMailbox()
	if err != nil {
		return messages, fmt.Errorf("error loading messages: %w", err)
	}
	return messages, nil
}
