package streaming

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/shankarg87/agent/internal/store"
)

// SetSSEHeaders sets the required headers for Server-Sent Events streaming
func SetSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// GetFlusher attempts to get an http.Flusher from the ResponseWriter
func GetFlusher(w http.ResponseWriter) (http.Flusher, bool) {
	flusher, ok := w.(http.Flusher)
	return flusher, ok
}

// WriteSSEEvent writes an event in Server-Sent Events format
// Format: event: <type>\ndata: <json>\n\n
func WriteSSEEvent(w http.ResponseWriter, event *store.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// SSE format: event: <type>\ndata: <json>\n\n
	fmt.Fprintf(w, "event: %s\n", event.Type)
	fmt.Fprintf(w, "data: %s\n\n", string(data))

	return nil
}

// WriteSSEData writes arbitrary data in SSE format (without event type)
// Format: data: <content>\n\n
func WriteSSEData(w http.ResponseWriter, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
	return nil
}

// WriteSSEDone writes the [DONE] marker to signal end of stream
func WriteSSEDone(w http.ResponseWriter) {
	fmt.Fprintf(w, "data: [DONE]\n\n")
}
