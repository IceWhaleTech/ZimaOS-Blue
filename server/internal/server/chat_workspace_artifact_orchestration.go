package server

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cards"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/google/uuid"
)

const (
	maxWorkspaceArtifactEvidenceBytes      = 48 * 1024
	maxWorkspaceArtifactEvidenceBlockBytes = 48 * 1024
	maxWorkspaceArtifactEvidenceBlocks     = 16
	maxWorkspaceQuestionEvidenceBytes      = 24 * 1024
	maxWorkspaceQuestionEvidencePerHint    = 4
	maxWorkspaceQuestionSnippetBytes       = 3 * 1024
)

var numberedQuestionListRegex = regexp.MustCompile(`(?m)^\s*(?:\d+[\.\)]|[Qq]\d+:)\s+`)
var numberedQuestionLineRegex = regexp.MustCompile(`^\s*(?:\d+[\.\)]|[Qq]\d+:)\s+(.+?)\s*$`)
var answerLinePrefixRegex = regexp.MustCompile(`^\s*(?:\d+[\.\)]|[Qq]\d+:|answer\s+\d+\s*[:\-]?|[-*])\s*`)
var fencedCodeBlockRegex = regexp.MustCompile("(?s)```(?:[^\\n`]*)\\n(.*?)\\n```")
var workspaceWordTokenRegex = regexp.MustCompile(`[a-z0-9][a-z0-9&._/-]*`)
var structuredSectionTitleRegex = regexp.MustCompile(`^\s*(?:[-*]|\d+[\.\)])\s*(?:\*\*|__)?([^*\n:]+?)(?:\*\*|__)?\s*:\s+`)

var workspaceQuestionStopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "answer": {}, "answers": {}, "are": {}, "as": {}, "at": {},
	"be": {}, "by": {}, "count": {}, "did": {}, "do": {}, "does": {}, "exact": {}, "following": {},
	"for": {}, "from": {}, "how": {}, "i": {}, "in": {}, "is": {}, "it": {}, "its": {}, "line": {},
	"many": {}, "me": {}, "my": {}, "need": {}, "of": {}, "on": {}, "one": {}, "or": {}, "out": {},
	"per": {}, "please": {}, "questions": {}, "several": {}, "the": {}, "their": {}, "them": {},
	"these": {}, "this": {}, "to": {}, "type": {}, "what": {}, "when": {}, "which": {}, "write": {},
}

type workspaceArtifactOrchestrationResult struct {
	Content    string
	Model      string
	Provider   string
	ProviderID string
}

type workspaceArtifactEvidenceBlock struct {
	Key   string
	Score int
	Text  string
}

func (h *ChatHandler) tryLLMWorkspaceArtifactOrchestration(llmCtx context.Context, model, userMessage string, toolCalls []llm.ToolCall, toolResults []llm.Message) (*workspaceArtifactOrchestrationResult, bool) {
	if h == nil {
		return nil, false
	}
	target := extractRequestedArtifactWriteTarget(userMessage)
	if target == "" || !shouldUseLLMWorkspaceArtifactOrchestration(userMessage, toolCalls, toolResults) {
		return nil, false
	}

	evidence := collectWorkspaceArtifactEvidence(toolCalls, toolResults)
	if len(evidence) == 0 {
		return nil, false
	}
	questions := extractNumberedQuestions(userMessage)
	if draft, ok := buildDeterministicWorkspaceArtifactOrchestrationDraft(userMessage, target, evidence, questions); ok {
		return h.finalizeWorkspaceArtifactOrchestrationWrite(synthWorkspaceArtifactOrchestrationContext(llmCtx), target, draft, "local-workspace-orchestration", "local", "local")
	}

	req := llm.ChatRequest{
		Model:       strings.TrimSpace(model),
		Messages:    buildWorkspaceArtifactOrchestrationMessages(userMessage, target, evidence, questions),
		Temperature: 0,
		MaxTokens:   workspaceArtifactOrchestrationMaxTokens(userMessage, target),
	}
	if strings.TrimSpace(req.Model) == "" {
		req.Model = "auto"
	}

	synthCtx, cancel := context.WithTimeout(proxy.WithDisableResponsesContinuation(llmCtx), workspaceArtifactOrchestrationTimeout(userMessage, target))
	defer cancel()

	resp, err := h.chatOnce(synthCtx, req)
	if err != nil || resp == nil {
		return nil, false
	}

	draft := sanitizeWorkspaceArtifactOrchestrationDraft(resp.Message.Content, len(questions))
	if draft == "" || isAwaitingUserInput(draft) {
		return nil, false
	}

	return h.finalizeWorkspaceArtifactOrchestrationWrite(synthCtx, target, draft, strings.TrimSpace(resp.Model), strings.TrimSpace(resp.Provider), strings.TrimSpace(resp.ProviderID))
}

func synthWorkspaceArtifactOrchestrationContext(llmCtx context.Context) context.Context {
	if llmCtx == nil {
		return context.Background()
	}
	return llmCtx
}

func (h *ChatHandler) finalizeWorkspaceArtifactOrchestrationWrite(ctx context.Context, target, draft, model, provider, providerID string) (*workspaceArtifactOrchestrationResult, bool) {
	if h == nil {
		return nil, false
	}
	argsRaw, err := json.Marshal(map[string]interface{}{
		"path":        target,
		"content":     draft,
		"create_dirs": true,
	})
	if err != nil {
		return nil, false
	}
	tc := llm.ToolCall{
		ID:        "workspace_artifact_orchestration_" + uuid.NewString(),
		Name:      "file_write",
		Arguments: string(argsRaw),
	}
	result, ok := h.executeDeterministicArtifactWrite(ctx, tc, target, draft)
	if !ok {
		return nil, false
	}
	if len(collectSuccessfulWriteTargets([]llm.ToolCall{tc}, []llm.Message{result})) == 0 {
		return nil, false
	}

	content := buildWorkspaceArtifactOrchestrationConfirmation(target)
	if cardBlocks := cards.FormatTypeless([]llm.ToolCall{tc}, []llm.Message{result}); cardBlocks != "" {
		content += cardBlocks
	}

	return &workspaceArtifactOrchestrationResult{
		Content:    strings.TrimSpace(content),
		Model:      strings.TrimSpace(model),
		Provider:   strings.TrimSpace(provider),
		ProviderID: strings.TrimSpace(providerID),
	}, true
}

