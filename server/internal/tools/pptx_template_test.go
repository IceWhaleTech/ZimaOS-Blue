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

func TestPPTXToolTemplateUpdateChartDataRefreshesWorkbookBackedChart(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_template.pptx",
		"title":  "Chart Update Template",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original chart slide"},
				"chart": map[string]interface{}{
					"type":       "bar",
					"categories": []interface{}{"North", "South"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 98}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_template.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"type":       "bar",
			"categories": []interface{}{"East", "West", "Central"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{140, 110, 95}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	slideRels := officeZipEntryText(t, data, "ppt/slides/_rels/slide3.xml.rels")
	if !containsSubstring(slideRels, `Target="../charts/chart1.xml"`) {
		t.Fatalf("expected slide3 rels to keep chart1 target, got %s", slideRels)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:cat><c:strRef><c:f>Data!$A$2:$A$4</c:f><c:strCache><c:ptCount val="3"/>`,
		`<c:pt idx="0"><c:v>East</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>West</c:v></c:pt>`,
		`<c:pt idx="2"><c:v>Central</c:v></c:pt>`,
		`<c:val><c:numRef><c:f>Data!$B$2:$B$4</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="3"/>`,
		`<c:pt idx="0"><c:v>140</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>110</c:v></c:pt>`,
		`<c:pt idx="2"><c:v>95</c:v></c:pt>`,
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	for _, needle := range []string{"North", "South", `<c:pt idx="0"><c:v>120</c:v></c:pt>`, `<c:pt idx="1"><c:v>98</c:v></c:pt>`} {
		if containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to drop stale chart data %q, got %s", needle, chartXML)
		}
	}

	workbookBytes := officeZipEntryBytes(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx")
	sheetXML := officeZipEntryText(t, workbookBytes, "xl/worksheets/sheet1.xml")
	for _, needle := range []string{
		`<t xml:space="preserve">East</t>`,
		`<t xml:space="preserve">West</t>`,
		`<t xml:space="preserve">Central</t>`,
		`<v>140</v>`,
		`<v>110</v>`,
		`<v>95</v>`,
	} {
		if !containsSubstring(sheetXML, needle) {
			t.Fatalf("expected embedded sheet1.xml to include %q, got %s", needle, sheetXML)
		}
	}

	validateResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate_template",
		"path":   "decks/chart_update_template.pptx",
	})
	if err != nil {
		t.Fatalf("validate_template failed: %v", err)
	}
	validatePayload := parseNativeDocumentPayload(t, validateResult)
	validation, ok := validatePayload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", validatePayload["validation"])
	}
	for key, want := range map[string]int{
		"slide_count":                    3,
		"chart_count":                    1,
		"chart_rel_count":                1,
		"embedded_workbook_count":        1,
		"referenced_chart_count":         1,
		"referenced_workbook_count":      1,
		"orphan_chart_count":             0,
		"orphan_chart_rel_count":         0,
		"orphan_embedded_workbook_count": 0,
		"missing_chart_target_count":     0,
		"missing_workbook_target_count":  0,
	} {
		if got := asNativeToolInt(t, validation[key]); got != want {
			t.Fatalf("%s = %d, want %d", key, got, want)
		}
	}
	if templateGraphOK, ok := validation["template_graph_ok"].(bool); !ok || !templateGraphOK {
		t.Fatalf("template_graph_ok = %#v, validation=%v", validation["template_graph_ok"], validation)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Chart Update Template", "Original chart slide", "Revenue", "East", "West", "Central"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
	for _, needle := range []string{"North", "South"} {
		if containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to drop stale label %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolTemplateDuplicateThenUpdateChartDataKeepsOriginalChart(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_duplicate_update_template.pptx",
		"title":  "Chart Duplicate Update Template",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original chart slide"},
				"chart": map[string]interface{}{
					"type":       "bar",
					"categories": []interface{}{"North", "South"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 98}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	for _, args := range []map[string]interface{}{
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_duplicate_update_template.pptx",
			"slide":  3,
		},
		{
			"action": "update_chart_data",
			"path":   "decks/chart_duplicate_update_template.pptx",
			"slide":  4,
			"chart": map[string]interface{}{
				"type":       "bar",
				"categories": []interface{}{"East", "West"},
				"series": []interface{}{
					map[string]interface{}{"name": "Revenue", "values": []interface{}{140, 110}},
				},
			},
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	path := filepath.Join(tmpDir, "decks", "chart_duplicate_update_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	slide3Rels := officeZipEntryText(t, data, "ppt/slides/_rels/slide3.xml.rels")
	if !containsSubstring(slide3Rels, `Target="../charts/chart1.xml"`) {
		t.Fatalf("expected slide3 rels to keep chart1 target, got %s", slide3Rels)
	}
	slide4Rels := officeZipEntryText(t, data, "ppt/slides/_rels/slide4.xml.rels")
	if !containsSubstring(slide4Rels, `Target="../charts/chart2.xml"`) {
		t.Fatalf("expected slide4 rels to target chart2, got %s", slide4Rels)
	}

	chart1XML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:pt idx="0"><c:v>North</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>South</c:v></c:pt>`,
		`<c:pt idx="0"><c:v>120</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>98</c:v></c:pt>`,
	} {
		if !containsSubstring(chart1XML, needle) {
			t.Fatalf("expected chart1.xml to preserve original data %q, got %s", needle, chart1XML)
		}
	}
	for _, needle := range []string{"East", "West", `<c:pt idx="0"><c:v>140</c:v></c:pt>`, `<c:pt idx="1"><c:v>110</c:v></c:pt>`} {
		if containsSubstring(chart1XML, needle) {
			t.Fatalf("expected chart1.xml to avoid updated clone data %q, got %s", needle, chart1XML)
		}
	}

	chart2XML := officeZipEntryText(t, data, "ppt/charts/chart2.xml")
	for _, needle := range []string{
		`<c:pt idx="0"><c:v>East</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>West</c:v></c:pt>`,
		`<c:pt idx="0"><c:v>140</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>110</c:v></c:pt>`,
	} {
		if !containsSubstring(chart2XML, needle) {
			t.Fatalf("expected chart2.xml to include updated clone data %q, got %s", needle, chart2XML)
		}
	}

	validateResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate_template",
		"path":   "decks/chart_duplicate_update_template.pptx",
	})
	if err != nil {
		t.Fatalf("validate_template failed: %v", err)
	}
	validatePayload := parseNativeDocumentPayload(t, validateResult)
	validation, ok := validatePayload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", validatePayload["validation"])
	}
	for key, want := range map[string]int{
		"slide_count":                    4,
		"chart_count":                    2,
		"chart_rel_count":                2,
		"embedded_workbook_count":        2,
		"referenced_chart_count":         2,
		"referenced_workbook_count":      2,
		"orphan_chart_count":             0,
		"orphan_chart_rel_count":         0,
		"orphan_embedded_workbook_count": 0,
		"missing_chart_target_count":     0,
		"missing_workbook_target_count":  0,
	} {
		if got := asNativeToolInt(t, validation[key]); got != want {
			t.Fatalf("%s = %d, want %d", key, got, want)
		}
	}
	if templateGraphOK, ok := validation["template_graph_ok"].(bool); !ok || !templateGraphOK {
		t.Fatalf("template_graph_ok = %#v, validation=%v", validation["template_graph_ok"], validation)
	}
}

