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

// RegisterOpenAIResponsesAPI registers the OpenAI Responses API /v1/responses endpoint
func RegisterOpenAIResponsesAPI(mux *http.ServeMux, rt *runtime.Runtime) {
	mux.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleResponses(w, r, rt)
	})
}

func handleResponses(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime) {
	var req types.ResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Extract the last user message as input (same as chat completions)
	var input string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			input = req.Messages[i].Content
			break
		}
	}

	// Create a run (each response creates a new run)
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
		handleResponsesStreaming(w, r, rt, run.ID, req.Model)
	} else {
		handleResponsesNonStreaming(w, r, rt, run.ID, req.Model)
	}
}

func handleResponsesNonStreaming(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string, model string) {
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
				// Build OpenAI Responses API response
				resp := types.ResponsesResponse{
					ID:      runID,
					Object:  "response",
					Created: run.CreatedAt.Unix(),
					Model:   model,
					Choices: []types.ResponseChoice{
						{
							Index: 0,
							Message: types.OpenAIMessage{
								Role:    "assistant",
								Content: run.Output,
							},
							FinishReason: "stop",
						},
					},
					Usage: types.OpenAIUsage{
						PromptTokens:     0, // TODO: track tokens
						CompletionTokens: 0,
						TotalTokens:      0,
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

func handleResponsesStreaming(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string, model string) {
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

	chunkIndex := 0

	for {
		select {
		case <-r.Context().Done():
			return

		case event, ok := <-eventChan:
			if !ok {
				// Send [DONE]
				streaming.WriteSSEDone(w)
				flusher.Flush()
				return
			}

			// Convert our events to OpenAI Responses streaming format
			var chunk *types.ResponsesStreamChunk

			switch event.Type {
			case store.EventTypeTextDelta:
				text, _ := event.Data["text"].(string)
				chunk = &types.ResponsesStreamChunk{
					ID:      runID,
					Object:  "response.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []types.ResponsesStreamChoice{
						{
							Index: 0,
							Delta: types.OpenAIDelta{
								Content: text,
							},
						},
					},
				}

			case store.EventTypeRunCompleted:
				chunk = &types.ResponsesStreamChunk{
					ID:      runID,
					Object:  "response.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []types.ResponsesStreamChoice{
						{
							Index:        0,
							Delta:        types.OpenAIDelta{},
							FinishReason: "stop",
						},
					},
				}

			case store.EventTypeRunFailed, store.EventTypeRunCancelled:
				chunk = &types.ResponsesStreamChunk{
					ID:      runID,
					Object:  "response.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []types.ResponsesStreamChoice{
						{
							Index:        0,
							Delta:        types.OpenAIDelta{},
							FinishReason: "error",
						},
					},
				}

			case store.EventTypeRunPaused:
				chunk = &types.ResponsesStreamChunk{
					ID:      runID,
					Object:  "response.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []types.ResponsesStreamChoice{
						{
							Index: 0,
							Delta: types.OpenAIDelta{
								Content: "\n[Run paused. Use /runs/{id}/resume to continue.]\n",
							},
							FinishReason: "",
						},
					},
				}

			case store.EventTypeRunResumed:
				chunk = &types.ResponsesStreamChunk{
					ID:      runID,
					Object:  "response.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []types.ResponsesStreamChoice{
						{
							Index: 0,
							Delta: types.OpenAIDelta{
								Content: "\n[Run resumed.]\n",
							},
							FinishReason: "",
						},
					},
				}
			}

			if chunk != nil {
				if err := streaming.WriteSSEData(w, chunk); err != nil {
					return
				}
				flusher.Flush()
				chunkIndex++

				// If run is done, close
				if event.Type == store.EventTypeRunCompleted ||
					event.Type == store.EventTypeRunFailed ||
					event.Type == store.EventTypeRunCancelled {
					streaming.WriteSSEDone(w)
					flusher.Flush()
					return
				}
			}
		}
	}
}
