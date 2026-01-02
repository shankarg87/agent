package metrics

import (
	"context"
	"fmt"
	"net/http"
)

// Factory creates metrics providers based on configuration
type Factory struct{}

// NewFactory creates a new metrics factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateProvider creates a metrics provider based on the configuration
func (f *Factory) CreateProvider(namespace string, config *Config) (Provider, error) {
	if !config.Enabled {
		return NewNoOpProvider(), nil
	}

	switch config.Provider {
	case ProviderPrometheus:
		prometheusConfig := config.Prometheus
		if prometheusConfig == nil {
			prometheusConfig = &PrometheusConfig{
				Path:     "/metrics",
				Registry: "default",
				Labels:   make(map[string]string),
			}
		}

		provider, err := NewPrometheusProvider(namespace, prometheusConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Prometheus provider: %w", err)
		}

		// Register predefined metrics
		if err := provider.RegisterMetrics(AllMetrics()); err != nil {
			return nil, fmt.Errorf("failed to register metrics: %w", err)
		}

		return provider, nil

	case ProviderOTEL:
		otelConfig := config.OTEL
		if otelConfig == nil {
			otelConfig = &OTELConfig{
				Endpoint:  "http://localhost:4317",
				Protocol:  "grpc",
				Headers:   make(map[string]string),
				Resources: make(map[string]string),
			}
		}

		provider, err := NewOTELProvider(namespace, otelConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTEL provider: %w", err)
		}

		// Register predefined metrics
		if err := provider.RegisterMetrics(AllMetrics()); err != nil {
			return nil, fmt.Errorf("failed to register metrics: %w", err)
		}

		return provider, nil

	default:
		return nil, fmt.Errorf("unsupported metrics provider: %s", config.Provider)
	}
}

// NoOpProvider is a metrics provider that does nothing (for when metrics are disabled)
type NoOpProvider struct{}

// NewNoOpProvider creates a new no-op metrics provider
func NewNoOpProvider() *NoOpProvider {
	return &NoOpProvider{}
}

// IncrementCounter implements Provider interface (no-op)
func (n *NoOpProvider) IncrementCounter(ctx context.Context, name string, value int64, labels map[string]string) error {
	return nil
}

// RecordHistogram implements Provider interface (no-op)
func (n *NoOpProvider) RecordHistogram(ctx context.Context, name string, value float64, labels map[string]string) error {
	return nil
}

// SetGauge implements Provider interface (no-op)
func (n *NoOpProvider) SetGauge(ctx context.Context, name string, value float64, labels map[string]string) error {
	return nil
}

// AddGauge implements Provider interface (no-op)
func (n *NoOpProvider) AddGauge(ctx context.Context, name string, delta float64, labels map[string]string) error {
	return nil
}

// GetType implements Provider interface
func (n *NoOpProvider) GetType() ProviderType {
	return "noop"
}

// GetEndpoint implements Provider interface
func (n *NoOpProvider) GetEndpoint() string {
	return ""
}

// Close implements Provider interface (no-op)
func (n *NoOpProvider) Close() error {
	return nil
}

// HTTPHandlerProvider defines providers that can expose HTTP handlers
type HTTPHandlerProvider interface {
	Provider
	HTTPHandler() http.Handler
}

// GetHTTPHandler returns an HTTP handler if the provider supports it
func GetHTTPHandler(provider Provider) (http.Handler, bool) {
	if handlerProvider, ok := provider.(HTTPHandlerProvider); ok {
		return handlerProvider.HTTPHandler(), true
	}
	return nil, false
}

// ValidateConfig validates the metrics configuration
func ValidateConfig(config *Config) error {
	if !config.Enabled {
		return nil
	}

	if config.Provider == "" {
		return fmt.Errorf("metrics provider must be specified when enabled")
	}

	if config.Provider != ProviderPrometheus && config.Provider != ProviderOTEL {
		return fmt.Errorf("unsupported metrics provider: %s", config.Provider)
	}

	if config.Namespace == "" {
		config.Namespace = "agent" // Set default namespace
	}

	switch config.Provider {
	case ProviderPrometheus:
		if config.Prometheus != nil {
			if config.Prometheus.Path == "" {
				config.Prometheus.Path = "/metrics"
			}
			if config.Prometheus.Registry == "" {
				config.Prometheus.Registry = "default"
			}
		}

	case ProviderOTEL:
		if config.OTEL != nil {
			if config.OTEL.Endpoint == "" {
				config.OTEL.Endpoint = "http://localhost:4317"
			}
			if config.OTEL.Protocol == "" {
				config.OTEL.Protocol = "grpc"
			}
		}
	}

	return nil
}
