package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
)

func TestWebFetchToolExecute_HTMLExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Example Page</title></head>
  <body>
    <main>
      <h1>Hello World</h1>
      <p>This is a test page.</p>
    </main>
  </body>
</html>`))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	got := parseWebFetchResult(t, result)
	if got["extractor"] != "html" {
		t.Fatalf("extractor = %v, want %q", got["extractor"], "html")
	}
	if got["title"] != "Example Page" {
		t.Fatalf("title = %v, want %q", got["title"], "Example Page")
	}
	content, _ := got["content"].(string)
	if !strings.Contains(content, "Hello World") {
		t.Fatalf("content missing heading: %q", content)
	}
	if !strings.Contains(content, "This is a test page.") {
		t.Fatalf("content missing paragraph: %q", content)
	}
}

func TestWebFetchToolExecute_MarkdownExtractionModes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte("# Heading\n\nBody with **bold** text."))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})

	markdownRes, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "markdown",
	})
	if err != nil {
		t.Fatalf("markdown mode execute failed: %v", err)
	}
	markdownData := parseWebFetchResult(t, markdownRes)
	markdownContent, _ := markdownData["content"].(string)
	if !strings.Contains(markdownContent, "# Heading") {
		t.Fatalf("markdown content = %q, want heading marker", markdownContent)
	}

	textRes, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("text mode execute failed: %v", err)
	}
	textData := parseWebFetchResult(t, textRes)
	textContent, _ := textData["content"].(string)
	if strings.Contains(textContent, "# Heading") {
		t.Fatalf("text content still contains markdown heading marker: %q", textContent)
	}
	if !strings.Contains(textContent, "Heading") {
		t.Fatalf("text content missing heading text: %q", textContent)
	}
}

func TestWebFetchToolExecute_PDFExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4\n%stub pdf bytes"))
	}))
	defer srv.Close()

	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "Quarterly revenue grew 20%.", Document: pdfextract.DocumentInfo{FileName: "report.pdf"}}}
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	tool.SetPDFService(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL + "/report.pdf",
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if len(svc.extractReqs) != 1 {
		t.Fatalf("extract calls = %d, want 1", len(svc.extractReqs))
	}
	data := parseWebFetchResult(t, result)
	if data["extractor"] != "pdf" {
		t.Fatalf("extractor = %v, want %q", data["extractor"], "pdf")
	}
	if data["title"] != "report.pdf" {
		t.Fatalf("title = %v, want %q", data["title"], "report.pdf")
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "Quarterly revenue grew 20%.") {
		t.Fatalf("content = %q, want extracted pdf text", content)
	}
	if data["content_type"] != "application/pdf" {
		t.Fatalf("content_type = %v, want %q", data["content_type"], "application/pdf")
	}
}

func TestWebFetchToolExecute_MaxCharsTruncation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(strings.Repeat("x", 200)))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":       srv.URL,
		"max_chars": 120,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	got := parseWebFetchResult(t, result)
	content, _ := got["content"].(string)
	if len([]rune(content)) != 120 {
		t.Fatalf("content length = %d, want %d", len([]rune(content)), 120)
	}
	if truncated, _ := got["truncated"].(bool); !truncated {
		t.Fatalf("truncated = %v, want true", got["truncated"])
	}
}

func TestWebFetchToolExecute_RejectsNonHTTPURL(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"url": "file:///etc/passwd",
	})
	if err == nil {
		t.Fatal("expected error for non-http URL")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "http") {
		t.Fatalf("error = %v, want http/https scheme message", err)
	}
}

func TestGuardResolvedWebFetchAddr_AllowsFakeIPForPublicHostname(t *testing.T) {
	err := guardResolvedWebFetchAddr("www.thepaper.cn", netip.MustParseAddr("198.18.0.163"))
	if err != nil {
		t.Fatalf("guardResolvedWebFetchAddr returned error: %v", err)
	}
}

func TestGuardResolvedWebFetchAddr_BlocksFakeIPForNonPublicHostname(t *testing.T) {
	err := guardResolvedWebFetchAddr("printer.local", netip.MustParseAddr("198.18.0.163"))
	if err == nil {
		t.Fatal("expected fake-ip benchmark address to be blocked for non-public hostname")
	}
}

func TestGuardResolvedWebFetchAddr_BlocksPrivateAddrForPublicHostname(t *testing.T) {
	err := guardResolvedWebFetchAddr("news.example.com", netip.MustParseAddr("10.0.0.8"))
	if err == nil {
		t.Fatal("expected RFC1918 address to be blocked even for public hostname")
	}
}

func TestWebFetchToolExecute_UsesInMemoryCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("cache-hit-" + strconv.Itoa(int(n))))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		CacheTTL:          2 * time.Minute,
		AllowPrivateHosts: true,
	})

	first, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	second, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}

	if hits.Load() != 1 {
		t.Fatalf("server hit count = %d, want 1", hits.Load())
	}

	firstData := parseWebFetchResult(t, first)
	secondData := parseWebFetchResult(t, second)
	if firstData["content"] != secondData["content"] {
		t.Fatalf("cached content mismatch: first=%v second=%v", firstData["content"], secondData["content"])
	}
}

func TestWebFetchToolExecute_ReadabilitySelectsMainContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Readable Sample</title></head>
  <body>
    <nav id="main-nav">
      <a href="/home">Home</a>
      <a href="/pricing">Pricing</a>
      <a href="/docs">Docs</a>
      <a href="/about">About</a>
    </nav>
    <div class="article-content">
      <h1>Deep Dive</h1>
      <p>This paragraph carries the primary narrative. It has enough punctuation, context, and meaningful content to represent the real article body.</p>
      <p>Another sentence follows with concrete detail, making the main content score higher than navigation links.</p>
    </div>
  </body>
</html>`))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebFetchResult(t, result)
	content, _ := data["content"].(string)
	if !strings.Contains(content, "primary narrative") {
		t.Fatalf("content should contain main article text, got: %q", content)
	}
	if strings.Count(content, "Pricing") > 1 {
		t.Fatalf("content appears navigation-dominated, got: %q", content)
	}
}

