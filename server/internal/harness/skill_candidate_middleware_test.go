package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type skillCandidateProbeDriver struct {
	kind              RunKind
	expectedSkillID   string
	expectedContent   string
	observedWorkspace string
	observedSkillPath string
}

func (d *skillCandidateProbeDriver) Kind() RunKind { return d.kind }

func (d *skillCandidateProbeDriver) Validate(spec RunSpec) error {
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *skillCandidateProbeDriver) Start(_ context.Context, run *Run, _ RunEnv) error {
	if run == nil {
		return fmt.Errorf("run is required")
	}
	d.observedWorkspace = strings.TrimSpace(run.WorkspaceRoot)
	d.observedSkillPath = filepath.Join(d.observedWorkspace, ".agents", "skills", d.expectedSkillID, "SKILL.md")
	data, err := os.ReadFile(d.observedSkillPath)
	if err != nil {
		return fmt.Errorf("read candidate skill: %w", err)
	}
	if got := string(data); got != d.expectedContent {
		return fmt.Errorf("candidate skill content = %q, want %q", got, d.expectedContent)
	}
	return nil
}

func (d *skillCandidateProbeDriver) Cancel(_ context.Context, _ *Run) error { return nil }

func TestSkillCandidateMiddlewareWritesCandidateToWorkspaceAndPersistsMetadata(t *testing.T) {
	controller := newTestController(t)
	controller.UseExecutionMiddleware(NewSkillCandidateMiddleware())

	const skillID = "browser"
	const sourcePath = "assets/skills/browser/SKILL.md"
	skillContent := strings.TrimSpace(`
---
name: browser
description: Browser skill candidate
---

# Browser

Use the browser carefully.
`) + "\n"

	driver := &skillCandidateProbeDriver{
		kind:            RunKindAgentTask,
		expectedSkillID: skillID,
		expectedContent: skillContent,
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "exercise skill candidate middleware",
		UserID: "user-1",
		Metadata: map[string]interface{}{
			"skill_candidate": map[string]interface{}{
				"skill_id":    skillID,
				"content":     skillContent,
				"source_path": sourcePath,
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	stored, err := controller.GetStored(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetStored failed: %v", err)
	}
	if got := strings.TrimSpace(stored.WorkspaceRoot); got == "" {
		t.Fatal("expected workspace_root to be populated")
	}
	if got, want := stored.WorkspaceRoot, run.WorkspaceRoot; got != want {
		t.Fatalf("workspace_root = %q, want %q", got, want)
	}
	if got, want := driver.observedWorkspace, stored.WorkspaceRoot; got != want {
		t.Fatalf("driver observed workspace = %q, want %q", got, want)
	}
	if got, want := driver.observedSkillPath, filepath.Join(stored.WorkspaceRoot, ".agents", "skills", skillID, "SKILL.md"); got != want {
		t.Fatalf("driver observed skill path = %q, want %q", got, want)
	}

	candidateMeta := nestedMetadataMap(stored.Metadata, "skill_candidate")
	if got := metadataString(candidateMeta, "skill_id"); got != skillID {
		t.Fatalf("skill_candidate.skill_id = %q, want %q", got, skillID)
	}
	if got := metadataString(candidateMeta, "source_path"); got != sourcePath {
		t.Fatalf("skill_candidate.source_path = %q, want %q", got, sourcePath)
	}
	if got, want := metadataString(candidateMeta, "applied_path"), driver.observedSkillPath; got != want {
		t.Fatalf("skill_candidate.applied_path = %q, want %q", got, want)
	}
	sum := sha256.Sum256([]byte(skillContent))
	if got, want := metadataString(candidateMeta, "sha256"), hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("skill_candidate.sha256 = %q, want %q", got, want)
	}
	if got := metadataString(candidateMeta, "candidate_id"); got == "" {
		t.Fatal("expected skill_candidate.candidate_id to be synthesized")
	} else if topLevel := metadataString(stored.Metadata, "candidate_id"); topLevel != got {
		t.Fatalf("candidate_id = %q, want %q", topLevel, got)
	}

	events, err := controller.ListEvents(context.Background(), run.ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	appliedEventSeen := false
	for _, event := range events {
		if event.Type == "skill_candidate_applied" {
			appliedEventSeen = true
			break
		}
	}
	if !appliedEventSeen {
		t.Fatalf("events = %#v, want skill_candidate_applied", events)
	}
}
