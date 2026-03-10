package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// RuntimeState represents the explicit orchestrator FSM state.
type RuntimeState string

const (
	RuntimeStateIntake      RuntimeState = "INTAKE"
	RuntimeStateClarify     RuntimeState = "CLARIFY"
	RuntimeStatePlan        RuntimeState = "PLAN"
	RuntimeStateConfirmGate RuntimeState = "CONFIRM_GATE"
	RuntimeStateExecute     RuntimeState = "EXECUTE"
	RuntimeStateVerify      RuntimeState = "VERIFY"
	RuntimeStateReflect     RuntimeState = "REFLECT"
	RuntimeStateReport      RuntimeState = "REPORT"
	RuntimeStateRecover     RuntimeState = "RECOVER"
	RuntimeStateDone        RuntimeState = "DONE"
	RuntimeStateAborted     RuntimeState = "ABORTED"
)

// RuntimeAuditEvent records deterministic state transitions and routing decisions.
type RuntimeAuditEvent struct {
	Timestamp  time.Time       `json:"timestamp"`
	From       RuntimeState    `json:"from,omitempty"`
	To         RuntimeState    `json:"to,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Capability *CapabilityInfo `json:"capability,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// CapabilityKind identifies where a call is routed.
type CapabilityKind string

const (
	CapabilityKindTool  CapabilityKind = "tool"
	CapabilityKindSkill CapabilityKind = "skill"
	CapabilityKindMCP   CapabilityKind = "mcp"
)

// CapabilityInfo standardizes call metadata for deterministic routing/auditing.
type CapabilityInfo struct {
	Name             string         `json:"name"`
	Kind             CapabilityKind `json:"kind"`
	RiskLevel        string         `json:"risk_level"`
	DeterminismScore float64        `json:"determinism_score"`
	Idempotent       bool           `json:"idempotent"`
	LatencyProfile   string         `json:"latency_profile,omitempty"`
	CostProfile      string         `json:"cost_profile,omitempty"`
}

var runtimeTransitions = map[RuntimeState]map[RuntimeState]struct{}{
	RuntimeStateIntake: {
		RuntimeStateClarify: {},
		RuntimeStatePlan:    {},
		RuntimeStateAborted: {},
	},
	RuntimeStateClarify: {
		RuntimeStatePlan:    {},
		RuntimeStateReflect: {},
		RuntimeStateReport:  {},
		RuntimeStateAborted: {},
	},
	RuntimeStatePlan: {
		RuntimeStateConfirmGate: {},
		RuntimeStateExecute:     {},
		RuntimeStateReflect:     {},
		RuntimeStateReport:      {},
		RuntimeStateAborted:     {},
	},
	RuntimeStateConfirmGate: {
		RuntimeStateExecute: {},
		RuntimeStatePlan:    {},
		RuntimeStateRecover: {},
		RuntimeStateReflect: {},
		RuntimeStateReport:  {},
		RuntimeStateAborted: {},
	},
	RuntimeStateExecute: {
		RuntimeStateVerify:      {},
		RuntimeStateConfirmGate: {},
		RuntimeStateRecover:     {},
		RuntimeStateReflect:     {},
		RuntimeStateReport:      {},
		RuntimeStateAborted:     {},
	},
	RuntimeStateVerify: {
		RuntimeStateReflect: {},
		RuntimeStateReport:  {},
		RuntimeStateRecover: {},
		RuntimeStateAborted: {},
	},
	RuntimeStateRecover: {
		RuntimeStateExecute:     {},
		RuntimeStateVerify:      {},
		RuntimeStateConfirmGate: {},
		RuntimeStateReflect:     {},
		RuntimeStateReport:      {},
		RuntimeStateAborted:     {},
	},
	RuntimeStateReflect: {
		RuntimeStateReport: {},
		RuntimeStateDone:   {},
	},
	RuntimeStateReport: {
		RuntimeStateDone: {},
	},
	RuntimeStateDone:    {},
	RuntimeStateAborted: {},
}

func canTransition(from, to RuntimeState) bool {
	if from == "" {
		return to == RuntimeStateIntake
	}
	allowed, ok := runtimeTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}

type runtimePlan struct {
	Goal                 string   `json:"goal"`
	Subtasks             []string `json:"subtasks"`
	RequiresConfirmation []string `json:"requires_confirmation"`
	SuccessCriteria      []string `json:"success_criteria"`
	FallbackPlan         []string `json:"fallback_plan"`
}

func parseRuntimePlan(content string) (runtimePlan, error) {
	var out runtimePlan

	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var rawObj struct {
		Goal     string `json:"goal"`
		Subtasks []struct {
			Description string `json:"description"`
		} `json:"subtasks"`
		RequiresConfirmation []string `json:"requires_confirmation"`
		SuccessCriteria      []string `json:"success_criteria"`
		FallbackPlan         []string `json:"fallback_plan"`
	}
	if err := json.Unmarshal([]byte(trimmed), &rawObj); err == nil && len(rawObj.Subtasks) > 0 {
		out.Goal = strings.TrimSpace(rawObj.Goal)
		for _, st := range rawObj.Subtasks {
			if d := strings.TrimSpace(st.Description); d != "" {
				out.Subtasks = append(out.Subtasks, d)
			}
		}
		out.RequiresConfirmation = dedupeStrings(rawObj.RequiresConfirmation)
		out.SuccessCriteria = dedupeStrings(rawObj.SuccessCriteria)
		out.FallbackPlan = dedupeStrings(rawObj.FallbackPlan)
		if len(out.Subtasks) > 0 {
			return fillRuntimePlanDefaults(out), nil
		}
	}

	var rawSteps []struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(trimmed), &rawSteps); err != nil {
		return runtimePlan{}, fmt.Errorf("failed to parse runtime plan: %w", err)
	}
	for _, step := range rawSteps {
		if d := strings.TrimSpace(step.Description); d != "" {
			out.Subtasks = append(out.Subtasks, d)
		}
	}
	if len(out.Subtasks) == 0 {
		return runtimePlan{}, fmt.Errorf("runtime plan has no subtasks")
	}
	return fillRuntimePlanDefaults(out), nil
}

