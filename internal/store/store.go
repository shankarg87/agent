package store

import (
	"context"
	"time"
)

// Store defines the interface for persisting agent runtime data
type Store interface {
	// Sessions
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	ListSessions(ctx context.Context, tenantID string, limit, offset int) ([]*Session, error)

	// Runs
	CreateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, runID string) (*Run, error)
	UpdateRun(ctx context.Context, run *Run) error
	ListRuns(ctx context.Context, sessionID string) ([]*Run, error)

	// Messages
	AddMessage(ctx context.Context, sessionID string, message *Message) error
	GetMessages(ctx context.Context, sessionID string) ([]*Message, error)

	// Events
	AddEvent(ctx context.Context, runID string, event *Event) error
	GetEvents(ctx context.Context, runID string) ([]*Event, error)

	// Tool calls
	AddToolCall(ctx context.Context, runID string, toolCall *ToolCall) error
	UpdateToolCall(ctx context.Context, toolCall *ToolCall) error
	GetToolCalls(ctx context.Context, runID string) ([]*ToolCall, error)

	// Cleanup methods
	DeleteSession(ctx context.Context, sessionID string) error
	DeleteRun(ctx context.Context, runID string) error
	CleanupOldSessions(ctx context.Context, tenantID string, olderThan time.Duration) error
	CleanupOldRuns(ctx context.Context, sessionID string, olderThan time.Duration) error
}

// Session represents a conversation session
type Session struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	ProfileName string         `json:"profile_name"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Run represents a single execution run
type Run struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	TenantID  string         `json:"tenant_id"`
	Mode      string         `json:"mode"`   // interactive, autonomous
	Status    string         `json:"status"` // queued, running, paused_checkpoint, completed, failed, cancelled
	Input     string         `json:"input,omitempty"`
	Output    string         `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`

	// Stats
	ToolCallCount int     `json:"tool_call_count"`
	FailureCount  int     `json:"failure_count"`
	CostUSD       float64 `json:"cost_usd"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// Message represents a conversation message
type Message struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	Role      string         `json:"role"` // system, user, assistant, tool
	Content   string         `json:"content"`
	ToolCalls []ToolCallRef  `json:"tool_calls,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type ToolCallRef struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Event represents a runtime event
type Event struct {
	ID        string         `json:"id"`
	RunID     string         `json:"run_id"`
	Type      string         `json:"type"` // run_started, text_delta, tool_started, etc.
	Data      map[string]any `json:"data"`
	Timestamp time.Time      `json:"timestamp"`
}

// ToolCall represents a tool invocation
type ToolCall struct {
	ID          string         `json:"id"`
	RunID       string         `json:"run_id"`
	ToolName    string         `json:"tool_name"`
	ServerName  string         `json:"server_name"`
	Arguments   map[string]any `json:"arguments"`
	Status      string         `json:"status"` // pending, running, completed, failed, cancelled
	Output      string         `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	RetryCount  int            `json:"retry_count"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// RunState constants
const (
	RunStateQueued           = "queued"
	RunStateRunning          = "running"
	RunStatePausedCheckpoint = "paused_checkpoint"
	RunStatePaused           = "paused"
	RunStateCompleted        = "completed"
	RunStateFailed           = "failed"
	RunStateCancelled        = "cancelled"
)

// ToolCallStatus constants
const (
	ToolCallStatusPending   = "pending"
	ToolCallStatusRunning   = "running"
	ToolCallStatusCompleted = "completed"
	ToolCallStatusFailed    = "failed"
	ToolCallStatusCancelled = "cancelled"
)

// Event types
const (
	EventTypeRunStarted         = "run_started"
	EventTypeRunCompleted       = "run_completed"
	EventTypeRunFailed          = "run_failed"
	EventTypeRunCancelled       = "run_cancelled"
	EventTypeRunPaused          = "run_paused"
	EventTypeRunResumed         = "run_resumed"
	EventTypeTextDelta          = "text_delta"
	EventTypeFinalText          = "final_text"
	EventTypeToolStarted        = "tool_started"
	EventTypeToolStdout         = "tool_stdout"
	EventTypeToolStderr         = "tool_stderr"
	EventTypeToolCompleted      = "tool_completed"
	EventTypeToolFailed         = "tool_failed"
	EventTypeCheckpointRequired = "checkpoint_required"
	EventTypeArtifactCreated    = "artifact_created"
)
