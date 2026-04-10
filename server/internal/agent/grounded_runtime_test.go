package agent

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type groundedScriptLLM struct {
	plannerResponses   []string
	responderResponses []string
	plannerIndex       int
	responderIndex     int
}

type groundedKnowledgeCaptureLLM struct {
	groundedScriptLLM
	plannerRequests []llm.ChatRequest
}

func (m *groundedKnowledgeCaptureLLM) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if isGroundedPlannerPrompt(req) {
		m.plannerRequests = append(m.plannerRequests, req)
	}
	return m.groundedScriptLLM.Chat(ctx, req)
}

func (m *groundedScriptLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	switch {
	case isGroundedPlannerPrompt(req):
		content := `{"status":"complete","reason":"done","assertions":[]}`
		if m.plannerIndex < len(m.plannerResponses) {
			content = m.plannerResponses[m.plannerIndex]
			m.plannerIndex++
		}
		return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: content}}, nil
	case isGroundedResponderPrompt(req):
		content := `{"summary":"unknown","claims":[{"type":"unknown","text":"unknown"}]}`
		if m.responderIndex < len(m.responderResponses) {
			content = hydrateToolCallIDs(m.responderResponses[m.responderIndex], req)
			m.responderIndex++
		}
		return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: content}}, nil
	default:
		return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: defaultResponseForRequest(req)}}, nil
	}
}

func hydrateToolCallIDs(template string, req llm.ChatRequest) string {
	ids := extractToolCallIDsFromPrompt(req)
	if len(ids) == 0 {
		return strings.ReplaceAll(template, "__FIRST_TOOL_CALL_ID__", "missing")
	}
	template = strings.ReplaceAll(template, "__FIRST_TOOL_CALL_ID__", ids[0])
	template = strings.ReplaceAll(template, "__SECOND_TOOL_CALL_ID__", pickToolCallID(ids, 1))
	template = strings.ReplaceAll(template, "__THIRD_TOOL_CALL_ID__", pickToolCallID(ids, 2))
	return template
}

func pickToolCallID(ids []string, idx int) string {
	if idx >= 0 && idx < len(ids) {
		return ids[idx]
	}
	return ids[len(ids)-1]
}

func extractToolCallIDsFromPrompt(req llm.ChatRequest) []string {
	if len(req.Messages) < 2 {
		return nil
	}
	var ids []string
	for _, line := range strings.Split(req.Messages[1].Content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- task/") {
			continue
		}
		ids = append(ids, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
	}
	return ids
}

func newGroundedRuntimeForTest(t *testing.T, llmStub LLMCaller) (*GroundedRuntime, *Store, *Task) {
	t.Helper()
	store := testStore(t)
	registry := tools.NewRegistry()
	workspaceRoot := t.TempDir()
	registry.Register(tools.NewFileReadTool([]string{workspaceRoot}, 0))
	registry.Register(tools.NewFileWriteTool([]string{workspaceRoot}, 0))
	registry.Register(tools.NewLsTool([]string{workspaceRoot}))
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     llmStub,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-test-task",
		UserID:      "u1",
		Goal:        "exercise grounded runtime",
		GroundState: NewGroundTruthState(),
	}
	return rt, store, task
}

func TestGroundedRuntimeRejectsFileExistsClaimWithoutToolCall(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"complete","reason":"Assume the file exists.","assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"The file exists.","claims":[{"type":"fs_exists","path":"ghost.txt","value":"true"}]}`,
		},
	}
	rt, _, task := newGroundedRuntimeForTest(t, llmStub)
	step := PlanStep{Index: 0, Description: "report whether ghost.txt exists", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 4)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusFallback && result.GroundingStatus != GroundingStatusUnknown {
		t.Fatalf("grounding_status=%q, want fallback or unknown", result.GroundingStatus)
	}
	if strings.TrimSpace(result.Output) != "unknown" {
		t.Fatalf("output=%q, want unknown", result.Output)
	}
}

func TestGroundedRuntimeRejectsFabricatedLSOutput(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Need directory listing.","next_tool":{"tool":"ls","args":{"path":".","max_depth":1}},"assertions":[]}`,
			`{"status":"complete","reason":"Listing complete.","assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"ls shows fake-entry.txt.","claims":[{"type":"tool_output","tool_call_ids":["__FIRST_TOOL_CALL_ID__"],"excerpt":"fake-entry.txt"}]}`,
		},
	}
	rt, _, task := newGroundedRuntimeForTest(t, llmStub)
	step := PlanStep{Index: 0, Description: "inspect the workspace", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 4)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusFallback && result.GroundingStatus != GroundingStatusUnknown {
		t.Fatalf("grounding_status=%q, want fallback or unknown", result.GroundingStatus)
	}
	if strings.TrimSpace(result.Output) != "unknown" {
		t.Fatalf("output=%q, want unknown", result.Output)
	}
}

func TestGroundedRuntimeRejectsFileContentClaimWithoutReadTool(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Create the file first.","next_tool":{"tool":"write_file","args":{"path":"notes.txt","content":"hello grounded runtime"}},"assertions":[]}`,
			`{"status":"complete","reason":"Done writing.","assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"The file content is known.","claims":[{"type":"file_content_excerpt","tool_call_ids":["__FIRST_TOOL_CALL_ID__"],"path":"notes.txt","excerpt":"hello grounded runtime"}]}`,
		},
	}
	rt, _, task := newGroundedRuntimeForTest(t, llmStub)
	step := PlanStep{Index: 0, Description: "write and then describe notes.txt", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 4)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusFallback && result.GroundingStatus != GroundingStatusUnknown {
		t.Fatalf("grounding_status=%q, want fallback or unknown", result.GroundingStatus)
	}
	if strings.TrimSpace(result.Output) != "unknown" {
		t.Fatalf("output=%q, want unknown", result.Output)
	}
}

