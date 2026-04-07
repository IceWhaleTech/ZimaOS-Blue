package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveWritableSkillSourceStateAcceptsManagedInstalledSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "workspace_note")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skillDir: %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	content := "# Workspace Note\nInstalled skill content.\n"
	if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile skillPath: %v", err)
	}

	state, err := ResolveWritableSkillSourceState("workspace_note", skillPath)
	if err != nil {
		t.Fatalf("ResolveWritableSkillSourceState failed: %v", err)
	}
	if got, want := state.AbsolutePath, filepath.Clean(skillPath); got != want {
		t.Fatalf("AbsolutePath = %q, want %q", got, want)
	}
	if got, want := state.NormalizedPath, normalizeSkillSourcePath(skillPath); got != want {
		t.Fatalf("NormalizedPath = %q, want %q", got, want)
	}
	if got, want := state.Content, content; got != want {
		t.Fatalf("Content = %q, want %q", got, want)
	}
	if got, want := state.ContentSHA256, sha256HexForSkillRevisionTest(content); got != want {
		t.Fatalf("ContentSHA256 = %q, want %q", got, want)
	}
}

func TestResolveWritableSkillSourceStateRejectsPathOutsideWritableRoots(t *testing.T) {
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("# Unsafe\n"), 0o644); err != nil {
		t.Fatalf("WriteFile skillPath: %v", err)
	}

	_, err := ResolveWritableSkillSourceState("workspace_note", skillPath)
	if err == nil || !strings.Contains(err.Error(), "must stay under") {
		t.Fatalf("ResolveWritableSkillSourceState error = %v, want managed-root rejection", err)
	}
}

func TestController_PromoteSkillRevisionWritesManagedInstalledSkill(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	recorder := &recordingSkillRevisionPromotionRecorder{}
	controller.SetOptimizationTriggerer(recorder)

	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "workspace_note")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skillDir: %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	originalContent := "# Workspace Note\nOriginal installed skill.\n"
	if err := os.WriteFile(skillPath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("WriteFile skillPath: %v", err)
	}

	now := time.Now().UTC()
	revision := &SkillRevision{
		ID:                "rev-installed-promote",
		SkillID:           "workspace_note",
		Status:            SkillRevisionStatusAccepted,
		SourcePath:        skillPath,
		CandidateID:       "candidate-installed-promote",
		BaseContentSHA256: sha256HexForSkillRevisionTest(originalContent),
		Content:           "# Workspace Note\nPromoted installed skill.\n",
		ContentSHA256:     sha256HexForSkillRevisionTest("# Workspace Note\nPromoted installed skill.\n"),
		CreatedAt:         now,
	}
	if err := controller.store.CreateSkillRevision(ctx, revision); err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}

	result, err := controller.PromoteSkillRevision(ctx, revision.ID, SkillRevisionDecisionRequest{
		ReviewNote: "Promote installed skill after review.",
		ReviewedBy: "user-installed-promote",
	})
	if err != nil {
		t.Fatalf("PromoteSkillRevision failed: %v", err)
	}

	written, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile skillPath: %v", err)
	}
	if got, want := string(written), revision.Content; got != want {
		t.Fatalf("installed skill content = %q, want %q", got, want)
	}

	backup, err := controller.store.GetSkillRevision(ctx, result.BackupRevisionID)
	if err != nil {
		t.Fatalf("GetSkillRevision(backup) failed: %v", err)
	}
	if got, want := backup.Status, SkillRevisionStatusBackup; got != want {
		t.Fatalf("backup status = %q, want %q", got, want)
	}
	if got, want := backup.SourcePath, normalizeSkillSourcePath(skillPath); got != want {
		t.Fatalf("backup source_path = %q, want %q", got, want)
	}
	if got, want := backup.Content, originalContent; got != want {
		t.Fatalf("backup content = %q, want %q", got, want)
	}
	if recorder.calls != 1 {
		t.Fatalf("promotion recorder calls = %d, want 1", recorder.calls)
	}
}

