package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
)

const (
	defaultPDFDownloadTimeout = 5 * time.Minute
	maxPDFInputs              = 10
	defaultPDFMaxBytesMB      = 10
	hardPDFMaxBytesMB         = 100
)

// PDFService provides PDF metadata and extraction support.
type PDFService interface {
	Info(ctx context.Context, path string) (pdfextract.DocumentInfo, error)
	Extract(ctx context.Context, req pdfextract.ExtractRequest) (pdfextract.ExtractResult, error)
	InspectForm(ctx context.Context, path string) (pdfextract.FormInspectResult, error)
	FillForm(ctx context.Context, req pdfextract.FillFormRequest) (pdfextract.FillFormResult, error)
}

type resolvedPDFInput struct {
	Original string
	Path     string
	TempPath string
}

// PDFTool reads metadata and text from PDF documents.
type PDFTool struct {
	service    PDFService
	httpClient *http.Client
	scope      *fsToolScope
}

// NewPDFTool creates a new PDF tool.
func NewPDFTool(service PDFService) *PDFTool {
	return &PDFTool{
		service:    service,
		httpClient: newGuardedMediaHTTPClient(defaultPDFDownloadTimeout),
		scope:      newFSToolScope(nil),
	}
}

// SetHTTPClient overrides the HTTP client used for remote PDF downloads.
func (t *PDFTool) SetHTTPClient(client *http.Client) {
	if t == nil || client == nil {
		return
	}
	t.httpClient = client
}

// Definition returns the PDF tool schema.
func (t *PDFTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "pdf",
		Description: "Use when the task centers on a workspace .pdf file and needs PDF-native reading, form filling, printable output, or layout-preserving reformatting.",
		Icon:        "pdf",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"info", "read", "create", "fill", "reformat"},
					"description": "Operation to perform. Defaults to read.",
				},
				"path":        map[string]interface{}{"type": "string", "description": "Path or URL to a single PDF."},
				"input_path":  map[string]interface{}{"type": "string", "description": "Optional explicit source PDF path for action=reformat."},
				"output_path": map[string]interface{}{"type": "string", "description": "Destination workspace path for action=fill or action=reformat."},
				"paths":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Multiple PDF paths or URLs. Deduped and capped at 10."},
				"pages":       map[string]interface{}{"description": "Page selection as '1,3-5', a single number, or an array of page numbers."},
				"max_pages":   map[string]interface{}{"type": "integer", "description": "Maximum pages to extract."},
				"max_chars":   map[string]interface{}{"type": "integer", "description": "Maximum characters to return."},
				"max_bytes_mb": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum size per PDF in MB.",
				},
				"include": map[string]interface{}{
					"type":        "object",
					"description": "Optional output sections to include.",
					"properties": map[string]interface{}{
						"pages":           map[string]interface{}{"type": "boolean", "description": "Include per-page extracted text."},
						"markdown":        map[string]interface{}{"type": "boolean", "description": "Include layout-aware Markdown output."},
						"outline":         map[string]interface{}{"type": "boolean", "description": "Include document outline/bookmarks."},
						"layout":          map[string]interface{}{"type": "boolean", "description": "Include semantic blocks and tables."},
						"headers_footers": map[string]interface{}{"type": "boolean", "description": "Keep repeated headers and footers in markdown/layout output."},
					},
				},
				"fallback_mode": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"auto", "ocr_only", "vision_only", "text_only"},
					"description": "Fallback strategy for scanned or hard pages.",
				},
				"title":       map[string]interface{}{"type": "string", "description": "Optional title for action=create."},
				"subtitle":    map[string]interface{}{"type": "string", "description": "Optional subtitle for action=create."},
				"summary":     map[string]interface{}{"description": "Optional summary text or object for action=create."},
				"content":     map[string]interface{}{"type": "string", "description": "Optional Markdown-like body for action=create."},
				"markdown":    map[string]interface{}{"type": "string", "description": "Alias of content for action=create."},
				"body":        map[string]interface{}{"type": "string", "description": "Alias of content for action=create."},
				"text":        map[string]interface{}{"type": "string", "description": "Alias of content for action=create."},
				"sections":    map[string]interface{}{"type": "array", "description": "Optional structured sections for action=create."},
				"paragraphs":  map[string]interface{}{"type": "array", "description": "Optional top-level paragraphs for action=create."},
				"notes":       map[string]interface{}{"type": "array", "description": "Optional notes for action=create."},
				"theme":       map[string]interface{}{"type": "string", "description": "Optional theme hint reused from the native document writers."},
				"style_hint":  map[string]interface{}{"type": "string", "description": "Optional tone/style hint reused from the native document writers."},
				"create_dirs": map[string]interface{}{"type": "boolean", "description": "Create parent directories when needed. Default true for action=create."},
				"fields":      map[string]interface{}{"description": "Optional field values for action=fill. When omitted, fill inspects and returns available native form fields first."},
			},
		},
	}
}

// Execute performs the requested PDF action.
func (t *PDFTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	args = normalizePDFArgs(args)
	switch pdfAction(args) {
	case "create":
		return t.executeCreate(ctx, args)
	case "fill":
		return t.executeFill(ctx, args)
	case "reformat":
		return t.executeReformat(ctx, args)
	case "info":
		if t == nil || t.service == nil {
			return nil, errors.New("pdf service not available")
		}
		refs, err := collectPDFInputs(args)
		if err != nil {
			return nil, err
		}
		if len(refs) == 0 {
			return nil, errors.New("path/pdf is required")
		}
		inputs, cleanup, err := t.resolvePDFInputs(ctx, refs, pdfMaxBytes(args))
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return t.executeInfo(ctx, inputs)
	case "read":
		if t == nil || t.service == nil {
			return nil, errors.New("pdf service not available")
		}
		refs, err := collectPDFInputs(args)
		if err != nil {
			return nil, err
		}
		if len(refs) == 0 {
			return nil, errors.New("path/pdf is required")
		}
		inputs, cleanup, err := t.resolvePDFInputs(ctx, refs, pdfMaxBytes(args))
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return t.executeRead(ctx, args, inputs)
	default:
		return nil, fmt.Errorf("unsupported pdf action")
	}
}

