package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindRuntimeExecTool(
	target runtimeExecToolTarget,
	registry *tools.Registry,
	skills runtimeSkillRegistrySource,
	workspaceDir string,
	selectorSource runtimeExecSkillSelectionSource,
	auditStore *tools.ExecAuditStore,
) {
	if target == nil {
		return
	}
	if auditStore != nil {
		target.SetAuditStore(auditStore)
	}
	if registry != nil {
		target.SetToolNames(registry.List())
		target.SetRegistry(registry)
	}
	target.SetPinnedSkills(agentcore.PinnedSkills())
	if executor := newRuntimeExecSkillExecutor(skills, workspaceDir); executor != nil {
		target.SetSkillExecutor(executor)
	}
	if selector := newRuntimeExecSkillSelector(selectorSource); selector != nil {
		target.SetSkillSelector(selector)
	}
}
