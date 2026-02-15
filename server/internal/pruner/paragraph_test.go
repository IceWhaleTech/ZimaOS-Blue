package pruner

import (
	"strings"
	"testing"
)

func TestSegmentizeParagraphs_Headings(t *testing.T) {
	text := "# Title\n\nIntro paragraph.\n\n## Section One\n\nFirst section content.\n\n## Section Two\n\nSecond section content.\n"
	segs := SegmentizeParagraphs(text)
	if len(segs) == 0 {
		t.Fatal("expected segments")
	}
	// Should have heading segments
	hasHeading := false
	for _, s := range segs {
		if s.Kind == SegmentHeading {
			hasHeading = true
			break
		}
	}
	if !hasHeading {
		t.Error("expected at least one SegmentHeading")
	}
}

func TestSegmentizeParagraphs_Paragraphs(t *testing.T) {
	text := "First paragraph with some text.\n\nSecond paragraph with more text.\n\nThird paragraph here.\n"
	segs := SegmentizeParagraphs(text)
	if len(segs) < 3 {
		t.Errorf("expected at least 3 paragraph segments, got %d", len(segs))
	}
	for _, s := range segs {
		if s.Kind != SegmentParagraph {
			t.Errorf("expected SegmentParagraph, got %v", s.Kind)
		}
	}
}

func TestSegmentizeParagraphs_Empty(t *testing.T) {
	segs := SegmentizeParagraphs("")
	if len(segs) != 0 {
		t.Errorf("expected 0 segments for empty input, got %d", len(segs))
	}
}

func TestSegmentizeParagraphs_Mixed(t *testing.T) {
	text := "## Overview\n\nThis is the overview.\n\n## Details\n\nHere are the details.\nWith multiple lines.\n"
	segs := SegmentizeParagraphs(text)
	if len(segs) < 4 {
		t.Errorf("expected at least 4 segments (2 headings + 2 paragraphs), got %d", len(segs))
	}
}

func TestSegmentizeLogs_Timestamped(t *testing.T) {
	log := "2026-02-15T10:00:00Z INFO  Starting server\n" +
		"2026-02-15T10:00:01Z INFO  Listening on :8080\n" +
		"\n" +
		"2026-02-15T10:00:05Z ERROR Connection refused\n" +
		"2026-02-15T10:00:05Z ERROR Retry in 5s\n"
	segs := SegmentizeLogs(log)
	if len(segs) == 0 {
		t.Fatal("expected log segments")
	}
	for _, s := range segs {
		if s.Kind != SegmentLogGroup {
			t.Errorf("expected SegmentLogGroup, got %v", s.Kind)
		}
	}
}

func TestSegmentizeLogs_Empty(t *testing.T) {
	segs := SegmentizeLogs("")
	if len(segs) != 0 {
		t.Errorf("expected 0 segments for empty input, got %d", len(segs))
	}
}

func TestSegmentizeData_JSON(t *testing.T) {
	data := "{\n  \"name\": \"test\",\n  \"config\": {\n    \"key\": \"value\"\n  },\n  \"items\": [1, 2, 3]\n}\n"
	segs := SegmentizeData(data)
	if len(segs) == 0 {
		t.Fatal("expected data segments")
	}
	for _, s := range segs {
		if s.Kind != SegmentDataKey {
			t.Errorf("expected SegmentDataKey, got %v", s.Kind)
		}
	}
}

func TestSegmentizeData_Empty(t *testing.T) {
	segs := SegmentizeData("")
	if len(segs) != 0 {
		t.Errorf("expected 0 segments for empty input, got %d", len(segs))
	}
}

func TestAutoSegmentize_Code(t *testing.T) {
	code := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
	segs := AutoSegmentize(code, ContentCode)
	if len(segs) == 0 {
		t.Fatal("expected segments for code")
	}
}

func TestAutoSegmentize_Doc(t *testing.T) {
	doc := "## Title\n\nSome paragraph.\n\n## Another\n\nMore text.\n"
	segs := AutoSegmentize(doc, ContentDoc)
	if len(segs) == 0 {
		t.Fatal("expected segments for doc")
	}
	hasHeading := false
	for _, s := range segs {
		if s.Kind == SegmentHeading {
			hasHeading = true
		}
	}
	if !hasHeading {
		t.Error("expected heading segments for doc content")
	}
}

func TestAutoSegmentize_Log(t *testing.T) {
	log := strings.Repeat("2026-02-15T10:00:00Z INFO  test\n", 5)
	segs := AutoSegmentize(log, ContentLog)
	if len(segs) == 0 {
		t.Fatal("expected segments for log")
	}
}

func TestAutoSegmentize_Unknown(t *testing.T) {
	text := "Some plain text.\n\nAnother paragraph.\n"
	segs := AutoSegmentize(text, ContentUnknown)
	if len(segs) == 0 {
		t.Fatal("expected segments for unknown content (fallback to paragraphs)")
	}
}
