package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type TaskKind string

const (
	TaskKindCode     TaskKind = "code"
	TaskKindDocs     TaskKind = "docs"
	TaskKindResearch TaskKind = "research"
	TaskKindOps      TaskKind = "ops"
	TaskKindGeneric  TaskKind = "generic"
)

type CriterionResult struct {
	Criterion string `json:"criterion"`
	Status    string `json:"status"`
	Evidence  string `json:"evidence,omitempty"`
}

type VerificationResult struct {
	Status            string            `json:"status"`
	Summary           string            `json:"summary"`
	CriteriaResults   []CriterionResult `json:"criteria_results"`
	SuggestedRecovery string            `json:"suggested_recovery,omitempty"`
	ExecutedChecks    []string          `json:"executed_checks"`
}

type VerificationContext struct {
	TaskKind        TaskKind
	Goal            string
	SuccessCriteria []string
	FallbackPlan    []string
	PlannedSteps    []string
}

type ProgressSignatureState struct {
	lastCombinedSignature string
	stableRounds          int
	lastErrorSignature    string
	errorRounds           int
	lastFamilyOutcomeSig  string
	familyOutcomeRounds   int
}

var (
	progressWhitespaceRE       = regexp.MustCompile(`\s+`)
	progressDigitsRE           = regexp.MustCompile(`\d+`)
	defaultTaskSuccessCriteria = []string{
		"core task output is produced",
		"no blocking errors in final result",
	}
	defaultTaskFallbackPlan = []string{
		"retry once with narrower scope",
		"ask user to choose next recovery strategy",
	}
)

func effectiveSuccessCriteria(criteria []string) []string {
	cleaned := dedupeStrings(criteria)
	if len(cleaned) == 0 {
		return append([]string(nil), defaultTaskSuccessCriteria...)
	}
	return cleaned
}

func effectiveFallbackPlan(plan []string) []string {
	cleaned := dedupeStrings(plan)
	if len(cleaned) == 0 {
		return append([]string(nil), defaultTaskFallbackPlan...)
	}
	return cleaned
}

func compileVerificationContext(task *Task) VerificationContext {
	if task == nil {
		return VerificationContext{TaskKind: TaskKindGeneric}
	}
	steps := make([]string, 0, len(task.Plan))
	for _, step := range task.Plan {
		desc := strings.TrimSpace(step.Description)
		if desc == "" {
			continue
		}
		steps = append(steps, desc)
	}
	successCriteria := effectiveSuccessCriteria(task.SuccessCriteria)
	fallbackPlan := effectiveFallbackPlan(task.FallbackPlan)
	return VerificationContext{
		TaskKind:        inferTaskKind(task.Goal, task.Plan, successCriteria),
		Goal:            strings.TrimSpace(task.Goal),
		SuccessCriteria: successCriteria,
		FallbackPlan:    fallbackPlan,
		PlannedSteps:    steps,
	}
}

