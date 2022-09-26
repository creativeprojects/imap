package lib

import "testing"

type Logger interface {
	Print(a ...any)
	Println(a ...any)
	Printf(format string, a ...any)
}

type NoLog struct{}

func (l *NoLog) Print(a ...any)                 {}
func (l *NoLog) Println(a ...any)               {}
func (l *NoLog) Printf(format string, a ...any) {}

type TestLogger struct {
	t      *testing.T
	prefix string
}

func NewTestLogger(t *testing.T, prefix string) *TestLogger {
	return &TestLogger{
		t:      t,
		prefix: prefix,
	}
}

func (l *TestLogger) Print(a ...any) {
	if l.prefix == "" {
		l.t.Log(a...)
	} else {
		l.t.Log(append([]any{l.prefix + ":"}, a...)...)
	}
}

func (l *TestLogger) Println(a ...any) {
	l.Print(a...)
}

func (l *TestLogger) Printf(format string, a ...any) {
	if l.prefix != "" {
		format = l.prefix + ": " + format
	}
	l.t.Logf(format, a...)
}