func fillRuntimePlanDefaults(p runtimePlan) runtimePlan {
	if len(p.SuccessCriteria) == 0 {
		p.SuccessCriteria = []string{"core task output is produced", "no blocking errors in final result"}
	}
	if len(p.FallbackPlan) == 0 {
		p.FallbackPlan = []string{"retry once with narrower scope", "ask user to choose next recovery strategy"}
	}
	return p
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func classifyCapability(name, argsJSON string) CapabilityInfo {
	c := CapabilityInfo{
		Name:             name,
		Kind:             CapabilityKindTool,
		RiskLevel:        "low",
		DeterminismScore: 0.92,
		Idempotent:       true,
		LatencyProfile:   "low",
		CostProfile:      "low",
	}
	if strings.HasPrefix(name, "mcp_") || strings.HasPrefix(name, "mcp.") {
		c.Kind = CapabilityKindMCP
		c.DeterminismScore = 0.62
		c.LatencyProfile = "medium"
		c.CostProfile = "medium"
	}
	if name == "exec" {
		cmd := parseExecCommand(argsJSON)
		if strings.HasPrefix(cmd, "blue ") {
			c.Kind = CapabilityKindSkill
			c.DeterminismScore = 0.78
			c.LatencyProfile = "medium"
		}
		risk := tools.AnalyzeRisk(cmd)
		c.RiskLevel = string(risk.Level)
		c.Idempotent = risk.Total < 30
		if risk.Total >= 60 {
			c.CostProfile = "high"
		}
	}
	if !isReadLikeTool(name) {
		c.Idempotent = false
	}
	return c
}

func parseExecCommand(argsJSON string) string {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		return ""
	}
	cmd, _ := raw["command"].(string)
	return strings.TrimSpace(cmd)
}

func isReadLikeTool(name string) bool {
	switch name {
	case "read", "file_read", "memory", "web_search", "analyze", "ui_reviewer", "ask":
		return true
	default:
		return false
	}
}

func shouldRequireConfirm(cap CapabilityInfo, contextText string) bool {
	if cap.Name == "ask" {
		return false
	}
	if cap.RiskLevel == "high" || cap.RiskLevel == "critical" {
		return true
	}
	lower := strings.ToLower(contextText)
	if strings.Contains(lower, "production") || strings.Contains(lower, "delete") || strings.Contains(lower, "drop") || strings.Contains(lower, "publish") {
		return true
	}
	return false
}

func requiresClarification(goal string) bool {
	g := strings.TrimSpace(goal)
	if g == "" {
		return true
	}
	lower := strings.ToLower(g)
	return strings.Contains(lower, "tbd") || strings.Contains(lower, "to be decided")
}

func planNeedsConfirmation(plan runtimePlan) bool {
	if len(plan.RequiresConfirmation) > 0 {
		return true
	}
	for _, step := range plan.Subtasks {
		lower := strings.ToLower(step)
		if strings.Contains(lower, "production") || strings.Contains(lower, "delete") || strings.Contains(lower, "drop") || strings.Contains(lower, "publish") {
			return true
		}
	}
	return false
}