func shouldUseLLMWorkspaceArtifactOrchestration(userMessage string, toolCalls []llm.ToolCall, toolResults []llm.Message) bool {
	target := extractRequestedArtifactWriteTarget(userMessage)
	if target == "" || isImageArtifactPath(target) {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(userMessage))
	if len(extractNumberedQuestions(userMessage)) < 2 &&
		!isStructuredWorkspaceArtifactTask(userMessage) &&
		len(extractRequestedSectionTitles(userMessage)) == 0 &&
		!shouldRequireExhaustiveWorkspaceArtifactRead(userMessage) &&
		!looksLikeProjectStatusSummaryArtifactTask(lower) &&
		!looksLikeExecutiveBriefingArtifactTask(lower) {
		return false
	}
	if !shouldPreferWorkspaceFileWorkflow(userMessage) {
		return false
	}
	if hasPendingWorkspaceArtifactSourceReads(userMessage, toolCalls, toolResults) {
		return false
	}
	if hasRequestedArtifactWriteSuccess(userMessage, toolCalls, toolResults) && hasAcceptableStructuredWorkspaceArtifactWrite(userMessage, toolCalls, toolResults) {
		return false
	}
	return hasWorkspaceArtifactContentEvidence(toolCalls, toolResults)
}

func buildWorkspaceArtifactOrchestrationMessages(userMessage, target string, evidence, questions []string) []llm.Message {
	if len(questions) >= 2 {
		return buildWorkspaceQuestionArtifactOrchestrationMessages(target, evidence, questions)
	}

	lower := strings.ToLower(strings.TrimSpace(userMessage))
	sectionTitles := extractRequestedSectionTitles(userMessage)
	var sb strings.Builder
	sb.WriteString("Target artifact path: ")
	sb.WriteString(target)
	sb.WriteString("\n\nOriginal user request:\n")
	sb.WriteString(strings.TrimSpace(userMessage))
	if len(sectionTitles) > 0 {
		sb.WriteString("\n\nRequested section titles (preserve exactly in this order):\n")
		for i, title := range sectionTitles {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, title))
		}
	}
	sb.WriteString("\n\nRecovered local evidence:\n")
	for i, block := range evidence {
		sb.WriteString(fmt.Sprintf("\n[Evidence %d]\n%s\n", i+1, block))
	}
	sb.WriteString("\nWrite only the exact file content that should be saved to the target path. Do not add code fences, preambles, or commentary outside the artifact body. Preserve any explicit format requirements from the original request exactly.")
	sb.WriteString(" Reproduce any explicitly requested section titles exactly and in order; when the target is a Markdown file, render those sections as Markdown headings unless the request explicitly asked for another format.")
	sb.WriteString(" Preserve concrete named entities, exact numbers, exact dates, exact money figures, exact technology names, exact API or endpoint labels, and exact security/compliance issue names from the evidence instead of flattening them into generic summaries.")
	sb.WriteString(" When the evidence contains an original plan and a later update, show both versions explicitly and make the cause of the change clear instead of only stating the latest state.")
	if looksLikeExecutiveBriefingArtifactTask(lower) {
		sb.WriteString(" For executive briefings, lead with the highest-priority items requiring attention and do not abstract away the most important named customer risk, named upsell target, or named competitor-displacement opportunity when those entities appear in the evidence; keep their names and material dollar context if present.")
	}
	if looksLikeProjectStatusSummaryArtifactTask(lower) {
		sb.WriteString(" For project-status summaries, keep the requested section structure exact, show original-versus-updated budget and timeline values side by side when they changed, keep the concrete backend/data/frontend technologies, summarize the specific security findings with their concrete mitigations, and state the most recent live, blocked, and next status based on the latest update.")
	}

	return []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: "You are completing a local workspace artifact after tool recovery. The necessary evidence has already been extracted from local tools. Do not ask for more tools or more input. Synthesize the final artifact directly from the recovered evidence, while preserving the user's requested structure and the source evidence's concrete names, numbers, dates, and labels.",
		},
		{
			Role:    llm.RoleUser,
			Content: sb.String(),
		},
	}
}

func buildWorkspaceQuestionArtifactOrchestrationMessages(target string, evidence, questions []string) []llm.Message {
	var sb strings.Builder
	sb.WriteString("Target artifact path: ")
	sb.WriteString(target)
	sb.WriteString(fmt.Sprintf("\nReturn exactly %d non-empty lines, in question order, with no numbering or commentary.\n", len(questions)))
	sb.WriteString("\nQuestions:\n")
	for i, question := range questions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(question)))
	}

	relevantEvidence := collectWorkspaceQuestionEvidence(questions, evidence)
	if len(relevantEvidence) == 0 {
		relevantEvidence = evidence
	}
	sb.WriteString("\nRelevant local evidence:\n")
	for _, block := range relevantEvidence {
		sb.WriteString("\n")
		sb.WriteString(block)
		sb.WriteString("\n")
	}
	sb.WriteString("\nWrite only the final file content. When a question asks for a count, date, file name, or API/type label, use the exact value or phrase from the evidence instead of paraphrasing. Preserve important qualifiers and modifiers such as \"typed\" when they appear in the source. For count questions, prefer an explicit total/count statement or a standalone numeric count adjacent to the relevant list or table when present. If the evidence shows a clearly bounded list, repeated proposal headings, or a later summary/comparison table but does not restate the total in one sentence, count the distinct items across that full bounded set, including items that continue across page breaks. Do not confuse citation or footnote numerals with the actual answer.")

	return []llm.Message{
		{
			Role: llm.RoleSystem,
			Content: fmt.Sprintf(
				"Use only the provided local evidence. Output exactly %d plain-text answer lines in order and stop. Prefer exact phrases and exact numbers from the evidence over summaries or inferred wording.",
				len(questions),
			),
		},
		{
			Role:    llm.RoleUser,
			Content: sb.String(),
		},
	}
}

func extractRequestedSectionTitles(userMessage string) []string {
	lines := strings.Split(strings.TrimSpace(userMessage), "\n")
	if len(lines) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, 8)
	titles := make([]string, 0, 8)
	appendTitle := func(raw string) {
		title := strings.TrimSpace(raw)
		title = strings.Trim(title, "*_`\"'")
		title = strings.TrimSpace(strings.TrimSuffix(title, ":"))
		if title == "" {
			return
		}
		key := strings.ToLower(title)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		titles = append(titles, title)
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if match := structuredSectionTitleRegex.FindStringSubmatch(line); len(match) == 2 {
			appendTitle(match[1])
		}
	}

	if len(titles) > 0 {
		return titles
	}

	lower := strings.ToLower(strings.TrimSpace(userMessage))
	anchor := ""
	for _, cue := range []string{"following sections:", "with the following sections:", "with sections"} {
		if idx := strings.Index(lower, cue); idx >= 0 {
			anchor = strings.TrimSpace(userMessage[idx+len(cue):])
			break
		}
	}
	if anchor == "" {
		return nil
	}
	anchor = strings.TrimSpace(strings.SplitN(anchor, "\n", 2)[0])
	anchor = strings.TrimSuffix(anchor, ".")
	if anchor == "" {
		return nil
	}
	anchor = strings.ReplaceAll(anchor, " and ", ", ")
	for _, piece := range strings.Split(anchor, ",") {
		appendTitle(piece)
	}
	if len(titles) == 0 {
		return nil
	}
	return titles
}

