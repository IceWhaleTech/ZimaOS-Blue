package builtin

import (
	"context"
	"fmt"
	"testing"
)

// ============================================================
// Mock implementations
// ============================================================

// --- Mock CronService ---

type mockCronService struct {
	jobs map[string]CronJobInfo
}

func newMockCronService() *mockCronService {
	return &mockCronService{jobs: make(map[string]CronJobInfo)}
}

func (m *mockCronService) Create(name, description, schedule, handler string, payload map[string]interface{}) (CronJobInfo, error) {
	return m.CreateForOwner("", name, description, schedule, handler, payload)
}

func (m *mockCronService) CreateForOwner(ownerID, name, description, schedule, handler string, payload map[string]interface{}) (CronJobInfo, error) {
	id := fmt.Sprintf("job-%d", len(m.jobs)+1)
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

// --- Mock WorkflowService ---

type mockWorkflowService struct {
	wfs map[string]WorkflowInfo
}

func newMockWorkflowService() *mockWorkflowService {
	return &mockWorkflowService{wfs: make(map[string]WorkflowInfo)}
}

func (m *mockWorkflowService) CreateWorkflow(_ context.Context, name, description, status string) (WorkflowInfo, error) {
	id := fmt.Sprintf("wf-%d", len(m.wfs)+1)
	wf := WorkflowInfo{ID: id, Name: name, Description: description, Status: status, Version: 1}
	m.wfs[id] = wf
	return wf, nil
}

func (m *mockWorkflowService) ListWorkflows(_ context.Context) ([]WorkflowInfo, error) {
	var out []WorkflowInfo
	for _, wf := range m.wfs {
		out = append(out, wf)
	}
	return out, nil
}

func (m *mockWorkflowService) GetWorkflow(_ context.Context, id string) (WorkflowInfo, error) {
	wf, ok := m.wfs[id]
	if !ok {
		return WorkflowInfo{}, fmt.Errorf("workflow not found: %s", id)
	}
	return wf, nil
}

func (m *mockWorkflowService) DeleteWorkflow(_ context.Context, id string) error {
	if _, ok := m.wfs[id]; !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}
	delete(m.wfs, id)
	return nil
}

func (m *mockWorkflowService) EnableWorkflow(_ context.Context, id string) error {
	wf, ok := m.wfs[id]
	if !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}
	wf.Status = "active"
	m.wfs[id] = wf
	return nil
}

func (m *mockWorkflowService) DisableWorkflow(_ context.Context, id string) error {
	wf, ok := m.wfs[id]
	if !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}
	wf.Status = "disabled"
	m.wfs[id] = wf
	return nil
}

func (m *mockWorkflowService) ExecuteWorkflow(_ context.Context, id string, _ map[string]interface{}) (WorkflowExecutionInfo, error) {
	wf, ok := m.wfs[id]
	if !ok {
		return WorkflowExecutionInfo{}, fmt.Errorf("workflow not found: %s", id)
	}
	return WorkflowExecutionInfo{
		ID: "exec-1", WorkflowID: id, WorkflowName: wf.Name, Status: "completed",
	}, nil
}

// --- Mock SandboxService ---

type mockSandboxService struct {
	supported bool
	results   map[string]SandboxResultInfo
}

func newMockSandboxService() *mockSandboxService {
	return &mockSandboxService{
		supported: true,
		results:   make(map[string]SandboxResultInfo),
	}
}

func (m *mockSandboxService) Execute(_ context.Context, command string, _ []string, _ string, _ int) (SandboxResultInfo, error) {
	r := SandboxResultInfo{
		ID: "exec-1", Status: "completed", ExitCode: 0,
		Stdout: "hello", Duration: "100ms",
	}
	m.results[r.ID] = r
	return r, nil
}

func (m *mockSandboxService) GetStatus(id string) (SandboxResultInfo, error) {
	r, ok := m.results[id]
	if !ok {
		return SandboxResultInfo{}, fmt.Errorf("execution not found: %s", id)
	}
	return r, nil
}

func (m *mockSandboxService) Kill(id string) error {
	if _, ok := m.results[id]; !ok {
		return fmt.Errorf("execution not found: %s", id)
	}
	return nil
}

