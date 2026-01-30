// Package testutil provides testing utilities for the proxy package
package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Error modes for MockServer
const (
	ErrorNone            = "none"
	Error500             = "500"
	Error502             = "502"
	Error503             = "503"
	Error429             = "429"
	ErrorTimeout         = "timeout"
	ErrorConnectionReset = "conn_reset"
	ErrorPartialStream   = "partial"
	ErrorMalformedJSON   = "malformed"
)

// TokenUsage represents token usage in responses
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// MockServer is a configurable mock server for testing
type MockServer struct {
	server   *http.Server
	listener net.Listener
	port     int
	mu       sync.RWMutex

	// Configuration
	responseDelay    time.Duration
	streamChunkDelay time.Duration
	errorMode        string
	statusCode       int
	healthy          bool

	// Statistics
	requestCount int64

	// SSE control
	interruptAfter   int
	malformedChunkAt int
	tokenUsage       TokenUsage

	// Custom response content
	customContent string
	chunkSize     int
}

// NewMockServer creates a new mock server on the specified port
// If port is 0, a random available port will be used
func NewMockServer(port int) (*MockServer, error) {
	ms := &MockServer{
		port:       port,
		statusCode: 200,
		healthy:    true,
		chunkSize:  5,
		tokenUsage: TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", ms.handleChatCompletions)
	mux.HandleFunc("/v1/models", ms.handleModels)
	mux.HandleFunc("/health", ms.handleHealth)
	mux.HandleFunc("/ready", ms.handleReady)

	// Find available port
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	ms.listener = listener
	ms.port = listener.Addr().(*net.TCPAddr).Port

	ms.server = &http.Server{
		Handler: mux,
	}

	return ms, nil
}

// Start starts the mock server
func (ms *MockServer) Start() error {
	go func() {
		if err := ms.server.Serve(ms.listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't panic
			fmt.Printf("MockServer error: %v\n", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(50 * time.Millisecond)
	return nil
}

// Stop stops the mock server
func (ms *MockServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ms.server.Shutdown(ctx)
}

// Port returns the server's port
func (ms *MockServer) Port() int {
	return ms.port
}

// URL returns the server's base URL
func (ms *MockServer) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", ms.port)
}

func (ms *MockServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&ms.requestCount, 1)

	ms.mu.RLock()
	delay := ms.responseDelay
	errorMode := ms.errorMode
	statusCode := ms.statusCode
	streamDelay := ms.streamChunkDelay
	interruptAfter := ms.interruptAfter
	malformedAt := ms.malformedChunkAt
	tokenUsage := ms.tokenUsage
	customContent := ms.customContent
	chunkSize := ms.chunkSize
	ms.mu.RUnlock()

	// Apply initial delay
	if delay > 0 {
		time.Sleep(delay)
	}

	// Handle error modes
	if errorMode != "" && errorMode != ErrorNone {
		ms.handleError(w, errorMode)
		return
	}

	if statusCode != 200 {
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Mock error with status %d", statusCode),
				"type":    "mock_error",
				"code":    statusCode,
			},
		})
		return
	}

	// Parse request to check if streaming
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if stream, ok := req["stream"].(bool); ok && stream {
		ms.handleStreamResponse(w, streamDelay, interruptAfter, malformedAt, tokenUsage, customContent, chunkSize)
	} else {
		ms.handleNonStreamResponse(w, tokenUsage, customContent)
	}
}

func (ms *MockServer) handleStreamResponse(w http.ResponseWriter, chunkDelay time.Duration,
	interruptAfter, malformedAt int, usage TokenUsage, customContent string, chunkSize int) {

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()

	// Default content if not specified
	content := customContent
	if content == "" {
		content = "This is a test response from the mock server. It demonstrates SSE streaming capabilities."
	}

	chunks := splitIntoChunks(content, chunkSize)

	for i, chunk := range chunks {
		// Check for interrupt
		if interruptAfter > 0 && i >= interruptAfter {
			return // Simulate stream interruption
		}

		// Check for malformed chunk
		if malformedAt > 0 && i == malformedAt {
			fmt.Fprintf(w, "data: {invalid json\n\n")
			flusher.Flush()
			continue
		}

		data := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-chunk-%d", i),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   "mock-model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]string{
						"content": chunk,
					},
					"finish_reason": nil,
				},
			},
		}

		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()

		if chunkDelay > 0 {
			time.Sleep(chunkDelay)
		}
	}

	// Send final chunk with usage
	finalData := map[string]interface{}{
		"id":      "chatcmpl-final",
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   "mock-model",
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]string{},
				"finish_reason": "stop",
			},
		},
		"usage": usage,
	}
	jsonData, _ := json.Marshal(finalData)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (ms *MockServer) handleNonStreamResponse(w http.ResponseWriter, usage TokenUsage, customContent string) {
	content := customContent
	if content == "" {
		content = "This is a test response from the mock server."
	}

	response := map[string]interface{}{
		"id":      "chatcmpl-mock-123",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "mock-model",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": usage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (ms *MockServer) handleError(w http.ResponseWriter, errorMode string) {
	switch errorMode {
	case Error500:
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Internal Server Error",
				"type":    "server_error",
			},
		})
	case Error502:
		w.WriteHeader(502)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Bad Gateway",
				"type":    "gateway_error",
			},
		})
	case Error503:
		w.WriteHeader(503)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Service Unavailable",
				"type":    "service_unavailable",
			},
		})
	case Error429:
		w.Header().Set("Retry-After", "60")
		w.Header().Set("X-RateLimit-Limit", "1000")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(429)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Rate limit exceeded",
				"type":    "rate_limit_error",
			},
		})
	case ErrorTimeout:
		time.Sleep(120 * time.Second) // Exceed typical client timeout
	case ErrorConnectionReset:
		// Force close connection
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				conn.Close()
			}
		}
	}
}

