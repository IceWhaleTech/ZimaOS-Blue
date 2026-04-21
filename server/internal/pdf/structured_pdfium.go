//go:build !darwin

package pdf

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
)

func (s *Service) extractWithPDFium(
	ctx context.Context,
	req ExtractRequest,
	resolvedPath string,
	stat os.FileInfo,
) (ExtractResult, error) {
	instance, err := s.getInstance(ctx)
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
	limit := clampMaxPages(req.MaxPages)
	var selectedPages []int
	var warnings []string
	if len(req.Pages) == 0 && info.PageCount > limit {
		tocText := extractPDFiumTOCProbeText(instance, doc.Document, minInt(info.PageCount, 5))
		tocTargets := extractTOCPageTargets(tocText, info.PageCount)
		if len(tocTargets) > 0 {
			selectedPages, warnings = resolveDefaultSelectedPages(info.PageCount, limit, tocTargets)
		} else {
			selectedPages, warnings, err = resolveSelectedPages(info.PageCount, req.Pages, req.MaxPages)
			if err != nil {
				return ExtractResult{}, err
			}
		}
	} else {
		selectedPages, warnings, err = resolveSelectedPages(info.PageCount, req.Pages, req.MaxPages)
		if err != nil {
			return ExtractResult{}, err
		}
	}
	if len(selectedPages) == 0 {
		return ExtractResult{Document: info}, nil
	}

	vision := s.visionService()
	sources := make([]pdfPageSource, 0, len(selectedPages))
	result := ExtractResult{Document: info, Warnings: append([]string(nil), warnings...)}
	for _, pageNumber := range selectedPages {
		source, sourceErr := s.extractPageSource(ctx, instance, doc.Document, pageNumber, req, vision)
		if sourceErr != nil {
			return ExtractResult{}, sourceErr
		}
		sources = append(sources, source)
		if source.OCRUsed {
			result.OCRUsed = true
			result.OCRPages = append(result.OCRPages, pageNumber)
			result.OCREngine = source.OCRResult.Engine
			if source.OCRResult.Model != "" {
				result.OCRModels = appendUniqueString(result.OCRModels, source.OCRResult.Model)
			}
			result.Warnings = append(result.Warnings, source.OCRResult.Warnings...)
		}
		if source.VisionUsed {
			result.VisionUsed = true
			result.VisionPages = append(result.VisionPages, pageNumber)
			result.VisionEngine = source.VisionResult.Engine
			if source.VisionResult.Provider != "" {
				result.VisionProviders = appendUniqueString(result.VisionProviders, source.VisionResult.Provider)
			}
			if source.VisionResult.Model != "" {
				result.VisionModels = appendUniqueString(result.VisionModels, source.VisionResult.Model)
			}
		}
		result.Warnings = append(result.Warnings, source.Warnings...)
	}

	layoutCtx := buildPDFLayoutContext(sources)
	presentations := make([]pdfPresentation, 0, len(sources))
	for _, source := range sources {
		presentation := buildPDFPresentation(source, layoutCtx, req.IncludeHeadersFooters)
		presentations = append(presentations, presentation)
	}

	if req.IncludeOutline {
		result.Outline = buildPDFOutline(instance, doc.Document, selectedPages, presentations)
	}

	maxChars := clampMaxChars(req.MaxChars)
	remainingText := maxChars
	remainingRaw := maxChars
	remainingMarkdown := maxChars
	pageEntries := make([]PageText, 0, len(sources))
	markdownSections := make([]string, 0, len(sources))

	for idx, source := range sources {
		presentation := presentations[idx]
		prefix := resultTextPrefix(source.PageNumber, len(sources))

		textOutput := presentation.Text
		if prefix != "" && textOutput != "" {
			textOutput = prefix + textOutput
		}
		clippedText, textChars, textClipped := clipRunes(textOutput, remainingText)
		if clippedText != "" {
			if result.Text != "" {
				result.Text += "\n\n"
			}
			result.Text += clippedText
			remainingText -= textChars
		}

		rawOutput := source.RawText
		if prefix != "" && rawOutput != "" {
			rawOutput = prefix + rawOutput
		}
		clippedRaw, rawChars, rawClipped := clipRunes(rawOutput, remainingRaw)
		if clippedRaw != "" {
			if result.RawText != "" {
				result.RawText += "\n\n"
			}
			result.RawText += clippedRaw
			remainingRaw -= rawChars
		}

		if req.IncludeMarkdown && strings.TrimSpace(presentation.Markdown) != "" {
			pageMarkdown := strings.TrimSpace(presentation.Markdown)
			if len(sources) > 1 {
				pageMarkdown = fmt.Sprintf("[Page %d]\n\n%s", source.PageNumber, pageMarkdown)
			}
			clippedMarkdown, markdownChars, markdownClipped := clipRunes(pageMarkdown, remainingMarkdown)
			if clippedMarkdown != "" {
				markdownSections = append(markdownSections, clippedMarkdown)
				remainingMarkdown -= markdownChars
			}
			if markdownClipped {
				result.Truncated = true
			}
		}

		result.SelectedPages = append(result.SelectedPages, source.PageNumber)
		if req.IncludePages || req.IncludeLayout {
			pageEntry := PageText{
				Number:    source.PageNumber,
				Text:      presentation.Text,
				CharCount: utf8.RuneCountInString(presentation.Text),
				Source:    source.Source,
				Empty:     strings.TrimSpace(presentation.Text) == "",
			}
			if strings.TrimSpace(source.RawText) != "" && strings.TrimSpace(source.RawText) != strings.TrimSpace(presentation.Text) {
				pageEntry.RawText = source.RawText
			}
			if req.IncludeMarkdown {
				pageEntry.Markdown = presentation.Markdown
			}
			if req.IncludeLayout {
				pageEntry.Blocks = presentation.Blocks
				pageEntry.Tables = presentation.Tables
			}
			pageEntries = append(pageEntries, pageEntry)
		}

		if textClipped || rawClipped || remainingText <= 0 || remainingRaw <= 0 {
			result.Truncated = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("output truncated at %d characters", maxChars))
			break
		}
	}

	if req.IncludePages || req.IncludeLayout {
		result.Pages = pageEntries
	}
	if req.IncludeMarkdown && len(markdownSections) > 0 {
		result.Markdown = strings.Join(markdownSections, "\n\n---\n\n")
	}
	result.CharCount = utf8.RuneCountInString(result.Text)
	return result, nil
}

