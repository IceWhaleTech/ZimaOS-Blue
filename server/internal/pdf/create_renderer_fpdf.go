//go:build !darwin

package pdf

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

type createFPDFTextMeasurer struct {
	doc *fpdf.Fpdf
}

func (m createFPDFTextMeasurer) CellMargin() float64 {
	if m.doc == nil {
		return 0
	}
	return m.doc.GetCellMargin()
}

func (m createFPDFTextMeasurer) MeasureText(text string) float64 {
	if m.doc == nil {
		return 0
	}
	return m.doc.GetStringWidth(text)
}

func (createFPDFRenderer) render(lines []createStyledLine, fontPlan createFontPlan) ([]byte, int, int, error) {
	return renderCreatePDF(lines, fontPlan)
}

func renderCreatePDF(lines []createStyledLine, fontPlan createFontPlan) ([]byte, int, int, error) {
	doc := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		Size: fpdf.SizeType{
			Wd: createPageWidth,
			Ht: createPageHeight,
		},
	})
	doc.SetCompression(false)
	doc.SetMargins(createMarginLeft, createMarginTop, createMarginLeft)
	doc.SetAutoPageBreak(true, createMarginBottom)
	doc.SetCreator("ZimaOS Blue native_pdf_ir", true)
	if fontPlan.hasUnicodeFont() {
		doc.AddUTF8FontFromBytes(createUnicodeFontFamily, "", fontPlan.unicodeBytes)
		doc.AddUTF8FontFromBytes(createUnicodeFontFamily, "B", fontPlan.unicodeBytes)
	}
	doc.AddPage()

	lineCount := 0
	textWidth := createPageWidth - (createMarginLeft * 2)

	for _, line := range lines {
		if line.Table != nil {
			renderedLines, err := renderCreateTable(doc, line.Table, textWidth, fontPlan)
			if err != nil {
				return nil, 0, 0, err
			}
			lineCount += renderedLines
			if line.GapAfter > 0 {
				doc.Ln(line.GapAfter)
			}
			continue
		}
		if line.Divider != nil {
			renderedLines, err := renderCreateDivider(doc, line.Divider, textWidth)
			if err != nil {
				return nil, 0, 0, err
			}
			lineCount += renderedLines
			if line.GapAfter > 0 {
				doc.Ln(line.GapAfter)
			}
			continue
		}
		if line.ListItem != nil {
			renderedLines, err := renderCreateListItem(doc, line.ListItem, line.Font, line.FontSize, textWidth, fontPlan, line.ColorR, line.ColorG, line.ColorB)
			if err != nil {
				return nil, 0, 0, err
			}
			lineCount += renderedLines
			if line.GapAfter > 0 {
				doc.Ln(line.GapAfter)
			}
			continue
		}

		if strings.TrimSpace(line.Text) == "" {
			doc.Ln(createBlankLineHeight + line.GapAfter)
			continue
		}

		family, style := fontPlan.fontFor(line.Font)
		doc.SetFont(family, style, line.FontSize)
		doc.SetTextColor(line.ColorR, line.ColorG, line.ColorB)
		align := line.Align
		if align == "" {
			align = "L"
		}
		wrapped := createSplitText(doc, line.Text, textWidth)
		if len(wrapped) == 0 {
			wrapped = []string{line.Text}
		}
		lineCount += len(wrapped)
		for _, segment := range wrapped {
			if align == "R" && createHasArabicLetters(segment) {
				segment = createShapeArabicVisual(segment)
			}
			doc.CellFormat(textWidth, line.FontSize*createLineHeightScale, segment, "", 2, align, false, 0, "")
		}
		if line.GapAfter > 0 {
			doc.Ln(line.GapAfter)
		}
	}

	var out bytes.Buffer
	if err := doc.Output(&out); err != nil {
		return nil, 0, 0, err
	}
	return out.Bytes(), doc.PageNo(), lineCount, nil
}

func renderCreateDivider(doc *fpdf.Fpdf, divider *createStyledDivider, textWidth float64) (int, error) {
	if doc == nil || divider == nil {
		return 0, nil
	}

	lineY := doc.GetY() + 3
	if lineY > createPageHeight-createMarginBottom {
		doc.AddPage()
		lineY = doc.GetY() + 3
	}

	left := createMarginLeft + createDividerInset
	right := createMarginLeft + textWidth - createDividerInset
	if right <= left {
		left = createMarginLeft
		right = createMarginLeft + textWidth
	}

	doc.SetDrawColor(divider.LineR, divider.LineG, divider.LineB)
	doc.SetLineWidth(createDividerLineWidth)
	doc.Line(left, lineY, right, lineY)
	doc.SetXY(createMarginLeft, lineY)
	return 1, nil
}