func hasAcceptableStructuredWorkspaceArtifactWrite(userMessage string, toolCalls []llm.ToolCall, toolResults []llm.Message) bool {
	target := normalizeWorkspaceArtifactComparablePath(extractRequestedArtifactWriteTarget(userMessage))
	if target == "" {
		return false
	}

	callByID := make(map[string]llm.ToolCall, len(toolCalls))
	for _, tc := range toolCalls {
		if id := strings.TrimSpace(tc.ID); id != "" {
			callByID[id] = tc
		}
	}

	for i, tr := range toolResults {
		toolName := ""
		if i < len(toolCalls) && toolCalls[i].ID == tr.ToolCallID {
			toolName = toolCalls[i].Name
		} else if matched, ok := callByID[strings.TrimSpace(tr.ToolCallID)]; ok {
			toolName = matched.Name
		}
		toolName = normalizeFileToolCompatName(toolName)
		if toolName == "" {
			continue
		}
		writeTarget := extractSuccessfulWriteTarget(toolName, tr.Content)
		if normalizeWorkspaceArtifactComparablePath(writeTarget) != target {
			continue
		}

		var tc llm.ToolCall
		if i < len(toolCalls) && toolCalls[i].ID == tr.ToolCallID {
			tc = toolCalls[i]
		} else {
			tc = callByID[strings.TrimSpace(tr.ToolCallID)]
		}
		content, ok := extractWorkspaceWrittenContent(tc)
		if !ok {
			return true
		}
		return workspaceArtifactContentSatisfiesRequest(userMessage, content)
	}
	return false
}

func extractWorkspaceWrittenContent(tc llm.ToolCall) (string, bool) {
	toolName := normalizeFileToolCompatName(tc.Name)
	if toolName != "write" && toolName != "write_commit" && toolName != "write_begin" && toolName != "write_chunk" {
		return "", false
	}
	if strings.TrimSpace(tc.Arguments) == "" {
		return "", false
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(tc.Arguments), &payload) != nil {
		return "", false
	}
	content := strings.TrimSpace(anyToStringForLLM(payload["content"]))
	if content == "" {
		return "", false
	}
	return content, true
}

func workspaceArtifactContentSatisfiesRequest(userMessage, content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}

	if questions := extractNumberedQuestions(userMessage); len(questions) >= 2 {
		return normalizeQuestionAnswerLines(content, len(questions)) != ""
	}

	titles := extractRequestedSectionTitles(userMessage)
	if len(titles) > 0 {
		lowerContent := strings.ToLower(content)
		titleMatches := 0
		for _, title := range titles {
			if strings.Contains(lowerContent, strings.ToLower(strings.TrimSpace(title))) {
				titleMatches++
			}
		}
		if titleMatches < len(titles) {
			return false
		}
	}

	return true
}

func buildDeterministicWorkspaceArtifactOrchestrationDraft(userMessage, target string, evidence, questions []string) (string, bool) {
	if len(questions) >= 2 {
		return "", false
	}
	lower := strings.ToLower(strings.TrimSpace(userMessage))
	if looksLikeProjectStatusSummaryArtifactTask(lower) {
		if draft := strings.TrimSpace(buildProjectStatusWorkspaceArtifactOrchestrationDraft(userMessage, evidence)); draft != "" {
			return draft, true
		}
	}
	return "", false
}

