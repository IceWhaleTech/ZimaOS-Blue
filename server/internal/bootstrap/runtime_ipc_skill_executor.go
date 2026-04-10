package bootstrap

import (
	"context"
	"fmt"
	"strings"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeIPCSkillExecutor struct {
	skills       runtimeSkillRegistrySource
	toolRegistry *tools.Registry
	workspaceDir string
}

func newRuntimeIPCSkillExecutor(services *Services, workspaceDir string) sockipc.SkillExecutor {
	if services == nil {
		return nil
	}
	return runtimeIPCSkillExecutor{
		skills:       services.SkillRegistry,
		toolRegistry: services.ToolRegistry,
		workspaceDir: workspaceDir,
	}
}

func (e runtimeIPCSkillExecutor) Execute(ctx context.Context, skillID string, input map[string]any) (map[string]string, error) {
	resolved, err := resolveRuntimeSkillForExecution(e.skills, runtimeSkillExecutionWorkspace(e.workspaceDir, input), skillID)
	if err == nil && resolved.Skill != nil {
		result, execErr := resolved.Skill.Execute(ctx, input)
		if execErr != nil {
			return nil, execErr
		}
		return sockipc.SkillResultToMap(result.Data, result.Success, result.Error), nil
	}
	if err != nil && !strings.Contains(err.Error(), "unknown skill:") {
		return nil, err
	}

	if data, handled, err := tryExecuteToolFallback(ctx, e.toolRegistry, skillID, input); handled {
		return data, err
	}

	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("unknown skill: %s", skillID)
}

func (e runtimeIPCSkillExecutor) Help(_ context.Context, skillID string, input map[string]any) (map[string]string, error) {
	resolved, err := resolveRuntimeSkillForExecution(e.skills, runtimeSkillExecutionWorkspace(e.workspaceDir, input), skillID)
	if err != nil {
		return nil, err
	}
	if resolved.Skill == nil {
		return nil, fmt.Errorf("unknown skill: %s", skillID)
	}
	return skillpkg.HelpData(resolved.Skill.Manifest(), resolved.ID), nil
}
