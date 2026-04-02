package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

type mockAnalyzeExecTool struct {
	definition ToolDefinition
	execute    func(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

func (m *mockAnalyzeExecTool) Definition() ToolDefinition {
	return m.definition
}

func (m *mockAnalyzeExecTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if m.execute == nil {
		return nil, nil
	}
	return m.execute(ctx, args)
}

func readSingleAnalyzeReport(t *testing.T, dir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "analyze", "*.html"))
	if err != nil {
		t.Fatalf("glob report path: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 saved report, got %d (%v)", len(matches), matches)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	return string(data)
}

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
	for _, required := range []string{"topic", "urls", "text", "search_queries", "lang", "report_style"} {
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

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON},
	}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	dir := t.TempDir()
	tool.SetMediaDir(dir)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic":       "Test Topic",
		"text":        "Some content to analyze",
		"lang":        "en-US",
		"output_mode": "report",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bridge.calls) != 1 {
		t.Fatalf("expected 1 LLM call in report mode, got %d", len(bridge.calls))
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
	if resultMap["report_style"] != analyzeReportStyleDashboard {
		t.Fatalf("expected report_style=%s, got %v", analyzeReportStyleDashboard, resultMap["report_style"])
	}
	if resultMap["report_template_version"] != analyzeReportTemplateVersion {
		t.Fatalf("expected report template version %q, got %v", analyzeReportTemplateVersion, resultMap["report_template_version"])
	}
	html := readSingleAnalyzeReport(t, dir)
	if !strings.Contains(html, "research-question") {
		t.Fatal("expected saved report to include research-question block")
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

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON},
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

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON},
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

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON},
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

func TestAnalyzeTool_Execute_PromotesSingularURLAlias(t *testing.T) {
	analysisJSON := `{"summary":"Browser analysis","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`

	bridge := &mockLLMBridge{
		responses: []string{analysisJSON},
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
		"topic": "Summarise and extract the key points.",
		"url":   "https://example.com/blog",
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
		t.Error("expected browser tab to be closed after scraping singular url alias")
	}
	if browser.navCalls == 0 {
		t.Fatal("expected browser navigation when url alias is supplied")
	}
	if !strings.Contains(bridge.calls[0], "Example Blog") {
		t.Error("LLM should have received scraped blog content via singular url alias")
	}
}

