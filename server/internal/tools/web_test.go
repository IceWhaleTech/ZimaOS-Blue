package tools

import (
	"context"
	"testing"
)

func TestWebToolInfersActionsAndNormalizesArgs(t *testing.T) {
	searchTool := &captureArgsTool{def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	fetchTool := &captureArgsTool{def: ToolDefinition{Name: "web_fetch", Description: "fetch", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	readTool := &captureArgsTool{def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	extractTool := &captureArgsTool{def: ToolDefinition{Name: "web_extract", Description: "extract", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	crawlTool := &captureArgsTool{def: ToolDefinition{Name: "web_crawl", Description: "crawl", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	tool := NewWebTool(searchTool, fetchTool, readTool, extractTool, crawlTool)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"query": "zimaos"}); err != nil {
		t.Fatalf("search execute failed: %v", err)
	}
	if got := searchTool.args["query"]; got != "zimaos" {
		t.Fatalf("search query = %v, want zimaos", got)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"url": "https://example.com"}); err != nil {
		t.Fatalf("read execute failed: %v", err)
	}
	if got := readTool.args["url"]; got != "https://example.com" {
		t.Fatalf("read url = %v, want https://example.com", got)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"url": "https://example.com", "fields": map[string]interface{}{"title": "h1"}}); err != nil {
		t.Fatalf("extract execute failed: %v", err)
	}
	if _, ok := extractTool.args["fields"]; !ok {
		t.Fatalf("extract args missing fields: %#v", extractTool.args)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"seeds": []interface{}{"https://example.com"}, "depth": 2}); err != nil {
		t.Fatalf("crawl execute failed: %v", err)
	}
	if got := crawlTool.args["max_depth"]; got != 2 {
		t.Fatalf("crawl max_depth = %v, want 2", got)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "fetch", "href": "https://example.com/raw"}); err != nil {
		t.Fatalf("fetch execute failed: %v", err)
	}
	if got := fetchTool.args["url"]; got != "https://example.com/raw" {
		t.Fatalf("fetch url = %v, want https://example.com/raw", got)
	}
}