func TestGroundedRuntimeWriteLSReadPasses(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Create the file.","next_tool":{"tool":"write","args":{"path":"demo.txt","content":"hello"}},"assertions":[]}`,
			`{"status":"continue","reason":"List the directory.","next_tool":{"tool":"ls","args":{"path":".","max_depth":1}},"assertions":[]}`,
			`{"status":"continue","reason":"Read the file.","next_tool":{"tool":"read","args":{"path":"demo.txt"}},"assertions":[]}`,
			`{"status":"complete","reason":"Enough evidence collected.","assertions":[{"type":"file_exists","path":"demo.txt"},{"type":"tool_called","tool":"ls"}]}`,
		},
		responderResponses: []string{
			`{"summary":"The file now exists and contains the expected content.","claims":[{"type":"fs_exists","tool_call_ids":["__SECOND_TOOL_CALL_ID__"],"path":"demo.txt","value":"true"},{"type":"fs_size","tool_call_ids":["__THIRD_TOOL_CALL_ID__"],"path":"demo.txt","value":"5"},{"type":"file_content_excerpt","tool_call_ids":["__THIRD_TOOL_CALL_ID__"],"path":"demo.txt","excerpt":"hello"}]}`,
		},
	}
	rt, store, task := newGroundedRuntimeForTest(t, llmStub)
	step := PlanStep{Index: 0, Description: "create demo.txt and verify it", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusGrounded {
		t.Fatalf("grounding_status=%q, want %q", result.GroundingStatus, GroundingStatusGrounded)
	}
	for _, want := range []string{
		"demo.txt exists [tool_call_id=task/grounded-test-task/tc/2]",
		`demo.txt excerpt="hello" [tool_call_id=task/grounded-test-task/tc/3]`,
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("expected output to contain %q, got %q", want, result.Output)
		}
	}
	events, err := store.ListRuntimeEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListRuntimeEvents returned error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected runtime events to be persisted")
	}
}

