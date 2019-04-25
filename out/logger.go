package out

import (
	"log"
)

type ApmLogger struct {
	*log.Logger
}

func (l *ApmLogger) Debugf(format string, args ...interface{}) {
	l.Printf("[debug] "+format, args...)
}

func (l *ApmLogger) Errorf(format string, args ...interface{}) {
	l.Printf("[error] "+format, args...)
}

func NewApmLogger(logger *log.Logger) *ApmLogger {
	return &ApmLogger{
		Logger: logger,
	}
}
