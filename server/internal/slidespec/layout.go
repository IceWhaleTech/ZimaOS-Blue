package slidespec

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// LayoutSpec is a structured, renderer-friendly description of a slide.
// It gives callers a stable contract for describing canvas size, background,
// text blocks, images, and simple shapes without depending on prompt-only T2I.
type LayoutSpec struct {
	RenderMode  string           `json:"render_mode,omitempty"`
	StylePreset string           `json:"style_preset,omitempty"`
	TemplateID  string           `json:"template_id,omitempty"`
	Theme       string           `json:"theme,omitempty"`
	Canvas      LayoutCanvas     `json:"canvas,omitempty"`
	Background  LayoutBackground `json:"background,omitempty"`
	Elements    []LayoutElement  `json:"elements,omitempty"`
}

type LayoutCanvas struct {
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
}

type LayoutBackground struct {
	Fill        string   `json:"fill,omitempty"`
	Gradient    []string `json:"gradient,omitempty"`
	ImageSource string   `json:"image_source,omitempty"`
	ImageFit    string   `json:"image_fit,omitempty"`
	Overlay     string   `json:"overlay,omitempty"`
}

type LayoutElement struct {
	Kind        string    `json:"kind,omitempty"` // text | image | shape | chart
	Name        string    `json:"name,omitempty"`
	X           int       `json:"x,omitempty"`
	Y           int       `json:"y,omitempty"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	ZIndex      int       `json:"z_index,omitempty"`
	Radius      int       `json:"radius,omitempty"`
	Fill        string    `json:"fill,omitempty"`
	Stroke      string    `json:"stroke,omitempty"`
	StrokeWidth int       `json:"stroke_width,omitempty"`
	Text        string    `json:"text,omitempty"`
	FontFamily  string    `json:"font_family,omitempty"` // sans | mono | display
	FontRole    string    `json:"font_role,omitempty"`   // eyebrow | title | subtitle | body | chip
	FontSize    int       `json:"font_size,omitempty"`
	FontWeight  string    `json:"font_weight,omitempty"` // regular | medium | semibold | bold | italic
	Color       string    `json:"color,omitempty"`
	Align       string    `json:"align,omitempty"` // left | center | right
	MaxLines    int       `json:"max_lines,omitempty"`
	LineHeight  float64   `json:"line_height,omitempty"`
	Source      string    `json:"source,omitempty"`     // data URL or alias like "visual"
	Fit         string    `json:"fit,omitempty"`        // cover | contain
	ChartType   string    `json:"chart_type,omitempty"` // divider | progress | sparkline | bars
	Value       float64   `json:"value,omitempty"`
	MaxValue    float64   `json:"max_value,omitempty"`
	Values      []float64 `json:"values,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	ValueFormat string    `json:"value_format,omitempty"` // number | integer | decimal1 | decimal2 | percent | currency | multiplier
	Direction   string    `json:"direction,omitempty"`    // horizontal | vertical
	Accent      string    `json:"accent,omitempty"`
}

func (spec LayoutSpec) HasContent() bool {
	if len(spec.Elements) > 0 {
		return true
	}
	if strings.TrimSpace(spec.Background.Fill) != "" || len(spec.Background.Gradient) > 0 {
		return true
	}
	if strings.TrimSpace(spec.Background.ImageSource) != "" || strings.TrimSpace(spec.Background.Overlay) != "" {
		return true
	}
	return false
}

// DecodeLayoutSpec accepts a JSON string, generic map, or typed struct.
func DecodeLayoutSpec(raw any) (LayoutSpec, bool) {
	switch typed := raw.(type) {
	case nil:
		return LayoutSpec{}, false
	case LayoutSpec:
		return typed, typed.HasContent()
	case *LayoutSpec:
		if typed == nil {
			return LayoutSpec{}, false
		}
		return *typed, typed.HasContent()
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return LayoutSpec{}, false
		}
		var spec LayoutSpec
		if err := json.Unmarshal([]byte(trimmed), &spec); err != nil {
			return LayoutSpec{}, false
		}
		return spec, spec.HasContent()
	default:
		payload, err := json.Marshal(raw)
		if err != nil {
			return LayoutSpec{}, false
		}
		var spec LayoutSpec
		if err := json.Unmarshal(payload, &spec); err != nil {
			return LayoutSpec{}, false
		}
		return spec, spec.HasContent()
	}
}

