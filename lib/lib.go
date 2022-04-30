package lib

import "strings"

func VerifyDelimiter(name, existingDelimiter, expectedDelimiter string) string {
	if existingDelimiter == expectedDelimiter {
		return name
	}
	name = strings.ReplaceAll(name, expectedDelimiter, "\\"+expectedDelimiter)
	// TODO: verify we're not replacing \existingDelimiter (escaped delimiter)
	name = strings.ReplaceAll(name, existingDelimiter, expectedDelimiter)
	return name
}
