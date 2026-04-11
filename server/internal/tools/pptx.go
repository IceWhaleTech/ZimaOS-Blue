package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

type PPTXTool struct {
	scope  *fsToolScope
	reader *convertpkg.DocumentReader
}

func NewPPTXTool(allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) *PPTXTool {
	scope := newFSToolScope(allowedPaths)
	scope = scope.withApprovalFlow(approvals, dirStore)
	return &PPTXTool{
		scope:  scope,
		reader: convertpkg.NewDocumentReader(),
	}
}

func (t *PPTXTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "pptx",
		Description: "Read, create, or edit native .pptx workspace files with native-first OOXML packaging and explicit degradation telemetry.",
		Icon:        "presentation",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"read", "create", "edit", "validate"},
					"description": "Operation to perform. Defaults to read.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Workspace path to the target .pptx file.",
				},
				"title":      map[string]interface{}{"type": "string"},
				"subtitle":   map[string]interface{}{"type": "string"},
				"summary":    map[string]interface{}{},
				"content":    map[string]interface{}{"type": "string"},
				"sections":   map[string]interface{}{"type": "array"},
				"paragraphs": map[string]interface{}{"type": "array"},
				"notes":      map[string]interface{}{"type": "array"},
				"replacements": map[string]interface{}{
					"type":        "object",
					"description": "Literal text replacements applied inside slide XML.",
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

func (t *PPTXTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action")))
	if action == "" {
		action = "read"
	}
	switch action {
	case "read":
		return executeNativeDocumentRead(ctx, t.scope, "pptx", "pptx", t.reader, args)
	case "create":
		return t.executeCreateLike(ctx, args, "create")
	case "edit":
		return t.executeEdit(ctx, args)
	case "validate":
		return t.executeValidate(ctx, args)
	default:
		return nil, fmt.Errorf("unsupported pptx action %q", action)
	}
}

func (t *PPTXTool) executeCreateLike(ctx context.Context, args map[string]interface{}, action string) (string, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	title := strings.TrimSpace(firstCompatString(args, "title"))
	subtitle := strings.TrimSpace(firstCompatString(args, "subtitle"))
	spec, err := parseOfficeDocSpec(args, title, subtitle, resolveOfficeTheme("", "presentation"), "presentation")
	if err != nil {
		return "", err
	}
	slides := officeBuildPresentationSlides(spec)
	data, info, err := buildOfficePPTX(spec)
	if err != nil {
		return "", err
	}
	createDirs, err := parseCreateDirsArg(args)
	if err != nil {
		return "", err
	}
	absPath, relPath, err := executeCreateLikeDocumentWrite(ctx, "pptx", t.scope, path, createDirs, data)
	if err != nil {
		return "", err
	}
	validation, err := t.validatePath(ctx, absPath)
	if err != nil {
		return "", err
	}
	validation["slide_count"] = len(slides)
	if info.ParagraphCount > 0 {
		validation["paragraph_count"] = info.ParagraphCount
	}
	payload := nativeDocumentPayload{
		Action:       action,
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "pptx",
		Engine:       "native_pptx_ooxml",
		EngineChain:  []string{"native_pptx_ooxml"},
		Degraded:     false,
		Validation:   validation,
		Size:         int64(len(data)),
		SlideCount:   len(slides),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *PPTXTool) executeEdit(ctx context.Context, args map[string]interface{}) (string, error) {
	if _, ok := compatArgValue(args, "content", "sections", "summary", "paragraphs", "notes", "title", "subtitle"); ok {
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
		return "", fmt.Errorf("pptx edit requires replacements/variables or create-style content inputs")
	}
	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "pptx", path, false)
	if err != nil {
		return "", err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return "", err
	}
	changed, err := replaceArchiveEntries(absPath, isPPTXXMLEntry, func(_ string, data []byte) ([]byte, bool, error) {
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
		warnings = append(warnings, "no matching replacement tokens were found in slide XML")
	}
	info, _ := os.Stat(absPath)
	payload := nativeDocumentPayload{
		Action:       "edit",
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "pptx",
		Engine:       "native_pptx_ooxml",
		EngineChain:  []string{"native_pptx_ooxml"},
		Degraded:     false,
		Warnings:     warnings,
		Validation:   validation,
		Size:         info.Size(),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *PPTXTool) executeValidate(ctx context.Context, args map[string]interface{}) (string, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "pptx", path, false)
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
		Format:       "pptx",
		Engine:       "native_pptx_ooxml",
		EngineChain:  []string{"native_pptx_ooxml"},
		Degraded:     false,
		Validation:   validation,
		Size:         info.Size(),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *PPTXTool) validatePath(ctx context.Context, absPath string) (map[string]interface{}, error) {
	validation, err := validateZipEntries(absPath, []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"ppt/presentation.xml",
		"ppt/slides/slide1.xml",
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
	if slideCount, err := countZipEntriesWithPrefix(absPath, "ppt/slides/slide", ".xml"); err == nil {
		validation["slide_count"] = slideCount
	}
	return validation, nil
}

func isPPTXXMLEntry(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return lower == "ppt/presentation.xml" || strings.HasPrefix(lower, "ppt/slides/")
}
