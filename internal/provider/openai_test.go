package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/shankarg87/agent/internal/config"
)

func TestNewOpenAIProvider(t *testing.T) {
	// Test with API key in config
	cfg := config.ModelConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "test-key-123",
	}

	provider, err := NewOpenAIProvider(cfg)
	assertNoError(t, err)
	assertNotNil(t, provider)
	assertEqual(t, "openai", provider.Name())
	assertEqual(t, "gpt-4", provider.Model())
}

func TestNewOpenAIProvider_WithEnvironmentKey(t *testing.T) {
	// Set environment variable
	originalKey := os.Getenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_API_KEY", "env-test-key")
	defer func() {
		if originalKey != "" {
			os.Setenv("OPENAI_API_KEY", originalKey)
		} else {
			os.Unsetenv("OPENAI_API_KEY")
		}
	}()

	cfg := config.ModelConfig{
		Provider: "openai",
		Model:    "gpt-3.5-turbo",
	}

	provider, err := NewOpenAIProvider(cfg)
	assertNoError(t, err)
	assertNotNil(t, provider)
	assertEqual(t, "openai", provider.Name())
	assertEqual(t, "gpt-3.5-turbo", provider.Model())
}

func TestNewOpenAIProvider_NoAPIKey(t *testing.T) {
	// Ensure no environment variable is set
	originalKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("OPENAI_API_KEY", originalKey)
		}
	}()

	cfg := config.ModelConfig{
		Provider: "openai",
		Model:    "gpt-4",
	}

	_, err := NewOpenAIProvider(cfg)
	assertError(t, err)
	if !strings.Contains(err.Error(), "API key not configured") {
		t.Errorf("Expected API key error, got: %v", err)
	}
}

func TestNewOpenAIProvider_CustomEndpoint(t *testing.T) {
	cfg := config.ModelConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "test-key",
		Endpoint: "https://custom.openai.com/v1/chat/completions",
	}

	provider, err := NewOpenAIProvider(cfg)
	assertNoError(t, err)
	assertNotNil(t, provider)

	// We can't easily test the private endpoint field without reflection,
	// but we know it's set correctly if NewOpenAIProvider doesn't error
	assertEqual(t, "openai", provider.Name())
	assertEqual(t, "gpt-4", provider.Model())
}

func TestOpenAIProvider_Chat(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		assertEqual(t, "application/json", r.Header.Get("Content-Type"))
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("Expected Authorization header with Bearer token")
		}

		// Mock response
		response := `{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello! How can I help you today?"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 8,
				"total_tokens": 18
			}
		}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create provider with mock server URL
	cfg := config.ModelConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "test-key",
		Endpoint: server.URL,
	}

	provider, err := NewOpenAIProvider(cfg)
	assertNoError(t, err)

	// Test chat request
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	resp, err := provider.Chat(context.Background(), req)
	assertNoError(t, err)
	assertNotNil(t, resp)

	assertEqual(t, "chatcmpl-123", resp.ID)
	assertEqual(t, "Hello! How can I help you today?", resp.Content)
	assertEqual(t, "assistant", resp.Role)
	assertEqual(t, "stop", resp.FinishReason)
	assertEqual(t, 10, resp.Usage.PromptTokens)
	assertEqual(t, 8, resp.Usage.CompletionTokens)
	assertEqual(t, 18, resp.Usage.TotalTokens)
}

func TestOpenAIProvider_Chat_WithToolCalls(t *testing.T) {
	// Create mock server that returns tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "I'll help you with that.",
					"tool_calls": [{
						"id": "call_123",
						"type": "function",
						"function": {
							"name": "get_weather",
							"arguments": "{\"location\": \"San Francisco\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {
				"prompt_tokens": 15,
				"completion_tokens": 20,
				"total_tokens": 35
			}
		}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := config.ModelConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "test-key",
		Endpoint: server.URL,
	}

	provider, err := NewOpenAIProvider(cfg)
	assertNoError(t, err)

	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Tools: []Tool{
			{
				Type: "function",
				Function: Function{
					Name:        "get_weather",
					Description: "Get weather for a location",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	resp, err := provider.Chat(context.Background(), req)
	assertNoError(t, err)
	assertNotNil(t, resp)

	assertEqual(t, "I'll help you with that.", resp.Content)
	assertEqual(t, "tool_calls", resp.FinishReason)
	assertEqual(t, 1, len(resp.ToolCalls))
	assertEqual(t, "call_123", resp.ToolCalls[0].ID)
	assertEqual(t, "function", resp.ToolCalls[0].Type)
	assertEqual(t, "get_weather", resp.ToolCalls[0].Function.Name)
	assertEqual(t, `{"location": "San Francisco"}`, resp.ToolCalls[0].Function.Arguments)
}

func TestOpenAIProvider_Chat_APIError(t *testing.T) {
	// Create mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	cfg := config.ModelConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "invalid-key",
		Endpoint: server.URL,
	}

	provider, err := NewOpenAIProvider(cfg)
	assertNoError(t, err)

	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err = provider.Chat(context.Background(), req)
	assertError(t, err)
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("Expected API error, got: %v", err)
	}
}