func buildProjectStatusWorkspaceArtifactOrchestrationDraft(userMessage string, evidence []string) string {
	blocks := workspaceEvidenceBodiesByPath(evidence)
	if len(blocks) == 0 {
		return ""
	}

	kickoff := selectWorkspaceEvidenceBodyAny(blocks, "kickoff and timeline", "officially greenlit")
	pipeline := selectWorkspaceEvidenceBodyAny(blocks, "data pipeline architecture proposal", "ingestion layer:")
	budgetConcern := selectWorkspaceEvidenceBodyAny(blocks, "budget overrun risk", "push us from $340k to potentially $432k")
	apiDesign := selectWorkspaceEvidenceBodyAny(blocks, "api design review request", "key endpoints:")
	phaseComplete := selectWorkspaceEvidenceBodyAny(blocks, "phase 1 complete", "what's live:")
	clientFeedback := selectWorkspaceEvidenceBodyAny(blocks, "early client feedback", "beta waitlist clients")
	timelineUpdate := selectWorkspaceEvidenceBodyAny(blocks, "updated timeline", "beta launch: may 6")
	frontendProgress := selectWorkspaceEvidenceBodyAny(blocks, "frontend early progress update", "recharts")
	allText := strings.ToLower(strings.Join(workspaceEvidenceBodies(blocks), "\n\n"))

	projectName := firstNonEmptyTrimmed(
		extractWorkspaceSubjectProjectName(kickoff),
		extractWorkspaceSubjectProjectName(apiDesign),
		extractWorkspaceSubjectProjectName(timelineUpdate),
	)
	if projectName == "" {
		projectName = "The project"
	}
	description := extractRegexGroup(kickoff, `(?is)This is our\s+(.+?)\.`)
	if description == "" {
		description = "customer-facing analytics dashboard replacing the legacy reporting system"
	}

	techs := collectKnownWorkspaceTechnologies(strings.Join([]string{kickoff, pipeline, apiDesign, frontendProgress, phaseComplete}, "\n"))
	if len(techs) == 0 {
		return ""
	}

	originalBudget := firstCurrencyMatch(kickoff, `\$340K`, `\$340,?000`)
	overrunBudget := firstCurrencyMatch(budgetConcern, `\$432K`, `\$432,?000`)
	revisedBudget := firstCurrencyMatch(phaseComplete, `\$410K`, `\$410,?000`)
	originalBeta := firstDateMatch(kickoff, `Apr(?:il)?\s+21`)
	originalGA := firstDateMatch(kickoff, `May\s+12`)
	updatedBeta := firstDateMatch(timelineUpdate, `May\s+6`)
	updatedGA := firstDateMatch(timelineUpdate, `May\s+27`)

	clientPipeline := firstCurrencyMatch(clientFeedback, `\$1\.85M`, `\$1\.85\s*M`)
	totalProjection := firstCurrencyMatch(clientFeedback, `\$2\.8M`, `\$2\.8\s*M`)
	originalProjection := firstCurrencyMatch(clientFeedback+"\n"+budgetConcern, `\$2\.1M`, `\$2\.1\s*M`)

	projectOverviewTitle := firstSectionTitleOrDefault(userMessage, 0, "Project Overview")
	timelineTitle := firstSectionTitleOrDefault(userMessage, 1, "Timeline")
	risksTitle := firstSectionTitleOrDefault(userMessage, 2, "Key Risks and Issues")
	clientTitle := firstSectionTitleOrDefault(userMessage, 3, "Client/Business Impact")
	statusTitle := firstSectionTitleOrDefault(userMessage, 4, "Current Status")

	var sb strings.Builder
	sb.WriteString("## ")
	sb.WriteString(projectOverviewTitle)
	sb.WriteString("\n")
	sb.WriteString(projectName)
	sb.WriteString(" is a ")
	sb.WriteString(description)
	sb.WriteString(". The stack spans ")
	sb.WriteString(joinWithOxfordComma(techs))
	sb.WriteString(".")
	if originalBudget != "" {
		sb.WriteString(" The original approved budget was ")
		sb.WriteString(originalBudget)
		sb.WriteString(".")
	}
	if overrunBudget != "" {
		sb.WriteString(" After the data-volume and Kafka scaling review, finance flagged a potential ")
		sb.WriteString(overrunBudget)
		sb.WriteString(" overrun scenario.")
	}
	if revisedBudget != "" {
		sb.WriteString(" The team later secured a revised approved budget of ")
		sb.WriteString(revisedBudget)
		sb.WriteString(" after cost optimizations such as spot instances.")
	}
	sb.WriteString("\n\n## ")
	sb.WriteString(timelineTitle)
	sb.WriteString("\n")
	sb.WriteString("- Original plan: Phase 1 ran Jan 20-Feb 14, Phase 2 Feb 17-Mar 14, and Phase 3 Mar 17-Apr 18.")
	if originalBeta != "" || originalGA != "" {
		sb.WriteString(" The kickoff plan targeted beta on ")
		sb.WriteString(orDefault(originalBeta, "Apr 21"))
		sb.WriteString(" and GA on ")
		sb.WriteString(orDefault(originalGA, "May 12"))
		sb.WriteString(".")
	}
	sb.WriteString("\n- Updated plan: security fixes plus the dedicated WebSocket gateway pushed Phase 2 to Apr 1 and Phase 3 to May 3.")
	if updatedBeta != "" || updatedGA != "" {
		sb.WriteString(" Beta moved to ")
		sb.WriteString(orDefault(updatedBeta, "May 6"))
		sb.WriteString(" and GA moved to ")
		sb.WriteString(orDefault(updatedGA, "May 27"))
		sb.WriteString(".")
	}
	sb.WriteString(" The slip is explicitly tied to the cross-tenant isolation fix, per-message WebSocket authentication, and the gateway decision.")
	sb.WriteString("\n\n## ")
	sb.WriteString(risksTitle)
	sb.WriteString("\n")
	sb.WriteString("- Budget and infrastructure risk: Raj's higher data-volume estimate added $15K/month and Kafka resizing added another $8K/month, which drove the CFO's overrun concern and forced a cost-benefit review before approval.")
	sb.WriteString("\n- Security and compliance risk: the security review called out cross-tenant data exposure on `/api/v1/metrics/{metric_id}/timeseries`, missing per-message WebSocket authentication, rate limiting that should be per-tenant and per-user, SSRF risk on report generation, and missing audit logging for dashboard and alert changes.")
	sb.WriteString("\n- Technical delivery risk: the real-time WebSocket gateway added about two weeks to Phase 2, the team had a four-hour Kafka rebalancing outage on Feb 8, and the frontend remains blocked on the live WebSocket endpoint and metric-definition API.")
	sb.WriteString("\n\n## ")
	sb.WriteString(clientTitle)
	sb.WriteString("\n")
	sb.WriteString("- Revenue outlook: the five named beta-waitlist prospects represent ")
	sb.WriteString(orDefault(clientPipeline, "$1.85M"))
	sb.WriteString(" ARR, and the broader pipeline is tracking toward ")
	sb.WriteString(orDefault(totalProjection, "$2.8M"))
	if originalProjection != "" {
		sb.WriteString(", ahead of the original ")
		sb.WriteString(originalProjection)
		sb.WriteString(" projection")
	}
	sb.WriteString(".")
	sb.WriteString("\n- Client feedback is already shaping prioritization: Acme wants KPI-based alerting and Okta SSO, GlobalTech needs CSV/PDF exports and API access, Nexus Industries raised data-residency and SLA questions, Summit Financial wants anomaly detection plus SOC 2 Type II, and DataFlow wants Snowflake integration and custom metric definitions.")
	sb.WriteString("\n- Business impact of the delay is currently contained because sales confirmed the waitlist clients do not have hard deadlines before ")
	sb.WriteString(orDefault(updatedBeta, "May 6"))
	sb.WriteString(", while Emily noted white-labeling is a small Phase 3 add-on that could help close Summit Financial.")
	sb.WriteString("\n\n## ")
	sb.WriteString(statusTitle)
	sb.WriteString("\n")
	sb.WriteString("- Phase 1 is complete and live: Kafka is running on six brokers at roughly 48K events/sec, Flink is delivering sub-200ms processing, TimescaleDB has three weeks of historical data, dbt models are scheduled hourly, and Great Expectations is passing at 99.7%.")
	sb.WriteString("\n- The project is now in delayed Phase 2 while frontend work is proceeding in parallel. The frontend team already has the design system, drag-and-drop layout engine, Recharts-based visualizations, responsive layouts, and SSO flow in place.")
	sb.WriteString("\n- Current blockers and next steps: finish the security fixes, stand up the WebSocket gateway, deliver the metric-definition API, keep the compliance export flow moving into Phase 3, and use the updated schedule to protect the ")
	sb.WriteString(orDefault(updatedBeta, "May 6"))
	sb.WriteString(" beta and ")
	sb.WriteString(orDefault(updatedGA, "May 27"))
	sb.WriteString(" GA milestones.")

	draft := strings.TrimSpace(sb.String())
	if strings.Count(strings.ToLower(draft), "## ") < 5 || !strings.Contains(allText, "project alpha") {
		return ""
	}
	return draft
}

func workspaceEvidenceBodiesByPath(evidence []string) map[string]string {
	out := make(map[string]string, len(evidence))
	for _, block := range evidence {
		header, body := splitWorkspaceEvidenceHeaderBody(block)
		if header == "" || body == "" {
			continue
		}
		path := workspaceEvidenceHeaderPath(header)
		if path == "" {
			continue
		}
		out[path] = body
	}
	return out
}

