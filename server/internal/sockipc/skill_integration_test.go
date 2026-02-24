package sockipc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// setupAllSkills creates a server with all 5 SKILL handlers registered + ping.
// No token auth — Unix socket permissions are the auth boundary.
func setupAllSkills(t *testing.T) (net.Conn, func()) {
	t.Helper()

	// Media backend
	gen := func(_ context.Context, category, model, prompt string, params map[string]string) (string, error) {
		return "media-task-1", nil
	}
	query := func(_ context.Context, taskID string) (map[string]string, error) {
		if taskID == "media-task-1" {
			return map[string]string{
				"task_id":     "media-task-1",
				"task_status": "succeeded",
				"progress":    "1.00",
			}, nil
		}
		return nil, nil
	}

	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	log := zap.NewNop()

	RegisterMediaHandlers(srv, gen, query, log)
	RegisterBrowserHandlers(srv, &mockBrowser{}, log)
	RegisterUIReviewHandlers(srv, &mockUIReviewer{}, log)
	RegisterPushHandlers(srv, &mockPush{}, log)
	RegisterSkillManagerHandlers(srv, newMockSkillManager(), log)

	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}

	return conn, func() { conn.Close(); srv.Close() }
}

// --- Mock backends ---

type mockBrowser struct{}

func (m *mockBrowser) Start(_ context.Context) error { return nil }
func (m *mockBrowser) Navigate(_ context.Context, url, targetID string) (map[string]string, error) {
	return map[string]string{"title": "Test Page", "url": url, "target_id": "tab-1"}, nil
}
func (m *mockBrowser) AccessibilityTree(_ context.Context, targetID string, maxDepth int) (map[string]string, error) {
	return map[string]string{"tree": "[document] Test Page\n  [heading] Hello", "target_id": targetID}, nil
}
func (m *mockBrowser) InteractiveElements(_ context.Context, targetID string) (map[string]string, error) {
	return map[string]string{"elements": `[{"ref":1,"role":"button","name":"Submit"}]`, "target_id": targetID}, nil
}
func (m *mockBrowser) Screenshot(_ context.Context, url string) (string, error) {
	return "base64-png-data", nil
}
func (m *mockBrowser) ScreenshotTab(_ context.Context, targetID string) (string, error) {
	return "base64-tab-png", nil
}
func (m *mockBrowser) Act(_ context.Context, targetID string, ref int, actType, value string) error {
	return nil
}
func (m *mockBrowser) Tabs(_ context.Context) (string, error) {
	return `[{"target_id":"tab-1","title":"Test","url":"https://example.com"}]`, nil
}
func (m *mockBrowser) CloseTab(_ context.Context, targetID string) error { return nil }

type mockUIReviewer struct{}

func (m *mockUIReviewer) ReviewURL(_ context.Context, url, lang, device string) (string, error) {
	return fmt.Sprintf(`{"score":85,"url":"%s","lang":"%s"}`, url, lang), nil
}
func (m *mockUIReviewer) ReviewImage(_ context.Context, imageBase64, lang string) (string, error) {
	return `{"score":90,"type":"image"}`, nil
}
func (m *mockUIReviewer) CheckAccessibility(_ context.Context, url, lang string) (string, error) {
	return `{"violations":0,"url":"` + url + `"}`, nil
}

type mockPush struct{ items []PushResult }

func (m *mockPush) Add(_ context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (PushResult, error) {
	r := PushResult{ID: "push-1", Message: message, FireAt: fireAt, Recurring: recurring, SessionID: sessionID, Status: "pending", CreatedAt: time.Now()}
	m.items = append(m.items, r)
	return r, nil
}
func (m *mockPush) List(_ context.Context, ownerID string) ([]PushResult, error) {
	return m.items, nil
}
func (m *mockPush) Delete(_ context.Context, ownerID, id string) error {
	for i, item := range m.items {
		if item.ID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("not found")
}
func (m *mockPush) Clear(_ context.Context, ownerID string) (int64, error) {
	n := int64(len(m.items))
	m.items = nil
	return n, nil
}

// ============================================================
// Integration tests — no token auth, just command + params
// ============================================================

func TestSkillIntegration_Ping(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "ping"})
	if resp.Status != "ok" {
		t.Fatalf("ping: status=%q error=%q", resp.Status, resp.Error)
	}
}

// --- Mediagen SKILL ---

func TestSkillIntegration_MediaGenerate(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{
		Cmd:    "media.generate",
		Params: map[string]string{"prompt": "a sunset over mountains", "category": "t2i", "model": "flux-pro", "size": "1024x1024"},
	})
	if resp.Status != "ok" {
		t.Fatalf("media.generate: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["task_id"] != "media-task-1" {
		t.Errorf("task_id = %q", resp.Data["task_id"])
	}
}

