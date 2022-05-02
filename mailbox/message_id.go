package mailbox

import "strconv"

var (
	EmptyMessageID MessageID
)

type MessageID struct {
	uid uint32
	key string
}

func NewMessageIDFromUint(uid uint32) MessageID {
	return MessageID{
		uid: uid,
	}
}

func NewMessageIDFromString(key string) MessageID {
	return MessageID{
		key: key,
	}
}

func (i MessageID) IsZero() bool {
	return i.uid == 0 && i.key == ""
}

func (i MessageID) IsUint() bool {
	return i.uid > 0
}

func (i MessageID) IsString() bool {
	return i.key != ""
}

func (i MessageID) AsUint() uint32 {
	return i.uid
}

func (i MessageID) AsString() string {
	return i.key
}

func (i MessageID) String() string {
	if i.IsUint() {
		return strconv.FormatUint(uint64(i.uid), 10)
	}
	return i.key
}
