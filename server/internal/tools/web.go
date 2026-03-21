package tools

import (
	"context"
	"errors"
	"strings"
)

// WebTool provides a single action-based surface for live web work.
type WebTool struct {
	search  Tool
	fetch   Tool
	read    Tool
	extract Tool
	crawl   Tool
}

// NewWebTool creates a unified web tool backed by the existing web subtools.
func NewWebTool(search, fetch, read, extract, crawl Tool) *WebTool {
	return &WebTool{
		search:  search,
		fetch:   fetch,
		read:    read,
		extract: extract,
		crawl:   crawl,
	}
}

func (t *WebTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "web",
		Description: "Unified web access tool. Actions: search, fetch, read, extract, crawl. Omit action to infer it: query-only defaults to search; URL defaults to read; fields/schema defaults to extract; seeds/depth/checkpoint defaults to crawl. Use fetch only when you explicitly want a lightweight HTTP read.",
		Icon:        "web-search",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"search", "fetch", "read", "extract", "crawl"},
					"description": "Optional web action. When omitted, Blue infers the action from the provided arguments.",
				},
				"query":             map[string]interface{}{"type": "string", "description": "Search query for action=search."},
				"url":               map[string]interface{}{"type": "string", "description": "Target URL for fetch/read/extract."},
				"max_results":       map[string]interface{}{"type": "integer", "description": "Maximum search results for action=search."},
				"region":            map[string]interface{}{"type": "string", "description": "Search region for action=search."},
				"provider":          map[string]interface{}{"type": "string", "description": "Optional web search provider override for action=search."},
				"format":            map[string]interface{}{"type": "string", "description": "Output format for action=search/read."},
				"extract_mode":      map[string]interface{}{"type": "string", "description": "Fetch extraction mode for action=fetch."},
				"lane":              map[string]interface{}{"type": "string", "description": "Fetch lane preference for read/extract/crawl."},
				"max_chars":         map[string]interface{}{"type": "integer", "description": "Maximum characters for fetch/read output."},
				"headers":           map[string]interface{}{"type": "object", "description": "Optional request headers.", "additionalProperties": map[string]interface{}{"type": "string"}},
				"cookies":           map[string]interface{}{"type": "string", "description": "Optional Cookie header value."},
				"authorization":     map[string]interface{}{"type": "string", "description": "Optional Authorization header value."},
				"auth_bearer":       map[string]interface{}{"type": "string", "description": "Optional bearer token."},
				"browser_target_id": map[string]interface{}{"type": "string", "description": "Optional browser tab target ID used for cookie reuse or browser fallback."},
				"html":              map[string]interface{}{"type": "string", "description": "Optional raw HTML for action=extract."},
				"fields":            map[string]interface{}{"type": "object", "description": "Structured extraction field spec for action=extract."},
				"seeds":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Seed URLs for action=crawl."},
				"allowed_hosts":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional crawl host allowlist."},
				"max_depth":         map[string]interface{}{"type": "integer", "description": "Maximum crawl depth for action=crawl."},
				"max_pages":         map[string]interface{}{"type": "integer", "description": "Maximum crawl pages for action=crawl."},
				"max_concurrency":   map[string]interface{}{"type": "integer", "description": "Maximum crawl concurrency for action=crawl."},
				"max_retries":       map[string]interface{}{"type": "integer", "description": "Maximum crawl retries for action=crawl."},
				"rate_limit_ms":     map[string]interface{}{"type": "integer", "description": "Per-host crawl rate limit in milliseconds for action=crawl."},
				"page_max_chars":    map[string]interface{}{"type": "integer", "description": "Maximum stored page characters for action=crawl."},
				"checkpoint":        map[string]interface{}{"type": "object", "description": "Optional crawl checkpoint for action=crawl."},
			},
			"additionalProperties": true,
		},
	}
}

func (t *WebTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil {
		return nil, errors.New("web tool is not available")
	}
	action := resolveWebAction(args)
	switch action {
	case "search":
		if t.search == nil {
			return nil, errors.New("web search is not available")
		}
		return t.search.Execute(ctx, normalizeWebSearchCompatArgs("web", args))
	case "fetch":
		if t.fetch == nil {
			return nil, errors.New("web fetch is not available")
		}
		return t.fetch.Execute(ctx, normalizeWebFetchCompatArgs("web", args))
	case "read":
		if t.read == nil {
			return nil, errors.New("web read is not available")
		}
		return t.read.Execute(ctx, normalizeWebReadCompatArgs(args))
	case "extract":
		if t.extract == nil {
			return nil, errors.New("web extract is not available")
		}
		return t.extract.Execute(ctx, normalizeWebExtractCompatArgs(args))
	case "crawl":
		if t.crawl == nil {
			return nil, errors.New("web crawl is not available")
		}
		return t.crawl.Execute(ctx, normalizeWebCrawlCompatArgs(args))
	default:
		return nil, errors.New("unknown web action: use search, fetch, read, extract, or crawl")
	}
}