func inferTaskKind(goal string, steps []PlanStep, criteria []string) TaskKind {
	var corpus strings.Builder
	corpus.WriteString(strings.ToLower(goal))
	for _, step := range steps {
		if desc := strings.TrimSpace(step.Description); desc != "" {
			corpus.WriteByte(' ')
			corpus.WriteString(strings.ToLower(desc))
		}
	}
	for _, criterion := range criteria {
		if criterion = strings.TrimSpace(criterion); criterion != "" {
			corpus.WriteByte(' ')
			corpus.WriteString(strings.ToLower(criterion))
		}
	}
	text := corpus.String()
	if strings.TrimSpace(text) == "" {
		return TaskKindGeneric
	}

	type scoredKind struct {
		kind     TaskKind
		keywords []string
	}
	candidates := []scoredKind{
		{
			kind: TaskKindCode,
			keywords: []string{
				"implement", "implementation", "fix", "bug", "build", "test", "compile", "parser",
				"refactor", "patch", "code", "file", "function", "module", "handler", "router",
				".go", ".ts", ".js", ".vue", ".py",
			},
		},
		{
			kind: TaskKindDocs,
			keywords: []string{
				"readme", "documentation", "document", "docs", "spec", "specification", "changelog",
				"guide", "manual", "write-up", "mdx", ".md",
			},
		},
		{
			kind: TaskKindResearch,
			keywords: []string{
				"research", "search", "source", "sources", "evidence", "citation", "citations",
				"report", "reporting", "analysis", "timeline", "compare", "survey",
			},
		},
		{
			kind: TaskKindOps,
			keywords: []string{
				"service", "deploy", "deployment", "config", "configuration", "process",
				"log", "logs", "restart", "runtime", "worker", "daemon", "server", "port",
			},
		},
	}

	bestKind := TaskKindGeneric
	bestScore := 0
	for _, candidate := range candidates {
		score := scoreKeywordHits(text, candidate.keywords)
		if score > bestScore {
			bestScore = score
			bestKind = candidate.kind
		}
	}
	if bestScore == 0 {
		return TaskKindGeneric
	}
	return bestKind
}

func scoreKeywordHits(text string, keywords []string) int {
	score := 0
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			score++
		}
	}
	return score
}

func parseVerificationResult(content string) (*VerificationResult, error) {
	trimmed := trimStructuredContent(content)
	if trimmed == "" {
		return nil, fmt.Errorf("verification returned empty output")
	}
	var out VerificationResult
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("verification result is not valid JSON: %w", err)
	}
	out.Status = normalizeVerificationStatus(out.Status)
	out.Summary = strings.TrimSpace(out.Summary)
	out.SuggestedRecovery = strings.TrimSpace(out.SuggestedRecovery)
	if out.Status == "" {
		return nil, fmt.Errorf("verification result missing status")
	}
	if out.Status != "pass" && out.Status != "fail" {
		return nil, fmt.Errorf("verification result has unsupported status %q", out.Status)
	}
	if out.Summary == "" {
		return nil, fmt.Errorf("verification result missing summary")
	}
	if len(out.CriteriaResults) == 0 {
		return nil, fmt.Errorf("verification result missing criteria_results")
	}
	for i := range out.CriteriaResults {
		out.CriteriaResults[i].Criterion = strings.TrimSpace(out.CriteriaResults[i].Criterion)
		out.CriteriaResults[i].Status = normalizeVerificationStatus(out.CriteriaResults[i].Status)
		out.CriteriaResults[i].Evidence = strings.TrimSpace(out.CriteriaResults[i].Evidence)
		if out.CriteriaResults[i].Criterion == "" {
			return nil, fmt.Errorf("verification result criterion %d missing criterion text", i)
		}
		if out.CriteriaResults[i].Status != "pass" && out.CriteriaResults[i].Status != "fail" {
			return nil, fmt.Errorf("verification result criterion %q has unsupported status %q", out.CriteriaResults[i].Criterion, out.CriteriaResults[i].Status)
		}
	}
	if len(out.ExecutedChecks) == 0 {
		return nil, fmt.Errorf("verification result missing executed_checks")
	}
	checks := make([]string, 0, len(out.ExecutedChecks))
	for _, check := range out.ExecutedChecks {
		check = strings.TrimSpace(check)
		if check == "" {
			continue
		}
		checks = append(checks, check)
	}
	if len(checks) == 0 {
		return nil, fmt.Errorf("verification result executed_checks are empty")
	}
	out.ExecutedChecks = checks
	return &out, nil
}

func normalizeVerificationStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "pass", "passed", "ok", "success":
		return "pass"
	case "fail", "failed", "error":
		return "fail"
	default:
		return status
	}
}

