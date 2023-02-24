package logging

type Level int

const (
	// InfoLevel indicates Info log level.
	InfoLevel Level = iota
	// WarningLevel indicates Warning log level.
	WarningLevel
	// ErrorLevel indicates Error log level.
	ErrorLevel
	// FatalLevel indicates Fatal log level.
	FatalLevel
	// PanicLevel indicates Panic log level
	PanicLevel
	// DebugLevel indicates Debug log level
	DebugLevel
	// TraceLevel indicates Trace log level
	TraceLevel
	numLogLevels = 7
)

var levelName = [numLogLevels]string{
	InfoLevel:    "INFO",
	WarningLevel: "WARNING",
	ErrorLevel:   "ERROR",
	FatalLevel:   "FATAL",
	PanicLevel:   "PANIC",
	DebugLevel:   "DEBUG",
	TraceLevel:   "TRACE",
}
