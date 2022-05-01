package mdir

import "github.com/emersion/go-maildir"

func toFlags(source []string) []maildir.Flag {
	flags := make([]maildir.Flag, 0)
	for _, sourceFlag := range source {
		switch sourceFlag {
		case "\\Seen":
			flags = append(flags, maildir.FlagSeen)
		}
	}
	return flags
}
