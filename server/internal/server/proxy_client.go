package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
)

// ProxyClient is a client for making requests through the local proxy
type ProxyClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string // API key for Authorization header
}

// NewProxyClient creates a new proxy client
func NewProxyClient(port int) *ProxyClient {
	return &ProxyClient{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // Long timeout for LLM requests
		},
	}
}

// SetAPIKey sets the API key for Authorization header
func (c *ProxyClient) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// OpenAIChatRequest represents an OpenAI-compatible chat request
type OpenAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenAIChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

// OpenAIChatMessage represents an OpenAI-compatible message
type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatResponse represents an OpenAI-compatible chat response
type OpenAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Chat sends a chat request through the proxy
func (c *ProxyClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	// Convert to OpenAI format
	openaiReq := OpenAIChatRequest{
		Model:       req.Model,
		Messages:    make([]OpenAIChatMessage, len(req.Messages)),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	for i, msg := range req.Messages {
		openaiReq.Messages[i] = OpenAIChatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	// Marshal request
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var openaiResp OpenAIChatResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to llm.ChatResponse
	llmResp := &llm.ChatResponse{
		Usage: llm.Usage{
			PromptTokens:     openaiResp.Usage.PromptTokens,
			CompletionTokens: openaiResp.Usage.CompletionTokens,
			TotalTokens:      openaiResp.Usage.TotalTokens,
		},
	}

	if len(openaiResp.Choices) > 0 {
		llmResp.Message = llm.Message{
			Role:    llm.Role(openaiResp.Choices[0].Message.Role),
			Content: openaiResp.Choices[0].Message.Content,
		}
	}

	return llmResp, nil
}

// ChatStreamCallback sends a streaming chat request through the proxy
func (c *ProxyClient) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback func(llm.StreamChunk) error) error {
	// Convert to OpenAI format
	openaiReq := OpenAIChatRequest{
		Model:       req.Model,
		Messages:    make([]OpenAIChatMessage, len(req.Messages)),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	for i, msg := range req.Messages {
		openaiReq.Messages[i] = OpenAIChatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	// Marshal request
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for error status
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("proxy returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Read SSE stream using bufio.Scanner for better performance
	scanner := bufio.NewScanner(resp.Body)
	// Set a larger buffer for long lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var totalContent string
	var usage *llm.Usage
	lastActivity := time.Now()
	idleTimeout := 30 * time.Second // Timeout if no data for 30 seconds

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check for idle timeout
		if time.Since(lastActivity) > idleTimeout {
			return fmt.Errorf("stream idle timeout: no data received for %v", idleTimeout)
		}

		// Read next line
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("stream read error: %w", err)
			}
			// EOF - stream ended normally
			return callback(llm.StreamChunk{Done: true, Usage: usage})
		}
		lastActivity = time.Now()

		lineStr := scanner.Text()

		// Skip empty lines
		if lineStr == "" {
			continue
		}

		// Check for SSE data prefix
		if !strings.HasPrefix(lineStr, "data: ") {
			continue
		}

		data := strings.TrimPrefix(lineStr, "data: ")

		// Check for [DONE] marker
		if data == "[DONE]" {
			return callback(llm.StreamChunk{Done: true, Usage: usage})
		}

		// Parse SSE chunk
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Skip malformed chunks
			continue
		}

		// Extract content delta
		delta := ""
		done := false
		if len(chunk.Choices) > 0 {
			delta = chunk.Choices[0].Delta.Content
			totalContent += delta
			done = chunk.Choices[0].FinishReason != ""
		}

		// Extract usage if present
		if chunk.Usage != nil {
			usage = &llm.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		// Send chunk to callback
		if err := callback(llm.StreamChunk{
			Delta: delta,
			Done:  done,
			Usage: usage,
		}); err != nil {
			return err
		}

		if done {
			return nil
		}
	}
}
