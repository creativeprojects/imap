package mdir

import (
	"github.com/emersion/go-imap"
	"github.com/emersion/go-maildir"
)

func toFlags(source []string) []maildir.Flag {
	flags := make([]maildir.Flag, 0, len(source))
	for _, sourceFlag := range source {
		switch sourceFlag {
		case imap.SeenFlag:
			flags = append(flags, maildir.FlagSeen)

		case imap.AnsweredFlag:
			flags = append(flags, maildir.FlagReplied)

		case imap.FlaggedFlag:
			flags = append(flags, maildir.FlagFlagged)

		case imap.DeletedFlag:
			flags = append(flags, maildir.FlagTrashed)

		case imap.DraftFlag:
			flags = append(flags, maildir.FlagDraft)
		}
	}
	return flags
}

func flagsToStrings(source []maildir.Flag) []string {
	flags := make([]string, 0, len(source))
	for _, sourceFlag := range source {
		switch sourceFlag {
		case maildir.FlagSeen:
			flags = append(flags, imap.SeenFlag)

		case maildir.FlagReplied:
			flags = append(flags, imap.AnsweredFlag)

		case maildir.FlagFlagged:
			flags = append(flags, imap.FlaggedFlag)

		case maildir.FlagTrashed:
			flags = append(flags, imap.DeletedFlag)

		case maildir.FlagDraft:
			flags = append(flags, imap.DraftFlag)
		}
	}
	return flags
}
