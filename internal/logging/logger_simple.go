package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

// SimpleLogger wraps slog with additional functionality for structured logging
type SimpleLogger struct {
	*slog.Logger
	component string
	verbose   bool
}

// Use the existing LogLevel constants from logger.go

// LogConfig holds configuration for logging
type LogConfig struct {
	Level           LogLevel `yaml:"level"`
	Format          string   `yaml:"format"`           // "json" or "text"
	AddSource       bool     `yaml:"add_source"`       // Add source file/line info
	Component       string   `yaml:"component"`        // Component name for structured logging
	TimestampFormat string   `yaml:"timestamp_format"` // Custom timestamp format
	Verbose         bool     `yaml:"verbose"`          // Enable verbose logging
}

// NewSimpleLogger creates a new simple logger with the given configuration
func NewSimpleLogger(config LogConfig) *SimpleLogger {
	// Parse log level
	var level slog.Level
	switch config.Level {
	case LogLevelDebug:
		level = slog.LevelDebug
	case LogLevelInfo:
		level = slog.LevelInfo
	case LogLevelWarn:
		level = slog.LevelWarn
	case LogLevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// If verbose is enabled, force debug level
	if config.Verbose {
		level = slog.LevelDebug
	}

	// Configure handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
	}

	// Create handler based on format
	var handler slog.Handler
	if config.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	// Create base logger
	baseLogger := slog.New(handler)

	// Add component context if specified
	if config.Component != "" {
		baseLogger = baseLogger.With("component", config.Component)
	}

	return &SimpleLogger{
		Logger:    baseLogger,
		component: config.Component,
		verbose:   config.Verbose,
	}
}

// WithContext returns a logger with context information
func (l *SimpleLogger) WithContext(ctx context.Context) *SimpleLogger {
	return &SimpleLogger{
		Logger:    l.Logger,
		component: l.component,
		verbose:   l.verbose,
	}
}

// WithFields returns a logger with additional structured fields
func (l *SimpleLogger) WithFields(fields map[string]interface{}) *SimpleLogger {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	return &SimpleLogger{
		Logger:    l.Logger.With(args...),
		component: l.component,
		verbose:   l.verbose,
	}
}

// Verbose logs detailed information for debugging and troubleshooting
func (l *SimpleLogger) Verbose(msg string, args ...interface{}) {
	if l.verbose || l.Logger.Enabled(context.Background(), slog.LevelDebug) {
		allArgs := append([]interface{}{"level", "verbose"}, args...)
		l.Logger.Debug(msg, allArgs...)
	}
}

// VerboseWithContext logs detailed information with context
func (l *SimpleLogger) VerboseWithContext(ctx context.Context, msg string, args ...interface{}) {
	if l.verbose || l.Logger.Enabled(ctx, slog.LevelDebug) {
		allArgs := append([]interface{}{"level", "verbose"}, args...)
		l.Logger.DebugContext(ctx, msg, allArgs...)
	}
}

// LogRunStart logs the start of a run with detailed context
func (l *SimpleLogger) LogRunStart(runID, sessionID string, request interface{}) {
	l.Info("Run started",
		"run_id", runID,
		"session_id", sessionID,
		"timestamp", time.Now().UTC(),
	)

	if l.verbose {
		l.Verbose("Run request details",
			"run_id", runID,
			"request", fmt.Sprintf("%+v", request),
		)
	}
}

// LogRunComplete logs the completion of a run
func (l *SimpleLogger) LogRunComplete(runID string, duration time.Duration, status string) {
	l.Info("Run completed",
		"run_id", runID,
		"duration_ms", duration.Milliseconds(),
		"status", status,
		"timestamp", time.Now().UTC(),
	)
}

// LogToolCall logs a tool invocation with details
func (l *SimpleLogger) LogToolCall(runID, toolName string, args interface{}) {
	l.Verbose("Tool call initiated",
		"run_id", runID,
		"tool_name", toolName,
		"args", fmt.Sprintf("%+v", args),
		"timestamp", time.Now().UTC(),
		"caller", GetCaller(1),
	)
}

// LogToolResult logs the result of a tool call
func (l *SimpleLogger) LogToolResult(runID, toolName string, duration time.Duration, result interface{}, err error) {
	if err != nil {
		l.Error("Tool call failed",
			"run_id", runID,
			"tool_name", toolName,
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
			"timestamp", time.Now().UTC(),
		)
	} else {
		l.Verbose("Tool call completed",
			"run_id", runID,
			"tool_name", toolName,
			"duration_ms", duration.Milliseconds(),
			"timestamp", time.Now().UTC(),
		)

		if l.verbose && result != nil {
			l.Verbose("Tool call result",
				"run_id", runID,
				"tool_name", toolName,
				"result", fmt.Sprintf("%+v", result),
			)
		}
	}
}

