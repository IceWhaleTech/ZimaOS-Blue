package pdf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"go.uber.org/zap"
)

type fakeCreateTextMeasurer struct {
	cellMargin   float64
	defaultWidth float64
	runeWidths   map[rune]float64
}

type fakeCreateRenderer struct {
	data      []byte
	pageCount int
	lineCount int
	err       error
	called    bool
	gotLines  []createStyledLine
}

func (m fakeCreateTextMeasurer) CellMargin() float64 {
	return m.cellMargin
}

func (m fakeCreateTextMeasurer) MeasureText(text string) float64 {
	total := 0.0
	for _, r := range text {
		if width, ok := m.runeWidths[r]; ok {
			total += width
			continue
		}
		total += m.defaultWidth
	}
	return total
}

func (r *fakeCreateRenderer) render(lines []createStyledLine, _ createFontPlan) ([]byte, int, int, error) {
	r.called = true
	r.gotLines = append([]createStyledLine(nil), lines...)
	return r.data, r.pageCount, r.lineCount, r.err
}

func TestCreateDocumentPreservesChineseWhenUnicodeFontAvailable(t *testing.T) {
	if _, err := resolveCreateUnicodeFont(collectCreateRequiredRunes([]string{"中文标题", "中文摘要"})); err != nil {
		t.Skipf("no unicode font with Chinese glyph coverage available: %v", err)
	}

	_, info, err := CreateDocument(CreateRequest{
		Title:    "中文标题",
		Subtitle: "版本 1",
		Summary:  "中文摘要",
	})
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}

	for _, warning := range info.Warnings {
		if strings.Contains(warning, "replaced unsupported characters") {
			t.Fatalf("warnings = %#v, want Chinese text preserved without replacement warning", info.Warnings)
		}
	}
}

func TestCreateRequestTextValuesIncludeShapedArabicFormsForFontResolution(t *testing.T) {
	values := createRequestTextValues(CreateRequest{
		Title: "المصفوفة (Multidimentional Array) هي",
	})

	for _, value := range values {
		if containsArabicPresentationForm(value) {
			return
		}
	}

	t.Fatalf("createRequestTextValues() = %#v, want Arabic presentation-form text included for font resolution", values)
}

func TestCreateSplitTextWithMeasurerWrapsOnSpaces(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1}

	got := createSplitTextWithMeasurer(measurer, "alpha beta gamma", 6)
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("len(createSplitTextWithMeasurer()) = %d, want %d (%#v)", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("createSplitTextWithMeasurer()[%d] = %q, want %q (%#v)", idx, got[idx], want[idx], got)
		}
	}
}

func TestCreateSplitTextWithMeasurerWrapsOnCJKBoundaries(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1}

	got := createSplitTextWithMeasurer(measurer, "你好世界", 2)
	want := []string{"你好", "世界"}
	if len(got) != len(want) {
		t.Fatalf("len(createSplitTextWithMeasurer()) = %d, want %d (%#v)", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("createSplitTextWithMeasurer()[%d] = %q, want %q (%#v)", idx, got[idx], want[idx], got)
		}
	}
}

func TestCreateSplitTextWithMeasurerRespectsCellMargin(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1, cellMargin: 1}

	got := createSplitTextWithMeasurer(measurer, "abcd", 5)
	want := []string{"abc", "d"}
	if len(got) != len(want) {
		t.Fatalf("len(createSplitTextWithMeasurer()) = %d, want %d (%#v)", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("createSplitTextWithMeasurer()[%d] = %q, want %q (%#v)", idx, got[idx], want[idx], got)
		}
	}
}

func TestCreateDocumentUsesRendererFactory(t *testing.T) {
	fake := &fakeCreateRenderer{
		data:      []byte("%PDF-test"),
		pageCount: 2,
		lineCount: 4,
	}

	previousFactory := createRendererFactory
	createRendererFactory = func() createRenderer {
		return fake
	}
	defer func() {
		createRendererFactory = previousFactory
	}()

	data, info, err := CreateDocument(CreateRequest{
		Title:   "Native PDF",
		Summary: "Renderer seam test",
	})
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}
	if !fake.called {
		t.Fatal("expected CreateDocument to invoke renderer factory output")
	}
	if len(fake.gotLines) == 0 {
		t.Fatal("expected renderer to receive styled lines")
	}
	if got := string(data); got != "%PDF-test" {
		t.Fatalf("CreateDocument() data = %q, want %q", got, "%PDF-test")
	}
	if info.PageCount != 2 || info.LineCount != 4 {
		t.Fatalf("CreateDocument() info = %#v, want page_count=2 line_count=4", info)
	}
}

