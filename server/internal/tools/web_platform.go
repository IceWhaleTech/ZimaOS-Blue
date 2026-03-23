package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	webReadFormatMarkdown = "markdown"
	webReadFormatText     = "text"

	webAccessLaneAuto         = "auto"
	webAccessLaneHTTP         = "http"
	webAccessLaneHTTPNative   = "http_native"
	webAccessLaneBrowser      = "browser"
	webAccessLaneProxyFetcher = "proxy_fetcher"

	webAccessSourceHTTP         = "http"
	webAccessSourceHTTPNative   = "http_native"
	webAccessSourceBrowser      = "browser"
	webAccessSourceProxyFetcher = "proxy_fetcher"

	webExtractModeText      = "text"
	webExtractModeHTML      = "html"
	webExtractModeAttribute = "attribute"

	webCrawlDefaultMaxDepth       = 1
	webCrawlDefaultMaxPages       = 20
	webCrawlDefaultConcurrency    = 4
	webCrawlDefaultRateLimit      = 500 * time.Millisecond
	webCrawlDefaultRetries        = 1
	webCrawlDefaultPageMaxChars   = 2_000
	webCrawlDefaultLinksPerPage   = 32
	webExtractEvidenceSnippetSize = 160
)

type webAccessRuntime struct {
	base *WebFetchTool
}

type webAccessWarning struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type webAccessDocument struct {
	URL                 string
	FinalURL            string
	Title               string
	Format              string
	Content             string
	Source              string
	Warnings            []webAccessWarning
	InteractiveRequired bool
	StatusCode          int
	Truncated           bool
	ContentType         string
	RawHTML             string
	Links               []string
	BodyTruncated       bool
	Extractor           string
	StrategyUsed        string
	SessionReused       bool
	AdapterID           string
	NetworkObserved     bool
	ChallengeState      *ChallengeState
}

type webAccessFetchOptions struct {
	format               string
	maxChars             int
	lane                 string
	request              webFetchRequestOptions
	wantRawHTML          bool
	allowBrowserFallback bool
	allowProxyFallback   bool
	browserReasonCode    string
	browserReason        string
}

type WebReadTool struct {
	runtime *webAccessRuntime
}

type WebExtractTool struct {
	runtime *webAccessRuntime
}

type WebCrawlTool struct {
	runtime *webAccessRuntime
}

type webReadResponse struct {
	URL                 string          `json:"url"`
	Title               string          `json:"title,omitempty"`
	FinalURL            string          `json:"final_url,omitempty"`
	Format              string          `json:"format"`
	Content             string          `json:"content,omitempty"`
	Source              string          `json:"source"`
	Warnings            []string        `json:"warnings,omitempty"`
	WarningCodes        []string        `json:"warning_codes,omitempty"`
	InteractiveRequired bool            `json:"interactive_required,omitempty"`
	StatusCode          int             `json:"status_code,omitempty"`
	Truncated           bool            `json:"truncated,omitempty"`
	StrategyUsed        string          `json:"strategy_used,omitempty"`
	SessionReused       bool            `json:"session_reused,omitempty"`
	AdapterID           string          `json:"adapter_id,omitempty"`
	NetworkObserved     bool            `json:"network_observed,omitempty"`
	ChallengeState      *ChallengeState `json:"challenge_state,omitempty"`
}

type webExtractFieldSpec struct {
	Selector     string
	Selectors    []string
	XPath        string
	XPaths       []string
	TextContains string
	NearbyText   string
	Attribute    string
	Mode         string
	Multiple     bool
	Required     bool
}

type webExtractEvidence struct {
	Strategy  string  `json:"strategy"`
	Locator   string  `json:"locator,omitempty"`
	NodePath  string  `json:"node_path,omitempty"`
	Snippet   string  `json:"snippet,omitempty"`
	Attribute string  `json:"attribute,omitempty"`
	Score     float64 `json:"score,omitempty"`
}

type webExtractResponse struct {
	URL           string                          `json:"url,omitempty"`
	FinalURL      string                          `json:"final_url,omitempty"`
	Title         string                          `json:"title,omitempty"`
	Source        string                          `json:"source,omitempty"`
	Data          map[string]interface{}          `json:"data"`
	Evidence      map[string][]webExtractEvidence `json:"evidence,omitempty"`
	Warnings      []string                        `json:"warnings,omitempty"`
	MissingFields []string                        `json:"missing_fields,omitempty"`
}

type webCrawlPage struct {
	URL                 string   `json:"url"`
	FinalURL            string   `json:"final_url,omitempty"`
	Title               string   `json:"title,omitempty"`
	Depth               int      `json:"depth"`
	StatusCode          int      `json:"status_code,omitempty"`
	Content             string   `json:"content,omitempty"`
	Source              string   `json:"source,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
	InteractiveRequired bool     `json:"interactive_required,omitempty"`
	Links               []string `json:"links,omitempty"`
	Truncated           bool     `json:"truncated,omitempty"`
}

type webCrawlEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type webCrawlFailure struct {
	URL       string `json:"url"`
	Depth     int    `json:"depth"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

type webCrawlCheckpoint struct {
	Pending   []webCrawlQueueItem `json:"pending,omitempty"`
	Seen      []string            `json:"seen,omitempty"`
	Completed int                 `json:"completed,omitempty"`
	CreatedAt string              `json:"created_at,omitempty"`
}

type webCrawlQueueItem struct {
	URL      string `json:"url"`
	Depth    int    `json:"depth"`
	Attempts int    `json:"attempts,omitempty"`
}

type webCrawlResponse struct {
	Pages      []webCrawlPage     `json:"pages,omitempty"`
	Edges      []webCrawlEdge     `json:"edges,omitempty"`
	Failures   []webCrawlFailure  `json:"failures,omitempty"`
	Checkpoint webCrawlCheckpoint `json:"checkpoint"`
	Stats      map[string]int     `json:"stats,omitempty"`
	Warnings   []string           `json:"warnings,omitempty"`
}

type webCrawlWorkerResult struct {
	Item      webCrawlQueueItem
	Doc       webAccessDocument
	Err       error
	Code      string
	Message   string
	Retryable bool
	Links     []string
}

type webCrawlLimiter struct {
	mu       sync.Mutex
	lastSeen map[string]time.Time
	interval time.Duration
}

type cssSelector struct {
	Steps       []cssSelectorStep
	Combinators []string
}

type cssSelectorStep struct {
	Tag     string
	ID      string
	Classes []string
	Attrs   []cssSelectorAttr
}

type cssSelectorAttr struct {
	Name     string
	Value    string
	HasValue bool
}

type xpathLiteExpr struct {
	AbsolutePath  []string
	Tag           string
	AnyTag        bool
	AttrName      string
	AttrValue     string
	ClassContains string
	TextContains  string
}

type scoredHTMLNode struct {
	Node  *html.Node
	Score float64
}

func newWebAccessRuntime(config WebFetchConfig) *webAccessRuntime {
	return &webAccessRuntime{base: NewWebFetchTool(config)}
}

func (r *webAccessRuntime) SetBrowser(browser BrowserBackend) {
	if r == nil || r.base == nil {
		return
	}
	r.base.SetBrowser(browser)
}

func (r *webAccessRuntime) SetPDFService(service PDFService) {
	if r == nil || r.base == nil {
		return
	}
	r.base.SetPDFService(service)
}

func NewWebReadTool(config WebFetchConfig) *WebReadTool {
	return &WebReadTool{runtime: newWebAccessRuntime(config)}
}

func NewWebExtractTool(config WebFetchConfig) *WebExtractTool {
	return &WebExtractTool{runtime: newWebAccessRuntime(config)}
}

func NewWebCrawlTool(config WebFetchConfig) *WebCrawlTool {
	return &WebCrawlTool{runtime: newWebAccessRuntime(config)}
}

func (t *WebReadTool) SetBrowser(browser BrowserBackend) {
	if t == nil || t.runtime == nil {
		return
	}
	t.runtime.SetBrowser(browser)
}

func (t *WebReadTool) SetPDFService(service PDFService) {
	if t == nil || t.runtime == nil {
		return
	}
	t.runtime.SetPDFService(service)
}

func (t *WebReadTool) SetDocumentReadService(service DocumentReadService) {
	if t == nil || t.runtime == nil || t.runtime.base == nil {
		return
	}
	t.runtime.base.SetDocumentReadService(service)
}

func (t *WebExtractTool) SetBrowser(browser BrowserBackend) {
	if t == nil || t.runtime == nil {
		return
	}
	t.runtime.SetBrowser(browser)
}

func (t *WebExtractTool) SetPDFService(service PDFService) {
	if t == nil || t.runtime == nil {
		return
	}
	t.runtime.SetPDFService(service)
}

func (t *WebCrawlTool) SetPDFService(service PDFService) {
	if t == nil || t.runtime == nil {
		return
	}
	t.runtime.SetPDFService(service)
}

func (t *WebReadTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "web_read",
		Description: "Default page-reading tool for a known URL when you want normalized main content. Supports headers/cookies, browser_target_id reuse, lane-aware reading, and downloadable PDFs or office-style documents. Prefer it over browser when you only need content; if warning_codes include login_wall, challenge, or browser_required, switch to browser.",
		Icon:        "web-search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":               map[string]interface{}{"type": "string", "description": "HTTP or HTTPS URL to read."},
				"format":            map[string]interface{}{"type": "string", "description": "Output format: markdown (default) or text.", "enum": []string{webReadFormatMarkdown, webReadFormatText}, "default": webReadFormatMarkdown},
				"max_chars":         map[string]interface{}{"type": "integer", "description": "Maximum characters in returned content.", "minimum": 100},
				"lane":              map[string]interface{}{"type": "string", "description": "Fetch lane preference: auto (default), http, browser, proxy_fetcher.", "enum": []string{webAccessLaneAuto, webAccessLaneHTTP, webAccessLaneBrowser, webAccessLaneProxyFetcher}, "default": webAccessLaneAuto},
				"headers":           map[string]interface{}{"type": "object", "description": "Optional request headers.", "additionalProperties": map[string]interface{}{"type": "string"}},
				"cookies":           map[string]interface{}{"type": "string", "description": "Optional Cookie header value."},
				"authorization":     map[string]interface{}{"type": "string", "description": "Optional Authorization header value."},
				"auth_bearer":       map[string]interface{}{"type": "string", "description": "Optional bearer token."},
				"browser_target_id": map[string]interface{}{"type": "string", "description": "Optional browser tab target ID used for cookie reuse or browser fallback."},
			},
			"required": []string{"url"},
		},
	}
}

