package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/sync/singleflight"
)

const (
	webFetchExtractMarkdown = "markdown"
	webFetchExtractText     = "text"

	webFetchWarningCodeLoginWall       = "login_wall"
	webFetchWarningCodeChallenge       = "challenge"
	webFetchWarningCodeBrowserRequired = "browser_required"

	webFetchDefaultMaxChars         = 50_000
	webFetchDefaultMaxCharsCap      = 200_000
	webFetchDefaultMaxResponseBytes = 2_000_000
	webFetchDefaultTimeout          = 20 * time.Second
	webFetchDefaultMaxRedirects     = 3
	webFetchDefaultCacheTTL         = 5 * time.Minute
	webFetchDefaultFirecrawlBaseURL = "https://api.firecrawl.dev"
	webFetchDefaultFirecrawlTimeout = 15 * time.Second
	webFetchMinReadableChars        = 240
	webFetchDefaultUserAgent        = "Mozilla/5.0 (compatible; ZimaOS-Blue/1.0; +https://github.com/IceWhaleTech/ZimaOS-Blue)"
)

// WebFetchConfig configures the web_fetch tool runtime behavior.
type WebFetchConfig struct {
	Timeout           time.Duration
	MaxChars          int
	MaxCharsCap       int
	MaxResponseBytes  int64
	MaxRedirects      int
	CacheTTL          time.Duration
	UserAgent         string
	AllowPrivateHosts bool

	FirecrawlEnabled         bool
	FirecrawlAPIKey          string
	FirecrawlBaseURL         string
	FirecrawlTimeout         time.Duration
	FirecrawlOnlyMainContent bool
}

// WebFetchTool fetches and extracts readable content from a URL.
type WebFetchTool struct {
	config     WebFetchConfig
	httpClient *http.Client
	browser    BrowserBackend
	pdfService PDFService
	cacheMu    sync.RWMutex
	cache      map[string]webFetchCacheEntry
	fetchGroup singleflight.Group
}

type webFetchCacheEntry struct {
	expiresAt time.Time
	payload   webFetchPayload
}

type webFetchPayload struct {
	URL           string
	Title         string
	Content       string
	ContentType   string
	ExtractMode   string
	Extractor     string
	BodyTruncated bool
	Warning       string
	WarningCode   string
}

type webFetchRequestOptions struct {
	extraHeaders    map[string]string
	browserTargetID string
	cacheable       bool
}

var webFetchFakeIPPrefixes = []netip.Prefix{
	mustParseWebFetchPrefix("198.18.0.0/15"),
}

var webFetchBlockedPrefixes = []netip.Prefix{
	mustParseWebFetchPrefix("0.0.0.0/8"),
	mustParseWebFetchPrefix("100.64.0.0/10"),
	mustParseWebFetchPrefix("240.0.0.0/4"),
	mustParseWebFetchPrefix("::/128"),
}

// NewWebFetchTool creates a new web fetch tool.
func NewWebFetchTool(config WebFetchConfig) *WebFetchTool {
	if config.Timeout <= 0 {
		config.Timeout = webFetchDefaultTimeout
	}
	if config.MaxChars <= 0 {
		config.MaxChars = webFetchDefaultMaxChars
	}
	if config.MaxCharsCap <= 0 {
		config.MaxCharsCap = webFetchDefaultMaxCharsCap
	}
	if config.MaxChars > config.MaxCharsCap {
		config.MaxChars = config.MaxCharsCap
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = webFetchDefaultMaxResponseBytes
	}
	if config.MaxRedirects <= 0 {
		config.MaxRedirects = webFetchDefaultMaxRedirects
	}
	// CacheTTL=0 uses default; CacheTTL<0 disables cache.
	if config.CacheTTL == 0 {
		config.CacheTTL = webFetchDefaultCacheTTL
	} else if config.CacheTTL < 0 {
		config.CacheTTL = 0
	}
	if strings.TrimSpace(config.UserAgent) == "" {
		config.UserAgent = webFetchDefaultUserAgent
	}
	if strings.TrimSpace(config.FirecrawlAPIKey) == "" {
		config.FirecrawlAPIKey = strings.TrimSpace(os.Getenv("FIRECRAWL_API_KEY"))
	}
	if strings.TrimSpace(config.FirecrawlBaseURL) == "" {
		if envBase := strings.TrimSpace(os.Getenv("FIRECRAWL_BASE_URL")); envBase != "" {
			config.FirecrawlBaseURL = envBase
		} else {
			config.FirecrawlBaseURL = webFetchDefaultFirecrawlBaseURL
		}
	}
	if config.FirecrawlTimeout <= 0 {
		config.FirecrawlTimeout = webFetchDefaultFirecrawlTimeout
	}
	if !config.FirecrawlEnabled && config.FirecrawlAPIKey != "" {
		config.FirecrawlEnabled = true
	}
	if !config.FirecrawlOnlyMainContent {
		config.FirecrawlOnlyMainContent = true
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	baseDial := transport.DialContext
	if baseDial == nil {
		dialer := &net.Dialer{Timeout: config.Timeout}
		baseDial = dialer.DialContext
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host := address
		if h, _, err := net.SplitHostPort(address); err == nil && h != "" {
			host = h
		}
		if err := guardWebFetchHost(ctx, host, config.AllowPrivateHosts); err != nil {
			return nil, err
		}
		return baseDial(ctx, network, address)
	}

	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}
	maxRedirects := config.MaxRedirects
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := guardWebFetchHost(req.Context(), req.URL.Hostname(), config.AllowPrivateHosts); err != nil {
			return err
		}
		if maxRedirects <= 0 {
			return nil
		}
		if len(via) > maxRedirects {
			return fmt.Errorf("too many redirects (max=%d)", maxRedirects)
		}
		return nil
	}

	return &WebFetchTool{
		config:     config,
		httpClient: client,
		cache:      make(map[string]webFetchCacheEntry),
	}
}

// SetBrowser injects the browser backend used for session-cookie handoff.
func (w *WebFetchTool) SetBrowser(browser BrowserBackend) {
	w.browser = browser
}

// SetPDFService injects the PDF extraction service for PDF responses.
func (w *WebFetchTool) SetPDFService(service PDFService) {
	if w == nil {
		return
	}
	w.pdfService = service
}

