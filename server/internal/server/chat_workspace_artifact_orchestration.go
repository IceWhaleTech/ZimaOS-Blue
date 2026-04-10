package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cards"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
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
	if target == "" {
		return nil, false
	}

	evidence := collectWorkspaceArtifactEvidence(toolCalls, toolResults)
	if len(evidence) == 0 {
		return nil, false
	}
	if !shouldUseLLMWorkspaceArtifactOrchestration(userMessage, toolCalls, toolResults) {
		return nil, false
	}

	questions := extractNumberedQuestions(userMessage)
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
	if !shouldPreferWorkspaceFileWorkflow(userMessage) {
		return false
	}
	if shouldPreferWorkspaceEditWorkflow(userMessage) {
		return false
	}
	if hasPendingWorkspaceArtifactSourceReads(userMessage, toolCalls, toolResults) {
		return false
	}
	if hasRequestedArtifactWriteSuccess(userMessage, toolCalls, toolResults) {
		return false
	}
	return hasWorkspaceArtifactContentEvidence(toolCalls, toolResults)
}

func shouldUseImmediateWorkspaceArtifactOrchestration(userMessage string, currentToolCalls []llm.ToolCall, currentToolResults []llm.Message, historyToolCalls []llm.ToolCall, historyToolResults []llm.Message) bool {
	if len(collectSuccessfulWriteTargets(currentToolCalls, currentToolResults)) > 0 {
		return false
	}
	if shouldPreferWorkspaceEditWorkflow(userMessage) {
		return false
	}
	if !hasWorkspaceArtifactContentEvidence(currentToolCalls, currentToolResults) {
		return false
	}
	target := extractRequestedArtifactWriteTarget(userMessage)
	if target == "" {
		return false
	}
	evidence := collectWorkspaceArtifactEvidence(historyToolCalls, historyToolResults)
	if len(evidence) == 0 {
		return false
	}
	return true
}

