package server

import (
	"archive/zip"
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

func TestDecodePreviewImage_FallsBackToReadableOfficeTextPreview(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		ext   string
		write func(*testing.T, string)
	}{
		{name: "docx", ext: ".docx", write: writeMinimalOfficePreviewDOCX},
		{name: "pptx", ext: ".pptx", write: writeMinimalOfficePreviewPPTX},
		{name: "xlsx", ext: ".xlsx", write: writeMinimalOfficePreviewXLSX},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "preview"+tc.ext)
			tc.write(t, path)

			img, err := decodePreviewImage(path)
			if err != nil {
				t.Fatalf("decodePreviewImage(%s) error = %v", tc.ext, err)
			}
			if img == nil {
				t.Fatalf("decodePreviewImage(%s) returned nil image", tc.ext)
			}
			bounds := img.Bounds()
			if bounds.Dx() != 960 || bounds.Dy() != 640 {
				t.Fatalf("preview bounds = %dx%d, want 960x640 text preview fallback", bounds.Dx(), bounds.Dy())
			}
		})
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

func writeMinimalOfficePreviewDOCX(t *testing.T, path string) {
	t.Helper()

	var document strings.Builder
	document.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	document.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	document.WriteString(`<w:p><w:r><w:t>Office preview fallback should show this DOCX text.</w:t></w:r></w:p>`)
	document.WriteString(`</w:body></w:document>`)

	writeMinimalOfficePreviewArchive(t, path, map[string]string{
		"word/document.xml": document.String(),
	})
}

func writeMinimalOfficePreviewPPTX(t *testing.T, path string) {
	t.Helper()

	var slide strings.Builder
	slide.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	slide.WriteString(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody>`)
	slide.WriteString(`<a:p><a:r><a:t>Office preview fallback should show this PPTX text.</a:t></a:r></a:p>`)
	slide.WriteString(`</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`)

	writeMinimalOfficePreviewArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": slide.String(),
	})
}

func writeMinimalOfficePreviewXLSX(t *testing.T, path string) {
	t.Helper()

	writeMinimalOfficePreviewArchive(t, path, map[string]string{
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Preview" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
</Relationships>`,
		"xl/sharedStrings.xml": `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="2" uniqueCount="2">
  <si><t>Title</t></si>
  <si><t>Office preview fallback should show this XLSX text.</t></si>
</sst>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="s"><v>0</v></c>
      <c r="B1" t="s"><v>1</v></c>
    </row>
  </sheetData>
</worksheet>`,
	})
}

func writeMinimalOfficePreviewArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create office preview archive: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close office preview archive: %v", err)
	}
}
