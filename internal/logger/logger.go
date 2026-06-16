package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Level represents log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns string representation of level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses log level string
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Logger handles structured logging
type Logger struct {
	level  Level
	output io.Writer
	json   bool
}

// NewLogger creates a new logger
func NewLogger(level Level, output io.Writer, json bool) *Logger {
	return &Logger{
		level:  level,
		output: output,
		json:   json,
	}
}

// NewFileLogger creates a logger that writes to a file
func NewFileLogger(level Level, logDir string, json bool) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, err
	}

	logFile := filepath.Join(logDir, "dbbackupctl.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, err
	}

	return &Logger{
		level:  level,
		output: f,
		json:   json,
	}, nil
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields map[string]interface{}) {
	if l.level <= LevelDebug {
		l.log(LevelDebug, msg, fields)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string, fields map[string]interface{}) {
	if l.level <= LevelInfo {
		l.log(LevelInfo, msg, fields)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields map[string]interface{}) {
	if l.level <= LevelWarn {
		l.log(LevelWarn, msg, fields)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, fields map[string]interface{}) {
	if l.level <= LevelError {
		l.log(LevelError, msg, fields)
	}
}

// log writes a log entry
func (l *Logger) log(level Level, msg string, fields map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level.String(),
		Message:   msg,
		Fields:    fields,
	}

	if l.json {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(l.output, "%s\n", data)
	} else {
		fmt.Fprintf(l.output, "[%s] %s %s", entry.Timestamp, entry.Level, msg)
		if len(fields) > 0 {
			for k, v := range fields {
				fmt.Fprintf(l.output, " %s=%v", k, v)
			}
		}
		fmt.Fprintln(l.output)
	}
}