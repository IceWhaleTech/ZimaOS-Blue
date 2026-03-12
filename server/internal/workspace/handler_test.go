package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

func TestHandlerMeta(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	h := NewHandler(mgr)
	e := echo.New()
	h.RegisterRoutes(e.Group("/workspace"))

	req := httptest.NewRequest(http.MethodGet, "/workspace/meta", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var got struct {
		Dir string `json:"dir"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Dir != workspaceDir {
		t.Fatalf("expected dir %q, got %q", workspaceDir, got.Dir)
	}
}

func TestHandlerTree(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	dailyDir := filepath.Join(workspaceDir, "memory", "daily")
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	outputPath := filepath.Join(dailyDir, "report.txt")
	if err := os.WriteFile(outputPath, []byte("report"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	h := NewHandler(mgr)
	e := echo.New()
	h.RegisterRoutes(e.Group("/workspace"))

	req := httptest.NewRequest(http.MethodGet, "/workspace/tree?max_depth=4", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var got struct {
		Root    string `json:"root"`
		Entries []struct {
			Path    string `json:"path"`
			AbsPath string `json:"abs_path"`
			Type    string `json:"type"`
			Depth   int    `json:"depth"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Root != workspaceDir {
		t.Fatalf("expected root %q, got %q", workspaceDir, got.Root)
	}
	if len(got.Entries) == 0 {
		t.Fatal("expected non-empty tree entries")
	}

	var foundReport bool
	for _, entry := range got.Entries {
		if entry.Path == "memory/daily/report.txt" {
			foundReport = true
			if entry.Type != "file" {
				t.Fatalf("expected report entry type file, got %q", entry.Type)
			}
			if entry.AbsPath != outputPath {
				t.Fatalf("expected abs path %q, got %q", outputPath, entry.AbsPath)
			}
			if entry.Depth != 3 {
				t.Fatalf("expected depth 3, got %d", entry.Depth)
			}
			break
		}
	}
	if !foundReport {
		t.Fatal("expected to find memory/daily/report.txt in tree entries")
	}
}

func TestHandlerTreeAllowsWhitelistRoot(t *testing.T) {
	workspaceDir := t.TempDir()
	extraRoot := t.TempDir()

	mgr := NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	extraFile := filepath.Join(extraRoot, "from-whitelist.txt")
	if err := os.WriteFile(extraFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	h := NewHandler(mgr)
	h.SetAllowedRootsProvider(func() []string { return []string{extraRoot} })

	e := echo.New()
	h.RegisterRoutes(e.Group("/workspace"))

	req := httptest.NewRequest(http.MethodGet, "/workspace/tree?root="+url.QueryEscape(extraRoot), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got struct {
		Root    string `json:"root"`
		Entries []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Root != extraRoot {
		t.Fatalf("expected root %q, got %q", extraRoot, got.Root)
	}
	found := false
	for _, entry := range got.Entries {
		if entry.Path == "from-whitelist.txt" && entry.Type == "file" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find from-whitelist.txt in entries: %+v", got.Entries)
	}
}

func TestHandlerTreeRejectsNonAllowedRoot(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	h := NewHandler(mgr)
	e := echo.New()
	h.RegisterRoutes(e.Group("/workspace"))

	outside := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/workspace/tree?root="+url.QueryEscape(outside), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
