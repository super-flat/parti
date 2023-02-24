package logging

import (
	"fmt"
	"io"
	"log"
	"os"
)

var DefaultLogger = newLogger("PARTI", os.Stderr, io.Discard)

// Logger represents an active logging object that generates lines of
// output to an io.Writer.
type Logger interface {
	// Info logs to INFO log. Arguments are handled in the manner of fmt.Println.
	Info(...interface{})
	// Infof logs to INFO log. Arguments are handled in the manner of fmt.Printf.
	// A newline is appended if the last character of format is not
	// already a newline.
	Infof(string, ...interface{})
	// Warning logs to the WARNING and INFO logs. Arguments are handled in the manner of fmt.Println.
	Warning(...interface{})
	// Warningf logs to the WARNING and INFO logs. Arguments are handled in the manner of fmt.Printf.
	// A newline is appended if the last character of format is not
	// already a newline.
	Warningf(string, ...interface{})
	// Error logs to the ERROR, WARNING, and INFO logs. Arguments are handled in the manner of fmt.Print.
	Error(...interface{})
	// Errorf logs to the ERROR, WARNING, and INFO logs. Arguments are handled in the manner of fmt.Printf.
	// A newline is appended if the last character of format is not
	// already a newline.
	Errorf(string, ...interface{})
	// Fatal logs to the FATAL, ERROR, WARNING, and INFO logs followed by a call to os.Exit(1).
	// Arguments are handled in the manner of fmt.Println.
	Fatal(...interface{})
	// Fatalf logs to the FATAL, ERROR, WARNING, and INFO logs followed by a call to os.Exit(1).
	// Arguments are handled in the manner of fmt.Printf.
	// A newline is appended if the last character of format is not
	// already a newline.
	Fatalf(string, ...interface{})
	// Panic logs to the PANIC, FATAL, ERROR, WARNING, and INFO logs followed by a call to panic().
	// Arguments are handled in the manner of fmt.Println.
	Panic(...interface{})
	// Panicf logs to the PANIC, ERROR, WARNING, and INFO logs followed by a call to panic().
	// Arguments are handled in the manner of fmt.Printf.
	// A newline is appended if the last character of format is not
	// already a newline.
	Panicf(string, ...interface{})
	// Debug logs to DEBUG log. Arguments are handled in the manner of fmt.Println.
	Debug(...interface{})
	// Debugf logs to DEBUG log. Arguments are handled in the manner of fmt.Printf.
	// A newline is appended if the last character of format is not
	// already a newline.
	Debugf(string, ...interface{})
	// Trace logs to TRACE log
	Trace(...interface{})
	Tracef(string, ...interface{})
}

// Info logs to DEBUG, WARNING and INFO log. Arguments are handled in the manner of fmt.Println.
func Info(v ...any) {
	DefaultLogger.Info(v...)
}

// Infof logs to DEBUG, WARNING and INFO log. Arguments are handled in the manner of fmt.Printf.
// A newline is appended if the last character of format is not
// already a newline.
func Infof(format string, v ...any) {
	DefaultLogger.Infof(format, v...)
}

// Warning logs to the WARNING and INFO logs. Arguments are handled in the manner of fmt.Println.
func Warning(v ...any) {
	DefaultLogger.Warning(v...)
}

// Warningf logs to the WARNING and INFO logs. Arguments are handled in the manner of fmt.Printf.
// A newline is appended if the last character of format is not
// already a newline.
func Warningf(format string, v ...any) {
	DefaultLogger.Warningf(format, v...)
}

// Error logs to the ERROR, WARNING, DEBUG and INFO logs. Arguments are handled in the manner of fmt.Print.
func Error(v ...any) {
	DefaultLogger.Error(v...)
}

// Errorf logs to the ERROR, WARNING, DEBUG and INFO logs. Arguments are handled in the manner of fmt.Printf.
// A newline is appended if the last character of format is not
// already a newline.
func Errorf(format string, v ...any) {
	DefaultLogger.Errorf(format, v...)
}

