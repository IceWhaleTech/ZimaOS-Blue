package agent

import (
	"os"
	"strings"

	workspacepkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

const (
	taskMetaSubagentsAvailable    = "subagents_available"
	taskMetaSharedScratchpadPath  = "shared_scratchpad_path"
	taskMetaSharedScratchpadRel   = "shared_scratchpad_rel_path"
	taskMetaSharedScratchpadAlias = "shared_scratchpad_alias"
)

type taskCoordinationDetails struct {
	SubagentsAvailable bool
	ScratchpadPath     string
	ScratchpadRelPath  string
	ScratchpadAlias    string
}

func prepareTaskCoordinationMetadata(task *Task, subagentsAvailable bool) {
	if task == nil {
		return
	}
	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}
	task.Metadata[taskMetaSubagentsAvailable] = subagentsAvailable

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
