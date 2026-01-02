package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shankarg87/agent/api/streaming"
	"github.com/shankarg87/agent/api/types"
	"github.com/shankarg87/agent/internal/runtime"
	"github.com/shankarg87/agent/internal/store"
)

// RegisterAnthropicAPI registers the Anthropic Messages API /v1/messages endpoint
func RegisterAnthropicAPI(mux *http.ServeMux, rt *runtime.Runtime) {
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleMessages(w, r, rt)
	})
}

func handleMessages(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime) {
	var req types.MessagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Extract the last user message as input
	// Anthropic messages can have Content as string or []AnthropicContent
	var input string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			// Handle both string and array content formats
			switch content := req.Messages[i].Content.(type) {
			case string:
				input = content
			case []interface{}:
				// Extract text from content blocks
				for _, block := range content {
					if blockMap, ok := block.(map[string]interface{}); ok {
						if blockMap["type"] == "text" {
							if text, ok := blockMap["text"].(string); ok {
								input += text
							}
						}
					}
				}
			}
			if input != "" {
				break
			}
		}
	}

	// Prepend system prompt if provided
	if req.System != "" {
		input = req.System + "\n\n" + input
	}

	// Create a run (each message creates a new run)
	createReq := &runtime.CreateRunRequest{
		TenantID: "default", // TODO: extract from auth
		Mode:     "interactive",
		Input:    input,
	}

	run, err := rt.CreateRun(r.Context(), createReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create run: %v", err), http.StatusInternalServerError)
		return
	}

	// Handle streaming vs non-streaming
	if req.Stream {
		handleMessagesStreaming(w, r, rt, run.ID, req.Model)
	} else {
		handleMessagesNonStreaming(w, r, rt, run.ID, req.Model)
	}
}

func handleMessagesNonStreaming(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string, model string) {
	// Poll for completion
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-timeout:
			http.Error(w, "Request timeout", http.StatusGatewayTimeout)
			return
		case <-ticker.C:
			run, err := rt.GetRun(r.Context(), runID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get run: %v", err), http.StatusInternalServerError)
				return
			}

			if run.Status == store.RunStateCompleted {
				// Build Anthropic Messages API response
				resp := types.MessagesResponse{
					ID:   runID,
					Type: "message",
					Role: "assistant",
					Content: []types.AnthropicContent{
						{
							Type: "text",
							Text: run.Output,
						},
					},
					Model:      model,
					StopReason: "end_turn",
					Usage: types.AnthropicUsage{
						InputTokens:  0, // TODO: track tokens
						OutputTokens: 0,
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return

			} else if run.Status == store.RunStateFailed {
				http.Error(w, fmt.Sprintf("Run failed: %s", run.Error), http.StatusInternalServerError)
				return

			} else if run.Status == store.RunStateCancelled {
				http.Error(w, "Run cancelled", http.StatusInternalServerError)
				return

			} else if run.Status == store.RunStatePaused || run.Status == store.RunStatePausedCheckpoint {
				http.Error(w, "Run is paused", http.StatusAccepted)
				return
			}
		}
	}
}

