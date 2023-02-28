package logging

import (
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
)

// DefaultLogger represents the default logger to use
// This logger wraps zerolog under the hood
var DefaultLogger = newLogger(os.Stderr)

// Logger represents an active logging object that generates lines of
// output to an io.Writer.
type Logger interface {
	// Info starts a new message with info level.
	Info(...any)
	// Infof starts a new message with info level.
	Infof(string, ...any)
	// Warn starts a new message with warn level.
	Warn(...any)
	// Warnf starts a new message with warn level.
	Warnf(string, ...any)
	// Error starts a new message with error level.
	Error(...any)
	// Errorf starts a new message with error level.
	Errorf(string, ...any)
	// Fatal starts a new message with fatal level. The os.Exit(1) function
	// is called which terminates the program immediately.
	Fatal(...any)
	// Fatalf starts a new message with fatal level. The os.Exit(1) function
	// is called which terminates the program immediately.
	Fatalf(string, ...any)
	// Panic starts a new message with panic level. The panic() function
	// is called which stops the ordinary flow of a goroutine.
	Panic(...any)
	// Panicf starts a new message with panic level. The panic() function
	// is called which stops the ordinary flow of a goroutine.
	Panicf(string, ...any)
	// Debug starts a new message with debug level.
	Debug(...any)
	// Debugf starts a new message with debug level.
	Debugf(string, ...any)
	// Trace starts a new message with trace level
	Trace(...any)
	// Tracef starts a new message with trace level
	Tracef(string, ...any)
}

// Info starts a new message with info level.
func Info(v ...any) {
	DefaultLogger.Info(v...)
}

// Infof starts a new message with info level.
func Infof(format string, v ...any) {
	DefaultLogger.Infof(format, v...)
}

// Warn starts a new message with warn level.
func Warn(v ...any) {
	DefaultLogger.Warn(v...)
}

// Warnf starts a new message with warn level.
func Warnf(format string, v ...any) {
	DefaultLogger.Warnf(format, v...)
}

// Error starts a new message with error level.
func Error(v ...any) {
	DefaultLogger.Error(v...)
}

// Errorf starts a new message with error level.
func Errorf(format string, v ...any) {
	DefaultLogger.Errorf(format, v...)
}

// Fatal starts a new message with fatal level. The os.Exit(1) function
// is called which terminates the program immediately.
func Fatal(v ...any) {
	DefaultLogger.Fatal(v...)
}

// Fatalf starts a new message with fatal level. The os.Exit(1) function
// is called which terminates the program immediately.
func Fatalf(format string, v ...any) {
	DefaultLogger.Fatalf(format, v...)
}

// Panic starts a new message with panic level. The panic() function
// is called which stops the ordinary flow of a goroutine.
func Panic(v ...any) {
	DefaultLogger.Panic(v...)
}

// Panicf starts a new message with panic level. The panic() function
// is called which stops the ordinary flow of a goroutine.
func Panicf(format string, v ...any) {
	DefaultLogger.Panicf(format, v...)
}

// Debug starts a new message with debug level
func Debug(v ...any) {
	DefaultLogger.Debug(v...)
}

// Debugf starts a new message with debug level
func Debugf(format string, v ...any) {
	DefaultLogger.Debugf(format, v...)
}

// logger is the default logger
// it wraps the standard golang logging library
type logger struct {
	underlying zerolog.Logger
}

var _ Logger = &logger{}

// newLogger creates an instance of logger
func newLogger(w io.Writer) *logger {
	// create an instance of zerolog logger
	zlogger := zerolog.New(w).With().Timestamp().Logger()
	// create the instance of logger and returns it
	return &logger{underlying: zlogger}
}

// Debug starts a message with debug level
func (l *logger) Debug(v ...any) {
	l.underlying.Debug().Msg(fmt.Sprint(v...))
}

// Debugf starts a message with debug level
func (l *logger) Debugf(format string, v ...any) {
	l.underlying.Debug().Msgf(format, v...)
}

// Panic starts a new message with panic level. The panic() function
// is called which stops the ordinary flow of a goroutine.
func (l *logger) Panic(v ...any) {
	l.underlying.Panic().Msg(fmt.Sprint(v...))
}

// Panicf starts a new message with panic level. The panic() function
// is called which stops the ordinary flow of a goroutine.
func (l *logger) Panicf(format string, v ...any) {
	l.underlying.Panic().Msgf(format, v...)
}

// Fatal starts a new message with fatal level. The os.Exit(1) function
// is called which terminates the program immediately.
func (l *logger) Fatal(v ...any) {
	l.underlying.Fatal().Msg(fmt.Sprint(v...))
}

// Fatalf starts a new message with fatal level. The os.Exit(1) function
// is called which terminates the program immediately.
func (l *logger) Fatalf(format string, v ...any) {
	l.underlying.Fatal().Msgf(format, v...)
}

// Error starts a new message with error level.
func (l *logger) Error(v ...any) {
	l.underlying.Error().Msg(fmt.Sprint(v...))
}

// Errorf starts a new message with error level.
func (l *logger) Errorf(format string, v ...any) {
	l.underlying.Error().Msgf(format, v...)
}

// Warn starts a new message with warn level
func (l *logger) Warn(v ...any) {
	l.underlying.Warn().Msg(fmt.Sprint(v...))
}

// Warnf starts a new message with warn level
func (l *logger) Warnf(format string, v ...any) {
	l.underlying.Warn().Msgf(format, v...)
}

// Info starts a message with info level
func (l *logger) Info(v ...any) {
	l.underlying.Info().Msg(fmt.Sprint(v...))
}

// Infof starts a message with info level
func (l *logger) Infof(format string, v ...any) {
	l.underlying.Info().Msgf(format, v...)
}

// Trace starts a new message with trace level
func (l *logger) Trace(v ...any) {
	l.underlying.Trace().Msg(fmt.Sprint(v...))
}

// Tracef starts a new message with trace level
func (l *logger) Tracef(format string, v ...any) {
	l.underlying.Trace().Msgf(format, v...)
}
