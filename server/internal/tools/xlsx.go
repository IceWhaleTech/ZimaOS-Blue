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
		Description: "Use when the task centers on a workspace .xlsx file and needs a native spreadsheet for tables, formulas, sheet edits, analysis, or validation.",
		Icon:        "sheet",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"read", "create", "edit", "fix", "validate", "append_rows", "update_cells", "rename_sheet", "insert_rows", "delete_rows", "set_filter", "set_freeze", "profile_sheet", "group_by", "top_n", "sheet_compare"},
					"description": "Operation to perform. Defaults to read.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Workspace path to the target .xlsx file.",
				},
				"title":    map[string]interface{}{"type": "string"},
				"subtitle": map[string]interface{}{"type": "string"},
				"theme":    map[string]interface{}{"type": "string", "enum": []string{"analysis", "ui_review", "executive", "clean", "midnight", "terracotta", "forest", "coral"}},
				"style_hint": map[string]interface{}{
					"type":        "string",
					"description": "Optional tone/style hint used to infer the workbook theme when theme is omitted.",
				},
				"summary": map[string]interface{}{},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Optional Markdown-like workbook seed. Markdown tables become sheets; Markdown lists or paragraphs become content sheets or overview notes during create/edit.",
				},
				"markdown": map[string]interface{}{
					"type":        "string",
					"description": "Alias of content for Markdown-first workbook creation.",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Alias of content.",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Alias of content.",
				},
				"notes":    map[string]interface{}{"type": "array"},
				"sheets":   map[string]interface{}{"type": "array"},
				"sheet":    map[string]interface{}{},
				"columns":  map[string]interface{}{"type": "array"},
				"rows":     map[string]interface{}{"type": "array"},
				"cells":    map[string]interface{}{},
				"row":      map[string]interface{}{},
				"count":    map[string]interface{}{},
				"range":    map[string]interface{}{"type": "string"},
				"freeze":   map[string]interface{}{"type": "string"},
				"new_name": map[string]interface{}{"type": "string"},
				"group_by": map[string]interface{}{"type": "string"},
				"metric":   map[string]interface{}{"type": "string"},
				"n":        map[string]interface{}{},
				"compare_path": map[string]interface{}{
					"type": "string",
				},
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
	case "append_rows", "update_cells", "rename_sheet", "insert_rows", "delete_rows", "set_filter", "set_freeze":
		return t.executeSemanticMutation(ctx, args, action)
	case "profile_sheet", "group_by", "top_n", "sheet_compare":
		return t.executeAnalysisAction(ctx, args, action)
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
	styleHint := firstCompatString(args, "style_hint", "styleHint", "style", "visual_style", "visualStyle")
	theme := resolveOfficeTheme(firstCompatString(args, "theme"), styleHint)
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
	attachOfficeThemeMetadata(&payload, theme)
	return marshalNativeDocumentPayload(payload)
}

