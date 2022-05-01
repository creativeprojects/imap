package mailbox

import "github.com/creativeprojects/imap/lib"

type Info struct {
	// The mailbox attributes.
	Attributes []string
	// The server's path separator.
	Delimiter string
	// The mailbox name.
	Name string
}

func ChangeDelimiter(info Info, delimiter string) Info {
	return Info{
		Attributes: info.Attributes,
		Delimiter:  delimiter,
		Name:       lib.VerifyDelimiter(info.Name, info.Delimiter, delimiter),
	}
}
