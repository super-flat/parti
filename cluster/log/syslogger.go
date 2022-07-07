package log

import (
	"log"
)

type wrapper struct {
	*log.Logger
}

func (w *wrapper) Debug(args ...interface{}) {
	w.Print(args...)
}

func (w *wrapper) Debugf(format string, args ...interface{}) {
	w.Printf(format, args...)
}

func (w *wrapper) Info(args ...interface{}) {
	w.Print(args...)
}

func (w *wrapper) Infof(format string, args ...interface{}) {
	w.Printf(format, args...)
}

func (w *wrapper) Warn(args ...interface{}) {
	w.Print(args...)
}

func (w *wrapper) Warnf(format string, args ...interface{}) {
	w.Printf(format, args...)
}

func (w *wrapper) Error(args ...interface{}) {
	w.Print(args...)
}

func (w *wrapper) Errorf(format string, args ...interface{}) {
	w.Printf(format, args...)
}

func (w *wrapper) Fatal(args ...interface{}) {
	w.Logger.Fatal(args...)
}

func (w *wrapper) Fatalf(format string, args ...interface{}) {
	w.Logger.Fatalf(format, args...)
}

func (w *wrapper) Panic(args ...interface{}) {
	w.Logger.Panic(args...)
}

func (w *wrapper) Panicf(format string, args ...interface{}) {
	w.Logger.Panicf(format, args...)
}

func WrapSysLog(log *log.Logger) Logger {
	return &wrapper{log}
}
