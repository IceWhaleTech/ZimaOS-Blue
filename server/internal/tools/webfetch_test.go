package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
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

func TestWebFetchToolExecute_DocumentExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		_, _ = w.Write([]byte("stub docx bytes"))
	}))
	defer srv.Close()

	reader := &stubDocumentReadService{
		result: &convertpkg.DocumentReadResult{
			Format:       "docx",
			Text:         "Executive summary",
			ExtractedVia: "stub:txt",
		},
	}
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	tool.SetDocumentReadService(reader)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          srv.URL + "/report.docx",
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if reader.lastPath == "" || !strings.HasSuffix(reader.lastPath, ".docx") {
		t.Fatalf("reader path = %q, want temp .docx path", reader.lastPath)
	}
	data := parseWebFetchResult(t, result)
	if data["extractor"] != "document" {
		t.Fatalf("extractor = %v, want %q", data["extractor"], "document")
	}
	if data["title"] != "report.docx" {
		t.Fatalf("title = %v, want %q", data["title"], "report.docx")
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "Executive summary") {
		t.Fatalf("content = %q, want extracted document text", content)
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

func TestWebFetchToolExecute_ReusesSessionMemoryOnFollowUpCall(t *testing.T) {
	ctx := WithSessionID(context.Background(), "conv-webfetch-session")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "sid=ok" {
			http.Error(w, "login required", http.StatusUnauthorized)
			return
		}
		n := hits.Add(1)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("session-memory-hit-" + strconv.Itoa(int(n)) + " with enough readable text to keep the layered session lane as the winning result and avoid any unnecessary browser fallback on a successful cookie-backed fetch. The response intentionally includes extra explanatory detail about persisted browser affinity, remembered host strategy, and session reuse so the content comfortably exceeds the readable threshold used by the retrieval scorer."))
	}))
	defer srv.Close()

	browser := &mockBrowserBackend{cookieValue: "sid=ok"}
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:               5 * time.Second,
		CacheTTL:              -1,
		AllowPrivateHosts:     true,
		LayeredFetchEnabled:   true,
		SessionMemoryEnabled:  true,
		DomainStrategyEnabled: true,
	})
	tool.SetBrowser(browser)

	first, err := tool.Execute(ctx, map[string]interface{}{
		"url":               srv.URL,
		"extract_mode":      "text",
		"browser_target_id": "tab-session",
	})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	second, err := tool.Execute(ctx, map[string]interface{}{
		"url":          srv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}

	firstData := parseWebFetchResult(t, first)
	secondData := parseWebFetchResult(t, second)
	if firstData["strategy_used"] != webFetchStrategySession {
		t.Fatalf("first strategy_used = %v, want %q", firstData["strategy_used"], webFetchStrategySession)
	}
	if secondData["strategy_used"] != webFetchStrategySession {
		t.Fatalf("second strategy_used = %v, want %q", secondData["strategy_used"], webFetchStrategySession)
	}
	if secondData["session_reused"] != true {
		t.Fatalf("second session_reused = %v, want true", secondData["session_reused"])
	}
	if browser.cookieTabID != "tab-session" {
		t.Fatalf("cookie tab id = %q, want %q", browser.cookieTabID, "tab-session")
	}
	if hits.Load() != 2 {
		t.Fatalf("server hit count = %d, want 2", hits.Load())
	}
}

func TestWebFetchToolExecute_ClassifiesHardChallengeWithoutAutoSolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "challenge", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	browser := &mockBrowserBackend{
		navResult: BrowserNavResult{URL: srv.URL, Title: "Security Check", TargetID: "tab-hard"},
		a11yResult: BrowserA11yTreeResult{
			URL:      srv.URL,
			Title:    "Security Check",
			TargetID: "tab-hard",
			Tree:     "Please complete the Turnstile CAPTCHA before continuing",
		},
	}
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:               5 * time.Second,
		AllowPrivateHosts:     true,
		LayeredFetchEnabled:   true,
		SessionMemoryEnabled:  true,
		DomainStrategyEnabled: true,
		ChallengePolicy:       webFetchChallengePolicyTypedHandoff,
	})
	tool.SetBrowser(browser)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":               srv.URL,
		"extract_mode":      "text",
		"browser_target_id": "tab-hard",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebFetchResult(t, result)
	if data["strategy_used"] != webFetchStrategyBrowser {
		t.Fatalf("strategy_used = %v, want %q", data["strategy_used"], webFetchStrategyBrowser)
	}
	state, ok := data["challenge_state"].(map[string]interface{})
	if !ok {
		t.Fatalf("challenge_state = %T, want object", data["challenge_state"])
	}
	if state["kind"] != webFetchChallengeKindHard {
		t.Fatalf("challenge_state.kind = %v, want %q", state["kind"], webFetchChallengeKindHard)
	}
	if state["requires_human"] != true {
		t.Fatalf("challenge_state.requires_human = %v, want true", state["requires_human"])
	}
	if state["resume_action"] != "browser" {
		t.Fatalf("challenge_state.resume_action = %v, want browser", state["resume_action"])
	}
}

