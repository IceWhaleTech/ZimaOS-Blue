package mediagen

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/labstack/echo/v4"
)

func newTestMediaHandler() *Handler {
	manager := NewManager(nil, nil, "")
	return NewHandler(manager, nil, "")
}

func TestHandlerGenerateImageNoProviderReturnsActionableError(t *testing.T) {
	h := newTestMediaHandler()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/images/generations", bytes.NewBufferString(`{"prompt":"draw a cat"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GenerateImage(c); err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "no media provider is configured and enabled" {
		t.Fatalf("error = %q", body["error"])
	}
	if body["hint"] == "" {
		t.Fatal("expected hint in response")
	}
}

func TestHandlerDirectGenerateNoProviderReturnsActionableError(t *testing.T) {
	h := newTestMediaHandler()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/generate", bytes.NewBufferString(`{"category":"t2i","prompt":"draw a cat"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.DirectGenerate(c); err != nil {
		t.Fatalf("DirectGenerate returned error: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "no media provider is configured and enabled" {
		t.Fatalf("error = %q", body["error"])
	}
	if body["hint"] == "" {
		t.Fatal("expected hint in response")
	}
}

func TestHandlerUpdateProviderPersistsPriorityAndBaseURL(t *testing.T) {
	store := NewConfigStore(t.TempDir())
	manager := NewManager(nil, store, "")
	manager.InitConfigs()
	h := NewHandler(manager, nil, "")
	e := echo.New()

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/media/providers/gemini-image",
		strings.NewReader(`{"base_url":"https://media.example.com/v2","priority":5}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/media/providers/:id")
	c.SetParamNames("id")
	c.SetParamValues("gemini-image")

	if err := h.UpdateProvider(c); err != nil {
		t.Fatalf("UpdateProvider returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var updated MediaProviderConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.BaseURL != "https://media.example.com/v2" {
		t.Fatalf("base_url = %q, want %q", updated.BaseURL, "https://media.example.com/v2")
	}
	if updated.Priority != 5 {
		t.Fatalf("priority = %d, want %d", updated.Priority, 5)
	}

	current := manager.GetConfig("gemini-image")
	if current == nil {
		t.Fatal("expected manager config for gemini-image")
	}
	if current.BaseURL != "https://media.example.com/v2" {
		t.Fatalf("manager base_url = %q, want %q", current.BaseURL, "https://media.example.com/v2")
	}
	if current.Priority != 5 {
		t.Fatalf("manager priority = %d, want %d", current.Priority, 5)
	}

	reloaded := NewManager(nil, store, "")
	reloaded.InitConfigs()
	reloadedCfg := reloaded.GetConfig("gemini-image")
	if reloadedCfg == nil {
		t.Fatal("expected reloaded config for gemini-image")
	}
	if reloadedCfg.BaseURL != "https://media.example.com/v2" {
		t.Fatalf("reloaded base_url = %q, want %q", reloadedCfg.BaseURL, "https://media.example.com/v2")
	}
	if reloadedCfg.Priority != 5 {
		t.Fatalf("reloaded priority = %d, want %d", reloadedCfg.Priority, 5)
	}
}

func TestHandlerGetTaskRejectsCrossUserAccess(t *testing.T) {
	manager := NewManager(nil, nil, "")
	manager.tasks.Store("task-user-a", &MediaTask{
		BaseTask: basetask.BaseTask{ID: "task-user-a", Status: TaskStatusPending, CreatedAt: time.Now()},
		UserID:   "user-a",
		Type:     MediaTypeImage,
	})
	h := NewHandler(manager, nil, "")
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/media/tasks/task-user-a", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-b", Role: "user"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/media/tasks/:id")
	c.SetParamNames("id")
	c.SetParamValues("task-user-a")

	if err := h.GetTask(c); err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerCancelTaskRejectsCrossUserAccess(t *testing.T) {
	manager := NewManager(nil, nil, "")
	manager.tasks.Store("task-user-a", &MediaTask{
		BaseTask: basetask.BaseTask{ID: "task-user-a", Status: TaskStatusProcessing, CreatedAt: time.Now()},
		UserID:   "user-a",
		Type:     MediaTypeImage,
	})
	h := NewHandler(manager, nil, "")
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/tasks/task-user-a/cancel", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-b", Role: "user"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/media/tasks/:id/cancel")
	c.SetParamNames("id")
	c.SetParamValues("task-user-a")

	if err := h.CancelTask(c); err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}

	task, err := manager.GetTask("task-user-a", "user-a")
	if err != nil {
		t.Fatalf("owner lookup failed: %v", err)
	}
	if task.Status != TaskStatusProcessing {
		t.Fatalf("status = %q, want %q", task.Status, TaskStatusProcessing)
	}
}

func TestHandlerRetryTaskRejectsCrossUserAccess(t *testing.T) {
	manager := NewManager(nil, nil, "")
	manager.tasks.Store("task-user-a", &MediaTask{
		BaseTask: basetask.BaseTask{ID: "task-user-a", Status: TaskStatusFailed, CreatedAt: time.Now()},
		UserID:   "user-a",
		Type:     MediaTypeImage,
		Model:    "fake-model",
		Request: &MediaRequest{
			Type:   MediaTypeImage,
			Model:  "fake-model",
			Prompt: "draw cat",
		},
	})
	h := NewHandler(manager, nil, "")
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/tasks/task-user-a/retry", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-b", Role: "user"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/media/tasks/:id/retry")
	c.SetParamNames("id")
	c.SetParamValues("task-user-a")

	if err := h.RetryTask(c); err != nil {
		t.Fatalf("RetryTask returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerGetTaskByMessageScopesToUser(t *testing.T) {
	db := newTestDB(t)
	store, err := NewTaskStore(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, userID string
	}{
		{id: "task-a", userID: "user-a"},
		{id: "task-b", userID: "user-b"},
	} {
		if err := store.Create(&PersistentTask{
			ID:        tc.id,
			UserID:    tc.userID,
			MessageID: "msg-1",
			Status:    TaskStatusPending,
			Type:      MediaTypeImage,
			Category:  "t2i",
		}); err != nil {
			t.Fatal(err)
		}
	}

	manager := NewManager(nil, nil, "")
	manager.taskStore = store
	h := NewHandler(manager, nil, "")
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/media/tasks/by-message/msg-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-b", Role: "user"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/media/tasks/by-message/:message_id")
	c.SetParamNames("message_id")
	c.SetParamValues("msg-1")

	if err := h.GetTaskByMessage(c); err != nil {
		t.Fatalf("GetTaskByMessage returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Tasks []MediaTask `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Tasks) != 1 || body.Tasks[0].ID != "task-b" {
		t.Fatalf("tasks = %#v, want only task-b", body.Tasks)
	}
}

