package main

import (
	"context"
	"fmt"
	"log"

	"github.com/shankarg87/agent/internal/config"
	"github.com/shankarg87/agent/internal/events"
	"github.com/shankarg87/agent/internal/mcp"
	"github.com/shankarg87/agent/internal/provider"
	"github.com/shankarg87/agent/internal/runtime"
	"github.com/shankarg87/agent/internal/store"
)

func main() {
	fmt.Println("Testing pause/resume functionality...")

	// This is a simple test to demonstrate the pause/resume API
	// In a real application, you would use this via HTTP requests

	// Mock components (in real usage these would be properly initialized)
	cfg := &config.AgentConfig{
		MaxToolCalls:      10,
		MaxRunTimeSeconds: 300,
	}

	st := store.NewInMemoryStore() // You would use your actual store implementation
	eb := events.NewEventBus()

	// Mock provider - replace with actual provider
	var prov provider.Provider
	mcpReg := &mcp.Registry{}

	rt := runtime.NewRuntime(cfg, st, eb, prov, mcpReg)

	ctx := context.Background()

	// Example usage:
	fmt.Println("1. Create a run")
	fmt.Println("2. Pause the run: POST /runs/{id}/pause")
	fmt.Println("3. Resume the run: POST /runs/{id}/resume")
	fmt.Println("4. Check run status: GET /runs/{id}")

	// Example API calls (these would be HTTP requests in practice):
	runID := "example-run-id"

	// Pause run
	fmt.Printf("Pausing run %s...\n", runID)
	if err := rt.PauseRun(ctx, runID); err != nil {
		log.Printf("Error pausing run: %v", err)
	}

	// Resume run
	fmt.Printf("Resuming run %s...\n", runID)
	if err := rt.ResumeRun(ctx, runID); err != nil {
		log.Printf("Error resuming run: %v", err)
	}

	fmt.Println("Pause/resume functionality is now available!")
}
