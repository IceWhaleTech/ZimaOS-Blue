package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func withSystemUser(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: userID})
	return req.WithContext(ctx)
}

func setupSystemHandlerEcho() *echo.Echo {
	e := echo.New()
	h := NewSystemHandler("test-version", "test-build", "test-commit", "")
	g := e.Group("/api/v1")
	h.RegisterRoutes(g)
	h.RegisterFileBridgeRoutes(g)
	return e
}

func mustWritePNG(t *testing.T, path string) {
	t.Helper()
	img := makeTestImage()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode image: %v", err)
	}
}

func makeTestImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 45, G: 120, B: 220, A: 255})
		}
	}
	return img
}

func makeTestPNGBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, makeTestImage()); err != nil {
		t.Fatalf("encode png bytes: %v", err)
	}
	return buf.Bytes()
}

func mustWriteArchiveWithThumbnail(t *testing.T, path, entryName string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	entry, err := writer.Create(entryName)
	if err != nil {
		t.Fatalf("create archive entry: %v", err)
	}
	if _, err := entry.Write(makeTestPNGBytes(t)); err != nil {
		t.Fatalf("write archive entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
}

func ffmpegStubScript() string {
	return "#!/bin/sh\n" +
		"input=\"\"\n" +
		"prev=\"\"\n" +
		"for arg in \"$@\"; do\n" +
		"  if [ \"$prev\" = \"-i\" ]; then input=\"$arg\"; fi\n" +
		"  prev=\"$arg\"\n" +
		"done\n" +
		"eval \"output=\\${$#}\"\n" +
		"cp \"$input\" \"$output\"\n"
}

func TestResolveLocalFileRequiresAuthentication(t *testing.T) {
	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "report.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(filePath), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestRevealPathRejectsNonLoopbackClient(t *testing.T) {
	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "report.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"path": filePath})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/reveal-path", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "203.0.113.10:43210"
	req = withSystemUser(req, "user-1")

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestResolveLocalFileReturnsDownloadAndThumbnail(t *testing.T) {
	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "preview.png")
	mustWritePNG(t, imagePath)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(imagePath), nil)
	req = withSystemUser(req, "user-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if got := payload["path"]; got != imagePath {
		t.Fatalf("path=%v want=%s", got, imagePath)
	}

	downloadURL, _ := payload["download_url"].(string)
	if downloadURL == "" {
		t.Fatalf("download_url is empty")
	}
	if thumbnailURL, _ := payload["thumbnail_url"].(string); thumbnailURL == "" {
		t.Fatalf("thumbnail_url is empty")
	}
}

func TestResolveLocalFileRejectsAPILinks(t *testing.T) {
	e := setupSystemHandlerEcho()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape("/api/v1/media/report.html"), nil)
	req = withSystemUser(req, "user-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestGetLocalFileThumbnailReturnsJPEG(t *testing.T) {
	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "preview.png")
	mustWritePNG(t, imagePath)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/thumbnail?path="+url.QueryEscape(imagePath), nil)
	req = withSystemUser(req, "user-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get(echo.HeaderContentType); got != "image/jpeg" {
		t.Fatalf("content-type=%s want=image/jpeg", got)
	}
}

func TestGetLocalFileThumbnailFromOOXMLArchive(t *testing.T) {
	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "report.docx")
	mustWriteArchiveWithThumbnail(t, docPath, "docProps/thumbnail.png")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(docPath), nil)
	req = withSystemUser(req, "user-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	thumbnailURL, _ := payload["thumbnail_url"].(string)
	if thumbnailURL == "" {
		t.Fatalf("thumbnail_url is empty for OOXML archive")
	}

	reqThumb := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/thumbnail?path="+url.QueryEscape(docPath), nil)
	reqThumb = withSystemUser(reqThumb, "user-1")
	recThumb := httptest.NewRecorder()
	e.ServeHTTP(recThumb, reqThumb)

	if recThumb.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recThumb.Code, http.StatusOK, recThumb.Body.String())
	}
	if got := recThumb.Header().Get(echo.HeaderContentType); got != "image/jpeg" {
		t.Fatalf("content-type=%s want=image/jpeg", got)
	}
}

