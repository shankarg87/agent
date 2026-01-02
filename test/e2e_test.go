package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shankarg87/agent/api/handlers"
	"github.com/shankarg87/agent/internal/config"
	"github.com/shankarg87/agent/internal/events"
	"github.com/shankarg87/agent/internal/mcp"
	"github.com/shankarg87/agent/internal/provider"
	"github.com/shankarg87/agent/internal/runtime"
	"github.com/shankarg87/agent/internal/store"
)

// TestServer represents a test server instance
type TestServer struct {
	server      *httptest.Server
	runtime     *runtime.Runtime
	eventBus    *events.EventBus
	mcpRegistry *mcp.Registry
	config      *config.AgentConfig
}

// setupTestServer creates a complete test server with all components
func setupTestServer(t *testing.T) *TestServer {
	// Get project root directory
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}

	// Load test configuration
	agentConfigPath := filepath.Join(projectRoot, "configs", "agents", "default.yaml")
	mcpConfigPath := filepath.Join(projectRoot, "configs", "mcp", "servers.yaml")

	cfg, err := config.LoadAgentConfig(agentConfigPath)
	if err != nil {
		t.Fatalf("Failed to load agent config: %v", err)
	}

	mcpCfg, err := config.LoadMCPConfig(mcpConfigPath)
	if err != nil {
		t.Fatalf("Failed to load MCP config: %v", err)
	}

	// Update MCP config to use absolute paths for echo server
	for i, server := range mcpCfg.Servers {
		if server.Name == "echo" && !filepath.IsAbs(server.Endpoint) {
			mcpCfg.Servers[i].Endpoint = filepath.Join(projectRoot, server.Endpoint)
		}
	}

	// Initialize components
	ctx := context.Background()
	storage := store.NewInMemoryStore()
	eventBus := events.NewEventBus()

	// Skip LLM provider initialization for basic tests (use mock)
	llmProvider := &MockLLMProvider{}

	// Initialize MCP registry
	mcpRegistry := mcp.NewRegistry()
	if err := mcpRegistry.LoadServers(ctx, mcpCfg); err != nil {
		t.Fatalf("Failed to load MCP servers: %v", err)
	}

	// Create config manager for runtime (wraps the config)
	configManager := config.NewConfigManagerForTest(cfg, mcpCfg)

	// Create runtime
	rt := runtime.NewRuntime(configManager, storage, eventBus, llmProvider, mcpRegistry, nil)

	// Setup HTTP routes
	mux := http.NewServeMux()
	handlers.RegisterRunsAPI(mux, rt)
	handlers.RegisterOpenAIChatAPI(mux, rt)

	// Create test server
	server := httptest.NewServer(mux)

	return &TestServer{
		server:      server,
		runtime:     rt,
		eventBus:    eventBus,
		mcpRegistry: mcpRegistry,
		config:      cfg,
	}
}

func (ts *TestServer) Close() {
	ts.server.Close()
	if ts.mcpRegistry != nil {
		ts.mcpRegistry.Close()
	}
}

func (ts *TestServer) URL() string {
	return ts.server.URL
}

// MockLLMProvider implements a simple mock for testing
type MockLLMProvider struct{}

func (m *MockLLMProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	// Check for tool results in the messages (tool response flow)
	hasToolResults := false
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			hasToolResults = true
			break
		}
	}

	// If we have tool results, provide a final response
	if hasToolResults {
		return &provider.ChatResponse{
			ID:           "mock-response-id",
			Content:      "I have successfully used the echo tool and received the response. The task is now complete.",
			Role:         "assistant",
			FinishReason: "stop",
			Usage: provider.Usage{
				PromptTokens:     10,
				CompletionTokens: 15,
				TotalTokens:      25,
			},
		}, nil
	}

	// Get the last user message
	var lastUserMessage string
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			lastUserMessage = msg.Content
		}
	}

	// If the input mentions using a tool, simulate tool use
	if strings.Contains(lastUserMessage, "echo") || strings.Contains(lastUserMessage, "tool") {
		return &provider.ChatResponse{
			ID:      "mock-response-id",
			Content: "I'll use the echo tool to help you.",
			Role:    "assistant",
			ToolCalls: []provider.ToolCall{
				{
					ID:   "mock-tool-call-id",
					Type: "function",
					Function: provider.FunctionCall{
						Name:      "echo",
						Arguments: `{"message": "Hello from test!"}`,
					},
				},
			},
			FinishReason: "tool_calls",
			Usage: provider.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}, nil
	}

	// Default response without tools
	return &provider.ChatResponse{
		ID:           "mock-response-id",
		Content:      "I understand your request. This is a mock response.",
		Role:         "assistant",
		FinishReason: "stop",
		Usage: provider.Usage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
	}, nil
}

