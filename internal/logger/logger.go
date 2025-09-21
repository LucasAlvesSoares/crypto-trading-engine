package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// NewLogger creates a new configured logger
func NewLogger(level, format string) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	// Set log format
	if format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// Set output
	logger.SetOutput(os.Stdout)

	return logger
}

// WithComponent returns a logger entry with component field
func WithComponent(logger *logrus.Logger, component string) *logrus.Entry {
	return logger.WithField("component", component)
}

