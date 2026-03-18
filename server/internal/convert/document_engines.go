package convert

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	documentEngineX2T         = "x2t"
	documentEngineLibreOffice = "libreoffice"
	documentEngineOpenOffice  = "openoffice"
	documentEngineUnoconv     = "unoconv"
	documentEnginePandoc      = "pandoc"
	documentEngineTextutil    = "textutil"
)

const x2tFixedPath = "/usr/bin/x2t/x2t"

type commandLocator struct {
	lookPath   func(string) (string, error)
	stat       func(string) (fs.FileInfo, error)
	runVersion func(context.Context, string, ...string) (string, error)
}

func defaultCommandLocator() commandLocator {
	return commandLocator{
		lookPath: exec.LookPath,
		stat: func(path string) (fs.FileInfo, error) {
			return os.Stat(path)
		},
		runVersion: runCommandOutput,
	}
}

func detectDocumentEngines(ctx context.Context, goos string, locator commandLocator) []DocumentEngineInfo {
	if locator.lookPath == nil {
		locator.lookPath = exec.LookPath
	}
	if locator.stat == nil {
		locator.stat = func(path string) (fs.FileInfo, error) { return os.Stat(path) }
	}
	if locator.runVersion == nil {
		locator.runVersion = runCommandOutput
	}

	engines := []DocumentEngineInfo{
		{ID: documentEngineX2T, Priority: 10, Formats: append([]string(nil), x2tOfficeFormats...)},
		{ID: documentEngineLibreOffice, Priority: 20, Formats: append([]string(nil), officeDocumentFormats...)},
		{ID: documentEngineOpenOffice, Priority: 30, Formats: append([]string(nil), officeDocumentFormats...)},
		{ID: documentEngineUnoconv, Priority: 40, Formats: append([]string(nil), officeDocumentFormats...)},
		{ID: documentEnginePandoc, Priority: 50, Formats: append([]string(nil), pandocFormats...)},
	}
	if goos == "darwin" {
		engines = append(engines, DocumentEngineInfo{ID: documentEngineTextutil, Priority: 60, Formats: append([]string(nil), textutilFormats...)})
	}

	if path, ok := resolveX2TPath(locator); ok {
		setEngineAvailable(engines, documentEngineX2T, path)
	}

	officePath, officeVendor := resolveOfficeBinary(ctx, locator)
	if officePath != "" {
		if officeVendor == documentEngineOpenOffice {
			setEngineAvailable(engines, documentEngineOpenOffice, officePath)
		} else {
			setEngineAvailable(engines, documentEngineLibreOffice, officePath)
		}
	}

	if path, err := locator.lookPath(documentEngineUnoconv); err == nil && strings.TrimSpace(path) != "" {
		setEngineAvailable(engines, documentEngineUnoconv, path)
	}
	if path, err := locator.lookPath(documentEnginePandoc); err == nil && strings.TrimSpace(path) != "" {
		setEngineAvailable(engines, documentEnginePandoc, path)
	}
	if goos == "darwin" {
		if path, err := locator.lookPath(documentEngineTextutil); err == nil && strings.TrimSpace(path) != "" {
			setEngineAvailable(engines, documentEngineTextutil, path)
		}
	}

	sort.SliceStable(engines, func(i, j int) bool {
		if engines[i].Priority == engines[j].Priority {
			return engines[i].ID < engines[j].ID
		}
		return engines[i].Priority < engines[j].Priority
	})
	return engines
}

func setEngineAvailable(engines []DocumentEngineInfo, id, path string) {
	for i := range engines {
		if engines[i].ID != id {
			continue
		}
		engines[i].Available = true
		engines[i].Path = path
		return
	}
}

func resolveX2TPath(locator commandLocator) (string, bool) {
	if info, err := locator.stat(x2tFixedPath); err == nil && info != nil && !info.IsDir() {
		return x2tFixedPath, true
	}
	if path, err := locator.lookPath(documentEngineX2T); err == nil && strings.TrimSpace(path) != "" {
		return path, true
	}
	return "", false
}

func resolveOfficeBinary(ctx context.Context, locator commandLocator) (path string, vendor string) {
	candidates := []string{"libreoffice", "soffice", "openoffice", "openoffice4"}
	for _, candidate := range candidates {
		resolved, err := locator.lookPath(candidate)
		if err != nil || strings.TrimSpace(resolved) == "" {
			continue
		}
		vendor = officeVendorFromCandidate(candidate)
		if version, err := locator.runVersion(ctx, resolved, "--version"); err == nil {
			versionLower := strings.ToLower(version)
			switch {
			case strings.Contains(versionLower, "openoffice"):
				vendor = documentEngineOpenOffice
			case strings.Contains(versionLower, "libreoffice"):
				vendor = documentEngineLibreOffice
			}
		}
		return resolved, vendor
	}
	return "", ""
}

