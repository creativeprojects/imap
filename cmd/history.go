package cmd

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/term"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Display history of mailbox copy",
	RunE:  runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("missing account name")
	}
	accountName := args[0]
	account, ok := config.Accounts[accountName]
	if !ok {
		return fmt.Errorf("account not found: %s", accountName)
	}
	backend, err := NewBackend(account, nil)
	if err != nil {
		return fmt.Errorf("cannot open backend: %w", err)
	}

	mailboxes, err := backend.ListMailbox()
	if err != nil {
		return fmt.Errorf("cannot list account mailbox: %w", err)
	}

	if len(mailboxes) == 0 {
		term.Warn("No mailbox found on this account\n")
	}

	for _, mailbox := range mailboxes {
		term.Infof("%s:\n", mailbox.Name)
		history, err := backend.GetHistory(mailbox)
		if err != nil {
			term.Error(err)
		}
		displayHistory(history)
	}
	return nil
}

func displayHistory(history *mailbox.History) {
	table := pterm.DefaultTable.WithBoxed(true).WithHasHeader().WithData(pterm.TableData{
		{"Date", "Action", "Source", "Messages"},
	})
	for _, action := range history.Actions {
		table.Data = append(table.Data, []string{
			action.Date.Format("2006-01-02 15:04:05 MST"),
			action.Action,
			action.SourceAccountTag[0:16],
			strconv.Itoa(len(action.Entries)),
		})
	}
	table.Render()
}
