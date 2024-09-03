package util

import (
	"log"
	"os"
	"runtime"
	"strings"
)

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

var defaultLogger *Logger = nil

// Default returns the default logger. It will create a new logger with minimum log level based on the ENV_VERBOSE environment variable.
// If ENV_VERBOSE is set to "debug", the DEBUG and above level logs will be printed. Otherwise, only ERROR logs will be printed.
func Default() *Logger {
	if defaultLogger == nil {
		verb := ERROR
		if os.Getenv("ENV_VERBOSE") == "debug" {
			verb = DEBUG
		}
		new := log.Default()
		new.SetFlags(log.LstdFlags | log.Lmicroseconds)

		defaultLogger = NewLogger(verb, new)
	}
	return defaultLogger
}

// NewLogger creates a logger with the specified log level and logger.
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

func (l *Logger) PrintLogs(level LOG_TYPE, log string) {
	if l.LogLevel <= level {
		_, path, line, ok := runtime.Caller(1)
		filename := path[strings.LastIndex(path, "/")+1:]
		if ok {
			l.Logger.Printf("%s:%d: %s", filename, line, log)
		} else {
			l.Logger.Print(log)
		}
	}
}
