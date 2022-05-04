package mdir

import (
	"runtime"
	"testing"

	"github.com/creativeprojects/imap/storage/test"
	"github.com/stretchr/testify/require"
)

func TestMaildirBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("maildir is not supported on Windows")
		return
	}
	root := t.TempDir()
	backend, err := New(root)
	require.NoError(t, err)

	defer backend.Close()

	err = test.PrepareBackend(backend)
	require.NoError(t, err)

	test.RunTestsOnBackend(t, backend)
}