// NormalizeLayoutSpec fills missing metadata and falls back to the default
// internal layout when a partial spec omitted the element list.
func NormalizeLayoutSpec(spec LayoutSpec, brief Brief, canvasWidth, canvasHeight int, hasVisual bool) LayoutSpec {
	if spec.RenderMode == "" {
		spec.RenderMode = RenderModeSlide
	}
	if spec.StylePreset == "" {
		spec.StylePreset = brief.StylePreset
	}
	if spec.Theme == "" {
		spec.Theme = brief.Theme
	}
	if spec.TemplateID == "" || spec.TemplateID == TemplatePoster {
		spec.TemplateID = effectiveTemplateID(brief, hasVisual)
	}
	if spec.Canvas.Width <= 0 {
		spec.Canvas.Width = defaultCanvasWidth(canvasWidth)
	}
	if spec.Canvas.Height <= 0 {
		spec.Canvas.Height = defaultCanvasHeight(canvasHeight)
	}
	if spec.Canvas.AspectRatio == "" {
		spec.Canvas.AspectRatio = firstNonEmpty(brief.AspectRatio, DefaultAspectRatio)
	}
	if len(spec.Elements) == 0 {
		defaultSpec := BuildLayoutSpec(brief, spec.Canvas.Width, spec.Canvas.Height, hasVisual)
		spec.Elements = defaultSpec.Elements
		if !spec.BackgroundHasContent() {
			spec.Background = defaultSpec.Background
		}
	}
	return spec
}

func (spec LayoutSpec) BackgroundHasContent() bool {
	if strings.TrimSpace(spec.Background.Fill) != "" || len(spec.Background.Gradient) > 0 {
		return true
	}
	if strings.TrimSpace(spec.Background.ImageSource) != "" || strings.TrimSpace(spec.Background.Overlay) != "" {
		return true
	}
	return false
}

// BuildLayoutSpec creates an internal structured layout from the extracted
// slide brief. The coordinates are absolute in the returned canvas space.
func BuildLayoutSpec(brief Brief, canvasWidth, canvasHeight int, hasVisual bool) LayoutSpec {
	width := defaultCanvasWidth(canvasWidth)
	height := defaultCanvasHeight(canvasHeight)
	templateID := effectiveTemplateID(brief, hasVisual)
	spec := LayoutSpec{
		RenderMode:  RenderModeSlide,
		StylePreset: brief.StylePreset,
		TemplateID:  templateID,
		Theme:       brief.Theme,
		Canvas: LayoutCanvas{
			Width:       width,
			Height:      height,
			AspectRatio: firstNonEmpty(brief.AspectRatio, DefaultAspectRatio),
		},
	}
	switch templateID {
	case TemplateCover:
		spec.Elements = buildCoverLayoutElements(brief, width, height)
	case TemplateSplit:
		spec.Elements = buildSplitLayoutElements(brief, width, height)
	case TemplateAgenda:
		spec.Elements = buildAgendaLayoutElements(brief, width, height)
	case TemplateMetrics:
		spec.Elements = buildMetricsLayoutElements(brief, width, height)
	default:
		spec.TemplateID = TemplateTextOnly
		spec.Elements = buildTextOnlyLayoutElements(brief, width, height)
	}
	return spec
}

func effectiveTemplateID(brief Brief, hasVisual bool) string {
	switch cleanSpace(brief.TemplateID) {
	case TemplateCover:
		if hasVisual {
			return TemplateCover
		}
		return TemplateTextOnly
	case TemplateSplit:
		if hasVisual {
			return TemplateSplit
		}
		return TemplateTextOnly
	case TemplateTextOnly:
		return TemplateTextOnly
	case TemplateAgenda:
		return TemplateAgenda
	case TemplateMetrics:
		return TemplateMetrics
	}
	return DecideTemplate(brief, hasVisual)
}

func defaultCanvasWidth(width int) int {
	if width <= 0 {
		return 1280
	}
	return width
}

func defaultCanvasHeight(height int) int {
	if height <= 0 {
		return 720
	}
	return height
}

