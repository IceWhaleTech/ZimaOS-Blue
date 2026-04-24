//go:build darwin

package pdf

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	darwinCreateNativeOnce sync.Once
	darwinCreateNativeErr  error

	darwinCreateCFDataCreateMutable            func(allocator uintptr, capacity int64) uintptr
	darwinCreateCFDataGetLength                func(data uintptr) int64
	darwinCreateCFDataGetBytePtr               func(data uintptr) uintptr
	darwinCreateCFRelease                      func(ref uintptr)
	darwinCreateCGDataConsumerCreateWithCFData func(data uintptr) uintptr
	darwinCreateCGDataConsumerRelease          func(consumer uintptr)
	darwinCreateCGPDFContextCreate             func(consumer uintptr, mediaBox *darwinPDFRect, auxiliaryInfo uintptr) uintptr
	darwinCreateCGPDFContextClose              func(context uintptr)
	darwinCreateCGContextBeginPage             func(context uintptr, mediaBox *darwinPDFRect)
	darwinCreateCGContextEndPage               func(context uintptr)
	darwinCreateCGContextRelease               func(context uintptr)
	darwinCreateCGContextSetRGBFillColor       func(context uintptr, red, green, blue, alpha float64)
	darwinCreateCGContextSetRGBStrokeColor     func(context uintptr, red, green, blue, alpha float64)
	darwinCreateCGContextSetLineWidth          func(context uintptr, width float64)
	darwinCreateCGContextMoveToPoint           func(context uintptr, x, y float64)
	darwinCreateCGContextAddLineToPoint        func(context uintptr, x, y float64)
	darwinCreateCGContextStrokePath            func(context uintptr)
	darwinCreateCGContextFillRect              func(context uintptr, rect darwinPDFRect)
	darwinCreateCGContextStrokeRect            func(context uintptr, rect darwinPDFRect)

	darwinCreateSelAlloc                                objc.SEL
	darwinCreateSelInit                                 objc.SEL
	darwinCreateSelRelease                              objc.SEL
	darwinCreateSelStringWithUTF8                       objc.SEL
	darwinCreateSelUTF8String                           objc.SEL
	darwinCreateSelDictionary                           objc.SEL
	darwinCreateSelSetObjectForKey                      objc.SEL
	darwinCreateSelFontWithNameSize                     objc.SEL
	darwinCreateSelSystemFontOfSize                     objc.SEL
	darwinCreateSelBoldSystemFontOfSize                 objc.SEL
	darwinCreateSelColorWithCalibratedRedGreenBlueAlpha objc.SEL
	darwinCreateSelGraphicsContextWithCGContextFlipped  objc.SEL
	darwinCreateSelSetCurrentContext                    objc.SEL
	darwinCreateSelSaveGraphicsState                    objc.SEL
	darwinCreateSelRestoreGraphicsState                 objc.SEL
	darwinCreateSelSizeWithAttributes                   objc.SEL
	darwinCreateSelDrawInRectWithAttributes             objc.SEL
)

type darwinCreateRendererState struct {
	context      uintptr
	fontPlan     createFontPlan
	pageCount    int
	lineCount    int
	currentY     float64
	pageOpen     bool
	fontKey      objc.ID
	colorKey     objc.ID
	fontCache    map[string]objc.ID
	colorCache   map[string]objc.ID
	measureAttrs map[string]objc.ID
	drawAttrs    map[string]objc.ID
}

type darwinCreateTextMeasurer struct {
	state    *darwinCreateRendererState
	fontID   string
	fontSize float64
}

func (m darwinCreateTextMeasurer) CellMargin() float64 {
	return 0
}

func (m darwinCreateTextMeasurer) MeasureText(text string) float64 {
	if m.state == nil {
		return 0
	}
	width, err := m.state.measureText(text, m.fontID, m.fontSize)
	if err != nil {
		return 0
	}
	return width
}

