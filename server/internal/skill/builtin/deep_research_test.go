package builtin

import (
	"context"
	"testing"
)

type mockDeepResearchExecutor struct{}

func (m *mockDeepResearchExecutor) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if args["format"] == "xml" {
		return `<deep_research><query>test deep research</query><answer>test answer</answer></deep_research>`, nil
	}
	return map[string]any{
		"query":          args["query"],
		"mode":           "standard",
		"answer":         "test answer",
		"confidence":     0.88,
		"evidence_count": 3,
		"citations": []map[string]any{
			{"title": "Doc", "url": "https://example.com"},
		},
	}, nil
}

func TestDeepResearchSkill_ReturnsStructuredOutput(t *testing.T) {
	s := NewDeepResearch()
	s.SetExecutor(&mockDeepResearchExecutor{})

	res, err := s.Execute(context.Background(), map[string]any{
		"query": "test deep research",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}

	data, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", res.Data)
	}
	if _, exists := data["_card"]; exists {
		t.Fatalf("did not expect _card hint in deep_research output")
	}
	if answer, ok := data["answer"].(string); !ok || answer == "" {
		t.Fatalf("expected non-empty answer, got %#v", data["answer"])
	}
}

func TestDeepResearchSkill_XMLOutputPassThrough(t *testing.T) {
	s := NewDeepResearch()
	s.SetExecutor(&mockDeepResearchExecutor{})

	res, err := s.Execute(context.Background(), map[string]any{
		"query":  "test deep research",
		"format": "xml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}

	data, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", res.Data)
	}
	raw, ok := data["result"].(string)
	if !ok || raw == "" {
		t.Fatalf("expected xml string result, got %#v", data["result"])
	}
}

func TestDeepResearchSkill_ValidateRejectsInvalidFormat(t *testing.T) {
	s := NewDeepResearch()
	err := s.Validate(map[string]any{
		"query":  "test deep research",
		"format": "yaml",
	})
	if err == nil {
		t.Fatalf("expected invalid format validation error")
	}
}

func TestDeepResearchSkill_ValidateAcceptsFormatAliases(t *testing.T) {
	s := NewDeepResearch()
	input := map[string]any{
		"query":  "test deep research",
		"format": "text/xml;",
	}
	if err := s.Validate(input); err != nil {
		t.Fatalf("expected alias format to pass validation: %v", err)
	}
	if got, _ := input["format"].(string); got != "xml" {
		t.Fatalf("format = %q, want %q", got, "xml")
	}
}

func TestDeepResearchSkill_ValidateAcceptsKnowledgeBaseStyleAlias(t *testing.T) {
	s := NewDeepResearch()
	input := map[string]any{
		"query":        "test deep research",
		"report_style": "knowledge-base",
	}
	if err := s.Validate(input); err != nil {
		t.Fatalf("expected knowledge base style alias to pass validation: %v", err)
	}
	if got, _ := input["report_style"].(string); got != "knowledge_base" {
		t.Fatalf("report_style = %q, want %q", got, "knowledge_base")
	}
}

func TestDeepResearchSkill_ValidatePromotesInputAliasToQuery(t *testing.T) {
	s := NewDeepResearch()
	input := map[string]any{
		"input": " test deep research ",
	}
	if err := s.Validate(input); err != nil {
		t.Fatalf("expected input alias to pass validation: %v", err)
	}
	if got, _ := input["query"].(string); got != "test deep research" {
		t.Fatalf("query = %q, want %q", got, "test deep research")
	}
}

func TestDeepResearchSkill_ValidateRejectsConflictingQueryAliases(t *testing.T) {
	s := NewDeepResearch()
	err := s.Validate(map[string]any{
		"input":   "first research topic",
		"message": "second research topic",
	})
	if err == nil {
		t.Fatalf("expected conflicting aliases to remain invalid without explicit query")
	}
}

func TestDeepResearchSkill_ValidateDoesNotOverrideExplicitQuery(t *testing.T) {
	s := NewDeepResearch()
	input := map[string]any{
		"query": "canonical query",
		"input": "alias query",
	}
	if err := s.Validate(input); err != nil {
		t.Fatalf("expected explicit query to pass validation: %v", err)
	}
	if got, _ := input["query"].(string); got != "canonical query" {
		t.Fatalf("query = %q, want %q", got, "canonical query")
	}
}

func TestDeepResearchSkill_ExecuteAcceptsInputAlias(t *testing.T) {
	s := NewDeepResearch()
	s.SetExecutor(&mockDeepResearchExecutor{})

	res, err := s.Execute(context.Background(), map[string]any{
		"input": "test deep research",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}

	data, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", res.Data)
	}
	if got, _ := data["query"].(string); got != "test deep research" {
		t.Fatalf("query = %q, want %q", got, "test deep research")
	}
}

func TestDeepResearchSkill_ValidateRejectsInvalidRouteMode(t *testing.T) {
	s := NewDeepResearch()
	err := s.Validate(map[string]any{
		"query":      "test deep research",
		"route_mode": "lab",
	})
	if err == nil {
		t.Fatalf("expected invalid route_mode validation error")
	}
}
