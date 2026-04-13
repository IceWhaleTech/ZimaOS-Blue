package convert

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkReadDOCXLocal(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.docx")
	writeBenchmarkDOCXArchive(b, path, 600)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := readDOCXLocal(path); err != nil {
			b.Fatalf("readDOCXLocal failed: %v", err)
		}
	}
}

func BenchmarkReadPPTXLocal(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.pptx")
	writeBenchmarkPPTXArchive(b, path, 24, 18)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := readPPTXLocal(path); err != nil {
			b.Fatalf("readPPTXLocal failed: %v", err)
		}
	}
}

func BenchmarkLoadSpreadsheetWorkbook(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.xlsx")
	writeBenchmarkSpreadsheetXLSX(b, path, 900, 10)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loadSpreadsheetWorkbook(path); err != nil {
			b.Fatalf("loadSpreadsheetWorkbook failed: %v", err)
		}
	}
}

func writeBenchmarkDOCXArchive(tb testing.TB, path string, paragraphs int) {
	tb.Helper()
	var document strings.Builder
	document.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	document.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for i := 0; i < paragraphs; i++ {
		document.WriteString(`<w:p><w:r><w:t>`)
		document.WriteString(fmt.Sprintf("Benchmark paragraph %d contains repeated content for native docx read profiling.", i+1))
		document.WriteString(`</w:t></w:r></w:p>`)
	}
	document.WriteString(`</w:body></w:document>`)
	writeBenchmarkOOXMLArchive(tb, path, map[string]string{
		"word/document.xml": document.String(),
	})
}

func writeBenchmarkPPTXArchive(tb testing.TB, path string, slides, linesPerSlide int) {
	tb.Helper()
	entries := make(map[string]string, slides)
	for slideIndex := 0; slideIndex < slides; slideIndex++ {
		var slide strings.Builder
		slide.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		slide.WriteString(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody>`)
		for lineIndex := 0; lineIndex < linesPerSlide; lineIndex++ {
			slide.WriteString(`<a:p><a:r><a:t>`)
			slide.WriteString(fmt.Sprintf("Slide %d line %d for native pptx benchmark parsing.", slideIndex+1, lineIndex+1))
			slide.WriteString(`</a:t></a:r></a:p>`)
		}
		slide.WriteString(`</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`)
		entries[fmt.Sprintf("ppt/slides/slide%d.xml", slideIndex+1)] = slide.String()
	}
	writeBenchmarkOOXMLArchive(tb, path, entries)
}

func writeBenchmarkSpreadsheetXLSX(tb testing.TB, path string, rows, cols int) {
	tb.Helper()
	file, err := os.Create(path)
	if err != nil {
		tb.Fatalf("create xlsx: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	writeZipEntry := func(name, content string) {
		tb.Helper()
		w, err := zw.Create(name)
		if err != nil {
			tb.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			tb.Fatalf("write zip entry %s: %v", name, err)
		}
	}

	var shared strings.Builder
	shared.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	shared.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="`)
	shared.WriteString(fmt.Sprintf("%d", cols))
	shared.WriteString(`" uniqueCount="`)
	shared.WriteString(fmt.Sprintf("%d", cols))
	shared.WriteString(`">`)
	for colIndex := 0; colIndex < cols; colIndex++ {
		shared.WriteString(`<si><t>`)
		shared.WriteString(fmt.Sprintf("Column %d", colIndex+1))
		shared.WriteString(`</t></si>`)
	}
	shared.WriteString(`</sst>`)

	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	sheet.WriteString(`<row r="1">`)
	for colIndex := 0; colIndex < cols; colIndex++ {
		sheet.WriteString(`<c r="`)
		sheet.WriteString(spreadsheetCellRef(colIndex, 1))
		sheet.WriteString(`" t="s"><v>`)
		sheet.WriteString(fmt.Sprintf("%d", colIndex))
		sheet.WriteString(`</v></c>`)
	}
	sheet.WriteString(`</row>`)
	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		sheet.WriteString(`<row r="`)
		sheet.WriteString(fmt.Sprintf("%d", rowIndex+2))
		sheet.WriteString(`">`)
		for colIndex := 0; colIndex < cols; colIndex++ {
			sheet.WriteString(`<c r="`)
			sheet.WriteString(spreadsheetCellRef(colIndex, rowIndex+2))
			sheet.WriteString(`"><v>`)
			sheet.WriteString(fmt.Sprintf("%d", (rowIndex+1)*(colIndex+3)))
			sheet.WriteString(`</v></c>`)
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	writeZipEntry("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Bench" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)
	writeZipEntry("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
</Relationships>`)
	writeZipEntry("xl/sharedStrings.xml", shared.String())
	writeZipEntry("xl/worksheets/sheet1.xml", sheet.String())
	if err := zw.Close(); err != nil {
		tb.Fatalf("close xlsx zip: %v", err)
	}
}

func writeBenchmarkOOXMLArchive(tb testing.TB, path string, entries map[string]string) {
	tb.Helper()

	file, err := os.Create(path)
	if err != nil {
		tb.Fatalf("create archive: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, contents := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			tb.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			tb.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		tb.Fatalf("close zip writer: %v", err)
	}
}
