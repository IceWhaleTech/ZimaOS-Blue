//go:build darwin

package pdf

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

func defaultPDFRuntimePoolFactory() pdfRuntimePoolFactory {
	return nil
}

// Info reads document-level information for a PDF file on darwin using PDFKit.
func (s *Service) Info(ctx context.Context, path string) (DocumentInfo, error) {
	if err := ctx.Err(); err != nil {
		return DocumentInfo{}, err
	}
	resolvedPath, stat, err := resolvePath(path)
	if err != nil {
		return DocumentInfo{}, err
	}
	info, ok, err := tryNativePDFInfo(ctx, resolvedPath, stat)
	if ok && err == nil {
		return info, nil
	}
	if ok && err != nil {
		return DocumentInfo{}, err
	}
	return DocumentInfo{}, fmt.Errorf("macOS PDFKit native extraction unavailable")
}

// Extract reads text content from a PDF file on darwin using PDFKit.
func (s *Service) Extract(ctx context.Context, req ExtractRequest) (ExtractResult, error) {
	if err := ctx.Err(); err != nil {
		return ExtractResult{}, err
	}
	resolvedPath, stat, err := resolvePath(req.Path)
	if err != nil {
		return ExtractResult{}, err
	}
	internalReq := req
	internalReq.IncludePages = true
	result, ok, err := tryNativePDFExtract(ctx, internalReq, resolvedPath, stat)
	if ok && err == nil {
		result, err = s.applyDarwinFallbacks(ctx, resolvedPath, internalReq, result)
		if err != nil {
			return ExtractResult{}, err
		}
		if !req.IncludePages && !req.IncludeLayout {
			result.Pages = nil
		}
		return result, nil
	}
	if ok && err != nil {
		return ExtractResult{}, err
	}
	return ExtractResult{}, fmt.Errorf("macOS PDFKit native extraction unavailable")
}

func (s *Service) applyDarwinFallbacks(ctx context.Context, path string, req ExtractRequest, result ExtractResult) (ExtractResult, error) {
	if len(result.Pages) == 0 {
		return result, nil
	}

	vision := s.visionService()
	updated := false
	for idx := range result.Pages {
		page := &result.Pages[idx]
		if strings.TrimSpace(page.Text) != "" {
			continue
		}
		if req.DisableOCR && req.DisableVision {
			continue
		}

		renderedPNG, ok, err := renderDarwinPDFKitPagePNG(ctx, path, page.Number, ocrRenderDPI)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("page %d native render failed: %v", page.Number, err))
			continue
		}
		if !ok || len(renderedPNG) == 0 {
			continue
		}

		bestText := ""
		bestRaw := ""
		bestSource := page.Source

		if !req.DisableOCR && s.ocr != nil {
			ocrResult, ocrErr := s.ocr.Extract(ctx, renderedPNG)
			if ocrErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("page %d OCR failed: %v", page.Number, ocrErr))
			} else {
				text := normalizeText(ocrResult.Text)
				raw := effectiveRawPDFText(ocrResult.Text, text)
				if text != "" {
					bestText = text
					bestRaw = raw
					bestSource = "ocr"
					page.Source = bestSource
					page.Empty = false
					result.OCRUsed = true
					result.OCRPages = appendUniqueInt(result.OCRPages, page.Number)
					result.OCREngine = ocrResult.Engine
					if ocrResult.Model != "" {
						result.OCRModels = appendUniqueString(result.OCRModels, ocrResult.Model)
					}
					result.Warnings = append(result.Warnings, ocrResult.Warnings...)
				}
			}
		}

		if vision != nil && !req.DisableVision {
			visionResult, visionErr := vision.Extract(ctx, renderedPNG)
			if visionErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("page %d vision fallback failed: %v", page.Number, visionErr))
			} else if preferVisionText(bestText, visionResult.Text, bestSource) {
				text := normalizeText(visionResult.Text)
				raw := effectiveRawPDFText(visionResult.Text, text)
				if text != "" {
					bestText = text
					bestRaw = raw
					bestSource = "vision"
					result.VisionUsed = true
					result.VisionPages = appendUniqueInt(result.VisionPages, page.Number)
					result.VisionEngine = visionResult.Engine
					if visionResult.Provider != "" {
						result.VisionProviders = appendUniqueString(result.VisionProviders, visionResult.Provider)
					}
					if visionResult.Model != "" {
						result.VisionModels = appendUniqueString(result.VisionModels, visionResult.Model)
					}
				}
			}
		}

		if bestText == "" {
			continue
		}

		updated = true
		presentation := buildSyntheticPDFPresentation(pdfPageSource{
			PageNumber: page.Number,
			RawText:    bestRaw,
			Text:       bestText,
			Source:     bestSource,
		})
		page.Text = presentation.Text
		page.RawText = bestRaw
		page.CharCount = utf8.RuneCountInString(page.Text)
		page.Source = bestSource
		page.Empty = false
		if req.IncludeMarkdown {
			page.Markdown = presentation.Markdown
		}
		if req.IncludeLayout {
			page.Blocks = presentation.Blocks
			page.Tables = presentation.Tables
		}
	}

	if updated {
		rebuildDarwinExtractResult(req, &result)
	}
	return result, nil
}

func rebuildDarwinExtractResult(req ExtractRequest, result *ExtractResult) {
	if result == nil {
		return
	}
	result.Text = ""
	result.RawText = ""
	result.Markdown = ""
	result.CharCount = 0
	result.Truncated = false

	maxChars := clampMaxChars(req.MaxChars)
	remainingText := maxChars
	remainingRaw := maxChars
	remainingMarkdown := maxChars
	markdownSections := make([]string, 0, len(result.Pages))
	outline := make([]OutlineEntry, 0, 8)

	for _, page := range result.Pages {
		prefix := resultTextPrefix(page.Number, len(result.Pages))

		textOutput := page.Text
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

		rawOutput := effectiveRawPDFText(page.RawText, page.Text)
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

		if req.IncludeMarkdown && strings.TrimSpace(page.Markdown) != "" {
			pageMarkdown := strings.TrimSpace(page.Markdown)
			if len(result.Pages) > 1 {
				pageMarkdown = fmt.Sprintf("[Page %d]\n\n%s", page.Number, pageMarkdown)
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

		if req.IncludeOutline && len(page.Blocks) > 0 {
			outline = append(outline, darwinOutlineEntriesFromBlocks(page.Number, page.Blocks)...)
		}

		if textClipped || rawClipped || remainingText <= 0 || remainingRaw <= 0 {
			result.Truncated = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("output truncated at %d characters", maxChars))
			break
		}
	}

	if req.IncludeMarkdown && len(markdownSections) > 0 {
		result.Markdown = strings.Join(markdownSections, "\n\n---\n\n")
	}
	if req.IncludeOutline && len(outline) > 0 {
		result.Outline = outline
	}
	result.CharCount = utf8.RuneCountInString(result.Text)
}

func darwinOutlineEntriesFromBlocks(pageNumber int, blocks []PageBlock) []OutlineEntry {
	out := make([]OutlineEntry, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind == "heading" && strings.TrimSpace(block.Text) != "" {
			level := block.HeadingLevel
			if level <= 0 {
				level = 2
			}
			out = append(out, OutlineEntry{
				Title:      strings.TrimSpace(block.Text),
				Level:      level,
				PageNumber: pageNumber,
			})
		}
	}
	return out
}

func appendUniqueInt(values []int, value int) []int {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
