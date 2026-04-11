package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

type XLSXTool struct {
	scope  *fsToolScope
	reader *convertpkg.DocumentReader
}

func NewXLSXTool(allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) *XLSXTool {
	scope := newFSToolScope(allowedPaths)
	scope = scope.withApprovalFlow(approvals, dirStore)
	return &XLSXTool{
		scope:  scope,
		reader: convertpkg.NewDocumentReader(),
	}
}

func (t *XLSXTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "xlsx",
		Description: "Read, create, edit, fix, or validate native .xlsx workspace files with native-first OOXML handling and explicit degradation telemetry.",
		Icon:        "sheet",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"read", "create", "edit", "fix", "validate"},
					"description": "Operation to perform. Defaults to read.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Workspace path to the target .xlsx file.",
				},
				"title":    map[string]interface{}{"type": "string"},
				"subtitle": map[string]interface{}{"type": "string"},
				"theme":    map[string]interface{}{"type": "string"},
				"summary":  map[string]interface{}{},
				"notes":    map[string]interface{}{"type": "array"},
				"sheets":   map[string]interface{}{"type": "array"},
				"sheet":    map[string]interface{}{},
				"columns":  map[string]interface{}{"type": "array"},
				"rows":     map[string]interface{}{"type": "array"},
				"replacements": map[string]interface{}{
					"type":        "object",
					"description": "Literal text replacements applied inside workbook XML.",
				},
				"variables": map[string]interface{}{
					"type":        "object",
					"description": "Alias of replacements.",
				},
				"create_dirs": map[string]interface{}{
					"type":        "boolean",
					"description": "Create parent directories when needed. Default true.",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t *XLSXTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action")))
	if action == "" {
		action = "read"
	}
	switch action {
	case "read":
		return executeNativeDocumentRead(ctx, t.scope, "xlsx", "xlsx", t.reader, args)
	case "create":
		return t.executeCreateLike(ctx, args, "create")
	case "edit", "fix":
		return t.executeEdit(ctx, args, action)
	case "validate":
		return t.executeValidate(ctx, args)
	default:
		return nil, fmt.Errorf("unsupported xlsx action %q", action)
	}
}

func (t *XLSXTool) executeCreateLike(ctx context.Context, args map[string]interface{}, action string) (string, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	theme := resolveOfficeTheme(firstCompatString(args, "theme"), "")
	title := strings.TrimSpace(firstCompatString(args, "title"))
	subtitle := strings.TrimSpace(firstCompatString(args, "subtitle"))
	spec, err := parseOfficeWorkbookSpec(args, title, subtitle, theme)
	if err != nil {
		return "", err
	}
	data, info, err := buildOfficeXLSX(spec)
	if err != nil {
		return "", err
	}
	createDirs, err := parseCreateDirsArg(args)
	if err != nil {
		return "", err
	}
	absPath, relPath, err := executeCreateLikeDocumentWrite(ctx, "xlsx", t.scope, path, createDirs, data)
	if err != nil {
		return "", err
	}
	validation, err := t.validatePath(ctx, absPath)
	if err != nil {
		return "", err
	}
	if info.SheetCount > 0 {
		validation["sheet_count"] = info.SheetCount
	}
	if info.RowCount > 0 {
		validation["row_count"] = info.RowCount
	}
	payload := nativeDocumentPayload{
		Action:       action,
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "xlsx",
		Engine:       "native_xlsx_ooxml",
		EngineChain:  []string{"native_xlsx_ooxml"},
		Degraded:     false,
		Validation:   validation,
		Size:         int64(len(data)),
		SheetCount:   info.SheetCount,
		RowCount:     info.RowCount,
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *XLSXTool) executeEdit(ctx context.Context, args map[string]interface{}, action string) (string, error) {
	if _, ok := compatArgValue(args, "sheets", "sheet", "columns", "rows", "summary", "notes", "title", "subtitle"); ok {
		return t.executeCreateLike(ctx, args, action)
	}
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	replacements := parseReplacementMap(args, "replacements", "variables")
	if len(replacements) == 0 {
		return "", fmt.Errorf("xlsx %s requires replacements/variables or workbook content inputs", action)
	}
	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "xlsx", path, false)
	if err != nil {
		return "", err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return "", err
	}
	changed, err := replaceArchiveEntries(absPath, isXLSXXMLEntry, func(_ string, data []byte) ([]byte, bool, error) {
		text := string(data)
		updated := text
		for from, to := range replacements {
			updated = strings.ReplaceAll(updated, from, to)
		}
		if updated == text {
			return data, false, nil
		}
		return []byte(updated), true, nil
	})
	if err != nil {
		return "", err
	}
	validation, err := t.validatePath(ctx, absPath)
	if err != nil {
		return "", err
	}
	warnings := []string(nil)
	if !changed {
		warnings = append(warnings, "no matching replacement tokens were found in workbook XML")
	}
	info, _ := os.Stat(absPath)
	payload := nativeDocumentPayload{
		Action:       action,
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "xlsx",
		Engine:       "native_xlsx_ooxml",
		EngineChain:  []string{"native_xlsx_ooxml"},
		Degraded:     false,
		Warnings:     warnings,
		Validation:   validation,
		Size:         info.Size(),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *XLSXTool) executeValidate(ctx context.Context, args map[string]interface{}) (string, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "xlsx", path, false)
	if err != nil {
		return "", err
	}
	validation, err := t.validatePath(ctx, absPath)
	if err != nil {
		return "", err
	}
	info, _ := os.Stat(absPath)
	payload := nativeDocumentPayload{
		Action:       "validate",
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "xlsx",
		Engine:       "native_xlsx_ooxml",
		EngineChain:  []string{"native_xlsx_ooxml"},
		Degraded:     false,
		Validation:   validation,
		Size:         info.Size(),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *XLSXTool) validatePath(ctx context.Context, absPath string) (map[string]interface{}, error) {
	validation, err := validateZipEntries(absPath, []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"xl/workbook.xml",
		"xl/styles.xml",
		"xl/worksheets/sheet1.xml",
	})
	if err != nil {
		return nil, err
	}
	read, err := t.reader.ReadDocument(ctx, absPath)
	if err != nil {
		validation["ok"] = false
		validation["read_error"] = err.Error()
		return validation, nil
	}
	validation["readable"] = strings.TrimSpace(read.Text) != ""
	validation["extracted_via"] = read.ExtractedVia
	validation["char_count"] = len([]rune(read.Text))
	if read.TabularSummary != nil {
		validation["tabular_summary"] = read.TabularSummary
	}
	return validation, nil
}

func isXLSXXMLEntry(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return lower == "xl/workbook.xml" ||
		lower == "xl/sharedstrings.xml" ||
		lower == "docprops/core.xml" ||
		strings.HasPrefix(lower, "xl/worksheets/")
}
