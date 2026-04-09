package agent

import (
	"os"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	workspacepkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

const (
	taskMetaSubagentsAvailable    = "subagents_available"
	taskMetaSharedScratchpadPath  = "shared_scratchpad_path"
	taskMetaSharedScratchpadRel   = "shared_scratchpad_rel_path"
	taskMetaSharedScratchpadAlias = "shared_scratchpad_alias"
	taskMetaExecutionProfile      = "execution_profile"
	taskMetaCanonicalSkill        = "selected_canonical_skill"
)

type taskCoordinationDetails struct {
	SubagentsAvailable bool
	ScratchpadPath     string
	ScratchpadRelPath  string
	ScratchpadAlias    string
	CanonicalSkill     agentcore.CanonicalSkillID
	ExecutionProfile   agentcore.ExecutionProfile
}

func prepareTaskCoordinationMetadata(task *Task, subagentsAvailable bool) {
	if task == nil {
		return
	}
	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}
	task.Metadata[taskMetaSubagentsAvailable] = subagentsAvailable
	if canonical, profile, ok := coordinationExecutionProfile(task); ok {
		task.Metadata[taskMetaCanonicalSkill] = string(canonical)
		task.Metadata[taskMetaExecutionProfile] = string(profile)
	}

	scratchpadPath := workspacepkg.SharedScratchpadDir(task.WorkspaceRoot)
	if scratchpadPath == "" {
		delete(task.Metadata, taskMetaSharedScratchpadPath)
		delete(task.Metadata, taskMetaSharedScratchpadRel)
		delete(task.Metadata, taskMetaSharedScratchpadAlias)
		return
	}
	_ = os.MkdirAll(scratchpadPath, 0o755)
	task.Metadata[taskMetaSharedScratchpadPath] = scratchpadPath
	task.Metadata[taskMetaSharedScratchpadRel] = workspacepkg.SharedScratchpadRelPath()
	task.Metadata[taskMetaSharedScratchpadAlias] = workspacepkg.SharedScratchpadAlias
}

func coordinationDetailsForTask(task *Task) taskCoordinationDetails {
	details := taskCoordinationDetails{}
	if task == nil {
		return details
	}

	if available, ok := metadataBoolFromMaps(taskMetadataSources(task), taskMetaSubagentsAvailable); ok {
		details.SubagentsAvailable = available
	}

	meta := plannerTaskMetadata(task)
	details.ScratchpadPath = firstNonEmptyString(
		metadataStringValue(meta, taskMetaSharedScratchpadPath),
		workspacepkg.SharedScratchpadDir(task.WorkspaceRoot),
	)
	details.ScratchpadRelPath = firstNonEmptyString(
		metadataStringValue(meta, taskMetaSharedScratchpadRel),
		workspacepkg.SharedScratchpadRelPath(),
	)
	details.ScratchpadAlias = firstNonEmptyString(
		metadataStringValue(meta, taskMetaSharedScratchpadAlias),
		workspacepkg.SharedScratchpadAlias,
	)
	if canonical, profile, ok := coordinationExecutionProfile(task); ok {
		details.CanonicalSkill = canonical
		details.ExecutionProfile = profile
	}
	return details
}

func buildTaskCoordinationPromptContext(task *Task) string {
	details := coordinationDetailsForTask(task)
	if !details.SubagentsAvailable && details.ScratchpadPath == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Coordination context:\n")
	if details.SubagentsAvailable {
		sb.WriteString("- You are the coordinator for worker delegation; do not outsource understanding.\n")
		sb.WriteString("- Default workflow: research -> synthesis -> implementation -> verification.\n")
		sb.WriteString("- After research, synthesize the findings yourself and write a self-contained follow-up brief with concrete files, constraints, and done criteria.\n")
		sb.WriteString("- Continue the same worker when it already has the exact file/error context or is correcting its own failed attempt.\n")
		sb.WriteString("- Spawn a fresh worker when the next task is narrow after broad exploration, when the previous approach polluted context, or when you need independent verification with fresh context.\n")
		sb.WriteString("- Stop a worker promptly if requirements change or you discover it is heading in the wrong direction.\n")
		sb.WriteString("- The subagents tool is available for bounded worker delegation.\n")
		sb.WriteString("- Delegate independent research, implementation, or verification threads when that reduces total latency.\n")
		sb.WriteString("- Parallelize read-only work when useful, but serialize overlapping writes so only one worker edits a file set at a time.\n")
		sb.WriteString("- Bad delegation: \"Based on your findings, fix the bug.\"\n")
		sb.WriteString("- Good delegation: \"Fix the null pointer in src/auth/validate.ts:42 by guarding Session.user before reading user.id. Update the relevant test and report verification output.\"\n")
		switch details.ExecutionProfile {
		case agentcore.ExecutionProfilePreferFork:
			sb.WriteString("- Discover-first route: ")
			sb.WriteString(string(details.CanonicalSkill))
			sb.WriteString(" (execution profile: prefer_fork). Prefer an isolated worker for the implementation/research thread when subagents are available.\n")
			sb.WriteString("- Keep verification in fresh context instead of letting the implementation worker self-certify.\n")
		case agentcore.ExecutionProfileRequireFork:
			sb.WriteString("- Discover-first route: ")
			sb.WriteString(string(details.CanonicalSkill))
			sb.WriteString(" (execution profile: require_fork). Delegate this execution to an isolated worker when subagents are available.\n")
			sb.WriteString("- Keep verification in a separate fresh-context worker; do not reuse the implementation worker for self-verification.\n")
		}
	}
	if details.ScratchpadPath != "" {
		sb.WriteString("- Shared scratchpad root: @")
		sb.WriteString(details.ScratchpadAlias)
		sb.WriteString("/")
		if details.ScratchpadRelPath != "" {
			sb.WriteString(" (workspace-relative: ")
			sb.WriteString(details.ScratchpadRelPath)
			sb.WriteString(")")
		}
		sb.WriteString(".\n")
		sb.WriteString("- Use the shared scratchpad for durable task claims, interim findings, blockers, and worker handoffs. Keep it concise and current.\n")
	}
	return strings.TrimSpace(sb.String())
}

func coordinationExecutionProfile(task *Task) (agentcore.CanonicalSkillID, agentcore.ExecutionProfile, bool) {
	if task == nil {
		return agentcore.CanonicalUnknown, "", false
	}
	meta := plannerTaskMetadata(task)
	candidates := []string{
		metadataStringValue(meta, taskMetaCanonicalSkill),
		metadataStringValue(metadataMapValue(meta, "routing_contract"), "primary_route"),
		metadataStringValue(meta, "primary_route"),
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		switch strings.ToLower(candidate) {
		case "research", "deep_research", "analyze", "ui_reviewer", "ui_review":
			return agentcore.CanonicalResearch, agentcore.ExecutionProfileForSkill(agentcore.CanonicalResearch), true
		}
		if canonical, ok := agentcore.ResolveCanonicalSkill(candidate); ok {
			return canonical, agentcore.ExecutionProfileForSkill(canonical), true
		}
	}
	if profile := strings.TrimSpace(metadataStringValue(meta, taskMetaExecutionProfile)); profile != "" {
		return agentcore.CanonicalUnknown, agentcore.ExecutionProfile(profile), true
	}
	return agentcore.CanonicalUnknown, "", false
}
