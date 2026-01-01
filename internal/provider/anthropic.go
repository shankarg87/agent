package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/shankgan/agent/internal/config"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// AnthropicProvider implements Provider for Anthropic Claude models
type AnthropicProvider struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(cfg config.ModelConfig) (*AnthropicProvider, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key not configured")
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = anthropicAPIURL
	}

	return &AnthropicProvider{
		apiKey:   apiKey,
		model:    cfg.Model,
		endpoint: endpoint,
		client:   &http.Client{},
	}, nil
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Model() string {
	return p.model
}

func (p *AnthropicProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	anthropicReq := p.convertRequest(req)
	anthropicReq.Stream = false

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.convertResponse(&anthropicResp), nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	anthropicReq := p.convertRequest(req)
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	eventChan := make(chan StreamEvent, 10)

	go func() {
		defer resp.Body.Close()
		defer close(eventChan)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				eventChan <- StreamEvent{Type: "done", Done: true}
				return
			}

			var chunk anthropicStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				eventChan <- StreamEvent{Type: "error", Error: err}
				return
			}

			event := p.convertStreamChunk(&chunk)
			if event != nil {
				eventChan <- *event
			}
		}

		if err := scanner.Err(); err != nil {
			eventChan <- StreamEvent{Type: "error", Error: err}
		}
	}()

	return eventChan, nil
}

func (p *AnthropicProvider) convertRequest(req *ChatRequest) *anthropicRequest {
	anthropicReq := &anthropicRequest{
		Model:       p.model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Messages:    make([]anthropicMessage, 0),
	}

	if anthropicReq.MaxTokens == 0 {
		anthropicReq.MaxTokens = 4096
	}

	// Extract system message if present
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			anthropicReq.System = msg.Content
		} else {
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		anthropicReq.Tools = make([]anthropicTool, len(req.Tools))
		for i, tool := range req.Tools {
			anthropicReq.Tools[i] = anthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			}
		}
	}

	return anthropicReq
}

func (p *AnthropicProvider) convertResponse(resp *anthropicResponse) *ChatResponse {
	chatResp := &ChatResponse{
		ID:           resp.ID,
		Role:         resp.Role,
		FinishReason: resp.StopReason,
		Usage: Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	// Extract content and tool calls
	for _, content := range resp.Content {
		if content.Type == "text" {
			chatResp.Content += content.Text
		} else if content.Type == "tool_use" {
			argsJSON, _ := json.Marshal(content.Input)
			chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
				ID:   content.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      content.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	return chatResp
}

func (p *AnthropicProvider) convertStreamChunk(chunk *anthropicStreamChunk) *StreamEvent {
	switch chunk.Type {
	case "content_block_delta":
		if chunk.Delta.Type == "text_delta" {
			return &StreamEvent{
				Type:    "content_delta",
				Content: chunk.Delta.Text,
			}
		}
	case "message_delta":
		if chunk.Delta.StopReason != "" {
			return &StreamEvent{
				Type: "done",
				Done: true,
				Usage: &Usage{
					PromptTokens:     chunk.Usage.InputTokens,
					CompletionTokens: chunk.Usage.OutputTokens,
					TotalTokens:      chunk.Usage.InputTokens + chunk.Usage.OutputTokens,
				},
			}
		}
	}
	return nil
}

// Anthropic API types

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Content    []anthropicContent      `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      anthropicUsage          `json:"usage"`
}

type anthropicContent struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamChunk struct {
	Type  string `json:"type"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason,omitempty"`
	} `json:"delta"`
	Usage anthropicUsage `json:"usage,omitempty"`
}