func renderCreatePDFDarwinNative(lines []createStyledLine, fontPlan createFontPlan) ([]byte, int, int, error) {
	if err := initDarwinCreateNativeRuntime(); err != nil {
		return nil, 0, 0, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pool := newDarwinCreateAutoreleasePool()
	if pool != 0 {
		defer releaseDarwinCreateObject(pool)
	}

	data := darwinCreateCFDataCreateMutable(0, 0)
	if data == 0 {
		return nil, 0, 0, fmt.Errorf("create mutable PDF data buffer")
	}
	defer darwinCreateReleaseCFObject(data)

	consumer := darwinCreateCGDataConsumerCreateWithCFData(data)
	if consumer == 0 {
		return nil, 0, 0, fmt.Errorf("create PDF data consumer")
	}
	defer darwinCreateCGDataConsumerRelease(consumer)

	mediaBox := darwinPDFRect{
		Origin: darwinPDFPoint{},
		Size: darwinPDFSize{
			Width:  createPageWidth,
			Height: createPageHeight,
		},
	}
	context := darwinCreateCGPDFContextCreate(consumer, &mediaBox, 0)
	if context == 0 {
		return nil, 0, 0, fmt.Errorf("create Quartz PDF context")
	}
	defer darwinCreateCGContextRelease(context)

	graphicsContextClass := objc.ID(objc.GetClass("NSGraphicsContext"))
	if graphicsContextClass == 0 {
		return nil, 0, 0, fmt.Errorf("load NSGraphicsContext class")
	}
	graphicsContext := graphicsContextClass.Send(darwinCreateSelGraphicsContextWithCGContextFlipped, context, true)
	if graphicsContext == 0 {
		return nil, 0, 0, fmt.Errorf("create NSGraphicsContext from Quartz PDF context")
	}

	graphicsContextClass.Send(darwinCreateSelSaveGraphicsState)
	defer graphicsContextClass.Send(darwinCreateSelRestoreGraphicsState)
	graphicsContextClass.Send(darwinCreateSelSetCurrentContext, graphicsContext)

	state := &darwinCreateRendererState{
		context:      context,
		fontPlan:     fontPlan,
		fontKey:      darwinCreateNSString("NSFont"),
		colorKey:     darwinCreateNSString("NSColor"),
		fontCache:    make(map[string]objc.ID),
		colorCache:   make(map[string]objc.ID),
		measureAttrs: make(map[string]objc.ID),
		drawAttrs:    make(map[string]objc.ID),
	}
	state.beginPage()

	textWidth := createPageWidth - (createMarginLeft * 2)
	for _, line := range lines {
		switch {
		case line.Table != nil:
			if err := state.renderTable(line.Table, textWidth); err != nil {
				return nil, 0, 0, err
			}
			state.currentY += line.GapAfter
		case line.Divider != nil:
			if err := state.renderDivider(line.Divider, textWidth); err != nil {
				return nil, 0, 0, err
			}
			state.currentY += line.GapAfter
		case line.ListItem != nil:
			if err := state.renderListItem(line.ListItem, line.Font, line.FontSize, textWidth, line.ColorR, line.ColorG, line.ColorB); err != nil {
				return nil, 0, 0, err
			}
			state.currentY += line.GapAfter
		default:
			if strings.TrimSpace(line.Text) == "" {
				state.ensurePageFor(createBlankLineHeight)
				state.currentY += createBlankLineHeight + line.GapAfter
				continue
			}
			if err := state.renderTextBlock(line, textWidth); err != nil {
				return nil, 0, 0, err
			}
			state.currentY += line.GapAfter
		}
	}

	if state.pageOpen {
		darwinCreateCGContextEndPage(context)
		state.pageOpen = false
	}
	darwinCreateCGPDFContextClose(context)

	out := darwinCreateCFDataBytes(data)
	if len(out) == 0 {
		return nil, 0, 0, fmt.Errorf("native Quartz PDF renderer produced empty output")
	}
	return out, state.pageCount, state.lineCount, nil
}

func initDarwinCreateNativeRuntime() error {
	darwinCreateNativeOnce.Do(func() {
		if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinCreateNativeErr = fmt.Errorf("load Foundation.framework for native PDF create: %w", err)
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinCreateNativeErr = fmt.Errorf("load AppKit.framework for native PDF create: %w", err)
			return
		}
		coreFoundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			darwinCreateNativeErr = fmt.Errorf("load CoreFoundation.framework for native PDF create: %w", err)
			return
		}
		coreGraphics, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			darwinCreateNativeErr = fmt.Errorf("load CoreGraphics.framework for native PDF create: %w", err)
			return
		}

		purego.RegisterLibFunc(&darwinCreateCFDataCreateMutable, coreFoundation, "CFDataCreateMutable")
		purego.RegisterLibFunc(&darwinCreateCFDataGetLength, coreFoundation, "CFDataGetLength")
		purego.RegisterLibFunc(&darwinCreateCFDataGetBytePtr, coreFoundation, "CFDataGetBytePtr")
		purego.RegisterLibFunc(&darwinCreateCFRelease, coreFoundation, "CFRelease")
		purego.RegisterLibFunc(&darwinCreateCGDataConsumerCreateWithCFData, coreGraphics, "CGDataConsumerCreateWithCFData")
		purego.RegisterLibFunc(&darwinCreateCGDataConsumerRelease, coreGraphics, "CGDataConsumerRelease")
		purego.RegisterLibFunc(&darwinCreateCGPDFContextCreate, coreGraphics, "CGPDFContextCreate")
		purego.RegisterLibFunc(&darwinCreateCGPDFContextClose, coreGraphics, "CGPDFContextClose")
		purego.RegisterLibFunc(&darwinCreateCGContextBeginPage, coreGraphics, "CGContextBeginPage")
		purego.RegisterLibFunc(&darwinCreateCGContextEndPage, coreGraphics, "CGContextEndPage")
		purego.RegisterLibFunc(&darwinCreateCGContextRelease, coreGraphics, "CGContextRelease")
		purego.RegisterLibFunc(&darwinCreateCGContextSetRGBFillColor, coreGraphics, "CGContextSetRGBFillColor")
		purego.RegisterLibFunc(&darwinCreateCGContextSetRGBStrokeColor, coreGraphics, "CGContextSetRGBStrokeColor")
		purego.RegisterLibFunc(&darwinCreateCGContextSetLineWidth, coreGraphics, "CGContextSetLineWidth")
		purego.RegisterLibFunc(&darwinCreateCGContextMoveToPoint, coreGraphics, "CGContextMoveToPoint")
		purego.RegisterLibFunc(&darwinCreateCGContextAddLineToPoint, coreGraphics, "CGContextAddLineToPoint")
		purego.RegisterLibFunc(&darwinCreateCGContextStrokePath, coreGraphics, "CGContextStrokePath")
		purego.RegisterLibFunc(&darwinCreateCGContextFillRect, coreGraphics, "CGContextFillRect")
		purego.RegisterLibFunc(&darwinCreateCGContextStrokeRect, coreGraphics, "CGContextStrokeRect")

		darwinCreateSelAlloc = objc.RegisterName("alloc")
		darwinCreateSelInit = objc.RegisterName("init")
		darwinCreateSelRelease = objc.RegisterName("release")
		darwinCreateSelStringWithUTF8 = objc.RegisterName("stringWithUTF8String:")
		darwinCreateSelUTF8String = objc.RegisterName("UTF8String")
		darwinCreateSelDictionary = objc.RegisterName("dictionary")
		darwinCreateSelSetObjectForKey = objc.RegisterName("setObject:forKey:")
		darwinCreateSelFontWithNameSize = objc.RegisterName("fontWithName:size:")
		darwinCreateSelSystemFontOfSize = objc.RegisterName("systemFontOfSize:")
		darwinCreateSelBoldSystemFontOfSize = objc.RegisterName("boldSystemFontOfSize:")
		darwinCreateSelColorWithCalibratedRedGreenBlueAlpha = objc.RegisterName("colorWithCalibratedRed:green:blue:alpha:")
		darwinCreateSelGraphicsContextWithCGContextFlipped = objc.RegisterName("graphicsContextWithCGContext:flipped:")
		darwinCreateSelSetCurrentContext = objc.RegisterName("setCurrentContext:")
		darwinCreateSelSaveGraphicsState = objc.RegisterName("saveGraphicsState")
		darwinCreateSelRestoreGraphicsState = objc.RegisterName("restoreGraphicsState")
		darwinCreateSelSizeWithAttributes = objc.RegisterName("sizeWithAttributes:")
		darwinCreateSelDrawInRectWithAttributes = objc.RegisterName("drawInRect:withAttributes:")
	})
	return darwinCreateNativeErr
}

