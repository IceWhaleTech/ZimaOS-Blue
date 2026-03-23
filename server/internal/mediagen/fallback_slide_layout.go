package mediagen

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	stdDraw "image/draw"
	"image/png"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/slidespec"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
)

func slideLayoutSpecFromMediaRequest(
	req *MediaRequest,
	brief slidespec.Brief,
	canvas slideCanvasSpec,
	hasVisual bool,
) slidespec.LayoutSpec {
	if spec, ok := mediaRequestLayoutSpec(req); ok {
		return slidespec.NormalizeLayoutSpec(spec, brief, canvas.width, canvas.height, hasVisual)
	}
	return slidespec.BuildLayoutSpec(brief, canvas.width, canvas.height, hasVisual)
}

func mediaRequestLayoutSpec(req *MediaRequest) (slidespec.LayoutSpec, bool) {
	if req == nil || req.Extra == nil {
		return slidespec.LayoutSpec{}, false
	}
	for _, key := range []string{"layout_spec", "ppt_layout_spec", "slide_layout_spec"} {
		if raw, ok := req.Extra[key]; ok {
			if spec, ok := slidespec.DecodeLayoutSpec(raw); ok {
				return spec, true
			}
		}
	}
	return slidespec.LayoutSpec{}, false
}

func renderSlidePNGWithLayout(
	layout slidespec.LayoutSpec,
	brief slidespec.Brief,
	visualDataURL string,
	canvas slideCanvasSpec,
) (string, error) {
	effectiveBrief := brief
	if strings.TrimSpace(layout.StylePreset) != "" {
		effectiveBrief.StylePreset = layout.StylePreset
	}
	if strings.TrimSpace(layout.Theme) != "" {
		effectiveBrief.Theme = layout.Theme
	}
	if strings.TrimSpace(layout.TemplateID) != "" {
		effectiveBrief.TemplateID = layout.TemplateID
	}

	img := image.NewRGBA(image.Rect(0, 0, canvas.width, canvas.height))
	palette := pickSlidePalette(effectiveBrief)
	renderSlideLayoutBackground(img, layout, effectiveBrief, palette, visualDataURL, canvas)

	defaultFonts := newSlideFontPack(canvas, firstNonEmptyValue(layout.TemplateID, effectiveBrief.TemplateID))
	defer defaultFonts.close()
	renderSlideLayoutElements(img, layout, effectiveBrief, palette, visualDataURL, canvas, defaultFonts)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func renderSlideLayoutBackground(
	img *image.RGBA,
	layout slidespec.LayoutSpec,
	brief slidespec.Brief,
	palette slidePalette,
	visualDataURL string,
	canvas slideCanvasSpec,
) {
	hasBackground := layout.BackgroundHasContent()
	drewBase := false
	if len(layout.Background.Gradient) >= 2 {
		top, okTop := parseSlideColor(layout.Background.Gradient[0])
		bottom, okBottom := parseSlideColor(layout.Background.Gradient[1])
		if okTop && okBottom {
			fillVerticalGradient(img, img.Bounds(), top, bottom)
			drewBase = true
		}
	}
	if !drewBase {
		if fill, ok := parseSlideColor(layout.Background.Fill); ok {
			fillRect(img, img.Bounds(), fill)
			drewBase = true
		}
	}
	if !drewBase {
		fillVerticalGradient(img, img.Bounds(), palette.Top, palette.Bottom)
	}

	bgSource := resolveLayoutImageSource(layout.Background.ImageSource, visualDataURL)
	if bgSource != "" {
		if bgImage := decodeDataURLImage(bgSource); bgImage != nil {
			drawImageFitRect(img, bgImage, img.Bounds(), layout.Background.ImageFit)
		}
	}

	if !hasBackground {
		drawSlideBackdrop(img, canvas, palette, brief, firstNonEmptyValue(layout.TemplateID, brief.TemplateID))
	}

	if overlay, ok := parseSlideColor(layout.Background.Overlay); ok {
		overlayRect(img, img.Bounds(), overlay)
	}
}

func renderSlideLayoutElements(
	img *image.RGBA,
	layout slidespec.LayoutSpec,
	brief slidespec.Brief,
	palette slidePalette,
	visualDataURL string,
	canvas slideCanvasSpec,
	defaultFonts slideFontPack,
) {
	elements := append([]slidespec.LayoutElement(nil), layout.Elements...)
	sort.SliceStable(elements, func(i, j int) bool {
		return elements[i].ZIndex < elements[j].ZIndex
	})

	baseWidth := layout.Canvas.Width
	if baseWidth <= 0 {
		baseWidth = canvas.width
	}
	baseHeight := layout.Canvas.Height
	if baseHeight <= 0 {
		baseHeight = canvas.height
	}
	if baseWidth <= 0 || baseHeight <= 0 {
		return
	}

	for _, element := range elements {
		rect := slideLayoutRect(element, baseWidth, baseHeight, canvas)
		if rect.Empty() {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(element.Kind)) {
		case "shape":
			renderSlideLayoutShape(img, rect, element)
		case "image":
			renderSlideLayoutImage(img, rect, element, visualDataURL, palette, brief)
		case "text":
			renderSlideLayoutText(img, rect, element, canvas, baseWidth, defaultFonts, palette)
		case "chart":
			renderSlideLayoutChart(img, rect, element, palette, canvas, baseWidth, defaultFonts)
		}
	}
}

func slideLayoutRect(element slidespec.LayoutElement, baseWidth, baseHeight int, canvas slideCanvasSpec) image.Rectangle {
	scaleX := float64(canvas.width) / float64(maxInt(baseWidth, 1))
	scaleY := float64(canvas.height) / float64(maxInt(baseHeight, 1))
	x := int(math.Round(float64(element.X) * scaleX))
	y := int(math.Round(float64(element.Y) * scaleY))
	w := int(math.Round(float64(element.Width) * scaleX))
	h := int(math.Round(float64(element.Height) * scaleY))
	if w <= 0 || h <= 0 {
		return image.Rectangle{}
	}
	return image.Rect(x, y, x+w, y+h).Intersect(image.Rect(0, 0, canvas.width, canvas.height))
}

func renderSlideLayoutShape(img *image.RGBA, rect image.Rectangle, element slidespec.LayoutElement) {
	if fill, ok := parseSlideColor(element.Fill); ok {
		overlayRoundedRect(img, rect, element.Radius, fill)
	}
	if stroke, ok := parseSlideColor(element.Stroke); ok {
		strokeWidth := maxInt(1, element.StrokeWidth)
		strokeRoundedRect(img, rect, stroke, strokeWidth, element.Radius)
	}
}

func renderSlideLayoutImage(
	img *image.RGBA,
	rect image.Rectangle,
	element slidespec.LayoutElement,
	visualDataURL string,
	palette slidePalette,
	brief slidespec.Brief,
) {
	source := resolveLayoutImageSource(element.Source, visualDataURL)
	if source == "" {
		drawSlideVisualPanel(img, rect, nil, palette, brief)
		return
	}
	decoded := decodeDataURLImage(source)
	if decoded == nil {
		drawSlideVisualPanel(img, rect, nil, palette, brief)
		return
	}
	if fill, ok := parseSlideColor(element.Fill); ok {
		overlayRoundedRect(img, rect, element.Radius, fill)
	}
	drawImageFitRoundedRect(img, decoded, rect, element.Fit, element.Radius)
	if stroke, ok := parseSlideColor(element.Stroke); ok {
		strokeRoundedRect(img, rect, stroke, maxInt(1, element.StrokeWidth), element.Radius)
	}
}

func renderSlideLayoutChart(
	img *image.RGBA,
	rect image.Rectangle,
	element slidespec.LayoutElement,
	palette slidePalette,
	canvas slideCanvasSpec,
	baseWidth int,
	defaultFonts slideFontPack,
) {
	scale := float64(canvas.width) / float64(maxInt(baseWidth, 1))
	labelFace, owned := slideLayoutChartFontFace(element, defaultFonts, scale)
	if owned {
		defer closeSlideFace(labelFace)
	}
	switch strings.ToLower(strings.TrimSpace(element.ChartType)) {
	case "divider":
		renderSlideLayoutDivider(img, rect, element, palette)
	case "progress":
		renderSlideLayoutProgress(img, rect, element, palette, labelFace)
	case "bars":
		renderSlideLayoutBars(img, rect, element, palette, labelFace)
	default:
		renderSlideLayoutSparkline(img, rect, element, palette, labelFace)
	}
}

func renderSlideLayoutDivider(img *image.RGBA, rect image.Rectangle, element slidespec.LayoutElement, palette slidePalette) {
	thickness := maxInt(1, element.StrokeWidth)
	if rect.Dy() > thickness {
		midY := rect.Min.Y + rect.Dy()/2
		rect = image.Rect(rect.Min.X, midY-thickness/2, rect.Max.X, midY-thickness/2+thickness)
	}
	overlayRoundedRect(img, rect, normalizeRoundedRadius(rect, element.Radius), chartAccentColor(element, palette))
}

func renderSlideLayoutProgress(
	img *image.RGBA,
	rect image.Rectangle,
	element slidespec.LayoutElement,
	palette slidePalette,
	labelFace font.Face,
) {
	contentRect := rect
	labelText := chartPrimaryLabel(element)
	valueText := chartProgressValueText(element)
	labelColor := chartLabelColor(element, palette)
	if labelFace != nil && (labelText != "" || valueText != "") {
		headerHeight := slideLineHeight(labelFace)
		if rect.Dy() >= headerHeight+12 {
			headerRect := image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+headerHeight)
			if labelText != "" && valueText != "" {
				splitX := headerRect.Min.X + int(math.Round(float64(headerRect.Dx())*0.62))
				drawChartTextLine(img, labelFace, image.Rect(headerRect.Min.X, headerRect.Min.Y, splitX, headerRect.Max.Y), labelText, labelColor, "left")
				drawChartTextLine(img, labelFace, image.Rect(splitX+8, headerRect.Min.Y, headerRect.Max.X, headerRect.Max.Y), valueText, labelColor, "right")
			} else if labelText != "" {
				drawChartTextLine(img, labelFace, headerRect, labelText, labelColor, "left")
			} else {
				drawChartTextLine(img, labelFace, headerRect, valueText, labelColor, "right")
			}
			contentRect.Min.Y += headerHeight + 4
		}
	}
	if contentRect.Empty() {
		return
	}
	track := chartTrackColor(element, palette)
	radius := normalizeRoundedRadius(contentRect, maxInt(element.Radius, minInt(contentRect.Dx(), contentRect.Dy())/2))
	overlayRoundedRect(img, contentRect, radius, track)
	ratio := chartRatio(element)
	fillRect := contentRect
	if normalizeChartDirection(element.ChartType, element.Direction) == "vertical" {
		fillRect.Min.Y = fillRect.Max.Y - int(math.Round(float64(contentRect.Dy())*ratio))
	} else {
		fillRect.Max.X = fillRect.Min.X + int(math.Round(float64(contentRect.Dx())*ratio))
	}
	if fillRect.Dx() > 0 && fillRect.Dy() > 0 {
		overlayRoundedRect(img, fillRect, normalizeRoundedRadius(fillRect, maxInt(element.Radius, minInt(fillRect.Dx(), fillRect.Dy())/2)), chartAccentColor(element, palette))
	}
	if stroke, ok := parseSlideColor(element.Stroke); ok {
		strokeRoundedRect(img, contentRect, stroke, maxInt(1, element.StrokeWidth), maxInt(element.Radius, minInt(contentRect.Dx(), contentRect.Dy())/2))
	}
}

