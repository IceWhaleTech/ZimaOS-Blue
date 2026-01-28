package crawler_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ZimaOS-Echo/server/internal/crawler"
)

func Example_basicUsage() {
	// Create a crawler with default configuration
	config := crawler.DefaultConfig()
	config.MaxDepth = 1
	config.MaxConcurrency = 3

	c := crawler.New(config)

	// Crawl URLs
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	urls := []string{
		"https://example.com",
	}

	results := c.CrawlSync(ctx, urls)

	for _, result := range results {
		if result.Error != "" {
			fmt.Printf("Error crawling %s: %s\n", result.URL, result.Error)
			continue
		}
		fmt.Printf("Title: %s\n", result.Title)
		fmt.Printf("URL: %s\n", result.URL)
		fmt.Printf("Links found: %d\n", len(result.Links))
	}
}

func Example_withStorage() {
	config := crawler.DefaultConfig()
	c := crawler.New(config)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results := c.CrawlSync(ctx, []string{"https://example.com"})

	// Save to JSON
	jsonStorage := crawler.NewJSONStorage("./output/results.json")
	if err := jsonStorage.Save(results); err != nil {
		fmt.Printf("Failed to save JSON: %v\n", err)
	}

	// Save to CSV
	csvStorage := crawler.NewCSVStorage("./output/results.csv")
	if err := csvStorage.Save(results); err != nil {
		fmt.Printf("Failed to save CSV: %v\n", err)
	}
}

func Example_streamingResults() {
	config := crawler.DefaultConfig()
	c := crawler.New(config)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use streaming storage for large crawls
	storage, err := crawler.NewStreamingJSONStorage("./output/stream.jsonl")
	if err != nil {
		fmt.Printf("Failed to create storage: %v\n", err)
		return
	}
	defer storage.Close()

	// Process results as they come in
	resultChan := c.Crawl(ctx, []string{"https://example.com"})
	for result := range resultChan {
		// Save each result immediately
		if err := storage.SaveOne(result); err != nil {
			fmt.Printf("Failed to save result: %v\n", err)
		}
		fmt.Printf("Crawled: %s\n", result.URL)
	}
}

func Example_domainRestricted() {
	config := crawler.DefaultConfig()
	config.MaxDepth = 3
	config.AllowedDomains = []string{"example.com"} // Only crawl example.com

	c := crawler.New(config)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	results := c.CrawlSync(ctx, []string{"https://example.com"})

	fmt.Printf("Crawled %d pages from example.com\n", len(results))
}
