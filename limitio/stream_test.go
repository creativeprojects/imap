package limitio_test

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"testing"
	"time"

	"github.com/creativeprojects/imap/limitio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const burst = 1024 // 1KB of burst

func getRates() []float64 {
	return []float64{
		500 * 1024,       // 500KB/sec
		1024 * 1024,      // 1MB/sec
		10 * 1024 * 1024, // 10MB/sec
	}
}

func getSources() []*bytes.Reader {
	return []*bytes.Reader{
		bytes.NewReader(bytes.Repeat([]byte{10}, 64*1024)),   // 64KB
		bytes.NewReader(bytes.Repeat([]byte{11}, 256*1024)),  // 256KB
		bytes.NewReader(bytes.Repeat([]byte{12}, 1024*1024)), // 1MB
	}
}

func TestRead(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	for _, limit := range getRates() {
		for _, src := range getSources() {
			t.Run(fmt.Sprintf("Read %s at %s/sec", iBytes(uint64(src.Len())), iBytes(uint64(limit))), func(t *testing.T) {
				t.Parallel()
				_, err := src.Seek(0, 0)
				require.NoError(t, err)
				sio := limitio.NewReader(src)
				sio.SetRateLimit(limit, burst)
				start := time.Now()
				n, err := io.Copy(io.Discard, sio)
				elapsed := time.Since(start)
				if err != nil {
					t.Error("io.Copy failed", err)
				}
				realRate := float64(n) / elapsed.Seconds()
				percent := realRate / limit * 100
				assert.InDelta(t, 100, percent, 2) // 2% error margin
				t.Logf(
					"read %s / %s: Real %s/sec Limit %s/sec. (%.2f %%)",
					iBytes(uint64(n)),
					elapsed,
					iBytes(uint64(realRate)),
					iBytes(uint64(limit)),
					percent,
				)
			})
		}
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	for _, limit := range getRates() {
		for _, src := range getSources() {
			t.Run(fmt.Sprintf("Read %s at %s/sec", iBytes(uint64(src.Len())), iBytes(uint64(limit))), func(t *testing.T) {
				t.Parallel()
				_, err := src.Seek(0, 0)
				require.NoError(t, err)
				sio := limitio.NewWriter(io.Discard)
				sio.SetRateLimit(limit, burst)
				start := time.Now()
				n, err := io.Copy(sio, src)
				elapsed := time.Since(start)
				if err != nil {
					t.Error("io.Copy failed", err)
				}
				realRate := float64(n) / elapsed.Seconds()
				percent := realRate / limit * 100
				assert.InDelta(t, 100, percent, 2) // 2% error margin
				t.Logf(
					"write %s / %s: Real %s/sec Limit %s/sec. (%.2f %%)",
					iBytes(uint64(n)),
					elapsed,
					iBytes(uint64(realRate)),
					iBytes(uint64(limit)),
					percent,
				)
			})
		}
	}
}

func iBytes(s uint64) string {
	var base float64 = 1024
	sizes := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

	if s < 10 {
		return fmt.Sprintf("%d B", s)
	}
	e := math.Floor(logn(float64(s), base))
	suffix := sizes[int(e)]
	val := math.Floor(float64(s)/math.Pow(base, e)*10+0.5) / 10
	f := "%.0f %s"
	if val < 10 {
		f = "%.1f %s"
	}

	return fmt.Sprintf(f, val, suffix)
}

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}
