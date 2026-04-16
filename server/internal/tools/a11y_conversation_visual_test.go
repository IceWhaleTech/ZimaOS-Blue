package tools

import (
	"bytes"
	"context"
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

func TestLocateA11yConversationVisualHit_PrefersGroundingScreenshotBytes(t *testing.T) {
	backend := &a11yCompatBackend{
		hostOS:                   "darwin",
		screenshotGroundingBytes: []byte("grounding-png"),
	}
	tool := NewA11yTool()

	prevLocate := a11yLocateConversationVisualHitFromPNG
	var gotPNG []byte
	var gotSelector string
	a11yLocateConversationVisualHitFromPNG = func(_ context.Context, imagePNG []byte, selectorName string) (a11yConversationVisualHit, error) {
		gotPNG = append([]byte(nil), imagePNG...)
		gotSelector = selectorName
		return a11yConversationVisualHit{
			Point:      a11yruntime.NormalizedPoint{X: 0.33, Y: 0.44},
			Confidence: 0.91,
		}, nil
	}
	defer func() { a11yLocateConversationVisualHitFromPNG = prevLocate }()

	hit, err := tool.locateA11yConversationVisualHit(context.Background(), backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("locateA11yConversationVisualHit() error = %v", err)
	}
	if !bytes.Equal(gotPNG, []byte("grounding-png")) {
		t.Fatalf("png bytes = %q, want grounding-png", string(gotPNG))
	}
	if gotSelector != "Orca" {
		t.Fatalf("selectorName = %q, want Orca", gotSelector)
	}
	if hit.Point.X != 0.33 || hit.Point.Y != 0.44 {
		t.Fatalf("point = %#v, want forwarded visual hit", hit.Point)
	}
	if backend.lastGroundingScreenshotWindow != "win-feishu" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want win-feishu", backend.lastGroundingScreenshotWindow)
	}
	if backend.lastScreenshotWindow != "" {
		t.Fatalf("lastScreenshotWindow = %q, want public screenshot path to stay unused", backend.lastScreenshotWindow)
	}
}

func TestLocateA11yConversationVisualHit_PrefersChatGrounderCandidate(t *testing.T) {
	backend := &a11yCompatBackend{
		hostOS:                   "darwin",
		screenshotGroundingBytes: []byte("grounding-png"),
	}
	tool := NewA11yTool()
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{
						Role:       "conversation",
						Label:      "Orca",
						Confidence: 0.98,
						Bounds: a11yruntime.NormalizedRect{
							X:      0.10,
							Y:      0.20,
							Width:  0.30,
							Height: 0.10,
						},
					},
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)

	prevLocate := a11yLocateConversationVisualHitFromPNG
	a11yLocateConversationVisualHitFromPNG = func(context.Context, []byte, string) (a11yConversationVisualHit, error) {
		t.Fatal("expected chat grounder candidate to short-circuit OCR visual locator")
		return a11yConversationVisualHit{}, nil
	}
	defer func() { a11yLocateConversationVisualHitFromPNG = prevLocate }()

	state := newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca")
	hit, err := tool.locateA11yConversationVisualHit(withA11yChatExecutionState(context.Background(), state), backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("locateA11yConversationVisualHit() error = %v", err)
	}
	if hit.Point.X != 0.25 || hit.Point.Y != 0.25 {
		t.Fatalf("point = %#v, want center of grounded bounds", hit.Point)
	}
	if len(grounder.calls) != 1 {
		t.Fatalf("grounder.calls = %#v, want 1 locate_conversation request", grounder.calls)
	}
	if got := grounder.calls[0].AppProfile; got != "feishu_lark" {
		t.Fatalf("grounder call app_profile = %q, want feishu_lark", got)
	}
	if got := grounder.calls[0].TargetText; got != "Orca" {
		t.Fatalf("grounder call target_text = %q, want Orca", got)
	}
	if !bytes.Equal(grounder.calls[0].WindowScreenshot, []byte("grounding-png")) {
		t.Fatalf("grounder screenshot = %q, want grounding-png", string(grounder.calls[0].WindowScreenshot))
	}
	if state.groundingSource != "vision_model" {
		t.Fatalf("groundingSource = %q, want vision_model", state.groundingSource)
	}
}

func TestLocateA11yConversationVisualHit_FallsBackToOCRLocatorWhenChatGrounderMisses(t *testing.T) {
	backend := &a11yCompatBackend{
		hostOS:                   "darwin",
		screenshotGroundingBytes: []byte("grounding-png"),
	}
	tool := NewA11yTool()
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "search_field", Label: "Search", Confidence: 0.99},
				},
				Verification: map[string]interface{}{
					"rejected": true,
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)

	prevLocate := a11yLocateConversationVisualHitFromPNG
	ocrCalled := 0
	a11yLocateConversationVisualHitFromPNG = func(_ context.Context, imagePNG []byte, selectorName string) (a11yConversationVisualHit, error) {
		ocrCalled++
		if !bytes.Equal(imagePNG, []byte("grounding-png")) {
			t.Fatalf("png bytes = %q, want grounding-png", string(imagePNG))
		}
		if selectorName != "Orca" {
			t.Fatalf("selectorName = %q, want Orca", selectorName)
		}
		return a11yConversationVisualHit{
			Point:      a11yruntime.NormalizedPoint{X: 0.41, Y: 0.52},
			Confidence: 0.90,
		}, nil
	}
	defer func() { a11yLocateConversationVisualHitFromPNG = prevLocate }()

	hit, err := tool.locateA11yConversationVisualHit(context.Background(), backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("locateA11yConversationVisualHit() error = %v", err)
	}
	if ocrCalled != 1 {
		t.Fatalf("ocrCalled = %d, want 1 fallback invocation", ocrCalled)
	}
	if len(grounder.calls) != 1 {
		t.Fatalf("grounder.calls = %#v, want 1 locate_conversation request", grounder.calls)
	}
	if hit.Point.X != 0.41 || hit.Point.Y != 0.52 {
		t.Fatalf("point = %#v, want OCR fallback hit", hit.Point)
	}
}
