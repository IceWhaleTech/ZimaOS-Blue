package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebSearchTool_Definition(t *testing.T) {
	tool := NewWebSearchTool(WebSearchConfig{})
	def := tool.Definition()

	if def.Name != "web_search" {
		t.Errorf("expected name 'web_search', got '%s'", def.Name)
	}

	if def.Description == "" {
		t.Error("description should not be empty")
	}

	params, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters should have properties")
	}

	if _, ok := params["query"]; !ok {
		t.Error("parameters should have 'query' property")
	}
}

func TestWebSearchTool_Execute_MissingQuery(t *testing.T) {
	tool := NewWebSearchTool(WebSearchConfig{})

	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing query")
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{"query": ""})
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestWebSearchTool_SearXNG(t *testing.T) {
	// Create mock SearXNG server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		query := r.URL.Query().Get("q")
		if query != "test query" {
			t.Errorf("unexpected query: %s", query)
		}

		format := r.URL.Query().Get("format")
		if format != "json" {
			t.Errorf("expected format=json, got %s", format)
		}

		response := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "Test Result 1",
					"url":     "https://example.com/1",
					"content": "Description 1",
					"engine":  "google",
				},
				{
					"title":   "Test Result 2",
					"url":     "https://example.com/2",
					"content": "Description 2",
					"engine":  "bing",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider: "searxng",
		BaseURL:  server.URL,
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "test query",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response WebSearchResponse
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Query != "test query" {
		t.Errorf("expected query 'test query', got '%s'", response.Query)
	}

	if len(response.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(response.Results))
	}

	if response.Results[0].Title != "Test Result 1" {
		t.Errorf("unexpected title: %s", response.Results[0].Title)
	}

	if response.Provider != "searxng" {
		t.Errorf("expected provider 'searxng', got '%s'", response.Provider)
	}
}

func TestWebSearchTool_Brave(t *testing.T) {
	// Create mock Brave Search server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-Subscription-Token")
		if apiKey != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response := map[string]interface{}{
			"web": map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"title":       "Brave Result 1",
						"url":         "https://brave.com/1",
						"description": "Brave description 1",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test without API key
	tool := NewWebSearchTool(WebSearchConfig{
		Provider: "brave",
	})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "test",
	})
	if err == nil {
		t.Error("expected error without API key")
	}
}

func TestWebSearchTool_UnsupportedProvider(t *testing.T) {
	tool := NewWebSearchTool(WebSearchConfig{
		Provider: "unsupported",
	})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "test",
	})
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "unsupported search provider") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWebSearchTool_MaxResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return many results
		results := make([]map[string]interface{}, 30)
		for i := 0; i < 30; i++ {
			results[i] = map[string]interface{}{
				"title":   "Result",
				"url":     "https://example.com",
				"content": "Description",
			}
		}

		response := map[string]interface{}{
			"results": results,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider:   "searxng",
		BaseURL:    server.URL,
		MaxResults: 5,
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response WebSearchResponse
	json.Unmarshal([]byte(result.(string)), &response)

	if len(response.Results) > 5 {
		t.Errorf("expected max 5 results, got %d", len(response.Results))
	}

	// Test with max_results parameter
	result, err = tool.Execute(context.Background(), map[string]interface{}{
		"query":       "test",
		"max_results": float64(3),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	json.Unmarshal([]byte(result.(string)), &response)
	if len(response.Results) > 3 {
		t.Errorf("expected max 3 results, got %d", len(response.Results))
	}

	// Test max_results cap at 20
	result, err = tool.Execute(context.Background(), map[string]interface{}{
		"query":       "test",
		"max_results": float64(100),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	json.Unmarshal([]byte(result.(string)), &response)
	if len(response.Results) > 20 {
		t.Errorf("expected max 20 results (cap), got %d", len(response.Results))
	}
}

func TestWebSearchTool_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider: "searxng",
		BaseURL:  server.URL,
		Timeout:  50 * time.Millisecond,
	})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "test",
	})
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestWebSearchTool_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider: "searxng",
		BaseURL:  server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := tool.Execute(ctx, map[string]interface{}{
		"query": "test",
	})
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<a href='test'>Link</a>", "Link"},
		{"No tags", "No tags"},
		{"<b>Bold</b> and <i>italic</i>", "Bold and italic"},
		{"&amp; &lt; &gt;", "& < >"},
		{"&nbsp;spaces&nbsp;", "spaces"},
		{"  extra   spaces  ", "extra   spaces"},
	}

	for _, tt := range tests {
		result := stripHTML(tt.input)
		if result != tt.expected {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestWebSearchConfig_Defaults(t *testing.T) {
	tool := NewWebSearchTool(WebSearchConfig{})

	if tool.config.MaxResults != 10 {
		t.Errorf("expected default MaxResults 10, got %d", tool.config.MaxResults)
	}

	if tool.config.Timeout != 30*time.Second {
		t.Errorf("expected default Timeout 30s, got %v", tool.config.Timeout)
	}

	if tool.config.Provider != "duckduckgo" {
		t.Errorf("expected default Provider 'duckduckgo', got '%s'", tool.config.Provider)
	}

	if tool.config.Region != "wt-wt" {
		t.Errorf("expected default Region 'wt-wt', got '%s'", tool.config.Region)
	}
}
