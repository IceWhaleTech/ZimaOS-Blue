package tools

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path"
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
					"enum":        []string{"read", "create", "edit", "validate", "duplicate_slide", "delete_slide", "reorder_slides", "replace_text", "update_chart_data", "validate_template"},
					"description": "Operation to perform. Defaults to read.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Workspace path to the target .pptx file.",
				},
				"title":    map[string]interface{}{"type": "string"},
				"subtitle": map[string]interface{}{"type": "string"},
				"theme": map[string]interface{}{
					"type": "string",
					"enum": []string{"analysis", "ui_review", "executive", "clean", "midnight", "terracotta", "forest", "coral"},
				},
				"style_hint": map[string]interface{}{
					"type":        "string",
					"description": "Optional tone/style hint used to infer a presentation theme when theme is omitted.",
				},
				"summary":    map[string]interface{}{},
				"content":    map[string]interface{}{"type": "string"},
				"sections":   map[string]interface{}{"type": "array", "description": "Structured slides. Each section can include heading, paragraphs/body, bullets, table, and native chart data."},
				"paragraphs": map[string]interface{}{"type": "array"},
				"notes":      map[string]interface{}{"type": "array"},
				"replacements": map[string]interface{}{
					"type":        "object",
					"description": "Literal text replacements applied inside slide XML.",
				},
				"chart": map[string]interface{}{
					"type":        "object",
					"description": "Structured chart spec used by update_chart_data. Reuses native chart create fields such as type, categories, series, axis, labels, and styling controls.",
				},
				"variables": map[string]interface{}{
					"type":        "object",
					"description": "Alias of replacements.",
				},
				"slide":       map[string]interface{}{},
				"chart_index": map[string]interface{}{"type": "integer", "description": "1-based chart index within the target slide for update_chart_data. Defaults to 1."},
				"order":       map[string]interface{}{"type": "array"},
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
	case "duplicate_slide", "delete_slide", "reorder_slides", "replace_text", "update_chart_data":
		return t.executeTemplateMutation(ctx, args, action)
	case "validate_template":
		return t.executeValidateTemplate(ctx, args)
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
	styleHint := firstCompatString(args, "style_hint", "styleHint", "style", "visual_style", "visualStyle")
	if strings.TrimSpace(styleHint) == "" {
		styleHint = "presentation"
	}
	theme := resolveOfficeTheme(firstCompatString(args, "theme"), styleHint)
	spec, err := parseOfficeDocSpec(args, title, subtitle, theme, styleHint)
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
	attachOfficeThemeMetadata(&payload, theme)
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
	return t.executeValidateWithAction(ctx, args, "validate", false)
}

func (t *PPTXTool) executeValidateTemplate(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.executeValidateWithAction(ctx, args, "validate_template", true)
}

