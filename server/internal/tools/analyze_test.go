package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// mockLLMBridge is a test double for LLMBridge.
type mockLLMBridge struct {
	responses []string // returns responses in order
	calls     []string // records prompts
	idx       int
}

func (m *mockLLMBridge) Chat(_ context.Context, prompt string, _ int) (string, error) {
	m.calls = append(m.calls, prompt)
	if m.idx < len(m.responses) {
		resp := m.responses[m.idx]
		m.idx++
		return resp, nil
	}
	return "{}", nil
}

// mockBrowserBackend is a test double for BrowserBackend.
type mockBrowserBackend struct {
	startErr    error
	navResult   BrowserNavResult
	navErr      error
	a11yResult  BrowserA11yTreeResult
	a11yErr     error
	closeCalled bool
}

func (m *mockBrowserBackend) Start(_ context.Context) error { return m.startErr }
func (m *mockBrowserBackend) Navigate(_ context.Context, url, _ string) (BrowserNavResult, error) {
	if m.navErr != nil {
		return BrowserNavResult{}, m.navErr
	}
	r := m.navResult
	if r.URL == "" {
		r.URL = url
	}
	if r.TargetID == "" {
		r.TargetID = "tab-1"
	}
	return r, nil
}
func (m *mockBrowserBackend) AccessibilityTree(_ context.Context, _ string, _ int) (BrowserA11yTreeResult, error) {
	return m.a11yResult, m.a11yErr
}
func (m *mockBrowserBackend) InteractiveElements(_ context.Context, _ string) (BrowserInteractiveResult, error) {
	return BrowserInteractiveResult{}, nil
}
func (m *mockBrowserBackend) CountInteractiveElements(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (m *mockBrowserBackend) ActByRef(_ context.Context, _ string, _ int, _ map[int]int, _ string, _ string) error {
	return nil
}
func (m *mockBrowserBackend) ActByInteractiveRef(_ context.Context, _ string, _ int, _ map[int]string, _ string, _ string) error {
	return nil
}
func (m *mockBrowserBackend) Screenshot(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockBrowserBackend) ScreenshotTab(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockBrowserBackend) CloseTab(_ context.Context, _ string) error {
	m.closeCalled = true
	return nil
}
func (m *mockBrowserBackend) Tabs(_ context.Context) ([]BrowserTabResult, error) {
	return nil, nil
}
func (m *mockBrowserBackend) ExecuteRecipe(_ context.Context, _ string, _ map[string]string) (BrowserRecipeResult, error) {
	return BrowserRecipeResult{}, nil
}
func (m *mockBrowserBackend) ListRecipes(_ context.Context) []BrowserRecipeInfo { return nil }

func TestAnalyzeTool_Definition(t *testing.T) {
	tool := NewAnalyzeTool()
	def := tool.Definition()

	if def.Name != "analyze" {
		t.Errorf("expected name 'analyze', got %q", def.Name)
	}
	if def.Icon != "analyze" {
		t.Errorf("expected icon 'analyze', got %q", def.Icon)
	}

	props, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties in parameters")
	}
	for _, required := range []string{"topic", "urls", "text", "search_queries", "lang"} {
		if _, ok := props[required]; !ok {
			t.Errorf("missing parameter: %s", required)
		}
	}
}

func TestAnalyzeTool_Execute_MissingTopic(t *testing.T) {
	tool := NewAnalyzeTool()
	tool.SetLLMBridge(&mockLLMBridge{})

	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "topic is required") {
		t.Errorf("expected 'topic is required' error, got: %v", err)
	}
}

func TestAnalyzeTool_Execute_NoBridge(t *testing.T) {
	tool := NewAnalyzeTool()

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "test",
	})
	if err == nil || !strings.Contains(err.Error(), "LLM bridge not available") {
		t.Errorf("expected 'LLM bridge not available' error, got: %v", err)
	}
}

func TestAnalyzeTool_Execute_AnalyzeWithText(t *testing.T) {
	analysisJSON := `{"summary":"Test summary","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	htmlBody := `<div class="hero"><h1>Test</h1></div>`

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON, htmlBody},
	}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Test Topic",
		"text":  "Some content to analyze",
		"lang":  "en-US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have called LLM twice (extract + generate HTML)
	if len(bridge.calls) != 2 {
		t.Errorf("expected 2 LLM calls, got %d", len(bridge.calls))
	}

	// First call should contain the topic and content
	if !strings.Contains(bridge.calls[0], "Test Topic") {
		t.Error("first LLM call should contain topic")
	}
	if !strings.Contains(bridge.calls[0], "Some content to analyze") {
		t.Error("first LLM call should contain input text")
	}

	// Second call should contain the analysis JSON
	if !strings.Contains(bridge.calls[1], "Test summary") {
		t.Error("second LLM call should contain analysis data")
	}

	// Result should be JSON with success
	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &resultMap); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if resultMap["success"] != true {
		t.Error("expected success=true")
	}
	if resultMap["report_url"] == nil {
		t.Error("expected report_url in result")
	}
}

func TestAnalyzeTool_Execute_NoData(t *testing.T) {
	tool := NewAnalyzeTool()
	tool.SetLLMBridge(&mockLLMBridge{})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "test",
	})
	if err == nil || !strings.Contains(err.Error(), "no data collected") {
		t.Errorf("expected 'no data collected' error, got: %v", err)
	}
}

func TestAnalyzeTool_Execute_FullAnalysis_WithText(t *testing.T) {
	analysisJSON := `{"summary":"Analysis","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	htmlBody := `<div class="hero"><h1>Report</h1></div>`

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON, htmlBody},
	}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Full Test",
		"text":  "Direct text input for analysis",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &resultMap); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if resultMap["success"] != true {
		t.Error("expected success=true")
	}
}

