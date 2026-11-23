package server

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

// Rejected calls are unfinished work, not a successful toolless response.
func retryRejectedAssistantToolCalls(req *llm.ChatRequest, rejected []string, retries *int, remainingRounds int) (bool, error) {
	if len(rejected) == 0 || len(req.Tools) == 0 {
		return false, nil
	}
	if *retries >= 2 || remainingRounds <= 0 {
		return false, fmt.Errorf("model requested unavailable tools (%s) and the correction budget is exhausted; those calls were not executed", strings.Join(rejected, ", "))
	}
	*retries++
	names := make([]string, 0, len(req.Tools))
	for _, tool := range req.Tools {
		names = append(names, tool.Name)
	}
	feedback := fmt.Sprintf("Your requested tools (%s) are unavailable, so those calls were not executed. Use only the currently provided tools: %s.", strings.Join(rejected, ", "), strings.Join(names, ", "))
	if containsLLMToolName(req.Tools, "exec") {
		feedback += " Invoke skills through exec using their documented blue commands; skill names are not native tool names."
	}
	feedback += " Continue the original request using actual tool results and deliver the requested answer."
	req.Messages = append(req.Messages, llm.Message{Role: llm.RoleUser, Content: feedback})
	return true, nil
}
