package builtin

import (
	"context"
	"testing"
)

func TestUIReviewerManifest(t *testing.T) {
	ur := NewUIReviewer()
	m := ur.Manifest()
	if m.ID != "ui_reviewer" {
		t.Fatalf("expected id ui_reviewer, got %s", m.ID)
	}
	if m.Category != "system" {
		t.Fatalf("expected category system, got %s", m.Category)
	}
}

func TestUIReviewerValidate(t *testing.T) {
	ur := NewUIReviewer()

	tests := []struct {
		name       string
		input      map[string]any
		wantErr    bool
		wantAction string
	}{
		{"missing action", map[string]any{}, true, ""},
		{"infer review_url from url", map[string]any{"url": "http://example.com"}, false, "review_url"},
		{"infer review_url from href", map[string]any{"href": "http://example.com"}, false, "review_url"},
		{"infer review_image from image", map[string]any{"image": "base64data"}, false, "review_image"},
		{"infer review_image from image_base64", map[string]any{"image_base64": "base64data"}, false, "review_image"},
		{"infer review_image from screenshot", map[string]any{"screenshot": "base64data"}, false, "review_image"},
		{"canonicalize audit url alias", map[string]any{"action": "audit", "url": "http://example.com"}, false, "review_url"},
		{"canonicalize audit image alias", map[string]any{"action": "audit", "image": "base64data"}, false, "review_image"},
		{"canonicalize accessibility alias", map[string]any{"action": "a11y", "url": "http://example.com"}, false, "check_accessibility"},
		{"invalid action", map[string]any{"action": "bad"}, true, "bad"},
		{"review_url without url", map[string]any{"action": "review_url"}, true, "review_url"},
		{"review_url ok", map[string]any{"action": "review_url", "url": "http://example.com"}, false, "review_url"},
		{"review_image without image", map[string]any{"action": "review_image"}, true, "review_image"},
		{"review_image ok", map[string]any{"action": "review_image", "image": "base64data"}, false, "review_image"},
		{"check_accessibility without url", map[string]any{"action": "check_accessibility"}, true, "check_accessibility"},
		{"check_accessibility ok", map[string]any{"action": "check_accessibility", "url": "http://example.com"}, false, "check_accessibility"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ur.Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got, _ := tt.input["action"].(string); got != tt.wantAction {
				t.Errorf("input[action] = %q, want %q", got, tt.wantAction)
			}
		})
	}
}

func TestUIReviewerNoBrowser(t *testing.T) {
	ur := NewUIReviewer()
	// No browser service set — review_url should fail gracefully
	result, err := ur.Execute(context.Background(), map[string]any{
		"action": "review_url",
		"url":    "http://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when browser not available")
	}
}

func TestUIReviewerExecuteCanonicalizesAuditAlias(t *testing.T) {
	ur := NewUIReviewer()
	result, err := ur.Execute(context.Background(), map[string]any{
		"action": "audit",
		"url":    "http://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when browser not available")
	}
}

func TestUIReviewerExecuteAcceptsHrefAlias(t *testing.T) {
	ur := NewUIReviewer()
	result, err := ur.Execute(context.Background(), map[string]any{
		"href": "http://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when browser not available")
	}
	if result.Error != "browser service not available — cannot review URL" {
		t.Fatalf("error = %q, want browser service error", result.Error)
	}
}