func (s *darwinCreateRendererState) beginPage() {
	if s.pageOpen {
		darwinCreateCGContextEndPage(s.context)
	}
	mediaBox := darwinPDFRect{
		Origin: darwinPDFPoint{},
		Size: darwinPDFSize{
			Width:  createPageWidth,
			Height: createPageHeight,
		},
	}
	darwinCreateCGContextBeginPage(s.context, &mediaBox)
	s.pageOpen = true
	s.pageCount++
	s.currentY = createMarginTop
}

func (s *darwinCreateRendererState) ensurePageFor(height float64) {
	if !s.pageOpen {
		s.beginPage()
		return
	}
	if s.currentY+height <= createPageHeight-createMarginBottom {
		return
	}
	s.beginPage()
}

func (s *darwinCreateRendererState) renderTextBlock(line createStyledLine, textWidth float64) error {
	align := line.Align
	if align == "" {
		align = "L"
	}
	measurer := darwinCreateTextMeasurer{
		state:    s,
		fontID:   line.Font,
		fontSize: line.FontSize,
	}
	segments := createSplitTextWithMeasurer(measurer, line.Text, textWidth)
	if len(segments) == 0 {
		segments = []string{line.Text}
	}

	lineHeight := line.FontSize * createLineHeightScale
	for _, segment := range segments {
		s.ensurePageFor(lineHeight)
		if err := s.drawText(segment, createMarginLeft, s.currentY, textWidth, lineHeight, align, line.Font, line.FontSize, line.ColorR, line.ColorG, line.ColorB); err != nil {
			return err
		}
		s.currentY += lineHeight
		s.lineCount++
	}
	return nil
}

