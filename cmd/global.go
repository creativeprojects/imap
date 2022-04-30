package cmd

import "github.com/creativeprojects/imap/cfg"

type GlobalFlags struct {
	configFile string
	quiet      bool
	verbose    bool
}

var (
	global GlobalFlags
	config *cfg.Config
)
