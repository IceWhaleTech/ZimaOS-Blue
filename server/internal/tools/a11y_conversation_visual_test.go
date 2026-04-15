package tools

import (
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestResolveA11yConversationVisualHitFromLines_ReturnsUniqueMatch(t *testing.T) {
	hit, err := resolveA11yConversationVisualHitFromLines([]a11yruntime.TextLine{
		{
			Text:       "Orca",
			Confidence: 0.92,
			Bounds: a11yruntime.NormalizedRect{
				X:      0.10,
				Y:      0.20,
				Width:  0.30,
				Height: 0.10,
			},
		},
	}, "Orca", a11yConversationImageRegion{X: 0, Y: 0, Width: 0.45, Height: 1})
	if err != nil {
		t.Fatalf("resolveA11yConversationVisualHitFromLines() error = %v", err)
	}
	if hit.Point.X != 0.1125 || hit.Point.Y != 0.25 {
		t.Fatalf("point = %#v, want center mapped into crop region", hit.Point)
	}
	if hit.Confidence != 0.92 {
		t.Fatalf("confidence = %v, want 0.92", hit.Confidence)
	}
}

func TestResolveA11yConversationVisualHitFromLines_FailsClosedOnAmbiguousMatches(t *testing.T) {
	_, err := resolveA11yConversationVisualHitFromLines([]a11yruntime.TextLine{
		{
			Text:       "Orca",
			Confidence: 0.95,
			Bounds: a11yruntime.NormalizedRect{
				X:      0.10,
				Y:      0.20,
				Width:  0.20,
				Height: 0.08,
			},
		},
		{
			Text:       "Orca Bot",
			Confidence: 0.88,
			Bounds: a11yruntime.NormalizedRect{
				X:      0.10,
				Y:      0.42,
				Width:  0.22,
				Height: 0.08,
			},
		},
	}, "Orca", a11yConversationImageRegion{X: 0, Y: 0, Width: 1, Height: 1})
	if err == nil {
		t.Fatal("resolveA11yConversationVisualHitFromLines() error = nil, want target_not_found")
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "target_not_found" {
		t.Fatalf("error code = %q, want target_not_found", runtimeErr.Code)
	}
	if got := runtimeErr.Details["matches"]; got != 2 {
		t.Fatalf("matches = %#v, want 2", got)
	}
}

func TestResolveA11yConversationVisualHitFromLines_IgnoresLowConfidenceMatches(t *testing.T) {
	_, err := resolveA11yConversationVisualHitFromLines([]a11yruntime.TextLine{
		{
			Text:       "Orca",
			Confidence: a11yConversationVisualConfidenceThreshold - 0.01,
			Bounds: a11yruntime.NormalizedRect{
				X:      0.10,
				Y:      0.20,
				Width:  0.20,
				Height: 0.08,
			},
		},
	}, "Orca", a11yConversationImageRegion{X: 0, Y: 0, Width: 1, Height: 1})
	if err == nil {
		t.Fatal("resolveA11yConversationVisualHitFromLines() error = nil, want target_not_found")
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "target_not_found" {
		t.Fatalf("error code = %q, want target_not_found", runtimeErr.Code)
	}
	if got := runtimeErr.Details["matches"]; got != 0 {
		t.Fatalf("matches = %#v, want 0", got)
	}
}
