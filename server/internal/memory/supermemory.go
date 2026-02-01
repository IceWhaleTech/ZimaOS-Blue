package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultSupermemoryBaseURL = "https://api.supermemory.ai/v3"
	defaultTimeout            = 30 * time.Second
)

// SupermemoryConfig holds Supermemory API configuration.
type SupermemoryConfig struct {
	Enabled bool   `mapstructure:"enabled" json:"enabled"`
	APIKey  string `mapstructure:"api_key" json:"api_key"`
	BaseURL string `mapstructure:"base_url" json:"base_url"`
}

// SupermemoryClient is a client for the Supermemory API.
type SupermemoryClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewSupermemoryClient creates a new Supermemory client.
func NewSupermemoryClient(cfg SupermemoryConfig) *SupermemoryClient {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultSupermemoryBaseURL
	}

	return &SupermemoryClient{
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// SupermemoryAddRequest represents a request to add memory.
type SupermemoryAddRequest struct {
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SupermemoryAddResponse represents a response from adding memory.
type SupermemoryAddResponse struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// SupermemorySearchRequest represents a search request.
type SupermemorySearchRequest struct {
	Query       string   `json:"q"`
	TopK        int      `json:"topK,omitempty"`
	FilterTags  []string `json:"filterTags,omitempty"`
	CategoriesFilter []string `json:"categoriesFilter,omitempty"`
}

// SupermemorySearchResult represents a single search result.
type SupermemorySearchResult struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Score     float32           `json:"score"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

// SupermemorySearchResponse represents a search response.
type SupermemorySearchResponse struct {
	Results []SupermemorySearchResult `json:"results"`
}

// SupermemoryStatsResponse represents stats response.
type SupermemoryStatsResponse struct {
	TotalMemories int   `json:"totalMemories"`
	TotalSize     int64 `json:"totalSize"`
}

// Add adds a new memory to Supermemory.
func (c *SupermemoryClient) Add(ctx context.Context, content string, metadata map[string]string) (*SupermemoryAddResponse, error) {
	req := SupermemoryAddRequest{
		Content:  content,
		Metadata: metadata,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/add", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supermemory API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result SupermemoryAddResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Search searches memories in Supermemory.
func (c *SupermemoryClient) Search(ctx context.Context, query string, limit int) (*SupermemorySearchResponse, error) {
	req := SupermemorySearchRequest{
		Query: query,
		TopK:  limit,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supermemory API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result SupermemorySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Delete deletes a memory from Supermemory.
func (c *SupermemoryClient) Delete(ctx context.Context, id string) error {
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/memories/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supermemory API error: %s - %s", resp.Status, string(bodyBytes))
	}

	return nil
}

// Get retrieves a memory by ID.
func (c *SupermemoryClient) Get(ctx context.Context, id string) (*SupermemorySearchResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/memories/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supermemory API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result SupermemorySearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Stats returns memory statistics.
func (c *SupermemoryClient) Stats(ctx context.Context) (*SupermemoryStatsResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/stats", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Stats endpoint may not exist, return empty stats
		return &SupermemoryStatsResponse{}, nil
	}

	var result SupermemoryStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// TestConnection tests the connection to Supermemory API.
func (c *SupermemoryClient) TestConnection(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	// Accept any 2xx status as success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Try a simple search as fallback health check
	_, err = c.Search(ctx, "test", 1)
	if err != nil {
		return fmt.Errorf("API test failed: %w", err)
	}

	return nil
}

func (c *SupermemoryClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}