func renderSlideLayoutSparkline(
	img *image.RGBA,
	rect image.Rectangle,
	element slidespec.LayoutElement,
	palette slidePalette,
	labelFace font.Face,
) {
	values := normalizedChartValues(element.Values)
	if len(values) < 2 {
		return
	}
	if fill, ok := parseSlideColor(element.Fill); ok {
		overlayRoundedRect(img, rect, element.Radius, fill)
	}
	plotRect := rect
	labelColor := chartLabelColor(element, palette)
	headerUsed := false
	captionText := strings.TrimSpace(element.Text)
	valueText := chartSparklineValueText(element)
	if labelFace != nil && (captionText != "" || valueText != "") {
		headerHeight := slideLineHeight(labelFace)
		if rect.Dy() >= headerHeight+18 {
			headerRect := image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+headerHeight)
			if captionText != "" && valueText != "" {
				splitX := headerRect.Min.X + int(math.Round(float64(headerRect.Dx())*0.58))
				drawChartTextLine(img, labelFace, image.Rect(headerRect.Min.X, headerRect.Min.Y, splitX, headerRect.Max.Y), captionText, labelColor, "left")
				drawChartTextLine(img, labelFace, image.Rect(splitX+8, headerRect.Min.Y, headerRect.Max.X, headerRect.Max.Y), valueText, labelColor, "right")
			} else if captionText != "" {
				drawChartTextLine(img, labelFace, headerRect, captionText, labelColor, "left")
			} else {
				drawChartTextLine(img, labelFace, headerRect, valueText, labelColor, "right")
			}
			plotRect.Min.Y += headerHeight + 4
			headerUsed = true
		}
	}
	if plotRect.Empty() {
		return
	}
	points := chartPoints(plotRect, values)
	lineColor := chartAccentColor(element, palette)
	width := maxInt(1, element.StrokeWidth)
	for idx := 1; idx < len(points); idx++ {
		drawChartLine(img, points[idx-1], points[idx], lineColor, width)
	}
	for _, point := range points {
		drawChartDot(img, point, maxInt(1, width+1), lineColor)
	}
	drawChartDot(img, points[len(points)-1], maxInt(2, width+2), lineColor)
	if !headerUsed && labelFace != nil && valueText != "" {
		valueText = chartSingleLine(labelFace, valueText, maxInt(48, plotRect.Dx()/2))
		if valueText != "" {
			lineHeight := slideLineHeight(labelFace)
			textWidth := measureSlideText(labelFace, valueText)
			x := minInt(rect.Max.X-textWidth, points[len(points)-1].X+8)
			x = maxInt(rect.Min.X, x)
			y := minInt(rect.Max.Y-lineHeight, points[len(points)-1].Y-lineHeight)
			y = maxInt(rect.Min.Y, y)
			drawSlideTextLine(img, labelFace, x, y, valueText, labelColor)
		}
	}
}

