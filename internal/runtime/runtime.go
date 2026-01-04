package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shankarg87/agent/internal/config"
	"github.com/shankarg87/agent/internal/events"
	"github.com/shankarg87/agent/internal/logging"
	"github.com/shankarg87/agent/internal/mcp"
	"github.com/shankarg87/agent/internal/metrics"
	"github.com/shankarg87/agent/internal/provider"
	"github.com/shankarg87/agent/internal/store"
)

// Runtime manages the agent execution environment
type Runtime struct {
	configManager *config.ConfigManager
	store         store.Store
	eventBus      *events.EventBus
	provider      provider.Provider
	mcpRegistry   *mcp.Registry
	metrics       *metrics.AgentMetrics
	logger        *logging.SimpleLogger

	mu            sync.RWMutex
	activeRuns    map[string]*RunContext
	cancellations map[string]context.CancelFunc
}

// RunContext holds the execution context for a single run
type RunContext struct {
	Run           *store.Run
	Session       *store.Session
	Messages      []*store.Message
	Config        *config.AgentConfig // Snapshot of config at run creation time
	Cancel        context.CancelFunc
	ToolCallCount int
	FailureCount  int

	// Pause/resume state
	mu           sync.RWMutex
	isPaused     bool
	pauseSignal  chan struct{}
	resumeSignal chan struct{}
}

// NewRuntime creates a new runtime instance
func NewRuntime(
	configManager *config.ConfigManager,
	st store.Store,
	eb *events.EventBus,
	prov provider.Provider,
	mcpReg *mcp.Registry,
	met *metrics.AgentMetrics,
) *Runtime {
	logger := logging.VerboseLogger("runtime")
	logger.Verbose("Creating new runtime instance")

	return &Runtime{
		configManager: configManager,
		store:         st,
		eventBus:      eb,
		provider:      prov,
		mcpRegistry:   mcpReg,
		metrics:       met,
		logger:        logger,
		activeRuns:    make(map[string]*RunContext),
		cancellations: make(map[string]context.CancelFunc),
	}
}

