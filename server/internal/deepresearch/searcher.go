package deepresearch

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
	Search(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error)
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

func (s *ToolWebSearcher) Search(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
	region := searchRegionForLang(lang, query)
	result, err := s.tool.Execute(ctx, map[string]interface{}{
		"query":       query,
		"max_results": float64(maxResults),
		"region":      region,
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

func canonicalizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme == "" && u.Host == "") {
		return rawURL
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if u.Scheme == "http" {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	} else if u.Scheme == "https" {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}
	u.Fragment = ""

	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}

	q := u.Query()
	for key := range q {
		if isTrackingQueryParam(key) {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()

	return u.String()
}

func isTrackingQueryParam(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "utm_") {
		return true
	}
	switch name {
	case "fbclid", "gclid", "dclid", "msclkid", "yclid", "igshid", "mc_cid", "mc_eid", "ref", "ref_src", "source", "spm", "trk":
		return true
	default:
		return false
	}
}
