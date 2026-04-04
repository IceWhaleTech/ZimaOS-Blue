package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteStore_SkillRevisionCRUDAndFilter(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	now := time.Now().UTC()

	parentContent := strings.TrimSpace(`
---
name: browser
---

# Browser
Original browser skill candidate.
`) + "\n"
	parent := &SkillRevision{
		ID:                "rev-parent",
		SkillID:           "browser",
		Status:            SkillRevisionStatusCandidate,
		SourcePath:        "assets/skills/browser/SKILL.md",
		CandidateID:       "candidate-parent",
		Content:           parentContent,
		ContentSHA256:     sha256HexForSkillRevisionTest(parentContent),
		EvalRunID:         "eval-parent",
		OptimizationRunID: "opt-parent",
		CreatedAt:         now,
	}
	if err := controller.store.CreateSkillRevision(ctx, parent); err != nil {
		t.Fatalf("CreateSkillRevision(parent) failed: %v", err)
	}

	acceptedContent := strings.TrimSpace(`
---
name: browser
description: improved browser
---

# Browser
Improved browser skill candidate.
`) + "\n"
	accepted := &SkillRevision{
		ID:                "rev-accepted",
		SkillID:           "browser",
		Status:            SkillRevisionStatusAccepted,
		SourcePath:        "assets/skills/browser/SKILL.md",
		CandidateID:       "candidate-accepted",
		ParentRevisionID:  parent.ID,
		Content:           acceptedContent,
		ContentSHA256:     sha256HexForSkillRevisionTest(acceptedContent),
		EvalRunID:         "eval-accepted",
		OptimizationRunID: "opt-accepted",
		CreatedAt:         now.Add(time.Second),
	}
	if err := controller.store.CreateSkillRevision(ctx, accepted); err != nil {
		t.Fatalf("CreateSkillRevision(accepted) failed: %v", err)
	}

	stored, err := controller.store.GetSkillRevision(ctx, accepted.ID)
	if err != nil {
		t.Fatalf("GetSkillRevision failed: %v", err)
	}
	if got, want := stored.ParentRevisionID, parent.ID; got != want {
		t.Fatalf("ParentRevisionID = %q, want %q", got, want)
	}
	if got, want := stored.Content, acceptedContent; got != want {
		t.Fatalf("Content = %q, want %q", got, want)
	}
	if got, want := stored.ContentSHA256, sha256HexForSkillRevisionTest(acceptedContent); got != want {
		t.Fatalf("ContentSHA256 = %q, want %q", got, want)
	}

	promotedAt := now.Add(2 * time.Second)
	stored.Status = SkillRevisionStatusPromoted
	stored.PromotedAt = &promotedAt
	if err := controller.store.UpdateSkillRevision(ctx, stored); err != nil {
		t.Fatalf("UpdateSkillRevision failed: %v", err)
	}
	reloaded, err := controller.store.GetSkillRevision(ctx, stored.ID)
	if err != nil {
		t.Fatalf("GetSkillRevision(reloaded) failed: %v", err)
	}
	if reloaded.PromotedAt == nil || !reloaded.PromotedAt.Equal(promotedAt) {
		t.Fatalf("PromotedAt = %#v, want %v", reloaded.PromotedAt, promotedAt)
	}

	revisions, err := controller.store.ListSkillRevisions(ctx, SkillRevisionFilter{
		SkillID:  "browser",
		Statuses: []SkillRevisionStatus{SkillRevisionStatusPromoted},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListSkillRevisions failed: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("ListSkillRevisions len = %d, want 1", len(revisions))
	}
	if got, want := revisions[0].ID, accepted.ID; got != want {
		t.Fatalf("revision id = %q, want %q", got, want)
	}
	if got, want := revisions[0].Status, SkillRevisionStatusPromoted; got != want {
		t.Fatalf("revision status = %q, want %q", got, want)
	}
}

func TestController_PromoteSkillRevisionCreatesBackupAndWritesCanonicalFile(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	recorder := &recordingSkillRevisionPromotionRecorder{}
	controller.SetOptimizationTriggerer(recorder)
	repoRoot := t.TempDir()
	skillDir := filepath.Join(repoRoot, "assets", "skills", "browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skillDir: %v", err)
	}
	canonicalPath := filepath.Join(skillDir, "SKILL.md")
	originalContent := strings.TrimSpace(`
---
name: browser
---

# Browser
Original canonical skill.
`) + "\n"
	if err := os.WriteFile(canonicalPath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("WriteFile canonicalPath: %v", err)
	}

	restoreWD := chdirForSkillRevisionTest(t, repoRoot)
	defer restoreWD()

	replacementContent := strings.TrimSpace(`
---
name: browser
description: promoted revision
---

# Browser
Promoted canonical skill.
`) + "\n"
	revision := &SkillRevision{
		ID:                "rev-promote",
		SkillID:           "browser",
		Status:            SkillRevisionStatusAccepted,
		SourcePath:        "assets/skills/browser/SKILL.md",
		CandidateID:       "candidate-browser-promote",
		Content:           replacementContent,
		ContentSHA256:     sha256HexForSkillRevisionTest(replacementContent),
		EvalRunID:         "eval-promote",
		OptimizationRunID: "opt-promote",
		CreatedAt:         time.Now().UTC(),
	}
	if err := controller.store.CreateSkillRevision(ctx, revision); err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}

	result, err := controller.PromoteSkillRevision(ctx, revision.ID)
	if err != nil {
		t.Fatalf("PromoteSkillRevision failed: %v", err)
	}
	if got, want := result.PromotedRevisionID, revision.ID; got != want {
		t.Fatalf("PromotedRevisionID = %q, want %q", got, want)
	}
	if strings.TrimSpace(result.BackupRevisionID) == "" {
		t.Fatalf("BackupRevisionID = %q, want non-empty", result.BackupRevisionID)
	}
	gotPath, err := filepath.EvalSymlinks(result.WrittenSourcePath)
	if err != nil {
		t.Fatalf("EvalSymlinks(result.WrittenSourcePath) failed: %v", err)
	}
	wantPath, err := filepath.EvalSymlinks(canonicalPath)
	if err != nil {
		t.Fatalf("EvalSymlinks(canonicalPath) failed: %v", err)
	}
	if got, want := gotPath, wantPath; got != want {
		t.Fatalf("WrittenSourcePath = %q, want %q", got, want)
	}

	written, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("ReadFile canonicalPath: %v", err)
	}
	if got, want := string(written), replacementContent; got != want {
		t.Fatalf("canonical content = %q, want %q", got, want)
	}

	promoted, err := controller.store.GetSkillRevision(ctx, revision.ID)
	if err != nil {
		t.Fatalf("GetSkillRevision(promoted) failed: %v", err)
	}
	if got, want := promoted.Status, SkillRevisionStatusPromoted; got != want {
		t.Fatalf("promoted status = %q, want %q", got, want)
	}
	if promoted.PromotedAt == nil {
		t.Fatal("expected promoted revision to record promoted_at")
	}

	backup, err := controller.store.GetSkillRevision(ctx, result.BackupRevisionID)
	if err != nil {
		t.Fatalf("GetSkillRevision(backup) failed: %v", err)
	}
	if got, want := backup.Status, SkillRevisionStatusBackup; got != want {
		t.Fatalf("backup status = %q, want %q", got, want)
	}
	if got, want := backup.BackupOfRevisionID, revision.ID; got != want {
		t.Fatalf("backup.BackupOfRevisionID = %q, want %q", got, want)
	}
	if got, want := backup.Content, originalContent; got != want {
		t.Fatalf("backup content = %q, want %q", got, want)
	}
	if got, want := backup.SourcePath, "assets/skills/browser/SKILL.md"; got != want {
		t.Fatalf("backup source_path = %q, want %q", got, want)
	}
	if recorder.calls != 1 {
		t.Fatalf("promotion recorder calls = %d, want 1", recorder.calls)
	}
	if got, want := recorder.promotedRevisionID, revision.ID; got != want {
		t.Fatalf("recorded promoted revision id = %q, want %q", got, want)
	}
	if got, want := recorder.backupRevisionID, result.BackupRevisionID; got != want {
		t.Fatalf("recorded backup revision id = %q, want %q", got, want)
	}
}

