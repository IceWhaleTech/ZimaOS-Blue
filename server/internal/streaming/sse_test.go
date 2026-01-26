package streaming

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test SSEWriter creation
func TestNewSSEWriter(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)
	if writer == nil {
		t.Fatal("expected writer, got nil")
	}
}

// Test SSEWriter WriteEvent
func TestSSEWriterWriteEvent(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	err := writer.WriteEvent("message", map[string]string{"content": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: message") {
		t.Error("expected event type in output")
	}
	if !strings.Contains(body, `"content":"hello"`) {
		t.Error("expected data in output")
	}
}

// Test SSEWriter WriteData
func TestSSEWriterWriteData(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	err := writer.WriteData(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "data:") {
		t.Error("expected data prefix in output")
	}
	if !strings.Contains(body, `"key":"value"`) {
		t.Error("expected JSON data in output")
	}
}

// Test SSEWriter WriteString
func TestSSEWriterWriteString(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	err := writer.WriteString("hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: hello world") {
		t.Error("expected string data in output")
	}
}

// Test SSEWriter WriteDone
func TestSSEWriterWriteDone(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	err := writer.WriteDone()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: [DONE]") {
		t.Error("expected [DONE] marker in output")
	}
}

// Test SSEWriter SetHeaders
func TestSSEWriterSetHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	writer.SetHeaders()

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Error("expected Content-Type: text/event-stream")
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Error("expected Cache-Control: no-cache")
	}
	if w.Header().Get("Connection") != "keep-alive" {
		t.Error("expected Connection: keep-alive")
	}
}

// Test StreamHandler creation
func TestNewStreamHandler(t *testing.T) {
	handler := NewStreamHandler()
	if handler == nil {
		t.Fatal("expected handler, got nil")
	}
}

// Test StreamHandler HandleStream
func TestStreamHandlerHandleStream(t *testing.T) {
	handler := NewStreamHandler()

	// Create a channel to send chunks
	chunks := make(chan StreamChunk, 3)
	chunks <- StreamChunk{Delta: "Hello", Done: false}
	chunks <- StreamChunk{Delta: " World", Done: false}
	chunks <- StreamChunk{Delta: "", Done: true}
	close(chunks)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/stream", nil)

	handler.HandleStream(w, req, chunks)

	body := w.Body.String()
	if !strings.Contains(body, "Hello") {
		t.Error("expected 'Hello' in output")
	}
	if !strings.Contains(body, "World") {
		t.Error("expected 'World' in output")
	}
	if !strings.Contains(body, "[DONE]") {
		t.Error("expected [DONE] marker in output")
	}
}

// Test StreamHandler with context cancellation
func TestStreamHandlerContextCancellation(t *testing.T) {
	handler := NewStreamHandler()

	// Create a channel that won't close
	chunks := make(chan StreamChunk)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/stream", nil)

	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	// Cancel after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	handler.HandleStream(w, req, chunks)

	// Should return without hanging
}

// Test StreamChunk struct
func TestStreamChunk(t *testing.T) {
	chunk := StreamChunk{
		ID:    "chunk-123",
		Model: "gpt-4",
		Delta: "Hello",
		Done:  false,
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	if chunk.ID != "chunk-123" {
		t.Errorf("expected ID 'chunk-123', got '%s'", chunk.ID)
	}
	if chunk.Delta != "Hello" {
		t.Errorf("expected Delta 'Hello', got '%s'", chunk.Delta)
	}
	if chunk.Done {
		t.Error("expected Done to be false")
	}
	if chunk.Usage.TotalTokens != 15 {
		t.Errorf("expected TotalTokens 15, got %d", chunk.Usage.TotalTokens)
	}
}

// Test StreamChunk JSON serialization
func TestStreamChunkJSON(t *testing.T) {
	chunk := StreamChunk{
		ID:    "chunk-123",
		Delta: "Hello",
		Done:  false,
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded StreamChunk
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID != chunk.ID {
		t.Errorf("expected ID '%s', got '%s'", chunk.ID, decoded.ID)
	}
}

// Test SSE endpoint integration
func TestSSEEndpointIntegration(t *testing.T) {
	handler := NewStreamHandler()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunks := make(chan StreamChunk, 2)
		chunks <- StreamChunk{Delta: "Test", Done: false}
		chunks <- StreamChunk{Delta: "", Done: true}
		close(chunks)

		handler.HandleStream(w, r, chunks)
	}))
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}
}

// Test SSEWriter with Flusher
func TestSSEWriterWithFlusher(t *testing.T) {
	// httptest.ResponseRecorder implements http.Flusher
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	err := writer.WriteData(map[string]string{"test": "data"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify flush was called (no panic)
}

// Test multiple events
func TestSSEWriterMultipleEvents(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	events := []struct {
		eventType string
		data      interface{}
	}{
		{"start", map[string]string{"status": "starting"}},
		{"progress", map[string]int{"percent": 50}},
		{"complete", map[string]string{"status": "done"}},
	}

	for _, e := range events {
		if err := writer.WriteEvent(e.eventType, e.data); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: start") {
		t.Error("expected start event")
	}
	if !strings.Contains(body, "event: progress") {
		t.Error("expected progress event")
	}
	if !strings.Contains(body, "event: complete") {
		t.Error("expected complete event")
	}
}
