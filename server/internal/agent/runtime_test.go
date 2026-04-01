package agent

import (
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type recursiveAgentPayload struct {
	Name string                 `json:"name"`
	Self *recursiveAgentPayload `json:"self,omitempty"`
}

func TestCanTransition(t *testing.T) {
	if !canTransition("", RuntimeStateIntake) {
		t.Fatal("empty -> INTAKE should be allowed")
	}
	if !canTransition(RuntimeStatePlan, RuntimeStateExecute) {
		t.Fatal("PLAN -> EXECUTE should be allowed")
	}
	if !canTransition(RuntimeStateVerify, RuntimeStateReflect) {
		t.Fatal("VERIFY -> REFLECT should be allowed")
	}
	if canTransition(RuntimeStateDone, RuntimeStateExecute) {
		t.Fatal("DONE -> EXECUTE should not be allowed")
	}
}

func TestParseRuntimePlan_Object(t *testing.T) {
	content := `{
		"goal":"Build API",
		"subtasks":[{"description":"Create handler"},{"description":"Add tests"}],
		"requires_confirmation":["deploy to production"],
		"success_criteria":["tests pass"],
		"fallback_plan":["retry once"]
	}`
	plan, err := parseRuntimePlan(content)
	if err != nil {
		t.Fatalf("parseRuntimePlan error: %v", err)
	}
	if len(plan.Subtasks) != 2 {
		t.Fatalf("subtasks len=%d, want 2", len(plan.Subtasks))
	}
	if len(plan.RequiresConfirmation) != 1 {
		t.Fatalf("requires_confirmation len=%d, want 1", len(plan.RequiresConfirmation))
	}
}

func TestParseRuntimePlan_ArrayFallback(t *testing.T) {
	content := `[{"description":"step one"},{"description":"step two"}]`
	plan, err := parseRuntimePlan(content)
	if err != nil {
		t.Fatalf("parseRuntimePlan error: %v", err)
	}
	if len(plan.Subtasks) != 2 {
		t.Fatalf("subtasks len=%d, want 2", len(plan.Subtasks))
	}
	if len(plan.SuccessCriteria) == 0 {
		t.Fatal("default success_criteria should be populated")
	}
}

func TestClassifyCapability_ExecRisk(t *testing.T) {
	cap := classifyCapability("exec", `{"command":"sudo rm -rf /tmp/foo"}`)
	if cap.Kind != CapabilityKindTool {
		t.Fatalf("kind=%s, want tool", cap.Kind)
	}
	if cap.RiskLevel != "high" && cap.RiskLevel != "critical" {
		t.Fatalf("risk_level=%s, want high/critical", cap.RiskLevel)
	}
	if cap.Idempotent {
		t.Fatal("high-risk exec should not be idempotent")
	}
}

func TestClassifyCapability_SkillViaExec(t *testing.T) {
	cap := classifyCapability("exec", `{"command":"blue web_search query=llm"}`)
	if cap.Kind != CapabilityKindSkill {
		t.Fatalf("kind=%s, want skill", cap.Kind)
	}
}

func TestShouldRequireConfirm_UsesWholeWordDangerCues(t *testing.T) {
	lowRisk := CapabilityInfo{Name: "web_query", RiskLevel: "low", Idempotent: true}
	if shouldRequireConfirm(lowRisk, "Read the dropdown navigation docs and published examples.") {
		t.Fatal("embedded words like dropdown/published should not trigger confirmation")
	}
	if !shouldRequireConfirm(lowRisk, "Review the migration before we publish to production.") {
		t.Fatal("explicit dangerous action cues should still trigger confirmation")
	}
}

func TestPlanNeedsConfirmation_UsesWholeWordDangerCues(t *testing.T) {
	plan := runtimePlan{Subtasks: []string{"Inspect the dropdown menu docs", "Summarize the published changelog"}}
	if planNeedsConfirmation(plan) {
		t.Fatal("embedded words should not trigger plan confirmation")
	}
	plan.Subtasks = append(plan.Subtasks, "Publish the production release notes")
	if !planNeedsConfirmation(plan) {
		t.Fatal("explicit publish/production cue should trigger confirmation")
	}
}

func TestBuildGroundedPlannerUserPrompt_IncludesRoutingContract(t *testing.T) {
	out := buildGroundedPlannerUserPrompt(PlannerInput{
		Goal:            "inspect the workspace readme",
		PlanSummary:     "1. analyze the README",
		Step:            PlanStep{Description: "use the canonical analyze route"},
		PlannerRound:    1,
		MaxRounds:       4,
		RoutingContract: "- Primary route: analyze\n- Canonical CLI action: blue analyze",
	})
	if !strings.Contains(out, "Execution routing contract:") {
		t.Fatalf("expected grounded planner prompt to include routing contract header, got: %s", out)
	}
	if !strings.Contains(out, "Canonical CLI action: blue analyze") {
		t.Fatalf("expected grounded planner prompt to include canonical CLI action, got: %s", out)
	}
}

func TestBuildGroundedPlannerSystemPrompt_PrefersCanonicalCLIAction(t *testing.T) {
	out := buildGroundedPlannerSystemPrompt([]llm.Tool{{Name: "exec", Description: "run shell commands"}})
	if !strings.Contains(out, "canonical CLI action first") {
		t.Fatalf("expected grounded planner system prompt to mention canonical CLI precedence, got: %s", out)
	}
}

func TestBuildGroundStateSummary_IncludesRecentToolResults(t *testing.T) {
	state := NewGroundTruthState()
	state.Calls["task/demo/tc/2"] = GroundedToolCall{
		ToolCallID: "task/demo/tc/2",
		Tool:       "web_query",
	}
	state.Results["task/demo/tc/2"] = GroundedToolResult{
		ToolCallID: "task/demo/tc/2",
		Tool:       "web_query",
		OK:         true,
		Result: map[string]any{
			"status":    "ok",
			"title":     "Responses | OpenAI API Reference",
			"final_url": "https://developers.openai.com/api/reference/resources/responses",
		},
	}

	out := BuildGroundStateSummary(state)
	for _, want := range []string{
		"Tool results:",
		"id=task/demo/tc/2 web_query ok",
		`title="Responses | OpenAI API Reference"`,
		"url=https://developers.openai.com/api/reference/resources/responses",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected ground state summary to include %q, got: %s", want, out)
		}
	}
}

