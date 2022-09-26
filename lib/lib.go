package lib

import (
	"strings"
	"time"
)

func VerifyDelimiter(name, existingDelimiter, expectedDelimiter string) string {
	if existingDelimiter == "" || expectedDelimiter == "" {
		return name
	}
	if existingDelimiter == expectedDelimiter {
		return name
	}
	name = strings.ReplaceAll(name, expectedDelimiter, "\\"+expectedDelimiter)
	// TODO: verify we're not replacing \existingDelimiter (escaped delimiter)
	name = strings.ReplaceAll(name, existingDelimiter, expectedDelimiter)
	return name
}

// SafePadding subtract about one day to the date to make sure we don't miss a message
func SafePadding(since time.Time) time.Time {
	if since.IsZero() {
		return since
	}
	// removes a day
	return since.Add(-25 * time.Hour)
}
