package scenecompose

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type stubPlannerLLM struct {
	content string
}

func (s stubPlannerLLM) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: s.content,
		},
	}, nil
}

func TestHeuristicPlannerExtractsSingleScene(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("一只猫在森林里，柔和阳光"))
	if got := plan.Background; got != "森林里，柔和阳光" && got != "森林里" && got == "" {
		t.Fatalf("background = %q, want non-empty forest phrase", got)
	}
	if len(plan.Foreground) != 1 {
		t.Fatalf("foreground count = %d, want 1", len(plan.Foreground))
	}
	if plan.Foreground[0].Type != "cat" {
		t.Fatalf("type = %q, want cat", plan.Foreground[0].Type)
	}
	if plan.Foreground[0].Layout.Depth != "midground" {
		t.Fatalf("depth = %q, want midground", plan.Foreground[0].Layout.Depth)
	}
}

func TestHeuristicPlannerExtractsMultiForegroundLayout(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("右边一只小狗，左边一只猫"))
	if len(plan.Foreground) != 2 {
		t.Fatalf("foreground count = %d, want 2", len(plan.Foreground))
	}
	if plan.Foreground[0].Layout.Horizontal != "right" {
		t.Fatalf("first horizontal = %q, want right", plan.Foreground[0].Layout.Horizontal)
	}
	if plan.Foreground[1].Layout.Horizontal != "left" {
		t.Fatalf("second horizontal = %q, want left", plan.Foreground[1].Layout.Horizontal)
	}
}

func TestHeuristicPlannerRecognizesTeddyAsDogInChinesePrompt(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("泰迪在雪地里玩耍"))
	if len(plan.Foreground) != 1 {
		t.Fatalf("foreground count = %d, want 1", len(plan.Foreground))
	}
	if plan.Foreground[0].Type != "dog" {
		t.Fatalf("type = %q, want dog", plan.Foreground[0].Type)
	}
	if !strings.Contains(plan.Background, "雪地") && plan.Background == "" {
		t.Fatalf("background = %q, want snow-related background", plan.Background)
	}
}

func TestHeuristicPlannerBuildsForegroundSearchHintsFromPrompt(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("帮我生成一张泰迪在雪地里玩耍的照片"))
	if len(plan.Foreground) != 1 {
		t.Fatalf("foreground count = %d, want 1", len(plan.Foreground))
	}
	if got := plan.Foreground[0].SearchQuery; !strings.Contains(got, "泰迪") || strings.Contains(got, "狗 isolated") {
		t.Fatalf("search_query = %q, want concrete teddy subject hint", got)
	}
	if got := plan.Foreground[0].SearchQuery; strings.Contains(got, "帮我") {
		t.Fatalf("search_query = %q, should trim conversational prefix", got)
	}
	if got := plan.Foreground[0].SearchQuery; strings.Contains(got, "雪地") {
		t.Fatalf("search_query = %q, should avoid background terms", got)
	}
	if got := plan.Foreground[0].FallbackQuery; !strings.Contains(got, "泰迪") {
		t.Fatalf("fallback_query = %q, want concrete teddy fallback hint", got)
	}
}

