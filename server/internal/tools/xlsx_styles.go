package tools

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type xlsxStyleSheetXML struct {
	NumFmts xlsxStyleNumFmtsXML `xml:"numFmts"`
	CellXfs xlsxStyleCellXfsXML `xml:"cellXfs"`
}

type xlsxStyleNumFmtsXML struct {
	Count int               `xml:"count,attr"`
	Items []xlsxStyleNumFmt `xml:"numFmt"`
}

type xlsxStyleNumFmt struct {
	ID   int    `xml:"numFmtId,attr"`
	Code string `xml:"formatCode,attr"`
}

type xlsxStyleCellXfsXML struct {
	Count int           `xml:"count,attr"`
	Items []xlsxStyleXF `xml:"xf"`
}

type xlsxStyleXF struct {
	NumFmtID          int                 `xml:"numFmtId,attr"`
	FontID            int                 `xml:"fontId,attr"`
	FillID            int                 `xml:"fillId,attr"`
	BorderID          int                 `xml:"borderId,attr"`
	XFID              int                 `xml:"xfId,attr"`
	ApplyFont         int                 `xml:"applyFont,attr,omitempty"`
	ApplyFill         int                 `xml:"applyFill,attr,omitempty"`
	ApplyBorder       int                 `xml:"applyBorder,attr,omitempty"`
	ApplyAlignment    int                 `xml:"applyAlignment,attr,omitempty"`
	ApplyNumberFormat int                 `xml:"applyNumberFormat,attr,omitempty"`
	Alignment         *xlsxStyleAlignment `xml:"alignment"`
}

type xlsxStyleAlignment struct {
	Horizontal string `xml:"horizontal,attr,omitempty"`
	Vertical   string `xml:"vertical,attr,omitempty"`
	WrapText   int    `xml:"wrapText,attr,omitempty"`
}

type xlsxMutableStyles struct {
	data             []byte
	xfs              []xlsxStyleXF
	numFmtCodes      map[int]string
	nextCustomNumFmt int
	dirty            bool
	appendedNumFmts  []xlsxStyleNumFmt
	appendedXFs      []xlsxStyleXF
}

func loadMutableXLSXStyles(data []byte) (*xlsxMutableStyles, error) {
	var parsed xlsxStyleSheetXML
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode styles.xml: %w", err)
	}
	numFmtCodes := make(map[int]string, len(parsed.NumFmts.Items))
	maxCustom := 163
	for _, item := range parsed.NumFmts.Items {
		numFmtCodes[item.ID] = strings.TrimSpace(item.Code)
		if item.ID > maxCustom {
			maxCustom = item.ID
		}
	}
	return &xlsxMutableStyles{
		data:             append([]byte(nil), data...),
		xfs:              append([]xlsxStyleXF(nil), parsed.CellXfs.Items...),
		numFmtCodes:      numFmtCodes,
		nextCustomNumFmt: maxCustom + 1,
	}, nil
}

func (s *xlsxMutableStyles) hasStyle(styleID int) bool {
	return s != nil && styleID >= 0 && styleID < len(s.xfs)
}

func (s *xlsxMutableStyles) formatForStyle(styleID int) string {
	if !s.hasStyle(styleID) {
		return ""
	}
	xf := s.xfs[styleID]
	return xlsxClassifyNumFmt(xf.NumFmtID, s.numFmtCodes[xf.NumFmtID])
}

func (s *xlsxMutableStyles) ensureFormatStyle(format string, baseStyleID int) (int, error) {
	if s == nil {
		return 0, nil
	}
	format = officeNormalizeCellFormat(format)
	if format == "" || format == "general" {
		if s.hasStyle(baseStyleID) {
			return baseStyleID, nil
		}
		return 0, nil
	}

	numFmtID, numFmtCode := s.targetNumFmt(format)
	if numFmtCode != "" {
		existingID, ok := s.findOrAppendNumFmt(numFmtCode)
		if !ok {
			return 0, fmt.Errorf("allocate numFmt for %s", format)
		}
		numFmtID = existingID
	}

	if existing := s.findExistingXF(numFmtID, baseStyleID); existing >= 0 {
		return existing, nil
	}

	baseXF := s.baseXF(baseStyleID)
	baseXF.NumFmtID = numFmtID
	baseXF.ApplyNumberFormat = 1
	appended := s.appendXF(baseXF)
	return appended, nil
}

func (s *xlsxMutableStyles) targetNumFmt(format string) (int, string) {
	switch officeNormalizeCellFormat(format) {
	case "integer":
		return 1, ""
	case "decimal":
		return 2, ""
	case "currency":
		return 0, `"$"#,##0.00`
	case "percent":
		return 10, ""
	case "date":
		return 14, ""
	case "datetime":
		return 22, ""
	case "text":
		return 49, ""
	default:
		return 0, ""
	}
}