func TestSkillIntegration_MediaStatus(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "media.status", Params: map[string]string{"task_id": "media-task-1"}})
	if resp.Status != "ok" {
		t.Fatalf("media.status: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["task_status"] != "succeeded" {
		t.Errorf("task_status = %q", resp.Data["task_status"])
	}
}

// --- Browser SKILL ---

func TestSkillIntegration_BrowserNavigate(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "browser.navigate", Params: map[string]string{"url": "https://example.com"}})
	if resp.Status != "ok" {
		t.Fatalf("browser.navigate: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["title"] != "Test Page" {
		t.Errorf("title = %q", resp.Data["title"])
	}
}

func TestSkillIntegration_BrowserSnapshot(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "browser.snapshot", Params: map[string]string{"target_id": "tab-1", "max_depth": "5"}})
	if resp.Status != "ok" {
		t.Fatalf("browser.snapshot: status=%q error=%q", resp.Status, resp.Error)
	}
	if !strings.Contains(resp.Data["tree"], "Hello") {
		t.Errorf("tree = %q", resp.Data["tree"])
	}
}

func TestSkillIntegration_BrowserAct(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "browser.act", Params: map[string]string{"target_id": "tab-1", "ref": "1", "act_type": "click"}})
	if resp.Status != "ok" {
		t.Fatalf("browser.act: status=%q error=%q", resp.Status, resp.Error)
	}
	if !strings.Contains(resp.Data["message"], "click") {
		t.Errorf("message = %q", resp.Data["message"])
	}
}

func TestSkillIntegration_BrowserScreenshot(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "browser.screenshot", Params: map[string]string{"url": "https://example.com"}})
	if resp.Status != "ok" {
		t.Fatalf("browser.screenshot: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["screenshot"] != "base64-png-data" {
		t.Errorf("screenshot = %q", resp.Data["screenshot"])
	}
}

func TestSkillIntegration_BrowserTabs(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "browser.tabs"})
	if resp.Status != "ok" {
		t.Fatalf("browser.tabs: status=%q error=%q", resp.Status, resp.Error)
	}
	if !strings.Contains(resp.Data["tabs"], "tab-1") {
		t.Errorf("tabs = %q", resp.Data["tabs"])
	}
}

func TestSkillIntegration_BrowserClose(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "browser.close", Params: map[string]string{"target_id": "tab-1"}})
	if resp.Status != "ok" {
		t.Fatalf("browser.close: status=%q error=%q", resp.Status, resp.Error)
	}
}

// --- UI Reviewer SKILL ---

func TestSkillIntegration_UIReviewURL(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "ui.review_url", Params: map[string]string{"url": "https://example.com", "lang": "zh-CN"}})
	if resp.Status != "ok" {
		t.Fatalf("ui.review_url: status=%q error=%q", resp.Status, resp.Error)
	}
	if !strings.Contains(resp.Data["result"], "score") {
		t.Errorf("result = %q", resp.Data["result"])
	}
	if !strings.Contains(resp.Data["result"], "zh-CN") {
		t.Errorf("lang not passed through: result = %q", resp.Data["result"])
	}
}

func TestSkillIntegration_UIReviewImage(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "ui.review_image", Params: map[string]string{"image": "base64-image-data"}})
	if resp.Status != "ok" {
		t.Fatalf("ui.review_image: status=%q error=%q", resp.Status, resp.Error)
	}
	if !strings.Contains(resp.Data["result"], "image") {
		t.Errorf("result = %q", resp.Data["result"])
	}
}

func TestSkillIntegration_UICheckAccessibility(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "ui.check_accessibility", Params: map[string]string{"url": "https://example.com"}})
	if resp.Status != "ok" {
		t.Fatalf("ui.check_accessibility: status=%q error=%q", resp.Status, resp.Error)
	}
	if !strings.Contains(resp.Data["result"], "violations") {
		t.Errorf("result = %q", resp.Data["result"])
	}
}

// --- Push Notification SKILL ---

func TestSkillIntegration_PushAdd(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "push.add", Params: map[string]string{"message": "Remember to check the build", "time": "1h"}})
	if resp.Status != "ok" {
		t.Fatalf("push.add: status=%q error=%q", resp.Status, resp.Error)
	}
	if !strings.Contains(resp.Data["notification"], "push-1") {
		t.Errorf("notification = %q", resp.Data["notification"])
	}
}

