package local

import (
	"path/filepath"
	"testing"

	"github.com/creativeprojects/imap/storage/test"
	"github.com/stretchr/testify/require"
)

func TestStoreBackend(t *testing.T) {
	dir := t.TempDir()
	backend, err := NewBoltStore(filepath.Join(dir, "store.db"))
	require.NoError(t, err)

	defer backend.Close()

	err = backend.Init()
	require.NoError(t, err)

	err = test.PrepareBackend(backend)
	require.NoError(t, err)

	test.RunTestsOnBackend(t, backend)
}
