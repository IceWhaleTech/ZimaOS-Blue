package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	xhtml "golang.org/x/net/html"
)

const (
	webSearchFormatJSON = "json"
	webSearchFormatXML  = "xml"

	webSearchProviderSettleWindow = 180 * time.Millisecond
	webSearchDefaultCacheTTL      = 3 * time.Minute
	webSearchDefaultCacheMax      = 256
)

var defaultWebSearchProviders = []string{"bing", "duckduckgo"}

const (
	defaultWebSearchBrowserFallbackEngine           = "bing"
	defaultWebSearchBrowserFallbackTriggerMode      = "quality_or_failure"
	defaultWebSearchBrowserFallbackQualityThreshold = 52.0
	defaultWebSearchBrowserFallbackMaxRetries       = 1
)

// WebSearchConfig holds configuration for the web search tool.
type WebSearchConfig struct {
	// Provider specifies the search provider to use.
	// Supported: "duckduckgo", "bing", "searxng", "brave"
	Provider string

	// Providers specifies a prioritized provider list for fallback (high availability).
	// If empty, Provider is used.
	Providers []string

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

	// CacheTTL controls the in-memory public search result cache.
	// CacheTTL=0 uses the default TTL; CacheTTL<0 disables caching.
	CacheTTL time.Duration

	// CacheMaxEntries limits the number of cached search queries.
	// Values <= 0 use the default limit.
	CacheMaxEntries int

	// BrowserFallback controls browser-backed public SERP rescue for web_query.
	BrowserFallback WebSearchBrowserFallbackConfig

	// ProviderSettings stores provider-specific advanced configuration overrides.
	ProviderSettings map[string]WebSearchProviderSetting
}

// WebSearchBrowserFallbackConfig controls browser-backed search rescue.
type WebSearchBrowserFallbackConfig struct {
	Enabled           *bool
	Engine            string
	TriggerMode       string
	QualityThreshold  float64
	MaxBrowserRetries int
}

// WebSearchProviderSetting stores provider-specific advanced settings.
type WebSearchProviderSetting struct {
	APIKey  string
	BaseURL string
	Enabled *bool
}

// WebSearchTool performs web searches using various search providers.
type WebSearchTool struct {
	config     WebSearchConfig
	httpClient *http.Client
	cacheMu    sync.Mutex
	cache      map[string]webSearchCacheEntry
	inFlight   map[string]*inflightWebSearchCall
}

type webSearchCacheEntry struct {
	response  *WebSearchResponse
	expiresAt time.Time
	updatedAt time.Time
}

type inflightWebSearchCall struct {
	done     chan struct{}
	response *WebSearchResponse
	err      error
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

type webSearchProviderOutcome struct {
	Provider string
	Response *WebSearchResponse
	Err      error
}

// NewWebSearchTool creates a new web search tool.
func NewWebSearchTool(config WebSearchConfig) *WebSearchTool {
	if config.MaxResults <= 0 {
		config.MaxResults = 5
	}
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}
	config.Provider = strings.ToLower(strings.TrimSpace(config.Provider))
	config.Providers = normalizeProviderList(config.Providers)
	if len(config.Providers) == 0 {
		if config.Provider == "" {
			config.Provider = defaultWebSearchProviders[0]
			config.Providers = append([]string(nil), defaultWebSearchProviders...)
		} else {
			config.Providers = []string{config.Provider}
		}
	} else if config.Provider == "" {
		config.Provider = config.Providers[0]
	}
	if config.Region == "" {
		config.Region = "wt-wt" // Worldwide
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = webSearchDefaultCacheTTL
	} else if config.CacheTTL < 0 {
		config.CacheTTL = 0
	}
	if config.CacheMaxEntries <= 0 {
		config.CacheMaxEntries = webSearchDefaultCacheMax
	}
	config.ProviderSettings = normalizeWebSearchProviderSettings(config.ProviderSettings)
	config.BrowserFallback = normalizeWebSearchBrowserFallbackConfig(config.BrowserFallback)

	return &WebSearchTool{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		cache:    make(map[string]webSearchCacheEntry),
		inFlight: make(map[string]*inflightWebSearchCall),
	}
}

