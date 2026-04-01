package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

// mockLLMBridge is a test double for LLMBridge.
type mockLLMBridge struct {
	responses []string // returns responses in order
	calls     []string // records prompts
	idx       int
}

type deadlineProbeBridge struct {
	responses []string
	idx       int
	hadDL     []bool
	left      []time.Duration
}

type analyzeSmallModelMock struct {
	respText string
	err      error
	calls    int
}

type analyzeStatsMock struct {
	docAttempts  int
	docSuccess   int
	autoRollback int
	latencyMs    []time.Duration
	sceneLat     map[string][]time.Duration
	fallbacks    []string
}

func (m *analyzeSmallModelMock) Ready() bool { return true }

func (m *analyzeSmallModelMock) Generate(_ context.Context, _ smallmodel.GenerateRequest) (*smallmodel.GenerateResponse, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &smallmodel.GenerateResponse{Text: m.respText}, nil
}

func (m *analyzeStatsMock) RecordDocExtractAttempt() {
	m.docAttempts++
}

func (m *analyzeStatsMock) RecordDocExtractSuccess() {
	m.docSuccess++
}

func (m *analyzeStatsMock) RecordLatency(d time.Duration) {
	m.latencyMs = append(m.latencyMs, d)
}

func (m *analyzeStatsMock) RecordLatencyWithScene(scene string, d time.Duration) {
	if m.sceneLat == nil {
		m.sceneLat = make(map[string][]time.Duration)
	}
	m.sceneLat[scene] = append(m.sceneLat[scene], d)
}

func (m *analyzeStatsMock) RecordFallback(reason string) {
	m.fallbacks = append(m.fallbacks, reason)
}

