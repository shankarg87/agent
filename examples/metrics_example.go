package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shankgan/agent/internal/metrics"
)

func main() {
	// Example 1: Prometheus Setup
	fmt.Println("=== Prometheus Example ===")
	prometheusExample()

	fmt.Println("\n=== OpenTelemetry Example ===")
	otelExample()

	fmt.Println("\n=== Agent Metrics Usage ===")
	agentMetricsExample()
}

func prometheusExample() {
	config := &metrics.Config{
		Enabled:   true,
		Provider:  metrics.ProviderPrometheus,
		Namespace: "agent",
		Prometheus: &metrics.PrometheusConfig{
			Path:     "/metrics",
			Registry: "default",
			Labels: map[string]string{
				"service":     "agent-runtime",
				"environment": "development",
			},
		},
	}

	factory := metrics.NewFactory()
	provider, err := factory.CreateProvider("agent", config)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	// Record some metrics
	ctx := context.Background()
	provider.IncrementCounter(ctx, "agent_runs_created_total", 1, map[string]string{
		"tenant_id": "test-tenant",
		"mode":      "interactive",
	})

	provider.RecordHistogram(ctx, "agent_run_duration_seconds", 2.5, map[string]string{
		"status": "completed",
		"mode":   "interactive",
	})

	fmt.Printf("Prometheus provider created: %s\n", provider.GetEndpoint())
}

func otelExample() {
	config := &metrics.Config{
		Enabled:  true,
		Provider: metrics.ProviderOTEL,
		OTEL: &metrics.OTELConfig{
			Endpoint: "http://localhost:4317",
			Protocol: "grpc",
			Resources: map[string]string{
				"service.name":    "agent-runtime",
				"service.version": "1.0.0",
			},
			ExportTimeout: 30 * time.Second,
		},
	}

	factory := metrics.NewFactory()
	provider, err := factory.CreateProvider("agent", config)
	if err != nil {
		log.Printf("Note: OTEL provider creation failed (expected if no collector running): %v", err)
		return
	}
	defer provider.Close()

	// Record some metrics
	ctx := context.Background()
	provider.IncrementCounter(ctx, "agent_runs_created_total", 1, map[string]string{
		"tenant_id": "test-tenant",
		"mode":      "autonomous",
	})

	fmt.Printf("OTEL provider created: %s\n", provider.GetEndpoint())
}

func agentMetricsExample() {
	// Create a simple Prometheus provider for the example
	config := &metrics.Config{
		Enabled:   true,
		Provider:  metrics.ProviderPrometheus,
		Namespace: "agent",
	}

	factory := metrics.NewFactory()
	provider, err := factory.CreateProvider("agent", config)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	// Create agent metrics wrapper
	agentMetrics := metrics.NewAgentMetrics(provider)

	ctx := context.Background()

	// Simulate run lifecycle
	agentMetrics.RunCreated(ctx, "tenant-123", "interactive")
	agentMetrics.IncrementActiveRuns(ctx, "tenant-123")

	// Simulate tool usage
	agentMetrics.ToolInvocation(ctx, "file_read", "filesystem-server", "success", 150*time.Millisecond)
	agentMetrics.ToolInvocation(ctx, "execute_command", "shell-server", "success", 2*time.Second)

	// Simulate LLM request
	agentMetrics.LLMRequest(ctx, "anthropic", "claude-sonnet-4-5-20250929", "success", 1500*time.Millisecond)
	agentMetrics.LLMTokensUsed(ctx, "anthropic", "claude-sonnet-4-5-20250929", "prompt", 1234)
	agentMetrics.LLMTokensUsed(ctx, "anthropic", "claude-sonnet-4-5-20250929", "completion", 567)

	// Simulate HTTP request
	agentMetrics.HTTPRequest(ctx, "POST", "/runs", 201, 45*time.Millisecond)

	// Complete the run
	agentMetrics.RunCompleted(ctx, "tenant-123", "interactive", "completed", 5*time.Second)
	agentMetrics.DecrementActiveRuns(ctx, "tenant-123")

	fmt.Println("Agent metrics recorded successfully")
	fmt.Printf("Metrics endpoint: %s\n", provider.GetEndpoint())

	// Show how to get HTTP handler for serving metrics
	if handlerProvider, ok := provider.(metrics.HTTPHandlerProvider); ok {
		handler := handlerProvider.HTTPHandler()
		fmt.Printf("HTTP handler available: %T\n", handler)
	}
}
