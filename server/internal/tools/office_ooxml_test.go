package tools

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestParseMarkdownishOfficeDoc(t *testing.T) {
	spec := parseMarkdownishOfficeDoc(`# Quarterly Update
Wins from the quarter

## Highlights
- Revenue grew
- Costs fell

| Metric | Value |
| --- | --- |
| NRR | 121% |
`)

	if spec.Title != "Quarterly Update" {
		t.Fatalf("Title = %q, want Quarterly Update", spec.Title)
	}
	if spec.Subtitle != "Wins from the quarter" {
		t.Fatalf("Subtitle = %q, want first paragraph as subtitle", spec.Subtitle)
	}
	if len(spec.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(spec.Sections))
	}
	if len(spec.Sections[0].Bullets) != 2 {
		t.Fatalf("len(Bullets) = %d, want 2", len(spec.Sections[0].Bullets))
	}
	if spec.Sections[0].Table == nil || len(spec.Sections[0].Table.Rows) != 1 {
		t.Fatalf("table = %#v, want one row", spec.Sections[0].Table)
	}
}

func TestBuildOfficeArtifacts(t *testing.T) {
	xlsxData, xlsxInfo, err := buildOfficeXLSX(officeWorkbookSpec{
		Title: "Quarterly Metrics",
		Sheets: []officeSheetSpec{
			{
				Name: "Metrics",
				Columns: []officeColumnSpec{
					{Header: "Metric", Key: "metric"},
					{Header: "Value", Key: "value", Kind: "number"},
				},
				Rows: [][]interface{}{
					{"MRR", 1200},
				},
				Filter: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficeXLSX failed: %v", err)
	}
	if xlsxInfo.SheetCount != 2 {
		t.Fatalf("SheetCount = %d, want 2 with overview", xlsxInfo.SheetCount)
	}
	if !officeZipHasEntry(t, xlsxData, "xl/workbook.xml") {
		t.Fatal("xlsx missing xl/workbook.xml")
	}

	docxData, docxInfo, err := buildOfficeDOCX(officeDocSpec{
		Title:   "Quarterly Update",
		Summary: "Revenue grew 18 percent.",
		Sections: []officeDocSection{
			{Heading: "Highlights", Bullets: []string{"Pipeline improved"}},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}
	if docxInfo.SectionCount != 1 {
		t.Fatalf("SectionCount = %d, want 1", docxInfo.SectionCount)
	}
	xml := officeZipEntryText(t, docxData, "word/document.xml")
	if !strings.Contains(xml, "Quarterly Update") || !strings.Contains(xml, "Pipeline improved") {
		t.Fatalf("document.xml missing expected content: %s", xml)
	}
}

func TestBuildOfficeDOCX_CleansBracketNoiseAndDeduplicatesParagraphs(t *testing.T) {
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title:   "【【季度】】报告",
		Summary: "这是【【重点】】摘要。\n\n这是【【重点】】摘要。",
		Paragraphs: []string{
			"重复 段落。",
			"重复段落。",
		},
		Sections: []officeDocSection{
			{
				Heading: "发现【【列表】】",
				Bullets: []string{
					"存在【【】】异常符号",
					"存在异常符号",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	xml := officeZipEntryText(t, docxData, "word/document.xml")
	if strings.Contains(xml, "【【") || strings.Contains(xml, "】】") {
		t.Fatalf("document.xml still contains bracket artifacts: %s", xml)
	}
	if strings.Count(xml, "重复段落") != 1 {
		t.Fatalf("document.xml should contain one cleaned repeated paragraph, got %d: %s", strings.Count(xml, "重复段落"), xml)
	}
	if strings.Count(xml, "存在异常符号") != 1 {
		t.Fatalf("document.xml should deduplicate section bullets, got %d: %s", strings.Count(xml, "存在异常符号"), xml)
	}
}

func officeZipHasEntry(t *testing.T, data []byte, name string) bool {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, file := range reader.File {
		if file.Name == name {
			return true
		}
	}
	return false
}

func officeZipEntryBytes(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", name, err)
		}
		defer rc.Close()
		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read entry %s: %v", name, err)
		}
		return content
	}
	t.Fatalf("missing zip entry %s", name)
	return nil
}

func officeZipEntryText(t *testing.T, data []byte, name string) string {
	t.Helper()
	return string(officeZipEntryBytes(t, data, name))
}