func renderSlideLayoutBars(
	img *image.RGBA,
	rect image.Rectangle,
	element slidespec.LayoutElement,
	palette slidePalette,
	labelFace font.Face,
) {
	values := chartBarRatios(element.Values)
	if len(values) == 0 {
		return
	}
	if fill, ok := parseSlideColor(element.Fill); ok {
		overlayRoundedRect(img, rect, element.Radius, fill)
	}
	if normalizeChartDirection(element.ChartType, element.Direction) == "horizontal" {
		renderSlideLayoutHorizontalBars(img, rect, element, values, palette, labelFace)
	} else {
		renderSlideLayoutVerticalBars(img, rect, element, values, palette, labelFace)
	}
	if stroke, ok := parseSlideColor(element.Stroke); ok {
		strokeRoundedRect(img, rect, stroke, maxInt(1, element.StrokeWidth), normalizeRoundedRadius(rect, element.Radius))
	}
}

func renderSlideLayoutText(
	img *image.RGBA,
	rect image.Rectangle,
	element slidespec.LayoutElement,
	canvas slideCanvasSpec,
	baseWidth int,
	defaultFonts slideFontPack,
	palette slidePalette,
) {
	text := strings.TrimSpace(element.Text)
	if text == "" {
		return
	}
	scale := float64(canvas.width) / float64(maxInt(baseWidth, 1))
	face, owned := slideLayoutFontFace(element, defaultFonts, scale)
	if owned {
		defer closeSlideFace(face)
	}

	maxLines := element.MaxLines
	if maxLines <= 0 {
		switch strings.ToLower(strings.TrimSpace(element.FontRole)) {
		case "title":
			maxLines = 3
		case "subtitle":
			maxLines = 3
		default:
			maxLines = 4
		}
	}
	colorValue := palette.Text
	if parsed, ok := parseSlideColor(element.Color); ok {
		colorValue = parsed
	} else {
		switch strings.ToLower(strings.TrimSpace(element.FontRole)) {
		case "subtitle", "eyebrow", "chip":
			colorValue = palette.Muted
		}
	}
	drawSlideWrappedAlignedText(img, face, rect, text, colorValue, maxLines, element.Align, element.LineHeight)
}