func TestUIReviewerExecuteAcceptsImageBase64Alias(t *testing.T) {
	ur := NewUIReviewer()
	result, err := ur.Execute(context.Background(), map[string]any{
		"image_base64": "base64data",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when bridge not available")
	}
	if result.Error != "proxy bridge not available — cannot call VLM" {
		t.Fatalf("error = %q, want proxy bridge error", result.Error)
	}
}

func TestUIReviewerExecuteAcceptsScreenshotAlias(t *testing.T) {
	ur := NewUIReviewer()
	result, err := ur.Execute(context.Background(), map[string]any{
		"screenshot": "base64data",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when bridge not available")
	}
	if result.Error != "proxy bridge not available — cannot call VLM" {
		t.Fatalf("error = %q, want proxy bridge error", result.Error)
	}
}

func TestUIReviewerNoBridge(t *testing.T) {
	ur := NewUIReviewer()
	// No bridge set — review_image should fail gracefully
	result, err := ur.Execute(context.Background(), map[string]any{
		"action": "review_image",
		"image":  "base64data",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when bridge not available")
	}
}

func TestComputeScoresNoVLM(t *testing.T) {
	ur := NewUIReviewer()
	funcResult := &FunctionalCheckResult{Score: 90}
	a11yResult := &A11yCheckResult{Score: 80}

	result := ur.computeScores("http://example.com", []string{"desktop"}, funcResult, a11yResult, nil, 75)

	// No VLM: Functional 55%, Accessibility 45%
	expected := 90*0.55 + 80*0.45
	if result.Overall < expected-0.1 || result.Overall > expected+0.1 {
		t.Errorf("expected overall ~%.1f, got %.1f", expected, result.Overall)
	}
	if !result.Pass {
		t.Error("expected pass with score > 75")
	}
}

func TestComputeScoresWithVLM(t *testing.T) {
	ur := NewUIReviewer()
	funcResult := &FunctionalCheckResult{Score: 90}
	a11yResult := &A11yCheckResult{Score: 80}
	vlmResult := &VLMReviewResult{
		Scores: map[string]float64{
			"visual_hierarchy": 85,
			"layout_alignment": 90,
			"color_harmony":    80,
			"typography":       75,
			"professionalism":  82,
		},
	}

	result := ur.computeScores("http://example.com", []string{"desktop"}, funcResult, a11yResult, vlmResult, 75)

	// Visual avg = (85+90+80+75+82)/5 = 82.4
	// Overall = 82.4*0.30 + 90*0.40 + 80*0.30 = 24.72 + 36 + 24 = 84.72
	if result.Overall < 84 || result.Overall > 86 {
		t.Errorf("expected overall ~84.7, got %.1f", result.Overall)
	}
	if !result.Pass {
		t.Error("expected pass")
	}
}

func TestComputeScoresCriticalFail(t *testing.T) {
	ur := NewUIReviewer()
	funcResult := &FunctionalCheckResult{
		Score:  90,
		Issues: []UIIssue{{Severity: "critical", Category: "functional", Description: "page crash"}},
	}
	a11yResult := &A11yCheckResult{Score: 80}

	result := ur.computeScores("http://example.com", []string{"desktop"}, funcResult, a11yResult, nil, 75)

	if result.Pass {
		t.Error("expected fail due to critical issue")
	}
}

func TestFormatHumanReport(t *testing.T) {
	r := &ReviewResult{
		URL:           "http://example.com",
		Overall:       82.1,
		Pass:          true,
		Threshold:     75,
		Visual:        ScoreDetail{Score: 82},
		Functional:    ScoreDetail{Score: 90},
		Accessibility: ScoreDetail{Score: 75},
		Issues: []UIIssue{
			{Severity: "major", Description: "Mobile nav overflow"},
		},
	}

	human := formatHumanReport(r)
	if human == "" {
		t.Fatal("expected non-empty human report")
	}
	if !contains(human, "82.1") {
		t.Error("expected overall score in report")
	}
	if !contains(human, "PASS") {
		t.Error("expected PASS in report")
	}
	if !contains(human, "Mobile nav overflow") {
		t.Error("expected issue in report")
	}
}

func TestAvgScores(t *testing.T) {
	scores := map[string]float64{"a": 80, "b": 90}
	avg := avgScores(scores)
	if avg != 85 {
		t.Errorf("expected 85, got %.1f", avg)
	}

	if avgScores(nil) != 0 {
		t.Error("expected 0 for nil scores")
	}
}

func TestHasCriticalIssue(t *testing.T) {
	if hasCriticalIssue(nil) {
		t.Error("expected false for nil")
	}
	if hasCriticalIssue([]UIIssue{{Severity: "minor"}}) {
		t.Error("expected false for minor")
	}
	if !hasCriticalIssue([]UIIssue{{Severity: "critical"}}) {
		t.Error("expected true for critical")
	}
}

func TestParseViewports(t *testing.T) {
	vps := parseViewports(map[string]any{})
	if len(vps) != 1 || vps[0] != "desktop" {
		t.Errorf("expected [desktop], got %v", vps)
	}

	vps = parseViewports(map[string]any{"viewports": []any{"mobile", "desktop"}})
	if len(vps) != 2 {
		t.Errorf("expected 2 viewports, got %d", len(vps))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