func extractPDFiumTOCProbeText(instance pdfium.Pdfium, document references.FPDF_DOCUMENT, pages int) string {
	if instance == nil || pages <= 0 {
		return ""
	}
	var buf strings.Builder
	for page := 1; page <= pages; page++ {
		resp, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{ByIndex: &requests.PageByIndex{Document: document, Index: page - 1}},
		})
		if err != nil || resp == nil || strings.TrimSpace(resp.Text) == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(canonicalizeExtractedPDFText(resp.Text))
	}
	return buf.String()
}

func (s *Service) extractPageSource(
	ctx context.Context,
	instance pdfium.Pdfium,
	document references.FPDF_DOCUMENT,
	pageNumber int,
	req ExtractRequest,
	vision VisionService,
) (pdfPageSource, error) {
	source := pdfPageSource{PageNumber: pageNumber, Source: "none"}
	size, err := instance.GetPageSize(&requests.GetPageSize{
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: document, Index: pageNumber - 1}},
	})
	if err == nil && size != nil {
		source.Geometry = pdfPageGeometry{Width: size.Width, Height: size.Height}
	}

	structured, structuredErr := extractStructuredPage(instance, document, pageNumber, source.Geometry)
	if structuredErr != nil {
		source.Warnings = append(source.Warnings, fmt.Sprintf("page %d structured extraction failed: %v", pageNumber, structuredErr))
	}
	if structured != nil && strings.TrimSpace(structured.Text) != "" {
		source.Structured = structured
		source.RawText = structured.RawText
		source.Text = structured.Text
		source.Source = "text"
	}

	plainText, plainErr := instance.GetPageText(&requests.GetPageText{
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: document, Index: pageNumber - 1}},
	})
	if plainErr != nil {
		source.Warnings = append(source.Warnings, fmt.Sprintf("page %d plain text extraction failed: %v", pageNumber, plainErr))
	} else if plainText != nil {
		rawPlain := canonicalizeExtractedPDFText(plainText.Text)
		normalizedPlain := normalizeText(plainText.Text)
		if source.Source == "none" && normalizedPlain != "" {
			source.RawText = rawPlain
			source.Text = normalizedPlain
			source.Source = "text"
		} else if strings.TrimSpace(source.RawText) == "" && rawPlain != "" {
			source.RawText = rawPlain
		}
	}

	if source.Source == "none" && !req.DisableOCR && s.ocr != nil {
		ocrResult, ocrErr := s.extractPageOCR(ctx, instance, document, pageNumber)
		if ocrErr != nil {
			source.Warnings = append(source.Warnings, fmt.Sprintf("page %d OCR failed: %v", pageNumber, ocrErr))
		} else {
			source.OCRResult = ocrResult
			source.Text = normalizeText(ocrResult.Text)
			source.RawText = effectiveRawPDFText(ocrResult.Text, source.Text)
			if source.Text != "" {
				source.Source = "ocr"
				source.OCRUsed = true
			}
		}
	}

	shouldTryVision := vision != nil && !req.DisableVision && (source.Source == "none" || (source.Source == "ocr" && textLooksWeak(source.Text)))
	if shouldTryVision {
		visionResult, visionErr := s.extractPageVision(ctx, instance, document, pageNumber, vision)
		if visionErr != nil {
			source.Warnings = append(source.Warnings, fmt.Sprintf("page %d vision fallback failed: %v", pageNumber, visionErr))
		} else if preferVisionText(source.Text, visionResult.Text, source.Source) {
			if source.Source == "ocr" && source.Text != "" {
				source.Warnings = append(source.Warnings, fmt.Sprintf("page %d used vision fallback because OCR looked low-confidence", pageNumber))
			}
			source.VisionResult = visionResult
			source.Text = normalizeText(visionResult.Text)
			source.RawText = effectiveRawPDFText(visionResult.Text, source.Text)
			if source.Text != "" {
				source.Source = "vision"
				source.VisionUsed = true
				source.Structured = nil
			}
		}
	}

	if strings.TrimSpace(source.RawText) == "" {
		source.RawText = effectiveRawPDFText(source.RawText, source.Text)
	}
	if source.Source == "none" {
		source.Warnings = append(source.Warnings, fmt.Sprintf("page %d has no extractable text", pageNumber))
	}
	return source, nil
}

