package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOfficeBuildPresentationSlidesPreservesOrderedAndTaskListText(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title: "Deck",
		Sections: []officeDocSection{
			{Heading: "Checklist", Bullets: []string{"1. First step", "☑ Finished", "☐ Pending", "Regular bullet"}},
		},
	})
	if len(slides) < 3 {
		t.Fatalf("len(slides) = %d, want at least 3", len(slides))
	}
	lines := make([]string, 0, len(slides[2].Blocks))
	for _, block := range slides[2].Blocks {
		lines = append(lines, block.Text)
	}
	want := []string{"1. First step", "☑ Finished", "☐ Pending", "- Regular bullet"}
	if len(lines) != len(want) {
		t.Fatalf("len(lines) = %d, want %d (%#v)", len(lines), len(want), lines)
	}
	for idx, line := range want {
		if lines[idx] != line {
			t.Fatalf("line[%d] = %q, want %q", idx, lines[idx], line)
		}
	}
}

func TestBuildOfficePPTX_UsesNativeListParagraphs(t *testing.T) {
	pptxData, _, err := buildOfficePPTX(officeDocSpec{
		Title: "Deck",
		Sections: []officeDocSection{
			{
				Heading: "Checklist",
				Bullets: []string{"1. First step", "2. Second step", "Native formatting"},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficePPTX failed: %v", err)
	}

	slideXML := officeZipEntryText(t, pptxData, "ppt/slides/slide3.xml")
	if !containsSubstring(slideXML, `<a:buAutoNum type="arabicPeriod" startAt="1"/>`) {
		t.Fatalf("slide XML missing native ordered-list paragraph props: %s", slideXML)
	}
	if !containsSubstring(slideXML, `<a:buAutoNum type="arabicPeriod" startAt="2"/>`) {
		t.Fatalf("slide XML missing second ordered-list item numbering props: %s", slideXML)
	}
	if !containsSubstring(slideXML, `<a:buChar char="•"/>`) {
		t.Fatalf("slide XML missing native unordered-list bullet props: %s", slideXML)
	}
	if containsSubstring(slideXML, `<a:t>1. First step</a:t>`) || containsSubstring(slideXML, `<a:t>2. Second step</a:t>`) || containsSubstring(slideXML, `<a:t>- Native formatting</a:t>`) {
		t.Fatalf("slide XML should not keep literal list prefixes once native list props are used: %s", slideXML)
	}
	for _, needle := range []string{`<a:t>First step</a:t>`, `<a:t>Second step</a:t>`, `<a:t>Native formatting</a:t>`} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("slide XML missing list item body %q in %s", needle, slideXML)
		}
	}
}

func TestOfficePPTXSlideXMLConvertsInlineMarkdownToNativeRuns(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Metrics",
		Lines: []string{"Before **bold** *italic* and `code` after"},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`<a:r><a:rPr lang="en-US" b="1" sz="2200"/><a:t>bold</a:t></a:r>`,
		`<a:r><a:rPr lang="en-US" i="1" sz="2200"/><a:t>italic</a:t></a:r>`,
		`<a:r><a:rPr lang="en-US" sz="2200"><a:latin typeface="Menlo"/><a:ea typeface="Hiragino Sans GB"/><a:cs typeface="Menlo"/></a:rPr><a:t>code</a:t></a:r>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
	for _, marker := range []string{"**bold**", "*italic*", "`code`"} {
		if containsSubstring(xml, marker) {
			t.Fatalf("slide XML should not contain raw marker %q: %s", marker, xml)
		}
	}
}

func TestOfficePPTXSlideXMLConvertsCommonMarkdownVariantsToNativeRuns(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Metrics",
		Lines: []string{"Before __bold__ _italic_ ~~gone~~ after"},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`<a:r><a:rPr lang="en-US" b="1" sz="2200"/><a:t>bold</a:t></a:r>`,
		`<a:r><a:rPr lang="en-US" i="1" sz="2200"/><a:t>italic</a:t></a:r>`,
		`<a:r><a:rPr lang="en-US" strike="sngStrike" sz="2200"/><a:t>gone</a:t></a:r>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
	for _, marker := range []string{"__bold__", "_italic_", "~~gone~~"} {
		if containsSubstring(xml, marker) {
			t.Fatalf("slide XML should not contain raw marker %q: %s", marker, xml)
		}
	}
}

func TestOfficePPTXSlideXMLIncludesNativeTable(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Metrics",
		Lines: []string{"Performance snapshot"},
		Table: &officeTableSpec{
			Headers: []string{"Region", "Revenue", "Status"},
			Rows: [][]string{
				{"North", "120", "Good"},
				{"South", "98", "Watch"},
			},
		},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">`,
		`<a:tbl>`,
		`<a:tblPr firstRow="1" bandRow="1">`,
		`<a:gridCol `,
		`<a:tr h="`,
		`<a:t>Region</a:t>`,
		`<a:t>North</a:t>`,
		`<a:t>Performance snapshot</a:t>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
	if containsSubstring(xml, `<a:t>Region | Revenue | Status</a:t>`) {
		t.Fatalf("slide XML should not flatten table headers into one text run: %s", xml)
	}
}

func TestBuildOfficePPTX_ConvertsBlockMarkdownToNativeText(t *testing.T) {
	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"> Keep the UX obvious\n" +
		"> Prefer native docs\n\n" +
		"```go\n" +
		"fmt.Println(\"hello\")\n" +
		"fmt.Println(\"world\")\n" +
		"```\n")

	pptxData, _, err := buildOfficePPTX(spec)
	if err != nil {
		t.Fatalf("buildOfficePPTX failed: %v", err)
	}

	slideXML := officeZipEntryText(t, pptxData, "ppt/slides/slide2.xml")
	for _, needle := range []string{
		`Keep the UX obvious`,
		`Prefer native docs`,
		`<a:latin typeface="Menlo"/>`,
		`<a:ea typeface="Hiragino Sans GB"/>`,
		`fmt.Println(&#34;hello&#34;)`,
		`fmt.Println(&#34;world&#34;)`,
		`<a:br/>`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, slideXML)
		}
	}
	for _, unwanted := range []string{`&gt; Keep the UX obvious`, "```go", "```"} {
		if containsSubstring(slideXML, unwanted) {
			t.Fatalf("slide XML should not contain raw block markdown %q: %s", unwanted, slideXML)
		}
	}
}

