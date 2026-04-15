package convert

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/officemd"
)

type nativeMarkdownPPTXSlide struct {
	Title string
	Lines []string
}

var (
	nativeMarkdownLinkPattern     = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	nativeMarkdownImagePattern    = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)
	nativeMarkdownOrderedPattern  = regexp.MustCompile(`^\d+\.\s+`)
	nativeMarkdownStrongPatterns  = []*regexp.Regexp{regexp.MustCompile(`\*\*(.*?)\*\*`), regexp.MustCompile(`__(.*?)__`)}
	nativeMarkdownEmphasisPattern = []*regexp.Regexp{regexp.MustCompile(`\*(.*?)\*`), regexp.MustCompile(`_(.*?)_`), regexp.MustCompile("`(.*?)`"), regexp.MustCompile(`~~(.*?)~~`)}
)

func supportsNativeMarkdownPPTXConversion(sourceExt, targetExt string) bool {
	return normalizeFormat(sourceExt, "") == "md" && normalizeFormat(targetExt, "") == "pptx"
}

func buildNativeMarkdownPPTXFromFile(sourcePath string) ([]byte, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}
	slides := parseNativeMarkdownPPTXSlides(string(data))
	if len(slides) == 0 {
		return nil, fmt.Errorf("markdown deck contained no slide content")
	}
	return buildNativeMarkdownPPTX(slides)
}

func parseNativeMarkdownPPTXSlides(content string) []nativeMarkdownPPTXSlide {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	deckTitle := nativeMarkdownDeckTitle(normalized)
	sections := nativeMarkdownSplitSections(normalized)
	slides := make([]nativeMarkdownPPTXSlide, 0, len(sections))
	for _, section := range sections {
		slide := parseNativeMarkdownPPTXSection(section, deckTitle)
		if strings.TrimSpace(slide.Title) == "" && len(slide.Lines) == 0 {
			continue
		}
		slides = append(slides, slide)
	}
	if len(slides) == 0 && strings.TrimSpace(deckTitle) != "" {
		return []nativeMarkdownPPTXSlide{{Title: deckTitle}}
	}
	return slides
}

func nativeMarkdownDeckTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if title, level, ok := nativeMarkdownHeading(trimmed); ok && level == 1 {
			return title
		}
	}
	return ""
}

func nativeMarkdownSplitSections(content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	sections := make([]string, 0, 4)
	current := make([]string, 0, len(lines))
	flush := func() {
		section := strings.TrimSpace(strings.Join(current, "\n"))
		if section != "" {
			sections = append(sections, section)
		}
		current = current[:0]
	}
	for _, line := range lines {
		if nativeMarkdownIsThematicBreak(line) {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	if len(sections) > 1 {
		return sections
	}

	sections = sections[:0]
	current = current[:0]
	headingCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if _, level, ok := nativeMarkdownHeading(trimmed); ok && level == 2 {
			headingCount++
			if len(current) > 0 {
				flush()
			}
		}
		current = append(current, line)
	}
	flush()
	if headingCount > 0 && len(sections) > 0 {
		return sections
	}
	return []string{strings.TrimSpace(content)}
}

func parseNativeMarkdownPPTXSection(section, deckTitle string) nativeMarkdownPPTXSlide {
	lines := strings.Split(strings.ReplaceAll(section, "\r\n", "\n"), "\n")
	slide := nativeMarkdownPPTXSlide{}
	bodyLines := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || nativeMarkdownIsThematicBreak(line) {
			bodyLines = append(bodyLines, "")
			continue
		}
		if title, level, ok := nativeMarkdownHeading(line); ok {
			if level == 1 && title == deckTitle {
				continue
			}
			if strings.TrimSpace(slide.Title) == "" {
				slide.Title = title
				continue
			}
			bodyLines = append(bodyLines, title)
			continue
		}
		if nativeMarkdownIsFenceStart(line) {
			bodyLines = append(bodyLines, line)
			i++
			for ; i < len(lines); i++ {
				codeLine := strings.TrimSpace(lines[i])
				bodyLines = append(bodyLines, lines[i])
				if nativeMarkdownIsFenceEnd(codeLine) {
					break
				}
			}
			continue
		}
		if value, ok := nativeMarkdownMetadataValue(line, "title", "标题"); ok {
			if strings.TrimSpace(slide.Title) == "" {
				slide.Title = value
				continue
			}
			slide.Lines = append(slide.Lines, value)
			continue
		}
		if value, ok := nativeMarkdownMetadataValue(line, "subtitle", "副标题", "summary", "摘要"); ok {
			slide.Lines = append(slide.Lines, value)
			continue
		}
		bodyLines = append(bodyLines, lines[i])
	}

	parsed := officemd.ParseDocument(strings.TrimSpace(strings.Join(bodyLines, "\n")))
	if strings.TrimSpace(slide.Title) == "" {
		switch {
		case len(parsed.Sections) > 0 && strings.TrimSpace(parsed.Sections[0].Heading) != "":
			slide.Title = strings.TrimSpace(parsed.Sections[0].Heading)
		case strings.TrimSpace(parsed.Title) != "" && strings.TrimSpace(parsed.Title) != strings.TrimSpace(deckTitle):
			slide.Title = strings.TrimSpace(parsed.Title)
		}
	}
	slide.Lines = append(slide.Lines, nativeMarkdownLinesFromParsedDoc(parsed)...)
	if strings.TrimSpace(slide.Title) == "" {
		switch {
		case len(slide.Lines) > 0:
			slide.Title = firstNonEmptyDeckValue(deckTitle, "Slide")
		case strings.TrimSpace(deckTitle) != "":
			slide.Title = deckTitle
		}
	}
	slide.Lines = nativeMarkdownCompactLines(slide.Lines)
	return slide
}

