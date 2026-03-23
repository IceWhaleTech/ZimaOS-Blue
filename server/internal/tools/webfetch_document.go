package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

func isLikelyWebFetchDocument(contentType, targetURL string) bool {
	return detectWebFetchDocumentFormat(contentType, targetURL, nil) != ""
}

func detectWebFetchDocumentFormat(contentType, targetURL string, body []byte) string {
	if format := detectWebFetchDocumentFormatFromContentType(contentType); format != "" {
		return format
	}
	if format := detectWebFetchDocumentFormatFromURL(targetURL); format != "" {
		return format
	}
	if len(body) > 0 {
		return detectWebFetchDocumentFormatFromBytes(body)
	}
	return ""
}

func detectWebFetchDocumentFormatFromContentType(contentType string) string {
	normalized := strings.ToLower(normalizeContentType(contentType))
	switch {
	case strings.Contains(normalized, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"):
		return "docx"
	case strings.Contains(normalized, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"):
		return "xlsx"
	case strings.Contains(normalized, "application/vnd.openxmlformats-officedocument.presentationml.presentation"):
		return "pptx"
	case strings.Contains(normalized, "application/msword"):
		return "doc"
	case strings.Contains(normalized, "application/vnd.ms-excel"):
		return "xls"
	case strings.Contains(normalized, "application/vnd.ms-powerpoint"):
		return "ppt"
	case strings.Contains(normalized, "application/vnd.oasis.opendocument.text"):
		return "odt"
	case strings.Contains(normalized, "application/vnd.oasis.opendocument.spreadsheet"):
		return "ods"
	case strings.Contains(normalized, "application/vnd.oasis.opendocument.presentation"):
		return "odp"
	case strings.Contains(normalized, "application/rtf"), strings.Contains(normalized, "text/rtf"):
		return "rtf"
	default:
		return ""
	}
}

func detectWebFetchDocumentFormatFromURL(targetURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil {
		return ""
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed.Path)), ".")
	if !convertpkg.SupportsDocumentReadFormat(ext) {
		return ""
	}
	return convertpkg.NormalizeDocumentReadFormat(ext)
}

func detectWebFetchDocumentFormatFromBytes(body []byte) string {
	if len(body) < 4 || !bytes.HasPrefix(body, []byte("PK")) {
		return ""
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return ""
	}

	hasEntry := func(target string) bool {
		target = strings.ToLower(strings.TrimSpace(target))
		for _, file := range reader.File {
			if strings.ToLower(strings.TrimSpace(file.Name)) == target {
				return true
			}
		}
		return false
	}

	switch {
	case hasEntry("word/document.xml"):
		return "docx"
	case hasEntry("xl/workbook.xml"):
		return "xlsx"
	case hasEntry("ppt/presentation.xml") || hasEntry("ppt/slides/slide1.xml"):
		return "pptx"
	default:
		return ""
	}
}

func (w *WebFetchTool) extractDocumentContent(ctx context.Context, sourceURL, _ /* contentType */, format string, body []byte) (content, title, extractor string, truncated bool, err error) {
	if w == nil || w.documentReader == nil {
		return "", "", "", false, errors.New("document extraction service not available for web fetch")
	}
	if !convertpkg.SupportsDocumentReadFormat(format) {
		return "", "", "", false, fmt.Errorf("unsupported document format %q", format)
	}

	file, err := os.CreateTemp("", "zimaos-blue-webfetch-*."+format)
	if err != nil {
		return "", "", "", false, fmt.Errorf("create temp document: %w", err)
	}
	path := file.Name()
	defer func() {
		_ = os.Remove(path)
	}()
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return "", "", "", false, fmt.Errorf("write temp document: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", "", "", false, fmt.Errorf("close temp document: %w", err)
	}

	result, err := w.documentReader.ReadDocument(ctx, path)
	if err != nil {
		return "", "", "", false, fmt.Errorf("extract document: %w", err)
	}
	if result == nil || strings.TrimSpace(result.Text) == "" {
		return "", "", "", false, errors.New("document extraction returned empty content")
	}

	return strings.TrimSpace(result.Text), pdfDisplayName(sourceURL), "document", false, nil
}
