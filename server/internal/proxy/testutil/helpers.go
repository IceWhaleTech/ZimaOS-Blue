package testutil

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// TestClient provides helper methods for testing the proxy
type TestClient struct {
	client   *http.Client
	baseURL  string
	apiKey   string
	timeout  time.Duration
}

// NewTestClient creates a new test client
func NewTestClient(baseURL, apiKey string) *TestClient {
	return &TestClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		baseURL: baseURL,
		apiKey:  apiKey,
		timeout: 30 * time.Second,
	}
}

// SetTimeout sets the client timeout
func (tc *TestClient) SetTimeout(d time.Duration) {
	tc.timeout = d
	tc.client.Timeout = d
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

// ChatMessage represents a message in a chat request
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   TokenUsage   `json:"usage"`
}

// ChatChoice represents a choice in a chat response
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message,omitempty"`
	Delta        ChatMessage `json:"delta,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// SSEChunk represents a parsed SSE chunk
type SSEChunk struct {
	Data      string
	Parsed    *ChatResponse
	Timestamp time.Time
	IsDone    bool
	Error     error
}

// CreateTestRequest creates a simple test chat request
func CreateTestRequest(stream bool) *ChatRequest {
	return &ChatRequest{
		Model: "mock-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello, this is a test message."},
		},
		Stream:    stream,
		MaxTokens: 100,
	}
}

// SendRequest sends a non-streaming chat request
func (tc *TestClient) SendRequest(req *ChatRequest) (*ChatResponse, *http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", tc.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if tc.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+tc.apiKey)
	}

	resp, err := tc.client.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, resp, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		resp.Body.Close()
		return nil, resp, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, resp, nil
}

// SendStreamRequest sends a streaming chat request and returns the response
func (tc *TestClient) SendStreamRequest(req *ChatRequest) (*http.Response, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", tc.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if tc.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+tc.apiKey)
	}

	return tc.client.Do(httpReq)
}

// SendStreamRequestWithContext sends a streaming request with context
func (tc *TestClient) SendStreamRequestWithContext(ctx context.Context, req *ChatRequest) (*http.Response, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", tc.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if tc.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+tc.apiKey)
	}

	return tc.client.Do(httpReq)
}

// CollectSSEChunks reads all SSE chunks from a response body
func CollectSSEChunks(body io.ReadCloser) ([]SSEChunk, error) {
	defer body.Close()

	var chunks []SSEChunk
	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			chunk := SSEChunk{
				Data:      data,
				Timestamp: time.Now(),
			}

			if data == "[DONE]" {
				chunk.IsDone = true
			} else {
				var parsed ChatResponse
				if err := json.Unmarshal([]byte(data), &parsed); err != nil {
					chunk.Error = err
				} else {
					chunk.Parsed = &parsed
				}
			}

			chunks = append(chunks, chunk)
		}
	}

	if err := scanner.Err(); err != nil {
		return chunks, err
	}

	return chunks, nil
}

// ReconstructContent reconstructs the full content from SSE chunks
func ReconstructContent(chunks []SSEChunk) string {
	var content strings.Builder
	for _, chunk := range chunks {
		if chunk.Parsed != nil && len(chunk.Parsed.Choices) > 0 {
			content.WriteString(chunk.Parsed.Choices[0].Delta.Content)
		}
	}
	return content.String()
}

// MeasureTTFT measures Time to First Token
func (tc *TestClient) MeasureTTFT(req *ChatRequest) (time.Duration, error) {
	start := time.Now()
	resp, err := tc.SendStreamRequest(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			return time.Since(start), nil
		}
	}

	return 0, fmt.Errorf("no data received")
}

// TTFTStats holds TTFT statistics
type TTFTStats struct {
	Samples []time.Duration
	P50     time.Duration
	P95     time.Duration
	P99     time.Duration
	Avg     time.Duration
	Min     time.Duration
	Max     time.Duration
}

// MeasureTTFTMultiple measures TTFT over multiple requests
func (tc *TestClient) MeasureTTFTMultiple(req *ChatRequest, count int) (*TTFTStats, error) {
	samples := make([]time.Duration, 0, count)

	for i := 0; i < count; i++ {
		ttft, err := tc.MeasureTTFT(req)
		if err != nil {
			continue // Skip failed requests
		}
		samples = append(samples, ttft)
	}

	if len(samples) == 0 {
		return nil, fmt.Errorf("no successful measurements")
	}

	// Sort for percentile calculation
	sort.Slice(samples, func(i, j int) bool {
		return samples[i] < samples[j]
	})

	stats := &TTFTStats{
		Samples: samples,
		Min:     samples[0],
		Max:     samples[len(samples)-1],
	}

	// Calculate percentiles
	stats.P50 = samples[len(samples)*50/100]
	stats.P95 = samples[len(samples)*95/100]
	if len(samples) > 99 {
		stats.P99 = samples[len(samples)*99/100]
	} else {
		stats.P99 = samples[len(samples)-1]
	}

	// Calculate average
	var total time.Duration
	for _, s := range samples {
		total += s
	}
	stats.Avg = total / time.Duration(len(samples))

	return stats, nil
}

// GetModels fetches the models list
func (tc *TestClient) GetModels() ([]map[string]interface{}, error) {
	req, err := http.NewRequest("GET", tc.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	if tc.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+tc.apiKey)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetHealth checks the health endpoint
func (tc *TestClient) GetHealth() (int, map[string]interface{}, error) {
	resp, err := tc.client.Get(tc.baseURL + "/health")
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return resp.StatusCode, result, nil
}

// GetProxyStatus gets the proxy status
func (tc *TestClient) GetProxyStatus() (map[string]interface{}, error) {
	resp, err := tc.client.Get(tc.baseURL + "/api/v1/proxy/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetProxyMetrics gets the proxy metrics
func (tc *TestClient) GetProxyMetrics() (map[string]interface{}, error) {
	resp, err := tc.client.Get(tc.baseURL + "/api/v1/proxy/metrics")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetProviders gets the list of providers
func (tc *TestClient) GetProviders() ([]map[string]interface{}, error) {
	resp, err := tc.client.Get(tc.baseURL + "/api/v1/proxy/providers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Providers []map[string]interface{} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Providers, nil
}

// DrainResponse reads and discards the response body
func DrainResponse(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