func TestGroundedCLICommandMatches_PrefixArgs(t *testing.T) {
	if !groundedCLICommandMatches(`blue web_query input="latest docs"`, "blue web_query") {
		t.Fatal("expected canonical CLI matcher to accept appended arguments")
	}
	if groundedCLICommandMatches("blue analyze", "blue web_query") {
		t.Fatal("different canonical CLI actions should not match")
	}
}

func TestGroundedCanonicalCLICommand_ExpandsWebSearchQueryFromTask(t *testing.T) {
	task := &Task{
		Goal: "Search the latest Responses API docs",
		Metadata: map[string]any{
			"group_input": map[string]any{
				"query": "OpenAI Responses API latest docs",
			},
		},
	}
	got := groundedCanonicalCLICommand(task, "blue web_search", nil)
	if got != `blue web_search query="OpenAI Responses API latest docs"` {
		t.Fatalf("command = %q", got)
	}
}

func TestGroundedCanonicalCLICommand_PrefersPlannerDecisionQuery(t *testing.T) {
	task := &Task{
		Goal: "Search the latest Responses API docs",
		Metadata: map[string]any{
			"group_input": map[string]any{
				"query": "搜索 OpenAI Responses API 的最新文档。",
			},
		},
	}
	decision := &PlannerDecision{
		NextTool: &PlannerToolCall{
			Tool: "web_query",
			Args: map[string]any{
				"query": "OpenAI Responses API documentation site:platform.openai.com",
			},
		},
	}
	got := groundedCanonicalCLICommand(task, "blue web_query", decision)
	if got != `blue web_query query="OpenAI Responses API documentation site:platform.openai.com"` {
		t.Fatalf("command = %q", got)
	}
}

func TestGroundedCanonicalCLICommand_UsesAnalyzeTopicArgument(t *testing.T) {
	task := &Task{
		Goal: "Inspect the workspace README",
		Metadata: map[string]any{
			"group_input": map[string]any{
				"goal": "看下 workspace 里的 README.md，给出当前 tool->skill cutover 迁移状态摘要。",
			},
		},
	}
	decision := &PlannerDecision{
		NextTool: &PlannerToolCall{
			Tool: "analyze",
			Args: map[string]any{
				"topic": "workspace readme",
			},
		},
	}
	got := groundedCanonicalCLICommand(task, "blue analyze", decision)
	if got != `blue analyze topic="workspace readme"` {
		t.Fatalf("command = %q", got)
	}
}

func TestGroundedCanonicalCLICommand_ExtractsAnalyzeURLFromMultilingualTaskQuery(t *testing.T) {
	task := &Task{
		Goal: "Resumeix https://example.com/blog i extreu-ne els punts clau.",
		Metadata: map[string]any{
			"group_input": map[string]any{
				"query": "Resumeix https://example.com/blog i extreu-ne els punts clau.",
			},
		},
	}
	decision := &PlannerDecision{
		NextTool: &PlannerToolCall{
			Tool: "analyze",
			Args: map[string]any{
				"topic": "Resumeix https://example.com/blog i extreu-ne els punts clau.",
			},
		},
	}
	got := groundedCanonicalCLICommand(task, "blue analyze", decision)
	if got != `blue analyze topic="Resumeix i extreu-ne els punts clau" url="https://example.com/blog"` {
		t.Fatalf("command = %q", got)
	}
}

func TestGroundedCanonicalCLICommand_ExtractsAnalyzeURLWhenQueryStartsWithURL(t *testing.T) {
	task := &Task{
		Goal: "https://example.com/blog を要約して主要なポイントを抽出して。",
		Metadata: map[string]any{
			"group_input": map[string]any{
				"query": "https://example.com/blog を要約して主要なポイントを抽出して。",
			},
		},
	}
	got := groundedCanonicalCLICommand(task, "blue analyze", nil)
	if got != `blue analyze topic="を要約して主要なポイントを抽出して" url="https://example.com/blog"` {
		t.Fatalf("command = %q", got)
	}
}

