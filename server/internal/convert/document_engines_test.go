package convert

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeFileInfo struct {
	name string
}

func (f fakeFileInfo) Name() string      { return f.name }
func (f fakeFileInfo) Size() int64       { return 1 }
func (f fakeFileInfo) Mode() fs.FileMode { return 0o755 }
func (f fakeFileInfo) ModTime() time.Time {
	return time.Unix(0, 0)
}
func (f fakeFileInfo) IsDir() bool      { return false }
func (f fakeFileInfo) Sys() interface{} { return nil }

func TestDetectDocumentEnginesPrefersFixedX2TPath(t *testing.T) {
	locator := commandLocator{
		lookPath: func(name string) (string, error) {
			if name == documentEngineX2T {
				return "/usr/local/bin/x2t", nil
			}
			return "", exec.ErrNotFound
		},
		stat: func(path string) (fs.FileInfo, error) {
			if path == x2tFixedPath {
				return fakeFileInfo{name: "x2t"}, nil
			}
			return nil, os.ErrNotExist
		},
		runVersion: func(ctx context.Context, name string, args ...string) (string, error) {
			return "", errors.New("unsupported")
		},
	}

	engines := detectDocumentEngines(context.Background(), "linux", locator)
	x2t := mustFindEngine(t, engines, documentEngineX2T)
	if !x2t.Available {
		t.Fatalf("x2t should be available")
	}
	if x2t.Path != x2tFixedPath {
		t.Fatalf("x2t path = %q, want %q", x2t.Path, x2tFixedPath)
	}
}

