package tools

import (
	"context"
	"testing"
)

func TestEmitSkillResultCardFromData_DeepResearch(t *testing.T) {
	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		emitted = append(emitted, card)
	})

	data := map[string]string{
		"query":          "test topic",
		"answer":         "summary",
		"confidence":     "0.82",
		"evidence_count": "3",
		"citations":      `[{"title":"A","url":"https://example.com"}]`,
	}
	emitSkillResultCardFromData(ctx, "deep_research", data)

	if len(emitted) != 1 {
		t.Fatalf("emitted %d cards, want 1", len(emitted))
	}
	card := emitted[0]
	if got := card["type"]; got != "deep-research" {
		t.Fatalf("card type = %v, want deep-research", got)
	}
	if got := card["query"]; got != "test topic" {
		t.Fatalf("card query = %v, want test topic", got)
	}
	if got, ok := card["confidence"].(float64); !ok || got != 0.82 {
		t.Fatalf("card confidence = %#v, want 0.82", card["confidence"])
	}
}

func TestEmitSkillResultCardFromData_UIReviewerHint(t *testing.T) {
	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		emitted = append(emitted, card)
	})

	data := map[string]string{
		"_card":  "ui_reviewer",
		"result": `{"url":"http://x.com","overall":80,"pass":true,"issues":[],"suggestions":[]}`,
	}
	emitSkillResultCardFromData(ctx, "ui_reviewer", data)

	if len(emitted) != 1 {
		t.Fatalf("emitted %d cards, want 1", len(emitted))
	}
	card := emitted[0]
	if got := card["type"]; got != "ui-review" {
		t.Fatalf("card type = %v, want ui-review", got)
	}
	if got := card["url"]; got != "http://x.com" {
		t.Fatalf("card url = %v, want http://x.com", got)
	}
	if got, ok := card["pass"].(bool); !ok || !got {
		t.Fatalf("card pass = %#v, want true", card["pass"])
	}
}

func TestEmitSkillResultCardFromData_Analyze(t *testing.T) {
	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		emitted = append(emitted, card)
	})

	data := map[string]string{
		"topic":      "Market analysis",
		"report_url": "/reports/r1.html",
	}
	emitSkillResultCardFromData(ctx, "analyze", data)

	if len(emitted) != 1 {
		t.Fatalf("emitted %d cards, want 1", len(emitted))
	}
	card := emitted[0]
	if got := card["type"]; got != "result" {
		t.Fatalf("card type = %v, want result", got)
	}
	if got := card["title"]; got != "analyze" {
		t.Fatalf("card title = %v, want analyze", got)
	}
	if got := card["message"]; got != "Market analysis" {
		t.Fatalf("card message = %v, want Market analysis", got)
	}
}

func TestEmitSkillResultCardFromData_PreservesHintedSearch(t *testing.T) {
	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		emitted = append(emitted, card)
	})

	data := map[string]string{
		"_card":   "search",
		"query":   "zimaos",
		"results": `[{"title":"A","url":"https://example.com"}]`,
	}
	emitSkillResultCardFromData(ctx, "web_search", data)

	if len(emitted) != 1 {
		t.Fatalf("emitted %d cards, want 1", len(emitted))
	}
	card := emitted[0]
	if got := card["type"]; got != "search" {
		t.Fatalf("card type = %v, want search", got)
	}
	if got := card["query"]; got != "zimaos" {
		t.Fatalf("card query = %v, want zimaos", got)
	}
}
