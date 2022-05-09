package limitio

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

type Reader struct {
	source  io.Reader
	limiter *rate.Limiter
}

// NewReader returns a reader that implements io.Reader with rate limiting.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		source: r,
	}
}

// SetRateLimit sets rate limit (bytes/sec) to the reader.
func (s *Reader) SetRateLimit(bytesPerSec float64, burst int) {
	s.limiter = rate.NewLimiter(rate.Limit(bytesPerSec), burst)
}

// Read bytes into p.
func (s *Reader) Read(p []byte) (int, error) {
	if s.limiter == nil {
		return s.source.Read(p)
	}
	// ask for a burst of data
	err := s.limiter.WaitN(context.Background(), s.limiter.Burst())
	if err != nil {
		return 0, err
	}
	// read all data
	n, err := s.source.Read(p)
	if err != nil {
		return n, err
	}
	// then wait for the tokens to allow the time needed for reading it all
	left := n - s.limiter.Burst() // remove first burst
	for left > 0 {
		singleRead := left
		if singleRead > s.limiter.Burst() {
			singleRead = s.limiter.Burst()
		}
		err = s.limiter.WaitN(context.Background(), singleRead)
		if err != nil {
			return n, err
		}
		left -= singleRead
	}
	return n, nil
}