func TestDetectDocumentEnginesFallsBackToPathX2T(t *testing.T) {
	locator := commandLocator{
		lookPath: func(name string) (string, error) {
			if name == documentEngineX2T {
				return "/opt/bin/x2t", nil
			}
			return "", exec.ErrNotFound
		},
		stat: func(path string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		runVersion: func(ctx context.Context, name string, args ...string) (string, error) {
			return "", errors.New("unsupported")
		},
	}

	engines := detectDocumentEngines(context.Background(), "linux", locator)
	x2t := mustFindEngine(t, engines, documentEngineX2T)
	if !x2t.Available {
		t.Fatalf("x2t should be available")
	}
	if x2t.Path != "/opt/bin/x2t" {
		t.Fatalf("x2t path = %q, want /opt/bin/x2t", x2t.Path)
	}
}

func TestDetectDocumentEnginesDetectsOpenOfficeFromSofficeVersion(t *testing.T) {
	locator := commandLocator{
		lookPath: func(name string) (string, error) {
			if name == "soffice" {
				return "/usr/bin/soffice", nil
			}
			return "", exec.ErrNotFound
		},
		stat: func(path string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		runVersion: func(ctx context.Context, name string, args ...string) (string, error) {
			if name == "/usr/bin/soffice" {
				return "OpenOffice 4.1.15", nil
			}
			return "", errors.New("unexpected command")
		},
	}

	engines := detectDocumentEngines(context.Background(), "linux", locator)
	openoffice := mustFindEngine(t, engines, documentEngineOpenOffice)
	if !openoffice.Available {
		t.Fatalf("openoffice should be available")
	}
	libre := mustFindEngine(t, engines, documentEngineLibreOffice)
	if libre.Available {
		t.Fatalf("libreoffice should not be marked available when vendor is openoffice")
	}
}

func TestAvailableDocumentEnginesPriorityOrder(t *testing.T) {
	locator := commandLocator{
		lookPath: func(name string) (string, error) {
			switch name {
			case documentEngineX2T:
				return "/tmp/x2t", nil
			case "libreoffice":
				return "/tmp/libreoffice", nil
			case documentEngineUnoconv:
				return "/tmp/unoconv", nil
			case documentEnginePandoc:
				return "/tmp/pandoc", nil
			case documentEngineTextutil:
				return "/tmp/textutil", nil
			default:
				return "", exec.ErrNotFound
			}
		},
		stat: func(path string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		runVersion: func(ctx context.Context, name string, args ...string) (string, error) {
			if name == "/tmp/libreoffice" {
				return "LibreOffice 24.2", nil
			}
			return "", nil
		},
	}

	all := detectDocumentEngines(context.Background(), "darwin", locator)
	available := availableDocumentEngines(all)
	var ids []string
	for _, engine := range available {
		ids = append(ids, engine.ID)
	}
	got := strings.Join(ids, ",")
	want := strings.Join([]string{documentEngineX2T, documentEngineLibreOffice, documentEngineUnoconv, documentEnginePandoc, documentEngineTextutil}, ",")
	if got != want {
		t.Fatalf("priority order mismatch: got %s want %s", got, want)
	}
}

func TestEngineSupportsConversionCrossFamily(t *testing.T) {
	if !engineSupportsConversion(documentEngineX2T, "docx", "xlsx") {
		t.Fatalf("x2t should allow cross-family office conversion")
	}
	if !engineSupportsConversion(documentEngineLibreOffice, "pptx", "docx") {
		t.Fatalf("libreoffice should allow cross-family office conversion")
	}
	if engineSupportsConversion(documentEnginePandoc, "docx", "xlsx") {
		t.Fatalf("pandoc should not allow docx->xlsx")
	}
}

func TestEngineSupportsConversionTextutilMarkdownAndTextAlias(t *testing.T) {
	if engineSupportsConversion(documentEngineTextutil, "md", "pdf") {
		t.Fatalf("textutil should not claim md->pdf support")
	}
	if !engineSupportsConversion(documentEngineTextutil, "md", "text") {
		t.Fatalf("textutil should allow md->text alias")
	}
	if !engineSupportsConversion(documentEngineTextutil, "text", "rtf") {
		t.Fatalf("textutil should allow .text source alias")
	}
}

func TestConvertDocumentFallsBackToLowerPriorityEngine(t *testing.T) {
	svc := setupConvertTestService(t)
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source.docx")
	if err := os.WriteFile(sourcePath, []byte("dummy"), 0o640); err != nil {
		t.Fatalf("write source: %v", err)
	}

	x2tPath := filepath.Join(tmpDir, "x2t")
	unoconvPath := filepath.Join(tmpDir, "unoconv")
	if err := os.WriteFile(x2tPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write x2t script: %v", err)
	}
	unoconvScript := "#!/bin/sh\nout=\"\"\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = \"-o\" ]; then\n    out=\"$2\"\n    shift 2\n    continue\n  fi\n  shift\ndone\nif [ -z \"$out\" ]; then\n  exit 2\nfi\nprintf 'converted' > \"$out\"\n"
	if err := os.WriteFile(unoconvPath, []byte(unoconvScript), 0o755); err != nil {
		t.Fatalf("write unoconv script: %v", err)
	}

	svc.locator = commandLocator{
		lookPath: func(name string) (string, error) {
			switch name {
			case documentEngineX2T:
				return x2tPath, nil
			case documentEngineUnoconv:
				return unoconvPath, nil
			default:
				return "", exec.ErrNotFound
			}
		},
		stat: func(path string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		runVersion: func(ctx context.Context, name string, args ...string) (string, error) {
			return "", nil
		},
	}

	task := &ConvertTask{ID: "task-fallback"}
	source := ResolvedSource{Name: filepath.Base(sourcePath), Path: sourcePath, Category: "document"}
	outputs, message, err := svc.convertDocument(context.Background(), task, source, "pdf")
	if err != nil {
		t.Fatalf("convertDocument failed: %v", err)
	}
	if message != "Document converted" {
		t.Fatalf("message = %q, want %q", message, "Document converted")
	}
	if len(outputs) != 1 {
		t.Fatalf("outputs = %d, want 1", len(outputs))
	}
	if !strings.HasSuffix(outputs[0].Name, ".pdf") {
		t.Fatalf("output name = %q, want .pdf suffix", outputs[0].Name)
	}
}

func TestConvertDocumentFallsBackToHelperPDFWhenNoEngineSupportsSource(t *testing.T) {
	svc := setupConvertTestService(t)
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source.md")
	if err := os.WriteFile(sourcePath, []byte("# hello\n\nfrom helper fallback"), 0o640); err != nil {
		t.Fatalf("write source: %v", err)
	}

	helperPath := filepath.Join(tmpDir, "blue-convert-helper")
	helperScript := "#!/bin/sh\nreq=\"$1\"\noutdir=$(sed -n 's/.*\"output_dir\":\"\\([^\"]*\\)\".*/\\1/p' \"$req\" | head -n 1)\nif [ -z \"$outdir\" ]; then\n  echo '{\"error\":\"missing output_dir\"}'\n  exit 1\nfi\nmkdir -p \"$outdir\"\nout=\"$outdir/source.pdf\"\nprintf 'pdf' > \"$out\"\nprintf '{\"outputs\":[{\"path\":\"%s\",\"name\":\"source.pdf\",\"preview_kind\":\"pdf\"}]}\n' \"$out\"\n"
	if err := os.WriteFile(helperPath, []byte(helperScript), 0o755); err != nil {
		t.Fatalf("write helper script: %v", err)
	}
	t.Setenv("BLUE_CONVERT_HELPER", helperPath)

	svc.locator = commandLocator{
		lookPath: func(name string) (string, error) {
			return "", exec.ErrNotFound
		},
		stat: func(path string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		runVersion: func(ctx context.Context, name string, args ...string) (string, error) {
			return "", nil
		},
	}

	task := &ConvertTask{ID: "task-helper-fallback"}
	source := ResolvedSource{Name: filepath.Base(sourcePath), Path: sourcePath, Category: "document"}
	outputs, message, err := svc.convertDocument(context.Background(), task, source, "pdf")
	if err != nil {
		t.Fatalf("convertDocument failed: %v", err)
	}
	if message != "Document rendered to PDF" {
		t.Fatalf("message = %q, want %q", message, "Document rendered to PDF")
	}
	if len(outputs) != 1 {
		t.Fatalf("outputs = %d, want 1", len(outputs))
	}
	if !strings.HasSuffix(outputs[0].Name, ".pdf") {
		t.Fatalf("output name = %q, want .pdf suffix", outputs[0].Name)
	}
}

func TestConvertDocumentMarkdownToPDFFallsBackWithoutCallingTextutil(t *testing.T) {
	svc := setupConvertTestService(t)
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source.md")
	if err := os.WriteFile(sourcePath, []byte("# hello\n\nfrom helper fallback"), 0o640); err != nil {
		t.Fatalf("write source: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "textutil-called")
	textutilPath := filepath.Join(tmpDir, "textutil")
	textutilScript := "#!/bin/sh\nprintf called > \"" + markerPath + "\"\nexit 42\n"
	if err := os.WriteFile(textutilPath, []byte(textutilScript), 0o755); err != nil {
		t.Fatalf("write textutil script: %v", err)
	}

	helperPath := filepath.Join(tmpDir, "blue-convert-helper")
	helperScript := "#!/bin/sh\nreq=\"$1\"\noutdir=$(sed -n 's/.*\"output_dir\":\"\\([^\"]*\\)\".*/\\1/p' \"$req\" | head -n 1)\nif [ -z \"$outdir\" ]; then\n  echo '{\"error\":\"missing output_dir\"}'\n  exit 1\nfi\nmkdir -p \"$outdir\"\nout=\"$outdir/source.pdf\"\nprintf 'pdf' > \"$out\"\nprintf '{\"outputs\":[{\"path\":\"%s\",\"name\":\"source.pdf\",\"preview_kind\":\"pdf\"}]}\n' \"$out\"\n"
	if err := os.WriteFile(helperPath, []byte(helperScript), 0o755); err != nil {
		t.Fatalf("write helper script: %v", err)
	}
	t.Setenv("BLUE_CONVERT_HELPER", helperPath)

	svc.locator = commandLocator{
		lookPath: func(name string) (string, error) {
			if name == documentEngineTextutil {
				return textutilPath, nil
			}
			return "", exec.ErrNotFound
		},
		stat: func(path string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		runVersion: func(ctx context.Context, name string, args ...string) (string, error) {
			return "", nil
		},
	}

	task := &ConvertTask{ID: "task-helper-textutil-skip"}
	source := ResolvedSource{Name: filepath.Base(sourcePath), Path: sourcePath, Category: "document"}
	outputs, message, err := svc.convertDocument(context.Background(), task, source, "pdf")
	if err != nil {
		t.Fatalf("convertDocument failed: %v", err)
	}
	if message != "Document rendered to PDF" {
		t.Fatalf("message = %q, want %q", message, "Document rendered to PDF")
	}
	if len(outputs) != 1 {
		t.Fatalf("outputs = %d, want 1", len(outputs))
	}
	if _, err := os.Stat(markerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected textutil to be skipped, stat err = %v", err)
	}
}

func TestConvertDocumentUsesHelperPDFForSpreadsheetFormats(t *testing.T) {
	svc := setupConvertTestService(t)
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "sheet.xlsx")
	if err := os.WriteFile(sourcePath, []byte("dummy spreadsheet"), 0o640); err != nil {
		t.Fatalf("write source: %v", err)
	}

	helperPath := filepath.Join(tmpDir, "blue-convert-helper")
	helperScript := "#!/bin/sh\nreq=\"$1\"\noutdir=$(sed -n 's/.*\"output_dir\":\"\\([^\"]*\\)\".*/\\1/p' \"$req\" | head -n 1)\nif [ -z \"$outdir\" ]; then\n  echo '{\"error\":\"missing output_dir\"}'\n  exit 1\nfi\nmkdir -p \"$outdir\"\nout=\"$outdir/sheet.pdf\"\nprintf 'pdf' > \"$out\"\nprintf '{\"outputs\":[{\"path\":\"%s\",\"name\":\"sheet.pdf\",\"preview_kind\":\"pdf\"}]}\n' \"$out\"\n"
	if err := os.WriteFile(helperPath, []byte(helperScript), 0o755); err != nil {
		t.Fatalf("write helper script: %v", err)
	}
	t.Setenv("BLUE_CONVERT_HELPER", helperPath)

	svc.locator = commandLocator{
		lookPath: func(name string) (string, error) {
			return "", exec.ErrNotFound
		},
		stat: func(path string) (fs.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		runVersion: func(ctx context.Context, name string, args ...string) (string, error) {
			return "", nil
		},
	}

	task := &ConvertTask{ID: "task-helper-spreadsheet"}
	source := ResolvedSource{Name: filepath.Base(sourcePath), Path: sourcePath, Category: "document"}
	outputs, message, err := svc.convertDocument(context.Background(), task, source, "pdf")
	if err != nil {
		t.Fatalf("convertDocument failed: %v", err)
	}
	if message != "Document rendered to PDF" {
		t.Fatalf("message = %q, want %q", message, "Document rendered to PDF")
	}
	if len(outputs) != 1 {
		t.Fatalf("outputs = %d, want 1", len(outputs))
	}
	if outputs[0].Name != "sheet.pdf" {
		t.Fatalf("output name = %q, want sheet.pdf", outputs[0].Name)
	}
}

func TestSupportedActionsForPlatform(t *testing.T) {
	linuxActions := supportedActionsForPlatform("linux", true, false, false)
	if strings.Join(linuxActions, ",") != ActionConvert {
		t.Fatalf("linux actions = %v, want [%s]", linuxActions, ActionConvert)
	}
	linuxNoEngines := supportedActionsForPlatform("linux", false, false, false)
	if len(linuxNoEngines) != 0 {
		t.Fatalf("linux actions without engines = %v, want []", linuxNoEngines)
	}
	darwinActions := supportedActionsForPlatform("darwin", false, true, true)
	if !stringInSlice(ActionConvert, darwinActions) || !stringInSlice(ActionTTS, darwinActions) || !stringInSlice(ActionASR, darwinActions) {
		t.Fatalf("darwin actions missing expected entries: %v", darwinActions)
	}
}

func mustFindEngine(t *testing.T, engines []DocumentEngineInfo, id string) DocumentEngineInfo {
	t.Helper()
	for _, engine := range engines {
		if engine.ID == id {
			return engine
		}
	}
	t.Fatalf("engine %q not found in %+v", id, engines)
	return DocumentEngineInfo{}
}