func (m *analyzeStatsMock) RecordAutoRollback() {
	m.autoRollback++
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

func (d *deadlineProbeBridge) Chat(ctx context.Context, _ string, _ int) (string, error) {
	deadline, ok := ctx.Deadline()
	d.hadDL = append(d.hadDL, ok)
	if ok {
		d.left = append(d.left, time.Until(deadline))
	} else {
		d.left = append(d.left, 0)
	}

	if d.idx < len(d.responses) {
		resp := d.responses[d.idx]
		d.idx++
		return resp, nil
	}
	return "{}", nil
}

// mockBrowserBackend is a test double for BrowserBackend.
type mockBrowserBackend struct {
	startErr      error
	navResult     BrowserNavResult
	navErr        error
	navResults    []BrowserNavResult
	navErrs       []error
	a11yResult    BrowserA11yTreeResult
	a11yErr       error
	a11yResults   []BrowserA11yTreeResult
	a11yErrs      []error
	cookieValue   string
	cookieErr     error
	cookieURL     string
	cookieTabID   string
	closeCalled   bool
	observed      BrowserObservedNetworkResult
	observeErr    error
	waitIdleErr   error
	waitIdleErrs  []error
	navCalls      int
	a11yCalls     int
	waitIdleCalls int
}

func (m *mockBrowserBackend) Start(_ context.Context) error { return m.startErr }
func (m *mockBrowserBackend) Navigate(_ context.Context, url, _ string) (BrowserNavResult, error) {
	idx := m.navCalls
	m.navCalls++
	if idx < len(m.navErrs) && m.navErrs[idx] != nil {
		return BrowserNavResult{}, m.navErrs[idx]
	}
	if idx < len(m.navResults) {
		r := m.navResults[idx]
		if r.URL == "" {
			r.URL = url
		}
		if r.TargetID == "" {
			r.TargetID = "tab-1"
		}
		return r, nil
	}
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
func (m *mockBrowserBackend) CookieHeader(_ context.Context, targetID, url string) (string, error) {
	m.cookieTabID = targetID
	m.cookieURL = url
	return m.cookieValue, m.cookieErr
}
func (m *mockBrowserBackend) ObserveNetwork(_ context.Context, _ string, _ int, _ bool) (BrowserObservedNetworkResult, error) {
	return m.observed, m.observeErr
}
func (m *mockBrowserBackend) WaitNetworkIdle(_ context.Context, _ string, _ int, _ int) error {
	idx := m.waitIdleCalls
	m.waitIdleCalls++
	if idx < len(m.waitIdleErrs) {
		return m.waitIdleErrs[idx]
	}
	return m.waitIdleErr
}
func (m *mockBrowserBackend) AccessibilityTree(_ context.Context, _ string, _ int) (BrowserA11yTreeResult, error) {
	idx := m.a11yCalls
	m.a11yCalls++
	if idx < len(m.a11yErrs) && m.a11yErrs[idx] != nil {
		return BrowserA11yTreeResult{}, m.a11yErrs[idx]
	}
	if idx < len(m.a11yResults) {
		return m.a11yResults[idx], nil
	}
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

func TestAnalyzeTool_Execute_SupportsNestedCamelCaseArgs(t *testing.T) {
	analysisJSON := `{"summary":"Test summary","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`

	bridge := &mockLLMBridge{responses: []string{analysisJSON}}
	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"topic": "Test Topic",
			"text":  "Some content to analyze",
			"lang":  "en-US",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bridge.calls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(bridge.calls))
	}
	if !strings.Contains(bridge.calls[0], "Test Topic") || !strings.Contains(bridge.calls[0], "Some content to analyze") {
		t.Fatalf("unexpected first prompt: %q", bridge.calls[0])
	}
}

func TestAnalyzeTool_Execute_AnalyzeWithText(t *testing.T) {
	analysisJSON := `{"summary":"Test summary","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON},
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

	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 LLM call, got %d", len(bridge.calls))
	}

	// First call should contain the topic and content
	if !strings.Contains(bridge.calls[0], "Test Topic") {
		t.Error("first LLM call should contain topic")
	}
	if !strings.Contains(bridge.calls[0], "Some content to analyze") {
		t.Error("first LLM call should contain input text")
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
	if resultMap["output_mode"] != "inline" {
		t.Errorf("expected output_mode=inline, got %v", resultMap["output_mode"])
	}
	if resultMap["report_url"] != nil {
		t.Error("did not expect report_url in inline mode")
	}
	if strings.TrimSpace(resultMap["answer"].(string)) == "" {
		t.Error("expected inline answer in result")
	}
}

func TestAnalyzeTool_Execute_ReportModeGeneratesHTMLReport(t *testing.T) {
	analysisJSON := `{"summary":"Test summary","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	htmlBody := `<div class="hero"><h1>Test</h1></div>`

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON, htmlBody},
	}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic":       "Test Topic",
		"text":        "Some content to analyze",
		"lang":        "en-US",
		"output_mode": "report",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bridge.calls) != 2 {
		t.Fatalf("expected 2 LLM calls in report mode, got %d", len(bridge.calls))
	}
	if !strings.Contains(bridge.calls[1], "Test summary") {
		t.Fatal("expected second LLM call to contain analysis data in report mode")
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &resultMap); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if resultMap["report_url"] == nil {
		t.Fatal("expected report_url in report mode")
	}
}

func TestAnalyzeTool_Execute_EmitsDetailedProgressAndCollectionCard(t *testing.T) {
	analysisJSON := `{"summary":"Test summary","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	bridge := &mockLLMBridge{responses: []string{analysisJSON}}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())

	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		copyCard := make(map[string]interface{}, len(card))
		for k, v := range card {
			copyCard[k] = v
		}
		emitted = append(emitted, copyCard)
	})

	_, err := tool.Execute(ctx, map[string]interface{}{
		"topic": "Detailed Progress",
		"text":  strings.Repeat("content line\n", 12),
		"lang":  "en-US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sawTextStep, sawDocStep, sawSaveStep, sawCollectionCard bool
	for _, card := range emitted {
		typ, _ := card["type"].(string)
		switch typ {
		case "analyze-progress":
			step, _ := card["step"].(string)
			status, _ := card["status"].(string)
			switch step {
			case "text_input":
				if status == "success" {
					sawTextStep = true
					if card["char_count"] == nil {
						t.Fatal("expected text_input step to include char_count")
					}
				}
			case "doc_extract":
				if status == "skipped" || status == "success" {
					sawDocStep = true
				}
			case "save_report":
				if status == "skipped" {
					sawSaveStep = true
				}
			}
		case "result":
			if status, _ := card["status"].(string); status == "info" {
				sawCollectionCard = true
			}
		}
	}

	if !sawTextStep {
		t.Fatal("expected text_input progress step")
	}
	if !sawDocStep {
		t.Fatal("expected doc_extract progress step")
	}
	if !sawSaveStep {
		t.Fatal("expected save_report progress step")
	}
	if !sawCollectionCard {
		t.Fatal("expected collection summary card")
	}
}

func TestAnalyzeTool_Execute_AddsLongLLMDeadlineWhenParentHasNone(t *testing.T) {
	analysisJSON := `{"summary":"Test summary","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`

	bridge := &deadlineProbeBridge{responses: []string{analysisJSON}}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Timeout Test",
		"text":  "Some content",
		"lang":  "en-US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bridge.hadDL) != 1 {
		t.Fatalf("expected 1 bridge call, got %d", len(bridge.hadDL))
	}
	for i, ok := range bridge.hadDL {
		if !ok {
			t.Fatalf("call %d should carry deadline", i)
		}
		if bridge.left[i] < analyzeLLMRequestTimeout-10*time.Second {
			t.Fatalf("call %d deadline too short: %s", i, bridge.left[i])
		}
	}
}

