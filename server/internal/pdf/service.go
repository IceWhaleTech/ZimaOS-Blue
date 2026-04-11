package pdf

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"go.uber.org/zap"
)

const (
	defaultMaxPages            = 20
	hardMaxPages               = 200
	defaultMaxChars            = 50000
	hardMaxChars               = 200000
	instanceAcquireTimeout     = 30 * time.Second
	defaultRequestTimeout      = 2 * time.Minute
	engineName                 = "pdfium/webassembly"
	ocrRenderDPI               = 200
	pdfiumRuntimeFileName      = "pdfium.wasm"
	pdfiumRuntimeRepoOwner     = "klippa-app"
	pdfiumRuntimeRepoName      = "go-pdfium"
	pdfiumRuntimeRepoRef       = "852818152bff9c1e366b8737a0802481f3a80e0f"
	pdfiumRuntimeSourcePath    = "webassembly/pdfium.wasm"
	defaultRuntimeDirBaseName  = "zimaos-blue"
	defaultRuntimeDirAssetName = "pdfium"
)

var pdfTextRuneReplacer = strings.NewReplacer(
	"\u00a0", " ",
	"\u1680", " ",
	"\u2000", " ",
	"\u2001", " ",
	"\u2002", " ",
	"\u2003", " ",
	"\u2004", " ",
	"\u2005", " ",
	"\u2006", " ",
	"\u2007", " ",
	"\u2008", " ",
	"\u2009", " ",
	"\u200a", " ",
	"\u2028", "\n",
	"\u2029", "\n",
	"\u202f", " ",
	"\u205f", " ",
	"\u3000", " ",
	"\u00ad", "",
	"\u200b", "",
	"\u200c", "",
	"\u200d", "",
	"\u2060", "",
	"\ufeff", "",
	"\ufffe", "",
	"\ufffd", "",
	"ﬁ", "fi",
	"ﬂ", "fl",
	"ﬀ", "ff",
	"ﬃ", "ffi",
	"ﬄ", "ffl",
	"ﬅ", "ft",
	"ﬆ", "st",
	"“", "\"",
	"”", "\"",
	"„", "\"",
	"‟", "\"",
	"‘", "'",
	"’", "'",
	"‚", "'",
	"‛", "'",
)

// OCRService provides OCR fallback for rendered PDF pages.
type OCRService interface {
	Extract(ctx context.Context, imagePNG []byte) (ocrruntime.Result, error)
}

type ServiceConfig struct {
	RuntimeDir   string
	AutoDownload bool
	HTTPClient   *http.Client
}

type assetDownloader func(ctx context.Context, url, path string) error

type pdfRuntimeConfig struct {
	MinIdle      int
	MaxIdle      int
	MaxTotal     int
	ReuseWorkers bool
	Stdout       io.Writer
	Stderr       io.Writer
	WASM         []byte
}

type pdfRuntimePoolFactory func(config pdfRuntimeConfig) (any, error)

// DocumentInfo describes a PDF document.
type DocumentInfo struct {
	Path       string            `json:"path"`
	FileName   string            `json:"file_name"`
	SizeBytes  int64             `json:"size_bytes"`
	ModifiedAt time.Time         `json:"modified_at"`
	PageCount  int               `json:"page_count"`
	Engine     string            `json:"engine"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// PageText contains text extracted from a single page.
type PageText struct {
	Number    int         `json:"number"`
	Markdown  string      `json:"markdown,omitempty"`
	Blocks    []PageBlock `json:"blocks,omitempty"`
	Tables    []PageTable `json:"tables,omitempty"`
	Text      string      `json:"text"`
	RawText   string      `json:"raw_text,omitempty"`
	CharCount int         `json:"char_count"`
	Source    string      `json:"source,omitempty"`
	Empty     bool        `json:"empty,omitempty"`
}

// ExtractRequest controls PDF text extraction.
type ExtractRequest struct {
	Path                  string
	Pages                 []int
	MaxPages              int
	MaxChars              int
	IncludePages          bool
	IncludeMarkdown       bool
	IncludeOutline        bool
	IncludeLayout         bool
	IncludeHeadersFooters bool
	DisableOCR            bool
	DisableVision         bool
}

// ExtractResult contains extracted PDF text and related metadata.
type ExtractResult struct {
	Document        DocumentInfo   `json:"document"`
	Outline         []OutlineEntry `json:"outline,omitempty"`
	Pages           []PageText     `json:"pages,omitempty"`
	Text            string         `json:"text"`
	RawText         string         `json:"raw_text,omitempty"`
	Markdown        string         `json:"markdown,omitempty"`
	SelectedPages   []int          `json:"selected_pages,omitempty"`
	CharCount       int            `json:"char_count"`
	Truncated       bool           `json:"truncated,omitempty"`
	OCRUsed         bool           `json:"ocr_used,omitempty"`
	OCRPages        []int          `json:"ocr_pages,omitempty"`
	OCRModels       []string       `json:"ocr_models,omitempty"`
	OCREngine       string         `json:"ocr_engine,omitempty"`
	VisionUsed      bool           `json:"vision_used,omitempty"`
	VisionPages     []int          `json:"vision_pages,omitempty"`
	VisionProviders []string       `json:"vision_providers,omitempty"`
	VisionModels    []string       `json:"vision_models,omitempty"`
	VisionEngine    string         `json:"vision_engine,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
}

