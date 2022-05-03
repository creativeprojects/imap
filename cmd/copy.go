package cmd

import (
	"errors"
	"fmt"
	"log"

	"github.com/creativeprojects/imap/mailbox"
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
	backendSource, err := NewBackend(accountSource)
	if err != nil {
		return fmt.Errorf("cannot open source backend: %w", err)
	}
	if global.verbose {
		backendSource.DebugLogger(log.Default())
	}

	destination := args[1]
	accountDest, ok := config.Accounts[destination]
	if !ok {
		return fmt.Errorf("destination account not found: %s", destination)
	}
	backendDest, err := NewBackend(accountDest)
	if err != nil {
		return fmt.Errorf("cannot open destination backend: %w", err)
	}
	if global.verbose {
		backendDest.DebugLogger(log.Default())
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
		err = copyMessages(backendSource, backendDest, mbox, newProgresser(pbar))
		if pbar != nil {
			_, _ = pbar.Stop()
		}
		if err != nil {
			term.Error(err.Error())
		}
	}
	return nil
}

func copyMessages(backendSource, backendDest Backend, mbox mailbox.Info, pbar Progresser) error {
	err := backendDest.CreateMailbox(mbox)
	if err != nil {
		return fmt.Errorf("cannot create mailbox at destination: %w", err)
	}

	receiver := make(chan *mailbox.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- backendSource.FetchMessages(receiver)
	}()

	for msg := range receiver {
		if pbar != nil {
			pbar.Increment()
		}
		_, err = backendDest.PutMessage(mbox, msg.Flags, msg.InternalDate, msg.Body)
		msg.Body.Close()
		if err != nil {
			// display error but keep going
			term.Errorf("error saving message: %s", err)
		}
	}
	// wait until all the messages arrived
	err = <-done
	_ = backendSource.UnselectMailbox()
	if err != nil {
		return fmt.Errorf("error loading messages: %w", err)
	}
	return nil
}
