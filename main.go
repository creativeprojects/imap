package main

import (
	"github.com/creativeprojects/imap/cmd"
)

// These fields are populated by the goreleaser build
var (
	buildVersion = "0.1.0-dev"
	buildCommit  = "---"
	buildDate    = "today"
	buildBy      = "dev"
)

func main() {
	cmd.Execute(buildVersion, buildCommit, buildDate, buildBy)
}