func (s *darwinCreateRendererState) renderDivider(divider *createStyledDivider, textWidth float64) error {
	if divider == nil {
		return nil
	}

	lineY := s.currentY + 3
	if lineY > createPageHeight-createMarginBottom {
		s.beginPage()
		lineY = s.currentY + 3
	}

	left := createMarginLeft + createDividerInset
	right := createMarginLeft + textWidth - createDividerInset
	if right <= left {
		left = createMarginLeft
		right = createMarginLeft + textWidth
	}

	darwinCreateCGContextSetRGBStrokeColor(s.context, darwinCreateUnitColor(divider.LineR), darwinCreateUnitColor(divider.LineG), darwinCreateUnitColor(divider.LineB), 1)
	darwinCreateCGContextSetLineWidth(s.context, createDividerLineWidth)
	darwinCreateCGContextMoveToPoint(s.context, left, createPageHeight-lineY)
	darwinCreateCGContextAddLineToPoint(s.context, right, createPageHeight-lineY)
	darwinCreateCGContextStrokePath(s.context)

	s.currentY = lineY
	s.lineCount++
	return nil
}

func (s *darwinCreateRendererState) renderListItem(item *createStyledListItem, fontID string, fontSize, textWidth float64, textR, textG, textB int) error {
	if item == nil {
		return nil
	}

	lineHeight := fontSize * createLineHeightScale
	measurer := darwinCreateTextMeasurer{
		state:    s,
		fontID:   fontID,
		fontSize: fontSize,
	}
	prepared := prepareCreateListItem(measurer, *item, textWidth)
	startX := createMarginLeft

	for index, segment := range prepared.Segments {
		s.ensurePageFor(lineHeight)
		if index == 0 {
			if err := s.drawText(prepared.Marker, startX, s.currentY, prepared.MarkerTextWidth, lineHeight, "R", fontID, fontSize, textR, textG, textB); err != nil {
				return err
			}
		}
		if err := s.drawText(segment, startX+prepared.MarkerWidth, s.currentY, prepared.ContentWidth, lineHeight, prepared.Align, fontID, fontSize, textR, textG, textB); err != nil {
			return err
		}
		s.currentY += lineHeight
		s.lineCount++
	}
	return nil
}

