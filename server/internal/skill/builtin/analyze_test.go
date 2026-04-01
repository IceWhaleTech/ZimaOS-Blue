package builtin

import (
	"context"
	"testing"
)

type mockAnalyzeExecutor struct {
	lastArgs map[string]interface{}
}

func (m *mockAnalyzeExecutor) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.lastArgs = make(map[string]interface{}, len(args))
	for k, v := range args {
		m.lastArgs[k] = v
	}
	return map[string]any{
		"topic":          args["topic"],
		"urls":           args["urls"],
		"search_queries": args["search_queries"],
		"text":           args["text"],
		"lang":           args["lang"],
		"output_mode":    args["output_mode"],
	}, nil
}

func TestAnalyzeSkill_ValidateAcceptsSubjectAndHrefAliases(t *testing.T) {
	a := NewAnalyze()
	input := map[string]any{
		"subject": "Release notes",
		"href":    "https://example.com/release",
	}
	if err := a.Validate(input); err != nil {
		t.Fatalf("expected aliases to pass validation: %v", err)
	}
	if got, _ := input["topic"].(string); got != "Release notes" {
		t.Fatalf("topic = %q, want Release notes", got)
	}
	urls, ok := input["urls"].([]interface{})
	if !ok || len(urls) != 1 || urls[0] != "https://example.com/release" {
		t.Fatalf("urls = %#v, want href promoted", input["urls"])
	}
}

func TestAnalyzeSkill_ExecuteAcceptsModernAliases(t *testing.T) {
	exec := &mockAnalyzeExecutor{}
	a := NewAnalyze()
	a.SetExecutor(exec)

	res, err := a.Execute(context.Background(), map[string]any{
		"subject":       "Market map",
		"content":       "Some direct notes",
		"searchQueries": []interface{}{"query one"},
		"language":      "en-US",
		"outputMode":    "report",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if got, _ := exec.lastArgs["topic"].(string); got != "Market map" {
		t.Fatalf("topic = %q, want Market map", got)
	}
	if got, _ := exec.lastArgs["lang"].(string); got != "en-US" {
		t.Fatalf("lang = %q, want en-US", got)
	}
	if got, _ := exec.lastArgs["output_mode"].(string); got != "report" {
		t.Fatalf("output_mode = %q, want report", got)
	}
	if got, _ := exec.lastArgs["text"].(string); got != "Some direct notes" {
		t.Fatalf("text = %q, want Some direct notes", got)
	}
}

func TestAnalyzeSkill_ValidatePromotesURLFromTopic(t *testing.T) {
	a := NewAnalyze()
	input := map[string]any{
		"topic": "Summarise https://example.com/blog and extract the key points.",
	}
	if err := a.Validate(input); err != nil {
		t.Fatalf("expected topic URL promotion to pass validation: %v", err)
	}

	urls, ok := input["urls"].([]interface{})
	if !ok || len(urls) != 1 || urls[0] != "https://example.com/blog" {
		t.Fatalf("urls = %#v, want promoted topic URL", input["urls"])
	}
	if got, _ := input["topic"].(string); got != "Summarise and extract the key points." {
		t.Fatalf("topic = %q, want cleaned topic without URL", got)
	}
}
