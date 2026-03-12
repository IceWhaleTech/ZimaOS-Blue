package bootstrap

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
)

func newMediaRoutesTestHarness(t *testing.T) (*echo.Echo, *mediagen.TaskStore, *auth.JWTService) {
	t.Helper()

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "media_routes_test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	taskStore, err := mediagen.NewTaskStore(db)
	if err != nil {
		t.Fatalf("NewTaskStore: %v", err)
	}

	manager := mediagen.NewManager(nil, nil, "")
	manager.SetTaskStore(taskStore)
	handler := mediagen.NewHandler(manager, nil, "")

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	middleware := auth.NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	api := e.Group("/api/v1")
	mediaGroup := api.Group("/media")
	mediaGroup.Use(middleware.OptionalAuthenticate())
	handler.RegisterRoutes(mediaGroup)

	return e, taskStore, jwtSvc
}

func mustMediaAccessToken(t *testing.T, jwtSvc *auth.JWTService, userID string) string {
	t.Helper()

	token, err := jwtSvc.GenerateAccessToken(&auth.UserClaims{
		UserID:   userID,
		Username: userID,
		Role:     "user",
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	return token
}

func mustCreateMediaTask(t *testing.T, store *mediagen.TaskStore, id, userID, messageID string) {
	t.Helper()

	if err := store.Create(&mediagen.PersistentTask{
		ID:        id,
		UserID:    userID,
		MessageID: messageID,
		Status:    mediagen.TaskStatusPending,
		Type:      mediagen.MediaTypeImage,
		Category:  "t2i",
	}); err != nil {
		t.Fatalf("Create task %s: %v", id, err)
	}
}

func TestMediaRoutesOptionalAuthenticateScopesGetTask(t *testing.T) {
	e, store, jwtSvc := newMediaRoutesTestHarness(t)
	mustCreateMediaTask(t, store, "task-user-a", "user-a", "msg-1")
	mustCreateMediaTask(t, store, "task-user-b", "user-b", "msg-1")

	tokenUserB := mustMediaAccessToken(t, jwtSvc, "user-b")

	reqForbidden := httptest.NewRequest(http.MethodGet, "/api/v1/media/tasks/task-user-a", nil)
	reqForbidden.Header.Set(echo.HeaderAuthorization, "Bearer "+tokenUserB)
	recForbidden := httptest.NewRecorder()
	e.ServeHTTP(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusNotFound {
		t.Fatalf("cross-user status=%d, want 404, body=%s", recForbidden.Code, recForbidden.Body.String())
	}

	reqAllowed := httptest.NewRequest(http.MethodGet, "/api/v1/media/tasks/task-user-b", nil)
	reqAllowed.Header.Set(echo.HeaderAuthorization, "Bearer "+tokenUserB)
	recAllowed := httptest.NewRecorder()
	e.ServeHTTP(recAllowed, reqAllowed)
	if recAllowed.Code != http.StatusOK {
		t.Fatalf("own-task status=%d, want 200, body=%s", recAllowed.Code, recAllowed.Body.String())
	}
}

func TestMediaRoutesOptionalAuthenticateAnonymousTaskScope(t *testing.T) {
	e, store, _ := newMediaRoutesTestHarness(t)
	mustCreateMediaTask(t, store, "task-user-a", "user-a", "msg-1")
	mustCreateMediaTask(t, store, "task-public", "", "msg-1")

	reqPrivate := httptest.NewRequest(http.MethodGet, "/api/v1/media/tasks/task-user-a", nil)
	recPrivate := httptest.NewRecorder()
	e.ServeHTTP(recPrivate, reqPrivate)
	if recPrivate.Code != http.StatusNotFound {
		t.Fatalf("anonymous private status=%d, want 404, body=%s", recPrivate.Code, recPrivate.Body.String())
	}

	reqPublic := httptest.NewRequest(http.MethodGet, "/api/v1/media/tasks/task-public", nil)
	recPublic := httptest.NewRecorder()
	e.ServeHTTP(recPublic, reqPublic)
	if recPublic.Code != http.StatusOK {
		t.Fatalf("anonymous public status=%d, want 200, body=%s", recPublic.Code, recPublic.Body.String())
	}
}

func TestMediaRoutesOptionalAuthenticateScopesGetTaskByMessage(t *testing.T) {
	e, store, jwtSvc := newMediaRoutesTestHarness(t)
	mustCreateMediaTask(t, store, "task-user-a", "user-a", "msg-iso")
	mustCreateMediaTask(t, store, "task-user-b", "user-b", "msg-iso")
	mustCreateMediaTask(t, store, "task-public", "", "msg-iso")

	tokenUserB := mustMediaAccessToken(t, jwtSvc, "user-b")

	reqScoped := httptest.NewRequest(http.MethodGet, "/api/v1/media/tasks/by-message/msg-iso", nil)
	reqScoped.Header.Set(echo.HeaderAuthorization, "Bearer "+tokenUserB)
	recScoped := httptest.NewRecorder()
	e.ServeHTTP(recScoped, reqScoped)
	if recScoped.Code != http.StatusOK {
		t.Fatalf("scoped status=%d, want 200, body=%s", recScoped.Code, recScoped.Body.String())
	}
	var scopedBody struct {
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(recScoped.Body.Bytes(), &scopedBody); err != nil {
		t.Fatalf("decode scoped body: %v", err)
	}
	if len(scopedBody.Tasks) != 1 || scopedBody.Tasks[0].ID != "task-user-b" {
		t.Fatalf("scoped tasks=%#v, want only task-user-b", scopedBody.Tasks)
	}

	reqAnon := httptest.NewRequest(http.MethodGet, "/api/v1/media/tasks/by-message/msg-iso", nil)
	recAnon := httptest.NewRecorder()
	e.ServeHTTP(recAnon, reqAnon)
	if recAnon.Code != http.StatusOK {
		t.Fatalf("anon status=%d, want 200, body=%s", recAnon.Code, recAnon.Body.String())
	}
	var anonBody struct {
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(recAnon.Body.Bytes(), &anonBody); err != nil {
		t.Fatalf("decode anon body: %v", err)
	}
	if len(anonBody.Tasks) != 1 || anonBody.Tasks[0].ID != "task-public" {
		t.Fatalf("anon tasks=%#v, want only task-public", anonBody.Tasks)
	}
}
