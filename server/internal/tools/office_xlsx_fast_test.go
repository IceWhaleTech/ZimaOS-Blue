package tools

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestOfficeAppendXLSXCellRefBytesMatchesStringHelper(t *testing.T) {
	tests := []struct {
		name string
		col  int
		row  int
	}{
		{name: "first cell", col: 0, row: 1},
		{name: "last single letter column", col: 25, row: 99},
		{name: "double letter column", col: 26, row: 7},
		{name: "end of double letters", col: 701, row: 1234},
		{name: "triple letter column", col: 702, row: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := officeAppendXLSXCellRefBytes(nil, tt.col, tt.row)
			want := officeXLSXCellRef(tt.col, tt.row)
			if string(got) != want {
				t.Fatalf("officeAppendXLSXCellRefBytes() = %q, want %q", string(got), want)
			}
			if bytes.Equal(got, nil) {
				t.Fatalf("officeAppendXLSXCellRefBytes() returned empty bytes for %s", want)
			}
		})
	}
}

func TestOfficeAppendXLSXCellStartBytesMatchesExpectedTag(t *testing.T) {
	tests := []struct {
		name     string
		row      int
		col      int
		style    int
		cellType string
		expected string
	}{
		{
			name:     "ref only",
			row:      1,
			col:      0,
			expected: `<c r="A1"`,
		},
		{
			name:     "styled numeric cell",
			row:      2,
			col:      1,
			style:    28,
			expected: `<c r="B2" s="28"`,
		},
		{
			name:     "inline string cell",
			row:      7,
			col:      26,
			style:    5,
			cellType: "inlineStr",
			expected: `<c r="AA7" s="5" t="inlineStr"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := officeAppendXLSXCellStartBytes(nil, tt.row, tt.col, tt.style, tt.cellType)
			if string(got) != tt.expected {
				t.Fatalf("officeAppendXLSXCellStartBytes() = %q, want %q", string(got), tt.expected)
			}
		})
	}
}

func TestOfficeXMLTextCanWriteRawASCII(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "plain ascii", text: "Team 12", want: true},
		{name: "needs ampersand escape", text: "Alice & Bob", want: false},
		{name: "needs newline escape", text: "Line\nBreak", want: false},
		{name: "unicode falls back", text: "你好", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := officeXMLTextCanWriteRawASCII(tt.text); got != tt.want {
				t.Fatalf("officeXMLTextCanWriteRawASCII(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestOfficeAcquireStreamBufioWriterResetsTarget(t *testing.T) {
	var first bytes.Buffer
	writer := officeAcquireStreamBufioWriter(&first)
	if _, err := writer.WriteString("hello"); err != nil {
		t.Fatalf("first WriteString error: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("first Flush error: %v", err)
	}
	officeReleaseStreamBufioWriter(writer)

	var second bytes.Buffer
	writer = officeAcquireStreamBufioWriter(&second)
	if _, err := writer.WriteString("world"); err != nil {
		t.Fatalf("second WriteString error: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("second Flush error: %v", err)
	}
	officeReleaseStreamBufioWriter(writer)

	if got := first.String(); got != "hello" {
		t.Fatalf("first buffer = %q, want %q", got, "hello")
	}
	if got := second.String(); got != "world" {
		t.Fatalf("second buffer = %q, want %q", got, "world")
	}
}

func TestOfficeAppendXLSXCellXML(t *testing.T) {
	tests := []struct {
		name     string
		row      int
		col      int
		cell     officeXLSXCell
		expected string
	}{
		{
			name: "formula numeric value",
			row:  2,
			col:  1,
			cell: officeXLSXCell{
				Value:   125.5,
				Formula: "B2*(1+C2)",
				Style:   28,
				Format:  "currency",
			},
			expected: `<c r="B2" s="28"><f>B2*(1+C2)</f><v>125.5</v></c>`,
		},
		{
			name: "inline string preserves escaping",
			row:  1,
			col:  0,
			cell: officeXLSXCell{
				Value:       "Alice & Bob <Team>",
				Style:       5,
				ForceString: true,
			},
			expected: `<c r="A1" s="5" t="inlineStr"><is><t xml:space="preserve">Alice &amp; Bob &lt;Team&gt;</t></is></c>`,
		},
		{
			name: "styled empty cell",
			row:  3,
			col:  2,
			cell: officeXLSXCell{
				Style: 7,
			},
			expected: `<c r="C3" s="7"/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			officeAppendXLSXCellXML(&sb, tt.row, tt.col, tt.cell)
			if got := sb.String(); got != tt.expected {
				t.Fatalf("officeAppendXLSXCellXML() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestOfficeAppendXLSXWorksheetXML(t *testing.T) {
	sheet := officeXLSXSheetBuild{
		Name:       "Metrics",
		Freeze:     "B2",
		AutoFilter: "A1:B2",
		TabColor:   "#336699",
		ColWidths:  []float64{14, 20},
		ColStyles:  []int{5, 11},
		Merges:     []string{"A1:B1"},
		Rows: []officeXLSXRow{
			{
				Height: 24,
				Cells: []officeXLSXCell{
					{Value: "Summary", Style: 5, ForceString: true},
					{Style: 5},
				},
			},
			{
				Height: 20,
				Cells: []officeXLSXCell{
					{Value: "Revenue", Style: 6, ForceString: true},
					{Value: 125.5, Style: 28, Format: "currency"},
				},
			},
		},
	}

	var sb strings.Builder
	officeAppendXLSXWorksheetXML(&sb, sheet)
	got := sb.String()

	for _, needle := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<sheetPr><tabColor rgb="FF336699"/></sheetPr>`,
		`<dimension ref="A1:B2"/>`,
		`<pane xSplit="1" ySplit="1" topLeftCell="B2" activePane="bottomRight" state="frozen"/>`,
		`<selection pane="bottomRight" activeCell="B2" sqref="B2"/>`,
		`<col min="1" max="1" width="14.00" customWidth="1" style="5" customFormat="1"/>`,
		`<col min="2" max="2" width="20.00" customWidth="1" style="11" customFormat="1"/>`,
		`<row r="1" ht="24.00" customHeight="1">`,
		`<c r="A1" s="5" t="inlineStr"><is><t xml:space="preserve">Summary</t></is></c>`,
		`<c r="B1" s="5"/>`,
		`<c r="A2" s="6" t="inlineStr"><is><t xml:space="preserve">Revenue</t></is></c>`,
		`<c r="B2" s="28"><v>125.5</v></c>`,
		`<autoFilter ref="A1:B2"/>`,
		`<mergeCells count="1"><mergeCell ref="A1:B1"/></mergeCells>`,
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("worksheet XML missing %q in %s", needle, got)
		}
	}
}

func TestOfficeBuildDataSheetArtifactMatchesLegacySheetBuilder(t *testing.T) {
	theme := resolveOfficeTheme("analysis", "")
	spec := officeSheetSpec{
		Name: "Metrics",
		Columns: []officeColumnSpec{
			{Header: "Region", Key: "region", Kind: "text"},
			{Header: "Revenue", Key: "revenue", Kind: "number"},
			{Header: "Growth", Key: "growth", Kind: "delta"},
			{Header: "ClosedOn", Key: "closed_on", Kind: "text"},
			{Header: "Notes", Key: "notes", Kind: "wrap"},
		},
		Rows: [][]interface{}{
			{
				"East",
				map[string]interface{}{"value": 1250.5, "format": "currency"},
				"+12%",
				map[string]interface{}{"value": "2024-02-03", "format": "date"},
				"First line\nSecond line",
			},
			{
				"West",
				map[string]interface{}{"value": 990.0, "formula": "B2*0.79", "format": "currency"},
				"-5%",
				map[string]interface{}{"value": "2024-03-01T12:30:00Z", "format": "datetime"},
				"Short note",
			},
		},
		Freeze: "A2",
		Filter: true,
	}

	legacyUsed := map[string]int{}
	legacySheet := officeBuildDataSheet(theme, spec, legacyUsed)

	fastUsed := map[string]int{}
	artifact := officeBuildDataSheetArtifact(theme, spec, fastUsed)

	if artifact.Name != legacySheet.Name {
		t.Fatalf("artifact name = %q, want %q", artifact.Name, legacySheet.Name)
	}
	if artifact.XML != officeXLSXWorksheetXML(legacySheet) {
		t.Fatalf("artifact XML does not match legacy worksheet output")
	}
	if !reflect.DeepEqual(fastUsed, legacyUsed) {
		t.Fatalf("used names = %#v, want %#v", fastUsed, legacyUsed)
	}
}

func TestOfficeBuildDataSheetZipEntryMatchesArtifactXML(t *testing.T) {
	theme := resolveOfficeTheme("analysis", "")
	spec := officeSheetSpec{
		Name: "Metrics",
		Columns: []officeColumnSpec{
			{Header: "Region", Key: "region", Kind: "text"},
			{Header: "Revenue", Key: "revenue", Kind: "number"},
			{Header: "Notes", Key: "notes", Kind: "wrap"},
		},
		Rows: [][]interface{}{
			{"East", 12.5, "One\nTwo"},
			{"West", map[string]interface{}{"value": 15.0, "formula": "B2*1.2", "format": "currency"}, "Short"},
		},
		Freeze: "A2",
		Filter: true,
	}

	usedForArtifact := map[string]int{}
	artifact := officeBuildDataSheetArtifact(theme, spec, usedForArtifact)

	usedForEntry := map[string]int{}
	name, entry := officeBuildDataSheetZipEntry(theme, spec, usedForEntry, "xl/worksheets/sheet9.xml")

	if name != artifact.Name {
		t.Fatalf("stream entry name = %q, want %q", name, artifact.Name)
	}
	if entry.Name != "xl/worksheets/sheet9.xml" {
		t.Fatalf("entry name = %q", entry.Name)
	}
	if entry.SizeHint <= 0 {
		t.Fatalf("entry size hint = %d, want positive", entry.SizeHint)
	}

	zipData, err := officeBuildZip([]officeZipEntry{entry})
	if err != nil {
		t.Fatalf("officeBuildZip() error = %v", err)
	}
	got := officeZipEntryText(t, zipData, "xl/worksheets/sheet9.xml")
	if got != artifact.XML {
		t.Fatalf("streamed XML does not match artifact XML")
	}
	if !reflect.DeepEqual(usedForEntry, usedForArtifact) {
		t.Fatalf("used names = %#v, want %#v", usedForEntry, usedForArtifact)
	}
}