func (t *PDFTool) executeFill(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("pdf service not available")
	}
	refs, err := collectPDFInputs(args)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, errors.New("path/pdf is required for pdf fill")
	}
	if len(refs) != 1 {
		return nil, errors.New("pdf fill currently supports exactly one source PDF")
	}
	inputs, cleanup, err := t.resolvePDFInputs(ctx, refs, pdfMaxBytes(args))
	if err != nil {
		return nil, err
	}
	defer cleanup()

	fields, err := parsePDFFillFields(args)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		result, err := t.service.InspectForm(ctx, inputs[0].Path)
		if err != nil {
			return nil, err
		}
		result.Document = rewritePDFDocumentInfo(result.Document, inputs[0])
		engine := strings.TrimSpace(result.Document.Engine)
		if engine == "" {
			engine = "pdfium/webassembly"
		}
		validation, degraded, fallbackReason, inspectWarnings := buildPDFFillInspectStatus(result)
		payload := map[string]interface{}{
			"action":        "fill",
			"mode":          "inspect",
			"path":          inputs[0].Original,
			"absolute_path": inputs[0].Path,
			"format":        "pdf",
			"engine":        engine,
			"engine_chain":  []string{engine},
			"degraded":      degraded,
			"success":       true,
			"validation":    validation,
			"document":      result.Document,
			"form_type":     result.FormType,
			"field_count":   result.FieldCount,
			"fields":        result.Fields,
			"warnings":      compactDocumentWarnings(append(append([]string(nil), result.Warnings...), inspectWarnings...)),
		}
		if fallbackReason != "" {
			payload["fallback_reason"] = fallbackReason
		}
		return payload, nil
	}

	outputPath := strings.TrimSpace(firstCompatString(args, "output_path", "outputPath", "destination_path", "destinationPath", "output", "destination", "dest", "to"))
	if outputPath == "" {
		return nil, errors.New("output_path is required for pdf fill writes")
	}
	if strings.ToLower(strings.TrimSpace(filepath.Ext(outputPath))) != ".pdf" {
		return nil, fmt.Errorf("output_path must end in .pdf for pdf fill")
	}
	preflight, err := t.service.InspectForm(ctx, inputs[0].Path)
	if err != nil {
		return nil, err
	}
	if err := validatePDFFillWriteRequest(fields, preflight); err != nil {
		return nil, err
	}

	filled, err := t.service.FillForm(ctx, pdfextract.FillFormRequest{
		Path:   inputs[0].Path,
		Fields: fields,
	})
	if err != nil {
		return nil, err
	}
	if len(filled.Bytes) == 0 {
		return nil, errors.New("pdf fill returned empty output")
	}
	createDirs, err := parseCreateDirsArg(args)
	if err != nil {
		return nil, err
	}
	absPath, relPath, err := executeCreateLikeDocumentWrite(ctx, "pdf", t.scope, outputPath, createDirs, filled.Bytes)
	if err != nil {
		return nil, err
	}

	inspected, err := t.service.InspectForm(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("pdf fill verification failed: %w", err)
	}
	verifiedFields, err := verifyPDFFillOutput(fields, inspected)
	if err != nil {
		return nil, err
	}

	engine := strings.TrimSpace(filled.Document.Engine)
	if engine == "" {
		engine = strings.TrimSpace(inspected.Document.Engine)
	}
	if engine == "" {
		engine = "pdfium/webassembly"
	}
	validation := map[string]interface{}{
		"ok":              true,
		"readable":        true,
		"verified":        true,
		"form_type":       inspected.FormType,
		"field_count":     inspected.FieldCount,
		"updated_fields":  append([]string(nil), filled.UpdatedFields...),
		"verified_fields": verifiedFields,
	}
	payload := nativeDocumentPayload{
		Action:       "fill",
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: inputs[0].Original,
		Format:       "pdf",
		Engine:       engine,
		EngineChain:  []string{engine},
		Degraded:     false,
		Warnings:     compactDocumentWarnings(append(append([]string(nil), filled.Warnings...), inspected.Warnings...)),
		Validation:   validation,
		Size:         int64(len(filled.Bytes)),
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func (t *PDFTool) executeCreate(ctx context.Context, args map[string]interface{}) (string, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	if strings.ToLower(strings.TrimSpace(filepath.Ext(path))) != ".pdf" {
		return "", fmt.Errorf("path must end in .pdf for pdf create")
	}

	styleHint := firstCompatString(args, "style_hint", "styleHint", "style", "visual_style", "visualStyle")
	theme := resolveOfficeTheme(firstCompatString(args, "theme"), styleHint)
	spec, err := parseOfficeDocSpec(
		args,
		strings.TrimSpace(firstCompatString(args, "title")),
		strings.TrimSpace(firstCompatString(args, "subtitle")),
		theme,
		styleHint,
	)
	if err != nil {
		return "", errors.New(strings.Replace(err.Error(), "docx", "pdf", 1))
	}

	data, info, err := pdfextract.CreateDocument(nativePDFCreateRequest(spec))
	if err != nil {
		return "", err
	}
	createDirs, err := parseCreateDirsArg(args)
	if err != nil {
		return "", err
	}
	absPath, relPath, err := executeCreateLikeDocumentWrite(ctx, "pdf", t.scope, path, createDirs, data)
	if err != nil {
		return "", err
	}

	validation := map[string]interface{}{
		"ok":         true,
		"readable":   true,
		"page_count": info.PageCount,
		"line_count": info.LineCount,
		"char_count": info.CharCount,
	}
	payload := nativeDocumentPayload{
		Action:       "create",
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "pdf",
		Engine:       "native_pdf_ir",
		EngineChain:  []string{"native_pdf_ir"},
		Degraded:     false,
		Warnings:     compactDocumentWarnings(info.Warnings),
		Validation:   validation,
		Size:         int64(len(data)),
		Success:      true,
	}
	attachOfficeThemeMetadata(&payload, theme)
	return marshalNativeDocumentPayload(payload)
}

func (t *PDFTool) executeReformat(ctx context.Context, args map[string]interface{}) (string, error) {
	if t == nil || t.service == nil {
		return "", errors.New("pdf service not available")
	}
	refs, err := collectPDFInputs(args)
	if err != nil {
		return "", err
	}
	if len(refs) == 0 {
		return "", errors.New("path/pdf/input_path is required for pdf reformat")
	}
	if len(refs) != 1 {
		return "", errors.New("pdf reformat currently supports exactly one source PDF")
	}
	inputs, cleanup, err := t.resolvePDFInputs(ctx, refs, pdfMaxBytes(args))
	if err != nil {
		return "", err
	}
	defer cleanup()

	outputPath := strings.TrimSpace(firstCompatString(args, "output_path", "outputPath", "destination_path", "destinationPath", "output", "destination", "dest", "to"))
	if outputPath == "" {
		return "", errors.New("output_path is required for pdf reformat")
	}
	if strings.ToLower(strings.TrimSpace(filepath.Ext(outputPath))) != ".pdf" {
		return "", fmt.Errorf("output_path must end in .pdf for pdf reformat")
	}

	pages, err := parsePDFPages(args)
	if err != nil {
		return "", err
	}
	disableOCR, disableVision := parsePDFFallbackFlags(args)
	includeHeadersFooters, _ := compatBoolArg(args, "include_headers_footers", "includeHeadersFooters")
	extractReq := pdfextract.ExtractRequest{
		Path:                  inputs[0].Path,
		Pages:                 pages,
		MaxPages:              compatInt(args, "max_pages", "maxPages", "page_limit", "limit_pages"),
		MaxChars:              compatInt(args, "max_chars", "maxChars", "char_limit", "limit", "max_length"),
		IncludeMarkdown:       true,
		IncludeOutline:        true,
		IncludeHeadersFooters: includeHeadersFooters,
		DisableOCR:            disableOCR,
		DisableVision:         disableVision,
	}
	extracted, err := t.service.Extract(ctx, extractReq)
	if err != nil {
		return "", err
	}

	spec, err := buildPDFReformatSpec(args, inputs[0], extracted)
	if err != nil {
		return "", err
	}
	data, info, err := pdfextract.CreateDocument(nativePDFCreateRequest(spec))
	if err != nil {
		return "", err
	}
	createDirs, err := parseCreateDirsArg(args)
	if err != nil {
		return "", err
	}
	absPath, relPath, err := executeCreateLikeDocumentWrite(ctx, "pdf", t.scope, outputPath, createDirs, data)
	if err != nil {
		return "", err
	}

	warnings := make([]string, 0, len(extracted.Warnings)+len(info.Warnings)+1)
	warnings = append(warnings, extracted.Warnings...)
	if extracted.Truncated {
		warnings = append(warnings, "source pdf extraction truncated before reformat rendering")
	}
	warnings = append(warnings, info.Warnings...)

	engineChain := []string{"native_pdf_ir"}
	sourceEngine := strings.TrimSpace(extracted.Document.Engine)
	if sourceEngine != "" && sourceEngine != "native_pdf_ir" {
		engineChain = append([]string{sourceEngine}, engineChain...)
	}
	validation := map[string]interface{}{
		"ok":                true,
		"readable":          true,
		"page_count":        info.PageCount,
		"line_count":        info.LineCount,
		"char_count":        info.CharCount,
		"source_path":       inputs[0].Original,
		"source_engine":     sourceEngine,
		"source_char_count": extracted.CharCount,
		"source_page_count": extracted.Document.PageCount,
		"selected_pages":    append([]int(nil), extracted.SelectedPages...),
		"source_truncated":  extracted.Truncated,
		"source_markdown":   strings.TrimSpace(extracted.Markdown) != "",
	}
	payload := nativeDocumentPayload{
		Action:       "reformat",
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: inputs[0].Original,
		Format:       "pdf",
		Engine:       "native_pdf_ir",
		EngineChain:  engineChain,
		Degraded:     false,
		Warnings:     compactDocumentWarnings(warnings),
		Validation:   validation,
		Size:         int64(len(data)),
		Success:      true,
	}
	attachOfficeThemeMetadata(&payload, spec.Theme)
	return marshalNativeDocumentPayload(payload)
}

func (t *PDFTool) executeInfo(ctx context.Context, inputs []resolvedPDFInput) (interface{}, error) {
	if len(inputs) == 1 {
		info, err := t.service.Info(ctx, inputs[0].Path)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"document": rewritePDFDocumentInfo(info, inputs[0])}, nil
	}
	documents := make([]pdfextract.DocumentInfo, 0, len(inputs))
	for _, input := range inputs {
		info, err := t.service.Info(ctx, input.Path)
		if err != nil {
			return nil, err
		}
		documents = append(documents, rewritePDFDocumentInfo(info, input))
	}
	return map[string]interface{}{
		"mode":      "multi",
		"count":     len(documents),
		"documents": documents,
	}, nil
}

