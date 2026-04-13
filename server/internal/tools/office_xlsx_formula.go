package tools

import (
	"strconv"
	"strings"
)

type officeXLSXFormulaContext struct {
	sheet    officeXLSXSheetBuild
	refs     map[string]officeFormulaCellRef
	cache    map[string]officeFormulaNumber
	visiting map[string]bool
}

type officeFormulaCellRef struct {
	row int
	col int
}

type officeFormulaNumber struct {
	value float64
	ok    bool
}

func officeResolveSupportedFormulaCaches(sheet officeXLSXSheetBuild) officeXLSXSheetBuild {
	ctx := newOfficeXLSXFormulaContext(sheet)
	if len(ctx.refs) == 0 {
		return sheet
	}

	resolved := sheet
	resolved.Rows = make([]officeXLSXRow, len(sheet.Rows))
	for rowIndex, row := range sheet.Rows {
		resolved.Rows[rowIndex] = row
		if len(row.Cells) == 0 {
			continue
		}
		resolved.Rows[rowIndex].Cells = append([]officeXLSXCell(nil), row.Cells...)
		for colIndex, cell := range row.Cells {
			if strings.TrimSpace(cell.Formula) == "" {
				continue
			}
			if value, ok := ctx.evaluateCell(rowIndex, colIndex); ok {
				resolved.Rows[rowIndex].Cells[colIndex].Value = value
			}
		}
	}
	return resolved
}

func newOfficeXLSXFormulaContext(sheet officeXLSXSheetBuild) *officeXLSXFormulaContext {
	refs := make(map[string]officeFormulaCellRef)
	for rowIndex, row := range sheet.Rows {
		for colIndex, cell := range row.Cells {
			if cell.Value == nil && strings.TrimSpace(cell.Formula) == "" && cell.Style == 0 {
				continue
			}
			refs[officeXLSXCellRef(colIndex, rowIndex+1)] = officeFormulaCellRef{
				row: rowIndex,
				col: colIndex,
			}
		}
	}
	return &officeXLSXFormulaContext{
		sheet:    sheet,
		refs:     refs,
		cache:    make(map[string]officeFormulaNumber, len(refs)),
		visiting: make(map[string]bool),
	}
}

func (c *officeXLSXFormulaContext) evaluateCell(rowIndex, colIndex int) (float64, bool) {
	return c.resolveCellNumber(officeXLSXCellRef(colIndex, rowIndex+1))
}

func (c *officeXLSXFormulaContext) resolveCellNumber(ref string) (float64, bool) {
	ref = officeNormalizeFormulaCellRef(ref)
	if ref == "" {
		return 0, false
	}
	if cached, ok := c.cache[ref]; ok {
		return cached.value, cached.ok
	}
	location, ok := c.refs[ref]
	if !ok {
		return 0, true
	}
	cell := c.sheet.Rows[location.row].Cells[location.col]
	if strings.TrimSpace(cell.Formula) != "" && !c.visiting[ref] {
		c.visiting[ref] = true
		if value, ok := officeEvaluateSupportedFormula(cell.Formula, c); ok {
			c.visiting[ref] = false
			c.cache[ref] = officeFormulaNumber{value: value, ok: true}
			return value, true
		}
		c.visiting[ref] = false
	}

	if number, ok := officeCellNumber(cell.Value); ok {
		c.cache[ref] = officeFormulaNumber{value: number, ok: true}
		return number, true
	}
	if strings.TrimSpace(anyToStringForLLM(cell.Value)) == "" {
		c.cache[ref] = officeFormulaNumber{value: 0, ok: true}
		return 0, true
	}
	c.cache[ref] = officeFormulaNumber{value: 0, ok: false}
	return 0, false
}

func (c *officeXLSXFormulaContext) sumRange(startRef, endRef string) (float64, bool) {
	startCol, startRow, ok := xlsxParseCellRef(officeNormalizeFormulaCellRef(startRef))
	if !ok {
		return 0, false
	}
	endCol, endRow, ok := xlsxParseCellRef(officeNormalizeFormulaCellRef(endRef))
	if !ok {
		return 0, false
	}
	if startCol > endCol {
		startCol, endCol = endCol, startCol
	}
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}

	total := 0.0
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			ref := officeXLSXCellRef(col, row)
			location, ok := c.refs[ref]
			if !ok {
				continue
			}
			cell := c.sheet.Rows[location.row].Cells[location.col]
			if number, ok := c.resolveCellNumber(ref); ok {
				if strings.TrimSpace(anyToStringForLLM(cell.Value)) == "" && strings.TrimSpace(cell.Formula) == "" {
					continue
				}
				total += number
			}
		}
	}
	return total, true
}

func officeEvaluateSupportedFormula(formula string, ctx *officeXLSXFormulaContext) (float64, bool) {
	formula = strings.TrimSpace(formula)
	formula = strings.TrimPrefix(formula, "=")
	if formula == "" || strings.ContainsAny(formula, "\"'!") {
		return 0, false
	}
	parser := officeFormulaParser{
		text: formula,
		ctx:  ctx,
	}
	value, ok := parser.parseExpression()
	if !ok {
		return 0, false
	}
	parser.skipSpaces()
	if parser.pos != len(parser.text) {
		return 0, false
	}
	return value, true
}

type officeFormulaParser struct {
	text string
	pos  int
	ctx  *officeXLSXFormulaContext
}