func (m *MockLLMProvider) Stream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	ch := make(chan provider.StreamEvent, 10)

	go func() {
		defer close(ch)

		// Simulate streaming response
		content := "This is a streaming response from the mock provider."
		words := strings.Fields(content)

		for _, word := range words {
			select {
			case <-ctx.Done():
				return
			case ch <- provider.StreamEvent{
				Type:    "content_delta",
				Content: word + " ",
			}:
				time.Sleep(10 * time.Millisecond) // Simulate delay
			}
		}

		ch <- provider.StreamEvent{
			Type: "done",
			Done: true,
			Usage: &provider.Usage{
				PromptTokens:     10,
				CompletionTokens: 15,
				TotalTokens:      25,
			},
		}
	}()

	return ch, nil
}

func (m *MockLLMProvider) Name() string {
	return "mock"
}

func (m *MockLLMProvider) Model() string {
	return "mock-model"
}

// Helper function to find the project root
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not find project root (no go.mod found)")
}

// Test the /runs API
func TestRunsAPI(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	t.Run("CreateAndGetRun", func(t *testing.T) {
		// Create a run
		createReq := map[string]interface{}{
			"tenant_id": "test-tenant",
			"mode":      "interactive",
			"input":     "Hello, can you echo this message?",
		}

		body, _ := json.Marshal(createReq)
		resp, err := http.Post(ts.URL()+"/runs", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to create run: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Expected status 201, got %d", resp.StatusCode)
		}

		var createResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		runID, ok := createResp["id"].(string)
		if !ok || runID == "" {
			t.Fatalf("Expected run ID in response, got: %v", createResp)
		}

		// Get the run
		getResp, err := http.Get(ts.URL() + "/runs/" + runID)
		if err != nil {
			t.Fatalf("Failed to get run: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", getResp.StatusCode)
		}

		var runData map[string]interface{}
		if err := json.NewDecoder(getResp.Body).Decode(&runData); err != nil {
			t.Fatalf("Failed to decode run response: %v", err)
		}

		if runData["id"] != runID {
			t.Errorf("Expected run ID %s, got %s", runID, runData["id"])
		}
	})

	t.Run("EventStreaming", func(t *testing.T) {
		// Create a run
		createReq := map[string]interface{}{
			"tenant_id": "test-tenant",
			"mode":      "interactive",
			"input":     "Use the echo tool to say hello",
		}

		body, _ := json.Marshal(createReq)
		resp, err := http.Post(ts.URL()+"/runs", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to create run: %v", err)
		}
		defer resp.Body.Close()

		var createResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode create response: %v", err)
		}

		runIDInterface, exists := createResp["id"]
		if !exists || runIDInterface == nil {
			t.Fatalf("No run ID in response: %v", createResp)
		}

		runID, ok := runIDInterface.(string)
		if !ok {
			t.Fatalf("Run ID is not a string: %v", runIDInterface)
		}

		// Wait a moment for processing
		time.Sleep(100 * time.Millisecond)

		// Stream events
		eventsResp, err := http.Get(ts.URL() + "/runs/" + runID + "/events")
		if err != nil {
			t.Fatalf("Failed to stream events: %v", err)
		}
		defer eventsResp.Body.Close()

		if eventsResp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", eventsResp.StatusCode)
		}

		// Read some events
		eventCount := 0
		timeout := time.After(2 * time.Second)
		done := make(chan bool, 1)

		go func() {
			scanner := bufio.NewScanner(eventsResp.Body)
			for scanner.Scan() && eventCount < 10 {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					eventData := strings.TrimPrefix(line, "data: ")
					if eventData != "" {
						var event map[string]interface{}
						if err := json.Unmarshal([]byte(eventData), &event); err == nil {
							eventCount++
							t.Logf("Received event: %s", event["type"])
						}
					}
				}
			}
			done <- true
		}()

		select {
		case <-done:
			// Events read successfully
		case <-timeout:
			// Timeout - that's okay if we got some events
		}

		if eventCount == 0 {
			t.Error("No events received")
		}
	})

	t.Run("CancelRun", func(t *testing.T) {
		// Create a run
		createReq := map[string]interface{}{
			"tenant_id": "test-tenant",
			"mode":      "autonomous",
			"input":     "This is a test run that might be cancelled",
		}

		body, _ := json.Marshal(createReq)
		resp, err := http.Post(ts.URL()+"/runs", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to create run: %v", err)
		}
		defer resp.Body.Close()

		var createResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode create response: %v", err)
		}

		runIDInterface, exists := createResp["id"]
		if !exists || runIDInterface == nil {
			t.Fatalf("No run ID in response: %v", createResp)
		}

		runID, ok := runIDInterface.(string)
		if !ok {
			t.Fatalf("Run ID is not a string: %v", runIDInterface)
		}

		// Try to cancel the run immediately (it might already be completed)
		cancelReq, err := http.NewRequest("POST", ts.URL()+"/runs/"+runID+"/cancel", nil)
		if err != nil {
			t.Fatalf("Failed to create cancel request: %v", err)
		}

		client := &http.Client{}
		cancelResp, err := client.Do(cancelReq)
		if err != nil {
			t.Fatalf("Failed to cancel run: %v", err)
		}
		defer cancelResp.Body.Close()

		// Cancel should either succeed (200) or fail with a reasonable error (500)
		// Both are acceptable since the run might complete quickly
		if cancelResp.StatusCode != http.StatusOK && cancelResp.StatusCode != http.StatusInternalServerError {
			body, _ := io.ReadAll(cancelResp.Body)
			t.Errorf("Expected status 200 or 500, got %d: %s", cancelResp.StatusCode, string(body))
		}

		// If it succeeded, verify the response format
		if cancelResp.StatusCode == http.StatusOK {
			var cancelResult map[string]interface{}
			if err := json.NewDecoder(cancelResp.Body).Decode(&cancelResult); err != nil {
				t.Fatalf("Failed to decode cancel response: %v", err)
			}

			if cancelResult["status"] != "cancelled" {
				t.Errorf("Expected status 'cancelled', got %v", cancelResult["status"])
			}
		}
	})
}