func (t *PDFTool) executeRead(ctx context.Context, args map[string]interface{}, inputs []resolvedPDFInput) (interface{}, error) {
	pages, err := parsePDFPages(args)
	if err != nil {
		return nil, err
	}
	includePages, _ := compatBoolArg(args, "include_pages", "includePages")
	disableOCR, disableVision := parsePDFFallbackFlags(args)
	includeMarkdown := true
	if enabled, ok := compatBoolArg(args, "include_markdown", "includeMarkdown"); ok {
		includeMarkdown = enabled
	}
	includeOutline := true
	if enabled, ok := compatBoolArg(args, "include_outline", "includeOutline"); ok {
		includeOutline = enabled
	}
	includeLayout := false
	if enabled, ok := compatBoolArg(args, "include_layout", "includeLayout"); ok {
		includeLayout = enabled
	}
	includeHeadersFooters := false
	if enabled, ok := compatBoolArg(args, "include_headers_footers", "includeHeadersFooters"); ok {
		includeHeadersFooters = enabled
	}
	request := pdfextract.ExtractRequest{
		Pages:                 pages,
		MaxPages:              compatInt(args, "max_pages", "maxPages", "page_limit", "limit_pages"),
		MaxChars:              compatInt(args, "max_chars", "maxChars", "char_limit", "limit", "max_length"),
		IncludePages:          includePages,
		IncludeMarkdown:       includeMarkdown,
		IncludeOutline:        includeOutline,
		IncludeLayout:         includeLayout,
		IncludeHeadersFooters: includeHeadersFooters,
		DisableOCR:            disableOCR,
		DisableVision:         disableVision,
	}
	if len(inputs) == 1 {
		request.Path = inputs[0].Path
		result, err := t.service.Extract(ctx, request)
		if err != nil {
			return nil, err
		}
		result.Document = rewritePDFDocumentInfo(result.Document, inputs[0])
		return result, nil
	}
	results := make([]pdfextract.ExtractResult, 0, len(inputs))
	documents := make([]pdfextract.DocumentInfo, 0, len(inputs))
	warnings := make([]string, 0, len(inputs))
	var combined strings.Builder
	var combinedMarkdown strings.Builder
	truncated := false
	ocrUsed := false
	visionUsed := false
	charCount := 0
	for index, input := range inputs {
		request.Path = input.Path
		result, err := t.service.Extract(ctx, request)
		if err != nil {
			return nil, err
		}
		result.Document = rewritePDFDocumentInfo(result.Document, input)
		results = append(results, result)
		documents = append(documents, result.Document)
		ocrUsed = ocrUsed || result.OCRUsed
		visionUsed = visionUsed || result.VisionUsed
		truncated = truncated || result.Truncated
		if len(result.Warnings) > 0 {
			warnings = append(warnings, result.Warnings...)
		}
		if combined.Len() > 0 {
			combined.WriteString("\n\n")
		}
		combined.WriteString(fmt.Sprintf("[PDF %d] %s\n", index+1, result.Document.FileName))
		combined.WriteString(strings.TrimSpace(result.Text))
		if includeMarkdown {
			pageMarkdown := strings.TrimSpace(result.Markdown)
			if pageMarkdown != "" {
				if combinedMarkdown.Len() > 0 {
					combinedMarkdown.WriteString("\n\n---\n\n")
				}
				combinedMarkdown.WriteString(fmt.Sprintf("[PDF %d] %s\n\n%s", index+1, result.Document.FileName, pageMarkdown))
			}
		}
	}
	text := strings.TrimSpace(combined.String())
	text, charCount, clipped := clipPDFRunes(text, compatInt(args, "max_chars", "maxChars", "char_limit", "limit", "max_length"))
	if clipped {
		truncated = true
		warnings = append(warnings, fmt.Sprintf("combined multi-pdf text truncated to %d characters", charCount))
	}
	response := map[string]interface{}{
		"mode":          "multi",
		"count":         len(results),
		"documents":     documents,
		"results":       results,
		"text":          text,
		"char_count":    charCount,
		"truncated":     truncated,
		"ocr_used":      ocrUsed,
		"vision_used":   visionUsed,
		"warnings":      warnings,
		"selected_pdfs": collectPDFInputRefs(inputs),
	}
	if includeMarkdown {
		if markdown := strings.TrimSpace(combinedMarkdown.String()); markdown != "" {
			response["markdown"] = markdown
		}
	}
	return response, nil
}