func renderCreateListItem(doc *fpdf.Fpdf, item *createStyledListItem, fontID string, fontSize float64, textWidth float64, fontPlan createFontPlan, textR, textG, textB int) (int, error) {
	if doc == nil || item == nil {
		return 0, nil
	}

	family, style := fontPlan.fontFor(fontID)
	doc.SetFont(family, style, fontSize)
	doc.SetTextColor(textR, textG, textB)

	lineHeight := fontSize * createLineHeightScale
	if doc.GetY()+lineHeight > createPageHeight-createMarginBottom {
		doc.AddPage()
	}

	prepared := prepareCreateListItem(createFPDFTextMeasurer{doc: doc}, *item, textWidth)
	startX := createMarginLeft

	renderSegment := func(segment string) {
		if prepared.Align == "R" && createHasArabicLetters(segment) {
			segment = createShapeArabicVisual(segment)
		}
		doc.CellFormat(prepared.ContentWidth, lineHeight, segment, "", 2, prepared.Align, false, 0, "")
	}

	doc.SetX(startX)
	doc.CellFormat(prepared.MarkerWidth, lineHeight, prepared.Marker, "", 0, "R", false, 0, "")
	renderSegment(prepared.Segments[0])

	for _, segment := range prepared.Segments[1:] {
		doc.SetX(startX + prepared.MarkerWidth)
		renderSegment(segment)
	}

	return len(prepared.Segments), nil
}

func renderCreateTable(doc *fpdf.Fpdf, table *createStyledTable, textWidth float64, fontPlan createFontPlan) (int, error) {
	if table == nil {
		return 0, nil
	}
	columnCount := createTableColumnCount(table)
	if columnCount == 0 {
		return 0, nil
	}

	widths := createTableColumnWidths(columnCount, textWidth)
	lineCount := 0

	renderHeader := func() error {
		if len(table.Headers) == 0 {
			return nil
		}
		count, fits, err := renderCreateTableRow(doc, table.Headers, widths, createFontBold, table.FontSize, fontPlan, table.HeaderTextR, table.HeaderTextG, table.HeaderTextB, table.HeaderFillR, table.HeaderFillG, table.HeaderFillB, table.BorderR, table.BorderG, table.BorderB, true)
		if err != nil {
			return err
		}
		if !fits {
			doc.AddPage()
			count, fits, err = renderCreateTableRow(doc, table.Headers, widths, createFontBold, table.FontSize, fontPlan, table.HeaderTextR, table.HeaderTextG, table.HeaderTextB, table.HeaderFillR, table.HeaderFillG, table.HeaderFillB, table.BorderR, table.BorderG, table.BorderB, true)
			if err != nil {
				return err
			}
			if !fits {
				return fmt.Errorf("pdf table header too tall to fit on a single page")
			}
		}
		lineCount += count
		return nil
	}

	if err := renderHeader(); err != nil {
		return 0, err
	}

	for rowIndex, row := range table.Rows {
		fill := rowIndex%2 == 0
		count, fits, err := renderCreateTableRow(doc, row, widths, createFontRegular, table.FontSize, fontPlan, table.BodyTextR, table.BodyTextG, table.BodyTextB, table.RowFillR, table.RowFillG, table.RowFillB, table.BorderR, table.BorderG, table.BorderB, fill)
		if err != nil {
			return 0, err
		}
		if !fits {
			doc.AddPage()
			if err := renderHeader(); err != nil {
				return 0, err
			}
			count, fits, err = renderCreateTableRow(doc, row, widths, createFontRegular, table.FontSize, fontPlan, table.BodyTextR, table.BodyTextG, table.BodyTextB, table.RowFillR, table.RowFillG, table.RowFillB, table.BorderR, table.BorderG, table.BorderB, fill)
			if err != nil {
				return 0, err
			}
			if !fits {
				return 0, fmt.Errorf("pdf table row too tall to fit on a single page")
			}
		}
		lineCount += count
	}

	return lineCount, nil
}

func renderCreateTableRow(doc *fpdf.Fpdf, cells []createStyledTableCell, widths []float64, fontID string, fontSize float64, fontPlan createFontPlan, textR, textG, textB int, fillR, fillG, fillB int, borderR, borderG, borderB int, fill bool) (int, bool, error) {
	family, style := fontPlan.fontFor(fontID)
	doc.SetFont(family, style, fontSize)

	lineHeight := fontSize * createTableLineHeightScale
	row := prepareCreateTableRow(createFPDFTextMeasurer{doc: doc}, cells, widths, fontSize)
	rowHeight := row.RowHeight
	if doc.GetY()+rowHeight > createPageHeight-createMarginBottom {
		return 0, false, nil
	}

	doc.SetLineWidth(createTableBorderWidth)
	startX := createMarginLeft
	startY := doc.GetY()
	x := startX

	for idx, width := range widths {
		doc.SetDrawColor(borderR, borderG, borderB)
		if fill {
			doc.SetFillColor(fillR, fillG, fillB)
			doc.Rect(x, startY, width, rowHeight, "DF")
		} else {
			doc.Rect(x, startY, width, rowHeight, "D")
		}

		align := row.Aligns[idx]

		doc.SetXY(x+createTableCellPaddingX, startY+createTableCellPaddingY)
		doc.SetTextColor(textR, textG, textB)

		for _, segment := range row.Wrapped[idx] {
			if align == "R" && createHasArabicLetters(segment) {
				segment = createShapeArabicVisual(segment)
			}
			doc.CellFormat(row.InnerWidths[idx], lineHeight, segment, "", 2, align, false, 0, "")
		}

		x += width
		doc.SetXY(x, startY)
	}

	doc.SetXY(createMarginLeft, startY+rowHeight)
	return row.MaxLines, true, nil
}

func createSplitText(doc *fpdf.Fpdf, text string, width float64) []string {
	if doc == nil {
		return nil
	}
	return createSplitTextWithMeasurer(createFPDFTextMeasurer{doc: doc}, text, width)
}
