package mem

import (
	"testing"

	"github.com/creativeprojects/imap/storage/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryBackend(t *testing.T) {
	backend := New()

	defer backend.Close()

	err := test.PrepareBackend(backend)
	require.NoError(t, err)

	test.RunTestsOnBackend(t, backend)
}