func slideLayoutFontFace(element slidespec.LayoutElement, defaults slideFontPack, scale float64) (font.Face, bool) {
	defaultStyle := defaults.styleForRole(element.FontRole)
	size := defaultStyle.size
	if element.FontSize > 0 {
		size = float64(element.FontSize)
		if scale > 0 {
			size = size * scale
		}
	}
	family := defaultStyle.family
	if strings.TrimSpace(element.FontFamily) != "" {
		family = normalizeSlideFontFamily(element.FontFamily)
	}
	weight := defaultStyle.weight
	if strings.TrimSpace(element.FontWeight) != "" {
		weight = normalizeSlideFontWeight(element.FontWeight)
	}
	if element.FontSize <= 0 && family == defaultStyle.family && weight == defaultStyle.weight {
		return defaultStyle.face, false
	}
	return newSlideFontFaceVariant(size, family, weight), true
}

func slideLayoutChartFontFace(element slidespec.LayoutElement, defaults slideFontPack, scale float64) (font.Face, bool) {
	if strings.TrimSpace(element.FontRole) == "" {
		element.FontRole = "chip"
	}
	return slideLayoutFontFace(element, defaults, scale)
}

func chartAccentColor(element slidespec.LayoutElement, palette slidePalette) color.RGBA {
	for _, candidate := range []string{element.Accent, element.Color, element.Stroke} {
		if parsed, ok := parseSlideColor(candidate); ok {
			return parsed
		}
	}
	return palette.Accent
}

func chartTrackColor(element slidespec.LayoutElement, palette slidePalette) color.RGBA {
	if parsed, ok := parseSlideColor(element.Fill); ok {
		return parsed
	}
	return color.RGBA{R: palette.Panel.R, G: palette.Panel.G, B: palette.Panel.B, A: 46}
}

