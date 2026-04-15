package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
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

type contextAwareWebSearchFanoutTool struct {
	def       ToolDefinition
	providers []string
	exec      func(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

type scriptedWebQueryBrowserBackend struct {
	mu           sync.Mutex
	startErr     error
	executeErr   error
	execute      func(recipe string, params map[string]string) (BrowserRecipeResult, error)
	recipeCalls  int
	recipeName   string
	recipeParams map[string]string
	recipePlan   ExecutionPlan
	recipePlanOK bool
}

type blockingWebQueryBrowserBackend struct{}

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

func (t *contextAwareWebSearchFanoutTool) Definition() ToolDefinition {
	return t.def
}

func (t *contextAwareWebSearchFanoutTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t.exec == nil {
		return "{}", nil
	}
	return t.exec(ctx, args)
}

func (t *contextAwareWebSearchFanoutTool) providerChain(raw interface{}) []string {
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
func (b *scriptedWebQueryBrowserBackend) ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error) {
	cloned := make(map[string]string, len(params))
	for k, v := range params {
		cloned[k] = v
	}
	plan, ok := WebExecutionPlanFromContext(ctx)
	b.mu.Lock()
	b.recipeCalls++
	b.recipeName = recipe
	b.recipeParams = cloned
	b.recipePlan = plan
	b.recipePlanOK = ok
	b.mu.Unlock()
	if b.execute != nil {
		return b.execute(recipe, cloned)
	}
	return BrowserRecipeResult{}, b.executeErr
}
func (b *scriptedWebQueryBrowserBackend) ListRecipes(_ context.Context) []BrowserRecipeInfo {
	return nil
}

func (b *blockingWebQueryBrowserBackend) Start(_ context.Context) error { return nil }
func (b *blockingWebQueryBrowserBackend) Navigate(ctx context.Context, _ string, _ string) (BrowserNavResult, error) {
	<-ctx.Done()
	return BrowserNavResult{}, ctx.Err()
}
func (b *blockingWebQueryBrowserBackend) CookieHeader(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (b *blockingWebQueryBrowserBackend) ObserveNetwork(_ context.Context, _ string, _ int, _ bool) (BrowserObservedNetworkResult, error) {
	return BrowserObservedNetworkResult{}, nil
}
func (b *blockingWebQueryBrowserBackend) WaitNetworkIdle(_ context.Context, _ string, _ int, _ int) error {
	return nil
}
func (b *blockingWebQueryBrowserBackend) AccessibilityTree(_ context.Context, _ string, _ int) (BrowserA11yTreeResult, error) {
	return BrowserA11yTreeResult{}, nil
}
func (b *blockingWebQueryBrowserBackend) InteractiveElements(_ context.Context, _ string) (BrowserInteractiveResult, error) {
	return BrowserInteractiveResult{}, nil
}
func (b *blockingWebQueryBrowserBackend) CountInteractiveElements(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (b *blockingWebQueryBrowserBackend) ActByRef(_ context.Context, _ string, _ int, _ map[int]int, _ string, _ string) error {
	return nil
}
func (b *blockingWebQueryBrowserBackend) ActByInteractiveRef(_ context.Context, _ string, _ int, _ map[int]string, _ string, _ string) error {
	return nil
}
func (b *blockingWebQueryBrowserBackend) Screenshot(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (b *blockingWebQueryBrowserBackend) ScreenshotTab(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (b *blockingWebQueryBrowserBackend) CloseTab(_ context.Context, _ string) error { return nil }
func (b *blockingWebQueryBrowserBackend) Tabs(_ context.Context) ([]BrowserTabResult, error) {
	return nil, nil
}
func (b *blockingWebQueryBrowserBackend) ExecuteRecipe(_ context.Context, _ string, _ map[string]string) (BrowserRecipeResult, error) {
	return BrowserRecipeResult{}, nil
}
func (b *blockingWebQueryBrowserBackend) ListRecipes(_ context.Context) []BrowserRecipeInfo {
	return nil
}

func TestNormalizeWebQueryArgs_FlattensMediaAndRequestObjects(t *testing.T) {
	args := normalizeWebQueryArgs(map[string]interface{}{
		"media": map[string]interface{}{
			"mode":      "ocr",
			"max_items": 2,
		},
		"request": map[string]interface{}{
			"auth_bearer":       "tok",
			"browser_target_id": "tab-7",
			"headers":           map[string]interface{}{"X-Test": "1"},
		},
	})
	if got := asString(args["media_mode"]); got != "ocr" {
		t.Fatalf("media_mode = %q, want ocr", got)
	}
	if got := args["include_media"]; got != true {
		t.Fatalf("include_media = %#v, want true", got)
	}
	if got := args["max_media"]; got != 2 {
		t.Fatalf("max_media = %#v, want 2", got)
	}
	if got := asString(args["auth_bearer"]); got != "tok" {
		t.Fatalf("auth_bearer = %q, want tok", got)
	}
	if got := asString(args["browser_target_id"]); got != "tab-7" {
		t.Fatalf("browser_target_id = %q, want tab-7", got)
	}
	headers, ok := args["headers"].(map[string]interface{})
	if !ok || headers["X-Test"] != "1" {
		t.Fatalf("headers = %#v, want X-Test=1", args["headers"])
	}
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

func TestWebQueryToolEmitsSearchCardForSearchReadResults(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
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
				Title:    "Best Match",
				Content:  "A long readable article body that is comfortably above the strong-read threshold for search_read coverage in this regression test. It includes release notes, compatibility details, rollout steps, migration caveats, troubleshooting notes, several sentences of explanatory prose, and enough additional wording to clearly count as a strong successful read in the unified web query pipeline.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		cp := make(map[string]interface{}, len(card))
		for k, v := range card {
			cp[k] = v
		}
		emitted = append(emitted, cp)
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{"input": "zimaos blue"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !envelope.SearchCardEmitted {
		t.Fatal("expected search_card_emitted=true")
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted len=%d, want 1", len(emitted))
	}
	if got := emitted[0]["type"]; got != "search" {
		t.Fatalf("type=%v, want search", got)
	}
	if got := emitted[0]["status"]; got != "success" {
		t.Fatalf("status=%v, want success", got)
	}
	if got := emitted[0]["selectedUrl"]; got != "https://example.com/best" {
		t.Fatalf("selectedUrl=%v, want best result", got)
	}
	if got := emitted[0]["provider"]; got != "duckduckgo" {
		t.Fatalf("provider=%v, want duckduckgo", got)
	}
}

func TestWebQueryToolRecentProfileReturnsStructuredReportAndDropsUndatedGenericWeb(t *testing.T) {
	recentDate := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02")
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query:    asString(args["query"]),
				Provider: "duckduckgo",
				Results: []WebSearchResult{
					{Title: "Recent dated post", URL: "https://example.com/recent", Description: "Fresh discussion"},
					{Title: "Undated page", URL: "https://example.com/undated", Description: "No date available"},
				},
				TotalCount: 2,
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := asString(args["url"])
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
			}
			switch url {
			case "https://example.com/recent":
				resp.Title = "Recent dated post"
				resp.Content = "Published: " + recentDate + "\nThe community has been discussing storage reliability and performance improvements in detail over the last week."
			default:
				resp.Title = "Undated page"
				resp.Content = "This page talks about the product but never includes a publish date."
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":             "What are people saying in the last 30 days about ZimaOS Blue?",
		"retrieval_profile": "recent_multi_site_v1",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	payload := decodeResearchRunResultFromWebQuery(t, raw)
	if got := asString(payload["retrieval_profile"]); got != "recent_multi_site_v1" {
		t.Fatalf("retrieval_profile = %q, want recent_multi_site_v1", got)
	}
	if got := int(payload["lookback_days"].(float64)); got != 30 {
		t.Fatalf("lookback_days = %d, want 30", got)
	}
	itemsBySource, ok := payload["items_by_source"].(map[string]interface{})
	if !ok {
		t.Fatalf("items_by_source = %#v, want map", payload["items_by_source"])
	}
	webItems, ok := itemsBySource["web"].([]interface{})
	if !ok || len(webItems) != 1 {
		t.Fatalf("web items = %#v, want 1 dated item", itemsBySource["web"])
	}
	item, _ := webItems[0].(map[string]interface{})
	if got := asString(item["title"]); got != "Recent dated post" {
		t.Fatalf("item title = %q, want Recent dated post", got)
	}
}

func TestWebQueryToolRecentProfileMarksBrowserAssistedRedditFallback(t *testing.T) {
	recentDate := time.Now().UTC().Add(-72 * time.Hour).Format("2006-01-02")
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			query := asString(args["query"])
			resp := WebSearchResponse{Query: query, Provider: "duckduckgo"}
			if strings.Contains(query, "site:reddit.com") {
				resp.Results = []WebSearchResult{
					{Title: "Reddit thread", URL: "https://reddit.com/r/zimaos/comments/test/thread", Description: "Community thread"},
				}
				resp.TotalCount = 1
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:          asString(args["url"]),
				FinalURL:     asString(args["url"]),
				Format:       "text",
				Source:       webAccessSourceHTTP,
				Title:        "Reddit thread",
				WarningCodes: []string{"challenge"},
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	browserBackend := &scriptedWebQueryBrowserBackend{
		execute: func(recipe string, params map[string]string) (BrowserRecipeResult, error) {
			if recipe != "extract" {
				t.Fatalf("recipe = %q, want extract", recipe)
			}
			return BrowserRecipeResult{
				Success: true,
				Data: map[string]interface{}{
					"extracted": map[string]interface{}{
						"main_content": "Posted " + recentDate + "\nUsers say the update fixed several long-standing issues around storage reliability, app startup, disk scanning, and UI responsiveness. Multiple commenters mention that the patch feels more stable in day to day use, that file indexing completes faster, and that fewer background services stall under heavier workloads.",
						"page_heading": "Reddit thread",
					},
				},
			}, nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)
	tool.browser = browserBackend

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":             "最近30天大家怎么说 ZimaOS Blue？",
		"retrieval_profile": "recent_multi_site_v1",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	payload := decodeResearchRunResultFromWebQuery(t, raw)
	if got := payload["browser_assisted"]; got != true {
		t.Fatalf("browser_assisted = %#v, want true", got)
	}
	itemsBySource, ok := payload["items_by_source"].(map[string]interface{})
	if !ok {
		t.Fatalf("items_by_source = %#v, want map", payload["items_by_source"])
	}
	redditItems, ok := itemsBySource["reddit"].([]interface{})
	if !ok || len(redditItems) != 1 {
		t.Fatalf("reddit items = %#v, want 1 item", itemsBySource["reddit"])
	}
	item, _ := redditItems[0].(map[string]interface{})
	if got := item["browser_assisted"]; got != true {
		t.Fatalf("item browser_assisted = %#v, want true", item["browser_assisted"])
	}
}

func TestWebQueryToolRecentProfileGitHubAdapterCombinesIssuesReleasesAndReadmeSignals(t *testing.T) {
	recentIssue := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	recentRelease := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	recentRepoUpdate := time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339)
	readmeContent := base64.StdEncoding.EncodeToString([]byte("# ZimaOS Blue\n\nBlue is a self-hosted operating environment focused on reliable storage and app workflows."))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/search":
			_, _ = w.Write([]byte(`{"hits":[]}`))
		case r.URL.Path == "/public-search":
			_, _ = w.Write([]byte(`{"events":[]}`))
		case r.URL.Path == "/search/issues":
			_, _ = w.Write([]byte(`{"items":[{"title":"Storage bug report","html_url":"https://github.com/acme/blue/issues/1","body":"Users discuss storage fixes and indexing reliability.","created_at":"` + recentIssue + `","updated_at":"` + recentIssue + `","comments":12,"user":{"login":"maintainer"}}]}`))
		case r.URL.Path == "/search/repositories":
			_, _ = w.Write([]byte(`{"items":[{"full_name":"acme/blue","html_url":"https://github.com/acme/blue","description":"ZimaOS Blue repo","updated_at":"` + recentRepoUpdate + `","stargazers_count":42,"forks_count":7,"default_branch":"main","owner":{"login":"acme"}}]}`))
		case r.URL.Path == "/repos/acme/blue/releases":
			_, _ = w.Write([]byte(`[{"name":"v1.2.3","tag_name":"v1.2.3","html_url":"https://github.com/acme/blue/releases/tag/v1.2.3","body":"Release notes mention scheduler and disk scan improvements.","published_at":"` + recentRelease + `","author":{"login":"release-bot"}}]`))
		case r.URL.Path == "/repos/acme/blue/readme":
			_, _ = w.Write([]byte(`{"content":"` + readmeContent + `","encoding":"base64"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	origClient := webRecentHTTPClient
	webRecentHTTPClient = &http.Client{
		Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			rewritten := *req.URL
			rewritten.Scheme = target.Scheme
			rewritten.Host = target.Host
			cloned.URL = &rewritten
			return srv.Client().Transport.RoundTrip(cloned)
		}),
	}
	defer func() { webRecentHTTPClient = origClient }()

	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{Query: asString(args["query"]), Provider: "duckduckgo"}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      asString(args["url"]),
				FinalURL: asString(args["url"]),
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "unused",
				Content:  "unused",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":             "What are people saying in the last 30 days about ZimaOS Blue on GitHub?",
		"retrieval_profile": "recent_multi_site_v1",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	payload := decodeResearchRunResultFromWebQuery(t, raw)
	itemsBySource, ok := payload["items_by_source"].(map[string]interface{})
	if !ok {
		t.Fatalf("items_by_source = %#v, want map", payload["items_by_source"])
	}
	githubItems, ok := itemsBySource["github"].([]interface{})
	if !ok || len(githubItems) < 3 {
		t.Fatalf("github items = %#v, want issues + releases + readme signals", itemsBySource["github"])
	}
	titles := make([]string, 0, len(githubItems))
	for _, rawItem := range githubItems {
		item, _ := rawItem.(map[string]interface{})
		titles = append(titles, asString(item["title"]))
	}
	if !containsStringValue(titles, "Storage bug report") {
		t.Fatalf("github titles = %#v, want issue item", titles)
	}
	if !containsStringValue(titles, "v1.2.3") {
		t.Fatalf("github titles = %#v, want release item", titles)
	}
	if !containsStringValue(titles, "acme/blue README") {
		t.Fatalf("github titles = %#v, want readme context item", titles)
	}
}

func TestWebQueryToolRecentProfileCollectsHighCouplingSocialSourcesViaBrowserAssist(t *testing.T) {
	recentDate := time.Now().UTC().Add(-36 * time.Hour).Format("2006-01-02")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/search":
			_, _ = w.Write([]byte(`{"hits":[]}`))
		case r.URL.Path == "/search/issues":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.URL.Path == "/search/repositories":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.URL.Path == "/public-search":
			_, _ = w.Write([]byte(`{"events":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	origClient := webRecentHTTPClient
	webRecentHTTPClient = &http.Client{
		Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			rewritten := *req.URL
			rewritten.Scheme = target.Scheme
			rewritten.Host = target.Host
			cloned.URL = &rewritten
			return srv.Client().Transport.RoundTrip(cloned)
		}),
	}
	defer func() { webRecentHTTPClient = origClient }()

	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			query := asString(args["query"])
			resp := WebSearchResponse{Query: query, Provider: "duckduckgo"}
			switch {
			case strings.Contains(query, "site:x.com"):
				resp.Results = []WebSearchResult{{Title: "X thread", URL: "https://x.com/acme/status/1", Description: "x summary"}}
			case strings.Contains(query, "site:tiktok.com"):
				resp.Results = []WebSearchResult{{Title: "TikTok clip", URL: "https://www.tiktok.com/@acme/video/1", Description: "tiktok summary"}}
			case strings.Contains(query, "site:instagram.com"):
				resp.Results = []WebSearchResult{{Title: "Instagram post", URL: "https://www.instagram.com/p/ABC123/", Description: "instagram summary"}}
			case strings.Contains(query, "site:bsky.app"):
				resp.Results = []WebSearchResult{{Title: "Bluesky post", URL: "https://bsky.app/profile/acme/post/1", Description: "bluesky summary"}}
			default:
				resp.Results = nil
			}
			resp.TotalCount = len(resp.Results)
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			targetURL := asString(args["url"])
			resp := webReadResponse{
				URL:      targetURL,
				FinalURL: targetURL,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Short shell",
				Content:  "Open in app",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	browserBackend := &scriptedWebQueryBrowserBackend{
		execute: func(recipe string, params map[string]string) (BrowserRecipeResult, error) {
			if recipe != "extract" {
				t.Fatalf("recipe = %q, want extract", recipe)
			}
			targetURL := params["url"]
			label := "social"
			switch {
			case strings.Contains(targetURL, "x.com"):
				label = "X thread"
			case strings.Contains(targetURL, "tiktok.com"):
				label = "TikTok clip"
			case strings.Contains(targetURL, "instagram.com"):
				label = "Instagram post"
			case strings.Contains(targetURL, "bsky.app"):
				label = "Bluesky post"
			}
			body := "Posted " + recentDate + "\n" + label + " discussion with enough readable detail to pass the browser readability threshold and preserve the recent social signal. Multiple people mention storage reliability, update stability, indexing speed, app launch behavior, and day to day responsiveness improvements after the recent changes. The thread also includes concrete usage notes, upgrade observations, and short comparisons against earlier builds."
			return BrowserRecipeResult{
				Success: true,
				Data: map[string]interface{}{
					"extracted": map[string]interface{}{
						"main_content": body,
						"page_heading": label,
					},
				},
			}, nil
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)
	tool.browser = browserBackend

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input":             "What are people saying in the last 30 days about ZimaOS Blue on X TikTok Instagram Bluesky?",
		"retrieval_profile": "recent_multi_site_v1",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	payload := decodeResearchRunResultFromWebQuery(t, raw)
	if got := payload["browser_assisted"]; got != true {
		t.Fatalf("browser_assisted = %#v, want true", got)
	}
	itemsBySource, ok := payload["items_by_source"].(map[string]interface{})
	if !ok {
		t.Fatalf("items_by_source = %#v, want map", payload["items_by_source"])
	}
	for _, source := range []string{"x", "tiktok", "instagram", "bluesky"} {
		rawItems, ok := itemsBySource[source].([]interface{})
		if !ok || len(rawItems) != 1 {
			t.Fatalf("%s items = %#v, want 1 item", source, itemsBySource[source])
		}
		item, _ := rawItems[0].(map[string]interface{})
		if got := item["browser_assisted"]; got != true {
			t.Fatalf("%s item browser_assisted = %#v, want true", source, item["browser_assisted"])
		}
	}
}

func decodeResearchRunResultFromWebQuery(t *testing.T, raw interface{}) map[string]interface{} {
	t.Helper()
	payload, ok := raw.(string)
	if !ok {
		t.Fatalf("raw type = %T, want string", raw)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		t.Fatalf("decode recent profile payload: %v raw=%s", err, payload)
	}
	return out
}

func containsStringValue(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func TestWebQueryToolSearchCardKeepsSelectedPageWhenResultsShareHost(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "blue release notes",
				Results: []WebSearchResult{
					{Title: "Blue Overview", URL: "https://docs.example.com/overview", Description: "Overview page"},
					{Title: "Blue Release Notes v1.2.3", URL: "https://docs.example.com/releases/1.2.3", Description: "Release notes page"},
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
			case "https://docs.example.com/overview":
				resp.Title = "Blue Overview"
				resp.Content = "Short overview."
			case "https://docs.example.com/releases/1.2.3":
				resp.Title = "Blue Release Notes v1.2.3"
				resp.Content = "April 4, 2026 release notes with enough readable body text to count as the selected strong read. The article lists 12 improvements, 4 fixes, and 2 migrations, plus several paragraphs of rollout notes and compatibility guidance."
			default:
				t.Fatalf("unexpected read url=%q", url)
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		cp := make(map[string]interface{}, len(card))
		for k, v := range card {
			cp[k] = v
		}
		emitted = append(emitted, cp)
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{"input": "blue release notes"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.TargetURL != "https://docs.example.com/releases/1.2.3" {
		t.Fatalf("target_url=%q, want selected release page", envelope.TargetURL)
	}
	if len(envelope.Sources) != 2 {
		t.Fatalf("sources len=%d, want both same-host pages retained", len(envelope.Sources))
	}
	selectedCount := 0
	for _, source := range envelope.Sources {
		if source.Selected {
			selectedCount++
			if source.URL != "https://docs.example.com/releases/1.2.3" {
				t.Fatalf("selected source url=%q, want release page", source.URL)
			}
		}
	}
	if selectedCount != 1 {
		t.Fatalf("selected source count=%d, want 1", selectedCount)
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted len=%d, want 1", len(emitted))
	}
	if got := emitted[0]["selectedUrl"]; got != "https://docs.example.com/releases/1.2.3" {
		t.Fatalf("selectedUrl=%v, want selected release page", got)
	}
	if got := emitted[0]["status"]; got != "success" {
		t.Fatalf("status=%v, want success", got)
	}
}

func TestMergeWebQueryFallbackCandidates_PreservesResolvedAndDedupes(t *testing.T) {
	previous := []webQueryCandidate{
		{
			Rank: 1,
			Search: WebSearchResult{
				Title:       "Alpha",
				URL:         "https://example.com/alpha",
				Description: "alpha result",
			},
			Resolved: webQueryReadResult{
				Response: webReadResponse{
					URL:      "https://example.com/alpha",
					FinalURL: "https://example.com/alpha",
					Title:    "Alpha",
					Content:  "Alpha page content with enough readable detail to count as a strong result during fallback candidate rebuilding.",
				},
				HasSuccess: true,
				Strong:     true,
			},
		},
	}

	merged := mergeWebQueryFallbackCandidates(
		WebSearchResponse{
			Query: "alpha docs",
			Results: []WebSearchResult{
				{Title: "Alpha", URL: "https://example.com/alpha", Description: "alpha primary"},
				{Title: "Charlie", URL: "https://example.com/charlie", Description: "charlie primary"},
			},
		},
		WebSearchResponse{
			Query: "alpha docs",
			Results: []WebSearchResult{
				{Title: "Alpha duplicate", URL: "https://example.com/alpha", Description: "alpha duplicate"},
				{Title: "Beta", URL: "https://example.com/beta", Description: "beta secondary"},
			},
		},
		nil,
		previous,
		newWebQueryScoreProfile("alpha docs"),
		4,
	)

	if len(merged) != 3 {
		t.Fatalf("merged len = %d, want 3", len(merged))
	}
	if merged[0].Search.URL != "https://example.com/alpha" {
		t.Fatalf("merged[0].url = %q, want alpha", merged[0].Search.URL)
	}
	if !merged[0].Resolved.HasSuccess || !merged[0].Resolved.Strong {
		t.Fatalf("merged alpha candidate lost resolved state: %+v", merged[0].Resolved)
	}
	if merged[1].Search.URL != "https://example.com/charlie" {
		t.Fatalf("merged[1].url = %q, want charlie", merged[1].Search.URL)
	}
	if merged[2].Search.URL != "https://example.com/beta" {
		t.Fatalf("merged[2].url = %q, want beta", merged[2].Search.URL)
	}
}

func TestMergeWebQueryFallbackCandidates_AppliesMaxResultsBeforeAllowedHostFiltering(t *testing.T) {
	merged := mergeWebQueryFallbackCandidates(
		WebSearchResponse{
			Query: "alpha docs",
			Results: []WebSearchResult{
				{Title: "Offsite", URL: "https://offsite.example.org/page", Description: "offsite primary"},
				{Title: "Alpha", URL: "https://example.com/alpha", Description: "alpha primary"},
			},
		},
		WebSearchResponse{
			Query: "alpha docs",
			Results: []WebSearchResult{
				{Title: "Beta", URL: "https://example.com/beta", Description: "beta secondary"},
			},
		},
		[]string{"example.com"},
		nil,
		newWebQueryScoreProfile("alpha docs"),
		2,
	)

	if len(merged) != 1 {
		t.Fatalf("merged len = %d, want 1", len(merged))
	}
	if merged[0].Search.URL != "https://example.com/alpha" {
		t.Fatalf("merged[0].url = %q, want alpha only", merged[0].Search.URL)
	}
}

func legacyBuildWebQueryFallbackCandidatesForTest(primary, secondary WebSearchResponse, allowedHosts []string, previous []webQueryCandidate, scoreProfile webQueryScoreProfile, maxResults int) []webQueryCandidate {
	combined := append([]WebSearchResult(nil), primary.Results...)
	combined = append(combined, secondary.Results...)
	merged := buildWebQueryCandidates(combined, nil)
	if len(merged) == 0 {
		return nil
	}
	if maxResults > 0 && len(merged) > maxResults {
		merged = merged[:maxResults]
	}
	if len(allowedHosts) > 0 {
		filtered := make([]webQueryCandidate, 0, len(merged))
		for _, candidate := range merged {
			if !crawlHostAllowed(candidate.Search.URL, allowedHosts) {
				continue
			}
			filtered = append(filtered, candidate)
		}
		merged = filtered
	}
	if len(merged) == 0 {
		return nil
	}

	previousByURL := make(map[string]webQueryCandidate, len(previous))
	for _, candidate := range previous {
		targetURL := strings.TrimSpace(candidate.Search.URL)
		if targetURL == "" {
			continue
		}
		canonical, err := canonicalizeCrawlURL(targetURL)
		if err != nil {
			canonical = targetURL
		}
		previousByURL[canonical] = candidate
	}

	for idx := range merged {
		targetURL := strings.TrimSpace(merged[idx].Search.URL)
		canonical, err := canonicalizeCrawlURL(targetURL)
		if err != nil {
			canonical = targetURL
		}
		if prior, ok := previousByURL[canonical]; ok {
			merged[idx].Resolved = prior.Resolved
		}
		merged[idx].Score = scoreWebQueryCandidateWithProfile(scoreProfile, merged[idx])
	}
	return merged
}

func TestBuildWebQueryFallbackCandidates_MatchesLegacyMergePath(t *testing.T) {
	primary := WebSearchResponse{
		Query: "alpha docs",
		Results: []WebSearchResult{
			{Title: "Offsite", URL: "https://offsite.example.org/page", Description: "offsite primary"},
			{Title: "Alpha", URL: "https://example.com/alpha", Description: "alpha primary"},
			{Title: "Alpha duplicate", URL: "https://example.com/alpha", Description: "alpha duplicate"},
		},
	}
	secondary := WebSearchResponse{
		Query: "alpha docs",
		Results: []WebSearchResult{
			{Title: "Beta", URL: "https://example.com/beta", Description: "beta secondary"},
			{Title: "Gamma", URL: "https://example.com/gamma", Description: "gamma secondary"},
		},
	}
	previous := []webQueryCandidate{
		{
			Rank: 2,
			Search: WebSearchResult{
				Title: "Alpha",
				URL:   "https://example.com/alpha",
			},
			Resolved: webQueryReadResult{
				Response: webReadResponse{
					URL:      "https://example.com/alpha",
					FinalURL: "https://example.com/alpha",
					Title:    "Alpha",
					Content:  "Alpha page content with enough readable detail to preserve strong resolved state.",
				},
				HasSuccess: true,
				Strong:     true,
			},
		},
	}
	allowedHosts := []string{"example.com"}
	profile := newWebQueryScoreProfile("alpha docs")

	want := legacyBuildWebQueryFallbackCandidatesForTest(primary, secondary, allowedHosts, previous, profile, 3)
	got := buildWebQueryFallbackCandidates(primary.Results, secondary.Results, allowedHosts, previous, profile, 3)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fallback candidates = %+v, want %+v", got, want)
	}
}

func TestBuildWebQuerySources_SortsWithoutMutatingCallerOrder(t *testing.T) {
	candidates := []webQueryCandidate{
		{
			Rank:  2,
			Score: 10,
			Search: WebSearchResult{
				Title: "Beta",
				URL:   "https://example.com/beta",
			},
		},
		{
			Rank:  1,
			Score: 20,
			Search: WebSearchResult{
				Title: "Alpha",
				URL:   "https://example.com/alpha",
			},
		},
	}

	sources := buildWebQuerySources(candidates, "https://example.com/alpha")
	if len(sources) != 2 {
		t.Fatalf("sources len = %d, want 2", len(sources))
	}
	if sources[0].URL != "https://example.com/alpha" || sources[1].URL != "https://example.com/beta" {
		t.Fatalf("sources urls = [%s %s], want [alpha beta]", sources[0].URL, sources[1].URL)
	}

	if candidates[0].Search.URL != "https://example.com/beta" || candidates[1].Search.URL != "https://example.com/alpha" {
		t.Fatalf("caller candidate order mutated: [%s %s]", candidates[0].Search.URL, candidates[1].Search.URL)
	}
}

func TestBuildWebQuerySourcesFromSortedCandidates_UsesExistingOrder(t *testing.T) {
	candidates := []webQueryCandidate{
		{
			Rank:  1,
			Score: 30,
			Search: WebSearchResult{
				Title: "Alpha",
				URL:   "https://example.com/alpha",
			},
		},
		{
			Rank:  2,
			Score: 10,
			Search: WebSearchResult{
				Title: "Beta",
				URL:   "https://example.com/beta",
			},
		},
	}

	sources := buildWebQuerySourcesFromSortedCandidates(candidates, "https://example.com/beta")
	if len(sources) != 2 {
		t.Fatalf("sources len = %d, want 2", len(sources))
	}
	if sources[0].Rank != 1 || sources[0].URL != "https://example.com/alpha" {
		t.Fatalf("first source = %+v, want alpha rank 1", sources[0])
	}
	if sources[1].Rank != 2 || sources[1].URL != "https://example.com/beta" || !sources[1].Selected {
		t.Fatalf("second source = %+v, want beta rank 2 selected", sources[1])
	}
}

func TestBuildWebQuerySourcesFromSortedCandidatesWithSelectedRank_MatchesLegacy(t *testing.T) {
	candidates := []webQueryCandidate{
		{
			Rank:  1,
			Score: 30,
			Search: WebSearchResult{
				Title: "Alpha",
				URL:   "https://example.com/alpha",
			},
		},
		{
			Rank:  2,
			Score: 10,
			Search: WebSearchResult{
				Title: "Beta",
				URL:   "https://example.com/beta",
			},
		},
		{
			Rank:  3,
			Score: 8,
			Search: WebSearchResult{
				Title: "Gamma",
				URL:   "https://example.com/gamma",
			},
		},
	}

	wantSources := buildWebQuerySourcesFromSortedCandidates(candidates, "https://example.com/beta")
	wantRank := 2

	gotSources, gotRank := buildWebQuerySourcesFromSortedCandidatesWithSelectedRank(candidates, "https://example.com/beta")

	if gotRank != wantRank {
		t.Fatalf("selected rank = %d, want %d", gotRank, wantRank)
	}
	if !reflect.DeepEqual(gotSources, wantSources) {
		t.Fatalf("sources = %+v, want %+v", gotSources, wantSources)
	}
}

func TestScoreWebQueryCandidateWithProfile_MatchesLegacy(t *testing.T) {
	query := "OpenAI Responses API latest reference"
	candidate := webQueryCandidate{
		Rank: 2,
		Search: WebSearchResult{
			Title:       "OpenAI Responses API reference",
			URL:         "https://developers.openai.com/api/reference/resources/responses",
			Description: "Official latest reference docs for the Responses API.",
		},
		Resolved: webQueryReadResult{
			Response: webReadResponse{
				Title:   "Responses",
				Content: "The latest OpenAI Responses API reference explains request shape, response objects, tool calling, and output handling in the official documentation.",
			},
		},
	}

	legacy := scoreWebQueryCandidate(query, candidate)
	profile := newWebQueryScoreProfile(query)
	optimized := scoreWebQueryCandidateWithProfile(profile, candidate)

	if legacy != optimized {
		t.Fatalf("legacy score = %v, optimized score = %v", legacy, optimized)
	}
}

func TestWebQueryTextOverlapWithProfile_MatchesLegacy(t *testing.T) {
	query := "openai responses api api latest"
	text := "Latest OpenAI Responses API reference and examples"

	legacy := webQueryTextOverlap(query, text)
	profile := newWebQueryScoreProfile(query)
	optimized := webQueryTextOverlapWithProfile(profile, text)

	if legacy != optimized {
		t.Fatalf("legacy overlap = %v, optimized overlap = %v", legacy, optimized)
	}
}

func TestNewWebQueryScoreProfile_BuildsDedupedTokenSet(t *testing.T) {
	profile := newWebQueryScoreProfile("openai responses api api latest")

	if len(profile.QueryTokens) != 4 {
		t.Fatalf("query token len = %d, want 4", len(profile.QueryTokens))
	}
	if len(profile.QueryTokenSet) != 4 {
		t.Fatalf("query token set len = %d, want 4", len(profile.QueryTokenSet))
	}
	for _, token := range []string{"openai", "responses", "api", "latest"} {
		if _, ok := profile.QueryTokenSet[token]; !ok {
			t.Fatalf("query token set missing %q", token)
		}
	}
}

func TestWebQueryToolEmitsSearchCardForSnippetOnlyPartialResults(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "zimaos blue",
				Results: []WebSearchResult{
					{Title: "Best Match", URL: "https://example.com/best", Description: "Snippet only result"},
				},
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
			return nil, errors.New("read failed")
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		cp := make(map[string]interface{}, len(card))
		for k, v := range card {
			cp[k] = v
		}
		emitted = append(emitted, cp)
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{"input": "zimaos blue"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !envelope.SearchCardEmitted {
		t.Fatal("expected search_card_emitted=true")
	}
	if envelope.Status != webQueryStatusPartial {
		t.Fatalf("status=%q, want %q", envelope.Status, webQueryStatusPartial)
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted len=%d, want 1", len(emitted))
	}
	if got := emitted[0]["type"]; got != "search" {
		t.Fatalf("type=%v, want search", got)
	}
	if got := emitted[0]["status"]; got != "partial" {
		t.Fatalf("status=%v, want partial", got)
	}
	if got := emitted[0]["selectedUrl"]; got != "https://example.com/best" {
		t.Fatalf("selectedUrl=%v, want best result", got)
	}
}

func TestWebQueryToolEmitsSearchCardForNoResults(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query:      "zimaos blue",
				Results:    []WebSearchResult{},
				TotalCount: 0,
				Provider:   "duckduckgo",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, nil, nil, nil)

	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		cp := make(map[string]interface{}, len(card))
		for k, v := range card {
			cp[k] = v
		}
		emitted = append(emitted, cp)
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{"input": "zimaos blue"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !envelope.SearchCardEmitted {
		t.Fatal("expected search_card_emitted=true")
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted len=%d, want 1", len(emitted))
	}
	if got := emitted[0]["status"]; got != "empty" {
		t.Fatalf("status=%v, want empty", got)
	}
	if message := strings.TrimSpace(asString(emitted[0]["message"])); message == "" {
		t.Fatal("expected explicit empty-state message")
	}
}

func TestWebQueryToolEmitsSearchCardForSearchFailure(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			return nil, errors.New("search backend unavailable")
		},
	}
	tool := NewWebQueryTool(searchTool, nil, nil, nil, nil)

	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		cp := make(map[string]interface{}, len(card))
		for k, v := range card {
			cp[k] = v
		}
		emitted = append(emitted, cp)
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{"input": "zimaos blue"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !envelope.SearchCardEmitted {
		t.Fatal("expected search_card_emitted=true")
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted len=%d, want 1", len(emitted))
	}
	if got := emitted[0]["status"]; got != "empty" {
		t.Fatalf("status=%v, want empty", got)
	}
	if message := strings.ToLower(strings.TrimSpace(asString(emitted[0]["message"]))); !strings.Contains(message, "search") {
		t.Fatalf("message=%q, want search failure hint", message)
	}
}

func TestWebQueryToolReturnsNeedsBrowserWhenBrowserFallbackTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	readTool := NewWebReadTool(WebFetchConfig{
		Timeout:           75 * time.Millisecond,
		AllowPrivateHosts: true,
	})
	readTool.SetBrowser(&blockingWebQueryBrowserBackend{})
	tool := NewWebQueryTool(nil, nil, readTool, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	start := time.Now()
	raw, err := tool.Execute(ctx, map[string]interface{}{
		"input":  srv.URL,
		"format": "text",
		"lane":   webAccessLaneHTTP,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("elapsed = %s, want browser fallback to time out quickly", elapsed)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Status != webQueryStatusNeedsBrowser {
		t.Fatalf("status = %q, want %q", envelope.Status, webQueryStatusNeedsBrowser)
	}
	if envelope.NextAction != webQueryNextActionRetryBrowser {
		t.Fatalf("next_action = %q, want %q", envelope.NextAction, webQueryNextActionRetryBrowser)
	}
	if len(envelope.Diagnostics.Attempts) < 2 {
		t.Fatalf("attempts len = %d, want multiple read attempts", len(envelope.Diagnostics.Attempts))
	}
	var browserAttempt *webQueryAttempt
	for idx := range envelope.Diagnostics.Attempts {
		attempt := envelope.Diagnostics.Attempts[idx]
		if attempt.Mode == webAccessLaneBrowser {
			browserAttempt = &attempt
			break
		}
	}
	if browserAttempt == nil {
		t.Fatalf("attempts = %+v, want a browser lane attempt", envelope.Diagnostics.Attempts)
	}
	if strings.TrimSpace(browserAttempt.Error) == "" {
		t.Fatalf("browser attempt = %+v, want browser timeout error", browserAttempt)
	}
}

func TestWebQueryToolRefinesBrowserRedirectToReadableFinalURL(t *testing.T) {
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			lane := args["lane"].(string)
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
			}
			switch {
			case url == "https://platform.openai.com/docs/api-reference/responses" && lane == webAccessLaneHTTP:
				resp.Title = "Blocked"
				resp.Content = "Please enable cookies."
				resp.WarningCodes = []string{webFetchWarningCodeBrowserRequired}
				resp.Warnings = []string{"page requires a browser session"}
				resp.InteractiveRequired = true
			case url == "https://platform.openai.com/docs/api-reference/responses" && lane == webAccessLaneProxyFetcher:
				resp.Title = "Blocked"
				resp.Content = "Proxy preview still requires a browser."
				resp.Source = webAccessSourceProxyFetcher
				resp.WarningCodes = []string{webFetchWarningCodeBrowserRequired}
				resp.Warnings = []string{"proxy fetch still requires browser interaction"}
				resp.InteractiveRequired = true
			case url == "https://platform.openai.com/docs/api-reference/responses" && lane == webAccessLaneBrowser:
				resp.Title = "Responses | OpenAI API Reference"
				resp.FinalURL = "https://developers.openai.com/api/reference/resources/responses"
				resp.Source = webAccessSourceBrowser
				resp.Content = "[RootWebArea] \"Responses | OpenAI API Reference\"\n  @1 [link] \"Overview\"\n  @2 [link] \"Create a model response\""
			case url == "https://developers.openai.com/api/reference/resources/responses" && lane == webAccessLaneHTTP:
				resp.Title = "Responses | OpenAI API Reference"
				resp.FinalURL = "https://developers.openai.com/api/reference/resources/responses"
				resp.Content = "Use the Responses API to create, retrieve, cancel, and delete model responses. This page documents the endpoint shape, authentication requirements, request fields, streaming events, and related examples in readable prose."
			default:
				t.Fatalf("unexpected read call url=%q lane=%q", url, lane)
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(nil, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "https://platform.openai.com/docs/api-reference/responses",
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
	if envelope.FinalURL != "https://developers.openai.com/api/reference/resources/responses" {
		t.Fatalf("final_url = %q, want developers docs URL", envelope.FinalURL)
	}
	if strings.HasPrefix(envelope.Content, "[RootWebArea]") {
		t.Fatalf("content = %q, want refined readable content instead of browser scaffold", envelope.Content)
	}
	if !strings.Contains(envelope.Content, "create, retrieve, cancel, and delete model responses") {
		t.Fatalf("content = %q, want refined readable page body", envelope.Content)
	}
	foundRefineAttempt := false
	for _, attempt := range envelope.Diagnostics.Attempts {
		if attempt.Stage == "read_refine" && attempt.URL == "https://developers.openai.com/api/reference/resources/responses" {
			foundRefineAttempt = true
			break
		}
	}
	if !foundRefineAttempt {
		t.Fatalf("attempts = %+v, want read_refine attempt for final_url", envelope.Diagnostics.Attempts)
	}
	for _, attempt := range envelope.Diagnostics.Attempts {
		if attempt.Mode == webAccessLaneBrowser {
			t.Fatalf("attempts = %+v, want readable refinement to avoid browser lane", envelope.Diagnostics.Attempts)
		}
	}
}

func TestWebQueryToolHonorsSiteHintsWhenSelectingCandidates(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "OpenAI Responses API documentation site:platform.openai.com OR site:openai.com",
				Results: []WebSearchResult{
					{Title: "OpenAI_百度百科", URL: "https://baike.baidu.com/item/OpenAI/19758408", Description: "百科结果"},
					{Title: "Responses | OpenAI API Reference", URL: "https://platform.openai.com/docs/api-reference/responses", Description: "Official responses docs"},
					{Title: "OpenAI Docs Mirror", URL: "https://developers.openai.com/api/reference/resources/responses", Description: "Developers docs"},
				},
				TotalCount: 3,
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
				Source:   webAccessSourceHTTP,
				Title:    "Responses | OpenAI API Reference",
				Content:  "Official OpenAI Responses API reference content with endpoint details, parameters, and enough body text to count as a strong read.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "OpenAI Responses API documentation site:platform.openai.com OR site:openai.com",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.TargetURL != "https://platform.openai.com/docs/api-reference/responses" {
		t.Fatalf("target_url = %q, want official OpenAI docs", envelope.TargetURL)
	}
	if len(envelope.Sources) != 2 {
		t.Fatalf("sources len = %d, want 2 site-filtered candidates", len(envelope.Sources))
	}
	for _, source := range envelope.Sources {
		if strings.Contains(source.URL, "baike.baidu.com") {
			t.Fatalf("sources = %+v, want site-hinted filtering to exclude Baidu", envelope.Sources)
		}
	}
}

func TestWebQueryToolLocalizedLatestDocsShortcutUsesCanonicalURL(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			t.Fatalf("search should not run for localized latest-docs shortcut, args=%v", args)
			return nil, nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			if url != "https://developers.openai.com/api/reference/resources/responses" {
				t.Fatalf("read url = %q, want canonical Responses docs URL", url)
			}
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Responses | OpenAI API Reference",
				Content:  "Canonical Responses API documentation with endpoint details, request and response schemas, authentication requirements, supported parameters, streaming events, tool-calling guidance, pagination notes, and enough readable prose to exceed the strong-read threshold for the unified web query pipeline during execution gate verification.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "Suche die neueste Dokumentation zur OpenAI Responses API.",
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
	if envelope.Query != "Suche die neueste Dokumentation zur OpenAI Responses API." {
		t.Fatalf("query = %q, want original localized query", envelope.Query)
	}
	if envelope.FinalURL != "https://developers.openai.com/api/reference/resources/responses" {
		t.Fatalf("final_url = %q, want canonical Responses docs URL", envelope.FinalURL)
	}
	if envelope.Diagnostics.Route != "canonical_url" {
		t.Fatalf("route = %q, want canonical_url", envelope.Diagnostics.Route)
	}
	if len(searchTool.lastArgs) != 0 {
		t.Fatalf("search calls = %d, want 0", len(searchTool.lastArgs))
	}
}

func TestWebQueryToolSingleOpenAISiteHintUsesCanonicalURL(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			t.Fatalf("search should not run for single-site OpenAI docs shortcut, args=%v", args)
			return nil, nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			if url != "https://developers.openai.com/api/reference/resources/responses" {
				t.Fatalf("read url = %q, want canonical Responses docs URL", url)
			}
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Responses | OpenAI API Reference",
				Content:  "Canonical Responses API documentation reached through the single-site OpenAI shortcut with enough endpoint detail, authentication notes, request and response schema guidance, streaming event descriptions, and tool-calling examples to count as a strong successful read for the unified web query execution path.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "OpenAI Responses API official documentation site:platform.openai.com",
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
	if envelope.FinalURL != "https://developers.openai.com/api/reference/resources/responses" {
		t.Fatalf("final_url = %q, want canonical Responses docs URL", envelope.FinalURL)
	}
	if envelope.Diagnostics.Route != "canonical_url" {
		t.Fatalf("route = %q, want canonical_url", envelope.Diagnostics.Route)
	}
	if len(searchTool.lastArgs) != 0 {
		t.Fatalf("search calls = %d, want 0", len(searchTool.lastArgs))
	}
}

func TestWebQueryToolOfficialResponseFormatDocsShortcutUsesCanonicalURL(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			t.Fatalf("search should not run for official response-format docs shortcut, args=%v", args)
			return nil, nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			if url != "https://developers.openai.com/api/reference/resources/responses" {
				t.Fatalf("read url = %q, want canonical Responses docs URL", url)
			}
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Responses | OpenAI API Reference",
				Content:  "Canonical Responses API documentation reached through the official response-format shortcut with enough endpoint detail, response schema notes, streaming guidance, and readable prose to count as a strong successful read.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "OpenAI API official documentation response format",
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
	if envelope.FinalURL != "https://developers.openai.com/api/reference/resources/responses" {
		t.Fatalf("final_url = %q, want canonical Responses docs URL", envelope.FinalURL)
	}
	if envelope.Diagnostics.Route != "canonical_url" {
		t.Fatalf("route = %q, want canonical_url", envelope.Diagnostics.Route)
	}
	if len(searchTool.lastArgs) != 0 {
		t.Fatalf("search calls = %d, want 0", len(searchTool.lastArgs))
	}
}

func TestWebQueryToolOfficialChatCompletionsStructureDocsShortcutUsesCanonicalURL(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			t.Fatalf("search should not run for official chat completions structure docs shortcut, args=%v", args)
			return nil, nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			url := args["url"].(string)
			if url != "https://developers.openai.com/api/reference/chat-completions/overview" {
				t.Fatalf("read url = %q, want canonical Chat Completions overview URL", url)
			}
			resp := webReadResponse{
				URL:      url,
				FinalURL: url,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Chat Completions Overview | OpenAI API Reference",
				Content:  "Canonical Chat Completions overview documentation reached through the official response-structure shortcut with enough schema detail, response object guidance, and readable prose to count as a strong successful read.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "official OpenAI API documentation Chat Completions response object structure",
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
	if envelope.FinalURL != "https://developers.openai.com/api/reference/chat-completions/overview" {
		t.Fatalf("final_url = %q, want canonical Chat Completions overview URL", envelope.FinalURL)
	}
	if envelope.Diagnostics.Route != "canonical_url" {
		t.Fatalf("route = %q, want canonical_url", envelope.Diagnostics.Route)
	}
	if len(searchTool.lastArgs) != 0 {
		t.Fatalf("search calls = %d, want 0", len(searchTool.lastArgs))
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

func TestWebQueryToolHidesCanceledSearchAttemptsAfterProviderFanoutSuccess(t *testing.T) {
	searchTool := &contextAwareWebSearchFanoutTool{
		def:       ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		providers: []string{"alpha", "beta"},
		exec: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			provider := strings.TrimSpace(asString(args["provider"]))
			switch provider {
			case "alpha":
				resp := WebSearchResponse{
					Query: "fanout cancel hide",
					Results: []WebSearchResult{
						{Title: "Winner", URL: "https://example.com/winner", Description: "Winning provider result"},
					},
					TotalCount: 1,
					Provider:   "alpha",
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			case "beta":
				<-ctx.Done()
				return nil, ctx.Err()
			default:
				t.Fatalf("unexpected provider %q", provider)
				return nil, nil
			}
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
				Title:    "Winner",
				Content:  "Winning provider content with enough readable detail to complete the search-read flow without falling back anywhere else. It includes a full paragraph of explanatory text, concrete implementation notes, a short summary of the result quality, several extra sentences about relevance and freshness, and enough additional body copy to comfortably exceed the readable-content threshold used by the selection heuristics.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "fanout cancel hide",
		"depth": "standard",
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
	for _, attempt := range envelope.Diagnostics.Attempts {
		if attempt.Stage == "search" && strings.EqualFold(strings.TrimSpace(attempt.Mode), "beta") {
			t.Fatalf("attempts = %+v, want canceled beta fanout attempt hidden", envelope.Diagnostics.Attempts)
		}
	}
}

func TestHideCanceledSearchAttemptsAfterSearchSuccess(t *testing.T) {
	attempts := []webQueryAttempt{
		{Stage: "search", Mode: "alpha", Status: "ok"},
		{Stage: "search", Mode: "beta", Status: "canceled", Error: context.Canceled.Error()},
		{Stage: "read", Mode: "http", Status: "ok"},
	}

	filtered := hideCanceledSearchAttemptsAfterSearchSuccess(attempts)

	if len(filtered) != 2 {
		t.Fatalf("filtered attempts len = %d, want 2; attempts=%+v", len(filtered), filtered)
	}
	for _, attempt := range filtered {
		if attempt.Stage == "search" && attempt.Status == "canceled" {
			t.Fatalf("filtered attempts = %+v, want canceled search attempts hidden", filtered)
		}
	}
}

func TestWebQueryToolSearchQueryReadsNormalizedBingFinanceURL(t *testing.T) {
	searchServer := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`
<html><body><ol id="b_results">
  <li class="b_algo">
    <h2><a href="https://www.bing.com/ck/a?!&&p=demo&u=a1aHR0cHM6Ly9zdG9ja2FuYWx5c2lzLmNvbS9zdG9ja3MvYWFwbC8&ntb=1">Apple Stock Price</a></h2>
    <div class="b_caption"><p>Apple Inc. price, chart, and summary.</p></div>
  </li>
</ol></body></html>`))
	}))
	defer searchServer.Close()

	searchTool := NewWebSearchTool(WebSearchConfig{Provider: "bing", Region: "us-en"})
	searchTool.httpClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: rewriteHostTransport{
			host:   strings.TrimPrefix(searchServer.URL, "http://"),
			scheme: "http",
		},
	}

	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			readURL := args["url"].(string)
			resp := webReadResponse{
				URL:      readURL,
				FinalURL: readURL,
				Format:   "text",
				Source:   webAccessSourceHTTP,
			}
			if strings.Contains(readURL, "bing.com/ck/a") {
				resp.Title = "Bing Redirect"
				resp.Content = "Please click here if the page does not redirect automatically ..."
			} else {
				resp.Title = "Apple (AAPL) Stock Price"
				resp.Content = "Apple stock trades with live price updates, intraday movement, valuation multiples, and recent headline context. This body is intentionally long enough to count as a strong readable finance result so the search pipeline should stop after the normalized candidate instead of rescuing the query through browser search."
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "Find a public finance overview page for Apple AAPL and summarize it.",
		"depth": "standard",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.TargetURL != "https://stockanalysis.com/stocks/aapl/" {
		t.Fatalf("target_url = %q, want normalized stockanalysis URL", envelope.TargetURL)
	}
	for _, call := range readTool.lastArgs {
		if strings.Contains(asString(call["url"]), "bing.com/ck/a") {
			t.Fatalf("read calls = %+v, want normalized finance URLs only", readTool.lastArgs)
		}
	}
}

func TestWebQueryToolStrongNormalizedBingResultSkipsBrowserSearchFallback(t *testing.T) {
	searchServer := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`
<html><body><ol id="b_results">
  <li class="b_algo">
    <h2><a href="https://www.bing.com/ck/a?!&&p=demo&u=a1aHR0cHM6Ly93d3cuZ29vZ2xlLmNvbS9maW5hbmNlL3F1b3RlL0FBUEw6TkFTREFR&ntb=1">Google Finance Apple</a></h2>
    <div class="b_caption"><p>Live NASDAQ quote.</p></div>
  </li>
</ol></body></html>`))
	}))
	defer searchServer.Close()

	searchTool := NewWebSearchTool(WebSearchConfig{Provider: "bing", Region: "us-en"})
	searchTool.httpClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: rewriteHostTransport{
			host:   strings.TrimPrefix(searchServer.URL, "http://"),
			scheme: "http",
		},
	}

	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			readURL := args["url"].(string)
			resp := webReadResponse{
				URL:      readURL,
				FinalURL: readURL,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "AAPL:NASDAQ",
				Content:  "Google Finance shows Apple share price, intraday range, market cap, and recent performance with enough readable detail that the normalized result should be accepted immediately without any browser search rescue path.",
			}
			if strings.Contains(readURL, "bing.com/ck/a") {
				resp.Title = "Redirect placeholder"
				resp.Content = "Please click here if the page does not redirect automatically ..."
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	browser := &scriptedWebQueryBrowserBackend{}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "Find a public finance overview page for Apple AAPL.",
		"depth": "standard",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Diagnostics.Route != "search_http" {
		t.Fatalf("route = %q, want search_http", envelope.Diagnostics.Route)
	}
	for _, attempt := range envelope.Diagnostics.Attempts {
		if attempt.Stage == "search_browser" {
			t.Fatalf("attempts = %+v, want no search_browser fallback", envelope.Diagnostics.Attempts)
		}
	}
	if browser.recipeCalls != 0 {
		t.Fatalf("browser recipe calls = %d, want 0", browser.recipeCalls)
	}
}

func TestWebToolRunBrowserSearchDiscoveryUsesExplicitExecutionPlan(t *testing.T) {
	browser := &scriptedWebQueryBrowserBackend{
		execute: func(recipe string, params map[string]string) (BrowserRecipeResult, error) {
			if recipe != "search" {
				t.Fatalf("recipe = %q, want search", recipe)
			}
			payload := map[string]interface{}{
				"engine": "bing",
				"results": []map[string]string{{
					"title":   "Blue Docs",
					"url":     "https://example.com/docs",
					"snippet": "Planner-driven browser search result.",
				}},
			}
			return BrowserRecipeResult{Success: true, Data: payload}, nil
		},
	}

	tool := &WebTool{browser: browser}
	resp, attempt, err := tool.runBrowserSearchDiscovery(context.Background(), "latest blue docs", 3, WebSearchBrowserFallbackConfig{Engine: "bing"})
	if err != nil {
		t.Fatalf("runBrowserSearchDiscovery() error = %v", err)
	}
	if attempt.Stage != "search_browser" {
		t.Fatalf("attempt stage = %q, want search_browser", attempt.Stage)
	}
	if !browser.recipePlanOK {
		t.Fatal("expected ExecuteRecipe context to include explicit web execution plan")
	}
	if browser.recipePlan.Kind != WebTaskKindSearch {
		t.Fatalf("plan kind = %q, want %q", browser.recipePlan.Kind, WebTaskKindSearch)
	}
	if browser.recipePlan.PrimaryRuntime != webAccessLaneBrowser {
		t.Fatalf("plan primary runtime = %q, want %q", browser.recipePlan.PrimaryRuntime, webAccessLaneBrowser)
	}
	if len(browser.recipePlan.Steps) != 1 {
		t.Fatalf("plan steps len = %d, want 1", len(browser.recipePlan.Steps))
	}
	if browser.recipePlan.Steps[0].Runtime != webAccessLaneBrowser {
		t.Fatalf("plan step runtime = %q, want %q", browser.recipePlan.Steps[0].Runtime, webAccessLaneBrowser)
	}
	if got := browser.recipePlan.TraceLabels["engine"]; got != "bing" {
		t.Fatalf("plan trace engine = %q, want bing", got)
	}
	if resp.Provider != "browser:bing" {
		t.Fatalf("provider = %q, want browser:bing", resp.Provider)
	}
}

func TestWebQueryToolStockQuoteFastPathBypassesSearchFanout(t *testing.T) {
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			t.Fatalf("search fanout should not run for the stock quote fast path: %+v", args)
			return nil, nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			readURL := args["url"].(string)
			if readURL != "https://stockanalysis.com/stocks/aapl/" {
				t.Fatalf("url = %q, want canonical stockanalysis URL", readURL)
			}
			resp := webReadResponse{
				URL:      readURL,
				FinalURL: readURL,
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Apple (AAPL) Stock Price",
				Content:  "Apple stock price, intraday range, valuation, and market activity with enough readable detail that the stock quote fast path should satisfy the query without calling the general search fanout path first.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "Research the current stock price of Apple (AAPL) and return the price, date, and a brief market summary.",
		"depth": "standard",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.TargetURL != "https://stockanalysis.com/stocks/aapl/" {
		t.Fatalf("target_url = %q, want canonical stockanalysis URL", envelope.TargetURL)
	}
	if len(searchTool.lastArgs) != 0 {
		t.Fatalf("search calls = %d, want 0", len(searchTool.lastArgs))
	}
}

func TestWebQueryToolReusesBrowserTargetWithinSameHostCandidates(t *testing.T) {
	const (
		firstURL  = "https://stockanalysis.com/stocks/aapl/"
		secondURL = "https://stockanalysis.com/stocks/aapl/financials/"
		sharedTab = "tab-stockanalysis"
	)

	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "AAPL finance overview",
				Results: []WebSearchResult{
					{Title: "Apple Stock Price", URL: firstURL, Description: "Quote page"},
					{Title: "Apple Financials", URL: secondURL, Description: "Financial statements"},
				},
				TotalCount: 2,
				Provider:   "bing",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}

	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			readURL := args["url"].(string)
			lane := args["lane"].(string)
			targetID := asString(args["browser_target_id"])
			switch {
			case readURL == firstURL && lane == webAccessLaneHTTP:
				resp := webReadResponse{
					URL:                 readURL,
					FinalURL:            readURL,
					Format:              "text",
					Source:              webAccessSourceHTTP,
					Title:               "Apple Stock Price",
					Content:             "Please open in a browser.",
					Warnings:            []string{"page requires browser interaction"},
					WarningCodes:        []string{webFetchWarningCodeBrowserRequired},
					InteractiveRequired: true,
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			case readURL == firstURL && lane == webAccessLaneProxyFetcher:
				resp := webReadResponse{
					URL:                 readURL,
					FinalURL:            readURL,
					Format:              "text",
					Source:              webAccessSourceProxyFetcher,
					Title:               "Apple Stock Price",
					Content:             "Proxy preview still needs the real browser.",
					Warnings:            []string{"page requires browser interaction"},
					WarningCodes:        []string{webFetchWarningCodeBrowserRequired},
					InteractiveRequired: true,
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			case readURL == firstURL && lane == webAccessLaneBrowser:
				payload := map[string]interface{}{
					"url":               readURL,
					"final_url":         readURL,
					"format":            "text",
					"source":            webAccessSourceBrowser,
					"title":             "Apple Stock Price",
					"content":           "Browser-rendered Apple quote page with enough detail to count as a strong result and seed a reusable browser target for this host.",
					"browser_target_id": sharedTab,
				}
				b, _ := json.Marshal(payload)
				return string(b), nil
			case readURL == secondURL && lane == webAccessLaneHTTP:
				resp := webReadResponse{
					URL:      readURL,
					FinalURL: readURL,
					Format:   "text",
					Source:   webAccessSourceHTTP,
					Title:    "Apple Financials",
				}
				if targetID == sharedTab {
					resp.Content = "Session-reused financial content with enough detail to prove the same-host browser target was forwarded into the follow-up read."
				} else {
					resp.Content = "Please open in a browser."
					resp.Warnings = []string{"page requires browser interaction"}
					resp.WarningCodes = []string{webFetchWarningCodeBrowserRequired}
					resp.InteractiveRequired = true
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			case readURL == secondURL && lane == webAccessLaneProxyFetcher:
				resp := webReadResponse{
					URL:                 readURL,
					FinalURL:            readURL,
					Format:              "text",
					Source:              webAccessSourceProxyFetcher,
					Title:               "Apple Financials",
					Content:             "Proxy preview still needs the real browser.",
					Warnings:            []string{"page requires browser interaction"},
					WarningCodes:        []string{webFetchWarningCodeBrowserRequired},
					InteractiveRequired: true,
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			case readURL == secondURL && lane == webAccessLaneBrowser:
				resp := webReadResponse{
					URL:      readURL,
					FinalURL: readURL,
					Format:   "text",
					Source:   webAccessSourceBrowser,
					Title:    "Apple Financials",
					Content:  "Browser fallback still worked.",
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			default:
				t.Fatalf("unexpected read call url=%q lane=%q target=%q", readURL, lane, targetID)
				return nil, nil
			}
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "AAPL finance overview",
		"depth": "standard",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	foundReuse := false
	for _, call := range readTool.lastArgs {
		if asString(call["url"]) == secondURL && asString(call["browser_target_id"]) == sharedTab {
			foundReuse = true
			break
		}
	}
	if !foundReuse {
		t.Fatalf("read calls = %+v, want second same-host candidate to reuse %q", readTool.lastArgs, sharedTab)
	}
	if envelope.Status != webQueryStatusOK {
		t.Fatalf("status = %q, want %q", envelope.Status, webQueryStatusOK)
	}
}

func TestWebQueryToolDoesNotReuseBrowserTargetAcrossHosts(t *testing.T) {
	const (
		firstURL  = "https://stockanalysis.com/stocks/aapl/"
		secondURL = "https://finance.yahoo.com/quote/AAPL/"
		sharedTab = "tab-stockanalysis"
	)

	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "AAPL finance overview",
				Results: []WebSearchResult{
					{Title: "Apple Stock Price", URL: firstURL, Description: "Quote page"},
					{Title: "Yahoo Finance Apple", URL: secondURL, Description: "Yahoo quote"},
				},
				TotalCount: 2,
				Provider:   "bing",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}

	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			readURL := args["url"].(string)
			lane := args["lane"].(string)
			switch {
			case readURL == firstURL && lane == webAccessLaneHTTP:
				resp := webReadResponse{
					URL:                 readURL,
					FinalURL:            readURL,
					Format:              "text",
					Source:              webAccessSourceHTTP,
					Title:               "Apple Stock Price",
					Content:             "Please open in a browser.",
					Warnings:            []string{"page requires browser interaction"},
					WarningCodes:        []string{webFetchWarningCodeBrowserRequired},
					InteractiveRequired: true,
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			case readURL == firstURL && lane == webAccessLaneProxyFetcher:
				resp := webReadResponse{
					URL:                 readURL,
					FinalURL:            readURL,
					Format:              "text",
					Source:              webAccessSourceProxyFetcher,
					Title:               "Apple Stock Price",
					Content:             "Proxy preview still needs the real browser.",
					Warnings:            []string{"page requires browser interaction"},
					WarningCodes:        []string{webFetchWarningCodeBrowserRequired},
					InteractiveRequired: true,
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			case readURL == firstURL && lane == webAccessLaneBrowser:
				payload := map[string]interface{}{
					"url":               readURL,
					"final_url":         readURL,
					"format":            "text",
					"source":            webAccessSourceBrowser,
					"title":             "Apple Stock Price",
					"content":           "Browser-rendered Apple quote page with enough detail to count as a strong result and seed a reusable browser target for this host.",
					"browser_target_id": sharedTab,
				}
				b, _ := json.Marshal(payload)
				return string(b), nil
			default:
				resp := webReadResponse{
					URL:      readURL,
					FinalURL: readURL,
					Format:   "text",
					Source:   webAccessSourceHTTP,
					Title:    "Yahoo Finance Apple",
					Content:  "Independent cross-host result content that should never receive the stockanalysis browser target.",
				}
				b, _ := json.Marshal(resp)
				return string(b), nil
			}
		},
	}

	tool := NewWebQueryTool(searchTool, nil, readTool, nil, nil)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "AAPL finance overview",
		"depth": "standard",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	for _, call := range readTool.lastArgs {
		if asString(call["url"]) == secondURL && strings.TrimSpace(asString(call["browser_target_id"])) != "" {
			t.Fatalf("read calls = %+v, want no cross-host browser target reuse", readTool.lastArgs)
		}
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
