package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

func TestPPTXToolTemplateEditingActions(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/template.pptx",
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

	for _, args := range []map[string]interface{}{
		{
			"action": "duplicate_slide",
			"path":   "decks/template.pptx",
			"slide":  3,
		},
		{
			"action": "replace_text",
			"path":   "decks/template.pptx",
			"replacements": map[string]interface{}{
				"Overview": "Executive Overview",
			},
		},
		{
			"action": "delete_slide",
			"path":   "decks/template.pptx",
			"slide":  5,
		},
		{
			"action": "reorder_slides",
			"path":   "decks/template.pptx",
			"order":  []interface{}{1, 2, 5, 3, 4},
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	validateResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate_template",
		"path":   "decks/template.pptx",
	})
	if err != nil {
		t.Fatalf("validate_template failed: %v", err)
	}
	validatePayload := parseNativeDocumentPayload(t, validateResult)
	validation, ok := validatePayload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", validatePayload["validation"])
	}
	if got := asNativeToolInt(t, validation["slide_count"]); got != 5 {
		t.Fatalf("slide_count = %d, want 5", got)
	}

	deckPath := filepath.Join(tmpDir, "decks", "template.pptx")
	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), deckPath)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if count := strings.Count(doc.Text, "Executive Overview"); count != 3 {
		t.Fatalf("Executive Overview count = %d, want 3 including TOC, text=%q", count, doc.Text)
	}
	if !containsSubstring(doc.Text, "[Slide 3]\nSummary") {
		t.Fatalf("expected reordered slide 3 to be Summary, got %q", doc.Text)
	}
	if containsSubstring(doc.Text, "[Slide 4]\nRisks") || containsSubstring(doc.Text, "[Slide 5]\nRisks") {
		t.Fatalf("expected deleted Risks slide to be absent, got %q", doc.Text)
	}

	data, err := os.ReadFile(deckPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	presentationXML := officeZipEntryText(t, data, "ppt/presentation.xml")
	if count := strings.Count(presentationXML, "<p:sldId "); count != 5 {
		t.Fatalf("presentation.xml slide count = %d, want 5, xml=%s", count, presentationXML)
	}
	if officeZipHasEntry(t, data, "ppt/slides/slide6.xml") {
		t.Fatal("expected orphaned slide6.xml to be removed")
	}
}

