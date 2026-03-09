package pdf

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
	"go.uber.org/zap"
)

const (
	defaultMaxPages        = 20
	hardMaxPages           = 200
	defaultMaxChars        = 20000
	hardMaxChars           = 200000
	instanceAcquireTimeout = 30 * time.Second
	engineName             = "pdfium/webassembly"
	ocrRenderDPI           = 200
)

// OCRService provides OCR fallback for rendered PDF pages.
type OCRService interface {
	Extract(ctx context.Context, imagePNG []byte) (ocrruntime.Result, error)
}

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
	Number    int    `json:"number"`
	Text      string `json:"text"`
	CharCount int    `json:"char_count"`
	Source    string `json:"source,omitempty"`
	Empty     bool   `json:"empty,omitempty"`
}

// ExtractRequest controls PDF text extraction.
type ExtractRequest struct {
	Path          string
	Pages         []int
	MaxPages      int
	MaxChars      int
	IncludePages  bool
	DisableOCR    bool
	DisableVision bool
}

// ExtractResult contains extracted PDF text and related metadata.
type ExtractResult struct {
	Document        DocumentInfo `json:"document"`
	Text            string       `json:"text"`
	Pages           []PageText   `json:"pages,omitempty"`
	SelectedPages   []int        `json:"selected_pages,omitempty"`
	CharCount       int          `json:"char_count"`
	Truncated       bool         `json:"truncated,omitempty"`
	OCRUsed         bool         `json:"ocr_used,omitempty"`
	OCRPages        []int        `json:"ocr_pages,omitempty"`
	OCRModels       []string     `json:"ocr_models,omitempty"`
	OCREngine       string       `json:"ocr_engine,omitempty"`
	VisionUsed      bool         `json:"vision_used,omitempty"`
	VisionPages     []int        `json:"vision_pages,omitempty"`
	VisionProviders []string     `json:"vision_providers,omitempty"`
	VisionModels    []string     `json:"vision_models,omitempty"`
	VisionEngine    string       `json:"vision_engine,omitempty"`
	Warnings        []string     `json:"warnings,omitempty"`
}

// Service extracts metadata and text from PDFs via PDFium WASM.
type Service struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	ocr      OCRService
	vision   VisionService
	initOnce sync.Once
	initErr  error
	pool     pdfium.Pool
}

// NewService creates a new PDF extraction service.
func NewService(logger *zap.Logger, ocr OCRService) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{logger: logger, ocr: ocr}
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
	return s.pool.Close()
}

// Info reads document-level information for a PDF file.
func (s *Service) Info(ctx context.Context, path string) (DocumentInfo, error) {
	if err := ctx.Err(); err != nil {
		return DocumentInfo{}, err
	}
	resolvedPath, stat, err := resolvePath(path)
	if err != nil {
		return DocumentInfo{}, err
	}
	instance, err := s.getInstance()
	if err != nil {
		return DocumentInfo{}, err
	}
	defer instance.Close()

	doc, err := openDocument(instance, resolvedPath)
	if err != nil {
		return DocumentInfo{}, err
	}
	defer closeDocument(instance, doc.Document)

	return buildDocumentInfo(instance, resolvedPath, stat, doc.Document)
}