func TestPPTXToolTemplateUpdateChartDataInfersBarTypeWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_bar.pptx",
		"title":  "Chart Update Infer Bar",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original chart slide"},
				"chart": map[string]interface{}{
					"type":       "bar",
					"categories": []interface{}{"North", "South"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 98}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_bar.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"East", "West"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{140, 110}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_bar.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:barChart>`,
		`<c:cat><c:strRef><c:f>Data!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>East</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>West</c:v></c:pt>`,
		`<c:pt idx="0"><c:v>140</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>110</c:v></c:pt>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `<c:lineChart>`) {
		t.Fatalf("expected inferred chart type to remain bar, got %s", chartXML)
	}
}

func TestPPTXToolTemplateUpdateChartDataInfersDateAxisWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	firstSerial, ok := officeExcelDateSerial("2026-03-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-03-01")
	}
	secondSerial, ok := officeExcelDateSerial("2026-04-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-04-01")
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_date.pptx",
		"title":  "Chart Update Infer Date",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original date chart slide"},
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
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_date.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"2026-03-01", "2026-04-01"},
			"series": []interface{}{
				map[string]interface{}{"name": "Trend", "values": []interface{}{14, 18}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_date.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:lineChart>`,
		`<c:dateAx>`,
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>yyyy-mm-dd</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>`,
		`<c:pt idx="0"><c:v>14</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>18</c:v></c:pt>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `<c:catAx>`) {
		t.Fatalf("expected inferred axis type to remain date, got %s", chartXML)
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesDateAxisFormatWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	firstSerial, ok := officeExcelDateSerial("2026-03-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-03-01")
	}
	secondSerial, ok := officeExcelDateSerial("2026-04-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-04-01")
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_date_format.pptx",
		"title":  "Chart Update Infer Date Format",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original date chart slide"},
				"chart": map[string]interface{}{
					"type":          "line",
					"x_axis_type":   "date",
					"x_axis_format": "[$-409]mmm-yy",
					"categories":    []interface{}{"2026-01-01", "2026-02-01"},
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
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_date_format.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"2026-03-01", "2026-04-01"},
			"series": []interface{}{
				map[string]interface{}{"name": "Trend", "values": []interface{}{14, 18}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_date_format.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:lineChart>`,
		`<c:dateAx>`,
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>[$-409]mmm-yy</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>`,
		`<c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>yyyy-mm-dd</c:formatCode><c:ptCount val="2"/>`) {
		t.Fatalf("expected inferred date-axis format to avoid default fallback, got %s", chartXML)
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesSecondaryDateAxisFormatWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_secondary_date_format.pptx",
		"title":  "Chart Update Infer Secondary Date Format",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original combo date chart slide"},
				"chart": map[string]interface{}{
					"type":           "combo",
					"x_axis_type":    "date",
					"x_axis_format":  "m/d/yyyy",
					"x2_axis_format": "[$-409]mmm-yy",
					"categories":     []interface{}{"2026-01-01", "2026-02-01", "2026-03-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_secondary_date_format.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"2026-04-01", "2026-05-01", "2026-06-01"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{35, 37, 39}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_secondary_date_format.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$4</c:f><c:numCache><c:formatCode>m/d/yyyy</c:formatCode><c:ptCount val="3"/>`,
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$4</c:f><c:numCache><c:formatCode>[$-409]mmm-yy</c:formatCode><c:ptCount val="3"/>`,
		`<c:numFmt formatCode="m/d/yyyy" sourceLinked="0"/>`,
		`<c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	if strings.Count(chartXML, `<c:numFmt formatCode="yyyy-mm-dd" sourceLinked="0"/>`) != 0 {
		t.Fatalf("expected secondary inferred date-axis format to avoid default fallback, got %s", chartXML)
	}
}

func TestPPTXToolTemplateUpdateChartDataInfersComboSeriesTypeAndAxisWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_combo_series_defaults.pptx",
		"title":  "Chart Update Infer Combo Series Defaults",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original combo chart slide"},
				"chart": map[string]interface{}{
					"type":       "combo",
					"categories": []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Cost", "type": "bar", "values": []interface{}{88, 94, 99}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_combo_series_defaults.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q4", "Q5", "Q6"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Cost", "values": []interface{}{102, 108, 112}},
				map[string]interface{}{"name": "Margin", "values": []interface{}{35, 37, 39}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_combo_series_defaults.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	barStart := strings.Index(chartXML, `<c:barChart>`)
	if barStart < 0 {
		t.Fatalf("expected chart1.xml to include a barChart block, got %s", chartXML)
	}
	barEnd := strings.Index(chartXML[barStart:], `</c:barChart>`)
	if barEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing barChart block, got %s", chartXML)
	}
	barBlock := chartXML[barStart : barStart+barEnd+len(`</c:barChart>`)]

	lineStart := strings.Index(chartXML, `<c:lineChart>`)
	if lineStart < 0 {
		t.Fatalf("expected chart1.xml to include a lineChart block, got %s", chartXML)
	}
	lineEnd := strings.Index(chartXML[lineStart:], `</c:lineChart>`)
	if lineEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing lineChart block, got %s", chartXML)
	}
	lineBlock := chartXML[lineStart : lineStart+lineEnd+len(`</c:lineChart>`)]

	for _, needle := range []string{
		`<c:tx><c:v>Revenue</c:v></c:tx>`,
		`<c:tx><c:v>Cost</c:v></c:tx>`,
	} {
		if !containsSubstring(barBlock, needle) {
			t.Fatalf("expected barChart block to include %q, got %s", needle, barBlock)
		}
	}
	if containsSubstring(barBlock, `<c:tx><c:v>Margin</c:v></c:tx>`) {
		t.Fatalf("expected barChart block to avoid secondary line series, got %s", barBlock)
	}
	if !containsSubstring(lineBlock, `<c:tx><c:v>Margin</c:v></c:tx>`) {
		t.Fatalf("expected lineChart block to include Margin, got %s", lineBlock)
	}
	for _, needle := range []string{
		`<c:tx><c:v>Revenue</c:v></c:tx>`,
		`<c:tx><c:v>Cost</c:v></c:tx>`,
	} {
		if containsSubstring(lineBlock, needle) {
			t.Fatalf("expected lineChart block to avoid inferred bar series %q, got %s", needle, lineBlock)
		}
	}
	if !containsSubstring(chartXML, `<c:axPos val="r"/>`) {
		t.Fatalf("expected inferred secondary axis to remain rendered, got %s", chartXML)
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesValueAxisFormatWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_value_axis_format.pptx",
		"title":  "Chart Update Infer Value Axis Format",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original chart slide"},
				"chart": map[string]interface{}{
					"type":          "bar",
					"y_axis_format": "$#,##0",
					"categories":    []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132, 140}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_value_axis_format.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q4", "Q5", "Q6"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{150, 165, 172}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_value_axis_format.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:numFmt formatCode="$#,##0" sourceLinked="0"/>`) {
		t.Fatalf("expected chart1.xml to preserve primary value-axis format, got %s", chartXML)
	}
	valueAxisStart := strings.Index(chartXML, `<c:valAx>`)
	if valueAxisStart < 0 {
		t.Fatalf("expected chart1.xml to include a value-axis block, got %s", chartXML)
	}
	valueAxisEnd := strings.Index(chartXML[valueAxisStart:], `</c:valAx>`)
	if valueAxisEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing value-axis block, got %s", chartXML)
	}
	valueAxisBlock := chartXML[valueAxisStart : valueAxisStart+valueAxisEnd+len(`</c:valAx>`)]
	if containsSubstring(valueAxisBlock, `<c:numFmt formatCode="General" sourceLinked="1"/>`) {
		t.Fatalf("expected inferred primary value-axis format to avoid General fallback, got %s", valueAxisBlock)
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesSecondaryValueAxisFormatWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_secondary_value_axis_format.pptx",
		"title":  "Chart Update Infer Secondary Value Axis Format",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original combo chart slide"},
				"chart": map[string]interface{}{
					"type":           "combo",
					"y_axis_format":  "$#,##0",
					"y2_axis_format": "0.0%",
					"categories":     []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_secondary_value_axis_format.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q4", "Q5", "Q6"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{35, 37, 39}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_secondary_value_axis_format.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:numFmt formatCode="$#,##0" sourceLinked="0"/>`,
		`<c:numFmt formatCode="0.0%" sourceLinked="0"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	searchFrom := 0
	valueAxisBlocks := make([]string, 0, 2)
	for {
		valueAxisStart := strings.Index(chartXML[searchFrom:], `<c:valAx>`)
		if valueAxisStart < 0 {
			break
		}
		valueAxisStart += searchFrom
		valueAxisEnd := strings.Index(chartXML[valueAxisStart:], `</c:valAx>`)
		if valueAxisEnd < 0 {
			t.Fatalf("expected chart1.xml to include a closing value-axis block, got %s", chartXML)
		}
		valueAxisEnd += valueAxisStart + len(`</c:valAx>`)
		valueAxisBlocks = append(valueAxisBlocks, chartXML[valueAxisStart:valueAxisEnd])
		searchFrom = valueAxisEnd
	}
	for _, block := range valueAxisBlocks {
		if containsSubstring(block, `<c:numFmt formatCode="General" sourceLinked="1"/>`) {
			t.Fatalf("expected inferred value-axis formats to avoid General fallback in %s", block)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesChartPresentationMetadataWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_presentation_metadata.pptx",
		"title":  "Chart Update Infer Presentation Metadata",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original combo chart slide"},
				"chart": map[string]interface{}{
					"type":            "combo",
					"title":           "Revenue vs Margin",
					"show_legend":     true,
					"legend_position": "top",
					"x_axis_title":    "Quarter",
					"x2_axis_title":   "Quarter (Top)",
					"y_axis_title":    "Revenue ($M)",
					"y2_axis_title":   "Margin %",
					"categories":      []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_presentation_metadata.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q4", "Q5", "Q6"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{35, 37, 39}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_presentation_metadata.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<a:t>Revenue vs Margin</a:t>`,
		`<c:legend><c:legendPos val="t"/><c:layout/></c:legend>`,
		`<a:t>Quarter</a:t>`,
		`<a:t>Quarter (Top)</a:t>`,
		`<a:t>Revenue ($M)</a:t>`,
		`<a:t>Margin %</a:t>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesHiddenLegendWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_hidden_legend.pptx",
		"title":  "Chart Update Infer Hidden Legend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original multi-series chart slide"},
				"chart": map[string]interface{}{
					"type":        "bar",
					"show_legend": false,
					"categories":  []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Target", "values": []interface{}{140, 145, 150}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_hidden_legend.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q4", "Q5", "Q6"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Target", "values": []interface{}{155, 170, 180}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_hidden_legend.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if containsSubstring(chartXML, `<c:legend>`) {
		t.Fatalf("expected chart1.xml to preserve hidden legend state, got %s", chartXML)
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesChartDataLabelsWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_data_labels.pptx",
		"title":  "Chart Update Infer Data Labels",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Original pie chart slide"},
				"chart": map[string]interface{}{
					"type":              "pie",
					"labels":            true,
					"label_position":    "best_fit",
					"label_format":      "0.0%",
					"label_separator":   " / ",
					"show_leader_lines": true,
					"show_value":        false,
					"show_category":     false,
					"show_series_name":  true,
					"show_percent":      true,
					"show_legend_key":   true,
					"show_bubble_size":  true,
					"categories":        []interface{}{"Won", "Lost", "Open"},
					"series": []interface{}{
						map[string]interface{}{"name": "Pipeline", "values": []interface{}{55, 30, 15}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_data_labels.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Won", "Lost", "Open"},
			"series": []interface{}{
				map[string]interface{}{"name": "Pipeline", "values": []interface{}{58, 27, 15}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_data_labels.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	dLblsStart := strings.Index(chartXML, `<c:dLbls>`)
	if dLblsStart < 0 {
		t.Fatalf("expected chart1.xml to include a data-labels block, got %s", chartXML)
	}
	dLblsEnd := strings.Index(chartXML[dLblsStart:], `</c:dLbls>`)
	if dLblsEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing data-labels block, got %s", chartXML)
	}
	dLblsBlock := chartXML[dLblsStart : dLblsStart+dLblsEnd+len(`</c:dLbls>`)]
	for _, needle := range []string{
		`<c:dLblPos val="bestFit"/>`,
		`<c:numFmt formatCode="0.0%" sourceLinked="0"/>`,
		`<c:separator>/</c:separator>`,
		`<c:showLeaderLines val="1"/>`,
		`<c:showLegendKey val="1"/>`,
		`<c:showVal val="0"/>`,
		`<c:showCatName val="0"/>`,
		`<c:showSerName val="1"/>`,
		`<c:showPercent val="1"/>`,
		`<c:showBubbleSize val="1"/>`,
	} {
		if !containsSubstring(dLblsBlock, needle) {
			t.Fatalf("expected data-labels block to include %q, got %s", needle, dLblsBlock)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesBarChartAppearanceWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_bar_appearance.pptx",
		"title":  "Chart Update Infer Bar Appearance",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original bar chart slide"},
				"chart": map[string]interface{}{
					"type":        "bar",
					"vary_colors": true,
					"gap_width":   60,
					"overlap":     -20,
					"categories":  []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Target", "values": []interface{}{140, 145, 150}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_bar_appearance.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q2", "Q3", "Q4"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Target", "values": []interface{}{155, 170, 180}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_bar_appearance.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	barChartStart := strings.Index(chartXML, `<c:barChart>`)
	if barChartStart < 0 {
		t.Fatalf("expected chart1.xml to include a bar-chart block, got %s", chartXML)
	}
	barChartEnd := strings.Index(chartXML[barChartStart:], `</c:barChart>`)
	if barChartEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing bar-chart block, got %s", chartXML)
	}
	barChartBlock := chartXML[barChartStart : barChartStart+barChartEnd+len(`</c:barChart>`)]
	for _, needle := range []string{
		`<c:varyColors val="1"/>`,
		`<c:gapWidth val="60"/>`,
		`<c:overlap val="-20"/>`,
	} {
		if !containsSubstring(barChartBlock, needle) {
			t.Fatalf("expected bar-chart block to include %q, got %s", needle, barChartBlock)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesLineChartAppearanceWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_line_appearance.pptx",
		"title":  "Chart Update Infer Line Appearance",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Trend",
				"paragraphs": []interface{}{"Original line chart slide"},
				"chart": map[string]interface{}{
					"type":        "line",
					"vary_colors": true,
					"smooth":      true,
					"categories":  []interface{}{"Jan", "Feb", "Mar"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12, 15}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_line_appearance.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Feb", "Mar", "Apr"},
			"series": []interface{}{
				map[string]interface{}{"name": "Trend", "values": []interface{}{12, 15, 17}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_line_appearance.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	lineChartStart := strings.Index(chartXML, `<c:lineChart>`)
	if lineChartStart < 0 {
		t.Fatalf("expected chart1.xml to include a line-chart block, got %s", chartXML)
	}
	lineChartEnd := strings.Index(chartXML[lineChartStart:], `</c:lineChart>`)
	if lineChartEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing line-chart block, got %s", chartXML)
	}
	lineChartBlock := chartXML[lineChartStart : lineChartStart+lineChartEnd+len(`</c:lineChart>`)]
	for _, needle := range []string{
		`<c:varyColors val="1"/>`,
		`<c:smooth val="1"/>`,
	} {
		if !containsSubstring(lineChartBlock, needle) {
			t.Fatalf("expected line-chart block to include %q, got %s", needle, lineChartBlock)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesCircularChartAppearanceWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_circular_appearance.pptx",
		"title":  "Chart Update Infer Circular Appearance",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Original donut chart slide"},
				"chart": map[string]interface{}{
					"type":        "donut",
					"vary_colors": false,
					"start_angle": 120,
					"hole_size":   64,
					"categories":  []interface{}{"Won", "Lost", "Open"},
					"series": []interface{}{
						map[string]interface{}{"name": "Pipeline", "values": []interface{}{55, 30, 15}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_circular_appearance.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Won", "Lost", "Open"},
			"series": []interface{}{
				map[string]interface{}{"name": "Pipeline", "values": []interface{}{58, 27, 15}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_circular_appearance.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	donutChartStart := strings.Index(chartXML, `<c:doughnutChart>`)
	if donutChartStart < 0 {
		t.Fatalf("expected chart1.xml to include a donut-chart block, got %s", chartXML)
	}
	donutChartEnd := strings.Index(chartXML[donutChartStart:], `</c:doughnutChart>`)
	if donutChartEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing donut-chart block, got %s", chartXML)
	}
	donutChartBlock := chartXML[donutChartStart : donutChartStart+donutChartEnd+len(`</c:doughnutChart>`)]
	for _, needle := range []string{
		`<c:varyColors val="0"/>`,
		`<c:firstSliceAng val="120"/>`,
		`<c:holeSize val="64"/>`,
	} {
		if !containsSubstring(donutChartBlock, needle) {
			t.Fatalf("expected donut-chart block to include %q, got %s", needle, donutChartBlock)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesValueAxisBoundsAndUnitsWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_value_axis_scale.pptx",
		"title":  "Chart Update Infer Value Axis Scale",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original chart slide"},
				"chart": map[string]interface{}{
					"type":              "bar",
					"y_axis_min":        0,
					"y_axis_max":        200,
					"y_axis_major_unit": 50,
					"y_axis_minor_unit": 10,
					"categories":        []interface{}{"Q1", "Q2", "Q3"},
					"series":            []interface{}{map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132, 140}}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_value_axis_scale.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q4", "Q5", "Q6"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{150, 165, 172}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_value_axis_scale.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	valueAxisStart := strings.Index(chartXML, `<c:valAx>`)
	if valueAxisStart < 0 {
		t.Fatalf("expected chart1.xml to include a value-axis block, got %s", chartXML)
	}
	valueAxisEnd := strings.Index(chartXML[valueAxisStart:], `</c:valAx>`)
	if valueAxisEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing value-axis block, got %s", chartXML)
	}
	valueAxisBlock := chartXML[valueAxisStart : valueAxisStart+valueAxisEnd+len(`</c:valAx>`)]
	for _, needle := range []string{
		`<c:min val="0"/>`,
		`<c:max val="200"/>`,
		`<c:majorUnit val="50"/>`,
		`<c:minorUnit val="10"/>`,
	} {
		if !containsSubstring(valueAxisBlock, needle) {
			t.Fatalf("expected primary value-axis block to include %q, got %s", needle, valueAxisBlock)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesSecondaryValueAxisBoundsAndUnitsWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_secondary_value_axis_scale.pptx",
		"title":  "Chart Update Infer Secondary Value Axis Scale",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original combo chart slide"},
				"chart": map[string]interface{}{
					"type":               "combo",
					"y_axis_min":         0,
					"y_axis_max":         200,
					"y_axis_major_unit":  50,
					"y_axis_minor_unit":  10,
					"y2_axis_min":        0,
					"y2_axis_max":        100,
					"y2_axis_major_unit": 20,
					"y2_axis_minor_unit": 5,
					"categories":         []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_secondary_value_axis_scale.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q4", "Q5", "Q6"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{35, 37, 39}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_secondary_value_axis_scale.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	valueAxisBlocks := make([]string, 0, 2)
	searchFrom := 0
	for {
		valueAxisStart := strings.Index(chartXML[searchFrom:], `<c:valAx>`)
		if valueAxisStart < 0 {
			break
		}
		valueAxisStart += searchFrom
		valueAxisEnd := strings.Index(chartXML[valueAxisStart:], `</c:valAx>`)
		if valueAxisEnd < 0 {
			t.Fatalf("expected chart1.xml to include a closing value-axis block, got %s", chartXML)
		}
		valueAxisEnd += valueAxisStart + len(`</c:valAx>`)
		valueAxisBlocks = append(valueAxisBlocks, chartXML[valueAxisStart:valueAxisEnd])
		searchFrom = valueAxisEnd
	}
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected chart1.xml to include 2 value-axis blocks, got %d in %s", len(valueAxisBlocks), chartXML)
	}
	for _, needle := range []string{
		`<c:axPos val="l"/>`,
		`<c:min val="0"/>`,
		`<c:max val="200"/>`,
		`<c:majorUnit val="50"/>`,
		`<c:minorUnit val="10"/>`,
	} {
		if !containsSubstring(valueAxisBlocks[0], needle) {
			t.Fatalf("expected primary value-axis block to include %q, got %s", needle, valueAxisBlocks[0])
		}
	}
	for _, needle := range []string{
		`<c:axPos val="r"/>`,
		`<c:min val="0"/>`,
		`<c:max val="100"/>`,
		`<c:majorUnit val="20"/>`,
		`<c:minorUnit val="5"/>`,
	} {
		if !containsSubstring(valueAxisBlocks[1], needle) {
			t.Fatalf("expected secondary value-axis block to include %q, got %s", needle, valueAxisBlocks[1])
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesDateAxisBoundsAndUnitsWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	minSerial, ok := officeExcelDateSerial("2025-12-01", false)
	if !ok {
		t.Fatal("expected date serial for 2025-12-01")
	}
	maxSerial, ok := officeExcelDateSerial("2028-12-31", false)
	if !ok {
		t.Fatal("expected date serial for 2028-12-31")
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_date_axis_scale.pptx",
		"title":  "Chart Update Infer Date Axis Scale",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original date chart slide"},
				"chart": map[string]interface{}{
					"type":                   "line",
					"x_axis_type":            "date",
					"x_axis_min":             "2025-12-01",
					"x_axis_max":             "2028-12-31",
					"x_axis_base_time_unit":  "years",
					"x_axis_major_unit":      12,
					"x_axis_minor_unit":      1,
					"x_axis_major_time_unit": "years",
					"x_axis_minor_time_unit": "months",
					"categories":             []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132, 140}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_date_axis_scale.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"2026-02-01", "2027-02-01", "2028-02-01"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{150, 165, 172}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_date_axis_scale.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	dateAxisStart := strings.Index(chartXML, `<c:dateAx>`)
	if dateAxisStart < 0 {
		t.Fatalf("expected chart1.xml to include a date-axis block, got %s", chartXML)
	}
	dateAxisEnd := strings.Index(chartXML[dateAxisStart:], `</c:dateAx>`)
	if dateAxisEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing date-axis block, got %s", chartXML)
	}
	dateAxisBlock := chartXML[dateAxisStart : dateAxisStart+dateAxisEnd+len(`</c:dateAx>`)]
	for _, needle := range []string{
		`<c:min val="` + minSerial + `"/>`,
		`<c:max val="` + maxSerial + `"/>`,
		`<c:baseTimeUnit val="years"/>`,
		`<c:majorUnit val="12"/>`,
		`<c:minorUnit val="1"/>`,
		`<c:majorTimeUnit val="years"/>`,
		`<c:minorTimeUnit val="months"/>`,
	} {
		if !containsSubstring(dateAxisBlock, needle) {
			t.Fatalf("expected primary date-axis block to include %q, got %s", needle, dateAxisBlock)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesComboDateAxisBoundsAndUnitsWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	minSerial, ok := officeExcelDateSerial("2025-12-01", false)
	if !ok {
		t.Fatal("expected date serial for 2025-12-01")
	}
	maxSerial, ok := officeExcelDateSerial("2028-12-31", false)
	if !ok {
		t.Fatal("expected date serial for 2028-12-31")
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_combo_date_axis_scale.pptx",
		"title":  "Chart Update Infer Combo Date Axis Scale",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original combo date chart slide"},
				"chart": map[string]interface{}{
					"type":                   "combo",
					"x_axis_type":            "date",
					"x_axis_min":             "2025-12-01",
					"x_axis_max":             "2028-12-31",
					"x_axis_base_time_unit":  "years",
					"x_axis_major_unit":      12,
					"x_axis_minor_unit":      1,
					"x_axis_major_time_unit": "years",
					"x_axis_minor_time_unit": "months",
					"categories":             []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_combo_date_axis_scale.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"2026-02-01", "2027-02-01", "2028-02-01"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{35, 37, 39}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_combo_date_axis_scale.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if strings.Count(chartXML, `<c:min val="`+minSerial+`"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 date-axis min bounds, got %s", chartXML)
	}
	if strings.Count(chartXML, `<c:max val="`+maxSerial+`"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 date-axis max bounds, got %s", chartXML)
	}
	for _, needle := range []string{
		`<c:baseTimeUnit val="years"/>`,
		`<c:majorUnit val="12"/>`,
		`<c:minorUnit val="1"/>`,
		`<c:majorTimeUnit val="years"/>`,
		`<c:minorTimeUnit val="months"/>`,
	} {
		if strings.Count(chartXML, needle) != 2 {
			t.Fatalf("expected chart1.xml to include 2 copies of %q, got %s", needle, chartXML)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesDateAxisBehaviorWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_date_axis_behavior.pptx",
		"title":  "Chart Update Infer Date Axis Behavior",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original date chart slide"},
				"chart": map[string]interface{}{
					"type":                   "line",
					"x_axis_type":            "date",
					"x_axis_label_position":  "high",
					"x_axis_label_offset":    250,
					"x_axis_reverse_order":   true,
					"x_axis_crosses":         "max",
					"x_axis_major_tick_mark": "cross",
					"x_axis_minor_tick_mark": "in",
					"x_axis_visible":         false,
					"x_axis_auto":            false,
					"categories":             []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132, 140}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_date_axis_behavior.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"2026-02-01", "2027-02-01", "2028-02-01"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "values": []interface{}{150, 165, 172}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_date_axis_behavior.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	dateAxisStart := strings.Index(chartXML, `<c:dateAx>`)
	if dateAxisStart < 0 {
		t.Fatalf("expected chart1.xml to include a date-axis block, got %s", chartXML)
	}
	dateAxisEnd := strings.Index(chartXML[dateAxisStart:], `</c:dateAx>`)
	if dateAxisEnd < 0 {
		t.Fatalf("expected chart1.xml to include a closing date-axis block, got %s", chartXML)
	}
	dateAxisBlock := chartXML[dateAxisStart : dateAxisStart+dateAxisEnd+len(`</c:dateAx>`)]
	for _, needle := range []string{
		`<c:orientation val="maxMin"/>`,
		`<c:delete val="1"/>`,
		`<c:majorTickMark val="cross"/>`,
		`<c:minorTickMark val="in"/>`,
		`<c:lblOffset val="250"/>`,
		`<c:tickLblPos val="high"/>`,
		`<c:crosses val="max"/>`,
		`<c:auto val="0"/>`,
	} {
		if !containsSubstring(dateAxisBlock, needle) {
			t.Fatalf("expected primary date-axis block to include %q, got %s", needle, dateAxisBlock)
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesCategoryAxisBehaviorWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_category_axis_behavior.pptx",
		"title":  "Chart Update Infer Category Axis Behavior",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original combo chart slide"},
				"chart": map[string]interface{}{
					"type":                       "combo",
					"x_axis_label_position":      "high",
					"x2_axis_label_position":     "low",
					"x_axis_label_alignment":     "left",
					"x_axis_label_offset":        250,
					"x_axis_reverse_order":       true,
					"x2_axis_reverse_order":      true,
					"x_axis_crosses":             "max",
					"x2_axis_crosses":            "min",
					"x_axis_major_tick_mark":     "cross",
					"x_axis_minor_tick_mark":     "in",
					"x_axis_multi_level_labels":  false,
					"x2_axis_label_alignment":    "right",
					"x2_axis_label_offset":       500,
					"x2_axis_major_tick_mark":    "out",
					"x2_axis_minor_tick_mark":    "cross",
					"x2_axis_multi_level_labels": false,
					"x_axis_visible":             false,
					"x2_axis_visible":            true,
					"x_axis_auto":                false,
					"x2_axis_auto":               false,
					"categories":                 []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_category_axis_behavior.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q2", "Q3", "Q4"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{35, 37, 39}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_category_axis_behavior.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	categoryAxisBlocks := make([]string, 0, 2)
	searchFrom := 0
	for searchFrom < len(chartXML) {
		categoryAxisStart := strings.Index(chartXML[searchFrom:], `<c:catAx>`)
		if categoryAxisStart < 0 {
			break
		}
		categoryAxisStart += searchFrom
		categoryAxisEnd := strings.Index(chartXML[categoryAxisStart:], `</c:catAx>`)
		if categoryAxisEnd < 0 {
			t.Fatalf("expected chart1.xml to include a closing category-axis block, got %s", chartXML)
		}
		categoryAxisEnd += categoryAxisStart + len(`</c:catAx>`)
		categoryAxisBlocks = append(categoryAxisBlocks, chartXML[categoryAxisStart:categoryAxisEnd])
		searchFrom = categoryAxisEnd
	}
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected chart1.xml to include 2 category-axis blocks, got %d in %s", len(categoryAxisBlocks), chartXML)
	}

	for _, needle := range []string{
		`<c:orientation val="maxMin"/>`,
		`<c:delete val="1"/>`,
		`<c:majorTickMark val="cross"/>`,
		`<c:minorTickMark val="in"/>`,
		`<c:lblAlgn val="l"/>`,
		`<c:lblOffset val="250"/>`,
		`<c:noMultiLvlLbl val="1"/>`,
		`<c:tickLblPos val="high"/>`,
		`<c:crosses val="max"/>`,
		`<c:auto val="0"/>`,
	} {
		if !containsSubstring(categoryAxisBlocks[0], needle) {
			t.Fatalf("expected primary category-axis block to include %q, got %s", needle, categoryAxisBlocks[0])
		}
	}
	for _, needle := range []string{
		`<c:orientation val="maxMin"/>`,
		`<c:delete val="0"/>`,
		`<c:majorTickMark val="out"/>`,
		`<c:minorTickMark val="cross"/>`,
		`<c:lblAlgn val="r"/>`,
		`<c:lblOffset val="500"/>`,
		`<c:noMultiLvlLbl val="1"/>`,
		`<c:tickLblPos val="low"/>`,
		`<c:crosses val="min"/>`,
		`<c:auto val="0"/>`,
	} {
		if !containsSubstring(categoryAxisBlocks[1], needle) {
			t.Fatalf("expected secondary category-axis block to include %q, got %s", needle, categoryAxisBlocks[1])
		}
	}
}

func TestPPTXToolTemplateUpdateChartDataPreservesValueAxisBehaviorWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_update_infer_value_axis_behavior.pptx",
		"title":  "Chart Update Infer Value Axis Behavior",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Original combo chart slide"},
				"chart": map[string]interface{}{
					"type":                    "combo",
					"y_axis_label_position":   "high",
					"y2_axis_label_position":  "low",
					"y_axis_reverse_order":    true,
					"y2_axis_reverse_order":   true,
					"y_axis_major_gridlines":  false,
					"y_axis_minor_gridlines":  true,
					"y2_axis_major_gridlines": true,
					"y2_axis_minor_gridlines": true,
					"y_axis_crosses":          "max",
					"y_axis_cross_between":    "mid_cat",
					"y2_axis_crosses":         "min",
					"y2_axis_cross_between":   "mid_cat",
					"y_axis_major_tick_mark":  "none",
					"y_axis_minor_tick_mark":  "none",
					"y2_axis_major_tick_mark": "cross",
					"y2_axis_minor_tick_mark": "in",
					"categories":              []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_chart_data",
		"path":   "decks/chart_update_infer_value_axis_behavior.pptx",
		"slide":  3,
		"chart": map[string]interface{}{
			"categories": []interface{}{"Q2", "Q3", "Q4"},
			"series": []interface{}{
				map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{150, 165, 172}},
				map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{35, 37, 39}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update_chart_data failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "chart_update_infer_value_axis_behavior.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	valueAxisBlocks := make([]string, 0, 2)
	searchFrom := 0
	for searchFrom < len(chartXML) {
		valueAxisStart := strings.Index(chartXML[searchFrom:], `<c:valAx>`)
		if valueAxisStart < 0 {
			break
		}
		valueAxisStart += searchFrom
		valueAxisEnd := strings.Index(chartXML[valueAxisStart:], `</c:valAx>`)
		if valueAxisEnd < 0 {
			t.Fatalf("expected chart1.xml to include a closing value-axis block, got %s", chartXML)
		}
		valueAxisEnd += valueAxisStart + len(`</c:valAx>`)
		valueAxisBlocks = append(valueAxisBlocks, chartXML[valueAxisStart:valueAxisEnd])
		searchFrom = valueAxisEnd
	}
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected chart1.xml to include 2 value-axis blocks, got %d in %s", len(valueAxisBlocks), chartXML)
	}

	for _, needle := range []string{
		`<c:orientation val="maxMin"/>`,
		`<c:majorTickMark val="none"/>`,
		`<c:minorTickMark val="none"/>`,
		`<c:minorGridlines/>`,
		`<c:tickLblPos val="high"/>`,
		`<c:crosses val="max"/>`,
		`<c:crossBetween val="midCat"/>`,
	} {
		if !containsSubstring(valueAxisBlocks[0], needle) {
			t.Fatalf("expected primary value-axis block to include %q, got %s", needle, valueAxisBlocks[0])
		}
	}
	if containsSubstring(valueAxisBlocks[0], `<c:majorGridlines/>`) {
		t.Fatalf("expected primary value-axis block to preserve hidden major gridlines, got %s", valueAxisBlocks[0])
	}
	for _, needle := range []string{
		`<c:orientation val="maxMin"/>`,
		`<c:majorGridlines/>`,
		`<c:minorGridlines/>`,
		`<c:majorTickMark val="cross"/>`,
		`<c:minorTickMark val="in"/>`,
		`<c:tickLblPos val="low"/>`,
		`<c:crosses val="min"/>`,
		`<c:crossBetween val="midCat"/>`,
	} {
		if !containsSubstring(valueAxisBlocks[1], needle) {
			t.Fatalf("expected secondary value-axis block to include %q, got %s", needle, valueAxisBlocks[1])
		}
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

func TestPPTXToolTemplateDuplicateClonedChartSlideCreatesThirdChartPackage(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_duplicate_twice_template.pptx",
		"title":  "Chart Duplicate Twice Template",
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

	for _, args := range []map[string]interface{}{
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_duplicate_twice_template.pptx",
			"slide":  3,
		},
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_duplicate_twice_template.pptx",
			"slide":  4,
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	path := filepath.Join(tmpDir, "decks", "chart_duplicate_twice_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, entry := range []string{
		"ppt/charts/chart1.xml",
		"ppt/charts/chart2.xml",
		"ppt/charts/chart3.xml",
		"ppt/charts/_rels/chart1.xml.rels",
		"ppt/charts/_rels/chart2.xml.rels",
		"ppt/charts/_rels/chart3.xml.rels",
		"ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx",
		"ppt/embeddings/Microsoft_Excel_Worksheet2.xlsx",
		"ppt/embeddings/Microsoft_Excel_Worksheet3.xlsx",
	} {
		if !officeZipHasEntry(t, data, entry) {
			t.Fatalf("expected %s to exist after duplicating cloned chart slide", entry)
		}
	}
	for path, wantTarget := range map[string]string{
		"ppt/slides/_rels/slide3.xml.rels": `Target="../charts/chart1.xml"`,
		"ppt/slides/_rels/slide4.xml.rels": `Target="../charts/chart2.xml"`,
		"ppt/slides/_rels/slide5.xml.rels": `Target="../charts/chart3.xml"`,
		"ppt/charts/_rels/chart2.xml.rels": `Target="../embeddings/Microsoft_Excel_Worksheet2.xlsx"`,
		"ppt/charts/_rels/chart3.xml.rels": `Target="../embeddings/Microsoft_Excel_Worksheet3.xlsx"`,
	} {
		entryText := officeZipEntryText(t, data, path)
		if !containsSubstring(entryText, wantTarget) {
			t.Fatalf("expected %s to include %q, got %s", path, wantTarget, entryText)
		}
	}
	contentTypes := officeZipEntryText(t, data, "[Content_Types].xml")
	for _, needle := range []string{
		`<Override PartName="/ppt/charts/chart1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Override PartName="/ppt/charts/chart2.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Override PartName="/ppt/charts/chart3.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>`,
	} {
		if !containsSubstring(contentTypes, needle) {
			t.Fatalf("expected [Content_Types].xml to include %q, got %s", needle, contentTypes)
		}
	}

	validateResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate_template",
		"path":   "decks/chart_duplicate_twice_template.pptx",
	})
	if err != nil {
		t.Fatalf("validate_template failed: %v", err)
	}
	validatePayload := parseNativeDocumentPayload(t, validateResult)
	validation, ok := validatePayload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", validatePayload["validation"])
	}
	for key, want := range map[string]int{
		"slide_count":                    5,
		"chart_count":                    3,
		"chart_rel_count":                3,
		"embedded_workbook_count":        3,
		"referenced_chart_count":         3,
		"referenced_workbook_count":      3,
		"orphan_chart_count":             0,
		"orphan_chart_rel_count":         0,
		"orphan_embedded_workbook_count": 0,
		"missing_chart_target_count":     0,
		"missing_workbook_target_count":  0,
	} {
		if got := asNativeToolInt(t, validation[key]); got != want {
			t.Fatalf("%s = %d, want %d", key, got, want)
		}
	}
	templateGraphOK, ok := validation["template_graph_ok"].(bool)
	if !ok {
		t.Fatalf("template_graph_ok type = %T, want bool", validation["template_graph_ok"])
	}
	if !templateGraphOK {
		t.Fatalf("template_graph_ok = false, validation=%v", validation)
	}
}

