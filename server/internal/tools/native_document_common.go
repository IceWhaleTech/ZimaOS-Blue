package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

type nativeDocumentPayload struct {
	Action         string                 `json:"action"`
	Path           string                 `json:"path"`
	AbsolutePath   string                 `json:"absolute_path,omitempty"`
	OriginalPath   string                 `json:"original_path,omitempty"`
	Format         string                 `json:"format"`
	Theme          string                 `json:"theme,omitempty"`
	ThemePreview   *officeThemePreview    `json:"theme_preview,omitempty"`
	Engine         string                 `json:"engine"`
	EngineChain    []string               `json:"engine_chain"`
	Degraded       bool                   `json:"degraded"`
	FallbackReason string                 `json:"fallback_reason,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
	Validation     map[string]interface{} `json:"validation,omitempty"`
	Text           string                 `json:"text,omitempty"`
	Summary        string                 `json:"summary,omitempty"`
	Result         interface{}            `json:"result,omitempty"`
	Size           int64                  `json:"size,omitempty"`
	ExtractedVia   string                 `json:"extracted_via,omitempty"`
	TabularSummary interface{}            `json:"tabular_summary,omitempty"`
	SlideCount     int                    `json:"slide_count,omitempty"`
	SheetCount     int                    `json:"sheet_count,omitempty"`
	RowCount       int                    `json:"row_count,omitempty"`
	Success        bool                   `json:"success"`
}

type zipArchiveEntry struct {
	Name   string
	Data   []byte
	Method uint16
}

func marshalNativeDocumentPayload(payload nativeDocumentPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func attachOfficeThemeMetadata(payload *nativeDocumentPayload, theme officeTheme) {
	if payload == nil {
		return
	}
	if strings.TrimSpace(theme.Name) == "" {
		return
	}
	payload.Theme = theme.Name
	if preview, err := GetThemePreview(theme.Name); err == nil {
		payload.ThemePreview = &preview
	}
}

func nativeDocumentToolNameForPath(path string) string {
	switch strings.ToLower(strings.TrimSpace(filepath.Ext(path))) {
	case ".docx":
		return "docx"
	case ".xlsx":
		return "xlsx"
	case ".pptx":
		return "pptx"
	case ".pdf":
		return "pdf"
	default:
		return ""
	}
}

func nativeDocumentEngineForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "docx":
		return "native_docx_ooxml"
	case "xlsx":
		return "native_xlsx_ooxml"
	case "pptx":
		return "native_pptx_ooxml"
	case "pdf":
		return "native_pdf_ir"
	default:
		return "native_document"
	}
}

func buildReadPayloadFromDocumentResult(action, relPath, absPath, format, inputExt string, size int64, result *convertpkg.DocumentReadResult, warnings []string) nativeDocumentPayload {
	nativeEngine := nativeDocumentEngineForFormat(format)
	finalEngine := nativeEngine
	degraded := false
	fallbackReason := ""
	extractedVia := ""
	if result != nil {
		extractedVia = strings.TrimSpace(result.ExtractedVia)
		if extractedVia != "" && !strings.HasPrefix(extractedVia, "local_") {
			finalEngine = extractedVia
			degraded = true
			switch {
			case strings.TrimPrefix(strings.ToLower(strings.TrimSpace(inputExt)), ".") != format:
				fallbackReason = "legacy_input_requires_conversion"
			default:
				fallbackReason = "native_read_failed"
			}
		}
	}
	engineChain := []string{nativeEngine}
	if degraded {
		engineChain = append(engineChain, finalEngine)
	}
	payload := nativeDocumentPayload{
		Action:         action,
		Path:           relPath,
		AbsolutePath:   absPath,
		Format:         format,
		Engine:         finalEngine,
		EngineChain:    engineChain,
		Degraded:       degraded,
		FallbackReason: fallbackReason,
		Warnings:       compactDocumentWarnings(warnings),
		Validation: map[string]interface{}{
			"ok":       true,
			"readable": true,
		},
		Size:         size,
		Success:      true,
		ExtractedVia: extractedVia,
	}
	if result != nil {
		payload.Text = result.Text
		if result.TabularSummary != nil {
			payload.TabularSummary = result.TabularSummary
		}
	}
	return payload
}

func compactDocumentWarnings(warnings []string) []string {
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" || slices.Contains(out, warning) {
			continue
		}
		out = append(out, warning)
	}
	return out
}

func mergeValidationFields(dst, src map[string]interface{}) {
	if dst == nil || src == nil {
		return
	}
	for key, value := range src {
		dst[key] = value
	}
}

func validateZipEntries(path string, required []string) (map[string]interface{}, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer reader.Close()

	available := make(map[string]struct{}, len(reader.File))
	for _, file := range reader.File {
		available[file.Name] = struct{}{}
	}
	missing := make([]string, 0, len(required))
	for _, name := range required {
		if _, ok := available[name]; !ok {
			missing = append(missing, name)
		}
	}
	return map[string]interface{}{
		"ok":               len(missing) == 0,
		"required_entries": append([]string(nil), required...),
		"missing_entries":  missing,
	}, nil
}

func readZipArchive(path string) ([]zipArchiveEntry, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	entries := make([]zipArchiveEntry, 0, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open archive entry %s: %w", file.Name, err)
		}
		data, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read archive entry %s: %w", file.Name, readErr)
		}
		entries = append(entries, zipArchiveEntry{
			Name:   file.Name,
			Data:   data,
			Method: file.Method,
		})
	}
	return entries, nil
}

func writeZipArchive(path string, entries []zipArchiveEntry) error {
	var buf bytes.Buffer
	buf.Grow(estimateZipArchiveBuffer(entries))
	writer := newFastZipWriter(&buf)
	for _, entry := range entries {
		header := &zip.FileHeader{
			Name:   entry.Name,
			Method: entry.Method,
		}
		if header.Method == 0 {
			header.Method = zip.Deflate
		}
		w, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return fmt.Errorf("create archive entry %s: %w", entry.Name, err)
		}
		if _, err := w.Write(entry.Data); err != nil {
			_ = writer.Close()
			return fmt.Errorf("write archive entry %s: %w", entry.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close archive: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func estimateZipArchiveBuffer(entries []zipArchiveEntry) int {
	total := len(entries) * 256
	contentBytes := 0
	for _, entry := range entries {
		contentBytes += len(entry.Data)
	}
	if contentBytes <= 0 {
		return total
	}
	if contentBytes <= 8<<20 {
		return total + contentBytes
	}
	return total + (contentBytes / 2)
}

func replaceArchiveEntries(path string, match func(string) bool, replacer func(string, []byte) ([]byte, bool, error)) (bool, error) {
	entries, err := readZipArchive(path)
	if err != nil {
		return false, err
	}
	changed := false
	for idx := range entries {
		if !match(entries[idx].Name) {
			continue
		}
		updated, entryChanged, err := replacer(entries[idx].Name, entries[idx].Data)
		if err != nil {
			return false, err
		}
		if !entryChanged {
			continue
		}
		entries[idx].Data = updated
		changed = true
	}
	if !changed {
		return false, nil
	}
	if err := writeZipArchive(path, entries); err != nil {
		return false, err
	}
	return true, nil
}

func parseReplacementMap(args map[string]interface{}, keys ...string) map[string]string {
	for _, key := range keys {
		raw, ok := compatArgValue(args, key)
		if !ok {
			continue
		}
		if values, ok := coerceCompatMap(raw); ok {
			out := make(map[string]string, len(values))
			for from, to := range values {
				from = strings.TrimSpace(from)
				if from == "" {
					continue
				}
				out[from] = strings.TrimSpace(asString(to))
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func executeNativeDocumentRead(ctx context.Context, scope *fsToolScope, toolName, format string, reader *convertpkg.DocumentReader, args map[string]interface{}) (string, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	absPath, relPath, _, err := scope.resolvePathWithContext(ctx, toolName, path, false)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file")
	}
	result, err := reader.ReadDocument(ctx, absPath)
	if err != nil {
		return "", err
	}
	payload := buildReadPayloadFromDocumentResult("read", relPath, absPath, format, filepath.Ext(absPath), info.Size(), result, nil)
	if validation, err := validateNativeOfficeArchive(absPath, format, result); err == nil {
		payload.Validation = validation
	} else {
		payload.Validation = map[string]interface{}{
			"ok":              true,
			"quality_checked": false,
			"quality_error":   err.Error(),
		}
	}
	return marshalNativeDocumentPayload(payload)
}

func executeCreateLikeDocumentWrite(ctx context.Context, toolName string, scope *fsToolScope, path string, createDirs bool, data []byte) (absPath, relPath string, err error) {
	absPath, relPath, _, err = scope.resolvePathWithContext(ctx, toolName, path, false)
	if err != nil {
		return "", "", err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return "", "", err
	}
	if createDirs {
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return "", "", fmt.Errorf("failed to create directory: %w", err)
		}
	}
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return "", "", fmt.Errorf("failed to write file: %w", err)
	}
	return absPath, relPath, nil
}

func parseCreateDirsArg(args map[string]interface{}) (bool, error) {
	return fsAsBool(args, "create_dirs", true)
}

func countZipEntriesWithPrefix(path, prefix, suffix string) (int, error) {
	entries, err := readZipArchive(path)
	if err != nil {
		return 0, err
	}
	lowerPrefix := strings.ToLower(strings.TrimSpace(prefix))
	lowerSuffix := strings.ToLower(strings.TrimSpace(suffix))
	count := 0
	for _, entry := range entries {
		name := strings.ToLower(strings.TrimSpace(entry.Name))
		if strings.HasPrefix(name, lowerPrefix) && strings.HasSuffix(name, lowerSuffix) {
			count++
		}
	}
	return count, nil
}