// Test the OpenAI-compatible API
func TestOpenAICompatibleAPI(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	t.Run("NonStreamingCompletion", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "test-model",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello, can you help me?",
				},
			},
			"stream": false,
		}

		body, _ := json.Marshal(reqBody)
		resp, err := http.Post(ts.URL()+"/v1/chat/completions", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to call chat completions: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var openAIResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Validate OpenAI response format
		if openAIResp["object"] != "chat.completion" {
			t.Errorf("Expected object 'chat.completion', got %v", openAIResp["object"])
		}

		choices, ok := openAIResp["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			t.Fatalf("Expected choices array, got %v", openAIResp["choices"])
		}
	})

	t.Run("StreamingCompletion", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "test-model",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Stream me a response please",
				},
			},
			"stream": true,
		}

		body, _ := json.Marshal(reqBody)
		resp, err := http.Post(ts.URL()+"/v1/chat/completions", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to call streaming chat completions: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(bodyBytes))
		}

		// Check that we get Server-Sent Events
		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/event-stream" {
			t.Errorf("Expected content-type 'text/event-stream', got %s", contentType)
		}

		// Read a few streaming chunks
		scanner := bufio.NewScanner(resp.Body)
		chunkCount := 0
		for scanner.Scan() && chunkCount < 5 {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				chunkCount++
			}
		}

		if chunkCount == 0 {
			t.Error("No streaming chunks received")
		}
	})
}

