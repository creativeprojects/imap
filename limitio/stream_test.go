package limitio_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"testing"
	"time"

	"github.com/creativeprojects/imap/limitio"
	"github.com/stretchr/testify/assert"
)

const burst = 1024 // 1KB of burst

var rates = []float64{
	500 * 1024,       // 500KB/sec
	1024 * 1024,      // 1MB/sec
	10 * 1024 * 1024, // 10MB/sec
}

var srcs = []*bytes.Reader{
	bytes.NewReader(bytes.Repeat([]byte{0}, 64*1024)),   // 64KB
	bytes.NewReader(bytes.Repeat([]byte{1}, 256*1024)),  // 256KB
	bytes.NewReader(bytes.Repeat([]byte{2}, 1024*1024)), // 1MB
}

func TestRead(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	for _, src := range srcs {
		for _, limit := range rates {
			src.Seek(0, 0)
			sio := limitio.NewReader(src)
			sio.SetRateLimit(limit, burst)
			start := time.Now()
			n, err := io.Copy(ioutil.Discard, sio)
			elapsed := time.Since(start)
			if err != nil {
				t.Error("io.Copy failed", err)
			}
			realRate := float64(n) / elapsed.Seconds()
			percent := realRate / limit * 100
			assert.InDelta(t, 100, percent, 2) // 2% error margin
			t.Logf(
				"read %s / %s: Real %s/sec Limit %s/sec. (%f %%)",
				iBytes(uint64(n)),
				elapsed,
				iBytes(uint64(realRate)),
				iBytes(uint64(limit)),
				percent,
			)
		}
	}
}

func TestWrite(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	for _, src := range srcs {
		for _, limit := range rates {
			src.Seek(0, 0)
			sio := limitio.NewWriter(ioutil.Discard)
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
				"write %s / %s: Real %s/sec Limit %s/sec. (%f %%)",
				iBytes(uint64(n)),
				elapsed,
				iBytes(uint64(realRate)),
				iBytes(uint64(limit)),
				percent,
			)
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