func TestPPTXToolTemplateDuplicateReorderedChartSlideKeepsSourceBindings(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_duplicate_after_reorder_template.pptx",
		"title":  "Chart Duplicate After Reorder Template",
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

	for _, args := range []map[string]interface{}{
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_duplicate_after_reorder_template.pptx",
			"slide":  3,
		},
		{
			"action": "reorder_slides",
			"path":   "decks/chart_duplicate_after_reorder_template.pptx",
			"order":  []interface{}{1, 2, 4, 3},
		},
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_duplicate_after_reorder_template.pptx",
			"slide":  3,
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	path := filepath.Join(tmpDir, "decks", "chart_duplicate_after_reorder_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, entry := range []string{
		"ppt/charts/chart1.xml",
		"ppt/charts/chart2.xml",
		"ppt/charts/chart3.xml",
		"ppt/charts/_rels/chart1.xml.rels",
		"ppt/charts/_rels/chart2.xml.rels",
		"ppt/charts/_rels/chart3.xml.rels",
		"ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx",
		"ppt/embeddings/Microsoft_Excel_Worksheet2.xlsx",
		"ppt/embeddings/Microsoft_Excel_Worksheet3.xlsx",
	} {
		if !officeZipHasEntry(t, data, entry) {
			t.Fatalf("expected %s to exist after duplicating reordered chart slide", entry)
		}
	}
	for path, wantTarget := range map[string]string{
		"ppt/slides/_rels/slide3.xml.rels": `Target="../charts/chart2.xml"`,
		"ppt/slides/_rels/slide4.xml.rels": `Target="../charts/chart3.xml"`,
		"ppt/slides/_rels/slide5.xml.rels": `Target="../charts/chart1.xml"`,
		"ppt/charts/_rels/chart2.xml.rels": `Target="../embeddings/Microsoft_Excel_Worksheet2.xlsx"`,
		"ppt/charts/_rels/chart3.xml.rels": `Target="../embeddings/Microsoft_Excel_Worksheet3.xlsx"`,
	} {
		entryText := officeZipEntryText(t, data, path)
		if !containsSubstring(entryText, wantTarget) {
			t.Fatalf("expected %s to include %q, got %s", path, wantTarget, entryText)
		}
	}

	validateResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate_template",
		"path":   "decks/chart_duplicate_after_reorder_template.pptx",
	})
	if err != nil {
		t.Fatalf("validate_template failed: %v", err)
	}
	validatePayload := parseNativeDocumentPayload(t, validateResult)
	validation, ok := validatePayload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", validatePayload["validation"])
	}
	for key, want := range map[string]int{
		"slide_count":                    5,
		"chart_count":                    3,
		"chart_rel_count":                3,
		"embedded_workbook_count":        3,
		"referenced_chart_count":         3,
		"referenced_workbook_count":      3,
		"orphan_chart_count":             0,
		"orphan_chart_rel_count":         0,
		"orphan_embedded_workbook_count": 0,
		"missing_chart_target_count":     0,
		"missing_workbook_target_count":  0,
	} {
		if got := asNativeToolInt(t, validation[key]); got != want {
			t.Fatalf("%s = %d, want %d", key, got, want)
		}
	}
	templateGraphOK, ok := validation["template_graph_ok"].(bool)
	if !ok {
		t.Fatalf("template_graph_ok type = %T, want bool", validation["template_graph_ok"])
	}
	if !templateGraphOK {
		t.Fatalf("template_graph_ok = false, validation=%v", validation)
	}
}