func (t *XLSXTool) executeEdit(ctx context.Context, args map[string]interface{}, action string) (string, error) {
	if _, ok := compatArgValue(args, "sheets", "sheet", "columns", "headers", "rows", "table", "summary", "notes", "title", "subtitle", "content", "markdown", "body", "text"); ok {
		return t.executeCreateLike(ctx, args, action)
	}
	replacements := parseReplacementMap(args, "replacements", "variables")
	if len(replacements) > 0 {
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
	inferredAction, err := inferSemanticXLSXAction(args)
	if err != nil {
		return "", err
	}
	if inferredAction == "" {
		return "", fmt.Errorf("xlsx %s requires replacements/variables or a semantic edit payload", action)
	}
	return t.executeSemanticMutation(ctx, args, inferredAction)
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

func inferSemanticXLSXAction(args map[string]interface{}) (string, error) {
	switch {
	case hasCompatArg(args, "cells"):
		return "update_cells", nil
	case hasCompatArg(args, "new_name"):
		return "rename_sheet", nil
	case hasCompatArg(args, "freeze"):
		return "set_freeze", nil
	case hasCompatArg(args, "range"):
		return "set_filter", nil
	case hasCompatArg(args, "row") && hasCompatArg(args, "count"):
		return "delete_rows", nil
	case hasCompatArg(args, "row") && hasCompatArg(args, "rows"):
		return "insert_rows", nil
	case hasCompatArg(args, "rows"):
		return "append_rows", nil
	default:
		return "", nil
	}
}

func hasCompatArg(args map[string]interface{}, key string) bool {
	_, ok := compatArgValue(args, key)
	return ok
}

func (t *XLSXTool) executeSemanticMutation(ctx context.Context, args map[string]interface{}, action string) (string, error) {
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
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return "", err
	}

	workbook, err := loadMutableXLSXWorkbook(absPath)
	if err != nil {
		return "", err
	}

	sheetName := strings.TrimSpace(firstCompatString(args, "sheet"))
	var summary string

	switch action {
	case "rename_sheet":
		newName := strings.TrimSpace(firstCompatString(args, "new_name", "name"))
		if newName == "" {
			return "", fmt.Errorf("new_name is required")
		}
		target := sheetName
		if target == "" {
			target = strings.TrimSpace(firstCompatString(args, "from"))
		}
		if target == "" {
			sheet, err := workbook.resolveSheet("")
			if err != nil {
				return "", err
			}
			target = sheet.name
		}
		if err := workbook.renameSheet(target, newName); err != nil {
			return "", err
		}
		summary = fmt.Sprintf("Renamed sheet %s to %s", target, newName)
	default:
		sheet, err := workbook.resolveSheet(sheetName)
		if err != nil {
			return "", err
		}
		switch action {
		case "append_rows":
			rawRows, ok := compatArgValue(args, "rows")
			if !ok {
				return "", fmt.Errorf("rows are required")
			}
			if err := xlsxApplyAppendRows(sheet, rawRows); err != nil {
				return "", err
			}
			summary = fmt.Sprintf("Appended rows to %s", sheet.name)
		case "update_cells":
			rawCells, ok := compatArgValue(args, "cells")
			if !ok {
				return "", fmt.Errorf("cells are required")
			}
			if err := xlsxApplyUpdateCells(sheet, rawCells); err != nil {
				return "", err
			}
			summary = fmt.Sprintf("Updated cells on %s", sheet.name)
		case "insert_rows":
			rawRows, ok := compatArgValue(args, "rows")
			if !ok {
				return "", fmt.Errorf("rows are required")
			}
			row := compatInt(args, "row", "row_index")
			if row <= 0 {
				row = 1
			}
			count := compatInt(args, "count")
			if count <= 0 {
				count = 1
			}
			if err := xlsxApplyInsertRows(sheet, row, count, rawRows); err != nil {
				return "", err
			}
			summary = fmt.Sprintf("Inserted rows into %s", sheet.name)
		case "delete_rows":
			row := compatInt(args, "row", "row_index")
			if row <= 0 {
				return "", fmt.Errorf("row must be >= 1")
			}
			count := compatInt(args, "count")
			if count <= 0 {
				count = 1
			}
			if err := xlsxApplyDeleteRows(sheet, row, count); err != nil {
				return "", err
			}
			summary = fmt.Sprintf("Deleted rows from %s", sheet.name)
		case "set_filter":
			filterRange := strings.TrimSpace(firstCompatString(args, "range", "ref"))
			if filterRange == "" {
				columns := xlsxColumnSpecsForSheet(sheet.build)
				if len(columns) == 0 || len(sheet.build.Rows) == 0 {
					return "", fmt.Errorf("sheet has no used range")
				}
				filterRange = officeXLSXRangeRef(0, 1, len(columns)-1, len(sheet.build.Rows))
			}
			sheet.build.AutoFilter = filterRange
			sheet.dirty = true
			summary = fmt.Sprintf("Updated filter on %s", sheet.name)
		case "set_freeze":
			freeze := strings.TrimSpace(firstCompatString(args, "freeze"))
			if freeze == "" {
				return "", fmt.Errorf("freeze is required")
			}
			sheet.build.Freeze = freeze
			sheet.dirty = true
			summary = fmt.Sprintf("Updated freeze pane on %s", sheet.name)
		default:
			return "", fmt.Errorf("unsupported semantic xlsx action %q", action)
		}
	}

	if err := workbook.save(absPath); err != nil {
		return "", err
	}
	validation, err := t.validatePath(ctx, absPath)
	if err != nil {
		return "", err
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
		Validation:   validation,
		Size:         info.Size(),
		Summary:      summary,
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *XLSXTool) executeAnalysisAction(ctx context.Context, args map[string]interface{}, action string) (string, error) {
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
	workbook, err := loadMutableXLSXWorkbook(absPath)
	if err != nil {
		return "", err
	}
	sheet, err := workbook.resolveSheet(strings.TrimSpace(firstCompatString(args, "sheet")))
	if err != nil && action != "sheet_compare" {
		return "", err
	}

	var (
		result  map[string]interface{}
		summary string
	)

	switch action {
	case "profile_sheet":
		headers := xlsxHeadersForSheet(sheet.build)
		result = map[string]interface{}{
			"sheet":         sheet.name,
			"row_count":     len(xlsxSheetRecords(sheet.build)),
			"headers":       headers,
			"formula_count": xlsxSheetFormulaCount(sheet.build),
			"column_kinds":  xlsxSheetColumnKinds(sheet.build),
		}
		summary = fmt.Sprintf("Profiled %s with %d data rows", sheet.name, len(xlsxSheetRecords(sheet.build)))
	case "group_by":
		groupBy := strings.TrimSpace(firstCompatString(args, "group_by"))
		metric := strings.TrimSpace(firstCompatString(args, "metric"))
		if groupBy == "" || metric == "" {
			return "", fmt.Errorf("group_by and metric are required")
		}
		groups := xlsxGroupTotals(sheet.build, groupBy, metric)
		result = map[string]interface{}{
			"sheet":    sheet.name,
			"group_by": groupBy,
			"metric":   metric,
			"groups":   groups,
		}
		summary = fmt.Sprintf("Grouped %s by %s", metric, groupBy)
	case "top_n":
		metric := strings.TrimSpace(firstCompatString(args, "metric"))
		if metric == "" {
			return "", fmt.Errorf("metric is required")
		}
		limit := compatInt(args, "n", "limit")
		if limit <= 0 {
			limit = 5
		}
		rows := xlsxTopRows(sheet.build, metric, limit)
		result = map[string]interface{}{
			"sheet":  sheet.name,
			"metric": metric,
			"rows":   rows,
		}
		summary = fmt.Sprintf("Computed top %d rows by %s", limit, metric)
	case "sheet_compare":
		comparePath := strings.TrimSpace(firstCompatString(args, "compare_path", "other_path"))
		if comparePath == "" {
			return "", fmt.Errorf("compare_path is required")
		}
		compareAbs, _, _, err := t.scope.resolvePathWithContext(ctx, "xlsx", comparePath, false)
		if err != nil {
			return "", err
		}
		compareWorkbook, err := loadMutableXLSXWorkbook(compareAbs)
		if err != nil {
			return "", err
		}
		leftSheet, err := workbook.resolveSheet(strings.TrimSpace(firstCompatString(args, "sheet")))
		if err != nil {
			return "", err
		}
		rightSheet, err := compareWorkbook.resolveSheet(strings.TrimSpace(firstCompatString(args, "sheet")))
		if err != nil {
			return "", err
		}
		changed := xlsxChangedCellCount(leftSheet.build, rightSheet.build)
		result = map[string]interface{}{
			"sheet":         leftSheet.name,
			"compare_path":  comparePath,
			"changed_cells": changed,
		}
		summary = fmt.Sprintf("Compared %s against %s", leftSheet.name, comparePath)
	default:
		return "", fmt.Errorf("unsupported xlsx analysis action %q", action)
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
		Size:         info.Size(),
		Result:       result,
		Summary:      summary,
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}
