package bootstrap

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newBrowserApprovalRoutesTestHarness(t *testing.T) (*echo.Echo, *tools.BrowserSiteAllowlistStore, *auth.JWTService) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := tools.NewBrowserSiteAllowlistStore(db)
	if err != nil {
		t.Fatalf("NewBrowserSiteAllowlistStore: %v", err)
	}

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	authMiddleware := auth.NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	v1 := e.Group("/api/v1")
	registerBrowserApprovalRoutes(
		v1,
		authMiddleware.Authenticate(),
		permission.RequirePagePermission(nil, permission.PageChat),
		store,
	)

	return e, store, jwtSvc
}

func mustBrowserApprovalAccessToken(t *testing.T, jwtSvc *auth.JWTService, userID string) string {
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

func TestBrowserApprovalRoutesListFiltersEntriesByAuthenticatedUser(t *testing.T) {
	e, store, jwtSvc := newBrowserApprovalRoutesTestHarness(t)
	if err := store.Add("https://example.com/account", "user-a"); err != nil {
		t.Fatalf("store.Add(user-a): %v", err)
	}
	if err := store.Add("https://other.example/preferences", "user-b"); err != nil {
		t.Fatalf("store.Add(user-b): %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/browser/approvals/sites", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+mustBrowserApprovalAccessToken(t, jwtSvc, "user-a"))
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Entries []struct {
			ID         string `json:"id"`
			Origin     string `json:"origin"`
			AddedAt    string `json:"added_at"`
			LastUsed   string `json:"last_used"`
			ApprovedBy string `json:"approved_by"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Entries) != 1 {
		t.Fatalf("entries len=%d, want 1, body=%s", len(body.Entries), rec.Body.String())
	}
	if body.Entries[0].Origin != "https://example.com" {
		t.Fatalf("origin=%q, want %q", body.Entries[0].Origin, "https://example.com")
	}
	if body.Entries[0].ApprovedBy != "user-a" {
		t.Fatalf("approved_by=%q, want %q", body.Entries[0].ApprovedBy, "user-a")
	}
	if _, err := time.Parse(time.RFC3339, body.Entries[0].AddedAt); err != nil {
		t.Fatalf("added_at parse error: %v", err)
	}
	if _, err := time.Parse(time.RFC3339, body.Entries[0].LastUsed); err != nil {
		t.Fatalf("last_used parse error: %v", err)
	}
}

func TestBrowserApprovalRoutesDeleteEnforcesOwnership(t *testing.T) {
	e, store, jwtSvc := newBrowserApprovalRoutesTestHarness(t)
	if err := store.Add("https://example.com/account", "user-a"); err != nil {
		t.Fatalf("store.Add(user-a): %v", err)
	}
	if err := store.Add("https://other.example/preferences", "user-b"); err != nil {
		t.Fatalf("store.Add(user-b): %v", err)
	}

	userAEntry := store.Match("https://example.com/settings", "user-a")
	if userAEntry == nil {
		t.Fatal("expected user-a entry to exist")
	}
	userBEntry := store.Match("https://other.example/profile", "user-b")
	if userBEntry == nil {
		t.Fatal("expected user-b entry to exist")
	}

	reqForbidden := httptest.NewRequest(http.MethodDelete, "/api/v1/browser/approvals/sites/"+userBEntry.ID, nil)
	reqForbidden.Header.Set(echo.HeaderAuthorization, "Bearer "+mustBrowserApprovalAccessToken(t, jwtSvc, "user-a"))
	recForbidden := httptest.NewRecorder()
	e.ServeHTTP(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("cross-user delete status=%d, want 403, body=%s", recForbidden.Code, recForbidden.Body.String())
	}

	reqAllowed := httptest.NewRequest(http.MethodDelete, "/api/v1/browser/approvals/sites/"+userAEntry.ID, nil)
	reqAllowed.Header.Set(echo.HeaderAuthorization, "Bearer "+mustBrowserApprovalAccessToken(t, jwtSvc, "user-a"))
	recAllowed := httptest.NewRecorder()
	e.ServeHTTP(recAllowed, reqAllowed)
	if recAllowed.Code != http.StatusOK {
		t.Fatalf("own delete status=%d, want 200, body=%s", recAllowed.Code, recAllowed.Body.String())
	}

	var body struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.Unmarshal(recAllowed.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if !body.Deleted {
		t.Fatalf("deleted=%v, want true", body.Deleted)
	}
	if got := store.Match("https://example.com/settings", "user-a"); got != nil {
		t.Fatalf("expected user-a entry to be deleted, got %+v", got)
	}
	if got := store.Match("https://other.example/profile", "user-b"); got == nil {
		t.Fatal("expected user-b entry to remain after forbidden delete")
	}
}
