package officemd

import "testing"

func TestParseMarkdownishDocument(t *testing.T) {
	spec := ParseDocument(`# Quarterly Update
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

func TestParseMarkdownishDocument_PreservesRichBlocks(t *testing.T) {
	spec := ParseDocument("" +
		"# Release Notes\n\n" +
		"> Keep the UX obvious\n" +
		"> Prefer native docs\n\n" +
		"```go\n" +
		"fmt.Println(\"hello\")\n" +
		"fmt.Println(\"world\")\n" +
		"```\n\n" +
		"![Diagram](/tmp/sample.png)\n\n" +
		"---\n")

	if spec.Title != "Release Notes" {
		t.Fatalf("Title = %q, want Release Notes", spec.Title)
	}
	if len(spec.ParagraphBlocks) != 4 {
		t.Fatalf("len(ParagraphBlocks) = %d, want 4 (%#v)", len(spec.ParagraphBlocks), spec.ParagraphBlocks)
	}
	if spec.ParagraphBlocks[0].Kind != BlockQuote || spec.ParagraphBlocks[0].Text != "Keep the UX obvious\nPrefer native docs" {
		t.Fatalf("ParagraphBlocks[0] = %#v, want quote block", spec.ParagraphBlocks[0])
	}
	if spec.ParagraphBlocks[1].Kind != BlockCode || spec.ParagraphBlocks[1].Text != "fmt.Println(\"hello\")\nfmt.Println(\"world\")" {
		t.Fatalf("ParagraphBlocks[1] = %#v, want code block", spec.ParagraphBlocks[1])
	}
	if spec.ParagraphBlocks[2].Kind != BlockImage || spec.ParagraphBlocks[2].Text != "Diagram" || spec.ParagraphBlocks[2].Source != "/tmp/sample.png" {
		t.Fatalf("ParagraphBlocks[2] = %#v, want image block", spec.ParagraphBlocks[2])
	}
	if spec.ParagraphBlocks[3].Kind != BlockSeparator {
		t.Fatalf("ParagraphBlocks[3] = %#v, want separator block", spec.ParagraphBlocks[3])
	}
}