func normalizePDFArgs(args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+8)
	for k, v := range args {
		normalized[k] = v
	}

	if _, ok := normalized["pdfs"]; !ok {
		if value, ok := compatArgValue(normalized, "paths", "inputs"); ok {
			normalized["pdfs"] = value
		}
	}

	include, ok := coerceCompatMap(normalized["include"])
	if !ok || len(include) == 0 {
		return normalized
	}
	if _, ok := normalized["include_pages"]; !ok {
		if value, ok := compatArgValue(include, "pages", "include_pages", "includePages"); ok {
			normalized["include_pages"] = value
		}
	}
	if _, ok := normalized["include_markdown"]; !ok {
		if value, ok := compatArgValue(include, "markdown", "include_markdown", "includeMarkdown"); ok {
			normalized["include_markdown"] = value
		}
	}
	if _, ok := normalized["include_outline"]; !ok {
		if value, ok := compatArgValue(include, "outline", "include_outline", "includeOutline"); ok {
			normalized["include_outline"] = value
		}
	}
	if _, ok := normalized["include_layout"]; !ok {
		if value, ok := compatArgValue(include, "layout", "include_layout", "includeLayout"); ok {
			normalized["include_layout"] = value
		}
	}
	if _, ok := normalized["include_headers_footers"]; !ok {
		if value, ok := compatArgValue(include, "headers_footers", "headersFooters", "include_headers_footers", "includeHeadersFooters"); ok {
			normalized["include_headers_footers"] = value
		}
	}

	return normalized
}

func (t *PDFTool) resolvePDFInputs(ctx context.Context, refs []string, maxBytes int64) ([]resolvedPDFInput, func(), error) {
	inputs := make([]resolvedPDFInput, 0, len(refs))
	cleanupPaths := make([]string, 0, len(refs))
	cleanup := func() {
		for _, path := range cleanupPaths {
			_ = os.Remove(path)
		}
	}
	for _, ref := range refs {
		input, err := t.resolvePDFInput(ctx, ref, maxBytes)
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}
		if input.TempPath != "" {
			cleanupPaths = append(cleanupPaths, input.TempPath)
		}
		inputs = append(inputs, input)
	}
	return inputs, cleanup, nil
}

func (t *PDFTool) resolvePDFInput(ctx context.Context, ref string, maxBytes int64) (resolvedPDFInput, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return resolvedPDFInput{}, errors.New("pdf reference cannot be empty")
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" {
		switch strings.ToLower(parsed.Scheme) {
		case "file":
			localPath, err := fileURLToPath(parsed)
			if err != nil {
				return resolvedPDFInput{}, err
			}
			resolvedPath, err := t.resolveLocalPDFPath(ctx, localPath)
			if err != nil {
				return resolvedPDFInput{}, err
			}
			if err := validateLocalPDFSize(resolvedPath, maxBytes); err != nil {
				return resolvedPDFInput{}, err
			}
			return resolvedPDFInput{Original: trimmed, Path: resolvedPath}, nil
		case "http", "https":
			localPath, err := t.downloadRemotePDF(ctx, trimmed, maxBytes)
			if err != nil {
				return resolvedPDFInput{}, err
			}
			return resolvedPDFInput{Original: trimmed, Path: localPath, TempPath: localPath}, nil
		default:
			return resolvedPDFInput{}, fmt.Errorf("unsupported pdf reference scheme %q", parsed.Scheme)
		}
	}
	resolvedPath, err := t.resolveLocalPDFPath(ctx, trimmed)
	if err != nil {
		return resolvedPDFInput{}, err
	}
	if err := validateLocalPDFSize(resolvedPath, maxBytes); err != nil {
		return resolvedPDFInput{}, err
	}
	return resolvedPDFInput{Original: trimmed, Path: resolvedPath}, nil
}