func (p *officeFormulaParser) parseExpression() (float64, bool) {
	left, ok := p.parseTerm()
	if !ok {
		return 0, false
	}
	for {
		p.skipSpaces()
		switch p.peek() {
		case '+':
			p.pos++
			right, ok := p.parseTerm()
			if !ok {
				return 0, false
			}
			left += right
		case '-':
			p.pos++
			right, ok := p.parseTerm()
			if !ok {
				return 0, false
			}
			left -= right
		default:
			return left, true
		}
	}
}

func (p *officeFormulaParser) parseTerm() (float64, bool) {
	left, ok := p.parseFactor()
	if !ok {
		return 0, false
	}
	for {
		p.skipSpaces()
		switch p.peek() {
		case '*':
			p.pos++
			right, ok := p.parseFactor()
			if !ok {
				return 0, false
			}
			left *= right
		case '/':
			p.pos++
			right, ok := p.parseFactor()
			if !ok || right == 0 {
				return 0, false
			}
			left /= right
		default:
			return left, true
		}
	}
}

func (p *officeFormulaParser) parseFactor() (float64, bool) {
	p.skipSpaces()
	switch p.peek() {
	case '+':
		p.pos++
		return p.parseFactor()
	case '-':
		p.pos++
		value, ok := p.parseFactor()
		if !ok {
			return 0, false
		}
		return -value, true
	case '(':
		p.pos++
		value, ok := p.parseExpression()
		if !ok {
			return 0, false
		}
		p.skipSpaces()
		if p.peek() != ')' {
			return 0, false
		}
		p.pos++
		return value, true
	}

	if value, ok := p.parseFunction(); ok {
		return value, true
	}
	if value, ok := p.parseNumber(); ok {
		return value, true
	}
	ref, ok := p.parseCellRef()
	if !ok {
		return 0, false
	}
	return p.ctx.resolveCellNumber(ref)
}

func (p *officeFormulaParser) parseFunction() (float64, bool) {
	start := p.pos
	nameStart := p.pos
	for {
		ch := p.peek()
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			p.pos++
			continue
		}
		break
	}
	if p.pos == nameStart {
		return 0, false
	}
	name := strings.ToUpper(p.text[nameStart:p.pos])
	p.skipSpaces()
	if p.peek() != '(' {
		p.pos = start
		return 0, false
	}
	if name != "SUM" {
		p.pos = start
		return 0, false
	}
	p.pos++
	p.skipSpaces()
	if p.peek() == ')' {
		p.pos++
		return 0, true
	}

	total := 0.0
	for {
		arg, ok := p.parseSumArgument()
		if !ok {
			p.pos = start
			return 0, false
		}
		total += arg
		p.skipSpaces()
		switch p.peek() {
		case ',':
			p.pos++
		case ')':
			p.pos++
			return total, true
		default:
			p.pos = start
			return 0, false
		}
	}
}

func (p *officeFormulaParser) parseSumArgument() (float64, bool) {
	start := p.pos
	ref, ok := p.parseCellRef()
	if ok {
		p.skipSpaces()
		if p.peek() == ':' {
			p.pos++
			other, ok := p.parseCellRef()
			if !ok {
				p.pos = start
				return 0, false
			}
			return p.ctx.sumRange(ref, other)
		}
		p.pos = start
	}
	return p.parseExpression()
}

func (p *officeFormulaParser) parseNumber() (float64, bool) {
	start := p.pos
	seenDigit := false
	seenDot := false
	seenExp := false
	for p.pos < len(p.text) {
		ch := p.text[p.pos]
		switch {
		case ch >= '0' && ch <= '9':
			seenDigit = true
			p.pos++
		case ch == '.' && !seenDot && !seenExp:
			seenDot = true
			p.pos++
		case (ch == 'e' || ch == 'E') && seenDigit && !seenExp:
			seenExp = true
			p.pos++
			if p.pos < len(p.text) && (p.text[p.pos] == '+' || p.text[p.pos] == '-') {
				p.pos++
			}
		default:
			goto done
		}
	}
done:
	if !seenDigit {
		p.pos = start
		return 0, false
	}
	value, err := strconv.ParseFloat(p.text[start:p.pos], 64)
	if err != nil {
		p.pos = start
		return 0, false
	}
	return value, true
}

func (p *officeFormulaParser) parseCellRef() (string, bool) {
	start := p.pos
	if p.peek() == '$' {
		p.pos++
	}
	lettersStart := p.pos
	for {
		ch := p.peek()
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			p.pos++
			continue
		}
		break
	}
	if p.pos == lettersStart {
		p.pos = start
		return "", false
	}
	if p.peek() == '$' {
		p.pos++
	}
	digitsStart := p.pos
	for {
		ch := p.peek()
		if ch >= '0' && ch <= '9' {
			p.pos++
			continue
		}
		break
	}
	if p.pos == digitsStart {
		p.pos = start
		return "", false
	}
	ref := officeNormalizeFormulaCellRef(p.text[start:p.pos])
	if _, _, ok := xlsxParseCellRef(ref); !ok {
		p.pos = start
		return "", false
	}
	return ref, true
}

func (p *officeFormulaParser) skipSpaces() {
	for p.pos < len(p.text) {
		switch p.text[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *officeFormulaParser) peek() byte {
	if p.pos >= len(p.text) {
		return 0
	}
	return p.text[p.pos]
}

func officeNormalizeFormulaCellRef(ref string) string {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	ref = strings.ReplaceAll(ref, "$", "")
	return ref
}

func officeSheetRowsContainFormula(rows [][]interface{}) bool {
	for _, row := range rows {
		for _, value := range row {
			cell, ok := officeParseStructuredCell(value)
			if ok && strings.TrimSpace(cell.Formula) != "" {
				return true
			}
		}
	}
	return false
}