// Definition returns the tool's definition.
func (w *WebSearchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "web_search",
		Description: "Keyword web search for discovery. Use when you need candidate links, official docs, recent sources, or do not yet have the exact URL. Returns result listings only and does NOT open pages. After choosing a URL, use web_fetch for a quick public read, web_read for normalized page reading, and browser as the final fallback for login/JS/interaction.",
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
					"description": "Maximum number of results to return (default: 5, max: 20)",
				},
				"region": map[string]interface{}{
					"type":        "string",
					"description": "Search region (e.g., 'us-en', 'uk-en', 'wt-wt' for worldwide)",
				},
				"provider": map[string]interface{}{
					"type":        "string",
					"description": "Optional provider override. Supports single provider or fallback list, e.g. 'duckduckgo,bing' or 'searxng,brave,duckduckgo'",
				},
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Output format: xml (default) or json",
					"enum":        []string{"xml", "json"},
					"default":     "xml",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Execute performs the web search.
func (w *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query := firstCompatString(args, "query", "q")
	if query == "" {
		return nil, errors.New("query is required")
	}

	formatRaw, _ := compatArgValue(args, "format", "output_format", "outputFormat")
	format, err := parseWebSearchFormat(formatRaw)
	if err != nil {
		return nil, err
	}

	maxResultsRaw, _ := compatArgValue(args, "max_results", "maxResults", "limit", "n")
	maxResults := parseWebSearchMaxResults(maxResultsRaw, w.config.MaxResults)

	region := w.config.Region
	if r := firstCompatString(args, "region"); r != "" {
		region = r
	}

	providerRaw, _ := compatArgValue(args, "provider")
	providers := w.providerChain(providerRaw)
	response, err := w.searchWithProvidersCached(ctx, providers, query, maxResults, region)
	if err != nil {
		return nil, err
	}
	return marshalWebSearchResponse(response, format)
}

func (w *WebSearchTool) searchWithProvidersCached(ctx context.Context, providers []string, query string, maxResults int, region string) (*WebSearchResponse, error) {
	if w == nil {
		return nil, errors.New("web search not configured")
	}
	if w.config.CacheTTL <= 0 {
		return w.searchWithProviders(ctx, providers, query, maxResults, region)
	}

	key := buildWebSearchCacheKey(providers, query, maxResults, region)
	now := time.Now()

	w.cacheMu.Lock()
	w.pruneCacheLocked(now)
	if cached, ok := w.cache[key]; ok && now.Before(cached.expiresAt) {
		resp := cloneWebSearchResponse(cached.response)
		w.cacheMu.Unlock()
		return resp, nil
	}
	if call, ok := w.inFlight[key]; ok {
		wait := call.done
		w.cacheMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-wait:
		}
		if call.err != nil {
			return nil, call.err
		}
		return cloneWebSearchResponse(call.response), nil
	}

	call := &inflightWebSearchCall{done: make(chan struct{})}
	w.inFlight[key] = call
	w.cacheMu.Unlock()

	response, err := w.searchWithProviders(ctx, providers, query, maxResults, region)
	completedAt := time.Now()

	w.cacheMu.Lock()
	delete(w.inFlight, key)
	if err == nil {
		cloned := cloneWebSearchResponse(response)
		call.response = cloned
		w.cache[key] = webSearchCacheEntry{
			response:  cloneWebSearchResponse(cloned),
			expiresAt: completedAt.Add(w.config.CacheTTL),
			updatedAt: completedAt,
		}
		w.pruneCacheLocked(completedAt)
	} else {
		call.err = err
	}
	close(call.done)
	w.cacheMu.Unlock()

	if err != nil {
		return nil, err
	}
	return cloneWebSearchResponse(response), nil
}

