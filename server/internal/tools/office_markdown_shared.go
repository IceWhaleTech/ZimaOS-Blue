package tools

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/officemd"
)

func officeDocSpecFromMarkdown(spec officemd.DocSpec) officeDocSpec {
	out := officeDocSpec{
		Title:           spec.Title,
		Subtitle:        spec.Subtitle,
		Summary:         spec.Summary,
		ParagraphBlocks: officeDocBlocksFromMarkdown(spec.ParagraphBlocks),
		Paragraphs:      append([]string(nil), spec.Paragraphs...),
		Notes:           append([]string(nil), spec.Notes...),
	}
	for _, section := range spec.Sections {
		out.Sections = append(out.Sections, officeDocSection{
			Heading:         section.Heading,
			ParagraphBlocks: officeDocBlocksFromMarkdown(section.ParagraphBlocks),
			Paragraphs:      append([]string(nil), section.Paragraphs...),
			Bullets:         append([]string(nil), section.Bullets...),
			Table:           officeTableSpecFromMarkdown(section.Table),
		})
	}
	return out
}

func parseMarkdownishOfficeWorkbook(content string) officeWorkbookSpec {
	return officeWorkbookSpecFromMarkdown(officemd.ParseDocument(content))
}

func officeWorkbookSpecFromMarkdown(spec officemd.DocSpec) officeWorkbookSpec {
	out := officeWorkbookSpec{
		Title:    spec.Title,
		Subtitle: spec.Subtitle,
	}
	used := map[string]int{}

	for idx, section := range spec.Sections {
		rawName := strings.TrimSpace(section.Heading)
		if rawName == "" {
			rawName = fmt.Sprintf("Sheet %d", idx+1)
		}
		if section.Table != nil {
			if sheet, ok := officeWorkbookMarkdownTableSheet(rawName, section.Table, used); ok {
				out.Sheets = append(out.Sheets, sheet)
			}
			out.Notes = officeWorkbookAppendUniqueLines(out.Notes, officeWorkbookMarkdownSectionNotes(section)...)
			continue
		}
		if sheet, ok := officeWorkbookMarkdownContentSheet(rawName, officeWorkbookMarkdownSectionLines(section), used); ok {
			out.Sheets = append(out.Sheets, sheet)
		}
	}

	if len(out.Sheets) == 0 {
		if sheet, ok := officeWorkbookMarkdownContentSheet("Content", officeWorkbookMarkdownDocLines(spec), used); ok {
			out.Sheets = append(out.Sheets, sheet)
		}
	}
	out.Notes = officeWorkbookAppendUniqueLines(out.Notes, officeWorkbookMarkdownTopLevelNotes(spec)...)
	return out
}

func officeDocBlocksFromMarkdown(blocks []officemd.Block) []officeDocBlock {
	out := make([]officeDocBlock, 0, len(blocks))
	for _, block := range blocks {
		out = append(out, officeDocBlock{
			Kind:   officeDocBlockKind(block.Kind),
			Text:   block.Text,
			Source: block.Source,
		})
	}
	return out
}

func officeWorkbookMarkdownTableSheet(rawName string, table *officemd.TableSpec, used map[string]int) (officeSheetSpec, bool) {
	if table == nil {
		return officeSheetSpec{}, false
	}
	headers := append([]string(nil), table.Headers...)
	if len(headers) == 0 && len(table.Rows) > 0 {
		headers = make([]string, len(table.Rows[0]))
		for idx := range headers {
			headers[idx] = fmt.Sprintf("Column %d", idx+1)
		}
	}
	if len(headers) == 0 {
		return officeSheetSpec{}, false
	}

	columns := make([]officeColumnSpec, 0, len(headers))
	rows := make([][]interface{}, 0, len(table.Rows))
	for idx, header := range headers {
		column := officeColumnSpec{
			Header: header,
			Key:    header,
		}
		if idx < len(table.ColumnWidths) && table.ColumnWidths[idx] > 0 {
			column.Width = table.ColumnWidths[idx]
		}
		columns = append(columns, column)
	}
	for _, row := range table.Rows {
		copied := make([]interface{}, len(headers))
		for idx := range headers {
			if idx < len(row) {
				copied[idx] = strings.TrimSpace(row[idx])
			}
		}
		rows = append(rows, copied)
	}
	for idx := range columns {
		if strings.TrimSpace(columns[idx].Kind) == "" {
			columns[idx].Kind = inferOfficeColumnKind(columns[idx], rows, idx)
		}
	}

	return officeSheetSpec{
		Name:    officeUniqueSheetName(rawName, used),
		Columns: columns,
		Rows:    rows,
		Freeze:  "A2",
		Filter:  true,
	}, true
}

