package deepresearch

import (
	"context"
	"encoding/xml"
	"sync"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestSkillExecutorExecute_UsesUserFromContextAndEmitsProgress(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{
				{Title: "Doc A", URL: "https://example.com/a", Description: "A"},
				{Title: "Doc B", URL: "https://example.com/b", Description: "B"},
			}, nil
		},
	})
	exec := NewSkillExecutor(svc)

	var (
		mu    sync.Mutex
		cards []map[string]interface{}
	)
	ctx := tools.WithUserID(context.Background(), "user-42")
	ctx = tools.WithCardEmitter(ctx, func(card map[string]interface{}) {
		mu.Lock()
		defer mu.Unlock()
		cp := make(map[string]interface{}, len(card))
		for k, v := range card {
			cp[k] = v
		}
		cards = append(cards, cp)
	})

	out, err := exec.Execute(ctx, map[string]interface{}{
		"query": "deep research skill executor",
		"mode":  "standard",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	data, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", out)
	}
	jobID, _ := data["job_id"].(string)
	if jobID == "" {
		t.Fatalf("expected non-empty job_id in result")
	}
	if _, err := svc.GetJobForUser(jobID, "user-42", ""); err != nil {
		t.Fatalf("owner should be able to fetch job: %v", err)
	}
	if _, err := svc.GetJobForUser(jobID, "other-user", ""); err != ErrJobForbidden {
		t.Fatalf("non-owner access err = %v, want %v", err, ErrJobForbidden)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(cards) == 0 {
		t.Fatalf("expected progress cards to be emitted")
	}
	var sawProgress, sawIntermediate bool
	for _, card := range cards {
		switch card["type"] {
		case "deep-research-progress":
			sawProgress = true
		case "result":
			if title, _ := card["title"].(string); title == "deep_research" {
				sawIntermediate = true
			}
		}
	}
	if !sawProgress {
		t.Fatalf("expected deep-research-progress card in %v", cards)
	}
	if !sawIntermediate {
		t.Fatalf("expected deep research intermediate result card in %v", cards)
	}
}

func TestSkillExecutorExecute_XMLFormat(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{
				{Title: "Doc A", URL: "https://example.com/a", Description: "A"},
			}, nil
		},
	})
	exec := NewSkillExecutor(svc)

	out, err := exec.Execute(context.Background(), map[string]interface{}{
		"query":  "xml deep research",
		"mode":   "standard",
		"format": "xml",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	raw, ok := out.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", out)
	}
	var parsed struct {
		XMLName       xml.Name `xml:"deep_research"`
		Query         string   `xml:"query"`
		EvidenceCount int      `xml:"evidence_count"`
		Iterations    int      `xml:"iterations"`
		Answer        string   `xml:"answer"`
	}
	if err := xml.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("failed to parse xml output: %v", err)
	}
	if parsed.Query != "xml deep research" {
		t.Fatalf("query = %q, want %q", parsed.Query, "xml deep research")
	}
	if parsed.EvidenceCount == 0 {
		t.Fatalf("expected non-zero evidence_count")
	}
	if parsed.Answer == "" {
		t.Fatalf("expected non-empty answer")
	}
	if parsed.Iterations == 0 {
		t.Fatalf("expected non-zero iterations")
	}
}

func TestSkillExecutorExecute_InvalidFormat(t *testing.T) {
	exec := NewSkillExecutor(nil)
	_, err := exec.Execute(context.Background(), map[string]interface{}{
		"query":  "invalid format",
		"format": "yaml",
	})
	if err == nil {
		t.Fatalf("expected invalid format error")
	}
}

func TestSkillExecutorExecute_FormatAliases(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{
				{Title: "Doc A", URL: "https://example.com/a", Description: "A"},
			}, nil
		},
	})
	exec := NewSkillExecutor(svc)

	jsonOut, err := exec.Execute(context.Background(), map[string]interface{}{
		"query":  "alias json deep research",
		"mode":   "standard",
		"format": "markdown,",
	})
	if err != nil {
		t.Fatalf("Execute failed for json alias: %v", err)
	}
	if _, ok := jsonOut.(map[string]interface{}); !ok {
		t.Fatalf("json alias result type = %T, want map", jsonOut)
	}

	xmlOut, err := exec.Execute(context.Background(), map[string]interface{}{
		"query":  "alias xml deep research",
		"mode":   "standard",
		"format": "text/xml;",
	})
	if err != nil {
		t.Fatalf("Execute failed for xml alias: %v", err)
	}
	raw, ok := xmlOut.(string)
	if !ok {
		t.Fatalf("xml alias result type = %T, want string", xmlOut)
	}
	var parsed struct {
		XMLName xml.Name `xml:"deep_research"`
		Query   string   `xml:"query"`
	}
	if err := xml.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("failed to parse xml alias output: %v", err)
	}
}

func TestSkillExecutorExecute_ResultIncludesTraceAndVerificationSummary(t *testing.T) {
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{{
				ID:       "task_1",
				Question: "topic overview",
				Priority: 1,
				Depth:    1,
				Status:   "pending",
				Axis:     "official",
			}}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			if query == "topic overview" {
				return []SearchHit{{Title: "Blog post", URL: "https://blog.example.com/post", Description: "analysis"}}, nil
			}
			return []SearchHit{{Title: "Official docs", URL: "https://docs.example.com/official", Description: "official documentation"}}, nil
		},
	})
	exec := NewSkillExecutor(svc)

	out, err := exec.Execute(context.Background(), map[string]interface{}{
		"query": "topic",
		"mode":  "standard",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	data, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", out)
	}
	if parseIntArg(data["iterations"]) < 2 {
		t.Fatalf("expected iterations >= 2, got %v", data["iterations"])
	}
	trace, ok := data["research_trace"].([]ResearchTraceEntry)
	if !ok || len(trace) == 0 {
		t.Fatalf("expected non-empty research_trace, got %#v", data["research_trace"])
	}
	if _, ok := data["verification_summary"].(*VerificationSummary); !ok {
		t.Fatalf("expected verification_summary in result, got %#v", data["verification_summary"])
	}
}