func officeVendorFromCandidate(candidate string) string {
	switch candidate {
	case "openoffice", "openoffice4":
		return documentEngineOpenOffice
	default:
		return documentEngineLibreOffice
	}
}

func runCommandOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("%s failed: %w (%s)", name, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func availableDocumentEngines(engines []DocumentEngineInfo) []DocumentEngineInfo {
	available := make([]DocumentEngineInfo, 0, len(engines))
	for _, engine := range engines {
		if !engine.Available {
			continue
		}
		available = append(available, engine)
	}
	sort.SliceStable(available, func(i, j int) bool {
		if available[i].Priority == available[j].Priority {
			return available[i].ID < available[j].ID
		}
		return available[i].Priority < available[j].Priority
	})
	return available
}

func aggregateDocumentFormats(engines []DocumentEngineInfo) map[string][]string {
	out := make(map[string][]string)
	for _, engine := range engines {
		if !engine.Available {
			continue
		}
		out[engine.ID] = sortedUniqueStrings(engine.Formats)
	}
	return out
}

func hasAvailableDocumentEngine(engines []DocumentEngineInfo) bool {
	for _, engine := range engines {
		if engine.Available {
			return true
		}
	}
	return false
}

func engineSupportsConversion(engineID, sourceExt, targetExt string) bool {
	sourceExt = normalizeFormat(sourceExt, "")
	targetExt = normalizeFormat(targetExt, "")
	if sourceExt == "" || targetExt == "" {
		return false
	}

	switch engineID {
	case documentEngineX2T:
		return stringInSlice(sourceExt, x2tOfficeFormats) && stringInSlice(targetExt, x2tOfficeFormats)
	case documentEngineLibreOffice, documentEngineOpenOffice, documentEngineUnoconv:
		return stringInSlice(sourceExt, officeDocumentFormats) && stringInSlice(targetExt, officeDocumentFormats)
	case documentEnginePandoc:
		return stringInSlice(sourceExt, pandocFormats) && stringInSlice(targetExt, pandocFormats)
	case documentEngineTextutil:
		return stringInSlice(sourceExt, textutilFormats) && stringInSlice(targetExt, textutilFormats)
	default:
		return false
	}
}

func stringInSlice(value string, values []string) bool {
	for _, item := range values {
		if normalizeFormat(item, "") == value {
			return true
		}
	}
	return false
}

func officeOutputPath(outputDir, sourcePath, target string) (string, error) {
	base := trimExt(filepath.Base(sourcePath))
	expected := filepath.Join(outputDir, base+"."+target)
	if _, err := os.Stat(expected); err == nil {
		return expected, nil
	}

	matches, err := filepath.Glob(filepath.Join(outputDir, base+".*"))
	if err != nil {
		return "", err
	}
	for _, match := range matches {
		ext := normalizeFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(match)), "."), "")
		if ext == target {
			return match, nil
		}
	}
	for _, match := range matches {
		ext := normalizeFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(match)), "."), "")
		if (target == "html" && ext == "htm") || (target == "htm" && ext == "html") {
			return match, nil
		}
	}
	return "", fmt.Errorf("office conversion produced no output for target %q", target)
}

var (
	officeDocumentFormats    = []string{"doc", "docx", "odt", "rtf", "txt", "html", "htm", "md", "xls", "xlsx", "ods", "csv", "tsv", "ppt", "pptx", "odp", "pdf"}
	x2tOfficeFormats         = []string{"doc", "docx", "odt", "rtf", "xls", "xlsx", "ods", "csv", "tsv", "ppt", "pptx", "odp", "pdf"}
	pandocFormats            = []string{"txt", "md", "html", "htm", "rtf", "docx", "odt", "pdf"}
	textutilFormats          = []string{"txt", "rtf", "rtfd", "html", "doc", "docx", "odt", "wordml", "webarchive", "pdf"}
	helperPDFFallbackFormats = []string{"txt", "md", "rtf", "rtfd", "html", "htm", "doc", "docx", "odt", "wordml", "webarchive", "csv", "tsv", "xls", "xlsx", "ods", "ppt", "pptx", "odp"}
)

func helperSupportsDocumentPDFFallback(sourceExt, targetExt string) bool {
	sourceExt = normalizeFormat(sourceExt, "")
	targetExt = normalizeFormat(targetExt, "")
	return targetExt == "pdf" && stringInSlice(sourceExt, helperPDFFallbackFormats)
}
