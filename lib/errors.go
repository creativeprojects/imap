package lib

import "errors"

var (
	ErrMailboxNotFound = errors.New("mailbox not found")
	ErrInfoNotFound    = errors.New("mailbox info not found")
	ErrStatusNotFound  = errors.New("mailbox status not found")
	ErrNotSelected     = errors.New("mailbox not selected")
)