// Extract reads text content from a PDF file.
func (s *Service) Extract(ctx context.Context, req ExtractRequest) (ExtractResult, error) {
	if err := ctx.Err(); err != nil {
		return ExtractResult{}, err
	}
	resolvedPath, stat, err := resolvePath(req.Path)
	if err != nil {
		return ExtractResult{}, err
	}
	instance, err := s.getInstance()
	if err != nil {
		return ExtractResult{}, err
	}
	defer instance.Close()

	doc, err := openDocument(instance, resolvedPath)
	if err != nil {
		return ExtractResult{}, err
	}
	defer closeDocument(instance, doc.Document)

	info, err := buildDocumentInfo(instance, resolvedPath, stat, doc.Document)
	if err != nil {
		return ExtractResult{}, err
	}
	selectedPages, warnings, err := resolveSelectedPages(info.PageCount, req.Pages, req.MaxPages)
	if err != nil {
		return ExtractResult{}, err
	}
	if len(selectedPages) == 0 {
		return ExtractResult{Document: info}, nil
	}

	maxChars := clampMaxChars(req.MaxChars)
	result := ExtractResult{Document: info, Warnings: warnings}
	remainingChars := maxChars
	vision := s.visionService()

	for _, pageNumber := range selectedPages {
		if err := ctx.Err(); err != nil {
			return ExtractResult{}, err
		}
		pageText, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: pageNumber - 1}},
		})
		if err != nil {
			return ExtractResult{}, fmt.Errorf("extract page %d: %w", pageNumber, err)
		}

		pageOnlyText := normalizeText(pageText.Text)
		source := "text"
		if pageOnlyText == "" {
			source = "none"
		}

		var ocrResult ocrruntime.Result
		if pageOnlyText == "" && !req.DisableOCR && s.ocr != nil {
			ocrResult, err = s.extractPageOCR(ctx, instance, doc.Document, pageNumber)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("page %d OCR failed: %v", pageNumber, err))
			} else {
				pageOnlyText = normalizeText(ocrResult.Text)
				if pageOnlyText != "" {
					source = "ocr"
					result.OCRUsed = true
					result.OCRPages = append(result.OCRPages, pageNumber)
					result.OCREngine = ocrResult.Engine
					if ocrResult.Model != "" {
						result.OCRModels = appendUniqueString(result.OCRModels, ocrResult.Model)
					}
					result.Warnings = append(result.Warnings, ocrResult.Warnings...)
				}
			}
		}

		shouldTryVision := vision != nil && !req.DisableVision && (pageOnlyText == "" || (source == "ocr" && textLooksWeak(pageOnlyText)))
		if shouldTryVision {
			visionResult, err := s.extractPageVision(ctx, instance, doc.Document, pageNumber, vision)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("page %d vision fallback failed: %v", pageNumber, err))
			} else if preferVisionText(pageOnlyText, visionResult.Text, source) {
				if source == "ocr" && pageOnlyText != "" {
					result.Warnings = append(result.Warnings, fmt.Sprintf("page %d used vision fallback because OCR looked low-confidence", pageNumber))
				}
				pageOnlyText = normalizeText(visionResult.Text)
				if pageOnlyText != "" {
					source = "vision"
					result.VisionUsed = true
					result.VisionPages = append(result.VisionPages, pageNumber)
					if visionResult.Provider != "" {
						result.VisionProviders = appendUniqueString(result.VisionProviders, visionResult.Provider)
					}
					if visionResult.Model != "" {
						result.VisionModels = appendUniqueString(result.VisionModels, visionResult.Model)
					}
					result.VisionEngine = visionResult.Engine
				}
			}
		}

		if pageOnlyText == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("page %d has no extractable text", pageNumber))
		}

		prefix := resultTextPrefix(pageNumber, len(selectedPages))
		pageOutput := pageOnlyText
		if prefix != "" {
			pageOutput = prefix + pageOnlyText
		}

		clippedOutput, outputChars, wasClipped := clipRunes(pageOutput, remainingChars)
		if clippedOutput != "" {
			if result.Text != "" {
				result.Text += "\n\n"
			}
			result.Text += clippedOutput
			remainingChars -= outputChars
		}

		result.SelectedPages = append(result.SelectedPages, pageNumber)
		if req.IncludePages {
			pageEntryText := pageOnlyText
			pageEntryChars := utf8.RuneCountInString(pageOnlyText)
			if wasClipped {
				availableForText := outputChars - utf8.RuneCountInString(prefix)
				if availableForText < 0 {
					availableForText = 0
				}
				pageEntryText, pageEntryChars, _ = clipRunes(pageOnlyText, availableForText)
			}
			result.Pages = append(result.Pages, PageText{Number: pageNumber, Text: pageEntryText, CharCount: pageEntryChars, Source: source, Empty: pageOnlyText == ""})
		}

		if wasClipped || remainingChars <= 0 {
			result.Truncated = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("output truncated at %d characters", maxChars))
			break
		}
	}

	result.CharCount = utf8.RuneCountInString(result.Text)
	return result, nil
}