func buildWebSearchCacheKey(providers []string, query string, maxResults int, region string) string {
	return strings.Join([]string{
		strings.Join(providers, ","),
		strings.TrimSpace(query),
		strconv.Itoa(maxResults),
		strings.ToLower(strings.TrimSpace(region)),
	}, "|")
}

func cloneWebSearchResponse(resp *WebSearchResponse) *WebSearchResponse {
	if resp == nil {
		return nil
	}
	cloned := *resp
	if len(resp.Results) > 0 {
		cloned.Results = make([]WebSearchResult, len(resp.Results))
		copy(cloned.Results, resp.Results)
	}
	return &cloned
}

func (w *WebSearchTool) pruneCacheLocked(now time.Time) {
	for key, entry := range w.cache {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(w.cache, key)
		}
	}

	limit := w.config.CacheMaxEntries
	if limit <= 0 {
		limit = webSearchDefaultCacheMax
	}
	if len(w.cache) <= limit {
		return
	}

	type evictCandidate struct {
		key string
		ts  time.Time
	}
	candidates := make([]evictCandidate, 0, len(w.cache))
	for key, entry := range w.cache {
		candidates = append(candidates, evictCandidate{key: key, ts: entry.updatedAt})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ts.Before(candidates[j].ts)
	})

	excess := len(w.cache) - limit
	for i := 0; i < excess && i < len(candidates); i++ {
		delete(w.cache, candidates[i].key)
	}
}

func (w *WebSearchTool) searchWithProvider(ctx context.Context, provider, query string, maxResults int, region string) (*WebSearchResponse, error) {
	if !w.providerEnabled(provider) {
		return nil, fmt.Errorf("search provider %s is disabled", provider)
	}
	switch provider {
	case "duckduckgo":
		return w.searchDuckDuckGo(ctx, query, maxResults, region)
	case "bing":
		return w.searchBing(ctx, query, maxResults, region)
	case "searxng":
		return w.searchSearXNG(ctx, query, maxResults)
	case "brave":
		return w.searchBrave(ctx, query, maxResults)
	default:
		return nil, fmt.Errorf("unsupported search provider: %s", provider)
	}
}

func (w *WebSearchTool) providerChain(raw interface{}) []string {
	chain := parseProviderChainArg(raw)
	if len(chain) > 0 {
		return w.filterEnabledProviders(chain)
	}
	return w.filterEnabledProviders(w.config.Providers)
}

func (w *WebSearchTool) filterEnabledProviders(providers []string) []string {
	if len(providers) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(providers))
	for _, provider := range providers {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" || !w.providerEnabled(provider) {
			continue
		}
		filtered = append(filtered, provider)
	}
	return filtered
}

func (w *WebSearchTool) providerSetting(provider string) WebSearchProviderSetting {
	if w == nil {
		return WebSearchProviderSetting{}
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" || len(w.config.ProviderSettings) == 0 {
		return WebSearchProviderSetting{}
	}
	return w.config.ProviderSettings[provider]
}

func (w *WebSearchTool) providerEnabled(provider string) bool {
	setting := w.providerSetting(provider)
	if setting.Enabled != nil {
		return *setting.Enabled
	}
	return true
}

func (w *WebSearchTool) apiKeyForProvider(provider string) string {
	if apiKey := strings.TrimSpace(w.providerSetting(provider).APIKey); apiKey != "" {
		return apiKey
	}
	return strings.TrimSpace(w.config.APIKey)
}

func (w *WebSearchTool) baseURLForProvider(provider string) string {
	if baseURL := strings.TrimSpace(w.providerSetting(provider).BaseURL); baseURL != "" {
		return baseURL
	}
	return strings.TrimSpace(w.config.BaseURL)
}

func (w *WebSearchTool) browserFallbackConfig() WebSearchBrowserFallbackConfig {
	if w == nil {
		return normalizeWebSearchBrowserFallbackConfig(WebSearchBrowserFallbackConfig{})
	}
	return normalizeWebSearchBrowserFallbackConfig(w.config.BrowserFallback)
}