func (s *xlsxMutableStyles) findOrAppendNumFmt(code string) (int, bool) {
	code = strings.TrimSpace(code)
	for id, existing := range s.numFmtCodes {
		if strings.EqualFold(strings.TrimSpace(existing), code) {
			return id, true
		}
	}
	id := s.nextCustomNumFmt
	s.nextCustomNumFmt++
	s.numFmtCodes[id] = code
	s.appendedNumFmts = append(s.appendedNumFmts, xlsxStyleNumFmt{ID: id, Code: code})
	s.dirty = true
	return id, true
}

func (s *xlsxMutableStyles) findExistingXF(numFmtID, baseStyleID int) int {
	base := s.baseXF(baseStyleID)
	for idx, xf := range s.xfs {
		if xf.NumFmtID != numFmtID {
			continue
		}
		if xf.FontID == base.FontID &&
			xf.FillID == base.FillID &&
			xf.BorderID == base.BorderID &&
			xf.XFID == base.XFID &&
			xf.ApplyFont == base.ApplyFont &&
			xf.ApplyFill == base.ApplyFill &&
			xf.ApplyBorder == base.ApplyBorder &&
			xf.ApplyAlignment == base.ApplyAlignment &&
			xlsxStyleAlignmentEqual(xf.Alignment, base.Alignment) {
			return idx
		}
	}
	return -1
}

func xlsxStyleAlignmentEqual(left, right *xlsxStyleAlignment) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil {
		left = &xlsxStyleAlignment{}
	}
	if right == nil {
		right = &xlsxStyleAlignment{}
	}
	return strings.TrimSpace(left.Horizontal) == strings.TrimSpace(right.Horizontal) &&
		strings.TrimSpace(left.Vertical) == strings.TrimSpace(right.Vertical) &&
		left.WrapText == right.WrapText
}

func (s *xlsxMutableStyles) baseXF(styleID int) xlsxStyleXF {
	if s.hasStyle(styleID) {
		return s.xfs[styleID]
	}
	if len(s.xfs) > 0 {
		return s.xfs[0]
	}
	return xlsxStyleXF{}
}

func (s *xlsxMutableStyles) appendXF(xf xlsxStyleXF) int {
	s.xfs = append(s.xfs, xf)
	s.appendedXFs = append(s.appendedXFs, xf)
	s.dirty = true
	return len(s.xfs) - 1
}

func (s *xlsxMutableStyles) render() ([]byte, error) {
	if s == nil || !s.dirty {
		return s.data, nil
	}
	updated := string(s.data)

	if len(s.appendedNumFmts) > 0 {
		numFmtXML := strings.Builder{}
		for _, item := range s.appendedNumFmts {
			numFmtXML.WriteString(`<numFmt numFmtId="`)
			numFmtXML.WriteString(strconv.Itoa(item.ID))
			numFmtXML.WriteString(`" formatCode="`)
			numFmtXML.WriteString(officeXMLText(item.Code))
			numFmtXML.WriteString(`"/>`)
		}
		numFmtPattern := regexp.MustCompile(`(?s)<numFmts\b([^>]*)count="(\d+)"([^>]*)>(.*?)</numFmts>`)
		if numFmtPattern.MatchString(updated) {
			updated = numFmtPattern.ReplaceAllString(updated, `${0}`)
			updated = numFmtPattern.ReplaceAllStringFunc(updated, func(match string) string {
				sub := numFmtPattern.FindStringSubmatch(match)
				if len(sub) != 5 {
					return match
				}
				count, _ := strconv.Atoi(sub[2])
				return `<numFmts` + sub[1] + `count="` + strconv.Itoa(count+len(s.appendedNumFmts)) + `"` + sub[3] + `>` + sub[4] + numFmtXML.String() + `</numFmts>`
			})
		} else {
			insert := `<numFmts count="` + strconv.Itoa(len(s.appendedNumFmts)) + `">` + numFmtXML.String() + `</numFmts>`
			openPattern := regexp.MustCompile(`(<styleSheet\b[^>]*>)`)
			updated = openPattern.ReplaceAllString(updated, `${1}`+insert)
		}
	}

	if len(s.appendedXFs) > 0 {
		xfXML := strings.Builder{}
		for _, xf := range s.appendedXFs {
			xfXML.WriteString(xlsxStyleXFString(xf))
		}
		cellXfsPattern := regexp.MustCompile(`(?s)<cellXfs\b([^>]*)count="(\d+)"([^>]*)>(.*?)</cellXfs>`)
		if !cellXfsPattern.MatchString(updated) {
			return nil, fmt.Errorf("cellXfs block missing from styles.xml")
		}
		updated = cellXfsPattern.ReplaceAllStringFunc(updated, func(match string) string {
			sub := cellXfsPattern.FindStringSubmatch(match)
			if len(sub) != 5 {
				return match
			}
			count, _ := strconv.Atoi(sub[2])
			return `<cellXfs` + sub[1] + `count="` + strconv.Itoa(count+len(s.appendedXFs)) + `"` + sub[3] + `>` + sub[4] + xfXML.String() + `</cellXfs>`
		})
	}

	return []byte(updated), nil
}

