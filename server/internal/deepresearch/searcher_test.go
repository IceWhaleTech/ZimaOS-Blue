package deepresearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestNewToolWebSearcher_TavilyEnvVarPassesAPIKey(t *testing.T) {
	// Mock Tavily API server that validates the API key is present in the request body.
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if body["api_key"] != "test-tavily-env-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"title": "Env Tavily Result", "url": "https://example.com/tavily-env", "content": "from env var"},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("TAVILY_API_KEY", "test-tavily-env-key")
	searcher := NewToolWebSearcher()

	// Override httpClient on the inner tool so requests go to our mock server.
	serverURL, _ := url.Parse(srv.URL)
	searcher.tool.SetHTTPClientForTest(&http.Client{
		Transport: &rewriteHostTransport{host: serverURL.Host, scheme: serverURL.Scheme},
	})

	hits, err := searcher.Search(context.Background(), "tavily env test", 5, "en")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("len(hits) = %d, want 1", len(hits))
	}
	if hits[0].Title != "Env Tavily Result" {
		t.Fatalf("hits[0].Title = %q, want %q", hits[0].Title, "Env Tavily Result")
	}
}

// rewriteHostTransport redirects all requests to a local test server.
type rewriteHostTransport struct {
	base   http.RoundTripper
	host   string
	scheme string
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
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

func TestToolWebSearcherSearch_RequestsJSONFormat(t *testing.T) {
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"title": "ZimaOS Blue",
					"url": "https://example.com/zimaos-blue",
					"content": "ZimaOS Blue search result",
					"engine": "searxng"
				}
			]
		}`))
	}))
	defer srv.Close()

	searcher := NewToolWebSearcherWithConfig(tools.WebSearchConfig{
		Provider:   "searxng",
		BaseURL:    srv.URL,
		MaxResults: 8,
		Timeout:    3 * time.Second,
		Region:     "wt-wt",
	})

	hits, err := searcher.Search(context.Background(), "ZimaOS Blue", 5, "en")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("len(hits) = %d, want 1", len(hits))
	}
	if hits[0].Title != "ZimaOS Blue" {
		t.Fatalf("hits[0].Title = %q, want %q", hits[0].Title, "ZimaOS Blue")
	}
	if hits[0].URL != "https://example.com/zimaos-blue" {
		t.Fatalf("hits[0].URL = %q, want %q", hits[0].URL, "https://example.com/zimaos-blue")
	}
}
