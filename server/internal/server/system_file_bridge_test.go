package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func TestResolveLocalFileRejectsRemoteClient(t *testing.T) {
	file := tempFileWithContents(t, "report.txt", []byte("ok"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(file), nil)
	req.RemoteAddr = "203.0.113.5:12345"
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "preview-user",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var handler SystemHandler
	err := handler.ResolveLocalFile(c)
	if err == nil {
		t.Fatalf("expected HTTP error, got nil")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusForbidden)
	}
}

func TestDownloadLocalFileServesInlineWhenRequested(t *testing.T) {
	file := tempFileWithContents(t, "report.txt", []byte("ok"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/content?path="+url.QueryEscape(file)+"&inline=1", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "preview-user",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var handler SystemHandler
	if err := handler.DownloadLocalFile(c); err != nil {
		t.Fatalf("DownloadLocalFile() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Disposition"); strings.Contains(strings.ToLower(got), "attachment") {
		t.Fatalf("Content-Disposition = %q, want inline content without attachment", got)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}
}

func TestDownloadLocalFileDefaultsToAttachment(t *testing.T) {
	file := tempFileWithContents(t, "report.txt", []byte("ok"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/content?path="+url.QueryEscape(file), nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "preview-user",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var handler SystemHandler
	if err := handler.DownloadLocalFile(c); err != nil {
		t.Fatalf("DownloadLocalFile() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := strings.ToLower(rec.Header().Get("Content-Disposition")); !strings.Contains(got, "attachment") {
		t.Fatalf("Content-Disposition = %q, want attachment", rec.Header().Get("Content-Disposition"))
	}
}

func TestParentDirectoryForRevealFallback(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "report.txt")

	if got := parentDirectoryForRevealFallback(file, false); got != dir {
		t.Fatalf("parentDirectoryForRevealFallback(file) = %q, want %q", got, dir)
	}
	if got := parentDirectoryForRevealFallback(dir, true); got != "" {
		t.Fatalf("parentDirectoryForRevealFallback(dir, true) = %q, want empty", got)
	}

	root := string(os.PathSeparator)
	if volume := filepath.VolumeName(dir); volume != "" {
		root = volume + string(os.PathSeparator)
	}
	if got := parentDirectoryForRevealFallback(root, false); got != "" {
		t.Fatalf("parentDirectoryForRevealFallback(root) = %q, want empty", got)
	}
}

func TestRevealPathInFileManagerWithFallsBackToParentDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "report.txt")

	var calls []string
	err := revealPathInFileManagerWith(file, false, func(path string, isDir bool) error {
		calls = append(calls, path)
		if path == file {
			return errors.New("direct file reveal unsupported")
		}
		if path == dir && isDir {
			return nil
		}
		t.Fatalf("unexpected reveal call path=%q isDir=%v", path, isDir)
		return nil
	})
	if err != nil {
		t.Fatalf("revealPathInFileManagerWith() error = %v", err)
	}
	if !slices.Equal(calls, []string{file, dir}) {
		t.Fatalf("reveal calls = %v, want [%q %q]", calls, file, dir)
	}
}

func TestRevealPathInFileManagerWithReturnsCombinedErrorWhenFallbackFails(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "report.txt")

	err := revealPathInFileManagerWith(file, false, func(path string, isDir bool) error {
		if path == file {
			return errors.New("direct file reveal unsupported")
		}
		if path == dir && isDir {
			return errors.New("parent reveal failed")
		}
		t.Fatalf("unexpected reveal call path=%q isDir=%v", path, isDir)
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "direct file reveal unsupported") {
		t.Fatalf("error = %q, want direct reveal failure", err)
	}
	if !strings.Contains(err.Error(), "parent reveal failed") {
		t.Fatalf("error = %q, want parent fallback failure", err)
	}
}

func tempFileWithContents(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/" + name
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
