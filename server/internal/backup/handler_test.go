package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

type restoreResponse struct {
	Success         bool            `json:"success"`
	Message         string          `json:"message"`
	RequiresRestart bool            `json:"requires_restart"`
	Restarting      bool            `json:"restarting"`
	PendingRestore  *PendingRestore `json:"pending_restore"`
	Result          *RestoreResult  `json:"result"`
}

func setupHandlerWithBackup(t *testing.T) (*Handler, *Manager, *BackupInfo, string) {
	t.Helper()

	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "state.txt"), []byte("v1"), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	mgr, err := NewManager(Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := mgr.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	handler := NewHandler(mgr)
	return handler, mgr, info, filepath.Join(dataDir, "state.txt")
}

func postRestore(t *testing.T, h *Handler, backupID string, payload any) (*httptest.ResponseRecorder, restoreResponse) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/backup/"+backupID+"/restore", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/backup/:id/restore")
	c.SetParamNames("id")
	c.SetParamValues(backupID)

	if err := h.Restore(c); err != nil {
		t.Fatalf("restore handler returned error: %v", err)
	}

	var resp restoreResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	return rec, resp
}

func TestHandlerRestore_DefaultStagesAndAutoRestarts(t *testing.T) {
	h, mgr, info, _ := setupHandlerWithBackup(t)

	restarted := make(chan struct{}, 1)
	h.SetRestartFunc(func() error {
		restarted <- struct{}{}
		return nil
	})

	rec, resp := postRestore(t, h, info.ID, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if !resp.RequiresRestart {
		t.Fatalf("expected requires_restart=true")
	}
	if !resp.Restarting {
		t.Fatalf("expected restarting=true")
	}
	if resp.PendingRestore == nil {
		t.Fatalf("expected pending_restore in response")
	}
	if !mgr.HasPendingRestore() {
		t.Fatalf("expected pending restore marker to exist")
	}

	select {
	case <-restarted:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected restart callback to be invoked")
	}
}

func TestHandlerRestore_StageWithoutAutoRestart(t *testing.T) {
	h, mgr, info, _ := setupHandlerWithBackup(t)

	restarted := make(chan struct{}, 1)
	h.SetRestartFunc(func() error {
		restarted <- struct{}{}
		return nil
	})

	rec, resp := postRestore(t, h, info.ID, map[string]any{
		"auto_restart": false,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if !resp.RequiresRestart {
		t.Fatalf("expected requires_restart=true")
	}
	if resp.Restarting {
		t.Fatalf("expected restarting=false")
	}
	if resp.PendingRestore == nil {
		t.Fatalf("expected pending_restore in response")
	}
	if !mgr.HasPendingRestore() {
		t.Fatalf("expected pending restore marker to exist")
	}

	select {
	case <-restarted:
		t.Fatalf("did not expect restart callback")
	case <-time.After(600 * time.Millisecond):
	}
}

func TestHandlerRestore_ImmediateRestorePath(t *testing.T) {
	h, mgr, info, statePath := setupHandlerWithBackup(t)

	if err := os.WriteFile(statePath, []byte("v2"), 0644); err != nil {
		t.Fatalf("failed to write updated state: %v", err)
	}

	rec, resp := postRestore(t, h, info.ID, map[string]any{
		"require_restart": false,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if resp.Result == nil || !resp.Result.Success {
		t.Fatalf("expected restore result in immediate restore response")
	}
	if resp.Result.CheckpointID == "" {
		t.Fatalf("expected checkpoint id in immediate restore result")
	}
	if mgr.HasPendingRestore() {
		t.Fatalf("did not expect pending restore marker in immediate restore path")
	}

	restored, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read restored state: %v", err)
	}
	if string(restored) != "v1" {
		t.Fatalf("expected restored state v1, got %q", string(restored))
	}
}
