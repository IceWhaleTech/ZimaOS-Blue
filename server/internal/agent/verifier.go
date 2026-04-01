package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type ResponseClaimType string

const (
	ClaimTypeFSExists           ResponseClaimType = "fs_exists"
	ClaimTypeFSSize             ResponseClaimType = "fs_size"
	ClaimTypeFileContentExcerpt ResponseClaimType = "file_content_excerpt"
	ClaimTypeCommandExcerpt     ResponseClaimType = "command_output_excerpt"
	ClaimTypeToolOutput         ResponseClaimType = "tool_output"
	ClaimTypeUnknown            ResponseClaimType = "unknown"
)

type GroundedResponse struct {
	Summary string          `json:"summary"`
	Claims  []ResponseClaim `json:"claims"`
}

type ResponseClaim struct {
	Type        ResponseClaimType `json:"type"`
	Text        string            `json:"text,omitempty"`
	ToolCallIDs []string          `json:"tool_call_ids,omitempty"`
	Path        string            `json:"path,omitempty"`
	Value       string            `json:"value,omitempty"`
	Excerpt     string            `json:"excerpt,omitempty"`
}

type VerificationDecision struct {
	Valid      bool
	Violations []string
	Status     string
	Output     string
}

type GroundedVerifier struct {
	verifyResult func(GroundedToolResult) bool
}

func NewGroundedVerifier(verifyResult func(GroundedToolResult) bool) *GroundedVerifier {
	return &GroundedVerifier{verifyResult: verifyResult}
}

func (v *GroundedVerifier) Respond(ctx context.Context, llmCaller LLMCaller, input ResponderInput) (*GroundedResponse, error) {
	if deterministic := v.deterministicResponse(input); deterministic != nil {
		return deterministic, nil
	}
	if llmCaller == nil {
		return nil, fmt.Errorf("responder LLM is not configured")
	}
	resp, err := llmCaller.Chat(ctx, llm.ChatRequest{
		Model: firstNonEmptyString(input.Model, "auto"),
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: buildResponderSystemPrompt()},
			{Role: llm.RoleUser, Content: buildResponderUserPrompt(input)},
		},
		MaxTokens:   800,
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}
	return parseGroundedResponse(resp.Message.Content)
}

type ResponderInput struct {
	Model              string
	Goal               string
	Step               PlanStep
	GroundState        *GroundTruthState
	PriorToolCallIDs   []string
	PreviousViolations []string
}

func (v *GroundedVerifier) deterministicResponse(input ResponderInput) *GroundedResponse {
	if input.GroundState == nil {
		return nil
	}
	for _, toolCallID := range deterministicResponderToolCallIDs(input) {
		result, ok := input.GroundState.Results[toolCallID]
		if !ok || !result.OK {
			continue
		}
		if v.verifyResult != nil && !v.verifyResult(result) {
			continue
		}
		call := input.GroundState.Calls[toolCallID]
		if response := deterministicWebGroundedResponse(toolCallID, call, result); response != nil {
			return response
		}
		if response := deterministicAnalyzeGroundedResponse(toolCallID, call, result); response != nil {
			return response
		}
		if response := deterministicReminderGroundedResponse(toolCallID, call, result); response != nil {
			return response
		}
	}
	return nil
}