func buildCoverLayoutElements(brief Brief, width, height int) []LayoutElement {
	title := firstNonEmpty(cleanSpace(brief.Title), defaultLayoutTitle(brief, TemplateCover))
	subtitle := cleanSpace(brief.Subtitle)
	eyebrow := layoutEyebrowText(brief, TemplateCover)
	leftX := 92
	leftW := maxInt(280, int(float64(width)*0.30))
	heroX := int(float64(width) * 0.44)
	heroY := 52
	heroH := height - 104
	elements := []LayoutElement{
		{
			Kind:       "text",
			Name:       "eyebrow",
			X:          leftX,
			Y:          96,
			Width:      320,
			Height:     34,
			ZIndex:     2,
			Text:       eyebrow,
			FontRole:   "eyebrow",
			FontWeight: "semibold",
			Color:      "rgba(226,232,240,0.88)",
			MaxLines:   1,
		},
		{
			Kind:       "text",
			Name:       "title",
			X:          leftX,
			Y:          150,
			Width:      leftW,
			Height:     maxInt(180, int(float64(height)*0.30)),
			ZIndex:     2,
			Text:       title,
			FontRole:   "title",
			FontWeight: "bold",
			Color:      "#F8FAFC",
			MaxLines:   3,
		},
		{
			Kind:        "image",
			Name:        "hero",
			X:           heroX,
			Y:           heroY,
			Width:       width - heroX - 52,
			Height:      heroH,
			ZIndex:      1,
			Radius:      24,
			Source:      "visual",
			Fit:         "cover",
			Stroke:      "rgba(255,255,255,0.14)",
			StrokeWidth: 1,
		},
	}
	if subtitle != "" {
		elements = append(elements, LayoutElement{
			Kind:       "text",
			Name:       "subtitle",
			X:          leftX,
			Y:          358,
			Width:      maxInt(320, int(float64(width)*0.34)),
			Height:     110,
			ZIndex:     2,
			Text:       subtitle,
			FontRole:   "subtitle",
			Color:      "rgba(226,232,240,0.84)",
			MaxLines:   3,
			LineHeight: 1.34,
		})
	}
	return append(elements, buildCardElements(brief.Bullets[:minInt(len(brief.Bullets), 2)], leftX, maxInt(462, int(float64(height)*0.64)), leftW, height-maxInt(462, int(float64(height)*0.64))-92, 1)...)
}

func buildSplitLayoutElements(brief Brief, width, height int) []LayoutElement {
	title := firstNonEmpty(cleanSpace(brief.Title), defaultLayoutTitle(brief, TemplateSplit))
	subtitle := cleanSpace(brief.Subtitle)
	eyebrow := layoutEyebrowText(brief, TemplateSplit)
	leftX := 84
	leftW := maxInt(340, int(float64(width)*0.38))
	panelX := int(float64(width) * 0.54)
	panelW := width - panelX - 56
	elements := []LayoutElement{
		{
			Kind:        "image",
			Name:        "hero",
			X:           panelX,
			Y:           56,
			Width:       panelW,
			Height:      height - 112,
			ZIndex:      1,
			Radius:      28,
			Source:      "visual",
			Fit:         "cover",
			Stroke:      "rgba(255,255,255,0.14)",
			StrokeWidth: 1,
		},
		{
			Kind:       "text",
			Name:       "eyebrow",
			X:          leftX,
			Y:          96,
			Width:      320,
			Height:     34,
			ZIndex:     2,
			Text:       eyebrow,
			FontRole:   "eyebrow",
			FontWeight: "semibold",
			Color:      "rgba(226,232,240,0.88)",
			MaxLines:   1,
		},
		{
			Kind:       "text",
			Name:       "title",
			X:          leftX,
			Y:          146,
			Width:      leftW,
			Height:     maxInt(180, int(float64(height)*0.28)),
			ZIndex:     2,
			Text:       title,
			FontRole:   "title",
			FontWeight: "bold",
			Color:      "#F8FAFC",
			MaxLines:   3,
		},
	}
	cardTop := 430
	if subtitle != "" {
		elements = append(elements, LayoutElement{
			Kind:       "text",
			Name:       "subtitle",
			X:          leftX,
			Y:          350,
			Width:      leftW,
			Height:     100,
			ZIndex:     2,
			Text:       subtitle,
			FontRole:   "subtitle",
			Color:      "rgba(226,232,240,0.84)",
			MaxLines:   3,
			LineHeight: 1.34,
		})
		cardTop = 454
	}
	if len(brief.Bullets) > 0 {
		elements = append(elements, buildCardElements(brief.Bullets, leftX, cardTop, leftW, height-cardTop-84, 1)...)
		return elements
	}
	note := firstNonEmpty(cleanSpace(brief.VisualQuery), subtitle, "Presentation-ready summary")
	elements = append(elements, LayoutElement{
		Kind:       "text",
		Name:       "note",
		X:          leftX,
		Y:          cardTop,
		Width:      leftW,
		Height:     height - cardTop - 84,
		ZIndex:     2,
		Text:       note,
		FontRole:   "body",
		Color:      "rgba(219,234,254,0.90)",
		MaxLines:   5,
		LineHeight: 1.34,
	})
	return elements
}

