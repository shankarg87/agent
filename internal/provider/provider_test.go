package provider

import (
	"testing"

	"github.com/shankarg87/agent/internal/config"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name         string
		config       config.ModelConfig
		expectedType string
		shouldError  bool
	}{
		{
			name: "OpenAI provider",
			config: config.ModelConfig{
				Provider: "openai",
				Model:    "gpt-4",
				APIKey:   "test-key",
			},
			expectedType: "openai",
			shouldError:  false,
		},
		{
			name: "Anthropic provider",
			config: config.ModelConfig{
				Provider: "anthropic",
				Model:    "claude-3-sonnet",
				APIKey:   "test-key",
			},
			expectedType: "anthropic",
			shouldError:  false,
		},
		{
			name: "Unsupported provider",
			config: config.ModelConfig{
				Provider: "unsupported",
				Model:    "some-model",
			},
			expectedType: "",
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.config)

			if tt.shouldError {
				assertError(t, err)
				assertNil(t, provider)
			} else {
				assertNoError(t, err)
				assertNotNil(t, provider)
				assertEqual(t, tt.expectedType, provider.Name())
			}
		})
	}
}

func TestChatRequest_Structure(t *testing.T) {
	req := &ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
		Tools: []Tool{
			{
				Type: "function",
				Function: Function{
					Name:        "test_function",
					Description: "A test function",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"input": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
		Temperature: 0.7,
		MaxTokens:   1000,
		TopP:        0.9,
		Stop:        []string{"\n\n"},
		Stream:      false,
		Metadata:    map[string]any{"test": true},
	}

	assertEqual(t, 2, len(req.Messages))
	assertEqual(t, "system", req.Messages[0].Role)
	assertEqual(t, "You are helpful", req.Messages[0].Content)
	assertEqual(t, "user", req.Messages[1].Role)
	assertEqual(t, "Hello", req.Messages[1].Content)

	assertEqual(t, 1, len(req.Tools))
	assertEqual(t, "function", req.Tools[0].Type)
	assertEqual(t, "test_function", req.Tools[0].Function.Name)
	assertEqual(t, "A test function", req.Tools[0].Function.Description)

	assertEqual(t, 0.7, req.Temperature)
	assertEqual(t, 1000, req.MaxTokens)
	assertEqual(t, 0.9, req.TopP)
	assertEqual(t, 1, len(req.Stop))
	assertEqual(t, "\n\n", req.Stop[0])
	assertEqual(t, false, req.Stream)
	assertEqual(t, true, req.Metadata["test"])
}

func TestChatResponse_Structure(t *testing.T) {
	resp := &ChatResponse{
		ID:      "resp-123",
		Content: "Hello there!",
		Role:    "assistant",
		ToolCalls: []ToolCall{
			{
				ID:   "call-123",
				Type: "function",
				Function: FunctionCall{
					Name:      "test_func",
					Arguments: `{"input": "test"}`,
				},
			},
		},
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	assertEqual(t, "resp-123", resp.ID)
	assertEqual(t, "Hello there!", resp.Content)
	assertEqual(t, "assistant", resp.Role)
	assertEqual(t, 1, len(resp.ToolCalls))
	assertEqual(t, "call-123", resp.ToolCalls[0].ID)
	assertEqual(t, "function", resp.ToolCalls[0].Type)
	assertEqual(t, "test_func", resp.ToolCalls[0].Function.Name)
	assertEqual(t, `{"input": "test"}`, resp.ToolCalls[0].Function.Arguments)
	assertEqual(t, "stop", resp.FinishReason)
	assertEqual(t, 10, resp.Usage.PromptTokens)
	assertEqual(t, 5, resp.Usage.CompletionTokens)
	assertEqual(t, 15, resp.Usage.TotalTokens)
}

func TestStreamEvent_Structure(t *testing.T) {
	events := []StreamEvent{
		{
			Type:    "content_delta",
			Content: "Hello",
			Done:    false,
		},
		{
			Type: "tool_call",
			ToolCall: &ToolCall{
				ID:   "call-123",
				Type: "function",
				Function: FunctionCall{
					Name:      "test_func",
					Arguments: `{"test": true}`,
				},
			},
			Done: false,
		},
		{
			Type: "done",
			Done: true,
			Usage: &Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}

	assertEqual(t, "content_delta", events[0].Type)
	assertEqual(t, "Hello", events[0].Content)
	assertEqual(t, false, events[0].Done)

	assertEqual(t, "tool_call", events[1].Type)
	assertNotNil(t, events[1].ToolCall)
	assertEqual(t, "call-123", events[1].ToolCall.ID)
	assertEqual(t, "function", events[1].ToolCall.Type)

	assertEqual(t, "done", events[2].Type)
	assertEqual(t, true, events[2].Done)
	assertNotNil(t, events[2].Usage)
	assertEqual(t, 15, events[2].Usage.TotalTokens)
}

func TestMessage_Variants(t *testing.T) {
	// System message
	sysMsg := Message{
		Role:    "system",
		Content: "You are a helpful assistant",
	}
	assertEqual(t, "system", sysMsg.Role)
	assertEqual(t, "You are a helpful assistant", sysMsg.Content)

	// User message
	userMsg := Message{
		Role:    "user",
		Content: "What's the weather like?",
	}
	assertEqual(t, "user", userMsg.Role)
	assertEqual(t, "What's the weather like?", userMsg.Content)

	// Assistant message with tool call
	assistantMsg := Message{
		Role:    "assistant",
		Content: "I'll check the weather for you.",
		ToolCalls: []ToolCall{
			{
				ID:   "call-123",
				Type: "function",
				Function: FunctionCall{
					Name:      "get_weather",
					Arguments: `{"location": "San Francisco"}`,
				},
			},
		},
	}
	assertEqual(t, "assistant", assistantMsg.Role)
	assertEqual(t, "I'll check the weather for you.", assistantMsg.Content)
	assertEqual(t, 1, len(assistantMsg.ToolCalls))

	// Tool response message
	toolMsg := Message{
		Role:       "tool",
		Content:    `{"temperature": 72, "condition": "sunny"}`,
		ToolCallID: "call-123",
	}
	assertEqual(t, "tool", toolMsg.Role)
	assertEqual(t, `{"temperature": 72, "condition": "sunny"}`, toolMsg.Content)
	assertEqual(t, "call-123", toolMsg.ToolCallID)
}

func TestFunction_Schema(t *testing.T) {
	fn := Function{
		Name:        "get_weather",
		Description: "Get current weather for a location",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{
					"type":        "string",
					"description": "The city and state",
				},
				"unit": map[string]any{
					"type":        "string",
					"enum":        []string{"celsius", "fahrenheit"},
					"description": "Temperature unit",
				},
			},
			"required": []string{"location"},
		},
	}

	assertEqual(t, "get_weather", fn.Name)
	assertEqual(t, "Get current weather for a location", fn.Description)

	assertNotNil(t, fn.Parameters)
	assertEqual(t, "object", fn.Parameters["type"])

	properties := fn.Parameters["properties"].(map[string]any)
	assertNotNil(t, properties)

	location := properties["location"].(map[string]any)
	assertEqual(t, "string", location["type"])
	assertEqual(t, "The city and state", location["description"])

	unit := properties["unit"].(map[string]any)
	assertEqual(t, "string", unit["type"])
	assertEqual(t, "Temperature unit", unit["description"])

	required := fn.Parameters["required"].([]string)
	assertEqual(t, 1, len(required))
	assertEqual(t, "location", required[0])
}

// Test helpers
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

func assertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

func assertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value, got nil")
	}
}

func assertNil(t *testing.T, value any) {
	t.Helper()
	if value != nil {
		t.Fatalf("Expected nil value, got %v", value)
	}
}
