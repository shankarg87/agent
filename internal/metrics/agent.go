package metrics

import (
	"context"
	"strconv"
	"time"
)

// AgentMetrics provides high-level typed methods for agent runtime metrics
type AgentMetrics struct {
	provider Provider
}

// NewAgentMetrics creates a new agent metrics wrapper
func NewAgentMetrics(provider Provider) *AgentMetrics {
	return &AgentMetrics{
		provider: provider,
	}
}

// Provider returns the underlying metrics provider
func (m *AgentMetrics) Provider() Provider {
	return m.provider
}

// Run Metrics

// RunCreated increments the run created counter
func (m *AgentMetrics) RunCreated(ctx context.Context, tenantID, mode string) {
	m.provider.IncrementCounter(ctx, RunsCreatedMetric.Name, 1, map[string]string{
		"tenant_id": tenantID,
		"mode":      mode,
	})
}

// RunCompleted records a completed run with its duration and final status
func (m *AgentMetrics) RunCompleted(ctx context.Context, tenantID, mode, status string, duration time.Duration) {
	labels := map[string]string{
		"tenant_id": tenantID,
		"mode":      mode,
		"status":    status,
	}

	m.provider.IncrementCounter(ctx, RunsCompletedMetric.Name, 1, labels)
	m.provider.RecordHistogram(ctx, RunDurationMetric.Name, duration.Seconds(), labels)
}

// SetActiveRuns sets the current number of active runs for a tenant
func (m *AgentMetrics) SetActiveRuns(ctx context.Context, tenantID string, count int64) {
	m.provider.SetGauge(ctx, ActiveRunsMetric.Name, float64(count), map[string]string{
		"tenant_id": tenantID,
	})
}

// IncrementActiveRuns increments the active runs counter
func (m *AgentMetrics) IncrementActiveRuns(ctx context.Context, tenantID string) {
	m.provider.AddGauge(ctx, ActiveRunsMetric.Name, 1, map[string]string{
		"tenant_id": tenantID,
	})
}

// DecrementActiveRuns decrements the active runs counter
func (m *AgentMetrics) DecrementActiveRuns(ctx context.Context, tenantID string) {
	m.provider.AddGauge(ctx, ActiveRunsMetric.Name, -1, map[string]string{
		"tenant_id": tenantID,
	})
}

// Tool Metrics

// ToolInvocation records a tool invocation with its duration and status
func (m *AgentMetrics) ToolInvocation(ctx context.Context, toolName, serverName, status string, duration time.Duration) {
	labels := map[string]string{
		"tool_name":   toolName,
		"server_name": serverName,
		"status":      status,
	}

	m.provider.IncrementCounter(ctx, ToolInvocationsMetric.Name, 1, labels)
	m.provider.RecordHistogram(ctx, ToolDurationMetric.Name, duration.Seconds(), labels)
}

// LLM Provider Metrics

// LLMRequest records an LLM request with its duration and status
func (m *AgentMetrics) LLMRequest(ctx context.Context, provider, model, status string, duration time.Duration) {
	labels := map[string]string{
		"provider": provider,
		"model":    model,
		"status":   status,
	}

	m.provider.IncrementCounter(ctx, LLMRequestsMetric.Name, 1, labels)
	m.provider.RecordHistogram(ctx, LLMDurationMetric.Name, duration.Seconds(), labels)
}

// LLMTokensUsed records token usage by type (prompt, completion, total)
func (m *AgentMetrics) LLMTokensUsed(ctx context.Context, provider, model, tokenType string, tokens int64) {
	labels := map[string]string{
		"provider":   provider,
		"model":      model,
		"token_type": tokenType,
	}

	m.provider.IncrementCounter(ctx, LLMTokensUsedMetric.Name, tokens, labels)
}

// HTTP API Metrics

// HTTPRequest records an HTTP request with its duration and status
func (m *AgentMetrics) HTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	labels := map[string]string{
		"method":       method,
		"path":         m.sanitizePath(path),
		"status_code":  strconv.Itoa(statusCode),
		"status_class": m.getStatusClass(statusCode),
	}

	m.provider.IncrementCounter(ctx, HTTPRequestsMetric.Name, 1, labels)
	m.provider.RecordHistogram(ctx, HTTPDurationMetric.Name, duration.Seconds(), map[string]string{
		"method": method,
		"path":   m.sanitizePath(path),
	})
}

// Utility Methods

// sanitizePath converts dynamic path segments to template form to prevent metric explosion
func (m *AgentMetrics) sanitizePath(path string) string {
	// Convert UUIDs and IDs to template variables
	// /runs/550e8400-e29b-41d4-a716-446655440000 -> /runs/{id}
	// /runs/123/events -> /runs/{id}/events

	// This is a simple implementation - you might want more sophisticated path templating
	if len(path) > 6 && path[:6] == "/runs/" {
		if len(path) > 42 && path[42:43] == "/" { // UUID length + slash
			return "/runs/{id}" + path[42:]
		} else if len(path) > 6 {
			return "/runs/{id}"
		}
	}

	return path
}

// getStatusClass returns the HTTP status class (1xx, 2xx, 3xx, 4xx, 5xx)
func (m *AgentMetrics) getStatusClass(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		return "1xx"
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500 && statusCode < 600:
		return "5xx"
	default:
		return "unknown"
	}
}

// Advanced Metrics

// RunStateTransition records state transitions in the run lifecycle
func (m *AgentMetrics) RunStateTransition(ctx context.Context, fromState, toState string) {
	m.provider.IncrementCounter(ctx, "agent_run_state_transitions_total", 1, map[string]string{
		"from_state": fromState,
		"to_state":   toState,
	})
}

// MCPConnectionStatus records MCP server connection status
func (m *AgentMetrics) MCPConnectionStatus(ctx context.Context, serverName, transport, status string) {
	value := float64(0)
	if status == "connected" {
		value = 1
	}

	m.provider.SetGauge(ctx, "agent_mcp_connections", value, map[string]string{
		"server_name": serverName,
		"transport":   transport,
		"status":      status,
	})
}

// EventBusEvent records events published to the event bus
func (m *AgentMetrics) EventBusEvent(ctx context.Context, eventType, runID string) {
	m.provider.IncrementCounter(ctx, "agent_events_published_total", 1, map[string]string{
		"event_type": eventType,
		"run_id":     runID,
	})
}

// StorageOperation records storage operations (reads, writes, etc.)
func (m *AgentMetrics) StorageOperation(ctx context.Context, operation, status string, duration time.Duration) {
	labels := map[string]string{
		"operation": operation,
		"status":    status,
	}

	m.provider.IncrementCounter(ctx, "agent_storage_operations_total", 1, labels)
	m.provider.RecordHistogram(ctx, "agent_storage_operation_duration_seconds", duration.Seconds(), labels)
}

// Close closes the underlying metrics provider
func (m *AgentMetrics) Close() error {
	return m.provider.Close()
}