func (s *darwinCreateRendererState) renderTable(table *createStyledTable, textWidth float64) error {
	if table == nil {
		return nil
	}
	columnCount := createTableColumnCount(table)
	if columnCount == 0 {
		return nil
	}

	widths := createTableColumnWidths(columnCount, textWidth)

	renderHeader := func() error {
		if len(table.Headers) == 0 {
			return nil
		}
		_, fits, err := s.renderTableRow(table.Headers, widths, createFontBold, table.FontSize, table.HeaderTextR, table.HeaderTextG, table.HeaderTextB, table.HeaderFillR, table.HeaderFillG, table.HeaderFillB, table.BorderR, table.BorderG, table.BorderB, true)
		if err != nil {
			return err
		}
		if fits {
			return nil
		}
		s.beginPage()
		_, fits, err = s.renderTableRow(table.Headers, widths, createFontBold, table.FontSize, table.HeaderTextR, table.HeaderTextG, table.HeaderTextB, table.HeaderFillR, table.HeaderFillG, table.HeaderFillB, table.BorderR, table.BorderG, table.BorderB, true)
		if err != nil {
			return err
		}
		if !fits {
			return fmt.Errorf("pdf table header too tall to fit on a single page")
		}
		return nil
	}

	if err := renderHeader(); err != nil {
		return err
	}

	for rowIndex, row := range table.Rows {
		fill := rowIndex%2 == 0
		_, fits, err := s.renderTableRow(row, widths, createFontRegular, table.FontSize, table.BodyTextR, table.BodyTextG, table.BodyTextB, table.RowFillR, table.RowFillG, table.RowFillB, table.BorderR, table.BorderG, table.BorderB, fill)
		if err != nil {
			return err
		}
		if fits {
			continue
		}
		s.beginPage()
		if err := renderHeader(); err != nil {
			return err
		}
		_, fits, err = s.renderTableRow(row, widths, createFontRegular, table.FontSize, table.BodyTextR, table.BodyTextG, table.BodyTextB, table.RowFillR, table.RowFillG, table.RowFillB, table.BorderR, table.BorderG, table.BorderB, fill)
		if err != nil {
			return err
		}
		if !fits {
			return fmt.Errorf("pdf table row too tall to fit on a single page")
		}
	}

	return nil
}

func (s *darwinCreateRendererState) renderTableRow(cells []createStyledTableCell, widths []float64, fontID string, fontSize float64, textR, textG, textB int, fillR, fillG, fillB int, borderR, borderG, borderB int, fill bool) (int, bool, error) {
	lineHeight := fontSize * createTableLineHeightScale
	measurer := darwinCreateTextMeasurer{
		state:    s,
		fontID:   fontID,
		fontSize: fontSize,
	}
	row := prepareCreateTableRow(measurer, cells, widths, fontSize)
	rowHeight := row.RowHeight
	if s.currentY+rowHeight > createPageHeight-createMarginBottom {
		return 0, false, nil
	}

	startY := s.currentY
	x := createMarginLeft
	for idx, width := range widths {
		rect := darwinPDFRect{
			Origin: darwinPDFPoint{
				X: x,
				Y: createBottomOriginY(startY, rowHeight),
			},
			Size: darwinPDFSize{
				Width:  width,
				Height: rowHeight,
			},
		}
		if fill {
			darwinCreateCGContextSetRGBFillColor(s.context, darwinCreateUnitColor(fillR), darwinCreateUnitColor(fillG), darwinCreateUnitColor(fillB), 1)
			darwinCreateCGContextFillRect(s.context, rect)
		}
		darwinCreateCGContextSetRGBStrokeColor(s.context, darwinCreateUnitColor(borderR), darwinCreateUnitColor(borderG), darwinCreateUnitColor(borderB), 1)
		darwinCreateCGContextSetLineWidth(s.context, createTableBorderWidth)
		darwinCreateCGContextStrokeRect(s.context, rect)

		align := row.Aligns[idx]
		textTop := startY + createTableCellPaddingY
		for _, segment := range row.Wrapped[idx] {
			if err := s.drawText(segment, x+createTableCellPaddingX, textTop, row.InnerWidths[idx], lineHeight, align, fontID, fontSize, textR, textG, textB); err != nil {
				return 0, false, err
			}
			textTop += lineHeight
		}
		x += width
	}

	s.currentY = startY + rowHeight
	s.lineCount += row.MaxLines
	return row.MaxLines, true, nil
}