func (t *WebReadTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.runtime == nil || t.runtime.base == nil {
		return nil, errors.New("web_read runtime not available")
	}
	rawURL := firstCompatString(args, "url", "href", "target", "input")
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("url is required")
	}
	format, err := parseWebReadFormat(args)
	if err != nil {
		return nil, err
	}
	maxChars := parseWebFetchMaxChars(args, t.runtime.base.config.MaxChars, t.runtime.base.config.MaxCharsCap)
	lane, err := parseWebAccessLane(args)
	if err != nil {
		return nil, err
	}
	reqOpts, err := parseWebFetchRequestOptions(args)
	if err != nil {
		return nil, err
	}
	disableInternalFallbacks := parseBooleanFlag(args, "disable_internal_fallbacks", "disableInternalFallbacks")
	doc, err := t.runtime.fetch(ctx, rawURL, webAccessFetchOptions{
		format:               format,
		maxChars:             maxChars,
		lane:                 lane,
		request:              reqOpts,
		allowBrowserFallback: !disableInternalFallbacks,
		allowProxyFallback:   !disableInternalFallbacks,
	})
	if err != nil {
		return nil, err
	}
	return marshalWebReadResponse(webReadResponse{
		URL:                 rawURL,
		Title:               doc.Title,
		FinalURL:            doc.FinalURL,
		Format:              format,
		Content:             doc.Content,
		Source:              doc.Source,
		Warnings:            warningMessages(doc.Warnings),
		WarningCodes:        warningCodes(doc.Warnings),
		InteractiveRequired: doc.InteractiveRequired,
		StatusCode:          doc.StatusCode,
		Truncated:           doc.Truncated,
		StrategyUsed:        doc.StrategyUsed,
		SessionReused:       doc.SessionReused,
		AdapterID:           doc.AdapterID,
		NetworkObserved:     doc.NetworkObserved,
		ChallengeState:      cloneChallengeState(doc.ChallengeState),
	})
}

func (t *WebExtractTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "web_extract",
		Description: "Extract structured fields from webpage HTML using clean-room selectors, XPath-lite, text anchors, and locator recovery with evidence.",
		Icon:        "web-search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":               map[string]interface{}{"type": "string", "description": "Optional URL to fetch before extraction."},
				"html":              map[string]interface{}{"type": "string", "description": "Optional raw HTML to extract from directly."},
				"fields":            map[string]interface{}{"type": "object", "description": "Field map. Each field can declare selector/selectors, xpath/xpaths, text_contains, nearby_text, attribute, mode, multiple, required."},
				"lane":              map[string]interface{}{"type": "string", "description": "Fetch lane when url is provided. browser/proxy_fetcher are rejected for structured extraction because v1 requires HTML-capable input.", "enum": []string{webAccessLaneAuto, webAccessLaneHTTP}, "default": webAccessLaneAuto},
				"headers":           map[string]interface{}{"type": "object", "description": "Optional request headers.", "additionalProperties": map[string]interface{}{"type": "string"}},
				"cookies":           map[string]interface{}{"type": "string", "description": "Optional Cookie header value."},
				"authorization":     map[string]interface{}{"type": "string", "description": "Optional Authorization header value."},
				"auth_bearer":       map[string]interface{}{"type": "string", "description": "Optional bearer token."},
				"browser_target_id": map[string]interface{}{"type": "string", "description": "Optional browser tab target ID used for cookie reuse on HTTP requests."},
			},
			"required": []string{"fields"},
		},
	}
}

func (t *WebExtractTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.runtime == nil || t.runtime.base == nil {
		return nil, errors.New("web_extract runtime not available")
	}
	fields, err := parseWebExtractFieldSpecs(args)
	if err != nil {
		return nil, err
	}
	rawHTML := strings.TrimSpace(firstCompatString(args, "html"))
	rawURL := strings.TrimSpace(firstCompatString(args, "url", "href", "target", "input"))
	if rawHTML == "" && rawURL == "" {
		return nil, errors.New("url or html is required")
	}
	resp := webExtractResponse{Data: make(map[string]interface{}), Evidence: make(map[string][]webExtractEvidence)}
	if rawHTML == "" {
		lane, err := parseWebAccessLane(args)
		if err != nil {
			return nil, err
		}
		if lane == webAccessLaneBrowser || lane == webAccessLaneProxyFetcher {
			return nil, errors.New("web_extract currently requires an HTML-capable lane; use auto/http or pass html directly")
		}
		reqOpts, err := parseWebFetchRequestOptions(args)
		if err != nil {
			return nil, err
		}
		doc, err := t.runtime.fetch(ctx, rawURL, webAccessFetchOptions{
			format:               webReadFormatText,
			maxChars:             t.runtime.base.config.MaxCharsCap,
			lane:                 lane,
			request:              reqOpts,
			wantRawHTML:          true,
			allowBrowserFallback: false,
			allowProxyFallback:   false,
		})
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(doc.RawHTML) == "" {
			return nil, errors.New("web_extract could not obtain HTML from the target URL")
		}
		rawHTML = doc.RawHTML
		resp.URL = rawURL
		resp.FinalURL = doc.FinalURL
		resp.Title = doc.Title
		resp.Source = doc.Source
		resp.Warnings = warningMessages(doc.Warnings)
	}
	root, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	if resp.Title == "" {
		resp.Title = strings.TrimSpace(findTagText(root, "title"))
	}
	var missing []string
	for name, spec := range fields {
		nodes, evidence := selectNodesForField(root, spec)
		if len(nodes) == 0 {
			if spec.Required {
				missing = append(missing, name)
			}
			continue
		}
		resp.Evidence[name] = evidence
		value := extractFieldValue(nodes, spec)
		if value != nil {
			resp.Data[name] = value
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		resp.MissingFields = missing
		resp.Warnings = append(resp.Warnings, "some required fields could not be resolved")
	}
	return marshalWebExtractResponse(resp)
}

func (t *WebCrawlTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "web_crawl",
		Description: "Crawl a set of seed URLs with host-aware limits, deduplication, checkpoint/resume, normalized page results, and discovered edges.",
		Icon:        "web-search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"seeds":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Seed URLs to start crawling from."},
				"allowed_hosts":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional host allowlist. Defaults to seed hosts."},
				"max_depth":         map[string]interface{}{"type": "integer", "minimum": 0, "description": "Maximum crawl depth."},
				"max_pages":         map[string]interface{}{"type": "integer", "minimum": 1, "description": "Maximum number of completed pages."},
				"max_concurrency":   map[string]interface{}{"type": "integer", "minimum": 1, "description": "Maximum concurrent fetches."},
				"max_retries":       map[string]interface{}{"type": "integer", "minimum": 0, "description": "Retries for retryable fetch failures."},
				"rate_limit_ms":     map[string]interface{}{"type": "integer", "minimum": 0, "description": "Minimum delay between requests to the same host."},
				"page_max_chars":    map[string]interface{}{"type": "integer", "minimum": 100, "description": "Maximum content chars stored per page."},
				"lane":              map[string]interface{}{"type": "string", "description": "Fetch lane. v1 crawl uses HTML-capable lanes only.", "enum": []string{webAccessLaneAuto, webAccessLaneHTTP}, "default": webAccessLaneAuto},
				"checkpoint":        map[string]interface{}{"type": "object", "description": "Optional checkpoint returned by a previous web_crawl run."},
				"headers":           map[string]interface{}{"type": "object", "description": "Optional request headers.", "additionalProperties": map[string]interface{}{"type": "string"}},
				"cookies":           map[string]interface{}{"type": "string", "description": "Optional Cookie header value."},
				"authorization":     map[string]interface{}{"type": "string", "description": "Optional Authorization header value."},
				"auth_bearer":       map[string]interface{}{"type": "string", "description": "Optional bearer token."},
				"browser_target_id": map[string]interface{}{"type": "string", "description": "Optional browser tab target ID used for cookie reuse on HTTP requests."},
			},
		},
	}
}

