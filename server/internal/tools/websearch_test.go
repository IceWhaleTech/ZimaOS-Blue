package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
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
	if _, ok := params["format"]; !ok {
		t.Error("parameters should have 'format' property")
	}
	if _, ok := params["provider"]; !ok {
		t.Error("parameters should have 'provider' property")
	}
	formatProp, _ := params["format"].(map[string]interface{})
	if enumVals, ok := formatProp["enum"].([]string); !ok || len(enumVals) != 2 || enumVals[0] != "xml" || enumVals[1] != "json" {
		t.Errorf("format enum = %#v, want [xml json]", formatProp["enum"])
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
		"query":  "test query",
		"format": "json",
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

func TestWebSearchTool_ProviderFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "Fallback OK",
					"url":     "https://example.com/fallback",
					"content": "fallback",
					"engine":  "searxng",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider:  "unsupported",
		Providers: []string{"unsupported", "searxng"},
		BaseURL:   server.URL,
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":  "test query",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response WebSearchResponse
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Provider != "searxng" {
		t.Fatalf("provider = %q, want %q", response.Provider, "searxng")
	}
	if len(response.Results) != 1 || response.Results[0].Title != "Fallback OK" {
		t.Fatalf("unexpected response: %+v", response)
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
		"query":  "test",
		"format": "json",
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
		"format":      "json",
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
		"format":      "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	json.Unmarshal([]byte(result.(string)), &response)
	if len(response.Results) > 20 {
		t.Errorf("expected max 20 results (cap), got %d", len(response.Results))
	}

	// Test IPC-style string max_results
	result, err = tool.Execute(context.Background(), map[string]interface{}{
		"query":       "test",
		"max_results": "4",
		"format":      "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	json.Unmarshal([]byte(result.(string)), &response)
	if len(response.Results) > 4 {
		t.Errorf("expected max 4 results from string max_results, got %d", len(response.Results))
	}
}

func TestWebSearchTool_DefaultFormatXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "Test Result 1",
					"url":     "https://example.com/1",
					"content": "Description 1",
					"engine":  "google",
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

	xmlResult, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var parsed struct {
		XMLName    xml.Name `xml:"web_search"`
		Query      string   `xml:"query"`
		Provider   string   `xml:"provider"`
		TotalCount int      `xml:"total_count"`
		Results    []struct {
			Title string `xml:"title"`
		} `xml:"results>result"`
	}
	if err := xml.Unmarshal([]byte(xmlResult), &parsed); err != nil {
		t.Fatalf("failed to parse xml: %v", err)
	}
	if parsed.Query != "test query" {
		t.Fatalf("query = %q, want %q", parsed.Query, "test query")
	}
	if parsed.Provider != "searxng" {
		t.Fatalf("provider = %q, want %q", parsed.Provider, "searxng")
	}
	if parsed.TotalCount != 1 {
		t.Fatalf("total_count = %d, want 1", parsed.TotalCount)
	}
	if len(parsed.Results) != 1 || parsed.Results[0].Title != "Test Result 1" {
		t.Fatalf("unexpected results payload: %+v", parsed.Results)
	}
}

func TestWebSearchTool_InvalidFormat(t *testing.T) {
	tool := NewWebSearchTool(WebSearchConfig{Provider: "unsupported"})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":  "test",
		"format": "yaml",
	})
	if err == nil {
		t.Fatalf("expected invalid format error")
	}
	if !strings.Contains(err.Error(), "format must be one of: json, xml") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebSearchTool_FormatAliases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "Alias Result",
					"url":     "https://example.com/alias",
					"content": "Description",
					"engine":  "searxng",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider: "searxng",
		BaseURL:  server.URL,
	})

	cases := []struct {
		name          string
		format        string
		wantXMLResult bool
	}{
		{name: "markdown maps to json", format: "markdown", wantXMLResult: false},
		{name: "text maps to json", format: "text", wantXMLResult: false},
		{name: "json with punctuation", format: "json,", wantXMLResult: false},
		{name: "xml with punctuation", format: "xml.", wantXMLResult: true},
		{name: "mime json", format: "application/json; charset=utf-8", wantXMLResult: false},
		{name: "mime xml", format: "text/xml", wantXMLResult: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]interface{}{
				"query":  "format alias test",
				"format": tc.format,
			})
			if err != nil {
				t.Fatalf("unexpected error for format %q: %v", tc.format, err)
			}

			raw := result.(string)
			if tc.wantXMLResult {
				var parsed struct {
					XMLName xml.Name `xml:"web_search"`
					Query   string   `xml:"query"`
				}
				if err := xml.Unmarshal([]byte(raw), &parsed); err != nil {
					t.Fatalf("expected xml output for format %q: %v", tc.format, err)
				}
				return
			}

			var parsed WebSearchResponse
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				t.Fatalf("expected json output for format %q: %v", tc.format, err)
			}
			if parsed.Query != "format alias test" {
				t.Fatalf("query = %q, want %q", parsed.Query, "format alias test")
			}
		})
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

	if tool.config.MaxResults != 5 {
		t.Errorf("expected default MaxResults 5, got %d", tool.config.MaxResults)
	}

	if tool.config.Timeout != 30*time.Second {
		t.Errorf("expected default Timeout 30s, got %v", tool.config.Timeout)
	}

	if tool.config.Provider != "duckduckgo" {
		t.Errorf("expected default Provider 'duckduckgo', got '%s'", tool.config.Provider)
	}
	if len(tool.config.Providers) != 1 || tool.config.Providers[0] != "duckduckgo" {
		t.Errorf("expected default Providers ['duckduckgo'], got %#v", tool.config.Providers)
	}

	if tool.config.Region != "wt-wt" {
		t.Errorf("expected default Region 'wt-wt', got '%s'", tool.config.Region)
	}
}
