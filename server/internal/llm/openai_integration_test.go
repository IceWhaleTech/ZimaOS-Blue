package llm

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestOpenAIDirectIntegration tests direct requests to OpenAI API
// This test requires OPENAI_API_KEY environment variable to be set
// Run with: OPENAI_API_KEY=sk-xxx go test -v -run TestOpenAIDirectIntegration
func TestOpenAIDirectIntegration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	t.Run("DirectRequest_ChatCompletion", func(t *testing.T) {
		provider := NewOpenAIProvider(apiKey, "")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := ChatRequest{
			Model: "gpt-4o-mini",
			Messages: []Message{
				{Role: RoleUser, Content: "Say 'Hello, World!' and nothing else."},
			},
			MaxTokens: 20,
		}

		resp, err := provider.Chat(ctx, req)
		if err != nil {
			t.Fatalf("Direct OpenAI request failed: %v", err)
		}

		// Verify response
		if resp.Message.Content == "" {
			t.Error("Expected non-empty response content")
		}
		if resp.Usage.TotalTokens == 0 {
			t.Error("Expected non-zero token usage")
		}

		t.Logf("✓ Direct request successful")
		t.Logf("  Model: %s", resp.Model)
		t.Logf("  Response: %s", resp.Message.Content)
		t.Logf("  Tokens: %d (prompt: %d, completion: %d)",
			resp.Usage.TotalTokens,
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens)
	})

	t.Run("DirectRequest_StreamingChunked", func(t *testing.T) {
		provider := NewOpenAIProvider(apiKey, "")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := ChatRequest{
			Model: "gpt-4o-mini",
			Messages: []Message{
				{Role: RoleUser, Content: "Count from 1 to 5, one number per line."},
			},
			Stream:    true,
			MaxTokens: 50,
		}

		// Test using channel-based streaming (chunked)
		stream, err := provider.ChatStream(ctx, req)
		if err != nil {
			t.Fatalf("Direct OpenAI streaming request failed: %v", err)
		}

		var chunks []string
		chunkCount := 0

		// Read from channel until closed
		for chunk := range stream {
			chunkCount++
			if chunk.Delta != "" {
				chunks = append(chunks, chunk.Delta)
			}

			// Log first few chunks to verify streaming
			if chunkCount <= 3 {
				t.Logf("  Chunk %d: %q", chunkCount, chunk.Delta)
			}
		}

		if len(chunks) == 0 {
			t.Error("Expected at least one streaming chunk")
		}

		t.Logf("✓ Direct streaming (chunked) request successful")
		t.Logf("  Received %d chunks", chunkCount)
		t.Logf("  Total content length: %d chars", len(chunks))
	})

	t.Run("DirectRequest_StreamingCallback", func(t *testing.T) {
		provider := NewOpenAIProvider(apiKey, "")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := ChatRequest{
			Model: "gpt-4o-mini",
			Messages: []Message{
				{Role: RoleUser, Content: "Say hello in 3 different languages."},
			},
			Stream:    true,
			MaxTokens: 100,
		}

		// Test using callback-based streaming (for SSE compatibility)
		var receivedChunks []string
		chunkCount := 0

		err := provider.ChatStreamCallback(ctx, req, func(chunk StreamChunk) error {
			chunkCount++
			if chunk.Delta != "" {
				receivedChunks = append(receivedChunks, chunk.Delta)
			}
			return nil
		})

		if err != nil {
			t.Fatalf("Streaming callback failed: %v", err)
		}

		if len(receivedChunks) == 0 {
			t.Error("Expected at least one streaming chunk via callback")
		}

		t.Logf("✓ Direct streaming (callback/SSE) request successful")
		t.Logf("  Received %d chunks via callback", chunkCount)
	})

	t.Run("DirectRequest_WithTools", func(t *testing.T) {
		provider := NewOpenAIProvider(apiKey, "")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := ChatRequest{
			Model: "gpt-4o-mini",
			Messages: []Message{
				{Role: RoleUser, Content: "What's the weather in San Francisco?"},
			},
			Tools: []Tool{
				{
					Name:        "get_weather",
					Description: "Get the current weather in a location",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city and state, e.g. San Francisco, CA",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		}

		resp, err := provider.Chat(ctx, req)
		if err != nil {
			t.Fatalf("Direct OpenAI request with tools failed: %v", err)
		}

		// The model should either respond with text or a tool call
		if resp.Message.Content == "" && len(resp.Message.ToolCalls) == 0 {
			t.Error("Expected either content or tool calls in response")
		}

		t.Logf("✓ Direct request with tools successful")
		if len(resp.Message.ToolCalls) > 0 {
			t.Logf("  Tool called: %s", resp.Message.ToolCalls[0].Name)
		}
	})

	t.Run("DirectRequest_ErrorHandling", func(t *testing.T) {
		// Test with invalid API key
		provider := NewOpenAIProvider("sk-invalid-key-12345", "")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := ChatRequest{
			Model: "gpt-4o-mini",
			Messages: []Message{
				{Role: RoleUser, Content: "Hello"},
			},
		}

		_, err := provider.Chat(ctx, req)
		if err == nil {
			t.Error("Expected error with invalid API key, got nil")
		}

		t.Logf("✓ Error handling works correctly: %v", err)
	})
}

// TestOpenAICustomBaseURL tests OpenAI-compatible APIs with custom base URLs
// This can be used to test third-party OpenAI-compatible services
func TestOpenAICustomBaseURL(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	customBaseURL := os.Getenv("OPENAI_CUSTOM_BASE_URL")

	if apiKey == "" || customBaseURL == "" {
		t.Skip("Skipping custom base URL test: OPENAI_API_KEY or OPENAI_CUSTOM_BASE_URL not set")
	}

	t.Run("CustomBaseURL_ChatCompletion", func(t *testing.T) {
		provider := NewOpenAIProvider(apiKey, customBaseURL)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := ChatRequest{
			Model: "gpt-4o-mini",
			Messages: []Message{
				{Role: RoleUser, Content: "Say 'Hello' and nothing else."},
			},
			MaxTokens: 10,
		}

		resp, err := provider.Chat(ctx, req)
		if err != nil {
			t.Fatalf("Custom base URL request failed: %v", err)
		}

		if resp.Message.Content == "" {
			t.Error("Expected non-empty response content")
		}

		t.Logf("✓ Custom base URL request successful")
		t.Logf("  Base URL: %s", customBaseURL)
		t.Logf("  Response: %s", resp.Message.Content)
	})
}

// TestOpenAIModelsList tests fetching available models
func TestOpenAIModelsList(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping models list test: OPENAI_API_KEY not set")
	}

	provider := NewOpenAIProvider(apiKey, "")
	models := provider.Models()

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	t.Logf("✓ Models list retrieved successfully")
	t.Logf("  Available models: %d", len(models))
	if len(models) > 0 {
		t.Logf("  First model: %s", models[0])
	}
}

// TestOpenAIConnectionPersistence tests that streaming maintains connection
func TestOpenAIConnectionPersistence(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping connection persistence test: OPENAI_API_KEY not set")
	}

	provider := NewOpenAIProvider(apiKey, "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []Message{
			{Role: RoleUser, Content: "Write a short story about a robot. Make it at least 100 words."},
		},
		Stream:    true,
		MaxTokens: 200,
	}

	stream, err := provider.ChatStream(ctx, req)
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	startTime := time.Now()
	chunkCount := 0
	var lastChunkTime time.Time

	for chunk := range stream {
		chunkCount++
		lastChunkTime = time.Now()

		// Verify chunks are coming in continuously
		if chunkCount > 1 && chunkCount <= 5 {
			timeSinceStart := lastChunkTime.Sub(startTime)
			t.Logf("  Chunk %d received at %v: %q", chunkCount, timeSinceStart, chunk.Delta)
		}
	}

	totalDuration := time.Since(startTime)

	if chunkCount < 5 {
		t.Errorf("Expected at least 5 chunks for a longer response, got %d", chunkCount)
	}

	t.Logf("✓ Connection persistence test successful")
	t.Logf("  Total chunks: %d", chunkCount)
	t.Logf("  Total duration: %v", totalDuration)
	t.Logf("  Average time per chunk: %v", totalDuration/time.Duration(chunkCount))
}
