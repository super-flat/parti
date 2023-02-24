package cluster

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
type log struct {
	level  logging.Level
	logger logging.Logger
}

var _ hclog.Logger = &log{}

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

func (l log) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Info:
		l.logger.Infof(msg, args...)
	case hclog.Debug:
		l.logger.Debugf(msg, args...)
	case hclog.Warn:
		l.logger.Warningf(msg, args...)
	case hclog.Trace:
		l.logger.Tracef(msg, args...)
	case hclog.Error:
		l.logger.Errorf(msg, args...)
	default:
		l.logger.Debugf(msg, args...)
	}
}

func (l log) Trace(msg string, args ...interface{}) {
	l.logger.Tracef(msg, args...)
}

func (l log) Debug(msg string, args ...interface{}) {
	l.logger.Debugf(msg, args...)
}

func (l log) Info(msg string, args ...interface{}) {
	l.logger.Infof(msg, args...)
}

func (l log) Warn(msg string, args ...interface{}) {
	l.logger.Warningf(msg, args...)
}

func (l log) Error(msg string, args ...interface{}) {
	l.logger.Errorf(msg, args...)
}

func (l log) IsTrace() bool {
	return l.level == logging.TraceLevel
}

func (l log) IsDebug() bool {
	return l.level == logging.DebugLevel
}

func (l log) IsInfo() bool {
	return l.level == logging.InfoLevel
}

func (l log) IsWarn() bool {
	return l.level == logging.WarningLevel
}

func (l log) IsError() bool {
	return l.level == logging.ErrorLevel
}

func (l log) ImpliedArgs() []interface{} {
	return nil
}

func (l log) With(args ...interface{}) hclog.Logger {
	// TODO proper implementation
	return l
}

func (l log) Name() string {
	// TODO proper implementation
	return ""
}

func (l log) Named(name string) hclog.Logger {
	// TODO proper implementation
	return l
}

func (l log) ResetNamed(name string) hclog.Logger {
	// TODO proper implementation
	return l
}

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

func (l log) StandardLogger(opts *hclog.StandardLoggerOptions) *golog.Logger {
	if opts == nil {
		opts = &hclog.StandardLoggerOptions{}
	}

	return golog.New(l.StandardWriter(opts), "", 0)
}

func (l log) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	// TODO proper implementation
	return nil
}
