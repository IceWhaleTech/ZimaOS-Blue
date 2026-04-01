package harness

import "testing"

func TestDeriveHarnessAdaptivePolicy_ClaudeHaiku45UsesLightProfileForLightweightContract(t *testing.T) {
	contract := HarnessContract{
		RequiredObservations: []string{"evidence_tool_used"},
	}

	policy := DeriveHarnessAdaptivePolicy("claude-haiku-4-5-20251001", RunKindAgentTask, contract)
	if policy.Profile != "light" {
		t.Fatalf("profile = %q, want light", policy.Profile)
	}
	if policy.EnableExternalQA {
		t.Fatal("expected lightweight Claude Haiku 4.5 policy to skip external QA")
	}
	if policy.EnableCheckpoints {
		t.Fatal("expected lightweight Claude Haiku 4.5 policy to skip checkpoints")
	}
	if policy.MaxRecoveryAttempts != 0 {
		t.Fatalf("max recovery attempts = %d, want 0", policy.MaxRecoveryAttempts)
	}
}