func (ms *MockServer) handleModels(w http.ResponseWriter, r *http.Request) {
	ms.mu.RLock()
	healthy := ms.healthy
	ms.mu.RUnlock()

	if !healthy {
		w.WriteHeader(503)
		return
	}

	response := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{"id": "mock-model", "object": "model", "owned_by": "mock"},
			{"id": "mock-model-fast", "object": "model", "owned_by": "mock"},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (ms *MockServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	ms.mu.RLock()
	healthy := ms.healthy
	ms.mu.RUnlock()

	if healthy {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	} else {
		w.WriteHeader(503)
		json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
	}
}

func (ms *MockServer) handleReady(w http.ResponseWriter, r *http.Request) {
	ms.mu.RLock()
	healthy := ms.healthy
	ms.mu.RUnlock()

	if healthy {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	} else {
		w.WriteHeader(503)
		json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
	}
}

// Configuration methods

// SetErrorMode sets the error mode for the server
func (ms *MockServer) SetErrorMode(mode string) {
	ms.mu.Lock()
	ms.errorMode = mode
	ms.mu.Unlock()
}

// SetStatusCode sets the HTTP status code for responses
func (ms *MockServer) SetStatusCode(code int) {
	ms.mu.Lock()
	ms.statusCode = code
	ms.mu.Unlock()
}

// SetResponseDelay sets the initial response delay
func (ms *MockServer) SetResponseDelay(d time.Duration) {
	ms.mu.Lock()
	ms.responseDelay = d
	ms.mu.Unlock()
}

// SetStreamChunkDelay sets the delay between SSE chunks
func (ms *MockServer) SetStreamChunkDelay(d time.Duration) {
	ms.mu.Lock()
	ms.streamChunkDelay = d
	ms.mu.Unlock()
}

// SetHealthy sets the health status of the server
func (ms *MockServer) SetHealthy(healthy bool) {
	ms.mu.Lock()
	ms.healthy = healthy
	ms.mu.Unlock()
}

// SetInterruptAfterChunks sets the number of chunks after which to interrupt the stream
func (ms *MockServer) SetInterruptAfterChunks(n int) {
	ms.mu.Lock()
	ms.interruptAfter = n
	ms.mu.Unlock()
}

// SetMalformedChunkAt sets the chunk index at which to send malformed JSON
func (ms *MockServer) SetMalformedChunkAt(n int) {
	ms.mu.Lock()
	ms.malformedChunkAt = n
	ms.mu.Unlock()
}

// SetTokenUsage sets the token usage to return in responses
func (ms *MockServer) SetTokenUsage(usage TokenUsage) {
	ms.mu.Lock()
	ms.tokenUsage = usage
	ms.mu.Unlock()
}

// SetCustomContent sets custom response content
func (ms *MockServer) SetCustomContent(content string) {
	ms.mu.Lock()
	ms.customContent = content
	ms.mu.Unlock()
}

// SetChunkSize sets the size of SSE chunks
func (ms *MockServer) SetChunkSize(size int) {
	ms.mu.Lock()
	ms.chunkSize = size
	ms.mu.Unlock()
}

// Statistics methods

// GetRequestCount returns the number of requests received
func (ms *MockServer) GetRequestCount() int64 {
	return atomic.LoadInt64(&ms.requestCount)
}

// ResetRequestCount resets the request counter
func (ms *MockServer) ResetRequestCount() {
	atomic.StoreInt64(&ms.requestCount, 0)
}

// Reset resets all configuration to defaults
func (ms *MockServer) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.responseDelay = 0
	ms.streamChunkDelay = 0
	ms.errorMode = ""
	ms.statusCode = 200
	ms.healthy = true
	ms.interruptAfter = 0
	ms.malformedChunkAt = 0
	ms.customContent = ""
	ms.chunkSize = 5
	ms.tokenUsage = TokenUsage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
	}
	atomic.StoreInt64(&ms.requestCount, 0)
}

// Helper functions

func splitIntoChunks(s string, size int) []string {
	if size <= 0 {
		size = 5
	}
	var chunks []string
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}
