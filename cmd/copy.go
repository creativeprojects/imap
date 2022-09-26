package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/storage"
	"github.com/creativeprojects/imap/term"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
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

	var sourceLogger lib.Logger
	var destLogger lib.Logger

	if global.verbose {
		sourceLogger = log.New(os.Stdout, "source: ", 0)
		destLogger = log.New(os.Stdout, "dest: ", 0)
	}

	source := args[0]
	accountSource, ok := config.Accounts[source]
	if !ok {
		return fmt.Errorf("source account not found: %s", source)
	}
	backendSource, err := NewBackend(accountSource, sourceLogger)
	if err != nil {
		return fmt.Errorf("cannot open source backend: %w", err)
	}

	destination := args[1]
	accountDest, ok := config.Accounts[destination]
	if !ok {
		return fmt.Errorf("destination account not found: %s", destination)
	}
	backendDest, err := NewBackend(accountDest, destLogger)
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

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// load mailbox history
		history, err := backendDest.GetHistory(mbox)
		if err != nil {
			term.Infof("no history found on mailbox %s", mbox.Name)
		}
		var pbar *pterm.ProgressbarPrinter
		if !global.quiet && !global.verbose {
			pbar, _ = pterm.DefaultProgressbar.WithTotal(int(status.Messages)).Start()
		}
		entries, err := storage.CopyMessages(ctx, backendSource, backendDest, mbox, newProgresser(pbar), history)
		if pbar != nil {
			pbar.Add(pbar.Total - pbar.Current)
			_, _ = pbar.Stop()
		}
		if err != nil {
			term.Error(err.Error())
		}
		// we still save history even if an error occurred
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
