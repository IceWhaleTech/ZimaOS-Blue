package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const (
	toolApprovalRiskScoreTimeout         = 3 * time.Second
	toolApprovalRiskScoreMaxTokens       = 240
	toolApprovalRiskScoreAskThreshold    = 60.0
	toolApprovalRiskScoreMinConfidence   = 0.55
	toolApprovalRiskScoreMaxStringLen    = 240
	toolApprovalRiskScoreMaxSliceItems   = 6
	toolApprovalRiskScoreMaxMapEntries   = 12
	toolApprovalRiskScoreMaxSchemaFields = 12
	toolApprovalRiskScoreMaxDepth        = 3
)

// ToolApprovalRiskScore captures a context-aware approval recommendation for a
// tool call. The scorer is advisory: hard-coded deny/ask policies still win.
type ToolApprovalRiskScore struct {
	Score           float64  `json:"score"`
	Confidence      float64  `json:"confidence"`
	RiskLevel       string   `json:"risk_level"`
	RecommendedMode string   `json:"recommended_mode"`
	Reason          string   `json:"reason"`
	Signals         []string `json:"signals,omitempty"`
}

// ToolApprovalRiskScorer evaluates whether an otherwise auto-approved tool call
// should be escalated into an explicit user approval request.
type ToolApprovalRiskScorer interface {
	ScoreToolApproval(ctx context.Context, req tools.ToolApprovalRequest) (*ToolApprovalRiskScore, error)
}

type toolApprovalRiskLLM interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type llmToolApprovalRiskScorer struct {
	llm      toolApprovalRiskLLM
	registry *tools.Registry
	timeout  time.Duration
}

// NewLLMToolApprovalRiskScorer creates a best-effort scorer that uses the
// auxiliary LLM path to classify ambiguous tool calls. Returning nil keeps the
// runtime on static approval rules only.
func NewLLMToolApprovalRiskScorer(caller llm.Provider, registry *tools.Registry) ToolApprovalRiskScorer {
	if caller == nil {
		return nil
	}
	return &llmToolApprovalRiskScorer{
		llm:      caller,
		registry: registry,
		timeout:  toolApprovalRiskScoreTimeout,
	}
}

func (s *llmToolApprovalRiskScorer) ScoreToolApproval(ctx context.Context, req tools.ToolApprovalRequest) (*ToolApprovalRiskScore, error) {
	if s == nil || s.llm == nil {
		return nil, fmt.Errorf("tool approval risk scorer is not configured")
	}
	scoreCtx := ctx
	cancel := func() {}
	if s.timeout > 0 {
		scoreCtx, cancel = context.WithTimeout(ctx, s.timeout)
	}
	defer cancel()

	resp, err := s.llm.Chat(scoreCtx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: buildToolApprovalRiskSystemPrompt()},
			{Role: llm.RoleUser, Content: s.buildToolApprovalRiskUserPrompt(req)},
		},
		MaxTokens:   toolApprovalRiskScoreMaxTokens,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, err
	}
	return parseToolApprovalRiskScore(resp)
}

