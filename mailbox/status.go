package mailbox

type Status struct {
	// The mailbox name.
	Name string

	// The mailbox flags.
	Flags []string
	// The mailbox permanent flags.
	PermanentFlags []string
	// The sequence number of the first unseen message in the mailbox.
	UnseenSeqNum uint32

	// The number of messages in this mailbox.
	Messages uint32
	// The number of messages not seen since the last time the mailbox was opened.
	Recent uint32
	// The number of unread messages.
	Unseen uint32
	// The next UID.
	UidNext uint32
	// Together with a UID, it is a unique identifier for a message.
	// Must be greater than or equal to 1.
	UidValidity uint32
}
