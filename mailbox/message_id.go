package mailbox

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"strconv"
	"strings"
)

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

func (i MessageID) Value() any {
	if i.IsUint() {
		return i.uid
	}
	return i.key
}

func (i MessageID) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

func (i *MessageID) UnmarshalText(text []byte) error {
	str := string(text)
	value, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		// keep it as a string
		i.key = str
		return nil
	}
	i.uid = uint32(value)
	return nil
}

func (i MessageID) MarshalJSON() ([]byte, error) {
	if i.IsUint() {
		return json.Marshal(i.uid)
	}
	return json.Marshal(i.key)
}

func (i *MessageID) UnmarshalJSON(text []byte) error {
	str := string(text)
	if strings.HasPrefix(str, "\"") && strings.HasSuffix(str, "\"") {
		// keep it as a string
		i.key = strings.Trim(str, "\"")
		return nil
	}
	value, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		// keep it as a string
		i.key = str
		return nil
	}
	i.uid = uint32(value)
	return nil
}

func (i MessageID) MarshalBinary() ([]byte, error) {
	b := &bytes.Buffer{}
	encoder := gob.NewEncoder(b)
	if i.IsUint() {
		str := strconv.FormatUint(uint64(i.uid), 10)
		err := encoder.Encode(&str)
		if err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	err := encoder.Encode(i.key)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// UnmarshalBinary modifies the receiver so it must take a pointer receiver.
func (i *MessageID) UnmarshalBinary(data []byte) error {
	b := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(b)
	var value string
	err := decoder.Decode(&value)
	if err != nil {
		return err
	}
	uid, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		// keep it as a string
		i.key = value
		return nil
	}
	i.uid = uint32(uid)
	return nil
}