func TestHandlerGetMediaStatsScopesToUser(t *testing.T) {
	db := newTestDB(t)
	store, err := NewTaskStore(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, userID string
	}{
		{id: "s-a", userID: "user-a"},
		{id: "s-b", userID: "user-b"},
	} {
		pt := &PersistentTask{
			ID:       tc.id,
			UserID:   tc.userID,
			Status:   TaskStatusSucceeded,
			Type:     MediaTypeImage,
			Category: "t2i",
			Model:    "m1",
			Response: `{"created":1,"data":[{"url":"http://x"}]}`,
		}
		if err := store.Create(pt); err != nil {
			t.Fatal(err)
		}
		if err := store.UpdateStatus(tc.id, TaskStatusSucceeded, 1, "", pt.Response); err != nil {
			t.Fatal(err)
		}
	}

	manager := NewManager(nil, nil, "")
	manager.taskStore = store
	h := NewHandler(manager, nil, "")
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/media/stats", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-b", Role: "user"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetMediaStats(c); err != nil {
		t.Fatalf("GetMediaStats returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var stats MediaStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if stats.TotalTasks != 1 || stats.Succeeded != 1 {
		t.Fatalf("stats = %#v, want one succeeded task for user-b", stats)
	}
}

func TestHandlerGetFallbackModelStatus(t *testing.T) {
	manager := NewManager(nil, nil, "")
	engine := NewFallbackEngine(FallbackConfig{
		Enabled:         true,
		DataDir:         t.TempDir(),
		U2NetPStatusURL: "/api/v1/media/fallback/models/u2netp/status",
	}, nil, nil, func() FallbackBrowserService { return nil }, "")
	manager.SetFallbackEngine(engine)
	handler := NewHandler(manager, nil, "")
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/media/fallback/models/u2netp/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/media/fallback/models/:id/status")
	c.SetParamNames("id")
	c.SetParamValues("u2netp")

	if err := handler.GetFallbackModelStatus(c); err != nil {
		t.Fatalf("GetFallbackModelStatus returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["model_id"] != "u2netp" {
		t.Fatalf("model_id = %#v, want u2netp", body["model_id"])
	}
	if body["state"] != "not_downloaded" {
		t.Fatalf("state = %#v, want not_downloaded", body["state"])
	}
}