func (t *WebTool) SetBrowser(browser BrowserBackend) {
	for _, candidate := range []Tool{t.fetch, t.read, t.extract} {
		if setter, ok := candidate.(interface{ SetBrowser(BrowserBackend) }); ok {
			setter.SetBrowser(browser)
		}
	}
}

func (t *WebTool) SetPDFService(service PDFService) {
	for _, candidate := range []Tool{t.fetch, t.read, t.extract, t.crawl} {
		if setter, ok := candidate.(interface{ SetPDFService(PDFService) }); ok {
			setter.SetPDFService(service)
		}
	}
}

func resolveWebAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	switch action {
	case "search", "fetch", "read", "extract", "crawl":
		return action
	}
	if hasWebCrawlArgs(args) {
		return "crawl"
	}
	if hasWebExtractArgs(args) {
		return "extract"
	}
	if strings.TrimSpace(firstCompatStringDeep(args, "url", "href", "target")) != "" {
		return "read"
	}
	if strings.TrimSpace(firstCompatStringDeep(args, "query", "q", "search", "keyword", "text", "input")) != "" {
		return "search"
	}
	return "search"
}

func hasWebExtractArgs(args map[string]interface{}) bool {
	if _, ok := firstCompatValueDeep(args, "fields", "html"); ok {
		return true
	}
	return false
}

func hasWebCrawlArgs(args map[string]interface{}) bool {
	if _, ok := firstCompatValueDeep(args, "seeds", "allowed_hosts", "checkpoint"); ok {
		return true
	}
	if _, ok := firstCompatValueDeep(args, "max_depth", "maxDepth", "depth", "max_pages", "maxPages"); ok {
		return true
	}
	return false
}

func normalizeWebReadCompatArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeWebFetchCompatArgs("web", args)
	if strings.TrimSpace(asString(normalized["format"])) == "" {
		if format := firstCompatStringDeep(normalized, "format"); format != "" {
			normalized["format"] = format
		}
	}
	if strings.TrimSpace(asString(normalized["lane"])) == "" {
		if lane := firstCompatStringDeep(normalized, "lane"); lane != "" {
			normalized["lane"] = lane
		}
	}
	return normalized
}

func normalizeWebExtractCompatArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeWebReadCompatArgs(args)
	if _, ok := normalized["fields"]; !ok {
		if fields, exists := firstCompatValueDeep(normalized, "fields"); exists {
			normalized["fields"] = fields
		}
	}
	if strings.TrimSpace(asString(normalized["html"])) == "" {
		if rawHTML := firstCompatStringDeep(normalized, "html"); rawHTML != "" {
			normalized["html"] = rawHTML
		}
	}
	return normalized
}

func normalizeWebCrawlCompatArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeWebReadCompatArgs(args)
	for _, key := range []string{
		"seeds", "allowed_hosts", "checkpoint", "max_depth", "max_pages", "max_concurrency", "max_retries", "rate_limit_ms", "page_max_chars",
	} {
		if _, ok := normalized[key]; ok {
			continue
		}
		if value, exists := firstCompatValueDeep(normalized, key); exists {
			normalized[key] = value
		}
	}
	if _, ok := normalized["allowed_hosts"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "allowedHosts", "hosts"); exists {
			normalized["allowed_hosts"] = value
		}
	}
	if _, ok := normalized["max_depth"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "maxDepth", "depth"); exists {
			normalized["max_depth"] = value
		}
	}
	if _, ok := normalized["max_pages"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "maxPages"); exists {
			normalized["max_pages"] = value
		}
	}
	if _, ok := normalized["max_concurrency"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "maxConcurrency"); exists {
			normalized["max_concurrency"] = value
		}
	}
	if _, ok := normalized["max_retries"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "maxRetries"); exists {
			normalized["max_retries"] = value
		}
	}
	if _, ok := normalized["rate_limit_ms"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "rateLimitMs"); exists {
			normalized["rate_limit_ms"] = value
		}
	}
	if _, ok := normalized["page_max_chars"]; !ok {
		if value, exists := firstCompatValueDeep(normalized, "pageMaxChars"); exists {
			normalized["page_max_chars"] = value
		}
	}
	return normalized
}
