package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageToolAdapter_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"image_tool_adapter.go": {
			maxLines: 85,
			tokens: []string{
				"func newImageGenerateAdapter(",
				"func waitForGeneratedImageResult(",
			},
		},
		"image_tool_adapter_output.go": {
			maxLines: 195,
			tokens: []string{
				"func withImageOutputWorkspaceScope(",
				"func saveGeneratedImageOutput(",
				"func loadGeneratedImageData(",
			},
		},
		"image_tool_adapter_result.go": {
			maxLines: 190,
			tokens: []string{
				"func imageGenerateMediaRequest(",
				"func newImageTaskLookupAdapter(",
				"func imageTaskFromMediaTask(",
				"func decodeCompatImageBase64(",
			},
		},
	}

	for name, expectation := range files {
		content, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)
		if lines := strings.Count(source, "\n") + 1; lines > expectation.maxLines {
			t.Fatalf("expected %s to stay below %d lines, got %d", name, expectation.maxLines, lines)
		}
		for _, token := range expectation.tokens {
			if !strings.Contains(source, token) {
				t.Fatalf("expected %s to contain token %q", name, token)
			}
		}
	}
}