func TestGetLocalFileThumbnailViaPdftoppmFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script based pdftoppm stub is unix-only")
	}

	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()

	pdftoppmStubPath := filepath.Join(tmpDir, "pdftoppm")
	pdftoppmStub := "#!/bin/sh\ncp \"$3\" \"$4.jpg\"\n"
	if err := os.WriteFile(pdftoppmStubPath, []byte(pdftoppmStub), 0o755); err != nil {
		t.Fatalf("write pdftoppm stub: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	pdfPath := filepath.Join(tmpDir, "sample.pdf")
	if err := os.WriteFile(pdfPath, makeTestPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write pseudo pdf file: %v", err)
	}

	reqMeta := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(pdfPath), nil)
	reqMeta = withSystemUser(reqMeta, "user-1")
	recMeta := httptest.NewRecorder()
	e.ServeHTTP(recMeta, reqMeta)

	if recMeta.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recMeta.Code, http.StatusOK, recMeta.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recMeta.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if thumbnailURL, _ := payload["thumbnail_url"].(string); thumbnailURL == "" {
		t.Fatalf("thumbnail_url should be present when pdftoppm is available")
	}

	reqThumb := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/thumbnail?path="+url.QueryEscape(pdfPath), nil)
	reqThumb = withSystemUser(reqThumb, "user-1")
	recThumb := httptest.NewRecorder()
	e.ServeHTTP(recThumb, reqThumb)

	if recThumb.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recThumb.Code, http.StatusOK, recThumb.Body.String())
	}
	if got := recThumb.Header().Get(echo.HeaderContentType); got != "image/jpeg" {
		t.Fatalf("content-type=%s want=image/jpeg", got)
	}
}

func TestGetLocalFileThumbnailViaX2TFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script based x2t stub is unix-only")
	}
	if _, err := os.Stat(x2tFixedPath); err == nil {
		t.Skip("host fixed x2t binary exists; skipping PATH-based x2t stub test")
	}

	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()

	x2tStubPath := filepath.Join(tmpDir, "x2t")
	x2tStub := "#!/bin/sh\ncp \"$1\" \"$2\"\n"
	if err := os.WriteFile(x2tStubPath, []byte(x2tStub), 0o755); err != nil {
		t.Fatalf("write x2t stub: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	docPath := filepath.Join(tmpDir, "legacy.doc")
	if err := os.WriteFile(docPath, makeTestPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write pseudo doc file: %v", err)
	}

	reqMeta := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(docPath), nil)
	reqMeta = withSystemUser(reqMeta, "user-1")
	recMeta := httptest.NewRecorder()
	e.ServeHTTP(recMeta, reqMeta)

	if recMeta.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recMeta.Code, http.StatusOK, recMeta.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recMeta.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if thumbnailURL, _ := payload["thumbnail_url"].(string); thumbnailURL == "" {
		t.Fatalf("thumbnail_url should be present when x2t is available")
	}

	reqThumb := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/thumbnail?path="+url.QueryEscape(docPath), nil)
	reqThumb = withSystemUser(reqThumb, "user-1")
	recThumb := httptest.NewRecorder()
	e.ServeHTTP(recThumb, reqThumb)

	if recThumb.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recThumb.Code, http.StatusOK, recThumb.Body.String())
	}
	if got := recThumb.Header().Get(echo.HeaderContentType); got != "image/jpeg" {
		t.Fatalf("content-type=%s want=image/jpeg", got)
	}
}

