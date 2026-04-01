package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// ============================================================
// Mock implementations
// ============================================================

// --- Mock CronService ---

type mockCronService struct {
	jobs              map[string]CronJobInfo
	lastCreatePayload map[string]interface{}
}

func newMockCronService() *mockCronService {
	return &mockCronService{jobs: make(map[string]CronJobInfo)}
}

func (m *mockCronService) Create(name, description, schedule, handler string, payload map[string]interface{}) (CronJobInfo, error) {
	return m.CreateForOwner("", name, description, schedule, handler, payload)
}

func (m *mockCronService) CreateForOwner(ownerID, name, description, schedule, handler string, payload map[string]interface{}) (CronJobInfo, error) {
	id := fmt.Sprintf("job-%d", len(m.jobs)+1)
	m.lastCreatePayload = payload
	job := CronJobInfo{
		ID: id, Name: name, Description: description,
		Schedule: schedule, Handler: handler, Enabled: true, Status: "active",
	}
	m.jobs[id] = job
	return job, nil
}

func (m *mockCronService) List() []CronJobInfo {
	var out []CronJobInfo
	for _, j := range m.jobs {
		out = append(out, j)
	}
	return out
}

func (m *mockCronService) Get(id string) (CronJobInfo, bool) {
	job, ok := m.jobs[id]
	return job, ok
}

func (m *mockCronService) ListByOwner(ownerID string) []CronJobInfo {
	return m.List()
}

func (m *mockCronService) Delete(id string) error {
	if _, ok := m.jobs[id]; !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	delete(m.jobs, id)
	return nil
}

func (m *mockCronService) DeleteByOwner(id, ownerID string) error {
	return m.Delete(id)
}

func (m *mockCronService) Trigger(id string) error {
	if _, ok := m.jobs[id]; !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	return nil
}

func (m *mockCronService) Enable(id string) error {
	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	j.Enabled = true
	m.jobs[id] = j
	return nil
}

func (m *mockCronService) Disable(id string) error {
	j, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}
	j.Enabled = false
	m.jobs[id] = j
	return nil
}

func (m *mockCronService) GetExecutions(jobID string, limit int) ([]CronJobExecution, error) {
	if _, ok := m.jobs[jobID]; !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	if limit <= 0 {
		limit = 20
	}
	out := []CronJobExecution{
		{
			ID:        "exec-1",
			JobID:     jobID,
			StartedAt: "2026-01-01T00:00:00Z",
			Status:    "completed",
			Duration:  "100ms",
		},
	}
	if len(out) > limit {
		return out[:limit], nil
	}
	return out, nil
}

// --- Mock BrowserService ---

type mockBrowserService struct {
	started                 bool
	tabs                    []BrowserTabInfo
	interactiveCount        int // configurable for auto-snapshot tests
	screenshotData          string
	screenshotTabData       string
	extractedTextBySelector map[string]string
	recipeData              map[string]interface{}
	lastScreenshotURL       string
	lastScreenshotTab       string
	lastActMode             string
	lastActTargetID         string
	lastActRef              int
	lastActAction           string
	lastActValue            string
}

func newMockBrowserService() *mockBrowserService {
	return &mockBrowserService{interactiveCount: 5}
}

func (m *mockBrowserService) Start(_ context.Context) error {
	m.started = true
	return nil
}

func (m *mockBrowserService) Navigate(_ context.Context, url, _ string) (BrowserNavResult, error) {
	tab := BrowserTabInfo{TargetID: "tab-1", URL: url, Title: "Test Page", Active: true}
	m.tabs = []BrowserTabInfo{tab}
	return BrowserNavResult{URL: url, Title: "Test Page", TargetID: "tab-1"}, nil
}

func (m *mockBrowserService) currentTabMeta() (string, string) {
	for _, tab := range m.tabs {
		if tab.Active {
			return tab.URL, tab.Title
		}
	}
	if len(m.tabs) > 0 {
		return m.tabs[0].URL, m.tabs[0].Title
	}
	return "https://example.com", "Test Page"
}