func buildWorkspaceArtifactOrchestrationMessages(userMessage, target string, evidence, questions []string) []llm.Message {
	if len(questions) >= 2 {
		return buildWorkspaceQuestionArtifactOrchestrationMessages(target, evidence, questions)
	}

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
	sb.WriteString("\nWrite only the exact file content that should be saved to the target path. Do not add code fences, preambles, or commentary outside the artifact body. Follow the user's requested structure and formatting. Use only the recovered evidence. Preserve concrete names, numbers, dates, filenames, API labels, and other source wording when the evidence states them directly. If the evidence contains updates or conflicting statements, make that clear instead of silently flattening them.")

	return []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: "You are completing a local workspace artifact after tool recovery. The necessary evidence has already been extracted from local tools. Do not ask for more tools or more input. Write the final artifact directly from the recovered evidence and the user's request.",
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
	sb.WriteString("\nWrite only the final file content. Answer each question strictly from the provided evidence, keep the answers in question order, and preserve the source wording when it directly states the answer. If a fact is distributed across multiple snippets, reconcile those snippets carefully before answering. Do not add numbering, commentary, citations, or extra lines.")

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

func hasSatisfiedRequestedArtifactWrite(userMessage string, toolCalls []llm.ToolCall, toolResults []llm.Message) bool {
	if !hasRequestedArtifactWriteSuccess(userMessage, toolCalls, toolResults) {
		return false
	}
	if strings.TrimSpace(extractRequestedArtifactWriteTarget(userMessage)) == "" {
		return true
	}
	return hasAcceptableStructuredWorkspaceArtifactWrite(userMessage, toolCalls, toolResults)
}

func shouldRepairSuccessfulStructuredWorkspaceArtifactWrite(userMessage string, currentToolCalls []llm.ToolCall, currentToolResults []llm.Message, historyToolCalls []llm.ToolCall, historyToolResults []llm.Message) bool {
	_ = userMessage
	_ = currentToolCalls
	_ = currentToolResults
	_ = historyToolCalls
	_ = historyToolResults
	return false
}

func extractWorkspaceWrittenContent(tc llm.ToolCall) (string, bool) {
	toolName := normalizeFileToolCompatName(tc.Name)
	if toolName != "write" && toolName != "write_commit" && toolName != "write_begin" && toolName != "write_chunk" {
		return "", false
	}
	normalizedArgs := normalizeToolCallArgumentsForExecution(tc.Arguments)
	if strings.TrimSpace(normalizedArgs) == "" {
		return "", false
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(normalizedArgs), &payload) != nil {
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
	return true
}

func workspaceArtifactOrchestrationMaxTokens(userMessage, target string) int {
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
		var tc llm.ToolCall
		if i < len(toolCalls) && toolCalls[i].ID == tr.ToolCallID {
			tc = toolCalls[i]
			toolName = tc.Name
		} else if matched, ok := callByID[strings.TrimSpace(tr.ToolCallID)]; ok {
			tc = matched
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
		case "bash":
			if block, ok := extractWorkspaceExecReadEvidenceBlock(tc, payload); ok {
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

func extractWorkspaceExecReadEvidenceBlock(tc llm.ToolCall, payload map[string]interface{}) (workspaceArtifactEvidenceBlock, bool) {
	path, stdout, ok := extractWorkspaceExecReadEvidence(tc, payload)
	if !ok {
		return workspaceArtifactEvidenceBlock{}, false
	}
	return buildWorkspaceContentEvidenceBlock("read", path, "", "TEXT", stdout, 3), true
}

func workspaceArtifactExecReadShowsContent(tc llm.ToolCall, content string) bool {
	payload := parseWorkspaceArtifactResultPayload(content)
	_, _, ok := extractWorkspaceExecReadEvidence(tc, payload)
	return ok
}

func extractWorkspaceExecReadEvidence(tc llm.ToolCall, payload map[string]interface{}) (string, string, bool) {
	if len(payload) == 0 || classifyToolFallbackOutcome(payload) == "failed" {
		return "", "", false
	}
	if rawExit, ok := payload["exit_code"]; ok && anyToIntForLLM(rawExit) != 0 {
		return "", "", false
	}
	stdout := strings.TrimSpace(payloadStringField(payload, "stdout"))
	if stdout == "" {
		return "", "", false
	}
	path, ok := extractWorkspaceExecReadPathFromToolCall(tc, payload)
	if !ok {
		return "", "", false
	}
	return path, stdout, true
}

func extractWorkspaceExecReadPathFromToolCall(tc llm.ToolCall, payload map[string]interface{}) (string, bool) {
	command := strings.TrimSpace(extractWorkspaceExecCommand(tc))
	if command == "" {
		command = strings.TrimSpace(payloadStringField(payload, "command"))
	}
	if command == "" {
		return "", false
	}
	return extractWorkspaceExecReadPathFromCommand(command)
}

func extractWorkspaceExecReadPathFromArgs(rawArgs string) (string, bool) {
	command := strings.TrimSpace(extractWorkspaceExecCommandFromRawArgs(rawArgs))
	if command == "" {
		return "", false
	}
	return extractWorkspaceExecReadPathFromCommand(command)
}

func extractWorkspaceExecCommand(tc llm.ToolCall) string {
	return extractWorkspaceExecCommandFromRawArgs(tc.Arguments)
}

func extractWorkspaceExecCommandFromRawArgs(rawArgs string) string {
	normalizedArgs := normalizeToolCallArgumentsForExecution(rawArgs)
	if strings.TrimSpace(normalizedArgs) == "" {
		return ""
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(normalizedArgs), &payload) != nil || len(payload) == 0 {
		return ""
	}
	command := strings.TrimSpace(payloadStringField(payload, "command"))
	if command == "" {
		command = strings.TrimSpace(payloadStringField(payload, "cmd"))
	}
	return command
}

func extractWorkspaceExecReadPathFromCommand(command string) (string, bool) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", false
	}
	analysis := tools.AnalyzeCommand(command, "")
	if analysis == nil || !analysis.OK || len(analysis.Segments) != 1 {
		return "", false
	}
	segment := analysis.Segments[0]
	if len(segment.Argv) == 0 {
		return "", false
	}
	execName := strings.ToLower(strings.TrimSpace(segment.ExecutableName))
	if execName == "" {
		execName = strings.ToLower(strings.TrimSpace(filepath.Base(segment.Argv[0])))
	}

	var path string
	var ok bool
	switch execName {
	case "cat":
		path, ok = extractWorkspaceExecCatReadPath(segment.Argv)
	case "head", "tail":
		path, ok = extractWorkspaceExecHeadTailReadPath(segment.Argv)
	case "sed":
		path, ok = extractWorkspaceExecSedReadPath(segment.Argv)
	default:
		return "", false
	}
	if !ok {
		return "", false
	}
	path = normalizeWorkspaceArtifactComparablePath(path)
	if !isLikelyWorkspaceExecReadPath(path) {
		return "", false
	}
	return path, true
}

func extractWorkspaceExecCatReadPath(argv []string) (string, bool) {
	if len(argv) < 2 {
		return "", false
	}
	candidates := make([]string, 0, 1)
	for _, raw := range argv[1:] {
		arg := strings.TrimSpace(raw)
		if arg == "" || arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return "", false
		}
		candidates = append(candidates, arg)
	}
	if len(candidates) != 1 {
		return "", false
	}
	return candidates[0], true
}

func extractWorkspaceExecHeadTailReadPath(argv []string) (string, bool) {
	if len(argv) < 2 {
		return "", false
	}
	args := argv[1:]
	candidates := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		switch {
		case arg == "--":
			rest := make([]string, 0, len(args[i+1:]))
			for _, tailArg := range args[i+1:] {
				tailArg = strings.TrimSpace(tailArg)
				if tailArg != "" {
					rest = append(rest, tailArg)
				}
			}
			if len(rest) != 1 {
				return "", false
			}
			return rest[0], true
		case arg == "-n" || arg == "-c" || arg == "--lines" || arg == "--bytes":
			i++
			continue
		case strings.HasPrefix(arg, "-n") || strings.HasPrefix(arg, "-c") || strings.HasPrefix(arg, "--lines=") || strings.HasPrefix(arg, "--bytes="):
			continue
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			candidates = append(candidates, arg)
		}
	}
	if len(candidates) != 1 {
		return "", false
	}
	return candidates[0], true
}

func extractWorkspaceExecSedReadPath(argv []string) (string, bool) {
	if len(argv) < 3 {
		return "", false
	}
	path := strings.TrimSpace(argv[len(argv)-1])
	if path == "" || path == "--" || strings.HasPrefix(path, "-") {
		return "", false
	}
	for _, raw := range argv[1 : len(argv)-1] {
		if strings.TrimSpace(raw) == "--" {
			return "", false
		}
	}
	return path, true
}

func isLikelyWorkspaceExecReadPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || path == "." || path == ".." || path == "-" {
		return false
	}
	if strings.ContainsAny(path, "*?[]") {
		return false
	}
	for _, prefix := range []string{"/dev/", "/proc/", "/sys/"} {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	return true
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
	markdown := strings.TrimSpace(payloadStringField(payload, "markdown"))
	outlineText := extractWorkspacePDFOutlineText(payload["outline"])
	path := workspaceArtifactPayloadPath(payload)
	pageLabel := workspaceArtifactPayloadPageLabel(payload)
	blocks := make([]workspaceArtifactEvidenceBlock, 0, 4)
	isPDFEvidence := strings.HasSuffix(strings.ToLower(strings.TrimSpace(path)), ".pdf") || rawText != "" || markdown != "" || outlineText != ""

	pageBlocks := extractWorkspacePDFPageBlocksFromPayload(toolName, path, payload["pages"])
	if len(pageBlocks) == 0 && markdown != "" {
		pageBlocks = append(pageBlocks, extractWorkspacePDFPageEvidenceBlocks(toolName, path, "MARKDOWN", markdown)...)
	}
	if len(pageBlocks) > 0 {
		isPDFEvidence = true
		blocks = append(blocks, pageBlocks...)
	}
	if outlineText != "" && isPDFEvidence {
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, "OUTLINE", outlineText, 7))
	}
	if text == "" && rawText == "" && markdown == "" && outlineText == "" && len(blocks) == 0 {
		return nil
	}

	// Rich PDF payloads often include the same content three times:
	// per-page blocks, whole-document markdown/text, and whole-document raw_text.
	// Prefer the per-page representation so later pages are not crowded out.
	useWholeDocumentBlocks := !isPDFEvidence || len(pageBlocks) == 0

	if markdown != "" && useWholeDocumentBlocks {
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, "MARKDOWN", markdown, 5))
	}
	if outlineText != "" && useWholeDocumentBlocks && !isPDFEvidence {
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, "OUTLINE", outlineText, 2))
	}
	if text != "" && useWholeDocumentBlocks && markdown == "" {
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, "TEXT", text, 3))
	}
	if rawText != "" && rawText != text && rawText != markdown && isPDFEvidence && useWholeDocumentBlocks {
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, "RAW", rawText, 4))
	}
	if isPDFEvidence && len(pageBlocks) == 0 {
		if markdown != "" {
			blocks = append(blocks, extractWorkspacePDFPageEvidenceBlocks(toolName, path, "MARKDOWN", markdown)...)
		} else {
			blocks = append(blocks, extractWorkspacePDFPageEvidenceBlocks(toolName, path, "TEXT", text)...)
		}
		if rawText != "" {
			blocks = append(blocks, extractWorkspacePDFPageEvidenceBlocks(toolName, path, "RAW", rawText)...)
		}
	}

	return blocks
}

