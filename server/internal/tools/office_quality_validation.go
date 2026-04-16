package tools

import (
	"regexp"
	"strings"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

type officeQualitySignal struct {
	Key   string
	Issue string
	Expr  *regexp.Regexp
}

var (
	officeMarkdownHeadingLeakPattern = regexp.MustCompile(`(?m)(?:^|\n)\s{0,3}#{1,6}\s+\S`)
	officeMarkdownFenceLeakPattern   = regexp.MustCompile("```")
	officeMarkdownImageLeakPattern   = regexp.MustCompile(`!\[[^\]\n]*\]\([^)]+\)`)
	officeMarkdownTableLeakPattern   = regexp.MustCompile(`(?m)^\s*\|?(?:\s*:?-{3,}:?\s*\|){1,}\s*$`)

	officePlaceholderSlideTitlePattern    = regexp.MustCompile(`(?im)^(?:slide|幻灯片)\s*\d+(?:\s*[:：-].*)?$`)
	officePlaceholderSlideTitleXMLPattern = regexp.MustCompile(`(?i)<vt:lpstr>(?:slide|幻灯片)\s*\d+(?:\s*[:：-][^<]*)?</vt:lpstr>`)
	officePPTXSlidePartPattern            = regexp.MustCompile(`(?i)^ppt/slides/slide\d+\.xml$`)
	officePPTXSlideTargetPattern          = regexp.MustCompile(`(?i)Target="slides/(slide\d+\.xml)"`)
)

func validateOfficeArchiveQuality(path, format, readText string) (map[string]interface{}, error) {
	validation := map[string]interface{}{
		"quality_checked":     true,
		"quality_ok":          true,
		"quality_issues":      []string{},
		"quality_issue_count": 0,
	}

	issues := make([]string, 0, 4)
	issueSet := map[string]struct{}{}
	issueCounts := map[string]int{}

	recordIssue := func(issue string, count int) {
		if count <= 0 {
			return
		}
		issueCounts[issue] += count
		if _, exists := issueSet[issue]; exists {
			return
		}
		issueSet[issue] = struct{}{}
		issues = append(issues, issue)
	}

	for _, signal := range officeQualitySignalsForFormat(format) {
		recordIssue(signal.Issue, len(signal.Expr.FindAllStringIndex(readText, -1)))
	}

	if strings.EqualFold(strings.TrimSpace(format), "pptx") {
		entries, err := readZipArchive(path)
		if err != nil {
			return nil, err
		}
		if appProps, ok := officeArchiveEntryText(entries, "docProps/app.xml"); ok {
			recordIssue("placeholder_slide_title_detected", len(officePlaceholderSlideTitleXMLPattern.FindAllStringIndex(appProps, -1)))
		}
		recordIssue("orphan_slide_part_detected", officePPTXOrphanedSlidePartCount(entries))
	}

	for issue, count := range issueCounts {
		validation[issue+"_count"] = count
	}
	if len(issues) > 0 {
		validation["quality_ok"] = false
	}
	validation["quality_issues"] = issues
	validation["quality_issue_count"] = len(issues)
	return validation, nil
}

func officePPTXOrphanedSlidePartCount(entries []zipArchiveEntry) int {
	referenced := make(map[string]struct{}, 8)
	if rels, ok := officeArchiveEntryText(entries, "ppt/_rels/presentation.xml.rels"); ok {
		for _, match := range officePPTXSlideTargetPattern.FindAllStringSubmatch(rels, -1) {
			if len(match) < 2 {
				continue
			}
			referenced["ppt/slides/"+strings.ToLower(strings.TrimSpace(match[1]))] = struct{}{}
		}
	}
	if len(referenced) == 0 {
		return 0
	}
	orphaned := 0
	for _, entry := range entries {
		name := strings.ToLower(strings.TrimSpace(entry.Name))
		if !officePPTXSlidePartPattern.MatchString(name) {
			continue
		}
		if _, ok := referenced[name]; ok {
			continue
		}
		orphaned++
	}
	return orphaned
}

func officeQualitySignalsForFormat(format string) []officeQualitySignal {
	signals := []officeQualitySignal{
		{
			Key:   "raw_markdown_heading",
			Issue: "raw_markdown_heading_detected",
			Expr:  officeMarkdownHeadingLeakPattern,
		},
		{
			Key:   "raw_markdown_fence",
			Issue: "raw_markdown_fence_detected",
			Expr:  officeMarkdownFenceLeakPattern,
		},
		{
			Key:   "raw_markdown_image",
			Issue: "raw_markdown_image_detected",
			Expr:  officeMarkdownImageLeakPattern,
		},
		{
			Key:   "raw_markdown_table_divider",
			Issue: "raw_markdown_table_divider_detected",
			Expr:  officeMarkdownTableLeakPattern,
		},
	}
	if strings.EqualFold(strings.TrimSpace(format), "pptx") {
		signals = append(signals, officeQualitySignal{
			Key:   "placeholder_slide_title",
			Issue: "placeholder_slide_title_detected",
			Expr:  officePlaceholderSlideTitlePattern,
		})
	}
	return signals
}

func officeArchiveEntryText(entries []zipArchiveEntry, target string) (string, bool) {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, entry := range entries {
		if strings.ToLower(strings.TrimSpace(entry.Name)) != target {
			continue
		}
		return string(entry.Data), true
	}
	return "", false
}

func officeValidationRequiredEntries(format string) []string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "docx":
		return []string{
			"[Content_Types].xml",
			"_rels/.rels",
			"word/document.xml",
			"word/styles.xml",
		}
	case "xlsx":
		return []string{
			"[Content_Types].xml",
			"_rels/.rels",
			"xl/workbook.xml",
			"xl/styles.xml",
			"xl/worksheets/sheet1.xml",
		}
	case "pptx":
		return []string{
			"[Content_Types].xml",
			"_rels/.rels",
			"ppt/presentation.xml",
			"ppt/slides/slide1.xml",
		}
	default:
		return nil
	}
}

func validateNativeOfficeArchive(path, format string, read *convertpkg.DocumentReadResult) (map[string]interface{}, error) {
	required := officeValidationRequiredEntries(format)
	if len(required) == 0 {
		return map[string]interface{}{
			"ok": true,
		}, nil
	}

	validation, err := validateZipEntries(path, required)
	if err != nil {
		return nil, err
	}
	if read == nil {
		return validation, nil
	}

	validation["readable"] = strings.TrimSpace(read.Text) != ""
	validation["extracted_via"] = read.ExtractedVia
	validation["char_count"] = len([]rune(read.Text))

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "xlsx":
		if read.TabularSummary != nil {
			validation["tabular_summary"] = read.TabularSummary
		}
	case "pptx":
		if slideCount, err := countZipEntriesWithPrefix(path, "ppt/slides/slide", ".xml"); err == nil {
			validation["slide_count"] = slideCount
		}
	}

	quality, err := validateOfficeArchiveQuality(path, format, read.Text)
	if err != nil {
		validation["quality_checked"] = false
		validation["quality_error"] = err.Error()
		return validation, nil
	}
	mergeValidationFields(validation, quality)
	return validation, nil
}
