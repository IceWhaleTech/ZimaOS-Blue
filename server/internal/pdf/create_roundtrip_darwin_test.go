//go:build darwin

package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestCreateDocumentDarwinRoundTripPreservesWrappedSubtitleText(t *testing.T) {
	title := "\u793a\u4f8b\u4e2d\u6587\u957f\u6807\u9898\u6587\u6863"
	subtitle := "\u5b57\u6bb5\u7532\uff1a \u793a\u4f8b\u503c\u4e00 | \u5b57\u6bb5\u4e59\uff1a \u793a\u4f8b\u503c\u4e8c | \u5b57\u6bb5\u4e19\uff1a \u793a\u4f8b\u503c\u4e09 | \u5b57\u6bb5\u4e01\uff1a \u793a\u4f8b\u503c\u56db | \u5b57\u6bb5\u620a\uff1a 2026\u5e745\u67081\u65e5 | \u5b57\u6bb5\u5df1\uff1a \u793a\u4f8b\u503c\u4e94"
	if _, err := resolveCreateUnicodeFont(collectCreateRequiredRunes([]string{title, subtitle})); err != nil {
		t.Skipf("no unicode font with CJK glyph coverage available: %v", err)
	}

	result := runDarwinCreateRoundTrip(t, CreateRequest{
		Title:    title,
		Subtitle: subtitle,
	})

	requireExtractedTextContainsAll(t, result, []string{
		"\u5b57\u6bb5\u620a",
		"2026\u5e745\u67081\u65e5",
		"\u5b57\u6bb5\u5df1",
	})
}

func TestCreateDocumentDarwinRoundTripPreservesBulletsAndNotes(t *testing.T) {
	result := runDarwinCreateRoundTrip(t, CreateRequest{
		Title: "Weekly Summary",
		Sections: []CreateSection{
			{
				Heading:    "Highlights",
				Paragraphs: []string{"Execution remained stable across the migration window."},
				Bullets: []string{
					"1. Native renderer stayed active",
					"Fallback path remained unused",
				},
			},
		},
		Notes: []string{
			"Observed on macOS only",
			"Validated through native extract",
		},
	})

	requireExtractedTextContainsAll(t, result, []string{
		"Highlights",
		"Native renderer stayed active",
		"Observed on macOS only",
		"Validated through native extract",
	})
}

func TestCreateDocumentDarwinRoundTripPreservesTableText(t *testing.T) {
	result := runDarwinCreateRoundTrip(t, CreateRequest{
		Title: "Status Report",
		Sections: []CreateSection{
			{
				Heading: "Table Check",
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

	requireExtractedTextContainsAll(t, result, []string{
		"Table Check",
		"Name",
		"Role",
		"Sample",
		"Owner",
	})
}

func runDarwinCreateRoundTrip(t *testing.T, req CreateRequest) ExtractResult {
	t.Helper()

	data, _, err := CreateDocument(req)
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "native-create-roundtrip.pdf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   t.TempDir(),
		AutoDownload: false,
	})
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		_ = cfg
		t.Fatal("expected darwin create round-trip extraction to avoid pdfium initialization")
		return nil, nil
	}
	t.Cleanup(func() {
		_ = svc.Close()
	})

	result, err := svc.Extract(context.Background(), ExtractRequest{
		Path:         path,
		IncludePages: true,
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if result.Document.Engine != "pdfkit/native" {
		t.Fatalf("engine = %q, want %q", result.Document.Engine, "pdfkit/native")
	}
	if len(result.Pages) == 0 {
		t.Fatalf("pages = %#v, want at least one extracted page", result.Pages)
	}
	return result
}

func requireExtractedTextContainsAll(t *testing.T, result ExtractResult, fragments []string) {
	t.Helper()

	text := normalizeText(result.Text)
	if text == "" {
		t.Fatalf("result.Text = %q, want non-empty extracted text", result.Text)
	}

	for _, fragment := range fragments {
		if strings.Contains(text, fragment) {
			continue
		}
		t.Fatalf("normalized extracted text = %q, want fragment %q", text, fragment)
	}
}