func buildTextOnlyLayoutElements(brief Brief, width, height int) []LayoutElement {
	title := firstNonEmpty(cleanSpace(brief.Title), defaultLayoutTitle(brief, TemplateTextOnly))
	subtitle := cleanSpace(brief.Subtitle)
	eyebrow := layoutEyebrowText(brief, TemplateTextOnly)
	boardX := 56
	boardY := 52
	boardW := width - 112
	boardH := height - 104
	leftX := boardX + 40
	rightX := boardX + int(float64(boardW)*0.50)
	leftW := maxInt(320, int(float64(boardW)*0.38))
	rightW := boardX + boardW - 34 - rightX
	elements := []LayoutElement{
		{
			Kind:        "shape",
			Name:        "board",
			X:           boardX,
			Y:           boardY,
			Width:       boardW,
			Height:      boardH,
			ZIndex:      1,
			Radius:      30,
			Fill:        "rgba(255,255,255,0.08)",
			Stroke:      "rgba(255,255,255,0.10)",
			StrokeWidth: 1,
		},
		{
			Kind:       "text",
			Name:       "eyebrow",
			X:          leftX,
			Y:          boardY + 40,
			Width:      320,
			Height:     34,
			ZIndex:     2,
			Text:       eyebrow,
			FontRole:   "eyebrow",
			FontWeight: "semibold",
			Color:      "rgba(226,232,240,0.88)",
			MaxLines:   1,
		},
		{
			Kind:       "text",
			Name:       "title",
			X:          leftX,
			Y:          boardY + 96,
			Width:      leftW,
			Height:     maxInt(220, int(float64(boardH)*0.34)),
			ZIndex:     2,
			Text:       title,
			FontRole:   "title",
			FontWeight: "bold",
			Color:      "#F8FAFC",
			MaxLines:   4,
		},
	}
	if IsNanoSlidesPreset(brief.StylePreset) {
		elements = append(elements, LayoutElement{
			Kind:   "shape",
			Name:   "accent-rail",
			X:      leftX - 18,
			Y:      boardY + 34,
			Width:  10,
			Height: boardH - 68,
			ZIndex: 2,
			Radius: 5,
			Fill:   "rgba(244,196,92,0.44)",
			Stroke: "rgba(244,196,92,0.44)",
		})
	}
	bodyTop := boardY + 318
	if subtitle != "" {
		elements = append(elements, LayoutElement{
			Kind:       "text",
			Name:       "subtitle",
			X:          leftX,
			Y:          boardY + 278,
			Width:      leftW,
			Height:     110,
			ZIndex:     2,
			Text:       subtitle,
			FontRole:   "subtitle",
			Color:      "rgba(226,232,240,0.84)",
			MaxLines:   3,
			LineHeight: 1.34,
		})
		bodyTop = boardY + 394
	}
	if theme := cleanSpace(brief.Theme); theme != "" && !strings.EqualFold(theme, eyebrow) {
		elements = append(elements, LayoutElement{
			Kind:        "shape",
			Name:        "theme-chip-bg",
			X:           leftX,
			Y:           bodyTop,
			Width:       maxInt(180, minInt(leftW, 120+len([]rune(theme))*12)),
			Height:      38,
			ZIndex:      2,
			Radius:      18,
			Fill:        "rgba(125,211,252,0.16)",
			Stroke:      "rgba(125,211,252,0.22)",
			StrokeWidth: 1,
		}, LayoutElement{
			Kind:     "text",
			Name:     "theme-chip",
			X:        leftX + 16,
			Y:        bodyTop + 8,
			Width:    maxInt(148, leftW-24),
			Height:   24,
			ZIndex:   3,
			Text:     theme,
			FontRole: "chip",
			Color:    "#DBEAFE",
			MaxLines: 1,
		})
		bodyTop += 56
	}
	if len(brief.Bullets) > 0 {
		return append(elements, buildCardElements(brief.Bullets, rightX, boardY+34, rightW, boardH-68, 2)...)
	}
	note := firstNonEmpty(cleanSpace(brief.VisualQuery), subtitle, "Structured presentation-ready summary.")
	return append(elements, LayoutElement{
		Kind:       "text",
		Name:       "note",
		X:          rightX + 12,
		Y:          maxInt(bodyTop, boardY+112),
		Width:      maxInt(220, rightW-24),
		Height:     boardH - maxInt(bodyTop, boardY+112) - 16,
		ZIndex:     2,
		Text:       note,
		FontRole:   "body",
		Color:      "rgba(219,234,254,0.90)",
		MaxLines:   6,
		LineHeight: 1.34,
	})
}

