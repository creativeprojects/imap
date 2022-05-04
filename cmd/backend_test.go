package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/creativeprojects/imap/cfg"
	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/mdir"
	"github.com/creativeprojects/imap/mem"
	"github.com/creativeprojects/imap/remote"
	"github.com/creativeprojects/imap/store"
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
		DebugLogger: &testLogger{t},
	})
	assert.NoError(t, err)

	RunTestBackend(t, backend)
	err = backend.Close()
	assert.NoError(t, err)

	// close the server
	err = server.Close()
	assert.NoError(t, err)
	wg.Wait()
}

func TestMaildirBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("it looks like maildir is not working on Windows")
		return
	}
	root := t.TempDir()
	backend, err := mdir.New(root)
	require.NoError(t, err)

	defer backend.Close()

	backend.DebugLogger(&testLogger{t})

	err = prepareMaildirBackend(t, backend)
	require.NoError(t, err)

	RunTestBackend(t, backend)
}

func TestStoreBackend(t *testing.T) {
	dir := t.TempDir()
	backend, err := store.NewBoltStore(filepath.Join(dir, "store.db"))
	require.NoError(t, err)

	defer backend.Close()

	backend.DebugLogger(&testLogger{t})

	err = prepareLocalBackend(t, backend)
	require.NoError(t, err)

	RunTestBackend(t, backend)
}

func TestMemoryBackend(t *testing.T) {
	backend := mem.New()

	defer backend.Close()

	backend.DebugLogger(&testLogger{t})

	err := prepareBackend(backend)
	require.NoError(t, err)

	RunTestBackend(t, backend)
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
			backend, err := store.NewBoltStore(account.File)
			require.NoError(t, err)
			defer backend.Close()

			backend.DebugLogger(&testLogger{t})

			err = prepareLocalBackend(t, backend)
			require.NoError(t, err)

			t.Run(name, func(t *testing.T) {
				RunTestBackend(t, backend)
			})

		case cfg.MAILDIR:
			if runtime.GOOS == "windows" {
				t.Log("it looks like maildir is not working on Windows")
				continue
			}
			backend, err := mdir.New(account.Root)
			require.NoError(t, err)
			defer backend.Close()

			backend.DebugLogger(&testLogger{t})

			err = prepareMaildirBackend(t, backend)
			require.NoError(t, err)

			t.Run(name, func(t *testing.T) {
				RunTestBackend(t, backend)
			})

		case cfg.IMAP:
			backend, err := remote.NewImap(remote.Config{
				ServerURL:           account.ServerURL,
				Username:            account.Username,
				Password:            account.Password,
				SkipTLSVerification: account.SkipTLSVerification,
				DebugLogger:         &testLogger{t},
			})
			require.NoError(t, err)
			defer backend.Close()

			t.Run(name, func(t *testing.T) {
				RunTestBackend(t, backend)
			})
		default:
			t.Errorf("unexpected account type %q", account.Type)
		}
	}
}