func TestAnalyzeTool_Execute_UsesSmallModelDocExtractWhenEnabled(t *testing.T) {
	analysisJSON := `{"summary":"Test summary","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	bridge := &mockLLMBridge{
		responses: []string{analysisJSON},
	}
	sm := &analyzeSmallModelMock{
		respText: "Objective: summarize findings\nInputs: text\nSteps: collect -> analyze\nOutputs: report\nRisks: missing data",
	}
	stats := &analyzeStatsMock{}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())
	tool.SetSmallModelRuntime(sm)
	tool.SetSmallModelSwitchFuncs(
		func() bool { return true },
		func() bool { return true },
	)
	tool.SetSmallModelStatsRecorder(stats)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Doc Extract Topic",
		"text":  strings.Repeat("content line\n", 20),
		"lang":  "en-US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1", sm.calls)
	}
	if len(bridge.calls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(bridge.calls))
	}
	if !strings.Contains(bridge.calls[0], "=== Small-model structured extraction ===") {
		t.Fatalf("analysis prompt should include small-model extraction block, got: %s", bridge.calls[0])
	}
	if stats.docAttempts != 1 || stats.docSuccess != 1 {
		t.Fatalf("unexpected doc extract stats: attempts=%d success=%d", stats.docAttempts, stats.docSuccess)
	}
	if got := len(stats.sceneLat["doc_extract"]); got != 1 {
		t.Fatalf("doc_extract latency samples = %d, want 1", got)
	}
	if len(stats.fallbacks) != 0 {
		t.Fatalf("fallbacks=%v, want none", stats.fallbacks)
	}
}

func TestAnalyzeTool_Execute_DocExtractDisabledSkipsSmallModel(t *testing.T) {
	analysisJSON := `{"summary":"Test summary","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	bridge := &mockLLMBridge{
		responses: []string{analysisJSON},
	}
	sm := &analyzeSmallModelMock{
		respText: "Objective: should not appear",
	}
	stats := &analyzeStatsMock{}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())
	tool.SetSmallModelRuntime(sm)
	tool.SetSmallModelSwitchFuncs(
		func() bool { return true },
		func() bool { return false },
	)
	tool.SetSmallModelStatsRecorder(stats)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Doc Extract Disabled",
		"text":  strings.Repeat("content line\n", 20),
		"lang":  "en-US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0 when doc extract switch disabled", sm.calls)
	}
	if len(bridge.calls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(bridge.calls))
	}
	if strings.Contains(bridge.calls[0], "=== Small-model structured extraction ===") {
		t.Fatalf("analysis prompt should not include small-model extraction block when disabled")
	}
	if stats.docAttempts != 0 || stats.docSuccess != 0 || len(stats.sceneLat) != 0 || len(stats.fallbacks) != 0 {
		t.Fatalf("unexpected doc extract stats when disabled: %+v", stats)
	}
}