func TestRenderCreateWithFallbackUsesPrimaryWhenAvailable(t *testing.T) {
	fallback := &fakeCreateRenderer{
		data:      []byte("%PDF-fallback"),
		pageCount: 7,
		lineCount: 9,
	}
	lines := []createStyledLine{{Text: "primary"}}

	data, pageCount, lineCount, err := renderCreateWithFallback(func(gotLines []createStyledLine, _ createFontPlan) ([]byte, int, int, error) {
		if len(gotLines) != 1 || gotLines[0].Text != "primary" {
			t.Fatalf("primary got lines = %#v", gotLines)
		}
		return []byte("%PDF-primary"), 2, 3, nil
	}, fallback, lines, createFontPlan{})
	if err != nil {
		t.Fatalf("renderCreateWithFallback() error = %v", err)
	}
	if got := string(data); got != "%PDF-primary" {
		t.Fatalf("data = %q, want %q", got, "%PDF-primary")
	}
	if pageCount != 2 || lineCount != 3 {
		t.Fatalf("pageCount/lineCount = %d/%d, want 2/3", pageCount, lineCount)
	}
	if fallback.called {
		t.Fatal("expected fallback renderer to stay unused when primary succeeds")
	}
}

func TestRenderCreateWithFallbackFallsBackOnUnavailable(t *testing.T) {
	fallback := &fakeCreateRenderer{
		data:      []byte("%PDF-fallback"),
		pageCount: 7,
		lineCount: 9,
	}

	data, pageCount, lineCount, err := renderCreateWithFallback(func(_ []createStyledLine, _ createFontPlan) ([]byte, int, int, error) {
		return nil, 0, 0, errCreateRendererUnavailable
	}, fallback, []createStyledLine{{Text: "fallback"}}, createFontPlan{})
	if err != nil {
		t.Fatalf("renderCreateWithFallback() error = %v", err)
	}
	if !fallback.called {
		t.Fatal("expected fallback renderer to be called when primary is unavailable")
	}
	if got := string(data); got != "%PDF-fallback" {
		t.Fatalf("data = %q, want %q", got, "%PDF-fallback")
	}
	if pageCount != 7 || lineCount != 9 {
		t.Fatalf("pageCount/lineCount = %d/%d, want 7/9", pageCount, lineCount)
	}
}

func TestRenderCreateWithFallbackPropagatesPrimaryError(t *testing.T) {
	fallback := &fakeCreateRenderer{
		data:      []byte("%PDF-fallback"),
		pageCount: 7,
		lineCount: 9,
	}

	_, _, _, err := renderCreateWithFallback(func(_ []createStyledLine, _ createFontPlan) ([]byte, int, int, error) {
		return nil, 0, 0, context.DeadlineExceeded
	}, fallback, []createStyledLine{{Text: "error"}}, createFontPlan{})
	if err == nil {
		t.Fatal("expected primary error to be returned")
	}
	if !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("error = %v, want deadline context error", err)
	}
	if fallback.called {
		t.Fatal("expected fallback renderer to stay unused on non-availability errors")
	}
}

func TestRenderCreateWithOptionalFallbackUsesPrimaryWhenFallbackNil(t *testing.T) {
	data, pageCount, lineCount, err := renderCreateWithOptionalFallback(func(_ []createStyledLine, _ createFontPlan) ([]byte, int, int, error) {
		return []byte("%PDF-native"), 3, 5, nil
	}, nil, []createStyledLine{{Text: "native"}}, createFontPlan{})
	if err != nil {
		t.Fatalf("renderCreateWithOptionalFallback() error = %v", err)
	}
	if got := string(data); got != "%PDF-native" {
		t.Fatalf("data = %q, want %q", got, "%PDF-native")
	}
	if pageCount != 3 || lineCount != 5 {
		t.Fatalf("pageCount/lineCount = %d/%d, want 3/5", pageCount, lineCount)
	}
}

