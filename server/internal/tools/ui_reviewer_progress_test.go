package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

type mockUIReviewBrowser struct{}

func (m *mockUIReviewBrowser) Start(context.Context) error { return nil }
func (m *mockUIReviewBrowser) NavigateURL(context.Context, string) (UIReviewNavResult, error) {
	return UIReviewNavResult{URL: "https://example.com", Title: "Example", TargetID: "tab-1"}, nil
}
func (m *mockUIReviewBrowser) GetAccessibilityTree(context.Context, string, int) (UIReviewA11yResult, error) {
	return UIReviewA11yResult{Tree: "heading 'Home'\nnavigation 'Main'"}, nil
}
func (m *mockUIReviewBrowser) ScreenshotTab(context.Context, string) (string, error) { return "", nil }
func (m *mockUIReviewBrowser) CloseTab(context.Context, string) error                { return nil }
func (m *mockUIReviewBrowser) SetViewport(context.Context, string, int, int) error   { return nil }
func (m *mockUIReviewBrowser) ScrollTo(context.Context, string, int, int) error      { return nil }
func (m *mockUIReviewBrowser) PageDimensions(context.Context, string) (int, int, error) {
	return 800, 800, nil
}
func (m *mockUIReviewBrowser) ScreenshotViewportRaw(context.Context, string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 160, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type mockVLMBridge struct{}

func (m *mockVLMBridge) ChatWithVision(context.Context, string, string) (string, error) {
	return `{"scores":{"visual_hierarchy":88,"layout_alignment":86,"color_harmony":84,"typography":87,"professionalism":89},"issues":[{"severity":"minor","description":"Minor spacing issue","location":"header"}],"suggestions":["Tighten header spacing"]}`, nil
}

func TestUIReviewerReviewURL_EmitsIntermediateStageCards(t *testing.T) {
	tool := NewUIReviewerTool()
	tool.SetBrowser(&mockUIReviewBrowser{})
	tool.SetVLMBridge(&mockVLMBridge{})
	tool.SetMediaDir(t.TempDir())

	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		cp := make(map[string]interface{}, len(card))
		for k, v := range card {
			cp[k] = v
		}
		emitted = append(emitted, cp)
	})

	result, err := tool.reviewURL(ctx, "https://example.com", map[string]interface{}{}, 75, "json", i18n.LangEnUS)
	if err != nil {
		t.Fatalf("reviewURL failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	var setupCard, auditCard, screenshotCard, visualCard bool
	for _, card := range emitted {
		if typ, _ := card["type"].(string); typ != "result" {
			continue
		}
		if title, _ := card["title"].(string); title != "ui_review" {
			continue
		}
		message, _ := card["message"].(string)
		switch message {
		case "Page loaded and viewport ready":
			setupCard = true
		case "Core audits completed":
			auditCard = true
		case "Page screenshots captured":
			screenshotCard = true
		case "Visual review completed":
			visualCard = true
		}
	}

	payload, _ := json.Marshal(emitted)
	if !setupCard || !auditCard || !screenshotCard || !visualCard {
		t.Fatalf("expected intermediate ui_review cards, got %s", payload)
	}
}

func TestUIReviewerExecuteSupportsNestedCamelCaseArgs(t *testing.T) {
	tool := NewUIReviewerTool()
	tool.SetBrowser(&mockUIReviewBrowser{})
	tool.SetVLMBridge(&mockVLMBridge{})
	tool.SetMediaDir(t.TempDir())

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":    "review_url",
			"url":       "https://example.com",
			"device":    "mobile",
			"channel":   "telegram",
			"lang":      string(i18n.LangEnUS),
			"waitMs":    1,
			"threshold": 82,
			"format":    "json",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload UIReviewResult
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if payload.Device != "mobile" {
		t.Fatalf("device = %q, want mobile", payload.Device)
	}
	if payload.Channel != "telegram" {
		t.Fatalf("channel = %q, want telegram", payload.Channel)
	}
	if payload.Threshold != 82 {
		t.Fatalf("threshold = %v, want 82", payload.Threshold)
	}
}

func TestBuildVLMPromptPPTProfile(t *testing.T) {
	prompt := buildVLMPrompt("", i18n.LangEnUS, UIReviewProfilePPT)
	if !strings.Contains(prompt, "typography_or_text_safety") {
		t.Fatalf("prompt missing ppt score key: %q", prompt)
	}
	if !strings.Contains(prompt, "slide visual or presentation asset") {
		t.Fatalf("prompt missing ppt guidance: %q", prompt)
	}
	if !strings.Contains(prompt, "Do not critique missing buttons") {
		t.Fatalf("prompt missing non-UI reminder: %q", prompt)
	}
}