func TestSkillIntegration_PushList(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	sendRecv(t, conn, &Request{Cmd: "push.add", Params: map[string]string{"message": "test notification", "time": "30m"}})

	resp := sendRecv(t, conn, &Request{Cmd: "push.list"})
	if resp.Status != "ok" {
		t.Fatalf("push.list: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["count"] != "1" {
		t.Errorf("count = %q, want 1", resp.Data["count"])
	}
}

func TestSkillIntegration_PushDelete(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	sendRecv(t, conn, &Request{Cmd: "push.add", Params: map[string]string{"message": "to be deleted", "time": "1h"}})

	resp := sendRecv(t, conn, &Request{Cmd: "push.delete", Params: map[string]string{"id": "push-1"}})
	if resp.Status != "ok" {
		t.Fatalf("push.delete: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["deleted"] != "push-1" {
		t.Errorf("deleted = %q", resp.Data["deleted"])
	}
}

func TestSkillIntegration_PushClear(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	for i := 0; i < 2; i++ {
		sendRecv(t, conn, &Request{Cmd: "push.add", Params: map[string]string{"message": fmt.Sprintf("notification %d", i), "time": "1h"}})
	}

	resp := sendRecv(t, conn, &Request{Cmd: "push.clear"})
	if resp.Status != "ok" {
		t.Fatalf("push.clear: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["cleared"] != "2" {
		t.Errorf("cleared = %q, want 2", resp.Data["cleared"])
	}
}

// --- Skill Manager SKILL ---

func TestSkillIntegration_SkillSearch(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.search", Params: map[string]string{"query": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("skill.search: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["result"] == "" {
		t.Error("empty result")
	}
}

func TestSkillIntegration_SkillInstallAndUninstall(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	// Install
	resp := sendRecv(t, conn, &Request{Cmd: "skill.install", Params: map[string]string{"id": "new-skill"}})
	if resp.Status != "ok" {
		t.Fatalf("skill.install: status=%q error=%q", resp.Status, resp.Error)
	}

	// Uninstall
	resp = sendRecv(t, conn, &Request{Cmd: "skill.uninstall", Params: map[string]string{"id": "new-skill"}})
	if resp.Status != "ok" {
		t.Fatalf("skill.uninstall: status=%q error=%q", resp.Status, resp.Error)
	}
}

func TestSkillIntegration_SkillEnableDisable(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.enable", Params: map[string]string{"id": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("skill.enable: status=%q error=%q", resp.Status, resp.Error)
	}

	resp = sendRecv(t, conn, &Request{Cmd: "skill.disable", Params: map[string]string{"id": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("skill.disable: status=%q error=%q", resp.Status, resp.Error)
	}
}

func TestSkillIntegration_SkillList(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.list"})
	if resp.Status != "ok" {
		t.Fatalf("skill.list: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["skills"] == "" {
		t.Error("empty skills list")
	}
}

func TestSkillIntegration_SkillInfo(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "skill.info", Params: map[string]string{"id": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("skill.info: status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["skill"] == "" {
		t.Error("empty skill info")
	}
}

// --- Cross-SKILL: full flow on single connection ---

func TestSkillIntegration_FullFlowSingleConn(t *testing.T) {
	conn, cleanup := setupAllSkills(t)
	defer cleanup()

	// 1. Ping
	resp := sendRecv(t, conn, &Request{Cmd: "ping"})
	if resp.Status != "ok" {
		t.Fatalf("ping failed")
	}

	// 2. Media generate
	resp = sendRecv(t, conn, &Request{Cmd: "media.generate", Params: map[string]string{"prompt": "a dragon", "category": "t2v"}})
	if resp.Data["task_id"] != "media-task-1" {
		t.Fatalf("media.generate task_id = %q", resp.Data["task_id"])
	}

	// 3. Media status
	resp = sendRecv(t, conn, &Request{Cmd: "media.status", Params: map[string]string{"task_id": "media-task-1"}})
	if resp.Data["task_status"] != "succeeded" {
		t.Fatalf("media.status = %q", resp.Data["task_status"])
	}

	// 4. Browser navigate
	resp = sendRecv(t, conn, &Request{Cmd: "browser.navigate", Params: map[string]string{"url": "https://example.com"}})
	if resp.Data["title"] != "Test Page" {
		t.Fatalf("browser.navigate title = %q", resp.Data["title"])
	}

	// 5. UI review
	resp = sendRecv(t, conn, &Request{Cmd: "ui.review_url", Params: map[string]string{"url": "https://example.com"}})
	if !strings.Contains(resp.Data["result"], "score") {
		t.Fatalf("ui.review_url result = %q", resp.Data["result"])
	}

	// 6. Push add
	resp = sendRecv(t, conn, &Request{Cmd: "push.add", Params: map[string]string{"message": "test", "time": "1h"}})
	if resp.Status != "ok" {
		t.Fatalf("push.add failed: %s", resp.Error)
	}

	// 7. Push list
	resp = sendRecv(t, conn, &Request{Cmd: "push.list"})
	if resp.Data["count"] != "1" {
		t.Fatalf("push.list count = %q", resp.Data["count"])
	}

	// 8. Skill search
	resp = sendRecv(t, conn, &Request{Cmd: "skill.search", Params: map[string]string{"query": "weather"}})
	if resp.Status != "ok" {
		t.Fatalf("skill.search failed: %s", resp.Error)
	}

	// 9. Skill list
	resp = sendRecv(t, conn, &Request{Cmd: "skill.list"})
	if resp.Status != "ok" {
		t.Fatalf("skill.list failed: %s", resp.Error)
	}
}
