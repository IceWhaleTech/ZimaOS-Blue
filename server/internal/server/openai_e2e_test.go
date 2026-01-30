package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
)

// TestOpenAIThroughEchoServer tests OpenAI requests through the echo server
// This simulates how a CLI client would interact with the echo server
// Run with: OPENAI_API_KEY=sk-xxx go test -v -run TestOpenAIThroughEchoServer
func TestOpenAIThroughEchoServer(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping E2E test: OPENAI_API_KEY not set")
	}

	// Setup echo server with OpenAI provider
	e := echo.New()
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()

	// Register OpenAI provider
	openaiProvider := llm.NewOpenAIProvider(apiKey, "")
	registry.Register(openaiProvider)

	handler := NewChatHandler(store, registry, toolRegistry)

	// Register routes
	api := e.Group("/api/v1")
	conversations := api.Group("/conversations")
	conversations.POST("/:id/messages/stream", handler.StreamMessage)

	t.Run("ThroughServer_ChatCompletion", func(t *testing.T) {
		// Create request
		reqBody := map[string]interface{}{
			"message":  "Say 'Hello from Echo Server!' and nothing else.",
			"provider": "openai",
			"model":    "gpt-4o-mini",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/test-conv/messages/stream", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Parse SSE response
		responseText := parseSSEResponse(t, rec.Body.String())

		if responseText == "" {
			t.Error("Expected non-empty response from server")
		}

		t.Logf("✓ Request through echo server successful")
		t.Logf("  Response: %s", responseText)
	})

	t.Run("ThroughServer_MultipleMessages", func(t *testing.T) {
		convID := "test-conv-multi"

		// First message
		reqBody1 := map[string]interface{}{
			"message":  "My name is Alice.",
			"provider": "openai",
			"model":    "gpt-4o-mini",
		}
		body1, _ := json.Marshal(reqBody1)

		req1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/conversations/%s/messages/stream", convID), bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		rec1 := httptest.NewRecorder()

		e.ServeHTTP(rec1, req1)

		if rec1.Code != http.StatusOK {
			t.Fatalf("First message failed: %d", rec1.Code)
		}

		// Second message - should remember context
		reqBody2 := map[string]interface{}{
			"message":  "What is my name?",
			"provider": "openai",
			"model":    "gpt-4o-mini",
		}
		body2, _ := json.Marshal(reqBody2)

		req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/conversations/%s/messages/stream", convID), bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		rec2 := httptest.NewRecorder()

		e.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Fatalf("Second message failed: %d", rec2.Code)
		}

		responseText := parseSSEResponse(t, rec2.Body.String())

		// The response should mention "Alice"
		if !strings.Contains(strings.ToLower(responseText), "alice") {
			t.Errorf("Expected response to mention 'Alice', got: %s", responseText)
		}

		t.Logf("✓ Multi-message conversation successful")
		t.Logf("  Context preserved correctly")
	})

	t.Run("ThroughServer_ErrorHandling", func(t *testing.T) {
		// Test with invalid model
		reqBody := map[string]interface{}{
			"message":  "Hello",
			"provider": "openai",
			"model":    "invalid-model-xyz",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/test-error/messages/stream", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should return an error (either 400 or 500)
		if rec.Code == http.StatusOK {
			// Check if error is in SSE stream
			responseText := rec.Body.String()
			if !strings.Contains(responseText, "error") {
				t.Error("Expected error response for invalid model")
			}
		}

		t.Logf("✓ Error handling works correctly")
	})
}

// TestOpenAIProviderComparison compares direct vs proxied requests
func TestOpenAIProviderComparison(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping comparison test: OPENAI_API_KEY not set")
	}

	testMessage := "Say 'Test Response' and nothing else."

	// Test 1: Direct request
	t.Run("DirectRequest", func(t *testing.T) {
		provider := llm.NewOpenAIProvider(apiKey, "")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := llm.ChatRequest{
			Model: "gpt-4o-mini",
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: testMessage},
			},
			MaxTokens: 20,
		}

		startTime := time.Now()
		resp, err := provider.Chat(ctx, req)
		latency := time.Since(startTime)

		if err != nil {
			t.Fatalf("Direct request failed: %v", err)
		}

		t.Logf("✓ Direct request completed")
		t.Logf("  Latency: %v", latency)
		t.Logf("  Tokens: %d", resp.Usage.TotalTokens)
	})

	// Test 2: Through echo server
	t.Run("ThroughEchoServer", func(t *testing.T) {
		e := echo.New()
		store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
		registry := llm.NewProviderRegistry()
		toolRegistry := tools.NewRegistry()

		openaiProvider := llm.NewOpenAIProvider(apiKey, "")
		registry.Register(openaiProvider)

		handler := NewChatHandler(store, registry, toolRegistry)

		api := e.Group("/api/v1")
		conversations := api.Group("/conversations")
		conversations.POST("/:id/messages/stream", handler.StreamMessage)

		reqBody := map[string]interface{}{
			"message":  testMessage,
			"provider": "openai",
			"model":    "gpt-4o-mini",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/test-compare/messages/stream", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		startTime := time.Now()
		e.ServeHTTP(rec, req)
		latency := time.Since(startTime)

		if rec.Code != http.StatusOK {
			t.Fatalf("Proxied request failed: %d", rec.Code)
		}

		t.Logf("✓ Proxied request completed")
		t.Logf("  Latency: %v", latency)
		t.Logf("  Note: Proxied requests may have slightly higher latency due to server overhead")
	})
}