// FormFieldRect describes the widget rectangle for an interactive form field.
type FormFieldRect struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
}

// FormFieldOption describes a selectable option for choice controls.
type FormFieldOption struct {
	Index    int    `json:"index"`
	Label    string `json:"label,omitempty"`
	Selected bool   `json:"selected,omitempty"`
}

// FormField describes a single interactive PDF form field.
type FormField struct {
	PageNumber    int               `json:"page_number"`
	Name          string            `json:"name,omitempty"`
	AlternateName string            `json:"alternate_name,omitempty"`
	Type          string            `json:"type"`
	Value         string            `json:"value,omitempty"`
	ExportValue   string            `json:"export_value,omitempty"`
	ReadOnly      bool              `json:"read_only,omitempty"`
	Required      bool              `json:"required,omitempty"`
	NoExport      bool              `json:"no_export,omitempty"`
	Checked       bool              `json:"checked,omitempty"`
	Rect          *FormFieldRect    `json:"rect,omitempty"`
	Options       []FormFieldOption `json:"options,omitempty"`
}

// FormInspectResult contains interactive form metadata for a PDF document.
type FormInspectResult struct {
	Document   DocumentInfo `json:"document"`
	FormType   string       `json:"form_type"`
	FieldCount int          `json:"field_count"`
	Fields     []FormField  `json:"fields,omitempty"`
	Warnings   []string     `json:"warnings,omitempty"`
}

// FillFormRequest controls native PDF form filling for the first safe write slice.
type FillFormRequest struct {
	Path   string            `json:"path"`
	Fields map[string]string `json:"fields"`
}

// FillFormResult contains the saved PDF bytes and lightweight write metadata.
type FillFormResult struct {
	Document      DocumentInfo `json:"document"`
	Bytes         []byte       `json:"-"`
	UpdatedFields []string     `json:"updated_fields,omitempty"`
	Warnings      []string     `json:"warnings,omitempty"`
}

// Service extracts metadata and structured text from PDFs.
type Service struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	ocr          OCRService
	vision       VisionService
	runtimeDir   string
	autoDownload bool
	httpClient   *http.Client
	initOnce     sync.Once
	initErr      error
	pool         any
	initPool     pdfRuntimePoolFactory
	download     assetDownloader
}

// NewService creates a new PDF extraction service.
func NewService(logger *zap.Logger, ocr OCRService, configs ...ServiceConfig) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	var cfg ServiceConfig
	if len(configs) > 0 {
		cfg = configs[0]
	}
	if cfg.RuntimeDir == "" {
		cfg.RuntimeDir = filepath.Join(os.TempDir(), defaultRuntimeDirBaseName, defaultRuntimeDirAssetName)
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = network.NewPooledHTTPClient(defaultRequestTimeout)
	}
	svc := &Service{
		logger:       logger,
		ocr:          ocr,
		runtimeDir:   cfg.RuntimeDir,
		autoDownload: cfg.AutoDownload,
		httpClient:   cfg.HTTPClient,
	}
	svc.initPool = defaultPDFRuntimePoolFactory()
	svc.download = svc.downloadFile
	return svc
}

// SetVisionService wires an optional LLM vision fallback.
func (s *Service) SetVisionService(vision VisionService) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vision = vision
}

func (s *Service) visionService() VisionService {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vision
}

// Close releases the PDFium pool.
func (s *Service) Close() error {
	if s == nil || s.pool == nil {
		return nil
	}
	if closer, ok := s.pool.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func resolvePath(path string) (string, os.FileInfo, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil, fmt.Errorf("path is required")
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return "", nil, fmt.Errorf("resolve path: %w", err)
	}
	stat, err := os.Stat(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("stat pdf: %w", err)
	}
	if stat.IsDir() {
		return "", nil, fmt.Errorf("path must be a file")
	}
	if strings.ToLower(filepath.Ext(resolved)) != ".pdf" {
		return "", nil, fmt.Errorf("path must point to a .pdf file")
	}
	return resolved, stat, nil
}

