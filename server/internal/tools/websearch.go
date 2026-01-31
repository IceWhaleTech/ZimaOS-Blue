package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchConfig holds configuration for the web search tool.
type WebSearchConfig struct {
	// Provider specifies the search provider to use.
	// Supported: "duckduckgo", "searxng", "brave", "google"
	Provider string

	// APIKey is the API key for providers that require authentication.
	APIKey string

	// BaseURL is the base URL for self-hosted search engines (e.g., SearXNG).
	BaseURL string

	// MaxResults is the maximum number of results to return.
	MaxResults int

	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// SafeSearch enables safe search filtering.
	SafeSearch bool

	// Region specifies the search region (e.g., "us-en", "wt-wt" for worldwide).
	Region string
}

// WebSearchTool performs web searches using various search providers.
type WebSearchTool struct {
	config     WebSearchConfig
	httpClient *http.Client
}

// WebSearchResult represents a single search result.
type WebSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Source      string `json:"source,omitempty"`
}

// WebSearchResponse represents the search response.
type WebSearchResponse struct {
	Query      string            `json:"query"`
	Results    []WebSearchResult `json:"results"`
	TotalCount int               `json:"total_count"`
	Provider   string            `json:"provider"`
}

// NewWebSearchTool creates a new web search tool.
func NewWebSearchTool(config WebSearchConfig) *WebSearchTool {
	if config.MaxResults <= 0 {
		config.MaxResults = 10
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Provider == "" {
		config.Provider = "duckduckgo"
	}
	if config.Region == "" {
		config.Region = "wt-wt" // Worldwide
	}

	return &WebSearchTool{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Definition returns the tool's definition.
func (w *WebSearchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "Web Search",
		Description: "Searches the web for information. Returns a list of relevant web pages with titles, URLs, and descriptions.",
		Icon:        "web-search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return (default: 10, max: 20)",
				},
				"region": map[string]interface{}{
					"type":        "string",
					"description": "Search region (e.g., 'us-en', 'uk-en', 'wt-wt' for worldwide)",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Execute performs the web search.
func (w *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	maxResults := w.config.MaxResults
	if mr, ok := args["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
		if maxResults > 20 {
			maxResults = 20
		}
	}

	region := w.config.Region
	if r, ok := args["region"].(string); ok && r != "" {
		region = r
	}

	var response *WebSearchResponse
	var err error

	switch strings.ToLower(w.config.Provider) {
	case "duckduckgo":
		response, err = w.searchDuckDuckGo(ctx, query, maxResults, region)
	case "searxng":
		response, err = w.searchSearXNG(ctx, query, maxResults)
	case "brave":
		response, err = w.searchBrave(ctx, query, maxResults)
	default:
		return nil, fmt.Errorf("unsupported search provider: %s", w.config.Provider)
	}

	if err != nil {
		return nil, err
	}

	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

// searchDuckDuckGo performs a search using DuckDuckGo's HTML interface.
func (w *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string, maxResults int, region string) (*WebSearchResponse, error) {
	// Use DuckDuckGo's lite/html interface
	baseURL := "https://html.duckduckgo.com/html/"

	params := url.Values{}
	params.Set("q", query)
	params.Set("kl", region)
	if w.config.SafeSearch {
		params.Set("kp", "1")
	} else {
		params.Set("kp", "-1")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ZimaOS-Echo/1.0)")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	results := parseDuckDuckGoHTML(string(body), maxResults)

	return &WebSearchResponse{
		Query:      query,
		Results:    results,
		TotalCount: len(results),
		Provider:   "duckduckgo",
	}, nil
}

// parseDuckDuckGoHTML parses DuckDuckGo HTML response.
func parseDuckDuckGoHTML(html string, maxResults int) []WebSearchResult {
	var results []WebSearchResult

	// Simple HTML parsing for DuckDuckGo results
	// Look for result links in the format: <a rel="nofollow" class="result__a" href="...">
	parts := strings.Split(html, `class="result__a"`)

	for i := 1; i < len(parts) && len(results) < maxResults; i++ {
		part := parts[i]

		// Extract URL
		urlStart := strings.Index(part, `href="`)
		if urlStart == -1 {
			continue
		}
		urlStart += 6
		urlEnd := strings.Index(part[urlStart:], `"`)
		if urlEnd == -1 {
			continue
		}
		resultURL := part[urlStart : urlStart+urlEnd]

		// Decode DuckDuckGo redirect URL
		if strings.Contains(resultURL, "uddg=") {
			if decoded, err := url.QueryUnescape(resultURL); err == nil {
				if idx := strings.Index(decoded, "uddg="); idx != -1 {
					resultURL = decoded[idx+5:]
					if ampIdx := strings.Index(resultURL, "&"); ampIdx != -1 {
						resultURL = resultURL[:ampIdx]
					}
				}
			}
		}

		// Extract title
		titleStart := strings.Index(part, ">")
		if titleStart == -1 {
			continue
		}
		titleStart++
		titleEnd := strings.Index(part[titleStart:], "</a>")
		if titleEnd == -1 {
			continue
		}
		title := stripHTML(part[titleStart : titleStart+titleEnd])

		// Extract description
		description := ""
		snippetStart := strings.Index(part, `class="result__snippet"`)
		if snippetStart != -1 {
			snippetStart = strings.Index(part[snippetStart:], ">")
			if snippetStart != -1 {
				snippetStart += snippetStart + 1
				snippetEnd := strings.Index(part[snippetStart:], "</a>")
				if snippetEnd != -1 {
					description = stripHTML(part[snippetStart : snippetStart+snippetEnd])
				}
			}
		}

		if title != "" && resultURL != "" {
			results = append(results, WebSearchResult{
				Title:       title,
				URL:         resultURL,
				Description: description,
			})
		}
	}

	return results
}

// searchSearXNG performs a search using a SearXNG instance.
func (w *WebSearchTool) searchSearXNG(ctx context.Context, query string, maxResults int) (*WebSearchResponse, error) {
	if w.config.BaseURL == "" {
		return nil, errors.New("SearXNG base URL is required")
	}

	searchURL := strings.TrimSuffix(w.config.BaseURL, "/") + "/search"

	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("pageno", "1")

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "ZimaOS-Echo/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	var searxResp struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
			Engine  string `json:"engine"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searxResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []WebSearchResult
	for i, r := range searxResp.Results {
		if i >= maxResults {
			break
		}
		results = append(results, WebSearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Content,
			Source:      r.Engine,
		})
	}

	return &WebSearchResponse{
		Query:      query,
		Results:    results,
		TotalCount: len(results),
		Provider:   "searxng",
	}, nil
}

// searchBrave performs a search using Brave Search API.
func (w *WebSearchTool) searchBrave(ctx context.Context, query string, maxResults int) (*WebSearchResponse, error) {
	if w.config.APIKey == "" {
		return nil, errors.New("Brave Search API key is required")
	}

	searchURL := "https://api.search.brave.com/res/v1/web/search"

	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", maxResults))
	if w.config.SafeSearch {
		params.Set("safesearch", "strict")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Subscription-Token", w.config.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var braveResp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&braveResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []WebSearchResult
	for _, r := range braveResp.Web.Results {
		results = append(results, WebSearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Description,
		})
	}

	return &WebSearchResponse{
		Query:      query,
		Results:    results,
		TotalCount: len(results),
		Provider:   "brave",
	}, nil
}

// stripHTML removes HTML tags from a string.
func stripHTML(s string) string {
	var result strings.Builder
	inTag := false

	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}

	// Clean up whitespace and HTML entities
	text := result.String()
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.TrimSpace(text)

	return text
}