// TestOpenAIStreamingThroughServer tests streaming responses through the server
func TestOpenAIStreamingThroughServer(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping streaming test: OPENAI_API_KEY not set")
	}

	e := echo.New()
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()

	openaiProvider := llm.NewOpenAIProvider(apiKey, "")
	registry.Register(openaiProvider)

	handler := NewChatHandler(store, registry, toolRegistry)

	api := e.Group("/api/v1")
	conversations := api.Group("/conversations")
	conversations.POST("/:id/messages/stream", handler.StreamMessage)

	reqBody := map[string]interface{}{
		"message":  "Count from 1 to 5, one number per line.",
		"provider": "openai",
		"model":    "gpt-4o-mini",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/test-stream/messages/stream", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Streaming request failed: %d: %s", rec.Code, rec.Body.String())
	}

	// Verify we got SSE events
	responseBody := rec.Body.String()
	if !strings.Contains(responseBody, "data:") {
		t.Error("Expected SSE format with 'data:' prefix")
	}

	// Count the number of chunks
	chunks := strings.Count(responseBody, "data:")
	if chunks < 2 {
		t.Errorf("Expected multiple streaming chunks, got %d", chunks)
	}

	t.Logf("✓ Streaming through server successful")
	t.Logf("  Received %d chunks", chunks)
}

// parseSSEResponse extracts the text content from SSE response
func parseSSEResponse(t *testing.T, sseData string) string {
	var fullText strings.Builder

	lines := strings.Split(sseData, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			// Extract content from delta
			if delta, ok := chunk["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok {
					fullText.WriteString(content)
				}
			}
		}
	}

	return fullText.String()
}

// TestOpenAIHealthCheck tests the health of OpenAI provider through the server
func TestOpenAIHealthCheck(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping health check test: OPENAI_API_KEY not set")
	}

	provider := llm.NewOpenAIProvider(apiKey, "")

	// Test 1: Provider is registered
	if provider.Name() != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", provider.Name())
	}

	// Test 2: Models are available
	models := provider.Models()
	if len(models) == 0 {
		t.Error("Expected at least one model to be available")
	}

	// Test 3: Can make a simple request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := llm.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi"},
		},
		MaxTokens: 5,
	}

	_, err := provider.Chat(ctx, req)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}

	t.Logf("✓ OpenAI provider health check passed")
	t.Logf("  Available models: %d", len(models))
}

// BenchmarkOpenAIDirect benchmarks direct OpenAI requests
func BenchmarkOpenAIDirect(b *testing.B) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		b.Skip("Skipping benchmark: OPENAI_API_KEY not set")
	}

	provider := llm.NewOpenAIProvider(apiKey, "")

	req := llm.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi"},
		},
		MaxTokens: 5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := provider.Chat(ctx, req)
		cancel()

		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}

		// Rate limiting delay
		time.Sleep(100 * time.Millisecond)
	}
}

// BenchmarkOpenAIThroughServer benchmarks requests through echo server
func BenchmarkOpenAIThroughServer(b *testing.B) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		b.Skip("Skipping benchmark: OPENAI_API_KEY not set")
	}

	e := echo.New()
	store, err := memory.NewStore(":memory:")
	if err != nil {
		b.Fatalf("Failed to create memory store: %v", err)
	}
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()

	openaiProvider := llm.NewOpenAIProvider(apiKey, "")
	registry.Register(openaiProvider)

	handler := NewChatHandler(store, registry, toolRegistry)

	api := e.Group("/api/v1")
	conversations := api.Group("/conversations")
	conversations.POST("/:id/messages/stream", handler.StreamMessage)

	reqBody := map[string]interface{}{
		"message":  "Hi",
		"provider": "openai",
		"model":    "gpt-4o-mini",
	}
	body, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/conversations/bench-%d/messages/stream", i), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("Request failed: %d", rec.Code)
		}

		// Rate limiting delay
		time.Sleep(100 * time.Millisecond)
	}
}
