package convert

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestDocumentReaderReadDocument_DOCXLocalParserPreservesBreaksAndEntities(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rich.docx")
	writeTestOOXMLArchive(t, path, map[string]string{
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>R&amp;D</w:t></w:r><w:r><w:tab/></w:r><w:r><w:t>&lt;Core&gt;</w:t></w:r></w:p>
    <w:p><w:r><w:t>Line 1</w:t></w:r><w:r><w:br/></w:r><w:r><w:t>Line 2</w:t></w:r></w:p>
  </w:body>
</w:document>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	if !strings.Contains(result.Text, "R&D\t<Core>") {
		t.Fatalf("expected decoded entities and tab, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "Line 1\nLine 2") {
		t.Fatalf("expected line break inside paragraph, got %q", result.Text)
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

func TestDocumentReaderReadDocument_PPTXLocalParserPreservesBreaksAndEntities(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rich-deck.pptx")
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody>
    <a:p><a:r><a:t>Plan &amp; Scope</a:t></a:r><a:br/><a:r><a:t>&lt;Draft&gt;</a:t></a:r></a:p>
  </p:txBody></p:sp></p:spTree></p:cSld>
</p:sld>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	if !strings.Contains(result.Text, "Plan & Scope\n<Draft>") {
		t.Fatalf("expected decoded entities and line break, got %q", result.Text)
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserReadsChartParts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chart-deck.pptx")
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:barChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat><c:strLit><c:ptCount val="2"/><c:pt idx="0"><c:v>North</c:v></c:pt><c:pt idx="1"><c:v>South</c:v></c:pt></c:strLit></c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:barChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "North", "South"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserReadsDateChartParts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "date-chart-deck.pptx")
	firstSerial := pptxDateChartTestSerial(t, 2026, time.January, 1)
	secondSerial := pptxDateChartTestSerial(t, 2026, time.February, 1)
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat><c:numLit><c:formatCode>yyyy-mm-dd</c:formatCode><c:ptCount val="2"/><c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt><c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt></c:numLit></c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="yyyy-mm-dd" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "2026-01-01", "2026-02-01"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserReadsMonthYearDateChartParts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "month-year-chart-deck.pptx")
	firstSerial := pptxDateChartTestSerial(t, 2026, time.January, 1)
	secondSerial := pptxDateChartTestSerial(t, 2026, time.February, 1)
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat><c:numLit><c:formatCode>[$-409]mmm-yy</c:formatCode><c:ptCount val="2"/><c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt><c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt></c:numLit></c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "Jan-26", "Feb-26"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
	for _, needle := range []string{"2026-01-01", "2026-02-01"} {
		if strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to avoid ISO date label %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserFallsBackToDateAxisNumFmt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "axis-numfmt-fallback-chart-deck.pptx")
	firstSerial := pptxDateChartTestSerial(t, 2026, time.January, 1)
	secondSerial := pptxDateChartTestSerial(t, 2026, time.February, 1)
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt><c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt></c:numLit></c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "Jan-26", "Feb-26"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
	for _, needle := range []string{"2026-01-01", "2026-02-01"} {
		if strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to avoid ISO date label %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserReadsNumRefDateChartParts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "numref-date-chart-deck.pptx")
	firstSerial := pptxDateChartTestSerial(t, 2026, time.January, 1)
	secondSerial := pptxDateChartTestSerial(t, 2026, time.February, 1)
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat><c:numRef><c:f>Sheet1!$A$2:$A$3</c:f><c:numCache><c:formatCode>[$-409]mmm-yy</c:formatCode><c:ptCount val="2"/><c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt><c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt></c:numCache></c:numRef></c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "Jan-26", "Feb-26"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
	for _, needle := range []string{"2026-01-01", "2026-02-01"} {
		if strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to avoid ISO date label %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserReadsWhitespaceSeparatedNumLitDateChartParts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitespace-numlit-date-chart-deck.pptx")
	firstSerial := pptxDateChartTestSerial(t, 2026, time.January, 1)
	secondSerial := pptxDateChartTestSerial(t, 2026, time.February, 1)
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat>
            <c:numLit>
              <c:formatCode>[$-409]mmm-yy</c:formatCode>
              <c:ptCount val="2"/>
              <c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>
              <c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>
            </c:numLit>
          </c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "Jan-26", "Feb-26"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
	for _, needle := range []string{"2026-01-01", "2026-02-01"} {
		if strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to avoid ISO date label %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserReadsWhitespaceSeparatedNumRefDateChartParts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "whitespace-numref-date-chart-deck.pptx")
	firstSerial := pptxDateChartTestSerial(t, 2026, time.January, 1)
	secondSerial := pptxDateChartTestSerial(t, 2026, time.February, 1)
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat>
            <c:numRef>
              <c:f>Sheet1!$A$2:$A$3</c:f>
              <c:numCache>
                <c:formatCode>[$-409]mmm-yy</c:formatCode>
                <c:ptCount val="2"/>
                <c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>
                <c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>
              </c:numCache>
            </c:numRef>
          </c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "Jan-26", "Feb-26"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
	for _, needle := range []string{"2026-01-01", "2026-02-01"} {
		if strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to avoid ISO date label %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserFallsBackToEmbeddedWorkbookDateCategories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embedded-workbook-date-chart-deck.pptx")
	firstSerial := pptxDateChartTestSerial(t, 2026, time.January, 1)
	secondSerial := pptxDateChartTestSerial(t, 2026, time.February, 1)
	writeTestOOXMLArchiveBytes(t, path, map[string][]byte{
		"ppt/slides/slide1.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`),
		"ppt/slides/_rels/slide1.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`),
		"ppt/charts/chart1.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f></c:numRef></c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
    <c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>
  </c:chart>
</c:chartSpace>`),
		"ppt/charts/_rels/chart1.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="../embeddings/Microsoft_Excel_Worksheet1.xlsx"/>
</Relationships>`),
		"ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx": buildTestChartWorkbookXLSXBytes(t, "Data", "[$-409]mmm-yy", []string{firstSerial, secondSerial}),
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "Jan-26", "Feb-26"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserFallsBackToEmbeddedWorkbookQuotedSheetDateCategories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embedded-workbook-quoted-sheet-chart-deck.pptx")
	firstSerial := pptxDateChartTestSerial(t, 2026, time.January, 1)
	secondSerial := pptxDateChartTestSerial(t, 2026, time.February, 1)
	writeTestOOXMLArchiveBytes(t, path, map[string][]byte{
		"ppt/slides/slide1.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`),
		"ppt/slides/_rels/slide1.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`),
		"ppt/charts/chart1.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat><c:numRef><c:f>'Revenue Data'!$A$2:$A$3</c:f></c:numRef></c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
    <c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>
  </c:chart>
</c:chartSpace>`),
		"ppt/charts/_rels/chart1.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="../embeddings/Microsoft_Excel_Worksheet1.xlsx"/>
</Relationships>`),
		"ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx": buildTestChartWorkbookXLSXBytes(t, "Revenue Data", "[$-409]mmm-yy", []string{firstSerial, secondSerial}),
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "Jan-26", "Feb-26"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
}

func TestDocumentReaderReadDocument_PPTXLocalParserReadsTimeOnlyDateChartParts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "time-chart-deck.pptx")
	firstSerial := pptxDateChartTestDateTimeSerial(t, 2026, time.January, 1, 9, 30, 0)
	secondSerial := pptxDateChartTestDateTimeSerial(t, 2026, time.January, 1, 15, 45, 0)
	writeTestOOXMLArchive(t, path, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Revenue Trend</a:t></a:r></a:p></p:txBody></p:sp>
    <p:graphicFrame><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId2"/></a:graphicData></a:graphic></p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:lineChart>
        <c:ser>
          <c:tx><c:v>Revenue</c:v></c:tx>
          <c:cat><c:numLit><c:formatCode>hh:mm</c:formatCode><c:ptCount val="2"/><c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt><c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt></c:numLit></c:cat>
          <c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>120</c:v></c:pt><c:pt idx="1"><c:v>98</c:v></c:pt></c:numLit></c:val>
        </c:ser>
      </c:lineChart>
      <c:dateAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="hh:mm" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblOffset val="100"/><c:baseTimeUnit val="days"/></c:dateAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:numFmt formatCode="General" sourceLinked="1"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`,
	})

	reader := NewDocumentReader()
	result, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument failed: %v", err)
	}
	for _, needle := range []string{"Revenue Trend", "Revenue", "09:30", "15:45"} {
		if !strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to include %q, got %q", needle, result.Text)
		}
	}
	for _, needle := range []string{"2026-01-01 09:30", "2026-01-01 15:45"} {
		if strings.Contains(result.Text, needle) {
			t.Fatalf("expected extracted text to avoid full date-time label %q, got %q", needle, result.Text)
		}
	}
}

func pptxDateChartTestSerial(t *testing.T, year int, month time.Month, day int) string {
	t.Helper()
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	ts := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(ts.Sub(base).Hours()/24, 'f', -1, 64), "0"), ".")
}

func pptxDateChartTestDateTimeSerial(t *testing.T, year int, month time.Month, day, hour, minute, second int) string {
	t.Helper()
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	ts := time.Date(year, month, day, hour, minute, second, 0, time.UTC)
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(ts.Sub(base).Hours()/24, 'f', -1, 64), "0"), ".")
}

func writeTestOOXMLArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()

	byteEntries := make(map[string][]byte, len(entries))
	for name, contents := range entries {
		byteEntries[name] = []byte(contents)
	}
	writeTestOOXMLArchiveBytes(t, path, byteEntries)
}

func writeTestOOXMLArchiveBytes(t *testing.T, path string, entries map[string][]byte) {
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
		if _, err := entry.Write(contents); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
}

func buildTestChartWorkbookXLSXBytes(t *testing.T, sheetName, formatCode string, serials []string) []byte {
	t.Helper()

	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` + "\n")
	buf.WriteString(`  <sheetData>` + "\n")
	buf.WriteString(`    <row r="1"><c r="A1" t="inlineStr"><is><t>Date</t></is></c></row>` + "\n")
	for idx, serial := range serials {
		buf.WriteString(`    <row r="` + strconv.Itoa(idx+2) + `"><c r="A` + strconv.Itoa(idx+2) + `" s="1"><v>` + serial + `</v></c></row>` + "\n")
	}
	buf.WriteString(`  </sheetData>` + "\n")
	buf.WriteString(`</worksheet>`)

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	writeEntry := func(name, content string) {
		t.Helper()
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}

	writeEntry("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="`+sheetName+`" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)
	writeEntry("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`)
	writeEntry("xl/styles.xml", `<?xml version="1.0" encoding="UTF-8"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <numFmts count="1">
    <numFmt numFmtId="164" formatCode="`+formatCode+`"/>
  </numFmts>
  <fonts count="1"><font><sz val="11"/><name val="Aptos"/></font></fonts>
  <fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>
  <borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="2">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
  </cellXfs>
</styleSheet>`)
	writeEntry("xl/worksheets/sheet1.xml", buf.String())

	if err := zw.Close(); err != nil {
		t.Fatalf("close xlsx zip: %v", err)
	}
	return out.Bytes()
}
