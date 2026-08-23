// Package logx provides a minimal leveled logger backed by the standard library.
package logx

import (
	"io"
	"log"
)

type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

type Logger struct {
	level Level
	l     *log.Logger
}

func New(level string, w io.Writer, prefix string) *Logger {
	return &Logger{level: parseLevel(level), l: log.New(w, prefix, log.LstdFlags)}
}

func parseLevel(s string) Level {
	switch s {
	case "debug":
		return Debug
	case "warn":
		return Warn
	case "error":
		return Error
	default:
		return Info
	}
}

func (l *Logger) logf(min Level, format string, args ...any) {
	if l.level <= min {
		l.l.Printf(format, args...)
	}
}

func (l *Logger) Debugf(format string, args ...any) { l.logf(Debug, format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.logf(Info, format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.logf(Warn, format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.logf(Error, format, args...) }
func (l *Logger) Fatalf(format string, args ...any) { l.l.Fatalf(format, args...) }
