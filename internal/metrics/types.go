package metrics

import (
	"context"
	"time"
)

// Provider defines the interface that both Prometheus and OTEL implementations must satisfy
type Provider interface {
	// Counter operations
	IncrementCounter(ctx context.Context, name string, value int64, labels map[string]string) error

	// Histogram operations
	RecordHistogram(ctx context.Context, name string, value float64, labels map[string]string) error

	// Gauge operations
	SetGauge(ctx context.Context, name string, value float64, labels map[string]string) error
	AddGauge(ctx context.Context, name string, delta float64, labels map[string]string) error

	// Lifecycle
	Close() error

	// Provider-specific operations
	GetType() ProviderType
	GetEndpoint() string // For HTTP exposition (Prometheus /metrics, OTEL collector endpoint)
}

// ProviderType represents the metrics backend type
type ProviderType string

const (
	ProviderPrometheus ProviderType = "prometheus"
	ProviderOTEL       ProviderType = "otel"
)

// Config represents metrics configuration
type Config struct {
	Enabled    bool              `yaml:"enabled"`
	Provider   ProviderType      `yaml:"provider"`
	Namespace  string            `yaml:"namespace"`
	Endpoint   string            `yaml:"endpoint"`
	Prometheus *PrometheusConfig `yaml:"prometheus,omitempty"`
	OTEL       *OTELConfig       `yaml:"otel,omitempty"`
}

// PrometheusConfig holds Prometheus-specific configuration
type PrometheusConfig struct {
	Path     string            `yaml:"path"`     // Default: "/metrics"
	Registry string            `yaml:"registry"` // Default: "default"
	Labels   map[string]string `yaml:"labels"`   // Static labels
}

// OTELConfig holds OpenTelemetry-specific configuration
type OTELConfig struct {
	Endpoint      string            `yaml:"endpoint"`       // OTEL collector endpoint
	Protocol      string            `yaml:"protocol"`       // "grpc" or "http"
	Headers       map[string]string `yaml:"headers"`        // Additional headers
	Resources     map[string]string `yaml:"resources"`      // Resource attributes
	ExportTimeout time.Duration     `yaml:"export_timeout"` // Export timeout
}

// MetricDefinition represents a metric with its metadata
type MetricDefinition struct {
	Name        string
	Description string
	Unit        string
	Type        MetricType
	Buckets     []float64 // For histograms
}

// MetricType represents the type of metric
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeGauge     MetricType = "gauge"
)

// Common metric definitions for the agent runtime
var (
	// Run metrics
	RunsCreatedMetric = MetricDefinition{
		Name:        "agent_runs_created_total",
		Description: "Total number of runs created",
		Unit:        "1",
		Type:        MetricTypeCounter,
	}

	RunsCompletedMetric = MetricDefinition{
		Name:        "agent_runs_completed_total",
		Description: "Total number of completed runs",
		Unit:        "1",
		Type:        MetricTypeCounter,
	}

	RunDurationMetric = MetricDefinition{
		Name:        "agent_run_duration_seconds",
		Description: "Duration of run execution",
		Unit:        "s",
		Type:        MetricTypeHistogram,
		Buckets:     []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300, 600},
	}

	ActiveRunsMetric = MetricDefinition{
		Name:        "agent_runs_active",
		Description: "Number of currently active runs",
		Unit:        "1",
		Type:        MetricTypeGauge,
	}

	// Tool metrics
	ToolInvocationsMetric = MetricDefinition{
		Name:        "agent_tool_invocations_total",
		Description: "Total tool invocations",
		Unit:        "1",
		Type:        MetricTypeCounter,
	}

	ToolDurationMetric = MetricDefinition{
		Name:        "agent_tool_duration_seconds",
		Description: "Duration of tool execution",
		Unit:        "s",
		Type:        MetricTypeHistogram,
		Buckets:     []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30},
	}

	// LLM metrics
	LLMRequestsMetric = MetricDefinition{
		Name:        "agent_llm_requests_total",
		Description: "Total LLM requests",
		Unit:        "1",
		Type:        MetricTypeCounter,
	}

	LLMDurationMetric = MetricDefinition{
		Name:        "agent_llm_request_duration_seconds",
		Description: "Duration of LLM requests",
		Unit:        "s",
		Type:        MetricTypeHistogram,
		Buckets:     []float64{0.1, 0.5, 1, 2, 5, 10, 20, 60},
	}

	LLMTokensUsedMetric = MetricDefinition{
		Name:        "agent_llm_tokens_used_total",
		Description: "Total tokens used",
		Unit:        "1",
		Type:        MetricTypeCounter,
	}

	// HTTP metrics
	HTTPRequestsMetric = MetricDefinition{
		Name:        "agent_http_requests_total",
		Description: "Total HTTP requests",
		Unit:        "1",
		Type:        MetricTypeCounter,
	}

	HTTPDurationMetric = MetricDefinition{
		Name:        "agent_http_request_duration_seconds",
		Description: "Duration of HTTP requests",
		Unit:        "s",
		Type:        MetricTypeHistogram,
		Buckets:     []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10},
	}
)

// AllMetrics returns all predefined metrics for registration
func AllMetrics() []MetricDefinition {
	return []MetricDefinition{
		RunsCreatedMetric,
		RunsCompletedMetric,
		RunDurationMetric,
		ActiveRunsMetric,
		ToolInvocationsMetric,
		ToolDurationMetric,
		LLMRequestsMetric,
		LLMDurationMetric,
		LLMTokensUsedMetric,
		HTTPRequestsMetric,
		HTTPDurationMetric,
	}
}
