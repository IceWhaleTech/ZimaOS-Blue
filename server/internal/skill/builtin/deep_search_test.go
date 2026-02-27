package builtin

import (
	"context"
	"testing"
)

type mockDeepSearchExecutor struct{}

func (m *mockDeepSearchExecutor) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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

func TestDeepSearchSkill_ReturnsStructuredOutput(t *testing.T) {
	s := NewDeepSearch()
	s.SetExecutor(&mockDeepSearchExecutor{})

	res, err := s.Execute(context.Background(), map[string]any{
		"query": "test deep search",
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
		t.Fatalf("did not expect _card hint in deep_search output")
	}
	if answer, ok := data["answer"].(string); !ok || answer == "" {
		t.Fatalf("expected non-empty answer, got %#v", data["answer"])
	}
}
