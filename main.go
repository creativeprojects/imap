package main

import (
	"github.com/creativeprojects/imap/cmd"
)

// These fields are populated by the goreleaser build
var (
	version = "0.3.0-dev"
	commit  = "---"
	date    = "today"
	builtBy = "dev"
)

func main() {
	cmd.Execute(version, commit, date, builtBy)
}
