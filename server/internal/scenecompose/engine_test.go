package scenecompose

import (
	"context"
	"image"
	"image/color"
	"testing"
)

type stubSearcher struct {
	results map[string][]SearchResult
}

func (s stubSearcher) Search(_ context.Context, query string, _ int) ([]SearchResult, error) {
	return append([]SearchResult(nil), s.results[query]...), nil
}

type stubResolver struct {
	images map[string]*ResolvedImage
}

func (r stubResolver) Resolve(_ context.Context, result SearchResult) (*ResolvedImage, error) {
	return r.images[result.URL], nil
}

func TestEngineComposeReturnsImageAndAssets(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("cat in forest"))
	backgroundQuery := buildBackgroundQuery(plan)
	foregroundQuery := buildForegroundQuery(plan, plan.Foreground[0], true)
	engine := NewEngine(
		NewPlanner(nil),
		NewAssetSearcher(
			stubSearcher{results: map[string][]SearchResult{
				backgroundQuery: {{Title: "Forest", URL: "bg"}},
				foregroundQuery: {{Title: "Cat PNG", URL: "fg"}},
			}},
			stubResolver{images: map[string]*ResolvedImage{
				"bg": {
					Title:     "Forest",
					SourceURL: "https://example.com/bg.png",
					Image:     solidImage(1280, 896, color.RGBA{R: 40, G: 80, B: 50, A: 255}),
					Width:     1280,
					Height:    896,
				},
				"fg": {
					Title:     "Cat",
					SourceURL: "https://example.com/cat.png",
					Image:     solidImage(512, 512, color.RGBA{R: 220, G: 140, B: 80, A: 255}),
					Width:     512,
					Height:    512,
					HasAlpha:  true,
				},
			}},
		),
		NewCutoutStrategy(nil),
		NewRenderer(),
	)
	result, err := engine.Compose(context.Background(), ComposeRequest{
		Prompt: "cat in forest",
		Width:  800,
		Height: 600,
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}
	if result == nil || result.Image == nil {
		t.Fatal("expected composed image")
	}
	if len(result.UsedAssets) < 2 {
		t.Fatalf("used assets = %#v, want background + foreground", result.UsedAssets)
	}
}

func solidImage(width, height int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}
