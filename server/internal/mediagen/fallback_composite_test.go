package mediagen

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"sort"
	"strings"
	"testing"
)

func newCompositeAssetURL(t *testing.T, label string) string {
	t.Helper()

	img := newCompositeAssetImage(strings.TrimSpace(strings.ToLower(label)))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	return "data:image/png;label=" + strings.TrimSpace(label) + ";base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func newCompositeAssetImage(label string) image.Image {
	switch label {
	case "beach":
		return newBeachAsset()
	case "dog":
		return newDogAsset()
	case "cat":
		return newCatAsset()
	default:
		img := image.NewRGBA(image.Rect(0, 0, 512, 512))
		fillRect(img, img.Bounds(), color.RGBA{R: 180, G: 180, B: 180, A: 255})
		return img
	}
}

func newBeachAsset() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 960, 540))
	fillRectComposite(img, image.Rect(0, 0, 960, 250), color.RGBA{R: 132, G: 208, B: 255, A: 255})
	fillRectComposite(img, image.Rect(0, 250, 960, 360), color.RGBA{R: 66, G: 178, B: 214, A: 255})
	fillRectComposite(img, image.Rect(0, 360, 960, 540), color.RGBA{R: 232, G: 204, B: 132, A: 255})
	drawFilledCircle(img, 790, 95, 42, color.RGBA{R: 255, G: 225, B: 120, A: 255})
	return img
}

func newDogAsset() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	drawFilledEllipse(img, image.Rect(150, 240, 370, 390), color.RGBA{R: 135, G: 92, B: 58, A: 255})
	drawFilledEllipse(img, image.Rect(255, 160, 390, 285), color.RGBA{R: 152, G: 104, B: 68, A: 255})
	drawFilledTriangle(img, image.Point{275, 155}, image.Point{245, 215}, image.Point{310, 205}, color.RGBA{R: 92, G: 58, B: 38, A: 255})
	drawFilledTriangle(img, image.Point{352, 165}, image.Point{330, 220}, image.Point{382, 212}, color.RGBA{R: 92, G: 58, B: 38, A: 255})
	fillRectComposite(img, image.Rect(178, 360, 212, 472), color.RGBA{R: 110, G: 74, B: 46, A: 255})
	fillRectComposite(img, image.Rect(306, 360, 340, 472), color.RGBA{R: 110, G: 74, B: 46, A: 255})
	drawFilledTriangle(img, image.Point{150, 270}, image.Point{92, 238}, image.Point{125, 308}, color.RGBA{R: 110, G: 74, B: 46, A: 255})
	return img
}

func newCatAsset() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	drawFilledEllipse(img, image.Rect(168, 246, 344, 404), color.RGBA{R: 232, G: 140, B: 66, A: 255})
	drawFilledEllipse(img, image.Rect(168, 138, 346, 296), color.RGBA{R: 245, G: 152, B: 74, A: 255})
	drawFilledTriangle(img, image.Point{204, 148}, image.Point{162, 214}, image.Point{228, 210}, color.RGBA{R: 225, G: 118, B: 54, A: 255})
	drawFilledTriangle(img, image.Point{308, 146}, image.Point{282, 210}, image.Point{348, 206}, color.RGBA{R: 225, G: 118, B: 54, A: 255})
	fillRectComposite(img, image.Rect(204, 378, 230, 474), color.RGBA{R: 208, G: 124, B: 58, A: 255})
	fillRectComposite(img, image.Rect(286, 378, 312, 474), color.RGBA{R: 208, G: 124, B: 58, A: 255})
	drawFilledTriangle(img, image.Point{340, 320}, image.Point{420, 250}, image.Point{386, 360}, color.RGBA{R: 208, G: 124, B: 58, A: 255})
	return img
}

