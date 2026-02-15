package pruner

import (
	"strings"
	"testing"
)

func TestDetectContentType_GoCode(t *testing.T) {
	code := strings.Repeat("package main\nimport \"fmt\"\nfunc main() {\n"+
		"\tfmt.Println(\"hello\")\n}\n", 5)
	got := DetectContentType(code, 5)
	if got != ContentCode {
		t.Errorf("expected ContentCode, got %v", got)
	}
}

func TestDetectContentType_Markdown(t *testing.T) {
	doc := "# Title\n\nSome introduction paragraph.\n\n" +
		"## Section One\n\nThis is a paragraph about something.\n\n" +
		"## Section Two\n\nAnother paragraph with details.\n\n" +
		strings.Repeat("More text content here.\n", 10)
	got := DetectContentType(doc, 5)
	if got != ContentDoc {
		t.Errorf("expected ContentDoc, got %v", got)
	}
}

func TestDetectContentType_Logs(t *testing.T) {
	log := strings.Repeat("2026-02-15T10:30:00Z INFO  server started on :8080\n", 5) +
		strings.Repeat("2026-02-15T10:30:01Z ERROR failed to connect to database\n", 5)
	got := DetectContentType(log, 5)
	if got != ContentLog {
		t.Errorf("expected ContentLog, got %v", got)
	}
}

func TestDetectContentType_JSON(t *testing.T) {
	data := "{\n" +
		"  \"name\": \"test\",\n" +
		"  \"version\": \"1.0\",\n" +
		"  \"dependencies\": {\n" +
		"    \"foo\": \"^1.0.0\",\n" +
		"    \"bar\": \"^2.0.0\"\n" +
		"  },\n" +
		"  \"scripts\": {\n" +
		"    \"build\": \"go build\",\n" +
		"    \"test\": \"go test\"\n" +
		"  }\n" +
		"}\n"
	got := DetectContentType(data, 5)
	if got != ContentData {
		t.Errorf("expected ContentData, got %v", got)
	}
}

func TestDetectContentType_PlainText(t *testing.T) {
	text := strings.Repeat("This is a plain text paragraph with no special formatting or structure.\n", 15)
	got := DetectContentType(text, 5)
	if got != ContentUnknown {
		t.Errorf("expected ContentUnknown for plain text, got %v", got)
	}
}

func TestDetectContentType_BelowMinLines(t *testing.T) {
	code := "func main() {\n}\n"
	got := DetectContentType(code, 100)
	if got != ContentUnknown {
		t.Errorf("expected ContentUnknown for short content, got %v", got)
	}
}
