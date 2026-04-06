package mediagen

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/slidespec"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/basicfont"
)

func TestSlideBriefFromMediaRequestPrefersStructuredPPTExtras(t *testing.T) {
	req := &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "Create a presentation-ready PPT slide visual for this intent:\nWrapped prompt that should not become the title",
		Extra: map[string]any{
			"style_preset":     "banana_slides",
			"quality_profile":  "ppt",
			"ppt_description":  "帮我做一页PPT，标题：2026 产品战略；副标题：AI 驱动增长；要点：提升转化率；降低成本；全球化扩张",
			"ppt_title":        "2026 产品战略",
			"ppt_subtitle":     "AI 驱动增长",
			"ppt_bullets":      []string{"提升转化率", "降低成本", "全球化扩张"},
			"ppt_visual_query": "2026 产品战略, AI 驱动增长, 提升转化率",
			"render_mode":      "slide",
			"template_id":      "split",
		},
	}

	brief := slideBriefFromMediaRequest(req)
	if brief.RenderMode != slidespec.RenderModeSlide {
		t.Fatalf("render_mode = %q, want %q", brief.RenderMode, slidespec.RenderModeSlide)
	}
	if brief.TemplateID != slidespec.TemplateSplit {
		t.Fatalf("template_id = %q, want %q", brief.TemplateID, slidespec.TemplateSplit)
	}
	if brief.StylePreset != slidespec.StylePresetBananaSlides {
		t.Fatalf("style_preset = %q, want %q", brief.StylePreset, slidespec.StylePresetBananaSlides)
	}
	if brief.Title != "2026 产品战略" {
		t.Fatalf("title = %q", brief.Title)
	}
	if brief.Subtitle != "AI 驱动增长" {
		t.Fatalf("subtitle = %q", brief.Subtitle)
	}
	if len(brief.Bullets) != 3 {
		t.Fatalf("bullets = %#v, want 3", brief.Bullets)
	}
	if strings.Contains(brief.Title, "Create a presentation-ready") {
		t.Fatalf("title unexpectedly derived from wrapped prompt: %q", brief.Title)
	}
}

func TestSlideBriefFromMediaRequestKeepsPhotoPromptAsPoster(t *testing.T) {
	req := &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
	}

	brief := slideBriefFromMediaRequest(req)
	if brief.RenderMode != slidespec.RenderModePoster {
		t.Fatalf("render_mode = %q, want %q", brief.RenderMode, slidespec.RenderModePoster)
	}
	if brief.TemplateID != slidespec.TemplatePoster {
		t.Fatalf("template_id = %q, want %q", brief.TemplateID, slidespec.TemplatePoster)
	}
	if strings.TrimSpace(brief.VisualQuery) != req.Prompt {
		t.Fatalf("visual_query = %q, want prompt passthrough", brief.VisualQuery)
	}
}

func TestSlideBriefFromMediaRequestPromotesExplicitLayoutSpecToSlide(t *testing.T) {
	req := &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "Quarterly growth overview",
		Extra: map[string]any{
			"layout_spec": map[string]any{
				"template_id": "text_only",
				"elements": []map[string]any{
					{
						"kind":      "text",
						"text":      "Revenue growth",
						"font_role": "title",
						"x":         96,
						"y":         120,
						"width":     420,
						"height":    96,
					},
				},
			},
		},
	}

	brief := slideBriefFromMediaRequest(req)
	if brief.RenderMode != slidespec.RenderModeSlide {
		t.Fatalf("render_mode = %q, want %q", brief.RenderMode, slidespec.RenderModeSlide)
	}
	if brief.TemplateID != slidespec.TemplateTextOnly {
		t.Fatalf("template_id = %q, want %q", brief.TemplateID, slidespec.TemplateTextOnly)
	}
}