func syntheticVerificationFailure(ctx VerificationContext, summary, evidence string) *VerificationResult {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "Verification did not produce a usable result."
	}
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		evidence = summary
	}
	criteria := effectiveSuccessCriteria(ctx.SuccessCriteria)
	results := make([]CriterionResult, 0, len(criteria))
	for _, criterion := range criteria {
		criterion = strings.TrimSpace(criterion)
		if criterion == "" {
			continue
		}
		results = append(results, CriterionResult{
			Criterion: criterion,
			Status:    "fail",
			Evidence:  evidence,
		})
	}
	return &VerificationResult{
		Status:            "fail",
		Summary:           summary,
		CriteriaResults:   results,
		SuggestedRecovery: "Produce explicit evidence for every success criterion before marking the task complete.",
		ExecutedChecks:    []string{"verification contract validation"},
	}
}

func evaluateVerificationResult(result *VerificationResult, expectedCriteria []string) ([]CriterionResult, []CriterionResult, []string) {
	if result == nil {
		return nil, nil, append([]string(nil), expectedCriteria...)
	}
	indexed := make(map[string]CriterionResult, len(result.CriteriaResults))
	for _, item := range result.CriteriaResults {
		indexed[normalizeCriterionName(item.Criterion)] = item
	}
	ordered := make([]CriterionResult, 0, len(expectedCriteria))
	failed := make([]CriterionResult, 0)
	missing := make([]string, 0)
	for _, criterion := range expectedCriteria {
		normalized := normalizeCriterionName(criterion)
		item, ok := indexed[normalized]
		if !ok {
			missing = append(missing, criterion)
			ordered = append(ordered, CriterionResult{
				Criterion: criterion,
				Status:    "fail",
				Evidence:  "Missing explicit verification result for this criterion.",
			})
			continue
		}
		item.Criterion = criterion
		ordered = append(ordered, item)
		if item.Status != "pass" {
			failed = append(failed, item)
		}
	}
	if len(expectedCriteria) == 0 {
		ordered = append(ordered, result.CriteriaResults...)
		for _, item := range result.CriteriaResults {
			if item.Status != "pass" {
				failed = append(failed, item)
			}
		}
	}
	return ordered, failed, missing
}

func verificationPassed(result *VerificationResult, failed []CriterionResult, missing []string) bool {
	if result == nil {
		return false
	}
	if result.Status != "pass" {
		return false
	}
	return len(failed) == 0 && len(missing) == 0
}

func verificationStepDescription(kind TaskKind, retry bool) string {
	description := "Verify the completed work against the success criteria"
	switch kind {
	case TaskKindCode:
		description = "Verify the code changes against the success criteria"
	case TaskKindDocs:
		description = "Verify the documentation deliverable against the success criteria"
	case TaskKindResearch:
		description = "Verify the research deliverable against the success criteria"
	case TaskKindOps:
		description = "Verify the runtime or configuration outcome against the success criteria"
	}
	if retry {
		return description + " (bounded retry)"
	}
	return description
}

func recoveryStepDescription(kind TaskKind) string {
	switch kind {
	case TaskKindCode:
		return "Recovery: apply the fallback plan to address failed code verification"
	case TaskKindDocs:
		return "Recovery: apply the fallback plan to address failed document verification"
	case TaskKindResearch:
		return "Recovery: apply the fallback plan to address failed research verification"
	case TaskKindOps:
		return "Recovery: apply the fallback plan to address failed runtime verification"
	default:
		return "Recovery: apply the fallback plan to address failed verification"
	}
}

