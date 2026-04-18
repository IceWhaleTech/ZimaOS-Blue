package server

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

const (
	smallModelPseudoToolRepairTimeout     = 4 * time.Second
	smallModelPseudoToolRepairMaxTokens   = 224
	smallModelPseudoToolRepairTemperature = 0.1
)

func (h *ChatHandler) recoverPseudoToolCallsForContent(ctx context.Context, content string, allowedTools []llm.Tool) ([]llm.ToolCall, string, bool) {
	if recovered, ok := recoverSanitizedPseudoToolCallsFromContent(content, allowedTools); ok {
		return recovered, "deterministic", true
	}
	if recovered, ok := h.repairPseudoToolCallsWithSmallModel(ctx, content, allowedTools); ok {
		return recovered, "small_model", true
	}
	return nil, "", false
}

func (h *ChatHandler) repairPseudoToolCallsWithSmallModel(ctx context.Context, content string, allowedTools []llm.Tool) ([]llm.ToolCall, bool) {
	if !h.shouldAttemptSmallModelPseudoToolRepair(content, allowedTools) {
		return nil, false
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now()
	if h.smallModelBreaker != nil && !h.smallModelBreaker.Allow(now) {
		return nil, false
	}

	smCtx, cancel := context.WithTimeout(ctx, smallModelPseudoToolRepairTimeout)
	defer cancel()

	resp, err := h.smallModel.Generate(smCtx, smallmodel.GenerateRequest{
		Prompt:      buildSmallModelPseudoToolRepairPrompt(content, allowedTools),
		MaxTokens:   smallModelPseudoToolRepairMaxTokens,
		Temperature: smallModelPseudoToolRepairTemperature,
	})
	if err != nil {
		if h.smallModelBreaker != nil && h.smallModelBreaker.RecordFailure(time.Now()) {
			logger.Warn().Msg("[chat] small model circuit breaker opened during pseudo tool-call repair")
		}
		logger.Debug().Err(err).Msg("[chat] small model pseudo tool-call repair failed")
		return nil, false
	}
	if h.smallModelBreaker != nil {
		h.smallModelBreaker.RecordSuccess(time.Now())
	}
	if resp == nil {
		return nil, false
	}

	repaired := strings.TrimSpace(resp.Text)
	if repaired == "" {
		return nil, false
	}
	recovered, ok := recoverSanitizedPseudoToolCallsFromContent(repaired, allowedTools)
	if !ok || len(recovered) == 0 {
		logger.Debug().
			Str("repair_text", truncateUTF8Bytes(repaired, 512)).
			Msg("[chat] small model pseudo tool-call repair returned no valid tool calls")
		return nil, false
	}
	return recovered, true
}

func (h *ChatHandler) shouldAttemptSmallModelPseudoToolRepair(content string, allowedTools []llm.Tool) bool {
	if h == nil || len(allowedTools) == 0 {
		return false
	}
	if h.settingsHandler == nil || !h.settingsHandler.GetSmallModelEnabled() {
		return false
	}
	if !shouldAutoContinueForPseudoToolCall(content) {
		return false
	}
	ready, _, _ := h.smallModelReadinessState()
	return ready
}

func buildSmallModelPseudoToolRepairPrompt(content string, allowedTools []llm.Tool) string {
	var b strings.Builder
	b.WriteString("Repair leaked pseudo tool-calling text into strict JSON.\n")
	b.WriteString("Return ONLY JSON with this shape: {\"tool_calls\":[{\"name\":\"<tool>\",\"arguments\":{...}}]}.\n")
	b.WriteString("If no valid tool call can be repaired, return {\"tool_calls\":[]}.\n")
	b.WriteString("Use ONLY the allowed tool names below.\n")
	b.WriteString("Do not include markdown, prose, XML, or code fences.\n")
	b.WriteString("Allowed tools:\n")
	for _, hint := range smallModelPseudoToolRepairHints(allowedTools) {
		b.WriteString("- ")
		b.WriteString(hint)
		b.WriteByte('\n')
	}
	b.WriteString("Assistant text:\n")
	b.WriteString(truncateUTF8Bytes(strings.TrimSpace(content), 4096))
	return b.String()
}

func smallModelPseudoToolRepairHints(allowedTools []llm.Tool) []string {
	if len(allowedTools) == 0 {
		return nil
	}
	out := make([]string, 0, len(allowedTools))
	seen := make(map[string]struct{}, len(allowedTools))
	for _, tool := range allowedTools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}

		hint := name
		if props := smallModelPseudoToolRepairPropertyNames(tool.Parameters); len(props) > 0 {
			hint += " args: " + strings.Join(props, ", ")
		}
		out = append(out, hint)
	}
	sort.Strings(out)
	return out
}

func smallModelPseudoToolRepairPropertyNames(parameters map[string]interface{}) []string {
	if len(parameters) == 0 {
		return nil
	}
	rawProps, ok := parameters["properties"].(map[string]interface{})
	if !ok || len(rawProps) == 0 {
		return nil
	}
	names := make([]string, 0, len(rawProps))
	for name := range rawProps {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		names = append(names, trimmed)
	}
	sort.Strings(names)
	if len(names) > 8 {
		names = names[:8]
	}
	return names
}
