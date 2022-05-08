package mailbox

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryEncodingOfMessageID(t *testing.T) {
	fixtures := []MessageID{
		NewMessageIDFromString("toto"),
		NewMessageIDFromString(""),
		NewMessageIDFromUint(0),
		NewMessageIDFromUint(100),
	}
	for _, uid := range fixtures {
		t.Run(uid.String(), func(t *testing.T) {
			buffer := &bytes.Buffer{}
			encoder := gob.NewEncoder(buffer)
			err := encoder.Encode(&uid)
			require.NoError(t, err)
			binary := buffer.Bytes()

			var result MessageID
			buffer = bytes.NewBuffer(binary)
			decoder := gob.NewDecoder(buffer)
			err = decoder.Decode(&result)
			require.NoError(t, err)

			assert.Equal(t, uid, result)
		})
	}
}