func TestGroundedCanonicalCLICommand_PrefersPlannerAnalyzeURLArgument(t *testing.T) {
	task := &Task{
		Goal: "Inspect the linked release page",
		Metadata: map[string]any{
			"group_input": map[string]any{
				"query": "Inspect the linked release page",
			},
		},
	}
	decision := &PlannerDecision{
		NextTool: &PlannerToolCall{
			Tool: "analyze",
			Args: map[string]any{
				"topic": "release page summary",
				"url":   "https://example.com/releases/latest",
			},
		},
	}
	got := groundedCanonicalCLICommand(task, "blue analyze", decision)
	if got != `blue analyze topic="release page summary" url="https://example.com/releases/latest"` {
		t.Fatalf("command = %q", got)
	}
}

func TestShouldStopAfterGroundedExecutionEvidence_WebQueryCanonicalCLI(t *testing.T) {
	state := NewGroundTruthState()
	callID := "task/demo/tc/1"
	state.Calls[callID] = GroundedToolCall{
		ToolCallID: callID,
		Tool:       "bash",
		Args: map[string]any{
			"command": `blue web_query query="OpenAI Responses API documentation site:platform.openai.com"`,
		},
	}
	state.Results[callID] = GroundedToolResult{
		ToolCallID: callID,
		Tool:       "bash",
		ExitCode:   0,
		OK:         true,
		Result: map[string]any{
			"exit_code": 0,
			"stdout":    "input: OpenAI Responses API documentation site:platform.openai.com\ncontent: Responses | OpenAI API Reference",
		},
	}
	state.Commands[callID] = GroundedCommandFact{
		ToolCallID: callID,
		Tool:       "bash",
		Command:    `blue web_query query="OpenAI Responses API documentation site:platform.openai.com"`,
		ExitCode:   0,
	}

	task := &Task{
		Goal:        "Search the latest OpenAI Responses API documentation.",
		GroundState: state,
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_query",
				"expected_cli_action": "blue web_query",
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
			"task_success_criteria": []any{"evidence_tool_used"},
			"harness_contract": map[string]any{
				"required_observations": []any{"evidence_tool_used"},
			},
		},
	}

	if !shouldStopAfterGroundedExecutionEvidence(task) {
		t.Fatal("expected execution gate to stop after canonical web_query evidence is already present")
	}
}

func TestGroundedVerifyTask_ExecutionGateAllowsSkippedExpansionSteps(t *testing.T) {
	callID := "task/demo/tc/1"
	state := NewGroundTruthState()
	state.Calls[callID] = GroundedToolCall{
		ToolCallID: callID,
		Tool:       "bash",
		Args: map[string]any{
			"command": `blue web_query query="OpenAI Responses API documentation site:platform.openai.com"`,
		},
	}
	state.Results[callID] = GroundedToolResult{
		ToolCallID: callID,
		Tool:       "bash",
		ExitCode:   0,
		OK:         true,
		Result: map[string]any{
			"exit_code": 0,
			"stdout":    "final_url: https://developers.openai.com/api/reference/responses/overview\ncontent: Responses Overview | OpenAI API Reference",
		},
	}
	state.Commands[callID] = GroundedCommandFact{
		ToolCallID: callID,
		Tool:       "bash",
		Command:    `blue web_query query="OpenAI Responses API documentation site:platform.openai.com"`,
		ExitCode:   0,
	}

	task := &Task{
		Goal:            "Search the latest OpenAI Responses API documentation.",
		GroundState:     state,
		GroundingStatus: GroundingStatusGrounded,
		SuccessCriteria: []string{"evidence_tool_used"},
		Plan: []PlanStep{
			{Index: 0, Description: "Run the canonical web_query route.", Status: StepStatusCompleted},
			{Index: 1, Description: "Expand with extra optional follow-up.", Status: StepStatusSkipped},
		},
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_query",
				"expected_cli_action": "blue web_query",
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
			"required_observations": []any{"evidence_tool_used"},
		},
	}

	result, _, err := (&GroundedRuntime{}).VerifyTask(task)
	if err != nil {
		t.Fatalf("VerifyTask returned error: %v", err)
	}
	if result == nil || result.Status != "pass" {
		t.Fatalf("verification status = %#v", result)
	}
}

func TestNormalizeToolResultValueGuardsRecursivePayloads(t *testing.T) {
	payload := &recursiveAgentPayload{Name: "root"}
	payload.Self = payload

	got := normalizeToolResultValue(payload)
	asMap, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("normalizeToolResultValue() = %#v, want map", got)
	}
	if asMap["name"] != "root" {
		t.Fatalf("name = %v, want root", asMap["name"])
	}
	if asMap["self"] != "[circular payload omitted]" {
		t.Fatalf("self = %v, want circular marker", asMap["self"])
	}
}
