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
	if llmCaller == nil {
		return nil, fmt.Errorf("responder LLM is not configured")
	}
	resp, err := llmCaller.Chat(ctx, llm.ChatRequest{
		Model: "auto",
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
	Goal               string
	Step               PlanStep
	GroundState        *GroundTruthState
	PriorToolCallIDs   []string
	PreviousViolations []string
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