func TestAnalyzeTool_DocExtractAutoRollbackDisablesSwitchOnHighFailureRate(t *testing.T) {
	tool := NewAnalyzeTool()
	sm := &analyzeSmallModelMock{err: context.DeadlineExceeded}
	stats := &analyzeStatsMock{}
	docEnabled := true

	tool.SetSmallModelRuntime(sm)
	tool.SetSmallModelSwitchFuncs(
		func() bool { return true },
		func() bool { return docEnabled },
	)
	tool.SetSmallModelDocExtractToggle(func(enabled bool) (bool, error) {
		if docEnabled == enabled {
			return false, nil
		}
		docEnabled = enabled
		return true, nil
	})
	tool.SetSmallModelStatsRecorder(stats)

	raw := strings.Repeat("line content\n", 30)
	for i := 0; i < 40; i++ {
		got := tool.smallModelDocExtract(context.Background(), "Doc Auto Rollback", raw, "en-US")
		if got.content != "" {
			t.Fatalf("smallModelDocExtract should fallback empty on failure, got: %q", got.content)
		}
	}

	if docEnabled {
		t.Fatal("expected doc_extract switch to be auto-disabled")
	}
	if stats.autoRollback != 1 {
		t.Fatalf("auto rollback count = %d, want 1", stats.autoRollback)
	}
	found := false
	for _, reason := range stats.fallbacks {
		if reason == analyzeFallbackReasonAutoRollbackDoc {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected fallback reason %q, got %v", analyzeFallbackReasonAutoRollbackDoc, stats.fallbacks)
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

func TestAnalyzeTool_Execute_PromotesURLFromTopic(t *testing.T) {
	analysisJSON := `{"summary":"Browser analysis","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	htmlBody := `<div class="hero"><h1>Browser Report</h1></div>`

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON, htmlBody},
	}
	browser := &mockBrowserBackend{
		navResult: BrowserNavResult{
			URL:      "https://example.com/blog",
			Title:    "Example Blog",
			TargetID: "tab-1",
		},
		a11yResult: BrowserA11yTreeResult{
			Tree: "heading 'Example Blog'\ntext 'Some content here'",
		},
	}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetBrowser(browser)
	tool.SetMediaDir(t.TempDir())

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Summarise https://example.com/blog and extract the key points.",
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
	if !browser.closeCalled {
		t.Error("expected browser tab to be closed after scraping topic URL")
	}
	if browser.navCalls == 0 {
		t.Fatal("expected browser navigation when topic contains a URL")
	}
	if !strings.Contains(bridge.calls[0], "Example Blog") {
		t.Error("LLM should have received scraped blog content")
	}
}

func TestAnalyzeTool_ScrapeURL_RetriesAccessibilityTreeAfterNetworkIdle(t *testing.T) {
	browser := &mockBrowserBackend{
		navResult: BrowserNavResult{
			URL:      "https://example.com/blog",
			Title:    "Example Blog",
			TargetID: "tab-1",
		},
		a11yErrs: []error{context.DeadlineExceeded, nil},
		a11yResults: []BrowserA11yTreeResult{
			{},
			{
				URL:   "https://example.com/blog",
				Title: "Example Blog",
				Tree:  "heading 'Example Blog'\ntext 'Recovered after retry'",
			},
		},
	}

	tool := NewAnalyzeTool()
	content := tool.scrapeURL(context.Background(), browser, "https://example.com/blog")

	if !strings.Contains(content, "Recovered after retry") {
		t.Fatalf("expected retried scrape content, got %q", content)
	}
	if browser.a11yCalls != 2 {
		t.Fatalf("expected 2 accessibility attempts, got %d", browser.a11yCalls)
	}
	if browser.waitIdleCalls != 2 {
		t.Fatalf("expected 2 wait-idle attempts, got %d", browser.waitIdleCalls)
	}
	if !browser.closeCalled {
		t.Fatal("expected browser tab to be closed")
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

	content, _ := tool.gatherData(context.Background(), map[string]interface{}{
		"urls": urls,
	}, "en-US", browser, nil)

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

	url, err := tool.saveReport("<html>test</html>", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	url, err := tool.saveReport("<html>test</html>", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty URL when no mediaDir, got %q", url)
	}
}
