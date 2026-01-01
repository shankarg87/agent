package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shankgan/agent/internal/config"
	"github.com/shankgan/agent/internal/events"
	"github.com/shankgan/agent/internal/mcp"
	"github.com/shankgan/agent/internal/provider"
	"github.com/shankgan/agent/internal/store"
)

// Runtime manages the agent execution environment
type Runtime struct {
	config      *config.AgentConfig
	store       store.Store
	eventBus    *events.EventBus
	provider    provider.Provider
	mcpRegistry *mcp.Registry

	mu             sync.RWMutex
	activeRuns     map[string]*RunContext
	cancellations  map[string]context.CancelFunc
}

// RunContext holds the execution context for a single run
type RunContext struct {
	Run          *store.Run
	Session      *store.Session
	Messages     []*store.Message
	Cancel       context.CancelFunc
	ToolCallCount int
	FailureCount  int
}

// NewRuntime creates a new runtime instance
func NewRuntime(
	cfg *config.AgentConfig,
	st store.Store,
	eb *events.EventBus,
	prov provider.Provider,
	mcpReg *mcp.Registry,
) *Runtime {
	return &Runtime{
		config:        cfg,
		store:         st,
		eventBus:      eb,
		provider:      prov,
		mcpRegistry:   mcpReg,
		activeRuns:    make(map[string]*RunContext),
		cancellations: make(map[string]context.CancelFunc),
	}
}

// CreateRun creates a new run
func (r *Runtime) CreateRun(ctx context.Context, req *CreateRunRequest) (*store.Run, error) {
	// Get or create session
	var session *store.Session
	if req.SessionID != "" {
		s, err := r.store.GetSession(ctx, req.SessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
		session = s
	} else {
		session = &store.Session{
			ID:          uuid.New().String(),
			TenantID:    req.TenantID,
			ProfileName: r.config.ProfileName,
			Metadata:    req.Metadata,
		}
		if err := r.store.CreateSession(ctx, session); err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
	}

	// Create run
	run := &store.Run{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		TenantID:  req.TenantID,
		Mode:      req.Mode,
		Status:    store.RunStateQueued,
		Input:     req.Input,
		Metadata:  req.Metadata,
	}

	if err := r.store.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create run: %w", err)
	}

	// Add user message if input provided
	if req.Input != "" {
		msg := &store.Message{
			Role:      "user",
			Content:   req.Input,
			SessionID: session.ID,
		}
		if err := r.store.AddMessage(ctx, session.ID, msg); err != nil {
			return nil, fmt.Errorf("failed to add message: %w", err)
		}
	}

	// Start execution
	go r.executeRun(context.Background(), run.ID)

	return run, nil
}

// GetRun retrieves a run by ID
func (r *Runtime) GetRun(ctx context.Context, runID string) (*store.Run, error) {
	return r.store.GetRun(ctx, runID)
}

// CancelRun cancels a running run
func (r *Runtime) CancelRun(ctx context.Context, runID string) error {
	r.mu.Lock()
	cancel, ok := r.cancellations[runID]
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("run not found or not running")
	}

	cancel()

	// Update run status
	run, err := r.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}

	run.Status = store.RunStateCancelled
	now := time.Now()
	run.EndedAt = &now

	return r.store.UpdateRun(ctx, run)
}