func TestWebFetchToolExecute_BlocksPrivateHostsByDefault(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{Timeout: 5 * time.Second})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"url": "http://127.0.0.1:8080/private",
	})
	if err == nil {
		t.Fatal("expected private host to be blocked")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "blocked") && !strings.Contains(msg, "private") {
		t.Fatalf("error = %v, want blocked/private host message", err)
	}
}

func TestWebFetchToolExecute_AuthHeadersDisableCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization header = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Cookie"); got != "session=abc123" {
			t.Fatalf("cookie header = %q, want %q", got, "session=abc123")
		}
		if got := r.Header.Get("X-Trace-Id"); got != "trace-1" {
			t.Fatalf("x-trace-id header = %q, want %q", got, "trace-1")
		}
		n := hits.Add(1)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("auth-hit-" + strconv.Itoa(int(n))))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		CacheTTL:          2 * time.Minute,
		AllowPrivateHosts: true,
	})

	args := map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "text",
		"headers": map[string]interface{}{
			"X-Trace-Id": "trace-1",
		},
		"auth_bearer": "test-token",
		"cookies":     "session=abc123",
	}

	first, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	second, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}

	if hits.Load() != 2 {
		t.Fatalf("server hit count = %d, want 2 (cache disabled for auth headers)", hits.Load())
	}
	firstData := parseWebFetchResult(t, first)
	secondData := parseWebFetchResult(t, second)
	if firstData["content"] == secondData["content"] {
		t.Fatalf("content should differ across uncached calls: first=%v second=%v", firstData["content"], secondData["content"])
	}
}

func TestWebFetchToolExecute_UsesBrowserSessionCookies(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "reddit_session=abc123; pref=compact" {
			t.Fatalf("cookie header = %q, want %q", got, "reddit_session=abc123; pref=compact")
		}
		n := hits.Add(1)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("browser-cookie-hit-" + strconv.Itoa(int(n))))
	}))
	defer srv.Close()

	browser := &mockBrowserBackend{cookieValue: "reddit_session=abc123; pref=compact"}
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		CacheTTL:          2 * time.Minute,
		AllowPrivateHosts: true,
	})
	tool.SetBrowser(browser)

	args := map[string]interface{}{
		"url":               srv.URL,
		"extract_mode":      "text",
		"browser_target_id": "tab-reddit",
	}
	first, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	second, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}

	if hits.Load() != 2 {
		t.Fatalf("server hit count = %d, want 2 (browser-session fetches bypass cache)", hits.Load())
	}
	if browser.cookieTabID != "tab-reddit" {
		t.Fatalf("cookie tab id = %q, want %q", browser.cookieTabID, "tab-reddit")
	}
	if browser.cookieURL != srv.URL {
		t.Fatalf("cookie url = %q, want %q", browser.cookieURL, srv.URL)
	}
	firstData := parseWebFetchResult(t, first)
	secondData := parseWebFetchResult(t, second)
	if firstData["content"] == secondData["content"] {
		t.Fatalf("content should differ across uncached browser-session calls: first=%v second=%v", firstData["content"], secondData["content"])
	}
	if firstData["extractor"] != "text" || secondData["extractor"] != "text" {
		t.Fatalf("unexpected extractors: first=%v second=%v", firstData["extractor"], secondData["extractor"])
	}
	if firstData["content_type"] != "text/plain" || secondData["content_type"] != "text/plain" {
		t.Fatalf("unexpected content types: first=%v second=%v", firstData["content_type"], secondData["content_type"])
	}
}

