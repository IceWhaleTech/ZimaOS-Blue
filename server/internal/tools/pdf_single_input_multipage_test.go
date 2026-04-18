package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFToolCreate_FromSingleMarkdownInputProducesMultiplePages(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPDFTool(nil)
	tool.scope = newFSToolScope([]string{tmpDir})

	if err := os.MkdirAll(filepath.Join(tmpDir, "reports"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	var builder strings.Builder
	builder.WriteString("# Native PDF Magazine Report\n\n")
	builder.WriteString("This regression guards the single-input-file workflow.\n\n")
	for section := 1; section <= 8; section++ {
		builder.WriteString(fmt.Sprintf("## Section %d\n\n", section))
		for paragraph := 1; paragraph <= 6; paragraph++ {
			builder.WriteString(strings.Repeat("This paragraph is intentionally long so the PDF renderer must wrap lines and continue onto later pages without truncating the source markdown content. ", 8))
			builder.WriteString("\n\n")
		}
	}

	seedPath := filepath.Join(tmpDir, "reports", "magazine.md")
	if err := os.WriteFile(seedPath, []byte(builder.String()), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/magazine.md",
		"theme":  "editorial",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	validation, ok := payload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", payload["validation"])
	}
	if got := asNativeToolInt(t, validation["page_count"]); got <= 1 {
		t.Fatalf("page_count = %d, want > 1 for long single-input markdown seed", got)
	}
	if got := asNativeToolInt(t, validation["char_count"]); got < 1000 {
		t.Fatalf("char_count = %d, want large rendered payload", got)
	}
	if got := payload["path"]; got != "reports/magazine.pdf" {
		t.Fatalf("path = %v, want reports/magazine.pdf", got)
	}
}