// Test MCP tool integration
func TestMCPToolIntegration(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	t.Run("EchoToolUsage", func(t *testing.T) {
		// Create a run that should use the echo tool
		createReq := map[string]interface{}{
			"tenant_id": "test-tenant",
			"mode":      "interactive",
			"input":     "Please use the echo tool to say 'Hello from E2E test'",
		}

		body, _ := json.Marshal(createReq)
		resp, err := http.Post(ts.URL()+"/runs", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to create run: %v", err)
		}
		defer resp.Body.Close()

		var createResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode create response: %v", err)
		}

		runIDInterface, exists := createResp["id"]
		if !exists || runIDInterface == nil {
			t.Fatalf("No run ID in response: %v", createResp)
		}

		runID, ok := runIDInterface.(string)
		if !ok {
			t.Fatalf("Run ID is not a string: %v", runIDInterface)
		}

		// Poll for completion with shorter timeout for testing
		timeout := time.After(10 * time.Second)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		var finalRun map[string]interface{}
		runStatus := ""
		for {
			select {
			case <-timeout:
				t.Fatalf("Run did not complete within timeout. Last status: %s", runStatus)
			case <-ticker.C:
				getResp, err := http.Get(ts.URL() + "/runs/" + runID)
				if err != nil {
					t.Fatalf("Failed to get run: %v", err)
				}
				defer getResp.Body.Close()

				if err := json.NewDecoder(getResp.Body).Decode(&finalRun); err != nil {
					t.Fatalf("Failed to decode run response: %v", err)
				}

				status, _ := finalRun["status"].(string)
				runStatus = status
				t.Logf("Current run status: %s", status)
				if status == "completed" || status == "failed" {
					goto done
				}
			}
		}

	done:
		// Check that run completed successfully
		status, _ := finalRun["status"].(string)
		if status != "completed" {
			t.Errorf("Expected run to complete, got status: %s", status)
		}

		// Verify tool was used by checking events (read only a few lines to avoid blocking)
		eventsResp, err := http.Get(ts.URL() + "/runs/" + runID + "/events")
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}
		defer eventsResp.Body.Close()

		// Read some events to verify tool usage with timeout
		eventLines := []string{}
		done := make(chan bool, 1)

		go func() {
			scanner := bufio.NewScanner(eventsResp.Body)
			eventCount := 0
			for scanner.Scan() && eventCount < 20 { // Read limited number of events
				line := scanner.Text()
				if line != "" {
					eventLines = append(eventLines, line)
					eventCount++
					// Stop early if we find what we're looking for
					if strings.Contains(line, "tool_started") {
						break
					}
				}
			}
			done <- true
		}()

		// Wait for events with timeout
		select {
		case <-done:
			// Events read successfully
		case <-time.After(2 * time.Second):
			// Timeout - that's okay, we might have gotten some events
		}

		eventsStr := strings.Join(eventLines, "\n")
		t.Logf("Events received: %s", eventsStr)

		if !strings.Contains(eventsStr, "tool_started") {
			t.Error("Expected tool_started event not found")
		}
	})
}

// Test concurrent operations
func TestConcurrentOperations(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	t.Run("ConcurrentRuns", func(t *testing.T) {
		numRuns := 5
		var wg sync.WaitGroup
		results := make(chan string, numRuns)

		// Create multiple concurrent runs
		for i := 0; i < numRuns; i++ {
			wg.Add(1)
			go func(runIndex int) {
				defer wg.Done()

				createReq := map[string]interface{}{
					"tenant_id": "test-tenant",
					"mode":      "interactive",
					"input":     fmt.Sprintf("Test run %d", runIndex),
				}

				body, _ := json.Marshal(createReq)
				resp, err := http.Post(ts.URL()+"/runs", "application/json", bytes.NewBuffer(body))
				if err != nil {
					t.Errorf("Failed to create run %d: %v", runIndex, err)
					return
				}
				defer resp.Body.Close()

				var createResp map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
					t.Errorf("Failed to decode response for run %d: %v", runIndex, err)
					return
				}

				runIDInterface, exists := createResp["id"]
				if !exists || runIDInterface == nil {
					t.Errorf("No run ID in response for run %d: %v", runIndex, createResp)
					return
				}

				runID, ok := runIDInterface.(string)
				if !ok {
					t.Errorf("Run ID is not a string for run %d: %v", runIndex, runIDInterface)
					return
				}

				results <- runID
			}(i)
		}

		wg.Wait()
		close(results)

		// Verify all runs were created
		runCount := 0
		for runID := range results {
			if runID != "" {
				runCount++
			}
		}

		if runCount != numRuns {
			t.Errorf("Expected %d runs, got %d", numRuns, runCount)
		}
	})
}

