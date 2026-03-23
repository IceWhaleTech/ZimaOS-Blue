package ppt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/slidespec"
)

type stubLayoutPlannerLLM struct {
	content  string
	err      error
	requests []llm.ChatRequest
}

func (s *stubLayoutPlannerLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: s.content},
	}, nil
}

func TestLayoutPlannerUsesLLMJSONSpec(t *testing.T) {
	llmStub := &stubLayoutPlannerLLM{content: `{
		"template_id": "cover",
		"style_preset": "nano_slides",
		"background": {
			"gradient": ["#0F172A", "#1D4ED8"],
			"overlay": "rgba(15,23,42,0.24)"
		},
		"elements": [
			{
				"kind": "text",
				"name": "title",
				"x": 96,
				"y": 120,
				"width": 480,
				"height": 120,
				"text": "AI Strategy",
				"font_role": "title",
				"font_size": 58,
				"font_weight": "bold",
				"color": "#F8FAFC"
			},
			{
				"kind": "image",
				"name": "hero",
				"x": 700,
				"y": 72,
				"width": 500,
				"height": 560,
				"source": "visual",
				"fit": "cover",
				"radius": 24
			}
		]
	}`}
	planner := NewLayoutPlanner(llmStub)

	spec, err := planner.Plan(context.Background(), LayoutPlanRequest{
		Description:    "AI strategy overview",
		StylePreset:    slidespec.StylePresetBananaSlides,
		AspectRatio:    "16:9",
		Width:          1280,
		Height:         720,
		HasVisual:      true,
		ReferenceCount: 1,
		Brief: slidespec.Brief{
			RenderMode:  slidespec.RenderModeSlide,
			StylePreset: slidespec.StylePresetBananaSlides,
			TemplateID:  slidespec.TemplateSplit,
			Title:       "AI Strategy",
			Subtitle:    "2026 plan",
			AspectRatio: "16:9",
			VisualQuery: "abstract AI growth visual",
		},
	})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if spec == nil {
		t.Fatal("expected layout spec")
	}
	if spec.TemplateID != slidespec.TemplateCover {
		t.Fatalf("template_id = %q, want %q", spec.TemplateID, slidespec.TemplateCover)
	}
	if spec.StylePreset != slidespec.StylePresetNanoSlides {
		t.Fatalf("style_preset = %q, want %q", spec.StylePreset, slidespec.StylePresetNanoSlides)
	}
	if spec.Canvas.Width != 1280 || spec.Canvas.Height != 720 {
		t.Fatalf("canvas = %#v, want 1280x720", spec.Canvas)
	}
	if len(spec.Elements) != 2 {
		t.Fatalf("elements = %#v, want 2 planned elements", spec.Elements)
	}
	if len(llmStub.requests) != 1 {
		t.Fatalf("llm calls = %d, want 1", len(llmStub.requests))
	}
	systemPrompt := llmStub.requests[0].Messages[0].Content
	for _, token := range []string{`"labels"`, `"value_format"`, `"direction"`} {
		if !strings.Contains(systemPrompt, token) {
			t.Fatalf("system prompt missing %s: %s", token, systemPrompt)
		}
	}
}

func TestLayoutPlannerFallsBackWhenLLMReturnsInvalidJSON(t *testing.T) {
	planner := NewLayoutPlanner(&stubLayoutPlannerLLM{content: `not-json`})

	spec, err := planner.Plan(context.Background(), LayoutPlanRequest{
		Description: "Quarterly growth overview",
		AspectRatio: "16:9",
		HasVisual:   true,
		Brief: slidespec.Brief{
			RenderMode:  slidespec.RenderModeSlide,
			StylePreset: slidespec.StylePresetBananaSlides,
			TemplateID:  slidespec.TemplateSplit,
			Title:       "Quarterly growth overview",
			Subtitle:    "Highlights",
			AspectRatio: "16:9",
			VisualQuery: "business growth presentation visual",
		},
	})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if spec == nil {
		t.Fatal("expected fallback layout spec")
	}
	if spec.TemplateID != slidespec.TemplateSplit {
		t.Fatalf("template_id = %q, want heuristic %q", spec.TemplateID, slidespec.TemplateSplit)
	}
	if len(spec.Elements) == 0 {
		t.Fatalf("elements = %#v, want heuristic elements", spec.Elements)
	}
}

func TestLayoutPlannerFallsBackWhenLLMErrors(t *testing.T) {
	planner := NewLayoutPlanner(&stubLayoutPlannerLLM{err: errors.New("llm unavailable")})

	spec, err := planner.Plan(context.Background(), LayoutPlanRequest{
		Description: "Roadmap cover",
		AspectRatio: "16:9",
		HasVisual:   true,
		Brief: slidespec.Brief{
			RenderMode:  slidespec.RenderModeSlide,
			StylePreset: slidespec.StylePresetBananaSlides,
			TemplateID:  slidespec.TemplateCover,
			Title:       "Roadmap cover",
			AspectRatio: "16:9",
			VisualQuery: "future roadmap hero visual",
		},
	})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if spec == nil {
		t.Fatal("expected fallback layout spec")
	}
	if spec.TemplateID != slidespec.TemplateCover {
		t.Fatalf("template_id = %q, want %q", spec.TemplateID, slidespec.TemplateCover)
	}
}
