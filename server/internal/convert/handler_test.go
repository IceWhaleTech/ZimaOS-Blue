package convert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func withConvertUserClaims(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: userID})
	return req.WithContext(ctx)
}

func seedConvertTaskWithOutput(t *testing.T, svc *Service, userID, conversationID, taskID, outputID, outputPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("output"), 0o640); err != nil {
		t.Fatalf("write output file: %v", err)
	}
	task := &ConvertTask{
		ID:             taskID,
		UserID:         userID,
		ConversationID: conversationID,
		Action:         ActionConvert,
		Status:         StatusSucceeded,
		Outputs: []ConvertOutput{
			{
				ID:   outputID,
				Name: filepath.Base(outputPath),
				Path: outputPath,
			},
		},
	}
	if err := svc.store.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}
}

func TestGetOutputLocationSuccess(t *testing.T) {
	svc := setupConvertTestService(t)
	taskID := "task-location"
	outputID := "out-1"
	outputPath := filepath.Join(svc.outputDir(taskID), "result.pdf")
	seedConvertTaskWithOutput(t, svc, "user-1", "conv-1", taskID, outputID, outputPath)

	e := echo.New()
	h := NewHandler(svc, nil)
	h.RegisterRoutes(e.Group("/api/v1/convert"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/convert/tasks/"+taskID+"/outputs/"+outputID+"/location", nil)
	req = withConvertUserClaims(req, "user-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got := payload["path"]; got != outputPath {
		t.Fatalf("path=%q, want %q", got, outputPath)
	}
	if got := payload["parent_path"]; got != filepath.Dir(outputPath) {
		t.Fatalf("parent_path=%q, want %q", got, filepath.Dir(outputPath))
	}
}

func TestGetOutputLocationNotFound(t *testing.T) {
	svc := setupConvertTestService(t)
	e := echo.New()
	h := NewHandler(svc, nil)
	h.RegisterRoutes(e.Group("/api/v1/convert"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/convert/tasks/missing/outputs/out/location", nil)
	req = withConvertUserClaims(req, "user-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestGetOutputLocationEnforcesUserIsolation(t *testing.T) {
	svc := setupConvertTestService(t)
	taskID := "task-private"
	outputID := "out-1"
	outputPath := filepath.Join(svc.outputDir(taskID), "secret.txt")
	seedConvertTaskWithOutput(t, svc, "owner-user", "conv-1", taskID, outputID, outputPath)

	e := echo.New()
	h := NewHandler(svc, nil)
	h.RegisterRoutes(e.Group("/api/v1/convert"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/convert/tasks/"+taskID+"/outputs/"+outputID+"/location", nil)
	req = withConvertUserClaims(req, "other-user")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
