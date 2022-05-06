package test

import (
	"bytes"
	"crypto/sha256"
	"sync"
	"testing"
	"time"

	"github.com/creativeprojects/imap/lib"
	"github.com/creativeprojects/imap/mailbox"
	"github.com/creativeprojects/imap/storage"
	"github.com/emersion/go-imap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	sampleMessageHash  []byte
)

func init() {
	hasher := sha256.New()
	hasher.Write([]byte(sampleMessage))
	sampleMessageHash = hasher.Sum(nil)
}

// RunTestsOnBackend is the unit tests runner called by the concrete implementations of storage.Backend
func RunTestsOnBackend(t *testing.T, backend storage.Backend) {
	require.NotNil(t, backend)

	t.Run("ListMailbox", func(t *testing.T) {
		list, err := backend.ListMailbox()
		require.NoError(t, err)

		// check there's at least one mailbox
		require.Greater(t, len(list), 0)
		// check the expected delimiter
		assert.Equal(t, backend.Delimiter(), list[0].Delimiter)
	})

	t.Run("CreateExistingMailbox", func(t *testing.T) {
		list, err := backend.ListMailbox()
		require.NoError(t, err)

		assert.True(t, mailboxExists("INBOX", list))

		err = backend.CreateMailbox(mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "INBOX",
		})
		require.NoError(t, err)
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

	t.Run("SelectMailboxDoesNotExist", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "No mailbox at that name",
		}
		status, err := backend.SelectMailbox(info)
		assert.Nil(t, status)
		require.Error(t, err)
		// IMAP doesn't have a specific error (it's up to the server implementation)
		// assert.ErrorIs(t, err, lib.ErrMailboxNotFound)
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

	t.Run("NewMailboxHasNoHistory", func(t *testing.T) {
		history, err := backend.GetHistory(mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		})
		assert.NoError(t, err)
		require.NotNil(t, history)
		assert.Empty(t, history.Actions)
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
			if len(msg.Hash) > 0 {
				assert.Equal(t, sampleMessageHash, msg.Hash)
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

	t.Run("StoreOneAction", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		}
		action := mailbox.HistoryAction{
			SourceAccountTag: "tag",
			Date:             time.Now(),
			Action:           "TEST",
			UidValidity:      123,
			Entries: []mailbox.HistoryEntry{
				{
					SourceID:  mailbox.NewMessageIDFromUint(1),
					MessageID: mailbox.NewMessageIDFromString("c11"),
				},
			},
		}

		err := backend.AddToHistory(info, action)
		assert.NoError(t, err)
	})

	t.Run("LoadHistoryFromEmptyMailbox", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "INBOX",
		}
		history, err := backend.GetHistory(info)
		require.NoError(t, err)
		assert.NotNil(t, history)
		assert.Empty(t, history.Actions)
	})

	t.Run("LoadHistory", func(t *testing.T) {
		info := mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		}
		history, err := backend.GetHistory(info)
		require.NoError(t, err)
		require.NotNil(t, history)
		require.Len(t, history.Actions, 1)
		assert.Equal(t, "TEST", history.Actions[0].Action)
		assert.Equal(t, "tag", history.Actions[0].SourceAccountTag)
		assert.Equal(t, uint32(123), history.Actions[0].UidValidity)
		require.Len(t, history.Actions[0].Entries, 1)
		assert.Equal(t, uint32(1), history.Actions[0].Entries[0].SourceID.AsUint())
		assert.Equal(t, "c11", history.Actions[0].Entries[0].MessageID.AsString())
	})

	t.Run("DeleteSimpleMailbox", func(t *testing.T) {
		deleteMailbox(t, backend, mailbox.Info{
			Delimiter: backend.Delimiter(),
			Name:      "Work",
		})
	})

}

func PrepareBackend(backend storage.Backend) error {
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

func createMailbox(t *testing.T, backend storage.Backend, info mailbox.Info) {
	t.Helper()

	err := backend.CreateMailbox(info)
	require.NoError(t, err)

	list, err := backend.ListMailbox()
	require.NoError(t, err)

	name := lib.VerifyDelimiter(info.Name, info.Delimiter, backend.Delimiter())
	assert.True(t, mailboxExists(name, list))
}

func deleteMailbox(t *testing.T, backend storage.Backend, info mailbox.Info) {
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
