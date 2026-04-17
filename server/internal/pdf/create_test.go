package pdf

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"go.uber.org/zap"
)

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

	if !bytes.Contains(data, []byte("Name")) || !bytes.Contains(data, []byte("Analyst")) {
		t.Fatalf("pdf bytes missing expected table text content")
	}
	if bytes.Contains(data, []byte("Name | Role")) {
		t.Fatalf("pdf bytes contain flattened table header text, want structured table rendering")
	}
	if bytes.Contains(data, []byte("Sample | Analyst")) {
		t.Fatalf("pdf bytes contain flattened table row text, want structured table rendering")
	}
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

func TestCreateDocumentPreservesWrappedCJKSubtitleText(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("wrap-regression extraction currently verified with native darwin PDFKit")
	}
	if _, err := resolveCreateUnicodeFont(collectCreateRequiredRunes([]string{"字段戊", "示例值四"})); err != nil {
		t.Skipf("no unicode font with Chinese glyph coverage available: %v", err)
	}

	data, _, err := CreateDocument(CreateRequest{
		Title:    "示例中文长标题文档",
		Subtitle: "字段甲： 示例值一 | 字段乙： 示例值二 | 字段丙： 示例值三 | 字段丁： 示例值四 | 字段戊： 2026年4月11日 | 字段己： 示例值五",
	})
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "wrapped_subtitle.pdf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   "testdata",
		AutoDownload: false,
	})
	t.Cleanup(func() {
		_ = svc.Close()
	})

	result, err := svc.Extract(context.Background(), ExtractRequest{Path: path, IncludePages: true})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if !strings.Contains(result.Text, "字段戊") {
		t.Fatalf("extracted text = %q, want wrapped subtitle to preserve 字段戊", result.Text)
	}
}

func testCreateUnicodeGraphicPlan() createFontPlan {
	return createFontPlan{
		family:       createUnicodeFontFamily,
		unicodeBytes: []byte{1},
		supportsRune: func(r rune) bool { return unicode.IsGraphic(r) },
	}
}