func TestRenderCreateWithOptionalFallbackReturnsUnavailableWhenNoRendererExists(t *testing.T) {
	_, _, _, err := renderCreateWithOptionalFallback(nil, nil, []createStyledLine{{Text: "missing"}}, createFontPlan{})
	if !errors.Is(err, errCreateRendererUnavailable) {
		t.Fatalf("renderCreateWithOptionalFallback() error = %v, want errCreateRendererUnavailable", err)
	}
}

func TestDefaultDarwinCreateFallbackRendererReturnsNil(t *testing.T) {
	if got := defaultDarwinCreateFallbackRenderer(); got != nil {
		t.Fatalf("defaultDarwinCreateFallbackRenderer() = %#v, want nil", got)
	}
}

func TestCreateAlignedTextX(t *testing.T) {
	tests := []struct {
		name         string
		left         float64
		available    float64
		textWidth    float64
		align        string
		wantAlignedX float64
	}{
		{
			name:         "LeftAlignedUsesLeftEdge",
			left:         54,
			available:    160,
			textWidth:    48,
			align:        "L",
			wantAlignedX: 54,
		},
		{
			name:         "RightAlignedUsesTrailingEdge",
			left:         54,
			available:    160,
			textWidth:    48,
			align:        "R",
			wantAlignedX: 166,
		},
		{
			name:         "RightAlignedClampsWhenTextOverflows",
			left:         54,
			available:    32,
			textWidth:    48,
			align:        "R",
			wantAlignedX: 54,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := createAlignedTextX(tc.left, tc.available, tc.textWidth, tc.align); got != tc.wantAlignedX {
				t.Fatalf("createAlignedTextX(%v, %v, %v, %q) = %v, want %v", tc.left, tc.available, tc.textWidth, tc.align, got, tc.wantAlignedX)
			}
		})
	}
}

func TestCreateBottomOriginY(t *testing.T) {
	if got := createBottomOriginY(60, 18); got != 714 {
		t.Fatalf("createBottomOriginY(60, 18) = %v, want %v", got, 714.0)
	}
	if got := createBottomOriginY(createPageHeight-24, 40); got != 0 {
		t.Fatalf("createBottomOriginY(pageHeight-24, 40) = %v, want 0", got)
	}
}

func TestPrepareCreateTableRowWrapsCellsAndComputesHeight(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1}

	row := prepareCreateTableRow(
		measurer,
		[]createStyledTableCell{
			{Text: "alpha beta", Align: "R"},
			{Text: "ok"},
		},
		[]float64{20, 30},
		10,
	)

	if len(row.InnerWidths) != 2 || row.InnerWidths[0] != 20 || row.InnerWidths[1] != 14 {
		t.Fatalf("InnerWidths = %#v, want [20 14]", row.InnerWidths)
	}
	if len(row.Wrapped) != 2 {
		t.Fatalf("Wrapped len = %d, want 2", len(row.Wrapped))
	}
	wantFirst := []string{"alpha beta"}
	if len(row.Wrapped[0]) != len(wantFirst) {
		t.Fatalf("Wrapped[0] = %#v, want %#v", row.Wrapped[0], wantFirst)
	}
	for idx := range wantFirst {
		if row.Wrapped[0][idx] != wantFirst[idx] {
			t.Fatalf("Wrapped[0][%d] = %q, want %q (%#v)", idx, row.Wrapped[0][idx], wantFirst[idx], row.Wrapped[0])
		}
	}
	if row.MaxLines != 1 {
		t.Fatalf("MaxLines = %d, want 1", row.MaxLines)
	}
	if row.RowHeight != 22 {
		t.Fatalf("RowHeight = %v, want 22", row.RowHeight)
	}
}

func TestPrepareCreateTableRowDefaultsBlankAlignmentToLeft(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1}

	row := prepareCreateTableRow(
		measurer,
		[]createStyledTableCell{{Text: "value"}},
		[]float64{40},
		10,
	)

	if len(row.Aligns) != 1 || row.Aligns[0] != "L" {
		t.Fatalf("Aligns = %#v, want [\"L\"]", row.Aligns)
	}
	if len(row.Wrapped) != 1 || len(row.Wrapped[0]) != 1 || row.Wrapped[0][0] != "value" {
		t.Fatalf("Wrapped = %#v, want [[\"value\"]]", row.Wrapped)
	}
}

