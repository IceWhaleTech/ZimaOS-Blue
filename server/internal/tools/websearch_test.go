package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTCP4Server(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skip test server setup (tcp4 unavailable): %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	return srv
}

type rewriteHostTransport struct {
	base   http.RoundTripper
	host   string
	scheme string
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.scheme
	cloned.URL.Host = t.host
	cloned.Host = t.host
	return base.RoundTrip(cloned)
}

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

func TestWebSearchTool_Execute_SupportsNestedCamelCaseArgs(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query != "nested query" {
			t.Fatalf("unexpected query: %s", query)
		}
		response := map[string]interface{}{
			"results": []map[string]interface{}{{
				"title":   "Nested Result",
				"url":     "https://example.com/nested",
				"content": "Description",
				"engine":  "searxng",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{Provider: "searxng", BaseURL: server.URL})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"q":          "nested query",
			"format":     "json",
			"maxResults": 1,
			"provider":   "searxng",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var response WebSearchResponse
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Query != "nested query" || len(response.Results) != 1 {
		t.Fatalf("unexpected response: %+v", response)
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
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestWebSearchTool_ProviderSettingsOverrideLegacyFields(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		response := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "Override Result",
					"url":     "https://example.com/override",
					"content": "Description",
					"engine":  "bing",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider: "searxng",
		BaseURL:  "http://127.0.0.1:1",
		ProviderSettings: map[string]WebSearchProviderSetting{
			"searxng": {BaseURL: server.URL},
		},
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":  "override test",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response WebSearchResponse
	if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].URL != "https://example.com/override" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestWebSearchTool_ProviderChainSkipsDisabledProviders(t *testing.T) {
	enabled := false
	tool := NewWebSearchTool(WebSearchConfig{
		Providers: []string{"bing", "duckduckgo"},
		ProviderSettings: map[string]WebSearchProviderSetting{
			"bing": {Enabled: &enabled},
		},
	})

	chain := tool.providerChain(nil)
	if len(chain) != 1 || chain[0] != "duckduckgo" {
		t.Fatalf("providerChain = %#v, want [duckduckgo]", chain)
	}
}

func TestWebSearchTool_Brave(t *testing.T) {
	// Create mock Brave Search server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestWebSearchTool_Bing(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "test query" {
			t.Fatalf("unexpected query: %q", got)
		}
		if got := r.URL.Query().Get("mkt"); got != "en-US" {
			t.Fatalf("unexpected mkt: %q", got)
		}
		if got := r.URL.Query().Get("cc"); got != "US" {
			t.Fatalf("unexpected cc: %q", got)
		}
		if got := r.URL.Query().Get("setlang"); got != "en" {
			t.Fatalf("unexpected setlang: %q", got)
		}
		if got := r.URL.Query().Get("adlt"); got != "strict" {
			t.Fatalf("unexpected adlt: %q", got)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`
<html><body><ol id="b_results">
  <li class="b_algo">
    <h2><a href="https://example.com/1">Test Result 1</a></h2>
    <div class="b_caption"><p>Description 1</p></div>
  </li>
  <li class="b_algo">
    <h2><a href="https://example.com/2">Test Result 2</a></h2>
    <div class="b_caption"><p>Description 2</p></div>
  </li>
</ol></body></html>`))
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider:   "bing",
		SafeSearch: true,
	})
	tool.httpClient = server.Client()

	response, err := tool.searchBingAtURL(context.Background(), "test query", 2, "us-en", server.URL+"/search")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Provider != "bing" {
		t.Fatalf("provider = %q, want %q", response.Provider, "bing")
	}
	if len(response.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(response.Results))
	}
	if response.Results[0].Title != "Test Result 1" {
		t.Fatalf("title = %q, want %q", response.Results[0].Title, "Test Result 1")
	}
	if response.Results[0].URL != "https://example.com/1" {
		t.Fatalf("url = %q, want %q", response.Results[0].URL, "https://example.com/1")
	}
	if response.Results[0].Description != "Description 1" {
		t.Fatalf("description = %q, want %q", response.Results[0].Description, "Description 1")
	}
}