func (s *llmToolApprovalRiskScorer) buildToolApprovalRiskUserPrompt(req tools.ToolApprovalRequest) string {
	payload := map[string]interface{}{
		"tool_call": map[string]interface{}{
			"name":          strings.TrimSpace(req.ToolName),
			"tool_call_id":  strings.TrimSpace(req.ToolCallID),
			"route_kind":    strings.TrimSpace(string(req.RouteKind)),
			"session_id":    strings.TrimSpace(req.SessionID),
			"user_id":       strings.TrimSpace(req.UserID),
			"provider":      strings.TrimSpace(req.Provider),
			"provider_id":   strings.TrimSpace(req.ProviderID),
			"model":         strings.TrimSpace(req.Model),
			"agent_id":      strings.TrimSpace(req.AgentID),
			"risk_level":    normalizeRiskLevel(req.RiskLevel),
			"binding_hash":  strings.TrimSpace(req.BindingHash),
			"policy_source": strings.TrimSpace(req.PolicySource),
			"arguments":     summarizeApprovalRiskValue(req.Arguments, 0),
		},
	}
	if def, ok := s.lookupDefinition(strings.TrimSpace(req.ToolName)); ok {
		payload["tool_definition"] = summarizeApprovalRiskDefinition(def)
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func (s *llmToolApprovalRiskScorer) lookupDefinition(name string) (tools.ToolDefinition, bool) {
	if s == nil || s.registry == nil || strings.TrimSpace(name) == "" {
		return tools.ToolDefinition{}, false
	}
	return s.registry.LookupDefinition(strings.TrimSpace(name))
}

func (h *ApprovalHandler) applyRiskScore(
	ctx context.Context,
	req tools.ToolApprovalRequest,
	mode string,
	source string,
	riskLevel string,
) (string, string, string) {
	if !shouldScoreToolApprovalRequest(req, mode, riskLevel) {
		return mode, source, riskLevel
	}
	userID := nonEmpty(strings.TrimSpace(req.UserID), tools.GetUserID(ctx))
	if !h.hasApprovalDeliveryTarget(userID) {
		return mode, source, riskLevel
	}

	h.mu.RLock()
	scorer := h.riskScorer
	h.mu.RUnlock()
	if scorer == nil {
		return mode, source, riskLevel
	}

	scored, err := scorer.ScoreToolApproval(ctx, req)
	if err != nil || scored == nil {
		return mode, source, riskLevel
	}

	riskLevel = maxApprovalRiskLevel(riskLevel, scored.RiskLevel)
	if shouldEscalateToolApprovalMode(riskLevel, scored) {
		return "ask", "approval.llm_risk_score", riskLevel
	}
	return mode, source, riskLevel
}

func (h *ApprovalHandler) hasApprovalDeliveryTarget(userID string) bool {
	if h == nil {
		return false
	}
	h.mu.RLock()
	broker := h.broker
	h.mu.RUnlock()
	if broker == nil {
		return false
	}
	return broker.ClientCount(nonEmpty(strings.TrimSpace(userID), "default")) > 0
}

func shouldScoreToolApprovalRequest(req tools.ToolApprovalRequest, mode string, riskLevel string) bool {
	if normalizeApprovalMode(mode) != "auto" {
		return false
	}
	if approvalRiskRank(riskLevel) >= approvalRiskRank("medium") {
		return true
	}
	if isPotentiallyStatefulTool(req.ToolName) {
		return true
	}
	return approvalArgsSuggestSideEffects(req.Arguments)
}

func shouldEscalateToolApprovalMode(baseRisk string, scored *ToolApprovalRiskScore) bool {
	if scored == nil || normalizeScoredApprovalMode(scored.RecommendedMode) != "ask" {
		return false
	}
	if approvalRiskRank(baseRisk) >= approvalRiskRank("medium") {
		return true
	}
	if scored.Confidence >= toolApprovalRiskScoreMinConfidence {
		return true
	}
	if scored.Score >= toolApprovalRiskScoreAskThreshold {
		return true
	}
	return approvalRiskRank(scored.RiskLevel) > approvalRiskRank(baseRisk)
}

func parseToolApprovalRiskScore(resp *llm.ChatResponse) (*ToolApprovalRiskScore, error) {
	if resp == nil {
		return nil, fmt.Errorf("tool approval scorer returned no response")
	}
	content := strings.TrimSpace(resp.Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var parsed ToolApprovalRiskScore
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("parse tool approval score: %w", err)
	}
	parsed.Score = normalizeToolApprovalScore(parsed.Score)
	parsed.Confidence = normalizeToolApprovalConfidence(parsed.Confidence)
	parsed.RiskLevel = normalizeRiskLevel(parsed.RiskLevel)
	parsed.RecommendedMode = normalizeScoredApprovalMode(parsed.RecommendedMode)
	parsed.Reason = strings.TrimSpace(parsed.Reason)
	parsed.Signals = compactApprovalSignals(parsed.Signals)
	return &parsed, nil
}

func buildToolApprovalRiskSystemPrompt() string {
	return strings.TrimSpace(`You are a tool approval risk scorer for ZimaOS Blue.

Return JSON only:
{
  "score": 0-100,
  "confidence": 0-1,
  "risk_level": "low|medium|high|critical",
  "recommended_mode": "auto|ask",
  "reason": "brief grounded rationale",
  "signals": ["short signal", "..."]
}

Interpret "recommended_mode":
- "auto" only when the tool call is clearly low-impact, read-only, or safely reversible.
- "ask" when the action can modify files, change system state, interact with external websites, send or publish content, access sensitive data, incur cost, or when intent is ambiguous.

Rules:
- Hard blocks are handled elsewhere. Prefer "ask" over "deny".
- Base your judgment on the tool name, its definition, and the concrete arguments.
- Be conservative when the action touches external systems or user data.
- Keep the reason short and specific.`)
}

func summarizeApprovalRiskDefinition(def tools.ToolDefinition) map[string]interface{} {
	out := map[string]interface{}{
		"name":        strings.TrimSpace(def.Name),
		"description": truncateApprovalRiskString(def.Description),
	}
	if risk := strings.TrimSpace(def.RiskLevel); risk != "" {
		out["risk_level"] = normalizeRiskLevel(risk)
	}
	if params, ok := def.Parameters["properties"].(map[string]interface{}); ok && len(params) > 0 {
		keys := make([]string, 0, len(params))
		for key := range params {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > toolApprovalRiskScoreMaxSchemaFields {
			keys = append(keys[:toolApprovalRiskScoreMaxSchemaFields], fmt.Sprintf("+%d_more", len(keys)-toolApprovalRiskScoreMaxSchemaFields))
		}
		out["parameter_keys"] = keys
	}
	if required, ok := def.Parameters["required"].([]string); ok && len(required) > 0 {
		out["required"] = required
	} else if requiredAny, ok := def.Parameters["required"].([]interface{}); ok && len(requiredAny) > 0 {
		out["required"] = summarizeApprovalRiskValue(requiredAny, 0)
	}
	return out
}

func summarizeApprovalRiskValue(value interface{}, depth int) interface{} {
	if value == nil {
		return nil
	}
	if depth >= toolApprovalRiskScoreMaxDepth {
		return "[truncated]"
	}
	switch v := value.(type) {
	case string:
		return truncateApprovalRiskString(v)
	case []interface{}:
		out := make([]interface{}, 0, minInt(len(v), toolApprovalRiskScoreMaxSliceItems))
		for idx, item := range v {
			if idx >= toolApprovalRiskScoreMaxSliceItems {
				out = append(out, fmt.Sprintf("+%d_more", len(v)-toolApprovalRiskScoreMaxSliceItems))
				break
			}
			out = append(out, summarizeApprovalRiskValue(item, depth+1))
		}
		return out
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]interface{}, minInt(len(keys), toolApprovalRiskScoreMaxMapEntries))
		for idx, key := range keys {
			if idx >= toolApprovalRiskScoreMaxMapEntries {
				out["_truncated"] = fmt.Sprintf("+%d_more", len(keys)-toolApprovalRiskScoreMaxMapEntries)
				break
			}
			out[key] = summarizeApprovalRiskValue(v[key], depth+1)
		}
		return out
	default:
		return value
	}
}

func truncateApprovalRiskString(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= toolApprovalRiskScoreMaxStringLen {
		return trimmed
	}
	return fmt.Sprintf("%s...[truncated %d chars]", trimmed[:toolApprovalRiskScoreMaxStringLen], len(trimmed)-toolApprovalRiskScoreMaxStringLen)
}

func normalizeToolApprovalScore(raw float64) float64 {
	if raw > 0 && raw <= 1 {
		raw *= 100
	}
	switch {
	case raw < 0:
		return 0
	case raw > 100:
		return 100
	default:
		return raw
	}
}

func normalizeToolApprovalConfidence(raw float64) float64 {
	if raw > 1 && raw <= 100 {
		raw /= 100
	}
	switch {
	case raw < 0:
		return 0
	case raw > 1:
		return 1
	default:
		return raw
	}
}

func normalizeScoredApprovalMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ask", "deny":
		return "ask"
	default:
		return "auto"
	}
}

func compactApprovalSignals(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, minInt(len(in), toolApprovalRiskScoreMaxSliceItems))
	for _, raw := range in {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		out = append(out, truncateApprovalRiskString(trimmed))
		if len(out) == toolApprovalRiskScoreMaxSliceItems {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isPotentiallyStatefulTool(toolName string) bool {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "browser",
		"calendar",
		"canvas",
		"cron",
		"edit",
		"email",
		"file_delete",
		"file_write",
		"image",
		"mcp",
		"message",
		"office",
		"push",
		"research",
		"research_run",
		"subagents",
		"web",
		"web_crawl",
		"web_extract",
		"web_fetch",
		"web_query",
		"web_read",
		"write",
		"write_abort",
		"write_begin",
		"write_chunk",
		"write_commit":
		return true
	default:
		return false
	}
}

func approvalArgsSuggestSideEffects(args map[string]interface{}) bool {
	if len(args) == 0 {
		return false
	}
	for key := range args {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "action",
			"append",
			"command",
			"content",
			"create_dirs",
			"html",
			"line",
			"message",
			"new_text",
			"old_text",
			"operation",
			"recursive",
			"replace_all",
			"subject",
			"to",
			"url":
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
