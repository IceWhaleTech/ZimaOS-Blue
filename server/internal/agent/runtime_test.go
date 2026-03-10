package agent

import "testing"

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
