package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shankarg87/agent/api/streaming"
	"github.com/shankarg87/agent/internal/runtime"
	"github.com/shankarg87/agent/internal/store"
)

// RegisterRunsAPI registers the /runs API endpoints
func RegisterRunsAPI(mux *http.ServeMux, rt *runtime.Runtime) {
	mux.HandleFunc("/runs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleCreateRun(w, r, rt)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/runs/", func(w http.ResponseWriter, r *http.Request) {
		// Extract run ID from path
		path := strings.TrimPrefix(r.URL.Path, "/runs/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "Run ID required", http.StatusBadRequest)
			return
		}

		runID := parts[0]

		// Route based on path
		if len(parts) == 1 {
			// /runs/{id}
			switch r.Method {
			case http.MethodGet:
				handleGetRun(w, r, rt, runID)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		} else if len(parts) == 2 {
			switch parts[1] {
			case "events":
				// /runs/{id}/events
				handleGetRunEvents(w, r, rt, runID)
			case "cancel":
				// /runs/{id}/cancel
				if r.Method == http.MethodPost {
					handleCancelRun(w, r, rt, runID)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case "pause":
				// /runs/{id}/pause
				if r.Method == http.MethodPost {
					handlePauseRun(w, r, rt, runID)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case "resume":
				// /runs/{id}/resume
				if r.Method == http.MethodPost {
					handleResumeRun(w, r, rt, runID)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case "approve":
				// /runs/{id}/approve
				if r.Method == http.MethodPost {
					handleApproveToolCall(w, r, rt, runID)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			default:
				http.Error(w, "Not found", http.StatusNotFound)
			}
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})
}

func handleCreateRun(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime) {
	var req runtime.CreateRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Mode == "" {
		req.Mode = "interactive"
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}

	run, err := rt.CreateRun(r.Context(), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create run: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(run)
}

func handleGetRun(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string) {
	run, err := rt.GetRun(r.Context(), runID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, "Run not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to get run: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

func handleGetRunEvents(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string) {
	// Check if run exists
	_, err := rt.GetRun(r.Context(), runID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, "Run not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to get run: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Set SSE headers
	streaming.SetSSEHeaders(w)

	flusher, ok := streaming.GetFlusher(w)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Get historical events first
	events, err := rt.GetEvents(r.Context(), runID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get events: %v", err), http.StatusInternalServerError)
		return
	}

	// Send historical events
	for _, event := range events {
		if err := streaming.WriteSSEEvent(w, event); err != nil {
			return
		}
		flusher.Flush()
	}

	// Subscribe to new events
	eventChan := rt.SubscribeToEvents(runID)
	defer rt.UnsubscribeFromEvents(runID, eventChan)

	// Stream new events
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, run is done
				return
			}
			if err := streaming.WriteSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func handleCancelRun(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string) {
	if err := rt.CancelRun(r.Context(), runID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel run: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "cancelled",
		"run_id": runID,
	})
}

func handlePauseRun(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string) {
	if err := rt.PauseRun(r.Context(), runID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to pause run: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "paused",
		"run_id": runID,
	})
}

func handleResumeRun(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string) {
	if err := rt.ResumeRun(r.Context(), runID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to resume run: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "resumed",
		"run_id": runID,
	})
}

func handleApproveToolCall(w http.ResponseWriter, r *http.Request, rt *runtime.Runtime, runID string) {
	var req ApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := rt.ApproveToolCall(r.Context(), runID, req.Approved, req.Reason); err != nil {
		http.Error(w, fmt.Sprintf("Failed to process approval: %v", err), http.StatusInternalServerError)
		return
	}

	status := "approved"
	if !req.Approved {
		status = "denied"
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":   status,
		"run_id":   runID,
		"approved": req.Approved,
		"reason":   req.Reason,
	})
}

// ApprovalRequest represents a tool approval request
type ApprovalRequest struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}

// RunResponse is the API response for a run
type RunResponse struct {
	ID            string         `json:"id"`
	SessionID     string         `json:"session_id"`
	Status        string         `json:"status"`
	Mode          string         `json:"mode"`
	Input         string         `json:"input,omitempty"`
	Output        string         `json:"output,omitempty"`
	Error         string         `json:"error,omitempty"`
	ToolCallCount int            `json:"tool_call_count"`
	CostUSD       float64        `json:"cost_usd"`
	CreatedAt     time.Time      `json:"created_at"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	EndedAt       *time.Time     `json:"ended_at,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}