func (s *Service) extractPageOCR(ctx context.Context, instance pdfium.Pdfium, document references.FPDF_DOCUMENT, pageNumber int) (ocrruntime.Result, error) {
	rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
		DPI:  ocrRenderDPI,
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: document, Index: pageNumber - 1}},
	})
	if err != nil {
		return ocrruntime.Result{}, fmt.Errorf("render page: %w", err)
	}
	defer rendered.Cleanup()

	var buf bytes.Buffer
	if err := png.Encode(&buf, rendered.Result.Image); err != nil {
		return ocrruntime.Result{}, fmt.Errorf("encode page image: %w", err)
	}
	return s.ocr.Extract(ctx, buf.Bytes())
}

func (s *Service) extractPageVision(ctx context.Context, instance pdfium.Pdfium, document references.FPDF_DOCUMENT, pageNumber int, vision VisionService) (VisionResult, error) {
	rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
		DPI:  ocrRenderDPI,
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: document, Index: pageNumber - 1}},
	})
	if err != nil {
		return VisionResult{}, fmt.Errorf("render page: %w", err)
	}
	defer rendered.Cleanup()

	var buf bytes.Buffer
	if err := png.Encode(&buf, rendered.Result.Image); err != nil {
		return VisionResult{}, fmt.Errorf("encode page image: %w", err)
	}
	return vision.Extract(ctx, buf.Bytes())
}

func (s *Service) getInstance() (pdfium.Pdfium, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	instance, err := s.pool.GetInstance(instanceAcquireTimeout)
	if err != nil {
		return nil, fmt.Errorf("acquire pdfium instance: %w", err)
	}
	return instance, nil
}

func (s *Service) ensureReady() error {
	s.initOnce.Do(func() {
		s.pool, s.initErr = webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1, ReuseWorkers: true, Stdout: io.Discard, Stderr: io.Discard})
		if s.initErr != nil {
			s.initErr = fmt.Errorf("init pdfium: %w", s.initErr)
		}
	})
	return s.initErr
}

func buildDocumentInfo(instance pdfium.Pdfium, path string, stat os.FileInfo, document references.FPDF_DOCUMENT) (DocumentInfo, error) {
	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: document})
	if err != nil {
		return DocumentInfo{}, fmt.Errorf("get page count: %w", err)
	}
	info := DocumentInfo{Path: path, FileName: filepath.Base(path), SizeBytes: stat.Size(), ModifiedAt: stat.ModTime().UTC(), PageCount: pageCount.PageCount, Engine: engineName}
	if metadata, err := instance.GetMetaData(&requests.GetMetaData{Document: document}); err == nil && metadata != nil {
		info.Metadata = metadataToMap(metadata.Tags)
	}
	return info, nil
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

func openDocument(instance pdfium.Pdfium, path string) (*responses.OpenDocument, error) {
	doc, err := instance.OpenDocument(&requests.OpenDocument{FilePath: &path})
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	return doc, nil
}

func closeDocument(instance pdfium.Pdfium, document references.FPDF_DOCUMENT) {
	if document == "" {
		return
	}
	_, _ = instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: document})
}

func metadataToMap(tags []responses.GetMetaDataTag) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := strings.TrimSpace(tag.Tag)
		value := strings.TrimSpace(tag.Value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
	text = strings.ReplaceAll(text, "\x00", "")
	return strings.TrimSpace(text)
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
