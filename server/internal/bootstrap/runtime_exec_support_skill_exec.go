package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeExecSkillExecutor(skills runtimeSkillRegistrySource, workspaceDir string) tools.SkillExecFunc {
	if skills == nil {
		workspaceDir = strings.TrimSpace(workspaceDir)
		if workspaceDir == "" {
			return nil
		}
	}
	return func(ctx context.Context, skillID string, input map[string]any) (map[string]string, error) {
		resolved, err := resolveRuntimeSkillForExecution(skills, runtimeSkillExecutionWorkspace(workspaceDir, input), skillID)
		if err != nil {
			return nil, err
		}
		if resolved.Skill == nil {
			return nil, fmt.Errorf("unknown skill: %s", skillID)
		}
		if err := resolved.Skill.Validate(input); err != nil {
			return nil, fmt.Errorf("skill %s: %w", resolved.ID, err)
		}
		result, err := resolved.Skill.Execute(ctx, input)
		if err != nil {
			return nil, err
		}
		return sockipc.SkillResultToMap(result.Data, result.Success, result.Error), nil
	}
}
