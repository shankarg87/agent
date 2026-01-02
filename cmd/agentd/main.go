package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shankarg87/agent/internal/config"
	"github.com/shankarg87/agent/internal/events"
	"github.com/shankarg87/agent/internal/logging"
	"github.com/shankarg87/agent/internal/mcp"
	"github.com/shankarg87/agent/internal/metrics"
	"github.com/shankarg87/agent/internal/provider"
	"github.com/shankarg87/agent/internal/runtime"
	"github.com/shankarg87/agent/internal/store"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	// Setup CLI flags
	pflag.String("config", "configs/agents/default.yaml", "path to agent config file")
	pflag.String("mcp-config", "configs/mcp/servers.yaml", "path to MCP servers config")
	pflag.String("addr", ":8080", "HTTP server address")
	pflag.Bool("watch-config", true, "enable automatic config reloading")
	pflag.Bool("verbose", false, "enable verbose logging")
	pflag.Parse()

	// Setup Viper to handle CLI flags and environment variables
	viper.BindPFlags(pflag.CommandLine)
	viper.SetEnvPrefix("AGENT") // Environment variables like AGENT_CONFIG, AGENT_ADDR
	viper.AutomaticEnv()

	// Get configuration values
	configPath := viper.GetString("config")
	mcpConfigPath := viper.GetString("mcp-config")
	addr := viper.GetString("addr")
	watchConfig := viper.GetBool("watch-config")
	verbose := viper.GetBool("verbose")

	// Initialize logger
	var logger *logging.SimpleLogger
	if verbose {
		logger = logging.VerboseLogger("main")
		logger.Info("Verbose logging enabled")
	} else {
		logger = logging.DefaultLogger("main")
	}

	logger.LogConfigLoad("CLI flags", map[string]interface{}{
		"config":       configPath,
		"mcp-config":   mcpConfigPath,
		"addr":         addr,
		"watch-config": watchConfig,
		"verbose":      verbose,
	})

	// Initialize configuration manager
	logger.Verbose("Initializing configuration manager",
		"config_path", configPath,
		"mcp_config_path", mcpConfigPath,
	)

	configManager, err := config.NewConfigManager(configPath, mcpConfigPath)
	if err != nil {
		logger.Error("Failed to initialize configuration manager", "error", err)
		log.Fatalf("Failed to initialize configuration manager: %v", err)
	}
	defer func() {
		logger.Verbose("Closing configuration manager")
		configManager.Close()
	}()

	if watchConfig {
		logger.Info("Configuration file watching enabled",
			"config_path", configPath,
			"mcp_config_path", mcpConfigPath,
		)
	} else {
		logger.Info("Configuration file watching disabled")
	}

	// Get initial configuration
	logger.Verbose("Loading initial configuration")
	cfg := configManager.GetAgentConfig()
	mcpCfg := configManager.GetMCPConfig()

	logger.LogConfigLoad("agent configuration", cfg)
	logger.LogConfigLoad("MCP configuration", mcpCfg)

	// Initialize components
	ctx := context.Background()

	// Storage
	logger.Verbose("Initializing in-memory store")
	storage := store.NewInMemoryStore()

	// Event bus
	logger.Verbose("Initializing event bus")
	eventBus := events.NewEventBus()

	// LLM provider
	logger.Verbose("Initializing LLM provider",
		"provider", cfg.PrimaryModel.Provider,
		"model", cfg.PrimaryModel.Model,
	)
	llmProvider, err := provider.NewProvider(cfg.PrimaryModel)
	if err != nil {
		logger.Error("Failed to initialize LLM provider",
			"provider", cfg.PrimaryModel.Provider,
			"error", err,
		)
		log.Fatalf("Failed to initialize LLM provider: %v", err)
	}
	logger.Info("LLM provider initialized successfully",
		"provider", cfg.PrimaryModel.Provider,
		"model", cfg.PrimaryModel.Model,
	)

	// MCP registry
	logger.Verbose("Initializing MCP registry")
	mcpRegistry := mcp.NewRegistry()

	logger.Verbose("Loading MCP servers", "server_count", len(mcpCfg.Servers))
	if err := mcpRegistry.LoadServers(ctx, mcpCfg); err != nil {
		logger.Error("Failed to load MCP servers", "error", err)
		log.Fatalf("Failed to load MCP servers: %v", err)
	}
	defer func() {
		logger.Verbose("Closing MCP registry")
		mcpRegistry.Close()
	}()
	logger.Info("MCP servers loaded successfully", "server_count", len(mcpCfg.Servers))

	// Metrics
	var agentMetrics *metrics.AgentMetrics
	if cfg.MetricsEnabled {
		logger.Verbose("Metrics enabled, initializing metrics provider",
			"provider", cfg.MetricsConfig.Provider,
			"namespace", cfg.MetricsConfig.Namespace,
		)

		// Convert agent config to metrics config
		metricsConfig := &metrics.Config{
			Enabled:   cfg.MetricsEnabled,
			Provider:  metrics.ProviderType(cfg.MetricsConfig.Provider),
			Namespace: cfg.MetricsConfig.Namespace,
			Endpoint:  cfg.MetricsConfig.Endpoint,
		}

		// Convert sub-configs
		if cfg.MetricsConfig.Prometheus != nil {
			logger.Verbose("Configuring Prometheus metrics",
				"path", cfg.MetricsConfig.Prometheus.Path,
				"registry", cfg.MetricsConfig.Prometheus.Registry,
			)
			metricsConfig.Prometheus = &metrics.PrometheusConfig{
				Path:     cfg.MetricsConfig.Prometheus.Path,
				Registry: cfg.MetricsConfig.Prometheus.Registry,
				Labels:   cfg.MetricsConfig.Prometheus.Labels,
			}
		}

		if cfg.MetricsConfig.OTEL != nil {
			logger.Verbose("Configuring OTEL metrics",
				"endpoint", cfg.MetricsConfig.OTEL.Endpoint,
				"protocol", cfg.MetricsConfig.OTEL.Protocol,
			)
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
			logger.Error("Invalid metrics configuration", "error", err)
			log.Fatalf("Invalid metrics configuration: %v", err)
		}

		factory := metrics.NewFactory()
		provider, err := factory.CreateProvider("agent", metricsConfig)
		if err != nil {
			logger.Error("Failed to create metrics provider", "error", err)
			log.Fatalf("Failed to create metrics provider: %v", err)
		}
		defer func() {
			logger.Verbose("Closing metrics provider")
			provider.Close()
		}()

		agentMetrics = metrics.NewAgentMetrics(provider)
		logger.Info("Metrics enabled", "provider_type", provider.GetType())
	} else {
		logger.Info("Metrics disabled")
	}

	// Runtime
	logger.Verbose("Initializing agent runtime")
	rt := runtime.NewRuntime(configManager, storage, eventBus, llmProvider, mcpRegistry, agentMetrics)
	logger.Info("Agent runtime initialized successfully")

	// HTTP server
	logger.Verbose("Setting up HTTP server", "addr", addr)
	mux := http.NewServeMux()

	// Configuration endpoint - shows current config and reload status
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		logger.LogRequest(r.Method, r.URL.Path, r.RemoteAddr, nil)
		start := time.Now()

		w.Header().Set("Content-Type", "application/json")
		currentCfg := configManager.GetAgentConfig()
		lastReload := configManager.GetLastReload()

		response := map[string]interface{}{
			"profile_name":    currentCfg.ProfileName,
			"profile_version": currentCfg.ProfileVersion,
			"last_reload":     lastReload.Format(time.RFC3339),
			"system_prompt":   currentCfg.SystemPrompt,
			"temperature":     currentCfg.Temperature,
			"max_tool_calls":  currentCfg.MaxToolCalls,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode config response", "error", err)
			logger.LogResponse(r.Method, r.URL.Path, http.StatusInternalServerError, time.Since(start))
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

		logger.LogResponse(r.Method, r.URL.Path, http.StatusOK, time.Since(start))
	})

	// Native /runs API
	logger.Verbose("Registering native runs API")
	runtime.RegisterRunsAPI(mux, rt)

	// OpenAI-compatible /v1 API
	logger.Verbose("Registering OpenAI-compatible v1 API")
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
			logger.Info("Metrics endpoint registered", "endpoint", endpoint)
		}
	}

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		logger.Info("Shutdown signal received, starting graceful shutdown")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Server shutdown error", "error", err)
		} else {
			logger.Info("Server shutdown completed successfully")
		}
	}()

	logger.Info("Starting agent server",
		"addr", addr,
		"profile_name", cfg.ProfileName,
		"profile_version", cfg.ProfileVersion,
	)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("Server error", "error", err)
		log.Fatalf("Server error: %v", err)
	}

	logger.Info("Agent server stopped successfully")
}