func buildAgendaLayoutElements(brief Brief, width, height int) []LayoutElement {
	items := normalizeBullets(brief.Bullets, 4)
	if len(items) == 0 {
		return buildTextOnlyLayoutElements(brief, width, height)
	}
	title := firstNonEmpty(cleanSpace(brief.Title), defaultLayoutTitle(brief, TemplateAgenda))
	subtitle := cleanSpace(brief.Subtitle)
	eyebrow := layoutEyebrowText(brief, TemplateAgenda)
	boardX := 56
	boardY := 52
	boardW := width - 112
	boardH := height - 104
	innerX := boardX + 40
	innerW := boardW - 80
	headerBottom := boardY + 232
	elements := []LayoutElement{
		{
			Kind:        "shape",
			Name:        "board",
			X:           boardX,
			Y:           boardY,
			Width:       boardW,
			Height:      boardH,
			ZIndex:      1,
			Radius:      30,
			Fill:        "rgba(255,255,255,0.08)",
			Stroke:      "rgba(255,255,255,0.10)",
			StrokeWidth: 1,
		},
		{
			Kind:       "text",
			Name:       "eyebrow",
			X:          innerX,
			Y:          boardY + 40,
			Width:      360,
			Height:     34,
			ZIndex:     2,
			Text:       eyebrow,
			FontRole:   "eyebrow",
			FontWeight: "semibold",
			Color:      "rgba(226,232,240,0.88)",
			MaxLines:   1,
		},
		{
			Kind:       "text",
			Name:       "title",
			X:          innerX,
			Y:          boardY + 92,
			Width:      innerW,
			Height:     112,
			ZIndex:     2,
			Text:       title,
			FontRole:   "title",
			FontWeight: "bold",
			Color:      "#F8FAFC",
			MaxLines:   2,
		},
	}
	if subtitle != "" {
		elements = append(elements, LayoutElement{
			Kind:       "text",
			Name:       "subtitle",
			X:          innerX,
			Y:          boardY + 184,
			Width:      innerW,
			Height:     52,
			ZIndex:     2,
			Text:       subtitle,
			FontRole:   "subtitle",
			Color:      "rgba(226,232,240,0.84)",
			MaxLines:   2,
			LineHeight: 1.30,
		})
		headerBottom = boardY + 260
	}
	elements = append(elements, LayoutElement{
		Kind:        "chart",
		Name:        "agenda-divider",
		X:           innerX,
		Y:           headerBottom - 18,
		Width:       innerW,
		Height:      6,
		ZIndex:      2,
		ChartType:   "divider",
		StrokeWidth: 2,
		Accent:      "rgba(125,211,252,0.44)",
	})

	gap := 20
	cols := minInt(len(items), 3)
	rows := (len(items) + cols - 1) / cols
	cardW := (innerW - gap*(cols-1)) / cols
	cardH := (boardY + boardH - 40 - headerBottom - gap*(rows-1)) / rows
	cardH = maxInt(cardH, 146)
	for idx, item := range items {
		row := idx / cols
		col := idx % cols
		cardX := innerX + col*(cardW+gap)
		cardY := headerBottom + row*(cardH+gap)
		elements = append(elements,
			LayoutElement{
				Kind:        "shape",
				Name:        "step-card",
				X:           cardX,
				Y:           cardY,
				Width:       cardW,
				Height:      cardH,
				ZIndex:      2,
				Radius:      24,
				Fill:        "rgba(255,255,255,0.08)",
				Stroke:      "rgba(255,255,255,0.12)",
				StrokeWidth: 1,
			},
			LayoutElement{
				Kind:        "shape",
				Name:        "step-pill-bg",
				X:           cardX + 18,
				Y:           cardY + 18,
				Width:       74,
				Height:      30,
				ZIndex:      3,
				Radius:      15,
				Fill:        "rgba(125,211,252,0.16)",
				Stroke:      "rgba(125,211,252,0.22)",
				StrokeWidth: 1,
			},
			LayoutElement{
				Kind:       "text",
				Name:       "step-index",
				X:          cardX + 34,
				Y:          cardY + 25,
				Width:      42,
				Height:     16,
				ZIndex:     4,
				Text:       cardOrdinal(idx + 1),
				FontFamily: "display",
				FontRole:   "chip",
				Color:      "#DBEAFE",
				MaxLines:   1,
			},
			LayoutElement{
				Kind:       "text",
				Name:       "step-body",
				X:          cardX + 18,
				Y:          cardY + 64,
				Width:      maxInt(160, cardW-36),
				Height:     maxInt(72, cardH-82),
				ZIndex:     3,
				Text:       item,
				FontRole:   "body",
				FontSize:   24,
				Color:      "#F8FAFC",
				MaxLines:   3,
				LineHeight: 1.26,
			},
		)
	}
	return elements
}