func (t *PDFTool) resolveLocalPDFPath(ctx context.Context, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("pdf reference cannot be empty")
	}
	if t != nil && t.scope != nil {
		scopeCtx := ctx
		if !filepath.IsAbs(trimmed) {
			if roots, aliases := GetFSScope(ctx); len(roots) > 0 || len(aliases) > 0 {
				scopeCtx = WithFSRootOverride(ctx, roots, aliases)
			}
		}
		absPath, _, _, err := t.scope.resolvePathWithContext(scopeCtx, "pdf", trimmed, false)
		if err == nil {
			return absPath, nil
		}
		if filepath.IsAbs(trimmed) {
			return filepath.Clean(trimmed), nil
		}
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve pdf path: %w", err)
	}
	return absPath, nil
}

func (t *PDFTool) downloadRemotePDF(ctx context.Context, targetURL string, maxBytes int64) (string, error) {
	client := t.httpClient
	if client == nil {
		client = newGuardedMediaHTTPClient(defaultPDFDownloadTimeout)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("build pdf request: %w", err)
	}
	req.Header.Set("Accept", "application/pdf,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download remote pdf: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download remote pdf: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("read remote pdf: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("remote pdf exceeds %d bytes", maxBytes)
	}
	headerType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !strings.Contains(headerType, "pdf") && !looksLikePDFBytes(data) {
		return "", errors.New("url does not point to a PDF")
	}
	file, err := os.CreateTemp("", "zimaos-blue-pdf-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create temp pdf: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("write temp pdf: %w", err)
	}
	return file.Name(), nil
}

func collectPDFInputs(args map[string]interface{}) ([]string, error) {
	refs := make([]string, 0, 4)
	appendValues := func(value interface{}) error {
		items, err := collectPDFStringValues(value)
		if err != nil {
			return err
		}
		for _, item := range items {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			refs = append(refs, trimmed)
		}
		return nil
	}
	for _, key := range []string{"path", "pdf", "file", "filepath", "filePath", "source"} {
		if value, ok := compatArgValue(args, key); ok {
			if err := appendValues(value); err != nil {
				return nil, err
			}
		}
	}
	for _, key := range []string{"input_path", "inputPath", "source_path", "sourcePath"} {
		if value, ok := compatArgValue(args, key); ok {
			if err := appendValues(value); err != nil {
				return nil, err
			}
		}
	}
	if value, ok := compatArgValue(args, "pdfs"); ok {
		if err := appendValues(value); err != nil {
			return nil, err
		}
	}
	if len(refs) == 0 {
		return nil, nil
	}
	unique := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		unique = append(unique, ref)
	}
	if len(unique) > maxPDFInputs {
		return nil, fmt.Errorf("too many pdf inputs: %d exceeds maximum %d", len(unique), maxPDFInputs)
	}
	return unique, nil
}

func collectPDFStringValues(value interface{}) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
		return []string{typed}, nil
	case []string:
		return append([]string(nil), typed...), nil
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			values, err := collectPDFStringValues(item)
			if err != nil {
				return nil, err
			}
			out = append(out, values...)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported pdf input format")
	}
}

func rewritePDFDocumentInfo(info pdfextract.DocumentInfo, input resolvedPDFInput) pdfextract.DocumentInfo {
	if strings.TrimSpace(input.Original) != "" {
		info.Path = input.Original
		if name := pdfDisplayName(input.Original); name != "" {
			info.FileName = name
		}
	}
	return info
}

func collectPDFInputRefs(inputs []resolvedPDFInput) []string {
	out := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if strings.TrimSpace(input.Original) == "" {
			continue
		}
		out = append(out, input.Original)
	}
	return out
}

func fileURLToPath(parsed *url.URL) (string, error) {
	if parsed == nil {
		return "", errors.New("invalid file url")
	}
	path := parsed.Path
	if parsed.Host != "" && parsed.Host != "localhost" {
		path = "//" + parsed.Host + parsed.Path
	}
	unescaped, err := url.PathUnescape(path)
	if err != nil {
		return "", fmt.Errorf("decode file url: %w", err)
	}
	if strings.TrimSpace(unescaped) == "" {
		return "", errors.New("file url path is empty")
	}
	return unescaped, nil
}

func validateLocalPDFSize(path string, maxBytes int64) error {
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat pdf: %w", err)
	}
	if stat.IsDir() {
		return errors.New("path must be a file")
	}
	if stat.Size() > maxBytes {
		return fmt.Errorf("pdf exceeds %d bytes", maxBytes)
	}
	return nil
}

func looksLikePDFBytes(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte("%PDF-"))
}