func (s *darwinCreateRendererState) drawText(text string, left, top, availableWidth, lineHeight float64, align, fontID string, fontSize float64, colorR, colorG, colorB int) error {
	attrs, err := s.drawAttributes(fontID, fontSize, colorR, colorG, colorB)
	if err != nil {
		return err
	}

	drawText := text
	if align == "R" && createHasArabicLetters(drawText) {
		drawText = createShapeArabicVisual(drawText)
	}
	nsText := darwinCreateNSString(drawText)
	if nsText == 0 {
		return fmt.Errorf("create NSString for PDF text")
	}

	textSize := objc.Send[darwinPDFSize](nsText, darwinCreateSelSizeWithAttributes, attrs)
	x := createAlignedTextX(left, availableWidth, textSize.Width, align)
	rectWidth := availableWidth
	if rectWidth <= 0 {
		rectWidth = textSize.Width
	}
	if rectWidth < textSize.Width {
		rectWidth = textSize.Width
	}
	rect := darwinPDFRect{
		Origin: darwinPDFPoint{
			X: x,
			Y: top,
		},
		Size: darwinPDFSize{
			Width:  rectWidth,
			Height: lineHeight,
		},
	}
	nsText.Send(darwinCreateSelDrawInRectWithAttributes, rect, attrs)
	return nil
}

func (s *darwinCreateRendererState) measureText(text string, fontID string, fontSize float64) (float64, error) {
	attrs, err := s.measurementAttributes(fontID, fontSize)
	if err != nil {
		return 0, err
	}
	nsText := darwinCreateNSString(text)
	if nsText == 0 {
		return 0, fmt.Errorf("create NSString for PDF measurement")
	}
	size := objc.Send[darwinPDFSize](nsText, darwinCreateSelSizeWithAttributes, attrs)
	return size.Width, nil
}

func (s *darwinCreateRendererState) measurementAttributes(fontID string, fontSize float64) (objc.ID, error) {
	key := fmt.Sprintf("%s:%.2f", fontID, fontSize)
	if attrs := s.measureAttrs[key]; attrs != 0 {
		return attrs, nil
	}

	font, err := s.font(fontID, fontSize)
	if err != nil {
		return 0, err
	}
	attrsClass := objc.ID(objc.GetClass("NSMutableDictionary"))
	if attrsClass == 0 {
		return 0, fmt.Errorf("load NSMutableDictionary class")
	}
	attrs := attrsClass.Send(darwinCreateSelDictionary)
	if attrs == 0 {
		return 0, fmt.Errorf("create NSMutableDictionary for font attributes")
	}
	attrs.Send(darwinCreateSelSetObjectForKey, font, s.fontKey)
	s.measureAttrs[key] = attrs
	return attrs, nil
}

func (s *darwinCreateRendererState) drawAttributes(fontID string, fontSize float64, colorR, colorG, colorB int) (objc.ID, error) {
	key := fmt.Sprintf("%s:%.2f:%d:%d:%d", fontID, fontSize, colorR, colorG, colorB)
	if attrs := s.drawAttrs[key]; attrs != 0 {
		return attrs, nil
	}

	font, err := s.font(fontID, fontSize)
	if err != nil {
		return 0, err
	}
	color, err := s.color(colorR, colorG, colorB)
	if err != nil {
		return 0, err
	}

	attrsClass := objc.ID(objc.GetClass("NSMutableDictionary"))
	if attrsClass == 0 {
		return 0, fmt.Errorf("load NSMutableDictionary class")
	}
	attrs := attrsClass.Send(darwinCreateSelDictionary)
	if attrs == 0 {
		return 0, fmt.Errorf("create NSMutableDictionary for draw attributes")
	}
	attrs.Send(darwinCreateSelSetObjectForKey, font, s.fontKey)
	attrs.Send(darwinCreateSelSetObjectForKey, color, s.colorKey)
	s.drawAttrs[key] = attrs
	return attrs, nil
}

