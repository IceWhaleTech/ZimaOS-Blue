package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestBuildStepExecutionSystemPrompt_IncludesCodingDefaults(t *testing.T) {
	out := buildStepExecutionSystemPrompt(nil)

	required := []string{
		"superpowers and ui-ux-pro-max-skill",
		"If stack preferences are unclear, ask once and remember them for future steps.",
		"Default stack when not specified: backend Go, frontend React, mobile React Native, client Electron.",
		"for example Python or Node.js",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("expected step execution prompt to include %q, got: %s", want, out)
		}
	}
}

func TestBuildStepExecutionSystemPrompt_IncludesCoordinatorGuidanceWhenSubagentsAvailable(t *testing.T) {
	out := buildStepExecutionSystemPrompt([]llm.Tool{{Name: "subagents", Description: "spawn worker agents"}})

	required := []string{
		"bounded independent work",
		"parallel research, verification, or isolated implementation slices",
		"only one worker is writing",
		"research -> synthesis -> implementation -> verification",
		"self-contained worker prompts",
		"based on your findings",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("expected step execution prompt to include %q, got: %s", want, out)
		}
	}
}

func TestBuildTaskRoutingContractContext_IncludesExecutionEquivalenceHints(t *testing.T) {
	out := buildTaskRoutingContractContext(map[string]interface{}{
		"gate_type": "execution_equivalence",
		"routing_contract": map[string]interface{}{
			"primary_route":       "analyze",
			"expected_cli_action": "blue analyze",
			"allow_fallback":      false,
		},
	})

	required := []string{
		"Execution routing contract:",
		"Primary route: analyze",
		"Canonical CLI action: blue analyze",
		"Do not use blue task, blue session",
		"Do not substitute unrelated tools",
		"stop and report the blocker",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("expected routing contract context to include %q, got: %s", want, out)
		}
	}
}

func TestBuildTaskCoordinationPromptContext_IncludesScratchpadAndDelegationHints(t *testing.T) {
	task := &Task{
		WorkspaceRoot: t.TempDir(),
		Metadata:      map[string]interface{}{},
	}
	prepareTaskCoordinationMetadata(task, true)

	out := buildTaskCoordinationPromptContext(task)
	required := []string{
		"Coordination context:",
		"do not outsource understanding",
		"research -> synthesis -> implementation -> verification",
		"subagents tool is available",
		"@scratchpad/",
		".blue/scratchpad/shared",
		"task claims, interim findings, blockers, and worker handoffs",
		"Based on your findings, fix the bug.",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("expected coordination prompt to include %q, got: %s", want, out)
		}
	}
}

func TestRepairExecutionContractToolCalls_RewritesBareReminderCLIWithCanonicalArgs(t *testing.T) {
	withFixedGroundedCanonicalNow(t, time.Date(2026, 3, 31, 10, 0, 0, 0, time.FixedZone("CET", 1*60*60)))

	task := &Task{
		Goal: "Plane eine Erinnerung fur morgen um 9 Uhr, damit ich den Wochenbericht sende.",
		Metadata: map[string]interface{}{
			"routing_contract": map[string]interface{}{
				"expected_cli_action": "blue reminder add",
				"enforce_cli_route":   true,
				"gate_type":           "execution_equivalence",
			},
			"group_input": map[string]interface{}{
				"query":      "Plane eine Erinnerung fur morgen um 9 Uhr, damit ich den Wochenbericht sende.",
				"session_id": "batch1-reminder-de-de-localized_route",
			},
		},
	}
	calls := []llm.ToolCall{{
		ID:        "tc1",
		Name:      "bash",
		Arguments: `{"command":"blue reminder add"}`,
	}}

	repaired, ok := repairExecutionContractToolCalls(task, []llm.Tool{{Name: "bash"}}, calls)
	if !ok {
		t.Fatal("expected execution contract repair to trigger")
	}
	if len(repaired) != 1 {
		t.Fatalf("tool_call_count = %d, want 1", len(repaired))
	}
	command := agentToolCallCommand(repaired[0])
	if command == "blue reminder add" {
		t.Fatalf("command = %q, want canonical reminder args", command)
	}
	for _, want := range []string{
		`blue reminder add`,
		`time="2026-04-01 09:00"`,
		`session_id="batch1-reminder-de-de-localized_route"`,
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("expected repaired command to include %q, got %q", want, command)
		}
	}
}

func TestRepairExecutionContractToolCalls_LeavesCanonicalReminderCLIAlone(t *testing.T) {
	withFixedGroundedCanonicalNow(t, time.Date(2026, 3, 31, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60)))

	task := &Task{
		Goal: "帮我明早 9 点提醒我发送周报。",
		Metadata: map[string]interface{}{
			"routing_contract": map[string]interface{}{
				"expected_cli_action": "blue reminder add",
				"enforce_cli_route":   true,
				"gate_type":           "execution_equivalence",
			},
			"group_input": map[string]interface{}{
				"query":      "帮我明早 9 点提醒我发送周报。",
				"session_id": "batch1-reminder-zh-cn-localized_route",
			},
		},
	}
	canonicalCommand := groundedCanonicalCLICommand(task, "blue reminder add", nil)
	argsJSON, err := json.Marshal(map[string]any{"command": canonicalCommand})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	calls := []llm.ToolCall{{
		ID:        "tc1",
		Name:      "bash",
		Arguments: string(argsJSON),
	}}

	repaired, ok := repairExecutionContractToolCalls(task, []llm.Tool{{Name: "bash"}}, calls)
	if ok {
		t.Fatalf("expected no repair, got %#v", repaired)
	}
	if got := agentToolCallCommand(calls[0]); !strings.Contains(got, `time="2026-04-01 09:00"`) {
		t.Fatalf("command = %q, want canonical reminder command", got)
	}
}
