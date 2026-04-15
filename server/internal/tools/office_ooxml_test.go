package tools

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOfficeInlineMarkdown(t *testing.T) {
	runs := parseOfficeInlineMarkdown("Before **bold** *italic* and `code` after")
	if len(runs) != 7 {
		t.Fatalf("len(runs) = %d, want 7 (%#v)", len(runs), runs)
	}

	assertRun := func(idx int, text string, bold, italic, code bool) {
		t.Helper()
		if runs[idx].Text != text || runs[idx].Bold != bold || runs[idx].Italic != italic || runs[idx].Code != code {
			t.Fatalf("run[%d] = %#v, want text=%q bold=%v italic=%v code=%v", idx, runs[idx], text, bold, italic, code)
		}
	}

	assertRun(0, "Before ", false, false, false)
	assertRun(1, "bold", true, false, false)
	assertRun(2, " ", false, false, false)
	assertRun(3, "italic", false, true, false)
	assertRun(4, " and ", false, false, false)
	assertRun(5, "code", false, false, true)
	assertRun(6, " after", false, false, false)
}

func TestParseOfficeInlineMarkdown_CommonVariants(t *testing.T) {
	runs := parseOfficeInlineMarkdown("Before __bold__ _italic_ ~~gone~~ after")
	if len(runs) != 7 {
		t.Fatalf("len(runs) = %d, want 7 (%#v)", len(runs), runs)
	}

	assertRun := func(idx int, text string, bold, italic, code, strike bool) {
		t.Helper()
		if runs[idx].Text != text || runs[idx].Bold != bold || runs[idx].Italic != italic || runs[idx].Code != code || runs[idx].Strike != strike {
			t.Fatalf("run[%d] = %#v, want text=%q bold=%v italic=%v code=%v strike=%v", idx, runs[idx], text, bold, italic, code, strike)
		}
	}

	assertRun(0, "Before ", false, false, false, false)
	assertRun(1, "bold", true, false, false, false)
	assertRun(2, " ", false, false, false, false)
	assertRun(3, "italic", false, true, false, false)
	assertRun(4, " ", false, false, false, false)
	assertRun(5, "gone", false, false, false, true)
	assertRun(6, " after", false, false, false, false)
}

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

func TestParseMarkdownishOfficeDoc_PreservesOrderedAndTaskLists(t *testing.T) {
	spec := parseMarkdownishOfficeDoc(`## Checklist
1. First step
2) Second step
- [x] Finished
- [ ] Pending
`)

	if len(spec.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(spec.Sections))
	}
	want := []string{"1. First step", "2) Second step", "☑ Finished", "☐ Pending"}
	if len(spec.Sections[0].Bullets) != len(want) {
		t.Fatalf("len(Bullets) = %d, want %d (%#v)", len(spec.Sections[0].Bullets), len(want), spec.Sections[0].Bullets)
	}
	for idx, item := range want {
		if spec.Sections[0].Bullets[idx] != item {
			t.Fatalf("bullet[%d] = %q, want %q", idx, spec.Sections[0].Bullets[idx], item)
		}
	}
}

func TestParseMarkdownishOfficeDoc_PreservesBlockquotesAndCodeBlocks(t *testing.T) {
	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"> Keep the UX obvious\n" +
		"> Prefer native docs\n\n" +
		"```go\n" +
		"fmt.Println(\"hello\")\n" +
		"fmt.Println(\"world\")\n" +
		"```\n\n" +
		"Plain follow-up paragraph.\n")

	if spec.Title != "Release Notes" {
		t.Fatalf("Title = %q, want Release Notes", spec.Title)
	}
	if spec.Summary != "Plain follow-up paragraph." {
		t.Fatalf("Summary = %q, want plain follow-up paragraph", spec.Summary)
	}
	if len(spec.ParagraphBlocks) != 2 {
		t.Fatalf("len(ParagraphBlocks) = %d, want 2 (%#v)", len(spec.ParagraphBlocks), spec.ParagraphBlocks)
	}
	if spec.ParagraphBlocks[0].Kind != officeDocBlockQuote || spec.ParagraphBlocks[0].Text != "Keep the UX obvious\nPrefer native docs" {
		t.Fatalf("ParagraphBlocks[0] = %#v, want quote block", spec.ParagraphBlocks[0])
	}
	if spec.ParagraphBlocks[1].Kind != officeDocBlockCode || spec.ParagraphBlocks[1].Text != "fmt.Println(\"hello\")\nfmt.Println(\"world\")" {
		t.Fatalf("ParagraphBlocks[1] = %#v, want code block", spec.ParagraphBlocks[1])
	}
}