func extractStructuredPage(instance pdfium.Pdfium, document references.FPDF_DOCUMENT, pageNumber int, geometry pdfPageGeometry) (*pdfStructuredPage, error) {
	resp, err := instance.GetPageTextStructured(&requests.GetPageTextStructured{
		Page:                   requests.Page{ByIndex: &requests.PageByIndex{Document: document, Index: pageNumber - 1}},
		Mode:                   requests.GetPageTextStructuredModeRects,
		CollectFontInformation: true,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Rects) == 0 {
		return &pdfStructuredPage{PageNumber: pageNumber, Geometry: geometry}, nil
	}
	rects := make([]pdfStructuredRect, 0, len(resp.Rects))
	for _, item := range resp.Rects {
		if item == nil {
			continue
		}
		text := canonicalizeExtractedPDFText(item.Text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		rect := pdfStructuredRect{
			Text: text,
			BBox: bboxFromPosition(item.PointPosition),
		}
		if item.FontInformation != nil {
			rect.FontSize = item.FontInformation.Size
			rect.FontWeight = item.FontInformation.Weight
			rect.FontName = strings.TrimSpace(item.FontInformation.Name)
		}
		rects = append(rects, rect)
	}
	lines := buildStructuredLines(rects)
	page := &pdfStructuredPage{
		PageNumber: pageNumber,
		Geometry:   geometry,
		Lines:      lines,
	}
	page.RawText = buildStructuredPageRawText(lines)
	page.Text = normalizeText(page.RawText)
	return page, nil
}

func buildPDFOutline(
	instance pdfium.Pdfium,
	document references.FPDF_DOCUMENT,
	selectedPages []int,
	pages []pdfPresentation,
) []OutlineEntry {
	selected := make(map[int]struct{}, len(selectedPages))
	for _, page := range selectedPages {
		selected[page] = struct{}{}
	}
	bookmarks, err := instance.GetBookmarks(&requests.GetBookmarks{Document: document})
	if err == nil && bookmarks != nil && len(bookmarks.Bookmarks) > 0 {
		outline := flattenBookmarkOutline(bookmarks.Bookmarks, 1, selected)
		if len(outline) > 0 {
			return outline
		}
	}
	outline := make([]OutlineEntry, 0, 8)
	for _, page := range pages {
		outline = append(outline, page.HeadingText...)
	}
	return outline
}

func flattenBookmarkOutline(items []responses.GetBookmarksBookmark, level int, selected map[int]struct{}) []OutlineEntry {
	out := make([]OutlineEntry, 0, len(items))
	for _, item := range items {
		pageNumber := bookmarkPageNumber(item)
		if len(selected) == 0 || pageNumber == 0 {
			out = appendBookmarkOutlineEntry(&out, item.Title, level, pageNumber)
		} else if _, ok := selected[pageNumber]; ok {
			out = appendBookmarkOutlineEntry(&out, item.Title, level, pageNumber)
		}
		if len(item.Children) > 0 {
			out = append(out, flattenBookmarkOutline(item.Children, level+1, selected)...)
		}
	}
	return out
}

func appendBookmarkOutlineEntry(out *[]OutlineEntry, title string, level, pageNumber int) []OutlineEntry {
	title = strings.TrimSpace(title)
	if title == "" {
		return *out
	}
	*out = append(*out, OutlineEntry{Title: title, Level: max(1, level), PageNumber: pageNumber})
	return *out
}

func bookmarkPageNumber(item responses.GetBookmarksBookmark) int {
	switch {
	case item.DestInfo != nil && item.DestInfo.PageIndex >= 0:
		return item.DestInfo.PageIndex + 1
	case item.ActionInfo != nil && item.ActionInfo.DestInfo != nil && item.ActionInfo.DestInfo.PageIndex >= 0:
		return item.ActionInfo.DestInfo.PageIndex + 1
	default:
		return 0
	}
}

func bboxFromPosition(pos responses.CharPosition) BBox {
	left := math.Min(pos.Left, pos.Right)
	right := math.Max(pos.Left, pos.Right)
	top := math.Min(pos.Top, pos.Bottom)
	bottom := math.Max(pos.Top, pos.Bottom)
	return BBox{Left: left, Top: top, Right: right, Bottom: bottom}
}
