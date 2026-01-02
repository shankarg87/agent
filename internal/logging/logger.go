package logging

import (
"log/slog"
"os"
"strings"
)

// LogLevel represents the supported log levels
type LogLevel string

const (
LogLevelDebug LogLevel = "debug"
LogLevelInfo  LogLevel = "info"
LogLevelWarn  LogLevel = "warn"
LogLevelError LogLevel = "error"
)

// Logger wraps slog.Logger with additional convenience methods
type Logger struct {
*slog.Logger
}

// NewLogger creates a new logger with the specified level and component name
func NewLogger(level LogLevel, component string) *Logger {
var slogLevel slog.Level

switch strings.ToLower(string(level)) {
case "debug":
slogLevel = slog.LevelDebug
case "info":
slogLevel = slog.LevelInfo
case "warn", "warning":
slogLevel = slog.LevelWarn
case "error":
slogLevel = slog.LevelError
default:
slogLevel = slog.LevelInfo
}

opts := &slog.HandlerOptions{
Level: slogLevel,
}

handler := slog.NewTextHandler(os.Stdout, opts)
logger := slog.New(handler)

// Add component context to all logs
logger = logger.With("component", component)

return &Logger{logger}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...any) {
l.Logger.Debug(msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...any) {
l.Logger.Info(msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
l.Logger.Warn(msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
l.Logger.Error(msg, args...)
}

// With returns a Logger that includes the given attributes
func (l *Logger) With(args ...any) *Logger {
return &Logger{l.Logger.With(args...)}
}