func buildMetricsLayoutElements(brief Brief, width, height int) []LayoutElement {
	items := normalizeBullets(brief.Bullets, 3)
	if len(items) == 0 {
		return buildTextOnlyLayoutElements(brief, width, height)
	}
	title := firstNonEmpty(cleanSpace(brief.Title), defaultLayoutTitle(brief, TemplateMetrics))
	subtitle := cleanSpace(brief.Subtitle)
	eyebrow := layoutEyebrowText(brief, TemplateMetrics)
	boardX := 56
	boardY := 52
	boardW := width - 112
	boardH := height - 104
	innerX := boardX + 40
	innerW := boardW - 80
	cardsY := boardY + 240
	elements := []LayoutElement{
		{
			Kind:        "shape",
			Name:        "board",
			X:           boardX,
			Y:           boardY,
			Width:       boardW,
			Height:      boardH,
			ZIndex:      1,
			Radius:      30,
			Fill:        "rgba(255,255,255,0.08)",
			Stroke:      "rgba(255,255,255,0.10)",
			StrokeWidth: 1,
		},
		{
			Kind:       "text",
			Name:       "eyebrow",
			X:          innerX,
			Y:          boardY + 40,
			Width:      360,
			Height:     34,
			ZIndex:     2,
			Text:       eyebrow,
			FontRole:   "eyebrow",
			FontWeight: "semibold",
			Color:      "rgba(226,232,240,0.88)",
			MaxLines:   1,
		},
		{
			Kind:       "text",
			Name:       "title",
			X:          innerX,
			Y:          boardY + 92,
			Width:      innerW,
			Height:     112,
			ZIndex:     2,
			Text:       title,
			FontRole:   "title",
			FontWeight: "bold",
			Color:      "#F8FAFC",
			MaxLines:   2,
		},
	}
	if subtitle != "" {
		elements = append(elements, LayoutElement{
			Kind:       "text",
			Name:       "subtitle",
			X:          innerX,
			Y:          boardY + 186,
			Width:      innerW,
			Height:     48,
			ZIndex:     2,
			Text:       subtitle,
			FontRole:   "subtitle",
			Color:      "rgba(226,232,240,0.84)",
			MaxLines:   2,
			LineHeight: 1.28,
		})
		cardsY = boardY + 262
	}
	elements = append(elements, LayoutElement{
		Kind:        "chart",
		Name:        "metrics-divider",
		X:           innerX,
		Y:           cardsY - 18,
		Width:       innerW,
		Height:      6,
		ZIndex:      2,
		ChartType:   "divider",
		StrokeWidth: 2,
		Accent:      "rgba(125,211,252,0.44)",
	})

	gap := 18
	count := len(items)
	cardW := (innerW - gap*(count-1)) / count
	cardH := boardY + boardH - 40 - cardsY
	cardH = maxInt(cardH, 220)
	for idx, item := range items {
		cardX := innerX + idx*(cardW+gap)
		value, label := splitMetricBullet(item, idx+1)
		chartTop := cardsY + cardH - 54
		labelY := cardsY + 156
		labelH := maxInt(28, chartTop-labelY-10)
		elements = append(elements,
			LayoutElement{
				Kind:        "shape",
				Name:        "metric-card",
				X:           cardX,
				Y:           cardsY,
				Width:       cardW,
				Height:      cardH,
				ZIndex:      2,
				Radius:      24,
				Fill:        "rgba(255,255,255,0.08)",
				Stroke:      "rgba(255,255,255,0.12)",
				StrokeWidth: 1,
			},
			LayoutElement{
				Kind:        "shape",
				Name:        "metric-chip-bg",
				X:           cardX + 18,
				Y:           cardsY + 18,
				Width:       92,
				Height:      30,
				ZIndex:      3,
				Radius:      15,
				Fill:        "rgba(125,211,252,0.16)",
				Stroke:      "rgba(125,211,252,0.22)",
				StrokeWidth: 1,
			},
			LayoutElement{
				Kind:       "text",
				Name:       "metric-chip",
				X:          cardX + 32,
				Y:          cardsY + 25,
				Width:      68,
				Height:     16,
				ZIndex:     4,
				Text:       "Metric " + cardOrdinal(idx+1),
				FontFamily: "display",
				FontRole:   "chip",
				Color:      "#DBEAFE",
				MaxLines:   1,
			},
			LayoutElement{
				Kind:       "text",
				Name:       "metric-value",
				X:          cardX + 18,
				Y:          cardsY + 72,
				Width:      maxInt(150, cardW-36),
				Height:     76,
				ZIndex:     3,
				Text:       value,
				FontFamily: "mono",
				FontRole:   "title",
				FontSize:   56,
				FontWeight: "bold",
				Color:      "#F8FAFC",
				MaxLines:   1,
			},
			LayoutElement{
				Kind:       "text",
				Name:       "metric-label",
				X:          cardX + 18,
				Y:          labelY,
				Width:      maxInt(150, cardW-36),
				Height:     labelH,
				ZIndex:     3,
				Text:       label,
				FontRole:   "body",
				FontSize:   22,
				Color:      "rgba(226,232,240,0.92)",
				MaxLines:   3,
				LineHeight: 1.24,
			},
			LayoutElement{
				Kind:        "chart",
				Name:        "metric-trend",
				X:           cardX + 18,
				Y:           chartTop,
				Width:       maxInt(150, cardW-36),
				Height:      30,
				ZIndex:      3,
				Radius:      10,
				ChartType:   "sparkline",
				Values:      buildMetricTrendValues(value, label, idx),
				ValueFormat: inferMetricValueFormat(value),
				Fill:        "rgba(255,255,255,0.05)",
				StrokeWidth: 2,
				Accent:      "rgba(125,211,252,0.92)",
			},
		)
	}
	return elements
}