// CreateRun creates a new run
func (r *Runtime) CreateRun(ctx context.Context, req *CreateRunRequest) (*store.Run, error) {
	r.logger.LogRunStart("", "", req)
	start := time.Now()

	// Record run creation metrics
	if r.metrics != nil {
		r.metrics.RunCreated(ctx, req.TenantID, req.Mode)
		defer func() {
			r.metrics.SetActiveRuns(ctx, req.TenantID, int64(r.getActiveRunCountForTenant(req.TenantID)))
		}()
	}

	// Get current configuration snapshot for this run
	r.logger.Verbose("Getting current configuration snapshot")
	currentConfig := r.configManager.GetAgentConfig()
	r.logger.Verbose("Configuration snapshot obtained",
		"profile_name", currentConfig.ProfileName,
		"profile_version", currentConfig.ProfileVersion,
		"max_tool_calls", currentConfig.MaxToolCalls,
	)

	// Get or create session
	var session *store.Session
	if req.SessionID != "" {
		r.logger.Verbose("Getting existing session", "session_id", req.SessionID)
		s, err := r.store.GetSession(ctx, req.SessionID)
		if err != nil {
			r.logger.Error("Failed to get session",
				"session_id", req.SessionID,
				"error", err,
			)
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
		session = s
		r.logger.Verbose("Existing session retrieved", "session_id", session.ID)
	} else {
		r.logger.Verbose("Creating new session")
		session = &store.Session{
			ID:          uuid.New().String(),
			TenantID:    req.TenantID,
			ProfileName: currentConfig.ProfileName,
			Metadata:    req.Metadata,
		}
		if err := r.store.CreateSession(ctx, session); err != nil {
			r.logger.Error("Failed to create session", "error", err)
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		r.logger.Verbose("New session created", "session_id", session.ID)
	}

	// Create run
	r.logger.Verbose("Creating new run")
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
		r.logger.Error("Failed to create run", "run_id", run.ID, "error", err)
		return nil, fmt.Errorf("failed to create run: %w", err)
	}

	r.logger.Info("Run created",
		"run_id", run.ID,
		"session_id", session.ID,
		"tenant_id", req.TenantID,
		"mode", req.Mode,
	)

	// Add user message if input provided
	if req.Input != "" {
		r.logger.Verbose("Adding user message", "run_id", run.ID, "input_length", len(req.Input))
		msg := &store.Message{
			Role:      "user",
			Content:   req.Input,
			SessionID: session.ID,
		}
		if err := r.store.AddMessage(ctx, session.ID, msg); err != nil {
			r.logger.Error("Failed to add message", "run_id", run.ID, "error", err)
			return nil, fmt.Errorf("failed to add message: %w", err)
		}
		r.logger.Verbose("User message added", "run_id", run.ID)
	}

	// Start execution
	r.logger.Verbose("Starting run execution", "run_id", run.ID)
	go r.executeRun(context.Background(), run.ID)

	r.logger.LogPerformance("create_run", time.Since(start), map[string]interface{}{
		"run_id": run.ID,
		"mode":   req.Mode,
	})

	return run, nil
}

// GetRun retrieves a run by ID
func (r *Runtime) GetRun(ctx context.Context, runID string) (*store.Run, error) {
	r.logger.Verbose("Getting run", "run_id", runID)
	start := time.Now()

	run, err := r.store.GetRun(ctx, runID)
	if err != nil {
		r.logger.Error("Failed to get run", "run_id", runID, "error", err)
		return nil, err
	}

	r.logger.LogPerformance("get_run", time.Since(start), map[string]interface{}{
		"run_id": runID,
		"status": run.Status,
	})

	return run, nil
}

// CancelRun cancels a running run
func (r *Runtime) CancelRun(ctx context.Context, runID string) error {
	r.logger.Info("Canceling run", "run_id", runID)

	r.mu.Lock()
	cancel, ok := r.cancellations[runID]
	r.mu.Unlock()

	if !ok {
		r.logger.Warn("Run not found or not running for cancellation", "run_id", runID)
		return fmt.Errorf("run not found or not running")
	}

	r.logger.Verbose("Triggering cancellation", "run_id", runID)
	cancel()

	// Update run status
	run, err := r.store.GetRun(ctx, runID)
	if err != nil {
		r.logger.Error("Failed to get run for cancellation", "run_id", runID, "error", err)
		return err
	}

	r.logger.LogStateTransition(runID, string(run.Status), string(store.RunStateCancelled), "user_cancellation")
	run.Status = store.RunStateCancelled
	now := time.Now()
	run.EndedAt = &now

	if err := r.store.UpdateRun(ctx, run); err != nil {
		r.logger.Error("Failed to update run status after cancellation", "run_id", runID, "error", err)
		return err
	}

	r.logger.Info("Run cancelled successfully", "run_id", runID)
	return nil
}

// PauseRun pauses a running run
func (r *Runtime) PauseRun(ctx context.Context, runID string) error {
	r.logger.Info("Pausing run", "run_id", runID)

	r.mu.Lock()
	runCtx, ok := r.activeRuns[runID]
	r.mu.Unlock()

	if !ok {
		r.logger.Warn("Run not found or not active for pausing", "run_id", runID)
		return fmt.Errorf("run not found or not active")
	}

	runCtx.mu.Lock()
	defer runCtx.mu.Unlock()

	if runCtx.isPaused {
		r.logger.Warn("Run is already paused", "run_id", runID)
		return fmt.Errorf("run is already paused")
	}

	// Update run status
	run, err := r.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}

	if run.Status != store.RunStateRunning {
		return fmt.Errorf("run is not in running state, current status: %s", run.Status)
	}

	run.Status = store.RunStatePaused
	if err := r.store.UpdateRun(ctx, run); err != nil {
		return err
	}

	runCtx.isPaused = true
	runCtx.Run = run

	// Signal pause to the agent loop
	select {
	case runCtx.pauseSignal <- struct{}{}:
	default:
		// Channel already has a signal
	}

	// Publish pause event
	r.publishEvent(runID, store.EventTypeRunPaused, map[string]any{
		"run_id": runID,
		"reason": "user_requested",
	})

	return nil
}