func TestGroundedRuntimeExecuteStep_ResolvesKnowledgeOnceAndReusesAcrossPlannerRounds(t *testing.T) {
	llmStub := &groundedKnowledgeCaptureLLM{
		groundedScriptLLM: groundedScriptLLM{
			plannerResponses: []string{
				`{"status":"continue","reason":"Create the file.","next_tool":{"tool":"write","args":{"path":"demo.txt","content":"hello"}},"assertions":[]}`,
				`{"status":"continue","reason":"List the directory.","next_tool":{"tool":"ls","args":{"path":".","max_depth":1}},"assertions":[]}`,
				`{"status":"continue","reason":"Read the file.","next_tool":{"tool":"read","args":{"path":"demo.txt"}},"assertions":[]}`,
				`{"status":"complete","reason":"Enough evidence collected.","assertions":[{"type":"file_exists","path":"demo.txt"},{"type":"tool_called","tool":"ls"}]}`,
			},
			responderResponses: []string{
				`{"summary":"The file now exists and contains the expected content.","claims":[{"type":"fs_exists","tool_call_ids":["__SECOND_TOOL_CALL_ID__"],"path":"demo.txt","value":"true"},{"type":"fs_size","tool_call_ids":["__THIRD_TOOL_CALL_ID__"],"path":"demo.txt","value":"5"},{"type":"file_content_excerpt","tool_call_ids":["__THIRD_TOOL_CALL_ID__"],"path":"demo.txt","excerpt":"hello"}]}`,
			},
		},
	}
	resolver := &mockLoopKnowledgeResolver{
		result: &knowledge.LoopContextResult{
			Context:   "<loop_knowledge>\n- [knowledge slug=demo-step-evidence page_type=synthesis status=active confidence=high] Reuse one step-scoped evidence pack across planner rounds unless recovery needs fresh missing evidence.\n</loop_knowledge>",
			UsedCount: 1,
			UsedSlugs: []string{"demo-step-evidence"},
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	workspaceRoot := t.TempDir()
	registry.Register(tools.NewFileReadTool([]string{workspaceRoot}, 0))
	registry.Register(tools.NewFileWriteTool([]string{workspaceRoot}, 0))
	registry.Register(tools.NewLsTool([]string{workspaceRoot}))
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:        llmStub,
		ResponderLLM:      llmStub,
		Registry:          registry,
		Executor:          executor,
		Store:             store,
		Secret:            []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds:  6,
		KnowledgeResolver: resolver,
	})
	task := &Task{
		ID:          "grounded-knowledge-step-task",
		UserID:      "u1",
		Goal:        "reuse step knowledge across grounded planner rounds",
		GroundState: NewGroundTruthState(),
	}
	step := PlanStep{Index: 0, Description: "create demo.txt and verify it", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusGrounded {
		t.Fatalf("grounding_status=%q, want %q", result.GroundingStatus, GroundingStatusGrounded)
	}
	if resolver.called != 1 {
		t.Fatalf("resolver calls = %d, want 1", resolver.called)
	}
	if resolver.lastReq.Stage != knowledge.LoopContextStageExecution {
		t.Fatalf("resolver stage = %q, want execution", resolver.lastReq.Stage)
	}
	if len(llmStub.plannerRequests) < 4 {
		t.Fatalf("planner requests = %d, want multiple rounds", len(llmStub.plannerRequests))
	}
	for i, req := range llmStub.plannerRequests {
		if len(req.Messages) < 2 {
			t.Fatalf("planner request %d missing user message: %#v", i, req.Messages)
		}
		for _, want := range []string{"Relevant knowledge:", "<loop_knowledge>", "demo-step-evidence"} {
			if !strings.Contains(req.Messages[1].Content, want) {
				t.Fatalf("planner request %d missing %q in user prompt: %s", i, want, req.Messages[1].Content)
			}
		}
	}
}

func TestGroundedRuntimeStopsSearchLoopAndRespondsFromGroundedEvidence(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Need the latest docs URL.","next_tool":{"tool":"web_query","args":{"input":"https://platform.openai.com/docs/api-reference/responses","query":"OpenAI Responses API documentation latest 2025","url":"https://platform.openai.com/docs/api-reference/responses"}},"assertions":[]}`,
			`{"status":"continue","reason":"Double-check latest docs.","next_tool":{"tool":"web_query","args":{"input":"https://platform.openai.com/docs/api-reference/responses","query":"OpenAI Responses API documentation 2025 latest platform.openai.com","url":"https://platform.openai.com/docs/api-reference/responses"}},"assertions":[]}`,
			`{"status":"continue","reason":"Confirm one more time.","next_tool":{"tool":"web_query","args":{"input":"https://platform.openai.com/docs/api-reference/responses","query":"site:platform.openai.com/docs responses API","url":"https://platform.openai.com/docs/api-reference/responses"}},"assertions":[]}`,
			`{"status":"continue","reason":"This extra round should never execute.","next_tool":{"tool":"web_query","args":{"input":"https://example.com"}},"assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"The latest Responses API docs were retrieved.","claims":[{"type":"tool_output","tool_call_ids":["__FIRST_TOOL_CALL_ID__"],"excerpt":"Responses | OpenAI API Reference"}]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]any{
		"status":     "ok",
		"mode":       "read",
		"target_url": "https://platform.openai.com/docs/api-reference/responses",
		"final_url":  "https://developers.openai.com/api/reference/resources/responses",
		"title":      "Responses | OpenAI API Reference",
		"content":    "Responses | OpenAI API Reference",
	})
	registry.Register(web)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     llmStub,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-search-loop-task",
		UserID:      "u1",
		Goal:        "find the latest OpenAI Responses API docs",
		GroundState: NewGroundTruthState(),
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "Responses | OpenAI API Reference") {
		t.Fatalf("output=%q, want grounded summary from existing evidence", result.Output)
	}
	if llmStub.plannerIndex != 3 {
		t.Fatalf("planner rounds executed = %d, want 3 before loop cutoff", llmStub.plannerIndex)
	}
	events, err := store.ListRuntimeEvents(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListRuntimeEvents returned error: %v", err)
	}
	foundLoopDetection := false
	for _, event := range events {
		if event.EventType == "loop_detection" {
			foundLoopDetection = true
			break
		}
	}
	if !foundLoopDetection {
		t.Fatal("expected grounded runtime to persist a loop_detection runtime event")
	}
}

func TestGroundedRuntimeEscalatesWebQueryBrowserHintsToBrowserTool(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Try web_query first.","next_tool":{"tool":"web_query","args":{"input":"https://platform.openai.com/docs/api-reference/responses","query":"OpenAI Responses API documentation latest","url":"https://platform.openai.com/docs/api-reference/responses"}},"assertions":[]}`,
			`{"status":"complete","reason":"Browser evidence collected.","assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"Browser fallback reached the latest docs.","claims":[{"type":"tool_output","tool_call_ids":["__SECOND_TOOL_CALL_ID__"],"excerpt":"Responses | OpenAI API Reference"}]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]any{
		"status":      "needs_browser",
		"next_action": "retry_browser",
		"target_url":  "https://platform.openai.com/docs/api-reference/responses",
		"final_url":   "https://platform.openai.com/docs/api-reference/responses",
		"warnings":    []any{map[string]any{"code": "browser_required"}},
	})
	browser := tools.NewMockTool("browser", "mock browser")
	browser.SetResult(map[string]any{
		"status":  "ok",
		"url":     "https://platform.openai.com/docs/api-reference/responses",
		"title":   "Responses | OpenAI API Reference",
		"content": "Responses | OpenAI API Reference",
	})
	registry.Register(web)
	registry.Register(browser)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     llmStub,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-browser-escalation-task",
		UserID:      "u1",
		Goal:        "find the latest OpenAI Responses API docs",
		GroundState: NewGroundTruthState(),
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "Responses | OpenAI API Reference") {
		t.Fatalf("output=%q, want browser-backed grounded evidence", result.Output)
	}
	if len(task.GroundState.Calls) != 2 {
		t.Fatalf("grounded call count = %d, want 2", len(task.GroundState.Calls))
	}
	if got := task.GroundState.Calls["task/grounded-browser-escalation-task/tc/2"].Tool; got != "browser" {
		t.Fatalf("second tool = %q, want browser", got)
	}
	if llmStub.plannerIndex != 2 {
		t.Fatalf("planner rounds executed = %d, want 2 (initial plan + final complete)", llmStub.plannerIndex)
	}
}

func TestGroundedRuntimeEscalatesWrappedWebQueryBrowserHintsToBrowserTool(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Try web_query first.","next_tool":{"tool":"web_query","args":{"input":"https://platform.openai.com/docs/api-reference/responses","query":"OpenAI Responses API documentation latest","url":"https://platform.openai.com/docs/api-reference/responses"}},"assertions":[]}`,
			`{"status":"complete","reason":"Browser evidence collected.","assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"Wrapped browser fallback reached the latest docs.","claims":[{"type":"tool_output","tool_call_ids":["__SECOND_TOOL_CALL_ID__"],"excerpt":"Responses | OpenAI API Reference"}]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]any{
		"tool_name": "web_query",
		"result": `{
			"status":"needs_browser",
			"next_action":"retry_browser",
			"target_url":"https://platform.openai.com/docs/api-reference/responses",
			"final_url":"https://platform.openai.com/docs/api-reference/responses",
			"warnings":[{"code":"browser_required"}]
		}`,
	})
	browser := tools.NewMockTool("browser", "mock browser")
	browser.SetResult(map[string]any{
		"status":  "ok",
		"url":     "https://platform.openai.com/docs/api-reference/responses",
		"title":   "Responses | OpenAI API Reference",
		"content": "Responses | OpenAI API Reference",
	})
	registry.Register(web)
	registry.Register(browser)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     llmStub,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-browser-escalation-wrapped-task",
		UserID:      "u1",
		Goal:        "find the latest OpenAI Responses API docs",
		GroundState: NewGroundTruthState(),
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "Responses | OpenAI API Reference") {
		t.Fatalf("output=%q, want browser-backed grounded evidence", result.Output)
	}
	if len(task.GroundState.Calls) != 2 {
		t.Fatalf("grounded call count = %d, want 2", len(task.GroundState.Calls))
	}
	if got := task.GroundState.Calls["task/grounded-browser-escalation-wrapped-task/tc/2"].Tool; got != "browser" {
		t.Fatalf("second tool = %q, want browser", got)
	}
}

func TestGroundedRuntimeSuppressesBrowserAutoEscalationAfterCanonicalWebSearch(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Try web_query first.","next_tool":{"tool":"web_query","args":{"input":"OpenAI Responses API latest docs","query":"OpenAI Responses API latest docs"}},"assertions":[]}`,
			`{"status":"continue","reason":"Read the discovered docs URL.","next_tool":{"tool":"web_query","args":{"input":"https://platform.openai.com/docs/api-reference/responses","query":"OpenAI Responses API latest docs","url":"https://platform.openai.com/docs/api-reference/responses"}},"assertions":[]}`,
			`{"status":"complete","reason":"Canonical web_search evidence is enough for this gate.","assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"Canonical web_search evidence is enough for this gate.","claims":[{"type":"command_output_excerpt","tool_call_ids":["__FIRST_TOOL_CALL_ID__"],"excerpt":"format: xml"}]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "format: xml\nresult: <web_search><result>ok</result></web_search>",
	})
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]any{
		"status":      "needs_browser",
		"next_action": "retry_browser",
		"target_url":  "https://platform.openai.com/docs/api-reference/responses",
		"final_url":   "https://platform.openai.com/docs/api-reference/responses",
		"warnings":    []any{map[string]any{"code": "browser_required"}},
	})
	browser := tools.NewMockTool("browser", "mock browser")
	browser.SetResult(map[string]any{
		"status":  "ok",
		"url":     "https://platform.openai.com/docs/api-reference/responses",
		"title":   "Responses | OpenAI API Reference",
		"content": "Responses | OpenAI API Reference",
	})
	registry.Register(execTool)
	registry.Register(web)
	registry.Register(browser)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     llmStub,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-canonical-web-search-suppresses-browser-task",
		UserID:      "u1",
		Goal:        "find the latest OpenAI Responses API docs",
		GroundState: NewGroundTruthState(),
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_search",
				"expected_cli_action": "blue web_search",
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
			"group_input": map[string]any{
				"query": "OpenAI Responses API latest docs",
			},
		},
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, `command excerpt="format: xml"`) {
		t.Fatalf("output=%q, want grounded command evidence from canonical web_search", result.Output)
	}
	if len(task.GroundState.Calls) != 2 {
		t.Fatalf("grounded call count = %d, want 2", len(task.GroundState.Calls))
	}
	if got := task.GroundState.Calls["task/grounded-canonical-web-search-suppresses-browser-task/tc/1"].Tool; got != "bash" {
		t.Fatalf("first tool = %q, want bash canonical route", got)
	}
	if got := task.GroundState.Calls["task/grounded-canonical-web-search-suppresses-browser-task/tc/2"].Tool; got != "web_query" {
		t.Fatalf("second tool = %q, want web_query", got)
	}
	for _, call := range task.GroundState.Calls {
		if call.Tool == "browser" {
			t.Fatal("browser should not be auto-escalated after canonical web_search succeeded")
		}
	}
	if llmStub.plannerIndex != 3 {
		t.Fatalf("planner rounds executed = %d, want 3", llmStub.plannerIndex)
	}
}

func TestGroundedRuntimeCompletesExecutionContractAfterGroundedWebEvidence(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Try web_query first.","next_tool":{"tool":"web_query","args":{"input":"OpenAI Responses API latest docs","query":"OpenAI Responses API latest docs"}},"assertions":[]}`,
			`{"status":"continue","reason":"Read the discovered docs URL.","next_tool":{"tool":"web_query","args":{"input":"https://platform.openai.com/docs/api-reference/responses","query":"OpenAI Responses API latest docs","url":"https://platform.openai.com/docs/api-reference/responses"}},"assertions":[]}`,
			`{"status":"continue","reason":"Open the docs in browser.","next_tool":{"tool":"browser","args":{"action":"navigate","url":"https://developers.openai.com/api/reference/resources/responses"}},"assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"Canonical web_search evidence is enough for this execution gate.","claims":[{"type":"command_output_excerpt","tool_call_ids":["__FIRST_TOOL_CALL_ID__"],"excerpt":"format: xml"}]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "format: xml\nresult: <web_search><result>ok</result></web_search>",
	})
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]any{
		"status":    "ok",
		"mode":      "read",
		"final_url": "https://developers.openai.com/api/reference/resources/responses",
		"title":     "Responses | OpenAI API Reference",
		"content":   "Responses | OpenAI API Reference",
	})
	browser := tools.NewMockTool("browser", "mock browser")
	browser.SetResult(map[string]any{
		"status":  "ok",
		"title":   "browser should not run",
		"content": "browser should not run",
	})
	registry.Register(execTool)
	registry.Register(web)
	registry.Register(browser)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     llmStub,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-execution-contract-web-evidence-task",
		UserID:      "u1",
		Goal:        "搜索 OpenAI Responses API 的最新文档。",
		GroundState: NewGroundTruthState(),
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_search",
				"expected_cli_action": "blue web_search",
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
			"group_input": map[string]any{
				"query": "搜索 OpenAI Responses API 的最新文档。",
			},
			"task_success_criteria": []any{"evidence_tool_used", "planner_memory_skipped"},
			"harness_contract": map[string]any{
				"required_observations": []any{"evidence_tool_used", "planner_memory_skipped"},
			},
		},
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, `command excerpt="format: xml"`) {
		t.Fatalf("output=%q, want canonical web_search command evidence", result.Output)
	}
	if len(task.GroundState.Calls) != 1 {
		t.Fatalf("grounded call count = %d, want 1", len(task.GroundState.Calls))
	}
	if got := task.GroundState.Calls["task/grounded-execution-contract-web-evidence-task/tc/1"].Tool; got != "bash" {
		t.Fatalf("first tool = %q, want bash canonical route", got)
	}
	for _, call := range task.GroundState.Calls {
		if call.Tool == "web_query" || call.Tool == "browser" {
			t.Fatalf("unexpected follow-up tool after canonical web_search evidence: %q", call.Tool)
		}
	}
	if llmStub.plannerIndex != 1 {
		t.Fatalf("planner rounds executed = %d, want 1", llmStub.plannerIndex)
	}
}

func TestGroundedRuntimeForcesCanonicalCLIActionBeforeAlternateRoute(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Use the direct web tool first.","next_tool":{"tool":"web_query","args":{"input":"OpenAI Responses API latest docs","query":"OpenAI Responses API latest docs"}},"assertions":[]}`,
			`{"status":"complete","reason":"Canonical CLI output captured.","assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"The canonical CLI route was executed.","claims":[{"type":"tool_output","tool_call_ids":["__FIRST_TOOL_CALL_ID__"],"excerpt":"canonical web query route executed"}]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "canonical web query route executed",
	})
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]any{
		"status":  "ok",
		"content": "direct web tool should not be used first",
	})
	registry.Register(execTool)
	registry.Register(web)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     llmStub,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-canonical-cli-task",
		UserID:      "u1",
		Goal:        "find the latest OpenAI Responses API docs",
		GroundState: NewGroundTruthState(),
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_search",
				"expected_cli_action": `blue web_query input="OpenAI Responses API latest docs"`,
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
		},
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "canonical web query route executed") {
		t.Fatalf("output=%q, want canonical CLI evidence", result.Output)
	}
	if len(task.GroundState.Calls) != 1 {
		t.Fatalf("grounded call count = %d, want 1", len(task.GroundState.Calls))
	}
	if got := task.GroundState.Calls["task/grounded-canonical-cli-task/tc/1"].Tool; got != "bash" {
		t.Fatalf("first tool = %q, want bash/exec canonical route", got)
	}
	cmdFact, ok := task.GroundState.Commands["task/grounded-canonical-cli-task/tc/1"]
	if !ok {
		t.Fatal("expected canonical CLI command to be recorded in command history")
	}
	if cmdFact.Command != `blue web_query input="OpenAI Responses API latest docs"` {
		t.Fatalf("command=%q, want canonical CLI action", cmdFact.Command)
	}
	if llmStub.plannerIndex != 2 {
		t.Fatalf("planner rounds executed = %d, want 2", llmStub.plannerIndex)
	}
}

func TestGroundedRuntimeFallsBackToCanonicalCLIWhenPlannerReturnsEmptyContent(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			"",
			`{"status":"complete","reason":"Canonical CLI output captured.","assertions":[]}`,
		},
		responderResponses: []string{
			`{"summary":"The canonical CLI route was executed.","claims":[{"type":"tool_output","tool_call_ids":["__FIRST_TOOL_CALL_ID__"],"excerpt":"canonical web query route executed after planner fallback"}]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "canonical web query route executed after planner fallback",
	})
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]any{
		"status":  "ok",
		"content": "direct web tool should not be used first",
	})
	registry.Register(execTool)
	registry.Register(web)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     llmStub,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-canonical-cli-fallback-task",
		UserID:      "u1",
		Goal:        "find the latest OpenAI Responses API docs",
		GroundState: NewGroundTruthState(),
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_query",
				"expected_cli_action": `blue web_query input="OpenAI Responses API latest docs"`,
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
		},
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "canonical web query route executed after planner fallback") {
		t.Fatalf("output=%q, want canonical CLI fallback evidence", result.Output)
	}
	if len(task.GroundState.Calls) != 1 {
		t.Fatalf("grounded call count = %d, want 1", len(task.GroundState.Calls))
	}
	if got := task.GroundState.Calls["task/grounded-canonical-cli-fallback-task/tc/1"].Tool; got != "bash" {
		t.Fatalf("first tool = %q, want bash/exec canonical route", got)
	}
	cmdFact, ok := task.GroundState.Commands["task/grounded-canonical-cli-fallback-task/tc/1"]
	if !ok {
		t.Fatal("expected canonical CLI fallback command to be recorded in command history")
	}
	if cmdFact.Command != `blue web_query input="OpenAI Responses API latest docs"` {
		t.Fatalf("command=%q, want canonical CLI fallback action", cmdFact.Command)
	}
	if llmStub.plannerIndex != 2 {
		t.Fatalf("planner rounds executed = %d, want 2", llmStub.plannerIndex)
	}
}

