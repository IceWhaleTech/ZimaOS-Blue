package deepresearch

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

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
