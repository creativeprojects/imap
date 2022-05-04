package storage

import "testing"

type testLogger struct {
	t *testing.T
}

func (l *testLogger) Print(a ...any) {
	l.t.Log(a...)
}

func (l *testLogger) Printf(format string, a ...any) {
	l.t.Logf(format, a...)
}