func TestParseMarkdownishOfficeDoc_PreservesThematicBreaks(t *testing.T) {
	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"---\n")

	if spec.Title != "Release Notes" {
		t.Fatalf("Title = %q, want Release Notes", spec.Title)
	}
	if len(spec.ParagraphBlocks) != 1 {
		t.Fatalf("len(ParagraphBlocks) = %d, want 1 (%#v)", len(spec.ParagraphBlocks), spec.ParagraphBlocks)
	}
	if spec.ParagraphBlocks[0].Kind != officeDocBlockSeparator {
		t.Fatalf("ParagraphBlocks[0] = %#v, want separator block", spec.ParagraphBlocks[0])
	}
}

func TestParseMarkdownishOfficeDoc_PreservesImageBlocks(t *testing.T) {
	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"![Diagram](/tmp/sample.png)\n")

	if spec.Title != "Release Notes" {
		t.Fatalf("Title = %q, want Release Notes", spec.Title)
	}
	if len(spec.ParagraphBlocks) != 1 {
		t.Fatalf("len(ParagraphBlocks) = %d, want 1 (%#v)", len(spec.ParagraphBlocks), spec.ParagraphBlocks)
	}
	if spec.ParagraphBlocks[0].Kind != officeDocBlockImage || spec.ParagraphBlocks[0].Text != "Diagram" || spec.ParagraphBlocks[0].Source != "/tmp/sample.png" {
		t.Fatalf("ParagraphBlocks[0] = %#v, want image block", spec.ParagraphBlocks[0])
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

func TestBuildOfficeDOCX_ConvertsInlineMarkdownToOfficeRuns(t *testing.T) {
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title:      "Quarterly Update",
		Paragraphs: []string{"Before **bold** *italic* and `code` after"},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	xml := officeZipEntryText(t, docxData, "word/document.xml")
	for _, needle := range []string{
		`<w:rPr><w:b/></w:rPr><w:t xml:space="preserve">bold</w:t>`,
		`<w:rPr><w:i/></w:rPr><w:t xml:space="preserve">italic</w:t>`,
		`<w:rPr><w:rFonts w:ascii="Aptos Mono" w:hAnsi="Aptos Mono" w:eastAsia="Aptos Mono"/></w:rPr><w:t xml:space="preserve">code</w:t>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("document.xml missing %q in %s", needle, xml)
		}
	}
	for _, marker := range []string{"**bold**", "*italic*", "`code`"} {
		if containsSubstring(xml, marker) {
			t.Fatalf("document.xml should not contain raw marker %q: %s", marker, xml)
		}
	}
}

func TestBuildOfficeDOCX_ConvertsCommonMarkdownVariantsToOfficeRuns(t *testing.T) {
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title:      "Quarterly Update",
		Paragraphs: []string{"Before __bold__ _italic_ ~~gone~~ after"},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	xml := officeZipEntryText(t, docxData, "word/document.xml")
	for _, needle := range []string{
		`<w:rPr><w:b/></w:rPr><w:t xml:space="preserve">bold</w:t>`,
		`<w:rPr><w:i/></w:rPr><w:t xml:space="preserve">italic</w:t>`,
		`<w:rPr><w:strike/></w:rPr><w:t xml:space="preserve">gone</w:t>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("document.xml missing %q in %s", needle, xml)
		}
	}
	for _, marker := range []string{"__bold__", "_italic_", "~~gone~~"} {
		if containsSubstring(xml, marker) {
			t.Fatalf("document.xml should not contain raw marker %q: %s", marker, xml)
		}
	}
}