func (s *Service) downloadWithFallback(ctx context.Context, urls []string, path string, assetName string) error {
	var lastErr error
	for _, url := range urls {
		if err := s.download(ctx, url, path); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		return fmt.Errorf("no %s download sources configured", assetName)
	}
	return fmt.Errorf("download %s: %w", assetName, lastErr)
}

func (s *Service) downloadFile(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("store file: %w", err)
	}
	s.logger.Info("downloaded PDF asset", zap.String("path", path), zap.String("url", url))
	return nil
}

func resolveSelectedPages(pageCount int, requested []int, maxPages int) ([]int, []string, error) {
	limit := clampMaxPages(maxPages)
	if pageCount <= 0 {
		return nil, nil, nil
	}
	if len(requested) == 0 {
		selected := make([]int, 0, minInt(pageCount, limit))
		for i := 1; i <= pageCount && len(selected) < limit; i++ {
			selected = append(selected, i)
		}
		warnings := []string{}
		if pageCount > limit {
			warnings = append(warnings, fmt.Sprintf("defaulting to first %d pages", limit))
		}
		return selected, warnings, nil
	}

	seen := make(map[int]struct{}, len(requested))
	selected := make([]int, 0, len(requested))
	for _, page := range requested {
		if page < 1 || page > pageCount {
			return nil, nil, fmt.Errorf("page %d out of range (document has %d pages)", page, pageCount)
		}
		if _, ok := seen[page]; ok {
			continue
		}
		seen[page] = struct{}{}
		selected = append(selected, page)
	}
	warnings := []string{}
	if len(selected) > limit {
		selected = append([]int(nil), selected[:limit]...)
		warnings = append(warnings, fmt.Sprintf("page selection truncated to %d pages", limit))
	}
	return selected, warnings, nil
}

func clampMaxPages(value int) int {
	if value <= 0 {
		return defaultMaxPages
	}
	if value > hardMaxPages {
		return hardMaxPages
	}
	return value
}

func clampMaxChars(value int) int {
	if value <= 0 {
		return defaultMaxChars
	}
	if value > hardMaxChars {
		return hardMaxChars
	}
	return value
}

func normalizeText(text string) string {
	text = canonicalizePDFText(text)
	if text == "" {
		return ""
	}

	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		line := normalizePDFLine(raw)
		if line == "" {
			if len(lines) == 0 || lines[len(lines)-1] == "" {
				continue
			}
			lines = append(lines, "")
			continue
		}
		lines = append(lines, line)
	}

	lines = mergeWrappedPDFLines(lines)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func canonicalizeExtractedPDFText(text string) string {
	return strings.TrimSpace(canonicalizePDFText(text))
}

func effectiveRawPDFText(rawText, normalizedText string) string {
	rawText = canonicalizeExtractedPDFText(rawText)
	if rawText != "" {
		return rawText
	}
	return strings.TrimSpace(normalizedText)
}

func canonicalizePDFText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = pdfTextRuneReplacer.Replace(text)
	text = repairMalformedPDFText(text)

	var sb strings.Builder
	sb.Grow(len(text))
	for _, r := range text {
		switch r {
		case '\n':
			sb.WriteByte('\n')
		case '\t':
			sb.WriteByte(' ')
		default:
			if shouldDropPDFRune(r) {
				continue
			}
			if unicode.IsSpace(r) {
				sb.WriteByte(' ')
				continue
			}
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func shouldDropPDFRune(r rune) bool {
	switch r {
	case 0, 0x000b, 0x000c:
		return true
	case 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e:
		return true
	default:
		return unicode.IsControl(r) && r != '\n'
	}
}

func normalizePDFLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	line = normalizePDFListPrefix(line)
	if !looksLikePDFTableLine(line) {
		line = collapsePDFSpaces(line)
	}
	return strings.TrimSpace(line)
}

func normalizePDFListPrefix(line string) string {
	if line == "" {
		return ""
	}
	runes := []rune(line)
	if len(runes) == 0 {
		return ""
	}
	if isPDFBulletRune(runes[0]) {
		rest := strings.TrimSpace(string(runes[1:]))
		if rest == "" {
			return ""
		}
		return "- " + rest
	}

	fields := strings.Fields(line)
	if len(fields) >= 2 {
		if label := trimPDFEnumToken(fields[0]); label != "" {
			return label + ". " + strings.Join(fields[1:], " ")
		}
		if len(fields) >= 3 && isPDFEnumLabel(fields[0]) && isPDFEnumSeparator(fields[1]) {
			return fields[0] + ". " + strings.Join(fields[2:], " ")
		}
	}
	return line
}

func isPDFBulletRune(r rune) bool {
	switch r {
	case '-', '*', '•', '●', '◦', '▪', '■', '□', '▫', '◆', '◇', '‣', '∙', '·', '◾', '◽', '►', '▸', '▹', '→', '➤', '➢', '➣', '–', '—', '\uf0a7', '\uf0b7':
		return true
	default:
		return false
	}
}

func trimPDFEnumToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	last, size := utf8.DecodeLastRuneInString(token)
	if last == utf8.RuneError || size == 0 {
		return ""
	}
	if !isPDFEnumSeparator(string(last)) && last != '.' {
		return ""
	}
	label := strings.TrimSpace(token[:len(token)-size])
	if !isPDFEnumLabel(label) {
		return ""
	}
	return label
}

