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

func TestOfficeBuildPresentationSlidesDerivesTitleFromPlaceholderHeading(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Sections: []officeDocSection{
			{
				Heading:         "幻灯片 1：封面",
				ParagraphBlocks: []officeDocBlock{{Kind: officeDocBlockParagraph, Text: "Qwen3.5 最新优化介绍"}, {Kind: officeDocBlockParagraph, Text: "混合思考 · 高效推理 · 超长上下文"}},
			},
		},
	})
	if len(slides) != 1 {
		t.Fatalf("len(slides) = %d, want 1", len(slides))
	}
	if slides[0].Title != "Qwen3.5 最新优化介绍" {
		t.Fatalf("slide title = %q, want derived title from first real paragraph", slides[0].Title)
	}
}

func TestOfficeBuildPresentationSlidesCleansChinesePagePlaceholdersAndGuidanceLabels(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title:    "Qwen3 最新优化介绍",
		Subtitle: "Think Deeper, Act Faster · 混合思考 AI 模型",
		Sections: []officeDocSection{
			{
				Heading: "第1页：封面",
				ParagraphBlocks: []officeDocBlock{
					{Kind: officeDocBlockParagraph, Text: "标题：Qwen3 最新优化介绍"},
					{Kind: officeDocBlockParagraph, Text: "副标题：Think Deeper, Act Faster · 混合思考 AI 模型"},
					{Kind: officeDocBlockParagraph, Text: "风格：科技感、现代"},
				},
			},
			{
				Heading: "第2页：什么是 Qwen3？",
				ParagraphBlocks: []officeDocBlock{
					{Kind: officeDocBlockParagraph, Text: "标题：Qwen3 是什么？"},
					{Kind: officeDocBlockParagraph, Text: "Qwen3 是阿里巴巴通义千问推出的最新一代大语言模型系列。"},
				},
			},
		},
	})

	if len(slides) != 3 {
		t.Fatalf("len(slides) = %d, want 3 (%#v)", len(slides), slides)
	}
	if slides[0].Title != "Qwen3 最新优化介绍" {
		t.Fatalf("cover title = %q, want clean deck title", slides[0].Title)
	}
	if len(slides[0].Lines) != 1 || slides[0].Lines[0] != "Think Deeper, Act Faster · 混合思考 AI 模型" {
		t.Fatalf("cover lines = %#v, want only cleaned subtitle", slides[0].Lines)
	}
	if slides[1].Title != "目录" || len(slides[1].Lines) != 1 || slides[1].Lines[0] != "1. Qwen3 是什么？" {
		t.Fatalf("toc = %#v, want cleaned section title without 第X页 prefix", slides[1])
	}
	if slides[2].Title != "Qwen3 是什么？" {
		t.Fatalf("section title = %q, want cleaned title guidance value", slides[2].Title)
	}
	if len(slides[2].Blocks) != 1 || slides[2].Blocks[0].Text != "Qwen3 是阿里巴巴通义千问推出的最新一代大语言模型系列。" {
		t.Fatalf("section blocks = %#v, want guidance labels stripped from body", slides[2].Blocks)
	}
}

func TestOfficeBuildPresentationSlidesMergesLeadingDuplicateCoverSection(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title:    "Qwen3.5 最新优化介绍",
		Subtitle: "混合思考 · 高效推理 · 超长上下文",
		Sections: []officeDocSection{
			{
				Heading: "Qwen3.5 最新优化介绍",
				ParagraphBlocks: []officeDocBlock{
					{Kind: officeDocBlockParagraph, Text: "混合思考 · 高效推理 · 超长上下文"},
					{Kind: officeDocBlockParagraph, Text: "阿里巴巴通义千问团队"},
				},
			},
			{
				Heading: "Qwen3.5 是什么？",
				Bullets: []string{"Qwen3-Instruct-2507", "Qwen3-Thinking-2507"},
			},
		},
	})

	if len(slides) != 3 {
		t.Fatalf("len(slides) = %d, want 3 (%#v)", len(slides), slides)
	}
	if slides[0].Title != "Qwen3.5 最新优化介绍" {
		t.Fatalf("cover title = %q", slides[0].Title)
	}
	if len(slides[0].Lines) != 2 {
		t.Fatalf("len(cover lines) = %d, want 2 (%#v)", len(slides[0].Lines), slides[0].Lines)
	}
	if slides[0].Lines[0] != "混合思考 · 高效推理 · 超长上下文" || slides[0].Lines[1] != "阿里巴巴通义千问团队" {
		t.Fatalf("cover lines = %#v, want merged subtitle + author", slides[0].Lines)
	}
	if slides[1].Title != "目录" {
		t.Fatalf("slides[1].Title = %q, want 目录", slides[1].Title)
	}
	if len(slides[1].Lines) != 1 || slides[1].Lines[0] != "1. Qwen3.5 是什么？" {
		t.Fatalf("toc lines = %#v, want only non-cover sections", slides[1].Lines)
	}
	if slides[2].Title != "Qwen3.5 是什么？" {
		t.Fatalf("slides[2].Title = %q, want real first section", slides[2].Title)
	}
}

