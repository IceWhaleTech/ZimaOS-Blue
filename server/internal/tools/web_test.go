package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type scriptedWebTool struct {
	mu       sync.Mutex
	def      ToolDefinition
	exec     func(args map[string]interface{}) (interface{}, error)
	lastArgs []map[string]interface{}
}

type scriptedWebSearchFanoutTool struct {
	scriptedWebTool
	providers []string
}

type scriptedWebQueryBrowserBackend struct {
	mu           sync.Mutex
	startErr     error
	executeErr   error
	execute      func(recipe string, params map[string]string) (BrowserRecipeResult, error)
	recipeCalls  int
	recipeName   string
	recipeParams map[string]string
}

func (t *scriptedWebTool) Definition() ToolDefinition {
	return t.def
}

func (t *scriptedWebTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	cloned := make(map[string]interface{}, len(args))
	for k, v := range args {
		cloned[k] = v
	}
	t.mu.Lock()
	t.lastArgs = append(t.lastArgs, cloned)
	t.mu.Unlock()
	if t.exec == nil {
		return "{}", nil
	}
	return t.exec(args)
}

func (t *scriptedWebSearchFanoutTool) providerChain(raw interface{}) []string {
	if chain := parseProviderChainArg(raw); len(chain) > 0 {
		return chain
	}
	return append([]string(nil), t.providers...)
}

