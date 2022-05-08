package local

import (
	"testing"
	"time"

	"github.com/creativeprojects/imap/mailbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerializationOfMap(t *testing.T) {
	data := make(map[string]string)
	data["key1"] = "value1"
	data["key2"] = "value2"
	data["empty"] = ""
	ser, err := SerializeData(data)
	require.NoError(t, err)

	back, err := DeserializeData(ser)
	require.NoError(t, err)

	assert.Equal(t, back, data)
}

func TestSerializationOfProperties(t *testing.T) {
	fixtures := []mailbox.MessageProperties{
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
	for _, data := range fixtures {
		t.Run("", func(t *testing.T) {
			ser, err := SerializeObject(&data)
			require.NoError(t, err)

			result, err := DeserializeObject[mailbox.MessageProperties](ser)
			require.NoError(t, err)

			assert.ElementsMatch(t, data.Flags, result.Flags)
			assert.True(t, data.InternalDate.Equal(result.InternalDate))
			assert.Equal(t, data.Size, result.Size)
			assert.Equal(t, data.Hash, result.Hash)
		})
	}
}