func fillRectComposite(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	rect = rect.Intersect(img.Bounds())
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawFilledCircle(img *image.RGBA, cx, cy, radius int, c color.RGBA) {
	r2 := radius * radius
	bounds := img.Bounds()
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			if !image.Pt(x, y).In(bounds) {
				continue
			}
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawFilledEllipse(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	rect = rect.Intersect(img.Bounds())
	rx := float64(rect.Dx()) / 2
	ry := float64(rect.Dy()) / 2
	if rx <= 0 || ry <= 0 {
		return
	}
	cx := float64(rect.Min.X) + rx
	cy := float64(rect.Min.Y) + ry
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := (float64(x) - cx) / rx
			dy := (float64(y) - cy) / ry
			if dx*dx+dy*dy <= 1 {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawFilledTriangle(img *image.RGBA, a, b, cpt image.Point, c color.RGBA) {
	minX := minIntComposite(a.X, b.X, cpt.X)
	maxX := maxIntComposite(a.X, b.X, cpt.X)
	minY := minIntComposite(a.Y, b.Y, cpt.Y)
	maxY := maxIntComposite(a.Y, b.Y, cpt.Y)
	bounds := img.Bounds()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if !image.Pt(x, y).In(bounds) {
				continue
			}
			p := image.Point{X: x, Y: y}
			if pointInTriangle(p, a, b, cpt) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func pointInTriangle(p, a, b, c image.Point) bool {
	sign := func(p1, p2, p3 image.Point) int {
		return (p1.X-p3.X)*(p2.Y-p3.Y) - (p2.X-p3.X)*(p1.Y-p3.Y)
	}
	d1 := sign(p, a, b)
	d2 := sign(p, b, c)
	d3 := sign(p, c, a)
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

func minIntComposite(values ...int) int {
	best := values[0]
	for _, value := range values[1:] {
		if value < best {
			best = value
		}
	}
	return best
}

func maxIntComposite(values ...int) int {
	best := values[0]
	for _, value := range values[1:] {
		if value > best {
			best = value
		}
	}
	return best
}

func newCompositeFallbackSearcher(bgURL, dogURL, catURL string) stubFallbackSearcher {
	return stubFallbackSearcher{
		search: func(_ context.Context, query string, _ int, _ []string) ([]FallbackSearchResult, error) {
			normalized := strings.TrimSpace(strings.ToLower(query))
			switch {
			case strings.Contains(normalized, "beach") && strings.Contains(normalized, "background"):
				return []FallbackSearchResult{{Title: "Beach", URL: bgURL}}, nil
			case strings.Contains(normalized, "dog") && strings.Contains(normalized, "transparent"):
				return []FallbackSearchResult{{Title: "Dog", URL: dogURL}}, nil
			case strings.Contains(normalized, "cat") && strings.Contains(normalized, "transparent"):
				return []FallbackSearchResult{{Title: "Cat", URL: catURL}}, nil
			default:
				return nil, nil
			}
		},
	}
}

func TestRenderSceneComposeCompositePromptUsesMultipleForegroundAssets(t *testing.T) {
	bgURL := newCompositeAssetURL(t, "beach")
	dogURL := newCompositeAssetURL(t, "dog")
	catURL := newCompositeAssetURL(t, "cat")

	searcher := newCompositeFallbackSearcher(bgURL, dogURL, catURL)
	engine := NewFallbackEngine(FallbackConfig{Enabled: true}, nil, searcher, func() FallbackBrowserService { return nil }, "en-US")
	engine.sourceClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			_ = req
			return nil, errors.New("source search disabled in composite tests")
		}),
	}

	data, sourceURLs, err := engine.renderSceneCompose(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: "dog on right, cat on left in beach",
	})
	if err != nil {
		t.Fatalf("renderSceneCompose returned error: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodeConfig returned error: %v", err)
	}
	if cfg.Width != engine.config.ScreenshotWidth || cfg.Height != engine.config.ScreenshotHeight {
		t.Fatalf("output bounds = %dx%d, want %dx%d", cfg.Width, cfg.Height, engine.config.ScreenshotWidth, engine.config.ScreenshotHeight)
	}

	assertSameStringSet(t, sourceURLs, []string{
		bgURL,
		dogURL,
		catURL,
	})
}

func TestBuildNativeVideoPlanCompositePromptCreatesMultipleForegroundLayers(t *testing.T) {
	bgURL := newCompositeAssetURL(t, "beach")
	dogURL := newCompositeAssetURL(t, "dog")
	catURL := newCompositeAssetURL(t, "cat")

	searcher := newCompositeFallbackSearcher(bgURL, dogURL, catURL)
	engine := NewFallbackEngine(FallbackConfig{
		Enabled: true,
		NativeVideo: FallbackNativeVideoConfig{
			Enabled:            true,
			DefaultDurationSec: 2,
			MaxDurationSec:     2,
			AudioMode:          "silent",
		},
	}, nil, searcher, func() FallbackBrowserService { return nil }, "en-US")
	engine.sourceClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			_ = req
			return nil, errors.New("source search disabled in composite tests")
		}),
	}

	plan, sourceURLs, err := engine.buildNativeVideoPlan(context.Background(), &MediaRequest{
		Type:     MediaTypeVideo,
		Prompt:   "dog on right, cat on left in beach",
		Duration: 2,
	}, CategoryT2V, t.TempDir())
	if err != nil {
		t.Fatalf("buildNativeVideoPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil video plan")
	}
	if len(plan.Layers) != 3 {
		t.Fatalf("layer count = %d, want 3", len(plan.Layers))
	}

	var (
		backgroundFound bool
		dogLayerIndex   = -1
		catLayerIndex   = -1
	)
	for idx, layer := range plan.Layers {
		switch {
		case layer.ZIndex == 0 && layer.ContentMode == "fill":
			backgroundFound = true
		case strings.HasPrefix(layer.ID, "dog"):
			dogLayerIndex = idx
		case strings.HasPrefix(layer.ID, "cat"):
			catLayerIndex = idx
		}
	}
	if !backgroundFound {
		t.Fatalf("plan.Layers = %#v, want background fill layer", plan.Layers)
	}
	if dogLayerIndex < 0 || catLayerIndex < 0 {
		t.Fatalf("plan.Layers = %#v, want dog + cat foreground layers", plan.Layers)
	}
	if plan.Layers[dogLayerIndex].FrameStart.X <= plan.Layers[catLayerIndex].FrameStart.X {
		t.Fatalf("dog X = %.2f, cat X = %.2f, want dog on the right of cat", plan.Layers[dogLayerIndex].FrameStart.X, plan.Layers[catLayerIndex].FrameStart.X)
	}

	assertSameStringSet(t, sourceURLs, []string{
		bgURL,
		dogURL,
		catURL,
	})
}

func assertSameStringSet(t *testing.T, got, want []string) {
	t.Helper()

	gotCopy := append([]string(nil), got...)
	wantCopy := append([]string(nil), want...)
	sort.Strings(gotCopy)
	sort.Strings(wantCopy)
	if strings.Join(gotCopy, "\n") != strings.Join(wantCopy, "\n") {
		t.Fatalf("string set mismatch\ngot:\n%s\nwant:\n%s", strings.Join(gotCopy, "\n"), strings.Join(wantCopy, "\n"))
	}
}