func extractWorkspacePDFPageBlocksFromPayload(toolName, path string, raw interface{}) []workspaceArtifactEvidenceBlock {
	rows, ok := raw.([]interface{})
	if !ok || len(rows) == 0 {
		return nil
	}
	blocks := make([]workspaceArtifactEvidenceBlock, 0, len(rows))
	for _, item := range rows {
		page, ok := item.(map[string]interface{})
		if !ok || len(page) == 0 {
			continue
		}
		text := strings.TrimSpace(payloadStringField(page, "markdown"))
		variant := "MARKDOWN"
		if text == "" {
			text = buildWorkspaceStructuredPDFPageLayoutText(page["blocks"], page["tables"])
			variant = "STRUCTURED"
		}
		if text == "" {
			text = strings.TrimSpace(payloadStringField(page, "raw_text"))
			variant = "RAW"
		}
		if text == "" {
			text = strings.TrimSpace(payloadStringField(page, "text"))
			variant = "PAGE"
		}
		if text == "" {
			continue
		}
		pageLabel := ""
		if number := anyToIntForLLM(page["number"]); number > 0 {
			pageLabel = fmt.Sprintf("page=%d", number)
			if !strings.HasPrefix(text, fmt.Sprintf("[Page %d]", number)) {
				text = fmt.Sprintf("[Page %d]\n%s", number, text)
			}
		}
		score := 5
		if variant == "STRUCTURED" || variant == "MARKDOWN" {
			score = 6
		}
		blocks = append(blocks, buildWorkspaceContentEvidenceBlock(toolName, path, pageLabel, variant, text, score))
	}
	return blocks
}