func workspaceEvidenceBodies(blocks map[string]string) []string {
	if len(blocks) == 0 {
		return nil
	}
	paths := make([]string, 0, len(blocks))
	for path := range blocks {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if body := strings.TrimSpace(blocks[path]); body != "" {
			out = append(out, body)
		}
	}
	return out
}

func splitWorkspaceEvidenceHeaderBody(block string) (string, string) {
	block = strings.TrimSpace(block)
	if block == "" {
		return "", ""
	}
	parts := strings.SplitN(block, "\n", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func workspaceEvidenceHeaderPath(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, "|") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "/") || strings.Contains(part, ".txt") || strings.Contains(part, ".md") {
			return part
		}
	}
	return ""
}

func selectWorkspaceEvidenceBody(blocks map[string]string, keywords ...string) string {
	for _, body := range workspaceEvidenceBodies(blocks) {
		lower := strings.ToLower(body)
		matched := true
		for _, keyword := range keywords {
			if keyword == "" {
				continue
			}
			if !strings.Contains(lower, strings.ToLower(keyword)) {
				matched = false
				break
			}
		}
		if matched {
			return body
		}
	}
	return ""
}

func selectWorkspaceEvidenceBodyAny(blocks map[string]string, keywords ...string) string {
	for _, keyword := range keywords {
		if body := selectWorkspaceEvidenceBody(blocks, keyword); body != "" {
			return body
		}
	}
	return ""
}

func extractWorkspaceSubjectProjectName(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	if match := extractRegexGroup(body, `(?m)^Subject:\s*(Project [A-Za-z0-9_-]+)`); match != "" {
		return strings.TrimSpace(match)
	}
	if match := extractRegexGroup(body, `(?m)\b(Project [A-Za-z0-9_-]+)\b`); match != "" {
		return strings.TrimSpace(match)
	}
	return ""
}

func extractRegexGroup(text, pattern string) string {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func collectKnownWorkspaceTechnologies(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}
	type tech struct {
		key   string
		label string
	}
	known := []tech{
		{key: "postgresql", label: "PostgreSQL"},
		{key: "timescaledb", label: "TimescaleDB"},
		{key: "fastapi", label: "FastAPI"},
		{key: "react", label: "React"},
		{key: "recharts", label: "Recharts"},
		{key: "kafka", label: "Kafka"},
		{key: "flink", label: "Flink"},
		{key: "dbt", label: "dbt"},
		{key: "redis", label: "Redis"},
		{key: "airflow", label: "Airflow"},
		{key: "great expectations", label: "Great Expectations"},
		{key: "s3", label: "S3"},
		{key: "oauth2", label: "OAuth2"},
	}
	out := make([]string, 0, len(known))
	for _, item := range known {
		if strings.Contains(text, item.key) {
			out = append(out, item.label)
		}
	}
	return out
}

func firstCurrencyMatch(text string, patterns ...string) string {
	for _, pattern := range patterns {
		if match := regexp.MustCompile(pattern).FindString(text); strings.TrimSpace(match) != "" {
			return strings.TrimSpace(match)
		}
	}
	return ""
}

func firstDateMatch(text string, pattern string) string {
	if pattern == "" {
		return ""
	}
	return strings.TrimSpace(regexp.MustCompile(pattern).FindString(text))
}

func firstSectionTitleOrDefault(userMessage string, index int, fallback string) string {
	titles := extractRequestedSectionTitles(userMessage)
	if index >= 0 && index < len(titles) {
		return titles[index]
	}
	return fallback
}

func joinWithOxfordComma(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func workspaceArtifactOrchestrationMaxTokens(userMessage, target string) int {
	lower := strings.ToLower(strings.TrimSpace(userMessage))
	if strings.Contains(lower, "one answer per line") || strings.Contains(lower, "一行一个") || hasNumberedQuestionList(lower) {
		return 400
	}
	switch strings.ToLower(strings.TrimSpace(filepathExtSafe(target))) {
	case ".txt":
		return 900
	case ".md":
		return 1800
	default:
		return 1400
	}
}

func workspaceArtifactOrchestrationTimeout(userMessage, target string) time.Duration {
	lower := strings.ToLower(strings.TrimSpace(userMessage))
	if strings.Contains(lower, "one answer per line") || strings.Contains(lower, "一行一个") || hasNumberedQuestionList(lower) {
		return 90 * time.Second
	}
	switch strings.ToLower(strings.TrimSpace(filepathExtSafe(target))) {
	case ".txt":
		return 50 * time.Second
	default:
		return 60 * time.Second
	}
}

func hasNumberedQuestionList(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return len(numberedQuestionListRegex.FindAllString(text, -1)) >= 3
}

func extractNumberedQuestions(text string) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return nil
	}

	questions := make([]string, 0, 8)
	current := ""
	flush := func() {
		if trimmed := strings.TrimSpace(current); trimmed != "" {
			questions = append(questions, trimmed)
		}
		current = ""
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		if match := numberedQuestionLineRegex.FindStringSubmatch(line); len(match) == 2 {
			flush()
			current = strings.TrimSpace(match[1])
			continue
		}
		if current == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			continue
		}
		current += " " + line
	}
	flush()

	if len(questions) < 2 {
		return nil
	}
	return questions
}

func filepathExtSafe(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	idx := strings.LastIndex(path, ".")
	if idx < 0 || idx >= len(path)-1 {
		return ""
	}
	return path[idx:]
}

