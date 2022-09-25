package cmd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/storage"
	"github.com/creativeprojects/imap/term"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var duplicatesCmd = &cobra.Command{
	Use:   "duplicates",
	Short: "Find duplicate emails across mailboxes (in the same account)",
	RunE:  runDuplicates,
}

func init() {
	rootCmd.AddCommand(duplicatesCmd)
}

func runDuplicates(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("missing account name")
	}

	accountName := args[0]
	accountSource, ok := config.Accounts[accountName]
	if !ok {
		return fmt.Errorf("account not found: %s", accountName)
	}
	backend, err := NewBackend(accountSource, nil)
	if err != nil {
		return fmt.Errorf("cannot open backend: %w", err)
	}

	mailboxes, err := backend.ListMailbox()
	if err != nil {
		return fmt.Errorf("cannot list source account mailbox: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	duplicates := 0
	hashes := make(map[string][]mailbox.Message, 0)
	for _, mbox := range mailboxes {
		status, err := backend.SelectMailbox(mbox)
		if err != nil {
			continue
		}
		if status.Messages == 0 {
			// it's empty so don't bother
			continue
		}
		term.Infof("reading mailbox %s", mbox.Name)
		pbar, _ := pterm.DefaultProgressbar.WithTotal(int(status.Messages)).Start()
		entries, err := storage.LoadMessageProperties(ctx, backend, mbox, newProgresser(pbar))
		if pbar != nil {
			_, _ = pbar.Stop()
		}
		if err != nil {
			term.Error(err.Error())
		}

		for _, entry := range entries {
			key := hex.EncodeToString(entry.Hash)
			if key == "" {
				term.Errorf("missing hash on message %v", entry.Uid.Value())
				continue
			}
			if _, found := hashes[key]; found {
				// duplicate
				duplicates++
				hashes[key] = append(hashes[key], entry)
				continue
			}
			hashes[key] = []mailbox.Message{entry}
		}
	}

	fmt.Printf("total of %d unique messages\n", len(hashes))
	if duplicates == 0 {
		fmt.Print("no duplicate message\n")
	} else if duplicates == 1 {
		fmt.Print("found 1 duplicate message\n")
	} else {
		fmt.Printf("found %d duplicate messages\n", duplicates)
	}
	return nil
}
