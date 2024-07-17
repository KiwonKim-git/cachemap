package util

import "log"

type LOG_TYPE int

const (
	DEBUG LOG_TYPE = iota
	INFO
	WARN
	ERROR
)

func (l LOG_TYPE) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	}
	return "UNKNOWN"
}

type Logger struct {
	// Log level
	LogLevel LOG_TYPE
	Logger   *log.Logger
}

func NewLogger(logLevel LOG_TYPE, logger *log.Logger) *Logger {

	new := &Logger{
		LogLevel: logLevel,
		Logger:   logger,
	}

	if new.Logger == nil {
		new.Logger = log.Default()
	}

	new.Logger.Printf("Logger CREATE - Level: [%s], Flag: [%b]", new.LogLevel.String(), new.Logger.Flags())

	return new
}

func (l *Logger) PrintLogs(level LOG_TYPE, line string) {
	if l.LogLevel <= level {
		l.Logger.Print(line)
	}
}