func nativeMarkdownLinesFromParsedDoc(spec officemd.DocSpec) []string {
	lines := make([]string, 0, 12)
	if text := strings.TrimSpace(spec.Subtitle); text != "" {
		lines = append(lines, nativeMarkdownNormalizeInlineText(text))
	}
	if text := strings.TrimSpace(spec.Summary); text != "" {
		lines = append(lines, nativeMarkdownNormalizeInlineText(text))
	}
	lines = append(lines, nativeMarkdownLinesFromBlocks(spec.ParagraphBlocks)...)
	for _, paragraph := range spec.Paragraphs {
		if text := nativeMarkdownNormalizeInlineText(paragraph); text != "" {
			lines = append(lines, text)
		}
	}
	for _, section := range spec.Sections {
		lines = append(lines, nativeMarkdownLinesFromSection(section)...)
	}
	return nativeMarkdownCompactLines(lines)
}

func nativeMarkdownLinesFromSection(section officemd.Section) []string {
	lines := make([]string, 0, len(section.Bullets)+len(section.Paragraphs)+4)
	for _, block := range section.ParagraphBlocks {
		lines = append(lines, nativeMarkdownLinesFromBlocks([]officemd.Block{block})...)
	}
	for _, paragraph := range section.Paragraphs {
		if text := nativeMarkdownNormalizeInlineText(paragraph); text != "" {
			lines = append(lines, text)
		}
	}
	for _, bullet := range section.Bullets {
		if text := officemd.RenderedListLine(bullet, "- "); strings.TrimSpace(text) != "" {
			lines = append(lines, nativeMarkdownNormalizeInlineText(text))
		}
	}
	if section.Table != nil {
		if len(section.Table.Headers) > 0 {
			lines = append(lines, strings.Join(section.Table.Headers, " | "))
		}
		for _, row := range section.Table.Rows {
			lines = append(lines, strings.Join(row, " | "))
		}
	}
	return nativeMarkdownCompactLines(lines)
}

func nativeMarkdownLinesFromBlocks(blocks []officemd.Block) []string {
	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch block.Kind {
		case officemd.BlockParagraph, officemd.BlockQuote, officemd.BlockCode:
			for _, part := range strings.Split(block.Text, "\n") {
				if text := nativeMarkdownNormalizeInlineText(part); text != "" {
					lines = append(lines, text)
				}
			}
		case officemd.BlockImage:
			if text := nativeMarkdownNormalizeInlineText(block.Text); text != "" {
				lines = append(lines, text)
			}
		case officemd.BlockSeparator:
			// Slide-level separators are handled before parsing; inline separators are skipped in body text.
		}
	}
	return nativeMarkdownCompactLines(lines)
}

