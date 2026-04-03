package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	skillCandidateMetadataKey      = "skill_candidate"
	skillCandidateWorkspaceDirName = "skill-candidate-workspace"
	skillCandidateAppliedEventType = "skill_candidate_applied"
)

type skillCandidateMiddleware struct{}

func NewSkillCandidateMiddleware() ExecutionMiddleware {
	return skillCandidateMiddleware{}
}

func (skillCandidateMiddleware) BeforeStart(ctx context.Context, runCtx *RunContext) error {
	if runCtx == nil || runCtx.Run == nil {
		return nil
	}
	run := runCtx.Run
	candidate := nestedMetadataMap(run.Metadata, skillCandidateMetadataKey)
	if len(candidate) == 0 {
		return nil
	}

	skillID, err := normalizeSkillCandidateID(metadataString(candidate, "skill_id"))
	if err != nil {
		return err
	}
	content, err := skillCandidateContent(candidate)
	if err != nil {
		return err
	}

	workspaceRoot := strings.TrimSpace(run.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(strings.TrimSpace(run.ArtifactRoot), skillCandidateWorkspaceDirName)
	}
	if workspaceRoot == "" {
		return fmt.Errorf("skill candidate workspace root is required")
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return fmt.Errorf("create skill candidate workspace: %w", err)
	}

	targetPath := filepath.Join(workspaceRoot, ".agents", "skills", skillID, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create skill candidate directory: %w", err)
	}
	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write skill candidate: %w", err)
	}

	sum := sha256.Sum256([]byte(content))
	sha := hex.EncodeToString(sum[:])
	candidateID := firstNonEmpty(
		metadataString(candidate, "candidate_id"),
		metadataString(run.Metadata, "candidate_id"),
		fmt.Sprintf("%s-%s", skillID, sha[:12]),
	)

	run.WorkspaceRoot = workspaceRoot
	if run.Metadata == nil {
		run.Metadata = map[string]interface{}{}
	}
	if strings.TrimSpace(metadataString(run.Metadata, "optimization_surface")) == "" {
		run.Metadata["optimization_surface"] = string(OptimizationSurfaceSkillDefinition)
	}
	run.Metadata["candidate_id"] = candidateID

	candidate["skill_id"] = skillID
	candidate["content"] = content
	candidate["candidate_id"] = candidateID
	candidate["applied_path"] = targetPath
	candidate["sha256"] = sha
	run.Metadata[skillCandidateMetadataKey] = candidate

	if runCtx.Controller != nil && runCtx.Controller.store != nil {
		if err := runCtx.Controller.store.UpdateRun(ctx, run); err != nil {
			return err
		}
	}
	if runCtx.Controller != nil {
		_ = runCtx.Controller.AppendEvent(ctx, RunEvent{
			RunID:       run.ID,
			RootRunID:   run.RootRunID,
			ParentRunID: run.ParentRunID,
			Type:        skillCandidateAppliedEventType,
			Message:     skillID,
			PayloadJSON: marshalMetadata(map[string]interface{}{
				"skill_id":     skillID,
				"candidate_id": candidateID,
				"applied_path": targetPath,
				"sha256":       sha,
			}),
		})
	}
	return nil
}

func (skillCandidateMiddleware) AfterStart(_ context.Context, _ *RunContext) {}

func (skillCandidateMiddleware) OnStartError(_ context.Context, _ *RunContext, _ error) {}

func skillCandidateContent(candidate map[string]interface{}) (string, error) {
	if len(candidate) == 0 {
		return "", fmt.Errorf("skill candidate metadata is required")
	}
	raw, ok := candidate["content"]
	if !ok {
		return "", fmt.Errorf("skill candidate content is required")
	}
	content, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("skill candidate content must be a string")
	}
	if content == "" {
		return "", fmt.Errorf("skill candidate content is required")
	}
	return content, nil
}

func normalizeSkillCandidateID(raw string) (string, error) {
	skillID := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if skillID == "" {
		return "", fmt.Errorf("skill candidate skill_id is required")
	}
	if strings.HasPrefix(skillID, "/") {
		return "", fmt.Errorf("skill candidate skill_id must be relative")
	}
	clean := filepath.ToSlash(filepath.Clean(skillID))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("skill candidate skill_id %q is invalid", raw)
	}
	return clean, nil
}