func TestBuildOfficeDOCX_PreservesOrderedAndTaskListText(t *testing.T) {
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title: "Quarterly Update",
		Sections: []officeDocSection{
			{Heading: "Checklist", Bullets: []string{"1. First step", "☑ Finished", "☐ Pending"}},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	xml := officeZipEntryText(t, docxData, "word/document.xml")
	for _, needle := range []string{"1. First step", "☑ Finished", "☐ Pending"} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("document.xml missing %q in %s", needle, xml)
		}
	}
	for _, unwanted := range []string{"• 1. First step", "• ☑ Finished", "• ☐ Pending"} {
		if containsSubstring(xml, unwanted) {
			t.Fatalf("document.xml should not contain %q in %s", unwanted, xml)
		}
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

func TestParseOfficeInlineMarkdown_WithHyperlinks(t *testing.T) {
	runs := parseOfficeInlineMarkdown("Visit [OpenAI](https://openai.com) for AI")
	if len(runs) != 3 {
		t.Fatalf("len(runs) = %d, want 3 (%#v)", len(runs), runs)
	}
	if runs[0].Text != "Visit " || runs[0].LinkURL != "" {
		t.Fatalf("run[0] = %#v, want plain text 'Visit '", runs[0])
	}
	if runs[1].Text != "OpenAI" || runs[1].LinkURL != "https://openai.com" {
		t.Fatalf("run[1] = %#v, want link text 'OpenAI' with URL", runs[1])
	}
	if runs[2].Text != " for AI" || runs[2].LinkURL != "" {
		t.Fatalf("run[2] = %#v, want plain text ' for AI'", runs[2])
	}
}

func TestBuildOfficeDOCX_ConvertsHyperlinksToOfficeLinks(t *testing.T) {
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title:      "Links Report",
		Paragraphs: []string{"Visit [OpenAI](https://openai.com) for AI"},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	xml := officeZipEntryText(t, docxData, "word/document.xml")
	if !containsSubstring(xml, `<w:hyperlink`) {
		t.Fatalf("document.xml missing hyperlink element: %s", xml)
	}
	if !containsSubstring(xml, `>OpenAI<`) {
		t.Fatalf("document.xml missing link text: %s", xml)
	}
	if containsSubstring(xml, "[OpenAI]") || containsSubstring(xml, "(https://") {
		t.Fatalf("document.xml should not contain raw markdown markers: %s", xml)
	}
	relsXML := officeZipEntryText(t, docxData, "word/_rels/document.xml.rels")
	if !containsSubstring(relsXML, `Target="https://openai.com"`) {
		t.Fatalf("document rels missing hyperlink target: %s", relsXML)
	}
}

func TestBuildOfficeDOCX_UsesThemeTypographyAndAccentForStyles(t *testing.T) {
	theme := resolveOfficeTheme("midnight", "")
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title:      "Board Update",
		Subtitle:   "Q2 stakeholder highlights",
		Theme:      theme,
		Paragraphs: []string{"Visit [OpenAI](https://openai.com) for AI"},
		Sections: []officeDocSection{
			{Heading: "Strategic Focus"},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	stylesXML := officeZipEntryText(t, docxData, "word/styles.xml")
	for _, needle := range []string{
		`<w:style w:type="paragraph" w:styleId="BlueSubtitle"><w:name w:val="Blue Subtitle"/><w:basedOn w:val="Normal"/><w:next w:val="BlueBody"/><w:pPr><w:spacing w:after="40"/><w:keepNext/></w:pPr><w:rPr><w:rFonts w:ascii="Georgia" w:hAnsi="Georgia" w:eastAsia="PingFang SC"/>`,
		`<w:style w:type="paragraph" w:styleId="BlueHeading1"><w:name w:val="Blue Heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="BlueBody"/><w:uiPriority w:val="9"/><w:qFormat/><w:pPr><w:spacing w:before="280" w:after="80"/><w:keepNext/></w:pPr><w:rPr><w:rFonts w:ascii="Georgia" w:hAnsi="Georgia" w:eastAsia="PingFang SC"/>`,
	} {
		if !containsSubstring(stylesXML, needle) {
			t.Fatalf("styles.xml missing theme typography snippet %q in %s", needle, stylesXML)
		}
	}

	documentXML := officeZipEntryText(t, docxData, "word/document.xml")
	if !containsSubstring(documentXML, `<w:color w:val="00D4AA"/>`) {
		t.Fatalf("document.xml missing theme accent hyperlink color in %s", documentXML)
	}
}

func TestBuildOfficeDOCX_ConvertsBlockMarkdownToNativeParagraphStyles(t *testing.T) {
	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"> Keep the UX obvious\n" +
		"> Prefer native docs\n\n" +
		"```go\n" +
		"fmt.Println(\"hello\")\n" +
		"fmt.Println(\"world\")\n" +
		"```\n")

	docxData, _, err := buildOfficeDOCX(spec)
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	stylesXML := officeZipEntryText(t, docxData, "word/styles.xml")
	if !containsSubstring(stylesXML, `w:styleId="BlueCodeBlock"`) {
		t.Fatalf("styles.xml missing BlueCodeBlock style: %s", stylesXML)
	}

	documentXML := officeZipEntryText(t, docxData, "word/document.xml")
	for _, needle := range []string{
		`<w:pStyle w:val="BlueCallout"/>`,
		`<w:pStyle w:val="BlueCodeBlock"/>`,
		`fmt.Println(&#34;hello&#34;)`,
		`fmt.Println(&#34;world&#34;)`,
		`<w:br/>`,
	} {
		if !containsSubstring(documentXML, needle) {
			t.Fatalf("document.xml missing %q in %s", needle, documentXML)
		}
	}
	for _, unwanted := range []string{`&gt; Keep the UX obvious`, "```go", "```"} {
		if containsSubstring(documentXML, unwanted) {
			t.Fatalf("document.xml should not contain raw block markdown %q: %s", unwanted, documentXML)
		}
	}
}

func TestBuildOfficeDOCX_ConvertsThematicBreakToSeparatorStyle(t *testing.T) {
	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"---\n")

	docxData, _, err := buildOfficeDOCX(spec)
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	stylesXML := officeZipEntryText(t, docxData, "word/styles.xml")
	if !containsSubstring(stylesXML, `w:styleId="BlueSeparator"`) {
		t.Fatalf("styles.xml missing BlueSeparator style: %s", stylesXML)
	}

	documentXML := officeZipEntryText(t, docxData, "word/document.xml")
	if !containsSubstring(documentXML, `<w:pStyle w:val="BlueSeparator"/>`) {
		t.Fatalf("document.xml missing BlueSeparator paragraph in %s", documentXML)
	}
	if containsSubstring(documentXML, `&lt;---&gt;`) || containsSubstring(documentXML, `>---<`) {
		t.Fatalf("document.xml should not contain raw separator markdown: %s", documentXML)
	}
}