func TestPPTXToolReplaceTextOnlyTouchesTextNodes(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/text_only.pptx",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Token",
				"paragraphs": []interface{}{"Token body"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "text_only.pptx")
	_, err = replaceArchiveEntries(path, func(name string) bool {
		return name == "ppt/slides/slide2.xml"
	}, func(_ string, data []byte) ([]byte, bool, error) {
		updated := strings.Replace(string(data), `name="Title"`, `name="TokenShape"`, 1)
		return []byte(updated), true, nil
	})
	if err != nil {
		t.Fatalf("inject slide attribute token failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "replace_text",
		"path":   "decks/text_only.pptx",
		"replacements": map[string]interface{}{
			"Token": "Updated",
		},
	})
	if err != nil {
		t.Fatalf("replace_text failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	slideXML := officeZipEntryText(t, data, "ppt/slides/slide2.xml")
	if !strings.Contains(slideXML, ">Updated<") {
		t.Fatalf("expected slide text to be updated, got %s", slideXML)
	}
	if !strings.Contains(slideXML, `name="TokenShape"`) {
		t.Fatalf("expected non-text attribute to remain unchanged, got %s", slideXML)
	}
	if strings.Contains(slideXML, `name="UpdatedShape"`) {
		t.Fatalf("expected replace_text not to mutate attributes, got %s", slideXML)
	}
}

func TestPPTXToolTemplateMutationPreservesChartContentTypes(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_template.pptx",
		"title":  "Chart Template",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Date chart slide"},
				"chart": map[string]interface{}{
					"type":        "line",
					"x_axis_type": "date",
					"categories":  []interface{}{"2026-01-01", "2026-02-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "replace_text",
		"path":   "decks/chart_template.pptx",
		"replacements": map[string]interface{}{
			"Revenue": "Revenue Updated",
		},
	})
	if err != nil {
		t.Fatalf("replace_text failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	contentTypes := officeZipEntryText(t, data, "[Content_Types].xml")
	for _, needle := range []string{
		`<Override PartName="/ppt/charts/chart1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>`,
	} {
		if !containsSubstring(contentTypes, needle) {
			t.Fatalf("expected [Content_Types].xml to include %q, got %s", needle, contentTypes)
		}
	}
	if !officeZipHasEntry(t, data, "ppt/charts/chart1.xml") {
		t.Fatal("expected ppt/charts/chart1.xml to remain after template mutation")
	}
	if !officeZipHasEntry(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx") {
		t.Fatal("expected embedded workbook to remain after template mutation")
	}
	slideRels := officeZipEntryText(t, data, "ppt/slides/_rels/slide3.xml.rels")
	if !containsSubstring(slideRels, `Target="../charts/chart1.xml"`) {
		t.Fatalf("expected slide3 rels to keep chart target, got %s", slideRels)
	}
}

func TestPPTXToolTemplateDeleteChartSlideRemovesOrphanChartArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_delete_template.pptx",
		"title":  "Chart Delete Template",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Date chart slide"},
				"chart": map[string]interface{}{
					"type":        "line",
					"x_axis_type": "date",
					"categories":  []interface{}{"2026-01-01", "2026-02-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "delete_slide",
		"path":   "decks/chart_delete_template.pptx",
		"slide":  3,
	})
	if err != nil {
		t.Fatalf("delete_slide failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_delete_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if officeZipHasEntry(t, data, "ppt/charts/chart1.xml") {
		t.Fatal("expected orphan chart1.xml to be removed")
	}
	if officeZipHasEntry(t, data, "ppt/charts/_rels/chart1.xml.rels") {
		t.Fatal("expected orphan chart1.xml.rels to be removed")
	}
	if officeZipHasEntry(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx") {
		t.Fatal("expected orphan embedded workbook to be removed")
	}
	contentTypes := officeZipEntryText(t, data, "[Content_Types].xml")
	for _, needle := range []string{
		`/ppt/charts/chart1.xml`,
		`Extension="xlsx"`,
	} {
		if containsSubstring(contentTypes, needle) {
			t.Fatalf("expected [Content_Types].xml to drop %q after chart-slide deletion, got %s", needle, contentTypes)
		}
	}
}

func TestPPTXToolTemplateDuplicateChartSlideClonesChartArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_duplicate_template.pptx",
		"title":  "Chart Duplicate Template",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Date chart slide"},
				"chart": map[string]interface{}{
					"type":        "line",
					"x_axis_type": "date",
					"categories":  []interface{}{"2026-01-01", "2026-02-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "duplicate_slide",
		"path":   "decks/chart_duplicate_template.pptx",
		"slide":  3,
	})
	if err != nil {
		t.Fatalf("duplicate_slide failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_duplicate_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, entry := range []string{
		"ppt/charts/chart1.xml",
		"ppt/charts/chart2.xml",
		"ppt/charts/_rels/chart1.xml.rels",
		"ppt/charts/_rels/chart2.xml.rels",
		"ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx",
		"ppt/embeddings/Microsoft_Excel_Worksheet2.xlsx",
	} {
		if !officeZipHasEntry(t, data, entry) {
			t.Fatalf("expected %s to exist after duplicating chart slide", entry)
		}
	}
	slide3Rels := officeZipEntryText(t, data, "ppt/slides/_rels/slide3.xml.rels")
	if !containsSubstring(slide3Rels, `Target="../charts/chart1.xml"`) {
		t.Fatalf("expected slide3 rels to keep chart1 target, got %s", slide3Rels)
	}
	slide4Rels := officeZipEntryText(t, data, "ppt/slides/_rels/slide4.xml.rels")
	if !containsSubstring(slide4Rels, `Target="../charts/chart2.xml"`) {
		t.Fatalf("expected duplicated slide rels to target chart2, got %s", slide4Rels)
	}
	chart2Rels := officeZipEntryText(t, data, "ppt/charts/_rels/chart2.xml.rels")
	if !containsSubstring(chart2Rels, `Target="../embeddings/Microsoft_Excel_Worksheet2.xlsx"`) {
		t.Fatalf("expected chart2 rels to target workbook2, got %s", chart2Rels)
	}
	contentTypes := officeZipEntryText(t, data, "[Content_Types].xml")
	for _, needle := range []string{
		`<Override PartName="/ppt/charts/chart1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Override PartName="/ppt/charts/chart2.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>`,
	} {
		if !containsSubstring(contentTypes, needle) {
			t.Fatalf("expected [Content_Types].xml to include %q, got %s", needle, contentTypes)
		}
	}
}
