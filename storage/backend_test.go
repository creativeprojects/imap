package storage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/creativeprojects/imap/cfg"
	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/storage/local"
	"github.com/creativeprojects/imap/storage/mdir"
	"github.com/creativeprojects/imap/storage/mem"
	"github.com/creativeprojects/imap/storage/remote"
	"github.com/emersion/go-imap"
	compress "github.com/emersion/go-imap-compress"
	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

var (
	sampleMessage = "From: contact@example.org\r\n" +
		"To: contact@example.org\r\n" +
		"Subject: A little message, just for you\r\n" +
		"Date: Wed, 11 May 2016 14:31:59 +0000\r\n" +
		"Message-ID: <0000000@localhost/>\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Hi there :)"
	sampleMessageDate  = time.Date(2020, 10, 20, 12, 11, 0, 0, time.UTC)
	sampleMessageFlags = []string{imap.SeenFlag}
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

	backend, err := remote.NewImap(remote.Config{
		ServerURL:   listener.Addr().String(),
		Username:    "username",
		Password:    "password",
		NoTLS:       true,
		CacheDir:    t.TempDir(),
		DebugLogger: lib.NewTestLogger(t, "client"),
	})
	assert.NoError(t, err)

	RunIntegrationTestsOnBackend(t, backend)
	err = backend.Close()
	assert.NoError(t, err)

	// close the server
	err = server.Close()
	assert.NoError(t, err)
	wg.Wait()
}

func TestMaildirBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("maildir is not supported on Windows")
		return
	}
	root := t.TempDir()
	backend, err := mdir.NewWithLogger(root, lib.NewTestLogger(t, "client"))
	require.NoError(t, err)

	defer backend.Close()

	RunIntegrationTestsOnBackend(t, backend)
}

func TestStoreBackend(t *testing.T) {
	dir := t.TempDir()
	backend, err := local.NewBoltStoreWithLogger(filepath.Join(dir, "store.db"), lib.NewTestLogger(t, "client"))
	require.NoError(t, err)

	defer backend.Close()

	RunIntegrationTestsOnBackend(t, backend)
}

func TestMemoryBackend(t *testing.T) {
	backend := mem.NewWithLogger(lib.NewTestLogger(t, "client"))

	defer backend.Close()

	RunIntegrationTestsOnBackend(t, backend)
}

func TestBackendFromConfig(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	filename := filepath.Join(wd, "test.yaml")
	config, err := cfg.LoadFromFile(filename)
	if err != nil {
		t.Skip(err)
		return
	}

	for name, account := range config.Accounts {
		switch account.Type {
		case cfg.LOCAL:
			backend, err := local.NewBoltStoreWithLogger(account.File, lib.NewTestLogger(t, "client"))
			require.NoError(t, err)
			defer backend.Close()

			t.Run(name, func(t *testing.T) {
				RunIntegrationTestsOnBackend(t, backend)
			})

		case cfg.MAILDIR:
			if runtime.GOOS == "windows" {
				t.Log("maildir is not supported on Windows")
				continue
			}
			backend, err := mdir.NewWithLogger(account.Root, lib.NewTestLogger(t, "client"))
			require.NoError(t, err)
			defer backend.Close()

			t.Run(name, func(t *testing.T) {
				RunIntegrationTestsOnBackend(t, backend)
			})

		case cfg.IMAP:
			backend, err := remote.NewImap(remote.Config{
				ServerURL:           account.ServerURL,
				Username:            account.Username,
				Password:            account.Password,
				SkipTLSVerification: account.SkipTLSVerification,
				CacheDir:            t.TempDir(),
				DebugLogger:         lib.NewTestLogger(t, "client"),
			})
			require.NoError(t, err)
			defer backend.Close()

			t.Run(name, func(t *testing.T) {
				RunIntegrationTestsOnBackend(t, backend)
			})
		default:
			t.Errorf("unexpected account type %q", account.Type)
		}
	}
}

func RunIntegrationTestsOnBackend(t *testing.T, backend Backend) {
	require.NotNil(t, backend)

	t.Run("CopyMailbox", func(t *testing.T) {
		var total uint32 = 23
		info := mailbox.Info{Name: "Mailbox Copy", Delimiter: "."}

		memBackend := mem.New()
		memBackend.GenerateFakeEmails(info, total, 100, 100000)

		_, err := memBackend.SelectMailbox(info)
		assert.NoError(t, err)

		progress := &testProgress{}
		entries, err := CopyMessages(context.Background(), memBackend, backend, info, progress, nil)
		assert.NoError(t, err)

		assert.Equal(t, total, progress.count)
		assert.Equal(t, int(total), len(entries))

		// Verify the mailbox shows the right number of messages
		status, err := backend.SelectMailbox(info)
		require.NoError(t, err)

		assert.Equal(t, info.Name, status.Name)
		assert.Equal(t, total, status.Messages)

		err = backend.UnselectMailbox()
		assert.NoError(t, err)
		err = backend.DeleteMailbox(info)
		assert.NoError(t, err)
	})
}

type testProgress struct {
	count uint32
}

func (p *testProgress) Increment() {
	p.count++
}
