package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebReadToolExecute_StaticHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Readable Example</title></head>
  <body>
    <nav><a href="/pricing">Pricing</a></nav>
    <main>
      <h1>Hello Reader</h1>
      <p>This is the main body content for the article page.</p>
    </main>
  </body>
</html>`))
	}))
	defer srv.Close()

	tool := NewWebReadTool(WebFetchConfig{Timeout: 5 * time.Second, AllowPrivateHosts: true})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":    srv.URL,
		"format": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebPlatformResult(t, result)
	if data["source"] != webAccessSourceHTTP {
		t.Fatalf("source = %v, want %q", data["source"], webAccessSourceHTTP)
	}
	if data["title"] != "Readable Example" {
		t.Fatalf("title = %v, want %q", data["title"], "Readable Example")
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "Hello Reader") || !strings.Contains(content, "main body content") {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestWebReadToolExecute_BrowserFallbackOnDynamicShell(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Dynamic App</title></head>
  <body>
    <div id="__next">Loading...</div>
    <script>one</script><script>two</script><script>three</script>
    <script>four</script><script>five</script><script>six</script>
  </body>
</html>`))
	}))
	defer srv.Close()

	tool := NewWebReadTool(WebFetchConfig{Timeout: 5 * time.Second, AllowPrivateHosts: true})
	tool.SetBrowser(&mockBrowserBackend{
		navResult: BrowserNavResult{URL: srv.URL, Title: "Dynamic App", TargetID: "tab-dynamic"},
		a11yResult: BrowserA11yTreeResult{
			URL:      srv.URL,
			Title:    "Dynamic App",
			TargetID: "tab-dynamic",
			Tree:     "heading 'Dynamic App'\ntext 'Loaded by browser runtime'",
		},
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":    srv.URL,
		"format": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebPlatformResult(t, result)
	if data["source"] != webAccessSourceBrowser {
		t.Fatalf("source = %v, want %q", data["source"], webAccessSourceBrowser)
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "Loaded by browser runtime") {
		t.Fatalf("browser fallback content missing: %q", content)
	}
}

func TestWebReadToolExecute_InteractiveRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Sign In Required</title></head>
  <body>
    <main>
      <h1>Sign in</h1>
      <p>Please log in to continue with email and password.</p>
      <a href="/login">Create account</a>
    </main>
  </body>
</html>`))
	}))
	defer srv.Close()

	tool := NewWebReadTool(WebFetchConfig{Timeout: 5 * time.Second, AllowPrivateHosts: true})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":    srv.URL,
		"format": "text",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebPlatformResult(t, result)
	if interactive, _ := data["interactive_required"].(bool); !interactive {
		t.Fatalf("interactive_required = %v, want true", data["interactive_required"])
	}
	codes, _ := data["warning_codes"].([]interface{})
	if len(codes) == 0 || codes[0] != webFetchWarningCodeLoginWall {
		t.Fatalf("warning_codes = %v, want %q present", data["warning_codes"], webFetchWarningCodeLoginWall)
	}
	if data["source"] != webAccessSourceHTTP {
		t.Fatalf("source = %v, want %q", data["source"], webAccessSourceHTTP)
	}
}

func TestWebExtractToolExecute_ExtractsAndRecovers(t *testing.T) {
	tool := NewWebExtractTool(WebFetchConfig{Timeout: 5 * time.Second, AllowPrivateHosts: true})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"html": `<!doctype html>
<html>
  <head><title>Widget Detail</title></head>
  <body>
    <main>
      <article class="product-card featured">
        <h1 class="title">Clean Room Widget</h1>
        <span data-price="usd">$19</span>
        <div class="actions">
          <button class="buy-now primary" data-track="buy">Buy now</button>
        </div>
      </article>
    </main>
  </body>
</html>`,
		"fields": map[string]interface{}{
			"title": "article.product-card h1.title",
			"price_code": map[string]interface{}{
				"xpath":     "//span[@data-price='usd']",
				"attribute": "data-price",
			},
			"cta_track": map[string]interface{}{
				"selector":  "button.checkout",
				"attribute": "data-track",
				"required":  true,
			},
		},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	data := parseWebPlatformResult(t, result)
	payload, _ := data["data"].(map[string]interface{})
	if payload["title"] != "Clean Room Widget" {
		t.Fatalf("title field = %v, want %q", payload["title"], "Clean Room Widget")
	}
	if payload["price_code"] != "usd" {
		t.Fatalf("price_code = %v, want %q", payload["price_code"], "usd")
	}
	if payload["cta_track"] != "buy" {
		t.Fatalf("cta_track = %v, want %q", payload["cta_track"], "buy")
	}
	evidenceMap, _ := data["evidence"].(map[string]interface{})
	ctaEvidence, _ := evidenceMap["cta_track"].([]interface{})
	if len(ctaEvidence) == 0 {
		t.Fatal("expected evidence for cta_track")
	}
	firstEvidence, _ := ctaEvidence[0].(map[string]interface{})
	if firstEvidence["strategy"] != "recovered" {
		t.Fatalf("strategy = %v, want %q", firstEvidence["strategy"], "recovered")
	}
}

func TestWebCrawlToolExecute_CheckpointResume(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><head><title>Home</title></head><body><main><a href="/a">A</a><a href="/b">B</a></main></body></html>`))
		case "/a":
			_, _ = w.Write([]byte(`<html><head><title>Page A</title></head><body><main><a href="/b">B</a><p>A body</p></main></body></html>`))
		case "/b":
			_, _ = w.Write([]byte(`<html><head><title>Page B</title></head><body><main><p>B body</p></main></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tool := NewWebCrawlTool(WebFetchConfig{Timeout: 5 * time.Second, AllowPrivateHosts: true})
	first, err := tool.Execute(context.Background(), map[string]interface{}{
		"seeds":           []string{srv.URL},
		"max_depth":       1,
		"max_pages":       1,
		"max_concurrency": 1,
		"page_max_chars":  400,
	})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}

	firstData := parseWebPlatformResult(t, first)
	firstEdges, _ := firstData["edges"].([]interface{})
	if len(firstEdges) < 2 {
		t.Fatalf("expected seed crawl edges, got %v", firstData["edges"])
	}
	checkpoint, _ := firstData["checkpoint"].(map[string]interface{})
	pending, _ := checkpoint["pending"].([]interface{})
	if len(pending) == 0 {
		t.Fatalf("expected pending checkpoint items, got %v", checkpoint["pending"])
	}

	second, err := tool.Execute(context.Background(), map[string]interface{}{
		"checkpoint":      checkpoint,
		"max_depth":       1,
		"max_pages":       3,
		"max_concurrency": 1,
		"page_max_chars":  400,
	})
	if err != nil {
		t.Fatalf("resume execute failed: %v", err)
	}

	secondData := parseWebPlatformResult(t, second)
	pages, _ := secondData["pages"].([]interface{})
	if len(pages) != 2 {
		t.Fatalf("resume pages = %d, want 2", len(pages))
	}
	stats, _ := secondData["stats"].(map[string]interface{})
	if completed, _ := stats["completed"].(float64); completed != 3 {
		t.Fatalf("completed = %v, want 3", stats["completed"])
	}
}

func parseWebPlatformResult(t *testing.T, result interface{}) map[string]interface{} {
	t.Helper()
	text, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("decode result failed: %v", err)
	}
	return data
}
