package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shankgan/agent/internal/store"
)

// RegisterV1API registers the OpenAI-compatible /v1 API endpoints
func RegisterV1API(mux *http.ServeMux, rt *Runtime) {
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleChatCompletions(w, r, rt)
	})
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request, rt *Runtime) {
	var req OpenAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Extract the last user message as input
	var input string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			input = req.Messages[i].Content
			break
		}
	}

	// Create a run (each chat completion creates a new run)
	createReq := &CreateRunRequest{
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
		handleStreamingResponse(w, r, rt, run.ID, req.Model)
	} else {
		handleNonStreamingResponse(w, r, rt, run.ID, req.Model)
	}
}

func handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, rt *Runtime, runID string, model string) {
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
				// Build OpenAI response
				resp := OpenAIChatResponse{
					ID:      runID,
					Object:  "chat.completion",
					Created: run.CreatedAt.Unix(),
					Model:   model,
					Choices: []OpenAIChoice{
						{
							Index: 0,
							Message: OpenAIMessage{
								Role:    "assistant",
								Content: run.Output,
							},
							FinishReason: "stop",
						},
					},
					Usage: OpenAIUsage{
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

func handleStreamingResponse(w http.ResponseWriter, r *http.Request, rt *Runtime, runID string, model string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Subscribe to events
	eventChan := rt.eventBus.Subscribe(runID)
	defer rt.eventBus.Unsubscribe(runID, eventChan)

	chunkIndex := 0

	for {
		select {
		case <-r.Context().Done():
			return

		case event, ok := <-eventChan:
			if !ok {
				// Send [DONE]
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}

			// Convert our events to OpenAI streaming format
			var chunk *OpenAIStreamChunk

			switch event.Type {
			case store.EventTypeTextDelta:
				text, _ := event.Data["text"].(string)
				chunk = &OpenAIStreamChunk{
					ID:      runID,
					Object:  "chat.completion.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []OpenAIStreamChoice{
						{
							Index: 0,
							Delta: OpenAIDelta{
								Content: text,
							},
						},
					},
				}

			case store.EventTypeRunCompleted:
				chunk = &OpenAIStreamChunk{
					ID:      runID,
					Object:  "chat.completion.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []OpenAIStreamChoice{
						{
							Index:        0,
							Delta:        OpenAIDelta{},
							FinishReason: "stop",
						},
					},
				}

			case store.EventTypeRunFailed, store.EventTypeRunCancelled:
				chunk = &OpenAIStreamChunk{
					ID:      runID,
					Object:  "chat.completion.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []OpenAIStreamChoice{
						{
							Index:        0,
							Delta:        OpenAIDelta{},
							FinishReason: "error",
						},
					},
				}

			case store.EventTypeRunPaused:
				chunk = &OpenAIStreamChunk{
					ID:      runID,
					Object:  "chat.completion.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []OpenAIStreamChoice{
						{
							Index: 0,
							Delta: OpenAIDelta{
								Content: "\n[Run paused. Use /runs/{id}/resume to continue.]\n",
							},
							FinishReason: "",
						},
					},
				}

			case store.EventTypeRunResumed:
				chunk = &OpenAIStreamChunk{
					ID:      runID,
					Object:  "chat.completion.chunk",
					Created: event.Timestamp.Unix(),
					Model:   model,
					Choices: []OpenAIStreamChoice{
						{
							Index: 0,
							Delta: OpenAIDelta{
								Content: "\n[Run resumed.]\n",
							},
							FinishReason: "",
						},
					},
				}
			}

			if chunk != nil {
				data, err := json.Marshal(chunk)
				if err != nil {
					return
				}

				fmt.Fprintf(w, "data: %s\n\n", string(data))
				flusher.Flush()
				chunkIndex++

				// If run is done, close
				if event.Type == store.EventTypeRunCompleted ||
					event.Type == store.EventTypeRunFailed ||
					event.Type == store.EventTypeRunCancelled {
					fmt.Fprintf(w, "data: [DONE]\n\n")
					flusher.Flush()
					return
				}
			}
		}
	}
}

// OpenAI API types

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
