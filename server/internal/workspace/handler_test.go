package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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
			Path       string `json:"path"`
			AbsPath    string `json:"abs_path"`
			Type       string `json:"type"`
			Depth      int    `json:"depth"`
			ModifiedAt string `json:"modified_at"`
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
			if entry.ModifiedAt == "" {
				t.Fatal("expected modified_at to be present for report entry")
			}
			if _, err := time.Parse(time.RFC3339Nano, entry.ModifiedAt); err != nil {
				t.Fatalf("expected modified_at to be RFC3339Nano, got %q: %v", entry.ModifiedAt, err)
			}
			break
		}
	}
	if !foundReport {
		t.Fatal("expected to find memory/daily/report.txt in tree entries")
	}
}

func TestHandlerTreePreservesDirectorySubtreeOrder(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	siblingDir := filepath.Join(workspaceDir, "phone_specs_2026")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(siblingDir, "Redmi_Note_15.md"), []byte("child"), 0o644); err != nil {
		t.Fatalf("WriteFile child: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "phone_specs_2026.md"), []byte("sibling"), 0o644); err != nil {
		t.Fatalf("WriteFile sibling: %v", err)
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
		Entries []struct {
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	dirIndex := -1
	childIndex := -1
	siblingFileIndex := -1
	for idx, entry := range got.Entries {
		switch entry.Path {
		case "phone_specs_2026":
			dirIndex = idx
		case "phone_specs_2026/Redmi_Note_15.md":
			childIndex = idx
		case "phone_specs_2026.md":
			siblingFileIndex = idx
		}
	}

	if dirIndex == -1 || childIndex == -1 || siblingFileIndex == -1 {
		t.Fatalf("expected tree entries for dir, child, and sibling file; got %+v", got.Entries)
	}
	if !(dirIndex < childIndex && childIndex < siblingFileIndex) {
		t.Fatalf(
			"expected directory subtree to remain contiguous, got dir=%d child=%d sibling=%d",
			dirIndex,
			childIndex,
			siblingFileIndex,
		)
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

func TestHandlerTreePaginatesEntries(t *testing.T) {
	workspaceDir := t.TempDir()
	mgr := NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	pagedDir := filepath.Join(workspaceDir, "paged")
	if err := os.MkdirAll(pagedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll paged: %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(pagedDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	h := NewHandler(mgr)
	e := echo.New()
	h.RegisterRoutes(e.Group("/workspace"))

	run := func(offset int) struct {
		Root    string `json:"root"`
		Entries []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"entries"`
		NextOffset int  `json:"next_offset"`
		HasMore    bool `json:"has_more"`
	} {
		req := httptest.NewRequest(
			http.MethodGet,
			"/workspace/tree?root=paged&max_depth=2&limit=2&offset="+strconv.Itoa(offset),
			nil,
		)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 for offset=%d, got %d: %s", offset, rec.Code, rec.Body.String())
		}

		var got struct {
			Root    string `json:"root"`
			Entries []struct {
				Path string `json:"path"`
				Type string `json:"type"`
			} `json:"entries"`
			NextOffset int  `json:"next_offset"`
			HasMore    bool `json:"has_more"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal response offset=%d: %v", offset, err)
		}
		return got
	}

	page0 := run(0)
	if page0.Root != pagedDir {
		t.Fatalf("page0 root = %q, want %q", page0.Root, pagedDir)
	}
	if len(page0.Entries) != 2 {
		t.Fatalf("page0 len = %d, want 2", len(page0.Entries))
	}
	if page0.Entries[0].Path != "a.txt" || page0.Entries[1].Path != "b.txt" {
		t.Fatalf("page0 paths = %+v, want [a.txt b.txt]", page0.Entries)
	}
	if !page0.HasMore {
		t.Fatal("page0 has_more = false, want true")
	}
	if page0.NextOffset != 2 {
		t.Fatalf("page0 next_offset = %d, want 2", page0.NextOffset)
	}

	page1 := run(page0.NextOffset)
	if len(page1.Entries) != 1 {
		t.Fatalf("page1 len = %d, want 1", len(page1.Entries))
	}
	if page1.Entries[0].Path != "c.txt" || page1.Entries[0].Type != "file" {
		t.Fatalf("page1 entries = %+v, want c.txt file", page1.Entries)
	}
	if page1.HasMore {
		t.Fatal("page1 has_more = true, want false")
	}
	if page1.NextOffset != 3 {
		t.Fatalf("page1 next_offset = %d, want 3", page1.NextOffset)
	}
}
