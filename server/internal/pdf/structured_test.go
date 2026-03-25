package pdf

import (
	"strings"
	"testing"
)

func TestSortStructuredLinesReadingOrder_MultiColumnLeftBeforeRight(t *testing.T) {
	lines := []*pdfStructuredLine{
		makeTestStructuredLine("Executive Summary", BBox{Left: 40, Top: 40, Right: 560, Bottom: 60}, 22, 700),
		makeTestStructuredLine("Left column first", BBox{Left: 40, Top: 100, Right: 220, Bottom: 118}, 12, 400),
		makeTestStructuredLine("Right column first", BBox{Left: 320, Top: 100, Right: 500, Bottom: 118}, 12, 400),
		makeTestStructuredLine("Left column second", BBox{Left: 40, Top: 126, Right: 220, Bottom: 144}, 12, 400),
		makeTestStructuredLine("Right column second", BBox{Left: 320, Top: 126, Right: 500, Bottom: 144}, 12, 400),
	}

	ordered := sortStructuredLinesReadingOrder(lines)
	got := make([]string, 0, len(ordered))
	for _, line := range ordered {
		got = append(got, line.Text)
	}
	want := []string{
		"Executive Summary",
		"Left column first",
		"Left column second",
		"Right column first",
		"Right column second",
	}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Fatalf("reading order = %v, want %v", got, want)
	}
}

func TestBuildPDFPresentation_DetectsHeadingListTableAndSuppressesRepeatedHeaderFooter(t *testing.T) {
	headerPage1 := makeTestStructuredLine("Quarterly Review 2026", BBox{Left: 60, Top: 18, Right: 260, Bottom: 32}, 11, 500)
	headerPage2 := makeTestStructuredLine("Quarterly Review 2027", BBox{Left: 60, Top: 18, Right: 260, Bottom: 32}, 11, 500)
	footerPage1 := makeTestStructuredLine("Page 1", BBox{Left: 270, Top: 770, Right: 320, Bottom: 784}, 10, 400)
	footerPage2 := makeTestStructuredLine("Page 2", BBox{Left: 270, Top: 770, Right: 320, Bottom: 784}, 10, 400)
	heading := makeTestStructuredLine("Executive Summary", BBox{Left: 60, Top: 86, Right: 250, Bottom: 108}, 20, 700)
	list1 := makeTestStructuredLine("• First finding", BBox{Left: 72, Top: 130, Right: 250, Bottom: 146}, 12, 400)
	list2 := makeTestStructuredLine("2) Second finding", BBox{Left: 72, Top: 152, Right: 260, Bottom: 168}, 12, 400)
	tableHeader := makeTestStructuredTableLine(
		[]string{"Name", "Count"},
		[]BBox{
			{Left: 72, Top: 210, Right: 130, Bottom: 226},
			{Left: 220, Top: 210, Right: 270, Bottom: 226},
		},
		12,
		600,
	)
	tableRow := makeTestStructuredTableLine(
		[]string{"AI & LLMs", "287"},
		[]BBox{
			{Left: 72, Top: 232, Right: 150, Bottom: 248},
			{Left: 220, Top: 232, Right: 252, Bottom: 248},
		},
		12,
		400,
	)
	page2Body := makeTestStructuredLine("Follow-up notes for the next page.", BBox{Left: 72, Top: 120, Right: 320, Bottom: 138}, 12, 400)

	sources := []pdfPageSource{
		{
			PageNumber: 1,
			Geometry:   pdfPageGeometry{Width: 612, Height: 792},
			Structured: &pdfStructuredPage{
				PageNumber: 1,
				Lines:      []*pdfStructuredLine{headerPage1, heading, list1, list2, tableHeader, tableRow, footerPage1},
			},
			Source: "text",
		},
		{
			PageNumber: 2,
			Geometry:   pdfPageGeometry{Width: 612, Height: 792},
			Structured: &pdfStructuredPage{
				PageNumber: 2,
				Lines:      []*pdfStructuredLine{headerPage2, page2Body, footerPage2},
			},
			Source: "text",
		},
	}

	layoutCtx := buildPDFLayoutContext(sources)
	presentation := buildPDFPresentation(sources[0], layoutCtx, false)

	if !strings.Contains(presentation.Markdown, "# Executive Summary") {
		t.Fatalf("expected heading markdown, got=%q", presentation.Markdown)
	}
	if !strings.Contains(presentation.Markdown, "- First finding") || !strings.Contains(presentation.Markdown, "2. Second finding") {
		t.Fatalf("expected list markdown, got=%q", presentation.Markdown)
	}
	if !strings.Contains(presentation.Markdown, "| Name | Count |") || !strings.Contains(presentation.Markdown, "| AI & LLMs | 287 |") {
		t.Fatalf("expected table markdown, got=%q", presentation.Markdown)
	}
	if strings.Contains(presentation.Markdown, "Quarterly Review") {
		t.Fatalf("expected repeated header to be suppressed, got=%q", presentation.Markdown)
	}
	if strings.Contains(presentation.Markdown, "Page 1") {
		t.Fatalf("expected repeated footer to be suppressed, got=%q", presentation.Markdown)
	}
	if len(presentation.Tables) != 1 {
		t.Fatalf("table count = %d, want 1", len(presentation.Tables))
	}
	if len(presentation.HeadingText) != 1 || presentation.HeadingText[0].Title != "Executive Summary" {
		t.Fatalf("heading outline = %#v", presentation.HeadingText)
	}
}