func deterministicResponderToolCallIDs(input ResponderInput) []string {
	if len(input.PriorToolCallIDs) > 0 {
		seen := make(map[string]struct{}, len(input.PriorToolCallIDs))
		out := make([]string, 0, len(input.PriorToolCallIDs))
		for i := len(input.PriorToolCallIDs) - 1; i >= 0; i-- {
			toolCallID := strings.TrimSpace(input.PriorToolCallIDs[i])
			if toolCallID == "" {
				continue
			}
			if _, ok := seen[toolCallID]; ok {
				continue
			}
			seen[toolCallID] = struct{}{}
			out = append(out, toolCallID)
		}
		return out
	}
	if input.GroundState == nil || len(input.GroundState.Results) == 0 {
		return nil
	}
	out := make([]string, 0, len(input.GroundState.Results))
	for toolCallID := range input.GroundState.Results {
		out = append(out, toolCallID)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

func deterministicWebGroundedResponse(toolCallID string, call GroundedToolCall, result GroundedToolResult) *GroundedResponse {
	if !deterministicWebEvidenceEnabled(call, result) {
		return nil
	}
	title, url, snippet := deterministicWebEvidenceFields(result.Result)
	excerpts := deterministicClaimExcerpts(title, url, snippet)
	if len(excerpts) == 0 {
		return nil
	}
	claims := make([]ResponseClaim, 0, len(excerpts))
	for _, excerpt := range excerpts {
		claims = append(claims, ResponseClaim{
			Type:        ClaimTypeToolOutput,
			ToolCallIDs: []string{toolCallID},
			Excerpt:     excerpt,
		})
	}
	return &GroundedResponse{
		Summary: deterministicWebSummary(title, url),
		Claims:  claims,
	}
}

func deterministicWebEvidenceEnabled(call GroundedToolCall, result GroundedToolResult) bool {
	if !groundedResultHasUsefulEvidence(result) {
		return false
	}
	if !deterministicWebResultReady(result.Result) {
		return false
	}
	toolName := normalizeGroundToolName(firstNonEmptyString(call.Tool, result.Tool))
	if isGroundedWebToolFamily(toolName) {
		return true
	}
	if toolName != "bash" {
		return false
	}
	command := firstNonEmptyString(
		strings.TrimSpace(asString(call.Args["command"])),
		strings.TrimSpace(asString(call.Args["cmd"])),
	)
	return isGroundedWebToolFamily(groundedCLICommandSkillToken(command))
}

func deterministicWebResultReady(value any) bool {
	payloads := deterministicStructuredPayloads(value)
	if len(payloads) == 0 {
		return false
	}
	for _, payload := range payloads {
		if deterministicWebPayloadReady(payload) {
			return true
		}
	}
	return false
}

func deterministicWebEvidenceFields(value any) (string, string, string) {
	var title string
	var url string
	var snippet string
	for _, payload := range deterministicStructuredPayloads(value) {
		if !deterministicWebPayloadReady(payload) {
			continue
		}
		if title == "" {
			title = deterministicExcerpt(firstNonEmptyString(
				strings.TrimSpace(asString(extractField(payload, "title"))),
				strings.TrimSpace(asString(extractField(payload, "page_title"))),
				deterministicSourceTitle(payload),
			), 160)
		}
		if url == "" {
			url = deterministicExcerpt(firstNonEmptyString(
				strings.TrimSpace(asString(extractField(payload, "final_url"))),
				strings.TrimSpace(asString(extractField(payload, "target_url"))),
				strings.TrimSpace(asString(extractField(payload, "url"))),
				strings.TrimSpace(asString(extractField(payload, "input"))),
				deterministicSourceURL(payload),
			), 320)
		}
		if snippet == "" {
			snippet = deterministicExcerpt(firstNonEmptyString(
				strings.TrimSpace(asString(extractField(payload, "snippet"))),
				strings.TrimSpace(asString(extractField(payload, "summary"))),
				strings.TrimSpace(asString(extractField(payload, "message"))),
				strings.TrimSpace(asString(extractField(payload, "content"))),
				deterministicSourceSnippet(payload),
			), 180)
		}
		if title != "" && url != "" && snippet != "" {
			break
		}
	}
	return title, url, snippet
}

func deterministicWebPayloadReady(payload map[string]any) bool {
	if len(payload) == 0 {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(asString(extractField(payload, "status"))))
	nextAction := strings.ToLower(strings.TrimSpace(asString(extractField(payload, "next_action"))))
	if nextAction == "retry_browser" || nextAction == "authorize_provider" {
		return false
	}
	switch status {
	case "ok", "partial":
		return true
	case "failed", "error", "needs_browser", "login_wall", "challenge":
		return false
	}
	if status != "" {
		return false
	}
	if groundedNeedsBrowser(payload) {
		return false
	}
	if deterministicHasNestedStructuredPayload(payload) {
		return false
	}
	return deterministicPayloadHasWebEvidence(payload)
}

func deterministicHasNestedStructuredPayload(payload map[string]any) bool {
	for _, key := range []string{"data", "page", "result"} {
		next := extractField(payload, key)
		if next == nil {
			continue
		}
		if nested, ok := next.(map[string]any); ok && len(nested) > 0 {
			return true
		}
		if nested := parseGroundedNestedPayload(next); nested != nil {
			return true
		}
	}
	return false
}

func deterministicPayloadHasWebEvidence(payload map[string]any) bool {
	if firstNonEmptyString(
		strings.TrimSpace(asString(extractField(payload, "title"))),
		strings.TrimSpace(asString(extractField(payload, "page_title"))),
		strings.TrimSpace(asString(extractField(payload, "final_url"))),
		strings.TrimSpace(asString(extractField(payload, "target_url"))),
		strings.TrimSpace(asString(extractField(payload, "url"))),
		strings.TrimSpace(asString(extractField(payload, "input"))),
		strings.TrimSpace(asString(extractField(payload, "snippet"))),
		strings.TrimSpace(asString(extractField(payload, "summary"))),
		strings.TrimSpace(asString(extractField(payload, "message"))),
		strings.TrimSpace(asString(extractField(payload, "content"))),
	) != "" {
		return true
	}
	return deterministicSourceUsable(payload)
}

func deterministicAnalyzeGroundedResponse(toolCallID string, call GroundedToolCall, result GroundedToolResult) *GroundedResponse {
	if !deterministicAnalyzeEvidenceEnabled(call, result) {
		return nil
	}
	answer, topic, message := deterministicAnalyzeEvidenceFields(result.Result)
	excerpts := deterministicClaimExcerpts(answer, topic, message)
	if len(excerpts) == 0 {
		return nil
	}
	claims := make([]ResponseClaim, 0, len(excerpts))
	for _, excerpt := range excerpts {
		claims = append(claims, ResponseClaim{
			Type:        ClaimTypeToolOutput,
			ToolCallIDs: []string{toolCallID},
			Excerpt:     excerpt,
		})
	}
	return &GroundedResponse{
		Summary: deterministicAnalyzeSummary(answer, topic),
		Claims:  claims,
	}
}

func deterministicAnalyzeEvidenceEnabled(call GroundedToolCall, result GroundedToolResult) bool {
	if !groundedResultCarriesAnalyzeEvidence(call, result) {
		return false
	}
	return groundedResultHasAnalyzeEvidence(result.Result)
}

func deterministicAnalyzeEvidenceFields(value any) (string, string, string) {
	for _, payload := range deterministicStructuredPayloads(value) {
		if !deterministicAnalyzeResultReady(payload) {
			continue
		}
		answer := deterministicExcerpt(firstNonEmptyString(
			strings.TrimSpace(asString(extractField(payload, "answer"))),
			strings.TrimSpace(asString(extractField(payload, "summary"))),
			strings.TrimSpace(asString(extractField(payload, "content"))),
		), 220)
		topic := deterministicExcerpt(strings.TrimSpace(asString(extractField(payload, "topic"))), 160)
		message := deterministicExcerpt(strings.TrimSpace(asString(extractField(payload, "message"))), 180)
		if answer != "" || topic != "" || message != "" {
			return answer, topic, message
		}
	}
	return "", "", ""
}

func deterministicAnalyzeResultReady(value any) bool {
	status := strings.ToLower(strings.TrimSpace(asString(extractField(value, "status"))))
	if status == "failed" || status == "error" {
		return false
	}
	return firstNonEmptyString(
		strings.TrimSpace(asString(extractField(value, "answer"))),
		strings.TrimSpace(asString(extractField(value, "summary"))),
		strings.TrimSpace(asString(extractField(value, "message"))),
		strings.TrimSpace(asString(extractField(value, "topic"))),
	) != ""
}

func deterministicAnalyzeSummary(answer, topic string) string {
	switch {
	case answer != "" && topic != "":
		return fmt.Sprintf("Grounded analyze result collected for %s.", topic)
	case answer != "":
		return "Grounded analyze result collected."
	case topic != "":
		return fmt.Sprintf("Grounded analyze result collected for %s.", topic)
	default:
		return "Grounded analyze result collected."
	}
}

func deterministicReminderGroundedResponse(toolCallID string, call GroundedToolCall, result GroundedToolResult) *GroundedResponse {
	if !deterministicReminderEvidenceEnabled(call, result) {
		return nil
	}
	confirmation, fireAt, sessionID := deterministicReminderEvidenceFields(call, result)
	excerpts := deterministicClaimExcerpts(confirmation, fireAt, sessionID)
	if len(excerpts) == 0 {
		return nil
	}
	claims := make([]ResponseClaim, 0, len(excerpts))
	for _, excerpt := range excerpts {
		claims = append(claims, ResponseClaim{
			Type:        ClaimTypeCommandExcerpt,
			ToolCallIDs: []string{toolCallID},
			Excerpt:     excerpt,
		})
	}
	return &GroundedResponse{
		Summary: deterministicReminderSummary(confirmation, fireAt),
		Claims:  claims,
	}
}

func deterministicReminderEvidenceEnabled(call GroundedToolCall, result GroundedToolResult) bool {
	if !groundedResultCarriesReminderEvidence(call, result) {
		return false
	}
	return groundedReminderResultReady(result.Result)
}

func deterministicReminderEvidenceFields(call GroundedToolCall, result GroundedToolResult) (string, string, string) {
	confirmation := ""
	stdoutFallback := ""
	fireAt := ""
	sessionID := ""
	for _, payload := range deterministicStructuredPayloads(result.Result) {
		if confirmation == "" {
			confirmation = deterministicExcerpt(firstNonEmptyString(
				strings.TrimSpace(asString(extractField(payload, "message"))),
			), 220)
		}
		if stdoutFallback == "" {
			stdoutFallback = deterministicExcerpt(strings.TrimSpace(asString(extractField(payload, "stdout"))), 220)
		}
		if reminder := parseGroundedNestedPayload(extractField(payload, "reminder")); reminder != nil {
			if fireAt == "" {
				fireAt = deterministicExcerpt(firstNonEmptyString(
					strings.TrimSpace(asString(extractField(reminder, "fire_at"))),
					strings.TrimSpace(asString(extractField(reminder, "fireAt"))),
				), 96)
			}
			if sessionID == "" {
				sessionID = deterministicExcerpt(firstNonEmptyString(
					strings.TrimSpace(asString(extractField(reminder, "session_id"))),
					strings.TrimSpace(asString(extractField(reminder, "sessionId"))),
				), 160)
			}
		}
	}
	if confirmation == "" {
		confirmation = stdoutFallback
	}
	if sessionID == "" {
		sessionID = deterministicExcerpt(firstNonEmptyString(
			strings.TrimSpace(asString(extractField(call.Args, "session_id"))),
			strings.TrimSpace(asString(extractField(call.Args, "sessionId"))),
		), 160)
	}
	return confirmation, fireAt, sessionID
}

func deterministicReminderSummary(confirmation, fireAt string) string {
	switch {
	case confirmation != "" && fireAt != "":
		return fmt.Sprintf("Grounded reminder scheduled for %s.", fireAt)
	case confirmation != "":
		return "Grounded reminder scheduling confirmed."
	case fireAt != "":
		return fmt.Sprintf("Grounded reminder scheduled for %s.", fireAt)
	default:
		return "Grounded reminder scheduling confirmed."
	}
}

func deterministicStructuredPayloads(value any) []map[string]any {
	type queueItem struct {
		value any
		depth int
	}
	queue := []queueItem{{value: value, depth: 0}}
	seen := make(map[string]struct{})
	out := make([]map[string]any, 0, 4)
	appendPayload := func(payload map[string]any) {
		if len(payload) == 0 {
			return
		}
		key := serializedResult(payload)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, payload)
	}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if item.depth > 3 || item.value == nil {
			continue
		}
		if payload, ok := item.value.(map[string]any); ok {
			appendPayload(payload)
			for _, key := range []string{"data", "page", "result"} {
				if next := extractField(payload, key); next != nil {
					queue = append(queue, queueItem{value: next, depth: item.depth + 1})
				}
			}
		}
		if nested := parseGroundedNestedPayload(item.value); nested != nil {
			appendPayload(nested)
			for _, key := range []string{"data", "page", "result"} {
				if next := extractField(nested, key); next != nil {
					queue = append(queue, queueItem{value: next, depth: item.depth + 1})
				}
			}
		}
	}
	return out
}