func TestPrepareCreateTableRowFallsBackToFullWidthWhenPaddingWouldCollapseCell(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1}

	row := prepareCreateTableRow(
		measurer,
		[]createStyledTableCell{{Text: "1234567890"}},
		[]float64{10},
		10,
	)

	if len(row.InnerWidths) != 1 || row.InnerWidths[0] != 10 {
		t.Fatalf("InnerWidths = %#v, want [10]", row.InnerWidths)
	}
	if len(row.Wrapped[0]) != 1 || row.Wrapped[0][0] != "1234567890" {
		t.Fatalf("Wrapped = %#v, want unwrapped full-width cell", row.Wrapped)
	}
}

func TestPrepareCreateListItemDefaultsMarkerAndAlignment(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1}

	item := prepareCreateListItem(measurer, createStyledListItem{
		Text: "value",
	}, 80)

	if item.Marker != "-" {
		t.Fatalf("Marker = %q, want %q", item.Marker, "-")
	}
	if item.Align != "L" {
		t.Fatalf("Align = %q, want %q", item.Align, "L")
	}
	if item.MarkerWidth != 20 {
		t.Fatalf("MarkerWidth = %v, want %v", item.MarkerWidth, 20.0)
	}
	if item.MarkerTextWidth != 12 {
		t.Fatalf("MarkerTextWidth = %v, want %v", item.MarkerTextWidth, 12.0)
	}
	if item.ContentWidth != 60 {
		t.Fatalf("ContentWidth = %v, want %v", item.ContentWidth, 60.0)
	}
	if len(item.Segments) != 1 || item.Segments[0] != "value" {
		t.Fatalf("Segments = %#v, want [\"value\"]", item.Segments)
	}
}

func TestPrepareCreateListItemWrapsUsingContentWidth(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1}

	item := prepareCreateListItem(measurer, createStyledListItem{
		Marker: "1.",
		Text:   "alpha beta gamma",
	}, 30)

	if item.Marker != "1." {
		t.Fatalf("Marker = %q, want %q", item.Marker, "1.")
	}
	if item.MarkerWidth != 20 {
		t.Fatalf("MarkerWidth = %v, want %v", item.MarkerWidth, 20.0)
	}
	if item.ContentWidth != 10 {
		t.Fatalf("ContentWidth = %v, want %v", item.ContentWidth, 10.0)
	}
	want := []string{"alpha beta", "gamma"}
	if len(item.Segments) != len(want) {
		t.Fatalf("Segments = %#v, want %#v", item.Segments, want)
	}
	for idx := range want {
		if item.Segments[idx] != want[idx] {
			t.Fatalf("Segments[%d] = %q, want %q (%#v)", idx, item.Segments[idx], want[idx], item.Segments)
		}
	}
}

func TestPrepareCreateListItemPreservesWiderMarkerMeasurement(t *testing.T) {
	measurer := fakeCreateTextMeasurer{defaultWidth: 1}

	item := prepareCreateListItem(measurer, createStyledListItem{
		Marker: "123456789012345",
		Text:   "owner",
		Align:  "R",
	}, 100)

	if item.Align != "R" {
		t.Fatalf("Align = %q, want %q", item.Align, "R")
	}
	if item.MarkerWidth != 23 {
		t.Fatalf("MarkerWidth = %v, want %v", item.MarkerWidth, 23.0)
	}
	if item.MarkerTextWidth != 15 {
		t.Fatalf("MarkerTextWidth = %v, want %v", item.MarkerTextWidth, 15.0)
	}
	if item.ContentWidth != 77 {
		t.Fatalf("ContentWidth = %v, want %v", item.ContentWidth, 77.0)
	}
}