func TestBuildSlideHTMLUsesPresentationEyebrowInsteadOfFallbackLabel(t *testing.T) {
	engine := &FallbackEngine{}
	htmlDoc, err := engine.buildSlideHTML(slidespec.Brief{
		RenderMode:  slidespec.RenderModeSlide,
		StylePreset: slidespec.StylePresetNanoSlides,
		Title:       "Revenue growth",
		Subtitle:    "Enterprise expansion",
		Bullets:     []string{"Pipeline quality", "Retention lift", "Regional rollout"},
	}, slidespec.TemplateTextOnly, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("buildSlideHTML returned error: %v", err)
	}
	if strings.Contains(htmlDoc, "Slide Fallback") {
		t.Fatalf("html should not expose fallback label: %q", htmlDoc)
	}
	if !strings.Contains(htmlDoc, "Strategy Summary") {
		t.Fatalf("html missing nano-slides style eyebrow: %q", htmlDoc)
	}
	if !strings.Contains(htmlDoc, "style-nano") {
		t.Fatalf("html missing nano style class: %q", htmlDoc)
	}
}

func TestPendingInfoForRequestPreservesSlideStylePreset(t *testing.T) {
	engine := NewFallbackEngine(FallbackConfig{Enabled: true}, nil, nil, nil, "")
	info := engine.PendingInfoForRequest(&MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "Create a strategy summary slide",
		Extra: map[string]any{
			"style_preset": "nanoslides",
			"render_mode":  "slide",
			"template_id":  "text_only",
		},
	}, FallbackModelWebCanvasT2I)
	if info == nil {
		t.Fatal("expected fallback info")
	}
	if info.StylePreset != slidespec.StylePresetNanoSlides {
		t.Fatalf("style_preset = %q, want %q", info.StylePreset, slidespec.StylePresetNanoSlides)
	}
}

func TestRenderSlidePNGSmokeForNanoTextOnly(t *testing.T) {
	engine := &FallbackEngine{}
	payload, err := engine.renderSlidePNG(slidespec.Brief{
		RenderMode:  slidespec.RenderModeSlide,
		StylePreset: slidespec.StylePresetNanoSlides,
		Title:       "Revenue growth",
		Subtitle:    "Enterprise expansion plan",
		Bullets:     []string{"Pipeline quality", "Retention lift", "Regional rollout"},
	}, slidespec.TemplateTextOnly, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("renderSlidePNG returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png config: %v", err)
	}
	if cfg.Width != 1280 || cfg.Height != 720 {
		t.Fatalf("png bounds = %dx%d, want 1280x720", cfg.Width, cfg.Height)
	}
}

