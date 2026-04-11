package toolsintegration

import (
	"archive/zip"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestPPTXToolCreateBuildsPlannedDeckStructure(t *testing.T) {
	tmpDir := t.TempDir()
	tool := tools.NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/launch.pptx",
		"title":    "Launch Plan",
		"subtitle": "Q2 roll-out",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Overview",
				"paragraphs": []interface{}{"Beta in April", "GA in June"},
			},
			map[string]interface{}{
				"heading": "Risks",
				"bullets": []interface{}{"Migration timing", "Support readiness"},
			},
		},
		"summary": "Focus on a clean 16:9 executive structure.",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := int(payload["slide_count"].(float64)); got != 5 {
		t.Fatalf("slide_count = %d, want 5", got)
	}

	path := filepath.Join(tmpDir, "decks", "launch.pptx")
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open pptx: %v", err)
	}
	defer reader.Close()

	slideFiles := 0
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			slideFiles++
		}
	}
	if slideFiles != 5 {
		t.Fatalf("slide xml count = %d, want 5", slideFiles)
	}

	doc, err := convertpkg.NewDocumentReader().ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Launch Plan", "Table of Contents", "Overview", "Risks", "Summary"} {
		if !strings.Contains(doc.Text, needle) {
			t.Fatalf("expected deck text to contain %q, got %q", needle, doc.Text)
		}
	}
}
