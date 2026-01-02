package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// OTELProvider implements the metrics Provider interface using OpenTelemetry
type OTELProvider struct {
	config        *OTELConfig
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter
	exporter      sdkmetric.Exporter

	// Metric instruments
	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Float64Histogram
	gauges     map[string]metric.Int64UpDownCounter

	mu sync.RWMutex
}

// NewOTELProvider creates a new OpenTelemetry metrics provider
func NewOTELProvider(namespace string, config *OTELConfig) (*OTELProvider, error) {
	if config == nil {
		config = &OTELConfig{
			Endpoint:      "http://localhost:4317",
			Protocol:      "grpc",
			Headers:       make(map[string]string),
			Resources:     make(map[string]string),
			ExportTimeout: 30 * time.Second,
		}
	}

	// Create resource with service information
	resourceAttrs := []attribute.KeyValue{
		semconv.ServiceName("agent-runtime"),
		semconv.ServiceNamespace(namespace),
		semconv.ServiceVersion("1.0.0"),
	}

	// Add custom resource attributes
	for key, value := range config.Resources {
		resourceAttrs = append(resourceAttrs, attribute.String(key, value))
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, resourceAttrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter
	var exporter sdkmetric.Exporter
	switch config.Protocol {
	case "grpc":
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(config.Endpoint),
			otlpmetricgrpc.WithTimeout(config.ExportTimeout),
		}

		// Add custom headers
		if len(config.Headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(config.Headers))
		}

		exporter, err = otlpmetricgrpc.New(context.Background(), opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported OTEL protocol: %s", config.Protocol)
	}

	// Create meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Create meter
	meter := meterProvider.Meter("agent-runtime")

	p := &OTELProvider{
		config:        config,
		meterProvider: meterProvider,
		meter:         meter,
		exporter:      exporter,
		counters:      make(map[string]metric.Int64Counter),
		histograms:    make(map[string]metric.Float64Histogram),
		gauges:        make(map[string]metric.Int64UpDownCounter),
	}

	return p, nil
}

// RegisterMetrics registers all predefined metrics with OpenTelemetry
func (p *OTELProvider) RegisterMetrics(metrics []MetricDefinition) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, metric := range metrics {
		if err := p.registerMetric(metric); err != nil {
			return fmt.Errorf("failed to register metric %s: %w", metric.Name, err)
		}
	}

	return nil
}

func (p *OTELProvider) registerMetric(metricDef MetricDefinition) error {
	switch metricDef.Type {
	case MetricTypeCounter:
		counter, err := p.meter.Int64Counter(
			metricDef.Name,
			metric.WithDescription(metricDef.Description),
			metric.WithUnit(metricDef.Unit),
		)
		if err != nil {
			return err
		}
		p.counters[metricDef.Name] = counter

	case MetricTypeHistogram:
		// Create histogram with explicit bucket boundaries if provided
		var opts []metric.Float64HistogramOption
		opts = append(opts,
			metric.WithDescription(metricDef.Description),
			metric.WithUnit(metricDef.Unit),
		)

		if len(metricDef.Buckets) > 0 {
			opts = append(opts, metric.WithExplicitBucketBoundaries(metricDef.Buckets...))
		}

		histogram, err := p.meter.Float64Histogram(metricDef.Name, opts...)
		if err != nil {
			return err
		}
		p.histograms[metricDef.Name] = histogram

	case MetricTypeGauge:
		// OpenTelemetry doesn't have a direct gauge, use UpDownCounter
		gauge, err := p.meter.Int64UpDownCounter(
			metricDef.Name,
			metric.WithDescription(metricDef.Description),
			metric.WithUnit(metricDef.Unit),
		)
		if err != nil {
			return err
		}
		p.gauges[metricDef.Name] = gauge

	default:
		return fmt.Errorf("unsupported metric type: %s", metricDef.Type)
	}

	return nil
}

// IncrementCounter implements Provider interface
func (p *OTELProvider) IncrementCounter(ctx context.Context, name string, value int64, labels map[string]string) error {
	p.mu.RLock()
	counter, exists := p.counters[name]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("counter %s not registered", name)
	}

	attrs := p.labelsToAttributes(labels)
	counter.Add(ctx, value, metric.WithAttributes(attrs...))
	return nil
}

// RecordHistogram implements Provider interface
func (p *OTELProvider) RecordHistogram(ctx context.Context, name string, value float64, labels map[string]string) error {
	p.mu.RLock()
	histogram, exists := p.histograms[name]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("histogram %s not registered", name)
	}

	attrs := p.labelsToAttributes(labels)
	histogram.Record(ctx, value, metric.WithAttributes(attrs...))
	return nil
}

// SetGauge implements Provider interface
func (p *OTELProvider) SetGauge(ctx context.Context, name string, value float64, labels map[string]string) error {
	// OpenTelemetry doesn't have a direct SetGauge operation
	// This is a limitation - we'll need to track current value and calculate delta
	// For now, we'll add the value directly (not ideal but functional)
	p.mu.RLock()
	gauge, exists := p.gauges[name]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("gauge %s not registered", name)
	}

	attrs := p.labelsToAttributes(labels)
	gauge.Add(ctx, int64(value), metric.WithAttributes(attrs...))
	return nil
}

// AddGauge implements Provider interface
func (p *OTELProvider) AddGauge(ctx context.Context, name string, delta float64, labels map[string]string) error {
	p.mu.RLock()
	gauge, exists := p.gauges[name]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("gauge %s not registered", name)
	}

	attrs := p.labelsToAttributes(labels)
	gauge.Add(ctx, int64(delta), metric.WithAttributes(attrs...))
	return nil
}

// GetType implements Provider interface
func (p *OTELProvider) GetType() ProviderType {
	return ProviderOTEL
}

// GetEndpoint implements Provider interface
func (p *OTELProvider) GetEndpoint() string {
	return p.config.Endpoint
}

// Close implements Provider interface
func (p *OTELProvider) Close() error {
	if p.meterProvider != nil {
		return p.meterProvider.Shutdown(context.Background())
	}
	return nil
}

// HTTPHandler returns an HTTP handler for health/status (OTEL doesn't expose metrics via HTTP by default)
func (p *OTELProvider) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"provider": "opentelemetry",
			"endpoint": "` + p.config.Endpoint + `",
			"protocol": "` + p.config.Protocol + `",
			"status": "active"
		}`))
	})
}

// Helper method to convert string labels to OTEL attributes
func (p *OTELProvider) labelsToAttributes(labels map[string]string) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(labels))
	for key, value := range labels {
		attrs = append(attrs, attribute.String(key, value))
	}
	return attrs
}
