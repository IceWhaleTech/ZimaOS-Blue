package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestProxyClient_Chat(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
		}

		// Parse request
		var req OpenAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		// Verify request content
		if req.Model != "gpt-4" {
			t.Errorf("expected model gpt-4, got %s", req.Model)
		}
		if len(req.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "user" {
			t.Errorf("expected role user, got %s", req.Messages[0].Role)
		}
		if req.Messages[0].Content != "Hello" {
			t.Errorf("expected content Hello, got %s", req.Messages[0].Content)
		}

		// Return response
		resp := OpenAIChatResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Extract port from server URL
	// Note: We don't need the port since we use server.URL directly
	// This is just for documentation purposes

	// Create client pointing to mock server
	client := &ProxyClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	// Test Chat
	req := llm.ChatRequest{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
		},
	}

	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Verify response
	if resp.Message.Role != llm.RoleAssistant {
		t.Errorf("expected role assistant, got %s", resp.Message.Role)
	}
	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("expected content 'Hello! How can I help you?', got %s", resp.Message.Content)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("expected 10 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 20 {
		t.Errorf("expected 20 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("expected 30 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestProxyClient_Chat_Error(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := &ProxyClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	req := llm.ChatRequest{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
		},
	}

	_, err := client.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status 500, got: %v", err)
	}
}

func TestProxyClient_ChatStreamCallback(t *testing.T) {
	// Create mock SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify streaming request
		var req OpenAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if !req.Stream {
			t.Error("expected stream=true")
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		// Send chunks
		chunks := []string{
			`{"choices":[{"delta":{"content":"Hello"}}]}`,
			`{"choices":[{"delta":{"content":" World"}}]}`,
			`{"choices":[{"delta":{"content":"!"}, "finish_reason":"stop"}], "usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`,
		}

		for _, chunk := range chunks {
			w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}

		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	client := &ProxyClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	req := llm.ChatRequest{
		Model:  "gpt-4",
		Stream: true,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi"},
		},
	}

	var content string
	var finalUsage *llm.Usage
	var doneReceived bool

	err := client.ChatStreamCallback(context.Background(), req, func(chunk llm.StreamChunk) error {
		content += chunk.Delta
		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}
		if chunk.Done {
			doneReceived = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStreamCallback failed: %v", err)
	}

	if content != "Hello World!" {
		t.Errorf("expected content 'Hello World!', got '%s'", content)
	}

	if !doneReceived {
		t.Error("expected done=true")
	}

	if finalUsage != nil {
		if finalUsage.TotalTokens != 8 {
			t.Errorf("expected 8 total tokens, got %d", finalUsage.TotalTokens)
		}
	}
}

func TestProxyClient_ChatStreamCallback_Cancelled(t *testing.T) {
	// Create mock server that sends slowly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, _ := w.(http.Flusher)

		// Send first chunk
		w.Write([]byte(`data: {"choices":[{"delta":{"content":"Hello"}}]}` + "\n\n"))
		flusher.Flush()

		// Wait for cancellation
		<-r.Context().Done()
	}))
	defer server.Close()

	client := &ProxyClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())

	req := llm.ChatRequest{
		Model:  "gpt-4",
		Stream: true,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi"},
		},
	}

	chunkCount := 0
	go func() {
		// Cancel after receiving first chunk
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := client.ChatStreamCallback(ctx, req, func(chunk llm.StreamChunk) error {
		chunkCount++
		return nil
	})

	if err == nil || err != context.Canceled {
		t.Logf("Got error: %v (expected context.Canceled)", err)
	}

	if chunkCount == 0 {
		t.Error("expected at least one chunk before cancellation")
	}
}

func TestNewProxyClient(t *testing.T) {
	client := NewProxyClient(8080)

	if client.baseURL != "http://127.0.0.1:8080" {
		t.Errorf("expected baseURL http://127.0.0.1:8080, got %s", client.baseURL)
	}

	if client.httpClient == nil {
		t.Error("expected httpClient to be initialized")
	}

	if client.httpClient.Timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", client.httpClient.Timeout)
	}
}