func TestGroundedRuntimeUsesDeterministicStructuredWebEvidenceForCanonicalExec(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Run the canonical web query route.","next_tool":{"tool":"exec","args":{"command":"blue web_query input=\"OpenAI Responses API latest docs\""}},"assertions":[]}`,
			`{"status":"complete","reason":"Structured web evidence was collected.","assertions":[]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "status: ok",
		"data": map[string]any{
			"status":    "ok",
			"final_url": "https://developers.openai.com/api/reference/resources/responses",
			"title":     "Responses | OpenAI API Reference",
			"content":   "Responses | OpenAI API Reference\nBuild stateful interactions with the Responses API.",
		},
	})
	registry.Register(execTool)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     nil,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-deterministic-canonical-exec-task",
		UserID:      "u1",
		Goal:        "find the latest OpenAI Responses API docs",
		GroundState: NewGroundTruthState(),
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusGrounded {
		t.Fatalf("grounding_status=%q, want %q", result.GroundingStatus, GroundingStatusGrounded)
	}
	for _, want := range []string{
		"Responses | OpenAI API Reference",
		"https://developers.openai.com/api/reference/resources/responses",
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("expected output to contain %q, got %q", want, result.Output)
		}
	}
	if len(result.VerificationErrors) != 0 {
		t.Fatalf("verification_errors=%v, want none", result.VerificationErrors)
	}
	if llmStub.responderIndex != 0 {
		t.Fatalf("expected responder LLM to be skipped, got responderIndex=%d", llmStub.responderIndex)
	}
}

