package pkgs

import (
	"fmt"
	"strings"
)

type Logger interface {
	Trace(m ...any)
	Debug(m ...any)
	Info(m ...any)
	Warn(m ...any)
	Error(m ...any)
	Tracef(format string, args ...any)
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

type LogLevel int

const (
	LOG_TRACE LogLevel = iota
	LOG_DEBUG
	LOG_INFO
	LOG_WARN
	LOG_ERROR
	LOG_NONE
)

func ParseLogLevel(str string) LogLevel {
	switch strings.ToUpper(str) {
	case "TRACE":
		return LOG_TRACE
	case "DEBUG":
		return LOG_DEBUG
	case "INFO":
		return LOG_INFO
	case "WARN":
		return LOG_WARN
	case "ERROR":
		return LOG_ERROR
	default:
		return LOG_NONE
	}
}

func NewLogger(level LogLevel) Logger {
	return &defaultLogger{level: level}
}

type defaultLogger struct {
	level LogLevel
}

var _ = Logger((*defaultLogger)(nil))

func (l *defaultLogger) Trace(m ...any) {
	if l.level > LOG_TRACE {
		return
	}
	fmt.Println(m...)
}

func (l *defaultLogger) Debug(m ...any) {
	if l.level > LOG_DEBUG {
		return
	}
	fmt.Println(m...)
}

func (l *defaultLogger) Info(m ...any) {
	if l.level > LOG_INFO {
		return
	}
	fmt.Println(m...)
}

func (l *defaultLogger) Warn(m ...any) {
	if l.level > LOG_WARN {
		return
	}
	fmt.Println(m...)
}

func (l *defaultLogger) Error(m ...any) {
	if l.level > LOG_ERROR {
		return
	}
	fmt.Println(m...)
}

func (l *defaultLogger) Tracef(format string, args ...any) {
	if l.level > LOG_WARN {
		return
	}
	fmt.Printf(format+"\n", args...)
}

func (l *defaultLogger) Debugf(format string, args ...any) {
	if l.level > LOG_WARN {
		return
	}
	fmt.Printf(format+"\n", args...)
}

func (l *defaultLogger) Infof(format string, args ...any) {
	if l.level > LOG_WARN {
		return
	}
	fmt.Printf(format+"\n", args...)
}

func (l *defaultLogger) Warnf(format string, args ...any) {
	if l.level > LOG_ERROR {
		return
	}
	fmt.Printf(format+"\n", args...)
}

func (l *defaultLogger) Errorf(format string, args ...any) {
	if l.level > LOG_ERROR {
		return
	}
	fmt.Printf(format+"\n", args...)
}