func xlsxStyleXFString(xf xlsxStyleXF) string {
	var sb strings.Builder
	sb.WriteString(`<xf numFmtId="`)
	sb.WriteString(strconv.Itoa(xf.NumFmtID))
	sb.WriteString(`" fontId="`)
	sb.WriteString(strconv.Itoa(xf.FontID))
	sb.WriteString(`" fillId="`)
	sb.WriteString(strconv.Itoa(xf.FillID))
	sb.WriteString(`" borderId="`)
	sb.WriteString(strconv.Itoa(xf.BorderID))
	sb.WriteString(`" xfId="`)
	sb.WriteString(strconv.Itoa(xf.XFID))
	sb.WriteString(`"`)
	if xf.ApplyFont != 0 {
		sb.WriteString(` applyFont="1"`)
	}
	if xf.ApplyFill != 0 {
		sb.WriteString(` applyFill="1"`)
	}
	if xf.ApplyBorder != 0 {
		sb.WriteString(` applyBorder="1"`)
	}
	if xf.ApplyNumberFormat != 0 {
		sb.WriteString(` applyNumberFormat="1"`)
	}
	if xf.ApplyAlignment != 0 || xf.Alignment != nil {
		sb.WriteString(` applyAlignment="1">`)
		if xf.Alignment != nil {
			sb.WriteString(`<alignment`)
			if xf.Alignment.Horizontal != "" {
				sb.WriteString(` horizontal="`)
				sb.WriteString(officeXMLText(xf.Alignment.Horizontal))
				sb.WriteString(`"`)
			}
			if xf.Alignment.Vertical != "" {
				sb.WriteString(` vertical="`)
				sb.WriteString(officeXMLText(xf.Alignment.Vertical))
				sb.WriteString(`"`)
			}
			if xf.Alignment.WrapText != 0 {
				sb.WriteString(` wrapText="1"`)
			}
			sb.WriteString(`/>`)
		}
		sb.WriteString(`</xf>`)
		return sb.String()
	}
	sb.WriteString(`/>`)
	return sb.String()
}

func xlsxClassifyNumFmt(numFmtID int, code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		switch numFmtID {
		case 1:
			return "integer"
		case 2:
			return "decimal"
		case 9, 10:
			return "percent"
		case 14, 15, 16, 17:
			return "date"
		case 18, 19, 20, 21, 22, 45, 46, 47:
			return "datetime"
		case 49:
			return "text"
		}
		return ""
	}
	lower := strings.ToLower(code)
	switch {
	case strings.Contains(lower, "@"):
		return "text"
	case strings.Contains(lower, "%"):
		return "percent"
	case strings.ContainsAny(lower, "$¥€£") || strings.Contains(lower, "[$"):
		return "currency"
	case strings.Contains(lower, "yy") || strings.Contains(lower, "dd") || strings.Contains(lower, "mm-dd"):
		if strings.Contains(lower, "h") || strings.Contains(lower, "ss") {
			return "datetime"
		}
		return "date"
	case strings.Contains(lower, "."):
		return "decimal"
	default:
		return "integer"
	}
}

func xlsxResolveStyleID(styles *xlsxMutableStyles, sheet officeXLSXSheetBuild, rowIndex, colIndex int, existingStyle int, column officeColumnSpec, explicitFormat string, rawValue interface{}) (int, error) {
	if styles == nil {
		if existingStyle > 0 {
			return existingStyle, nil
		}
		return 0, nil
	}
	baseStyle := existingStyle
	if baseStyle <= 0 {
		baseStyle = xlsxInheritedColumnStyle(sheet, rowIndex, colIndex)
	}
	if format := officeNormalizeCellFormat(explicitFormat); format != "" {
		return styles.ensureFormatStyle(format, baseStyle)
	}
	if existingStyle > 0 && styles.hasStyle(existingStyle) {
		return existingStyle, nil
	}
	if baseStyle > 0 && styles.hasStyle(baseStyle) {
		return baseStyle, nil
	}
	inferred := officeXLSXDataStyle(column, rowIndex, rawValue)
	if styles.hasStyle(inferred) {
		return inferred, nil
	}
	return 0, nil
}

func xlsxInheritedColumnStyle(sheet officeXLSXSheetBuild, rowIndex, colIndex int) int {
	if colIndex >= 0 && colIndex < len(sheet.ColStyles) {
		if style := sheet.ColStyles[colIndex]; style > 0 {
			return style
		}
	}
	for idx := rowIndex; idx >= 0; idx-- {
		if idx >= len(sheet.Rows) || colIndex >= len(sheet.Rows[idx].Cells) {
			continue
		}
		if style := sheet.Rows[idx].Cells[colIndex].Style; style > 0 {
			return style
		}
	}
	for idx := rowIndex + 1; idx < len(sheet.Rows); idx++ {
		if colIndex >= len(sheet.Rows[idx].Cells) {
			continue
		}
		if style := sheet.Rows[idx].Cells[colIndex].Style; style > 0 {
			return style
		}
	}
	return 0
}