func (t *WebCrawlTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.runtime == nil || t.runtime.base == nil {
		return nil, errors.New("web_crawl runtime not available")
	}
	lane, err := parseWebAccessLane(args)
	if err != nil {
		return nil, err
	}
	if lane == webAccessLaneBrowser || lane == webAccessLaneProxyFetcher {
		return nil, errors.New("web_crawl currently requires an HTML-capable lane; use auto/http")
	}
	reqOpts, err := parseWebFetchRequestOptions(args)
	if err != nil {
		return nil, err
	}
	maxDepth := parsePositiveIntArg(args, []string{"max_depth", "maxDepth", "depth"}, webCrawlDefaultMaxDepth, 8)
	maxPages := parsePositiveIntArg(args, []string{"max_pages", "maxPages", "limit"}, webCrawlDefaultMaxPages, 500)
	maxConcurrency := parsePositiveIntArg(args, []string{"max_concurrency", "maxConcurrency"}, webCrawlDefaultConcurrency, 16)
	maxRetries := parsePositiveIntArg(args, []string{"max_retries", "maxRetries"}, webCrawlDefaultRetries, 5)
	pageMaxChars := parsePositiveIntArg(args, []string{"page_max_chars", "pageMaxChars"}, webCrawlDefaultPageMaxChars, t.runtime.base.config.MaxCharsCap)
	rateLimitMs := parsePositiveIntArg(args, []string{"rate_limit_ms", "rateLimitMs"}, int(webCrawlDefaultRateLimit/time.Millisecond), int((10*time.Second)/time.Millisecond))
	allowedHosts := parseStringListArg(args, "allowed_hosts", "allowedHosts", "hosts")
	checkpoint, err := parseWebCrawlCheckpoint(firstCompatRawValue(args, "checkpoint"))
	if err != nil {
		return nil, err
	}
	queue := append([]webCrawlQueueItem(nil), checkpoint.Pending...)
	if len(queue) == 0 {
		seeds := parseStringListArg(args, "seeds", "urls")
		if len(seeds) == 0 {
			if seed := firstCompatString(args, "url"); seed != "" {
				seeds = []string{seed}
			}
		}
		if len(seeds) == 0 {
			return nil, errors.New("seeds or checkpoint is required")
		}
		for _, seed := range seeds {
			canonical, err := canonicalizeCrawlURL(seed)
			if err != nil {
				continue
			}
			queue = append(queue, webCrawlQueueItem{URL: canonical, Depth: 0})
		}
	}
	if len(allowedHosts) == 0 {
		allowedHosts = deriveAllowedHosts(queue)
	}
	seen := make(map[string]struct{}, len(checkpoint.Seen)+len(queue))
	for _, item := range checkpoint.Seen {
		seen[item] = struct{}{}
	}
	for _, item := range queue {
		seen[item.URL] = struct{}{}
	}
	limiter := &webCrawlLimiter{lastSeen: make(map[string]time.Time), interval: time.Duration(rateLimitMs) * time.Millisecond}
	jobs := make(chan webCrawlQueueItem)
	results := make(chan webCrawlWorkerResult, maxConcurrency)
	var workers sync.WaitGroup
	for i := 0; i < maxConcurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				results <- t.runCrawlJob(ctx, item, lane, reqOpts, pageMaxChars, allowedHosts, limiter)
			}
		}()
	}
	completed := checkpoint.Completed
	inFlight := 0
	pages := make([]webCrawlPage, 0, maxPages)
	edges := make([]webCrawlEdge, 0, maxPages*2)
	failures := make([]webCrawlFailure, 0)
	warnings := make([]string, 0)
	dispatch := func(item webCrawlQueueItem) bool {
		if completed+inFlight >= maxPages {
			return false
		}
		jobs <- item
		inFlight++
		return true
	}
	for {
		for inFlight < maxConcurrency && len(queue) > 0 && completed+inFlight < maxPages {
			item := queue[0]
			queue = queue[1:]
			if !dispatch(item) {
				break
			}
		}
		if inFlight == 0 {
			break
		}
		select {
		case <-ctx.Done():
			warnings = append(warnings, ctx.Err().Error())
			inFlight = 0
			queue = append([]webCrawlQueueItem(nil), queue...)
			close(jobs)
			workers.Wait()
			return marshalWebCrawlResponse(webCrawlResponse{
				Pages:      pages,
				Edges:      edges,
				Failures:   failures,
				Checkpoint: webCrawlCheckpoint{Pending: queue, Seen: sortedSeen(seen), Completed: completed, CreatedAt: checkpointCreatedAt(checkpoint)},
				Stats:      map[string]int{"completed": completed, "failed": len(failures), "queued": len(queue)},
				Warnings:   warnings,
			})
		case result := <-results:
			inFlight--
			if result.Err != nil {
				if result.Retryable && result.Item.Attempts < maxRetries {
					result.Item.Attempts++
					queue = append(queue, result.Item)
					continue
				}
				failures = append(failures, webCrawlFailure{URL: result.Item.URL, Depth: result.Item.Depth, Code: result.Code, Message: result.Message, Retryable: result.Retryable})
				completed++
				continue
			}
			page := webCrawlPage{
				URL:                 result.Item.URL,
				FinalURL:            result.Doc.FinalURL,
				Title:               result.Doc.Title,
				Depth:               result.Item.Depth,
				StatusCode:          result.Doc.StatusCode,
				Content:             result.Doc.Content,
				Source:              result.Doc.Source,
				Warnings:            warningMessages(result.Doc.Warnings),
				InteractiveRequired: result.Doc.InteractiveRequired,
				Links:               trimStringList(result.Links, webCrawlDefaultLinksPerPage),
				Truncated:           result.Doc.Truncated,
			}
			pages = append(pages, page)
			completed++
			if result.Item.Depth >= maxDepth {
				continue
			}
			for _, link := range result.Links {
				canonical, err := canonicalizeCrawlURL(link)
				if err != nil || canonical == "" {
					continue
				}
				if !crawlHostAllowed(canonical, allowedHosts) {
					continue
				}
				edges = append(edges, webCrawlEdge{From: result.Item.URL, To: canonical})
				if _, ok := seen[canonical]; ok {
					continue
				}
				seen[canonical] = struct{}{}
				queue = append(queue, webCrawlQueueItem{URL: canonical, Depth: result.Item.Depth + 1})
			}
		}
	}
	close(jobs)
	workers.Wait()
	return marshalWebCrawlResponse(webCrawlResponse{
		Pages:    pages,
		Edges:    edges,
		Failures: failures,
		Checkpoint: webCrawlCheckpoint{
			Pending:   queue,
			Seen:      sortedSeen(seen),
			Completed: completed,
			CreatedAt: checkpointCreatedAt(checkpoint),
		},
		Stats: map[string]int{
			"completed": completed,
			"failed":    len(failures),
			"queued":    len(queue),
			"edges":     len(edges),
		},
		Warnings: warnings,
	})
}

func (t *WebCrawlTool) runCrawlJob(ctx context.Context, item webCrawlQueueItem, lane string, reqOpts webFetchRequestOptions, pageMaxChars int, allowedHosts []string, limiter *webCrawlLimiter) webCrawlWorkerResult {
	if !crawlHostAllowed(item.URL, allowedHosts) {
		return webCrawlWorkerResult{Item: item, Err: errors.New("host not allowed"), Code: "disallowed_host", Message: "host is not allowed", Retryable: false}
	}
	if err := limiter.Wait(ctx, item.URL); err != nil {
		return webCrawlWorkerResult{Item: item, Err: err, Code: "cancelled", Message: err.Error(), Retryable: false}
	}
	doc, err := t.runtime.fetch(ctx, item.URL, webAccessFetchOptions{
		format:               webReadFormatText,
		maxChars:             pageMaxChars,
		lane:                 lane,
		request:              reqOpts,
		wantRawHTML:          true,
		allowBrowserFallback: false,
		allowProxyFallback:   false,
	})
	if err != nil {
		code, retryable := classifyCrawlError(err)
		return webCrawlWorkerResult{Item: item, Err: err, Code: code, Message: err.Error(), Retryable: retryable}
	}
	return webCrawlWorkerResult{Item: item, Doc: doc, Links: doc.Links}
}