func (w *WebSearchTool) searchWithProviders(ctx context.Context, providers []string, query string, maxResults int, region string) (*WebSearchResponse, error) {
	if len(providers) == 0 {
		return nil, errors.New("no search providers configured")
	}
	if len(providers) == 1 {
		return w.searchWithProvider(ctx, providers[0], query, maxResults, region)
	}

	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan webSearchProviderOutcome, len(providers))
	for _, provider := range providers {
		provider := provider
		go func() {
			resp, err := w.searchWithProvider(searchCtx, provider, query, maxResults, region)
			results <- webSearchProviderOutcome{
				Provider: provider,
				Response: resp,
				Err:      err,
			}
		}()
	}

	var (
		successes   []webSearchProviderOutcome
		failures    []error
		settleTimer *time.Timer
	)
	for remaining := len(providers); remaining > 0; {
		var settle <-chan time.Time
		if settleTimer != nil {
			settle = settleTimer.C
		}

		select {
		case outcome := <-results:
			remaining--
			if outcome.Err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", outcome.Provider, outcome.Err))
				continue
			}
			successes = append(successes, outcome)
			if len(outcome.Response.Results) > 0 && settleTimer == nil && remaining > 0 {
				settleTimer = time.NewTimer(webSearchProviderSettleWindow)
			}
		case <-settle:
			cancel()
			settleTimer = nil
		}
	}
	if settleTimer != nil {
		settleTimer.Stop()
	}

	if len(successes) == 0 {
		if len(failures) == 1 {
			return nil, failures[0]
		}
		return nil, fmt.Errorf("all search providers failed (%s): %w", strings.Join(providers, ","), errors.Join(failures...))
	}
	if len(successes) == 1 {
		return successes[0].Response, nil
	}
	return mergeWebSearchResponses(query, maxResults, providers, successes), nil
}

func parseProviderChainArg(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return nil
	}
	return normalizeProviderList(strings.Split(value, ","))
}

func normalizeProviderList(providers []string) []string {
	if len(providers) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(providers))
	normalized := make([]string, 0, len(providers))
	for _, p := range providers {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		normalized = append(normalized, p)
	}
	return normalized
}

