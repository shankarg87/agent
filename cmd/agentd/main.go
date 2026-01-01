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

	// Runtime
	rt := runtime.NewRuntime(cfg, storage, eventBus, llmProvider, mcpRegistry)

	// HTTP server
	mux := http.NewServeMux()

	// Native /runs API
	runtime.RegisterRunsAPI(mux, rt)

	// OpenAI-compatible /v1 API
	runtime.RegisterV1API(mux, rt)

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