func TestGroundedRuntimeCompletesExecutionContractAfterCanonicalWebQueryEvidence(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Run the canonical web query route.","next_tool":{"tool":"exec","args":{"command":"blue web_query input=\"OpenAI Responses API latest docs\""}},"assertions":[]}`,
			`{"status":"continue","reason":"This extra round should not execute.","next_tool":{"tool":"web_query","args":{"input":"https://example.com"}},"assertions":[]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "status: ok",
		"data": map[string]any{
			"status":    "ok",
			"final_url": "https://developers.openai.com/api/reference/resources/responses",
			"title":     "Responses | OpenAI API Reference",
			"content":   "Responses | OpenAI API Reference\nBuild stateful interactions with the Responses API.",
		},
	})
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]any{
		"status":  "ok",
		"title":   "web query should not run",
		"content": "web query should not run",
	})
	registry.Register(execTool)
	registry.Register(web)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     nil,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-execution-contract-web-query-evidence-task",
		UserID:      "u1",
		Goal:        "find the latest OpenAI Responses API docs",
		GroundState: NewGroundTruthState(),
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_query",
				"expected_cli_action": `blue web_query input="OpenAI Responses API latest docs"`,
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
			"task_success_criteria": []any{"evidence_tool_used"},
			"harness_contract": map[string]any{
				"required_observations": []any{"evidence_tool_used"},
			},
		},
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	for _, want := range []string{
		"Responses | OpenAI API Reference",
		"https://developers.openai.com/api/reference/resources/responses",
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("expected output to contain %q, got %q", want, result.Output)
		}
	}
	if len(task.GroundState.Calls) != 1 {
		t.Fatalf("grounded call count = %d, want 1", len(task.GroundState.Calls))
	}
	if llmStub.plannerIndex != 1 {
		t.Fatalf("planner rounds executed = %d, want 1", llmStub.plannerIndex)
	}
}