func normalizeWebSearchProviderSettings(settings map[string]WebSearchProviderSetting) map[string]WebSearchProviderSetting {
	if len(settings) == 0 {
		return nil
	}
	normalized := make(map[string]WebSearchProviderSetting, len(settings))
	for provider, setting := range settings {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}
		normalized[provider] = WebSearchProviderSetting{
			APIKey:  strings.TrimSpace(setting.APIKey),
			BaseURL: strings.TrimSpace(setting.BaseURL),
			Enabled: setting.Enabled,
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeWebSearchBrowserFallbackConfig(cfg WebSearchBrowserFallbackConfig) WebSearchBrowserFallbackConfig {
	cfg.Engine = normalizeBrowserSearchFallbackEngine(cfg.Engine)
	cfg.TriggerMode = strings.ToLower(strings.TrimSpace(cfg.TriggerMode))
	if cfg.TriggerMode == "" {
		cfg.TriggerMode = defaultWebSearchBrowserFallbackTriggerMode
	}
	if cfg.TriggerMode != defaultWebSearchBrowserFallbackTriggerMode {
		cfg.TriggerMode = defaultWebSearchBrowserFallbackTriggerMode
	}
	if cfg.QualityThreshold <= 0 {
		cfg.QualityThreshold = defaultWebSearchBrowserFallbackQualityThreshold
	}
	if cfg.MaxBrowserRetries <= 0 {
		cfg.MaxBrowserRetries = defaultWebSearchBrowserFallbackMaxRetries
	}
	if cfg.MaxBrowserRetries > 1 {
		cfg.MaxBrowserRetries = 1
	}
	return cfg
}

func normalizeBrowserSearchFallbackEngine(engine string) string {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "", "bing":
		return defaultWebSearchBrowserFallbackEngine
	default:
		return defaultWebSearchBrowserFallbackEngine
	}
}

func mergeWebSearchResponses(query string, maxResults int, providers []string, outcomes []webSearchProviderOutcome) *WebSearchResponse {
	type mergedResult struct {
		Result        WebSearchResult
		Hits          int
		BestProvider  int
		BestRank      int
		AggregateRank float64
	}

	providerOrder := make(map[string]int, len(providers))
	for idx, provider := range providers {
		providerOrder[strings.TrimSpace(provider)] = idx
	}

	merged := make(map[string]*mergedResult, len(outcomes)*maxResults)
	successfulProviderSet := make(map[string]struct{}, len(outcomes))
	for _, outcome := range outcomes {
		resp := outcome.Response
		if resp == nil {
			continue
		}
		providerName := firstNonEmpty(strings.TrimSpace(resp.Provider), strings.TrimSpace(outcome.Provider))
		if providerName != "" {
			successfulProviderSet[providerName] = struct{}{}
		}
		providerIdx := providerOrder[strings.TrimSpace(outcome.Provider)]
		for rank, result := range resp.Results {
			targetURL := strings.TrimSpace(result.URL)
			if targetURL == "" {
				continue
			}
			canonical, err := canonicalizeCrawlURL(targetURL)
			if err != nil {
				canonical = targetURL
			}
			entry, ok := merged[canonical]
			if !ok {
				entry = &mergedResult{
					Result:       result,
					BestProvider: providerIdx,
					BestRank:     rank + 1,
				}
				if strings.TrimSpace(entry.Result.Source) == "" {
					entry.Result.Source = providerName
				}
				merged[canonical] = entry
			}
			entry.Hits++
			entry.AggregateRank += float64((len(providers)-providerIdx)*32) + float64(maxResults-rank)*9
			if providerIdx < entry.BestProvider || (providerIdx == entry.BestProvider && rank+1 < entry.BestRank) {
				entry.BestProvider = providerIdx
				entry.BestRank = rank + 1
				entry.Result = result
				if strings.TrimSpace(entry.Result.Source) == "" {
					entry.Result.Source = providerName
				}
			} else {
				if strings.TrimSpace(entry.Result.Title) == "" && strings.TrimSpace(result.Title) != "" {
					entry.Result.Title = result.Title
				}
				if strings.TrimSpace(entry.Result.Description) == "" && strings.TrimSpace(result.Description) != "" {
					entry.Result.Description = result.Description
				}
				if strings.TrimSpace(entry.Result.Source) == "" {
					entry.Result.Source = providerName
				}
			}
		}
	}

	ranked := make([]mergedResult, 0, len(merged))
	for _, entry := range merged {
		ranked = append(ranked, *entry)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Hits == ranked[j].Hits {
			if ranked[i].AggregateRank == ranked[j].AggregateRank {
				if ranked[i].BestProvider == ranked[j].BestProvider {
					return ranked[i].BestRank < ranked[j].BestRank
				}
				return ranked[i].BestProvider < ranked[j].BestProvider
			}
			return ranked[i].AggregateRank > ranked[j].AggregateRank
		}
		return ranked[i].Hits > ranked[j].Hits
	})

	limit := maxResults
	if limit <= 0 {
		limit = len(ranked)
	}
	if limit > len(ranked) {
		limit = len(ranked)
	}
	results := make([]WebSearchResult, 0, limit)
	for idx := 0; idx < limit; idx++ {
		results = append(results, ranked[idx].Result)
	}
	successfulProviders := make([]string, 0, len(successfulProviderSet))
	for _, provider := range providers {
		if _, ok := successfulProviderSet[strings.TrimSpace(provider)]; ok {
			successfulProviders = append(successfulProviders, strings.TrimSpace(provider))
		}
	}

	return &WebSearchResponse{
		Query:      query,
		Results:    results,
		TotalCount: len(results),
		Provider:   strings.Join(successfulProviders, ","),
	}
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ZimaOS-Blue/1.0)")

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

