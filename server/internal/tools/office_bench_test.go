package tools

import (
	"fmt"
	"io"
	"testing"
)

func BenchmarkBuildOfficeDOCX(b *testing.B) {
	spec := benchmarkOfficeDocSpec()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := buildOfficeDOCX(spec); err != nil {
			b.Fatalf("buildOfficeDOCX failed: %v", err)
		}
	}
}

func BenchmarkBuildOfficeXLSX(b *testing.B) {
	spec := benchmarkOfficeWorkbookSpec()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := buildOfficeXLSX(spec); err != nil {
			b.Fatalf("buildOfficeXLSX failed: %v", err)
		}
	}
}

func BenchmarkBuildOfficePPTX(b *testing.B) {
	spec := benchmarkOfficeDocSpec()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := buildOfficePPTX(spec); err != nil {
			b.Fatalf("buildOfficePPTX failed: %v", err)
		}
	}
}

func BenchmarkOfficeWriteXLSXDataSheet(b *testing.B) {
	spec := benchmarkOfficeWorkbookSpec()
	sheet := spec.Sheets[0]
	used := map[string]int{}
	_, columns := officePrepareDataSheet(sheet.Name, sheet, used)
	freeze, autoFilter := officeDataSheetViewSettings(sheet, len(columns), len(sheet.Rows)+1)
	widths := officeDataSheetWidths(columns, sheet.Rows)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := officeWriteXLSXDataSheet(io.Discard, spec.Theme, columns, widths, freeze, autoFilter, sheet.Rows); err != nil {
			b.Fatalf("officeWriteXLSXDataSheet failed: %v", err)
		}
	}
}

func benchmarkOfficeDocSpec() officeDocSpec {
	sections := make([]officeDocSection, 0, 10)
	for sectionIndex := 0; sectionIndex < 10; sectionIndex++ {
		paragraphs := make([]string, 0, 8)
		bullets := make([]string, 0, 6)
		rows := make([][]string, 0, 6)
		for paragraphIndex := 0; paragraphIndex < 8; paragraphIndex++ {
			paragraphs = append(paragraphs, fmt.Sprintf("Section %d paragraph %d explains the rollout plan, tradeoffs, metrics, and operational notes in a native Office benchmark payload.", sectionIndex+1, paragraphIndex+1))
		}
		for bulletIndex := 0; bulletIndex < 6; bulletIndex++ {
			bullets = append(bullets, fmt.Sprintf("Benchmark bullet %d for section %d covering latency, fidelity, caching, and throughput details.", bulletIndex+1, sectionIndex+1))
		}
		for rowIndex := 0; rowIndex < 6; rowIndex++ {
			rows = append(rows, []string{
				fmt.Sprintf("Metric %d", rowIndex+1),
				fmt.Sprintf("%d", 80+rowIndex+sectionIndex),
				"Stable",
			})
		}
		sections = append(sections, officeDocSection{
			Heading:    fmt.Sprintf("Section %d", sectionIndex+1),
			Paragraphs: paragraphs,
			Bullets:    bullets,
			Table: &officeTableSpec{
				Headers: []string{"Name", "Value", "Status"},
				Rows:    rows,
			},
		})
	}
	return officeDocSpec{
		Title:      "Native Office Benchmark Report",
		Subtitle:   "Shared generator performance fixture",
		Summary:    "This synthetic report is intentionally dense so benchmark runs exercise the OOXML builders with repeated sections, paragraphs, bullets, and tables.",
		Theme:      resolveOfficeTheme("analysis", ""),
		StyleHint:  "benchmark",
		Sections:   sections,
		Paragraphs: []string{"Overview paragraph one with moderate length.", "Overview paragraph two with moderate length.", "Overview paragraph three with moderate length."},
		Notes:      []string{"Generated for benchmark use.", "Measures shared OOXML generation overhead."},
	}
}

func benchmarkOfficeWorkbookSpec() officeWorkbookSpec {
	sheets := make([]officeSheetSpec, 0, 3)
	for sheetIndex := 0; sheetIndex < 3; sheetIndex++ {
		columns := []officeColumnSpec{
			{Header: "Team", Key: "team", Kind: "text", Width: 18},
			{Header: "Metric", Key: "metric", Kind: "text", Width: 18},
			{Header: "Value", Key: "value", Kind: "number", Width: 14},
			{Header: "Delta", Key: "delta", Kind: "delta", Width: 14},
			{Header: "Status", Key: "status", Kind: "tone", Width: 16},
			{Header: "Notes", Key: "notes", Kind: "wrap", Width: 30},
		}
		rows := make([][]interface{}, 0, 400)
		for rowIndex := 0; rowIndex < 400; rowIndex++ {
			rows = append(rows, []interface{}{
				fmt.Sprintf("Team %d", (rowIndex%12)+1),
				fmt.Sprintf("Metric %d", rowIndex+1),
				float64((sheetIndex+1)*(rowIndex+10)) / 3,
				fmt.Sprintf("+%d%%", (rowIndex%15)+1),
				[]string{"Good", "Warning", "Critical"}[rowIndex%3],
				fmt.Sprintf("Row %d note for sheet %d with enough text to trigger wrapping and formatting logic.", rowIndex+1, sheetIndex+1),
			})
		}
		sheets = append(sheets, officeSheetSpec{
			Name:    fmt.Sprintf("Sheet %d", sheetIndex+1),
			Columns: columns,
			Rows:    rows,
			Freeze:  "A2",
			Filter:  true,
		})
	}
	return officeWorkbookSpec{
		Title:    "Native Office Benchmark Workbook",
		Subtitle: "Shared workbook performance fixture",
		Theme:    resolveOfficeTheme("analysis", ""),
		Stats: []officeStat{
			{Label: "Rows", Value: "1200", Tone: "success"},
			{Label: "Sheets", Value: "3", Tone: "muted"},
		},
		Notes:  []string{"Generated for benchmark use."},
		Sheets: sheets,
	}
}
