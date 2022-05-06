package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/storage"
	"github.com/creativeprojects/imap/term"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy an account mailboxes to another one",
	RunE:  runCopy,
}

func init() {
	rootCmd.AddCommand(copyCmd)
}

func runCopy(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("missing account names (source and destination)")
	} else if len(args) < 2 {
		return errors.New("missing destination account name")
	}

	source := args[0]
	accountSource, ok := config.Accounts[source]
	if !ok {
		return fmt.Errorf("source account not found: %s", source)
	}
	backendSource, err := NewBackend(accountSource, nil)
	if err != nil {
		return fmt.Errorf("cannot open source backend: %w", err)
	}

	destination := args[1]
	accountDest, ok := config.Accounts[destination]
	if !ok {
		return fmt.Errorf("destination account not found: %s", destination)
	}
	backendDest, err := NewBackend(accountDest, nil)
	if err != nil {
		return fmt.Errorf("cannot open destination backend: %w", err)
	}

	mailboxes, err := backendSource.ListMailbox()
	if err != nil {
		return fmt.Errorf("cannot list source account mailbox: %w", err)
	}

	for _, mbox := range mailboxes {
		status, err := backendSource.SelectMailbox(mbox)
		if err != nil {
			continue
		}
		if status.Messages == 0 {
			// it's empty so don't bother
			continue
		}
		term.Infof("copying mailbox %s", mbox.Name)
		pbar, _ := pterm.DefaultProgressbar.WithTotal(int(status.Messages)).Start()
		entries, err := storage.CopyMessages(backendSource, backendDest, mbox, newProgresser(pbar))
		if pbar != nil {
			_, _ = pbar.Stop()
		}
		if err != nil {
			term.Error(err.Error())
		}
		if len(entries) > 0 {
			action := mailbox.HistoryAction{
				SourceAccountTag: mailbox.AccountTag(accountSource.ServerURL, accountSource.Username),
				Date:             time.Now(),
				Action:           mailbox.ActionCopy,
				UidValidity:      status.UidValidity,
				Entries:          entries,
			}
			err = backendDest.AddToHistory(mbox, action)
			if err != nil {
				term.Error(err.Error())
			}
		}
	}
	return nil
}