// Definition returns the tool definition.
func (w *WebFetchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "web_fetch",
		Description: "Fetch and extract readable content from a URL via HTTP. Supports lightweight HTML, text, and PDF reads without browser automation.",
		Icon:        "web-search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "HTTP or HTTPS URL to fetch.",
				},
				"extract_mode": map[string]interface{}{
					"type":        "string",
					"description": "Extraction mode: markdown (default) or text.",
					"enum":        []string{webFetchExtractMarkdown, webFetchExtractText},
					"default":     webFetchExtractMarkdown,
				},
				"max_chars": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of characters in returned content.",
					"minimum":     100,
				},
				"headers": map[string]interface{}{
					"type":                 "object",
					"description":          "Optional request headers. For authenticated pages, pass Authorization/Cookie here.",
					"additionalProperties": map[string]interface{}{"type": "string"},
				},
				"cookies": map[string]interface{}{
					"type":        "string",
					"description": "Optional Cookie header value (e.g., session tokens).",
				},
				"authorization": map[string]interface{}{
					"type":        "string",
					"description": "Optional Authorization header value.",
				},
				"auth_bearer": map[string]interface{}{
					"type":        "string",
					"description": "Optional bearer token (auto-converted to Authorization: Bearer <token>).",
				},
				"browser_target_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional browser tab target ID. When set, reuse matching cookies from that browser session for this fetch.",
				},
			},
			"required": []string{"url"},
		},
	}
}

// Execute performs URL fetch + content extraction.
func (w *WebFetchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	rawURL := firstCompatString(args, "url", "href", "target", "input")
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("url is required")
	}

	mode, err := parseWebFetchExtractMode(args)
	if err != nil {
		return nil, err
	}
	maxChars := parseWebFetchMaxChars(args, w.config.MaxChars, w.config.MaxCharsCap)
	reqOpts, err := parseWebFetchRequestOptions(args)
	if err != nil {
		return nil, err
	}

	normalizedURL, err := normalizeWebFetchURL(rawURL)
	if err != nil {
		return nil, err
	}
	if err := guardWebFetchURL(ctx, normalizedURL, w.config.AllowPrivateHosts); err != nil {
		return nil, err
	}
	if err := w.applyBrowserSessionCookies(ctx, normalizedURL, &reqOpts); err != nil {
		return nil, err
	}

	cacheable := reqOpts.cacheable && w.config.CacheTTL > 0
	if !cacheable {
		payload, fetchErr := w.fetchAndExtract(ctx, normalizedURL, mode, reqOpts)
		if fetchErr != nil {
			return nil, fetchErr
		}
		return marshalWebFetchPayload(payload, maxChars), nil
	}

	cacheKey := buildWebFetchCacheKey(normalizedURL, mode)
	if cached, ok := w.loadCache(cacheKey); ok {
		return marshalWebFetchPayload(cached, maxChars), nil
	}

	freshAny, err, _ := w.fetchGroup.Do(cacheKey, func() (interface{}, error) {
		if cached, ok := w.loadCache(cacheKey); ok {
			return cached, nil
		}

		payload, fetchErr := w.fetchAndExtract(ctx, normalizedURL, mode, reqOpts)
		if fetchErr != nil {
			return nil, fetchErr
		}
		w.storeCache(cacheKey, payload)
		return payload, nil
	})
	if err != nil {
		return nil, err
	}

	fresh, ok := freshAny.(webFetchPayload)
	if !ok {
		return nil, errors.New("web fetch internal error: invalid cache payload type")
	}
	return marshalWebFetchPayload(fresh, maxChars), nil
}