func (m *mockBrowserService) ExtractText(_ context.Context, _ string, selector string) (string, error) {
	if m.extractedTextBySelector == nil {
		return "", fmt.Errorf("selector not found: %s", selector)
	}
	if text, ok := m.extractedTextBySelector[selector]; ok {
		return text, nil
	}
	return "", fmt.Errorf("selector not found: %s", selector)
}

func (m *mockBrowserService) AccessibilityTree(_ context.Context, _ string, _ int) (BrowserA11yResult, error) {
	url, title := m.currentTabMeta()
	return BrowserA11yResult{
		Tree:     "@1 [button] \"Submit\"\n@2 [textbox] \"Search\"",
		URL:      url,
		Title:    title,
		TargetID: "tab-1",
		RefMap:   map[int]int{1: 100, 2: 200},
	}, nil
}

func (m *mockBrowserService) InteractiveElements(_ context.Context, _ string) (BrowserInteractiveResult, error) {
	url, title := m.currentTabMeta()
	return BrowserInteractiveResult{
		Tree:     "@1 [button] \"Submit\"\n@2 [input] \"Search\" type=text",
		URL:      url,
		Title:    title,
		TargetID: "tab-1",
		RefMap:   map[int]string{1: "__interactive_ref_0", 2: "__interactive_ref_1"},
		Count:    2,
	}, nil
}

func (m *mockBrowserService) CountInteractiveElements(_ context.Context, _ string) (int, error) {
	return m.interactiveCount, nil
}