func TestOfficeBuildPresentationSlidesLocalizesDeckChromeForChineseDeck(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title:    "发布计划",
		Subtitle: "第二季度推进节奏",
		Summary:  "保持发布窗口清晰",
		Sections: []officeDocSection{
			{Heading: "概览", Paragraphs: []string{"四月启动 Beta"}},
		},
	})

	if len(slides) != 4 {
		t.Fatalf("len(slides) = %d, want 4 (%#v)", len(slides), slides)
	}
	if slides[0].Title != "发布计划" {
		t.Fatalf("slides[0].Title = %q, want 发布计划", slides[0].Title)
	}
	if slides[1].Title != "目录" {
		t.Fatalf("slides[1].Title = %q, want 目录", slides[1].Title)
	}
	if slides[2].Title != "概览" {
		t.Fatalf("slides[2].Title = %q, want 概览", slides[2].Title)
	}
	if slides[3].Title != "总结" {
		t.Fatalf("slides[3].Title = %q, want 总结", slides[3].Title)
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

func TestOfficePPTXSlideXMLUsesHeroCoverLayoutWithoutBullets(t *testing.T) {
	slide := officePPTXSlide{
		Title:   "Qwen3.5 最新优化介绍",
		IsCover: true,
		Lines:   []string{"混合思考 · 高效推理 · 超长上下文", "阿里巴巴通义千问团队"},
	}

	xml := officePPTXSlideXML(slide)
	if !containsSubstring(xml, `<a:pPr algn="l"><a:buNone/>`) {
		t.Fatalf("cover slide XML missing left-aligned bulletless paragraph props: %s", xml)
	}
	for _, text := range slide.Lines {
		if !containsSubstring(xml, officeXMLText(text)) {
			t.Fatalf("cover slide XML missing line %q in %s", text, xml)
		}
	}
	for _, needle := range []string{
		`name="Cover Background"`,
		`name="Cover Orb"`,
		`name="Cover Halo"`,
		`name="Cover Eyebrow"`,
		`name="Cover Accent Rail"`,
		`name="Cover Anchor Line"`,
		`name="Cover Signature"`,
		`<p:cNvPr id="2" name="Title"/>`,
		`<p:spPr><a:xfrm><a:off x="1508760" y="2971800"/><a:ext cx="8534400" cy="1188720"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:noLn/></p:spPr>`,
		`<p:cNvPr id="3" name="Content"/>`,
		`<p:spPr><a:xfrm><a:off x="1508760" y="4343400"/><a:ext cx="6400800" cy="731520"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:noLn/></p:spPr>`,
		`<p:spPr><a:xfrm><a:off x="7772400" y="5943600"/><a:ext cx="3048000" cy="320040"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:noLn/></p:spPr>`,
		`<a:defRPr sz="4400">`,
		`<a:t>Powered By ZimaOS-Blue</a:t>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("cover slide XML missing %q in %s", needle, xml)
		}
	}
	for _, unwanted := range []string{`name="Theme Canvas"`, `name="Theme Header Panel"`} {
		if containsSubstring(xml, unwanted) {
			t.Fatalf("cover slide XML should not contain standard slide chrome %q in %s", unwanted, xml)
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

func TestBuildOfficePPTX_ParsesStructuredSectionBodyMarkdownWithoutBlankLines(t *testing.T) {
	spec, err := parseOfficeDocSpec(map[string]interface{}{
		"sections": []interface{}{
			map[string]interface{}{
				"heading": "幻灯片 6：性能基准",
				"body": "" +
					"# 性能表现\n" +
					"- AIME（数学推理）：顶尖水平\n" +
					"- BFCL（函数调用）：领先竞品\n" +
					"| 维度 | 结论 |\n" +
					"| --- | --- |\n" +
					"| 多语言理解 | 支持 119 种语言 |\n" +
					"| 对比范围 | DeepSeek-R1、o1、o3-mini |\n" +
					"对比模型：Gemini-2.5-Pro",
			},
		},
	}, "", "", resolveOfficeTheme("analysis", "board presentation"), "board presentation")
	if err != nil {
		t.Fatalf("parseOfficeDocSpec failed: %v", err)
	}
	if len(spec.Sections) != 1 {
		t.Fatalf("len(spec.Sections) = %d, want 1", len(spec.Sections))
	}
	section := spec.Sections[0]
	if section.Heading != "性能表现" {
		t.Fatalf("section heading = %q, want parsed markdown heading", section.Heading)
	}
	if len(section.Bullets) != 2 {
		t.Fatalf("len(section.Bullets) = %d, want 2 (%#v)", len(section.Bullets), section.Bullets)
	}
	if section.Table == nil || len(section.Table.Headers) != 2 || len(section.Table.Rows) != 2 {
		t.Fatalf("section table = %#v, want native parsed markdown table", section.Table)
	}
	if len(section.ParagraphBlocks) != 1 || section.ParagraphBlocks[0].Text != "对比模型：Gemini-2.5-Pro" {
		t.Fatalf("section paragraph blocks = %#v, want trailing paragraph note", section.ParagraphBlocks)
	}

	pptxData, _, err := buildOfficePPTX(spec)
	if err != nil {
		t.Fatalf("buildOfficePPTX failed: %v", err)
	}

	slideXML := officeZipEntryText(t, pptxData, "ppt/slides/slide1.xml")
	for _, needle := range []string{
		`<a:t>性能表现</a:t>`,
		`<a:buChar char="•"/>`,
		`<a:t>AIME（数学推理）：顶尖水平</a:t>`,
		`<a:tbl>`,
		`<a:t>维度</a:t>`,
		`<a:t>多语言理解</a:t>`,
		`<a:t>对比模型：Gemini-2.5-Pro</a:t>`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, slideXML)
		}
	}
	for _, unwanted := range []string{"# 性能表现", "| 维度 | 结论 |", "| --- | --- |"} {
		if containsSubstring(slideXML, unwanted) {
			t.Fatalf("slide XML should not contain raw markdown %q: %s", unwanted, slideXML)
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
		`name="Theme Canvas"`,
		`name="Theme Header Rule"`,
		`typeface="Georgia"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
		`val="6B341F"`,
		`val="5F5249"`,
		`val="FFF9F5"`,
		`val="1F8A70"`,
		`name="Separator 1"`,
		`val="B08968"`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
	for _, unwanted := range []string{
		`name="Theme Header Panel"`,
		`name="Theme Accent Pill"`,
		`name="Theme Footer Rule"`,
	} {
		if containsSubstring(xml, unwanted) {
			t.Fatalf("slide XML should not contain layered theme patch %q in %s", unwanted, xml)
		}
	}
}

func TestOfficePPTXSlideXMLUsesDarkMidnightChromeAndLightText(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Qwen3 最新优化介绍",
		Theme: resolveOfficeTheme("midnight", ""),
		Lines: []string{"Alibaba Cloud Qwen Team · 2025"},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`val="05070B"`,
		`val="0B0D12"`,
		`val="F5F3EE"`,
		`val="D8DCE5"`,
		`val="4D7CFE"`,
		`name="Theme Canvas"`,
		`name="Theme Header Rule"`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("midnight slide XML missing %q in %s", needle, xml)
		}
	}
	for _, unwanted := range []string{
		`name="Theme Header Panel"`,
		`name="Theme Accent Pill"`,
		`name="Theme Footer Rule"`,
	} {
		if containsSubstring(xml, unwanted) {
			t.Fatalf("midnight slide XML should not contain layered patch %q in %s", unwanted, xml)
		}
	}
	if containsSubstring(xml, `name="Theme Background"`) && containsSubstring(xml, `name="Theme Background"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="12192000" cy="6858000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="F8FAFC"/>`) {
		t.Fatalf("midnight theme background should not remain light in %s", xml)
	}
}