func chartLabelColor(element slidespec.LayoutElement, palette slidePalette) color.RGBA {
	if parsed, ok := parseSlideColor(element.Color); ok {
		return parsed
	}
	return palette.Text
}

func normalizeChartDirection(chartType, raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "horizontal", "vertical":
		return strings.ToLower(strings.TrimSpace(raw))
	}
	if strings.EqualFold(strings.TrimSpace(chartType), "progress") {
		return "horizontal"
	}
	return "vertical"
}

func chartRatio(element slidespec.LayoutElement) float64 {
	maxValue := element.MaxValue
	if maxValue <= 0 {
		maxValue = 100
	}
	ratio := element.Value / maxValue
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

func chartBarRatios(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	maxValue := 0.0
	for _, value := range values {
		magnitude := math.Abs(value)
		if magnitude > maxValue {
			maxValue = magnitude
		}
	}
	out := make([]float64, 0, len(values))
	if maxValue <= 0 {
		for range values {
			out = append(out, 0.5)
		}
		return out
	}
	for _, value := range values {
		out = append(out, math.Abs(value)/maxValue)
	}
	return out
}

func normalizedChartValues(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	minValue := values[0]
	maxValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	span := maxValue - minValue
	out := make([]float64, 0, len(values))
	for _, value := range values {
		if span <= 0 {
			out = append(out, 0.5)
			continue
		}
		out = append(out, (value-minValue)/span)
	}
	return out
}

func chartPoints(rect image.Rectangle, values []float64) []image.Point {
	points := make([]image.Point, 0, len(values))
	if len(values) == 1 {
		return []image.Point{{X: rect.Min.X + rect.Dx()/2, Y: rect.Min.Y + rect.Dy()/2}}
	}
	paddingY := maxInt(3, rect.Dy()/10)
	for idx, value := range values {
		x := rect.Min.X + int(math.Round(float64(rect.Dx()-1)*float64(idx)/float64(len(values)-1)))
		y := rect.Max.Y - 1 - paddingY - int(math.Round(value*float64(maxInt(1, rect.Dy()-paddingY*2-1))))
		if y < rect.Min.Y {
			y = rect.Min.Y
		}
		if y >= rect.Max.Y {
			y = rect.Max.Y - 1
		}
		points = append(points, image.Point{X: x, Y: y})
	}
	return points
}

func renderSlideLayoutVerticalBars(
	img *image.RGBA,
	rect image.Rectangle,
	element slidespec.LayoutElement,
	values []float64,
	palette slidePalette,
	labelFace font.Face,
) {
	valueHeight := 0
	labelHeight := 0
	if labelFace != nil && strings.TrimSpace(element.ValueFormat) != "" && rect.Dy() >= slideLineHeight(labelFace)*2+28 {
		valueHeight = slideLineHeight(labelFace)
	}
	if labelFace != nil && len(element.Labels) > 0 && rect.Dy() >= slideLineHeight(labelFace)+28 {
		labelHeight = slideLineHeight(labelFace)
	}

	contentRect := rect
	if valueHeight > 0 {
		contentRect.Min.Y += valueHeight + 4
	}
	if labelHeight > 0 {
		contentRect.Max.Y -= labelHeight + 4
	}
	if contentRect.Empty() {
		return
	}

	gap := maxInt(6, contentRect.Dx()/maxInt(24, len(values)*4))
	barWidth := maxInt(6, (contentRect.Dx()-gap*(len(values)-1))/len(values))
	accent := chartAccentColor(element, palette)
	labelColor := chartLabelColor(element, palette)
	for idx, value := range values {
		barMinX := contentRect.Min.X + idx*(barWidth+gap)
		barRect := image.Rect(
			barMinX,
			contentRect.Max.Y-maxInt(2, int(math.Round(float64(contentRect.Dy())*value))),
			minInt(contentRect.Max.X, barMinX+barWidth),
			contentRect.Max.Y,
		)
		if barRect.Empty() {
			continue
		}
		overlayRoundedRect(img, barRect, minInt(maxInt(2, element.Radius), barRect.Dx()/2), accent)
		if valueHeight > 0 {
			valueRect := image.Rect(barRect.Min.X, rect.Min.Y, barRect.Max.X, rect.Min.Y+valueHeight)
			drawChartTextLine(img, labelFace, valueRect, formatChartValue(element.Values[idx], element.ValueFormat), labelColor, "center")
		}
		if labelHeight > 0 && idx < len(element.Labels) {
			labelRect := image.Rect(barRect.Min.X, rect.Max.Y-labelHeight, barRect.Max.X, rect.Max.Y)
			drawChartTextLine(img, labelFace, labelRect, element.Labels[idx], labelColor, "center")
		}
	}
}

func renderSlideLayoutHorizontalBars(
	img *image.RGBA,
	rect image.Rectangle,
	element slidespec.LayoutElement,
	values []float64,
	palette slidePalette,
	labelFace font.Face,
) {
	labelWidth := 0
	if labelFace != nil && len(element.Labels) > 0 && rect.Dx() >= 120 {
		labelWidth = chartHorizontalLabelWidth(labelFace, element.Labels, rect.Dx())
	}
	valueWidth := 0
	if labelFace != nil && strings.TrimSpace(element.ValueFormat) != "" && rect.Dx()-labelWidth >= 80 {
		valueWidth = chartHorizontalValueWidth(labelFace, element.Values, element.ValueFormat, rect.Dx()-labelWidth)
	}

	contentRect := rect
	if labelWidth > 0 {
		contentRect.Min.X += labelWidth + 8
	}
	if valueWidth > 0 {
		contentRect.Max.X -= valueWidth + 8
	}
	if contentRect.Empty() {
		return
	}

	gap := maxInt(6, rect.Dy()/maxInt(18, len(values)*4))
	rowHeight := maxInt(10, (rect.Dy()-gap*(len(values)-1))/len(values))
	accent := chartAccentColor(element, palette)
	track := chartTrackColor(element, palette)
	labelColor := chartLabelColor(element, palette)
	for idx, value := range values {
		rowMinY := rect.Min.Y + idx*(rowHeight+gap)
		barRect := image.Rect(contentRect.Min.X, rowMinY, contentRect.Max.X, minInt(rect.Max.Y, rowMinY+rowHeight))
		if barRect.Empty() {
			continue
		}
		overlayRoundedRect(img, barRect, normalizeRoundedRadius(barRect, maxInt(element.Radius, barRect.Dy()/2)), track)
		fillRect := barRect
		fillRect.Max.X = fillRect.Min.X + int(math.Round(float64(barRect.Dx())*value))
		if fillRect.Dx() > 0 {
			overlayRoundedRect(img, fillRect, normalizeRoundedRadius(fillRect, maxInt(element.Radius, fillRect.Dy()/2)), accent)
		}
		if labelWidth > 0 && idx < len(element.Labels) {
			labelRect := image.Rect(rect.Min.X, rowMinY, rect.Min.X+labelWidth, minInt(rect.Max.Y, rowMinY+rowHeight))
			drawChartTextLine(img, labelFace, labelRect, element.Labels[idx], labelColor, "right")
		}
		if valueWidth > 0 {
			valueRect := image.Rect(rect.Max.X-valueWidth, rowMinY, rect.Max.X, minInt(rect.Max.Y, rowMinY+rowHeight))
			drawChartTextLine(img, labelFace, valueRect, formatChartValue(element.Values[idx], element.ValueFormat), labelColor, "right")
		}
	}
}

func chartHorizontalLabelWidth(face font.Face, labels []string, totalWidth int) int {
	limit := maxInt(48, totalWidth/3)
	maxWidth := 0
	for _, label := range labels {
		width := measureSlideText(face, chartSingleLine(face, label, limit))
		if width > maxWidth {
			maxWidth = width
		}
	}
	if maxWidth == 0 {
		return 0
	}
	return minInt(limit, maxWidth+10)
}

func chartHorizontalValueWidth(face font.Face, values []float64, valueFormat string, availableWidth int) int {
	limit := maxInt(42, minInt(availableWidth/4, 120))
	maxWidth := 0
	for _, value := range values {
		width := measureSlideText(face, chartSingleLine(face, formatChartValue(value, valueFormat), limit))
		if width > maxWidth {
			maxWidth = width
		}
	}
	if maxWidth == 0 {
		return 0
	}
	return minInt(limit, maxWidth+6)
}

func chartPrimaryLabel(element slidespec.LayoutElement) string {
	if text := strings.TrimSpace(element.Text); text != "" {
		return text
	}
	for _, label := range element.Labels {
		if text := strings.TrimSpace(label); text != "" {
			return text
		}
	}
	return ""
}

func chartLastLabel(element slidespec.LayoutElement) string {
	for idx := len(element.Labels) - 1; idx >= 0; idx-- {
		if text := strings.TrimSpace(element.Labels[idx]); text != "" {
			return text
		}
	}
	return ""
}

func chartProgressValueText(element slidespec.LayoutElement) string {
	if strings.TrimSpace(element.ValueFormat) == "" {
		return ""
	}
	value := element.Value
	switch strings.ToLower(strings.TrimSpace(element.ValueFormat)) {
	case "percent", "percentage":
		value = chartRatio(element) * 100
	}
	return formatChartValue(value, element.ValueFormat)
}

func chartSparklineValueText(element slidespec.LayoutElement) string {
	if len(element.Values) == 0 || strings.TrimSpace(element.ValueFormat) == "" {
		return ""
	}
	valueText := formatChartValue(element.Values[len(element.Values)-1], element.ValueFormat)
	if label := chartLastLabel(element); label != "" {
		return strings.TrimSpace(label + " " + valueText)
	}
	return valueText
}

func formatChartValue(value float64, valueFormat string) string {
	switch strings.ToLower(strings.TrimSpace(valueFormat)) {
	case "integer", "int":
		return strconv.Itoa(int(math.Round(value)))
	case "decimal1":
		return strconv.FormatFloat(value, 'f', 1, 64)
	case "decimal2":
		return strconv.FormatFloat(value, 'f', 2, 64)
	case "percent", "percentage":
		return trimChartFloat(value) + "%"
	case "currency", "usd":
		return "$" + trimChartFloat(value)
	case "multiplier", "x":
		return trimChartFloat(value) + "x"
	default:
		return trimChartFloat(value)
	}
}

func trimChartFloat(value float64) string {
	rounded := math.Round(value*100) / 100
	if math.Abs(rounded-math.Round(rounded)) < 0.001 {
		return strconv.Itoa(int(math.Round(rounded)))
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(rounded, 'f', 2, 64), "0"), ".")
}

