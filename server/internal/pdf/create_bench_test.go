package pdf

import (
	"fmt"
	"testing"
)

func BenchmarkCreateDocument(b *testing.B) {
	req := benchmarkCreateRequest()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := CreateDocument(req); err != nil {
			b.Fatalf("CreateDocument failed: %v", err)
		}
	}
}

func benchmarkCreateRequest() CreateRequest {
	sections := make([]CreateSection, 0, 12)
	for sectionIndex := 0; sectionIndex < 12; sectionIndex++ {
		paragraphs := make([]string, 0, 8)
		bullets := make([]string, 0, 6)
		rows := make([][]string, 0, 8)
		for paragraphIndex := 0; paragraphIndex < 8; paragraphIndex++ {
			paragraphs = append(paragraphs, fmt.Sprintf("PDF benchmark section %d paragraph %d includes enough text to exercise wrapping, pagination, and PDF object generation work.", sectionIndex+1, paragraphIndex+1))
		}
		for bulletIndex := 0; bulletIndex < 6; bulletIndex++ {
			bullets = append(bullets, fmt.Sprintf("Bullet %d for section %d about throughput, latency, and output stability.", bulletIndex+1, sectionIndex+1))
		}
		for rowIndex := 0; rowIndex < 8; rowIndex++ {
			rows = append(rows, []string{
				fmt.Sprintf("Metric %d", rowIndex+1),
				fmt.Sprintf("%d", 100+rowIndex+sectionIndex),
				"OK",
			})
		}
		sections = append(sections, CreateSection{
			Heading:    fmt.Sprintf("Section %d", sectionIndex+1),
			Paragraphs: paragraphs,
			Bullets:    bullets,
			Table: &CreateTable{
				Headers: []string{"Name", "Value", "Status"},
				Rows:    rows,
			},
		})
	}
	return CreateRequest{
		Title:      "Native PDF Benchmark",
		Subtitle:   "Synthetic throughput fixture",
		Summary:    "This summary is intentionally non-trivial so PDF generation benchmarks spend real time on sanitization, wrapping, pagination, and output assembly.",
		Paragraphs: []string{"Intro paragraph one.", "Intro paragraph two.", "Intro paragraph three."},
		Notes:      []string{"Generated for benchmark use.", "Measures native PDF generator throughput."},
		Sections:   sections,
	}
}
