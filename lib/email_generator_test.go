package lib

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateGenerator(t *testing.T) {
	from := time.Date(2010, 1, 1, 12, 0, 0, 0, time.Local)

	for range 100000 {
		result := GenerateDateFrom(from)
		now := time.Now()
		assert.Truef(t, result.After(from), "%v is not after %v", result, from)
		assert.Truef(t, result.Before(now), "%v is not before %v", result, now)
	}
}

func TestGenerateFlags(t *testing.T) {
	maxInt := 5
	for range 100000 {
		flags := GenerateFlags(maxInt)
		require.NotNil(t, flags)
		require.GreaterOrEqual(t, len(flags), 0)
		require.Less(t, len(flags), maxInt)
	}
}