func TestRenderSlidePNGWithLayoutUsesExplicitBackgroundFill(t *testing.T) {
	payload, err := renderSlidePNGWithLayout(slidespec.LayoutSpec{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
		Canvas: slidespec.LayoutCanvas{
			Width:  1280,
			Height: 720,
		},
		Background: slidespec.LayoutBackground{
			Fill: "#112233",
		},
		Elements: []slidespec.LayoutElement{
			{
				Kind:     "text",
				Text:     "Revenue growth",
				FontRole: "title",
				Color:    "#ffffff",
				X:        96,
				Y:        120,
				Width:    520,
				Height:   120,
				MaxLines: 2,
				ZIndex:   1,
			},
		},
	}, slidespec.Brief{
		RenderMode:  slidespec.RenderModeSlide,
		TemplateID:  slidespec.TemplateTextOnly,
		StylePreset: slidespec.StylePresetNanoSlides,
		Title:       "Revenue growth",
	}, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("renderSlidePNGWithLayout returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	r, g, b, _ := img.At(8, 8).RGBA()
	if uint8(r>>8) != 0x11 || uint8(g>>8) != 0x22 || uint8(b>>8) != 0x33 {
		t.Fatalf("background pixel = #%02x%02x%02x, want #112233", uint8(r>>8), uint8(g>>8), uint8(b>>8))
	}
}

func TestRenderSlidePNGWithLayoutHonorsShapeRadius(t *testing.T) {
	payload, err := renderSlidePNGWithLayout(slidespec.LayoutSpec{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
		Canvas: slidespec.LayoutCanvas{
			Width:  1280,
			Height: 720,
		},
		Background: slidespec.LayoutBackground{
			Fill: "#112233",
		},
		Elements: []slidespec.LayoutElement{
			{
				Kind:   "shape",
				X:      100,
				Y:      100,
				Width:  120,
				Height: 120,
				Radius: 36,
				Fill:   "#ffeecc",
			},
		},
	}, slidespec.Brief{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
	}, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("renderSlidePNGWithLayout returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	cornerR, cornerG, cornerB, _ := img.At(104, 104).RGBA()
	if uint8(cornerR>>8) != 0x11 || uint8(cornerG>>8) != 0x22 || uint8(cornerB>>8) != 0x33 {
		t.Fatalf("rounded corner pixel = #%02x%02x%02x, want background #112233", uint8(cornerR>>8), uint8(cornerG>>8), uint8(cornerB>>8))
	}
	centerR, centerG, centerB, _ := img.At(160, 160).RGBA()
	if uint8(centerR>>8) != 0xff || uint8(centerG>>8) != 0xee || uint8(centerB>>8) != 0xcc {
		t.Fatalf("center pixel = #%02x%02x%02x, want shape fill #ffeecc", uint8(centerR>>8), uint8(centerG>>8), uint8(centerB>>8))
	}
}

func TestDrawSlideWrappedAlignedTextRespectsLineHeightMultiplier(t *testing.T) {
	defaultImg := image.NewRGBA(image.Rect(0, 0, 200, 200))
	expandedImg := image.NewRGBA(image.Rect(0, 0, 200, 200))
	defaultHeight := drawSlideWrappedAlignedText(
		defaultImg,
		basicfont.Face7x13,
		image.Rect(0, 0, 84, 200),
		"alpha beta gamma delta epsilon zeta",
		color.White,
		4,
		"left",
		0,
	)
	expandedHeight := drawSlideWrappedAlignedText(
		expandedImg,
		basicfont.Face7x13,
		image.Rect(0, 0, 84, 200),
		"alpha beta gamma delta epsilon zeta",
		color.White,
		4,
		"left",
		1.8,
	)
	if expandedHeight <= defaultHeight {
		t.Fatalf("expanded line height = %d, want greater than default %d", expandedHeight, defaultHeight)
	}
}

func TestLoadSlideFontVariantSupportsControlledFamilies(t *testing.T) {
	configureSlideFontRuntimeDataDir(writeSlideRuntimeTestFonts(t))
	sansBold, err := loadSlideFontVariant("sans", "bold")
	if err != nil || sansBold == nil {
		t.Fatalf("load sans bold font: font=%v err=%v", sansBold, err)
	}
	monoBold, err := loadSlideFontVariant("mono", "bold")
	if err != nil || monoBold == nil {
		t.Fatalf("load mono bold font: font=%v err=%v", monoBold, err)
	}
	displayRegular, err := loadSlideFontVariant("display", "regular")
	if err != nil || displayRegular == nil {
		t.Fatalf("load display regular font: font=%v err=%v", displayRegular, err)
	}
	if sansBold == monoBold {
		t.Fatal("expected mono font variant to differ from sans font variant")
	}
	if sansBold != displayRegular {
		t.Fatal("expected display font variant to reuse the sans bold runtime font")
	}
}

func TestCanonicalSlideFontRuntimeRequestCollapsesFamiliesAndWeights(t *testing.T) {
	cases := []struct {
		name   string
		family string
		weight string
		want   slideFontRuntimeRequest
	}{
		{name: "sans italic", family: "sans", weight: "italic", want: slideFontRuntimeRequest{family: "sans", weight: "italic"}},
		{name: "sans medium", family: "sans", weight: "medium", want: slideFontRuntimeRequest{family: "sans", weight: "bold"}},
		{name: "mono italic", family: "mono", weight: "italic", want: slideFontRuntimeRequest{family: "mono", weight: "regular"}},
		{name: "mono bold italic", family: "mono", weight: "bold_italic", want: slideFontRuntimeRequest{family: "mono", weight: "bold"}},
		{name: "display regular", family: "display", weight: "regular", want: slideFontRuntimeRequest{family: "sans", weight: "bold"}},
		{name: "display italic", family: "display", weight: "italic", want: slideFontRuntimeRequest{family: "sans", weight: "bold"}},
		{name: "display bold italic", family: "display", weight: "bold_italic", want: slideFontRuntimeRequest{family: "sans", weight: "bold"}},
		{name: "sans medium italic", family: "sans", weight: "medium_italic", want: slideFontRuntimeRequest{family: "sans", weight: "bold"}},
		{name: "sans bold italic", family: "sans", weight: "bold_italic", want: slideFontRuntimeRequest{family: "sans", weight: "bold"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalSlideFontRuntimeRequest(tc.family, tc.weight)
			if got != tc.want {
				t.Fatalf("canonicalSlideFontRuntimeRequest(%q, %q) = %#v, want %#v", tc.family, tc.weight, got, tc.want)
			}
		})
	}
}

func TestResolveSlideFontRuntimeSourcePrefersExternalFontFiles(t *testing.T) {
	dataDir := writeSlideRuntimeTestFonts(t)

	cases := []struct {
		name      string
		family    string
		weight    string
		wantSuffix string
	}{
		{name: "sans regular", family: "sans", weight: "regular", wantSuffix: filepath.Join("media", "fonts", "sans-regular.ttf")},
		{name: "sans bold via display", family: "display", weight: "regular", wantSuffix: filepath.Join("media", "fonts", "sans-bold.ttf")},
		{name: "mono bold", family: "mono", weight: "bold", wantSuffix: filepath.Join("media", "fonts", "mono-bold.ttf")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configureSlideFontRuntimeDataDir(dataDir)
			path, index, err := resolveSlideFontRuntimeSource(tc.family, tc.weight)
			if err != nil {
				t.Fatalf("resolveSlideFontRuntimeSource(%q, %q) error: %v", tc.family, tc.weight, err)
			}
			if index != 0 {
				t.Fatalf("resolveSlideFontRuntimeSource(%q, %q) index = %d, want 0", tc.family, tc.weight, index)
			}
			if !strings.HasSuffix(path, tc.wantSuffix) {
				t.Fatalf("resolveSlideFontRuntimeSource(%q, %q) path = %q, want suffix %q", tc.family, tc.weight, path, tc.wantSuffix)
			}
		})
	}
}

func TestSlideLayoutFontFaceUsesExplicitFamilyAndWeight(t *testing.T) {
	configureSlideFontRuntimeDataDir(writeSlideRuntimeTestFonts(t))
	defaultFonts := newSlideFontPack(slideCanvasSpec{width: 1280, height: 720}, slidespec.TemplateTextOnly)
	defer defaultFonts.close()

	defaultFace, owned := slideLayoutFontFace(slidespec.LayoutElement{
		Kind:     "text",
		FontRole: "body",
	}, defaultFonts, 1)
	if owned {
		t.Fatal("expected default body face to reuse cached default face")
	}
	overrideFace, owned := slideLayoutFontFace(slidespec.LayoutElement{
		Kind:       "text",
		FontRole:   "body",
		FontFamily: "mono",
		FontWeight: "bold",
	}, defaultFonts, 1)
	if !owned {
		t.Fatal("expected explicit font override to allocate a new face")
	}
	defer closeSlideFace(overrideFace)
	if measureSlideText(defaultFace, "111111") == measureSlideText(overrideFace, "111111") {
		t.Fatalf("expected mono bold face width to differ from default sans face width")
	}
}

func writeSlideRuntimeTestFonts(t *testing.T) string {
	t.Helper()

	dataDir := t.TempDir()
	fontDir := filepath.Join(dataDir, "media", "fonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		t.Fatalf("mkdir font dir: %v", err)
	}
	fonts := map[string][]byte{
		"sans-regular.ttf": goregular.TTF,
		"sans-bold.ttf":    gobold.TTF,
		"mono-regular.ttf": gomono.TTF,
		"mono-bold.ttf":    gomonobold.TTF,
	}
	for name, data := range fonts {
		if err := os.WriteFile(filepath.Join(fontDir, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		configureSlideFontRuntimeDataDir("")
	})
	return dataDir
}

func TestRenderSlidePNGWithLayoutRendersProgressChart(t *testing.T) {
	payload, err := renderSlidePNGWithLayout(slidespec.LayoutSpec{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
		Canvas: slidespec.LayoutCanvas{
			Width:  1280,
			Height: 720,
		},
		Background: slidespec.LayoutBackground{
			Fill: "#112233",
		},
		Elements: []slidespec.LayoutElement{
			{
				Kind:      "chart",
				ChartType: "progress",
				X:         100,
				Y:         100,
				Width:     120,
				Height:    16,
				Fill:      "#223344",
				Accent:    "#ffeecc",
				Value:     50,
				MaxValue:  100,
			},
		},
	}, slidespec.Brief{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
	}, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("renderSlidePNGWithLayout returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	fillR, fillG, fillB, _ := img.At(120, 108).RGBA()
	if uint8(fillR>>8) != 0xff || uint8(fillG>>8) != 0xee || uint8(fillB>>8) != 0xcc {
		t.Fatalf("progress fill pixel = #%02x%02x%02x, want #ffeecc", uint8(fillR>>8), uint8(fillG>>8), uint8(fillB>>8))
	}
	trackR, trackG, trackB, _ := img.At(190, 108).RGBA()
	if uint8(trackR>>8) != 0x22 || uint8(trackG>>8) != 0x33 || uint8(trackB>>8) != 0x44 {
		t.Fatalf("progress track pixel = #%02x%02x%02x, want #223344", uint8(trackR>>8), uint8(trackG>>8), uint8(trackB>>8))
	}
}

func TestRenderSlidePNGWithLayoutRendersVerticalProgressChart(t *testing.T) {
	payload, err := renderSlidePNGWithLayout(slidespec.LayoutSpec{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
		Canvas: slidespec.LayoutCanvas{
			Width:  1280,
			Height: 720,
		},
		Background: slidespec.LayoutBackground{
			Fill: "#112233",
		},
		Elements: []slidespec.LayoutElement{
			{
				Kind:      "chart",
				ChartType: "progress",
				Direction: "vertical",
				X:         100,
				Y:         100,
				Width:     24,
				Height:    120,
				Fill:      "#223344",
				Accent:    "#ffeecc",
				Value:     25,
				MaxValue:  100,
			},
		},
	}, slidespec.Brief{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
	}, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("renderSlidePNGWithLayout returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	fillR, fillG, fillB, _ := img.At(112, 206).RGBA()
	if uint8(fillR>>8) != 0xff || uint8(fillG>>8) != 0xee || uint8(fillB>>8) != 0xcc {
		t.Fatalf("vertical progress fill pixel = #%02x%02x%02x, want #ffeecc", uint8(fillR>>8), uint8(fillG>>8), uint8(fillB>>8))
	}
	trackR, trackG, trackB, _ := img.At(112, 126).RGBA()
	if uint8(trackR>>8) != 0x22 || uint8(trackG>>8) != 0x33 || uint8(trackB>>8) != 0x44 {
		t.Fatalf("vertical progress track pixel = #%02x%02x%02x, want #223344", uint8(trackR>>8), uint8(trackG>>8), uint8(trackB>>8))
	}
}

func TestRenderSlidePNGWithLayoutRendersSparklineChart(t *testing.T) {
	payload, err := renderSlidePNGWithLayout(slidespec.LayoutSpec{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
		Canvas: slidespec.LayoutCanvas{
			Width:  1280,
			Height: 720,
		},
		Background: slidespec.LayoutBackground{
			Fill: "#112233",
		},
		Elements: []slidespec.LayoutElement{
			{
				Kind:        "chart",
				ChartType:   "sparkline",
				X:           100,
				Y:           100,
				Width:       120,
				Height:      40,
				Fill:        "rgba(255,255,255,0.04)",
				Accent:      "#ffeecc",
				StrokeWidth: 2,
				Values:      []float64{10, 20, 15, 35, 28},
			},
		},
	}, slidespec.Brief{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
	}, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("renderSlidePNGWithLayout returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	foundAccent := false
	for y := 100; y < 140 && !foundAccent; y++ {
		for x := 100; x < 220; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == 0xff && uint8(g>>8) == 0xee && uint8(b>>8) == 0xcc {
				foundAccent = true
				break
			}
		}
	}
	if !foundAccent {
		t.Fatal("expected sparkline accent pixels in chart area")
	}
}

func TestRenderSlidePNGWithLayoutRendersHorizontalBarsWithLabels(t *testing.T) {
	payload, err := renderSlidePNGWithLayout(slidespec.LayoutSpec{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
		Canvas: slidespec.LayoutCanvas{
			Width:  1280,
			Height: 720,
		},
		Background: slidespec.LayoutBackground{
			Fill: "#112233",
		},
		Elements: []slidespec.LayoutElement{
			{
				Kind:        "chart",
				ChartType:   "bars",
				Direction:   "horizontal",
				X:           100,
				Y:           100,
				Width:       220,
				Height:      110,
				Fill:        "#223344",
				Accent:      "#ffeecc",
				Color:       "#ffffff",
				Values:      []float64{20, 40, 80},
				Labels:      []string{"North", "EMEA", "APAC"},
				ValueFormat: "percent",
			},
		},
	}, slidespec.Brief{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
	}, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("renderSlidePNGWithLayout returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	foundAccent := false
	foundLabelText := false
	foundValueText := false
	for y := 100; y < 210; y++ {
		for x := 165; x < 280; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == 0xff && uint8(g>>8) == 0xee && uint8(b>>8) == 0xcc {
				foundAccent = true
				break
			}
		}
		if foundAccent {
			break
		}
	}
	for y := 100; y < 210 && !foundLabelText; y++ {
		for x := 100; x < 160; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) > 0xf0 && uint8(g>>8) > 0xf0 && uint8(b>>8) > 0xf0 {
				foundLabelText = true
				break
			}
		}
	}
	for y := 100; y < 210 && !foundValueText; y++ {
		for x := 275; x < 320; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) > 0xf0 && uint8(g>>8) > 0xf0 && uint8(b>>8) > 0xf0 {
				foundValueText = true
				break
			}
		}
	}
	if !foundAccent {
		t.Fatal("expected horizontal bars accent pixels in chart area")
	}
	if !foundLabelText {
		t.Fatal("expected horizontal bars label text pixels in left label area")
	}
	if !foundValueText {
		t.Fatal("expected horizontal bars value text pixels in right value area")
	}
}

func TestRenderSlidePNGWithLayoutRendersSparklineInlineValueLabel(t *testing.T) {
	payload, err := renderSlidePNGWithLayout(slidespec.LayoutSpec{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
		Canvas: slidespec.LayoutCanvas{
			Width:  1280,
			Height: 720,
		},
		Background: slidespec.LayoutBackground{
			Fill: "#112233",
		},
		Elements: []slidespec.LayoutElement{
			{
				Kind:        "chart",
				ChartType:   "sparkline",
				X:           100,
				Y:           100,
				Width:       140,
				Height:      30,
				Fill:        "rgba(255,255,255,0.04)",
				Accent:      "#ffeecc",
				Color:       "#ffffff",
				StrokeWidth: 2,
				Values:      []float64{10, 20, 15, 35, 28},
				ValueFormat: "percent",
			},
		},
	}, slidespec.Brief{
		RenderMode: slidespec.RenderModeSlide,
		TemplateID: slidespec.TemplateTextOnly,
	}, "", slideCanvasSpec{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("renderSlidePNGWithLayout returned error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	foundLabelText := false
	for y := 96; y < 132 && !foundLabelText; y++ {
		for x := 188; x < 240; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) > 0xf0 && uint8(g>>8) > 0xf0 && uint8(b>>8) > 0xf0 {
				foundLabelText = true
				break
			}
		}
	}
	if !foundLabelText {
		t.Fatal("expected sparkline inline value label pixels near the endpoint")
	}
}
