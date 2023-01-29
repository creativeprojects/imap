package mailbox

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAccountTag(t *testing.T) {
	expected := "d6549d2a410fe02063abe508d42102f65b3ef71e8b68ce11b8f4e62072a2a1d8"
	tag := AccountTag("mail.example.com:993", "user@example.com")
	assert.Equal(t, expected, tag)
}

func TestGetEmptyHistory(t *testing.T) {
	history, err := GetHistoryFromFile("/file_really_should_not_exist_here")
	assert.NoError(t, err)
	assert.Equal(t, &History{}, history) // empty history
}

func TestSaveAndLoadHistory(t *testing.T) {
	history := &History{
		Actions: []HistoryAction{
			{
				SourceAccountTag: "source",
				Action:           "test",
				UidValidity:      123,
				Entries: []HistoryEntry{
					{NewMessageIDFromUint(1), time.Time{}, NewMessageIDFromUint(2)},
					{NewMessageIDFromString("3"), time.Time{}, NewMessageIDFromString("4")},
				},
			},
		},
	}
	filename := filepath.Join(t.TempDir(), "TestSaveAndLoadHistory.json")
	err := SaveHistoryToFile(filename, history)
	require.NoError(t, err)

	loaded, err := GetHistoryFromFile(filename)
	require.NoError(t, err)

	assert.Equal(t, history, loaded)
}

func TestFindHistoryFromSourceID(t *testing.T) {
	history := &History{
		Actions: []HistoryAction{
			{
				SourceAccountTag: "source",
				Action:           "test",
				UidValidity:      123,
				Entries: []HistoryEntry{
					{NewMessageIDFromUint(1), time.Time{}, NewMessageIDFromUint(2)},
					{NewMessageIDFromString("3"), time.Time{}, NewMessageIDFromString("4")},
				},
			},
			{
				SourceAccountTag: "source",
				Action:           "test",
				UidValidity:      123,
				Entries: []HistoryEntry{
					{NewMessageIDFromUint(5), time.Time{}, NewMessageIDFromUint(6)},
					{NewMessageIDFromString("7"), time.Time{}, NewMessageIDFromString("8")},
				},
			},
		},
	}

	testCases := []struct {
		sourceID MessageID
		found    bool
	}{
		{NewMessageIDFromUint(1), true},
		{NewMessageIDFromString("1"), false},
		{NewMessageIDFromString("3"), true},
		{NewMessageIDFromUint(3), false},
		{NewMessageIDFromUint(5), true},
		{NewMessageIDFromString("5"), false},
		{NewMessageIDFromString("7"), true},
		{NewMessageIDFromUint(7), false},
		{NewMessageIDFromString("10"), false},
		{NewMessageIDFromUint(10), false},
	}

	for _, testCase := range testCases {
		found := FindHistoryEntryFromSourceID(history, testCase.sourceID)
		if testCase.found {
			assert.NotNil(t, found)
		} else {
			assert.Nil(t, found)
		}
	}
}