func (m *mockBrowserService) ActByRef(_ context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error {
	if _, ok := refMap[ref]; !ok {
		return fmt.Errorf("unknown ref @%d", ref)
	}
	m.lastActMode = "a11y"
	m.lastActTargetID = targetID
	m.lastActRef = ref
	m.lastActAction = action
	m.lastActValue = value
	return nil
}

func (m *mockBrowserService) ActByInteractiveRef(_ context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error {
	if _, ok := refMap[ref]; !ok {
		return fmt.Errorf("unknown ref @%d", ref)
	}
	m.lastActMode = "interactive"
	m.lastActTargetID = targetID
	m.lastActRef = ref
	m.lastActAction = action
	m.lastActValue = value
	return nil
}

func (m *mockBrowserService) Screenshot(_ context.Context, url string) (string, error) {
	m.lastScreenshotURL = url
	if m.screenshotData != "" {
		return m.screenshotData, nil
	}
	return "base64data", nil
}

func (m *mockBrowserService) ScreenshotTab(_ context.Context, targetID string) (string, error) {
	m.lastScreenshotTab = targetID
	if m.screenshotTabData != "" {
		return m.screenshotTabData, nil
	}
	return "base64tabdata", nil
}

func (m *mockBrowserService) ScreenshotViewport(_ context.Context, _ string) (string, error) {
	return "base64viewportdata", nil
}

func (m *mockBrowserService) ScreenshotViewportRaw(_ context.Context, _ string) ([]byte, error) {
	return []byte("rawviewportdata"), nil
}

func (m *mockBrowserService) SetViewport(_ context.Context, _ string, _, _ int) error {
	return nil
}

func (m *mockBrowserService) ScrollTo(_ context.Context, _ string, _, _ int) error {
	return nil
}

func (m *mockBrowserService) PageDimensions(_ context.Context, _ string) (int, int, error) {
	return 800, 800, nil // single viewport height = scroll height (no chunking needed)
}

func (m *mockBrowserService) Stop(_ context.Context) error {
	m.started = false
	return nil
}

func (m *mockBrowserService) CloseTab(_ context.Context, targetID string) error {
	m.tabs = nil
	return nil
}

func (m *mockBrowserService) Tabs(_ context.Context) ([]BrowserTabInfo, error) {
	return m.tabs, nil
}

func (m *mockBrowserService) ExecuteRecipe(_ context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error) {
	data := m.recipeData
	if data == nil {
		data = map[string]interface{}{
			"recipe": recipe,
			"params": params,
		}
	}
	return BrowserRecipeResult{
		Success:  true,
		Data:     data,
		TargetID: "tab-1",
		Message:  "recipe executed",
	}, nil
}

func (m *mockBrowserService) ListRecipes(_ context.Context) []BrowserRecipeInfo {
	return []BrowserRecipeInfo{
		{Name: "search", Description: "Search by query", KeepTab: true},
		{Name: "extract", Description: "Extract page content", KeepTab: true},
	}
}

// ============================================================
// Tests
// ============================================================

func TestScheduler(t *testing.T) {
	sched := NewScheduler()
	mock := newMockCronService()
	sched.SetCronService(mock)

	t.Run("manifest", func(t *testing.T) {
		m := sched.Manifest()
		if m.ID != "scheduler" {
			t.Errorf("expected ID 'scheduler', got '%s'", m.ID)
		}
	})

	t.Run("validate_missing_action", func(t *testing.T) {
		err := sched.Validate(map[string]any{})
		if err == nil {
			t.Error("expected error for missing action")
		}
	})

	t.Run("validate_invalid_action", func(t *testing.T) {
		err := sched.Validate(map[string]any{"action": "nope"})
		if err == nil {
			t.Error("expected error for invalid action")
		}
	})

	t.Run("validate_create_missing_fields", func(t *testing.T) {
		err := sched.Validate(map[string]any{"action": "create"})
		if err == nil {
			t.Error("expected error for missing name")
		}
	})

	t.Run("create_and_list", func(t *testing.T) {
		ctx := tools.WithSessionID(tools.WithUserID(context.Background(), "user-42"), "conv-42")
		result, err := sched.Execute(ctx, map[string]any{
			"action":   "create",
			"name":     "test-job",
			"schedule": "*/5 * * * *",
			"command":  "echo hello",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}
		if got := mock.lastCreatePayload["user_id"]; got != "user-42" {
			t.Fatalf("payload user_id = %v, want user-42", got)
		}
		if got := mock.lastCreatePayload["conversation_id"]; got != "conv-42" {
			t.Fatalf("payload conversation_id = %v, want conv-42", got)
		}

		// List
		result, err = sched.Execute(context.Background(), map[string]any{"action": "list"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["count"] != 1 {
			t.Errorf("expected 1 job, got %v", data["count"])
		}
	})

	t.Run("trigger", func(t *testing.T) {
		result, err := sched.Execute(context.Background(), map[string]any{
			"action": "trigger", "id": "job-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}
	})

	t.Run("disable_enable", func(t *testing.T) {
		result, err := sched.Execute(context.Background(), map[string]any{
			"action": "disable", "id": "job-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}

		result, err = sched.Execute(context.Background(), map[string]any{
			"action": "enable", "id": "job-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}
	})

	t.Run("delete", func(t *testing.T) {
		result, err := sched.Execute(context.Background(), map[string]any{
			"action": "delete", "id": "job-1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}

		// List should be empty
		result, err = sched.Execute(context.Background(), map[string]any{"action": "list"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data := result.Data.(map[string]any)
		if data["count"] != 0 {
			t.Errorf("expected 0 jobs, got %v", data["count"])
		}
	})

	t.Run("no_service", func(t *testing.T) {
		noSvc := NewScheduler()
		result, err := noSvc.Execute(context.Background(), map[string]any{"action": "list"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Success {
			t.Error("expected failure when service is nil")
		}
	})
}

func TestBrowserSkill(t *testing.T) {
	br := NewBrowser()
	mock := newMockBrowserService()
	br.SetBrowserService(mock)

	t.Run("manifest", func(t *testing.T) {
		if br.Manifest().ID != "browser" {
			t.Errorf("expected ID 'browser', got '%s'", br.Manifest().ID)
		}
	})

	t.Run("validate", func(t *testing.T) {
		if err := br.Validate(map[string]any{}); err == nil {
			t.Error("expected error for missing action")
		}
		if err := br.Validate(map[string]any{"action": "navigate"}); err == nil {
			t.Error("expected error for missing url")
		}
		if err := br.Validate(map[string]any{"action": "act"}); err == nil {
			t.Error("expected error for missing ref")
		}
		if err := br.Validate(map[string]any{"action": "act", "ref": 1}); err == nil {
			t.Error("expected error for missing act_type")
		}
		if err := br.Validate(map[string]any{"action": "tabs"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if err := br.Validate(map[string]any{"action": "snapshot_interactive"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if err := br.Validate(map[string]any{"action": "screenshot"}); err != nil {
			t.Errorf("unexpected error for screenshot without url/target_id: %v", err)
		}
		if err := br.Validate(map[string]any{"action": "scroll", "ref": 1}); err != nil {
			t.Errorf("unexpected error for legacy action alias: %v", err)
		}
		input := map[string]any{"href": "https://example.com/page"}
		if err := br.Validate(input); err != nil {
			t.Errorf("unexpected error for href alias: %v", err)
		}
		if got, _ := input["action"].(string); got != "navigate" {
			t.Errorf("action = %q, want navigate", got)
		}
		if got, _ := input["url"].(string); got != "https://example.com/page" {
			t.Errorf("url = %q, want https://example.com/page", got)
		}
	})

	t.Run("navigate", func(t *testing.T) {
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "navigate", "url": "https://example.com",
		})
		if err != nil || !result.Success {
			t.Fatalf("navigate failed: err=%v success=%v", err, result.Success)
		}
		data := result.Data.(map[string]any)
		if data["url"] != "https://example.com" {
			t.Errorf("expected url 'https://example.com', got '%v'", data["url"])
		}
		if data["tree"] == nil || data["tree"] == "" {
			t.Error("expected non-empty tree")
		}
	})

	t.Run("navigate_href_alias", func(t *testing.T) {
		result, err := br.Execute(context.Background(), map[string]any{
			"href": "https://example.com/from-href",
		})
		if err != nil || !result.Success {
			t.Fatalf("navigate href alias failed: err=%v success=%v", err, result.Success)
		}
		if len(mock.tabs) == 0 || mock.tabs[0].URL != "https://example.com/from-href" {
			t.Errorf("expected browser to navigate to href alias, tabs=%#v", mock.tabs)
		}
	})

	t.Run("snapshot", func(t *testing.T) {
		result, err := br.Execute(context.Background(), map[string]any{"action": "snapshot"})
		if err != nil || !result.Success {
			t.Fatalf("snapshot failed: err=%v", err)
		}
	})

	t.Run("act_a11y_ref", func(t *testing.T) {
		// After navigate, lastRefMode should be "a11y"
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "navigate", "url": "https://example.com",
		})
		if err != nil || !result.Success {
			t.Fatalf("navigate failed: err=%v", err)
		}

		result, err = br.Execute(context.Background(), map[string]any{
			"action": "act", "ref": float64(1), "act_type": "click",
		})
		if err != nil || !result.Success {
			t.Fatalf("act failed: err=%v success=%v", err, result.Success)
		}
	})

	t.Run("act_a11y_ref_with_at_prefix", func(t *testing.T) {
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "navigate", "url": "https://example.com",
		})
		if err != nil || !result.Success {
			t.Fatalf("navigate failed: err=%v", err)
		}

		result, err = br.Execute(context.Background(), map[string]any{
			"action": "act", "ref": "@1", "act_type": "click",
		})
		if err != nil || !result.Success {
			t.Fatalf("act failed: err=%v success=%v", err, result.Success)
		}
	})

	t.Run("snapshot_interactive_and_act", func(t *testing.T) {
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "snapshot_interactive",
		})
		if err != nil || !result.Success {
			t.Fatalf("snapshot_interactive failed: err=%v", err)
		}
		data := result.Data.(map[string]any)
		if data["count"] != 2 {
			t.Errorf("expected 2 interactive elements, got %v", data["count"])
		}

		// Act using interactive ref
		result, err = br.Execute(context.Background(), map[string]any{
			"action": "act", "ref": float64(1), "act_type": "click",
		})
		if err != nil || !result.Success {
			t.Fatalf("act with interactive ref failed: err=%v", err)
		}
	})

	t.Run("legacy_scroll_action_alias", func(t *testing.T) {
		mock.lastActMode = ""
		mock.lastActTargetID = ""
		mock.lastActRef = 0
		mock.lastActAction = ""
		mock.lastActValue = ""

		result, err := br.Execute(context.Background(), map[string]any{
			"action": "snapshot_interactive",
		})
		if err != nil || !result.Success {
			t.Fatalf("snapshot_interactive failed: err=%v", err)
		}

		result, err = br.Execute(context.Background(), map[string]any{
			"action": "scroll", "ref": float64(1),
		})
		if err != nil || !result.Success {
			t.Fatalf("legacy scroll action failed: err=%v success=%v", err, result.Success)
		}
		if mock.lastActMode != "interactive" {
			t.Fatalf("lastActMode = %q, want interactive", mock.lastActMode)
		}
		if mock.lastActAction != "scroll" {
			t.Fatalf("lastActAction = %q, want scroll", mock.lastActAction)
		}
		if mock.lastActRef != 1 {
			t.Fatalf("lastActRef = %d, want 1", mock.lastActRef)
		}
	})

	t.Run("act_no_snapshot", func(t *testing.T) {
		fresh := NewBrowser()
		fresh.SetBrowserService(mock)
		result, err := fresh.Execute(context.Background(), map[string]any{
			"action": "act", "ref": float64(1), "act_type": "click",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Success {
			t.Error("expected failure when no snapshot loaded")
		}
	})

	t.Run("screenshot", func(t *testing.T) {
		mock.screenshotData = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="
		mock.lastScreenshotURL = ""
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "screenshot", "url": "https://example.com",
		})
		if err != nil || !result.Success {
			t.Fatalf("screenshot failed: err=%v", err)
		}
		got, _ := result.Data.(map[string]any)["screenshot"].(string)
		if got == "" {
			t.Fatal("expected screenshot path")
		}
		if !filepath.IsAbs(got) {
			t.Fatalf("screenshot = %q, want absolute path", got)
		}
		if _, err := os.Stat(got); err != nil {
			t.Fatalf("expected saved screenshot at %s: %v", got, err)
		}
		if mock.lastScreenshotURL != "https://example.com" {
			t.Fatalf("lastScreenshotURL = %q, want https://example.com", mock.lastScreenshotURL)
		}
		mock.screenshotData = "base64data"
	})

	t.Run("screenshot_target_id", func(t *testing.T) {
		mock.screenshotTabData = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="
		mock.lastScreenshotURL = ""
		mock.lastScreenshotTab = ""
		result, err := br.Execute(context.Background(), map[string]any{
			"action":    "screenshot",
			"target_id": "tab-7",
		})
		if err != nil || !result.Success {
			t.Fatalf("screenshot target_id failed: err=%v", err)
		}
		if mock.lastScreenshotURL != "" {
			t.Fatalf("lastScreenshotURL = %q, want empty", mock.lastScreenshotURL)
		}
		if mock.lastScreenshotTab != "tab-7" {
			t.Fatalf("lastScreenshotTab = %q, want tab-7", mock.lastScreenshotTab)
		}
		if got := result.Data.(map[string]any)["target_id"]; got != "tab-7" {
			t.Fatalf("target_id = %v, want tab-7", got)
		}
		mock.screenshotTabData = ""
	})

	t.Run("screenshot_active_tab", func(t *testing.T) {
		mock.screenshotTabData = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="
		mock.lastScreenshotURL = ""
		mock.lastScreenshotTab = "non-empty"
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "screenshot",
		})
		if err != nil || !result.Success {
			t.Fatalf("screenshot active tab failed: err=%v", err)
		}
		if mock.lastScreenshotURL != "" {
			t.Fatalf("lastScreenshotURL = %q, want empty", mock.lastScreenshotURL)
		}
		if mock.lastScreenshotTab != "" {
			t.Fatalf("lastScreenshotTab = %q, want empty for active-tab fallback", mock.lastScreenshotTab)
		}
		if got := result.Data.(map[string]any)["message"]; got != "Screenshot captured for active tab" {
			t.Fatalf("message = %v, want active-tab message", got)
		}
		mock.screenshotTabData = ""
	})

	t.Run("screenshot_persists_file_when_media_dir_configured", func(t *testing.T) {
		const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="
		mock.screenshotData = pngBase64
		fresh := NewBrowser()
		fresh.SetBrowserService(mock)
		mediaDir := t.TempDir()
		fresh.SetMediaDir(mediaDir)

		result, err := fresh.Execute(context.Background(), map[string]any{
			"action": "screenshot", "url": "https://example.com",
		})
		if err != nil || !result.Success {
			t.Fatalf("screenshot failed: err=%v", err)
		}
		got, _ := result.Data.(map[string]any)["screenshot"].(string)
		if got == "" {
			t.Fatal("expected screenshot file path")
		}
		if !filepath.IsAbs(got) {
			t.Fatalf("screenshot = %q, want absolute path", got)
		}
		if !strings.HasPrefix(got, filepath.Join(mediaDir, "browser")+string(os.PathSeparator)) {
			t.Fatalf("screenshot = %q, want under media dir %q", got, filepath.Join(mediaDir, "browser"))
		}
		if _, err := os.Stat(got); err != nil {
			t.Fatalf("expected saved screenshot at %s: %v", got, err)
		}
		mock.screenshotData = ""
	})

	t.Run("tabs", func(t *testing.T) {
		// Navigate first to populate tabs
		br.Execute(context.Background(), map[string]any{
			"action": "navigate", "url": "https://example.com",
		})
		result, err := br.Execute(context.Background(), map[string]any{"action": "tabs"})
		if err != nil || !result.Success {
			t.Fatalf("tabs failed: err=%v", err)
		}
	})

	t.Run("close", func(t *testing.T) {
		// Set lastTarget so close has a target
		br.mu.Lock()
		br.lastTarget = "tab-1"
		br.mu.Unlock()

		result, err := br.Execute(context.Background(), map[string]any{"action": "close"})
		if err != nil || !result.Success {
			t.Fatalf("close failed: err=%v", err)
		}
	})

	t.Run("close_no_target", func(t *testing.T) {
		fresh := NewBrowser()
		fresh.SetBrowserService(mock)
		result, _ := fresh.Execute(context.Background(), map[string]any{"action": "close"})
		if result.Success {
			t.Error("expected failure when no target")
		}
	})

	t.Run("no_service", func(t *testing.T) {
		noSvc := NewBrowser()
		result, _ := noSvc.Execute(context.Background(), map[string]any{
			"action": "navigate", "url": "https://example.com",
		})
		if result.Success {
			t.Error("expected failure when service is nil")
		}
	})

	t.Run("snapshot_auto_simple_page", func(t *testing.T) {
		// interactiveCount=5 (≤30) → should use interactive strategy
		mock.interactiveCount = 5
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "snapshot_auto",
		})
		if err != nil || !result.Success {
			t.Fatalf("snapshot_auto failed: err=%v", err)
		}
		data := result.Data.(map[string]any)
		if data["strategy"] != "interactive" {
			t.Errorf("expected strategy 'interactive', got '%v'", data["strategy"])
		}
	})

	t.Run("snapshot_auto_complex_with_vision", func(t *testing.T) {
		// interactiveCount=50 (>30) + vision=true → screenshot+interactive
		mock.interactiveCount = 50
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "snapshot_auto", "vision": true,
		})
		if err != nil || !result.Success {
			t.Fatalf("snapshot_auto failed: err=%v", err)
		}
		data := result.Data.(map[string]any)
		if data["strategy"] != "screenshot+interactive" {
			t.Errorf("expected strategy 'screenshot+interactive', got '%v'", data["strategy"])
		}
		if data["screenshot"] == nil {
			t.Error("expected screenshot data")
		}
		if data["tree"] == nil {
			t.Error("expected interactive tree")
		}
	})

	t.Run("snapshot_auto_complex_no_vision", func(t *testing.T) {
		// interactiveCount=50 (>30) + vision=false → tries a11y tree
		// Mock a11y tree is short (<6000) so it should use a11y
		mock.interactiveCount = 50
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "snapshot_auto", "vision": false,
		})
		if err != nil || !result.Success {
			t.Fatalf("snapshot_auto failed: err=%v", err)
		}
		data := result.Data.(map[string]any)
		if data["strategy"] != "a11y" {
			t.Errorf("expected strategy 'a11y', got '%v'", data["strategy"])
		}
	})

	t.Run("navigate_uses_auto", func(t *testing.T) {
		// Navigate should use auto-snapshot
		mock.interactiveCount = 5
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "navigate", "url": "https://example.com",
		})
		if err != nil || !result.Success {
			t.Fatalf("navigate failed: err=%v", err)
		}
		data := result.Data.(map[string]any)
		if data["strategy"] != "interactive" {
			t.Errorf("expected strategy 'interactive' from navigate auto, got '%v'", data["strategy"])
		}
	})

	t.Run("snapshot_auto_docs_page_adds_readable_content", func(t *testing.T) {
		mock.interactiveCount = 66
		mock.extractedTextBySelector = map[string]string{
			"main": "Create a model response POST /responses. Get a model response GET /responses/{response_id}. Delete a model response DELETE /responses/{response_id}. This reference explains request fields, authentication, response objects, streaming events, and related examples with enough readable detail for the browser fallback to stop looping on navigation chrome alone.",
			"h1":   "Responses",
		}
		mock.recipeData = map[string]interface{}{
			"extracted": map[string]interface{}{
				"page_heading": "Responses",
				"main_content": "Create a model response POST /responses. Get a model response GET /responses/{response_id}. Delete a model response DELETE /responses/{response_id}. This reference explains request fields, authentication, response objects, streaming events, and related examples with enough readable detail for the browser fallback to stop looping on navigation chrome alone.",
			},
		}
		defer func() {
			mock.extractedTextBySelector = nil
			mock.recipeData = nil
		}()
		mock.tabs = []BrowserTabInfo{{
			TargetID: "tab-1",
			URL:      "https://developers.openai.com/api/reference/resources/responses",
			Title:    "Responses | OpenAI API Reference",
			Active:   true,
		}}
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "snapshot_auto",
		})
		if err != nil || !result.Success {
			t.Fatalf("snapshot_auto failed: err=%v", err)
		}
		data := result.Data.(map[string]any)
		content, _ := data["content"].(string)
		if !strings.Contains(content, "Create a model response POST /responses") {
			t.Fatalf("content = %q, want extracted documentation body", content)
		}
		if data["content_strategy"] != "extract_recipe" {
			t.Fatalf("content_strategy = %v, want extract_recipe", data["content_strategy"])
		}
		message, _ := data["message"].(string)
		if !strings.Contains(message, "Main content:") {
			t.Fatalf("message = %q, want readable content section", message)
		}
	})
}