func deterministicSourceURL(payload map[string]any) string {
	source := deterministicSelectedSource(payload)
	if len(source) == 0 {
		return ""
	}
	return firstNonEmptyString(
		strings.TrimSpace(asString(extractField(source, "final_url"))),
		strings.TrimSpace(asString(extractField(source, "url"))),
	)
}

func deterministicSourceTitle(payload map[string]any) string {
	source := deterministicSelectedSource(payload)
	if len(source) == 0 {
		return ""
	}
	return strings.TrimSpace(asString(extractField(source, "title")))
}

func deterministicSourceSnippet(payload map[string]any) string {
	source := deterministicSelectedSource(payload)
	if len(source) == 0 {
		return ""
	}
	return strings.TrimSpace(asString(extractField(source, "snippet")))
}

func deterministicSelectedSource(payload map[string]any) map[string]any {
	rawSources, ok := extractField(payload, "sources").([]any)
	if !ok {
		return nil
	}
	var fallback map[string]any
	for _, raw := range rawSources {
		source, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if fallback == nil && deterministicSourceUsable(source) {
			fallback = source
		}
		if asBool(extractField(source, "selected")) && deterministicSourceUsable(source) {
			return source
		}
	}
	return fallback
}

func deterministicSourceUsable(source map[string]any) bool {
	return firstNonEmptyString(
		strings.TrimSpace(asString(extractField(source, "title"))),
		strings.TrimSpace(asString(extractField(source, "url"))),
		strings.TrimSpace(asString(extractField(source, "final_url"))),
		strings.TrimSpace(asString(extractField(source, "snippet"))),
	) != ""
}

