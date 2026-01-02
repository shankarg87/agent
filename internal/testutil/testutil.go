package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/shankarg87/agent/internal/config"
	"github.com/shankarg87/agent/internal/provider"
	"github.com/shankarg87/agent/internal/store"
)

// MockLLMProvider implements provider.LLMProvider for testing
type MockLLMProvider struct {
	ChatFunc   func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error)
	StreamFunc func(ctx context.Context, req *provider.ChatRequest, handler func(*provider.ChatResponse) error) error
}

func (m *MockLLMProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	if m.ChatFunc != nil {
		return m.ChatFunc(ctx, req)
	}
	return &provider.ChatResponse{
		ID:           "mock-response-id",
		Content:      "Mock response",
		Role:         "assistant",
		FinishReason: "stop",
		Usage: provider.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (m *MockLLMProvider) Stream(ctx context.Context, req *provider.ChatRequest, handler func(*provider.ChatResponse) error) error {
	if m.StreamFunc != nil {
		return m.StreamFunc(ctx, req, handler)
	}
	return handler(&provider.ChatResponse{
		ID:           "mock-stream-id",
		Content:      "Mock stream response",
		Role:         "assistant",
		FinishReason: "stop",
		Usage: provider.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	})
}

// TestAgentConfig returns a minimal valid agent config for testing
func TestAgentConfig() *config.AgentConfig {
	return &config.AgentConfig{
		ProfileName:    "test-agent",
		ProfileVersion: "1.0.0",
		Description:    "Test agent configuration",
		Labels:         []string{"test"},
		PrimaryModel: config.ModelConfig{
			Provider: "openai",
			Model:    "gpt-4",
		},
		RoutingStrategy:     "single",
		MaxContextTokens:    4096,
		MaxOutputTokens:     1024,
		Temperature:         0.7,
		SystemPrompt:        "You are a helpful assistant.",
		OutputStyle:         "concise",
		ApprovalMode:        "never",
		AutoApproveInDaemon: true,
		MaxToolCalls:        10,
		MaxRunTimeSeconds:   300,
		MaxFailuresPerRun:   3,
		MemoryEnabled:       false,
		LogLevel:            "info",
	}
}

// TestSession returns a test session
func TestSession(tenantID, profileName string) *store.Session {
	return &store.Session{
		ID:          "test-session-123",
		TenantID:    tenantID,
		ProfileName: profileName,
		Metadata:    map[string]any{"test": true},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// TestRun returns a test run
func TestRun(sessionID, tenantID string) *store.Run {
	return &store.Run{
		ID:        "test-run-123",
		SessionID: sessionID,
		TenantID:  tenantID,
		Mode:      "interactive",
		Status:    "running",
		Metadata:  map[string]any{"test": true},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// TestMessage returns a test message
func TestMessage(role, content string) *store.Message {
	return &store.Message{
		ID:        "test-msg-123",
		Role:      role,
		Content:   content,
		Metadata:  map[string]any{"test": true},
		CreatedAt: time.Now(),
	}
}

// TestEvent returns a test event
func TestEvent(eventType string, data map[string]any) *store.Event {
	return &store.Event{
		ID:        "test-event-123",
		RunID:     "test-run-123",
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// TestToolCall returns a test tool call
func TestToolCall(toolName string) *store.ToolCall {
	return &store.ToolCall{
		ID:         "test-tool-123",
		RunID:      "test-run-123",
		ToolName:   toolName,
		ServerName: "test-server",
		Arguments:  map[string]any{"input": "test"},
		Status:     store.ToolCallStatusPending,
		Output:     "",
		Error:      "",
		RetryCount: 0,
		CreatedAt:  time.Now(),
	}
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

// AssertNotNil fails the test if value is nil
func AssertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value, got nil")
	}
}