func (w *WebFetchTool) fetchAndExtract(ctx context.Context, normalizedURL string, mode string, opts webFetchRequestOptions) (webFetchPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL, nil)
	if err != nil {
		return webFetchPayload{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/markdown, text/html;q=0.9, application/pdf;q=0.8, */*;q=0.1")
	req.Header.Set("User-Agent", w.config.UserAgent)
	for k, v := range opts.extraHeaders {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		if fallback, ok, fbErr := w.tryFirecrawlFallback(ctx, normalizedURL, mode); ok {
			return fallback, nil
		} else if fbErr != nil {
			return webFetchPayload{}, fmt.Errorf("failed to fetch URL: %w (firecrawl fallback error: %v)", err, fbErr)
		}
		return webFetchPayload{}, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	finalURL := normalizedURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	bodyLimit := w.responseBodyLimit(resp.Header.Get("Content-Type"), finalURL)
	body, bodyTruncated, err := readLimitedBody(resp.Body, bodyLimit)
	if err != nil {
		return webFetchPayload{}, fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	content, title, extractor, extractedTruncated, err := w.extractContent(ctx, finalURL, contentType, body, mode, bodyTruncated)
	if err != nil {
		if fallback, ok, fbErr := w.tryFirecrawlFallback(ctx, normalizedURL, mode); ok {
			return fallback, nil
		} else if fbErr != nil {
			return webFetchPayload{}, fmt.Errorf("%w (firecrawl fallback error: %v)", err, fbErr)
		}
		return webFetchPayload{}, err
	}

	if err := guardWebFetchURL(ctx, finalURL, w.config.AllowPrivateHosts); err != nil {
		return webFetchPayload{}, err
	}

	authWall, authWallCode, authWallWarning := detectWebFetchAuthWall(resp.StatusCode, finalURL, title, content)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if authWall {
			if fallback, ok, fbErr := w.tryBrowserSessionFallback(ctx, finalURL, mode, opts.browserTargetID, authWallCode, authWallWarning); ok {
				return fallback, nil
			} else if fbErr != nil {
				return webFetchPayload{}, fmt.Errorf("web fetch hit login/challenge wall for %s: %w", normalizedURL, fbErr)
			}
		}
		if fallback, ok, fbErr := w.tryFirecrawlFallback(ctx, finalURL, mode); ok {
			return fallback, nil
		} else if fbErr != nil {
			return webFetchPayload{}, fmt.Errorf("web fetch failed: HTTP %d for %s (firecrawl fallback error: %v)", resp.StatusCode, normalizedURL, fbErr)
		}
		detail := strings.TrimSpace(content)
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		if len(detail) > 300 {
			detail = detail[:300]
		}
		if authWallWarning != "" {
			return webFetchPayload{}, fmt.Errorf("%s", authWallWarning)
		}
		return webFetchPayload{}, fmt.Errorf("web fetch failed: HTTP %d for %s (%s)", resp.StatusCode, normalizedURL, detail)
	}

	if authWall {
		if fallback, ok, _ := w.tryBrowserSessionFallback(ctx, finalURL, mode, opts.browserTargetID, authWallCode, authWallWarning); ok {
			return fallback, nil
		}
	}

	// HTML extraction quality gate: if extraction is too short, try Firecrawl for better main-content parsing.
	if extractor == "html" && len([]rune(strings.TrimSpace(content))) < webFetchMinReadableChars {
		if fallback, ok, _ := w.tryFirecrawlFallback(ctx, finalURL, mode); ok && strings.TrimSpace(fallback.Content) != "" {
			return fallback, nil
		}
	}

	return webFetchPayload{
		URL:           finalURL,
		Title:         title,
		Content:       content,
		ContentType:   normalizeContentType(contentType),
		ExtractMode:   mode,
		Extractor:     extractor,
		BodyTruncated: bodyTruncated || extractedTruncated,
		Warning:       authWallWarning,
		WarningCode:   authWallCode,
	}, nil
}

func (w *WebFetchTool) responseBodyLimit(contentType, targetURL string) int64 {
	limit := w.config.MaxResponseBytes
	if limit <= 0 {
		limit = webFetchDefaultMaxResponseBytes
	}
	if isLikelyWebFetchPDF(contentType, targetURL) {
		pdfLimit := int64(defaultPDFMaxBytesMB) << 20
		if pdfLimit > limit {
			limit = pdfLimit
		}
	}
	return limit
}

func isLikelyWebFetchPDF(contentType, targetURL string) bool {
	if strings.Contains(normalizeContentType(contentType), "pdf") {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(parsed.Path), ".pdf")
}

func (w *WebFetchTool) extractContent(ctx context.Context, sourceURL, contentType string, body []byte, mode string, bodyTruncated bool) (content, title, extractor string, extractedTruncated bool, err error) {
	if isLikelyWebFetchPDF(contentType, sourceURL) || looksLikePDFBytes(body) {
		if bodyTruncated {
			return "", "", "", false, fmt.Errorf("pdf response exceeded %d bytes limit", w.responseBodyLimit(contentType, sourceURL))
		}
		content, title, extractor, extractedTruncated, err = w.extractPDFContent(ctx, sourceURL, body)
		return content, title, extractor, extractedTruncated, err
	}
	content, title, extractor, err = extractWebFetchContent(contentType, body, mode)
	return content, title, extractor, false, err
}

func (w *WebFetchTool) extractPDFContent(ctx context.Context, sourceURL string, body []byte) (content, title, extractor string, truncated bool, err error) {
	if w == nil || w.pdfService == nil {
		return "", "", "", false, errors.New("pdf extraction service not available for web fetch")
	}
	file, err := os.CreateTemp("", "zimaos-blue-webfetch-*.pdf")
	if err != nil {
		return "", "", "", false, fmt.Errorf("create temp pdf: %w", err)
	}
	path := file.Name()
	defer func() {
		_ = os.Remove(path)
	}()
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return "", "", "", false, fmt.Errorf("write temp pdf: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", "", "", false, fmt.Errorf("close temp pdf: %w", err)
	}

	result, err := w.pdfService.Extract(ctx, pdfextract.ExtractRequest{
		Path:     path,
		MaxChars: w.config.MaxCharsCap,
	})
	if err != nil {
		return "", "", "", false, fmt.Errorf("extract pdf: %w", err)
	}

	title = strings.TrimSpace(result.Document.Metadata["Title"])
	if title == "" {
		title = strings.TrimSpace(result.Document.FileName)
	}
	if title == "" {
		title = pdfDisplayName(sourceURL)
	}
	return strings.TrimSpace(result.Text), title, "pdf", result.Truncated, nil
}

func marshalWebFetchPayload(payload webFetchPayload, maxChars int) string {
	content, charsTruncated := truncateWebFetchContent(payload.Content, maxChars)
	result := map[string]interface{}{
		"url":          payload.URL,
		"title":        payload.Title,
		"content":      content,
		"content_type": payload.ContentType,
		"extract_mode": payload.ExtractMode,
		"extractor":    payload.Extractor,
		"truncated":    payload.BodyTruncated || charsTruncated,
	}
	if strings.TrimSpace(payload.Warning) != "" {
		result["warning"] = payload.Warning
	}
	if strings.TrimSpace(payload.WarningCode) != "" {
		result["warning_code"] = payload.WarningCode
	}
	b, _ := json.Marshal(result)
	return string(b)
}

func buildWebFetchCacheKey(normalizedURL, mode string) string {
	return mode + "|" + normalizedURL
}

func (w *WebFetchTool) loadCache(key string) (webFetchPayload, bool) {
	if w.config.CacheTTL <= 0 {
		return webFetchPayload{}, false
	}
	now := time.Now()
	w.cacheMu.RLock()
	entry, ok := w.cache[key]
	w.cacheMu.RUnlock()
	if !ok {
		return webFetchPayload{}, false
	}
	if entry.expiresAt.Before(now) {
		w.cacheMu.Lock()
		latest, exists := w.cache[key]
		if exists && latest.expiresAt.Before(now) {
			delete(w.cache, key)
		}
		w.cacheMu.Unlock()
		return webFetchPayload{}, false
	}
	return entry.payload, true
}

func (w *WebFetchTool) storeCache(key string, payload webFetchPayload) {
	if w.config.CacheTTL <= 0 {
		return
	}
	w.cacheMu.Lock()
	w.cache[key] = webFetchCacheEntry{
		expiresAt: time.Now().Add(w.config.CacheTTL),
		payload:   payload,
	}
	w.cacheMu.Unlock()
}

func (w *WebFetchTool) tryFirecrawlFallback(ctx context.Context, targetURL, mode string) (webFetchPayload, bool, error) {
	if !w.config.FirecrawlEnabled {
		return webFetchPayload{}, false, nil
	}
	if strings.TrimSpace(w.config.FirecrawlAPIKey) == "" {
		return webFetchPayload{}, false, nil
	}
	payload, err := w.fetchViaFirecrawl(ctx, targetURL, mode)
	if err != nil {
		return webFetchPayload{}, false, err
	}
	return payload, true, nil
}

func (w *WebFetchTool) tryBrowserSessionFallback(ctx context.Context, targetURL, mode, browserTargetID, reasonCode, reason string) (webFetchPayload, bool, error) {
	if strings.TrimSpace(browserTargetID) == "" || w.browser == nil {
		return webFetchPayload{}, false, nil
	}
	payload, err := w.fetchViaBrowserSession(ctx, targetURL, mode, browserTargetID, reasonCode, reason)
	if err != nil {
		return webFetchPayload{}, false, err
	}
	return payload, true, nil
}

func (w *WebFetchTool) fetchViaBrowserSession(ctx context.Context, targetURL, mode, browserTargetID, reasonCode, reason string) (webFetchPayload, error) {
	nav, err := w.browser.Navigate(ctx, targetURL, browserTargetID)
	if err != nil {
		return webFetchPayload{}, fmt.Errorf("browser session navigate failed: %w", err)
	}
	a11y, err := w.browser.AccessibilityTree(ctx, nav.TargetID, 12)
	if err != nil {
		return webFetchPayload{}, fmt.Errorf("browser session snapshot failed: %w", err)
	}
	finalURL := strings.TrimSpace(a11y.URL)
	if finalURL == "" {
		finalURL = strings.TrimSpace(nav.URL)
	}
	if finalURL == "" {
		finalURL = targetURL
	}
	title := strings.TrimSpace(a11y.Title)
	if title == "" {
		title = strings.TrimSpace(nav.Title)
	}
	content := strings.TrimSpace(a11y.Tree)
	if mode == webFetchExtractMarkdown && title != "" {
		content = "# " + title + "\n\n" + content
	}
	warning := strings.TrimSpace(reason)
	if warning == "" {
		warning = "web_fetch used browser session fallback because the page appears to require login or an interactive browser"
	}
	return webFetchPayload{
		URL:         finalURL,
		Title:       title,
		Content:     content,
		ContentType: "text/plain",
		ExtractMode: mode,
		Extractor:   "browser-a11y",
		Warning:     warning,
		WarningCode: strings.TrimSpace(reasonCode),
	}, nil
}

func (w *WebFetchTool) fetchViaFirecrawl(ctx context.Context, targetURL, mode string) (webFetchPayload, error) {
	normalizedTarget, err := normalizeWebFetchURL(targetURL)
	if err != nil {
		return webFetchPayload{}, err
	}
	if err := guardWebFetchURL(ctx, normalizedTarget, w.config.AllowPrivateHosts); err != nil {
		return webFetchPayload{}, err
	}

	endpoint := resolveWebFetchFirecrawlEndpoint(w.config.FirecrawlBaseURL)
	firecrawlReq := map[string]interface{}{
		"url":             normalizedTarget,
		"formats":         []string{"markdown"},
		"onlyMainContent": w.config.FirecrawlOnlyMainContent,
		"timeout":         int(w.config.FirecrawlTimeout / time.Millisecond),
	}
	body, err := json.Marshal(firecrawlReq)
	if err != nil {
		return webFetchPayload{}, fmt.Errorf("failed to encode firecrawl request: %w", err)
	}

	fallbackCtx := ctx
	cancel := func() {}
	if w.config.FirecrawlTimeout > 0 {
		fallbackCtx, cancel = context.WithTimeout(ctx, w.config.FirecrawlTimeout)
	}
	defer cancel()

	req, err := http.NewRequestWithContext(fallbackCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return webFetchPayload{}, fmt.Errorf("failed to build firecrawl request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", w.config.UserAgent)
	req.Header.Set("Authorization", "Bearer "+normalizeWebFetchBearerToken(w.config.FirecrawlAPIKey))

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return webFetchPayload{}, fmt.Errorf("firecrawl request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _, err := readLimitedBody(resp.Body, w.config.MaxResponseBytes)
	if err != nil {
		return webFetchPayload{}, fmt.Errorf("failed to read firecrawl response: %w", err)
	}

	type firecrawlResponse struct {
		Success bool `json:"success"`
		Data    struct {
			Markdown string `json:"markdown"`
			Content  string `json:"content"`
			Metadata struct {
				Title      string `json:"title"`
				SourceURL  string `json:"sourceURL"`
				StatusCode int    `json:"statusCode"`
			} `json:"metadata"`
		} `json:"data"`
		Warning string `json:"warning"`
		Error   string `json:"error"`
	}

	var parsed firecrawlResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return webFetchPayload{}, fmt.Errorf("failed to decode firecrawl response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !parsed.Success {
		detail := strings.TrimSpace(parsed.Error)
		if detail == "" {
			detail = strings.TrimSpace(string(raw))
		}
		if len(detail) > 240 {
			detail = detail[:240]
		}
		return webFetchPayload{}, fmt.Errorf("firecrawl fetch failed: HTTP %d (%s)", resp.StatusCode, detail)
	}

	content := strings.TrimSpace(parsed.Data.Markdown)
	if content == "" {
		content = strings.TrimSpace(parsed.Data.Content)
	}
	if content == "" {
		return webFetchPayload{}, errors.New("firecrawl returned empty content")
	}
	if mode == webFetchExtractText {
		content = markdownToPlainText(content)
	}

	finalURL := strings.TrimSpace(parsed.Data.Metadata.SourceURL)
	if finalURL == "" {
		finalURL = normalizedTarget
	} else {
		normalizedFinalURL, err := normalizeWebFetchURL(finalURL)
		if err != nil {
			finalURL = normalizedTarget
		} else {
			finalURL = normalizedFinalURL
		}
	}
	if err := guardWebFetchURL(ctx, finalURL, w.config.AllowPrivateHosts); err != nil {
		return webFetchPayload{}, err
	}

	contentType := "text/markdown"
	if mode == webFetchExtractText {
		contentType = "text/plain"
	}
	return webFetchPayload{
		URL:           finalURL,
		Title:         strings.TrimSpace(parsed.Data.Metadata.Title),
		Content:       strings.TrimSpace(content),
		ContentType:   contentType,
		ExtractMode:   mode,
		Extractor:     "firecrawl",
		BodyTruncated: false,
	}, nil
}

func resolveWebFetchFirecrawlEndpoint(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return webFetchDefaultFirecrawlBaseURL + "/v2/scrape"
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return webFetchDefaultFirecrawlBaseURL + "/v2/scrape"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return webFetchDefaultFirecrawlBaseURL + "/v2/scrape"
	}
	if strings.TrimSpace(parsed.Path) == "" || parsed.Path == "/" {
		parsed.Path = "/v2/scrape"
	}
	return parsed.String()
}

func normalizeWebFetchBearerToken(raw string) string {
	token := strings.TrimSpace(raw)
	token = strings.ReplaceAll(token, "\r", "")
	token = strings.ReplaceAll(token, "\n", "")
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[len("bearer "):])
	}
	return token
}

func parseWebFetchExtractMode(args map[string]interface{}) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(firstCompatString(args, "extract_mode", "extractMode", "mode")))
	if mode == "" {
		mode = webFetchExtractMarkdown
	}
	switch mode {
	case webFetchExtractMarkdown, webFetchExtractText:
		return mode, nil
	default:
		return "", fmt.Errorf("extract_mode must be one of: %s, %s", webFetchExtractMarkdown, webFetchExtractText)
	}
}

func parseWebFetchMaxChars(args map[string]interface{}, fallback, cap int) int {
	if cap <= 0 {
		cap = webFetchDefaultMaxCharsCap
	}
	if fallback <= 0 {
		fallback = webFetchDefaultMaxChars
	}
	value := fallback
	for _, key := range []string{"max_chars", "maxChars", "limit"} {
		raw, ok := compatArgValue(args, key)
		if !ok {
			continue
		}
		if v, ok := coerceCompatInt(raw); ok {
			value = v
			break
		}
	}
	if value < 100 {
		value = 100
	}
	if value > cap {
		value = cap
	}
	return value
}

func parseWebFetchRequestOptions(args map[string]interface{}) (webFetchRequestOptions, error) {
	opts := webFetchRequestOptions{
		extraHeaders:    make(map[string]string),
		browserTargetID: strings.TrimSpace(firstCompatString(args, "browser_target_id", "browserTargetId")),
		cacheable:       true,
	}

	for _, key := range []string{"headers", "request_headers", "requestHeaders"} {
		raw, ok := compatArgValue(args, key)
		if !ok || raw == nil {
			continue
		}
		if err := mergeWebFetchHeaders(opts.extraHeaders, raw); err != nil {
			return webFetchRequestOptions{}, err
		}
	}

	authorization := firstCompatString(args, "authorization", "Authorization")
	bearer := firstCompatString(args, "auth_bearer", "authBearer", "bearer_token")
	cookies := firstCompatString(args, "cookies", "cookie", "Cookie")

	if strings.TrimSpace(bearer) != "" {
		normalized := strings.TrimSpace(bearer)
		if !strings.HasPrefix(strings.ToLower(normalized), "bearer ") {
			normalized = "Bearer " + normalized
		}
		authorization = normalized
	}
	if strings.TrimSpace(authorization) != "" {
		if err := setWebFetchHeader(opts.extraHeaders, "Authorization", authorization); err != nil {
			return webFetchRequestOptions{}, err
		}
	}
	if strings.TrimSpace(cookies) != "" {
		if err := setWebFetchHeader(opts.extraHeaders, "Cookie", cookies); err != nil {
			return webFetchRequestOptions{}, err
		}
	}

	for name, value := range opts.extraHeaders {
		if strings.TrimSpace(value) == "" {
			delete(opts.extraHeaders, name)
			continue
		}
		if strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "Cookie") {
			opts.cacheable = false
		}
	}
	if opts.browserTargetID != "" {
		opts.cacheable = false
	}
	return opts, nil
}

func (w *WebFetchTool) applyBrowserSessionCookies(ctx context.Context, normalizedURL string, opts *webFetchRequestOptions) error {
	if opts == nil || strings.TrimSpace(opts.browserTargetID) == "" {
		return nil
	}
	if w.browser == nil {
		return errors.New("browser_target_id requires browser backend support")
	}
	cookieHeader, err := w.browser.CookieHeader(ctx, opts.browserTargetID, normalizedURL)
	if err != nil {
		return fmt.Errorf("failed to read browser session cookies: %w", err)
	}
	if strings.TrimSpace(cookieHeader) == "" {
		return nil
	}
	if existing := strings.TrimSpace(opts.extraHeaders[http.CanonicalHeaderKey("Cookie")]); existing != "" {
		cookieHeader = mergeWebFetchCookieHeaders(cookieHeader, existing)
	}
	return setWebFetchHeader(opts.extraHeaders, "Cookie", cookieHeader)
}

func detectWebFetchAuthWall(statusCode int, finalURL, title, content string) (bool, string, string) {
	lowerURL := strings.ToLower(strings.TrimSpace(finalURL))
	lowerTitle := strings.ToLower(strings.TrimSpace(title))
	lowerContent := strings.ToLower(strings.TrimSpace(content))

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return true, webFetchWarningCodeBrowserRequired, "page appears to require login or a verified browser session; use browser or pass browser_target_id"
	}
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable {
		if containsWebFetchChallengeSignal(lowerURL, lowerTitle, lowerContent) {
			return true, webFetchWarningCodeChallenge, "page appears blocked by a login, rate-limit, or anti-bot challenge; use browser or pass browser_target_id"
		}
	}
	if containsWebFetchLoginSignal(lowerURL, lowerTitle, lowerContent) {
		return true, webFetchWarningCodeLoginWall, "page appears to be a login wall; use browser or pass browser_target_id"
	}
	if containsWebFetchChallengeSignal(lowerURL, lowerTitle, lowerContent) {
		return true, webFetchWarningCodeChallenge, "page appears blocked by an anti-bot or verification challenge; use browser or pass browser_target_id"
	}
	return false, "", ""
}

func containsWebFetchLoginSignal(lowerURL, lowerTitle, lowerContent string) bool {
	if strings.Contains(lowerURL, "/login") || strings.Contains(lowerURL, "/signin") || strings.Contains(lowerURL, "authwall") {
		return true
	}
	loginSignals := []string{
		"log in",
		"sign in",
		"create account",
		"create an account",
		"forgot password",
		"continue with google",
		"continue with apple",
		"continue with email",
		"password",
		"username",
	}
	matches := 0
	for _, signal := range loginSignals {
		if strings.Contains(lowerTitle, signal) || strings.Contains(lowerContent, signal) {
			matches++
		}
	}
	return matches >= 2
}

func containsWebFetchChallengeSignal(lowerURL, lowerTitle, lowerContent string) bool {
	challengeSignals := []string{
		"captcha",
		"verify you are human",
		"verify you’re human",
		"are you a robot",
		"attention required",
		"access denied",
		"unusual traffic",
		"cloudflare",
		"enable javascript and cookies",
		"human verification",
	}
	for _, signal := range challengeSignals {
		if strings.Contains(lowerURL, signal) || strings.Contains(lowerTitle, signal) || strings.Contains(lowerContent, signal) {
			return true
		}
	}
	return false
}

func mergeWebFetchCookieHeaders(base, override string) string {
	type cookiePair struct {
		name  string
		value string
	}
	ordered := make([]cookiePair, 0, 8)
	index := make(map[string]int)
	merge := func(raw string) {
		for _, part := range strings.Split(raw, ";") {
			segment := strings.TrimSpace(part)
			if segment == "" {
				continue
			}
			pieces := strings.SplitN(segment, "=", 2)
			name := strings.TrimSpace(pieces[0])
			if name == "" {
				continue
			}
			value := ""
			if len(pieces) == 2 {
				value = strings.TrimSpace(pieces[1])
			}
			if pos, ok := index[name]; ok {
				ordered[pos].value = value
				continue
			}
			index[name] = len(ordered)
			ordered = append(ordered, cookiePair{name: name, value: value})
		}
	}
	merge(base)
	merge(override)
	parts := make([]string, 0, len(ordered))
	for _, pair := range ordered {
		parts = append(parts, pair.name+"="+pair.value)
	}
	return strings.Join(parts, "; ")
}

func mergeWebFetchHeaders(dst map[string]string, raw interface{}) error {
	switch typed := raw.(type) {
	case map[string]string:
		for name, value := range typed {
			if err := setWebFetchHeader(dst, name, value); err != nil {
				return err
			}
		}
		return nil
	case map[string]interface{}:
		for name, value := range typed {
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("headers.%s must be a string", strings.TrimSpace(name))
			}
			if err := setWebFetchHeader(dst, name, text); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("headers must be an object with string values")
	}
}

func setWebFetchHeader(dst map[string]string, name, value string) error {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return errors.New("header name cannot be empty")
	}
	if !isValidWebFetchHeaderName(trimmedName) {
		return fmt.Errorf("invalid header name: %s", trimmedName)
	}

	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		delete(dst, http.CanonicalHeaderKey(trimmedName))
		return nil
	}
	if strings.ContainsAny(trimmedValue, "\r\n") {
		return fmt.Errorf("invalid header value for %s: newlines are not allowed", trimmedName)
	}
	for _, r := range trimmedValue {
		if r == 0 || (r < 0x20 && r != '\t') {
			return fmt.Errorf("invalid header value for %s: control characters are not allowed", trimmedName)
		}
	}

	dst[http.CanonicalHeaderKey(trimmedName)] = trimmedValue
	return nil
}

func isValidWebFetchHeaderName(name string) bool {
	for _, r := range name {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			continue
		}
		switch r {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}

func normalizeWebFetchURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme == "" {
		parsed, err = url.Parse("https://" + strings.TrimSpace(raw))
		if err != nil {
			return "", fmt.Errorf("invalid url: %w", err)
		}
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", errors.New("url scheme must be http or https")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return "", errors.New("url host is required")
	}
	return parsed.String(), nil
}

func guardWebFetchURL(ctx context.Context, rawURL string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return errors.New("url host is required")
	}
	return guardWebFetchHost(ctx, host, allowPrivate)
}

func guardWebFetchHost(ctx context.Context, host string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}
	trimmedHost := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if trimmedHost == "" {
		return errors.New("url host is required")
	}
	if isBlockedWebFetchHostname(trimmedHost) {
		return fmt.Errorf("blocked hostname for web_fetch: %s", trimmedHost)
	}

	if literal, err := netip.ParseAddr(trimmedHost); err == nil {
		if isFakeIPWebFetchAddr(literal) || isBlockedWebFetchAddr(literal) {
			return fmt.Errorf("blocked private/internal address for web_fetch: %s", literal.Unmap().String())
		}
		return nil
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, trimmedHost)
	if err != nil {
		// If DNS lookup fails at this stage, let the actual request surface the network error.
		return nil
	}
	for _, item := range ips {
		addr, ok := netip.AddrFromSlice(item.IP)
		if !ok {
			continue
		}
		if err := guardResolvedWebFetchAddr(trimmedHost, addr); err != nil {
			return fmt.Errorf("blocked private/internal destination for web_fetch: %s -> %s", trimmedHost, addr.Unmap().String())
		}
	}
	return nil
}

func guardResolvedWebFetchAddr(host string, addr netip.Addr) error {
	addr = addr.Unmap()
	if isFakeIPWebFetchAddr(addr) {
		if isLikelyPublicWebFetchHostname(host) {
			return nil
		}
		return errors.New("benchmark fake-ip destination is not allowed for non-public hosts")
	}
	if isBlockedWebFetchAddr(addr) {
		return errors.New("private/internal destination is not allowed")
	}
	return nil
}

func isBlockedWebFetchHostname(host string) bool {
	if host == "localhost" || host == "localhost.localdomain" {
		return true
	}
	return strings.HasSuffix(host, ".localhost")
}

func isLikelyPublicWebFetchHostname(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" || isBlockedWebFetchHostname(host) || strings.HasSuffix(host, ".local") {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}
	registrable, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil || registrable == "" {
		return false
	}
	suffix, icann := publicsuffix.PublicSuffix(host)
	if !icann || suffix == "" {
		return false
	}
	return !strings.EqualFold(registrable, suffix)
}

func isFakeIPWebFetchAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, blocked := range webFetchFakeIPPrefixes {
		if blocked.Contains(addr) {
			return true
		}
	}
	return false
}

func isBlockedWebFetchAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() || addr.IsMulticast() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return true
	}
	for _, blocked := range webFetchBlockedPrefixes {
		if blocked.Contains(addr) {
			return true
		}
	}
	return false
}

func mustParseWebFetchPrefix(raw string) netip.Prefix {
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		panic(fmt.Sprintf("invalid web fetch blocked prefix %q: %v", raw, err))
	}
	return prefix
}

func readLimitedBody(r io.Reader, maxBytes int64) ([]byte, bool, error) {
	if maxBytes <= 0 {
		body, err := io.ReadAll(r)
		return body, false, err
	}
	limited := io.LimitReader(r, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > maxBytes {
		return body[:maxBytes], true, nil
	}
	return body, false, nil
}

func extractWebFetchContent(contentType string, body []byte, mode string) (content, title, extractor string, err error) {
	ct := strings.ToLower(contentType)
	raw := string(body)

	switch {
	case strings.Contains(ct, "text/markdown"):
		extractor = "cf-markdown"
		content = strings.TrimSpace(raw)
		if mode == webFetchExtractText {
			content = markdownToPlainText(content)
		}
		return content, "", extractor, nil
	case strings.Contains(ct, "text/html"), strings.Contains(ct, "application/xhtml+xml"), looksLikeHTMLBody(body):
		extractor = "html"
		title, content = extractReadableHTML(raw)
		if mode == webFetchExtractMarkdown {
			if strings.TrimSpace(title) != "" {
				content = "# " + strings.TrimSpace(title) + "\n\n" + content
			}
		} else {
			content = strings.TrimSpace(content)
			if strings.TrimSpace(title) != "" {
				content = strings.TrimSpace(title) + "\n\n" + content
			}
		}
		return strings.TrimSpace(content), strings.TrimSpace(title), extractor, nil
	case strings.HasPrefix(normalizeContentType(contentType), "text/"), looksLikeTextContentType(ct):
		extractor = "text"
		content = strings.TrimSpace(raw)
		if mode == webFetchExtractText {
			return content, "", extractor, nil
		}
		return content, "", extractor, nil
	default:
		// Last chance: if body itself looks HTML-ish, parse as HTML.
		if looksLikeHTMLBody(body) {
			extractor = "html"
			title, content = extractReadableHTML(raw)
			return strings.TrimSpace(content), strings.TrimSpace(title), extractor, nil
		}
		return "", "", "", fmt.Errorf("unsupported content-type for web fetch: %q", normalizeContentType(contentType))
	}
}

func truncateWebFetchContent(s string, maxChars int) (string, bool) {
	if maxChars <= 0 {
		return s, false
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s, false
	}
	return string(runes[:maxChars]), true
}

func normalizeContentType(ct string) string {
	ct = strings.TrimSpace(strings.ToLower(ct))
	if ct == "" {
		return ""
	}
	if idx := strings.Index(ct, ";"); idx >= 0 {
		return strings.TrimSpace(ct[:idx])
	}
	return ct
}

func looksLikeTextContentType(ct string) bool {
	return strings.Contains(ct, "json") || strings.Contains(ct, "xml") || strings.Contains(ct, "javascript")
}

func looksLikeHTMLBody(body []byte) bool {
	head := strings.ToLower(strings.TrimSpace(string(bytes.TrimSpace(body))))
	if len(head) > 256 {
		head = head[:256]
	}
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html") || strings.HasPrefix(head, "<head") || strings.HasPrefix(head, "<body")
}

func markdownToPlainText(markdown string) string {
	lines := strings.Split(markdown, "\n")
	out := make([]string, 0, len(lines))
	replacer := strings.NewReplacer("`", "", "*", "", "_", "", "~~", "")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			out = append(out, "")
			continue
		}
		for len(line) > 0 && line[0] == '#' {
			line = strings.TrimSpace(line[1:])
		}
		line = strings.TrimSpace(strings.TrimLeft(line, "-*+>"))
		if len(line) > 2 && unicode.IsDigit(rune(line[0])) && line[1] == '.' {
			line = strings.TrimSpace(line[2:])
		}
		line = replacer.Replace(line)
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func extractReadableHTML(raw string) (title, text string) {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return "", stripHTML(raw)
	}

	title = strings.TrimSpace(findTagText(doc, "title"))
	target := selectReadableNode(doc)

	writer := &readableWriter{}
	writeReadableText(writer, target)
	text = cleanTextBlocks(writer.String())
	if text == "" {
		text = stripHTML(raw)
	}
	return title, text
}

func selectReadableNode(doc *html.Node) *html.Node {
	body := findFirstTag(doc, "body")
	if body == nil {
		body = doc
	}

	mainNode := findFirstTag(body, "main")
	articleNode := findFirstTag(body, "article")
	bestNode, bestScore := findBestReadableCandidate(body)

	// Prefer explicit semantic containers when they carry enough text.
	if mainNode != nil {
		if score := scoreReadableNode(mainNode); score >= bestScore*0.85 && score > 120 {
			return mainNode
		}
	}
	if articleNode != nil {
		if score := scoreReadableNode(articleNode); score >= bestScore*0.85 && score > 120 {
			return articleNode
		}
	}
	if bestNode != nil {
		return bestNode
	}
	return body
}

func findBestReadableCandidate(root *html.Node) (*html.Node, float64) {
	var best *html.Node
	bestScore := -1.0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			tag := strings.ToLower(strings.TrimSpace(n.Data))
			if isCandidateTag(tag) {
				if score := scoreReadableNode(n); score > bestScore {
					best = n
					bestScore = score
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if bestScore < 120 {
		return nil, bestScore
	}
	return best, bestScore
}

func isCandidateTag(tag string) bool {
	switch tag {
	case "main", "article", "section", "div":
		return true
	default:
		return false
	}
}

type readableStats struct {
	textLen     int
	linkTextLen int
	punctuation int
}

func scoreReadableNode(n *html.Node) float64 {
	stats := collectReadableStats(n, false)
	if stats.textLen < 80 {
		return -1
	}

	density := float64(stats.linkTextLen) / float64(maxInt(stats.textLen, 1))
	score := float64(stats.textLen) + float64(stats.punctuation*24) - density*450

	tag := ""
	if n.Type == html.ElementNode {
		tag = strings.ToLower(strings.TrimSpace(n.Data))
	}
	switch tag {
	case "article":
		score += 260
	case "main":
		score += 220
	case "section":
		score += 100
	case "div":
		score += 20
	case "nav", "header", "footer", "aside":
		score -= 300
	}

	classID := strings.ToLower(nodeAttr(n, "class") + " " + nodeAttr(n, "id"))
	for _, positive := range []string{"article", "content", "post", "entry", "main", "story", "text", "body"} {
		if strings.Contains(classID, positive) {
			score += 80
		}
	}
	for _, negative := range []string{"nav", "menu", "footer", "sidebar", "comment", "promo", "ad", "banner", "header"} {
		if strings.Contains(classID, negative) {
			score -= 80
		}
	}
	return score
}

func collectReadableStats(n *html.Node, inLink bool) readableStats {
	if n == nil {
		return readableStats{}
	}
	if n.Type == html.ElementNode {
		tag := strings.ToLower(strings.TrimSpace(n.Data))
		if htmlSkipTags[tag] {
			return readableStats{}
		}
		if tag == "a" {
			inLink = true
		}
	}

	total := readableStats{}
	if n.Type == html.TextNode {
		token := strings.Join(strings.Fields(n.Data), " ")
		token = strings.TrimSpace(token)
		if token != "" {
			l := len([]rune(token))
			total.textLen += l
			if inLink {
				total.linkTextLen += l
			}
			total.punctuation += countReadablePunctuation(token)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		child := collectReadableStats(c, inLink)
		total.textLen += child.textLen
		total.linkTextLen += child.linkTextLen
		total.punctuation += child.punctuation
	}
	return total
}

func countReadablePunctuation(s string) int {
	count := 0
	for _, r := range s {
		switch r {
		case '.', ',', ';', ':', '!', '?', '。', '，', '；', '：', '！', '？':
			count++
		}
	}
	return count
}

func nodeAttr(n *html.Node, key string) string {
	if n == nil || n.Type != html.ElementNode {
		return ""
	}
	key = strings.ToLower(strings.TrimSpace(key))
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func findFirstTag(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, tag) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstTag(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func findTagText(n *html.Node, tag string) string {
	node := findFirstTag(n, tag)
	if node == nil {
		return ""
	}
	writer := &readableWriter{}
	writeReadableText(writer, node)
	return cleanTextBlocks(writer.String())
}

var htmlSkipTags = map[string]bool{
	"script":   true,
	"style":    true,
	"noscript": true,
	"svg":      true,
	"path":     true,
	"head":     true,
}

var htmlBlockTags = map[string]bool{
	"article":    true,
	"aside":      true,
	"blockquote": true,
	"br":         true,
	"div":        true,
	"footer":     true,
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"header":     true,
	"li":         true,
	"main":       true,
	"nav":        true,
	"ol":         true,
	"p":          true,
	"pre":        true,
	"section":    true,
	"table":      true,
	"tr":         true,
	"ul":         true,
}

type readableWriter struct {
	builder  strings.Builder
	lastByte byte
}

func (w *readableWriter) WriteToken(token string) {
	if strings.TrimSpace(token) == "" {
		return
	}
	if w.builder.Len() > 0 && w.lastByte != '\n' && w.lastByte != ' ' && w.lastByte != '\t' {
		w.builder.WriteByte(' ')
		w.lastByte = ' '
	}
	w.builder.WriteString(token)
	if len(token) > 0 {
		w.lastByte = token[len(token)-1]
	}
}

func (w *readableWriter) Newline() {
	if w.builder.Len() == 0 || w.lastByte == '\n' {
		return
	}
	w.builder.WriteByte('\n')
	w.lastByte = '\n'
}

func (w *readableWriter) String() string {
	return w.builder.String()
}

func writeReadableText(w *readableWriter, n *html.Node) {
	if n == nil {
		return
	}
	if n.Type == html.ElementNode {
		tag := strings.ToLower(strings.TrimSpace(n.Data))
		if htmlSkipTags[tag] {
			return
		}
		if htmlBlockTags[tag] {
			w.Newline()
		}
	}

	if n.Type == html.TextNode {
		token := strings.Join(strings.Fields(n.Data), " ")
		token = strings.TrimSpace(token)
		if token != "" {
			w.WriteToken(token)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeReadableText(w, c)
	}

	if n.Type == html.ElementNode {
		tag := strings.ToLower(strings.TrimSpace(n.Data))
		if htmlBlockTags[tag] {
			w.Newline()
		}
	}
}

func cleanTextBlocks(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if lastBlank {
				continue
			}
			lastBlank = true
			out = append(out, "")
			continue
		}
		lastBlank = false
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func normalizeWebFetchCompatArgs(_ string, args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+6)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["url"])) == "" {
		if target := firstCompatStringDeep(normalized, "url", "href", "target", "input", "query", "q"); target != "" {
			normalized["url"] = target
		}
	}

	if strings.TrimSpace(asString(normalized["extract_mode"])) == "" {
		if mode := firstCompatStringDeep(normalized, "extract_mode", "extractMode", "mode"); mode != "" {
			normalized["extract_mode"] = strings.ToLower(strings.TrimSpace(mode))
		}
	}

	if _, ok := normalized["max_chars"]; !ok {
		if limit, ok := firstCompatIntDeep(normalized, "max_chars", "maxChars", "limit"); ok && limit > 0 {
			normalized["max_chars"] = limit
		}
	}

	if _, ok := normalized["headers"]; !ok {
		if headers, exists := firstCompatValueDeep(normalized, "headers", "request_headers", "requestHeaders"); exists {
			normalized["headers"] = headers
		}
	}

	if strings.TrimSpace(asString(normalized["authorization"])) == "" {
		if v := firstCompatStringDeep(normalized, "authorization", "Authorization"); v != "" {
			normalized["authorization"] = v
		}
	}
	if strings.TrimSpace(asString(normalized["auth_bearer"])) == "" {
		if v := firstCompatStringDeep(normalized, "auth_bearer", "authBearer", "bearer_token"); v != "" {
			normalized["auth_bearer"] = v
		}
	}
	if strings.TrimSpace(asString(normalized["cookies"])) == "" {
		if v := firstCompatStringDeep(normalized, "cookies", "cookie", "Cookie"); v != "" {
			normalized["cookies"] = v
		}
	}
	if strings.TrimSpace(asString(normalized["browser_target_id"])) == "" {
		if v := firstCompatStringDeep(normalized, "browser_target_id", "browserTargetId"); v != "" {
			normalized["browser_target_id"] = v
		}
	}

	return normalized
}