// LogStateTransition logs state machine transitions
func (l *SimpleLogger) LogStateTransition(runID, fromState, toState, reason string) {
	l.Verbose("State transition",
		"run_id", runID,
		"from_state", fromState,
		"to_state", toState,
		"reason", reason,
		"timestamp", time.Now().UTC(),
		"caller", GetCaller(1),
	)
}

// LogEvent logs a generic event with structured data
func (l *SimpleLogger) LogEvent(eventType string, data map[string]interface{}) {
	args := []interface{}{
		"event_type", eventType,
		"timestamp", time.Now().UTC(),
	}
	for k, v := range data {
		args = append(args, k, v)
	}
	l.Verbose("Event occurred", args...)
}

// LogPerformance logs performance metrics
func (l *SimpleLogger) LogPerformance(operation string, duration time.Duration, metadata map[string]interface{}) {
	args := []interface{}{
		"operation", operation,
		"duration_ms", duration.Milliseconds(),
		"timestamp", time.Now().UTC(),
	}
	for k, v := range metadata {
		args = append(args, k, v)
	}
	l.Verbose("Performance metric", args...)
}

// LogRequest logs HTTP request details
func (l *SimpleLogger) LogRequest(method, path, remoteAddr string, headers map[string]string) {
	l.Verbose("HTTP request received",
		"method", method,
		"path", path,
		"remote_addr", remoteAddr,
		"timestamp", time.Now().UTC(),
	)

	if l.verbose && len(headers) > 0 {
		l.Verbose("HTTP request headers",
			"method", method,
			"path", path,
			"headers", headers,
		)
	}
}

// LogResponse logs HTTP response details
func (l *SimpleLogger) LogResponse(method, path string, statusCode int, duration time.Duration) {
	l.Verbose("HTTP response sent",
		"method", method,
		"path", path,
		"status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
		"timestamp", time.Now().UTC(),
	)
}

// LogConfigLoad logs configuration loading
func (l *SimpleLogger) LogConfigLoad(configPath string, config interface{}) {
	l.Info("Configuration loaded",
		"config_path", configPath,
		"timestamp", time.Now().UTC(),
	)

	if l.verbose {
		l.Verbose("Configuration details",
			"config_path", configPath,
			"config", fmt.Sprintf("%+v", config),
		)
	}
}

// LogProviderCall logs LLM provider API calls
func (l *SimpleLogger) LogProviderCall(provider, model string, tokenCount int, cost float64) {
	l.Verbose("Provider API call",
		"provider", provider,
		"model", model,
		"token_count", tokenCount,
		"cost_usd", cost,
		"timestamp", time.Now().UTC(),
	)
}

// LogMCPConnection logs MCP server connections
func (l *SimpleLogger) LogMCPConnection(serverName, transport, endpoint string, connected bool) {
	if connected {
		l.Info("MCP server connected",
			"server_name", serverName,
			"transport", transport,
			"endpoint", endpoint,
			"timestamp", time.Now().UTC(),
		)
	} else {
		l.Warn("MCP server disconnected",
			"server_name", serverName,
			"transport", transport,
			"endpoint", endpoint,
			"timestamp", time.Now().UTC(),
		)
	}
}

// LogMemoryOperation logs memory store operations
func (l *SimpleLogger) LogMemoryOperation(operation, key string, success bool, duration time.Duration) {
	l.Verbose("Memory operation",
		"operation", operation,
		"key", key,
		"success", success,
		"duration_ms", duration.Milliseconds(),
		"timestamp", time.Now().UTC(),
	)
}

// GetCaller returns information about the calling function
func GetCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown"
	}

	// Get just the filename, not the full path
	parts := strings.Split(file, "/")
	filename := parts[len(parts)-1]

	return fmt.Sprintf("%s:%d", filename, line)
}

// DefaultLogger creates a default logger for the application
func DefaultLogger(component string) *SimpleLogger {
	return NewSimpleLogger(LogConfig{
		Level:     LogLevelInfo,
		Format:    "json",
		AddSource: true,
		Component: component,
		Verbose:   false,
	})
}

// VerboseLogger creates a verbose logger for detailed debugging
func VerboseLogger(component string) *SimpleLogger {
	return NewSimpleLogger(LogConfig{
		Level:     LogLevelDebug,
		Format:    "json",
		AddSource: true,
		Component: component,
		Verbose:   true,
	})
}