func TestBuildOfficePPTX_ConvertsHyperlinksToOfficeLinks(t *testing.T) {
	pptxData, _, err := buildOfficePPTX(officeDocSpec{
		Title:           "Links Deck",
		ParagraphBlocks: []officeDocBlock{{Kind: officeDocBlockParagraph, Text: "Visit [OpenAI](https://openai.com) for AI"}},
	})
	if err != nil {
		t.Fatalf("buildOfficePPTX failed: %v", err)
	}

	slideXML := officeZipEntryText(t, pptxData, "ppt/slides/slide2.xml")
	if !containsSubstring(slideXML, `<a:hlinkClick r:id="rId2"/>`) {
		t.Fatalf("slide XML missing hyperlink click relation: %s", slideXML)
	}
	if containsSubstring(slideXML, "[OpenAI]") || containsSubstring(slideXML, "(https://") {
		t.Fatalf("slide XML should not contain raw markdown markers: %s", slideXML)
	}

	relsXML := officeZipEntryText(t, pptxData, "ppt/slides/_rels/slide2.xml.rels")
	if !containsSubstring(relsXML, `Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"`) {
		t.Fatalf("slide rels missing hyperlink relationship type: %s", relsXML)
	}
	if !containsSubstring(relsXML, `Target="https://openai.com"`) || !containsSubstring(relsXML, `TargetMode="External"`) {
		t.Fatalf("slide rels missing hyperlink target: %s", relsXML)
	}
}

