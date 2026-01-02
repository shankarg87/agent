package types

// Anthropic Messages API types for /v1/messages

type MessagesRequest struct {
	Model       string             `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"` // Separate from messages
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Tools       []AnthropicTool    `json:"tools,omitempty"` // Ignored by agent
}

type AnthropicMessage struct {
	Role    string              `json:"role"` // "user" | "assistant"
	Content interface{}         `json:"content"` // Can be string or []AnthropicContent
}

type AnthropicContent struct {
	Type      string      `json:"type"` // "text" | "tool_use" | "tool_result"
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`       // For tool_use
	Name      string      `json:"name,omitempty"`     // For tool_use
	Input     interface{} `json:"input,omitempty"`    // For tool_use
	ToolUseID string      `json:"tool_use_id,omitempty"` // For tool_result
	Content   string      `json:"content,omitempty"`  // For tool_result
}

type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type MessagesResponse struct {
	ID           string              `json:"id"`
	Type         string              `json:"type"` // "message"
	Role         string              `json:"role"` // "assistant"
	Content      []AnthropicContent  `json:"content"`
	Model        string              `json:"model"`
	StopReason   string              `json:"stop_reason,omitempty"`
	StopSequence string              `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage      `json:"usage"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Anthropic SSE event types for streaming

type MessageStartEvent struct {
	Type    string          `json:"type"` // "message_start"
	Message MessageMetadata `json:"message"`
}

type MessageMetadata struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"` // "message"
	Role   string         `json:"role"` // "assistant"
	Model  string         `json:"model"`
	Usage  AnthropicUsage `json:"usage"`
}

type ContentBlockStart struct {
	Type         string           `json:"type"` // "content_block_start"
	Index        int              `json:"index"`
	ContentBlock AnthropicContent `json:"content_block"`
}

type ContentBlockDelta struct {
	Type  string           `json:"type"` // "content_block_delta"
	Index int              `json:"index"`
	Delta ContentDeltaData `json:"delta"`
}

type ContentDeltaData struct {
	Type string `json:"type"` // "text_delta"
	Text string `json:"text"`
}

type ContentBlockStop struct {
	Type  string `json:"type"` // "content_block_stop"
	Index int    `json:"index"`
}

type MessageDelta struct {
	Type  string          `json:"type"` // "message_delta"
	Delta MessageDeltaData `json:"delta"`
	Usage AnthropicUsage   `json:"usage,omitempty"`
}

type MessageDeltaData struct {
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

type MessageStop struct {
	Type string `json:"type"` // "message_stop"
}

// Custom extension for tool execution visibility
type AgentToolExecution struct {
	ToolName   string      `json:"tool_name"`
	Arguments  interface{} `json:"arguments"`
	Output     interface{} `json:"output"`
	Status     string      `json:"status"` // "started" | "completed" | "failed"
	Error      string      `json:"error,omitempty"`
}