func (b *scriptedWebQueryBrowserBackend) Start(_ context.Context) error { return b.startErr }
func (b *scriptedWebQueryBrowserBackend) Navigate(_ context.Context, url string, targetID string) (BrowserNavResult, error) {
	return BrowserNavResult{URL: url, TargetID: targetID}, nil
}
func (b *scriptedWebQueryBrowserBackend) CookieHeader(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (b *scriptedWebQueryBrowserBackend) ObserveNetwork(_ context.Context, _ string, _ int, _ bool) (BrowserObservedNetworkResult, error) {
	return BrowserObservedNetworkResult{}, nil
}
func (b *scriptedWebQueryBrowserBackend) WaitNetworkIdle(_ context.Context, _ string, _ int, _ int) error {
	return nil
}
func (b *scriptedWebQueryBrowserBackend) AccessibilityTree(_ context.Context, _ string, _ int) (BrowserA11yTreeResult, error) {
	return BrowserA11yTreeResult{}, nil
}
func (b *scriptedWebQueryBrowserBackend) InteractiveElements(_ context.Context, _ string) (BrowserInteractiveResult, error) {
	return BrowserInteractiveResult{}, nil
}
func (b *scriptedWebQueryBrowserBackend) CountInteractiveElements(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (b *scriptedWebQueryBrowserBackend) ActByRef(_ context.Context, _ string, _ int, _ map[int]int, _ string, _ string) error {
	return nil
}
func (b *scriptedWebQueryBrowserBackend) ActByInteractiveRef(_ context.Context, _ string, _ int, _ map[int]string, _ string, _ string) error {
	return nil
}
func (b *scriptedWebQueryBrowserBackend) Screenshot(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (b *scriptedWebQueryBrowserBackend) ScreenshotTab(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (b *scriptedWebQueryBrowserBackend) CloseTab(_ context.Context, _ string) error { return nil }
func (b *scriptedWebQueryBrowserBackend) Tabs(_ context.Context) ([]BrowserTabResult, error) {
	return nil, nil
}
func (b *scriptedWebQueryBrowserBackend) ExecuteRecipe(_ context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error) {
	cloned := make(map[string]string, len(params))
	for k, v := range params {
		cloned[k] = v
	}
	b.mu.Lock()
	b.recipeCalls++
	b.recipeName = recipe
	b.recipeParams = cloned
	b.mu.Unlock()
	if b.execute != nil {
		return b.execute(recipe, cloned)
	}
	return BrowserRecipeResult{}, b.executeErr
}
func (b *scriptedWebQueryBrowserBackend) ListRecipes(_ context.Context) []BrowserRecipeInfo {
	return nil
}

func TestWebQueryToolAutoRoutesSearchAndReturnsEnvelope(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			if got := args["format"]; got != "json" {
				t.Fatalf("search format = %v, want json", got)
			}
			resp := WebSearchResponse{
				Query: "zimaos blue",
				Results: []WebSearchResult{
					{Title: "Best Match", URL: "https://example.com/best", Description: "Official best result"},
					{Title: "Second Match", URL: "https://example.com/second", Description: "Secondary result"},
				},
				TotalCount: 2,
				Provider:   "duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
			}
			switch url {
			case "https://example.com/best":
				resp.Title = "Best Match"
				resp.Content = "ZimaOS Blue release notes and product details with enough readable text to count as the primary result. This version includes release changes, deployment notes, compatibility details, and several sentences of body copy so the orchestrator should keep it as a strong successful read."
			default:
				resp.Title = "Second Match"
				resp.Content = "Short fallback."
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "zimaos blue",
		"depth": "standard",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Mode != "search_read" {
		t.Fatalf("mode = %q, want search_read", envelope.Mode)
	}
	if envelope.Status != webQueryStatusOK {
		t.Fatalf("status = %q, want %q", envelope.Status, webQueryStatusOK)
	}
	if envelope.Title != "Best Match" {
		t.Fatalf("title = %q, want Best Match", envelope.Title)
	}
	if envelope.TargetURL != "https://example.com/best" {
		t.Fatalf("target_url = %q, want https://example.com/best", envelope.TargetURL)
	}
	if len(envelope.Sources) < 2 {
		t.Fatalf("sources len = %d, want at least 2", len(envelope.Sources))
	}
}

func TestWebQueryToolDoesNotAnalyzeMediaUnlessRequested(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "release image",
				Results: []WebSearchResult{{
					Title:       "Example",
					URL:         "https://example.com/article",
					Description: "Article result",
				}},
				TotalCount: 1,
				Provider:   "duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      "https://example.com/article",
				FinalURL: "https://example.com/article",
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Example",
				Content:  "A fully readable article body that is long enough to count as a strong successful read for the web query pipeline.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	extractTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_extract", Description: "extract", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
	}
	imageTool := &scriptedWebTool{
		def: ToolDefinition{Name: "image", Description: "image", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, extractTool, nil)
	tool.SetImageTool(imageTool)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "release image",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Media != nil {
		t.Fatalf("media = %+v, want nil by default", envelope.Media)
	}
	if len(extractTool.lastArgs) != 0 {
		t.Fatalf("extract calls = %d, want 0 when include_media is unset", len(extractTool.lastArgs))
	}
	if len(imageTool.lastArgs) != 0 {
		t.Fatalf("image calls = %d, want 0 when include_media is unset", len(imageTool.lastArgs))
	}
}

func TestWebQueryToolIncludeMediaAddsImageSummary(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "dashboard launch",
				Results: []WebSearchResult{{
					Title:       "Launch Post",
					URL:         "https://example.com/launch",
					Description: "Launch details",
				}},
				TotalCount: 1,
				Provider:   "duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      "https://example.com/launch",
				FinalURL: "https://example.com/launch",
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Launch Post",
				Content:  "This launch post includes a long enough article body to satisfy the strong-read threshold before the media enrichment step runs.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	extractTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_extract", Description: "extract", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webQueryMediaExtractResponse{
				Data: map[string]interface{}{
					"og_image":     "/images/hero.png",
					"og_image_alt": "Launch dashboard hero",
				},
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	imageTool := &scriptedWebTool{
		def: ToolDefinition{Name: "image", Description: "image", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"mode":       "vision",
				"source_url": "https://example.com/images/hero.png",
				"analysis":   "The hero image shows a product analytics dashboard with KPI cards and a trend chart.",
			}, nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, extractTool, nil)
	tool.SetImageTool(imageTool)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":         "dashboard launch",
		"include_media": true,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Media == nil {
		t.Fatal("expected media summary to be populated")
	}
	if !strings.Contains(envelope.Media.Summary, "analytics dashboard") {
		t.Fatalf("media summary = %q, want dashboard analysis", envelope.Media.Summary)
	}
	if len(envelope.Media.Items) != 1 {
		t.Fatalf("media items len = %d, want 1", len(envelope.Media.Items))
	}
	if envelope.Media.Items[0].URL != "https://example.com/images/hero.png" {
		t.Fatalf("media url = %q, want resolved hero url", envelope.Media.Items[0].URL)
	}
	if got := extractTool.lastArgs[0]["url"]; got != "https://example.com/launch" {
		t.Fatalf("extract url = %v, want https://example.com/launch", got)
	}
	if got := imageTool.lastArgs[0]["prompt"]; got == "" {
		t.Fatal("expected media analysis prompt to be forwarded")
	}
	if got := imageTool.lastArgs[0]["analysis_mode"]; got != imageAnalysisModeOCRFirst {
		t.Fatalf("analysis_mode = %v, want %q", got, imageAnalysisModeOCRFirst)
	}
}

func TestWebQueryToolMediaModeOCRForcesOCROnly(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "quarterly report",
				Results: []WebSearchResult{{
					Title:       "Quarterly Report",
					URL:         "https://example.com/report",
					Description: "Results and charts",
				}},
				TotalCount: 1,
				Provider:   "duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      "https://example.com/report",
				FinalURL: "https://example.com/report",
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Quarterly Report",
				Content:  "The report page has a readable body, but the attached chart image contains the key numbers that OCR should be allowed to extract directly.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	extractTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_extract", Description: "extract", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webQueryMediaExtractResponse{
				Data: map[string]interface{}{
					"og_image":     "/images/report-chart.png",
					"og_image_alt": "Quarterly revenue chart screenshot",
				},
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	imageTool := &scriptedWebTool{
		def: ToolDefinition{Name: "image", Description: "image", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"mode":       "ocr",
				"source_url": "https://example.com/images/report-chart.png",
				"analysis":   "Revenue rose 18% year over year.",
			}, nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, extractTool, nil)
	tool.SetImageTool(imageTool)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":         "quarterly report",
		"include_media": true,
		"media_mode":    "ocr",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Media == nil {
		t.Fatal("expected media payload")
	}
	if envelope.Media.AnalysisMode != imageAnalysisModeOCROnly {
		t.Fatalf("media analysis_mode = %q, want %q", envelope.Media.AnalysisMode, imageAnalysisModeOCROnly)
	}
	if len(envelope.Media.Items) != 1 || envelope.Media.Items[0].Mode != "ocr" {
		t.Fatalf("media items = %+v, want one OCR result", envelope.Media.Items)
	}
	if got := imageTool.lastArgs[0]["analysis_mode"]; got != imageAnalysisModeOCROnly {
		t.Fatalf("analysis_mode = %v, want %q", got, imageAnalysisModeOCROnly)
	}
}

func TestWebQueryToolAutoMediaPrefersOCRFirstForTextHeavyImages(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "dashboard launch",
				Results: []WebSearchResult{{
					Title:       "Dashboard Launch",
					URL:         "https://example.com/dashboard",
					Description: "Dashboard screenshots",
				}},
				TotalCount: 1,
				Provider:   "duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      "https://example.com/dashboard",
				FinalURL: "https://example.com/dashboard",
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Dashboard Launch",
				Content:  "This page includes a solid launch write-up and a screenshot that contains the actual dashboard metrics.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	extractTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_extract", Description: "extract", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webQueryMediaExtractResponse{
				Data: map[string]interface{}{
					"og_image":     "/images/dashboard-screenshot.png",
					"og_image_alt": "Analytics dashboard screenshot with KPI cards",
				},
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	imageTool := &scriptedWebTool{
		def: ToolDefinition{Name: "image", Description: "image", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"mode":       "ocr",
				"source_url": "https://example.com/images/dashboard-screenshot.png",
				"analysis":   "KPI cards show conversion, ARR, and active users.",
			}, nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, extractTool, nil)
	tool.SetImageTool(imageTool)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":         "dashboard launch",
		"include_media": true,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Media == nil {
		t.Fatal("expected media payload")
	}
	if envelope.Media.AnalysisMode != imageAnalysisModeOCRFirst {
		t.Fatalf("media analysis_mode = %q, want %q", envelope.Media.AnalysisMode, imageAnalysisModeOCRFirst)
	}
	if got := imageTool.lastArgs[0]["analysis_mode"]; got != imageAnalysisModeOCRFirst {
		t.Fatalf("analysis_mode = %v, want %q", got, imageAnalysisModeOCRFirst)
	}
}

func TestWebQueryToolAutoMediaUsesCheapFirstForVisualImages(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "nas launch",
				Results: []WebSearchResult{{
					Title:       "NAS Launch",
					URL:         "https://example.com/nas",
					Description: "Product visuals",
				}},
				TotalCount: 1,
				Provider:   "duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      "https://example.com/nas",
				FinalURL: "https://example.com/nas",
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "NAS Launch",
				Content:  "This launch page includes standard product copy and a hardware hero photo that adds visual context but is not primarily a screenshot or text-heavy asset.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	extractTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_extract", Description: "extract", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webQueryMediaExtractResponse{
				Data: map[string]interface{}{
					"og_image":     "/images/device-photo.png",
					"og_image_alt": "Rackmount NAS product photo",
				},
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	imageTool := &scriptedWebTool{
		def: ToolDefinition{Name: "image", Description: "image", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"mode":       "small_model",
				"source_url": "https://example.com/images/device-photo.png",
				"analysis":   "A rackmount NAS device on a clean studio background.",
			}, nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, extractTool, nil)
	tool.SetImageTool(imageTool)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":         "nas launch",
		"include_media": true,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Media == nil {
		t.Fatal("expected media payload")
	}
	if envelope.Media.AnalysisMode != imageAnalysisModeCheapFirst {
		t.Fatalf("media analysis_mode = %q, want %q", envelope.Media.AnalysisMode, imageAnalysisModeCheapFirst)
	}
	if got := imageTool.lastArgs[0]["analysis_mode"]; got != imageAnalysisModeCheapFirst {
		t.Fatalf("analysis_mode = %v, want %q", got, imageAnalysisModeCheapFirst)
	}
}

func TestWebQueryToolAutoMediaSkipsLowValuePreviewImages(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "marketing launch",
				Results: []WebSearchResult{{
					Title:       "Marketing Launch",
					URL:         "https://example.com/launch",
					Description: "Hero art only",
				}},
				TotalCount: 1,
				Provider:   "duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      "https://example.com/launch",
				FinalURL: "https://example.com/launch",
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Marketing Launch",
				Content:  "The article body is already long enough to fully explain the launch without needing a decorative social hero image. It includes the product positioning, rollout timing, audience details, packaging notes, and launch rationale in plain text, along with enough surrounding context that a reader can understand the page without looking at the share banner artwork.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	extractTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_extract", Description: "extract", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webQueryMediaExtractResponse{
				Data: map[string]interface{}{
					"og_image": "/images/social-hero-banner.png",
				},
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	imageTool := &scriptedWebTool{
		def: ToolDefinition{Name: "image", Description: "image", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, extractTool, nil)
	tool.SetImageTool(imageTool)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":         "marketing launch",
		"include_media": true,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Media != nil {
		t.Fatalf("media = %+v, want nil when only low-value preview images remain", envelope.Media)
	}
	if len(imageTool.lastArgs) != 0 {
		t.Fatalf("image calls = %d, want 0 when media is skipped", len(imageTool.lastArgs))
	}
	foundSkipWarning := false
	for _, warning := range envelope.Warnings {
		if warning.Code == "media_skipped" && strings.Contains(warning.Message, "low-value") {
			foundSkipWarning = true
			break
		}
	}
	if !foundSkipWarning {
		t.Fatalf("warnings = %+v, want media_skipped low-value warning", envelope.Warnings)
	}
}

func TestWebQueryToolURLPipelineFallsBackToBrowser(t *testing.T) {
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			lane := args["lane"].(string)
			resp := webReadResponse{
				URL:      "https://example.com/protected",
				FinalURL: "https://example.com/protected",
				Format:   "text",
				Source:   lane,
			}
			switch lane {
			case webAccessLaneHTTP:
				resp.Title = "Protected"
				resp.Content = "Please open in a browser."
				resp.Warnings = []string{"page requires a browser session"}
				resp.WarningCodes = []string{webFetchWarningCodeBrowserRequired}
				resp.InteractiveRequired = true
			case webAccessLaneProxyFetcher:
				resp.Title = "Protected"
				resp.Content = "Proxy fetch still cannot unlock this page."
				resp.Source = webAccessSourceProxyFetcher
				resp.Warnings = []string{"page still requires browser interaction"}
				resp.WarningCodes = []string{webFetchWarningCodeBrowserRequired}
				resp.InteractiveRequired = true
			case webAccessLaneBrowser:
				resp.Title = "Protected"
				resp.Content = "Full browser-rendered content after the automatic fallback."
				resp.Source = webAccessSourceBrowser
			default:
				t.Fatalf("unexpected lane %q", lane)
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(nil, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "https://example.com/protected",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Status != webQueryStatusOK {
		t.Fatalf("status = %q, want %q", envelope.Status, webQueryStatusOK)
	}
	if len(envelope.Diagnostics.Attempts) < 2 {
		t.Fatalf("attempts len = %d, want at least 2", len(envelope.Diagnostics.Attempts))
	}
	if envelope.Diagnostics.Attempts[0].Mode != webAccessLaneHTTP {
		t.Fatalf("first attempt = %+v, want http", envelope.Diagnostics.Attempts[0])
	}
	browserSeen := false
	for _, attempt := range envelope.Diagnostics.Attempts {
		if attempt.Mode == webAccessLaneBrowser {
			browserSeen = true
			break
		}
	}
	if !browserSeen {
		t.Fatalf("attempts = %+v, want browser lane present", envelope.Diagnostics.Attempts)
	}
	if got := envelope.Sources[0].Source; got != webAccessSourceBrowser {
		t.Fatalf("source = %q, want %q", got, webAccessSourceBrowser)
	}
}

func TestWebQueryToolURLPipelineReportsHTTPNativeAttemptMode(t *testing.T) {
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Title:    "Native Winner",
				Source:   webAccessSourceHTTPNative,
				Content:  "This response came back from the internal native HTTP backend and is intentionally long enough to count as a strong readable result for the direct URL pipeline. The diagnostics layer should report the winning attempt as http_native even though the public read lane remained the ordinary HTTP entry point.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(nil, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "https://example.com/native",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	foundHTTPNative := false
	for _, attempt := range envelope.Diagnostics.Attempts {
		if attempt.Stage == "read" && attempt.Mode == webAccessLaneHTTPNative && attempt.Status == "ok" {
			foundHTTPNative = true
			break
		}
	}
	if !foundHTTPNative {
		t.Fatalf("attempts = %+v, want successful http_native read attempt", envelope.Diagnostics.Attempts)
	}
	if len(envelope.Sources) == 0 {
		t.Fatal("expected at least one source in envelope")
	}
	if got := envelope.Sources[0].Source; got != webAccessSourceHTTPNative {
		t.Fatalf("source = %q, want %q", got, webAccessSourceHTTPNative)
	}
}

func TestWebQueryToolURLPipelinePrefersBrowserAfterAdapterMemory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "login required", http.StatusUnauthorized)
	}))
	defer srv.Close()

	browser := &mockBrowserBackend{
		navResult: BrowserNavResult{URL: srv.URL, Title: "Adapter Page", TargetID: "tab-adapter"},
		a11yResult: BrowserA11yTreeResult{
			URL:      srv.URL,
			Title:    "Adapter Page",
			TargetID: "tab-adapter",
			Tree:     "Browser rendered content after rescue with enough detail to count as a strong read result.",
		},
		observed: BrowserObservedNetworkResult{
			TargetID: "tab-adapter",
			Events: []BrowserNetworkEvent{{
				Method:       http.MethodGet,
				URL:          srv.URL + "/api/items",
				Status:       http.StatusOK,
				ContentType:  "application/json",
				ResourceType: "xhr",
				BodySample:   `{"items":[{"title":"Adapter Page"}]}`,
			}},
		},
	}

	readTool := NewWebReadTool(WebFetchConfig{
		Timeout:               5 * time.Second,
		AllowPrivateHosts:     true,
		LayeredFetchEnabled:   true,
		DomainStrategyEnabled: true,
		AdapterMemoryEnabled:  true,
		NetworkObserveEnabled: true,
	})
	readTool.SetBrowser(browser)

	tool := NewWebQueryTool(nil, nil, readTool, nil, nil)
	tool.SetBrowser(browser)

	primed, err := readTool.runtime.base.orchestrator.Fetch(context.Background(), FetchRequest{
		URL:               srv.URL,
		Mode:              webReadFormatText,
		PreferredLane:     webAccessLaneBrowser,
		AllowBrowser:      true,
		AllowSession:      true,
		AllowAutoFallback: true,
	})
	if err != nil {
		t.Fatalf("prime fetch failed: %v", err)
	}
	if primed.StrategyUsed != webFetchStrategyBrowser {
		t.Fatalf("primed strategy_used = %q, want %q", primed.StrategyUsed, webFetchStrategyBrowser)
	}
	if !primed.NetworkObserved {
		t.Fatal("expected primed browser fetch to record observed network activity")
	}
	if strings.TrimSpace(primed.AdapterID) == "" {
		t.Fatal("expected primed browser fetch to synthesize an adapter id")
	}

	secondRaw, err := tool.Execute(context.Background(), map[string]interface{}{"input": srv.URL})
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}
	var secondEnvelope webQueryEnvelope
	if err := json.Unmarshal([]byte(secondRaw.(string)), &secondEnvelope); err != nil {
		t.Fatalf("decode second envelope: %v", err)
	}
	if len(secondEnvelope.Diagnostics.Attempts) != 1 {
		t.Fatalf("second attempts len = %d, want 1; attempts=%+v", len(secondEnvelope.Diagnostics.Attempts), secondEnvelope.Diagnostics.Attempts)
	}
	if secondEnvelope.Diagnostics.Attempts[0].Mode != webAccessLaneBrowser {
		t.Fatalf("second attempt = %+v, want browser first", secondEnvelope.Diagnostics.Attempts[0])
	}
	if secondEnvelope.Diagnostics.StrategyUsed != webFetchStrategyBrowser {
		t.Fatalf("second strategy_used = %q, want %q", secondEnvelope.Diagnostics.StrategyUsed, webFetchStrategyBrowser)
	}
}

func TestWebQueryToolFansOutSearchProviders(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	done := make(chan struct{})

	searchTool := &scriptedWebSearchFanoutTool{
		providers: []string{"alpha", "beta"},
		scriptedWebTool: scriptedWebTool{
			def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
			exec: func(args map[string]interface{}) (interface{}, error) {
				provider, _ := args["provider"].(string)
				started <- provider
				<-release

				resp := WebSearchResponse{Query: "parallel fetchers", Provider: provider}
				switch provider {
				case "alpha":
					resp.Results = []WebSearchResult{
						{Title: "Shared Result", URL: "https://example.com/shared", Description: "Shared official page"},
						{Title: "Alpha Result", URL: "https://example.com/alpha", Description: "Alpha source"},
					}
				case "beta":
					resp.Results = []WebSearchResult{
						{Title: "Shared Result", URL: "https://example.com/shared", Description: "Shared official page"},
						{Title: "Beta Result", URL: "https://example.com/beta", Description: "Beta source"},
					}
				default:
					t.Fatalf("unexpected provider %q", provider)
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			},
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Shared Result",
				Content:  "Shared result carries the densest readable content and should win after provider aggregation.",
			}
			if strings.HasSuffix(url, "/alpha") {
				resp.Title = "Alpha Result"
				resp.Content = "Alpha content."
			}
			if strings.HasSuffix(url, "/beta") {
				resp.Title = "Beta Result"
				resp.Content = "Beta content."
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	var (
		raw interface{}
		err error
	)
	go func() {
		defer close(done)
		raw, err = tool.Execute(context.Background(), map[string]interface{}{
			"input": "parallel fetchers",
			"depth": "standard",
		})
	}()

	for idx := 0; idx < 2; idx++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("search providers did not start concurrently")
		}
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("web query did not finish after releasing provider fanout")
	}
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.TargetURL != "https://example.com/shared" {
		t.Fatalf("target_url = %q, want shared merged result", envelope.TargetURL)
	}
	if len(searchTool.lastArgs) != 2 {
		t.Fatalf("search calls = %d, want 2", len(searchTool.lastArgs))
	}
	if len(envelope.Diagnostics.Attempts) < 2 {
		t.Fatalf("attempts len = %d, want at least 2", len(envelope.Diagnostics.Attempts))
	}
}

func TestWebQueryToolReadsCandidatesInParallel(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	done := make(chan struct{})

	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "parallel reads",
				Results: []WebSearchResult{
					{Title: "First", URL: "https://example.com/one", Description: "First result"},
					{Title: "Second", URL: "https://example.com/two", Description: "Second result"},
				},
				TotalCount: 2,
				Provider:   "alpha",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			if got := args["disable_internal_fallbacks"]; got != true {
				t.Fatalf("disable_internal_fallbacks = %v, want true", got)
			}
			started <- args["url"].(string)
			<-release

			url := args["url"].(string)
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "First",
				Content:  "First result has enough readable content to be selected after the parallel read completes.",
			}
			if strings.HasSuffix(url, "/two") {
				resp.Title = "Second"
				resp.Content = "Second result is shorter."
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	var (
		raw interface{}
		err error
	)
	go func() {
		defer close(done)
		raw, err = tool.Execute(context.Background(), map[string]interface{}{
			"input": "parallel reads",
			"depth": "standard",
		})
	}()

	startedURLs := map[string]struct{}{}
	for idx := 0; idx < 2; idx++ {
		select {
		case url := <-started:
			startedURLs[url] = struct{}{}
		case <-time.After(2 * time.Second):
			t.Fatal("candidate reads did not start in parallel")
		}
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("web query did not finish after releasing candidate reads")
	}
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(startedURLs) != 2 {
		t.Fatalf("started URLs = %v, want both top candidates", startedURLs)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.TargetURL != "https://example.com/one" {
		t.Fatalf("target_url = %q, want first stronger candidate", envelope.TargetURL)
	}
}

func TestWebQueryToolFallsBackToBrowserSearchWhenHTTPSearchFails(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			return nil, errors.New("all HTTP providers failed")
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Browser Result",
				Content:  "Browser fallback found a readable page with enough content to satisfy the web query orchestrator after HTTP discovery failed completely. The page now includes release notes, deployment guidance, compatibility details, troubleshooting hints, and several full sentences so the result clearly crosses the strong-read threshold used by the selection pipeline.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	browser := &scriptedWebQueryBrowserBackend{
		execute: func(recipe string, params map[string]string) (BrowserRecipeResult, error) {
			if recipe != "search" {
				t.Fatalf("recipe = %q, want search", recipe)
			}
			return BrowserRecipeResult{
				Success: true,
				Data: map[string]interface{}{
					"engine": "bing",
					"results": []map[string]interface{}{{
						"title":   "Browser Result",
						"url":     "https://example.com/browser",
						"snippet": "Fallback result",
					}},
				},
			}, nil
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"input": "blue fallback"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Status != webQueryStatusOK {
		t.Fatalf("status = %q, want %q", envelope.Status, webQueryStatusOK)
	}
	if envelope.Diagnostics.Route != "search_browser" {
		t.Fatalf("route = %q, want search_browser", envelope.Diagnostics.Route)
	}
	if envelope.TargetURL != "https://example.com/browser" {
		t.Fatalf("target_url = %q, want browser fallback result", envelope.TargetURL)
	}
	if browser.recipeCalls != 1 {
		t.Fatalf("browser recipe calls = %d, want 1", browser.recipeCalls)
	}
	if browser.recipeParams["engine"] != "bing" {
		t.Fatalf("browser engine = %q, want bing", browser.recipeParams["engine"])
	}
	foundBrowserAttempt := false
	for _, attempt := range envelope.Diagnostics.Attempts {
		if attempt.Stage == "search_browser" && attempt.Status == "ok" {
			foundBrowserAttempt = true
			break
		}
	}
	if !foundBrowserAttempt {
		t.Fatalf("attempts = %+v, want successful search_browser attempt", envelope.Diagnostics.Attempts)
	}
}

func TestWebQueryToolFallsBackToBrowserSearchWhenCandidatesAreUnreadable(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "browser rescue",
				Results: []WebSearchResult{
					{Title: "Shared Result", URL: "https://example.com/shared", Description: "Shared HTTP result"},
					{Title: "Stale Result", URL: "https://example.com/stale", Description: "Stale HTTP result"},
				},
				TotalCount: 2,
				Provider:   "bing,duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Title:    "Unreadable",
				Source:   webAccessSourceHTTP,
			}
			switch url {
			case "https://example.com/shared", "https://example.com/stale":
				resp.Content = "Preview only."
				resp.WarningCodes = []string{webFetchWarningCodeBrowserRequired}
				resp.Warnings = []string{"browser required"}
				resp.InteractiveRequired = true
			case "https://example.com/browser":
				resp.Title = "Browser Winner"
				resp.Content = "Browser fallback produced the first fully readable body, with enough structured detail and length to beat the stale HTTP search results. It includes concrete product details, setup notes, compatibility constraints, and a few additional explanatory sentences so the content is clearly strong enough for the read scoring heuristics to prefer it over preview-only pages."
			default:
				t.Fatalf("unexpected read url %q", url)
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	browser := &scriptedWebQueryBrowserBackend{
		execute: func(recipe string, params map[string]string) (BrowserRecipeResult, error) {
			return BrowserRecipeResult{
				Success: true,
				Data: map[string]interface{}{
					"engine": "bing",
					"results": []map[string]interface{}{
						{"title": "Shared Result", "url": "https://example.com/shared", "snippet": "Duplicate result"},
						{"title": "Browser Winner", "url": "https://example.com/browser", "snippet": "Fresh browser result"},
					},
				},
			}, nil
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"input": "browser rescue"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.TargetURL != "https://example.com/browser" {
		t.Fatalf("target_url = %q, want browser rescue result", envelope.TargetURL)
	}
	if envelope.Diagnostics.Route != "search_browser" {
		t.Fatalf("route = %q, want search_browser", envelope.Diagnostics.Route)
	}
	sharedCount := 0
	for _, source := range envelope.Sources {
		if source.URL == "https://example.com/shared" {
			sharedCount++
		}
	}
	if sharedCount != 1 {
		t.Fatalf("shared result count = %d, want deduped once; sources=%+v", sharedCount, envelope.Sources)
	}
}

func TestWebQueryToolBrowserSearchFallbackOnlyRunsOnce(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "single retry",
				Results: []WebSearchResult{
					{Title: "HTTP Result", URL: "https://example.com/http", Description: "HTTP result"},
				},
				TotalCount: 1,
				Provider:   "bing",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Title:    "Still Weak",
				Source:   webAccessSourceHTTP,
				Content:  "Too short.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	browser := &scriptedWebQueryBrowserBackend{
		execute: func(recipe string, params map[string]string) (BrowserRecipeResult, error) {
			return BrowserRecipeResult{
				Success: true,
				Data: map[string]interface{}{
					"engine": "bing",
					"results": []map[string]interface{}{
						{"title": "Browser Weak", "url": "https://example.com/browser-weak", "snippet": "Still weak"},
					},
				},
			}, nil
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"input": "single retry"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Status != webQueryStatusPartial {
		t.Fatalf("status = %q, want %q", envelope.Status, webQueryStatusPartial)
	}
	if browser.recipeCalls != 1 {
		t.Fatalf("browser recipe calls = %d, want 1", browser.recipeCalls)
	}
	browserAttempts := 0
	for _, attempt := range envelope.Diagnostics.Attempts {
		if attempt.Stage == "search_browser" {
			browserAttempts++
		}
	}
	if browserAttempts != 1 {
		t.Fatalf("browser attempts = %d, want 1; attempts=%+v", browserAttempts, envelope.Diagnostics.Attempts)
	}
}

func TestWebQueryToolSearchFailureWithoutBrowserReturnsPartial(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			return nil, errors.New("temporary HTTP search failure")
		},
	}
	tool := NewWebQueryTool(searchTool, nil, nil, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"input": "no browser available"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Status != webQueryStatusPartial {
		t.Fatalf("status = %q, want %q", envelope.Status, webQueryStatusPartial)
	}
	if envelope.NextAction != webQueryNextActionRefineQuery {
		t.Fatalf("next_action = %q, want %q", envelope.NextAction, webQueryNextActionRefineQuery)
	}
	for _, warning := range envelope.Warnings {
		if strings.Contains(strings.ToLower(warning.Message), "config") {
			t.Fatalf("unexpected config prompt in warning: %+v", warning)
		}
	}
}

func TestWebQueryToolDelegatesLegacyActions(t *testing.T) {
	searchTool := &captureArgsTool{def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	fetchTool := &captureArgsTool{def: ToolDefinition{Name: "web_fetch", Description: "fetch", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	readTool := &captureArgsTool{def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	extractTool := &captureArgsTool{def: ToolDefinition{Name: "web_extract", Description: "extract", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	crawlTool := &captureArgsTool{def: ToolDefinition{Name: "web_crawl", Description: "crawl", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}}}
	tool := NewWebQueryTool(searchTool, fetchTool, readTool, extractTool, crawlTool)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "fetch", "href": "https://example.com/raw"}); err != nil {
		t.Fatalf("fetch execute failed: %v", err)
	}
	if got := fetchTool.args["url"]; got != "https://example.com/raw" {
		t.Fatalf("fetch url = %v, want https://example.com/raw", got)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"fields": map[string]interface{}{"title": "h1"}, "url": "https://example.com"}); err != nil {
		t.Fatalf("extract execute failed: %v", err)
	}
	if _, ok := extractTool.args["fields"]; !ok {
		t.Fatalf("extract args missing fields: %#v", extractTool.args)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"seeds": []interface{}{"https://example.com"}, "max_depth": 1}); err != nil {
		t.Fatalf("crawl execute failed: %v", err)
	}
	if got := crawlTool.args["seeds"]; got == nil {
		t.Fatalf("crawl args missing seeds: %#v", crawlTool.args)
	}
}
