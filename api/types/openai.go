package types

// OpenAI API types for /v1/chat/completions

type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []OpenAIStreamChoice `json:"choices"`
}

type OpenAIStreamChoice struct {
	Index        int         `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

type OpenAIDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// OpenAI Responses API types for /v1/responses

type ResponsesRequest struct {
	Model       string   `json:"model"`
	Messages    []OpenAIMessage `json:"messages"` // Similar to chat, but could have different structure
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
	Store       bool     `json:"store,omitempty"` // Responses are stored by default
}

type ResponsesResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"` // "response"
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []ResponseChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type ResponseChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// Streaming types for Responses API
type ResponsesStreamChunk struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"` // "response.chunk"
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []ResponsesStreamChoice `json:"choices"`
}

type ResponsesStreamChoice struct {
	Index        int         `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}