func TestCreateShapeArabicVisual(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "SingleWord",
			text: "بالعربي",
			want: "ﻲﺑﺮﻌﻟﺎﺑ",
		},
		{
			name: "MixedText",
			text: "المصفوفة (Multidimentional Array) هي",
			want: "ﻲﻫ (Multidimentional Array) ﺔﻓﻮﻔﺼﻤﻟا",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := createShapeArabicVisual(tc.text)
			if got != tc.want {
				t.Fatalf("createShapeArabicVisual(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestBuildCreateStyledLinesRightAlignsArabicWhenGlyphsAreWritable(t *testing.T) {
	lines, _ := buildCreateStyledLines(CreateRequest{
		Title: "مرحبا بالعالم",
	}, testCreateUnicodeGraphicPlan())

	if len(lines) == 0 {
		t.Fatalf("buildCreateStyledLines() returned no lines")
	}
	if lines[0].Align != "R" {
		t.Fatalf("lines[0].Align = %q, want %q", lines[0].Align, "R")
	}
}

func containsArabicPresentationForm(text string) bool {
	for _, r := range text {
		if r >= 0xFB50 && r <= 0xFEFF {
			return true
		}
	}
	return false
}

func TestCreateDocumentPreservesScriptGlyphsWhenUnicodeFontAvailable(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "Chinese", text: "中文标题"},
		{name: "Japanese", text: "日本語の要約"},
		{name: "Korean", text: "한국어 보고서"},
		{name: "Arabic", text: "مرحبا بالعالم"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolveCreateUnicodeFont(collectCreateRequiredRunes([]string{tc.text})); err != nil {
				t.Skipf("no unicode font with %s glyph coverage available: %v", tc.name, err)
			}

			_, info, err := CreateDocument(CreateRequest{
				Title:   tc.text,
				Summary: tc.text,
			})
			if err != nil {
				t.Fatalf("CreateDocument() error = %v", err)
			}

			for _, warning := range info.Warnings {
				if strings.Contains(warning, "replaced unsupported characters") {
					t.Fatalf("%s warnings = %#v, want script glyphs preserved without replacement warning", tc.name, info.Warnings)
				}
			}
		})
	}
}

func TestResolveCreateFontPlanKeepsUnicodeCoverageWhenEmojiPresent(t *testing.T) {
	if _, err := resolveCreateUnicodeFont(collectCreateRequiredRunes([]string{"字段摘要", "字段条目"})); err != nil {
		t.Skipf("no unicode font with Chinese glyph coverage available: %v", err)
	}

	plan := resolveCreateFontPlan([]string{"字段摘要", "📌 字段条目", "📈 2026 概览"})
	if !plan.hasUnicodeFont() {
		t.Fatal("expected unicode font plan to remain available when unsupported emoji are mixed into otherwise supported CJK text")
	}
	if !plan.supportsRune('字') || !plan.supportsRune('段') {
		t.Fatal("expected selected unicode font plan to keep Chinese glyph support")
	}

	sanitized, replaced := sanitizeCreateText("📌 字段条目", plan)
	if !replaced {
		t.Fatal("expected emoji fallback to mark the line as replaced")
	}
	if strings.Contains(sanitized, "?") {
		t.Fatalf("sanitizeCreateText() = %q, want emoji degraded without question-mark corruption", sanitized)
	}
	if !strings.Contains(sanitized, "字段条目") {
		t.Fatalf("sanitizeCreateText() = %q, want Chinese text preserved", sanitized)
	}
}

func TestBuildCreateStyledLines_DropsVariationSelectorWithoutReplacementWarning(t *testing.T) {
	plan := createFontPlan{
		family:       createUnicodeFontFamily,
		unicodeBytes: []byte{1},
		supportsRune: func(r rune) bool {
			return r != '\ufe0f' && unicode.IsGraphic(r)
		},
	}
	lines, warnings := buildCreateStyledLines(CreateRequest{
		Paragraphs: []string{"⚠️ 严重缺水和"},
	}, plan)

	if len(lines) == 0 {
		t.Fatal("expected at least one rendered line")
	}
	if got := lines[0].Text; got != "⚠ 严重缺水和" {
		t.Fatalf("lines[0].Text = %q, want %q", got, "⚠ 严重缺水和")
	}
	for _, warning := range warnings {
		if strings.Contains(warning, "replaced unsupported characters") {
			t.Fatalf("warnings = %#v, want variation-selector-only cleanup to stay silent", warnings)
		}
	}
}