func (m *mockSandboxService) IsSupported() bool {
	return m.supported
}

// --- Mock BrowserService ---

type mockBrowserService struct {
	started          bool
	tabs             []BrowserTabInfo
	interactiveCount int // configurable for auto-snapshot tests
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

func (m *mockBrowserService) AccessibilityTree(_ context.Context, _ string, _ int) (BrowserA11yResult, error) {
	return BrowserA11yResult{
		Tree:     "@1 [button] \"Submit\"\n@2 [textbox] \"Search\"",
		URL:      "https://example.com",
		Title:    "Test Page",
		TargetID: "tab-1",
		RefMap:   map[int]int{1: 100, 2: 200},
	}, nil
}

func (m *mockBrowserService) InteractiveElements(_ context.Context, _ string) (BrowserInteractiveResult, error) {
	return BrowserInteractiveResult{
		Tree:     "@1 [button] \"Submit\"\n@2 [input] \"Search\" type=text",
		URL:      "https://example.com",
		Title:    "Test Page",
		TargetID: "tab-1",
		RefMap:   map[int]string{1: "__interactive_ref_0", 2: "__interactive_ref_1"},
		Count:    2,
	}, nil
}

func (m *mockBrowserService) CountInteractiveElements(_ context.Context, _ string) (int, error) {
	return m.interactiveCount, nil
}

func (m *mockBrowserService) ActByRef(_ context.Context, _ string, ref int, refMap map[int]int, _ string, _ string) error {
	if _, ok := refMap[ref]; !ok {
		return fmt.Errorf("unknown ref @%d", ref)
	}
	return nil
}

func (m *mockBrowserService) ActByInteractiveRef(_ context.Context, _ string, ref int, refMap map[int]string, _ string, _ string) error {
	if _, ok := refMap[ref]; !ok {
		return fmt.Errorf("unknown ref @%d", ref)
	}
	return nil
}

func (m *mockBrowserService) Screenshot(_ context.Context, _ string) (string, error) {
	return "base64data", nil
}

func (m *mockBrowserService) ScreenshotTab(_ context.Context, _ string) (string, error) {
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
		result, err := sched.Execute(context.Background(), map[string]any{
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

func TestWorkflows(t *testing.T) {
	wf := NewWorkflows()
	mock := newMockWorkflowService()
	wf.SetWorkflowService(mock)

	t.Run("manifest", func(t *testing.T) {
		if wf.Manifest().ID != "workflows" {
			t.Errorf("expected ID 'workflows', got '%s'", wf.Manifest().ID)
		}
	})

	t.Run("validate", func(t *testing.T) {
		if err := wf.Validate(map[string]any{}); err == nil {
			t.Error("expected error for missing action")
		}
		if err := wf.Validate(map[string]any{"action": "create"}); err == nil {
			t.Error("expected error for missing name")
		}
		if err := wf.Validate(map[string]any{"action": "get"}); err == nil {
			t.Error("expected error for missing id")
		}
		if err := wf.Validate(map[string]any{"action": "list"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("create_list_get", func(t *testing.T) {
		result, err := wf.Execute(context.Background(), map[string]any{
			"action": "create", "name": "my-workflow", "description": "test",
		})
		if err != nil || !result.Success {
			t.Fatalf("create failed: err=%v success=%v", err, result.Success)
		}

		result, err = wf.Execute(context.Background(), map[string]any{"action": "list"})
		if err != nil || !result.Success {
			t.Fatalf("list failed: err=%v", err)
		}
		if result.Data.(map[string]any)["count"] != 1 {
			t.Error("expected 1 workflow")
		}

		result, err = wf.Execute(context.Background(), map[string]any{"action": "get", "id": "wf-1"})
		if err != nil || !result.Success {
			t.Fatalf("get failed: err=%v", err)
		}
	})

	t.Run("enable_disable_execute", func(t *testing.T) {
		result, err := wf.Execute(context.Background(), map[string]any{"action": "enable", "id": "wf-1"})
		if err != nil || !result.Success {
			t.Fatalf("enable failed: err=%v", err)
		}

		result, err = wf.Execute(context.Background(), map[string]any{"action": "disable", "id": "wf-1"})
		if err != nil || !result.Success {
			t.Fatalf("disable failed: err=%v", err)
		}

		result, err = wf.Execute(context.Background(), map[string]any{"action": "execute", "id": "wf-1"})
		if err != nil || !result.Success {
			t.Fatalf("execute failed: err=%v", err)
		}
		exec := result.Data.(map[string]any)["execution"].(WorkflowExecutionInfo)
		if exec.Status != "completed" {
			t.Errorf("expected status 'completed', got '%s'", exec.Status)
		}
	})

	t.Run("delete", func(t *testing.T) {
		result, err := wf.Execute(context.Background(), map[string]any{"action": "delete", "id": "wf-1"})
		if err != nil || !result.Success {
			t.Fatalf("delete failed: err=%v", err)
		}
	})

	t.Run("no_service", func(t *testing.T) {
		noSvc := NewWorkflows()
		result, _ := noSvc.Execute(context.Background(), map[string]any{"action": "list"})
		if result.Success {
			t.Error("expected failure when service is nil")
		}
	})
}

func TestSandboxSkill(t *testing.T) {
	sb := NewSandbox()
	mock := newMockSandboxService()
	sb.SetSandboxService(mock)

	t.Run("manifest", func(t *testing.T) {
		if sb.Manifest().ID != "sandbox" {
			t.Errorf("expected ID 'sandbox', got '%s'", sb.Manifest().ID)
		}
	})

	t.Run("validate", func(t *testing.T) {
		if err := sb.Validate(map[string]any{}); err == nil {
			t.Error("expected error for missing action")
		}
		if err := sb.Validate(map[string]any{"action": "nope"}); err == nil {
			t.Error("expected error for invalid action")
		}
		if err := sb.Validate(map[string]any{"action": "execute"}); err == nil {
			t.Error("expected error for missing command")
		}
		if err := sb.Validate(map[string]any{"action": "status"}); err == nil {
			t.Error("expected error for missing id")
		}
		if err := sb.Validate(map[string]any{"action": "info"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("execute", func(t *testing.T) {
		result, err := sb.Execute(context.Background(), map[string]any{
			"action": "execute", "command": "echo", "args": []interface{}{"hello"},
		})
		if err != nil || !result.Success {
			t.Fatalf("execute failed: err=%v success=%v", err, result.Success)
		}
		r := result.Data.(map[string]any)["result"].(SandboxResultInfo)
		if r.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", r.ExitCode)
		}
	})

	t.Run("status", func(t *testing.T) {
		result, err := sb.Execute(context.Background(), map[string]any{
			"action": "status", "id": "exec-1",
		})
		if err != nil || !result.Success {
			t.Fatalf("status failed: err=%v", err)
		}
	})

	t.Run("kill", func(t *testing.T) {
		result, err := sb.Execute(context.Background(), map[string]any{
			"action": "kill", "id": "exec-1",
		})
		if err != nil || !result.Success {
			t.Fatalf("kill failed: err=%v", err)
		}
	})

	t.Run("info", func(t *testing.T) {
		result, err := sb.Execute(context.Background(), map[string]any{"action": "info"})
		if err != nil || !result.Success {
			t.Fatalf("info failed: err=%v", err)
		}
		if result.Data.(map[string]any)["supported"] != true {
			t.Error("expected supported=true")
		}
	})

	t.Run("timeout_cap", func(t *testing.T) {
		// Timeout > 300 should be capped
		result, err := sb.Execute(context.Background(), map[string]any{
			"action": "execute", "command": "sleep", "timeout": float64(999),
		})
		if err != nil || !result.Success {
			t.Fatalf("execute with high timeout failed: err=%v", err)
		}
	})

	t.Run("no_service", func(t *testing.T) {
		noSvc := NewSandbox()
		result, _ := noSvc.Execute(context.Background(), map[string]any{"action": "info"})
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
		result, err := br.Execute(context.Background(), map[string]any{
			"action": "screenshot", "url": "https://example.com",
		})
		if err != nil || !result.Success {
			t.Fatalf("screenshot failed: err=%v", err)
		}
		if result.Data.(map[string]any)["screenshot"] != "base64data" {
			t.Error("expected base64data")
		}
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
}