func RunTestBackend(t *testing.T, backend Backend) {
	require.NotNil(t, backend)

	t.Run("ListMailbox", func(t *testing.T) {
		list, err := backend.ListMailbox()
		require.NoError(t, err)

		// check there's at least one mailbox
		require.Greater(t, len(list), 0)
		// check the expected delimiter
		assert.Equal(t, backend.Delimiter(), list[0].Delimiter)
	})

	t.Run("CreateDeleteMailboxSameDelimiter", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Path" + backend.Delimiter() + "Mailbox",
		}
		createMailbox(t, backend, info)
		deleteMailbox(t, backend, info)
		// also deletes the "Path" one if exists (it should on IMAP)
		_ = backend.DeleteMailbox(mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Path",
		})
	})

	t.Run("CreateDeleteMailboxDifferentDelimiter", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: "#",
			Name:      "Path#Mailbox",
		}
		createMailbox(t, backend, info)
		deleteMailbox(t, backend, info)
		// also deletes the "Path" one if exists (it should on IMAP)
		_ = backend.DeleteMailbox(mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Path",
		})
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
		props := mailbox.MessageProperties{
			Flags:        sampleMessageFlags,
			InternalDate: sampleMessageDate,
			Size:         uint32(len(sampleMessage)),
		}
		body := bytes.NewBufferString(sampleMessage)
		uid, err := backend.PutMessage(info, props, body)
		require.NoError(t, err)
		if backend.SupportMessageID() {
			assert.NotZero(t, uid)
		}

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

		count := 0
		for msg := range receiver {
			count++
			assert.NotNil(t, msg)
			buffer := &bytes.Buffer{}
			read, err := buffer.ReadFrom(msg.Body)
			assert.NoError(t, err)
			msg.Body.Close()
			assert.Equal(t, int64(len(sampleMessage)), read)
			if msg.Size > 0 {
				assert.Equal(t, read, int64(msg.Size))
			}
			assert.True(t, sampleMessageDate.Equal(msg.InternalDate))
			assert.ElementsMatch(t, sampleMessageFlags, msg.Flags)
			t.Logf("Received message uid=%s size=%d flags=%+v", msg.Uid.String(), read, msg.Flags)
		}
		assert.Equal(t, 1, count)

		// wait until all the messages arrived
		err := <-done
		assert.NoError(t, err)

		err = backend.UnselectMailbox()
		assert.NoError(t, err)
	})

	t.Run("AppendTwoMoreMessages", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		}
		for i := 0; i < 2; i++ {
			props := mailbox.MessageProperties{
				Flags:        sampleMessageFlags,
				InternalDate: sampleMessageDate,
				Size:         uint32(len(sampleMessage)),
			}
			body := bytes.NewBufferString(sampleMessage)
			uid, err := backend.PutMessage(info, props, body)
			require.NoError(t, err)
			if backend.SupportMessageID() {
				assert.NotZero(t, uid)
			}
		}

		// Verify the mailbox shows 3 messages
		status, err := backend.SelectMailbox(info)
		require.NoError(t, err)
		t.Logf("%v", status)
		assert.Equal(t, info.Name, status.Name)
		assert.Equal(t, uint32(3), status.Messages)
		assert.Equal(t, uint32(0), status.Unseen)

		err = backend.UnselectMailbox()
		assert.NoError(t, err)
	})

	t.Run("FetchThreeMessages", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		}
		_, err := backend.SelectMailbox(info)
		require.NoError(t, err)

		receiver := make(chan *mailbox.Message, 10)
		done := make(chan error, 1)
		go func() {
			done <- backend.FetchMessages(receiver)
		}()

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			count := 0
			for msg := range receiver {
				count++
				assert.NotNil(t, msg)
				buffer := &bytes.Buffer{}
				read, err := buffer.ReadFrom(msg.Body)
				assert.NoError(t, err)
				msg.Body.Close()
				assert.Equal(t, int64(len(sampleMessage)), read)
				if msg.Size > 0 {
					assert.Equal(t, read, int64(msg.Size))
				}
				assert.True(t, sampleMessageDate.Equal(msg.InternalDate))
				assert.ElementsMatch(t, sampleMessageFlags, msg.Flags)
				t.Logf("Received message uid=%s size=%d flags=%+v", msg.Uid.String(), read, msg.Flags)
			}
			assert.Equal(t, 3, count)
		}()
		// wait until all the messages arrived
		err = <-done
		assert.NoError(t, err)

		wg.Wait()

		err = backend.UnselectMailbox()
		assert.NoError(t, err)
	})

	t.Run("AppendMessageWithWrongSize", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		}
		props := mailbox.MessageProperties{
			Flags:        sampleMessageFlags,
			InternalDate: sampleMessageDate,
			Size:         uint32(len(sampleMessage)) - 1,
		}
		body := bytes.NewBufferString(sampleMessage)
		_, err := backend.PutMessage(info, props, body)
		assert.Error(t, err)

		// Verify the mailbox still shows 3 messages
		status, err := backend.SelectMailbox(info)
		assert.NoError(t, err)
		t.Logf("%v", status)
		assert.Equal(t, uint32(3), status.Messages)

		err = backend.UnselectMailbox()
		assert.NoError(t, err)
	})

	t.Run("DeleteSimpleMailbox", func(t *testing.T) {
		deleteMailbox(t, backend, mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		})
	})

	t.Run("CopyMailbox", func(t *testing.T) {
		var total uint32 = 23
		info := mailbox.Info{Name: "Mailbox Copy", Delimiter: "."}

		memBackend := mem.New()
		memBackend.GenerateFakeEmails(info, total, 100, 100000)

		_, err := memBackend.SelectMailbox(info)
		assert.NoError(t, err)

		progress := &testProgress{}
		err = CopyMessages(memBackend, backend, info, progress)
		assert.NoError(t, err)

		assert.Equal(t, total, progress.count)

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
	props := mailbox.MessageProperties{
		Flags:        []string{"\\Seen"},
		InternalDate: time.Now(),
		Size:         uint32(len(sampleMessage)),
	}
	buffer := bytes.NewBufferString(sampleMessage)
	_, err = backend.PutMessage(info, props, buffer)
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

type testProgress struct {
	count uint32
}

func (p *testProgress) Increment() {
	p.count++
}
