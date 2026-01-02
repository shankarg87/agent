package provider

import (
	"context"
	"fmt"

	"github.com/shankarg87/agent/internal/config"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// Chat sends a chat completion request
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Stream sends a streaming chat completion request
	Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

	// Name returns the provider name
	Name() string

	// Model returns the model identifier
	Model() string
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Messages    []Message      `json:"messages"`
	Tools       []Tool         `json:"tools,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	TopP        float64        `json:"top_p,omitempty"`
	Stop        []string       `json:"stop,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID           string     `json:"id"`
	Content      string     `json:"content"`
	Role         string     `json:"role"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	Usage        Usage      `json:"usage"`
}

// StreamEvent represents a streaming response event
type StreamEvent struct {
	Type     string    `json:"type"` // content_delta, tool_call, done, error
	Content  string    `json:"content,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Done     bool      `json:"done,omitempty"`
	Error    error     `json:"error,omitempty"`
	Usage    *Usage    `json:"usage,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"` // system, user, assistant, tool
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // for tool response messages
}

// Tool represents a tool definition
type Tool struct {
	Type     string   `json:"type"` // function
	Function Function `json:"function"`
}

// Function represents a function tool definition
type Function struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// ToolCall represents a tool call request from the model
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // function
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewProvider creates a new provider based on the model config
func NewProvider(cfg config.ModelConfig) (Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		return NewAnthropicProvider(cfg)
	case "openai":
		return NewOpenAIProvider(cfg)
	case "gemini":
		return NewGeminiProvider(cfg)
	case "ollama":
		return NewOllamaProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}
