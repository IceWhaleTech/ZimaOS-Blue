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
		Description: "Read PDF metadata or extract text from local/remote PDFs with page selection, output limits, and fallback control.",
		Icon:        "pdf",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"info", "read"},
					"description": "Operation to perform. Defaults to read.",
				},
				"path":      map[string]interface{}{"type": "string", "description": "Path or URL to a single PDF."},
				"paths":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Multiple PDF paths or URLs. Deduped and capped at 10."},
				"pages":     map[string]interface{}{"description": "Page selection as '1,3-5', a single number, or an array of page numbers."},
				"max_pages": map[string]interface{}{"type": "integer", "description": "Maximum pages to extract."},
				"max_chars": map[string]interface{}{"type": "integer", "description": "Maximum characters to return."},
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
			},
		},
	}
}

// Execute performs the requested PDF action.
func (t *PDFTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("pdf service not available")
	}
	args = normalizePDFArgs(args)
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

	switch pdfAction(args) {
	case "info":
		return t.executeInfo(ctx, inputs)
	case "read":
		return t.executeRead(ctx, args, inputs)
	default:
		return nil, fmt.Errorf("unsupported pdf action")
	}
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
