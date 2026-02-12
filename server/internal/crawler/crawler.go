// Package crawler provides a simple web crawler with concurrent support
package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// Result represents a crawled page result
type Result struct {
	URL       string            `json:"url"`
	Title     string            `json:"title"`
	Links     []string          `json:"links"`
	Text      string            `json:"text"`
	Headers   map[string]string `json:"headers"`
	Status    int               `json:"status"`
	Error     string            `json:"error,omitempty"`
	CrawledAt time.Time         `json:"crawled_at"`
}

// Config holds crawler configuration
type Config struct {
	// MaxDepth is the maximum depth to crawl (0 = only initial URLs)
	MaxDepth int
	// MaxConcurrency is the maximum number of concurrent requests
	MaxConcurrency int
	// RequestTimeout is the timeout for each HTTP request
	RequestTimeout time.Duration
	// UserAgent is the User-Agent header to use
	UserAgent string
	// RateLimit is the minimum delay between requests to the same domain
	RateLimit time.Duration
	// AllowedDomains restricts crawling to specific domains (empty = allow all)
	AllowedDomains []string
}

// DefaultConfig returns a default crawler configuration
func DefaultConfig() Config {
	return Config{
		MaxDepth:       2,
		MaxConcurrency: 5,
		RequestTimeout: 30 * time.Second,
		UserAgent:      "ZimaOS-Blue-Crawler/1.0",
		RateLimit:      time.Second,
		AllowedDomains: nil,
	}
}

// Crawler is a concurrent web crawler
type Crawler struct {
	config     Config
	client     *http.Client
	visited    map[string]bool
	visitedMu  sync.RWMutex
	rateLimits map[string]time.Time
	rateMu     sync.Mutex
	results    chan Result
}

// New creates a new Crawler with the given configuration
func New(config Config) *Crawler {
	return &Crawler{
		config: config,
		client: &http.Client{
			Timeout: config.RequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		visited:    make(map[string]bool),
		rateLimits: make(map[string]time.Time),
		results:    make(chan Result, 100),
	}
}

// Crawl starts crawling from the given URLs and returns results through a channel
func (c *Crawler) Crawl(ctx context.Context, urls []string) <-chan Result {
	var wg sync.WaitGroup
	sem := make(chan struct{}, c.config.MaxConcurrency)

	for _, u := range urls {
		wg.Add(1)
		go func(startURL string) {
			defer wg.Done()
			c.crawlURL(ctx, startURL, 0, sem)
		}(u)
	}

	go func() {
		wg.Wait()
		close(c.results)
	}()

	return c.results
}

// CrawlSync crawls URLs and returns all results as a slice
func (c *Crawler) CrawlSync(ctx context.Context, urls []string) []Result {
	resultChan := c.Crawl(ctx, urls)
	var results []Result
	for result := range resultChan {
		results = append(results, result)
	}
	return results
}

func (c *Crawler) crawlURL(ctx context.Context, targetURL string, depth int, sem chan struct{}) {
	if depth > c.config.MaxDepth {
		return
	}

	// Check if already visited
	c.visitedMu.Lock()
	if c.visited[targetURL] {
		c.visitedMu.Unlock()
		return
	}
	c.visited[targetURL] = true
	c.visitedMu.Unlock()

	// Check domain restrictions
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		c.results <- Result{URL: targetURL, Error: err.Error(), CrawledAt: time.Now()}
		return
	}

	if !c.isDomainAllowed(parsedURL.Host) {
		return
	}

	// Rate limiting
	c.waitForRateLimit(parsedURL.Host)

	// Acquire semaphore
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return
	}

	// Fetch the page
	result := c.fetchPage(ctx, targetURL)
	c.results <- result

	// Crawl discovered links
	if result.Error == "" && depth < c.config.MaxDepth {
		var wg sync.WaitGroup
		for _, link := range result.Links {
			absoluteURL := c.resolveURL(targetURL, link)
			if absoluteURL == "" {
				continue
			}

			wg.Add(1)
			go func(u string) {
				defer wg.Done()
				c.crawlURL(ctx, u, depth+1, sem)
			}(absoluteURL)
		}
		wg.Wait()
	}
}

func (c *Crawler) fetchPage(ctx context.Context, targetURL string) Result {
	result := Result{
		URL:       targetURL,
		Headers:   make(map[string]string),
		CrawledAt: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := c.client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.Status = resp.StatusCode

	// Store response headers
	for key := range resp.Header {
		result.Headers[key] = resp.Header.Get(key)
	}

	// Read body with limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Title = c.extractTitle(doc)
	result.Links = c.extractLinks(doc)
	result.Text = c.extractText(doc)

	return result
}

func (c *Crawler) extractTitle(doc *html.Node) string {
	var title string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = n.FirstChild.Data
			}
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			f(child)
		}
	}
	f(doc)
	return strings.TrimSpace(title)
}

func (c *Crawler) extractLinks(doc *html.Node) []string {
	var links []string
	seen := make(map[string]bool)

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href := strings.TrimSpace(attr.Val)
					if href != "" && !seen[href] && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
						seen[href] = true
						links = append(links, href)
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			f(child)
		}
	}
	f(doc)
	return links
}

func (c *Crawler) extractText(doc *html.Node) string {
	var sb strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Skip script, style, and other non-content elements
			switch n.Data {
			case "script", "style", "noscript", "iframe", "svg":
				return
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			f(child)
		}
	}
	f(doc)

	// Limit text length
	text := sb.String()
	if len(text) > 10000 {
		text = text[:10000] + "..."
	}
	return strings.TrimSpace(text)
}

func (c *Crawler) resolveURL(base, href string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}

	refURL, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := baseURL.ResolveReference(refURL)

	// Only allow http and https
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}

	return resolved.String()
}

func (c *Crawler) isDomainAllowed(domain string) bool {
	if len(c.config.AllowedDomains) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowedDomains {
		if domain == allowed || strings.HasSuffix(domain, "."+allowed) {
			return true
		}
	}
	return false
}

func (c *Crawler) waitForRateLimit(domain string) {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	if lastRequest, ok := c.rateLimits[domain]; ok {
		elapsed := time.Since(lastRequest)
		if elapsed < c.config.RateLimit {
			time.Sleep(c.config.RateLimit - elapsed)
		}
	}
	c.rateLimits[domain] = time.Now()
}