func TestController_PromoteSkillRevisionRejectsNonAcceptedRevision(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	revision := &SkillRevision{
		ID:            "rev-reject",
		SkillID:       "browser",
		Status:        SkillRevisionStatusCandidate,
		SourcePath:    "assets/skills/browser/SKILL.md",
		CandidateID:   "candidate-browser-reject",
		Content:       "# Browser\nCandidate only.\n",
		ContentSHA256: sha256HexForSkillRevisionTest("# Browser\nCandidate only.\n"),
		CreatedAt:     time.Now().UTC(),
	}
	if err := controller.store.CreateSkillRevision(ctx, revision); err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}

	_, err := controller.PromoteSkillRevision(ctx, revision.ID)
	if err == nil || !strings.Contains(err.Error(), "accepted") {
		t.Fatalf("PromoteSkillRevision error = %v, want accepted status validation", err)
	}
}

func TestController_PromoteSkillRevisionRejectsUnsafeSourcePath(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "server", "internal", "skill", "embedded", "skills", "browser"), 0o755); err != nil {
		t.Fatalf("MkdirAll embedded path: %v", err)
	}
	restoreWD := chdirForSkillRevisionTest(t, repoRoot)
	defer restoreWD()

	revision := &SkillRevision{
		ID:            "rev-unsafe",
		SkillID:       "browser",
		Status:        SkillRevisionStatusAccepted,
		SourcePath:    "server/internal/skill/embedded/skills/browser/SKILL.md",
		CandidateID:   "candidate-browser-unsafe",
		Content:       "# Browser\nUnsafe promote target.\n",
		ContentSHA256: sha256HexForSkillRevisionTest("# Browser\nUnsafe promote target.\n"),
		CreatedAt:     time.Now().UTC(),
	}
	if err := controller.store.CreateSkillRevision(ctx, revision); err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}

	_, err := controller.PromoteSkillRevision(ctx, revision.ID)
	if err == nil || !strings.Contains(err.Error(), "embedded") {
		t.Fatalf("PromoteSkillRevision error = %v, want embedded path rejection", err)
	}
}

