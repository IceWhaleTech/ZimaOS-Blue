package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type PlannerDecisionStatus string

const (
	PlannerDecisionContinue PlannerDecisionStatus = "continue"
	PlannerDecisionComplete PlannerDecisionStatus = "complete"
	PlannerDecisionBlocked  PlannerDecisionStatus = "blocked"
	PlannerDecisionUnknown  PlannerDecisionStatus = "unknown"
)

type PlannerToolCall struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

type PlannerDecision struct {
	Status     PlannerDecisionStatus `json:"status"`
	Reason     string                `json:"reason"`
	NextTool   *PlannerToolCall      `json:"next_tool,omitempty"`
	Assertions []PlannerAssertion    `json:"assertions,omitempty"`
}

type PlannerInput struct {
	Model              string
	Goal               string
	PlanSummary        string
	Step               PlanStep
	PlannerRound       int
	MaxRounds          int
	GroundState        *GroundTruthState
	ToolCatalog        []llm.Tool
	PriorToolCallIDs   []string
	PreviousViolations []string
	KnowledgeContext   string
	RoutingContract    string
	CoordinationCtx    string
}

type GroundedPlanner struct {
	llm LLMCaller
}

func NewGroundedPlanner(llm LLMCaller) *GroundedPlanner {
	return &GroundedPlanner{llm: llm}
}

func (p *GroundedPlanner) Decide(ctx context.Context, input PlannerInput) (*PlannerDecision, error) {
	if p == nil || p.llm == nil {
		return nil, fmt.Errorf("grounded planner is not configured")
	}
	resp, err := p.llm.Chat(ctx, llm.ChatRequest{
		Model: firstNonEmptyString(input.Model, "auto"),
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: buildGroundedPlannerSystemPrompt(input.ToolCatalog)},
			{Role: llm.RoleUser, Content: buildGroundedPlannerUserPrompt(input)},
		},
		MaxTokens:   800,
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}
	return parsePlannerDecision(resp.Message.Content)
}

func buildGroundedPlannerSystemPrompt(tools []llm.Tool) string {
	var sb strings.Builder
	sb.WriteString("You are the Planner in a hallucination-safe runtime.\n")
	sb.WriteString("You NEVER execute tools and you NEVER invent tool results or filesystem facts.\n\n")
	sb.WriteString("Return ONLY one JSON object with this schema:\n")
	sb.WriteString("{\n")
	sb.WriteString(`  "status": "continue|complete|blocked|unknown",` + "\n")
	sb.WriteString(`  "reason": "short reason",` + "\n")
	sb.WriteString(`  "next_tool": {"tool":"name","args":{}} ,` + "\n")
	sb.WriteString(`  "assertions": [{"type":"file_exists","path":"..."},{"type":"tool_called","tool":"ls"}]` + "\n")
	sb.WriteString("}\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- At most one next_tool.\n")
	sb.WriteString("- status=continue requires next_tool.\n")
	sb.WriteString("- status=complete, blocked, or unknown must omit next_tool.\n")
	sb.WriteString("- Do not claim that files exist, that ls showed something, or that file contents are known unless a tool will verify it.\n")
	sb.WriteString("- If evidence is insufficient, use unknown instead of guessing.\n")
	sb.WriteString("- If the execution routing contract provides a canonical CLI action and exec/bash is available, choose that canonical CLI action first instead of substituting direct alternative tools.\n")
	sb.WriteString("- Assertions are optional and must be limited to file_exists(path) and tool_called(name).\n")
	sb.WriteString("- Keep reasons concise.\n")
	if hasPlannerTool(tools, "subagents") {
		sb.WriteString("- If subagents is available, you may delegate bounded independent work after you identify the exact purpose of the worker.\n")
		sb.WriteString("- Follow the coordinator workflow: research -> synthesis -> implementation -> verification.\n")
		sb.WriteString("- After research, synthesize the findings yourself before follow-up work. Write self-contained worker briefs with concrete files, constraints, and done criteria.\n")
		sb.WriteString("- Never issue vague follow-up prompts such as 'based on your findings'; that delegates understanding instead of grounding the plan.\n")
		sb.WriteString("- Continue the same worker when context overlap is high or when it is correcting its own failed attempt.\n")
		sb.WriteString("- Spawn a fresh worker for narrow implementation after broad research, for clean-slate retries after a wrong approach, or for independent verification with fresh context.\n")
		sb.WriteString("- serialize overlapping writes and use the shared scratchpad context for handoffs, task claims, blockers, and interim findings.\n")
	}
	if len(tools) > 0 {
		sb.WriteString("\nAvailable tools:\n")
		for _, tool := range tools {
			sb.WriteString("- ")
			sb.WriteString(tool.Name)
			if desc := strings.TrimSpace(tool.Description); desc != "" {
				sb.WriteString(": ")
				sb.WriteString(desc)
			}
			sb.WriteByte('\n')
		}
	}
	return strings.TrimSpace(sb.String())
}

