package cmd

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/creativeprojects/imap/cfg"
	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/mdir"
	"github.com/creativeprojects/imap/remote"
	"github.com/creativeprojects/imap/store"
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
	// authentication over unencrypted connections
	server.AllowInsecureAuth = true

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

	client, err := remote.NewImap(remote.Config{
		ServerURL:   listener.Addr().String(),
		Username:    "username",
		Password:    "password",
		NoTLS:       true,
		DebugLogger: &testLogger{t},
	})
	assert.NoError(t, err)

	runTestBackend(t, client)
	err = client.Close()
	assert.NoError(t, err)

	// close the server
	err = server.Close()
	assert.NoError(t, err)
	wg.Wait()
}

func TestMaildirBackend(t *testing.T) {
	root := t.TempDir()
	backend, err := mdir.New(root)
	require.NoError(t, err)

	err = prepareMaildirBackend(backend)
	require.NoError(t, err)

	runTestBackend(t, backend)
}

func TestStoreBackend(t *testing.T) {
	dir := t.TempDir()
	backend, err := store.NewBoltStore(filepath.Join(dir, "store.db"))
	require.NoError(t, err)

	err = prepareLocalBackend(backend)
	require.NoError(t, err)

	runTestBackend(t, backend)
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
		backend, err := NewBackend(account)
		require.NoError(t, err)

		// switch account.Type{
		// case cfg.LOCAL:
		// 	err:=prepareLocalBackend(backend)
		// }

		t.Run(name, func(t *testing.T) {
			runTestBackend(t, backend)
		})
	}
}

func runTestBackend(t *testing.T, backend Backend) {
	require.NotNil(t, backend)

	t.Run("ListMailbox", func(t *testing.T) {
		list, err := backend.ListMailbox()
		require.NoError(t, err)

		require.Len(t, list, 1)
		assert.Equal(t, "INBOX", list[0].Name)
		assert.Equal(t, backend.Delimiter(), list[0].Delimiter)
	})

	t.Run("CreateSimpleMailbox", func(t *testing.T) {
		createMailbox(t, backend, mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		})
	})

	t.Run("DeleteSimpleMailbox", func(t *testing.T) {
		deleteMailbox(t, backend, mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		})
	})

	t.Run("CreateDeleteMailboxSameDelimiter", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Path" + backend.Delimiter() + "Mailbox",
		}
		createMailbox(t, backend, info)
		deleteMailbox(t, backend, info)
	})

	t.Run("CreateDeleteMailboxDifferentDelimiter", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: "#",
			Name:      "Path#Mailbox",
		}
		createMailbox(t, backend, info)
		deleteMailbox(t, backend, info)
	})
}

func prepareMaildirBackend(backend *mdir.Maildir) error {
	err := backend.CreateMailbox(mailbox.Info{
		Delimiter: backend.Delimiter(),
		Name:      "INBOX",
	})
	if err != nil {
		return err
	}

	// adds a random file at the root of maildir
	file, err := os.Create(filepath.Join(backend.Root(), "info.json"))
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func prepareLocalBackend(backend *store.BoltStore) error {
	err := backend.Init()
	if err != nil {
		return err
	}
	err = backend.CreateMailbox(mailbox.Info{
		Delimiter: backend.Delimiter(),
		Name:      "INBOX",
	})
	if err != nil {
		return err
	}
	return nil
}

func createMailbox(t *testing.T, backend Backend, info mailbox.Info) {
	t.Helper()

	err := backend.CreateMailbox(info)
	require.NoError(t, err)

	list, err := backend.ListMailbox()
	require.NoError(t, err)

	name := lib.VerifyDelimiter(info.Name, info.Delimiter, backend.Delimiter())
	assert.True(t, mailboxExists(name, list))
}

func deleteMailbox(t *testing.T, backend Backend, info mailbox.Info) {
	t.Helper()

	err := backend.DeleteMailbox(info)
	require.NoError(t, err)

	list, err := backend.ListMailbox()
	require.NoError(t, err)

	name := lib.VerifyDelimiter(info.Name, info.Delimiter, backend.Delimiter())
	assert.False(t, mailboxExists(name, list))
}

func mailboxExists(name string, in []mailbox.Info) bool {
	for _, mailbox := range in {
		if mailbox.Name == name {
			return true
		}
	}
	return false
}
