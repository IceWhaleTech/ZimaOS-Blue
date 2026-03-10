package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHandlerGitCapability(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	origLookPath := gitLookPath
	origRun := gitRun
	t.Cleanup(func() {
		gitLookPath = origLookPath
		gitRun = origRun
	})

	gitLookPath = func(file string) (string, error) {
		return "/usr/bin/git", nil
	}
	gitRun = func(_ context.Context, dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "config --get user.name":
			return "Blue", nil
		case "config --get user.email":
			return "blue@example.com", nil
		case "rev-parse --show-toplevel":
			return dir, nil
		default:
			return "", nil
		}
	}

	h := NewHandler(mgr)
	e := echo.New()
	h.RegisterRoutes(e.Group("/workspace"))

	req := httptest.NewRequest(http.MethodGet, "/workspace/git/capability", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var got GitCapability
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !got.Available {
		t.Fatal("expected git to be available")
	}
	if !got.Repository || !got.WorkspaceRooted {
		t.Fatalf("expected rooted repository, got %+v", got)
	}
	if !got.CanCommit {
		t.Fatalf("expected can_commit=true, got %+v", got)
	}
}
