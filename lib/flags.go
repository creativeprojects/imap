package lib

import "github.com/emersion/go-imap"

func StripRecentFlag(source []string) []string {
	output := make([]string, 0, len(source))
	for _, flag := range source {
		if flag == imap.RecentFlag {
			continue
		}
		output = append(output, flag)
	}
	return output
}
