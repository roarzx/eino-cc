package observe

import (
	"fmt"
	"time"
)

type Logger struct {
	serviceName string
}

func NewLogger(serviceName string) *Logger {
	return &Logger{serviceName: serviceName}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.log("INFO", msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.log("ERROR", msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log("WARN", msg, args...)
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log("DEBUG", msg, args...)
}

func (l *Logger) log(level, msg string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedMsg := msg
	if len(args) > 0 {
		formattedMsg = fmt.Sprintf(msg, args...)
	}
	fmt.Printf("[%s] %s %s: %s\n", timestamp, l.serviceName, level, formattedMsg)
}