func isPDFEnumSeparator(token string) bool {
	switch token {
	case "-", ":", "：", ")", "]", "}":
		return true
	default:
		return false
	}
}

func isPDFEnumLabel(label string) bool {
	label = strings.TrimSpace(label)
	if label == "" || utf8.RuneCountInString(label) > 8 {
		return false
	}
	allDigits := true
	for _, r := range label {
		if !unicode.IsDigit(r) {
			allDigits = false
			break
		}
	}
	if allDigits {
		return true
	}
	runes := []rune(label)
	if len(runes) == 1 && unicode.IsLetter(runes[0]) {
		return true
	}
	lower := strings.ToLower(label)
	if utf8.RuneCountInString(lower) > 5 {
		return false
	}
	for _, r := range lower {
		switch r {
		case 'i', 'v', 'x':
		default:
			return false
		}
	}
	return true
}

func looksLikePDFTableLine(line string) bool {
	if strings.Contains(line, "|") {
		return true
	}
	gaps := 0
	run := 0
	for _, r := range line {
		if r == ' ' {
			run++
			continue
		}
		if run >= 2 {
			gaps++
		}
		run = 0
	}
	if run >= 2 {
		gaps++
	}
	return gaps >= 2
}

func collapsePDFSpaces(line string) string {
	var sb strings.Builder
	sb.Grow(len(line))
	lastSpace := false
	for _, r := range line {
		if r == ' ' {
			if lastSpace {
				continue
			}
			lastSpace = true
			sb.WriteByte(' ')
			continue
		}
		lastSpace = false
		sb.WriteRune(r)
	}
	return sb.String()
}

func mergeWrappedPDFLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}

	merged := make([]string, 0, len(lines))
	current := ""
	flush := func() {
		if strings.TrimSpace(current) == "" {
			return
		}
		merged = append(merged, strings.TrimSpace(current))
		current = ""
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			if len(merged) == 0 || merged[len(merged)-1] == "" {
				continue
			}
			merged = append(merged, "")
			continue
		}
		if current == "" {
			current = line
			continue
		}
		if shouldMergePDFLines(current, line) {
			current = joinPDFWrappedLines(current, line)
			continue
		}
		flush()
		current = line
	}
	flush()

	for len(merged) > 0 && merged[0] == "" {
		merged = merged[1:]
	}
	for len(merged) > 0 && merged[len(merged)-1] == "" {
		merged = merged[:len(merged)-1]
	}
	return merged
}

func shouldMergePDFLines(prev, next string) bool {
	prev = strings.TrimSpace(prev)
	next = strings.TrimSpace(next)
	if prev == "" || next == "" {
		return false
	}
	if looksLikePDFTableLine(prev) || looksLikePDFTableLine(next) {
		return false
	}
	if isPDFListLikeLine(prev) {
		return !isPDFListLikeLine(next) && !isPDFHeadingLikeLine(next)
	}
	if isPDFListLikeLine(next) || isPDFHeadingLikeLine(prev) || isPDFHeadingLikeLine(next) {
		return false
	}
	if endsWithPDFSentenceBoundary(prev) || strings.HasSuffix(prev, ":") {
		return false
	}
	if strings.HasSuffix(prev, "-") && startsWithPDFWord(next) {
		return true
	}
	if endsWithPDFContinuationPunctuation(prev) {
		return true
	}
	if endsWithPDFWord(prev) && startsWithPDFWord(next) {
		return true
	}
	return utf8.RuneCountInString(prev) >= 60 && startsWithPDFWord(next)
}

