package cmd

import (
	"os"

	"github.com/creativeprojects/imap/cfg"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/mdir"
	"github.com/creativeprojects/imap/term"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "imap",
	Short: "IMAP tools: backup, copy, move",
	Long:  "\nIMAP tools: backup, copy, move",
	Run: func(cmd *cobra.Command, args []string) {
		// this function needs to be defined
		backend, err := mdir.New("./maildir")
		if err != nil {
			term.Error(err)
		}
		err = backend.CreateMailbox(mailbox.Info{Name: "INBOX", Delimiter: "/"})
		if err != nil {
			term.Error(err)
		}
	},
}

func init() {
	cobra.OnInitialize(initConfig, initLog)
	flag := rootCmd.PersistentFlags()
	flag.StringVarP(&global.configFile, "config", "c", "imap.yaml", "configuration file")
	flag.BoolVarP(&global.quiet, "quiet", "q", false, "only display warnings and errors")
	flag.BoolVarP(&global.verbose, "verbose", "v", false, "display debugging information")
}

func initConfig() {
	var err error
	config, err = cfg.LoadFromFile(global.configFile)
	if err != nil {
		term.Errorf("cannot open or read configuration file: %s", err)
		os.Exit(1)
	}
}

func initLog() {
	switch {
	case global.verbose:
		term.SetLevel(term.LevelDebug)
	case global.quiet:
		term.SetLevel(term.LevelWarn)
	}
	term.Info("IMAP tools")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		term.Error(err)
		os.Exit(1)
	}
}
