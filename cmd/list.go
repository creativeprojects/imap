package cmd

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Display list of mailboxes",
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
	backend, err := NewBackend(account)
	if err != nil {
		return fmt.Errorf("cannot open backend: %w", err)
	}
	if global.verbose {
		backend.DebugLogger(log.Default())
	}

	mailboxes, err := backend.ListMailbox()
	if err != nil {
		return fmt.Errorf("cannot list account mailbox: %w", err)
	}
	table := pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData{
		{"Mailbox", "Messages", "Flags"},
	})
	for _, mailbox := range mailboxes {
		var messages, flags string
		status, err := backend.SelectMailbox(mailbox)
		if err == nil {
			messages = strconv.FormatUint(uint64(status.Messages), 10)
			flags = displayFlags(status.Flags)
		}
		table.Data = append(table.Data, []string{mailbox.Name, messages, flags})
	}
	return table.Render()
}

func displayFlags(source []string) string {
	flags := make([]string, len(source))
	for i, flag := range source {
		flags[i] = strings.TrimPrefix(flag, "\\")
	}
	return strings.Join(flags, ", ")
}