// executeRun is the main execution loop for a run
func (r *Runtime) executeRun(parentCtx context.Context, runID string) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Register cancellation
	r.mu.Lock()
	r.cancellations[runID] = cancel
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.cancellations, runID)
		delete(r.activeRuns, runID)
		r.mu.Unlock()

		r.eventBus.CloseAll(runID)
	}()

	// Apply timeout if configured
	if r.config.MaxRunTimeSeconds > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(r.config.MaxRunTimeSeconds)*time.Second)
		defer timeoutCancel()
	}

	// Load run and session
	run, err := r.store.GetRun(parentCtx, runID)
	if err != nil {
		r.failRun(parentCtx, runID, fmt.Errorf("failed to load run: %w", err))
		return
	}

	session, err := r.store.GetSession(parentCtx, run.SessionID)
	if err != nil {
		r.failRun(parentCtx, runID, fmt.Errorf("failed to load session: %w", err))
		return
	}

	messages, err := r.store.GetMessages(parentCtx, session.ID)
	if err != nil {
		r.failRun(parentCtx, runID, fmt.Errorf("failed to load messages: %w", err))
		return
	}

	runCtx := &RunContext{
		Run:      run,
		Session:  session,
		Messages: messages,
		Cancel:   cancel,
	}

	r.mu.Lock()
	r.activeRuns[runID] = runCtx
	r.mu.Unlock()

	// Start execution
	r.publishEvent(runID, store.EventTypeRunStarted, map[string]any{
		"run_id":     runID,
		"session_id": session.ID,
		"mode":       run.Mode,
	})

	run.Status = store.RunStateRunning
	now := time.Now()
	run.StartedAt = &now
	r.store.UpdateRun(parentCtx, run)

	// Execute the agent loop
	if err := r.runAgentLoop(ctx, runCtx); err != nil {
		r.failRun(parentCtx, runID, err)
		return
	}

	// Complete the run
	run.Status = store.RunStateCompleted
	now = time.Now()
	run.EndedAt = &now
	r.store.UpdateRun(parentCtx, run)

	r.publishEvent(runID, store.EventTypeRunCompleted, map[string]any{
		"run_id":     runID,
		"output":     run.Output,
		"tool_calls": run.ToolCallCount,
		"cost_usd":   run.CostUSD,
	})
}

// runAgentLoop executes the main agent reasoning loop
func (r *Runtime) runAgentLoop(ctx context.Context, runCtx *RunContext) error {
	maxIterations := r.config.MaxToolCalls
	iteration := 0

	for iteration < maxIterations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Build messages for LLM
		providerMessages := r.buildProviderMessages(runCtx)

		// Build tools for LLM
		tools := r.buildProviderTools()

		// Call LLM
		req := &provider.ChatRequest{
			Messages:    providerMessages,
			Tools:       tools,
			Temperature: r.config.Temperature,
			MaxTokens:   r.config.MaxOutputTokens,
			TopP:        r.config.TopP,
		}

		resp, err := r.provider.Chat(ctx, req)
		if err != nil {
			runCtx.FailureCount++
			if runCtx.FailureCount >= r.config.MaxFailuresPerRun {
				return fmt.Errorf("max failures exceeded: %w", err)
			}
			continue
		}

		// Update usage
		runCtx.Run.CostUSD += r.estimateCost(resp.Usage)

		// Handle response
		if resp.Content != "" {
			// Add assistant message
			msg := &store.Message{
				Role:      "assistant",
				Content:   resp.Content,
				SessionID: runCtx.Session.ID,
			}
			r.store.AddMessage(ctx, runCtx.Session.ID, msg)
			runCtx.Messages = append(runCtx.Messages, msg)

			r.publishEvent(runCtx.Run.ID, store.EventTypeTextDelta, map[string]any{
				"text": resp.Content,
			})

			runCtx.Run.Output = resp.Content
		}

		// Handle tool calls
		if len(resp.ToolCalls) > 0 {
			if err := r.handleToolCalls(ctx, runCtx, resp.ToolCalls); err != nil {
				return fmt.Errorf("tool execution failed: %w", err)
			}
			iteration++
			continue
		}

		// No tool calls, we're done
		if resp.FinishReason == "stop" || resp.FinishReason == "end_turn" {
			break
		}

		iteration++
	}

	if iteration >= maxIterations {
		return fmt.Errorf("max iterations exceeded")
	}

	return nil
}

