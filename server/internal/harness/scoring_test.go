package harness

import "testing"

func TestScoreRunByHeuristicJudge_DoesNotPromoteFallbackToPass(t *testing.T) {
	card := scoreRunByHeuristicJudge(&RunGroup{
		ScoringConfig: GroupScoringConfig{
			PassThreshold: 0.5,
		},
	}, &RunGroupItem{
		Profile: "agent_task",
	}, &Run{
		Status: RunStatusCompleted,
		Result: "This is a long-looking result.\nIt looks substantial enough that the old fallback would have promoted it to pass.",
	}, "heuristic", nil)

	if card.Verdict != ScoreVerdictPartial {
		t.Fatalf("verdict = %s, want partial", card.Verdict)
	}
	if card.Score > 0.49 {
		t.Fatalf("score = %.2f, want <= 0.49", card.Score)
	}
}

func TestScoreGroupRun_VerificationFailureOverridesRulePass(t *testing.T) {
	card := scoreGroupRunWithContext(nil, &RunGroup{
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
	}, &RunGroupItem{
		Profile: "agent_task",
		Expected: map[string]interface{}{
			"contains": "done",
		},
	}, &Run{
		Status: RunStatusCompleted,
		Result: "done",
	}, &HarnessVerificationResult{
		Passed:         false,
		Retryable:      false,
		FailureLabel:   "missing_artifact",
		Summary:        "expected artifact was not produced",
		OutcomeScore:   0.7,
		EvidenceScore:  0,
		ExecutionScore: 0.9,
	}, nil)

	if card.Verdict != ScoreVerdictFail {
		t.Fatalf("verdict = %s, want fail", card.Verdict)
	}
	breakdown := decodeJSONMap(card.BreakdownJSON)
	if passed, ok := mapBool(breakdown, "verification_passed"); !ok || passed {
		t.Fatalf("verification_passed = %#v, want false", breakdown["verification_passed"])
	}
	if label := metadataString(breakdown, "failure_label"); label != "missing_artifact" {
		t.Fatalf("failure_label = %q, want missing_artifact", label)
	}
}

func TestScoreGroupRun_VerificationSuccessCanPromoteRulePassWithDeterministicEvidence(t *testing.T) {
	card := scoreGroupRunWithContext(nil, &RunGroup{
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
	}, &RunGroupItem{
		Profile: "agent_task",
	}, &Run{
		Status: RunStatusCompleted,
		Result: "done",
	}, &HarnessVerificationResult{
		Passed:         true,
		Retryable:      false,
		Summary:        "verification passed",
		OutcomeScore:   1,
		EvidenceScore:  1,
		ExecutionScore: 1,
		Checks: []map[string]interface{}{
			{
				"name":     "run_completed",
				"expected": "completed",
				"actual":   "completed",
				"passed":   true,
			},
			{
				"name":     "required_observation",
				"expected": "artifact_emitted",
				"actual":   []string{"artifact_emitted"},
				"passed":   true,
			},
		},
		Observations: []string{"artifact_emitted"},
	}, nil)

	if card.Verdict != ScoreVerdictPass {
		t.Fatalf("verdict = %s, want pass", card.Verdict)
	}
	breakdown := decodeJSONMap(card.BreakdownJSON)
	if scorer := metadataString(breakdown, "scorer"); scorer != "verification_gate" {
		t.Fatalf("scorer = %q, want verification_gate", scorer)
	}
	if passed, ok := mapBool(breakdown, "verification_passed"); !ok || !passed {
		t.Fatalf("verification_passed = %#v, want true", breakdown["verification_passed"])
	}
}