// ResumeRun resumes a paused run
func (r *Runtime) ResumeRun(ctx context.Context, runID string) error {
	r.mu.Lock()
	runCtx, ok := r.activeRuns[runID]
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("run not found or not active")
	}

	runCtx.mu.Lock()
	defer runCtx.mu.Unlock()

	if !runCtx.isPaused {
		return fmt.Errorf("run is not paused")
	}

	// Update run status
	run, err := r.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}

	if run.Status != store.RunStatePaused {
		return fmt.Errorf("run is not in paused state, current status: %s", run.Status)
	}

	run.Status = store.RunStateRunning
	if err := r.store.UpdateRun(ctx, run); err != nil {
		return err
	}

	runCtx.isPaused = false
	runCtx.Run = run

	// Signal resume to the agent loop
	select {
	case runCtx.resumeSignal <- struct{}{}:
	default:
		// Channel already has a signal
	}

	// Publish resume event
	r.publishEvent(runID, store.EventTypeRunResumed, map[string]any{
		"run_id": runID,
	})

	return nil
}

// ApproveToolCall approves a tool call that is waiting for user consent
func (r *Runtime) ApproveToolCall(ctx context.Context, runID string, approved bool, reason string) error {
	r.logger.Info("Processing tool approval",
		"run_id", runID,
		"approved", approved,
		"reason", reason,
	)

	r.mu.Lock()
	runCtx, ok := r.activeRuns[runID]
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("run not found or not active")
	}

	runCtx.mu.Lock()
	defer runCtx.mu.Unlock()

	if !runCtx.isPaused || runCtx.Run.Status != store.RunStatePausedCheckpoint {
		return fmt.Errorf("run is not paused for approval, current status: %s", runCtx.Run.Status)
	}

	if !approved {
		// User denied the tool execution - cancel the run
		r.logger.Info("Tool execution denied by user",
			"run_id", runID,
			"reason", reason,
		)

		runCtx.Run.Status = store.RunStateCancelled
		runCtx.Run.Error = fmt.Sprintf("Tool execution denied by user: %s", reason)
		now := time.Now()
		runCtx.Run.EndedAt = &now

		if err := r.store.UpdateRun(ctx, runCtx.Run); err != nil {
			return err
		}

		// Emit cancellation event
		r.publishEvent(runID, store.EventTypeRunCancelled, map[string]any{
			"reason":      "tool_execution_denied",
			"user_reason": reason,
		})

		// Signal cancellation (will unblock pauseForApproval)
		cancel := r.cancellations[runID]
		if cancel != nil {
			cancel()
		}

		return nil
	}

	// User approved - resume execution
	r.logger.Info("Tool execution approved by user",
		"run_id", runID,
		"reason", reason,
	)

	// Emit approval event
	r.publishEvent(runID, store.EventTypeRunResumed, map[string]any{
		"reason":      "tool_approved",
		"user_reason": reason,
	})

	runCtx.isPaused = false

	// Signal resume to unblock pauseForApproval
	select {
	case runCtx.resumeSignal <- struct{}{}:
		r.logger.Verbose("Resume signal sent", "run_id", runID)
	default:
		r.logger.Verbose("Resume signal channel already has value", "run_id", runID)
	}

	return nil
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

	// Apply timeout if configured (get current config)
	timeoutConfig := r.configManager.GetAgentConfig()
	if timeoutConfig.MaxRunTimeSeconds > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(timeoutConfig.MaxRunTimeSeconds)*time.Second)
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

	// Get current configuration snapshot for this execution
	currentConfig := r.configManager.GetAgentConfig()

	runCtx := &RunContext{
		Run:          run,
		Session:      session,
		Messages:     messages,
		Config:       currentConfig, // Snapshot config at run start
		Cancel:       cancel,
		pauseSignal:  make(chan struct{}, 1),
		resumeSignal: make(chan struct{}, 1),
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
	maxIterations := runCtx.Config.MaxToolCalls
	iteration := 0

	for iteration < maxIterations {
		// Check for pause signal
		select {
		case <-runCtx.pauseSignal:
			// Handle pause - wait for resume signal
			r.publishEvent(runCtx.Run.ID, store.EventTypeTextDelta, map[string]any{
				"text": "\n[Run paused by user. Use /runs/{id}/resume to continue...]\n",
			})

			// Wait for resume or context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-runCtx.resumeSignal:
				// Continue execution
				r.publishEvent(runCtx.Run.ID, store.EventTypeTextDelta, map[string]any{
					"text": "\n[Run resumed by user. Continuing...]\n",
				})
			}
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Build messages for LLM
		providerMessages := r.buildProviderMessages(runCtx)

		// Build tools for LLM (filtered by agent configuration)
		tools := r.buildProviderTools(runCtx)

		// Call LLM
		req := &provider.ChatRequest{
			Messages:    providerMessages,
			Tools:       tools,
			Temperature: runCtx.Config.Temperature,
			MaxTokens:   runCtx.Config.MaxOutputTokens,
			TopP:        runCtx.Config.TopP,
		}

		resp, err := r.provider.Chat(ctx, req)
		if err != nil {
			runCtx.FailureCount++
			if runCtx.FailureCount >= runCtx.Config.MaxFailuresPerRun {
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
				Role:      "tool",
				Content:   fmt.Sprintf("Error: %v", err),
				SessionID: runCtx.Session.ID,
				ToolCalls: []store.ToolCallRef{{ID: tc.ID}},
			}
			r.store.AddMessage(ctx, runCtx.Session.ID, errorMsg)
			runCtx.Messages = append(runCtx.Messages, errorMsg)

			runCtx.FailureCount++
			if runCtx.FailureCount >= runCtx.Config.MaxFailuresPerRun {
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

	// Find tool configuration for authorization checks
	var toolConfig *config.ToolConfig
	for i := range runCtx.Config.Tools {
		// Check if this tool call matches any configured tool server
		tool, err := r.mcpRegistry.GetTool(tc.Function.Name)
		if err == nil && tool.ServerName == runCtx.Config.Tools[i].ServerName {
			toolConfig = &runCtx.Config.Tools[i]
			break
		}
	}

	// Check if tool requires user consent
	if toolConfig != nil {
		requiresConsent, reason := r.mcpRegistry.RequiresUserConsent(tc.Function.Name, args, toolConfig)
		if requiresConsent {
			r.logger.Warn("Tool requires user consent",
				"tool", tc.Function.Name,
				"reason", reason,
				"run_id", runCtx.Run.ID,
			)

			// In daemon mode with auto-approve, log and continue
			if runCtx.Config.AutoApproveInDaemon && runCtx.Run.Mode == "autonomous" {
				r.logger.Info("Auto-approving tool in daemon mode",
					"tool", tc.Function.Name,
					"run_id", runCtx.Run.ID,
				)
			} else {
				// Pause execution and wait for user approval
				return r.pauseForApproval(ctx, runCtx, tc, reason)
			}
		}
	}

	// Execute via MCP with tool configuration
	result, err := r.mcpRegistry.CallTool(ctx, tc.Function.Name, args, toolConfig)
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

	// Apply redaction if configured
	if toolConfig != nil && toolConfig.Redaction.Outputs {
		resultText = "[REDACTED]"
		r.logger.Info("Tool output redacted",
			"tool", tc.Function.Name,
			"run_id", runCtx.Run.ID,
		)
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

// pauseForApproval pauses execution and waits for user approval for a tool call
func (r *Runtime) pauseForApproval(ctx context.Context, runCtx *RunContext, tc provider.ToolCall, reason string) error {
	r.logger.Info("Pausing run for tool approval",
		"run_id", runCtx.Run.ID,
		"tool", tc.Function.Name,
		"reason", reason,
	)

	// Update run status to paused_checkpoint
	runCtx.Run.Status = store.RunStatePausedCheckpoint
	r.store.UpdateRun(ctx, runCtx.Run)

	// Set pause state
	runCtx.mu.Lock()
	runCtx.isPaused = true
	if runCtx.pauseSignal == nil {
		runCtx.pauseSignal = make(chan struct{})
	}
	if runCtx.resumeSignal == nil {
		runCtx.resumeSignal = make(chan struct{})
	}
	runCtx.mu.Unlock()

	// Emit checkpoint event for user interaction
	r.publishEvent(runCtx.Run.ID, store.EventTypeCheckpointRequired, map[string]any{
		"tool_call_id":      tc.ID,
		"tool_name":         tc.Function.Name,
		"reason":            reason,
		"prompt":            fmt.Sprintf("Do you approve executing '%s'? %s", tc.Function.Name, reason),
		"tool_arguments":    tc.Function.Arguments,
		"approval_required": true,
		"approval_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"approved": map[string]any{
					"type":        "boolean",
					"description": "Whether to approve the tool execution",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Optional reason for the decision",
				},
			},
			"required": []string{"approved"},
		},
	})

	// Emit pause event
	r.publishEvent(runCtx.Run.ID, store.EventTypeRunPaused, map[string]any{
		"reason":    "tool_approval_required",
		"tool_name": tc.Function.Name,
	})

	// Wait for resume signal or context cancellation
	select {
	case <-runCtx.resumeSignal:
		r.logger.Info("Run resumed after approval",
			"run_id", runCtx.Run.ID,
			"tool", tc.Function.Name,
		)

		// Update run status back to running
		runCtx.Run.Status = store.RunStateRunning
		r.store.UpdateRun(ctx, runCtx.Run)

		// Emit resume event
		r.publishEvent(runCtx.Run.ID, store.EventTypeRunResumed, map[string]any{
			"reason":    "tool_approved",
			"tool_name": tc.Function.Name,
		})

		// Clear pause state
		runCtx.mu.Lock()
		runCtx.isPaused = false
		runCtx.mu.Unlock()

		return nil // Continue with tool execution

	case <-ctx.Done():
		r.logger.Info("Run cancelled while waiting for approval",
			"run_id", runCtx.Run.ID,
			"tool", tc.Function.Name,
		)

		// Update run status to cancelled
		runCtx.Run.Status = store.RunStateCancelled
		r.store.UpdateRun(ctx, runCtx.Run)

		return ctx.Err()
	}
}

func (r *Runtime) buildProviderMessages(runCtx *RunContext) []provider.Message {
	messages := []provider.Message{}

	// Add system message
	if runCtx.Config.SystemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: runCtx.Config.SystemPrompt,
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

func (r *Runtime) buildProviderTools(runCtx *RunContext) []provider.Tool {
	// Use filtered tools based on agent configuration
	mcpTools := r.mcpRegistry.ListToolsFiltered(runCtx.Config.Tools)
	tools := make([]provider.Tool, len(mcpTools))

	r.logger.Info("Building provider tools with filtering",
		"total_tools", len(mcpTools),
		"run_id", runCtx.Run.ID,
	)

	for i, t := range mcpTools {
		tools[i] = provider.Tool{
			Type: "function",
			Function: provider.Function{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}

		r.logger.Verbose("Tool included in LLM schema",
			"tool", t.Name,
			"server", t.ServerName,
			"run_id", runCtx.Run.ID,
		)
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

	// Record event bus metrics
	if r.metrics != nil {
		r.metrics.EventBusEvent(context.Background(), eventType, runID)
	}

	r.eventBus.Publish(runID, event)
}

// getActiveRunCountForTenant returns the number of active runs for a specific tenant
func (r *Runtime) getActiveRunCountForTenant(tenantID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, runCtx := range r.activeRuns {
		if runCtx.Run.TenantID == tenantID {
			count++
		}
	}
	return count
}

// GetEvents retrieves events for a run from the store
func (r *Runtime) GetEvents(ctx context.Context, runID string) ([]*store.Event, error) {
	return r.store.GetEvents(ctx, runID)
}

// SubscribeToEvents subscribes to events for a run
func (r *Runtime) SubscribeToEvents(runID string) <-chan *store.Event {
	return r.eventBus.Subscribe(runID)
}

// UnsubscribeFromEvents unsubscribes from events for a run
func (r *Runtime) UnsubscribeFromEvents(runID string, ch <-chan *store.Event) {
	r.eventBus.Unsubscribe(runID, ch)
}

// CreateRunRequest represents a request to create a new run
type CreateRunRequest struct {
	SessionID string         `json:"session_id,omitempty"`
	TenantID  string         `json:"tenant_id"`
	Mode      string         `json:"mode"` // interactive, autonomous
	Input     string         `json:"input"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