func nativeMarkdownHeading(line string) (string, int, bool) {
	line = strings.TrimSpace(line)
	for level := 1; level <= 6; level++ {
		prefix := strings.Repeat("#", level) + " "
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):]), level, true
		}
	}
	return "", 0, false
}

func nativeMarkdownIsThematicBreak(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 3 {
		return false
	}
	marker := line[0]
	if marker != '-' && marker != '*' && marker != '_' {
		return false
	}
	count := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case marker:
			count++
		case ' ', '\t':
		default:
			return false
		}
	}
	return count >= 3
}

func nativeMarkdownIsFenceStart(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func nativeMarkdownIsFenceEnd(line string) bool {
	line = strings.TrimSpace(line)
	return line == "```" || line == "~~~"
}

func nativeMarkdownMetadataValue(line string, keys ...string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	lower := strings.ToLower(line)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		for _, separator := range []string{":", "："} {
			prefix := strings.ToLower(key) + separator
			if strings.HasPrefix(lower, prefix) {
				value := strings.TrimSpace(line[len(prefix):])
				if value != "" {
					return nativeMarkdownNormalizeInlineText(value), true
				}
			}
		}
	}
	return "", false
}

func nativeMarkdownTableRow(line string) ([]string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.Contains(line, "|") {
		return nil, false
	}
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, nativeMarkdownNormalizeInlineText(strings.TrimSpace(part)))
	}
	return out, len(out) > 0
}

func nativeMarkdownIsTableDividerRow(row []string) bool {
	if len(row) == 0 {
		return false
	}
	for _, cell := range row {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			return false
		}
		for _, ch := range cell {
			if ch != '-' && ch != ':' {
				return false
			}
		}
	}
	return true
}

func nativeMarkdownBulletLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "- [x] "), strings.HasPrefix(line, "- [X] "), strings.HasPrefix(line, "* [x] "), strings.HasPrefix(line, "* [X] "):
		return "[x] " + nativeMarkdownNormalizeInlineText(strings.TrimSpace(line[6:])), true
	case strings.HasPrefix(line, "- [ ] "), strings.HasPrefix(line, "* [ ] "):
		return "[ ] " + nativeMarkdownNormalizeInlineText(strings.TrimSpace(line[6:])), true
	case strings.HasPrefix(line, "- "), strings.HasPrefix(line, "* "), strings.HasPrefix(line, "+ "):
		return "- " + nativeMarkdownNormalizeInlineText(strings.TrimSpace(line[2:])), true
	case nativeMarkdownOrderedPattern.MatchString(line):
		prefix := nativeMarkdownOrderedPattern.FindString(line)
		return prefix + nativeMarkdownNormalizeInlineText(strings.TrimSpace(strings.TrimPrefix(line, prefix))), true
	default:
		return "", false
	}
}

func nativeMarkdownQuoteLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, ">") {
		return "", false
	}
	return nativeMarkdownNormalizeInlineText(strings.TrimSpace(strings.TrimPrefix(line, ">"))), true
}

func nativeMarkdownNormalizeInlineText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = nativeMarkdownImagePattern.ReplaceAllString(text, "$1")
	text = nativeMarkdownLinkPattern.ReplaceAllString(text, "$1")
	for _, pattern := range nativeMarkdownStrongPatterns {
		text = pattern.ReplaceAllString(text, "$1")
	}
	for _, pattern := range nativeMarkdownEmphasisPattern {
		text = pattern.ReplaceAllString(text, "$1")
	}
	text = strings.ReplaceAll(text, `\|`, "|")
	text = strings.TrimSpace(text)
	return text
}

func nativeMarkdownCompactLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func buildNativeMarkdownPPTX(slides []nativeMarkdownPPTXSlide) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	addString := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		return nil
	}

	if err := addString("[Content_Types].xml", nativeMarkdownPPTXContentTypesXML(len(slides))); err != nil {
		return nil, err
	}
	if err := addString("_rels/.rels", nativeMarkdownPPTXPackageRelsXML()); err != nil {
		return nil, err
	}
	if err := addString("docProps/core.xml", nativeMarkdownPPTXCorePropsXML(firstNonEmptyDeckValue(slides[0].Title, "Presentation"))); err != nil {
		return nil, err
	}
	if err := addString("docProps/app.xml", nativeMarkdownPPTXAppPropsXML(slides)); err != nil {
		return nil, err
	}
	if err := addString("ppt/presentation.xml", nativeMarkdownPPTXPresentationXML(len(slides))); err != nil {
		return nil, err
	}
	if err := addString("ppt/_rels/presentation.xml.rels", nativeMarkdownPPTXPresentationRelsXML(len(slides))); err != nil {
		return nil, err
	}
	if err := addString("ppt/slideMasters/slideMaster1.xml", nativeMarkdownPPTXSlideMasterXML()); err != nil {
		return nil, err
	}
	if err := addString("ppt/slideMasters/_rels/slideMaster1.xml.rels", nativeMarkdownPPTXSlideMasterRelsXML()); err != nil {
		return nil, err
	}
	if err := addString("ppt/slideLayouts/slideLayout1.xml", nativeMarkdownPPTXSlideLayoutXML()); err != nil {
		return nil, err
	}
	if err := addString("ppt/slideLayouts/_rels/slideLayout1.xml.rels", nativeMarkdownPPTXSlideLayoutRelsXML()); err != nil {
		return nil, err
	}
	if err := addString("ppt/theme/theme1.xml", nativeMarkdownPPTXThemeXML()); err != nil {
		return nil, err
	}
	for i, slide := range slides {
		if err := addString(fmt.Sprintf("ppt/slides/slide%d.xml", i+1), nativeMarkdownPPTXSlideXML(slide)); err != nil {
			return nil, err
		}
		if err := addString(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1), nativeMarkdownPPTXSlideRelsXML()); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close pptx zip: %w", err)
	}
	return buf.Bytes(), nil
}