// searchBing performs a search using Bing's HTML results page.
func (w *WebSearchTool) searchBing(ctx context.Context, query string, maxResults int, region string) (*WebSearchResponse, error) {
	return w.searchBingAtURL(ctx, query, maxResults, region, "https://www.bing.com/search")
}

func (w *WebSearchTool) searchBingAtURL(ctx context.Context, query string, maxResults int, region string, searchURL string) (*WebSearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", maxResults))
	if w.config.SafeSearch {
		params.Set("adlt", "strict")
	} else {
		params.Set("adlt", "off")
	}
	if market, country, language := bingLocaleForRegion(region); market != "" {
		params.Set("mkt", market)
		if country != "" {
			params.Set("cc", country)
		}
		if language != "" {
			params.Set("setlang", language)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ZimaOS-Blue/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

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

	results := parseBingHTML(string(body), maxResults)
	return &WebSearchResponse{
		Query:      query,
		Results:    results,
		TotalCount: len(results),
		Provider:   "bing",
	}, nil
}

func bingLocaleForRegion(region string) (market string, country string, language string) {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" || region == "wt-wt" {
		return "", "", ""
	}

	parts := strings.Split(region, "-")
	if len(parts) != 2 {
		return "", "", ""
	}

	country = strings.ToUpper(strings.TrimSpace(parts[0]))
	language = strings.ToLower(strings.TrimSpace(parts[1]))
	if len(country) != 2 || len(language) != 2 {
		return "", "", ""
	}

	return fmt.Sprintf("%s-%s", language, country), country, language
}

// parseBingHTML parses Bing HTML search results.
func parseBingHTML(rawHTML string, maxResults int) []WebSearchResult {
	doc, err := xhtml.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}

	var results []WebSearchResult
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node == nil || len(results) >= maxResults {
			return
		}
		if node.Type == xhtml.ElementNode && node.Data == "li" && webSearchHTMLNodeHasClass(node, "b_algo") {
			if result, ok := extractBingResult(node); ok {
				results = append(results, result)
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
			if len(results) >= maxResults {
				return
			}
		}
	}
	walk(doc)

	return results
}

func extractBingResult(node *xhtml.Node) (WebSearchResult, bool) {
	heading := firstWebSearchHTMLDescendant(node, func(n *xhtml.Node) bool {
		return n.Type == xhtml.ElementNode && n.Data == "h2"
	})
	if heading == nil {
		return WebSearchResult{}, false
	}

	link := firstWebSearchHTMLDescendant(heading, func(n *xhtml.Node) bool {
		return n.Type == xhtml.ElementNode && n.Data == "a" && strings.TrimSpace(webSearchHTMLAttr(n, "href")) != ""
	})
	if link == nil {
		return WebSearchResult{}, false
	}

	title := webSearchHTMLNodeText(link)
	resultURL := strings.TrimSpace(webSearchHTMLAttr(link, "href"))
	if title == "" || resultURL == "" {
		return WebSearchResult{}, false
	}

	description := ""
	if caption := firstWebSearchHTMLDescendant(node, func(n *xhtml.Node) bool {
		return n.Type == xhtml.ElementNode && webSearchHTMLNodeHasClass(n, "b_caption")
	}); caption != nil {
		description = webSearchHTMLNodeText(caption)
	}

	return WebSearchResult{
		Title:       title,
		URL:         resultURL,
		Description: description,
	}, true
}

func firstWebSearchHTMLDescendant(node *xhtml.Node, predicate func(*xhtml.Node) bool) *xhtml.Node {
	if node == nil {
		return nil
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if predicate(child) {
			return child
		}
		if match := firstWebSearchHTMLDescendant(child, predicate); match != nil {
			return match
		}
	}
	return nil
}

