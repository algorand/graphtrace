package graphtrace

import "log"

// Logger will receive error level logs
type Logger interface {
	// Printf will receive error level logs
	Printf(format string, args ...interface{})
}

// AdvancedLogger receives error and debug level logs
type AdvancedLogger interface {
	Debugf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type logLogger struct {
}

func (l *logLogger) Errorf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (l *logLogger) Debugf(format string, args ...interface{}) {
	// nothing, drop debug level logs
}

var logLoggerSingleton logLogger

type basicLogWrapper struct {
	l Logger
}

func (l *basicLogWrapper) Errorf(format string, args ...interface{}) {
	l.l.Printf(format, args...)
}

func (l *basicLogWrapper) Debugf(format string, args ...interface{}) {
	// nothing, drop debug level logs
}