func nativeMarkdownPPTXContentTypesXML(slideCount int) string {
	var overrides strings.Builder
	for i := 1; i <= slideCount; i++ {
		overrides.WriteString(`<Override PartName="/ppt/slides/slide`)
		overrides.WriteString(strconv.Itoa(i))
		overrides.WriteString(`.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>` +
		`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
		overrides.String() +
		`</Types>`
}

func nativeMarkdownPPTXPackageRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
}

func nativeMarkdownPPTXCorePropsXML(title string) string {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:title>` + nativeMarkdownPPTXXMLText(title) + `</dc:title>` +
		`<dc:subject>Native markdown PPTX fallback generated by ZimaOS Blue</dc:subject>` +
		`<dc:creator>ZimaOS Blue</dc:creator>` +
		`<cp:lastModifiedBy>ZimaOS Blue</cp:lastModifiedBy>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created>` +
		`<dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified>` +
		`</cp:coreProperties>`
}

func nativeMarkdownPPTXAppPropsXML(slides []nativeMarkdownPPTXSlide) string {
	var parts strings.Builder
	for _, slide := range slides {
		parts.WriteString("<vt:lpstr>")
		parts.WriteString(nativeMarkdownPPTXXMLText(firstNonEmptyDeckValue(slide.Title, "Slide")))
		parts.WriteString("</vt:lpstr>")
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">` +
		`<Application>ZimaOS Blue</Application>` +
		`<PresentationFormat>Custom</PresentationFormat>` +
		`<Slides>` + strconv.Itoa(len(slides)) + `</Slides>` +
		`<Notes>0</Notes><HiddenSlides>0</HiddenSlides><MMClips>0</MMClips><ScaleCrop>false</ScaleCrop>` +
		`<HeadingPairs><vt:vector size="2" baseType="variant"><vt:variant><vt:lpstr>Slides</vt:lpstr></vt:variant><vt:variant><vt:i4>` + strconv.Itoa(len(slides)) + `</vt:i4></vt:variant></vt:vector></HeadingPairs>` +
		`<TitlesOfParts><vt:vector size="` + strconv.Itoa(len(slides)) + `" baseType="lpstr">` + parts.String() + `</vt:vector></TitlesOfParts>` +
		`</Properties>`
}

func nativeMarkdownPPTXPresentationXML(slideCount int) string {
	var slideIDs strings.Builder
	for i := 1; i <= slideCount; i++ {
		slideIDs.WriteString(`<p:sldId id="`)
		slideIDs.WriteString(strconv.Itoa(255 + i))
		slideIDs.WriteString(`" r:id="rId`)
		slideIDs.WriteString(strconv.Itoa(i + 1))
		slideIDs.WriteString(`"/>`)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
		`<p:sldIdLst>` + slideIDs.String() + `</p:sldIdLst>` +
		`<p:sldSz cx="12192000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/>` +
		`</p:presentation>`
}

func nativeMarkdownPPTXPresentationRelsXML(slideCount int) string {
	var rels strings.Builder
	rels.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for i := 1; i <= slideCount; i++ {
		rels.WriteString(`<Relationship Id="rId`)
		rels.WriteString(strconv.Itoa(i + 1))
		rels.WriteString(`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide`)
		rels.WriteString(strconv.Itoa(i))
		rels.WriteString(`.xml"/>`)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		rels.String() +
		`</Relationships>`
}

func nativeMarkdownPPTXSlideMasterXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld name="Blue Master"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld>` +
		`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
		`<p:sldLayoutIdLst><p:sldLayoutId id="1" r:id="rId1"/></p:sldLayoutIdLst>` +
		`</p:sldMaster>`
}

func nativeMarkdownPPTXSlideMasterRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
		`</Relationships>`
}

func nativeMarkdownPPTXSlideLayoutXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">` +
		`<p:cSld name="Blank"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`
}

func nativeMarkdownPPTXSlideLayoutRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`
}

func nativeMarkdownPPTXThemeXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Analysis Theme">` +
		`<a:themeElements>` +
		`<a:clrScheme name="Analysis"><a:dk1><a:srgbClr val="0D4A8A"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="475569"/></a:dk2><a:lt2><a:srgbClr val="F8FAFC"/></a:lt2><a:accent1><a:srgbClr val="1A6FC4"/></a:accent1><a:accent2><a:srgbClr val="0EA5E9"/></a:accent2><a:accent3><a:srgbClr val="64748B"/></a:accent3><a:accent4><a:srgbClr val="166534"/></a:accent4><a:accent5><a:srgbClr val="B45309"/></a:accent5><a:accent6><a:srgbClr val="B91C1C"/></a:accent6><a:hlink><a:srgbClr val="0EA5E9"/></a:hlink><a:folHlink><a:srgbClr val="64748B"/></a:folHlink></a:clrScheme>` +
		`<a:fontScheme name="Analysis"><a:majorFont><a:latin typeface="Aptos Display"/></a:majorFont><a:minorFont><a:latin typeface="Aptos"/></a:minorFont></a:fontScheme>` +
		`<a:fmtScheme name="Analysis"><a:fillStyleLst><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="9525"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle/></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme>` +
		`</a:themeElements></a:theme>`
}

func nativeMarkdownPPTXSlideXML(slide nativeMarkdownPPTXSlide) string {
	const (
		slideWidth      = 12192000
		slideHeight     = 6858000
		bandHeight      = 548640
		accentWidth     = 914400
		accentHeight    = 182880
		accentRightPad  = 685800
		accentTopOffset = 182880
		titleX          = 685800
		titleY          = 731520
		titleW          = 10858500
		titleH          = 731520
		bodyX           = 685800
		bodyY           = 1737360
		bodyW           = 10858500
		bodyH           = 4343400
	)
	accentX := slideWidth - accentRightPad - accentWidth
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		nativeMarkdownPPTXDecorativeRectShapeXML(10, "Background", 0, 0, slideWidth, slideHeight, "F8FAFC") +
		nativeMarkdownPPTXDecorativeRectShapeXML(11, "Band", 0, 0, slideWidth, bandHeight, "EAF4FF") +
		nativeMarkdownPPTXDecorativeRectShapeXML(12, "Accent", accentX, accentTopOffset, accentWidth, accentHeight, "0EA5E9") +
		nativeMarkdownPPTXTextBoxShapeXML(20, "Title", titleX, titleY, titleW, titleH, nativeMarkdownPPTXParagraphsXML([]string{firstNonEmptyDeckValue(slide.Title, "Slide")}, 2800, true, "Aptos Display", "0D4A8A")) +
		nativeMarkdownPPTXTextBoxShapeXML(21, "Content", bodyX, bodyY, bodyW, bodyH, nativeMarkdownPPTXParagraphsXML(slide.Lines, 2200, false, "Aptos", "475569")) +
		`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
}

func nativeMarkdownPPTXDecorativeRectShapeXML(id int, name string, x, y, cx, cy int, fill string) string {
	return `<p:sp><p:nvSpPr><p:cNvPr id="` +
		strconv.Itoa(id) +
		`" name="` + nativeMarkdownPPTXXMLText(name) +
		`"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="` +
		strconv.Itoa(x) +
		`" y="` + strconv.Itoa(y) +
		`"/><a:ext cx="` + strconv.Itoa(cx) +
		`" cy="` + strconv.Itoa(cy) +
		`"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="` +
		nativeMarkdownPPTXHex(fill) +
		`"/></a:solidFill><a:noLn/></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody></p:sp>`
}

func nativeMarkdownPPTXTextBoxShapeXML(id int, name string, x, y, cx, cy int, bodyXML string) string {
	if strings.TrimSpace(bodyXML) == "" {
		bodyXML = `<a:p/>`
	}
	return `<p:sp><p:nvSpPr><p:cNvPr id="` +
		strconv.Itoa(id) +
		`" name="` + nativeMarkdownPPTXXMLText(name) +
		`"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="` +
		strconv.Itoa(x) +
		`" y="` + strconv.Itoa(y) +
		`"/><a:ext cx="` + strconv.Itoa(cx) +
		`" cy="` + strconv.Itoa(cy) +
		`"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:noLn/></p:spPr><p:txBody><a:bodyPr wrap="square" lIns="137160" rIns="137160" tIns="91440" bIns="91440"/><a:lstStyle/>` +
		bodyXML +
		`</p:txBody></p:sp>`
}

func nativeMarkdownPPTXParagraphsXML(lines []string, size int, bold bool, font, color string) string {
	if len(lines) == 0 {
		return `<a:p>` + nativeMarkdownPPTXParagraphPropertiesXML(font, color, size) + `</a:p>`
	}
	var sb strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sb.WriteString(`<a:p>`)
		sb.WriteString(nativeMarkdownPPTXParagraphPropertiesXML(font, color, size))
		sb.WriteString(`<a:r><a:rPr lang="en-US"`)
		if size > 0 {
			sb.WriteString(` sz="`)
			sb.WriteString(strconv.Itoa(size))
			sb.WriteString(`"`)
		}
		if bold {
			sb.WriteString(` b="1"`)
		}
		sb.WriteString(`>`)
		if strings.TrimSpace(font) != "" {
			sb.WriteString(`<a:latin typeface="`)
			sb.WriteString(nativeMarkdownPPTXXMLText(font))
			sb.WriteString(`"/>`)
		}
		sb.WriteString(`</a:rPr><a:t>`)
		sb.WriteString(nativeMarkdownPPTXXMLText(line))
		sb.WriteString(`</a:t></a:r></a:p>`)
	}
	if sb.Len() == 0 {
		return `<a:p>` + nativeMarkdownPPTXParagraphPropertiesXML(font, color, size) + `</a:p>`
	}
	return sb.String()
}

func nativeMarkdownPPTXParagraphPropertiesXML(font, color string, size int) string {
	var sb strings.Builder
	sb.WriteString(`<a:pPr><a:defRPr`)
	if size > 0 {
		sb.WriteString(` sz="`)
		sb.WriteString(strconv.Itoa(size))
		sb.WriteString(`"`)
	}
	sb.WriteString(`>`)
	if strings.TrimSpace(font) != "" {
		sb.WriteString(`<a:latin typeface="`)
		sb.WriteString(nativeMarkdownPPTXXMLText(font))
		sb.WriteString(`"/>`)
	}
	if hex := nativeMarkdownPPTXHex(color); hex != "" {
		sb.WriteString(`<a:solidFill><a:srgbClr val="`)
		sb.WriteString(hex)
		sb.WriteString(`"/></a:solidFill>`)
	}
	sb.WriteString(`</a:defRPr></a:pPr>`)
	return sb.String()
}

func nativeMarkdownPPTXSlideRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
		`</Relationships>`
}

func nativeMarkdownPPTXXMLText(value string) string {
	return html.EscapeString(strings.TrimSpace(value))
}

func nativeMarkdownPPTXHex(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "#"))
	return strings.ToUpper(value)
}

func firstNonEmptyDeckValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
