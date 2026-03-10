package selfreflect

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type mockLLM struct {
	content  string
	err      error
	requests []llm.ChatRequest
}

func (m *mockLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return nil, m.err
	}
	return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: m.content}}, nil
}

type memoryWrite struct {
	content string
	tags    []string
}

type mockWriter struct {
	writes []memoryWrite
}

func (m *mockWriter) Write(_ context.Context, content string, tags []string) error {
	m.writes = append(m.writes, memoryWrite{content: content, tags: tags})
	return nil
}

func TestServiceReflect_FiltersAndWritesTopLessons(t *testing.T) {
	llmStub := &mockLLM{content: `{
		"summary":"Reflection complete.",
		"lessons":[
			{"kind":"heuristic","lesson":"Run focused verification immediately after changing the parser path.","when_to_apply":"After touching parser control flow or branching logic.","evidence":"Verification caught the parser regression before the broader test pass."},
			{"kind":"heuristic","lesson":"Run focused verification immediately after changing the parser path.","when_to_apply":"After touching parser control flow or branching logic.","evidence":"Verification caught the parser regression before the broader test pass."},
			{"kind":"guardrail","lesson":"Keep a bounded recovery step before declaring the task done.","when_to_apply":"When a verification step fails and a safer retry is available.","evidence":"The recovery retry narrowed the failure instead of repeating the same broken path."},
			{"kind":"heuristic","lesson":"Follow best practices.","when_to_apply":"Always.","evidence":"General good engineering hygiene."},
			{"kind":"anti_pattern","lesson":"Assume deployment caching is the problem.","when_to_apply":"When a flaky test appears after a parser edit.","evidence":"Deployment cache invalidation was the root cause in production only."}
		]
	}`}
	writer := &mockWriter{}
	svc := NewService(llmStub, writer)

	result, err := svc.Reflect(context.Background(), Input{
		TaskID:        "task-1",
		Goal:          "Stabilize the parser release",
		FinalStatus:   "completed",
		ResultSummary: "The parser patch landed and verification passed after a focused check.",
		Plan: []Step{
			{Description: "patch parser branch", Status: "completed", Output: "updated parser branch handling"},
			{Description: "verify parser fix", Status: "completed", Output: "verification caught regression before broader tests"},
		},
		VerificationOutput: "verification caught the parser regression before the broader test pass, then the focused retry passed",
	})
	if err != nil {
		t.Fatalf("Reflect returned error: %v", err)
	}
	if len(result.Lessons) != 2 {
		t.Fatalf("lessons len = %d, want 2", len(result.Lessons))
	}
	if result.MemoryWritten != 2 {
		t.Fatalf("memory_written = %d, want 2", result.MemoryWritten)
	}
	if len(writer.writes) != 2 {
		t.Fatalf("writes len = %d, want 2", len(writer.writes))
	}
	if got := result.Lessons[0].Kind; got != LessonKindHeuristic {
		t.Fatalf("first kind = %s, want heuristic", got)
	}
	if got := writer.writes[0].tags[0]; got != "agent_reflection" {
		t.Fatalf("first tag = %q, want agent_reflection", got)
	}
	if llmStub.requests[0].Temperature != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", llmStub.requests[0].Temperature)
	}
}

func TestServiceReflect_FailedTaskPrioritizesAntiPatternAndGuardrail(t *testing.T) {
	svc := NewService(&mockLLM{content: `{
		"summary":"Failure reflection complete.",
		"lessons":[
			{"kind":"heuristic","lesson":"Start with a focused reproduction before broad recovery.","when_to_apply":"When a parser test fails after a small patch.","evidence":"The focused reproduction isolated the failing parser input quickly."},
			{"kind":"anti_pattern","lesson":"Do not retry the same failing recovery command without changing the inputs.","when_to_apply":"When the first recovery attempt returns the same verification failure.","evidence":"The second identical recovery attempt produced the same verification failure."},
			{"kind":"guardrail","lesson":"Capture the failing verification output before switching to recovery.","when_to_apply":"When verification fails and the task is about to retry with a safer path.","evidence":"The recorded verification output made the recovery path easier to constrain."}
		]
	}`}, nil)

	result, err := svc.Reflect(context.Background(), Input{
		Goal:               "Fix parser regression",
		FinalStatus:        "failed",
		FailureReason:      "verification failed and recovery failed",
		VerificationOutput: "verification failure repeated across both retries with the same parser input",
		Plan:               []Step{{Description: "verify fix", Status: "failed", Output: "same verification failure repeated"}},
	})
	if err != nil {
		t.Fatalf("Reflect returned error: %v", err)
	}
	if len(result.Lessons) != 3 {
		t.Fatalf("lessons len = %d, want 3", len(result.Lessons))
	}
	if result.Lessons[0].Kind != LessonKindAntiPattern {
		t.Fatalf("first lesson kind = %s, want anti_pattern", result.Lessons[0].Kind)
	}
	if result.Lessons[1].Kind != LessonKindGuardrail {
		t.Fatalf("second lesson kind = %s, want guardrail", result.Lessons[1].Kind)
	}
}

func TestServiceReflect_SkipsWhenNoGroundedLessonsRemain(t *testing.T) {
	svc := NewService(&mockLLM{content: `{
		"summary":"Nothing useful.",
		"lessons":[
			{"kind":"heuristic","lesson":"Be more careful.","when_to_apply":"Always.","evidence":"General care helps."}
		]
	}`}, &mockWriter{})

	result, err := svc.Reflect(context.Background(), Input{
		Goal:          "Clean up release notes",
		FinalStatus:   "completed",
		ResultSummary: "Release notes were updated.",
	})
	if err != nil {
		t.Fatalf("Reflect returned error: %v", err)
	}
	if len(result.Lessons) != 0 {
		t.Fatalf("lessons len = %d, want 0", len(result.Lessons))
	}
	if result.SkippedReason == "" {
		t.Fatal("expected skipped_reason to be populated")
	}
	if result.MemoryWritten != 0 {
		t.Fatalf("memory_written = %d, want 0", result.MemoryWritten)
	}
}