func TestWebSearchTool_ProviderFallback(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestWebSearchTool_ProviderFanoutAggregatesResults(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			started <- "searxng"
			<-release
			response := map[string]interface{}{
				"results": []map[string]interface{}{
					{"title": "Shared Result", "url": "https://example.com/shared", "content": "Shared result from searxng", "engine": "searxng"},
					{"title": "SearXNG Result", "url": "https://example.com/searxng", "content": "Searxng unique", "engine": "searxng"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		case "/res/v1/web/search":
			if got := r.Header.Get("X-Subscription-Token"); got != "test-api-key" {
				t.Fatalf("unexpected brave token: %q", got)
			}
			started <- "brave"
			<-release
			response := map[string]interface{}{
				"web": map[string]interface{}{
					"results": []map[string]interface{}{
						{"title": "Shared Result", "url": "https://example.com/shared", "description": "Shared result from brave"},
						{"title": "Brave Result", "url": "https://example.com/brave", "description": "Brave unique"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	tool := NewWebSearchTool(WebSearchConfig{
		Providers: []string{"searxng", "brave"},
		BaseURL:   server.URL,
		APIKey:    "test-api-key",
		Timeout:   2 * time.Second,
	})
	client := server.Client()
	client.Transport = rewriteHostTransport{
		base:   client.Transport,
		host:   serverURL.Host,
		scheme: serverURL.Scheme,
	}
	tool.httpClient = client

	done := make(chan struct{})
	var raw interface{}
	go func() {
		defer close(done)
		raw, err = tool.Execute(context.Background(), map[string]interface{}{
			"query":       "fanout query",
			"format":      "json",
			"max_results": 3,
		})
	}()

	seen := map[string]struct{}{}
	for idx := 0; idx < 2; idx++ {
		select {
		case provider := <-started:
			seen[provider] = struct{}{}
		case <-time.After(2 * time.Second):
			t.Fatal("search providers did not fan out in parallel")
		}
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("search execute did not finish after releasing provider fanout")
	}
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("providers started = %v, want both providers", seen)
	}

	var response WebSearchResponse
	if err := json.Unmarshal([]byte(raw.(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Provider != "searxng,brave" {
		t.Fatalf("provider = %q, want %q", response.Provider, "searxng,brave")
	}
	if len(response.Results) != 3 {
		t.Fatalf("len(results) = %d, want 3 merged unique results", len(response.Results))
	}
	if response.Results[0].URL != "https://example.com/shared" {
		t.Fatalf("top url = %q, want shared merged result", response.Results[0].URL)
	}
}

func TestWebSearchTool_MaxResults(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestWebSearchTool_CacheHitAcrossFormats(t *testing.T) {
	var hits atomic.Int32
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		response := map[string]interface{}{
			"results": []map[string]interface{}{{
				"title":   "Cached Result",
				"url":     "https://example.com/cache",
				"content": "Description",
				"engine":  "searxng",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider:        "searxng",
		BaseURL:         server.URL,
		CacheTTL:        2 * time.Minute,
		CacheMaxEntries: 8,
	})

	first, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":  "cached query",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	second, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":  "cached query",
		"format": "xml",
	})
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}

	if got := hits.Load(); got != 1 {
		t.Fatalf("search backend calls = %d, want 1", got)
	}

	var response WebSearchResponse
	if err := json.Unmarshal([]byte(first.(string)), &response); err != nil {
		t.Fatalf("failed to unmarshal json response: %v", err)
	}
	if response.Query != "cached query" || len(response.Results) != 1 || response.Results[0].Title != "Cached Result" {
		t.Fatalf("unexpected cached json response: %+v", response)
	}
	if !strings.Contains(second.(string), "<web_search>") || !strings.Contains(second.(string), "Cached Result") {
		t.Fatalf("unexpected xml response: %s", second.(string))
	}
}

func TestWebSearchTool_CacheTTLExpiry(t *testing.T) {
	var hits atomic.Int32
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		response := map[string]interface{}{
			"results": []map[string]interface{}{{
				"title":   fmt.Sprintf("Result %d", n),
				"url":     fmt.Sprintf("https://example.com/cache/%d", n),
				"content": "Description",
				"engine":  "searxng",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider:        "searxng",
		BaseURL:         server.URL,
		CacheTTL:        200 * time.Millisecond,
		CacheMaxEntries: 8,
	})

	runSearch := func() WebSearchResponse {
		result, err := tool.Execute(context.Background(), map[string]interface{}{
			"query":  "ttl query",
			"format": "json",
		})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		var response WebSearchResponse
		if err := json.Unmarshal([]byte(result.(string)), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		return response
	}

	first := runSearch()
	second := runSearch()
	if got := hits.Load(); got != 1 {
		t.Fatalf("search backend calls before ttl expiry = %d, want 1", got)
	}
	if first.Results[0].Title != "Result 1" || second.Results[0].Title != "Result 1" {
		t.Fatalf("unexpected cached responses before ttl expiry: first=%+v second=%+v", first, second)
	}

	time.Sleep(300 * time.Millisecond)
	third := runSearch()
	if got := hits.Load(); got != 2 {
		t.Fatalf("search backend calls after ttl expiry = %d, want 2", got)
	}
	if third.Results[0].Title != "Result 2" {
		t.Fatalf("unexpected response after ttl expiry: %+v", third)
	}
}

func TestWebSearchTool_CacheDeduplicatesConcurrentIdenticalQueries(t *testing.T) {
	var hits atomic.Int32
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(120 * time.Millisecond)
		response := map[string]interface{}{
			"results": []map[string]interface{}{{
				"title":   "Concurrent Result",
				"url":     "https://example.com/concurrent",
				"content": "Description",
				"engine":  "searxng",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tool := NewWebSearchTool(WebSearchConfig{
		Provider:        "searxng",
		BaseURL:         server.URL,
		CacheTTL:        2 * time.Minute,
		CacheMaxEntries: 8,
	})

	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := tool.Execute(context.Background(), map[string]interface{}{
				"query":  "same concurrent query",
				"format": "json",
			})
			errCh <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent execute failed: %v", err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("search backend calls = %d, want 1", got)
	}
}

func TestWebSearchConfig_Defaults(t *testing.T) {
	tool := NewWebSearchTool(WebSearchConfig{})

	if tool.config.MaxResults != 5 {
		t.Errorf("expected default MaxResults 5, got %d", tool.config.MaxResults)
	}

	if tool.config.Timeout != 5*time.Minute {
		t.Errorf("expected default Timeout 5m, got %v", tool.config.Timeout)
	}

	if tool.config.Provider != "bing" {
		t.Errorf("expected default Provider 'bing', got '%s'", tool.config.Provider)
	}
	if len(tool.config.Providers) != 2 || tool.config.Providers[0] != "bing" || tool.config.Providers[1] != "duckduckgo" {
		t.Errorf("expected default Providers ['bing', 'duckduckgo'], got %#v", tool.config.Providers)
	}

	if tool.config.Region != "wt-wt" {
		t.Errorf("expected default Region 'wt-wt', got '%s'", tool.config.Region)
	}
	if tool.config.CacheTTL != 3*time.Minute {
		t.Errorf("expected default CacheTTL 3m, got %v", tool.config.CacheTTL)
	}
	if tool.config.CacheMaxEntries != 256 {
		t.Errorf("expected default CacheMaxEntries 256, got %d", tool.config.CacheMaxEntries)
	}
	if tool.config.BrowserFallback.Engine != "bing" {
		t.Errorf("expected default browser fallback engine 'bing', got %q", tool.config.BrowserFallback.Engine)
	}
	if tool.config.BrowserFallback.TriggerMode != "quality_or_failure" {
		t.Errorf("expected default browser fallback trigger_mode 'quality_or_failure', got %q", tool.config.BrowserFallback.TriggerMode)
	}
	if tool.config.BrowserFallback.MaxBrowserRetries != 1 {
		t.Errorf("expected default browser fallback retries 1, got %d", tool.config.BrowserFallback.MaxBrowserRetries)
	}
}