func TestWebFetchToolExecute_BrowserTargetRequiresBackend(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":               "https://example.com",
		"browser_target_id": "tab-1",
	})
	if err == nil {
		t.Fatal("expected browser_target_id without backend to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "browser") {
		t.Fatalf("error = %v, want browser backend message", err)
	}
}

func TestWebFetchToolExecute_WarnsOnLoginWall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Sign in</title></head>
  <body>
    <main>
      <h1>Log in</h1>
      <p>Continue with Google</p>
      <p>Forgot password?</p>
    </main>
  </body>
</html>`))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebFetchResult(t, result)
	warning, _ := data["warning"].(string)
	if !strings.Contains(strings.ToLower(warning), "login") {
		t.Fatalf("warning = %q, want login wall guidance", warning)
	}
	if data["warning_code"] != webFetchWarningCodeLoginWall {
		t.Fatalf("warning_code = %v, want %q", data["warning_code"], webFetchWarningCodeLoginWall)
	}
	if data["extractor"] != "html" {
		t.Fatalf("extractor = %v, want %q", data["extractor"], "html")
	}
}

func TestWebFetchToolExecute_FallsBackToBrowserOnLoginWall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Sign in</title></head>
  <body>
    <main>
      <h1>Log in</h1>
      <p>Continue with Google</p>
      <p>Forgot password?</p>
    </main>
  </body>
</html>`))
	}))
	defer srv.Close()

	browser := &mockBrowserBackend{
		navResult:  BrowserNavResult{URL: srv.URL, Title: "Reddit Thread", TargetID: "tab-reddit"},
		a11yResult: BrowserA11yTreeResult{URL: srv.URL, Title: "Reddit Thread", TargetID: "tab-reddit", Tree: "heading 'Reddit Thread'\ntext 'Visible after login'"},
	}
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	tool.SetBrowser(browser)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":               srv.URL,
		"extract_mode":      "text",
		"browser_target_id": "tab-reddit",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebFetchResult(t, result)
	if data["extractor"] != "browser-a11y" {
		t.Fatalf("extractor = %v, want %q", data["extractor"], "browser-a11y")
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "Visible after login") {
		t.Fatalf("content = %q, want browser fallback content", content)
	}
	warning, _ := data["warning"].(string)
	if !strings.Contains(strings.ToLower(warning), "login") {
		t.Fatalf("warning = %q, want login wall guidance", warning)
	}
	if data["warning_code"] != webFetchWarningCodeLoginWall {
		t.Fatalf("warning_code = %v, want %q", data["warning_code"], webFetchWarningCodeLoginWall)
	}
}

func TestDetectWebFetchAuthWall_ReturnsStructuredCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		url        string
		title      string
		content    string
		wantHit    bool
		wantCode   string
	}{
		{name: "browser required", statusCode: http.StatusForbidden, url: "https://example.com/private", wantHit: true, wantCode: webFetchWarningCodeBrowserRequired},
		{name: "login wall", statusCode: http.StatusOK, url: "https://example.com/login", title: "Sign in", content: "Log in Continue with Google Forgot password", wantHit: true, wantCode: webFetchWarningCodeLoginWall},
		{name: "challenge", statusCode: http.StatusTooManyRequests, url: "https://example.com/", title: "Attention Required", content: "Verify you are human", wantHit: true, wantCode: webFetchWarningCodeChallenge},
		{name: "normal page", statusCode: http.StatusOK, url: "https://example.com/", title: "Example", content: "Regular article text", wantHit: false, wantCode: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotHit, gotCode, gotWarning := detectWebFetchAuthWall(tc.statusCode, tc.url, tc.title, tc.content)
			if gotHit != tc.wantHit {
				t.Fatalf("hit = %v, want %v", gotHit, tc.wantHit)
			}
			if gotCode != tc.wantCode {
				t.Fatalf("warning code = %q, want %q", gotCode, tc.wantCode)
			}
			if tc.wantHit && strings.TrimSpace(gotWarning) == "" {
				t.Fatal("expected non-empty warning message")
			}
		})
	}
}