func TestPPTXToolTemplateDeleteOriginalChartSlideAfterDuplicateKeepsCloneArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_duplicate_delete_template.pptx",
		"title":  "Chart Duplicate Delete Template",
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

	for _, args := range []map[string]interface{}{
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_duplicate_delete_template.pptx",
			"slide":  3,
		},
		{
			"action": "delete_slide",
			"path":   "decks/chart_duplicate_delete_template.pptx",
			"slide":  3,
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	path := filepath.Join(tmpDir, "decks", "chart_duplicate_delete_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if officeZipHasEntry(t, data, "ppt/charts/chart1.xml") {
		t.Fatal("expected original chart1.xml to be removed after deleting the original chart slide")
	}
	if officeZipHasEntry(t, data, "ppt/charts/_rels/chart1.xml.rels") {
		t.Fatal("expected original chart1.xml.rels to be removed after deleting the original chart slide")
	}
	if officeZipHasEntry(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx") {
		t.Fatal("expected original workbook1 to be removed after deleting the original chart slide")
	}
	for _, entry := range []string{
		"ppt/charts/chart2.xml",
		"ppt/charts/_rels/chart2.xml.rels",
		"ppt/embeddings/Microsoft_Excel_Worksheet2.xlsx",
	} {
		if !officeZipHasEntry(t, data, entry) {
			t.Fatalf("expected surviving clone artifact %s", entry)
		}
	}
	slide3Rels := officeZipEntryText(t, data, "ppt/slides/_rels/slide3.xml.rels")
	if !containsSubstring(slide3Rels, `Target="../charts/chart2.xml"`) {
		t.Fatalf("expected surviving chart slide rels to point at chart2, got %s", slide3Rels)
	}
	chart2Rels := officeZipEntryText(t, data, "ppt/charts/_rels/chart2.xml.rels")
	if !containsSubstring(chart2Rels, `Target="../embeddings/Microsoft_Excel_Worksheet2.xlsx"`) {
		t.Fatalf("expected surviving chart2 rels to point at workbook2, got %s", chart2Rels)
	}
	contentTypes := officeZipEntryText(t, data, "[Content_Types].xml")
	if containsSubstring(contentTypes, `/ppt/charts/chart1.xml`) {
		t.Fatalf("expected [Content_Types].xml to drop chart1 override, got %s", contentTypes)
	}
	for _, needle := range []string{
		`<Override PartName="/ppt/charts/chart2.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>`,
	} {
		if !containsSubstring(contentTypes, needle) {
			t.Fatalf("expected [Content_Types].xml to include %q, got %s", needle, contentTypes)
		}
	}
}

