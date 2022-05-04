package local

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerialization(t *testing.T) {
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
