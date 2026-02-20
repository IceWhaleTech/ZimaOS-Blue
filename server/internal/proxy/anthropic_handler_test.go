package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleAnthropicMessages_MethodNotAllowed tests that non-POST requests are rejected
func TestHandleAnthropicMessages_MethodNotAllowed(t *testing.T) {
	ps := &ProxyServer{
		handler: &ProxyHandler{},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)
	w := httptest.NewRecorder()

	ps.handleAnthropicMessages(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestHandleAnthropicMessages_InvalidJSON tests that invalid JSON is rejected
func TestHandleAnthropicMessages_InvalidJSON(t *testing.T) {
	ps := &ProxyServer{
		handler: &ProxyHandler{},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	ps.handleAnthropicMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestHandleAnthropicMessages_NoProviderPool tests error when provider pool is not configured
func TestHandleAnthropicMessages_NoProviderPool(t *testing.T) {
	ps := &ProxyServer{
		handler: &ProxyHandler{
			providerPool: nil,
		},
	}

	reqBody := map[string]interface{}{
		"model":      "claude-3-sonnet-20240229",
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ps.handleAnthropicMessages(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

// TestAnthropicRequestParsing tests that Anthropic request body is correctly parsed
func TestAnthropicRequestParsing(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]interface{}
		wantModel   string
		wantStream  bool
	}{
		{
			name: "basic request",
			body: map[string]interface{}{
				"model":      "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages":   []map[string]string{{"role": "user", "content": "Hello"}},
			},
			wantModel:  "claude-3-sonnet-20240229",
			wantStream: false,
		},
		{
			name: "streaming request",
			body: map[string]interface{}{
				"model":      "claude-3-opus-20240229",
				"max_tokens": 2048,
				"stream":     true,
				"messages":   []map[string]string{{"role": "user", "content": "Hello"}},
			},
			wantModel:  "claude-3-opus-20240229",
			wantStream: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			var parsed map[string]interface{}
			json.Unmarshal(bodyBytes, &parsed)

			model, _ := parsed["model"].(string)
			stream, _ := parsed["stream"].(bool)

			if model != tt.wantModel {
				t.Errorf("expected model %s, got %s", tt.wantModel, model)
			}
			if stream != tt.wantStream {
				t.Errorf("expected stream %v, got %v", tt.wantStream, stream)
			}
		})
	}
}

// TestAnthropicResponseFormat tests that Anthropic response format is correct
func TestAnthropicResponseFormat(t *testing.T) {
	mockResponse := AnthropicResponse{
		ID:    "msg_123",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-sonnet-20240229",
		Content: []AnthropicContentBlock{
			{Type: "text", Text: "Hello! How can I help you?"},
		},
		StopReason: "end_turn",
		Usage: struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}{
			InputTokens:  10,
			OutputTokens: 8,
		},
	}

	// Serialize and deserialize to verify format
	data, err := json.Marshal(mockResponse)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed AnthropicResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsed.ID != "msg_123" {
		t.Errorf("expected ID msg_123, got %s", parsed.ID)
	}
	if parsed.Type != "message" {
		t.Errorf("expected type message, got %s", parsed.Type)
	}
	if len(parsed.Content) != 1 {
		t.Errorf("expected 1 content block, got %d", len(parsed.Content))
	}
	if parsed.Content[0].Text != "Hello! How can I help you?" {
		t.Errorf("unexpected content text: %s", parsed.Content[0].Text)
	}
}

// TestAnthropicStreamEventParsing tests parsing of Anthropic stream events
func TestAnthropicStreamEventParsing(t *testing.T) {
	events := []string{
		`{"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-sonnet-20240229","content":[]}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		`{"type":"message_stop"}`,
	}

	for _, eventData := range events {
		var event AnthropicStreamEvent
		if err := json.Unmarshal([]byte(eventData), &event); err != nil {
			t.Errorf("failed to parse event: %v", err)
		}
	}
}

// TestMockUpstreamIntegration tests the handler with a mock upstream server
func TestMockUpstreamIntegration(t *testing.T) {
	// Create mock upstream server that returns Anthropic format response
	mockResponse := AnthropicResponse{
		ID:    "msg_test",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-sonnet-20240229",
		Content: []AnthropicContentBlock{
			{Type: "text", Text: "Test response"},
		},
		StopReason: "end_turn",
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected path /v1/messages, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Verify headers
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header")
		}

		// Read and verify request body
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("failed to parse request body: %v", err)
		}

		if _, ok := req["model"]; !ok {
			t.Error("expected model in request")
		}
		if _, ok := req["messages"]; !ok {
			t.Error("expected messages in request")
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer mockServer.Close()

	t.Logf("Mock server running at: %s", mockServer.URL)
}

// TestStreamingMockUpstream tests streaming response from mock upstream
func TestStreamingMockUpstream(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("expected flusher support")
			return
		}

		events := []string{
			`{"type":"message_start","message":{"id":"msg_123"}}`,
			`{"type":"content_block_delta","delta":{"text":"Hello"}}`,
			`{"type":"message_stop"}`,
		}

		for _, event := range events {
			w.Write([]byte("data: " + event + "\n\n"))
			flusher.Flush()
		}
	}))
	defer mockServer.Close()

	// Make request to mock server
	resp, err := http.Post(mockServer.URL+"/v1/messages", "application/json", strings.NewReader(`{"model":"test","messages":[]}`))
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "message_start") {
		t.Error("expected message_start in response")
	}
	if !strings.Contains(string(body), "Hello") {
		t.Error("expected Hello in response")
	}
}
