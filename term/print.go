package term

import "github.com/pterm/pterm"

type Level int

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
)

var lvl = LevelInfo

func SetLevel(level Level) {
	lvl = level
}

func Debug(a ...any) {
	if lvl > LevelDebug {
		return
	}
	pterm.FgLightCyan.Println(a...)
}

func Debugf(format string, a ...any) {
	if lvl > LevelDebug {
		return
	}
	pterm.FgLightCyan.Printfln(format, a...)
}

func Info(a ...any) {
	if lvl > LevelInfo {
		return
	}
	pterm.FgLightGreen.Println(a...)
}

func Infof(format string, a ...any) {
	if lvl > LevelInfo {
		return
	}
	pterm.FgLightGreen.Printfln(format, a...)
}

func Warn(a ...any) {
	if lvl > LevelWarn {
		return
	}
	pterm.FgYellow.Println(a...)
}

func Warnf(format string, a ...any) {
	if lvl > LevelWarn {
		return
	}
	pterm.FgYellow.Printfln(format, a...)
}

func Error(a ...any) {
	pterm.FgLightRed.Println(a...)
}

func Errorf(format string, a ...any) {
	pterm.FgLightRed.Printfln(format, a...)
}