func TestGetLocalFileThumbnailViaFFmpegFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script based ffmpeg stub is unix-only")
	}

	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()

	ffmpegStubPath := filepath.Join(tmpDir, "ffmpeg")
	ffmpegStub := ffmpegStubScript()
	if err := os.WriteFile(ffmpegStubPath, []byte(ffmpegStub), 0o755); err != nil {
		t.Fatalf("write ffmpeg stub: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	videoPath := filepath.Join(tmpDir, "sample.mp4")
	if err := os.WriteFile(videoPath, makeTestPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write pseudo video file: %v", err)
	}

	reqMeta := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(videoPath), nil)
	reqMeta = withSystemUser(reqMeta, "user-1")
	recMeta := httptest.NewRecorder()
	e.ServeHTTP(recMeta, reqMeta)

	if recMeta.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recMeta.Code, http.StatusOK, recMeta.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recMeta.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if thumbnailURL, _ := payload["thumbnail_url"].(string); thumbnailURL == "" {
		t.Fatalf("thumbnail_url should be present when ffmpeg is available")
	}

	reqThumb := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/thumbnail?path="+url.QueryEscape(videoPath), nil)
	reqThumb = withSystemUser(reqThumb, "user-1")
	recThumb := httptest.NewRecorder()
	e.ServeHTTP(recThumb, reqThumb)

	if recThumb.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recThumb.Code, http.StatusOK, recThumb.Body.String())
	}
	if got := recThumb.Header().Get(echo.HeaderContentType); got != "image/jpeg" {
		t.Fatalf("content-type=%s want=image/jpeg", got)
	}
}

func TestGetLocalFileThumbnailViaAudioCoverFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script based ffmpeg stub is unix-only")
	}

	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()

	ffmpegStubPath := filepath.Join(tmpDir, "ffmpeg")
	ffmpegStub := ffmpegStubScript()
	if err := os.WriteFile(ffmpegStubPath, []byte(ffmpegStub), 0o755); err != nil {
		t.Fatalf("write ffmpeg stub: %v", err)
	}
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	audioPath := filepath.Join(tmpDir, "sample.mp3")
	if err := os.WriteFile(audioPath, makeTestPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write pseudo audio file: %v", err)
	}

	reqMeta := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(audioPath), nil)
	reqMeta = withSystemUser(reqMeta, "user-1")
	recMeta := httptest.NewRecorder()
	e.ServeHTTP(recMeta, reqMeta)

	if recMeta.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recMeta.Code, http.StatusOK, recMeta.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recMeta.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if thumbnailURL, _ := payload["thumbnail_url"].(string); thumbnailURL == "" {
		t.Fatalf("thumbnail_url should be present when ffmpeg is available for audio")
	}

	reqThumb := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/thumbnail?path="+url.QueryEscape(audioPath), nil)
	reqThumb = withSystemUser(reqThumb, "user-1")
	recThumb := httptest.NewRecorder()
	e.ServeHTTP(recThumb, reqThumb)

	if recThumb.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recThumb.Code, http.StatusOK, recThumb.Body.String())
	}
	if got := recThumb.Header().Get(echo.HeaderContentType); got != "image/jpeg" {
		t.Fatalf("content-type=%s want=image/jpeg", got)
	}
}

func TestGetLocalFileThumbnailFromTextPreview(t *testing.T) {
	e := setupSystemHandlerEcho()
	tmpDir := t.TempDir()

	textPath := filepath.Join(tmpDir, "notes.md")
	content := strings.Join([]string{
		"# Report",
		"",
		"- Item A",
		"- Item B",
		"Long line for preview rendering that should be wrapped into multiple segments for thumbnail generation.",
	}, "\n")
	if err := os.WriteFile(textPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write text file: %v", err)
	}

	reqMeta := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(textPath), nil)
	reqMeta = withSystemUser(reqMeta, "user-1")
	recMeta := httptest.NewRecorder()
	e.ServeHTTP(recMeta, reqMeta)

	if recMeta.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recMeta.Code, http.StatusOK, recMeta.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recMeta.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if thumbnailURL, _ := payload["thumbnail_url"].(string); thumbnailURL == "" {
		t.Fatalf("thumbnail_url should be present for text preview files")
	}

	reqThumb := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file/thumbnail?path="+url.QueryEscape(textPath), nil)
	reqThumb = withSystemUser(reqThumb, "user-1")
	recThumb := httptest.NewRecorder()
	e.ServeHTTP(recThumb, reqThumb)

	if recThumb.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recThumb.Code, http.StatusOK, recThumb.Body.String())
	}
	if got := recThumb.Header().Get(echo.HeaderContentType); got != "image/jpeg" {
		t.Fatalf("content-type=%s want=image/jpeg", got)
	}
}