func TestGroundedRuntimeStopsBeforeScratchpadWriteAfterLocalizedCanonicalWebQueryEvidence(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			`{"status":"continue","reason":"Run the canonical web query route.","next_tool":{"tool":"exec","args":{"command":"blue web_query query=\"Wyszukaj najnowsza dokumentacje OpenAI Responses API.\""}},"assertions":[]}`,
			`{"status":"continue","reason":"This scratchpad write should never run.","next_tool":{"tool":"write","args":{"path":".blue/scratchpad/shared/openai-responses-api.md","content":"unexpected write"}},"assertions":[]}`,
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "status: ok",
		"data": map[string]any{
			"target_url": "https://developers.openai.com/api/reference/resources/responses",
			"title":      "Responses | OpenAI API Reference",
			"content":    "Responses | OpenAI API Reference\nCreate a model response\nPOST /responses",
		},
	})
	writeTool := tools.NewMockTool("write", "mock write")
	writeTool.SetResult(map[string]any{
		"success": true,
		"path":    ".blue/scratchpad/shared/openai-responses-api.md",
		"size":    16,
	})
	registry.Register(execTool)
	registry.Register(writeTool)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     nil,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-localized-web-query-stop-task",
		UserID:      "u1",
		Goal:        "Wyszukaj najnowsza dokumentacje OpenAI Responses API.",
		GroundState: NewGroundTruthState(),
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_query",
				"expected_cli_action": "blue web_query",
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
			"group_input": map[string]any{
				"query": "Wyszukaj najnowsza dokumentacje OpenAI Responses API.",
			},
			"harness_contract": map[string]any{
				"required_observations": []any{"evidence_tool_used"},
			},
		},
	}
	step := PlanStep{Index: 0, Description: "retrieve the latest OpenAI Responses API docs", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusGrounded {
		t.Fatalf("grounding_status=%q, want %q", result.GroundingStatus, GroundingStatusGrounded)
	}
	if len(task.GroundState.Calls) != 1 {
		t.Fatalf("grounded call count = %d, want 1", len(task.GroundState.Calls))
	}
	if _, ok := task.GroundState.Calls["task/grounded-localized-web-query-stop-task/tc/2"]; ok {
		t.Fatal("unexpected scratchpad write after localized canonical web_query evidence")
	}
	if llmStub.plannerIndex != 1 {
		t.Fatalf("planner rounds executed = %d, want 1", llmStub.plannerIndex)
	}
}

