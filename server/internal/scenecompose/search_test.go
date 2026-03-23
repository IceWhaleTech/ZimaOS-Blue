package scenecompose

import (
	"context"
	"image/color"
	"testing"
)

func TestBackgroundScore_UsesCompiledCueMatchers(t *testing.T) {
	good := backgroundScore(
		SearchResult{Title: "Mountain landscape background wallpaper", Description: "wide scenic vista"},
		&ResolvedImage{SourceURL: "https://example.com/hero.jpg", Width: 1600, Height: 900},
	)
	bad := backgroundScore(
		SearchResult{Title: "Landscape poster template", Description: "shop logo asset"},
		&ResolvedImage{SourceURL: "https://example.com/poster.jpg", Width: 1600, Height: 900},
	)
	if good <= bad {
		t.Fatalf("expected background-friendly asset to outscore poster/template result: good=%v bad=%v", good, bad)
	}
}

func TestForegroundScore_UsesCompiledCueMatchers(t *testing.T) {
	good := foregroundScore(
		SearchResult{Title: "Robot PNG transparent isolated cutout", Description: "clean alpha subject"},
		&ResolvedImage{SourceURL: "https://example.com/robot.png", Width: 768, Height: 768, HasAlpha: true},
	)
	bad := foregroundScore(
		SearchResult{Title: "Robot vector clipart poster", Description: "logo template"},
		&ResolvedImage{SourceURL: "https://example.com/robot-vector.jpg", Width: 768, Height: 768},
	)
	if good <= bad {
		t.Fatalf("expected transparent cutout to outscore vector/template result: good=%v bad=%v", good, bad)
	}
}

func TestBuildBackgroundQuery_LocalizesChineseSceneSearch(t *testing.T) {
	query := buildBackgroundQuery(&ScenePlan{
		Background: "雪地",
		Style:      "realistic",
		Weather:    "snowy",
		TimeOfDay:  "day",
	})
	if query != "雪地 snowy 背景 照片 风景" {
		t.Fatalf("query = %q, want localized snow background query", query)
	}
}

func TestBuildBackgroundQuery_PrefersPlannerHint(t *testing.T) {
	query := buildBackgroundQuery(&ScenePlan{
		Background:      "雪地",
		BackgroundQuery: "雪地 冬季 户外 背景 照片",
	})
	if query != "雪地 冬季 户外 背景 照片" {
		t.Fatalf("query = %q, want planner-provided background query", query)
	}
}

func TestBuildForegroundQuery_PrefersPlannerHints(t *testing.T) {
	plan := &ScenePlan{Style: "realistic"}
	fg := ForegroundPlan{
		Type:          "dog",
		SearchQuery:   "泰迪犬 奔跑 isolated png transparent",
		FallbackQuery: "泰迪犬 奔跑 写实照片",
	}
	if query := buildForegroundQuery(plan, fg, true); query != "泰迪犬 奔跑 isolated png transparent" {
		t.Fatalf("foreground query = %q, want planner hint", query)
	}
	if query := buildForegroundFallbackQuery(plan, fg); query != "泰迪犬 奔跑 写实照片" {
		t.Fatalf("foreground fallback query = %q, want planner fallback hint", query)
	}
}

func TestFindBackgroundDisclosesPageURLWhenResolvedFromDirectAsset(t *testing.T) {
	plan := &ScenePlan{BackgroundQuery: "snow background"}
	searcher := NewAssetSearcher(
		stubSearcher{results: map[string][]SearchResult{
			"snow background": {{
				Title:    "Snow field",
				URL:      "https://example.com/page",
				ImageURL: "https://cdn.example.com/background.jpg",
			}},
		}},
		stubResolver{images: map[string]*ResolvedImage{
			"https://example.com/page": {
				Title:     "Snow field",
				PageURL:   "https://example.com/page",
				SourceURL: "https://cdn.example.com/background.jpg",
				Image:     solidImage(1280, 896, color.RGBA{R: 200, G: 220, B: 240, A: 255}),
				Width:     1280,
				Height:    896,
			},
		}},
	)

	_, _, ref, err := searcher.FindBackground(context.Background(), plan)
	if err != nil {
		t.Fatalf("FindBackground() error = %v", err)
	}
	if ref.SourceURL != "https://example.com/page" {
		t.Fatalf("ref.SourceURL = %q, want page URL for disclosure", ref.SourceURL)
	}
}
