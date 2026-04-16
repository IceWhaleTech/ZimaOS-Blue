package convert

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestBuildNativeMarkdownPPTX_UsesSafeMidnightLikeThemeAndFonts(t *testing.T) {
	data, err := buildNativeMarkdownPPTX([]nativeMarkdownPPTXSlide{{
		Title: "Qwen3 最新优化介绍",
		Lines: []string{"混合思考架构", "多语言能力继续提升"},
	}})
	if err != nil {
		t.Fatalf("buildNativeMarkdownPPTX failed: %v", err)
	}

	themeXML := nativeMarkdownPPTXZipEntryText(t, data, "ppt/theme/theme1.xml")
	for _, needle := range []string{
		`name="Midnight Theme"`,
		`val="1E3A5F"`,
		`val="00D4AA"`,
		`typeface="Georgia"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
		`script="Hans" typeface="Hiragino Sans GB"`,
	} {
		if !strings.Contains(themeXML, needle) {
			t.Fatalf("expected theme1.xml to include %q, got %s", needle, themeXML)
		}
	}

	slideXML := nativeMarkdownPPTXZipEntryText(t, data, "ppt/slides/slide1.xml")
	for _, needle := range []string{
		`name="Background"`,
		`name="Band"`,
		`name="Accent"`,
		`<a:latin typeface="Georgia"/>`,
		`<a:latin typeface="Helvetica"/>`,
		`<a:ea typeface="Hiragino Sans GB"/>`,
		`<a:cs typeface="Helvetica"/>`,
		`val="E8EEF4"`,
		`val="00D4AA"`,
	} {
		if !strings.Contains(slideXML, needle) {
			t.Fatalf("expected slide1.xml to include %q, got %s", needle, slideXML)
		}
	}
}

func TestBuildNativeMarkdownPPTX_AppliesThemeFromStyleHint(t *testing.T) {
	data, err := buildNativeMarkdownPPTXWithOptions([]nativeMarkdownPPTXSlide{{
		Title: "Launch Deck",
		Lines: []string{"Fast install"},
	}}, PresentationOptions{
		StyleHint: "startup pitch",
	})
	if err != nil {
		t.Fatalf("buildNativeMarkdownPPTXWithOptions failed: %v", err)
	}

	themeXML := nativeMarkdownPPTXZipEntryText(t, data, "ppt/theme/theme1.xml")
	for _, needle := range []string{
		`name="Coral Theme"`,
		`val="FF6B6B"`,
		`typeface="Helvetica Neue"`,
		`typeface="Helvetica"`,
	} {
		if !strings.Contains(themeXML, needle) {
			t.Fatalf("expected theme1.xml to include %q, got %s", needle, themeXML)
		}
	}
}

func TestBuildNativeMarkdownPPTX_AppliesEditorialThemeFromStyleHint(t *testing.T) {
	data, err := buildNativeMarkdownPPTXWithOptions([]nativeMarkdownPPTXSlide{{
		Title: "Signal Systems",
		Lines: []string{"Editorial layout", "High-contrast narrative"},
	}}, PresentationOptions{
		StyleHint: "editorial poster manifesto",
	})
	if err != nil {
		t.Fatalf("buildNativeMarkdownPPTXWithOptions failed: %v", err)
	}

	themeXML := nativeMarkdownPPTXZipEntryText(t, data, "ppt/theme/theme1.xml")
	for _, needle := range []string{
		`name="Editorial Theme"`,
		`val="0A0D14"`,
		`val="FFB700"`,
		`val="00E5FF"`,
		`typeface="Arial Black"`,
		`typeface="Helvetica"`,
	} {
		if !strings.Contains(themeXML, needle) {
			t.Fatalf("expected theme1.xml to include %q, got %s", needle, themeXML)
		}
	}

	slideXML := nativeMarkdownPPTXZipEntryText(t, data, "ppt/slides/slide1.xml")
	for _, needle := range []string{
		`name="Background"`,
		`name="Band"`,
		`name="Accent"`,
		`val="F5F6F8"`,
		`val="FFB700"`,
		`typeface="Arial Black"`,
		`typeface="Helvetica"`,
	} {
		if !strings.Contains(slideXML, needle) {
			t.Fatalf("expected slide1.xml to include %q, got %s", needle, slideXML)
		}
	}
}

func nativeMarkdownPPTXZipEntryText(t *testing.T, data []byte, name string) string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("Open(%s) error = %v", name, err)
		}
		defer rc.Close()
		body, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll(%s) error = %v", name, err)
		}
		return string(body)
	}

	t.Fatalf("zip entry %s not found", name)
	return ""
}
