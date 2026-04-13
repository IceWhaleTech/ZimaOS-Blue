package embedded

import (
	"io/fs"
	"strings"
	"testing"
)

func TestPacksFSIncludesOpenAIResponsesDoc(t *testing.T) {
	data, err := fs.ReadFile(PacksFS, "packs/openai/docs/responses-api/DOC.md")
	if err != nil {
		t.Fatalf("ReadFile(openai responses doc): %v", err)
	}
	if !strings.Contains(string(data), "responses") {
		t.Fatal("expected bundled responses doc")
	}
}
