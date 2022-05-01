package remote

type Logger interface {
	Print(a ...any)
	Printf(format string, a ...any)
}

type noLog struct{}

func (l *noLog) Print(a ...any)                 {}
func (l *noLog) Printf(format string, a ...any) {}
