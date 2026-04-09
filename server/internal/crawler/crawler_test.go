package crawler_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/crawler"
)

func TestCrawler_BasicFetch(t *testing.T) {
	// Create a test server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head><title>Test Page</title></head>
			<body>
				<h1>Hello World</h1>
				<a href="/page1">Page 1</a>
				<a href="/page2">Page 2</a>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	config := crawler.DefaultConfig()
	config.MaxDepth = 0 // Only fetch the initial URL
	config.MaxConcurrency = 1

	c := crawler.New(config)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := c.CrawlSync(ctx, []string{server.URL})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Title != "Test Page" {
		t.Errorf("expected title 'Test Page', got '%s'", result.Title)
	}

	if result.Status != 200 {
		t.Errorf("expected status 200, got %d", result.Status)
	}

	if len(result.Links) != 2 {
		t.Errorf("expected 2 links, got %d", len(result.Links))
	}

	if !strings.Contains(result.Text, "Hello World") {
		t.Errorf("expected text to contain 'Hello World', got '%s'", result.Text)
	}
}

func TestCrawler_DepthCrawling(t *testing.T) {
	visitCount := 0
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visitCount++
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><head><title>Home</title></head><body><a href="/page1">Page 1</a></body></html>`))
		case "/page1":
			w.Write([]byte(`<html><head><title>Page 1</title></head><body><a href="/page2">Page 2</a></body></html>`))
		case "/page2":
			w.Write([]byte(`<html><head><title>Page 2</title></head><body>End</body></html>`))
		}
	}))
	defer server.Close()

	config := crawler.DefaultConfig()
	config.MaxDepth = 2
	config.MaxConcurrency = 1
	config.RateLimit = 0 // Disable rate limiting for tests

	c := crawler.New(config)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results := c.CrawlSync(ctx, []string{server.URL})

	if len(results) < 2 {
		t.Errorf("expected at least 2 results with depth crawling, got %d", len(results))
	}
}

func TestCrawler_DomainRestriction(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Test</title></head><body><a href="https://external.com/page">External</a></body></html>`))
	}))
	defer server.Close()

	config := crawler.DefaultConfig()
	config.MaxDepth = 1
	config.AllowedDomains = []string{"127.0.0.1"}

	c := crawler.New(config)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := c.CrawlSync(ctx, []string{server.URL})

	// Should only have the initial page, not the external link
	if len(results) != 1 {
		t.Errorf("expected 1 result (domain restricted), got %d", len(results))
	}
}

func TestCrawler_ErrorHandling(t *testing.T) {
	config := crawler.DefaultConfig()
	config.RequestTimeout = 1 * time.Second

	c := crawler.New(config)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to crawl an invalid URL
	results := c.CrawlSync(ctx, []string{"http://invalid.localhost.test:99999"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Error == "" {
		t.Error("expected an error for invalid URL")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := crawler.DefaultConfig()

	if config.MaxDepth != 2 {
		t.Errorf("expected MaxDepth 2, got %d", config.MaxDepth)
	}

	if config.MaxConcurrency != 5 {
		t.Errorf("expected MaxConcurrency 5, got %d", config.MaxConcurrency)
	}

	if config.RequestTimeout != 5*time.Minute {
		t.Errorf("expected RequestTimeout 5m, got %v", config.RequestTimeout)
	}
}
