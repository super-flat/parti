package raft

import (
	"io"
	golog "log"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/super-flat/parti/logging"
)

var (
	protect sync.Once
	def     *log
)

// log implements the hashicorp log
// this is a wrapper around the parti logger to
// properly display raft logging info according to the logger provided
// by the implementor
type log struct {
	level  logging.Level
	logger logging.Logger
}

// newLog returns an instance of log
func newLog(level logging.Level, logger logging.Logger) *log {
	protect.Do(func() {
		if def == nil {
			def = &log{
				level:  level,
				logger: logger,
			}
		}
	})
	return def
}

// Log emits the message and args at the provided level
func (l log) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Info:
		l.logger.Infof(msg, args...)
	case hclog.Debug:
		l.logger.Debugf(msg, args...)
	case hclog.Warn:
		l.logger.Warnf(msg, args...)
	case hclog.Trace:
		l.logger.Tracef(msg, args...)
	case hclog.Error:
		l.logger.Errorf(msg, args...)
	default:
		l.logger.Debugf(msg, args...)
	}
}

// Trace emits the message and args at TRACE level
func (l log) Trace(msg string, args ...interface{}) {
	l.logger.Tracef(msg, args...)
}

// Debug emits the message and args at DEBUG level
func (l log) Debug(msg string, args ...interface{}) {
	l.logger.Debugf(msg, args...)
}

// Info emits the message and args at INFO level
func (l log) Info(msg string, args ...interface{}) {
	l.logger.Infof(msg, args...)
}

// Warn emits the message and args at WARN level
func (l log) Warn(msg string, args ...interface{}) {
	l.logger.Warnf(msg, args...)
}

// Error emits the message and args at ERROR level
func (l log) Error(msg string, args ...interface{}) {
	l.logger.Errorf(msg, args...)
}

// IsTrace indicates that the logger would emit TRACE level logs
func (l log) IsTrace() bool {
	return l.level == logging.TraceLevel
}

// IsDebug indicates that the logger would emit DEBUG level logs
func (l log) IsDebug() bool {
	return l.level == logging.DebugLevel
}

// IsInfo indicates that the logger would emit INFO level logs
func (l log) IsInfo() bool {
	return l.level == logging.InfoLevel
}

// IsWarn indicates that the logger would emit WARN level logs
func (l log) IsWarn() bool {
	return l.level == logging.WarningLevel
}

// IsError indicates that the logger would emit ERROR level logs
func (l log) IsError() bool {
	return l.level == logging.ErrorLevel
}

// ImpliedArgs returns the loggers implied args
func (l log) ImpliedArgs() []interface{} {
	return nil
}

// With return a sub-Logger for which every emitted log message will contain
// the given key/value pairs. This is used to create a context specific
// Logger. No necessary in this current use case
func (l log) With(args ...interface{}) hclog.Logger {
	return l
}

// Name returns the loggers name
func (l log) Name() string {
	return ""
}

// Named create a new sub-Logger that a name descending from the current name.
// This is used to create a subsystem specific Logger. However, this is not needed in this implementation
func (l log) Named(name string) hclog.Logger {
	return l
}

func (l log) ResetNamed(name string) hclog.Logger {
	// TODO proper implementation
	return l
}

// SetLevel update the logging level on-the-fly. This will affect all subloggers as
// well.
func (l log) SetLevel(level hclog.Level) {
	switch level {
	case hclog.Info:
		l.level = logging.InfoLevel
	case hclog.Debug:
		l.level = logging.DebugLevel
	case hclog.Warn:
		l.level = logging.WarningLevel
	case hclog.Trace:
		l.level = logging.TraceLevel
	case hclog.Error:
		l.level = logging.ErrorLevel
	default:
		l.level = logging.DebugLevel
	}
}

// GetLevel returns the log level
func (l log) GetLevel() hclog.Level {
	switch l.level {
	case logging.InfoLevel:
		return hclog.Info
	case logging.DebugLevel:
		return hclog.Debug
	case logging.TraceLevel:
		return hclog.Trace
	case logging.ErrorLevel:
	case logging.FatalLevel:
	case logging.PanicLevel:
		return hclog.Error
	case logging.WarningLevel:
		return hclog.Warn
	}
	return hclog.Debug
}

// StandardLogger create a *log.Logger that will send it's data through this Logger. This
// allows packages that expect to be using the standard library log to actually
// use this logger.
func (l log) StandardLogger(opts *hclog.StandardLoggerOptions) *golog.Logger {
	if opts == nil {
		opts = &hclog.StandardLoggerOptions{}
	}
	return golog.New(l.StandardWriter(opts), "", 0)
}

func (l log) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return nil
}
