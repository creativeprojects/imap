package mailbox

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryEncodingOfProperties(t *testing.T) {
	fixtures := []MessageProperties{
		{
			Flags: nil,
			Hash:  nil,
		},
		{
			Flags:        []string{"\\Seen"},
			InternalDate: time.Now(),
			Size:         1110,
			Hash:         []byte("test_hash"),
		},
	}
	for _, props := range fixtures {
		t.Run("", func(t *testing.T) {
			buffer := &bytes.Buffer{}
			encoder := gob.NewEncoder(buffer)
			err := encoder.Encode(&props)
			require.NoError(t, err)
			binary := buffer.Bytes()

			var result MessageProperties
			buffer = bytes.NewBuffer(binary)
			decoder := gob.NewDecoder(buffer)
			err = decoder.Decode(&result)
			require.NoError(t, err)

			assert.ElementsMatch(t, props.Flags, result.Flags)
			assert.True(t, props.InternalDate.Equal(result.InternalDate))
			assert.Equal(t, props.Size, result.Size)
			assert.Equal(t, props.Hash, result.Hash)
		})
	}
}
