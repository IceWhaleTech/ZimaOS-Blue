package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	DefaultPromptPolicyVersion = "2026-04-03"
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
		return "tool_guidance_v5|toolless_nudge_v9|post_tool_nudge_v3|context_rules_v1"
	default:
		return "tool_guidance_v5|toolless_nudge_v9|post_tool_nudge_v3|context_rules_v1"
	}
}

func (p PromptPolicy) ToolGuidanceConstraints() string {
	return "Tool guidance constraints: (1) Never print tool-call syntax as plain text (no {\"cmd\":...}, {\"command\":...}, ```tool, <exec>). (2) When using tools, emit structured tool_calls only, with valid JSON arguments. (3) One step at a time: prefer one high-confidence tool call per round, then wait for results. (4) Only use listed tools; do not invent capabilities. (5) For exec, arguments must contain a concrete non-empty command without placeholders. (6) If tools are unavailable or unnecessary, provide direct executable steps instead of fake calls. (7) Built-in tools first; MCP fallback only when needed. (8) External CLI skills are not native `blue <skill>` subcommands; when a skill manual documents a terminal binary, run it through `blue exec command='...'` instead of inventing commands like `blue summarize`. (9) Treat documentation-only examples and removed legacy helper names as docs, not live runtime tool targets. (10) Prefer `web_query` as the unified public-web tool; `web_search`, `web_fetch`, and `web_read` are compatibility aliases only when exposed, and `browser` is the fallback for login or interaction. (11) For reminder requests, call `reminder` directly with action/message/time and do not run `blue reminder --help`. (12) For large file writes, prefer transactional tools when available: use `write_begin`, then `write_chunk`, then `write_commit`. Otherwise never send one huge `write` payload: write the first chunk, then continue with smaller chunks using `append=true`."
}

func (p PromptPolicy) ToollessAutoContinueNudge(agentMode bool, reason string) string {
	if agentMode {
		if reason == "summary_intro" {
			return "Your previous reply started a summary intro but stopped before the actual summary. Continue the response immediately WITHOUT calling tools. Keep it concise and structured: (1) key findings, (2) supporting evidence from completed tool results, (3) an optional-help section in the same language as the user, headed like the localized equivalent of `If you'd like, I can also help with:`, with 1-3 concrete, low-risk optional help offers phrased like the localized equivalent of `If you'd like, I can help you ...`."
		}
		if reason == "missing_next_steps" {
			return "You already provided a completion summary but missed the required next-step guidance. Reply WITHOUT calling tools. Keep the completion concise, then add an optional-help section in the same language as the user, headed like the localized equivalent of `If you'd like, I can also help with:`, with 1-3 concrete, low-risk optional help offers phrased like the localized equivalent of `If you'd like, I can help you ...`, not commands for the user. If no further action is needed, explicitly include the localized equivalent of `1. No further action needed.` Stop only if the user explicitly asks to stop."
		}
		if reason == "missing_todo" {
			return "Agent mode checklist bootstrap required. FIRST output a canonical TODO checklist using markdown checkboxes (`- [ ] step`) for all remaining concrete tasks. Then execute the first unchecked item with tools. Reprint the full checklist with updated checkbox states before each new action or summary, and continue until all items are checked. " + p.ToolGuidanceConstraints() + " Stop only if the user explicitly asks to stop."
		}
		if reason == "pending_todo" {
			return "A canonical TODO checklist already exists. In this turn, FIRST re-output the full checklist with updated checkbox states. Then execute the first unchecked item immediately by emitting at least one real tool call (prefer `exec`). If no tool is needed, provide a concrete completion summary with deliverables. Keep the checklist canonical by rewriting the full checklist on each progress turn so status stays accurate. " + p.ToolGuidanceConstraints() + " Stop only if the user explicitly asks to stop."
		}
		if reason == "todo_reconcile" {
			return "Your reply sounds like a wrap-up, but the canonical TODO checklist still shows pending work. Before ending, FIRST reconcile checklist state. Prefer calling `plan_update` for the completed items; if plan tools are unavailable, re-output the full canonical checklist with corrected checkbox states. Only after the checklist is synchronized may you provide the final summary. If the task is actually complete, mark the remaining items complete before stopping. " + p.ToolGuidanceConstraints() + " Stop only if the user explicitly asks to stop."
		}
		return "You described what to do but did not call any tools. Now actually execute by calling available tools (especially exec for file creation/edit/run steps). Do not describe - act. " + p.ToolGuidanceConstraints() + " Keep agent mode in a continuous improvement loop: after each completed action, find the next concrete improvement and execute it while continuing from the existing canonical TODO checklist. Update checklist status first, and reprint the full checklist with updated checkbox states before the next action or summary. Stop only if the user explicitly asks to stop."
	}
	if reason == "summary_intro" {
		return "Your previous reply started a summary intro but stopped early. Continue immediately WITHOUT calling tools. Provide: (1) concise key findings, (2) key evidence from completed tool results, and (3) an optional-help section in the same language as the user, headed like the localized equivalent of `If you'd like, I can also help with:`, with 1-3 concrete optional help offers phrased like the localized equivalent of `If you'd like, I can help you ...` (or explicitly state no further action is needed)."
	}
	if reason == "missing_next_steps" {
		return "You already provided a completion summary but missed next-step guidance. Reply with a concise summary plus an optional-help section in the same language as the user, headed like the localized equivalent of `If you'd like, I can also help with:`, and 1-3 concrete optional help offers phrased like the localized equivalent of `If you'd like, I can help you ...` (or explicitly state no further action is needed)."
	}
	return "You described what to do but did not call any tools. Now actually execute by calling available tools (especially exec for file creation/edit/run steps). Do not describe - act. " + p.ToolGuidanceConstraints()
}

func (p PromptPolicy) PostToolAutoContinueNudge(agentMode bool) string {
	if agentMode {
		return "Continue the agent loop using the existing canonical TODO checklist. The tools above have been executed successfully. Review the results, reprint the full checklist with updated checkbox states, identify the next concrete improvement opportunity for the next unchecked TODO, execute it, and repeat. " + p.ToolGuidanceConstraints() + " Stop only if the user explicitly asks to stop."
	}
	return "Continue with the task. The tools above have been executed successfully. Review the results and proceed with the next step, or provide a summary if the task is complete. " + p.ToolGuidanceConstraints()
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