func (s *darwinCreateRendererState) font(fontID string, fontSize float64) (objc.ID, error) {
	key := fmt.Sprintf("%s:%.2f", fontID, fontSize)
	if font := s.fontCache[key]; font != 0 {
		return font, nil
	}

	fontClass := objc.ID(objc.GetClass("NSFont"))
	if fontClass == 0 {
		return 0, fmt.Errorf("load NSFont class")
	}

	family, style := s.fontPlan.fontFor(fontID)
	var font objc.ID
	if s.fontPlan.hasUnicodeFont() {
		if style == "B" {
			font = fontClass.Send(darwinCreateSelBoldSystemFontOfSize, fontSize)
		} else {
			font = fontClass.Send(darwinCreateSelSystemFontOfSize, fontSize)
		}
	} else {
		name := family
		if strings.TrimSpace(name) == "" {
			name = createCoreFontFamily
		}
		if style == "B" {
			if !strings.Contains(strings.ToLower(name), "bold") {
				name += "-Bold"
			}
		}
		nsName := darwinCreateNSString(name)
		if nsName != 0 {
			font = fontClass.Send(darwinCreateSelFontWithNameSize, nsName, fontSize)
		}
	}
	if font == 0 {
		if style == "B" {
			font = fontClass.Send(darwinCreateSelBoldSystemFontOfSize, fontSize)
		} else {
			font = fontClass.Send(darwinCreateSelSystemFontOfSize, fontSize)
		}
	}
	if font == 0 {
		return 0, fmt.Errorf("resolve native macOS font for %q size %.2f", fontID, fontSize)
	}
	s.fontCache[key] = font
	return font, nil
}

func (s *darwinCreateRendererState) color(r, g, b int) (objc.ID, error) {
	key := fmt.Sprintf("%d:%d:%d", r, g, b)
	if color := s.colorCache[key]; color != 0 {
		return color, nil
	}

	colorClass := objc.ID(objc.GetClass("NSColor"))
	if colorClass == 0 {
		return 0, fmt.Errorf("load NSColor class")
	}
	color := colorClass.Send(darwinCreateSelColorWithCalibratedRedGreenBlueAlpha, darwinCreateUnitColor(r), darwinCreateUnitColor(g), darwinCreateUnitColor(b), 1.0)
	if color == 0 {
		return 0, fmt.Errorf("create NSColor")
	}
	s.colorCache[key] = color
	return color, nil
}

func newDarwinCreateAutoreleasePool() objc.ID {
	poolClass := objc.ID(objc.GetClass("NSAutoreleasePool"))
	if poolClass == 0 {
		return 0
	}
	return poolClass.Send(darwinCreateSelAlloc).Send(darwinCreateSelInit)
}

func releaseDarwinCreateObject(id objc.ID) {
	if id == 0 {
		return
	}
	id.Send(darwinCreateSelRelease)
}

func darwinCreateReleaseCFObject(ref uintptr) {
	if ref == 0 {
		return
	}
	if darwinCreateCFRelease != nil {
		darwinCreateCFRelease(ref)
	}
}

func darwinCreateNSString(value string) objc.ID {
	bytes := append([]byte(value), 0)
	class := objc.ID(objc.GetClass("NSString"))
	if class == 0 {
		return 0
	}
	return class.Send(darwinCreateSelStringWithUTF8, uintptr(unsafe.Pointer(&bytes[0])))
}

func darwinCreateCFDataBytes(data uintptr) []byte {
	if data == 0 {
		return nil
	}
	length := int(darwinCreateCFDataGetLength(data))
	if length <= 0 {
		return nil
	}
	ptr := darwinCreateCFDataGetBytePtr(data)
	if ptr == 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length)...)
}

func darwinCreateUnitColor(component int) float64 {
	if component <= 0 {
		return 0
	}
	if component >= 255 {
		return 1
	}
	return float64(component) / 255.0
}