func formatVerificationOutput(ctx VerificationContext, result *VerificationResult, ordered []CriterionResult, missing []string) string {
	if result == nil {
		return "Verification failed.\nSummary: Verification did not produce a usable result."
	}
	var sb strings.Builder
	status := strings.ToUpper(strings.TrimSpace(result.Status))
	if status == "" {
		status = "FAIL"
	}
	sb.WriteString("Verification: ")
	sb.WriteString(status)
	sb.WriteString("\n")
	if ctx.TaskKind != "" {
		sb.WriteString("Task kind: ")
		sb.WriteString(string(ctx.TaskKind))
		sb.WriteString("\n")
	}
	sb.WriteString("Summary: ")
	sb.WriteString(strings.TrimSpace(result.Summary))
	sb.WriteString("\n")
	if len(result.ExecutedChecks) > 0 {
		sb.WriteString("Executed checks:\n")
		for _, check := range result.ExecutedChecks {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(check))
			sb.WriteString("\n")
		}
	}
	if len(ordered) > 0 {
		sb.WriteString("Criteria results:\n")
		for _, criterion := range ordered {
			label := "FAIL"
			if criterion.Status == "pass" {
				label = "PASS"
			}
			sb.WriteString("- [")
			sb.WriteString(label)
			sb.WriteString("] ")
			sb.WriteString(strings.TrimSpace(criterion.Criterion))
			if evidence := strings.TrimSpace(criterion.Evidence); evidence != "" {
				sb.WriteString(" -- ")
				sb.WriteString(truncate(evidence, 220))
			}
			sb.WriteString("\n")
		}
	}
	if len(missing) > 0 {
		sb.WriteString("Missing criteria coverage:\n")
		for _, criterion := range missing {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(criterion))
			sb.WriteString("\n")
		}
	}
	if suggestion := strings.TrimSpace(result.SuggestedRecovery); suggestion != "" {
		sb.WriteString("Suggested recovery: ")
		sb.WriteString(suggestion)
	}
	return strings.TrimSpace(sb.String())
}

func normalizeCriterionName(input string) string {
	return normalizeProgressText(input)
}

func trimStructuredContent(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = stripMarkdownCodeFence(trimmed)
	if extracted := extractBalancedJSONSnippet(trimmed); extracted != "" {
		return extracted
	}
	return strings.TrimSpace(trimmed)
}

func stripMarkdownCodeFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
			body := strings.TrimSpace(trimmed[newline+1:])
			body = strings.TrimSuffix(body, "```")
			return strings.TrimSpace(body)
		}
	}
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func extractBalancedJSONSnippet(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	for offset := 0; offset < len(trimmed); {
		next := strings.IndexAny(trimmed[offset:], "{[")
		if next < 0 {
			return ""
		}
		start := offset + next
		if candidate := balancedJSONFromStart(trimmed, start); candidate != "" {
			return candidate
		}
		offset = start + 1
	}
	return ""
}

func balancedJSONFromStart(content string, start int) string {
	if start < 0 || start >= len(content) {
		return ""
	}
	first := content[start]
	if first != '{' && first != '[' {
		return ""
	}

	stack := make([]byte, 0, 8)
	inString := false
	escaped := false

	for i := start; i < len(content); i++ {
		ch := content[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}', ']':
			if len(stack) == 0 {
				return ""
			}
			top := stack[len(stack)-1]
			if (top == '{' && ch != '}') || (top == '[' && ch != ']') {
				return ""
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				candidate := strings.TrimSpace(content[start : i+1])
				if json.Valid([]byte(candidate)) {
					return candidate
				}
				return ""
			}
		}
	}
	return ""
}

func toolCallSignature(calls []llm.ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, tc := range calls {
		if i > 0 {
			sb.WriteByte('|')
		}
		sb.WriteString(tc.Name)
		sb.WriteByte(':')
		sb.WriteString(tc.Arguments)
	}
	return sb.String()
}

func normalizeProgressSummary(content string) string {
	return tools.NormalizeToolProgressSummary(content)
}

func normalizeProgressText(content string) string {
	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = progressDigitsRE.ReplaceAllString(normalized, "#")
	normalized = progressWhitespaceRE.ReplaceAllString(normalized, " ")
	if len(normalized) > 160 {
		normalized = normalized[:160]
	}
	return normalized
}

func isErrorProgressSummary(summary string) bool {
	return strings.HasPrefix(summary, "error:")
}