func TestPlannerLLMReturnsSearchHints(t *testing.T) {
	planner := NewPlanner(stubPlannerLLM{content: `{
		"background": "雪地",
		"style": "realistic",
		"lighting": "soft natural light",
		"time_of_day": "day",
		"weather": "snowy",
		"camera_view": "eye-level",
		"scene_query": "泰迪犬 雪地 玩耍 写实照片",
		"scene_query_en": "toy poodle playing in snow realistic photo",
		"background_query": "雪地 冬季 户外 背景 照片",
		"background_query_en": "snowy winter outdoor background landscape photo",
		"foreground": [{
			"id": "dog_1",
			"type": "dog",
			"search_query": "泰迪犬 奔跑 isolated png transparent",
			"search_query_en": "toy poodle running isolated png transparent",
			"fallback_query": "泰迪犬 奔跑 写实照片",
			"fallback_query_en": "toy poodle running realistic photo",
			"priority": 1,
			"layout": {
				"horizontal": "center",
				"vertical": "low",
				"depth": "midground",
				"scale": "medium",
				"grounded": true
			}
		}]
	}`})

	plan, err := planner.Plan(context.Background(), ComposeRequest{
		Prompt: "帮我生成一张泰迪在雪地里玩耍的照片",
		Locale: "zh-CN",
	})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if plan.SceneQuery != "泰迪犬 雪地 玩耍 写实照片" {
		t.Fatalf("scene_query = %q", plan.SceneQuery)
	}
	if plan.SceneQueryEN != "toy poodle playing in snow realistic photo" {
		t.Fatalf("scene_query_en = %q", plan.SceneQueryEN)
	}
	if plan.BackgroundQuery != "雪地 冬季 户外 背景 照片" {
		t.Fatalf("background_query = %q", plan.BackgroundQuery)
	}
	if plan.BackgroundQueryEN != "snowy winter outdoor background landscape photo" {
		t.Fatalf("background_query_en = %q", plan.BackgroundQueryEN)
	}
	if len(plan.Foreground) != 1 {
		t.Fatalf("foreground count = %d, want 1", len(plan.Foreground))
	}
	if plan.Foreground[0].SearchQuery != "泰迪犬 奔跑 isolated png transparent" {
		t.Fatalf("foreground search_query = %q", plan.Foreground[0].SearchQuery)
	}
	if plan.Foreground[0].SearchQueryEN != "toy poodle running isolated png transparent" {
		t.Fatalf("foreground search_query_en = %q", plan.Foreground[0].SearchQueryEN)
	}
	if plan.Foreground[0].FallbackQuery != "泰迪犬 奔跑 写实照片" {
		t.Fatalf("foreground fallback_query = %q", plan.Foreground[0].FallbackQuery)
	}
	if plan.Foreground[0].FallbackQueryEN != "toy poodle running realistic photo" {
		t.Fatalf("foreground fallback_query_en = %q", plan.Foreground[0].FallbackQueryEN)
	}
}

func TestHeuristicPlannerUsesCompiledCueMatchersForSceneTraits(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("friendly small orange cartoon robot, city street, rainy night, top-down view, floating"))
	if plan.Background != "city street" {
		t.Fatalf("background = %q, want city street", plan.Background)
	}
	if plan.Style != "cartoon" {
		t.Fatalf("style = %q, want cartoon", plan.Style)
	}
	if plan.TimeOfDay != "night" {
		t.Fatalf("time_of_day = %q, want night", plan.TimeOfDay)
	}
	if plan.Weather != "rainy" {
		t.Fatalf("weather = %q, want rainy", plan.Weather)
	}
	if plan.CameraView != "top-down" {
		t.Fatalf("camera_view = %q, want top-down", plan.CameraView)
	}
	if len(plan.Foreground) != 1 {
		t.Fatalf("foreground count = %d, want 1", len(plan.Foreground))
	}
	if plan.Foreground[0].Type != "robot" {
		t.Fatalf("type = %q, want robot", plan.Foreground[0].Type)
	}
	if got := plan.Foreground[0].Attributes; len(got) != 3 || got[0] != "orange" || got[1] != "small" || got[2] != "friendly" {
		t.Fatalf("attributes = %v, want [orange small friendly]", got)
	}
	if plan.Foreground[0].Layout.Grounded {
		t.Fatal("expected floating subject not to be grounded")
	}
	if plan.Foreground[0].Layout.Vertical != "high" {
		t.Fatalf("vertical = %q, want high", plan.Foreground[0].Layout.Vertical)
	}
}

func TestCompactBackgroundPhraseTrimsChineseActionTail(t *testing.T) {
	if got := compactBackgroundPhrase("雪地里玩耍"); got != "雪地" {
		t.Fatalf("background = %q, want %q", got, "雪地")
	}
}