func TestSanitizeCreateText_FallsBackWarningSignWithoutQuestionMarkCorruption(t *testing.T) {
	plan := createFontPlan{
		family:       createUnicodeFontFamily,
		unicodeBytes: []byte{1},
		supportsRune: func(r rune) bool {
			return r != '⚠' && r != '\ufe0f' && unicode.IsGraphic(r)
		},
	}

	sanitized, replaced := sanitizeCreateText("⚠️ 严重缺水和", plan)
	if !replaced {
		t.Fatal("expected warning-sign fallback to count as replacement")
	}
	if strings.Contains(sanitized, "?") {
		t.Fatalf("sanitizeCreateText() = %q, want warning-sign fallback without question marks", sanitized)
	}
	if sanitized != "[!] 严重缺水和" {
		t.Fatalf("sanitizeCreateText() = %q, want %q", sanitized, "[!] 严重缺水和")
	}
}

func TestBuildCreateStyledLinesStripsInlineMarkdownMarkers(t *testing.T) {
	lines, _ := buildCreateStyledLines(CreateRequest{
		Title: "示例报告",
		Sections: []CreateSection{
			{
				Heading:    "示例分组",
				Paragraphs: []string{"**字段标签：** 示例值甲 | 示例值乙 | 示例值丙"},
				Bullets:    []string{"`附注标签` 示例说明"},
			},
		},
	}, testCreateUnicodeGraphicPlan())

	var foundParagraph bool
	for _, line := range lines {
		if strings.Contains(line.Text, "字段标签") {
			foundParagraph = true
			if strings.Contains(line.Text, "**") {
				t.Fatalf("line.Text = %q, want markdown emphasis markers removed", line.Text)
			}
			if line.Text != "字段标签： 示例值甲 | 示例值乙 | 示例值丙" {
				t.Fatalf("line.Text = %q, want cleaned paragraph text", line.Text)
			}
		}
		if line.ListItem != nil && strings.Contains(line.ListItem.Text, "附注标签") && strings.Contains(line.ListItem.Text, "`") {
			t.Fatalf("bullet text = %q, want inline code markers removed", line.ListItem.Text)
		}
	}
	if !foundParagraph {
		t.Fatalf("lines = %#v, want cleaned paragraph containing 字段标签", lines)
	}
}

func TestBuildCreateStyledLinesConvertsSeparatorSentinelToDividerBlock(t *testing.T) {
	lines, _ := buildCreateStyledLines(CreateRequest{
		Title: "示例报告",
		Sections: []CreateSection{
			{
				Heading:    "示例分组",
				Paragraphs: []string{createDividerSentinel, "后续说明"},
			},
		},
	}, testCreateUnicodeGraphicPlan())

	dividerCount := 0
	foundParagraph := false
	for _, line := range lines {
		if line.Divider != nil {
			dividerCount++
		}
		if strings.Contains(line.Text, createDividerSentinel) {
			t.Fatalf("line.Text = %q, want separator sentinel rendered as divider block instead of text", line.Text)
		}
		if line.Text == "后续说明" {
			foundParagraph = true
		}
	}

	if dividerCount != 1 {
		t.Fatalf("dividerCount = %d, want 1 (%#v)", dividerCount, lines)
	}
	if !foundParagraph {
		t.Fatalf("lines = %#v, want paragraph after divider preserved", lines)
	}
}

func TestBuildCreateStyledLinesPreservesOrderedAndUnorderedListMarkers(t *testing.T) {
	lines, _ := buildCreateStyledLines(CreateRequest{
		Title: "示例报告",
		Sections: []CreateSection{
			{
				Heading: "示例清单",
				Bullets: []string{
					"1. 第一项说明",
					"继续处理后续步骤",
					"☑ 已完成示例准备",
				},
			},
		},
	}, testCreateUnicodeGraphicPlan())

	var (
		orderedFound   bool
		unorderedFound bool
		checkboxFound  bool
	)
	for _, line := range lines {
		if strings.Contains(line.Text, "- 1. 第一项说明") {
			t.Fatalf("line.Text = %q, want ordered list marker preserved without injected '- ' prefix", line.Text)
		}
		if line.ListItem == nil {
			continue
		}
		switch {
		case line.ListItem.Marker == "1." && line.ListItem.Text == "第一项说明":
			orderedFound = true
		case line.ListItem.Marker == "•" && line.ListItem.Text == "继续处理后续步骤":
			unorderedFound = true
		case line.ListItem.Marker == "☑" && line.ListItem.Text == "已完成示例准备":
			checkboxFound = true
		}
	}

	if !orderedFound || !unorderedFound || !checkboxFound {
		t.Fatalf("list markers not preserved as expected: ordered=%v unordered=%v checkbox=%v lines=%#v", orderedFound, unorderedFound, checkboxFound, lines)
	}
}