func TestGroundedRuntimeCompletesExecutionContractAfterCanonicalAnalyzeEvidence(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			"",
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "answer: Analysis completed for Summarize and extract the key points.",
		"data": map[string]any{
			"answer":      "Analysis completed for Summarize and extract the key points.",
			"message":     "Analysis ready: Summarize and extract the key points",
			"output_mode": "inline",
			"topic":       "Summarize and extract the key points",
		},
	})
	registry.Register(execTool)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     nil,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-deterministic-canonical-analyze-task",
		UserID:      "u1",
		Goal:        "Summarize https://example.com/blog and extract the key points.",
		GroundState: NewGroundTruthState(),
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"expected_cli_action": "blue analyze",
				"enforce_cli_route":   true,
				"gate_type":           "execution_equivalence",
			},
			"group_input": map[string]any{
				"query": "Summarize https://example.com/blog and extract the key points.",
			},
		},
	}
	step := PlanStep{Index: 0, Description: "collect and analyze the referenced public URL", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusGrounded {
		t.Fatalf("grounding_status=%q, want %q", result.GroundingStatus, GroundingStatusGrounded)
	}
	if !strings.Contains(result.Output, "Analysis completed for Summarize and extract the key points.") {
		t.Fatalf("expected output to contain deterministic analyze answer, got %q", result.Output)
	}
	if len(result.VerificationErrors) != 0 {
		t.Fatalf("verification_errors=%v, want none", result.VerificationErrors)
	}
	if llmStub.responderIndex != 0 {
		t.Fatalf("expected responder LLM to be skipped, got responderIndex=%d", llmStub.responderIndex)
	}
}

func TestGroundedRuntimeCompletesExecutionContractAfterCanonicalReminderEvidence(t *testing.T) {
	llmStub := &groundedScriptLLM{
		plannerResponses: []string{
			"",
		},
	}

	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    `reminder: {"id":"push_demo","message":"send weekly report","fire_at":"2026-04-02T09:00:00+08:00","session_id":"batch1-reminder-en-us-localized_route","status":"pending"}`,
		"data": map[string]any{
			"message":  "Reminder set: send weekly report - 2026-04-02 09:00",
			"reminder": `{"id":"push_demo","message":"send weekly report","fire_at":"2026-04-02T09:00:00+08:00","session_id":"batch1-reminder-en-us-localized_route","status":"pending"}`,
			"success":  "true",
		},
	})
	registry.Register(execTool)
	executor := tools.NewExecutor(registry)
	rt := NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmStub,
		ResponderLLM:     nil,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		Secret:           []byte("grounded-runtime-test-secret-123456"),
		MaxPlannerRounds: 6,
	})
	task := &Task{
		ID:          "grounded-deterministic-canonical-reminder-task",
		UserID:      "u1",
		Goal:        "Schedule a reminder for tomorrow at 9 AM to send the weekly report.",
		GroundState: NewGroundTruthState(),
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"expected_cli_action": "blue reminder add",
				"enforce_cli_route":   true,
				"gate_type":           "execution_equivalence",
			},
			"group_input": map[string]any{
				"query":      "Schedule a reminder for tomorrow at 9 AM to send the weekly report.",
				"session_id": "batch1-reminder-en-us-localized_route",
			},
			"harness_contract": map[string]any{
				"required_observations": []any{"session_context_propagated"},
			},
		},
	}
	step := PlanStep{Index: 0, Description: "schedule the reminder through the canonical CLI route", Status: StepStatusRunning}

	result, err := rt.ExecuteStep(context.Background(), task, step, []PlanStep{step}, 6)
	if err != nil {
		t.Fatalf("ExecuteStep returned unexpected error: %v", err)
	}
	if result.GroundingStatus != GroundingStatusGrounded {
		t.Fatalf("grounding_status=%q, want %q", result.GroundingStatus, GroundingStatusGrounded)
	}
	for _, want := range []string{
		"Reminder set: send weekly report - 2026-04-02 09:00",
		"batch1-reminder-en-us-localized_route",
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("expected output to contain %q, got %q", want, result.Output)
		}
	}
	if len(result.VerificationErrors) != 0 {
		t.Fatalf("verification_errors=%v, want none", result.VerificationErrors)
	}
	if llmStub.responderIndex != 0 {
		t.Fatalf("expected responder LLM to be skipped, got responderIndex=%d", llmStub.responderIndex)
	}
}