func normalizeProgressSummaries(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "error:") ||
			strings.HasPrefix(value, "status:") ||
			strings.HasPrefix(value, "summary:") ||
			strings.HasPrefix(value, "json:") ||
			value == "empty" {
			out = append(out, value)
			continue
		}
		out = append(out, normalizeProgressSummary(value))
	}
	return out
}

func progressToolFamilySignature(signature string) string {
	normalized := normalizeProgressText(signature)
	switch {
	case strings.Contains(normalized, "web_query:"),
		strings.Contains(normalized, "web_search:"),
		strings.Contains(normalized, "web_fetch:"),
		strings.Contains(normalized, "web_read:"),
		strings.Contains(normalized, "web_extract:"),
		strings.Contains(normalized, "web_crawl:"):
		return "web_query_family"
	case strings.Contains(normalized, "browser:"):
		return "browser"
	case strings.Contains(normalized, "deep_research:"),
		strings.Contains(normalized, "research_run:"),
		strings.Contains(normalized, "research_status:"):
		return "research_family"
	default:
		return ""
	}
}

func (s *ProgressSignatureState) ObserveDetailed(toolSig, decision string, toolSummaries []string) tools.ToolLoopDetection {
	normalizedToolSig := normalizeProgressText(toolSig)
	normalizedDecision := normalizeProgressText(decision)
	normalizedSummaries := normalizeProgressSummaries(toolSummaries)
	outcomeSig := strings.Join(normalizedSummaries, "|")
	if outcomeSig == "" {
		outcomeSig = "empty"
	}

	combined := normalizedToolSig + "|" + outcomeSig
	if combined == s.lastCombinedSignature {
		s.stableRounds++
	} else {
		s.lastCombinedSignature = combined
		s.stableRounds = 1
	}

	allErrors := len(normalizedSummaries) > 0
	for _, summary := range normalizedSummaries {
		if !isErrorProgressSummary(summary) {
			allErrors = false
			break
		}
	}
	if allErrors {
		errorSig := normalizedDecision + "|" + outcomeSig
		if errorSig == s.lastErrorSignature {
			s.errorRounds++
		} else {
			s.lastErrorSignature = errorSig
			s.errorRounds = 1
		}
	} else {
		s.lastErrorSignature = ""
		s.errorRounds = 0
	}

	familySig := progressToolFamilySignature(normalizedToolSig)
	if familySig != "" {
		familyOutcomeSig := familySig + "|" + outcomeSig
		if familyOutcomeSig == s.lastFamilyOutcomeSig {
			s.familyOutcomeRounds++
		} else {
			s.lastFamilyOutcomeSig = familyOutcomeSig
			s.familyOutcomeRounds = 1
		}
	} else {
		s.lastFamilyOutcomeSig = ""
		s.familyOutcomeRounds = 0
	}

	switch {
	case s.errorRounds >= 3:
		return tools.ToolLoopDetection{
			Abort:     true,
			Reason:    tools.ToolLoopReasonErrorRepeat,
			Streak:    s.errorRounds,
			Signature: s.lastErrorSignature,
		}
	case s.stableRounds >= 3:
		return tools.ToolLoopDetection{
			Abort:     true,
			Reason:    tools.ToolLoopReasonPollingNoProgress,
			Streak:    s.stableRounds,
			Signature: s.lastCombinedSignature,
		}
	case s.familyOutcomeRounds >= 3:
		return tools.ToolLoopDetection{
			Abort:     true,
			Reason:    tools.ToolLoopReasonPollingNoProgress,
			Streak:    s.familyOutcomeRounds,
			Signature: s.lastFamilyOutcomeSig,
		}
	default:
		return tools.ToolLoopDetection{}
	}
}

func (s *ProgressSignatureState) Observe(toolSig, decision string, toolSummaries []string) bool {
	return s.ObserveDetailed(toolSig, decision, toolSummaries).Abort
}