func buildGroundedPlannerUserPrompt(input PlannerInput) string {
	var sb strings.Builder
	sb.WriteString("Goal:\n")
	sb.WriteString(strings.TrimSpace(input.Goal))
	sb.WriteString("\n\nCurrent step:\n")
	sb.WriteString(strings.TrimSpace(input.Step.Description))
	sb.WriteString("\n\nPlan summary:\n")
	sb.WriteString(strings.TrimSpace(input.PlanSummary))
	sb.WriteString("\n\nPlanner round:\n")
	sb.WriteString(fmt.Sprintf("%d/%d", input.PlannerRound, input.MaxRounds))
	sb.WriteString("\n\nGrounded state summary:\n")
	sb.WriteString(BuildGroundStateSummary(input.GroundState))
	if strings.TrimSpace(input.KnowledgeContext) != "" {
		sb.WriteString("\n\nRelevant knowledge:\n")
		sb.WriteString(strings.TrimSpace(input.KnowledgeContext))
	}
	if len(input.PriorToolCallIDs) > 0 {
		sb.WriteString("\n\nPrior tool_call_ids:\n")
		for _, id := range input.PriorToolCallIDs {
			sb.WriteString("- ")
			sb.WriteString(id)
			sb.WriteByte('\n')
		}
	}
	if len(input.PreviousViolations) > 0 {
		sb.WriteString("\nPrevious verifier violations:\n")
		for _, item := range input.PreviousViolations {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteByte('\n')
		}
	}
	if strings.TrimSpace(input.RoutingContract) != "" {
		sb.WriteString("\nExecution routing contract:\n")
		sb.WriteString(strings.TrimSpace(input.RoutingContract))
		sb.WriteByte('\n')
	}
	if strings.TrimSpace(input.CoordinationCtx) != "" {
		sb.WriteString("\n")
		sb.WriteString(strings.TrimSpace(input.CoordinationCtx))
		sb.WriteByte('\n')
	}
	sb.WriteString("\nDecide the single next grounded action.")
	return strings.TrimSpace(sb.String())
}

func hasPlannerTool(tools []llm.Tool, name string) bool {
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == name {
			return true
		}
	}
	return false
}

func parsePlannerDecision(content string) (*PlannerDecision, error) {
	trimmed := trimStructuredContent(content)
	if trimmed == "" {
		return nil, fmt.Errorf("planner returned empty content")
	}
	var out PlannerDecision
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("planner decision is not valid JSON: %w", err)
	}
	out.Status = PlannerDecisionStatus(strings.ToLower(strings.TrimSpace(string(out.Status))))
	out.Reason = strings.TrimSpace(out.Reason)
	if out.NextTool != nil {
		out.NextTool.Tool = strings.TrimSpace(out.NextTool.Tool)
		if out.NextTool.Args == nil {
			out.NextTool.Args = map[string]any{}
		}
	}
	switch out.Status {
	case PlannerDecisionContinue:
		if out.NextTool == nil || out.NextTool.Tool == "" {
			return nil, fmt.Errorf("planner decision continue requires next_tool")
		}
	case PlannerDecisionComplete, PlannerDecisionBlocked, PlannerDecisionUnknown:
		out.NextTool = nil
	default:
		return nil, fmt.Errorf("unsupported planner status %q", out.Status)
	}
	return &out, nil
}