func buildCardElements(bullets []string, x, y, width, height, preferredCols int) []LayoutElement {
	if len(bullets) == 0 || width <= 0 || height <= 0 {
		return nil
	}
	items := normalizeBullets(bullets, 4)
	if len(items) == 0 {
		return nil
	}
	cols := preferredCols
	if cols <= 0 {
		cols = 1
	}
	if cols > len(items) {
		cols = len(items)
	}
	rows := (len(items) + cols - 1) / cols
	gap := 18
	cardW := (width - gap*(cols-1)) / cols
	cardH := (height - gap*(rows-1)) / rows
	cardH = maxInt(cardH, 118)

	elements := make([]LayoutElement, 0, len(items)*3)
	for idx, bullet := range items {
		row := idx / cols
		col := idx % cols
		cardX := x + col*(cardW+gap)
		cardY := y + row*(cardH+gap)
		elements = append(elements, LayoutElement{
			Kind:        "shape",
			Name:        "card",
			X:           cardX,
			Y:           cardY,
			Width:       cardW,
			Height:      cardH,
			ZIndex:      2,
			Radius:      20,
			Fill:        "rgba(255,255,255,0.08)",
			Stroke:      "rgba(255,255,255,0.10)",
			StrokeWidth: 1,
		}, LayoutElement{
			Kind:   "shape",
			Name:   "card-accent",
			X:      cardX,
			Y:      cardY,
			Width:  cardW,
			Height: 6,
			ZIndex: 3,
			Fill:   "rgba(125,211,252,0.92)",
		}, LayoutElement{
			Kind:     "text",
			Name:     "card-index",
			X:        cardX + 16,
			Y:        cardY + 18,
			Width:    40,
			Height:   18,
			ZIndex:   3,
			Text:     cardOrdinal(idx + 1),
			FontRole: "eyebrow",
			Color:    "rgba(219,234,254,0.88)",
			MaxLines: 1,
		}, LayoutElement{
			Kind:       "text",
			Name:       "card-body",
			X:          cardX + 16,
			Y:          cardY + 42,
			Width:      maxInt(160, cardW-32),
			Height:     maxInt(60, cardH-58),
			ZIndex:     3,
			Text:       bullet,
			FontRole:   "body",
			Color:      "#F8FAFC",
			MaxLines:   3,
			LineHeight: 1.30,
		})
	}
	return elements
}

func splitMetricBullet(bullet string, ordinal int) (string, string) {
	ensureMetricValueTokenRE()
	cleaned := cleanBulletLine(bullet)
	if cleaned == "" {
		return cardOrdinal(ordinal), defaultMetricLabel(ordinal, false)
	}
	if idx := strings.IndexAny(cleaned, ":："); idx > 0 && idx < len(cleaned)-1 {
		left := cleanLine(cleaned[:idx])
		right := cleanLine(cleaned[idx+1:])
		if metricValueTokenRE.MatchString(right) {
			return right, firstNonEmpty(left, defaultMetricLabel(ordinal, containsCJK(cleaned)))
		}
		if metricValueTokenRE.MatchString(left) {
			return left, firstNonEmpty(right, defaultMetricLabel(ordinal, containsCJK(cleaned)))
		}
	}
	if bounds := metricValueTokenRE.FindStringIndex(cleaned); len(bounds) == 2 {
		value := cleanLine(cleaned[bounds[0]:bounds[1]])
		label := cleanLine(strings.TrimSpace(cleaned[:bounds[0]] + " " + cleaned[bounds[1]:]))
		if label == "" {
			label = defaultMetricLabel(ordinal, containsCJK(cleaned))
		}
		return value, label
	}
	return cardOrdinal(ordinal), cleaned
}

