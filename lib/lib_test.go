package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDelimiter(t *testing.T) {
	fixtures := []struct {
		name             string
		currentDelimiter string
		newDelimiter     string
		expected         string
	}{
		{"name", "", "", "name"},
		{"name", "n", "", "name"},
		{"name", "", "n", "name"},
		{"name", "n", "n", "name"},
		{"name", ".", "/", "name"},
		{"name", "/", ".", "name"},
		{"folder/name", "/", ".", "folder.name"},
		{"folder.name", ".", "/", "folder/name"},
		{"folder/na.me", "/", ".", "folder.na\\.me"},
		{"folder.na/me", ".", "/", "folder/na\\/me"},
	}

	for _, fixture := range fixtures {
		result := VerifyDelimiter(fixture.name, fixture.currentDelimiter, fixture.newDelimiter)
		assert.Equal(t, fixture.expected, result)
	}
}
