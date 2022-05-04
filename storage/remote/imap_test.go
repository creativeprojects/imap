package remote

import (
	"sync"
	"testing"
	"time"

	"github.com/creativeprojects/imap/storage/test"
	compress "github.com/emersion/go-imap-compress"
	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

func TestImapBackend(t *testing.T) {
	// Create a memory backend
	be := memory.New()

	// Create a new server
	server := server.New(be)
	// Since we will use this server for testing only, we can allow plain text
	// authentication over non-encrypted connections
	server.AllowInsecureAuth = true
	server.Enable(compress.NewExtension())

	listener, err := nettest.NewLocalListener("tcp")
	require.NoError(t, err)

	t.Logf("Starting IMAP server at %s", listener.Addr().String())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.Serve(listener)
	}()

	time.Sleep(100 * time.Millisecond)

	backend, err := NewImap(Config{
		ServerURL: listener.Addr().String(),
		Username:  "username",
		Password:  "password",
		NoTLS:     true,
	})
	assert.NoError(t, err)

	test.RunTestsOnBackend(t, backend)
	err = backend.Close()
	assert.NoError(t, err)

	// close the server
	err = server.Close()
	assert.NoError(t, err)
	wg.Wait()
}