func nativePDFCreateRequest(spec officeDocSpec) pdfextract.CreateRequest {
	paragraphs := make([]string, 0, len(spec.ParagraphBlocks)+len(spec.Paragraphs))
	for _, block := range officeDocBlocksOrParagraphs(spec.ParagraphBlocks, spec.Paragraphs) {
		if text := strings.TrimSpace(officePDFTextForDocBlock(block)); text != "" {
			paragraphs = append(paragraphs, text)
		}
	}
	req := pdfextract.CreateRequest{
		Title:         spec.Title,
		Subtitle:      spec.Subtitle,
		Summary:       spec.Summary,
		Paragraphs:    paragraphs,
		Notes:         append([]string(nil), spec.Notes...),
		TitleColor:    spec.Theme.PrimaryDark,
		SubtitleColor: spec.Theme.Secondary,
		HeadingColor:  spec.Theme.PrimaryDark,
		BodyColor:     spec.Theme.Slate,
		MutedColor:    spec.Theme.Secondary,
	}
	for _, section := range spec.Sections {
		sectionParagraphs := make([]string, 0, len(section.ParagraphBlocks)+len(section.Paragraphs))
		for _, block := range officeDocBlocksOrParagraphs(section.ParagraphBlocks, section.Paragraphs) {
			if text := strings.TrimSpace(officePDFTextForDocBlock(block)); text != "" {
				sectionParagraphs = append(sectionParagraphs, text)
			}
		}
		next := pdfextract.CreateSection{
			Heading:    section.Heading,
			Paragraphs: sectionParagraphs,
			Bullets:    append([]string(nil), section.Bullets...),
		}
		if section.Table != nil {
			next.Table = &pdfextract.CreateTable{
				Headers: append([]string(nil), section.Table.Headers...),
				Rows:    make([][]string, 0, len(section.Table.Rows)),
			}
			for _, row := range section.Table.Rows {
				next.Table.Rows = append(next.Table.Rows, append([]string(nil), row...))
			}
		}
		req.Sections = append(req.Sections, next)
	}
	return req
}

func officePDFTextForDocBlock(block officeDocBlock) string {
	if block.Kind == officeDocBlockSeparator {
		return "────────"
	}
	if block.Kind == officeDocBlockImage {
		return "[Image: " + officeImageAltText(block) + "]"
	}
	return block.Text
}

func buildPDFReformatSpec(args map[string]interface{}, input resolvedPDFInput, extracted pdfextract.ExtractResult) (officeDocSpec, error) {
	styleHint := firstCompatString(args, "style_hint", "styleHint", "style", "visual_style", "visualStyle")
	theme := resolveOfficeTheme(firstCompatString(args, "theme"), styleHint)
	title := strings.TrimSpace(firstCompatString(args, "title"))
	subtitle := strings.TrimSpace(firstCompatString(args, "subtitle"))

	content := strings.TrimSpace(extracted.Markdown)
	if content == "" {
		content = strings.TrimSpace(extracted.Text)
	} else if title != "" {
		content = pdfReformatDemoteMarkdownHeadings(content)
	}
	if content == "" {
		return officeDocSpec{}, errors.New("pdf reformat requires extracted source text")
	}

	specArgs := map[string]interface{}{
		"content": content,
	}
	if summary, ok := compatArgValue(args, "summary"); ok {
		specArgs["summary"] = summary
	}
	if notes, ok := compatArgValue(args, "notes"); ok {
		specArgs["notes"] = notes
	}
	spec, err := parseOfficeDocSpec(specArgs, title, subtitle, theme, styleHint)
	if err != nil {
		return officeDocSpec{}, err
	}
	if strings.TrimSpace(spec.Title) == "" {
		spec.Title = pdfReformatDefaultTitle(input, extracted)
	}
	if strings.TrimSpace(spec.Subtitle) == "" {
		spec.Subtitle = "Reformatted PDF"
	}
	return spec, nil
}

func pdfReformatDefaultTitle(input resolvedPDFInput, extracted pdfextract.ExtractResult) string {
	if title := strings.TrimSpace(extracted.Document.Metadata["Title"]); title != "" {
		return title
	}
	if name := strings.TrimSpace(extracted.Document.FileName); name != "" {
		return strings.TrimSuffix(name, filepath.Ext(name))
	}
	name := strings.TrimSpace(pdfDisplayName(input.Original))
	if name == "" {
		name = strings.TrimSpace(filepath.Base(input.Path))
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func pdfReformatDemoteMarkdownHeadings(content string) string {
	lines := strings.Split(content, "\n")
	for idx, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		hashes := 0
		for hashes < len(trimmed) && trimmed[hashes] == '#' {
			hashes++
		}
		if hashes == 0 || hashes >= 6 || len(trimmed) <= hashes || trimmed[hashes] != ' ' {
			continue
		}
		prefixLen := len(line) - len(trimmed)
		lines[idx] = line[:prefixLen] + "#" + trimmed
	}
	return strings.Join(lines, "\n")
}

func pdfDisplayName(ref string) string {
	parsed, err := url.Parse(ref)
	if err == nil && parsed.Scheme != "" {
		base := filepath.Base(parsed.Path)
		if strings.TrimSpace(base) != "" && base != "." && base != "/" {
			return base
		}
		return "document.pdf"
	}
	base := filepath.Base(ref)
	if strings.TrimSpace(base) == "" || base == "." || base == string(filepath.Separator) {
		return "document.pdf"
	}
	return base
}

func pdfMaxBytes(args map[string]interface{}) int64 {
	value := compatInt(args, "max_bytes_mb", "maxBytesMb")
	if value <= 0 {
		value = defaultPDFMaxBytesMB
	} else {
		value = fsClamp(value, 1, hardPDFMaxBytesMB)
	}
	return int64(value) << 20
}

func clipPDFRunes(text string, limit int) (string, int, bool) {
	runes := []rune(text)
	if limit <= 0 {
		return text, len(runes), false
	}
	if len(runes) <= limit {
		return text, len(runes), false
	}
	return string(runes[:limit]), limit, true
}

func pdfAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(compatStringArg(args, "action", "op", "operation", "command")))
	switch action {
	case "", "read", "get", "show", "extract", "analyze":
		return "read"
	case "info", "metadata", "stat", "stats":
		return "info"
	default:
		return action
	}
}

func hasPDFFieldValues(args map[string]interface{}) bool {
	for _, key := range []string{"fields", "field_values", "fieldValues", "values"} {
		value, ok := compatArgValue(args, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(typed) != "" {
				return true
			}
		case []interface{}:
			if len(typed) > 0 {
				return true
			}
		case []string:
			if len(typed) > 0 {
				return true
			}
		case map[string]interface{}:
			if len(typed) > 0 {
				return true
			}
		default:
			return true
		}
	}
	return false
}

func parsePDFFillFields(args map[string]interface{}) (map[string]string, error) {
	fields := parseReplacementMap(args, "fields", "field_values", "fieldValues", "values")
	if len(fields) > 0 {
		return fields, nil
	}
	if hasPDFFieldValues(args) {
		return nil, errors.New("fields must be an object mapping pdf field names to replacement text")
	}
	return nil, nil
}