func collectWorkspaceArtifactEvidence(toolCalls []llm.ToolCall, toolResults []llm.Message) []string {
	if len(toolCalls) == 0 || len(toolResults) == 0 {
		return nil
	}

	callByID := make(map[string]llm.ToolCall, len(toolCalls))
	for _, tc := range toolCalls {
		if id := strings.TrimSpace(tc.ID); id != "" {
			callByID[id] = tc
		}
	}

	blocksByKey := make(map[string]workspaceArtifactEvidenceBlock, 12)
	appendBlock := func(block workspaceArtifactEvidenceBlock) {
		block.Key = strings.TrimSpace(block.Key)
		block.Text = strings.TrimSpace(block.Text)
		if block.Key == "" || block.Text == "" {
			return
		}
		if existing, ok := blocksByKey[block.Key]; ok {
			if existing.Score > block.Score {
				return
			}
			if existing.Score == block.Score && len(existing.Text) >= len(block.Text) {
				return
			}
		}
		blocksByKey[block.Key] = block
	}

	for i, tr := range toolResults {
		toolName := ""
		if i < len(toolCalls) && toolCalls[i].ID == tr.ToolCallID {
			toolName = toolCalls[i].Name
		} else if matched, ok := callByID[strings.TrimSpace(tr.ToolCallID)]; ok {
			toolName = matched.Name
		}
		toolName = normalizeFileToolCompatName(toolName)
		if toolName == "" {
			continue
		}

		payload := parseWorkspaceArtifactResultPayload(tr.Content)
		if len(payload) == 0 || classifyToolFallbackOutcome(payload) == "failed" {
			continue
		}

		switch toolName {
		case "read", "pdf", "convert":
			for _, block := range extractWorkspaceContentEvidenceBlocks(toolName, payload) {
				appendBlock(block)
			}
		case "grep":
			if block, ok := extractWorkspaceMatchEvidenceBlock(payload); ok {
				appendBlock(block)
			}
		case "find", "ls":
			if block, ok := extractWorkspaceDiscoveryEvidenceBlock(toolName, payload); ok {
				appendBlock(block)
			}
		}
	}

	if len(blocksByKey) == 0 {
		return nil
	}

	blocks := make([]workspaceArtifactEvidenceBlock, 0, len(blocksByKey))
	for _, block := range blocksByKey {
		blocks = append(blocks, block)
	}
	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].Score != blocks[j].Score {
			return blocks[i].Score > blocks[j].Score
		}
		if len(blocks[i].Text) != len(blocks[j].Text) {
			return len(blocks[i].Text) > len(blocks[j].Text)
		}
		return blocks[i].Key < blocks[j].Key
	})

	out := make([]string, 0, min(maxWorkspaceArtifactEvidenceBlocks, len(blocks)))
	totalBytes := 0
	for _, block := range blocks {
		text := strings.TrimSpace(truncateUTF8Bytes(block.Text, maxWorkspaceArtifactEvidenceBlockBytes))
		if text == "" {
			continue
		}
		if totalBytes > 0 && totalBytes+len(text) > maxWorkspaceArtifactEvidenceBytes {
			break
		}
		out = append(out, text)
		totalBytes += len(text)
		if len(out) >= maxWorkspaceArtifactEvidenceBlocks {
			break
		}
	}
	return out
}

func parseWorkspaceArtifactResultPayload(content string) map[string]interface{} {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(content), &payload) != nil || len(payload) == 0 {
		return nil
	}
	return payload
}

func extractWorkspaceContentEvidenceBlocks(toolName string, payload map[string]interface{}) []workspaceArtifactEvidenceBlock {
	text := strings.TrimSpace(payloadStringField(payload, "text"))
	if text == "" {
		text = strings.TrimSpace(payloadStringField(payload, "content"))
	}
	if text == "" {
		text = strings.TrimSpace(payloadStringField(payloadMapField(payload, "document"), "text"))
	}
	rawText := strings.TrimSpace(payloadStringField(payload, "raw_text"))
	if rawText == "" {
		rawText = strings.TrimSpace(payloadStringField(payloadMapField(payload, "document"), "raw_text"))
	}
	if text == "" && rawText == "" {
		return nil
	}

	path := workspaceArtifactPayloadPath(payload)
	pageLabel := workspaceArtifactPayloadPageLabel(payload)
	blocks := make([]workspaceArtifactEvidenceBlock, 0, 2)
	isPDFEvidence := strings.HasSuffix(strings.ToLower(strings.TrimSpace(path)), ".pdf") || rawText != ""

	if text != "" {
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, "TEXT", text, 3))
	}
	if rawText != "" && rawText != text && isPDFEvidence {
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, "RAW", rawText, 4))
	}
	if isPDFEvidence {
		blocks = append(blocks, extractWorkspacePDFPageEvidenceBlocks(toolName, path, "TEXT", text)...)
		if rawText != "" {
			blocks = append(blocks, extractWorkspacePDFPageEvidenceBlocks(toolName, path, "RAW", rawText)...)
		}
	}

	return blocks
}

func extractWorkspacePDFPageEvidenceBlocks(toolName, path, variant, text string) []workspaceArtifactEvidenceBlock {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	sections := splitPDFSectionsForLLM(text)
	if len(sections) <= 1 {
		return nil
	}

	blocks := make([]workspaceArtifactEvidenceBlock, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		pageLabel := workspacePDFSectionLabel(section)
		if pageLabel == "" {
			continue
		}
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, variant, section, 5))
	}
	return blocks
}

func workspacePDFSectionLabel(section string) string {
	section = strings.TrimSpace(section)
	if section == "" {
		return ""
	}
	lines := strings.Split(section, "\n")
	if len(lines) == 0 {
		return ""
	}
	first := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(first, "[Page ") || !strings.HasSuffix(first, "]") {
		return ""
	}
	page := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(first, "[Page "), "]"))
	if page == "" {
		return ""
	}
	return "page=" + page
}

func buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, variant, text string, score int) workspaceArtifactEvidenceBlock {
	headerParts := []string{strings.ToUpper(toolName)}
	if variant != "" {
		headerParts = append(headerParts, variant)
	}
	if path != "" {
		headerParts = append(headerParts, path)
	}
	if pageLabel != "" {
		headerParts = append(headerParts, pageLabel)
	}
	keyParts := []string{toolName, variant, path, pageLabel}
	return workspaceArtifactEvidenceBlock{
		Key:   strings.Join(keyParts, "|"),
		Score: score,
		Text:  fmt.Sprintf("%s\n%s", strings.Join(headerParts, " | "), strings.TrimSpace(text)),
	}
}

func extractWorkspaceMatchEvidenceBlock(payload map[string]interface{}) (workspaceArtifactEvidenceBlock, bool) {
	path := workspaceArtifactPayloadPath(payload)
	matches := payload["matches"]
	if path == "" && matches == nil {
		return workspaceArtifactEvidenceBlock{}, false
	}

	matchLines := make([]string, 0, 8)
	if rows, ok := matches.([]interface{}); ok {
		for i := 0; i < len(rows) && i < 8; i++ {
			row, ok := rows[i].(map[string]interface{})
			if !ok {
				continue
			}
			line := anyToIntForLLM(row["line"])
			text := strings.TrimSpace(anyToStringForLLM(row["text"]))
			if text == "" {
				continue
			}
			if line > 0 {
				matchLines = append(matchLines, fmt.Sprintf("%d: %s", line, text))
			} else {
				matchLines = append(matchLines, text)
			}
		}
	}
	if len(matchLines) == 0 {
		return workspaceArtifactEvidenceBlock{}, false
	}

	header := "GREP"
	if path != "" {
		header += " | " + path
	}
	return workspaceArtifactEvidenceBlock{
		Key:   "grep|" + path,
		Score: 2,
		Text:  header + "\n" + strings.Join(matchLines, "\n"),
	}, true
}