func TestFetchOrchestratorTracksLightpandaShimAsReadLayer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Shim Article</title></head><body><main><h1>Shim Article</h1><p>This read-layer page is intentionally long enough to count as a strong fetch result for the layered retrieval scorer. It contains enough concrete prose to avoid falling back to Chromium, demonstrates that the lightpanda shim can act as a successful structured reader, and should be remembered as a non-browser lane by the domain strategy memory.</p></main></body></html>`))
	}))
	defer srv.Close()

	cfg := browser.DefaultConfig()
	shim := browser.NewLightpandaService(cfg)
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:               5 * time.Second,
		AllowPrivateHosts:     true,
		LayeredFetchEnabled:   true,
		DomainStrategyEnabled: true,
	})
	tool.SetLightpandaShim(shim)

	result, err := tool.orchestrator.Fetch(context.Background(), FetchRequest{
		URL:               srv.URL,
		Mode:              webFetchExtractText,
		PreferredLane:     webAccessLaneLightpandaShim,
		AllowAutoFallback: true,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.StrategyUsed != webAccessLaneLightpandaShim {
		t.Fatalf("strategy_used = %q, want %q", result.StrategyUsed, webAccessLaneLightpandaShim)
	}
	if result.Payload.Source != webAccessSourceLightpandaShim {
		t.Fatalf("source = %q, want %q", result.Payload.Source, webAccessSourceLightpandaShim)
	}
	if !strings.Contains(result.Payload.Content, "read-layer page is intentionally long enough") {
		t.Fatalf("content = %q, want extracted shim text", result.Payload.Content)
	}

	host := webFetchHostForURL(srv.URL)
	strategy, ok := tool.orchestrator.memory.loadDomain(host)
	if !ok {
		t.Fatal("expected domain strategy memory to record lightpanda_shim result")
	}
	if strategy.PreferredLane != webAccessLaneLightpandaShim {
		t.Fatalf("PreferredLane = %q, want %q", strategy.PreferredLane, webAccessLaneLightpandaShim)
	}
	if strategy.NeedsRealBrowser {
		t.Fatal("expected lightpanda_shim success to keep NeedsRealBrowser=false")
	}
}

func TestWebFetchToolRejectsLightpandaShimForGitHubHosts(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
	})
	tool.SetLightpandaShim(browser.NewLightpandaService(browser.DefaultConfig()))

	_, err := tool.fetchViaLightpandaShim(context.Background(), "https://github.com/search?q=openclaw&type=repositories", webFetchExtractText)
	if err == nil {
		t.Fatal("expected github.com to reject lightpanda shim")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("error = %v, want unsupported host guidance", err)
	}
}

func TestFetchOrchestratorPrefersBrowserForGitHubHosts(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:               5 * time.Second,
		AllowPrivateHosts:     true,
		CacheTTL:              -1,
		LayeredFetchEnabled:   true,
		DomainStrategyEnabled: true,
	})
	tool.httpClient.Transport = testRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected direct HTTP request for browser-first github host: %s", req.URL.String())
		return nil, errors.New("unexpected direct HTTP request")
	})
	tool.SetBrowser(&mockBrowserBackend{
		navResult: BrowserNavResult{
			URL:      "https://github.com/search?q=openclaw&type=repositories",
			Title:    "GitHub Search",
			TargetID: "tab-github",
		},
		a11yResult: BrowserA11yTreeResult{
			URL:      "https://github.com/search?q=openclaw&type=repositories",
			Title:    "GitHub Search",
			TargetID: "tab-github",
			Tree:     "OpenClaw repositories search results with enough readable text to exceed the layered retrieval threshold, keep the browser lane as the winning strategy, and prove that github.com can be handled through a full browser session without touching the shim or direct HTTP lanes first.",
		},
	})

	result, err := tool.orchestrator.Fetch(context.Background(), FetchRequest{
		URL:               "https://github.com/search?q=openclaw&type=repositories",
		Mode:              webFetchExtractText,
		AllowBrowser:      true,
		AllowAutoFallback: true,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.StrategyUsed != webFetchStrategyBrowser {
		t.Fatalf("strategy_used = %q, want %q", result.StrategyUsed, webFetchStrategyBrowser)
	}
	if result.Payload.Source != webAccessSourceBrowser {
		t.Fatalf("source = %q, want %q", result.Payload.Source, webAccessSourceBrowser)
	}
	if !strings.Contains(result.Payload.Content, "OpenClaw repositories search results") {
		t.Fatalf("content = %q, want browser accessibility text", result.Payload.Content)
	}
}

func TestWebFetchToolExecute_PrefersHTTPNativeForConfiguredHosts(t *testing.T) {
	native := &stubHTTPNativeClient{
		available: true,
		do: func(_ context.Context, req webFetchHTTPNativeRequest) (webFetchHTTPNativeResponse, error) {
			if req.URL != "https://news.thepaper.cn/story" {
				t.Fatalf("native request URL = %q, want %q", req.URL, "https://news.thepaper.cn/story")
			}
			return webFetchHTTPNativeResponse{
				FinalURL:    req.URL,
				StatusCode:  http.StatusOK,
				ContentType: "text/html; charset=utf-8",
				Body:        []byte(`<!doctype html><html><head><title>Native Preferred</title></head><body><main><h1>Native Preferred</h1><p>This response is intentionally long enough to count as a fully readable body for the internal native lane selection logic. It includes concrete detail about a preferred-host article, several clauses of descriptive text, and enough readable characters to avoid any short-content fallback behavior.</p></main></body></html>`),
			}, nil
		},
	}

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:               5 * time.Second,
		AllowPrivateHosts:     true,
		HTTPNativeEnabled:     true,
		HTTPNativePreferHosts: []string{"thepaper.cn"},
	})
	tool.nativeClient = native
	tool.httpClient.Transport = testRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected net/http request for preferred native host: %s", req.URL.String())
		return nil, errors.New("unexpected net/http request")
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          "https://news.thepaper.cn/story",
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebFetchResult(t, result)
	if data["source"] != webAccessSourceHTTPNative {
		t.Fatalf("source = %v, want %q", data["source"], webAccessSourceHTTPNative)
	}
	if native.callCount() != 1 {
		t.Fatalf("native call count = %d, want 1", native.callCount())
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "preferred-host article") {
		t.Fatalf("content = %q, want native response body", content)
	}
}

func TestWebFetchToolExecute_FallsBackToHTTPNativeOnTransportError(t *testing.T) {
	native := &stubHTTPNativeClient{
		available: true,
		do: func(_ context.Context, req webFetchHTTPNativeRequest) (webFetchHTTPNativeResponse, error) {
			return webFetchHTTPNativeResponse{
				FinalURL:    req.URL,
				StatusCode:  http.StatusOK,
				ContentType: "text/html; charset=utf-8",
				Body:        []byte(`<!doctype html><html><head><title>Native Recovery</title></head><body><main><h1>Native Recovery</h1><p>The native lane recovered after a transport-level failure from net/http. This body is long enough to be treated as a healthy readable document and proves the internal fallback stayed behind the existing public entry point.</p></main></body></html>`),
			}, nil
		},
	}

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
		HTTPNativeEnabled: true,
	})
	tool.nativeClient = native
	httpCalls := 0
	tool.httpClient.Transport = testRoundTripper(func(req *http.Request) (*http.Response, error) {
		httpCalls++
		return nil, errors.New("http2: server sent GOAWAY and closed the connection")
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          "https://example.com/transport-flake",
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if httpCalls != 1 {
		t.Fatalf("net/http call count = %d, want 1", httpCalls)
	}
	if native.callCount() != 1 {
		t.Fatalf("native call count = %d, want 1", native.callCount())
	}
	data := parseWebFetchResult(t, result)
	if data["source"] != webAccessSourceHTTPNative {
		t.Fatalf("source = %v, want %q", data["source"], webAccessSourceHTTPNative)
	}
}

func TestWebFetchToolExecute_RetriesHTTPNativeOnBlockedResponse(t *testing.T) {
	native := &stubHTTPNativeClient{
		available: true,
		do: func(_ context.Context, req webFetchHTTPNativeRequest) (webFetchHTTPNativeResponse, error) {
			return webFetchHTTPNativeResponse{
				FinalURL:    req.URL,
				StatusCode:  http.StatusOK,
				ContentType: "text/html; charset=utf-8",
				Body:        []byte(`<!doctype html><html><head><title>Native Unlock</title></head><body><main><h1>Native Unlock</h1><p>The native lane produced a readable document after the ordinary HTTP backend only saw a blocked response. This gives the selector enough useful text to prefer the internal native result and clear the browser-required warning state.</p></main></body></html>`),
			}, nil
		},
	}

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:           5 * time.Second,
		AllowPrivateHosts: true,
		HTTPNativeEnabled: true,
	})
	tool.nativeClient = native
	httpCalls := 0
	tool.httpClient.Transport = testRoundTripper(func(req *http.Request) (*http.Response, error) {
		httpCalls++
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Header: http.Header{
				"Content-Type": []string{"text/html; charset=utf-8"},
			},
			Body:    io.NopCloser(strings.NewReader(`<!doctype html><html><head><title>Blocked</title></head><body><main><h1>Blocked</h1><p>Forbidden.</p></main></body></html>`)),
			Request: req,
		}, nil
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          "https://example.com/blocked",
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if httpCalls != 1 {
		t.Fatalf("net/http call count = %d, want 1", httpCalls)
	}
	if native.callCount() != 1 {
		t.Fatalf("native call count = %d, want 1", native.callCount())
	}
	data := parseWebFetchResult(t, result)
	if data["source"] != webAccessSourceHTTPNative {
		t.Fatalf("source = %v, want %q", data["source"], webAccessSourceHTTPNative)
	}
	if got := data["warning_code"]; got != nil {
		t.Fatalf("warning_code = %v, want nil after native recovery", got)
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "clear the browser-required warning state") {
		t.Fatalf("content = %q, want native recovery body", content)
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

func TestWebFetchToolExecute_JinaReaderFallbackAfterFirecrawlFailure(t *testing.T) {
	var firecrawlHits atomic.Int32
	firecrawlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firecrawlHits.Add(1)
		http.Error(w, "firecrawl unavailable", http.StatusBadGateway)
	}))
	defer firecrawlSrv.Close()

	var jinaHits atomic.Int32
	jinaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jinaHits.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer jina-test" {
			t.Fatalf("jina Authorization = %q, want %q", got, "Bearer jina-test")
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("# Jina Reader Title\n\nRecovered from the secondary proxy fetcher backend."))
	}))
	defer jinaSrv.Close()

	originSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "origin unavailable", http.StatusServiceUnavailable)
	}))
	defer originSrv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:               5 * time.Second,
		AllowPrivateHosts:     true,
		FirecrawlEnabled:      true,
		FirecrawlAPIKey:       "firecrawl-test",
		FirecrawlBaseURL:      firecrawlSrv.URL,
		FirecrawlTimeout:      5 * time.Second,
		JinaReaderEnabled:     true,
		JinaReaderAPIKey:      "jina-test",
		JinaReaderBaseURL:     jinaSrv.URL,
		JinaReaderTimeout:     5 * time.Second,
		ProxyFetcherProviders: []string{webFetchProxyProviderFirecrawl, webFetchProxyProviderJinaReader},
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
	if jinaHits.Load() != 1 {
		t.Fatalf("jina hit count = %d, want 1", jinaHits.Load())
	}

	data := parseWebFetchResult(t, result)
	if data["extractor"] != "jina-reader" {
		t.Fatalf("extractor = %v, want %q", data["extractor"], "jina-reader")
	}
	if data["title"] != "Jina Reader Title" {
		t.Fatalf("title = %v, want %q", data["title"], "Jina Reader Title")
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "Recovered from the secondary proxy fetcher backend.") {
		t.Fatalf("content missing jina fallback text: %q", content)
	}
}

func TestWebFetchToolExecute_JinaReaderWorksWithoutAPIKeyWhenEnabled(t *testing.T) {
	var jinaHits atomic.Int32
	jinaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jinaHits.Add(1)
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("jina Authorization = %q, want empty", got)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("# Keyless Jina Reader\n\nWorks without an API key when explicitly enabled."))
	}))
	defer jinaSrv.Close()

	originSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "origin unavailable", http.StatusServiceUnavailable)
	}))
	defer originSrv.Close()

	tool := NewWebFetchTool(WebFetchConfig{
		Timeout:               5 * time.Second,
		AllowPrivateHosts:     true,
		JinaReaderEnabled:     true,
		JinaReaderBaseURL:     jinaSrv.URL,
		JinaReaderTimeout:     5 * time.Second,
		ProxyFetcherProviders: []string{webFetchProxyProviderJinaReader},
	})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          originSrv.URL,
		"extract_mode": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if jinaHits.Load() != 1 {
		t.Fatalf("jina hit count = %d, want 1", jinaHits.Load())
	}

	data := parseWebFetchResult(t, result)
	if data["extractor"] != "jina-reader" {
		t.Fatalf("extractor = %v, want %q", data["extractor"], "jina-reader")
	}
	if data["title"] != "Keyless Jina Reader" {
		t.Fatalf("title = %v, want %q", data["title"], "Keyless Jina Reader")
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

func TestParseWebFetchRequestOptions_SupportsGroupedRequestObject(t *testing.T) {
	args := map[string]interface{}{
		"request": map[string]interface{}{
			"headers":           map[string]interface{}{"X-Test": "1"},
			"auth_bearer":       "tok",
			"browser_target_id": "tab_2",
		},
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
	if opts.browserTargetID != "tab_2" {
		t.Fatalf("browserTargetID = %q, want tab_2", opts.browserTargetID)
	}
}
