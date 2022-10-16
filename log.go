package gotogen

import (
	"fmt"
)

type Logger interface {
	Debug(msg string)
	Debugf(format string, v ...any)
	Info(msg string)
	Infof(format string, v ...any)
}

// stdoutLogger is a bare-bones logger that outputs to whatever println is hooked up to. It has no concept of levels and
// will output everything at every level.
type stderrLogger struct{}

func (stderrLogger) Debug(msg string) {
	println(msg)
}

func (stderrLogger) Debugf(format string, v ...any) {
	println(fmt.Sprintf(format, v...))
}

func (stderrLogger) Info(msg string) {
	println(msg)
}

func (stderrLogger) Infof(format string, v ...any) {
	println(fmt.Sprintf(format, v...))
}