func TestAnalyzeTool_Execute_FullAnalysis_WithBrowser(t *testing.T) {
	analysisJSON := `{"summary":"Browser analysis","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	htmlBody := `<div class="hero"><h1>Browser Report</h1></div>`

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON, htmlBody},
	}
	browser := &mockBrowserBackend{
		navResult: BrowserNavResult{
			URL:      "https://example.com",
			Title:    "Example",
			TargetID: "tab-1",
		},
		a11yResult: BrowserA11yTreeResult{
			Tree: "heading 'Example Page'\ntext 'Some content here'",
		},
	}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetBrowser(browser)
	tool.SetMediaDir(t.TempDir())

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Browser Test",
		"urls":  []interface{}{"https://example.com"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &resultMap); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if resultMap["success"] != true {
		t.Error("expected success=true")
	}

	// Browser should have been used
	if !browser.closeCalled {
		t.Error("expected browser tab to be closed after scraping")
	}

	// LLM should have received the scraped content
	if !strings.Contains(bridge.calls[0], "Example") {
		t.Error("LLM should have received scraped page title")
	}
}

func TestAnalyzeTool_GatherData_URLLimit(t *testing.T) {
	tool := NewAnalyzeTool()
	browser := &mockBrowserBackend{
		a11yResult: BrowserA11yTreeResult{Tree: "content"},
	}

	// Create 10 URLs — should only process 5
	urls := make([]interface{}, 10)
	for i := range urls {
		urls[i] = "https://example.com"
	}

	content := tool.gatherData(context.Background(), map[string]interface{}{
		"urls": urls,
	}, browser, nil)

	// Count URL sections
	count := strings.Count(content, "=== URL:")
	if count > analyzeMaxURLs {
		t.Errorf("expected max %d URLs, got %d", analyzeMaxURLs, count)
	}
}

func TestBuildAnalyzeHTML(t *testing.T) {
	html := buildAnalyzeHTML("Test Report", "en-US", "<div>body</div>")

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected DOCTYPE")
	}
	if !strings.Contains(html, `lang="en-US"`) {
		t.Error("expected lang attribute")
	}
	if !strings.Contains(html, "<title>Test Report</title>") {
		t.Error("expected title")
	}
	if !strings.Contains(html, "<div>body</div>") {
		t.Error("expected body content")
	}
	if !strings.Contains(html, ".hero") {
		t.Error("expected CSS to be included")
	}
	if !strings.Contains(html, "Generated on") {
		t.Error("expected footer with generation date")
	}
}

func TestBuildAnalyzeHTML_DefaultLang(t *testing.T) {
	html := buildAnalyzeHTML("Test", "", "<div>body</div>")
	if !strings.Contains(html, `lang="zh-CN"`) {
		t.Error("expected default lang zh-CN")
	}
}

func TestLangDisplayName(t *testing.T) {
	tests := []struct {
		lang     string
		expected string
	}{
		{"zh-CN", "Chinese"},
		{"zh-TW", "Chinese"},
		{"en-US", "English"},
		{"ja-JP", "Japanese"},
		{"ko-KR", "Korean"},
		{"fr-FR", "French"},
		{"de-DE", "German"},
		{"es-ES", "Spanish"},
		{"unknown", "English"},
	}
	for _, tt := range tests {
		got := langDisplayName(tt.lang)
		if got != tt.expected {
			t.Errorf("langDisplayName(%q) = %q, want %q", tt.lang, got, tt.expected)
		}
	}
}

func TestAnalyzeTool_SaveReport(t *testing.T) {
	tool := NewAnalyzeTool()
	dir := t.TempDir()

	url := tool.saveReport("<html>test</html>", dir)
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	if !strings.HasPrefix(url, "/api/v1/media/analyze/") {
		t.Errorf("unexpected URL prefix: %s", url)
	}
	if !strings.HasSuffix(url, ".html") {
		t.Errorf("expected .html suffix: %s", url)
	}
}

func TestAnalyzeTool_SaveReport_NoMediaDir(t *testing.T) {
	tool := NewAnalyzeTool()
	url := tool.saveReport("<html>test</html>", "")
	if url != "" {
		t.Errorf("expected empty URL when no mediaDir, got %q", url)
	}
}