func deterministicClaimExcerpts(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func deterministicWebSummary(title, url string) string {
	switch {
	case title != "" && url != "":
		return fmt.Sprintf("Grounded web evidence collected for %s (%s).", title, url)
	case title != "":
		return fmt.Sprintf("Grounded web evidence collected for %s.", title)
	case url != "":
		return fmt.Sprintf("Grounded web evidence collected from %s.", url)
	default:
		return "Grounded web evidence collected."
	}
}

func deterministicExcerpt(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexAny(value, "\r\n\t\"\\"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if limit > 0 && len(runes) > limit {
		value = string(runes[:limit])
	}
	return strings.TrimSpace(value)
}

func buildResponderSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are the Responder in a hallucination-safe runtime.\n")
	sb.WriteString("You can read grounded evidence only. You cannot invent tool results or filesystem facts.\n\n")
	sb.WriteString("Return ONLY one JSON object:\n")
	sb.WriteString("{\n")
	sb.WriteString(`  "summary": "short summary",` + "\n")
	sb.WriteString(`  "claims": [` + "\n")
	sb.WriteString(`    {"type":"fs_exists","tool_call_ids":["task/..."],"path":"README.md","value":"true"},` + "\n")
	sb.WriteString(`    {"type":"fs_size","tool_call_ids":["task/..."],"path":"README.md","value":"123"},` + "\n")
	sb.WriteString(`    {"type":"file_content_excerpt","tool_call_ids":["task/..."],"path":"README.md","excerpt":"hello"},` + "\n")
	sb.WriteString(`    {"type":"command_output_excerpt","tool_call_ids":["task/..."],"excerpt":"PASS"},` + "\n")
	sb.WriteString(`    {"type":"tool_output","tool_call_ids":["task/..."],"excerpt":"entry shown by ls"},` + "\n")
	sb.WriteString(`    {"type":"unknown","text":"unknown"}` + "\n")
	sb.WriteString("  ]\n")
	sb.WriteString("}\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Every non-unknown claim MUST cite real tool_call_ids from the evidence.\n")
	sb.WriteString("- If a fact is not verified by evidence, use an unknown claim.\n")
	sb.WriteString("- Do not write prose outside the JSON object.\n")
	return strings.TrimSpace(sb.String())
}

func buildResponderUserPrompt(input ResponderInput) string {
	var sb strings.Builder
	sb.WriteString("Goal:\n")
	sb.WriteString(strings.TrimSpace(input.Goal))
	sb.WriteString("\n\nCurrent step:\n")
	sb.WriteString(strings.TrimSpace(input.Step.Description))
	sb.WriteString("\n\nGrounded state:\n")
	sb.WriteString(BuildGroundStateSummary(input.GroundState))
	if len(input.PriorToolCallIDs) > 0 {
		sb.WriteString("\n\nGrounded tool_call_ids available:\n")
		for _, id := range input.PriorToolCallIDs {
			sb.WriteString("- ")
			sb.WriteString(id)
			sb.WriteByte('\n')
		}
	}
	if len(input.PreviousViolations) > 0 {
		sb.WriteString("\nPrevious verifier violations to avoid:\n")
		for _, violation := range input.PreviousViolations {
			sb.WriteString("- ")
			sb.WriteString(violation)
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("\nReturn only grounded claims.")
	return strings.TrimSpace(sb.String())
}

func parseGroundedResponse(content string) (*GroundedResponse, error) {
	trimmed := trimStructuredContent(content)
	if trimmed == "" {
		return nil, fmt.Errorf("responder returned empty content")
	}
	var out GroundedResponse
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("responder output is not valid JSON: %w", err)
	}
	out.Summary = strings.TrimSpace(out.Summary)
	for i := range out.Claims {
		out.Claims[i].Type = ResponseClaimType(strings.ToLower(strings.TrimSpace(string(out.Claims[i].Type))))
		out.Claims[i].Text = strings.TrimSpace(out.Claims[i].Text)
		out.Claims[i].Path = normalizeGroundPath(out.Claims[i].Path)
		out.Claims[i].Value = strings.TrimSpace(out.Claims[i].Value)
		out.Claims[i].Excerpt = strings.TrimSpace(out.Claims[i].Excerpt)
		for j := range out.Claims[i].ToolCallIDs {
			out.Claims[i].ToolCallIDs[j] = strings.TrimSpace(out.Claims[i].ToolCallIDs[j])
		}
	}
	return &out, nil
}

func (v *GroundedVerifier) Verify(state *GroundTruthState, response *GroundedResponse) VerificationDecision {
	if response == nil {
		return VerificationDecision{
			Valid:      false,
			Status:     GroundingStatusRejected,
			Violations: []string{"response is missing"},
			Output:     "unknown",
		}
	}
	var violations []string
	for i, claim := range response.Claims {
		violations = append(violations, v.verifyClaim(state, i, claim)...)
	}
	if len(violations) == 0 {
		return VerificationDecision{
			Valid:  true,
			Status: groundedStatusForClaims(response.Claims),
			Output: renderGroundedClaims(response.Claims),
		}
	}
	fallback := v.buildFallback(state, response)
	return VerificationDecision{
		Valid:      false,
		Status:     GroundingStatusFallback,
		Violations: violations,
		Output:     renderGroundedClaims(fallback.Claims),
	}
}

func groundedStatusForClaims(claims []ResponseClaim) string {
	if len(claims) == 0 {
		return GroundingStatusUnknown
	}
	for _, claim := range claims {
		if claim.Type == ClaimTypeUnknown {
			return GroundingStatusUnknown
		}
	}
	return GroundingStatusGrounded
}

func (v *GroundedVerifier) verifyClaim(state *GroundTruthState, idx int, claim ResponseClaim) []string {
	prefix := fmt.Sprintf("claim[%d]", idx)
	switch claim.Type {
	case ClaimTypeUnknown:
		return nil
	case ClaimTypeFSExists:
		return v.verifyFSExistsClaim(state, prefix, claim)
	case ClaimTypeFSSize:
		return v.verifyFSSizeClaim(state, prefix, claim)
	case ClaimTypeFileContentExcerpt:
		return v.verifyFileExcerptClaim(state, prefix, claim)
	case ClaimTypeCommandExcerpt:
		return v.verifyCommandExcerptClaim(state, prefix, claim)
	case ClaimTypeToolOutput:
		return v.verifyToolOutputClaim(state, prefix, claim)
	default:
		return []string{fmt.Sprintf("%s has unsupported type %q", prefix, claim.Type)}
	}
}

func (v *GroundedVerifier) verifyFSExistsClaim(state *GroundTruthState, prefix string, claim ResponseClaim) []string {
	var violations []string
	results := v.claimResults(state, claim.ToolCallIDs, prefix)
	violations = append(violations, results.violations...)
	if claim.Path == "" {
		violations = append(violations, prefix+" is missing path")
		return violations
	}
	want, err := strconv.ParseBool(strings.ToLower(claim.Value))
	if err != nil {
		violations = append(violations, prefix+" has invalid boolean value")
		return violations
	}
	fact, ok := state.Files[claim.Path]
	if !ok {
		violations = append(violations, prefix+" references unknown file fact")
		return violations
	}
	if fact.Exists != want {
		violations = append(violations, fmt.Sprintf("%s mismatches grounded existence for %s", prefix, claim.Path))
	}
	if !supportingPathEvidence(results.results, claim.Path) {
		violations = append(violations, fmt.Sprintf("%s lacks supporting tool evidence for %s", prefix, claim.Path))
	}
	return violations
}

func (v *GroundedVerifier) verifyFSSizeClaim(state *GroundTruthState, prefix string, claim ResponseClaim) []string {
	var violations []string
	results := v.claimResults(state, claim.ToolCallIDs, prefix)
	violations = append(violations, results.violations...)
	if claim.Path == "" {
		violations = append(violations, prefix+" is missing path")
		return violations
	}
	want, err := strconv.ParseInt(claim.Value, 10, 64)
	if err != nil {
		violations = append(violations, prefix+" has invalid size value")
		return violations
	}
	fact, ok := state.Files[claim.Path]
	if !ok {
		violations = append(violations, prefix+" references unknown file fact")
		return violations
	}
	if fact.Size != want {
		violations = append(violations, fmt.Sprintf("%s mismatches grounded size for %s", prefix, claim.Path))
	}
	if !supportingPathEvidence(results.results, claim.Path) {
		violations = append(violations, fmt.Sprintf("%s lacks supporting size evidence for %s", prefix, claim.Path))
	}
	return violations
}

func (v *GroundedVerifier) verifyFileExcerptClaim(state *GroundTruthState, prefix string, claim ResponseClaim) []string {
	var violations []string
	results := v.claimResults(state, claim.ToolCallIDs, prefix)
	violations = append(violations, results.violations...)
	if claim.Path == "" || claim.Excerpt == "" {
		violations = append(violations, prefix+" requires path and excerpt")
		return violations
	}
	supported := false
	for _, result := range results.results {
		if !supportingPathEvidence([]GroundedToolResult{result}, claim.Path) {
			continue
		}
		if strings.Contains(serializedResult(result.Result), claim.Excerpt) {
			supported = true
			break
		}
	}
	if !supported {
		violations = append(violations, fmt.Sprintf("%s excerpt is not present in grounded file evidence", prefix))
	}
	return violations
}

func (v *GroundedVerifier) verifyCommandExcerptClaim(state *GroundTruthState, prefix string, claim ResponseClaim) []string {
	var violations []string
	results := v.claimResults(state, claim.ToolCallIDs, prefix)
	violations = append(violations, results.violations...)
	if claim.Excerpt == "" {
		violations = append(violations, prefix+" requires excerpt")
		return violations
	}
	supported := false
	for _, result := range results.results {
		if normalizeGroundToolName(result.Tool) != "bash" {
			continue
		}
		if strings.Contains(serializedResult(result.Result), claim.Excerpt) {
			supported = true
			break
		}
	}
	if !supported {
		violations = append(violations, fmt.Sprintf("%s excerpt is not present in grounded command evidence", prefix))
	}
	return violations
}

func (v *GroundedVerifier) verifyToolOutputClaim(state *GroundTruthState, prefix string, claim ResponseClaim) []string {
	var violations []string
	results := v.claimResults(state, claim.ToolCallIDs, prefix)
	violations = append(violations, results.violations...)
	if claim.Excerpt == "" {
		violations = append(violations, prefix+" requires excerpt")
		return violations
	}
	supported := false
	for _, result := range results.results {
		if strings.Contains(serializedResult(result.Result), claim.Excerpt) {
			supported = true
			break
		}
	}
	if !supported {
		violations = append(violations, fmt.Sprintf("%s excerpt is not present in grounded tool output", prefix))
	}
	return violations
}

type claimResultBundle struct {
	results    []GroundedToolResult
	violations []string
}

func (v *GroundedVerifier) claimResults(state *GroundTruthState, ids []string, prefix string) claimResultBundle {
	if len(ids) == 0 {
		return claimResultBundle{violations: []string{prefix + " is missing tool_call_ids"}}
	}
	var out claimResultBundle
	for _, id := range ids {
		result, ok := state.Results[id]
		if !ok {
			out.violations = append(out.violations, fmt.Sprintf("%s references unknown tool_call_id %q", prefix, id))
			continue
		}
		if v.verifyResult != nil && !v.verifyResult(result) {
			out.violations = append(out.violations, fmt.Sprintf("%s references untrusted tool result %q", prefix, id))
			continue
		}
		out.results = append(out.results, result)
	}
	return out
}

func supportingPathEvidence(results []GroundedToolResult, path string) bool {
	for _, result := range results {
		if normalizeGroundPath(asString(extractField(result.Result, "path"))) == path {
			return true
		}
		if base := normalizeGroundPath(asString(extractField(result.Result, "base_path"))); base != "" {
			if rawEntries, ok := extractField(result.Result, "entries").([]any); ok {
				for _, raw := range rawEntries {
					entry, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					if normalizeGroundPath(joinGroundPaths(base, asString(entry["path"]))) == path {
						return true
					}
				}
			}
		}
	}
	return false
}

func (v *GroundedVerifier) buildFallback(state *GroundTruthState, response *GroundedResponse) *GroundedResponse {
	fallback := &GroundedResponse{}
	for _, claim := range response.Claims {
		decision := v.verifyClaim(state, 0, claim)
		if len(decision) == 0 {
			fallback.Claims = append(fallback.Claims, claim)
			continue
		}
		fallback.Claims = append(fallback.Claims, ResponseClaim{
			Type: ClaimTypeUnknown,
			Text: "unknown",
		})
	}
	if len(fallback.Claims) == 0 {
		fallback.Claims = []ResponseClaim{{Type: ClaimTypeUnknown, Text: "unknown"}}
	}
	return fallback
}

func renderGroundedClaims(claims []ResponseClaim) string {
	if len(claims) == 0 {
		return "unknown"
	}
	lines := make([]string, 0, len(claims))
	for _, claim := range claims {
		lines = append(lines, renderGroundedClaim(claim))
	}
	return strings.Join(lines, "\n")
}

func renderGroundedClaim(claim ResponseClaim) string {
	sort.Strings(claim.ToolCallIDs)
	tag := ""
	if len(claim.ToolCallIDs) > 0 {
		tag = " [tool_call_id=" + strings.Join(claim.ToolCallIDs, ",") + "]"
	}
	switch claim.Type {
	case ClaimTypeFSExists:
		if strings.EqualFold(claim.Value, "true") {
			return fmt.Sprintf("%s exists%s", claim.Path, tag)
		}
		return fmt.Sprintf("%s does not exist%s", claim.Path, tag)
	case ClaimTypeFSSize:
		return fmt.Sprintf("%s size=%s%s", claim.Path, claim.Value, tag)
	case ClaimTypeFileContentExcerpt:
		return fmt.Sprintf("%s excerpt=%q%s", claim.Path, claim.Excerpt, tag)
	case ClaimTypeCommandExcerpt:
		return fmt.Sprintf("command excerpt=%q%s", claim.Excerpt, tag)
	case ClaimTypeToolOutput:
		return fmt.Sprintf("tool output excerpt=%q%s", claim.Excerpt, tag)
	default:
		return "unknown"
	}
}

func serializedResult(result any) string {
	b, _ := json.Marshal(result)
	return string(b)
}