func TestGroundedToolCatalogUsesFileReadWriteCanonicalNames(t *testing.T) {
	registry := tools.NewRegistry()
	workspaceRoot := t.TempDir()
	registry.Register(tools.NewFileReadTool([]string{workspaceRoot}, 0))
	registry.Register(tools.NewFileWriteTool([]string{workspaceRoot}, 0))

	catalog := groundedToolCatalog(registry)
	names := make(map[string]llm.Tool, len(catalog))
	for _, tool := range catalog {
		names[tool.Name] = tool
	}
	if _, ok := names["read"]; !ok {
		t.Fatalf("expected read in grounded tool catalog, got=%v", groundedToolCatalogNames(names))
	}
	if _, ok := names["write"]; !ok {
		t.Fatalf("expected write in grounded tool catalog, got=%v", groundedToolCatalogNames(names))
	}
	if _, ok := names["read_file"]; ok {
		t.Fatalf("expected read_file alias to be absent from grounded tool catalog, got=%v", groundedToolCatalogNames(names))
	}
	if _, ok := names["write_file"]; ok {
		t.Fatalf("expected write_file alias to be absent from grounded tool catalog, got=%v", groundedToolCatalogNames(names))
	}

	writeProps, _ := names["write"].Parameters["properties"].(map[string]any)
	if writeProps == nil || writeProps["file_path"] == nil || writeProps["text"] == nil {
		t.Fatalf("expected write schema to expose path/content aliases, got=%v", names["write"].Parameters)
	}
}

func TestGroundedExecutorNormalizesFileToolAliases(t *testing.T) {
	workspaceRoot := t.TempDir()
	registry := tools.NewRegistry()
	registry.Register(tools.NewFileReadTool([]string{workspaceRoot}, 0))
	registry.Register(tools.NewFileWriteTool([]string{workspaceRoot}, 0))
	executor := tools.NewExecutor(registry)
	grounded := NewGroundedExecutor(executor, registry, NewGroundTruthStateStore(), []byte("grounded-runtime-test-secret-123456"), nil)
	task := &Task{ID: "alias-task", UserID: "u1", Goal: "alias test", GroundState: NewGroundTruthState()}

	writeExec, err := grounded.Execute(context.Background(), task, 0, 0, PlannerToolCall{
		Tool: "write_file",
		Args: map[string]any{
			"file_path": "alias.txt",
			"text":      "hello alias",
		},
	})
	if err != nil {
		t.Fatalf("write execute returned error: %v", err)
	}
	if !writeExec.Result.OK {
		t.Fatalf("expected write result ok, got=%+v", writeExec.Result)
	}

	readExec, err := grounded.Execute(context.Background(), task, 1, 0, PlannerToolCall{
		Tool: "read_file",
		Args: map[string]any{
			"file_path": "alias.txt",
		},
	})
	if err != nil {
		t.Fatalf("read execute returned error: %v", err)
	}
	if !readExec.Result.OK {
		t.Fatalf("expected read result ok, got=%+v", readExec.Result)
	}

	payload, ok := readExec.Result.Result.(map[string]any)
	if !ok {
		t.Fatalf("read result type = %T, want map[string]any", readExec.Result.Result)
	}
	if got := payload["content"]; got != "hello alias" {
		t.Fatalf("content = %v, want %q", got, "hello alias")
	}
}

func groundedToolCatalogNames(items map[string]llm.Tool) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestGroundedVerifierRejectsForgedToolResult(t *testing.T) {
	state := NewGroundTruthState()
	call := GroundedToolCall{
		ToolCallID: "task/forged/tc/1",
		TaskID:     "forged",
		Tool:       "ls",
		Args:       map[string]any{"path": "."},
	}
	result := GroundedToolResult{
		ToolCallID: call.ToolCallID,
		Tool:       "ls",
		ExitCode:   0,
		OK:         true,
		Result: map[string]any{
			"base_path": ".",
			"entries":   []any{map[string]any{"path": "real.txt", "type": "file", "size": 4}},
		},
		AuthTag: "forged",
	}
	state.Calls[call.ToolCallID] = call
	state.Results[call.ToolCallID] = result
	state.Files["real.txt"] = GroundedFileFact{Path: "real.txt", Exists: true, Size: 4}

	verifier := NewGroundedVerifier(func(result GroundedToolResult) bool { return false })
	decision := verifier.Verify(state, &GroundedResponse{
		Claims: []ResponseClaim{{
			Type:        ClaimTypeFSExists,
			ToolCallIDs: []string{call.ToolCallID},
			Path:        "real.txt",
			Value:       "true",
		}},
	})
	if decision.Valid {
		t.Fatal("expected forged tool result to be rejected")
	}
	if strings.TrimSpace(decision.Output) != "unknown" {
		t.Fatalf("output=%q, want unknown", decision.Output)
	}
}

func TestGroundedAssertionsFailWhenEvidenceMissing(t *testing.T) {
	state := NewGroundTruthState()
	failures := EvaluateAssertions(state, []PlannerAssertion{
		{Type: "file_exists", Path: "missing.txt"},
		{Type: "tool_called", Tool: "ls"},
	})
	if len(failures) != 2 {
		t.Fatalf("expected 2 assertion failures, got %d: %v", len(failures), failures)
	}
}
