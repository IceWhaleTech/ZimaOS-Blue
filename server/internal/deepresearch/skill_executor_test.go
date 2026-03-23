package deepresearch

import (
	"context"
	"encoding/xml"
	"strings"
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
	ctx = tools.WithSessionID(ctx, "conv-123")
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
	if got, _ := data["conversation_id"].(string); got != "conv-123" {
		t.Fatalf("conversation_id = %q, want %q", got, "conv-123")
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
	var sawProgress, sawEvent bool
	for _, card := range cards {
		switch card["type"] {
		case "deep-research-progress":
			sawProgress = true
			if got, _ := card["job_id"].(string); got != jobID {
				t.Fatalf("progress job_id = %q, want %q", got, jobID)
			}
			if got, _ := card["conversation_id"].(string); got != "conv-123" {
				t.Fatalf("progress conversation_id = %q, want %q", got, "conv-123")
			}
			if got, _ := card["id"].(string); got != "deep-research-progress-"+jobID {
				t.Fatalf("progress id = %q, want %q", got, "deep-research-progress-"+jobID)
			}
		case "deep-research-event":
			sawEvent = true
			if got, _ := card["job_id"].(string); got != jobID {
				t.Fatalf("event job_id = %q, want %q", got, jobID)
			}
			if got, _ := card["conversation_id"].(string); got != "conv-123" {
				t.Fatalf("event conversation_id = %q, want %q", got, "conv-123")
			}
			if got, _ := card["id"].(string); got == "" {
				t.Fatalf("expected stable event id in %v", card)
			}
		}
	}
	if !sawProgress {
		t.Fatalf("expected deep-research-progress card in %v", cards)
	}
	if !sawEvent {
		t.Fatalf("expected deep-research-event card in %v", cards)
	}
}

func TestSkillExecutorExecute_SupportsNestedCamelCaseArgs(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{{Title: "Doc A", URL: "https://example.com/a", Description: "A"}}, nil
		},
	})
	exec := NewSkillExecutor(svc)

	out, err := exec.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"query":        "nested deep research",
			"mode":         "deep",
			"routeMode":    "web",
			"strictEntity": true,
			"timeWindows":  []interface{}{"30d"},
			"reportStyle":  "knowledge-base",
			"maxSources":   3,
			"maxSeconds":   5,
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	data, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", out)
	}
	if got := data["query"]; got != "nested deep research" {
		t.Fatalf("query = %v, want nested deep research", got)
	}
	if got := data["mode"]; got != "deep" {
		t.Fatalf("mode = %v, want deep", got)
	}
	if got := data["requested_route_mode"]; got != "web" {
		t.Fatalf("requested_route_mode = %v, want web", got)
	}
	if got := data["strict_entity"]; got != true {
		t.Fatalf("strict_entity = %v, want true", got)
	}
	if got, ok := data["time_windows"].([]string); !ok || len(got) != 1 || got[0] != "30d" {
		t.Fatalf("time_windows = %#v, want [30d]", data["time_windows"])
	}
	if got := data["report_style"]; got != "knowledge_base" {
		t.Fatalf("report_style = %v, want knowledge_base", got)
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
	if _, ok := data["source_inventory"].([]map[string]interface{}); !ok {
		t.Fatalf("expected source_inventory in result, got %#v", data["source_inventory"])
	}
	if _, ok := data["workflow_phases"].([]map[string]interface{}); !ok {
		t.Fatalf("expected workflow_phases in result, got %#v", data["workflow_phases"])
	}
}

func TestSkillExecutorExecute_KnowledgeBaseStyleIncludesArtifacts(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{{Title: "Official docs", URL: "https://docs.example.com/official", Description: "official documentation"}}, nil
		},
	})
	exec := NewSkillExecutor(svc)

	out, err := exec.Execute(context.Background(), map[string]interface{}{
		"query":        "build a knowledge base for Blue research",
		"mode":         "standard",
		"report_style": "knowledge_base",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	data, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", out)
	}
	if got, _ := data["report_style"].(string); got != "knowledge_base" {
		t.Fatalf("report_style = %q, want %q", got, "knowledge_base")
	}
	if _, ok := data["object_map"].([]map[string]interface{}); !ok {
		t.Fatalf("expected object_map in result, got %#v", data["object_map"])
	}
	if _, ok := data["source_inventory"].([]map[string]interface{}); !ok {
		t.Fatalf("expected source_inventory in result, got %#v", data["source_inventory"])
	}
	if _, ok := data["coverage_summary"].(map[string]interface{}); !ok {
		t.Fatalf("expected coverage_summary in result, got %#v", data["coverage_summary"])
	}
	if _, ok := data["workflow_phases"].([]map[string]interface{}); !ok {
		t.Fatalf("expected workflow_phases in result, got %#v", data["workflow_phases"])
	}
	answer, _ := data["answer"].(string)
	if !strings.Contains(answer, "## Executive Summary") {
		t.Fatalf("expected knowledge-base answer headings, got %q", answer)
	}
}

func TestDeepResearchSourceCards_MergesAndDeduplicates(t *testing.T) {
	payload := map[string]interface{}{
		"query": "official roadmap",
		"sources": []interface{}{
			map[string]interface{}{"title": "Roadmap A", "url": "https://example.com/a", "domain": "example.com"},
			map[string]interface{}{"title": "Roadmap B", "url": "https://example.com/b", "domain": "example.com"},
		},
		"title":  "Roadmap C",
		"url":    "https://example.com/c",
		"domain": "example.com",
	}

	got := deepResearchSourceCards(payload)
	if len(got) != 3 {
		t.Fatalf("len(deepResearchSourceCards) = %d, want 3", len(got))
	}
	for i, source := range got {
		if query, _ := source["query"].(string); query != "official roadmap" {
			t.Fatalf("source[%d].query = %q, want %q", i, query, "official roadmap")
		}
	}

	merged := deepResearchMergeSourceCards(got, []map[string]interface{}{
		{"title": "Roadmap A", "url": "https://example.com/a", "domain": "example.com"},
		{"title": "Roadmap D", "url": "https://example.com/d", "domain": "example.com"},
	})
	if len(merged) != 4 {
		t.Fatalf("len(deepResearchMergeSourceCards) = %d, want 4", len(merged))
	}
}

func TestSkillExecutorExecute_KnowledgeBaseStyleAutoDetectedFromQuery(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{{Title: "Doc A", URL: "https://example.com/a", Description: "A"}}, nil
		},
	})
	exec := NewSkillExecutor(svc)

	out, err := exec.Execute(context.Background(), map[string]interface{}{
		"query": "please build a knowledge base for this topic",
		"mode":  "standard",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	data, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", out)
	}
	if got, _ := data["report_style"].(string); got != "knowledge_base" {
		t.Fatalf("auto report_style = %q, want %q", got, "knowledge_base")
	}
}