func handleMessagesStreaming(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string, model string) {
	// Set SSE headers
	streaming.SetSSEHeaders(w)

	flusher, ok := streaming.GetFlusher(w)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Subscribe to events
	eventChan := rt.SubscribeToEvents(runID)
	defer rt.UnsubscribeFromEvents(runID, eventChan)

	// Track if we've sent message_start
	messageStarted := false
	contentBlockIndex := 0

	for {
		select {
		case <-r.Context().Done():
			return

		case event, ok := <-eventChan:
			if !ok {
				return
			}

			// Convert our events to Anthropic streaming format
			switch event.Type {
			case store.EventTypeRunStarted:
				// Send message_start event
				if !messageStarted {
					messageStart := types.MessageStartEvent{
						Type: "message_start",
						Message: types.MessageMetadata{
							ID:    runID,
							Type:  "message",
							Role:  "assistant",
							Model: model,
							Usage: types.AnthropicUsage{
								InputTokens:  0,
								OutputTokens: 0,
							},
						},
					}
					if err := writeAnthropicSSEEvent(w, "message_start", messageStart); err != nil {
						return
					}
					flusher.Flush()
					messageStarted = true
				}

			case store.EventTypeTextDelta:
				text, _ := event.Data["text"].(string)

				// Send content_block_start on first text delta
				if contentBlockIndex == 0 {
					blockStart := types.ContentBlockStart{
						Type:  "content_block_start",
						Index: 0,
						ContentBlock: types.AnthropicContent{
							Type: "text",
							Text: "",
						},
					}
					if err := writeAnthropicSSEEvent(w, "content_block_start", blockStart); err != nil {
						return
					}
					flusher.Flush()
					contentBlockIndex++
				}

				// Send content_block_delta
				blockDelta := types.ContentBlockDelta{
					Type:  "content_block_delta",
					Index: 0,
					Delta: types.ContentDeltaData{
						Type: "text_delta",
						Text: text,
					},
				}
				if err := writeAnthropicSSEEvent(w, "content_block_delta", blockDelta); err != nil {
					return
				}
				flusher.Flush()

			case store.EventTypeToolStarted:
				// Tool execution - include as custom metadata
				toolName, _ := event.Data["tool_name"].(string)
				args, _ := event.Data["arguments"]

				toolExec := map[string]interface{}{
					"type": "agent_tool_execution",
					"tool": types.AgentToolExecution{
						ToolName:  toolName,
						Arguments: args,
						Status:    "started",
					},
				}
				if err := writeAnthropicSSEEvent(w, "agent_tool", toolExec); err != nil {
					return
				}
				flusher.Flush()

			case store.EventTypeToolCompleted:
				// Tool completion - include as custom metadata
				toolName, _ := event.Data["tool_name"].(string)
				args, _ := event.Data["arguments"]
				output, _ := event.Data["output"]

				toolExec := map[string]interface{}{
					"type": "agent_tool_execution",
					"tool": types.AgentToolExecution{
						ToolName:  toolName,
						Arguments: args,
						Output:    output,
						Status:    "completed",
					},
				}
				if err := writeAnthropicSSEEvent(w, "agent_tool", toolExec); err != nil {
					return
				}
				flusher.Flush()

			case store.EventTypeRunCompleted:
				// Send content_block_stop
				if contentBlockIndex > 0 {
					blockStop := types.ContentBlockStop{
						Type:  "content_block_stop",
						Index: 0,
					}
					if err := writeAnthropicSSEEvent(w, "content_block_stop", blockStop); err != nil {
						return
					}
					flusher.Flush()
				}

				// Send message_delta with stop_reason
				messageDelta := types.MessageDelta{
					Type: "message_delta",
					Delta: types.MessageDeltaData{
						StopReason: "end_turn",
					},
					Usage: types.AnthropicUsage{
						InputTokens:  0,
						OutputTokens: 0,
					},
				}
				if err := writeAnthropicSSEEvent(w, "message_delta", messageDelta); err != nil {
					return
				}
				flusher.Flush()

				// Send message_stop
				messageStop := types.MessageStop{
					Type: "message_stop",
				}
				if err := writeAnthropicSSEEvent(w, "message_stop", messageStop); err != nil {
					return
				}
				flusher.Flush()
				return

			case store.EventTypeRunFailed, store.EventTypeRunCancelled:
				// Send error stop reason
				messageDelta := types.MessageDelta{
					Type: "message_delta",
					Delta: types.MessageDeltaData{
						StopReason: "error",
					},
				}
				if err := writeAnthropicSSEEvent(w, "message_delta", messageDelta); err != nil {
					return
				}
				flusher.Flush()

				messageStop := types.MessageStop{
					Type: "message_stop",
				}
				if err := writeAnthropicSSEEvent(w, "message_stop", messageStop); err != nil {
					return
				}
				flusher.Flush()
				return
			}
		}
	}
}

// writeAnthropicSSEEvent writes an event in Anthropic's SSE format
func writeAnthropicSSEEvent(w http.ResponseWriter, eventType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
	return nil
}
