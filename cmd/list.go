package cmd

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/creativeprojects/imap/term"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Display list of mailboxes in an account",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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

	accountID := backend.AccountID()
	if accountID != "" {
		term.Debugf("Account ID: %s", accountID)
	}

	mailboxes, err := backend.ListMailbox()
	if err != nil {
		return fmt.Errorf("cannot list account mailbox: %w", err)
	}
	if len(mailboxes) == 0 {
		term.Warn("No mailbox found on this account")
		return nil
	}
	table := pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData{
		{"Mailbox", "Messages"},
	})
	for _, mailbox := range mailboxes {
		var messages string
		status, err := backend.SelectMailbox(mailbox)
		if err == nil {
			messages = strconv.FormatUint(uint64(status.Messages), 10)
		}
		table.Data = append(table.Data, []string{mailbox.Name, messages})
	}
	return table.Render()
}