func TestWebFetchToolExecute_FirecrawlFallbackOnHTTPFailure(t *testing.T) {
	var firecrawlHits atomic.Int32
	firecrawlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firecrawlHits.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("firecrawl method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v2/scrape" {
			t.Fatalf("firecrawl path = %s, want /v2/scrape", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer firecrawl-test" {
			t.Fatalf("firecrawl Authorization = %q, want %q", got, "Bearer firecrawl-test")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "success": true,
  "data": {
    "markdown": "# Firecrawl Title\n\nRecovered via fallback.",
    "metadata": {
      "title": "Firecrawl Title",
      "sourceURL": "https://example.com/from-firecrawl"
    }
  }
}`))
	}))
	defer firecrawlSrv.Close()

	originSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, "origin unavailable", http.StatusServiceUnavailable)
	}))
	defer originSrv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
		FirecrawlEnabled:  true,
		FirecrawlAPIKey:   "firecrawl-test",
		FirecrawlBaseURL:  firecrawlSrv.URL,
		FirecrawlTimeout:  5 * time.Second,
	})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          originSrv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if firecrawlHits.Load() != 1 {
		t.Fatalf("firecrawl hit count = %d, want 1", firecrawlHits.Load())
	}

	data := parseWebFetchResult(t, result)
	if data["extractor"] != "firecrawl" {
		t.Fatalf("extractor = %v, want %q", data["extractor"], "firecrawl")
	}
	content, _ := data["content"].(string)
	if strings.Contains(content, "# Firecrawl") {
		t.Fatalf("content should be markdown->text converted in text mode: %q", content)
	}
	if !strings.Contains(content, "Recovered via fallback.") {
		t.Fatalf("content missing fallback text: %q", content)
	}
}

func TestWebFetchToolExecute_RejectsInvalidHeaderValue(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"url": "https://example.com",
		"headers": map[string]interface{}{
			"X-Test": "bad\nvalue",
		},
	})
	if err == nil {
		t.Fatal("expected invalid header value error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "header") {
		t.Fatalf("error = %v, want header validation error", err)
	}
}

func BenchmarkWebFetchToolExecute_NoCache(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("benchmark payload"))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		CacheTTL:          -1,
		AllowPrivateHosts: true,
	})
	args := map[string]interface{}{"url": srv.URL, "extract_mode": "text"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			b.Fatalf("execute failed: %v", err)
		}
	}
}

func BenchmarkWebFetchToolExecute_CacheHit(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("benchmark payload"))
	}))
	defer srv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		CacheTTL:          2 * time.Minute,
		AllowPrivateHosts: true,
	})
	args := map[string]interface{}{"url": srv.URL, "extract_mode": "text"}
	if _, err := tool.Execute(context.Background(), args); err != nil {
		b.Fatalf("warmup failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			b.Fatalf("execute failed: %v", err)
		}
	}
}

func parseWebFetchResult(t *testing.T, result interface{}) map[string]interface{} {
	t.Helper()
	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}
	return data
}

func TestParseWebFetchHelpersSupportNestedArgs(t *testing.T) {
	args := map[string]interface{}{
		"input": map[string]interface{}{
			"maxChars":        321,
			"requestHeaders":  map[string]interface{}{"X-Test": "1"},
			"authBearer":      "tok",
			"browserTargetId": "tab_1",
		},
	}
	if got := parseWebFetchMaxChars(args, 1000, 2000); got != 321 {
		t.Fatalf("max chars = %d, want 321", got)
	}
	opts, err := parseWebFetchRequestOptions(args)
	if err != nil {
		t.Fatalf("parse request options failed: %v", err)
	}
	if got := opts.extraHeaders["X-Test"]; got != "1" {
		t.Fatalf("X-Test = %q, want 1", got)
	}
	if got := opts.extraHeaders["Authorization"]; got != "Bearer tok" {
		t.Fatalf("Authorization = %q, want Bearer tok", got)
	}
	if opts.browserTargetID != "tab_1" {
		t.Fatalf("browserTargetID = %q, want tab_1", opts.browserTargetID)
	}
	if opts.cacheable {
		t.Fatal("expected cacheable=false when auth/browser target is present")
	}
}
