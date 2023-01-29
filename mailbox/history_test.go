package mailbox

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestFindLatestActionFromHistory(t *testing.T) {
	day := 24 * time.Hour
	initialTime := time.Date(2020, 1, 1, 12, 20, 0, 0, time.Local)
	dayBefore := initialTime.Add(-day)
	dayAfter := initialTime.Add(day)
	history := &History{
		Actions: []HistoryAction{
			{
				SourceAccountTag: "source",
				Action:           "test",
				UidValidity:      123,
				Date:             initialTime,
				Entries: []HistoryEntry{
					{SourceInternalDate: initialTime.Add(-4 * day)},
					{SourceInternalDate: initialTime.Add(-3 * day)},
				},
			},
			{
				SourceAccountTag: "source",
				Action:           "test",
				UidValidity:      123,
				Date:             dayAfter,
				Entries: []HistoryEntry{
					{SourceInternalDate: initialTime.Add(-2 * day)},
					{SourceInternalDate: dayBefore},
				},
			},
			{
				SourceAccountTag: "another source",
				Action:           "test",
				UidValidity:      123,
				Date:             dayAfter.Add(day),
				Entries: []HistoryEntry{
					{SourceInternalDate: initialTime.Add(-5 * day)},
				},
			},
		},
	}

	lastAction := FindLastAction("source", history)
	assert.True(t, lastAction.Equal(dayAfter))

	latestMessage := FindLatestInternalDateFromHistory("source", history)
	assert.True(t, latestMessage.Equal(dayBefore))
}