func TestBuildOfficePPTX_ConvertsImageMarkdownToNativeMedia(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "diagram.png")
	if err := os.WriteFile(imagePath, testPNGBytes(t), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"![Diagram](" + imagePath + ")\n")

	pptxData, _, err := buildOfficePPTX(spec)
	if err != nil {
		t.Fatalf("buildOfficePPTX failed: %v", err)
	}

	contentTypesXML := officeZipEntryText(t, pptxData, "[Content_Types].xml")
	if !containsSubstring(contentTypesXML, `Extension="png" ContentType="image/png"`) {
		t.Fatalf("content types missing png default: %s", contentTypesXML)
	}

	slideXML := officeZipEntryText(t, pptxData, "ppt/slides/slide2.xml")
	for _, needle := range []string{`<p:pic>`, `descr="Diagram"`, `<a:blip r:embed="`} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, slideXML)
		}
	}
	if containsSubstring(slideXML, `![Diagram](`) {
		t.Fatalf("slide XML should not contain raw image markdown: %s", slideXML)
	}

	relsXML := officeZipEntryText(t, pptxData, "ppt/slides/_rels/slide2.xml.rels")
	if !containsSubstring(relsXML, `relationships/image`) || !containsSubstring(relsXML, `Target="../media/image1.png"`) {
		t.Fatalf("slide rels missing image relationship: %s", relsXML)
	}

	mediaBytes := officeZipEntryBytes(t, pptxData, "ppt/media/image1.png")
	if len(mediaBytes) == 0 {
		t.Fatal("ppt/media/image1.png should not be empty")
	}
}

func TestBuildOfficePPTX_ConvertsThematicBreakToSeparatorShape(t *testing.T) {
	spec := parseMarkdownishOfficeDoc("" +
		"# Release Notes\n\n" +
		"---\n")

	pptxData, _, err := buildOfficePPTX(spec)
	if err != nil {
		t.Fatalf("buildOfficePPTX failed: %v", err)
	}

	slideXML := officeZipEntryText(t, pptxData, "ppt/slides/slide2.xml")
	for _, needle := range []string{
		`name="Separator 1"`,
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`,
		`<a:ln w="12700">`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, slideXML)
		}
	}
	if containsSubstring(slideXML, `>---<`) {
		t.Fatalf("slide XML should not contain raw separator markdown: %s", slideXML)
	}
}

func TestOfficePPTXSlideXMLUsesThemeTypographyAndSeparatorColor(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Design Review",
		Theme: resolveOfficeTheme("terracotta", ""),
		Blocks: []officeDocBlock{
			{Kind: officeDocBlockParagraph, Text: "Palette direction stays warm and grounded."},
			{Kind: officeDocBlockSeparator},
		},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`name="Theme Background"`,
		`name="Theme Band"`,
		`name="Theme Accent Mark"`,
		`typeface="Georgia"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
		`val="8B4513"`,
		`val="57534E"`,
		`val="FFF8F5"`,
		`val="FDF2ED"`,
		`val="2E8B57"`,
		`name="Separator 1"`,
		`val="D4A574"`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
}

func TestOfficePPTXSlideXMLUsesThemeAwareTableChrome(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Metrics",
		Theme: resolveOfficeTheme("terracotta", ""),
		Table: &officeTableSpec{
			Headers: []string{"Metric", "Value"},
			Rows: [][]string{
				{"NPS", "42"},
				{"Retention", "91%"},
			},
		},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`<a:solidFill><a:srgbClr val="C65D3B"/></a:solidFill>`,
		`<a:solidFill><a:srgbClr val="FFFCFA"/></a:solidFill>`,
		`<a:solidFill><a:srgbClr val="FFF8F5"/></a:solidFill>`,
		`<a:srgbClr val="D6D3D1"/>`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
}