func chartSingleLine(face font.Face, text string, width int) string {
	if face == nil || strings.TrimSpace(text) == "" || width <= 0 {
		return ""
	}
	lines := wrapSlideText(face, text, width, 1)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func drawChartTextLine(img *image.RGBA, face font.Face, rect image.Rectangle, text string, c color.Color, align string) {
	if face == nil || rect.Empty() {
		return
	}
	line := chartSingleLine(face, text, rect.Dx())
	if line == "" {
		return
	}
	x := rect.Min.X
	switch strings.ToLower(strings.TrimSpace(align)) {
	case "center":
		x = rect.Min.X + maxInt(0, (rect.Dx()-measureSlideText(face, line))/2)
	case "right":
		x = rect.Max.X - measureSlideText(face, line)
	}
	y := rect.Min.Y + maxInt(0, (rect.Dy()-slideLineHeight(face))/2)
	drawSlideTextLine(img, face, x, y, line, c)
}

func drawChartLine(img *image.RGBA, start, end image.Point, c color.Color, width int) {
	if img == nil {
		return
	}
	dx := float64(end.X - start.X)
	dy := float64(end.Y - start.Y)
	steps := maxInt(int(math.Abs(dx)), int(math.Abs(dy)))
	if steps == 0 {
		drawChartDot(img, start, width, c)
		return
	}
	for step := 0; step <= steps; step++ {
		t := float64(step) / float64(steps)
		x := int(math.Round(float64(start.X) + dx*t))
		y := int(math.Round(float64(start.Y) + dy*t))
		drawChartDot(img, image.Point{X: x, Y: y}, width, c)
	}
}

func drawChartDot(img *image.RGBA, center image.Point, radius int, c color.Color) {
	if img == nil {
		return
	}
	radius = maxInt(1, radius)
	rect := image.Rect(center.X-radius, center.Y-radius, center.X+radius+1, center.Y+radius+1).Intersect(img.Bounds())
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := x - center.X
			dy := y - center.Y
			if dx*dx+dy*dy <= radius*radius {
				img.Set(x, y, c)
			}
		}
	}
}