func TestOfficePPTXSlideXMLUsesExplicitTextBoxGeometryForTitleAndContent(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Design Review",
		Lines: []string{"Pages and Office should be able to place this text box."},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`<p:cNvPr id="2" name="Title"/>`,
		`<p:spPr><a:xfrm><a:off x="685800" y="731520"/><a:ext cx="10858500" cy="731520"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:noLn/></p:spPr>`,
		`<p:cNvPr id="3" name="Content"/>`,
		`<p:spPr><a:xfrm><a:off x="685800" y="1737360"/><a:ext cx="10858500" cy="4343400"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:noLn/></p:spPr>`,
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
		`<a:solidFill><a:srgbClr val="B85C38"/></a:solidFill>`,
		`<a:solidFill><a:srgbClr val="FFF9F5"/></a:solidFill>`,
		`<a:solidFill><a:srgbClr val="FFF4EE"/></a:solidFill>`,
		`<a:srgbClr val="E3D6CE"/>`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
}

func TestOfficePPTXSlideXMLUsesLightTableHeaderTextForDarkThemes(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Metrics",
		Theme: resolveOfficeTheme("midnight", ""),
		Table: &officeTableSpec{
			Headers: []string{"Metric", "Value"},
			Rows: [][]string{
				{"Latency", "118ms"},
			},
		},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`<a:solidFill><a:srgbClr val="242C38"/></a:solidFill>`,
		`<a:solidFill><a:srgbClr val="F5F3EE"/></a:solidFill>`,
		`<a:solidFill><a:srgbClr val="0B0D12"/></a:solidFill>`,
		`<a:solidFill><a:srgbClr val="05070B"/></a:solidFill>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("dark table XML missing %q in %s", needle, xml)
		}
	}
}
