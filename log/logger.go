package log

import (
	"fmt"
	"io"
	"log"
	"os"
)

// DefaultLogger define the standard log used by the package-level output functions.
var DefaultLogger = New("[PARTI]", os.Stderr, io.Discard)

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
	// Warningf logs to the WARNING and INFO logs.. Arguments are handled in the manner of fmt.Printf.
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
}

// New create new log from the given writers and verbosity.
// Each log level must have its own writer, if len of writers less than
// num of log level New will use last writer to fulfill missing.
// Otherwise, New will use os.Stderr as default.
//
// Use io.Discard to suppress message repetition to all lower writers.
//
// info := os.Stderr
// warn := io.Discard
// New("", info, warn)
//
// Use io.Discard in all lower writer's to set desired log level.
//
// info := io.Discard
// warn := io.Discard
// err :=  os.Stderr
// ....
// New("", info, warn, err)
//
// Note: a messages of a given log level are logged not only in the writer for that log level,
// but also in all writer's of lower log level. E.g.,
// a message of log level FATAL will be logged to the writers of log level FATAL, ERROR, WARNING, DEBUG, and INFO.
func New(prefix string, writers ...io.Writer) Logger {
	if len(writers) == 0 {
		writers = []io.Writer{os.Stderr}
	}

	if len(writers) < numLogLevels {
		last := writers[len(writers)-1]
		for i := len(writers); i < numLogLevels; i++ {
			writers = append(writers, last)
		}
	}

	ll := make([]*log.Logger, numLogLevels)
	for i := range writers {
		mw := make([]io.Writer, i+1)
		for j := 0; j <= i; j++ {
			mw[j] = writers[j]
		}

		w := io.MultiWriter(mw...)
		ll[i] = log.New(w, prefix, log.LstdFlags)
	}

	return &logger{
		ll: ll,
	}
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

func (l *logger) Debug(v ...any) {
	l.output(debugLevel, fmt.Sprint(v...))
}

func (l *logger) Debugf(format string, v ...any) {
	l.output(infoLevel, fmt.Sprintf(format, v...))
}

func (l *logger) Panic(v ...any) {
	s := fmt.Sprint(v...)
	l.output(panicLevel, s)
	panic(s)
}

func (l *logger) Panicf(format string, v ...any) {
	s := fmt.Sprintf(format, v...)
	l.output(panicLevel, s)
	panic(s)
}

func (l *logger) Fatal(v ...any) {
	l.output(fatalLevel, fmt.Sprint(v...))
	os.Exit(1)
}

func (l *logger) Fatalf(format string, v ...any) {
	l.output(fatalLevel, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (l *logger) Error(v ...any) {
	l.output(errorLevel, fmt.Sprint(v...))
}

func (l *logger) Errorf(format string, v ...any) {
	l.output(errorLevel, fmt.Sprintf(format, v...))
}

func (l *logger) Warning(v ...any) {
	l.output(warningLevel, fmt.Sprint(v...))
}

func (l *logger) Warningf(format string, v ...any) {
	l.output(warningLevel, fmt.Sprintf(format, v...))
}

func (l *logger) Info(v ...any) {
	l.output(infoLevel, fmt.Sprint(v...))
}

func (l *logger) Infof(format string, v ...any) {
	l.output(infoLevel, fmt.Sprintf(format, v...))
}

func (l *logger) output(level int, s string) {
	sevStr := levelName[level]
	err := l.ll[level].Output(2, fmt.Sprintf("%v: %v", sevStr, s))
	if err != nil {
		Panic(err)
	}
}
