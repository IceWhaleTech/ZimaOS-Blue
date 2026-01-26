// Package streaming provides Server-Sent Events (SSE) support for streaming responses.
package streaming

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Usage represents token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a chunk of a streaming response.
type StreamChunk struct {
	ID    string `json:"id,omitempty"`
	Model string `json:"model,omitempty"`
	Delta string `json:"delta"`
	Done  bool   `json:"done"`
	Usage *Usage `json:"usage,omitempty"`
}

// SSEWriter writes Server-Sent Events to an HTTP response.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer.
func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	flusher, _ := w.(http.Flusher)
	return &SSEWriter{
		w:       w,
		flusher: flusher,
	}
}

// SetHeaders sets the required headers for SSE.
func (s *SSEWriter) SetHeaders() {
	s.w.Header().Set("Content-Type", "text/event-stream")
	s.w.Header().Set("Cache-Control", "no-cache")
	s.w.Header().Set("Connection", "keep-alive")
	s.w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
}

// WriteEvent writes an event with a specific type.
func (s *SSEWriter) WriteEvent(eventType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	_, err = fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	s.flush()
	return nil
}

// WriteData writes data without an event type.
func (s *SSEWriter) WriteData(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	_, err = fmt.Fprintf(s.w, "data: %s\n\n", jsonData)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	s.flush()
	return nil
}

// WriteString writes a string as data.
func (s *SSEWriter) WriteString(str string) error {
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", str)
	if err != nil {
		return fmt.Errorf("failed to write string: %w", err)
	}

	s.flush()
	return nil
}

// WriteDone writes the [DONE] marker to signal end of stream.
func (s *SSEWriter) WriteDone() error {
	_, err := fmt.Fprint(s.w, "data: [DONE]\n\n")
	if err != nil {
		return fmt.Errorf("failed to write done: %w", err)
	}

	s.flush()
	return nil
}

// flush flushes the response writer if it supports flushing.
func (s *SSEWriter) flush() {
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// StreamHandler handles streaming responses.
type StreamHandler struct{}

// NewStreamHandler creates a new stream handler.
func NewStreamHandler() *StreamHandler {
	return &StreamHandler{}
}

// HandleStream handles a streaming response from a channel of chunks.
func (h *StreamHandler) HandleStream(w http.ResponseWriter, r *http.Request, chunks <-chan StreamChunk) {
	writer := NewSSEWriter(w)
	writer.SetHeaders()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-chunks:
			if !ok {
				// Channel closed
				writer.WriteDone()
				return
			}

			if chunk.Done {
				// Write final chunk with usage info if available
				if chunk.Usage != nil {
					writer.WriteData(chunk)
				}
				writer.WriteDone()
				return
			}

			// Write delta content
			if chunk.Delta != "" {
				writer.WriteData(chunk)
			}
		}
	}
}

// HandleStreamFunc returns an http.HandlerFunc that streams from a channel.
func (h *StreamHandler) HandleStreamFunc(getChunks func(r *http.Request) (<-chan StreamChunk, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chunks, err := getChunks(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.HandleStream(w, r, chunks)
	}
}