func TestBuildOfficeDOCX_ConvertsImageMarkdownToNativeMedia(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "diagram.png")
	if err := os.WriteFile(imagePath, testPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"![Diagram](" + imagePath + ")\n")

	docxData, _, err := buildOfficeDOCX(spec)
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	contentTypesXML := officeZipEntryText(t, docxData, "[Content_Types].xml")
	if !containsSubstring(contentTypesXML, `Extension="png" ContentType="image/png"`) {
		t.Fatalf("content types missing png default: %s", contentTypesXML)
	}

	documentXML := officeZipEntryText(t, docxData, "word/document.xml")
	for _, needle := range []string{`<w:drawing>`, `descr="Diagram"`} {
		if !containsSubstring(documentXML, needle) {
			t.Fatalf("document.xml missing %q in %s", needle, documentXML)
		}
	}
	if containsSubstring(documentXML, `![Diagram](`) {
		t.Fatalf("document.xml should not contain raw image markdown: %s", documentXML)
	}

	relsXML := officeZipEntryText(t, docxData, "word/_rels/document.xml.rels")
	if !containsSubstring(relsXML, `relationships/image`) || !containsSubstring(relsXML, `Target="media/image1.png"`) {
		t.Fatalf("document rels missing image relationship: %s", relsXML)
	}

	mediaBytes := officeZipEntryBytes(t, docxData, "word/media/image1.png")
	if len(mediaBytes) == 0 {
		t.Fatal("word/media/image1.png should not be empty")
	}
}

