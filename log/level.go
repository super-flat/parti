package log

const (
	// infoLevel indicates Info log level.
	infoLevel int = iota
	// warningLevel indicates Warning log level.
	warningLevel
	// errorLevel indicates Error log level.
	errorLevel
	// fatalLevel indicates Fatal log level.
	fatalLevel
	// panicLevel indicates Panic log level
	panicLevel
	// debugLevel indicates Debug log level
	debugLevel

	// specifies the total number of log level
	numLogLevels = 6
)

var levelName = [numLogLevels]string{
	infoLevel:    "INFO",
	warningLevel: "WARNING",
	errorLevel:   "ERROR",
	fatalLevel:   "FATAL",
	panicLevel:   "PANIC",
	debugLevel:   "DEBUG",
}