func TestAnalyzeTool_Execute_UsesWebReadFallbackWhenBrowserNavigateFails(t *testing.T) {
	analysisJSON := `{"summary":"Web read analysis","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	bridge := &mockLLMBridge{responses: []string{analysisJSON}}
	browser := &mockBrowserBackend{
		navErr: errors.New("navigate failed: net::ERR_CERT_AUTHORITY_INVALID"),
	}

	registry := NewRegistry()
	registry.Register(&mockAnalyzeExecTool{
		definition: ToolDefinition{Name: "web_read"},
		execute: func(_ context.Context, args map[string]interface{}) (interface{}, error) {
			return marshalWebReadResponse(webReadResponse{
				URL:      firstCompatString(args, "url"),
				FinalURL: "https://example.com/blog",
				Title:    "Example Domain",
				Format:   webReadFormatText,
				Content:  "Example Domain content",
				Source:   webAccessSourceHTTP,
			})
		},
	})

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetExecutor(NewExecutor(registry))
	tool.SetBrowser(browser)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Summarise the page",
		"url":   "https://example.com/blog",
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
		t.Fatalf("expected success=true, got %#v", resultMap)
	}
	if browser.navCalls != 0 {
		t.Fatalf("expected web_read fallback to avoid browser navigation, got %d nav calls", browser.navCalls)
	}
	if len(bridge.calls) == 0 || !strings.Contains(bridge.calls[0], "Example Domain content") {
		t.Fatalf("expected LLM prompt to include web_read content, got %#v", bridge.calls)
	}
}

func TestAnalyzeTool_Execute_FallsBackToBrowserWhenWebReadRequestsInteraction(t *testing.T) {
	analysisJSON := `{"summary":"Browser fallback analysis","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`
	bridge := &mockLLMBridge{responses: []string{analysisJSON}}
	browser := &mockBrowserBackend{
		navResult: BrowserNavResult{
			URL:      "https://example.com/blog",
			Title:    "Example Blog",
			TargetID: "tab-1",
		},
		a11yResult: BrowserA11yTreeResult{
			URL:   "https://example.com/blog",
			Title: "Example Blog",
			Tree:  "heading 'Browser content'",
		},
	}

	registry := NewRegistry()
	registry.Register(&mockAnalyzeExecTool{
		definition: ToolDefinition{Name: "web_read"},
		execute: func(_ context.Context, args map[string]interface{}) (interface{}, error) {
			return marshalWebReadResponse(webReadResponse{
				URL:                 firstCompatString(args, "url"),
				FinalURL:            "https://example.com/blog",
				Title:               "Login wall",
				Format:              webReadFormatText,
				Content:             "Please sign in",
				Source:              webAccessSourceHTTP,
				WarningCodes:        []string{webFetchWarningCodeBrowserRequired},
				InteractiveRequired: true,
			})
		},
	})

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetExecutor(NewExecutor(registry))
	tool.SetBrowser(browser)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic": "Summarise the page",
		"url":   "https://example.com/blog",
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
		t.Fatalf("expected success=true, got %#v", resultMap)
	}
	if browser.navCalls != 1 {
		t.Fatalf("expected browser fallback after interactive web_read result, got %d nav calls", browser.navCalls)
	}
	if len(bridge.calls) == 0 || !strings.Contains(bridge.calls[0], "Browser content") {
		t.Fatalf("expected LLM prompt to include browser content, got %#v", bridge.calls)
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

	content, _, _ := tool.gatherData(context.Background(), map[string]interface{}{
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

func TestResolveAnalyzeReportStyle_AutoRules(t *testing.T) {
	if got := resolveAnalyzeReportStyle(map[string]interface{}{"text": "请帮我做模型选型 benchmark 对比"}, "模型分析"); got != analyzeReportStyleBriefing {
		t.Fatalf("expected briefing for comparison topic, got %q", got)
	}
	if got := resolveAnalyzeReportStyle(map[string]interface{}{"text": "用户反馈整理"}, "产品反馈汇总"); got != analyzeReportStyleDashboard {
		t.Fatalf("expected dashboard for aggregation topic, got %q", got)
	}
	if got := resolveAnalyzeReportStyle(map[string]interface{}{"report_style": "dashboard", "text": "best model comparison"}, "compare"); got != analyzeReportStyleDashboard {
		t.Fatalf("expected explicit dashboard override, got %q", got)
	}
}

func TestAnalyzeTool_Execute_ReportModeBriefingContainsStableBlocks(t *testing.T) {
	analysisJSON := `{
		"refined_title":"模型对比报告",
		"research_question":"哪一个模型更适合作为主力？",
		"summary":"这是一个用于选型的简要结论。",
		"stats":[{"label":"候选模型","value":"4","color":"blue"}],
		"comparison_items":[{"name":"Model A","summary":"更均衡","metrics":[{"label":"吞吐","value":"35 tok/s"}],"strengths":["稳定"],"tradeoffs":["成本略高"]}],
		"scenario_recommendations":[{"scenario":"24GB 显存","recommended":"Model A","reason":"综合性价比更高"}],
		"references":[{"label":"Model Card","url":"https://example.com/model-card","description":"官方模型卡"}]
	}`
	bridge := &mockLLMBridge{responses: []string{analysisJSON}}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	dir := t.TempDir()
	tool.SetMediaDir(dir)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic":        "模型 benchmark 对比与选型建议",
		"text":         "需要做模型对比和推荐",
		"lang":         "zh-CN",
		"output_mode":  "report",
		"report_style": "briefing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resultStr := result.(string)
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &resultMap); err != nil {
		t.Fatalf("invalid result json: %v", err)
	}
	if resultMap["report_style"] != analyzeReportStyleBriefing {
		t.Fatalf("expected briefing style, got %v", resultMap["report_style"])
	}

	html := readSingleAnalyzeReport(t, dir)
	for _, needle := range []string{"research-question", "section-number", "callout", "references", "comparison-card"} {
		if !strings.Contains(html, needle) {
			t.Fatalf("expected briefing report html to contain %q", needle)
		}
	}
}

func TestAnalyzeTool_Execute_ReportModeAutoSelectsBriefing(t *testing.T) {
	analysisJSON := `{
		"refined_title":"Benchmark 报告",
		"research_question":"哪个方案更适合？",
		"summary":"用于验证自动样式选择。",
		"comparison_items":[{"name":"方案 A","summary":"更稳"}],
		"references":[{"label":"Example","url":"https://example.com"}]
	}`
	bridge := &mockLLMBridge{responses: []string{analysisJSON}}

	tool := NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	dir := t.TempDir()
	tool.SetMediaDir(dir)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"topic":       "模型 benchmark 对比",
		"text":        "请比较这几个模型并给推荐",
		"lang":        "zh-CN",
		"output_mode": "report",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &resultMap); err != nil {
		t.Fatalf("invalid result json: %v", err)
	}
	if resultMap["report_style"] != analyzeReportStyleBriefing {
		t.Fatalf("expected auto style to resolve to briefing, got %v", resultMap["report_style"])
	}

	html := readSingleAnalyzeReport(t, dir)
	if !strings.Contains(html, "briefing-root") {
		t.Fatal("expected auto-selected briefing report html")
	}
}

func TestBuildAnalyzeReportBody_RendersReferencesForSourceMixes(t *testing.T) {
	analysis := map[string]interface{}{
		"summary":           "summary",
		"research_question": "question",
	}
	cases := []struct {
		name string
		refs []analyzeSourceReference
		want string
	}{
		{
			name: "url_only",
			refs: []analyzeSourceReference{{Kind: "url", Label: "Example", URL: "https://example.com/a"}},
			want: "https://example.com/a",
		},
		{
			name: "search_only",
			refs: []analyzeSourceReference{{Kind: "search_result", Label: "Result", URL: "https://example.com/search"}},
			want: "https://example.com/search",
		},
		{
			name: "mixed",
			refs: []analyzeSourceReference{
				{Kind: "url", Label: "Example", URL: "https://example.com/a"},
				{Kind: "search_result", Label: "Result", URL: "https://example.com/search"},
			},
			want: "https://example.com/search",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			html := buildAnalyzeReportBody("Report", analysis, tc.refs, "en-US", analyzeReportStyleBriefing)
			if !strings.Contains(html, "references") {
				t.Fatal("expected references block")
			}
			if !strings.Contains(html, tc.want) {
				t.Fatalf("expected references html to include %q", tc.want)
			}
		})
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
