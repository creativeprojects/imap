package limitio

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

type Writer struct {
	w       io.Writer
	limiter *rate.Limiter
}

// NewWriter returns a writer that implements io.Writer with rate limiting.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w: w,
	}
}

// SetRateLimit sets rate limit (bytes/sec) to the writer.
func (s *Writer) SetRateLimit(bytesPerSec float64, burst int) {
	s.limiter = rate.NewLimiter(rate.Limit(bytesPerSec), burst)
}

// Write writes bytes from p.
func (s *Writer) Write(p []byte) (int, error) {
	if s.limiter == nil {
		return s.w.Write(p)
	}
	// ask for a burst of data
	err := s.limiter.WaitN(context.Background(), s.limiter.Burst())
	if err != nil {
		return 0, err
	}
	// write all data
	n, err := s.w.Write(p)
	if err != nil {
		return n, err
	}
	// then wait for the tokens to allow the time needed for writing it all
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