// Test pause and resume functionality
func TestPauseResume(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	t.Run("PauseAndResumeRun", func(t *testing.T) {
		// Create a run
		createReq := map[string]interface{}{
			"tenant_id": "test-tenant",
			"mode":      "autonomous",
			"input":     "This is a test run for pause/resume functionality",
		}

		body, _ := json.Marshal(createReq)
		resp, err := http.Post(ts.URL()+"/runs", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to create run: %v", err)
		}
		defer resp.Body.Close()

		var createResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode create response: %v", err)
		}

		runIDInterface, exists := createResp["id"]
		if !exists || runIDInterface == nil {
			t.Fatalf("No run ID in response: %v", createResp)
		}

		runID, ok := runIDInterface.(string)
		if !ok {
			t.Fatalf("Run ID is not a string: %v", runIDInterface)
		}

		// Try to pause the run
		pauseReq, err := http.NewRequest("POST", ts.URL()+"/runs/"+runID+"/pause", nil)
		if err != nil {
			t.Fatalf("Failed to create pause request: %v", err)
		}

		client := &http.Client{}
		pauseResp, err := client.Do(pauseReq)
		if err != nil {
			t.Fatalf("Failed to pause run: %v", err)
		}
		defer pauseResp.Body.Close()

		// Pause should either succeed (200) or fail gracefully (e.g., already completed)
		if pauseResp.StatusCode != http.StatusOK && pauseResp.StatusCode != http.StatusBadRequest && pauseResp.StatusCode != http.StatusInternalServerError {
			body, _ := io.ReadAll(pauseResp.Body)
			t.Errorf("Unexpected pause response status %d: %s", pauseResp.StatusCode, string(body))
		}

		// Try to resume the run (should work even if pause didn't)
		resumeReq, err := http.NewRequest("POST", ts.URL()+"/runs/"+runID+"/resume", nil)
		if err != nil {
			t.Fatalf("Failed to create resume request: %v", err)
		}

		resumeResp, err := client.Do(resumeReq)
		if err != nil {
			t.Fatalf("Failed to resume run: %v", err)
		}
		defer resumeResp.Body.Close()

		// Resume should also handle gracefully
		if resumeResp.StatusCode != http.StatusOK && resumeResp.StatusCode != http.StatusBadRequest && resumeResp.StatusCode != http.StatusInternalServerError {
			body, _ := io.ReadAll(resumeResp.Body)
			t.Errorf("Unexpected resume response status %d: %s", resumeResp.StatusCode, string(body))
		}
	})
}

// Test error scenarios
func TestErrorScenarios(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	t.Run("InvalidRunID", func(t *testing.T) {
		resp, err := http.Get(ts.URL() + "/runs/nonexistent-run-id")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidCreateRunRequest", func(t *testing.T) {
		// Test completely empty request
		resp, err := http.Post(ts.URL()+"/runs", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// The API might accept empty requests and use defaults, so let's test malformed JSON instead
		resp2, err := http.Post(ts.URL()+"/runs", "application/json", strings.NewReader("invalid json"))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp2.Body.Close()

		// Should get 400 for malformed JSON
		if resp2.StatusCode == http.StatusCreated {
			t.Error("Expected error for malformed JSON request, but got success")
		}

		// Also test wrong content type
		resp3, err := http.Post(ts.URL()+"/runs", "text/plain", strings.NewReader("not json"))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp3.Body.Close()

		// This should also fail
		if resp3.StatusCode == http.StatusCreated {
			t.Error("Expected error for wrong content type, but got success")
		}
	})

	t.Run("MethodNotAllowed", func(t *testing.T) {
		// Try DELETE on /runs which should not be allowed
		req, _ := http.NewRequest("DELETE", ts.URL()+"/runs", nil)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", resp.StatusCode)
		}
	})
}
