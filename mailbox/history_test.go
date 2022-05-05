package mailbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateAccountTag(t *testing.T) {
	expected := "d6549d2a410fe02063abe508d42102f65b3ef71e8b68ce11b8f4e62072a2a1d8"
	tag := AccountTag("mail.example.com:993", "user@example.com")
	assert.Equal(t, expected, tag)
}
