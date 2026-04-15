package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

type DOCXTool struct {
	scope  *fsToolScope
	reader *convertpkg.DocumentReader
}

func NewDOCXTool(allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) *DOCXTool {
	scope := newFSToolScope(allowedPaths)
	scope = scope.withApprovalFlow(approvals, dirStore)
	return &DOCXTool{
		scope:  scope,
		reader: convertpkg.NewDocumentReader(),
	}
}

func (t *DOCXTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "docx",
		Description: "Read, create, edit, template, or validate native .docx workspace files with native-first OOXML handling and explicit degradation telemetry.",
		Icon:        "file-text",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"read", "create", "edit", "apply_template", "validate"},
					"description": "Operation to perform. Defaults to read.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Workspace path to the target .docx file.",
				},
				"template_path": map[string]interface{}{
					"type":        "string",
					"description": "Optional source template .docx path for apply_template.",
				},
				"title":    map[string]interface{}{"type": "string"},
				"subtitle": map[string]interface{}{"type": "string"},
				"theme": map[string]interface{}{
					"type": "string",
					"enum": []string{"analysis", "ui_review", "executive", "clean", "midnight", "terracotta", "forest", "coral"},
				},
				"style_hint": map[string]interface{}{"type": "string"},
				"summary":    map[string]interface{}{},
				"content":    map[string]interface{}{"type": "string"},
				"sections":   map[string]interface{}{"type": "array"},
				"notes":      map[string]interface{}{"type": "array"},
				"paragraphs": map[string]interface{}{"type": "array"},
				"replacements": map[string]interface{}{
					"type":        "object",
					"description": "Literal placeholder or text replacements applied inside document/header/footer XML.",
				},
				"variables": map[string]interface{}{
					"type":        "object",
					"description": "Alias of replacements for template application.",
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

func (t *DOCXTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action")))
	if action == "" {
		action = "read"
	}
	switch action {
	case "read":
		return executeNativeDocumentRead(ctx, t.scope, "docx", "docx", t.reader, args)
	case "create":
		return t.executeCreateLike(ctx, args, "create")
	case "edit":
		return t.executeEdit(ctx, args)
	case "apply_template":
		return t.executeApplyTemplate(ctx, args)
	case "validate":
		return t.executeValidate(ctx, args)
	default:
		return nil, fmt.Errorf("unsupported docx action %q", action)
	}
}

func (t *DOCXTool) executeCreateLike(ctx context.Context, args map[string]interface{}, action string) (string, error) {
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
	spec, err := parseOfficeDocSpec(args, title, subtitle, theme, styleHint)
	if err != nil {
		return "", err
	}
	data, info, err := buildOfficeDOCX(spec)
	if err != nil {
		return "", err
	}
	createDirs, err := parseCreateDirsArg(args)
	if err != nil {
		return "", err
	}
	absPath, relPath, err := executeCreateLikeDocumentWrite(ctx, "docx", t.scope, path, createDirs, data)
	if err != nil {
		return "", err
	}
	validation, err := t.validatePath(ctx, absPath)
	if err != nil {
		return "", err
	}
	payload := nativeDocumentPayload{
		Action:       action,
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "docx",
		Engine:       "native_docx_ooxml",
		EngineChain:  []string{"native_docx_ooxml"},
		Degraded:     false,
		Warnings:     nil,
		Validation:   validation,
		Size:         int64(len(data)),
		Success:      true,
	}
	attachOfficeThemeMetadata(&payload, theme)
	if info.SectionCount > 0 {
		payload.Validation["section_count"] = info.SectionCount
	}
	if info.ParagraphCount > 0 {
		payload.Validation["paragraph_count"] = info.ParagraphCount
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *DOCXTool) executeEdit(ctx context.Context, args map[string]interface{}) (string, error) {
	if _, ok := compatArgValue(args, "content", "sections", "summary", "paragraphs", "notes"); ok {
		return t.executeCreateLike(ctx, args, "edit")
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
		return "", fmt.Errorf("docx edit requires replacements/variables or create-style content inputs")
	}
	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "docx", path, false)
	if err != nil {
		return "", err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return "", err
	}
	changed, err := replaceArchiveEntries(absPath, isDOCXXMLEntry, func(_ string, data []byte) ([]byte, bool, error) {
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
		warnings = append(warnings, "no matching replacement tokens were found in document/header/footer XML")
	}
	info, _ := os.Stat(absPath)
	payload := nativeDocumentPayload{
		Action:       "edit",
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "docx",
		Engine:       "native_docx_ooxml",
		EngineChain:  []string{"native_docx_ooxml"},
		Degraded:     false,
		Warnings:     warnings,
		Validation:   validation,
		Size:         info.Size(),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *DOCXTool) executeApplyTemplate(ctx context.Context, args map[string]interface{}) (string, error) {
	templatePath, err := fsAsString(args, "template_path")
	if err != nil || strings.TrimSpace(templatePath) == "" {
		return "", fmt.Errorf("template_path must be a non-empty string")
	}
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var pathErr error
		path, pathErr = fsAsString(args, "path")
		if pathErr != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	absTemplate, _, _, err := t.scope.resolvePathWithContext(ctx, "docx", templatePath, false)
	if err != nil {
		return "", err
	}
	templateData, err := os.ReadFile(absTemplate)
	if err != nil {
		return "", fmt.Errorf("read template: %w", err)
	}
	createDirs, err := parseCreateDirsArg(args)
	if err != nil {
		return "", err
	}
	absPath, relPath, err := executeCreateLikeDocumentWrite(ctx, "docx", t.scope, path, createDirs, templateData)
	if err != nil {
		return "", err
	}
	replacements := parseReplacementMap(args, "replacements", "variables")
	if len(replacements) > 0 {
		if _, err := replaceArchiveEntries(absPath, isDOCXXMLEntry, func(_ string, data []byte) ([]byte, bool, error) {
			text := string(data)
			updated := text
			for from, to := range replacements {
				updated = strings.ReplaceAll(updated, from, to)
			}
			if updated == text {
				return data, false, nil
			}
			return []byte(updated), true, nil
		}); err != nil {
			return "", err
		}
	}
	validation, err := t.validatePath(ctx, absPath)
	if err != nil {
		return "", err
	}
	info, _ := os.Stat(absPath)
	payload := nativeDocumentPayload{
		Action:       "apply_template",
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "docx",
		Engine:       "native_docx_ooxml",
		EngineChain:  []string{"native_docx_ooxml"},
		Degraded:     false,
		Validation:   validation,
		Size:         info.Size(),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *DOCXTool) executeValidate(ctx context.Context, args map[string]interface{}) (string, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "docx", path, false)
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
		Format:       "docx",
		Engine:       "native_docx_ooxml",
		EngineChain:  []string{"native_docx_ooxml"},
		Degraded:     false,
		Validation:   validation,
		Size:         info.Size(),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *DOCXTool) validatePath(ctx context.Context, absPath string) (map[string]interface{}, error) {
	validation, err := validateZipEntries(absPath, []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"word/document.xml",
		"word/styles.xml",
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
	return validation, nil
}

func isDOCXXMLEntry(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "word/document.xml" {
		return true
	}
	return strings.HasPrefix(lower, "word/header") || strings.HasPrefix(lower, "word/footer")
}