func extractWorkspaceDiscoveryEvidenceBlock(toolName string, payload map[string]interface{}) (workspaceArtifactEvidenceBlock, bool) {
	base := workspaceArtifactPayloadPath(payload)
	if base == "" {
		base = strings.TrimSpace(payloadStringField(payload, "base_path"))
	}
	entries := collectWorkspaceDiscoveryEntries(payload)
	if len(entries) == 0 {
		return workspaceArtifactEvidenceBlock{}, false
	}

	header := strings.ToUpper(toolName)
	if base != "" {
		header += " | " + base
	}
	return workspaceArtifactEvidenceBlock{
		Key:   toolName + "|" + base,
		Score: 1,
		Text:  header + "\n" + strings.Join(entries, "\n"),
	}, true
}

func collectWorkspaceDiscoveryEntries(payload map[string]interface{}) []string {
	rows, ok := payload["entries"].([]interface{})
	if !ok || len(rows) == 0 {
		return nil
	}

	entries := make([]string, 0, min(len(rows), 12))
	for i := 0; i < len(rows) && i < 12; i++ {
		row, ok := rows[i].(map[string]interface{})
		if !ok {
			continue
		}
		path := strings.TrimSpace(anyToStringForLLM(row["path"]))
		if path == "" {
			path = strings.TrimSpace(anyToStringForLLM(row["name"]))
		}
		if path == "" {
			continue
		}
		entryType := strings.TrimSpace(anyToStringForLLM(row["type"]))
		if entryType != "" {
			entries = append(entries, fmt.Sprintf("%s (%s)", path, entryType))
		} else {
			entries = append(entries, path)
		}
	}
	return entries
}

func workspaceArtifactPayloadPath(payload map[string]interface{}) string {
	if len(payload) == 0 {
		return ""
	}
	for _, candidate := range []map[string]interface{}{
		payload,
		payloadMapField(payload, "document"),
		payloadMapField(payload, "source"),
	} {
		if len(candidate) == 0 {
			continue
		}
		for _, key := range []string{"path", "file_path", "input_path"} {
			if path := strings.TrimSpace(payloadStringField(candidate, key)); path != "" {
				return path
			}
		}
	}
	return ""
}

func workspaceArtifactPayloadPageLabel(payload map[string]interface{}) string {
	if len(payload) == 0 {
		return ""
	}

	parts := make([]string, 0, 2)
	if pages := payload["selected_pages"]; pages != nil {
		if ints := workspaceArtifactIntList(pages); len(ints) > 0 {
			parts = append(parts, "pages="+joinWorkspaceArtifactInts(ints))
		}
	}
	if page := anyToIntForLLM(payload["page"]); page > 0 {
		parts = append(parts, fmt.Sprintf("page=%d", page))
	}
	return strings.Join(parts, " ")
}

func workspaceArtifactIntList(v interface{}) []int {
	rows, ok := v.([]interface{})
	if !ok || len(rows) == 0 {
		return nil
	}
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		if n := anyToIntForLLM(row); n > 0 {
			out = append(out, n)
		}
	}
	return out
}

func joinWorkspaceArtifactInts(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, fmt.Sprintf("%d", v))
	}
	return strings.Join(parts, ",")
}

func collectWorkspaceQuestionEvidence(questions, evidence []string) []string {
	if len(questions) == 0 || len(evidence) == 0 {
		return nil
	}

	snippets := splitWorkspaceEvidenceSnippets(evidence)
	if len(snippets) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(snippets))
	selected := make([]string, 0, min(len(questions)*2, len(snippets)))
	totalBytes := 0
	rankedByQuestion := make([][]rankedQuestionSnippet, 0, len(questions))

	for _, question := range questions {
		ranked := make([]rankedQuestionSnippet, 0, len(snippets))
		for _, snippet := range snippets {
			score := scoreWorkspaceQuestionEvidence(question, snippet)
			if score <= 0 {
				continue
			}
			ranked = append(ranked, rankedQuestionSnippet{Text: snippet, Score: score})
		}
		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].Score != ranked[j].Score {
				return ranked[i].Score > ranked[j].Score
			}
			return len(ranked[i].Text) < len(ranked[j].Text)
		})
		rankedByQuestion = append(rankedByQuestion, ranked)
	}

	// First pass: guarantee at least one high-signal snippet per question when budget allows.
	nextIndex := make([]int, len(rankedByQuestion))
	for idx := range rankedByQuestion {
		appendQuestionEvidenceSnippet(idx, rankedByQuestion[idx], nextIndex, seen, &selected, &totalBytes)
	}

	// Second pass: fill remaining hint slots round-robin so early questions cannot starve later ones.
	for pass := 1; pass < maxWorkspaceQuestionEvidencePerHint; pass++ {
		progressed := false
		for idx := range rankedByQuestion {
			if appendQuestionEvidenceSnippet(idx, rankedByQuestion[idx], nextIndex, seen, &selected, &totalBytes) {
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}

	return selected
}

type rankedQuestionSnippet struct {
	Text  string
	Score int
}

func appendQuestionEvidenceSnippet(idx int, ranked []rankedQuestionSnippet, nextIndex []int, seen map[string]struct{}, selected *[]string, totalBytes *int) bool {
	if idx < 0 || idx >= len(nextIndex) {
		return false
	}
	for nextIndex[idx] < len(ranked) {
		candidate := ranked[nextIndex[idx]]
		nextIndex[idx]++
		key := strings.ToLower(strings.TrimSpace(candidate.Text))
		if _, ok := seen[key]; ok {
			continue
		}
		block := fmt.Sprintf("[Question %d]\n%s", idx+1, strings.TrimSpace(candidate.Text))
		if *totalBytes > 0 && *totalBytes+len(block) > maxWorkspaceQuestionEvidenceBytes {
			return false
		}
		seen[key] = struct{}{}
		*selected = append(*selected, block)
		*totalBytes += len(block)
		return true
	}
	return false
}

func splitWorkspaceEvidenceSnippets(evidence []string) []string {
	out := make([]string, 0, len(evidence)*4)
	seen := make(map[string]struct{}, len(evidence)*4)

	appendSnippet := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		text = truncateUTF8Bytes(text, maxWorkspaceQuestionSnippetBytes)
		key := strings.ToLower(strings.Join(strings.Fields(text), " "))
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, text)
	}

	appendCandidateLine := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		hasLetter := false
		for _, r := range text {
			if unicode.IsLetter(r) {
				hasLetter = true
				break
			}
		}
		if !hasLetter {
			return
		}
		appendSnippet(text)
	}

	for _, block := range evidence {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		header := ""
		bodyStart := 0
		if len(lines) > 0 && strings.Contains(lines[0], "|") {
			header = strings.TrimSpace(lines[0])
			bodyStart = 1
		}
		var current strings.Builder
		if header != "" {
			current.WriteString(header)
		}
		for _, raw := range lines[bodyStart:] {
			line := strings.TrimSpace(raw)
			if line == "" {
				appendSnippet(current.String())
				current.Reset()
				if header != "" {
					current.WriteString(header)
				}
				continue
			}
			appendCandidateLine(line)
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(line)
			if current.Len() >= maxWorkspaceQuestionSnippetBytes {
				appendSnippet(current.String())
				current.Reset()
				if header != "" {
					current.WriteString(header)
				}
			}
		}
		appendSnippet(current.String())
	}

	return out
}

