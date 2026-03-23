package slidespec

import (
	"strings"
	"testing"
)

func TestBuildParsesChineseStructuredSlidePrompt(t *testing.T) {
	brief := Build(Input{
		Prompt: "帮我做一页PPT，标题：2026 产品战略；副标题：AI 驱动增长；要点：提升转化率；降低成本；全球化扩张",
	})

	if brief.RenderMode != RenderModeSlide {
		t.Fatalf("render_mode = %q, want %q", brief.RenderMode, RenderModeSlide)
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
	if brief.TemplateID != TemplateSplit {
		t.Fatalf("template_id = %q, want %q", brief.TemplateID, TemplateSplit)
	}
}

func TestBuildUsesSignalsFromPresetProfileAndSource(t *testing.T) {
	cases := []Input{
		{Prompt: "季度增长总览", StylePreset: "banana_slides"},
		{Prompt: "季度增长总览", StylePreset: "nano_slides"},
		{Prompt: "季度增长总览", StylePreset: "nanoslides"},
		{Prompt: "季度增长总览", StylePreset: "nano slides"},
		{Prompt: "季度增长总览", QualityProfile: "ppt"},
		{Prompt: "季度增长总览", Source: "slides"},
	}

	for idx, input := range cases {
		brief := Build(input)
		if brief.RenderMode != RenderModeSlide {
			t.Fatalf("case %d render_mode = %q, want slide", idx, brief.RenderMode)
		}
	}
}

func TestWrapPublicSpacePromptUsesNanoSlidesStyleLanguage(t *testing.T) {
	prompt := WrapPublicSpacePrompt(Brief{
		RenderMode:  RenderModeSlide,
		StylePreset: StylePresetNanoSlides,
		TemplateID:  TemplateTextOnly,
		Title:       "Platform strategy",
		Subtitle:    "2026 operating model",
		Bullets:     []string{"Security", "Reliability", "Growth"},
		AspectRatio: "16:9",
	}, "platform strategy slide")

	if !strings.Contains(prompt, "nanoslides-inspired") {
		t.Fatalf("prompt missing nano slides descriptor: %q", prompt)
	}
	if !strings.Contains(prompt, "executive-summary board") {
		t.Fatalf("prompt missing nano text-only motif: %q", prompt)
	}
}

func TestStylePresetDisplayNameUsesBranding(t *testing.T) {
	if got := StylePresetDisplayName("nano slides"); got != "nanoslides" {
		t.Fatalf("nano display name = %q, want nanoslides", got)
	}
	if got := StylePresetDisplayName("banana_slides"); got != "bananaslides" {
		t.Fatalf("banana display name = %q, want bananaslides", got)
	}
}

func TestBuildAutoGeneratesTitleAndSubtitleForCoverPrompt(t *testing.T) {
	brief := Build(Input{
		Prompt: "Create a cover slide about revenue growth for enterprise expansion",
	})

	if brief.RenderMode != RenderModeSlide {
		t.Fatalf("render_mode = %q, want slide", brief.RenderMode)
	}
	if brief.Title == "" {
		t.Fatal("expected title")
	}
	if brief.Subtitle == "" {
		t.Fatal("expected subtitle")
	}
}

func TestBuildTruncatesBulletsToThree(t *testing.T) {
	brief := Build(Input{
		Prompt: "PPT slide, title: Platform roadmap; bullets: security hardening; context compression; provider routing; benchmark tooling; release automation",
	})

	if len(brief.Bullets) != 3 {
		t.Fatalf("bullets = %#v, want 3", brief.Bullets)
	}
}

func TestBuildPrefersAgendaTemplateForOutlineSlides(t *testing.T) {
	brief := Build(Input{
		Prompt: "PPT slide, title: Q3 rollout agenda; agenda: Foundation; Pilot launch; Scale-up",
	})

	if brief.TemplateID != TemplateAgenda {
		t.Fatalf("template_id = %q, want %q", brief.TemplateID, TemplateAgenda)
	}
}

func TestBuildPrefersMetricsTemplateForKPIContent(t *testing.T) {
	brief := Build(Input{
		Prompt: "PPT slide, title: Growth metrics; bullets: Conversion +18%; CAC -22%; ROI 3.4x",
	})

	if brief.TemplateID != TemplateMetrics {
		t.Fatalf("template_id = %q, want %q", brief.TemplateID, TemplateMetrics)
	}
}