func TestController_RollbackSkillRevisionRestoresManagedInstalledSkill(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	recorder := &recordingSkillRevisionPromotionRecorder{}
	controller.SetOptimizationTriggerer(recorder)

	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".claude", "skills", "workspace_note")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skillDir: %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	rolledBackContent := "# Workspace Note\nOriginal installed skill.\n"
	currentContent := "# Workspace Note\nCurrent promoted installed skill.\n"
	if err := os.WriteFile(skillPath, []byte(currentContent), 0o644); err != nil {
		t.Fatalf("WriteFile skillPath: %v", err)
	}

	now := time.Now().UTC()
	currentRevision := &SkillRevision{
		ID:            "rev-installed-current",
		SkillID:       "workspace_note",
		Status:        SkillRevisionStatusPromoted,
		SourcePath:    skillPath,
		CandidateID:   "candidate-installed-current",
		Content:       currentContent,
		ContentSHA256: sha256HexForSkillRevisionTest(currentContent),
		CreatedAt:     now.Add(-time.Minute),
		PromotedAt:    ptrTimeForSkillRevisionTest(now.Add(-time.Minute)),
	}
	if err := controller.store.CreateSkillRevision(ctx, currentRevision); err != nil {
		t.Fatalf("CreateSkillRevision(currentRevision) failed: %v", err)
	}

	backupRevision := &SkillRevision{
		ID:                 "rev-installed-backup",
		SkillID:            "workspace_note",
		Status:             SkillRevisionStatusBackup,
		SourcePath:         skillPath,
		CandidateID:        currentRevision.CandidateID,
		BaseContentSHA256:  sha256HexForSkillRevisionTest(currentContent),
		BackupOfRevisionID: currentRevision.ID,
		Content:            rolledBackContent,
		ContentSHA256:      sha256HexForSkillRevisionTest(rolledBackContent),
		CreatedAt:          now.Add(-time.Minute),
	}
	if err := controller.store.CreateSkillRevision(ctx, backupRevision); err != nil {
		t.Fatalf("CreateSkillRevision(backupRevision) failed: %v", err)
	}

	result, err := controller.RollbackSkillRevision(ctx, backupRevision.ID, SkillRevisionDecisionRequest{
		ReviewNote: "Rollback installed skill after regression.",
		ReviewedBy: "user-installed-rollback",
	})
	if err != nil {
		t.Fatalf("RollbackSkillRevision failed: %v", err)
	}

	written, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile skillPath: %v", err)
	}
	if got, want := string(written), rolledBackContent; got != want {
		t.Fatalf("installed skill content after rollback = %q, want %q", got, want)
	}

	rollbackRevision, err := controller.store.GetSkillRevision(ctx, result.PromotedRevisionID)
	if err != nil {
		t.Fatalf("GetSkillRevision(rollback revision) failed: %v", err)
	}
	if got, want := rollbackRevision.SourcePath, normalizeSkillSourcePath(skillPath); got != want {
		t.Fatalf("rollback source_path = %q, want %q", got, want)
	}
	if got, want := rollbackRevision.Status, SkillRevisionStatusPromoted; got != want {
		t.Fatalf("rollback status = %q, want %q", got, want)
	}
	if recorder.calls != 1 {
		t.Fatalf("promotion recorder calls = %d, want 1", recorder.calls)
	}
}

func TestController_OptimizeSkillCreatesCandidateRevisionForManagedInstalledSkill(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	triggerer := &recordingOptimizationTriggerer{}
	controller.SetOptimizationTriggerer(triggerer)

	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "workspace_note")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skillDir: %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	content := "# Workspace Note\nInstalled skill content.\n"
	if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile skillPath: %v", err)
	}

	evalRun := createManualOptimizeEvalRunForSkillRevisionTest(t, controller, map[string]interface{}{
		"candidate_id": "candidate-installed-manual",
		"skill_candidate": map[string]interface{}{
			"skill_id":    "workspace_note",
			"source_path": skillPath,
		},
		"optimization_surface": string(OptimizationSurfaceSkillDefinition),
	})

	event, err := controller.OptimizeSkill(ctx, "workspace_note", SkillOptimizeRequest{
		EvalRunID: evalRun.ID,
	})
	if err != nil {
		t.Fatalf("OptimizeSkill failed: %v", err)
	}

	revisions, err := controller.store.ListSkillRevisions(ctx, SkillRevisionFilter{
		SkillID:  "workspace_note",
		Statuses: []SkillRevisionStatus{SkillRevisionStatusCandidate},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListSkillRevisions failed: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("ListSkillRevisions len = %d, want 1", len(revisions))
	}
	revision := revisions[0]
	if got, want := revision.SourcePath, normalizeSkillSourcePath(skillPath); got != want {
		t.Fatalf("revision source_path = %q, want %q", got, want)
	}
	if got, want := revision.Content, content; got != want {
		t.Fatalf("revision content = %q, want %q", got, want)
	}

	skillCandidate := nestedMetadataMap(event.Metadata, "skill_candidate")
	if got, want := strings.TrimSpace(metadataString(skillCandidate, "source_path")), normalizeSkillSourcePath(skillPath); got != want {
		t.Fatalf("skill_candidate.source_path = %q, want %q", got, want)
	}
}
