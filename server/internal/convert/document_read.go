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

	switch ext := normalizeFormat(docExt(cleanPath), ""); ext {
	case "xlsx":
		workbook, err := loadSpreadsheetWorkbook(cleanPath)
		if err != nil {
			return nil, err
		}
		return &DocumentReadResult{
			Format:         ext,
			Text:           strings.TrimSpace(spreadsheetTextContent(workbook)),
			TabularSummary: workbook.Summary,
			ExtractedVia:   "local_spreadsheet",
		}, nil
	case "docx":
		return r.readConvertedDocument(ctx, cleanPath, ext, []string{"txt", "md", "html"})
	case "pptx":
		return r.readConvertedDocument(ctx, cleanPath, ext, []string{"txt", "html"})
	default:
		return nil, fmt.Errorf("document read does not support .%s files", ext)
	}
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
	for _, target := range targets {
		outputPath := filepath.Join(tempDir, base+"."+target)
		for _, engine := range engines {
			if !engineSupportsConversion(engine.ID, sourceExt, target) {
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
				Format:       sourceExt,
				Text:         text,
				ExtractedVia: fmt.Sprintf("%s:%s", engine.ID, target),
			}, nil
		}
	}
	if len(attempts) == 0 {
		return nil, fmt.Errorf("no document conversion engine supports readable extraction for .%s", sourceExt)
	}
	return nil, fmt.Errorf("document read is unsupported for .%s on this host (%s)", sourceExt, strings.Join(attempts, "; "))
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