func drawSlideWrappedAlignedText(
	img *image.RGBA,
	face font.Face,
	rect image.Rectangle,
	text string,
	c color.Color,
	maxLines int,
	align string,
	lineHeightMultiplier float64,
) int {
	lines := wrapSlideText(face, text, rect.Dx(), maxLines)
	if len(lines) == 0 {
		return 0
	}
	lineHeight := slideLineHeightWithMultiplier(face, lineHeightMultiplier)
	y := rect.Min.Y
	drawn := 0
	for _, line := range lines {
		if y+lineHeight > rect.Max.Y {
			break
		}
		x := rect.Min.X
		switch strings.ToLower(strings.TrimSpace(align)) {
		case "center":
			x = rect.Min.X + maxInt(0, (rect.Dx()-measureSlideText(face, line))/2)
		case "right":
			x = rect.Max.X - measureSlideText(face, line)
		}
		drawSlideTextLine(img, face, x, y, line, c)
		y += lineHeight
		drawn++
	}
	return drawn * lineHeight
}

func drawImageFitRect(dst *image.RGBA, src image.Image, rect image.Rectangle, fit string) {
	if strings.EqualFold(strings.TrimSpace(fit), "contain") {
		drawImageContainRect(dst, src, rect)
		return
	}
	drawImageCoverRect(dst, src, rect)
}

