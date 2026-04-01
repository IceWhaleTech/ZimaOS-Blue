package bootstrap

import (
	"context"
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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newExecApprovalRoutesTestHarness(t *testing.T) (*echo.Echo, *tools.ApprovalManager, *tools.DirAllowlistStore, *auth.JWTService) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dirStore, err := tools.NewDirAllowlistStore(db)
	if err != nil {
		t.Fatalf("NewDirAllowlistStore: %v", err)
	}

	broker := sse.NewBroker()
	t.Cleanup(broker.Close)
	approvals := tools.NewApprovalManager(broker)

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	authMiddleware := auth.NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	v1 := e.Group("/api/v1")
	execGroup := v1.Group("/exec", authMiddleware.Authenticate(), permission.RequirePagePermission(nil, permission.PageChat))
	registerExecApprovalRoutes(execGroup, approvals)
	registerExecDirectoryApprovalRoutes(execGroup, dirStore)

	return e, approvals, dirStore, jwtSvc
}

func mustExecApprovalAccessToken(t *testing.T, jwtSvc *auth.JWTService, userID string) string {
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

func TestExecApprovalRoutesExposePendingAndResolveWithBinding(t *testing.T) {
	e, approvals, _, jwtSvc := newExecApprovalRoutesTestHarness(t)

	decisionCh := make(chan tools.ApprovalDecision, 1)
	errCh := make(chan error, 1)
	go func() {
		decision, err := approvals.RequestApproval(context.Background(), tools.ApprovalRequest{
			Type:      "command",
			Command:   "echo hello",
			UserID:    "user-a",
			SessionID: "session-1",
		})
		if err != nil {
			errCh <- err
			return
		}
		decisionCh <- decision
	}()

	var pending *tools.ApprovalRequest
	deadline := time.Now().Add(2 * time.Second)
	for pending == nil && time.Now().Before(deadline) {
		pending = approvals.GetPending("user-a")
		if pending == nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if pending == nil {
		t.Fatal("expected pending approval request")
	}

	reqPending := httptest.NewRequest(http.MethodGet, "/api/v1/exec/approvals/pending", nil)
	reqPending.Header.Set(echo.HeaderAuthorization, "Bearer "+mustExecApprovalAccessToken(t, jwtSvc, "user-a"))
	recPending := httptest.NewRecorder()
	e.ServeHTTP(recPending, reqPending)
	if recPending.Code != http.StatusOK {
		t.Fatalf("pending status=%d, want 200, body=%s", recPending.Code, recPending.Body.String())
	}

	var pendingBody struct {
		Pending  bool                   `json:"pending"`
		Approval *tools.ApprovalRequest `json:"approval"`
	}
	if err := json.Unmarshal(recPending.Body.Bytes(), &pendingBody); err != nil {
		t.Fatalf("decode pending response: %v", err)
	}
	if !pendingBody.Pending || pendingBody.Approval == nil || pendingBody.Approval.ID != pending.ID {
		t.Fatalf("unexpected pending payload: %+v", pendingBody)
	}

	reqResolve := httptest.NewRequest(http.MethodPost, "/api/v1/exec/approvals/"+pending.ID, strings.NewReader(`{"decision":"allow-once","binding_hash":"`+pending.BindingHash+`"}`))
	reqResolve.Header.Set(echo.HeaderAuthorization, "Bearer "+mustExecApprovalAccessToken(t, jwtSvc, "user-a"))
	reqResolve.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recResolve := httptest.NewRecorder()
	e.ServeHTTP(recResolve, reqResolve)
	if recResolve.Code != http.StatusOK {
		t.Fatalf("resolve status=%d, want 200, body=%s", recResolve.Code, recResolve.Body.String())
	}

	select {
	case err := <-errCh:
		t.Fatalf("approval request returned error: %v", err)
	case decision := <-decisionCh:
		if decision != tools.ApprovalAllowOnce {
			t.Fatalf("decision=%q, want %q", decision, tools.ApprovalAllowOnce)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval resolution")
	}
}

func TestExecDirectoryApprovalRoutesListAndDeleteEnforceOwnership(t *testing.T) {
	e, _, dirStore, jwtSvc := newExecApprovalRoutesTestHarness(t)
	base := t.TempDir()
	userAPath := filepath.Join(base, "user-a")
	userBPath := filepath.Join(base, "user-b")

	if err := dirStore.Add(userAPath, "user-a"); err != nil {
		t.Fatalf("dirStore.Add(user-a): %v", err)
	}
	if err := dirStore.Add(userBPath, "user-b"); err != nil {
		t.Fatalf("dirStore.Add(user-b): %v", err)
	}

	userAEntry := dirStore.Match(filepath.Join(userAPath, "workspace"))
	if userAEntry == nil {
		t.Fatal("expected user-a directory entry")
	}
	userBEntry := dirStore.Match(filepath.Join(userBPath, "workspace"))
	if userBEntry == nil {
		t.Fatal("expected user-b directory entry")
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/exec/approvals/directories", nil)
	reqList.Header.Set(echo.HeaderAuthorization, "Bearer "+mustExecApprovalAccessToken(t, jwtSvc, "user-a"))
	recList := httptest.NewRecorder()
	e.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("list status=%d, want 200, body=%s", recList.Code, recList.Body.String())
	}

	var listBody struct {
		Entries []struct {
			ID         string `json:"id"`
			Path       string `json:"path"`
			AddedAt    string `json:"added_at"`
			LastUsed   string `json:"last_used"`
			ApprovedBy string `json:"approved_by"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listBody.Entries) != 1 || listBody.Entries[0].ID != userAEntry.ID || listBody.Entries[0].Path != userAPath {
		t.Fatalf("unexpected entries payload: %+v", listBody.Entries)
	}

	reqForbidden := httptest.NewRequest(http.MethodDelete, "/api/v1/exec/approvals/directories/"+userBEntry.ID, nil)
	reqForbidden.Header.Set(echo.HeaderAuthorization, "Bearer "+mustExecApprovalAccessToken(t, jwtSvc, "user-a"))
	recForbidden := httptest.NewRecorder()
	e.ServeHTTP(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("cross-user delete status=%d, want 403, body=%s", recForbidden.Code, recForbidden.Body.String())
	}

	reqAllowed := httptest.NewRequest(http.MethodDelete, "/api/v1/exec/approvals/directories/"+userAEntry.ID, nil)
	reqAllowed.Header.Set(echo.HeaderAuthorization, "Bearer "+mustExecApprovalAccessToken(t, jwtSvc, "user-a"))
	recAllowed := httptest.NewRecorder()
	e.ServeHTTP(recAllowed, reqAllowed)
	if recAllowed.Code != http.StatusOK {
		t.Fatalf("own delete status=%d, want 200, body=%s", recAllowed.Code, recAllowed.Body.String())
	}

	var deleteBody struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.Unmarshal(recAllowed.Body.Bytes(), &deleteBody); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if !deleteBody.Deleted {
		t.Fatalf("deleted=%v, want true", deleteBody.Deleted)
	}
	if got := dirStore.Match(filepath.Join(userAPath, "workspace")); got != nil {
		t.Fatalf("expected user-a directory to be deleted, got %+v", got)
	}
	if got := dirStore.Match(filepath.Join(userBPath, "workspace")); got == nil {
		t.Fatal("expected user-b directory to remain")
	}
}
