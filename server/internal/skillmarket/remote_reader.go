package skillmarket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type RemoteReadRequest struct {
	URL         string
	Headers     map[string]string
	Format      string
	MaxChars    int
	WantRawHTML bool
}

type RemoteReadResult struct {
	URL                 string   `json:"url,omitempty"`
	FinalURL            string   `json:"final_url,omitempty"`
	Title               string   `json:"title,omitempty"`
	Content             string   `json:"content,omitempty"`
	Source              string   `json:"source,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
	InteractiveRequired bool     `json:"interactive_required,omitempty"`
	StatusCode          int      `json:"status_code,omitempty"`
	Links               []string `json:"links,omitempty"`
	ContentType         string   `json:"content_type,omitempty"`
	RawHTML             string   `json:"raw_html,omitempty"`
	Truncated           bool     `json:"truncated,omitempty"`
}

type RemoteCrawlRequest struct {
	Seeds        []string
	AllowedHosts []string
	Headers      map[string]string
	MaxDepth     int
	MaxPages     int
	MaxRetries   int
	PageMaxChars int
	Checkpoint   *RemoteCrawlCheckpoint
}

type RemoteCrawlQueueItem struct {
	URL      string `json:"url"`
	Depth    int    `json:"depth"`
	Attempts int    `json:"attempts,omitempty"`
}

type RemoteCrawlCheckpoint struct {
	Pending   []RemoteCrawlQueueItem `json:"pending,omitempty"`
	Seen      []string               `json:"seen,omitempty"`
	Completed int                    `json:"completed,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

type RemoteCrawlFailure struct {
	URL       string `json:"url"`
	Depth     int    `json:"depth"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

type RemoteCrawlPage struct {
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
	RawHTML             string   `json:"raw_html,omitempty"`
	ContentType         string   `json:"content_type,omitempty"`
}

type RemoteCrawlResult struct {
	Pages      []RemoteCrawlPage     `json:"pages,omitempty"`
	Failures   []RemoteCrawlFailure  `json:"failures,omitempty"`
	Checkpoint RemoteCrawlCheckpoint `json:"checkpoint"`
	Stats      map[string]int        `json:"stats,omitempty"`
	Warnings   []string              `json:"warnings,omitempty"`
}

type RemoteReader interface {
	ReadURL(ctx context.Context, req RemoteReadRequest) (*RemoteReadResult, error)
	CrawlSite(ctx context.Context, req RemoteCrawlRequest) (*RemoteCrawlResult, error)
}

type httpRemoteReader struct {
	client *http.Client
}

func newHTTPRemoteReader(client *http.Client) RemoteReader {
	return &httpRemoteReader{client: client}
}

func (r *httpRemoteReader) ReadURL(ctx context.Context, req RemoteReadRequest) (*RemoteReadResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(req.URL), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "ZimaOS-SkillMarket/1.0")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,text/markdown,application/json,*/*")
	for key, value := range req.Headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			httpReq.Header.Set(key, value)
		}
	}
	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("remote read status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	finalURL := req.URL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	result := &RemoteReadResult{
		URL:         req.URL,
		FinalURL:    finalURL,
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		Source:      "http",
	}
	if looksLikeRemoteHTMLDocument(body, contentType) {
		rawHTML := string(body)
		doc, err := html.Parse(strings.NewReader(rawHTML))
		if err != nil {
			return nil, err
		}
		result.Title = strings.TrimSpace(extractHTMLTitle(doc))
		result.Content = strings.TrimSpace(extractHTMLText(doc))
		result.Links = extractHTMLLinks(doc)
		if req.WantRawHTML {
			result.RawHTML = rawHTML
		}
		return result, nil
	}
	result.Content = string(body)
	if req.MaxChars > 0 && len(result.Content) > req.MaxChars {
		result.Content = result.Content[:req.MaxChars]
		result.Truncated = true
	}
	return result, nil
}

func (r *httpRemoteReader) CrawlSite(ctx context.Context, req RemoteCrawlRequest) (*RemoteCrawlResult, error) {
	maxDepth := req.MaxDepth
	if maxDepth < 0 {
		maxDepth = 0
	}
	maxPages := req.MaxPages
	if maxPages <= 0 {
		maxPages = 20
	}
	pageMaxChars := req.PageMaxChars
	if pageMaxChars <= 0 {
		pageMaxChars = 2000
	}
	checkpoint := req.Checkpoint
	if checkpoint == nil {
		checkpoint = &RemoteCrawlCheckpoint{}
	}
	queue := append([]RemoteCrawlQueueItem(nil), checkpoint.Pending...)
	if len(queue) == 0 {
		for _, seed := range req.Seeds {
			canonical, err := canonicalizeRemoteCrawlURL(seed)
			if err != nil {
				continue
			}
			queue = append(queue, RemoteCrawlQueueItem{URL: canonical, Depth: 0})
		}
	}
	if len(req.AllowedHosts) == 0 {
		req.AllowedHosts = deriveRemoteAllowedHosts(queue)
	}
	seen := make(map[string]struct{}, len(checkpoint.Seen)+len(queue))
	for _, item := range checkpoint.Seen {
		seen[item] = struct{}{}
	}
	for _, item := range queue {
		seen[item.URL] = struct{}{}
	}

	completed := checkpoint.Completed
	pages := make([]RemoteCrawlPage, 0, maxPages)
	failures := make([]RemoteCrawlFailure, 0)
	for len(queue) > 0 && len(pages) < maxPages {
		item := queue[0]
		queue = queue[1:]
		if !remoteCrawlHostAllowed(item.URL, req.AllowedHosts) {
			failures = append(failures, RemoteCrawlFailure{URL: item.URL, Depth: item.Depth, Code: "disallowed_host", Message: "host is not allowed"})
			completed++
			continue
		}
		read, err := r.ReadURL(ctx, RemoteReadRequest{
			URL:         item.URL,
			Headers:     req.Headers,
			Format:      "text",
			MaxChars:    pageMaxChars,
			WantRawHTML: true,
		})
		if err != nil {
			failures = append(failures, RemoteCrawlFailure{URL: item.URL, Depth: item.Depth, Code: "read_failed", Message: err.Error()})
			completed++
			continue
		}
		page := RemoteCrawlPage{
			URL:                 item.URL,
			FinalURL:            read.FinalURL,
			Title:               read.Title,
			Depth:               item.Depth,
			StatusCode:          read.StatusCode,
			Content:             trimRemoteContent(read.Content, pageMaxChars),
			Source:              read.Source,
			Warnings:            append([]string(nil), read.Warnings...),
			InteractiveRequired: read.InteractiveRequired,
			Links:               append([]string(nil), read.Links...),
			Truncated:           read.Truncated || len(read.Content) > pageMaxChars,
			RawHTML:             read.RawHTML,
			ContentType:         read.ContentType,
		}
		pages = append(pages, page)
		completed++
		if item.Depth >= maxDepth {
			continue
		}
		for _, link := range read.Links {
			resolved, ok := resolveRemoteCrawlLink(firstNonBlank(read.FinalURL, item.URL), link)
			if !ok || !remoteCrawlHostAllowed(resolved, req.AllowedHosts) {
				continue
			}
			if _, exists := seen[resolved]; exists {
				continue
			}
			seen[resolved] = struct{}{}
			queue = append(queue, RemoteCrawlQueueItem{URL: resolved, Depth: item.Depth + 1})
		}
	}
	return &RemoteCrawlResult{
		Pages:    pages,
		Failures: failures,
		Checkpoint: RemoteCrawlCheckpoint{
			Pending:   queue,
			Seen:      sortedRemoteSeen(seen),
			Completed: completed,
			CreatedAt: checkpointCreatedAt(checkpoint),
		},
		Stats: map[string]int{
			"completed": completed,
			"failed":    len(failures),
			"queued":    len(queue),
		},
	}, nil
}

func canonicalizeRemoteCrawlURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func resolveRemoteCrawlLink(baseURL, href string) (string, bool) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", false
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return "", false
	}
	resolved.Fragment = ""
	return resolved.String(), true
}

func deriveRemoteAllowedHosts(queue []RemoteCrawlQueueItem) []string {
	seen := make(map[string]struct{}, len(queue))
	out := make([]string, 0, len(queue))
	for _, item := range queue {
		parsed, err := url.Parse(item.URL)
		if err != nil {
			continue
		}
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		if host == "" {
			continue
		}
		if _, exists := seen[host]; exists {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func remoteCrawlHostAllowed(rawURL string, allowedHosts []string) bool {
	if len(allowedHosts) == 0 {
		return true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	for _, candidate := range allowedHosts {
		if host == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func sortedRemoteSeen(seen map[string]struct{}) []string {
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for item := range seen {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func checkpointCreatedAt(checkpoint *RemoteCrawlCheckpoint) string {
	if checkpoint == nil || strings.TrimSpace(checkpoint.CreatedAt) == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return checkpoint.CreatedAt
}

func trimRemoteContent(content string, maxChars int) string {
	if maxChars <= 0 || len(content) <= maxChars {
		return content
	}
	return content[:maxChars]
}

func looksLikeRemoteHTMLDocument(body []byte, contentType string) bool {
	lowerType := strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(lowerType, "text/html") || strings.Contains(lowerType, "application/xhtml+xml") {
		return true
	}
	snippet := strings.ToLower(strings.TrimSpace(string(body)))
	if len(snippet) > 1024 {
		snippet = snippet[:1024]
	}
	return strings.HasPrefix(snippet, "<!doctype html") || strings.HasPrefix(snippet, "<html") || (strings.Contains(snippet, "<head") && strings.Contains(snippet, "<body"))
}