// handleToolCalls executes tool calls and adds results to messages
func (r *Runtime) handleToolCalls(ctx context.Context, runCtx *RunContext, toolCalls []provider.ToolCall) error {
	// Add assistant message with tool calls
	msg := &store.Message{
		Role:      "assistant",
		Content:   "",
		SessionID: runCtx.Session.ID,
	}

	for _, tc := range toolCalls {
		msg.ToolCalls = append(msg.ToolCalls, store.ToolCallRef{
			ID:   tc.ID,
			Type: tc.Type,
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	r.store.AddMessage(ctx, runCtx.Session.ID, msg)
	runCtx.Messages = append(runCtx.Messages, msg)

	// Execute tool calls (potentially in parallel)
	for _, tc := range toolCalls {
		if err := r.executeToolCall(ctx, runCtx, tc); err != nil {
			// Add error result
			errorMsg := &store.Message{
				Role:       "tool",
				Content:    fmt.Sprintf("Error: %v", err),
				SessionID:  runCtx.Session.ID,
				ToolCalls:  []store.ToolCallRef{{ID: tc.ID}},
			}
			r.store.AddMessage(ctx, runCtx.Session.ID, errorMsg)
			runCtx.Messages = append(runCtx.Messages, errorMsg)

			runCtx.FailureCount++
			if runCtx.FailureCount >= r.config.MaxFailuresPerRun {
				return fmt.Errorf("max failures exceeded")
			}
		}

		runCtx.ToolCallCount++
		runCtx.Run.ToolCallCount++
	}

	return nil
}

// executeToolCall executes a single tool call
func (r *Runtime) executeToolCall(ctx context.Context, runCtx *RunContext, tc provider.ToolCall) error {
	r.publishEvent(runCtx.Run.ID, store.EventTypeToolStarted, map[string]any{
		"tool_call_id": tc.ID,
		"tool_name":    tc.Function.Name,
		"arguments":    tc.Function.Arguments,
	})

	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Execute via MCP
	result, err := r.mcpRegistry.CallTool(ctx, tc.Function.Name, args)
	if err != nil {
		r.publishEvent(runCtx.Run.ID, store.EventTypeToolFailed, map[string]any{
			"tool_call_id": tc.ID,
			"error":        err.Error(),
		})
		return err
	}

	// Build result content
	var resultText string
	for _, content := range result.Content {
		if content.Type == "text" {
			resultText += content.Text
		}
	}

	// Add tool result message
	toolMsg := &store.Message{
		Role:      "tool",
		Content:   resultText,
		SessionID: runCtx.Session.ID,
	}
	r.store.AddMessage(ctx, runCtx.Session.ID, toolMsg)
	runCtx.Messages = append(runCtx.Messages, toolMsg)

	r.publishEvent(runCtx.Run.ID, store.EventTypeToolCompleted, map[string]any{
		"tool_call_id": tc.ID,
		"output":       resultText,
	})

	return nil
}

func (r *Runtime) buildProviderMessages(runCtx *RunContext) []provider.Message {
	messages := []provider.Message{}

	// Add system message
	if r.config.SystemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: r.config.SystemPrompt,
		})
	}

	// Add conversation messages
	for _, msg := range runCtx.Messages {
		provMsg := provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		if len(msg.ToolCalls) > 0 {
			provMsg.ToolCalls = make([]provider.ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				provMsg.ToolCalls[i] = provider.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: provider.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}

		messages = append(messages, provMsg)
	}

	return messages
}

func (r *Runtime) buildProviderTools() []provider.Tool {
	mcpTools := r.mcpRegistry.ListTools()
	tools := make([]provider.Tool, len(mcpTools))

	for i, t := range mcpTools {
		tools[i] = provider.Tool{
			Type: "function",
			Function: provider.Function{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}

	return tools
}

func (r *Runtime) estimateCost(usage provider.Usage) float64 {
	// Simplified cost estimation
	// TODO: Implement provider-specific pricing
	return float64(usage.TotalTokens) * 0.00001
}

func (r *Runtime) failRun(ctx context.Context, runID string, err error) {
	run, getErr := r.store.GetRun(ctx, runID)
	if getErr != nil {
		return
	}

	run.Status = store.RunStateFailed
	run.Error = err.Error()
	now := time.Now()
	run.EndedAt = &now

	r.store.UpdateRun(ctx, run)

	r.publishEvent(runID, store.EventTypeRunFailed, map[string]any{
		"run_id": runID,
		"error":  err.Error(),
	})
}

func (r *Runtime) publishEvent(runID string, eventType string, data map[string]any) {
	event := &store.Event{
		RunID: runID,
		Type:  eventType,
		Data:  data,
	}

	r.store.AddEvent(context.Background(), runID, event)
	r.eventBus.Publish(runID, event)
}

// CreateRunRequest represents a request to create a new run
type CreateRunRequest struct {
	SessionID string         `json:"session_id,omitempty"`
	TenantID  string         `json:"tenant_id"`
	Mode      string         `json:"mode"` // interactive, autonomous
	Input     string         `json:"input"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