func buildPDFFillInspectStatus(inspected pdfextract.FormInspectResult) (map[string]interface{}, bool, string, []string) {
	supportedTypes := supportedPDFFillTypes()
	supportedSet := make(map[string]struct{}, len(supportedTypes))
	for _, fieldType := range supportedTypes {
		supportedSet[fieldType] = struct{}{}
	}

	unsupportedTypes := make([]string, 0, len(inspected.Fields))
	seenUnsupported := make(map[string]struct{}, len(inspected.Fields))
	for _, field := range inspected.Fields {
		fieldType := strings.TrimSpace(field.Type)
		if fieldType == "" {
			continue
		}
		if _, ok := supportedSet[fieldType]; ok {
			continue
		}
		if _, ok := seenUnsupported[fieldType]; ok {
			continue
		}
		seenUnsupported[fieldType] = struct{}{}
		unsupportedTypes = append(unsupportedTypes, fieldType)
	}
	sort.Strings(unsupportedTypes)

	fillable := strings.TrimSpace(inspected.FormType) == "acro_form" && inspected.FieldCount > 0 && len(unsupportedTypes) == 0
	degraded := false
	fallbackReason := ""
	warnings := make([]string, 0, 1)
	switch {
	case strings.TrimSpace(inspected.FormType) != "" && strings.TrimSpace(inspected.FormType) != "acro_form" && strings.TrimSpace(inspected.FormType) != "none":
		degraded = true
		fallbackReason = "native_fill_unsupported_form_type"
		warnings = append(warnings, fmt.Sprintf("native pdf fill does not currently support form type %q", inspected.FormType))
	case len(unsupportedTypes) > 0:
		degraded = true
		fallbackReason = "native_fill_unsupported_field_types"
		warnings = append(warnings, fmt.Sprintf("native pdf fill does not currently support field types: %s", strings.Join(unsupportedTypes, ", ")))
	}

	validation := map[string]interface{}{
		"ok":                      true,
		"inspectable":             true,
		"fillable":                fillable,
		"form_type":               inspected.FormType,
		"field_count":             inspected.FieldCount,
		"supported_fill_types":    supportedTypes,
		"unsupported_field_types": unsupportedTypes,
	}
	return validation, degraded, fallbackReason, warnings
}

func supportedPDFFillTypes() []string {
	return []string{"text", "combo", "list", "checkbox", "radio"}
}

func isSupportedPDFFillType(fieldType string) bool {
	switch strings.TrimSpace(fieldType) {
	case "text", "combo", "list", "checkbox", "radio":
		return true
	default:
		return false
	}
}

func validatePDFFillWriteRequest(expected map[string]string, inspected pdfextract.FormInspectResult) error {
	formType := strings.TrimSpace(inspected.FormType)
	switch formType {
	case "none":
		return errors.New("pdf fill cannot natively write this document because it does not contain an interactive form")
	case "", "acro_form":
		// Continue; field-level validation below handles unsupported requested field types.
	default:
		return fmt.Errorf("pdf fill cannot natively write form type %q yet", formType)
	}

	groups := groupPDFFillFieldsByName(inspected.Fields)
	for _, name := range sortedPDFFillFieldNames(expected) {
		fields, ok := groups[name]
		if !ok || len(fields) == 0 {
			continue
		}
		fieldType := strings.TrimSpace(fields[0].Type)
		if isSupportedPDFFillType(fieldType) {
			continue
		}
		return fmt.Errorf("pdf fill cannot natively write field %q with type %q yet", name, fieldType)
	}
	return nil
}

func verifyPDFFillOutput(expected map[string]string, inspected pdfextract.FormInspectResult) ([]string, error) {
	verified := make([]string, 0, len(expected))
	groups := groupPDFFillFieldsByName(inspected.Fields)
	for _, name := range sortedPDFFillFieldNames(expected) {
		fields, ok := groups[name]
		if !ok || len(fields) == 0 {
			return nil, fmt.Errorf("pdf fill verification failed: field %q not found in output", name)
		}
		if !pdfFieldGroupMatchesExpectedValue(fields, expected[name]) {
			return nil, fmt.Errorf("pdf fill verification failed for field %q: got %q want %q", name, describePDFFillFieldGroupValue(fields), expected[name])
		}
		verified = append(verified, name)
	}
	return verified, nil
}

func groupPDFFillFieldsByName(fields []pdfextract.FormField) map[string][]pdfextract.FormField {
	groups := make(map[string][]pdfextract.FormField, len(fields))
	for _, field := range fields {
		if name := strings.TrimSpace(field.Name); name != "" {
			groups[name] = append(groups[name], field)
		}
		if alt := strings.TrimSpace(field.AlternateName); alt != "" {
			if alt != strings.TrimSpace(field.Name) {
				groups[alt] = append(groups[alt], field)
			}
		}
	}
	return groups
}

func pdfFieldGroupMatchesExpectedValue(fields []pdfextract.FormField, expected string) bool {
	if len(fields) == 0 {
		return false
	}
	if fields[0].Type == "radio" {
		return pdfRadioGroupMatchesExpectedValue(fields, expected)
	}
	return pdfFieldMatchesExpectedValue(fields[0], expected)
}

func pdfRadioGroupMatchesExpectedValue(fields []pdfextract.FormField, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}

	matched := false
	for _, field := range fields {
		exportValue := strings.TrimSpace(field.ExportValue)
		isTarget := exportValue != "" && strings.EqualFold(exportValue, expected)
		if isTarget {
			matched = true
			if !field.Checked {
				return false
			}
			continue
		}
		if field.Checked {
			return false
		}
	}
	return matched
}

func describePDFFillFieldGroupValue(fields []pdfextract.FormField) string {
	if len(fields) == 0 {
		return ""
	}
	if fields[0].Type != "radio" {
		return strings.TrimSpace(fields[0].Value)
	}

	checked := make([]string, 0, 1)
	for _, field := range fields {
		if !field.Checked {
			continue
		}
		value := strings.TrimSpace(field.ExportValue)
		if value == "" {
			value = strings.TrimSpace(field.Value)
		}
		checked = append(checked, value)
	}
	return strings.Join(checked, ",")
}

