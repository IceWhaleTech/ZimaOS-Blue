package scenecompose

import "testing"

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