func TestController_OptimizeSkillTriggersManualOptimizationForCompletedEvalRun(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	triggerer := &recordingOptimizationTriggerer{}
	controller.SetOptimizationTriggerer(triggerer)

	evalRun := createManualOptimizeEvalRunForSkillRevisionTest(t, controller, map[string]interface{}{
		"candidate_id": "candidate-browser-manual",
		"skill_candidate": map[string]interface{}{
			"skill_id":    "browser",
			"source_path": "assets/skills/browser/SKILL.md",
		},
		"optimization_surface": string(OptimizationSurfaceSkillDefinition),
	})

	event, err := controller.OptimizeSkill(ctx, "browser", SkillOptimizeRequest{
		EvalRunID: evalRun.ID,
	})
	if err != nil {
		t.Fatalf("OptimizeSkill failed: %v", err)
	}
	if len(triggerer.events) != 1 {
		t.Fatalf("trigger count = %d, want 1", len(triggerer.events))
	}
	if got, want := event.Reason, OptimizationReasonManualSkillOptimize; got != want {
		t.Fatalf("event reason = %q, want %q", got, want)
	}
	if got, want := triggerer.events[0].Reason, OptimizationReasonManualSkillOptimize; got != want {
		t.Fatalf("triggered reason = %q, want %q", got, want)
	}
	if got, want := event.EvalRunID, evalRun.ID; got != want {
		t.Fatalf("event eval_run_id = %q, want %q", got, want)
	}
	if got, want := event.BaseEvalRunID, evalRun.BaselineEvalRunID; got != want {
		t.Fatalf("event base_eval_run_id = %q, want %q", got, want)
	}
	if got, want := event.CandidateID, "candidate-browser-manual"; got != want {
		t.Fatalf("event candidate_id = %q, want %q", got, want)
	}
	if got, want := event.OptimizationSurface, OptimizationSurfaceSkillDefinition; got != want {
		t.Fatalf("event optimization_surface = %q, want %q", got, want)
	}
	if got := strings.TrimSpace(metadataString(event.Metadata, "followup_gate")); got != "selector" {
		t.Fatalf("metadata.followup_gate = %q, want selector", got)
	}
	skillCandidate := nestedMetadataMap(event.Metadata, "skill_candidate")
	if got := strings.TrimSpace(metadataString(skillCandidate, "skill_id")); got != "browser" {
		t.Fatalf("skill_candidate.skill_id = %q, want browser", got)
	}
	if got := strings.TrimSpace(metadataString(skillCandidate, "source_path")); got != "assets/skills/browser/SKILL.md" {
		t.Fatalf("skill_candidate.source_path = %q, want assets/skills/browser/SKILL.md", got)
	}
}

