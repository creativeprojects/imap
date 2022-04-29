package mailbox

type Info struct {
	// The mailbox attributes.
	Attributes []string
	// The server's path separator.
	Delimiter string
	// The mailbox name.
	Name string
}