func TestBuildCreateStyledLinesElevatesLeadInParagraphs(t *testing.T) {
	lines, _ := buildCreateStyledLines(CreateRequest{
		Title: "示例报告",
		Sections: []CreateSection{
			{
				Heading:    "示例分节",
				Paragraphs: []string{"摘要标签： 示例正文甲，示例正文乙，示例正文丙。"},
			},
		},
		HeadingColor: "#1F2937",
		BodyColor:    "#111827",
	}, testCreateUnicodeGraphicPlan())

	var (
		foundLead bool
		foundBody bool
	)
	for _, line := range lines {
		if line.Text == "摘要标签：" {
			foundLead = true
			if line.Font != createFontBold {
				t.Fatalf("lead line font = %q, want %q", line.Font, createFontBold)
			}
			if line.FontSize != 12 {
				t.Fatalf("lead line font size = %v, want 12", line.FontSize)
			}
		}
		if strings.HasPrefix(line.Text, "示例正文甲，示例正文乙") {
			foundBody = true
			if line.Font != createFontRegular {
				t.Fatalf("body line font = %q, want %q", line.Font, createFontRegular)
			}
		}
		if strings.Contains(line.Text, "摘要标签： 示例正文甲") {
			t.Fatalf("line.Text = %q, want lead-in label split into heading and body lines", line.Text)
		}
	}

	if !foundLead || !foundBody {
		t.Fatalf("expected split lead paragraph, foundLead=%v foundBody=%v lines=%#v", foundLead, foundBody, lines)
	}
}

func TestBuildCreateStyledLinesElevatesMonthTitleParagraphs(t *testing.T) {
	lines, _ := buildCreateStyledLines(CreateRequest{
		Title: "示例报告",
		Sections: []CreateSection{
			{
				Heading: "阶段明细",
				Paragraphs: []string{
					"五月示例（6月6日-7月7日） ★☆☆ 最佳",
					"这是示例段落，保留模式验证。",
				},
			},
		},
		HeadingColor: "#1F2937",
		BodyColor:    "#111827",
	}, testCreateUnicodeGraphicPlan())

	var (
		foundMonthTitle bool
		foundBody       bool
	)
	for _, line := range lines {
		if line.Text == "五月示例（6月6日-7月7日） ★☆☆ 最佳" {
			foundMonthTitle = true
			if line.Font != createFontBold {
				t.Fatalf("month title font = %q, want %q", line.Font, createFontBold)
			}
			if line.FontSize != 12 {
				t.Fatalf("month title font size = %v, want 12", line.FontSize)
			}
		}
		if strings.HasPrefix(line.Text, "这是示例段落") {
			foundBody = true
			if line.Font != createFontRegular {
				t.Fatalf("month body font = %q, want %q", line.Font, createFontRegular)
			}
		}
	}

	if !foundMonthTitle || !foundBody {
		t.Fatalf("expected elevated month title and body, foundMonthTitle=%v foundBody=%v lines=%#v", foundMonthTitle, foundBody, lines)
	}
}

func TestBuildCreateStyledLinesMergesMonthTitleWithFollowingRatingLead(t *testing.T) {
	lines, _ := buildCreateStyledLines(CreateRequest{
		Title: "示例报告",
		Sections: []CreateSection{
			{
				Heading: "阶段明细",
				Paragraphs: []string{
					"五月示例（6月6日-7月7日）",
					"★☆☆ 提示这是示例段落，保留模式验证。",
				},
			},
		},
		HeadingColor: "#1F2937",
		BodyColor:    "#111827",
	}, testCreateUnicodeGraphicPlan())

	var (
		foundMonthTitle bool
		foundBody       bool
	)
	for _, line := range lines {
		if line.Text == "五月示例（6月6日-7月7日） ★☆☆" {
			foundMonthTitle = true
			if line.Font != createFontBold {
				t.Fatalf("merged month title font = %q, want %q", line.Font, createFontBold)
			}
		}
		if strings.HasPrefix(line.Text, "提示这是示例段落") {
			foundBody = true
			if strings.HasPrefix(line.Text, "★☆☆") {
				t.Fatalf("line.Text = %q, want star rating moved into month title line", line.Text)
			}
		}
	}

	if !foundMonthTitle || !foundBody {
		t.Fatalf("expected merged month title and trimmed body, foundMonthTitle=%v foundBody=%v lines=%#v", foundMonthTitle, foundBody, lines)
	}
}

