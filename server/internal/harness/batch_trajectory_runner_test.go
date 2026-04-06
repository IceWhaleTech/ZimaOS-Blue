package harness

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type stubBatchTrajectoryExecutor struct {
	failuresBeforeSuccess map[string]int
	attempts              map[string]int
	calls                 []string
}

func (s *stubBatchTrajectoryExecutor) ExecuteBatchTrajectoryCase(_ context.Context, c BatchTrajectoryCase) (*BatchTrajectoryCaseResult, error) {
	key := batchTrajectoryCaseKey(c)
	if s.attempts == nil {
		s.attempts = make(map[string]int)
	}
	s.attempts[key]++
	s.calls = append(s.calls, key)
	if s.attempts[key] <= s.failuresBeforeSuccess[key] {
		return nil, errors.New("temporary trajectory failure")
	}
	input := cloneMetadataMap(c.Input)
	if input == nil {
		input = map[string]interface{}{}
	}
	if _, ok := input["goal"]; !ok && c.Prompt != "" {
		input["goal"] = c.Prompt
	}
	return &BatchTrajectoryCaseResult{
		Input:        input,
		Expected:     map[string]interface{}{"status": "completed"},
		Metadata:     map[string]interface{}{"runner_attempt": s.attempts[key]},
		TraceSummary: "trajectory summary for " + key,
		Provenance: map[string]interface{}{
			"executor": "stub",
			"case_key": key,
		},
	}, nil
}

func TestBatchTrajectoryRunnerResumeRetryAndDatasetOutput(t *testing.T) {
	cases := []BatchTrajectoryCase{
		{
			Key:    "case-alpha",
			Prompt: "alpha prompt",
			Input:  map[string]interface{}{"goal": "alpha prompt"},
			Metadata: map[string]interface{}{
				"kind": "selector",
			},
		},
		{
			Prompt: "beta prompt",
			Input:  map[string]interface{}{"goal": "beta prompt"},
			Metadata: map[string]interface{}{
				"kind": "selector",
			},
		},
		{
			Key:    "case-gamma",
			Prompt: "gamma prompt",
			Input:  map[string]interface{}{"goal": "gamma prompt"},
			Metadata: map[string]interface{}{
				"kind": "selector",
			},
		},
	}
	betaKey := batchTrajectoryCaseKey(cases[1])
	checkpointPath := filepath.Join(t.TempDir(), "trajectory-checkpoint.json")

	firstExecutor := &stubBatchTrajectoryExecutor{
		failuresBeforeSuccess: map[string]int{
			betaKey: 1,
		},
	}
	firstRunner := NewBatchTrajectoryRunner(BatchTrajectoryRunnerConfig{
		DatasetName:    "trajectory-dataset",
		DatasetSubject: SelectorCuratedDatasetSubject,
		DatasetVersion: "trajectory-v1",
		SourceType:     "batch_trajectory",
		SourceRef:      "fixtures/trajectory-prompts.jsonl",
		CreatedBy:      "user-1",
		CandidateID:    "candidate-batch-1",
		CheckpointPath: checkpointPath,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: "selector_dry_run",
		MaxRetries:     0,
	}, firstExecutor)

	firstResult, err := firstRunner.Run(context.Background(), cases)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if len(firstResult.FailedCaseKeys) != 1 || firstResult.FailedCaseKeys[0] != betaKey {
		t.Fatalf("first failed_case_keys = %#v, want [%q]", firstResult.FailedCaseKeys, betaKey)
	}
	if len(firstResult.Manifest.Items) != 2 {
		t.Fatalf("first manifest item count = %d, want 2", len(firstResult.Manifest.Items))
	}

	secondExecutor := &stubBatchTrajectoryExecutor{
		failuresBeforeSuccess: map[string]int{
			betaKey: 1,
		},
	}
	secondRunner := NewBatchTrajectoryRunner(BatchTrajectoryRunnerConfig{
		DatasetName:    "trajectory-dataset",
		DatasetSubject: SelectorCuratedDatasetSubject,
		DatasetVersion: "trajectory-v1",
		SourceType:     "batch_trajectory",
		SourceRef:      "fixtures/trajectory-prompts.jsonl",
		CreatedBy:      "user-1",
		CandidateID:    "candidate-batch-1",
		CheckpointPath: checkpointPath,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: "selector_dry_run",
		MaxRetries:     1,
	}, secondExecutor)

	secondResult, err := secondRunner.Run(context.Background(), cases)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if len(secondResult.FailedCaseKeys) != 0 {
		t.Fatalf("second failed_case_keys = %#v, want empty", secondResult.FailedCaseKeys)
	}
	if len(secondExecutor.calls) != 2 || secondExecutor.calls[0] != betaKey || secondExecutor.calls[1] != betaKey {
		t.Fatalf("second executor calls = %#v, want only retried beta case twice", secondExecutor.calls)
	}
	if len(secondResult.Manifest.Items) != 3 {
		t.Fatalf("second manifest item count = %d, want 3", len(secondResult.Manifest.Items))
	}
	if got, want := secondResult.Manifest.Items[0].ID, "case-alpha"; got != want {
		t.Fatalf("first manifest item id = %q, want %q", got, want)
	}
	if got, want := secondResult.Manifest.Items[1].ID, betaKey; got != want {
		t.Fatalf("second manifest item id = %q, want %q", got, want)
	}
	if got, want := secondResult.Manifest.Items[2].ID, "case-gamma"; got != want {
		t.Fatalf("third manifest item id = %q, want %q", got, want)
	}

	betaMeta := secondResult.Manifest.Items[1].Metadata
	if got, want := metadataString(betaMeta, "candidate_id"), "candidate-batch-1"; got != want {
		t.Fatalf("candidate_id = %q, want %q", got, want)
	}
	provenance := nestedMetadataMap(betaMeta, "trajectory_provenance")
	if got, want := metadataString(provenance, "source_type"), "batch_trajectory"; got != want {
		t.Fatalf("trajectory_provenance.source_type = %q, want %q", got, want)
	}
	if got, want := metadataString(provenance, "source_ref"), "fixtures/trajectory-prompts.jsonl"; got != want {
		t.Fatalf("trajectory_provenance.source_ref = %q, want %q", got, want)
	}
	if got, want := metadataString(provenance, "case_key"), betaKey; got != want {
		t.Fatalf("trajectory_provenance.case_key = %q, want %q", got, want)
	}
	if got := metadataString(betaMeta, "trajectory_summary"); got == "" {
		t.Fatal("expected trajectory_summary to be populated")
	}

	if got, want := secondResult.DatasetVersionSpec.SourceType, "batch_trajectory"; got != want {
		t.Fatalf("dataset version source_type = %q, want %q", got, want)
	}
	if got, want := secondResult.DatasetVersionSpec.SourceRef, "fixtures/trajectory-prompts.jsonl"; got != want {
		t.Fatalf("dataset version source_ref = %q, want %q", got, want)
	}
	if got, want := metadataString(secondResult.DatasetVersionSpec.Metadata, "candidate_id"), "candidate-batch-1"; got != want {
		t.Fatalf("dataset version metadata candidate_id = %q, want %q", got, want)
	}
	checkpoint, err := loadBatchTrajectoryCheckpoint(checkpointPath)
	if err != nil {
		t.Fatalf("loadBatchTrajectoryCheckpoint() error = %v", err)
	}
	if got, want := checkpoint.Cases[betaKey].Status, batchTrajectoryCheckpointStatusCompleted; got != want {
		t.Fatalf("checkpoint beta status = %q, want %q", got, want)
	}
}