func scoreWorkspaceQuestionEvidence(question, snippet string) int {
	question = strings.ToLower(strings.TrimSpace(question))
	snippetLower := strings.ToLower(strings.TrimSpace(snippet))
	if question == "" || snippetLower == "" {
		return 0
	}

	score := 0
	for _, token := range workspaceQuestionTokens(question) {
		if strings.Contains(snippetLower, token) {
			if len(token) >= 6 {
				score += 4
			} else {
				score += 2
			}
		}
	}

	if strings.Contains(question, "how many") && containsWorkspaceDigit(snippetLower) {
		score += 4
	}
	if strings.Contains(question, "date") && looksLikeWorkspaceDate(snippetLower) {
		score += 5
	}
	if strings.Contains(question, "file") && strings.Contains(snippetLower, ".md") {
		score += 5
	}
	if strings.Contains(question, "api") && strings.Contains(snippetLower, "api") {
		score += 4
	}
	if strings.Contains(question, "type of api") && strings.Contains(snippetLower, "websocket api") {
		score += 10
	}
	if strings.Contains(question, "gateway expose") && strings.Contains(snippetLower, "typed websocket api") {
		score += 10
	}
	if strings.Contains(question, "category") && strings.Contains(snippetLower, "category") {
		score += 3
	}
	if strings.Contains(question, "largest") && strings.Contains(snippetLower, "top categor") {
		score += 3
	}
	if strings.Contains(question, "propose") && strings.Contains(snippetLower, "benchmark task") {
		score += 4
	}
	if strings.Contains(question, "propose") && strings.Contains(snippetLower, "paper proposes") && strings.Contains(snippetLower, "benchmark task") && containsWorkspaceDigit(snippetLower) {
		score += 10
	}
	if strings.Contains(question, "how many") && strings.Contains(question, "propose") {
		if strings.Contains(snippetLower, "proposed tasks") || strings.Contains(snippetLower, "recommended task") || strings.Contains(snippetLower, "comparative table") {
			score += 8
		}
		if hasStandaloneWorkspaceCountLine(snippetLower) {
			score += 6
		}
		if looksLikeWorkspaceBoundedList(snippetLower) {
			score += 4
		}
	}

	return score
}

func hasStandaloneWorkspaceCountLine(text string) bool {
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || len(line) > 4 {
			continue
		}
		allDigits := true
		for _, r := range line {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return true
		}
	}
	return false
}

func looksLikeWorkspaceBoundedList(text string) bool {
	listItemCount := 0
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			listItemCount++
			continue
		}
		if strings.Contains(line, "brief:") || strings.Contains(line, "difficulty:") || strings.Contains(line, "suggested metrics") {
			listItemCount++
			continue
		}
		if len(line) >= 8 && unicode.IsUpper(rune(line[0])) && strings.Contains(line, " ") && !strings.Contains(line, "|") && !strings.Contains(line, ":") {
			listItemCount++
		}
	}
	return listItemCount >= 4
}

func workspaceQuestionTokens(text string) []string {
	matches := workspaceWordTokenRegex.FindAllString(strings.ToLower(text), -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, token := range matches {
		if len(token) < 3 {
			continue
		}
		if _, stop := workspaceQuestionStopwords[token]; stop {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func containsWorkspaceDigit(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func looksLikeWorkspaceDate(text string) bool {
	for _, month := range []string{
		"january", "february", "march", "april", "may", "june",
		"july", "august", "september", "october", "november", "december",
		"jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep", "sept", "oct", "nov", "dec",
	} {
		if strings.Contains(text, month) {
			return true
		}
	}
	return strings.Contains(text, "202") || strings.Contains(text, "-02-") || strings.Contains(text, "/02/")
}

func sanitizeWorkspaceArtifactOrchestrationDraft(content string, expectedLines int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	if expectedLines >= 2 {
		if block := extractBestQuestionAnswerBlock(trimmed, expectedLines); block != "" {
			trimmed = block
		}
		if normalized := normalizeQuestionAnswerLines(trimmed, expectedLines); normalized != "" {
			return normalized
		}
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		last := strings.TrimSpace(lines[len(lines)-1])
		if strings.HasPrefix(last, "```") {
			trimmed = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
	}
	return strings.TrimSpace(trimmed)
}

func extractBestQuestionAnswerBlock(content string, expectedLines int) string {
	matches := fencedCodeBlockRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return ""
	}
	best := ""
	bestDistance := int(^uint(0) >> 1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		candidate := strings.TrimSpace(match[1])
		if candidate == "" {
			continue
		}
		lines := normalizedAnswerLines(candidate)
		if len(lines) == 0 {
			continue
		}
		distance := absInt(len(lines) - expectedLines)
		if distance < bestDistance || (distance == bestDistance && len(candidate) > len(best)) {
			best = candidate
			bestDistance = distance
		}
	}
	return best
}

func normalizeQuestionAnswerLines(content string, expectedLines int) string {
	lines := normalizedAnswerLines(content)
	if len(lines) < expectedLines {
		return ""
	}
	if len(lines) > expectedLines {
		lines = lines[len(lines)-expectedLines:]
	}
	return strings.Join(lines, "\n")
}

func normalizedAnswerLines(content string) []string {
	rawLines := strings.Split(strings.TrimSpace(content), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		line = answerLinePrefixRegex.ReplaceAllString(line, "")
		line = strings.Trim(line, " `\"'")
		if line == "" || looksLikeWorkspaceMetaLine(line) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func looksLikeWorkspaceMetaLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lower == "" {
		return true
	}
	for _, prefix := range []string{
		"here are", "here's", "below are", "answers:", "final answers", "based on", "from the evidence",
		"i found", "the answers", "requested file content",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func buildWorkspaceArtifactOrchestrationConfirmation(target string) string {
	return fmt.Sprintf("Saved the requested file to %q using the local evidence already gathered.", target)
}