func drawImageFitRoundedRect(dst *image.RGBA, src image.Image, rect image.Rectangle, fit string, radius int) {
	if radius <= 0 {
		drawImageFitRect(dst, src, rect, fit)
		return
	}
	if dst == nil || src == nil {
		return
	}
	rect = rect.Intersect(dst.Bounds())
	if rect.Empty() {
		return
	}
	temp := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	drawImageFitRect(temp, src, temp.Bounds(), fit)
	mask := roundedRectMask(temp.Bounds(), radius)
	stdDraw.DrawMask(dst, rect, temp, image.Point{}, mask, image.Point{}, stdDraw.Over)
}

func drawImageContainRect(dst *image.RGBA, src image.Image, rect image.Rectangle) {
	if dst == nil || src == nil {
		return
	}
	rect = rect.Intersect(dst.Bounds())
	if rect.Empty() {
		return
	}
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	if sw <= 0 || sh <= 0 {
		return
	}
	scale := math.Min(float64(rect.Dx())/float64(sw), float64(rect.Dy())/float64(sh))
	tw := maxInt(1, int(float64(sw)*scale))
	th := maxInt(1, int(float64(sh)*scale))
	scaled := image.NewRGBA(image.Rect(0, 0, tw, th))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), stdDraw.Over, nil)
	drawRect := image.Rect(
		rect.Min.X+(rect.Dx()-tw)/2,
		rect.Min.Y+(rect.Dy()-th)/2,
		rect.Min.X+(rect.Dx()-tw)/2+tw,
		rect.Min.Y+(rect.Dy()-th)/2+th,
	)
	stdDraw.Draw(dst, drawRect, scaled, image.Point{}, stdDraw.Over)
}

func resolveLayoutImageSource(raw, visualDataURL string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "none":
		return ""
	case "visual", "hero", "reference":
		return strings.TrimSpace(visualDataURL)
	default:
		return strings.TrimSpace(raw)
	}
}

func parseSlideColor(raw string) (color.RGBA, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return color.RGBA{}, false
	}
	if strings.HasPrefix(trimmed, "#") {
		return parseSlideHexColor(trimmed)
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "rgba(") || strings.HasPrefix(lower, "rgb(") {
		return parseSlideRGBFunc(trimmed)
	}
	return color.RGBA{}, false
}

func parseSlideHexColor(raw string) (color.RGBA, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(raw), "#")
	switch len(trimmed) {
	case 3:
		trimmed = strings.Repeat(string(trimmed[0]), 2) +
			strings.Repeat(string(trimmed[1]), 2) +
			strings.Repeat(string(trimmed[2]), 2) + "ff"
	case 6:
		trimmed += "ff"
	case 8:
	default:
		return color.RGBA{}, false
	}
	value, err := strconv.ParseUint(trimmed, 16, 32)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{
		R: uint8(value >> 24),
		G: uint8(value >> 16),
		B: uint8(value >> 8),
		A: uint8(value),
	}, true
}

func parseSlideRGBFunc(raw string) (color.RGBA, bool) {
	start := strings.IndexByte(raw, '(')
	end := strings.LastIndexByte(raw, ')')
	if start < 0 || end <= start {
		return color.RGBA{}, false
	}
	parts := strings.Split(raw[start+1:end], ",")
	if len(parts) < 3 {
		return color.RGBA{}, false
	}
	r, okR := parseSlideColorComponent(parts[0], false)
	g, okG := parseSlideColorComponent(parts[1], false)
	b, okB := parseSlideColorComponent(parts[2], false)
	if !okR || !okG || !okB {
		return color.RGBA{}, false
	}
	a := uint8(255)
	if len(parts) >= 4 {
		alpha, ok := parseSlideColorComponent(parts[3], true)
		if !ok {
			return color.RGBA{}, false
		}
		a = alpha
	}
	return color.RGBA{R: r, G: g, B: b, A: a}, true
}

func parseSlideColorComponent(raw string, alpha bool) (uint8, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	if strings.HasSuffix(trimmed, "%") {
		value, err := strconv.ParseFloat(strings.TrimSuffix(trimmed, "%"), 64)
		if err != nil {
			return 0, false
		}
		value = maxFloat(0, minFloat(100, value))
		return uint8(math.Round(value * 255 / 100)), true
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	if alpha && value >= 0 && value <= 1 {
		return uint8(math.Round(value * 255)), true
	}
	value = maxFloat(0, minFloat(255, value))
	return uint8(math.Round(value)), true
}