func TestController_OptimizeSkillRejectsUnsafeSourcePath(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	triggerer := &recordingOptimizationTriggerer{}
	controller.SetOptimizationTriggerer(triggerer)

	evalRun := createManualOptimizeEvalRunForSkillRevisionTest(t, controller, map[string]interface{}{
		"candidate_id": "candidate-browser-manual",
		"skill_candidate": map[string]interface{}{
			"skill_id": "browser",
		},
		"optimization_surface": string(OptimizationSurfaceSkillDefinition),
	})

	_, err := controller.OptimizeSkill(ctx, "browser", SkillOptimizeRequest{
		EvalRunID:  evalRun.ID,
		SourcePath: "server/internal/skill/embedded/skills/browser/SKILL.md",
	})
	if err == nil || !strings.Contains(err.Error(), "embedded") {
		t.Fatalf("OptimizeSkill error = %v, want embedded path rejection", err)
	}
	if len(triggerer.events) != 0 {
		t.Fatalf("trigger count = %d, want 0", len(triggerer.events))
	}
}

func sha256HexForSkillRevisionTest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func chdirForSkillRevisionTest(t *testing.T, dir string) func() {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd failed: %v", err)
		}
	}
}

func createManualOptimizeEvalRunForSkillRevisionTest(t *testing.T, controller *Controller, metadata map[string]interface{}) *EvalRun {
	t.Helper()
	ctx := context.Background()
	dataset, err := controller.CreateDataset(ctx, DatasetSpec{
		Name:           "manual-optimize-dataset",
		OwnerUserID:    "user-1",
		Subject:        SelectorCuratedDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: "selector_dry_run",
	})
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}
	version, err := controller.CreateDatasetVersion(ctx, dataset.ID, DatasetVersionSpec{
		Version: "v1",
		Manifest: map[string]interface{}{
			"defaults": map[string]interface{}{
				"run_kind": "agent_task",
				"profile":  "selector_dry_run",
			},
			"items": []interface{}{
				map[string]interface{}{
					"id": "case-1",
					"input": map[string]interface{}{
						"goal": "manual optimize eval run",
					},
				},
			},
		},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}
	evalSpec, err := controller.CreateEvalSpec(ctx, EvalSpecSpec{
		Name:             "manual-optimize-eval-spec",
		OwnerUserID:      "user-1",
		Subject:          SelectorCuratedDatasetSubject,
		RunKind:          RunKindAgentTask,
		Profile:          "selector_dry_run",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		Metadata: map[string]interface{}{
			"gate_type": "selection",
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}
	now := time.Now().UTC()
	evalRun := &EvalRun{
		ID:                "eval-manual-optimize",
		EvalSpecID:        evalSpec.ID,
		GroupID:           "group-manual-optimize",
		DatasetVersionID:  version.ID,
		BaselineEvalRunID: "eval-baseline-manual",
		Title:             "manual optimize parent",
		OwnerUserID:       "user-1",
		Status:            RunGroupStatusCompleted,
		Metadata:          metadata,
		CreatedAt:         now,
		UpdatedAt:         now,
		StartedAt:         &now,
		FinishedAt:        &now,
	}
	if err := controller.store.CreateEvalRun(ctx, evalRun); err != nil {
		t.Fatalf("CreateEvalRun failed: %v", err)
	}
	return evalRun
}

type recordingSkillRevisionPromotionRecorder struct {
	recordingOptimizationTriggerer
	promotedRevisionID string
	backupRevisionID   string
	writtenSourcePath  string
	calls              int
}

func (r *recordingSkillRevisionPromotionRecorder) RecordSkillRevisionPromotion(_ context.Context, promotedRevision *SkillRevision, backupRevision *SkillRevision, writtenSourcePath string) error {
	if promotedRevision != nil {
		r.promotedRevisionID = promotedRevision.ID
	}
	if backupRevision != nil {
		r.backupRevisionID = backupRevision.ID
	}
	r.writtenSourcePath = writtenSourcePath
	r.calls++
	return nil
}
