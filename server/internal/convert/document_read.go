package convert

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/net/html"
)

// DocumentReadResult is the normalized read-only extraction payload used by the
// workspace read tool for office-style documents.
type DocumentReadResult struct {
	Format         string                  `json:"format"`
	Text           string                  `json:"text"`
	TabularSummary *TabularWorkbookSummary `json:"tabular_summary,omitempty"`
	ExtractedVia   string                  `json:"extracted_via,omitempty"`
}

// DocumentReader extracts readable text from local office-style documents
// without exposing convert tasks/output artifacts to callers.
type DocumentReader struct {
	locator commandLocator
	tempDir string
}

// NewDocumentReader creates a default document reader backed by local parsers
// and detected document engines.
func NewDocumentReader() *DocumentReader {
	return &DocumentReader{locator: defaultCommandLocator()}
}

// NormalizeDocumentReadFormat normalizes office-style document extensions into
// the extraction families understood by the reader.
func NormalizeDocumentReadFormat(raw string) string {
	value := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), ".")
	value = normalizeFormat(value, "")
	switch value {
	case "docm", "dotx", "dotm":
		return "docx"
	case "xlsm", "xltx", "xltm":
		return "xlsx"
	case "pptm", "potx", "potm", "ppsx", "ppsm":
		return "pptx"
	default:
		return value
	}
}

// SupportsDocumentReadFormat reports whether the reader can attempt a readable
// extraction for the provided extension.
func SupportsDocumentReadFormat(raw string) bool {
	switch NormalizeDocumentReadFormat(raw) {
	case "doc", "docx", "odt", "rtf", "xls", "xlsx", "ods", "ppt", "pptx", "odp":
		return true
	default:
		return false
	}
}

// ReadDocument extracts readable text from a local office-style document.
func (r *DocumentReader) ReadDocument(ctx context.Context, path string) (*DocumentReadResult, error) {
	if r == nil {
		return nil, fmt.Errorf("document reader unavailable")
	}
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" {
		return nil, fmt.Errorf("document path is required")
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("document path is a directory")
	}

	inputExt := normalizeFormat(docExt(cleanPath), "")
	ext := NormalizeDocumentReadFormat(inputExt)
	switch ext {
	case "xlsx":
		workbook, err := loadSpreadsheetWorkbook(cleanPath)
		if err == nil {
			return &DocumentReadResult{
				Format:         firstNonEmptyDocumentValue(inputExt, ext),
				Text:           strings.TrimSpace(spreadsheetTextContent(workbook)),
				TabularSummary: workbook.Summary,
				ExtractedVia:   "local_spreadsheet",
			}, nil
		}
	case "docx":
		if text, err := readDOCXLocal(cleanPath); err == nil && strings.TrimSpace(text) != "" {
			return &DocumentReadResult{
				Format:       firstNonEmptyDocumentValue(inputExt, ext),
				Text:         text,
				ExtractedVia: "local_docx",
			}, nil
		}
	case "pptx":
		if text, err := readPPTXLocal(cleanPath); err == nil && strings.TrimSpace(text) != "" {
			return &DocumentReadResult{
				Format:       firstNonEmptyDocumentValue(inputExt, ext),
				Text:         text,
				ExtractedVia: "local_pptx",
			}, nil
		}
	}
	if !SupportsDocumentReadFormat(inputExt) {
		return nil, fmt.Errorf("document read does not support .%s files", inputExt)
	}
	return r.readConvertedDocument(ctx, cleanPath, inputExt, documentReadConversionTargets(ext))
}

func (r *DocumentReader) readConvertedDocument(ctx context.Context, sourcePath, sourceExt string, targets []string) (*DocumentReadResult, error) {
	locator := r.locator
	if locator.lookPath == nil || locator.stat == nil {
		locator = defaultCommandLocator()
	}
	engines := availableDocumentEngines(detectDocumentEngines(ctx, runtime.GOOS, locator))
	if len(engines) == 0 {
		return nil, fmt.Errorf("no document conversion engine is available on this host")
	}

	tempRoot := strings.TrimSpace(r.tempDir)
	tempDir, err := os.MkdirTemp(tempRoot, "zimaos-blue-docread-*")
	if err != nil {
		return nil, fmt.Errorf("create document read temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	base := trimExt(filepath.Base(sourcePath))
	if base == "" {
		base = "document"
	}

	attempts := make([]string, 0, len(engines)*len(targets))
	normalizedSourceExt := NormalizeDocumentReadFormat(sourceExt)
	for _, target := range targets {
		outputPath := filepath.Join(tempDir, base+"."+target)
		for _, engine := range engines {
			if !engineSupportsConversion(engine.ID, normalizedSourceExt, target) {
				continue
			}
			_ = os.Remove(outputPath)
			if err := runDocumentConversionWithEngine(ctx, engine, sourcePath, outputPath, target); err != nil {
				attempts = append(attempts, fmt.Sprintf("%s->%s: %v", engine.ID, target, err))
				continue
			}
			data, err := os.ReadFile(outputPath)
			if err != nil {
				attempts = append(attempts, fmt.Sprintf("%s->%s: read output: %v", engine.ID, target, err))
				continue
			}
			text := strings.TrimSpace(documentTextFromFormat(target, data))
			if text == "" {
				attempts = append(attempts, fmt.Sprintf("%s->%s: empty output", engine.ID, target))
				continue
			}
			return &DocumentReadResult{
				Format:       firstNonEmptyDocumentValue(normalizeFormat(sourceExt, ""), normalizedSourceExt),
				Text:         text,
				ExtractedVia: fmt.Sprintf("%s:%s", engine.ID, target),
			}, nil
		}
	}
	if len(attempts) == 0 {
		return nil, fmt.Errorf("no document conversion engine supports readable extraction for .%s", normalizedSourceExt)
	}
	return nil, fmt.Errorf("document read is unsupported for .%s on this host (%s)", normalizedSourceExt, strings.Join(attempts, "; "))
}

func documentReadConversionTargets(ext string) []string {
	switch NormalizeDocumentReadFormat(ext) {
	case "xls", "xlsx", "ods":
		return []string{"csv", "txt", "html"}
	case "ppt", "pptx", "odp":
		return []string{"txt", "html"}
	default:
		return []string{"txt", "md", "html"}
	}
}

func firstNonEmptyDocumentValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func documentTextFromFormat(format string, data []byte) string {
	switch normalizeFormat(format, "") {
	case "html":
		return htmlToPlainText(data)
	default:
		return string(bytes.TrimSpace(data))
	}
}

func htmlToPlainText(data []byte) string {
	root, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return string(data)
	}
	var buf bytes.Buffer
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				if buf.Len() > 0 {
					buf.WriteString("\n")
				}
				buf.WriteString(text)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return strings.TrimSpace(buf.String())
}
