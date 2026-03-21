package agent

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type groundedScriptLLM struct {
	plannerResponses   []string
	responderResponses []string
	plannerIndex       int
	responderIndex     int
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
			`{"status":"continue","reason":"Create the file.","next_tool":{"tool":"file_write","args":{"path":"demo.txt","content":"hello"}},"assertions":[]}`,
			`{"status":"continue","reason":"List the directory.","next_tool":{"tool":"ls","args":{"path":".","max_depth":1}},"assertions":[]}`,
			`{"status":"continue","reason":"Read the file.","next_tool":{"tool":"file_read","args":{"path":"demo.txt"}},"assertions":[]}`,
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
	if _, ok := names["file_read"]; !ok {
		t.Fatalf("expected file_read in grounded tool catalog, got=%v", groundedToolCatalogNames(names))
	}
	if _, ok := names["file_write"]; !ok {
		t.Fatalf("expected file_write in grounded tool catalog, got=%v", groundedToolCatalogNames(names))
	}
	if _, ok := names["read_file"]; ok {
		t.Fatalf("expected read_file alias to be absent from grounded tool catalog, got=%v", groundedToolCatalogNames(names))
	}
	if _, ok := names["write_file"]; ok {
		t.Fatalf("expected write_file alias to be absent from grounded tool catalog, got=%v", groundedToolCatalogNames(names))
	}

	writeProps, _ := names["file_write"].Parameters["properties"].(map[string]any)
	if writeProps == nil || writeProps["file_path"] == nil || writeProps["text"] == nil {
		t.Fatalf("expected file_write schema to expose path/content aliases, got=%v", names["file_write"].Parameters)
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