func (l *webCrawlLimiter) Wait(ctx context.Context, targetURL string) error {
	if l == nil || l.interval <= 0 {
		return nil
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return nil
	}
	for {
		l.mu.Lock()
		last := l.lastSeen[host]
		now := time.Now()
		wait := last.Add(l.interval).Sub(now)
		if wait <= 0 {
			l.lastSeen[host] = now
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *webAccessRuntime) fetch(ctx context.Context, rawURL string, opts webAccessFetchOptions) (webAccessDocument, error) {
	if r == nil || r.base == nil {
		return webAccessDocument{}, errors.New("web runtime not available")
	}
	normalizedURL, err := normalizeWebFetchURL(rawURL)
	if err != nil {
		return webAccessDocument{}, err
	}
	if err := guardWebFetchURL(ctx, normalizedURL, r.base.config.AllowPrivateHosts); err != nil {
		return webAccessDocument{}, err
	}
	if opts.request.extraHeaders == nil {
		opts.request.extraHeaders = make(map[string]string)
	}
	if err := r.base.applyBrowserSessionCookies(ctx, normalizedURL, &opts.request); err != nil {
		return webAccessDocument{}, err
	}
	format := opts.format
	if format == "" {
		format = webReadFormatMarkdown
	}
	lane := opts.lane
	if lane == "" {
		lane = webAccessLaneAuto
	}
	if !opts.wantRawHTML && r.base.orchestrator != nil && r.base.orchestrator.enabled() {
		preferredLane := lane
		if preferredLane == webAccessLaneAuto {
			preferredLane = r.base.preferredReadLane(ctx, normalizedURL, opts.request.browserTargetID)
		}
		result, err := r.base.orchestrator.Fetch(ctx, FetchRequest{
			URL:               normalizedURL,
			Mode:              format,
			MaxChars:          opts.maxChars,
			PreferredLane:     preferredLane,
			Options:           opts.request,
			AllowBrowser:      r.base.browser != nil,
			AllowProxy:        len(r.base.config.ProxyFetcherProviders) > 0,
			AllowSession:      true,
			AllowAutoFallback: lane == webAccessLaneAuto && (opts.allowBrowserFallback || opts.allowProxyFallback),
			BrowserReasonCode: opts.browserReasonCode,
			BrowserReason:     opts.browserReason,
		})
		if err == nil {
			content, truncated := truncateWebFetchContent(result.Payload.Content, opts.maxChars)
			doc := webAccessDocument{
				URL:             normalizedURL,
				FinalURL:        firstNonEmpty(result.Payload.URL, normalizedURL),
				Title:           result.Payload.Title,
				Format:          format,
				Content:         content,
				Source:          firstNonEmpty(result.Payload.Source, strategySourceForFetchResult(result)),
				StatusCode:      statusCodeForChallenge(result.ChallengeState),
				Truncated:       truncated || result.Payload.BodyTruncated,
				ContentType:     result.Payload.ContentType,
				BodyTruncated:   result.Payload.BodyTruncated,
				Extractor:       result.Payload.Extractor,
				StrategyUsed:    result.StrategyUsed,
				SessionReused:   result.SessionReused,
				AdapterID:       result.AdapterID,
				NetworkObserved: result.NetworkObserved,
				ChallengeState:  cloneChallengeState(result.ChallengeState),
			}
			if doc.ChallengeState != nil {
				doc.InteractiveRequired = doc.ChallengeState.RequiresBrowser
				if strings.TrimSpace(doc.ChallengeState.Message) != "" {
					doc.Warnings = append(doc.Warnings, webAccessWarning{
						Code:    firstNonEmpty(doc.ChallengeState.Code, doc.ChallengeState.Kind),
						Message: doc.ChallengeState.Message,
					})
				}
			}
			if strings.TrimSpace(result.Payload.Warning) != "" {
				doc.Warnings = append(doc.Warnings, webAccessWarning{
					Code:    result.Payload.WarningCode,
					Message: result.Payload.Warning,
				})
			}
			return doc, nil
		}
		if lane != webAccessLaneAuto {
			return webAccessDocument{}, err
		}
	}
	if lane == webAccessLaneBrowser {
		if opts.wantRawHTML {
			return webAccessDocument{}, errors.New("browser lane is not supported when raw HTML is required")
		}
		return r.fetchViaBrowser(ctx, normalizedURL, format, opts.request.browserTargetID, opts.browserReasonCode, opts.browserReason, opts.maxChars)
	}
	if lane == webAccessLaneProxyFetcher {
		if opts.wantRawHTML {
			return webAccessDocument{}, errors.New("proxy_fetcher lane is not supported when raw HTML is required")
		}
		return r.fetchViaProxy(ctx, normalizedURL, format, opts.maxChars)
	}
	if lane == webAccessLaneHTTPNative {
		return r.fetchViaHTTPBackend(ctx, normalizedURL, opts, webAccessLaneHTTPNative)
	}
	doc, err := r.fetchViaHTTP(ctx, normalizedURL, opts)
	if err != nil {
		if opts.allowProxyFallback && !opts.wantRawHTML {
			if proxyDoc, proxyErr := r.fetchViaProxy(ctx, normalizedURL, format, opts.maxChars); proxyErr == nil {
				return proxyDoc, nil
			}
		}
		return webAccessDocument{}, err
	}
	if opts.allowBrowserFallback && !opts.wantRawHTML && r.base.browser != nil && (doc.InteractiveRequired || looksLikeDynamicShell(doc.RawHTML, doc.Content)) {
		reasonCode := opts.browserReasonCode
		reason := opts.browserReason
		if reasonCode == "" && len(doc.Warnings) > 0 {
			reasonCode = doc.Warnings[0].Code
			reason = doc.Warnings[0].Message
		}
		browserDoc, browserErr := r.fetchViaBrowser(ctx, normalizedURL, format, opts.request.browserTargetID, reasonCode, reason, opts.maxChars)
		if browserErr == nil {
			return browserDoc, nil
		}
	}
	doc.Content, doc.Truncated = truncateWebFetchContent(doc.Content, opts.maxChars)
	return doc, nil
}

func (r *webAccessRuntime) fetchViaHTTP(ctx context.Context, normalizedURL string, opts webAccessFetchOptions) (webAccessDocument, error) {
	primary := webAccessLaneHTTP
	secondary := ""
	if r != nil && r.base != nil {
		if r.base.shouldPreferHTTPNative(normalizedURL) && r.base.canUseHTTPNative() {
			primary = webAccessLaneHTTPNative
			secondary = webAccessLaneHTTP
		} else if r.base.canUseHTTPNative() {
			secondary = webAccessLaneHTTPNative
		}
	}

	doc, err := r.fetchViaHTTPBackend(ctx, normalizedURL, opts, primary)
	if err != nil {
		if secondary == "" || (primary != webAccessLaneHTTPNative && !r.base.shouldRetryHTTPNativeOnError(err)) {
			return webAccessDocument{}, err
		}
		fallbackDoc, fallbackErr := r.fetchViaHTTPBackend(ctx, normalizedURL, opts, secondary)
		if fallbackErr != nil {
			return webAccessDocument{}, fmt.Errorf("%w (secondary %s failed: %v)", err, secondary, fallbackErr)
		}
		doc = fallbackDoc
	}
	if doc.Source != webAccessSourceHTTPNative && r.base.shouldRetryHTTPNativeOnResponse(webFetchHTTPResult{
		FinalURL:      doc.FinalURL,
		StatusCode:    doc.StatusCode,
		ContentType:   doc.ContentType,
		Body:          []byte(doc.RawHTML),
		BodyTruncated: doc.BodyTruncated,
		Source:        doc.Source,
	}, webFetchPayload{
		URL:         doc.FinalURL,
		Title:       doc.Title,
		Content:     doc.Content,
		Extractor:   doc.Extractor,
		Source:      doc.Source,
		WarningCode: firstWarningCode(doc.Warnings),
	}) {
		nativeDoc, nativeErr := r.fetchViaHTTPBackend(ctx, normalizedURL, opts, webAccessLaneHTTPNative)
		if nativeErr == nil && preferHTTPNativeDocument(doc, nativeDoc) {
			doc = nativeDoc
		}
	}
	return doc, nil
}

func (r *webAccessRuntime) fetchViaHTTPBackend(ctx context.Context, normalizedURL string, opts webAccessFetchOptions, backend string) (webAccessDocument, error) {
	if r == nil || r.base == nil {
		return webAccessDocument{}, errors.New("web runtime not available")
	}
	acceptHeader := "text/html,application/xhtml+xml,text/markdown;q=0.9,text/plain;q=0.8,application/pdf;q=0.7,application/vnd.openxmlformats-officedocument.wordprocessingml.document;q=0.7,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet;q=0.7,application/vnd.openxmlformats-officedocument.presentationml.presentation;q=0.7,application/msword;q=0.6,application/vnd.ms-excel;q=0.6,application/vnd.ms-powerpoint;q=0.6,application/vnd.oasis.opendocument.text;q=0.6,application/vnd.oasis.opendocument.spreadsheet;q=0.6,application/vnd.oasis.opendocument.presentation;q=0.6,application/rtf;q=0.6,text/rtf;q=0.6,*/*;q=0.2"
	result, err := r.base.fetchHTTPResultViaBackend(ctx, normalizedURL, opts.request, acceptHeader, backend)
	if err != nil {
		return webAccessDocument{}, fmt.Errorf("failed to fetch URL: %w", err)
	}
	if err := guardWebFetchURL(ctx, result.FinalURL, r.base.config.AllowPrivateHosts); err != nil {
		return webAccessDocument{}, err
	}
	content, title, extractor, extractedTruncated, err := r.base.extractContent(ctx, result.FinalURL, result.ContentType, result.Body, opts.format, result.BodyTruncated)
	if err != nil {
		return webAccessDocument{}, err
	}
	doc := webAccessDocument{
		URL:           normalizedURL,
		FinalURL:      result.FinalURL,
		Title:         title,
		Format:        opts.format,
		Content:       content,
		Source:        firstNonEmpty(strings.TrimSpace(result.Source), webAccessSourceHTTP),
		StatusCode:    result.StatusCode,
		ContentType:   result.ContentType,
		BodyTruncated: result.BodyTruncated || extractedTruncated,
		Extractor:     extractor,
		StrategyUsed:  firstNonEmpty(strings.TrimSpace(result.Source), webAccessSourceHTTP),
	}
	if strings.Contains(strings.ToLower(result.ContentType), "html") || looksLikeHTMLBody(result.Body) {
		bodyText := string(result.Body)
		doc.RawHTML = bodyText
		if doc.Title == "" {
			doc.Title = strings.TrimSpace(findTitleInHTML(bodyText))
		}
		doc.Links = extractLinksFromHTML(bodyText, result.FinalURL)
	}
	hit, code, warning := detectWebFetchAuthWall(result.StatusCode, result.FinalURL, doc.Title, doc.Content)
	if hit {
		doc.InteractiveRequired = true
		doc.Warnings = append(doc.Warnings, webAccessWarning{Code: code, Message: warning})
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		if !hit {
			doc.Warnings = append(doc.Warnings, webAccessWarning{Code: "http_status", Message: fmt.Sprintf("HTTP %d returned from target", result.StatusCode)})
		}
	}
	if result.BodyTruncated {
		doc.Warnings = append(doc.Warnings, webAccessWarning{Code: "body_truncated", Message: "response body reached the configured byte limit"})
	}
	return doc, nil
}

func (r *webAccessRuntime) fetchViaBrowser(ctx context.Context, normalizedURL, format, targetID, reasonCode, reason string, maxChars int) (webAccessDocument, error) {
	payload, err := r.base.fetchViaBrowserSession(ctx, normalizedURL, format, targetID, reasonCode, reason)
	if err != nil {
		return webAccessDocument{}, err
	}
	content, truncated := truncateWebFetchContent(payload.Content, maxChars)
	doc := webAccessDocument{
		URL:             normalizedURL,
		FinalURL:        payload.URL,
		Title:           payload.Title,
		Format:          format,
		Content:         content,
		Source:          webAccessSourceBrowser,
		Truncated:       truncated,
		ContentType:     payload.ContentType,
		Extractor:       payload.Extractor,
		StrategyUsed:    webFetchStrategyBrowser,
		SessionReused:   payload.SessionReused,
		AdapterID:       payload.AdapterID,
		NetworkObserved: payload.NetworkObserved,
		ChallengeState:  cloneChallengeState(payload.ChallengeState),
	}
	if strings.TrimSpace(payload.Warning) != "" {
		doc.Warnings = append(doc.Warnings, webAccessWarning{Code: payload.WarningCode, Message: payload.Warning})
	}
	return doc, nil
}

func (r *webAccessRuntime) fetchViaProxy(ctx context.Context, normalizedURL, format string, maxChars int) (webAccessDocument, error) {
	payload, err := r.base.tryProxyFetchFamily(ctx, normalizedURL, format)
	if err != nil {
		return webAccessDocument{}, err
	}
	content, truncated := truncateWebFetchContent(payload.Content, maxChars)
	return webAccessDocument{
		URL:             normalizedURL,
		FinalURL:        payload.URL,
		Title:           payload.Title,
		Format:          format,
		Content:         content,
		Source:          webAccessSourceProxyFetcher,
		Truncated:       truncated,
		ContentType:     payload.ContentType,
		Extractor:       payload.Extractor,
		StrategyUsed:    webFetchStrategyProxy,
		AdapterID:       payload.AdapterID,
		NetworkObserved: payload.NetworkObserved,
		ChallengeState:  cloneChallengeState(payload.ChallengeState),
	}, nil
}

func strategySourceForFetchResult(result FetchResult) string {
	switch strings.TrimSpace(result.StrategyUsed) {
	case webFetchStrategyBrowser:
		return webAccessSourceBrowser
	case webFetchStrategyProxy:
		return webAccessSourceProxyFetcher
	case webAccessLaneHTTPNative:
		return webAccessSourceHTTPNative
	default:
		if strings.TrimSpace(result.Payload.Source) != "" {
			return result.Payload.Source
		}
		return webAccessSourceHTTP
	}
}

func statusCodeForChallenge(state *ChallengeState) int {
	if state == nil {
		return http.StatusOK
	}
	switch state.Kind {
	case webFetchChallengeKindLogin:
		return http.StatusUnauthorized
	case webFetchChallengeKindBrowserRequired:
		return http.StatusForbidden
	case webFetchChallengeKindHard, webFetchChallengeKindSoft:
		return http.StatusTooManyRequests
	default:
		return http.StatusOK
	}
}

func parseWebReadFormat(args map[string]interface{}) (string, error) {
	format := strings.ToLower(strings.TrimSpace(firstCompatString(args, "format", "extract_mode", "extractMode", "mode")))
	if format == "" {
		format = webReadFormatMarkdown
	}
	switch format {
	case webReadFormatMarkdown, webReadFormatText:
		return format, nil
	default:
		return "", fmt.Errorf("format must be one of: %s, %s", webReadFormatMarkdown, webReadFormatText)
	}
}

func parseWebAccessLane(args map[string]interface{}) (string, error) {
	lane := strings.ToLower(strings.TrimSpace(firstCompatString(args, "lane", "transport", "source_preference")))
	if lane == "" {
		return webAccessLaneAuto, nil
	}
	switch lane {
	case webAccessLaneAuto, webAccessLaneHTTP, webAccessLaneHTTPNative, webAccessLaneBrowser, webAccessLaneProxyFetcher, webFetchStrategySession:
		return lane, nil
	default:
		return "", fmt.Errorf("lane must be one of: %s, %s, %s, %s", webAccessLaneAuto, webAccessLaneHTTP, webAccessLaneBrowser, webAccessLaneProxyFetcher)
	}
}

func marshalWebReadResponse(resp webReadResponse) (string, error) {
	b, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func marshalWebExtractResponse(resp webExtractResponse) (string, error) {
	b, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func marshalWebCrawlResponse(resp webCrawlResponse) (string, error) {
	b, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func warningMessages(warnings []webAccessWarning) []string {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if strings.TrimSpace(warning.Message) != "" {
			out = append(out, strings.TrimSpace(warning.Message))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func warningCodes(warnings []webAccessWarning) []string {
	if len(warnings) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(warnings))
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		code := strings.TrimSpace(warning.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstWarningCode(warnings []webAccessWarning) string {
	for _, warning := range warnings {
		code := strings.TrimSpace(warning.Code)
		if code != "" {
			return code
		}
	}
	return ""
}

func preferHTTPNativeDocument(current, native webAccessDocument) bool {
	if native.Source != webAccessSourceHTTPNative {
		return false
	}
	if current.Source != webAccessSourceHTTPNative && current.InteractiveRequired && !native.InteractiveRequired {
		return true
	}
	currentLen := len([]rune(strings.TrimSpace(current.Content)))
	nativeLen := len([]rune(strings.TrimSpace(native.Content)))
	if current.Extractor == "html" && currentLen < webFetchMinReadableChars && nativeLen > currentLen {
		return true
	}
	return nativeLen > currentLen+32
}

func looksLikeDynamicShell(rawHTML, content string) bool {
	if strings.TrimSpace(rawHTML) == "" {
		return false
	}
	if len([]rune(strings.TrimSpace(content))) >= webFetchMinReadableChars {
		return false
	}
	lower := strings.ToLower(rawHTML)
	signals := 0
	for _, marker := range []string{"id=\"__next\"", "id=\"root\"", "id=\"app\"", "data-reactroot", "window.__nuxt", "__next_data__", "hydrate(", "webpack", "enable javascript", "loading..."} {
		if strings.Contains(lower, marker) {
			signals++
		}
	}
	if strings.Count(lower, "<script") >= 6 {
		signals++
	}
	return signals >= 2
}

func findTitleInHTML(raw string) string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	return findTagText(doc, "title")
}

func extractLinksFromHTML(raw, base string) []string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return nil
	}
	baseURL, _ := url.Parse(base)
	seen := make(map[string]struct{})
	links := make([]string, 0, 16)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "a") {
			href := strings.TrimSpace(nodeAttr(node, "href"))
			if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(strings.ToLower(href), "javascript:") {
				resolved := href
				if baseURL != nil {
					if ref, err := url.Parse(href); err == nil {
						resolved = baseURL.ResolveReference(ref).String()
					}
				}
				if resolved != "" {
					if _, ok := seen[resolved]; !ok {
						seen[resolved] = struct{}{}
						links = append(links, resolved)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return links
}

func parseWebExtractFieldSpecs(args map[string]interface{}) (map[string]webExtractFieldSpec, error) {
	raw, ok := compatArgValue(args, "fields", "schema")
	if raw == nil {
		return nil, errors.New("fields is required")
	}
	typed, ok := raw.(map[string]interface{})
	if !ok {
		return nil, errors.New("fields must be an object")
	}
	out := make(map[string]webExtractFieldSpec, len(typed))
	for name, value := range typed {
		fieldName := strings.TrimSpace(name)
		if fieldName == "" {
			continue
		}
		spec, err := parseWebExtractFieldSpec(value)
		if err != nil {
			return nil, fmt.Errorf("fields.%s: %w", fieldName, err)
		}
		out[fieldName] = spec
	}
	if len(out) == 0 {
		return nil, errors.New("fields cannot be empty")
	}
	return out, nil
}

func parseWebExtractFieldSpec(raw interface{}) (webExtractFieldSpec, error) {
	if selector, ok := raw.(string); ok {
		trimmed := strings.TrimSpace(selector)
		if trimmed == "" {
			return webExtractFieldSpec{}, errors.New("selector cannot be empty")
		}
		return webExtractFieldSpec{Selector: trimmed, Selectors: []string{trimmed}, Mode: webExtractModeText}, nil
	}
	typed, ok := raw.(map[string]interface{})
	if !ok {
		return webExtractFieldSpec{}, errors.New("field spec must be a string or object")
	}
	spec := webExtractFieldSpec{
		Selector:     strings.TrimSpace(firstCompatString(typed, "selector", "css")),
		XPath:        strings.TrimSpace(firstCompatString(typed, "xpath")),
		TextContains: strings.TrimSpace(firstCompatString(typed, "text_contains", "textContains", "text", "anchor_text", "anchorText")),
		NearbyText:   strings.TrimSpace(firstCompatString(typed, "nearby_text", "nearbyText", "label", "label_text")),
		Attribute:    strings.TrimSpace(firstCompatString(typed, "attribute", "attr")),
		Mode:         strings.ToLower(strings.TrimSpace(firstCompatString(typed, "mode"))),
		Multiple:     compatBool(typed["multiple"]),
		Required:     compatBool(typed["required"]),
	}
	spec.Selectors = appendSelectorValue(nil, typed["selectors"])
	spec.XPaths = appendSelectorValue(nil, typed["xpaths"])
	if spec.Selector != "" {
		spec.Selectors = appendUniqueString(spec.Selectors, spec.Selector)
	}
	if spec.XPath != "" {
		spec.XPaths = appendUniqueString(spec.XPaths, spec.XPath)
	}
	if spec.Mode == "" {
		if spec.Attribute != "" {
			spec.Mode = webExtractModeAttribute
		} else {
			spec.Mode = webExtractModeText
		}
	}
	switch spec.Mode {
	case webExtractModeText, webExtractModeHTML, webExtractModeAttribute:
	default:
		return webExtractFieldSpec{}, fmt.Errorf("unsupported mode %q", spec.Mode)
	}
	if len(spec.Selectors) == 0 && len(spec.XPaths) == 0 && spec.TextContains == "" && spec.NearbyText == "" {
		return webExtractFieldSpec{}, errors.New("at least one of selector, xpath, text_contains, or nearby_text is required")
	}
	return spec, nil
}

func appendSelectorValue(dst []string, raw interface{}) []string {
	items, ok := coerceCompatStringList(raw)
	if !ok {
		return dst
	}
	for _, item := range items {
		dst = appendUniqueString(dst, asString(item))
	}
	return dst
}

func appendUniqueString(dst []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return dst
	}
	for _, existing := range dst {
		if existing == value {
			return dst
		}
	}
	return append(dst, value)
}

func compatBool(raw interface{}) bool {
	switch typed := raw.(type) {
	case bool:
		return typed
	case string:
		value := strings.ToLower(strings.TrimSpace(typed))
		return value == "true" || value == "1" || value == "yes"
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func parseBooleanFlag(args map[string]interface{}, keys ...string) bool {
	raw, ok := compatArgValue(args, keys...)
	if !ok {
		return false
	}
	return compatBool(raw)
}

func selectNodesForField(root *html.Node, spec webExtractFieldSpec) ([]*html.Node, []webExtractEvidence) {
	if root == nil {
		return nil, nil
	}
	for _, selector := range spec.Selectors {
		nodes := findNodesByCSSSelector(root, selector)
		if len(nodes) == 0 {
			continue
		}
		return dedupeHTMLNodes(nodes), buildFieldEvidence(nodes, "css", selector, spec.Attribute, 0)
	}
	for _, expr := range spec.XPaths {
		nodes := findNodesByXPathLite(root, expr)
		if len(nodes) == 0 {
			continue
		}
		return dedupeHTMLNodes(nodes), buildFieldEvidence(nodes, "xpath", expr, spec.Attribute, 0)
	}
	if spec.TextContains != "" {
		nodes := findNodesByTextAnchor(root, spec.TextContains)
		if len(nodes) > 0 {
			return dedupeHTMLNodes(nodes), buildFieldEvidence(nodes, "text", spec.TextContains, spec.Attribute, 0)
		}
	}
	if spec.NearbyText != "" {
		nodes := findNodesNearText(root, spec)
		if len(nodes) > 0 {
			return dedupeHTMLNodes(nodes), buildFieldEvidence(nodes, "anchor", spec.NearbyText, spec.Attribute, 0)
		}
	}
	recovered := recoverNodesForField(root, spec)
	if len(recovered) == 0 {
		return nil, nil
	}
	nodes := make([]*html.Node, 0, len(recovered))
	evidence := make([]webExtractEvidence, 0, len(recovered))
	for _, candidate := range recovered {
		nodes = append(nodes, candidate.Node)
		evidence = append(evidence, webExtractEvidence{
			Strategy:  "recovered",
			NodePath:  htmlNodePath(candidate.Node),
			Snippet:   previewString(htmlNodeText(candidate.Node), webExtractEvidenceSnippetSize),
			Attribute: spec.Attribute,
			Score:     candidate.Score,
		})
		if !spec.Multiple {
			break
		}
	}
	return dedupeHTMLNodes(nodes), evidence
}

func extractFieldValue(nodes []*html.Node, spec webExtractFieldSpec) interface{} {
	if len(nodes) == 0 {
		return nil
	}
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		value := strings.TrimSpace(extractNodeValue(node, spec))
		if value == "" {
			continue
		}
		values = append(values, value)
		if !spec.Multiple {
			break
		}
	}
	if len(values) == 0 {
		return nil
	}
	if spec.Multiple {
		return values
	}
	return values[0]
}

func extractNodeValue(node *html.Node, spec webExtractFieldSpec) string {
	if node == nil {
		return ""
	}
	switch spec.Mode {
	case webExtractModeHTML:
		return renderNodeHTML(node)
	case webExtractModeAttribute:
		if spec.Attribute == "" {
			return ""
		}
		return nodeAttr(node, spec.Attribute)
	default:
		if spec.Attribute != "" {
			if attr := nodeAttr(node, spec.Attribute); strings.TrimSpace(attr) != "" {
				return attr
			}
		}
		return htmlNodeText(node)
	}
}

func renderNodeHTML(node *html.Node) string {
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	if node.Type == html.ElementNode {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			_ = html.Render(&buf, child)
		}
		return strings.TrimSpace(buf.String())
	}
	_ = html.Render(&buf, node)
	return strings.TrimSpace(buf.String())
}

func htmlNodeText(node *html.Node) string {
	writer := &readableWriter{}
	writeReadableText(writer, node)
	return cleanTextBlocks(writer.String())
}

func buildFieldEvidence(nodes []*html.Node, strategy, locator, attribute string, score float64) []webExtractEvidence {
	if len(nodes) == 0 {
		return nil
	}
	evidence := make([]webExtractEvidence, 0, len(nodes))
	for _, node := range nodes {
		evidence = append(evidence, webExtractEvidence{
			Strategy:  strategy,
			Locator:   locator,
			NodePath:  htmlNodePath(node),
			Snippet:   previewString(htmlNodeText(node), webExtractEvidenceSnippetSize),
			Attribute: attribute,
			Score:     score,
		})
	}
	return evidence
}

func dedupeHTMLNodes(nodes []*html.Node) []*html.Node {
	if len(nodes) <= 1 {
		return nodes
	}
	seen := make(map[*html.Node]struct{}, len(nodes))
	out := make([]*html.Node, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if _, ok := seen[node]; ok {
			continue
		}
		seen[node] = struct{}{}
		out = append(out, node)
	}
	return out
}

func findNodesByTextAnchor(root *html.Node, text string) []*html.Node {
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return nil
	}
	matches := make([]*html.Node, 0)
	for _, node := range collectElementNodes(root) {
		content := strings.ToLower(htmlNodeText(node))
		if content == "" || !strings.Contains(content, needle) {
			continue
		}
		matches = append(matches, node)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return len(htmlNodeText(matches[i])) < len(htmlNodeText(matches[j]))
	})
	return matches
}

func findNodesNearText(root *html.Node, spec webExtractFieldSpec) []*html.Node {
	anchors := findNodesByTextAnchor(root, spec.NearbyText)
	if len(anchors) == 0 {
		return nil
	}
	hints := deriveFieldHints(spec)
	var out []*html.Node
	for _, anchor := range anchors {
		for _, candidate := range nearbyCandidates(anchor) {
			if scoreNodeHints(candidate, hints, spec) < 25 {
				continue
			}
			out = append(out, candidate)
		}
	}
	return dedupeHTMLNodes(out)
}

func recoverNodesForField(root *html.Node, spec webExtractFieldSpec) []scoredHTMLNode {
	hints := deriveFieldHints(spec)
	if hints.tag == "" && hints.id == "" && len(hints.classes) == 0 && spec.TextContains == "" && spec.NearbyText == "" {
		return nil
	}
	candidates := make([]scoredHTMLNode, 0)
	for _, node := range collectElementNodes(root) {
		score := scoreNodeHints(node, hints, spec)
		if score < 30 {
			continue
		}
		candidates = append(candidates, scoredHTMLNode{Node: node, Score: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return len(htmlNodeText(candidates[i].Node)) < len(htmlNodeText(candidates[j].Node))
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

type fieldHints struct {
	tag     string
	id      string
	classes []string
}

func deriveFieldHints(spec webExtractFieldSpec) fieldHints {
	hints := fieldHints{}
	consumeSelector := func(raw string) {
		parsed := parseCSSSelector(raw)
		if len(parsed.Steps) == 0 {
			return
		}
		step := parsed.Steps[len(parsed.Steps)-1]
		if hints.tag == "" {
			hints.tag = step.Tag
		}
		if hints.id == "" {
			hints.id = step.ID
		}
		for _, className := range step.Classes {
			hints.classes = appendUniqueString(hints.classes, className)
		}
	}
	for _, selector := range spec.Selectors {
		consumeSelector(selector)
	}
	for _, expr := range spec.XPaths {
		parsed := parseXPathLite(expr)
		if hints.tag == "" && !parsed.AnyTag {
			hints.tag = parsed.Tag
		}
		if hints.id == "" && parsed.AttrName == "id" {
			hints.id = parsed.AttrValue
		}
		if parsed.ClassContains != "" {
			hints.classes = appendUniqueString(hints.classes, parsed.ClassContains)
		}
	}
	return hints
}

func scoreNodeHints(node *html.Node, hints fieldHints, spec webExtractFieldSpec) float64 {
	if node == nil || node.Type != html.ElementNode {
		return 0
	}
	var score float64
	if hints.tag != "" && strings.EqualFold(node.Data, hints.tag) {
		score += 35
	}
	if hints.id != "" && strings.EqualFold(nodeAttr(node, "id"), hints.id) {
		score += 50
	}
	classValue := strings.ToLower(nodeAttr(node, "class"))
	for _, className := range hints.classes {
		if strings.Contains(classValue, strings.ToLower(className)) {
			score += 12
		}
	}
	text := strings.ToLower(htmlNodeText(node))
	if spec.TextContains != "" && strings.Contains(text, strings.ToLower(spec.TextContains)) {
		score += 25
	}
	if spec.NearbyText != "" && containsNearbyText(node, spec.NearbyText) {
		score += 20
	}
	if spec.Attribute != "" && strings.TrimSpace(nodeAttr(node, spec.Attribute)) != "" {
		score += 10
	}
	return score
}

func containsNearbyText(node *html.Node, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" || node == nil {
		return false
	}
	for _, candidate := range nearbyCandidates(node) {
		if strings.Contains(strings.ToLower(htmlNodeText(candidate)), needle) {
			return true
		}
	}
	return false
}

func nearbyCandidates(node *html.Node) []*html.Node {
	if node == nil {
		return nil
	}
	seen := make(map[*html.Node]struct{})
	out := make([]*html.Node, 0, 8)
	push := func(candidate *html.Node) {
		if candidate == nil || candidate.Type != html.ElementNode {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	push(node.Parent)
	if node.Parent != nil {
		for sibling := node.Parent.FirstChild; sibling != nil; sibling = sibling.NextSibling {
			if sibling == node {
				continue
			}
			push(sibling)
		}
		push(node.Parent.Parent)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		push(child)
	}
	return out
}

func collectElementNodes(root *html.Node) []*html.Node {
	if root == nil {
		return nil
	}
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode {
			out = append(out, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return out
}

func htmlNodePath(node *html.Node) string {
	if node == nil {
		return ""
	}
	parts := make([]string, 0, 8)
	for current := node; current != nil; current = current.Parent {
		if current.Type != html.ElementNode {
			continue
		}
		tag := strings.ToLower(current.Data)
		index := 1
		for sibling := current.PrevSibling; sibling != nil; sibling = sibling.PrevSibling {
			if sibling.Type == html.ElementNode && strings.EqualFold(sibling.Data, current.Data) {
				index++
			}
		}
		parts = append(parts, fmt.Sprintf("%s:nth-of-type(%d)", tag, index))
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ">")
}

func previewString(input string, limit int) string {
	trimmed := strings.TrimSpace(input)
	if limit <= 0 || len([]rune(trimmed)) <= limit {
		return trimmed
	}
	runes := []rune(trimmed)
	return string(runes[:limit])
}

func findNodesByCSSSelector(root *html.Node, raw string) []*html.Node {
	selector := parseCSSSelector(raw)
	if len(selector.Steps) == 0 {
		return nil
	}
	matches := make([]*html.Node, 0)
	for _, node := range collectElementNodes(root) {
		if matchesCSSSelector(node, selector) {
			matches = append(matches, node)
		}
	}
	return matches
}

func parseCSSSelector(raw string) cssSelector {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cssSelector{}
	}
	steps := make([]cssSelectorStep, 0)
	combinators := make([]string, 0)
	var token strings.Builder
	inBracket := 0
	inQuote := rune(0)
	flush := func() {
		part := strings.TrimSpace(token.String())
		token.Reset()
		if part == "" {
			return
		}
		steps = append(steps, parseCSSSelectorStep(part))
	}
	for _, r := range raw {
		switch {
		case inQuote != 0:
			token.WriteRune(r)
			if r == inQuote {
				inQuote = 0
			}
		case r == '\'' || r == '"':
			token.WriteRune(r)
			inQuote = r
		case r == '[':
			inBracket++
			token.WriteRune(r)
		case r == ']':
			if inBracket > 0 {
				inBracket--
			}
			token.WriteRune(r)
		case inBracket == 0 && r == '>':
			flush()
			if len(steps) > 0 {
				combinators = append(combinators, ">")
			}
		case inBracket == 0 && (r == ' ' || r == '\t' || r == '\n'):
			flush()
			if len(steps) > 0 && (len(combinators) < len(steps)) {
				combinators = append(combinators, " ")
			}
		default:
			token.WriteRune(r)
		}
	}
	flush()
	if len(combinators) > len(steps)-1 {
		combinators = combinators[:len(steps)-1]
	}
	return cssSelector{Steps: steps, Combinators: combinators}
}

func parseCSSSelectorStep(raw string) cssSelectorStep {
	step := cssSelectorStep{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return step
	}
	for idx := strings.Index(raw, ":"); idx >= 0; idx = strings.Index(raw, ":") {
		raw = strings.TrimSpace(raw[:idx])
		break
	}
	i := 0
	for i < len(raw) && raw[i] != '#' && raw[i] != '.' && raw[i] != '[' {
		i++
	}
	if i > 0 {
		step.Tag = strings.ToLower(strings.TrimSpace(raw[:i]))
	}
	for i < len(raw) {
		switch raw[i] {
		case '#':
			i++
			start := i
			for i < len(raw) && raw[i] != '#' && raw[i] != '.' && raw[i] != '[' {
				i++
			}
			step.ID = strings.TrimSpace(raw[start:i])
		case '.':
			i++
			start := i
			for i < len(raw) && raw[i] != '#' && raw[i] != '.' && raw[i] != '[' {
				i++
			}
			if className := strings.TrimSpace(raw[start:i]); className != "" {
				step.Classes = append(step.Classes, className)
			}
		case '[':
			end := i + 1
			for end < len(raw) && raw[end] != ']' {
				end++
			}
			if end <= len(raw) {
				segment := strings.TrimSpace(raw[i+1 : end])
				if segment != "" {
					attr := cssSelectorAttr{}
					if idx := strings.Index(segment, "="); idx >= 0 {
						attr.Name = strings.TrimSpace(segment[:idx])
						attr.Value = strings.Trim(strings.TrimSpace(segment[idx+1:]), "\"'")
						attr.HasValue = true
					} else {
						attr.Name = strings.TrimSpace(segment)
					}
					step.Attrs = append(step.Attrs, attr)
				}
			}
			i = end + 1
		default:
			i++
		}
	}
	return step
}

func matchesCSSSelector(node *html.Node, selector cssSelector) bool {
	if node == nil || node.Type != html.ElementNode || len(selector.Steps) == 0 {
		return false
	}
	current := node
	for idx := len(selector.Steps) - 1; idx >= 0; idx-- {
		if current == nil || !matchesCSSStep(current, selector.Steps[idx]) {
			return false
		}
		if idx == 0 {
			return true
		}
		comb := " "
		if idx-1 < len(selector.Combinators) {
			comb = selector.Combinators[idx-1]
		}
		if comb == ">" {
			current = current.Parent
			continue
		}
		parent := current.Parent
		for parent != nil && (parent.Type != html.ElementNode || !matchesCSSStep(parent, selector.Steps[idx-1])) {
			parent = parent.Parent
		}
		current = parent
	}
	return false
}

func matchesCSSStep(node *html.Node, step cssSelectorStep) bool {
	if node == nil || node.Type != html.ElementNode {
		return false
	}
	if step.Tag != "" && !strings.EqualFold(node.Data, step.Tag) {
		return false
	}
	if step.ID != "" && !strings.EqualFold(nodeAttr(node, "id"), step.ID) {
		return false
	}
	classValue := strings.Fields(strings.ToLower(nodeAttr(node, "class")))
	classSet := make(map[string]struct{}, len(classValue))
	for _, className := range classValue {
		classSet[className] = struct{}{}
	}
	for _, className := range step.Classes {
		if _, ok := classSet[strings.ToLower(className)]; !ok {
			return false
		}
	}
	for _, attr := range step.Attrs {
		value := nodeAttr(node, attr.Name)
		if !attr.HasValue {
			if strings.TrimSpace(value) == "" {
				return false
			}
			continue
		}
		if value != attr.Value {
			return false
		}
	}
	return true
}

func findNodesByXPathLite(root *html.Node, raw string) []*html.Node {
	expr := parseXPathLite(raw)
	if len(expr.AbsolutePath) > 0 {
		return findNodesByAbsoluteXPath(root, expr.AbsolutePath)
	}
	out := make([]*html.Node, 0)
	for _, node := range collectElementNodes(root) {
		if matchesXPathLiteNode(node, expr) {
			out = append(out, node)
		}
	}
	return out
}

func parseXPathLite(raw string) xpathLiteExpr {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return xpathLiteExpr{}
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		parts := strings.Split(strings.Trim(raw, "/"), "/")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if idx := strings.Index(part, "["); idx >= 0 {
				part = part[:idx]
			}
			out = append(out, strings.ToLower(part))
		}
		return xpathLiteExpr{AbsolutePath: out}
	}
	trimmed := strings.TrimPrefix(raw, "//")
	segment := trimmed
	predicate := ""
	if idx := strings.Index(trimmed, "["); idx >= 0 {
		segment = strings.TrimSpace(trimmed[:idx])
		predicate = strings.TrimSpace(strings.TrimSuffix(trimmed[idx+1:], "]"))
	}
	expr := xpathLiteExpr{}
	if segment == "*" || segment == "" {
		expr.AnyTag = true
	} else {
		expr.Tag = strings.ToLower(segment)
	}
	lowerPred := strings.ToLower(predicate)
	switch {
	case strings.HasPrefix(lowerPred, "@") && strings.Contains(predicate, "="):
		idx := strings.Index(predicate, "=")
		expr.AttrName = strings.TrimPrefix(strings.TrimSpace(predicate[:idx]), "@")
		expr.AttrValue = strings.Trim(strings.TrimSpace(predicate[idx+1:]), "\"'")
	case strings.HasPrefix(lowerPred, "contains(@class"):
		if start := strings.Index(predicate, ","); start >= 0 {
			expr.ClassContains = strings.Trim(strings.TrimSpace(strings.TrimSuffix(predicate[start+1:], ")")), "\"'")
		}
	case strings.HasPrefix(lowerPred, "contains(normalize-space(.)"):
		if start := strings.Index(predicate, ","); start >= 0 {
			expr.TextContains = strings.Trim(strings.TrimSpace(strings.TrimSuffix(predicate[start+1:], ")")), "\"'")
		}
	}
	return expr
}

func matchesXPathLiteNode(node *html.Node, expr xpathLiteExpr) bool {
	if node == nil || node.Type != html.ElementNode {
		return false
	}
	if !expr.AnyTag && expr.Tag != "" && !strings.EqualFold(node.Data, expr.Tag) {
		return false
	}
	if expr.AttrName != "" && nodeAttr(node, expr.AttrName) != expr.AttrValue {
		return false
	}
	if expr.ClassContains != "" && !strings.Contains(strings.ToLower(nodeAttr(node, "class")), strings.ToLower(expr.ClassContains)) {
		return false
	}
	if expr.TextContains != "" && !strings.Contains(strings.ToLower(htmlNodeText(node)), strings.ToLower(expr.TextContains)) {
		return false
	}
	return true
}

func findNodesByAbsoluteXPath(root *html.Node, path []string) []*html.Node {
	if len(path) == 0 || root == nil {
		return nil
	}
	current := []*html.Node{root}
	for _, tag := range path {
		next := make([]*html.Node, 0)
		for _, node := range current {
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.ElementNode && strings.EqualFold(child.Data, tag) {
					next = append(next, child)
				}
			}
		}
		current = next
		if len(current) == 0 {
			break
		}
	}
	return current
}

func parsePositiveIntArg(args map[string]interface{}, keys []string, fallback, max int) int {
	value := fallback
	for _, key := range keys {
		valueRaw, ok := compatArgValue(args, key)
		if !ok {
			continue
		}
		if v, ok := coerceCompatInt(valueRaw); ok && v > 0 {
			value = v
			break
		}
	}
	if value <= 0 {
		value = fallback
	}
	if max > 0 && value > max {
		value = max
	}
	return value
}

func parseStringListArg(args map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		valueRaw, ok := compatArgValue(args, key)
		if !ok {
			continue
		}
		items, ok := coerceCompatStringList(valueRaw)
		if !ok {
			continue
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			value := strings.TrimSpace(asString(item))
			if value != "" {
				out = append(out, value)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func parseWebCrawlCheckpoint(raw interface{}) (webCrawlCheckpoint, error) {
	if raw == nil {
		return webCrawlCheckpoint{}, nil
	}
	switch typed := raw.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return webCrawlCheckpoint{}, nil
		}
		var checkpoint webCrawlCheckpoint
		if err := json.Unmarshal([]byte(typed), &checkpoint); err != nil {
			return webCrawlCheckpoint{}, fmt.Errorf("invalid checkpoint JSON: %w", err)
		}
		return checkpoint, nil
	case map[string]interface{}:
		b, _ := json.Marshal(typed)
		var checkpoint webCrawlCheckpoint
		if err := json.Unmarshal(b, &checkpoint); err != nil {
			return webCrawlCheckpoint{}, fmt.Errorf("invalid checkpoint object: %w", err)
		}
		return checkpoint, nil
	default:
		return webCrawlCheckpoint{}, errors.New("checkpoint must be an object or JSON string")
	}
}

func checkpointCreatedAt(checkpoint webCrawlCheckpoint) string {
	if strings.TrimSpace(checkpoint.CreatedAt) != "" {
		return checkpoint.CreatedAt
	}
	return time.Now().UTC().Format(time.RFC3339)
}

func deriveAllowedHosts(items []webCrawlQueueItem) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		parsed, err := url.Parse(item.URL)
		if err != nil {
			continue
		}
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func crawlHostAllowed(rawURL string, allowedHosts []string) bool {
	if len(allowedHosts) == 0 {
		return true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	for _, allowed := range allowedHosts {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "" {
			continue
		}
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

func canonicalizeCrawlURL(raw string) (string, error) {
	normalized, err := normalizeWebFetchURL(raw)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	parsed.Fragment = ""
	query := parsed.Query()
	for _, key := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "gclid", "fbclid"} {
		query.Del(key)
	}
	parsed.RawQuery = query.Encode()
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}

func trimStringList(values []string, limit int) []string {
	if len(values) == 0 {
		return nil
	}
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return append([]string(nil), values[:limit]...)
}

func sortedSeen(seen map[string]struct{}) []string {
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func classifyCrawlError(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "timeout"):
		return "timeout", true
	case strings.Contains(lower, "temporary") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "eof"):
		return "network", true
	case strings.Contains(lower, "5") && strings.Contains(lower, "http"):
		return "upstream_http", true
	case strings.Contains(lower, "unsupported content-type"):
		return "unsupported_content", false
	default:
		return "fetch_failed", false
	}
}
