package cmd

import (
	"bytes"
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

const (
	sampleMessage = "From: contact@example.org\r\n" +
		"To: contact@example.org\r\n" +
		"Subject: A little message, just for you\r\n" +
		"Date: Wed, 11 May 2016 14:31:59 +0000\r\n" +
		"Message-ID: <0000000@localhost/>\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Hi there :)"
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

// func TestMaildirBackend(t *testing.T) {
// 	root := t.TempDir()
// 	backend, err := mdir.New(root)
// 	require.NoError(t, err)

// 	err = prepareMaildirBackend(backend)
// 	require.NoError(t, err)

// 	runTestBackend(t, backend)
// }

// func TestStoreBackend(t *testing.T) {
// 	dir := t.TempDir()
// 	backend, err := store.NewBoltStore(filepath.Join(dir, "store.db"))
// 	require.NoError(t, err)

// 	err = prepareLocalBackend(backend)
// 	require.NoError(t, err)

// 	runTestBackend(t, backend)
// }

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
			backend, err := store.NewBoltStore(account.File)
			require.NoError(t, err)
			err = prepareLocalBackend(t, backend)
			require.NoError(t, err)

			t.Run(name, func(t *testing.T) {
				runTestBackend(t, backend)
			})

		case cfg.MAILDIR:
			backend, err := mdir.New(account.Root)
			require.NoError(t, err)
			err = prepareMaildirBackend(t, backend)
			require.NoError(t, err)

			t.Run(name, func(t *testing.T) {
				runTestBackend(t, backend)
			})

		case cfg.IMAP:
			backend, err := remote.NewImap(remote.Config{
				ServerURL: account.ServerURL,
				Username:  account.Username,
				Password:  account.Password,
			})
			require.NoError(t, err)

			t.Run(name, func(t *testing.T) {
				runTestBackend(t, backend)
			})
		default:
			t.Errorf("unexpected account type %q", account.Type)
		}
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

	t.Run("SelectMailbox", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "INBOX",
		}
		status, err := backend.SelectMailbox(info)
		require.NoError(t, err)
		t.Logf("%v", status)
		assert.Equal(t, info.Name, status.Name)
		assert.Equal(t, uint32(1), status.Messages)
	})

	t.Run("CreateSimpleMailbox", func(t *testing.T) {
		createMailbox(t, backend, mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		})
	})

	t.Run("AppendMessage", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		}
		body := bytes.NewBufferString(sampleMessage)
		err := backend.PutMessage(info, []string{"\\Seen"}, time.Now(), body)
		require.NoError(t, err)

		// Verify the mailbox shows 1 message
		status, err := backend.SelectMailbox(info)
		require.NoError(t, err)
		t.Logf("%v", status)
		assert.Equal(t, info.Name, status.Name)
		assert.Equal(t, uint32(1), status.Messages)
		assert.Equal(t, uint32(0), status.Unseen)
	})

	t.Run("FetchOneMessage", func(t *testing.T) {
		receiver := make(chan *mailbox.Message, 10)
		done := make(chan error, 1)
		go func() {
			done <- backend.FetchMessages(receiver)
		}()

		for msg := range receiver {
			if msg == nil {
				break
			}
			t.Logf("Received message seq=%d uid=%d size=%d flags=%+v", msg.SeqNum, msg.Uid, msg.Size, msg.Flags)
		}

		// wait until all the messages arrived
		err := <-done
		// close(receiver)
		require.NoError(t, err)
	})

	t.Run("DeleteSimpleMailbox", func(t *testing.T) {
		deleteMailbox(t, backend, mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		})
	})
}

func prepareMaildirBackend(t *testing.T, backend *mdir.Maildir) error {
	t.Helper()
	backend.DebugLogger(&testLogger{t})
	return prepareBackend(backend)
}

func prepareLocalBackend(t *testing.T, backend *store.BoltStore) error {
	t.Helper()
	backend.DebugLogger(&testLogger{t})
	err := backend.Init()
	if err != nil {
		return err
	}
	return prepareBackend(backend)
}

func prepareBackend(backend Backend) error {
	info := mailbox.Info{
		Delimiter: backend.Delimiter(),
		Name:      "INBOX",
	}
	existing, err := backend.ListMailbox()
	if err != nil {
		return err
	}
	if mailboxExists(info.Name, existing) {
		// no need to create the mailbox and add a message to it
		return nil
	}
	err = backend.CreateMailbox(info)
	if err != nil {
		return err
	}
	buffer := bytes.NewBufferString(sampleMessage)
	err = backend.PutMessage(info, []string{"\\Seen"}, time.Now(), buffer)
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