func (t *PPTXTool) executeValidateWithAction(ctx context.Context, args map[string]interface{}, action string, includeTemplateGraph bool) (string, error) {
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
	if includeTemplateGraph {
		templateValidation, err := validatePPTXTemplateGraph(absPath)
		if err != nil {
			validation["template_graph_ok"] = false
			validation["template_graph_error"] = err.Error()
		} else {
			for key, value := range templateValidation {
				validation[key] = value
			}
		}
	}
	info, _ := os.Stat(absPath)
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
		Size:         info.Size(),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func validatePPTXTemplateGraph(absPath string) (map[string]interface{}, error) {
	entries, err := readZipArchive(absPath)
	if err != nil {
		return nil, err
	}

	entryData := make(map[string][]byte, len(entries))
	charts := make(map[string]struct{})
	chartRels := make(map[string]struct{})
	workbooks := make(map[string]struct{})
	for _, entry := range entries {
		name := normalizePPTXZipEntryName(entry.Name)
		entryData[name] = entry.Data
		switch {
		case strings.HasPrefix(name, "ppt/charts/chart") && strings.HasSuffix(name, ".xml"):
			charts[name] = struct{}{}
		case strings.HasPrefix(name, "ppt/charts/_rels/chart") && strings.HasSuffix(name, ".xml.rels"):
			chartRels[name] = struct{}{}
		case strings.HasPrefix(name, "ppt/embeddings/") && strings.HasSuffix(name, ".xlsx"):
			workbooks[name] = struct{}{}
		}
	}

	referencedCharts := make(map[string]struct{})
	for name, data := range entryData {
		if !strings.HasPrefix(name, "ppt/slides/_rels/slide") || !strings.HasSuffix(name, ".xml.rels") {
			continue
		}
		rels, err := decodePPTXRelationships(data)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		for _, rel := range rels.Relationships {
			if !strings.Contains(rel.Type, "chart") {
				continue
			}
			target := strings.TrimSpace(rel.Target)
			if target == "" {
				continue
			}
			resolved := normalizePPTXZipEntryName(path.Clean(path.Join("ppt/slides", target)))
			referencedCharts[resolved] = struct{}{}
		}
	}

	referencedWorkbooks := make(map[string]struct{})
	referencedChartRels := make(map[string]struct{})
	missingChartTargetCount := 0
	for chartName := range referencedCharts {
		if _, ok := charts[chartName]; !ok {
			missingChartTargetCount++
			continue
		}
		chartRelsName := normalizePPTXZipEntryName("ppt/charts/_rels/" + path.Base(chartName) + ".rels")
		if data, ok := entryData[chartRelsName]; ok {
			referencedChartRels[chartRelsName] = struct{}{}
			rels, err := decodePPTXRelationships(data)
			if err != nil {
				return nil, fmt.Errorf("decode %s: %w", chartRelsName, err)
			}
			for _, rel := range rels.Relationships {
				if !strings.Contains(rel.Type, "package") || !strings.Contains(rel.Target, "embeddings") {
					continue
				}
				target := strings.TrimSpace(rel.Target)
				if target == "" {
					continue
				}
				resolved := normalizePPTXZipEntryName(path.Clean(path.Join("ppt/charts", target)))
				referencedWorkbooks[resolved] = struct{}{}
			}
		}
	}

	missingWorkbookTargetCount := 0
	for workbookName := range referencedWorkbooks {
		if _, ok := workbooks[workbookName]; !ok {
			missingWorkbookTargetCount++
		}
	}

	return map[string]interface{}{
		"chart_count":                    len(charts),
		"chart_rel_count":                len(chartRels),
		"embedded_workbook_count":        len(workbooks),
		"referenced_chart_count":         len(referencedCharts),
		"referenced_workbook_count":      len(referencedWorkbooks),
		"orphan_chart_count":             countMissingPPTXRefs(charts, referencedCharts),
		"orphan_chart_rel_count":         countMissingPPTXRefs(chartRels, referencedChartRels),
		"orphan_embedded_workbook_count": countMissingPPTXRefs(workbooks, referencedWorkbooks),
		"missing_chart_target_count":     missingChartTargetCount,
		"missing_workbook_target_count":  missingWorkbookTargetCount,
		"template_graph_ok": missingChartTargetCount == 0 &&
			missingWorkbookTargetCount == 0 &&
			countMissingPPTXRefs(charts, referencedCharts) == 0 &&
			countMissingPPTXRefs(chartRels, referencedChartRels) == 0 &&
			countMissingPPTXRefs(workbooks, referencedWorkbooks) == 0,
	}, nil
}

func decodePPTXRelationships(data []byte) (pptxRelationshipsXML, error) {
	var rels pptxRelationshipsXML
	if err := xml.Unmarshal(data, &rels); err != nil {
		return pptxRelationshipsXML{}, err
	}
	return rels, nil
}

func normalizePPTXZipEntryName(name string) string {
	return strings.TrimPrefix(strings.TrimSpace(strings.ReplaceAll(name, "\\", "/")), "./")
}

func countMissingPPTXRefs(actual, referenced map[string]struct{}) int {
	missing := 0
	for name := range actual {
		if _, ok := referenced[name]; !ok {
			missing++
		}
	}
	return missing
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