func TestPPTXToolTemplateReorderDuplicatedChartSlidesKeepsChartBindings(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_duplicate_reorder_template.pptx",
		"title":  "Chart Duplicate Reorder Template",
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

	for _, args := range []map[string]interface{}{
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_duplicate_reorder_template.pptx",
			"slide":  3,
		},
		{
			"action": "reorder_slides",
			"path":   "decks/chart_duplicate_reorder_template.pptx",
			"order":  []interface{}{1, 2, 4, 3},
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	path := filepath.Join(tmpDir, "decks", "chart_duplicate_reorder_template.pptx")
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
			t.Fatalf("expected %s to remain after reorder", entry)
		}
	}
	slide3Rels := officeZipEntryText(t, data, "ppt/slides/_rels/slide3.xml.rels")
	if !containsSubstring(slide3Rels, `Target="../charts/chart2.xml"`) {
		t.Fatalf("expected reordered slide3 rels to point at chart2, got %s", slide3Rels)
	}
	slide4Rels := officeZipEntryText(t, data, "ppt/slides/_rels/slide4.xml.rels")
	if !containsSubstring(slide4Rels, `Target="../charts/chart1.xml"`) {
		t.Fatalf("expected reordered slide4 rels to point at chart1, got %s", slide4Rels)
	}
	contentTypes := officeZipEntryText(t, data, "[Content_Types].xml")
	for _, needle := range []string{
		`<Override PartName="/ppt/charts/chart1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Override PartName="/ppt/charts/chart2.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
	} {
		if !containsSubstring(contentTypes, needle) {
			t.Fatalf("expected [Content_Types].xml to include %q, got %s", needle, contentTypes)
		}
	}
}

func TestPPTXToolTemplateDeleteReorderedDuplicateChartSlideKeepsOriginalArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_reorder_delete_duplicate_template.pptx",
		"title":  "Chart Reorder Delete Duplicate Template",
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

	for _, args := range []map[string]interface{}{
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_reorder_delete_duplicate_template.pptx",
			"slide":  3,
		},
		{
			"action": "reorder_slides",
			"path":   "decks/chart_reorder_delete_duplicate_template.pptx",
			"order":  []interface{}{1, 2, 4, 3},
		},
		{
			"action": "delete_slide",
			"path":   "decks/chart_reorder_delete_duplicate_template.pptx",
			"slide":  3,
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	path := filepath.Join(tmpDir, "decks", "chart_reorder_delete_duplicate_template.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, entry := range []string{
		"ppt/charts/chart1.xml",
		"ppt/charts/_rels/chart1.xml.rels",
		"ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx",
	} {
		if !officeZipHasEntry(t, data, entry) {
			t.Fatalf("expected surviving original artifact %s", entry)
		}
	}
	for _, entry := range []string{
		"ppt/charts/chart2.xml",
		"ppt/charts/_rels/chart2.xml.rels",
		"ppt/embeddings/Microsoft_Excel_Worksheet2.xlsx",
	} {
		if officeZipHasEntry(t, data, entry) {
			t.Fatalf("expected deleted duplicate artifact %s to be removed", entry)
		}
	}
	slide3Rels := officeZipEntryText(t, data, "ppt/slides/_rels/slide3.xml.rels")
	if !containsSubstring(slide3Rels, `Target="../charts/chart1.xml"`) {
		t.Fatalf("expected surviving reordered chart slide rels to point at chart1, got %s", slide3Rels)
	}
	contentTypes := officeZipEntryText(t, data, "[Content_Types].xml")
	if containsSubstring(contentTypes, `/ppt/charts/chart2.xml`) {
		t.Fatalf("expected [Content_Types].xml to drop chart2 override, got %s", contentTypes)
	}
	for _, needle := range []string{
		`<Override PartName="/ppt/charts/chart1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`,
		`<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>`,
	} {
		if !containsSubstring(contentTypes, needle) {
			t.Fatalf("expected [Content_Types].xml to include %q, got %s", needle, contentTypes)
		}
	}
}

