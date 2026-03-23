package convert

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentReaderReadDocument_DOCXLocalParser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.docx")
	writeTestOOXMLArchive(t, path, map[string]string{
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Quarterly Update</w:t></w:r></w:p>
    <w:p><w:r><w:t>Revenue grew 18 percent.</w:t></w:r></w:p>
  </w:body>
</w:document>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	if result.ExtractedVia != "local_docx" {
		t.Fatalf("ExtractedVia = %q, want %q", result.ExtractedVia, "local_docx")
	}
	if !strings.Contains(result.Text, "Quarterly Update") || !strings.Contains(result.Text, "Revenue grew 18 percent.") {
		t.Fatalf("unexpected text: %q", result.Text)
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deck.pptx")
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody>
    <a:p><a:r><a:t>Launch Plan</a:t></a:r></a:p>
    <a:p><a:r><a:t>Beta in April</a:t></a:r></a:p>
  </p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/slide2.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody>
    <a:p><a:r><a:t>Next Steps</a:t></a:r></a:p>
  </p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	if result.ExtractedVia != "local_pptx" {
		t.Fatalf("ExtractedVia = %q, want %q", result.ExtractedVia, "local_pptx")
	}
	if !strings.Contains(result.Text, "[Slide 1]") || !strings.Contains(result.Text, "Launch Plan") || !strings.Contains(result.Text, "Next Steps") {
		t.Fatalf("unexpected text: %q", result.Text)
	}
}

func writeTestOOXMLArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, contents := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
}
