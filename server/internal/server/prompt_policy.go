package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	DefaultPromptPolicyVersion = "2026-03-04"
	DefaultPromptPolicyProfile = "default"
)

// PromptPolicy captures the active prompt strategy profile and computed hash.
type PromptPolicy struct {
	Version string `json:"prompt_policy_version"`
	Profile string `json:"prompt_policy_profile"`
	Hash    string `json:"prompt_policy_hash"`
}

func resolvePromptPolicy(version, profile string) PromptPolicy {
	normalizedVersion := strings.TrimSpace(version)
	if normalizedVersion == "" {
		normalizedVersion = DefaultPromptPolicyVersion
	}

	normalizedProfile := strings.ToLower(strings.TrimSpace(profile))
	switch normalizedProfile {
	case "", "default":
		normalizedProfile = DefaultPromptPolicyProfile
	default:
		normalizedProfile = DefaultPromptPolicyProfile
	}

	sum := sha256.Sum256([]byte(normalizedVersion + "|" + normalizedProfile + "|" + policyFingerprint(normalizedProfile)))
	return PromptPolicy{
		Version: normalizedVersion,
		Profile: normalizedProfile,
		Hash:    hex.EncodeToString(sum[:]),
	}
}

func policyFingerprint(profile string) string {
	// Keep this stable unless policy text/behavior changes.
	switch profile {
	case DefaultPromptPolicyProfile:
		return "tool_guidance_v1|toolless_nudge_v1|post_tool_nudge_v1"
	default:
		return "tool_guidance_v1|toolless_nudge_v1|post_tool_nudge_v1"
	}
}

func (p PromptPolicy) ToolGuidanceConstraints() string {
	return "Tool guidance constraints: (1) Never print tool-call syntax as plain text (no {\"cmd\":...}, {\"command\":...}, ```tool, <exec>). (2) When using tools, emit structured tool_calls only, with valid JSON arguments. (3) Prefer one high-confidence tool call per round, then wait for results before the next call. (4) For exec, arguments must contain a concrete non-empty command without placeholders. (5) If tools are unavailable or unnecessary, provide direct executable steps instead of fake calls. (6) For reminder requests, call `reminder` directly with action/message/time and do not run `blue reminder --help`."
}

func (p PromptPolicy) ToollessAutoContinueNudge(agentMode bool, reason string) string {
	if agentMode {
		if reason == "missing_todo" {
			return "Agent mode checklist bootstrap required. FIRST output a canonical TODO checklist using markdown checkboxes (`- [ ] step`) for all remaining concrete tasks. Then execute the first unchecked item with tools. Now actually execute by calling available tools. Keep updating the same checklist (mark completed items first), and continue until all items are checked. " + p.ToolGuidanceConstraints() + " Stop only if the user explicitly asks to stop."
		}
		if reason == "pending_todo" {
			return "A canonical TODO checklist already exists. Do NOT output another TODO list and do NOT rewrite the current checklist in this turn. Execute the first unchecked item immediately by emitting at least one real tool call (prefer `exec`). If no tool is needed, provide a concrete completion summary with deliverables. " + p.ToolGuidanceConstraints() + " Stop only if the user explicitly asks to stop."
		}
		return "You described what to do but did not call any tools. Now actually execute by calling available tools (especially exec for file creation/edit/run steps). Do not describe - act. " + p.ToolGuidanceConstraints() + " Keep agent mode in a continuous improvement loop: after each completed action, find the next concrete improvement and execute it while continuing from the existing canonical TODO checklist. Update checklist status first, and avoid rewriting the full checklist unless scope changed. Stop only if the user explicitly asks to stop."
	}
	return "You described what to do but did not call any tools. Now actually execute by calling available tools (especially exec for file creation/edit/run steps). Do not describe - act. " + p.ToolGuidanceConstraints()
}

func (p PromptPolicy) PostToolAutoContinueNudge(agentMode bool) string {
	if agentMode {
		return "Continue the agent loop using the existing canonical TODO checklist. The tools above have been executed successfully. Review the results, mark completed items, identify the next concrete improvement opportunity for the next unchecked TODO, execute it, and repeat. Avoid rewriting the full checklist unless scope changed. Stop only if the user explicitly asks to stop."
	}
	return "Continue with the task. The tools above have been executed successfully. Review the results and proceed with the next step, or provide a summary if the task is complete."
}

// AgentLoopPolicy is the normalized loop budget policy used by chat + agent surfaces.
type AgentLoopPolicy struct {
	MaxToolRounds        int `json:"max_tool_rounds"`
	MaxAutoContinue      int `json:"max_auto_continue"`
	PseudoToolCallBudget int `json:"pseudo_tool_call_budget"`
	ActionPledgeBudget   int `json:"action_pledge_budget"`
	MissingTodoBudget    int `json:"missing_todo_budget"`
	PendingTodoBudget    int `json:"pending_todo_budget"`
}

func (p AgentLoopPolicy) String() string {
	return fmt.Sprintf("tool_rounds=%d auto_continue=%d pseudo=%d action_pledge=%d missing_todo=%d pending_todo=%d",
		p.MaxToolRounds, p.MaxAutoContinue, p.PseudoToolCallBudget, p.ActionPledgeBudget, p.MissingTodoBudget, p.PendingTodoBudget)
}