func TestBuildSyntheticPDFPresentation_ProvidesStableMarkdownForOCRFallback(t *testing.T) {
	source := pdfPageSource{
		PageNumber: 3,
		Source:     "ocr",
		Text: strings.Join([]string{
			"Executive Summary",
			"• First finding",
			"2) Second finding",
			"Closing paragraph.",
		}, "\n"),
	}

	presentation := buildSyntheticPDFPresentation(source)
	if strings.TrimSpace(presentation.Markdown) == "" {
		t.Fatal("expected synthetic markdown")
	}
	if len(presentation.Blocks) == 0 {
		t.Fatal("expected synthetic blocks")
	}
	if !strings.Contains(presentation.Markdown, "Executive Summary") || !strings.Contains(presentation.Markdown, "- First finding") {
		t.Fatalf("unexpected synthetic markdown: %q", presentation.Markdown)
	}
}

func makeTestStructuredLine(text string, bbox BBox, fontSize float64, fontWeight int) *pdfStructuredLine {
	normalized := normalizePDFLine(text)
	if normalized == "" {
		normalized = text
	}
	return &pdfStructuredLine{
		Text:       normalized,
		RawText:    text,
		BBox:       bbox,
		FontSize:   fontSize,
		FontWeight: fontWeight,
		Fragments: []pdfStructuredRect{{
			Text:       text,
			BBox:       bbox,
			FontSize:   fontSize,
			FontWeight: fontWeight,
		}},
	}
}

func makeTestStructuredTableLine(texts []string, boxes []BBox, fontSize float64, fontWeight int) *pdfStructuredLine {
	fragments := make([]pdfStructuredRect, 0, len(texts))
	rawParts := make([]string, 0, len(texts))
	var lineBox BBox
	for idx, text := range texts {
		rawParts = append(rawParts, text)
		fragment := pdfStructuredRect{
			Text:       text,
			BBox:       boxes[idx],
			FontSize:   fontSize,
			FontWeight: fontWeight,
		}
		fragments = append(fragments, fragment)
		lineBox = lineBox.union(boxes[idx])
	}
	rawText := strings.Join(rawParts, "     ")
	normalized := normalizePDFLine(rawText)
	if normalized == "" {
		normalized = rawText
	}
	return &pdfStructuredLine{
		Text:       normalized,
		RawText:    rawText,
		BBox:       lineBox,
		FontSize:   fontSize,
		FontWeight: fontWeight,
		Fragments:  fragments,
	}
}
