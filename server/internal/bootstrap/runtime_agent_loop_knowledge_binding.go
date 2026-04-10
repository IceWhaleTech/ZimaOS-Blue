package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
)

func bindRuntimeKnowledgeResolverToAgentRunner(runner *agent.Runner, workspaceDir string) {
	if runner == nil {
		return
	}
	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return
	}
	runner.SetKnowledgeResolver(knowledge.NewService(knowledge.ServiceOptions{
		WorkspaceDir: workspaceDir,
		RepoRoot:     resolveKnowledgeRepoRoot(workspaceDir),
	}))
}