func pdfFieldMatchesExpectedValue(field pdfextract.FormField, expected string) bool {
	expected = strings.TrimSpace(expected)
	if field.Value == expected {
		return true
	}
	switch field.Type {
	case "combo", "list":
		for _, option := range field.Options {
			if !option.Selected {
				continue
			}
			if strings.TrimSpace(option.Label) == expected {
				return true
			}
			if idx, err := strconv.Atoi(expected); err == nil && option.Index == idx {
				return true
			}
		}
		return false
	case "checkbox":
		desired, ok := parseExpectedCheckboxState(field, expected)
		return ok && field.Checked == desired
	default:
		return false
	}
}

func parseExpectedCheckboxState(field pdfextract.FormField, expected string) (bool, bool) {
	expected = strings.TrimSpace(expected)
	switch strings.ToLower(expected) {
	case "1", "true", "yes", "on", "checked":
		return true, true
	case "0", "false", "no", "off", "unchecked":
		return false, true
	}
	if exportValue := strings.TrimSpace(field.ExportValue); exportValue != "" && strings.EqualFold(exportValue, expected) {
		return true, true
	}
	return false, false
}

func sortedPDFFillFieldNames(fields map[string]string) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		names = append(names, trimmed)
	}
	sort.Strings(names)
	return names
}

func parsePDFFallbackFlags(args map[string]interface{}) (disableOCR bool, disableVision bool) {
	switch strings.ToLower(strings.TrimSpace(compatStringArg(args, "fallback_mode", "fallbackMode"))) {
	case "", "auto":
		// Use compatibility toggles below.
	case "ocr", "ocr_only":
		return false, true
	case "vision", "vision_only":
		return true, false
	case "text", "text_only", "none", "off", "disabled":
		return true, true
	}

	if enabled, ok := compatBoolArg(args, "ocr"); ok && !enabled {
		disableOCR = true
	}
	if disabled, ok := compatBoolArg(args, "disable_ocr", "disableOcr"); ok && disabled {
		disableOCR = true
	}
	if enabled, ok := compatBoolArg(args, "vision"); ok && !enabled {
		disableVision = true
	}
	if disabled, ok := compatBoolArg(args, "disable_vision", "disableVision"); ok && disabled {
		disableVision = true
	}
	return disableOCR, disableVision
}

func compatInt(args map[string]interface{}, keys ...string) int {
	if value, ok := compatArgValue(args, keys...); ok {
		if n, ok := coerceCompatInt(value); ok {
			return n
		}
	}
	return 0
}

func compatArgValue(args map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		if value, ok := args[key]; ok {
			return value, true
		}
	}
	for _, containerKey := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(args[containerKey])
		if !ok {
			continue
		}
		for _, key := range keys {
			if value, ok := nested[key]; ok {
				return value, true
			}
		}
	}
	return nil, false
}

func compatStringArg(args map[string]interface{}, keys ...string) string {
	value, ok := compatArgValue(args, keys...)
	if !ok {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func compatBoolArg(args map[string]interface{}, keys ...string) (bool, bool) {
	value, ok := compatArgValue(args, keys...)
	if !ok {
		return false, false
	}
	return asCompatBool(value)
}

func parsePDFPages(args map[string]interface{}) ([]int, error) {
	collected := make([]int, 0, 8)
	if value, ok := compatArgValue(args, "page"); ok {
		page, ok := coerceCompatInt(value)
		if !ok || page < 1 {
			return nil, fmt.Errorf("page must be a positive integer")
		}
		collected = appendUniqueInt(collected, page)
	}
	if value, ok := compatArgValue(args, "pages"); ok {
		pages, err := parsePDFPageValue(value)
		if err != nil {
			return nil, err
		}
		for _, page := range pages {
			collected = appendUniqueInt(collected, page)
		}
	}
	return collected, nil
}

func parsePDFPageValue(value interface{}) ([]int, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case int, int32, int64, float64, float32:
		page, ok := coerceCompatInt(typed)
		if !ok || page < 1 {
			return nil, fmt.Errorf("pages must contain positive integers")
		}
		return []int{page}, nil
	case string:
		return parsePDFPageRange(typed)
	case []string:
		out := make([]int, 0, len(typed))
		for _, item := range typed {
			pages, err := parsePDFPageRange(item)
			if err != nil {
				return nil, err
			}
			for _, page := range pages {
				out = appendUniqueInt(out, page)
			}
		}
		return out, nil
	case []interface{}:
		out := make([]int, 0, len(typed))
		for _, item := range typed {
			pages, err := parsePDFPageValue(item)
			if err != nil {
				return nil, err
			}
			for _, page := range pages {
				out = appendUniqueInt(out, page)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported pages format")
	}
}

func parsePDFPageRange(raw string) ([]int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		if strings.Contains(item, "-") {
			bounds := strings.SplitN(item, "-", 2)
			start, ok := coerceCompatInt(strings.TrimSpace(bounds[0]))
			if !ok || start < 1 {
				return nil, fmt.Errorf("invalid page range %q", item)
			}
			end, ok := coerceCompatInt(strings.TrimSpace(bounds[1]))
			if !ok || end < 1 || end < start {
				return nil, fmt.Errorf("invalid page range %q", item)
			}
			for page := start; page <= end; page++ {
				out = appendUniqueInt(out, page)
			}
			continue
		}
		page, ok := coerceCompatInt(item)
		if !ok || page < 1 {
			return nil, fmt.Errorf("invalid page number %q", item)
		}
		out = appendUniqueInt(out, page)
	}
	return out, nil
}

func appendUniqueInt(values []int, value int) []int {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// RegisterPDFTool registers the native PDF tool.
func RegisterPDFTool(registry *Registry, service PDFService) {
	if registry == nil || service == nil {
		return
	}
	tool := NewPDFTool(service)
	if read, ok := registry.Get("file_read").(*FileReadTool); ok && read != nil {
		tool.scope = newFSToolScope(read.AllowedPaths)
	}
	registry.Register(tool)
}