// Fatal logs to the FATAL, ERROR, WARNING, DEBUG and INFO logs followed by a call to os.Exit(1).
// Arguments are handled in the manner of fmt.Println.
func Fatal(v ...any) {
	DefaultLogger.Fatal(v...)
}

// Fatalf logs to the FATAL, ERROR, WARNING, DEBUG and INFO logs followed by a call to os.Exit(1).
// Arguments are handled in the manner of fmt.Printf.
// A newline is appended if the last character of format is not
// already a newline.
func Fatalf(format string, v ...any) {
	DefaultLogger.Fatalf(format, v...)
}

// Panic logs to the PANIC, FATAL, ERROR, WARNING, DEBUG and INFO logs followed by a call to panic().
// Arguments are handled in the manner of fmt.Println.
func Panic(v ...any) {
	DefaultLogger.Panic(v...)
}

// Panicf logs to the PANIC, ERROR, WARNING, DEBUG and INFO logs followed by a call to panic().
// Arguments are handled in the manner of fmt.Printf.
// A newline is appended if the last character of format is not
// already a newline.
func Panicf(format string, v ...any) {
	DefaultLogger.Panicf(format, v...)
}

// logger is the default logger used by raft.
type logger struct {
	ll []*log.Logger
}

func newLogger(prefix string, writers ...io.Writer) *logger {
	if len(writers) == 0 {
		writers = []io.Writer{os.Stderr}
	}

	if len(writers) < numLogLevels {
		last := writers[len(writers)-1]
		for i := len(writers); i < numLogLevels; i++ {
			writers = append(writers, last)
		}
	}

	loggers := make([]*log.Logger, numLogLevels)
	for i := range writers {
		mw := make([]io.Writer, i+1)
		for j := 0; j <= i; j++ {
			mw[j] = writers[j]
		}

		w := io.MultiWriter(mw...)
		loggers[i] = log.New(w, prefix, log.LstdFlags)
	}

	return &logger{
		ll: loggers,
	}
}

func (l *logger) Debug(v ...any) {
	l.output(DebugLevel, fmt.Sprint(v...))
}

func (l *logger) Debugf(format string, v ...any) {
	l.output(InfoLevel, fmt.Sprintf(format, v...))
}

func (l *logger) Panic(v ...any) {
	s := fmt.Sprint(v...)
	l.output(PanicLevel, s)
	panic(s)
}

func (l *logger) Panicf(format string, v ...any) {
	s := fmt.Sprintf(format, v...)
	l.output(PanicLevel, s)
	panic(s)
}

func (l *logger) Fatal(v ...any) {
	l.output(FatalLevel, fmt.Sprint(v...))
	os.Exit(1)
}

func (l *logger) Fatalf(format string, v ...any) {
	l.output(FatalLevel, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (l *logger) Error(v ...any) {
	l.output(ErrorLevel, fmt.Sprint(v...))
}

func (l *logger) Errorf(format string, v ...any) {
	l.output(ErrorLevel, fmt.Sprintf(format, v...))
}

func (l *logger) Warning(v ...any) {
	l.output(WarningLevel, fmt.Sprint(v...))
}

func (l *logger) Warningf(format string, v ...any) {
	l.output(WarningLevel, fmt.Sprintf(format, v...))
}

func (l *logger) Info(v ...any) {
	l.output(InfoLevel, fmt.Sprint(v...))
}

func (l *logger) Infof(format string, v ...any) {
	l.output(InfoLevel, fmt.Sprintf(format, v...))
}

func (l *logger) Trace(v ...interface{}) {
	l.output(TraceLevel, fmt.Sprint(v...))
}

func (l *logger) Tracef(format string, v ...interface{}) {
	l.output(TraceLevel, fmt.Sprintf(format, v...))
}

func (l *logger) output(level Level, s string) {
	sevStr := levelName[level]
	err := l.ll[level].Output(2, fmt.Sprintf("%v: %v", sevStr, s))
	if err != nil {
		panic(err)
	}
}