func defaultMetricLabel(ordinal int, cjk bool) string {
	if cjk {
		return fmt.Sprintf("关键指标 %s", cardOrdinal(ordinal))
	}
	return fmt.Sprintf("Key Metric %s", cardOrdinal(ordinal))
}

func buildMetricTrendValues(valueText, label string, ordinal int) []float64 {
	base, negative := parseMetricMagnitude(valueText)
	if base <= 0 {
		base = float64(48 + ordinal*8 + len([]rune(label))%9)
	}
	if negative {
		return []float64{
			base * 1.28,
			base * 1.16,
			base * 1.07,
			base * 0.94,
			base * 0.84,
		}
	}
	return []float64{
		base * 0.58,
		base * 0.67,
		base * 0.76,
		base * 0.88,
		base,
	}
}

func parseMetricMagnitude(valueText string) (float64, bool) {
	ensureMetricValueTokenRE()
	text := cleanLine(valueText)
	if text == "" {
		return 0, false
	}
	match := metricValueTokenRE.FindString(text)
	if match == "" {
		return 0, false
	}
	numeric := strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == ',', r == '-':
			return r
		default:
			return -1
		}
	}, match)
	numeric = strings.ReplaceAll(numeric, ",", "")
	if numeric == "" || numeric == "-" {
		return 0, strings.Contains(match, "-")
	}
	value, err := strconv.ParseFloat(numeric, 64)
	if err != nil {
		return 0, strings.Contains(match, "-")
	}
	return math.Abs(value), value < 0 || strings.Contains(match, "-")
}

func inferMetricValueFormat(valueText string) string {
	text := strings.ToLower(cleanLine(valueText))
	switch {
	case strings.Contains(text, "%"):
		return "percent"
	case strings.ContainsAny(text, "$€£¥"):
		return "currency"
	case strings.Contains(text, "x"):
		return "multiplier"
	case strings.Contains(text, "."):
		return "decimal1"
	default:
		return "number"
	}
}

func cardOrdinal(value int) string {
	switch {
	case value <= 0:
		return "00"
	default:
		return fmt.Sprintf("%02d", value)
	}
}

func layoutEyebrowText(brief Brief, templateID string) string {
	if theme := cleanSpace(brief.Theme); theme != "" {
		return theme
	}
	cjk := containsCJK(strings.Join([]string{brief.Title, brief.Subtitle, brief.VisualQuery}, " "))
	switch templateID {
	case TemplateTextOnly:
		if cjk {
			if IsNanoSlidesPreset(brief.StylePreset) {
				return "战略摘要"
			}
			return "执行摘要"
		}
		if IsNanoSlidesPreset(brief.StylePreset) {
			return "Strategy Summary"
		}
		return "Executive Summary"
	case TemplateAgenda:
		if cjk {
			return "议程结构"
		}
		return "Agenda Structure"
	case TemplateMetrics:
		if cjk {
			return "核心指标"
		}
		return "Key Metrics"
	case TemplateSplit:
		if cjk {
			if IsNanoSlidesPreset(brief.StylePreset) {
				return "关键洞察"
			}
			return "核心要点"
		}
		if IsNanoSlidesPreset(brief.StylePreset) {
			return "Key Insights"
		}
		return "Key Highlights"
	default:
		if cjk {
			if IsNanoSlidesPreset(brief.StylePreset) {
				return "战略概览"
			}
			return "演示概览"
		}
		if IsNanoSlidesPreset(brief.StylePreset) {
			return "Strategy Overview"
		}
		return "Presentation Overview"
	}
}

func defaultLayoutTitle(brief Brief, templateID string) string {
	if IsNanoSlidesPreset(brief.StylePreset) {
		switch templateID {
		case TemplateAgenda:
			return "Strategic agenda"
		case TemplateMetrics:
			return "Performance metrics"
		case TemplateTextOnly:
			return "Strategy summary"
		case TemplateSplit:
			return "Key insights"
		default:
			return "Strategy overview"
		}
	}
	switch templateID {
	case TemplateAgenda:
		return "Agenda"
	case TemplateMetrics:
		return "Key metrics"
	case TemplateTextOnly:
		return "Executive summary"
	case TemplateSplit:
		return "Key highlights"
	default:
		return "Presentation overview"
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