func TestPPTXToolValidateTemplateReportsChartPackageIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/chart_validate_template.pptx",
		"title":  "Chart Validate Template",
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

	for _, args := range []map[string]interface{}{
		{
			"action": "duplicate_slide",
			"path":   "decks/chart_validate_template.pptx",
			"slide":  3,
		},
		{
			"action": "reorder_slides",
			"path":   "decks/chart_validate_template.pptx",
			"order":  []interface{}{1, 2, 4, 3},
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	validateResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate_template",
		"path":   "decks/chart_validate_template.pptx",
	})
	if err != nil {
		t.Fatalf("validate_template failed: %v", err)
	}
	validatePayload := parseNativeDocumentPayload(t, validateResult)
	validation, ok := validatePayload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", validatePayload["validation"])
	}
	for key, want := range map[string]int{
		"slide_count":                    4,
		"chart_count":                    2,
		"chart_rel_count":                2,
		"embedded_workbook_count":        2,
		"referenced_chart_count":         2,
		"referenced_workbook_count":      2,
		"orphan_chart_count":             0,
		"orphan_chart_rel_count":         0,
		"orphan_embedded_workbook_count": 0,
		"missing_chart_target_count":     0,
		"missing_workbook_target_count":  0,
	} {
		if got := asNativeToolInt(t, validation[key]); got != want {
			t.Fatalf("%s = %d, want %d", key, got, want)
		}
	}
	templateGraphOK, ok := validation["template_graph_ok"].(bool)
	if !ok {
		t.Fatalf("template_graph_ok type = %T, want bool", validation["template_graph_ok"])
	}
	if !templateGraphOK {
		t.Fatalf("template_graph_ok = false, validation=%v", validation)
	}
}