func isPDFListLikeLine(line string) bool {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return true
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	return trimPDFEnumToken(fields[0]) != ""
}

func isPDFHeadingLikeLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || isPDFListLikeLine(line) || looksLikePDFTableLine(line) {
		return false
	}
	if utf8.RuneCountInString(line) > 80 {
		return false
	}
	last := lastNonSpacePDFRune(line)
	switch last {
	case '.', ',', ';', '?', '!':
		return false
	}
	words := strings.Fields(line)
	if len(words) == 0 || len(words) > 10 {
		return false
	}
	alphaWords := 0
	titleishWords := 0
	for _, word := range words {
		clean := strings.Trim(word, "\"'`()[]{}.,;:-")
		if clean == "" {
			continue
		}
		r, _ := utf8.DecodeRuneInString(clean)
		if !unicode.IsLetter(r) {
			continue
		}
		alphaWords++
		if clean == strings.ToUpper(clean) || unicode.IsUpper(r) {
			titleishWords++
		}
	}
	if alphaWords == 0 {
		return false
	}
	if len(words) <= 4 && titleishWords == alphaWords {
		return true
	}
	return alphaWords >= 3 && titleishWords*2 >= alphaWords*3
}

func endsWithPDFSentenceBoundary(text string) bool {
	switch lastNonSpacePDFRune(text) {
	case '.', '!', '?':
		return true
	default:
		return false
	}
}

func endsWithPDFContinuationPunctuation(text string) bool {
	switch lastNonSpacePDFRune(text) {
	case ',', ';', '/', '(', '[', '{':
		return true
	default:
		return false
	}
}

func endsWithPDFWord(text string) bool {
	last := lastNonSpacePDFRune(text)
	return unicode.IsLetter(last) || unicode.IsDigit(last) || last == ')' || last == '"' || last == '\''
}

func startsWithPDFWord(text string) bool {
	first := firstNonSpacePDFRune(text)
	return unicode.IsLetter(first) || unicode.IsDigit(first) || first == '(' || first == '[' || first == '"' || first == '\''
}

func joinPDFWrappedLines(prev, next string) string {
	prev = strings.TrimSpace(prev)
	next = strings.TrimSpace(next)
	if prev == "" {
		return next
	}
	if next == "" {
		return prev
	}
	if strings.HasSuffix(prev, "-") && startsWithPDFWord(next) {
		trimmedPrev := strings.TrimSuffix(prev, "-")
		return strings.TrimSpace(trimmedPrev) + next
	}
	return prev + " " + next
}

func lastNonSpacePDFRune(text string) rune {
	for len(text) > 0 {
		r, size := utf8.DecodeLastRuneInString(text)
		if r == utf8.RuneError && size == 0 {
			return 0
		}
		if !unicode.IsSpace(r) {
			return r
		}
		text = text[:len(text)-size]
	}
	return 0
}

func firstNonSpacePDFRune(text string) rune {
	for _, r := range text {
		if !unicode.IsSpace(r) {
			return r
		}
	}
	return 0
}

func resultTextPrefix(pageNumber, total int) string {
	if total <= 1 {
		return ""
	}
	return fmt.Sprintf("[Page %d]\n", pageNumber)
}

func clipRunes(text string, limit int) (string, int, bool) {
	runes := []rune(text)
	if len(runes) <= limit {
		return text, len(runes), false
	}
	return string(runes[:limit]), limit, true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func textLooksWeak(text string) bool {
	trimmed := normalizeText(text)
	if trimmed == "" {
		return true
	}
	lettersDigits := 0
	punct := 0
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			lettersDigits++
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			punct++
		}
	}
	runes := utf8.RuneCountInString(trimmed)
	if runes < 16 {
		return true
	}
	if lettersDigits == 0 {
		return true
	}
	return punct > lettersDigits*2
}

func textQualityScore(text string) int {
	score := 0
	for _, r := range normalizeText(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			score += 3
		case unicode.IsSpace(r):
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			score--
		default:
			score--
		}
	}
	return score
}

func preferVisionText(currentText, visionText, currentSource string) bool {
	visionText = normalizeText(visionText)
	currentText = normalizeText(currentText)
	if visionText == "" {
		return false
	}
	if currentText == "" {
		return true
	}
	if currentSource != "ocr" {
		return false
	}
	return textQualityScore(visionText) > textQualityScore(currentText)
}
