package lib

type Logger interface {
	Print(a ...any)
	Printf(format string, a ...any)
}

type NoLog struct{}

func (l *NoLog) Print(a ...any)                 {}
func (l *NoLog) Printf(format string, a ...any) {}
