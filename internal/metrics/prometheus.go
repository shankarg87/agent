package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusProvider implements the metrics Provider interface using Prometheus
type PrometheusProvider struct {
	namespace string
	registry  *prometheus.Registry
	config    *PrometheusConfig

	// Metric storage
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	gauges     map[string]*prometheus.GaugeVec

	mu sync.RWMutex
}

// NewPrometheusProvider creates a new Prometheus metrics provider
func NewPrometheusProvider(namespace string, config *PrometheusConfig) (*PrometheusProvider, error) {
	if config == nil {
		config = &PrometheusConfig{
			Path:     "/metrics",
			Registry: "default",
			Labels:   make(map[string]string),
		}
	}

	var registry *prometheus.Registry
	if config.Registry == "default" {
		registry = prometheus.DefaultRegisterer.(*prometheus.Registry)
	} else {
		registry = prometheus.NewRegistry()
	}

	p := &PrometheusProvider{
		namespace:  namespace,
		registry:   registry,
		config:     config,
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
	}

	return p, nil
}

// RegisterMetrics registers all predefined metrics with Prometheus
func (p *PrometheusProvider) RegisterMetrics(metrics []MetricDefinition) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, metric := range metrics {
		if err := p.registerMetric(metric); err != nil {
			return fmt.Errorf("failed to register metric %s: %w", metric.Name, err)
		}
	}

	return nil
}

func (p *PrometheusProvider) registerMetric(metric MetricDefinition) error {
	name := p.formatMetricName(metric.Name)

	switch metric.Type {
	case MetricTypeCounter:
		counter := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: p.namespace,
				Name:      name,
				Help:      metric.Description,
			},
			[]string{}, // Will be populated dynamically based on labels
		)
		p.counters[metric.Name] = counter
		return p.registry.Register(counter)

	case MetricTypeHistogram:
		buckets := metric.Buckets
		if len(buckets) == 0 {
			buckets = prometheus.DefBuckets
		}

		histogram := prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: p.namespace,
				Name:      name,
				Help:      metric.Description,
				Buckets:   buckets,
			},
			[]string{},
		)
		p.histograms[metric.Name] = histogram
		return p.registry.Register(histogram)

	case MetricTypeGauge:
		gauge := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: p.namespace,
				Name:      name,
				Help:      metric.Description,
			},
			[]string{},
		)
		p.gauges[metric.Name] = gauge
		return p.registry.Register(gauge)

	default:
		return fmt.Errorf("unsupported metric type: %s", metric.Type)
	}
}

func (p *PrometheusProvider) formatMetricName(name string) string {
	// Remove namespace if already present to avoid duplication
	if len(name) > len(p.namespace) && name[:len(p.namespace)] == p.namespace {
		return name[len(p.namespace)+1:] // +1 for underscore
	}
	return name
}

// IncrementCounter implements Provider interface
func (p *PrometheusProvider) IncrementCounter(ctx context.Context, name string, value int64, labels map[string]string) error {
	p.mu.RLock()
	counter, exists := p.counters[name]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("counter %s not registered", name)
	}

	// Get or create metric with labels
	metric, err := p.getCounterWithLabels(counter, labels)
	if err != nil {
		return err
	}

	metric.Add(float64(value))
	return nil
}

// RecordHistogram implements Provider interface
func (p *PrometheusProvider) RecordHistogram(ctx context.Context, name string, value float64, labels map[string]string) error {
	p.mu.RLock()
	histogram, exists := p.histograms[name]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("histogram %s not registered", name)
	}

	metric, err := p.getHistogramWithLabels(histogram, labels)
	if err != nil {
		return err
	}

	metric.Observe(value)
	return nil
}

// SetGauge implements Provider interface
func (p *PrometheusProvider) SetGauge(ctx context.Context, name string, value float64, labels map[string]string) error {
	p.mu.RLock()
	gauge, exists := p.gauges[name]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("gauge %s not registered", name)
	}

	metric, err := p.getGaugeWithLabels(gauge, labels)
	if err != nil {
		return err
	}

	metric.Set(value)
	return nil
}

// AddGauge implements Provider interface
func (p *PrometheusProvider) AddGauge(ctx context.Context, name string, delta float64, labels map[string]string) error {
	p.mu.RLock()
	gauge, exists := p.gauges[name]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("gauge %s not registered", name)
	}

	metric, err := p.getGaugeWithLabels(gauge, labels)
	if err != nil {
		return err
	}

	metric.Add(delta)
	return nil
}

// GetType implements Provider interface
func (p *PrometheusProvider) GetType() ProviderType {
	return ProviderPrometheus
}

// GetEndpoint implements Provider interface
func (p *PrometheusProvider) GetEndpoint() string {
	return p.config.Path
}

// Close implements Provider interface
func (p *PrometheusProvider) Close() error {
	// Prometheus doesn't require explicit cleanup
	return nil
}

// HTTPHandler returns an HTTP handler for the /metrics endpoint
func (p *PrometheusProvider) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Helper methods for getting metrics with dynamic labels

func (p *PrometheusProvider) getCounterWithLabels(counterVec *prometheus.CounterVec, labels map[string]string) (prometheus.Counter, error) {
	return counterVec.GetMetricWith(prometheus.Labels(labels))
}

func (p *PrometheusProvider) getHistogramWithLabels(histogramVec *prometheus.HistogramVec, labels map[string]string) (prometheus.Observer, error) {
	return histogramVec.GetMetricWith(prometheus.Labels(labels))
}

func (p *PrometheusProvider) getGaugeWithLabels(gaugeVec *prometheus.GaugeVec, labels map[string]string) (prometheus.Gauge, error) {
	return gaugeVec.GetMetricWith(prometheus.Labels(labels))
}