func extractWorkspacePDFOutlineText(raw interface{}) string {
	entries := normalizeCompactPDFOutlineEntriesForLLM(raw)
	if len(entries) == 0 {
		return ""
	}
	lines := make([]string, 0, min(len(entries), 16))
	for _, entry := range entries {
		prefix := "- "
		if entry.Level > 1 {
			prefix = strings.Repeat("  ", entry.Level-1) + "- "
		}
		line := prefix + entry.Title
		meta := make([]string, 0, 2)
		if entry.ChildCount > 0 {
			meta = append(meta, fmt.Sprintf("%d child sections", entry.ChildCount))
		}
		if entry.PageNumber > 0 {
			meta = append(meta, fmt.Sprintf("page %d", entry.PageNumber))
		}
		if len(meta) > 0 {
			line += " (" + strings.Join(meta, ", ") + ")"
		}
		lines = append(lines, line)
		if len(lines) >= 16 {
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func buildWorkspaceStructuredPDFPageLayoutText(blockRaw, tableRaw interface{}) string {
	parts := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	appendPart := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		key := text
		if len(key) > 256 {
			key = key[:256]
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		parts = append(parts, text)
	}

	if rows, ok := blockRaw.([]interface{}); ok {
		for _, item := range rows {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			text := strings.TrimSpace(anyToStringForLLM(block["markdown"]))
			if text == "" {
				text = strings.TrimSpace(anyToStringForLLM(block["text"]))
			}
			appendPart(text)
			if len(parts) >= 8 {
				break
			}
		}
	}
	if rows, ok := tableRaw.([]interface{}); ok {
		for _, item := range rows {
			table, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			text := strings.TrimSpace(anyToStringForLLM(table["markdown"]))
			if text == "" {
				text = buildWorkspaceStructuredPDFTableText(table["rows"])
			}
			appendPart(text)
			if len(parts) >= 8 {
				break
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func buildWorkspaceStructuredPDFTableText(raw interface{}) string {
	rows, ok := raw.([]interface{})
	if !ok || len(rows) == 0 {
		return ""
	}
	lines := make([]string, 0, min(len(rows), 6))
	for _, item := range rows {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cells, ok := row["cells"].([]interface{})
		if !ok || len(cells) == 0 {
			continue
		}
		values := make([]string, 0, len(cells))
		for _, cellRaw := range cells {
			cell, ok := cellRaw.(map[string]interface{})
			if !ok {
				continue
			}
			text := strings.TrimSpace(anyToStringForLLM(cell["text"]))
			if text == "" {
				continue
			}
			values = append(values, text)
		}
		if len(values) == 0 {
			continue
		}
		lines = append(lines, strings.Join(values, "\t"))
		if len(lines) >= 6 {
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
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
		rankedByQuestion = append(rankedByQuestion, rankWorkspaceQuestionEvidenceSnippets(question, evidence, false))
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

func rankWorkspaceQuestionEvidenceSnippets(question string, evidence []string, includeZero bool) []rankedQuestionSnippet {
	if strings.TrimSpace(question) == "" || len(evidence) == 0 {
		return nil
	}
	snippets := splitWorkspaceEvidenceSnippets(evidence)
	if len(snippets) == 0 {
		return nil
	}

	ranked := make([]rankedQuestionSnippet, 0, len(snippets))
	for _, snippet := range snippets {
		score := scoreWorkspaceQuestionEvidence(question, snippet)
		if score <= 0 && !includeZero {
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
	return ranked
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
	question = strings.TrimSpace(question)
	snippet = strings.TrimSpace(snippet)
	if question == "" || snippet == "" {
		return 0
	}

	tokens := workspaceQuestionTokens(question)
	textTokens := workspaceQuestionTokens(snippet)
	if len(tokens) == 0 || len(textTokens) == 0 {
		return 0
	}
	matched := countWorkspaceSharedTokens(tokens, textTokens)
	if matched == 0 {
		return 0
	}
	score := scoreWorkspaceTokenOverlap(tokens, textTokens)
	score += scoreWorkspacePhraseOverlap(tokens, textTokens)
	switch coverage := matched * 100 / len(tokens); {
	case coverage >= 75:
		score += 8
	case coverage >= 50:
		score += 5
	case coverage >= 25:
		score += 2
	}
	if matched >= 3 {
		score += matched
	}
	if len(textTokens) <= len(tokens)+4 {
		score += 2
	}
	return score
}

func workspaceQuestionTokens(text string) []string {
	matches := workspaceWordTokenRegex.FindAllString(strings.ToLower(text), -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, token := range matches {
		token = normalizeWorkspaceComparableToken(token)
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

func normalizeWorkspaceComparableToken(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	token = strings.Trim(token, "`\"'()[]{}.,:;!?")
	if token == "" {
		return ""
	}
	switch {
	case strings.HasSuffix(token, "ies") && len(token) > 4:
		token = token[:len(token)-3] + "y"
	case strings.HasSuffix(token, "ing") && len(token) > 5:
		token = token[:len(token)-3]
	case strings.HasSuffix(token, "ed") && len(token) > 4:
		token = token[:len(token)-2]
	case strings.HasSuffix(token, "es") && len(token) > 4:
		token = token[:len(token)-2]
	case strings.HasSuffix(token, "s") && len(token) > 3:
		token = token[:len(token)-1]
	}
	if strings.HasSuffix(token, "e") && len(token) > 3 {
		token = token[:len(token)-1]
	}
	return token
}

func scoreWorkspaceTokenOverlap(questionTokens, textTokens []string) int {
	if len(questionTokens) == 0 || len(textTokens) == 0 {
		return 0
	}
	textSet := workspaceTokenSet(textTokens)
	score := 0
	for _, token := range questionTokens {
		if _, ok := textSet[token]; ok {
			if len(token) >= 6 {
				score += 4
			} else {
				score += 2
			}
		}
	}
	return score
}

func countWorkspaceSharedTokens(questionTokens, textTokens []string) int {
	if len(questionTokens) == 0 || len(textTokens) == 0 {
		return 0
	}
	textSet := workspaceTokenSet(textTokens)
	count := 0
	for _, token := range questionTokens {
		if _, ok := textSet[token]; ok {
			count++
		}
	}
	return count
}

func scoreWorkspacePhraseOverlap(questionTokens, textTokens []string) int {
	if len(questionTokens) < 2 || len(textTokens) < 2 {
		return 0
	}
	score := 0
	textBigrams := make(map[string]struct{}, max(0, len(textTokens)-1))
	for i := 0; i < len(textTokens)-1; i++ {
		textBigrams[textTokens[i]+" "+textTokens[i+1]] = struct{}{}
	}
	for i := 0; i < len(questionTokens)-1; i++ {
		if _, ok := textBigrams[questionTokens[i]+" "+questionTokens[i+1]]; ok {
			score += 4
		}
	}
	if len(questionTokens) < 3 || len(textTokens) < 3 {
		return score
	}
	textTrigrams := make(map[string]struct{}, max(0, len(textTokens)-2))
	for i := 0; i < len(textTokens)-2; i++ {
		textTrigrams[textTokens[i]+" "+textTokens[i+1]+" "+textTokens[i+2]] = struct{}{}
	}
	for i := 0; i < len(questionTokens)-2; i++ {
		if _, ok := textTrigrams[questionTokens[i]+" "+questionTokens[i+1]+" "+questionTokens[i+2]]; ok {
			score += 6
		}
	}
	return score
}

func workspaceTokenSet(tokens []string) map[string]struct{} {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		out[token] = struct{}{}
	}
	return out
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

func shouldReplaceSavedWorkspaceArtifactFallbackReply(ctx context.Context, userMessage, currentContent string) bool {
	if !shouldPreferWorkspaceArtifactWorkflow(userMessage) {
		return false
	}
	if !hasSavedWorkspaceArtifactOnDisk(ctx, userMessage) {
		return false
	}
	return isToolFallbackRetrySummary(currentContent)
}

func isToolFallbackRetrySummary(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return false
	}
	for _, marker := range []string{
		"here is a concise fallback summary based on completed tool results",
		"here is a concise summary based on the completed tool results so far",
		"based on the completed tool results, here are the key takeaways",
		"completed tools:",
		"ask me to retry summarizing for a fuller report",
		"if you'd like, i can expand this into a fuller report",
		"我先根据已完成的工具结果，给你一个简要汇总",
		"我先根据已完成的工具结果，整理出一版简要摘要",
		"根据已完成的工具结果，整理如下",
		"如需，我可以继续补一版更完整的总结",
		"如果你愿意，我可以继续把这份结果扩展成更完整的总结",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
