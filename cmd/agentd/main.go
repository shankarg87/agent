package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shankgan/agent/internal/config"
	"github.com/shankgan/agent/internal/events"
	"github.com/shankgan/agent/internal/mcp"
	"github.com/shankgan/agent/internal/metrics"
	"github.com/shankgan/agent/internal/provider"
	"github.com/shankgan/agent/internal/runtime"
	"github.com/shankgan/agent/internal/store"
)

func main() {
	configPath := flag.String("config", "configs/agents/default.yaml", "path to agent config file")
	mcpConfigPath := flag.String("mcp-config", "configs/mcp/servers.yaml", "path to MCP servers config")
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadAgentConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load agent config: %v", err)
	}

	mcpCfg, err := config.LoadMCPConfig(*mcpConfigPath)
	if err != nil {
		log.Fatalf("Failed to load MCP config: %v", err)
	}

	// Initialize components
	ctx := context.Background()

	// Storage
	storage := store.NewInMemoryStore()

	// Event bus
	eventBus := events.NewEventBus()

	// LLM provider
	llmProvider, err := provider.NewProvider(cfg.PrimaryModel)
	if err != nil {
		log.Fatalf("Failed to initialize LLM provider: %v", err)
	}

	// MCP registry
	mcpRegistry := mcp.NewRegistry()
	if err := mcpRegistry.LoadServers(ctx, mcpCfg); err != nil {
		log.Fatalf("Failed to load MCP servers: %v", err)
	}
	defer mcpRegistry.Close()

	// Metrics
	var agentMetrics *metrics.AgentMetrics
	if cfg.MetricsEnabled {
		// Convert agent config to metrics config
		metricsConfig := &metrics.Config{
			Enabled:   cfg.MetricsEnabled,
			Provider:  metrics.ProviderType(cfg.MetricsConfig.Provider),
			Namespace: cfg.MetricsConfig.Namespace,
			Endpoint:  cfg.MetricsConfig.Endpoint,
		}

		// Convert sub-configs
		if cfg.MetricsConfig.Prometheus != nil {
			metricsConfig.Prometheus = &metrics.PrometheusConfig{
				Path:     cfg.MetricsConfig.Prometheus.Path,
				Registry: cfg.MetricsConfig.Prometheus.Registry,
				Labels:   cfg.MetricsConfig.Prometheus.Labels,
			}
		}

		if cfg.MetricsConfig.OTEL != nil {
			metricsConfig.OTEL = &metrics.OTELConfig{
				Endpoint:      cfg.MetricsConfig.OTEL.Endpoint,
				Protocol:      cfg.MetricsConfig.OTEL.Protocol,
				Headers:       cfg.MetricsConfig.OTEL.Headers,
				Resources:     cfg.MetricsConfig.OTEL.Resources,
				ExportTimeout: cfg.MetricsConfig.OTEL.ExportTimeout,
			}
		}

		// Validate and create metrics provider
		if err := metrics.ValidateConfig(metricsConfig); err != nil {
			log.Fatalf("Invalid metrics configuration: %v", err)
		}

		factory := metrics.NewFactory()
		provider, err := factory.CreateProvider("agent", metricsConfig)
		if err != nil {
			log.Fatalf("Failed to create metrics provider: %v", err)
		}
		defer provider.Close()

		agentMetrics = metrics.NewAgentMetrics(provider)
		log.Printf("Metrics enabled with provider: %s", provider.GetType())
	}

	// Runtime
	rt := runtime.NewRuntime(cfg, storage, eventBus, llmProvider, mcpRegistry, agentMetrics)

	// HTTP server
	mux := http.NewServeMux()

	// Native /runs API
	runtime.RegisterRunsAPI(mux, rt)

	// OpenAI-compatible /v1 API
	runtime.RegisterV1API(mux, rt)

	// Metrics endpoint (if metrics are enabled)
	if agentMetrics != nil {
		// Try to get HTTP handler from metrics provider
		if handler, ok := metrics.GetHTTPHandler(agentMetrics.Provider()); ok {
			endpoint := "/metrics" // default endpoint
			if cfg.MetricsConfig.Endpoint != "" {
				endpoint = cfg.MetricsConfig.Endpoint
			}
			mux.Handle(endpoint, handler)
			log.Printf("Metrics endpoint available at: %s", endpoint)
		}
	}

	server := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("Agent server starting on %s", *addr)
	log.Printf("Agent profile: %s (v%s)", cfg.ProfileName, cfg.ProfileVersion)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}