func officeWorkbookMarkdownContentSheet(rawName string, lines []string, used map[string]int) (officeSheetSpec, bool) {
	lines = officeWorkbookUniqueLines(lines)
	if len(lines) == 0 {
		return officeSheetSpec{}, false
	}
	rows := make([][]interface{}, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, []interface{}{line})
	}
	return officeSheetSpec{
		Name: officeUniqueSheetName(rawName, used),
		Columns: []officeColumnSpec{
			{Header: "Content", Key: "Content", Kind: "wrap", Width: 42},
		},
		Rows:   rows,
		Filter: false,
	}, true
}

func officeWorkbookMarkdownSectionLines(section officemd.Section) []string {
	lines := officeWorkbookMarkdownLinesFromBlocks(section.ParagraphBlocks)
	lines = append(lines, officeWorkbookUniqueLines(section.Paragraphs)...)
	lines = append(lines, officeWorkbookUniqueLines(section.Bullets)...)
	return officeWorkbookUniqueLines(lines)
}

func officeWorkbookMarkdownSectionNotes(section officemd.Section) []string {
	lines := officeWorkbookMarkdownSectionLines(section)
	if len(lines) == 0 {
		return nil
	}
	heading := strings.TrimSpace(section.Heading)
	if heading == "" {
		return []string{strings.Join(lines, " • ")}
	}
	return []string{heading + ": " + strings.Join(lines, " • ")}
}

func officeWorkbookMarkdownTopLevelNotes(spec officemd.DocSpec) []string {
	lines := make([]string, 0, 1+len(spec.Notes)+len(spec.ParagraphBlocks)+len(spec.Paragraphs))
	if summary := strings.TrimSpace(spec.Summary); summary != "" {
		lines = append(lines, summary)
	}
	lines = append(lines, officeWorkbookUniqueLines(spec.Notes)...)
	lines = append(lines, officeWorkbookMarkdownLinesFromBlocks(spec.ParagraphBlocks)...)
	lines = append(lines, officeWorkbookUniqueLines(spec.Paragraphs)...)
	return officeWorkbookUniqueLines(lines)
}

func officeWorkbookMarkdownDocLines(spec officemd.DocSpec) []string {
	lines := make([]string, 0, 1+len(spec.Notes)+len(spec.ParagraphBlocks)+len(spec.Paragraphs))
	if summary := strings.TrimSpace(spec.Summary); summary != "" {
		lines = append(lines, summary)
	}
	lines = append(lines, officeWorkbookUniqueLines(spec.Notes)...)
	lines = append(lines, officeWorkbookMarkdownLinesFromBlocks(spec.ParagraphBlocks)...)
	lines = append(lines, officeWorkbookUniqueLines(spec.Paragraphs)...)
	return officeWorkbookUniqueLines(lines)
}

func officeWorkbookMarkdownLinesFromBlocks(blocks []officemd.Block) []string {
	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if text := officeWorkbookMarkdownBlockText(block); text != "" {
			lines = append(lines, text)
		}
	}
	return officeWorkbookUniqueLines(lines)
}

func officeWorkbookMarkdownBlockText(block officemd.Block) string {
	switch block.Kind {
	case officemd.BlockImage:
		alt := strings.TrimSpace(block.Text)
		source := strings.TrimSpace(block.Source)
		switch {
		case alt != "" && source != "":
			return "Image: " + alt + " (" + source + ")"
		case alt != "":
			return "Image: " + alt
		case source != "":
			return "Image: " + source
		default:
			return ""
		}
	case officemd.BlockSeparator:
		return ""
	default:
		return strings.TrimSpace(block.Text)
	}
}

func officeWorkbookUniqueLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	seen := map[string]struct{}{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func officeWorkbookAppendUniqueLines(base []string, extra ...string) []string {
	return officeWorkbookUniqueLines(append(append([]string(nil), base...), extra...))
}

func officeTableSpecFromMarkdown(table *officemd.TableSpec) *officeTableSpec {
	if table == nil {
		return nil
	}
	out := &officeTableSpec{
		Headers: append([]string(nil), table.Headers...),
		Rows:    make([][]string, 0, len(table.Rows)),
	}
	if len(table.ColumnWidths) > 0 {
		out.ColumnWidths = append([]float64(nil), table.ColumnWidths...)
	}
	if len(table.ColumnAlignments) > 0 {
		out.ColumnAlignments = append([]string(nil), table.ColumnAlignments...)
	}
	for _, row := range table.Rows {
		out.Rows = append(out.Rows, append([]string(nil), row...))
	}
	return out
}