func TestBuildCreateStyledLinesSplitsCombinedMonthParagraph(t *testing.T) {
	lines, _ := buildCreateStyledLines(CreateRequest{
		Title: "示例报告",
		Sections: []CreateSection{
			{
				Heading: "阶段明细",
				Paragraphs: []string{
					"正月示例（2月4日-3月5日） ★★☆示例正文甲，示例正文乙，保留模式验证。",
				},
			},
		},
		HeadingColor: "#1F2937",
		BodyColor:    "#111827",
	}, testCreateUnicodeGraphicPlan())

	var (
		foundMonthTitle bool
		foundBody       bool
	)
	for _, line := range lines {
		if line.Text == "正月示例（2月4日-3月5日） ★★☆" {
			foundMonthTitle = true
			if line.Font != createFontBold {
				t.Fatalf("combined month title font = %q, want %q", line.Font, createFontBold)
			}
		}
		if strings.HasPrefix(line.Text, "示例正文甲，示例正文乙") {
			foundBody = true
			if line.Font != createFontRegular {
				t.Fatalf("combined month body font = %q, want %q", line.Font, createFontRegular)
			}
		}
		if strings.Contains(line.Text, "★★☆示例正文甲") {
			t.Fatalf("line.Text = %q, want combined month paragraph split into title and body lines", line.Text)
		}
	}

	if !foundMonthTitle || !foundBody {
		t.Fatalf("expected split combined month paragraph, foundMonthTitle=%v foundBody=%v lines=%#v", foundMonthTitle, foundBody, lines)
	}
}

func TestCreateDocumentDoesNotFlattenTablesIntoPipeText(t *testing.T) {
	data, _, err := CreateDocument(CreateRequest{
		Title: "Report",
		Sections: []CreateSection{
			{
				Heading: "Snapshot",
				Table: &CreateTable{
					Headers: []string{"Name", "Role"},
					Rows: [][]string{
						{"Sample", "Analyst"},
						{"Example", "Owner"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "table.pdf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result := extractCreatedPDFForTest(t, path)
	text := normalizeText(result.Text)
	for _, needle := range []string{"Name", "Role", "Sample", "Analyst", "Example", "Owner"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("extracted text = %q, want fragment %q", text, needle)
		}
	}
	if strings.Contains(text, "Name | Role") {
		t.Fatalf("pdf bytes contain flattened table header text, want structured table rendering")
	}
	if strings.Contains(text, "Sample | Analyst") {
		t.Fatalf("pdf bytes contain flattened table row text, want structured table rendering")
	}
}

func extractCreatedPDFForTest(t *testing.T, path string) ExtractResult {
	t.Helper()

	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   t.TempDir(),
		AutoDownload: false,
	})
	t.Cleanup(func() { _ = svc.Close() })

	result, err := svc.Extract(context.Background(), ExtractRequest{
		Path:         path,
		IncludePages: true,
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	return result
}

func TestCleanCreateInlineMarkdownPreservesHardBreakAsSeparator(t *testing.T) {
	got := cleanCreateInlineMarkdown("**字段标签：** 示例值甲  \n**第二字段：** 示例值乙")
	want := "字段标签： 示例值甲 · 第二字段： 示例值乙"
	if got != want {
		t.Fatalf("cleanCreateInlineMarkdown() = %q, want %q", got, want)
	}
}

func TestCleanCreateInlineMarkdownTreatsPlainNewlinesAsSeparators(t *testing.T) {
	got := cleanCreateInlineMarkdown("示例值甲\n第二字段： 示例值乙")
	want := "示例值甲 · 第二字段： 示例值乙"
	if got != want {
		t.Fatalf("cleanCreateInlineMarkdown() = %q, want %q", got, want)
	}
}

func testCreateUnicodeGraphicPlan() createFontPlan {
	return createFontPlan{
		family:       createUnicodeFontFamily,
		unicodeBytes: []byte{1},
		supportsRune: func(r rune) bool { return unicode.IsGraphic(r) },
	}
}
