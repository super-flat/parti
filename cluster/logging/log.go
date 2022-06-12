package logging

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Debug starts a new message with debug level.
func Debug(args ...interface{}) {
	log.Debug().Msg(fmt.Sprint(args...))
}

// Debugf starts a new message with debug level.
func Debugf(format string, args ...interface{}) {
	log.Debug().Msgf(format, args...)
}

// Info starts a new message with info level.
func Info(args ...interface{}) {
	log.Info().Msg(fmt.Sprint(args...))
}

// Infof starts a new message with info level.
func Infof(format string, args ...interface{}) {
	log.Info().Msgf(format, args...)
}

// Warn starts a new message with warn level.
func Warn(args ...interface{}) {
	log.Warn().Msg(fmt.Sprint(args...))
}

// Warnf starts a new message with warn level.
func Warnf(format string, args ...interface{}) {
	log.Warn().Msgf(format, args...)
}

// Error starts a new message with error level. .
func Error(args ...interface{}) {
	log.Error().Msg(fmt.Sprint(args...))
}

// Errorf starts a new message with error level.
func Errorf(format string, args ...interface{}) {
	log.Error().Msgf(format, args...)
}

// Fatal starts a new message with fatal level. The os.Exit(1) function
// is called by the Msg method.
func Fatal(args ...interface{}) {
	log.Fatal().Msg(fmt.Sprint(args...))
}

// Fatalf starts a new message with fatal level. The os.Exit(1) function
// is called by the Msg method.
func Fatalf(format string, args ...interface{}) {
	log.Fatal().Msgf(format, args...)
}

// Panic starts a new message with panic level. The message is also sent
// to the panic function.
func Panic(args ...interface{}) {
	log.Panic().Msg(fmt.Sprint(args...))
}

// Panicf starts a new message with panic level. The message is also sent
// to the panic function.
func Panicf(format string, args ...interface{}) {
	log.Panic().Msgf(format, args...)
}

// WithLevel starts a new message with level.
func WithLevel(level zerolog.Level) {
	log.WithLevel(level)
}

// Log starts a new message with no level. Setting zerolog.GlobalLevel to
// zerolog.Disabled will still disable events produced by this method.
func Log(args ...interface{}) {
	log.Log().Msg(fmt.Sprint(args...))
}

// Print sends a log event using debug level and no extra field.
// Arguments are handled in the manner of fmt.Print.
func Print(args ...interface{}) {
	log.Print(args...)
}

// Printf sends a log event using debug level and no extra field.
// Arguments are handled in the manner of fmt.Printf.
func Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// Trace sends a log event using trace level and no extra field.
// Arguments are handled in the manner of fmt.Print.
func Trace(args ...interface{}) {
	log.Trace().Msg(fmt.Sprint(args...))
}

// Tracef sends a log event using trace level and no extra field.
// Arguments are handled in the manner of fmt.Printf.
func Tracef(format string, args ...interface{}) {
	log.Trace().Msgf(format, args...)
}
