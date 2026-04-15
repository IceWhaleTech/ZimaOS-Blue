package pdf

import (
	"strings"
	"testing"
	"unicode"
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
	}, createFontPlan{
		family:       createUnicodeFontFamily,
		unicodeBytes: []byte{1},
		supportsRune: func(r rune) bool { return unicode.IsGraphic(r) },
	})

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
