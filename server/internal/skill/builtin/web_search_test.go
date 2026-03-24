package builtin

import (
	"context"
	"testing"
)

type mockWebSearcher struct {
	result   interface{}
	err      error
	lastArgs map[string]interface{}
}

func (m *mockWebSearcher) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if args != nil {
		m.lastArgs = make(map[string]interface{}, len(args))
		for k, v := range args {
			m.lastArgs[k] = v
		}
	}
	return m.result, m.err
}

func TestWebSearchSkill_JSONBuildsSearchCard(t *testing.T) {
	s := NewWebSearch()
	s.SetSearcher(&mockWebSearcher{
		result: `{"query":"test query","results":[{"title":"Doc A","url":"https://example.com/a","description":"A"}],"total_count":1,"provider":"duckduckgo"}`,
	})

	res, err := s.Execute(context.Background(), map[string]any{
		"query":  "test query",
		"format": "json",
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
	if card, _ := data["_card"].(string); card != "search" {
		t.Fatalf("_card = %q, want %q", card, "search")
	}
	if query, _ := data["query"].(string); query != "test query" {
		t.Fatalf("query = %q, want %q", query, "test query")
	}
}

func TestWebSearchSkill_DefaultsToXML(t *testing.T) {
	s := NewWebSearch()
	s.SetSearcher(&mockWebSearcher{
		result: `<web_search><query>test query</query></web_search>`,
	})

	res, err := s.Execute(context.Background(), map[string]any{
		"query": "test query",
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
	if format, _ := data["format"].(string); format != "xml" {
		t.Fatalf("format = %q, want %q", format, "xml")
	}
	if result, _ := data["result"].(string); result != `<web_search><query>test query</query></web_search>` {
		t.Fatalf("unexpected result payload: %q", result)
	}
	if _, hasCard := data["_card"]; hasCard {
		t.Fatalf("did not expect _card for xml output")
	}
}

func TestWebSearchSkill_XMLReturnsRawResult(t *testing.T) {
	s := NewWebSearch()
	s.SetSearcher(&mockWebSearcher{
		result: `<web_search><query>test query</query></web_search>`,
	})

	res, err := s.Execute(context.Background(), map[string]any{
		"query":  "test query",
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
	if format, _ := data["format"].(string); format != "xml" {
		t.Fatalf("format = %q, want %q", format, "xml")
	}
	if result, _ := data["result"].(string); result != `<web_search><query>test query</query></web_search>` {
		t.Fatalf("unexpected result payload: %q", result)
	}
	if _, hasCard := data["_card"]; hasCard {
		t.Fatalf("did not expect _card for xml output")
	}
}

func TestWebSearchSkill_ValidateRejectsInvalidFormat(t *testing.T) {
	s := NewWebSearch()
	err := s.Validate(map[string]any{
		"query":  "test query",
		"format": "yaml",
	})
	if err == nil {
		t.Fatalf("expected validation error for invalid format")
	}
}

func TestWebSearchSkill_ValidateAcceptsFormatAliases(t *testing.T) {
	s := NewWebSearch()
	input := map[string]any{
		"query":  "test query",
		"format": "markdown,",
	}
	if err := s.Validate(input); err != nil {
		t.Fatalf("expected alias format to pass validation: %v", err)
	}
	if got, _ := input["format"].(string); got != "json" {
		t.Fatalf("format = %q, want %q", got, "json")
	}
}

func TestWebSearchSkill_ValidatePromotesInputAliasToQuery(t *testing.T) {
	s := NewWebSearch()
	input := map[string]any{
		"input": " test query ",
	}
	if err := s.Validate(input); err != nil {
		t.Fatalf("expected input alias to pass validation: %v", err)
	}
	if got, _ := input["query"].(string); got != "test query" {
		t.Fatalf("query = %q, want %q", got, "test query")
	}
}

func TestWebSearchSkill_ExecuteAcceptsQueryAlias(t *testing.T) {
	searcher := &mockWebSearcher{
		result: `{"query":"test query","results":[],"total_count":0,"provider":"duckduckgo"}`,
	}
	s := NewWebSearch()
	s.SetSearcher(searcher)

	res, err := s.Execute(context.Background(), map[string]any{
		"q":      "test query",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if got, _ := searcher.lastArgs["query"].(string); got != "test query" {
		t.Fatalf("query = %q, want %q", got, "test query")
	}
}