func webSearchHTMLAttr(node *xhtml.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func webSearchHTMLNodeHasClass(node *xhtml.Node, className string) bool {
	for _, token := range strings.Fields(webSearchHTMLAttr(node, "class")) {
		if token == className {
			return true
		}
	}
	return false
}

func webSearchHTMLNodeText(node *xhtml.Node) string {
	if node == nil {
		return ""
	}

	var builder strings.Builder
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current == nil {
			return
		}
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
			return
		}
		if current.Type == xhtml.ElementNode {
			switch current.Data {
			case "script", "style", "noscript":
				return
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)

	return strings.Join(strings.Fields(builder.String()), " ")
}

// searchSearXNG performs a search using a SearXNG instance.
func (w *WebSearchTool) searchSearXNG(ctx context.Context, query string, maxResults int) (*WebSearchResponse, error) {
	baseURL := w.baseURLForProvider("searxng")
	if baseURL == "" {
		return nil, errors.New("SearXNG base URL is required")
	}

	searchURL := strings.TrimSuffix(baseURL, "/") + "/search"

	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("pageno", "1")

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "ZimaOS-Blue/1.0")
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
	apiKey := w.apiKeyForProvider("brave")
	if apiKey == "" {
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

	req.Header.Set("X-Subscription-Token", apiKey)
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

func parseWebSearchFormat(raw interface{}) (string, error) {
	if raw == nil {
		return webSearchFormatXML, nil
	}

	value, ok := raw.(string)
	if !ok {
		return "", errors.New("format must be a string")
	}

	normalized, err := normalizeSearchFormat(value)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		return webSearchFormatXML, nil
	}
	return normalized, nil
}

func normalizeSearchFormat(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, ",.;:!?")
	if idx := strings.Index(value, ";"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}

	switch value {
	case "", webSearchFormatJSON, webSearchFormatXML:
		return value, nil
	case "md", "markdown", "text", "txt", "plain", "plaintext", "human", "jsonl", "application/json":
		return webSearchFormatJSON, nil
	case "application/xml", "text/xml":
		return webSearchFormatXML, nil
	default:
		return "", errors.New("format must be one of: json, xml")
	}
}

func parseWebSearchMaxResults(raw interface{}, fallback int) int {
	maxResults := fallback

	switch v := raw.(type) {
	case float64:
		if v > 0 {
			maxResults = int(v)
		}
	case float32:
		if v > 0 {
			maxResults = int(v)
		}
	case int:
		if v > 0 {
			maxResults = v
		}
	case int32:
		if v > 0 {
			maxResults = int(v)
		}
	case int64:
		if v > 0 {
			maxResults = int(v)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && parsed > 0 {
			maxResults = parsed
		}
	}

	if maxResults > 20 {
		return 20
	}
	return maxResults
}

func marshalWebSearchResponse(response *WebSearchResponse, format string) (string, error) {
	if format == webSearchFormatXML {
		return marshalWebSearchXML(response)
	}

	jsonResult, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to encode search response: %w", err)
	}
	return string(jsonResult), nil
}

type webSearchXMLResponse struct {
	XMLName    xml.Name             `xml:"web_search"`
	Query      string               `xml:"query"`
	Provider   string               `xml:"provider"`
	TotalCount int                  `xml:"total_count"`
	Results    []webSearchXMLResult `xml:"results>result"`
}

type webSearchXMLResult struct {
	Title       string `xml:"title"`
	URL         string `xml:"url"`
	Description string `xml:"description,omitempty"`
	Source      string `xml:"source,omitempty"`
}

func marshalWebSearchXML(response *WebSearchResponse) (string, error) {
	payload := webSearchXMLResponse{
		Query:      response.Query,
		Provider:   response.Provider,
		TotalCount: response.TotalCount,
		Results:    make([]webSearchXMLResult, len(response.Results)),
	}
	for i, result := range response.Results {
		payload.Results[i] = webSearchXMLResult{
			Title:       result.Title,
			URL:         result.URL,
			Description: result.Description,
			Source:      result.Source,
		}
	}

	encoded, err := xml.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode search response as XML: %w", err)
	}
	return string(encoded), nil
}
