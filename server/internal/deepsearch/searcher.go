package deepsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type SearchHit struct {
	Title       string
	URL         string
	Description string
	Source      string
}

type Searcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]SearchHit, error)
}

type ToolWebSearcher struct {
	tool *tools.WebSearchTool
}

func NewToolWebSearcher() *ToolWebSearcher {
	return &ToolWebSearcher{
		tool: tools.NewWebSearchTool(tools.WebSearchConfig{
			Provider:   "duckduckgo",
			MaxResults: 8,
			Timeout:    15 * time.Second,
			Region:     "wt-wt",
		}),
	}
}

func (s *ToolWebSearcher) Search(ctx context.Context, query string, maxResults int) ([]SearchHit, error) {
	result, err := s.tool.Execute(ctx, map[string]interface{}{
		"query":       query,
		"max_results": float64(maxResults),
	})
	if err != nil {
		return nil, err
	}

	raw, ok := result.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected web search result type: %T", result)
	}

	var resp tools.WebSearchResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, err
	}

	hits := make([]SearchHit, 0, len(resp.Results))
	for _, r := range resp.Results {
		hits = append(hits, SearchHit{
			Title:       strings.TrimSpace(r.Title),
			URL:         strings.TrimSpace(r.URL),
			Description: strings.TrimSpace(r.Description),
			Source:      strings.TrimSpace(r.Source),
		})
	}
	return hits, nil
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	return host
}