func TestBuildOfficeDOCX_NumberedListUsesNativeNumbering(t *testing.T) {
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title: "Ordered List",
		Sections: []officeDocSection{{
			Heading: "Steps",
			Bullets: []string{"1. First step", "2. Second step", "3. Third step"},
		}},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	xml := officeZipEntryText(t, docxData, "word/document.xml")
	if !containsSubstring(xml, `<w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr>`) {
		t.Fatalf("document.xml missing ordered-list numbering props: %s", xml)
	}
	if containsSubstring(xml, `>1. First step<`) || containsSubstring(xml, `>2. Second step<`) {
		t.Fatalf("document.xml should not keep literal ordered-list prefixes once native numbering is used: %s", xml)
	}
	if !containsSubstring(xml, `>First step<`) || !containsSubstring(xml, `>Second step<`) {
		t.Fatalf("document.xml missing ordered-list item text: %s", xml)
	}

	numberingXML := officeZipEntryText(t, docxData, "word/numbering.xml")
	if !containsSubstring(numberingXML, `<w:numFmt w:val="decimal"/>`) || !containsSubstring(numberingXML, `<w:lvlText w:val="%1."/>`) {
		t.Fatalf("numbering.xml missing decimal numbering definition: %s", numberingXML)
	}

	relsXML := officeZipEntryText(t, docxData, "word/_rels/document.xml.rels")
	if !containsSubstring(relsXML, `relationships/numbering`) || !containsSubstring(relsXML, `Target="numbering.xml"`) {
		t.Fatalf("document rels missing numbering relationship: %s", relsXML)
	}
}

func TestBuildOfficeDOCX_TaskListUsesCheckbox(t *testing.T) {
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title: "Tasks",
		Sections: []officeDocSection{{
			Heading: "Todo",
			Bullets: []string{"☑ Finished task", "☐ Pending task"},
		}},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	xml := officeZipEntryText(t, docxData, "word/document.xml")
	// Should render checkbox characters
	if !containsSubstring(xml, "☑") {
		t.Fatalf("document.xml missing checked checkbox: %s", xml)
	}
	if !containsSubstring(xml, "☐") {
		t.Fatalf("document.xml missing unchecked checkbox: %s", xml)
	}
}

func TestBuildOfficeDOCX_UnorderedListUsesNativeBullets(t *testing.T) {
	docxData, _, err := buildOfficeDOCX(officeDocSpec{
		Title: "Bullets",
		Sections: []officeDocSection{{
			Heading: "Highlights",
			Bullets: []string{"Fast setup", "Native formatting"},
		}},
	})
	if err != nil {
		t.Fatalf("buildOfficeDOCX failed: %v", err)
	}

	xml := officeZipEntryText(t, docxData, "word/document.xml")
	if !containsSubstring(xml, `<w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr>`) {
		t.Fatalf("document.xml missing unordered-list numbering props: %s", xml)
	}
	if containsSubstring(xml, `>• Fast setup<`) || containsSubstring(xml, `>• Native formatting<`) {
		t.Fatalf("document.xml should not keep literal bullet glyphs once native bullets are used: %s", xml)
	}
	if !containsSubstring(xml, `>Fast setup<`) || !containsSubstring(xml, `>Native formatting<`) {
		t.Fatalf("document.xml missing unordered-list item text: %s", xml)
	}

	numberingXML := officeZipEntryText(t, docxData, "word/numbering.xml")
	if !containsSubstring(numberingXML, `<w:numFmt w:val="bullet"/>`) || !containsSubstring(numberingXML, `<w:lvlText w:val="•"/>`) {
		t.Fatalf("numbering.xml missing bullet definition: %s", numberingXML)
	}
}
