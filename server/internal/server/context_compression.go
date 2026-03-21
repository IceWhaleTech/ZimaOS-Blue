package server

import (
	"context"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

const (
	historicalContextBackgroundPrefix = "历史背景，仅供参考；若与最新用户消息冲突，以最新用户消息为准。"
	historicalCarryOverReminder       = "历史未完成事项统一标记为 [carry-over]；除非用户明确要求继续，否则不要执行。"
	pausedCarryOverAssistantContent   = "(paused previous carry-over so the latest user message can take priority)"
	contextCompressSummaryMaxRunes    = 1500
	contextCompressTranscriptMaxRunes = 3400
)

var (
	reLatestIntentResume        = regexp.MustCompile(`(?i)(?:\bcontinue(?: that| it| previous| from where we left off)?\b|\bresume\b|\bpick up where we left off\b|\bcarry on\b|继续上次|继续刚才|按刚才接着做|接着上次|继续那个任务|继续那个|继续这个任务|接着做)`)
	reLatestIntentOverride      = regexp.MustCompile(`(?i)(?:\bdon'?t continue\b|\bdo not continue\b|\bstop\b|\binstead\b|\banswer this first\b|\bnow first\b|\bhold off\b|\bpause that\b|不要继续|先别|改成|现在先|先回答|先解释|先不要)`)
	reLatestIntentQuestion      = regexp.MustCompile(`(?i)(?:\?|？|\bwhat\b|\bwhy\b|\bhow\b|\bwhich\b|\bcan you explain\b|\bexplain\b|\brisk\b|\breason\b|\b原因\b|\b解释\b|\b风险\b|\b为什么\b|\b怎么\b|\b是否\b|\b先回答\b|\b先解释\b)`)
	reLatestIntentInvestigate   = regexp.MustCompile(`(?i)(?:\bcheck\b|\binspect\b|\blook up\b|\bsearch\b|\bopen\b|\bread\b|\bfetch\b|\bverify\b|\banalyze\b|\btrace\b|\bextract\b|\bparse\b|查一下|查查|看一下|看看|打开|读取|检索|搜索|分析|排查|确认|提取|解析)`)
	reCarryOverItem             = regexp.MustCompile(`(?i)(?:\bpending\b|\btodo\b|\bnext(?: step)?\b|\bremaining\b|\bfollow[- ]?up\b|\bleft to\b|\bneed(?:s)? to\b|\bstill need(?:s)?\b|待处理|待办|后续|下一步|剩余|还需要|继续)`)
	reIdentifierHeavy           = regexp.MustCompile(`(?:/|\\|#L\d+|\b\d{4}-\d{2}-\d{2}\b|\b[A-Fa-f0-9]{7,}\b|\b[A-Za-z0-9._-]*[0-9][A-Za-z0-9._-]*\b|\b[A-Za-z0-9._-]*[._/-][A-Za-z0-9._-]*\b)`)
	reProcessOnly               = regexp.MustCompile(`(?i)^(?:i(?:'m| am)?|we(?:'re| are)?|let me|going to|will|first|next|then|checking|inspecting|looking into|starting with|我先|我会|正在|先看看|先检查|接下来|然后|先去)(?:\b|[ ,.:;])`)
	reGoalLike                  = regexp.MustCompile(`(?i)(?:\bgoal\b|\btask\b|\bobjective\b|\bimplement\b|\bfix\b|\banswer\b|\bresearch\b|\bwrite\b|\bupdate\b|目标|任务|实现|修复|回答|调研|编写|更新)`)
	reInstructionLike           = regexp.MustCompile(`(?i)(?:\bmust\b|\bshould\b|\bneed to\b|\bremember\b|\bkeep\b|\bprefer\b|\bavoid\b|\buse\b|\bdon'?t\b|\bdo not\b|必须|应该|需要|记住|保留|优先|避免|不要|先)`)
	reDiscoveryLike             = regexp.MustCompile(`(?i)(?:\bfound\b|\bconfirmed\b|\broot cause\b|\berror\b|\bfailed\b|\bbecause\b|\bdiscovered\b|\bverified\b|\bobserved\b|\bdiagnosed\b|发现|确认|根因|报错|失败|因为|验证|排查|原因)`)
	reAccomplishedLike          = regexp.MustCompile(`(?i)(?:\bfixed\b|\bupdated\b|\bwrote\b|\bcreated\b|\badded\b|\bimplemented\b|\bsaved\b|\bcompleted\b|\bresolved\b|\bpatched\b|修复|更新|编写|创建|新增|实现|保存|完成|解决|修改)`)
	reFormattingOnlyInstruction = regexp.MustCompile(`(?i)(?:\bsummary\b|\bsummar(?:y|ize)\b|\bconcise\b|\breplay\b|\bprocess notes?\b|\bexploratory\b|\bif they matter\b|不要重放|不要复述|简洁|总结|摘要|过程说明|探索性)`)
)

type latestIntentCarryoverDecision struct {
	ShouldPause          bool
	Reason               string
	LatestUser           string
	ExplicitResume       bool
	ExplicitOverride     bool
	QuestionLike         bool
	AllowsInvestigation  bool
	LowOverlap           bool
	SideEffectingTools   bool
	HasHistoricalContext bool
}

type compressionSentenceCandidate struct {
	Text    string
	Section string
	Score   int
	Index   int
}

var multilingualProcessOnlyPrefixes = []string{
	"i will ", "i'm ", "i am ", "we will ", "we're ", "we are ", "let me ", "going to ",
	"voy a ", "déjame ", "dejame ", "déjanos ", "dejanos ", "primero ", "vaig a ", "primer ",
	"deixa'm ", "deixeu-me ", "je vais ", "laisse-moi ", "laissez-moi ", "d'abord ",
	"ich werde ", "lass mich ", "wir werden ", "zuerst ", "andrò ", "andrò a ", "lascia che ",
	"sto ", "prima ", "ik ga ", "laat me ", "wij gaan ", "eerst ", "vou ", "deixa eu ", "vamos ",
	"eu vou ", "primeiro ", "jag ska ", "låt mig ", "först ", "jeg vil ", "lad mig ", "først ",
	"la meg ", "først ", "θα ", "ας ", "πρώτα ", "meg fogom ", "megnézem ", "először ",
	"pogledat ću ", "provjeravam ", "sprawdzę ", "zobaczę ", "najpierw ", "pozriem ",
	"skontrolujem ", "najprv ", "zkontroluji ", "podívám se ", "nejdřív ", "nejdriv ",
	"podivam se ", "voi ", "verific ", "mai întâi ", "mai intai ", "lasă-mă ", "lasa-ma ",
	"я ", "сначала ", "проверю ", "сейчас ", "まず", "先に", "確認します", "見てみます",
	"먼저", "우선", "확인", "살펴볼게", "我先", "我会", "正在", "先看看", "先检查", "接下来", "然后", "先去",
	"നോക്കാം", "പരിശോധിക്കാം", "ഞാൻ ", "lig dom ", "rachaidh mé ",
}

var multilingualFormattingOnlyMarkers = []string{
	"summary", "summarize", "concise", "replay", "process notes", "exploratory",
	"resumen", "resum", "resumo", "résumé", "riassunto", "zusammenfassung", "samenvatting",
	"sammanfattning", "oppsummering", "opsummering", "shrnutí", "podsumowanie", "zhrnutie",
	"sažetak", "összefoglaló", "rezumat", "achoimre", "σύνοψη", "сводка",
	"要約", "요약", "总结", "摘要", "സംഗ്രഹം",
	"conciso", "concis", "kurz", "knapp", "beknopt", "kort", "kortfattat", "rövid", "kratko", "krótko", "stručne", "简洁", "간결",
	"do not replay", "don't replay", "ne répète", "nicht wiederholen", "non ripetere", "não repita", "nao repita", "no repitas", "niet herhalen", "不 要重放", "不要重放", "不要复述", "繰り返さない", "반복하지",
}

var multilingualProcessOnlyPrefixMatcher = newUnicodeAhoMatcher(multilingualProcessOnlyPrefixes)

var multilingualFormattingOnlyMatcher = newUnicodeAhoMatcher(multilingualFormattingOnlyMarkers)

func looksLikeFormattingOnlyInstruction(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return reFormattingOnlyInstruction.MatchString(lower) || multilingualFormattingOnlyMatcher.ContainsAnyFold(lower)
}

func looksLikeProcessOnlyItem(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if multilingualProcessOnlyPrefixMatcher.HasAnyPrefixFold(lower) {
		return true
	}
	return reProcessOnly.MatchString(lower)
}

func latestUserMessageFromMemory(messages []memory.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "user") {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func latestUserMessageFromLLM(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleUser {
			return strings.TrimSpace(flattenLLMMessageForSummary(messages[i]))
		}
	}
	return ""
}

func hasHistoricalCarryoverContext(messages []llm.Message) bool {
	latestUserIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleUser {
			latestUserIndex = i
			break
		}
	}
	if latestUserIndex <= 0 {
		return false
	}

	hasPriorUser := false
	hasPriorAssistantOrTool := false
	for _, msg := range messages[:latestUserIndex] {
		switch msg.Role {
		case llm.RoleUser:
			if strings.TrimSpace(flattenLLMMessageForSummary(msg)) != "" {
				hasPriorUser = true
			}
		case llm.RoleAssistant:
			if len(msg.ToolCalls) > 0 || strings.TrimSpace(flattenLLMMessageForSummary(msg)) != "" {
				hasPriorAssistantOrTool = true
			}
		case llm.RoleTool:
			hasPriorAssistantOrTool = true
		}
		if hasPriorUser && hasPriorAssistantOrTool {
			return true
		}
	}
	return false
}

func buildCurrentIntentAnchorMessage(latestUser string) llm.Message {
	latestUser = strings.TrimSpace(latestUser)
	if latestUser == "" {
		return llm.Message{}
	}
	return llm.Message{
		Role: llm.RoleSystem,
		Content: strings.Join([]string{
			"Current-turn anchor:",
			"- Follow only the latest user message for this turn.",
			"- Keep [carry-over] paused unless the user explicitly says continue.",
			"- Latest user message: " + truncateRunes(latestUser, 320),
		}, "\n"),
	}
}

func wrapHistoricalSummaryForContext(summaryText string) string {
	summaryText = strings.TrimSpace(summaryText)
	if summaryText == "" {
		return ""
	}
	summaryText = sanitizeCompressedSummaryOutput(summaryText)
	summaryText = markCarryOverItems(summaryText)
	return strings.TrimSpace(historicalContextBackgroundPrefix + "\n" + historicalCarryOverReminder + "\n\n" + summaryText)
}

func compressedHistoryContextMessages(latestUser, summaryText string) []llm.Message {
	summaryText = strings.TrimSpace(summaryText)
	if summaryText == "" {
		return nil
	}
	out := make([]llm.Message, 0, 2)
	if anchor := buildCurrentIntentAnchorMessage(latestUser); strings.TrimSpace(anchor.Content) != "" {
		out = append(out, anchor)
	}
	if wrapped := wrapHistoricalSummaryForContext(summaryText); wrapped != "" {
		out = append(out, llm.Message{
			Role:    llm.RoleSystem,
			Content: wrapped,
		})
	}
	return out
}

func summaryMessageHasBackgroundWrapper(messages []llm.Message) bool {
	for _, msg := range messages {
		if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, historicalContextBackgroundPrefix) {
			return true
		}
	}
	return false
}

func markCarryOverItems(summaryText string) string {
	lines := strings.Split(strings.TrimSpace(summaryText), "\n")
	if len(lines) == 0 {
		return ""
	}
	currentSection := ""
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch line {
		case "Goal", "Instructions", "Discoveries", "Accomplished", "Relevant Files":
			currentSection = line
			continue
		}
		if currentSection != "Accomplished" || !strings.HasPrefix(line, "- ") || strings.Contains(line, "[carry-over]") {
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if !reCarryOverItem.MatchString(item) {
			continue
		}
		lines[i] = "- [carry-over] " + item
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func sanitizeCompressedSummaryOutput(summaryText string) string {
	lines := strings.Split(strings.TrimSpace(summaryText), "\n")
	if len(lines) == 0 {
		return ""
	}
	out := make([]string, 0, len(lines))
	currentSection := ""
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch line {
		case "Goal", "Instructions", "Discoveries", "Accomplished", "Relevant Files":
			currentSection = line
			out = append(out, line)
			continue
		}
		if !strings.HasPrefix(line, "- ") {
			out = append(out, line)
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		lower := strings.ToLower(item)
		if looksLikeProcessOnlyItem(lower) &&
			!reIdentifierHeavy.MatchString(item) &&
			!reCarryOverItem.MatchString(lower) &&
			!reDiscoveryLike.MatchString(lower) &&
			!reAccomplishedLike.MatchString(lower) {
			continue
		}
		item = rewriteHistoricalSummaryItem(currentSection, item)
		lower = strings.ToLower(item)
		if currentSection == "Accomplished" && reCarryOverItem.MatchString(lower) && !strings.Contains(item, "[carry-over]") {
			item = "[carry-over] " + item
		}
		out = append(out, "- "+item)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func rewriteHistoricalSummaryItem(section, item string) string {
	item = strings.TrimSpace(item)
	if item == "" {
		return ""
	}
	lower := strings.ToLower(item)
	switch section {
	case "Goal":
		if strings.HasPrefix(lower, "historical task:") {
			return item
		}
		return "Historical task: " + item
	case "Instructions":
		if strings.HasPrefix(lower, "historical instruction:") {
			return item
		}
		return "Historical instruction: " + item
	default:
		return item
	}
}

func (h *ChatHandler) shouldUseSmallModelContextCompress() bool {
	if h == nil || h.settingsHandler == nil || !h.isSmallModelReady() {
		return false
	}
	return h.settingsHandler.GetSmallModelEnabled() && h.settingsHandler.GetSmallModelContextCompressEnabled()
}

func (h *ChatHandler) contextCompressionMode() string {
	if h == nil || h.settingsHandler == nil {
		return "auto"
	}
	return h.settingsHandler.GetContextCompressionMode()
}

func (h *ChatHandler) buildOfflineConversationCompression(olderMessages, relevantMessages []llm.Message) string {
	if len(olderMessages) == 0 {
		return ""
	}

	latestUser := latestUserMessageFromLLM(relevantMessages)
	queryTokens := extractIntentKeywords(latestUser)
	selected := make([]compressionSentenceCandidate, 0, 24)
	seen := map[string]struct{}{}

	for idx, msg := range olderMessages {
		for _, part := range splitCompressionUnits(normalizeCompressionSourceMessage(msg)) {
			text := normalizeCompressionUnit(part)
			if text == "" {
				continue
			}
			section, score, keep := classifyCompressionUnit(msg.Role, text, idx, len(olderMessages), queryTokens)
			if !keep {
				continue
			}
			key := dedupeCompressionKey(text)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			selected = append(selected, compressionSentenceCandidate{
				Text:    text,
				Section: section,
				Score:   score,
				Index:   idx,
			})
		}
	}

	if len(selected) == 0 {
		if fallback := fallbackGoalFromMessages(olderMessages); fallback != "" {
			raw := "Goal\n- " + fallback
			return sanitizeCompressedSummaryOutput(claudecode.NormalizeStructuredSummary(raw, "", relevantMessages))
		}
		return ""
	}

	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Score == selected[j].Score {
			return selected[i].Index > selected[j].Index
		}
		return selected[i].Score > selected[j].Score
	})

	perSectionLimit := map[string]int{
		"Goal":         3,
		"Instructions": 4,
		"Discoveries":  6,
		"Accomplished": 5,
	}
	sections := map[string][]string{
		"Goal":         {},
		"Instructions": {},
		"Discoveries":  {},
		"Accomplished": {},
	}
	totalRunes := 0
	for _, candidate := range selected {
		items := sections[candidate.Section]
		if len(items) >= perSectionLimit[candidate.Section] {
			continue
		}
		text := truncateRunes(candidate.Text, 220)
		size := len([]rune(text))
		if totalRunes+size > contextCompressSummaryMaxRunes && len(sections["Goal"])+len(sections["Instructions"])+len(sections["Discoveries"])+len(sections["Accomplished"]) > 0 {
			continue
		}
		sections[candidate.Section] = append(sections[candidate.Section], text)
		totalRunes += size
	}

	if len(sections["Goal"]) == 0 {
		if fallback := fallbackGoalFromMessages(olderMessages); fallback != "" {
			sections["Goal"] = append(sections["Goal"], fallback)
		}
	}
	if len(sections["Instructions"]) == 0 {
		if fallback := fallbackInstructionsFromMessages(olderMessages); fallback != "" {
			sections["Instructions"] = append(sections["Instructions"], fallback)
		}
	}
	dedupeCompressionSectionsInPlace(sections, []string{"Goal", "Instructions", "Discoveries", "Accomplished"})

	var blocks []string
	for _, section := range []string{"Goal", "Instructions", "Discoveries", "Accomplished"} {
		if len(sections[section]) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString(section)
		for _, item := range sections[section] {
			if strings.TrimSpace(item) == "" {
				continue
			}
			b.WriteString("\n- ")
			b.WriteString(item)
		}
		blocks = append(blocks, b.String())
	}
	if len(blocks) == 0 {
		return ""
	}
	return sanitizeCompressedSummaryOutput(claudecode.NormalizeStructuredSummary(strings.Join(blocks, "\n\n"), "", relevantMessages))
}

func (h *ChatHandler) buildSmallModelConversationCompression(ctx context.Context, olderMessages, relevantMessages []llm.Message) string {
	if !h.shouldUseSmallModelContextCompress() || len(olderMessages) == 0 {
		return ""
	}
	if ctx == nil {
		ctx = context.Background()
	}

	latestUser := latestUserMessageFromLLM(relevantMessages)
	transcript := buildCompressionTranscript(olderMessages, contextCompressTranscriptMaxRunes)
	if transcript == "" {
		return ""
	}

	prefix := claudecode.StructuredSummaryInstructions(summaryCustomFocus) +
		"\nAdditional rules:" +
		"\n- This is historical context compression, not current-turn intent arbitration." +
		"\n- Treat the latest user message only as a relevance hint for what historical facts still matter." +
		"\n- Mark unfinished historical items with '[carry-over]' and keep them paused." +
		"\n- Omit process chatter like 'I will inspect' unless it changed the outcome." +
		"\nOutput ONLY the summary text.\n\nLatest user message (relevance hint only):\n"
	suffix := truncateRunes(latestUser, 320) + "\n\nHistorical conversation snippets:\n" + transcript + "\nSummary:"

	smCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	h.smallModelStats.RecordContextCompressAttempt()
	started := time.Now()
	resp, err := h.generateWithSmallModelPrefixReuse(smCtx, "smallmodel:context_compress:v1", prefix, suffix, 360, 0.2)
	h.smallModelStats.RecordLatencyWithScene("context_compress", time.Since(started))
	if err != nil {
		h.smallModelStats.RecordFallback(smallModelFallbackReason(err))
		return ""
	}
	if resp == nil {
		h.smallModelStats.RecordFallback(smallmodel.FallbackReasonResourceGuard)
		return ""
	}
	summary := strings.TrimSpace(resp.Text)
	if summary == "" {
		h.smallModelStats.RecordFallback(smallmodel.FallbackReasonLowConfidence)
		return ""
	}
	if strings.HasPrefix(strings.ToLower(summary), "summary:") {
		summary = strings.TrimSpace(summary[len("summary:"):])
	}
	summary = claudecode.NormalizeStructuredSummary(summary, "", relevantMessages)
	summary = sanitizeCompressedSummaryOutput(summary)
	if summary == "" {
		h.smallModelStats.RecordFallback(smallmodel.FallbackReasonLowConfidence)
		return ""
	}
	h.smallModelStats.RecordContextCompressSuccess()
	return summary
}

func latestIntentVsCarryover(messages []llm.Message, latestUser string, toolCalls []llm.ToolCall) latestIntentCarryoverDecision {
	latestUser = strings.TrimSpace(latestUser)
	if latestUser == "" {
		latestUser = latestUserMessageFromLLM(messages)
	}
	decision := latestIntentCarryoverDecision{
		LatestUser:           latestUser,
		ExplicitResume:       reLatestIntentResume.MatchString(latestUser),
		ExplicitOverride:     reLatestIntentOverride.MatchString(latestUser),
		QuestionLike:         reLatestIntentQuestion.MatchString(latestUser),
		AllowsInvestigation:  reLatestIntentInvestigate.MatchString(latestUser),
		SideEffectingTools:   toolCallsContainSideEffects(toolCalls),
		HasHistoricalContext: hasHistoricalCarryoverContext(messages),
	}
	if decision.ExplicitResume || latestUser == "" || len(toolCalls) == 0 {
		return decision
	}

	decision.LowOverlap = intentTokenOverlapLow(latestUser, flattenToolCallsForIntent(toolCalls))
	explicitPathCueMatch := toolCallsMatchLatestIntentExplicitPath(latestUser, toolCalls)
	switch {
	case decision.ExplicitOverride && decision.SideEffectingTools && !explicitPathCueMatch:
		decision.ShouldPause = true
		decision.Reason = "explicit_override"
	case decision.QuestionLike && decision.SideEffectingTools && !explicitPathCueMatch && (summaryMessageHasBackgroundWrapper(messages) || decision.HasHistoricalContext):
		decision.ShouldPause = true
		decision.Reason = "question_before_side_effect"
	case decision.SideEffectingTools && !explicitPathCueMatch && (summaryMessageHasBackgroundWrapper(messages) || (decision.HasHistoricalContext && decision.LowOverlap)):
		decision.ShouldPause = true
		if decision.LowOverlap {
			decision.Reason = "low_overlap_side_effect"
		} else {
			decision.Reason = "compressed_carry_over_side_effect"
		}
	case decision.QuestionLike && !decision.AllowsInvestigation && decision.HasHistoricalContext && decision.LowOverlap:
		decision.ShouldPause = true
		decision.Reason = "question_low_overlap"
	}
	return decision
}

func toolCallsMatchLatestIntentExplicitPath(latestUser string, toolCalls []llm.ToolCall) bool {
	if strings.TrimSpace(latestUser) == "" || len(toolCalls) == 0 {
		return false
	}

	toolText := strings.ToLower(flattenToolCallsForIntent(toolCalls))
	if strings.TrimSpace(toolText) == "" {
		return false
	}

	cues := explicitPathCuesFromLatestIntent(latestUser)
	for _, cue := range cues {
		cue = strings.ToLower(filepath.ToSlash(strings.TrimSpace(cue)))
		if cue == "" {
			continue
		}
		if strings.Contains(toolText, cue) {
			return true
		}
		base := strings.ToLower(strings.TrimSpace(filepath.Base(cue)))
		if base != "" && base != "." && base != "/" && strings.Contains(toolText, base) {
			return true
		}
	}
	return false
}

func explicitPathCuesFromLatestIntent(latestUser string) []string {
	matches := requestedArtifactPathRegex.FindAllStringSubmatch(latestUser, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches)*2)
	out := make([]string, 0, len(matches)*2)
	for _, match := range matches {
		for _, group := range match[1:] {
			candidate := strings.TrimSpace(group)
			if candidate == "" {
				continue
			}
			candidate = filepath.ToSlash(candidate)
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			out = append(out, candidate)
		}
	}
	return out
}

func buildLatestIntentPauseNudge(latestUser string) string {
	latestUser = strings.TrimSpace(latestUser)
	if latestUser == "" {
		return "Pause any [carry-over] work from earlier context. The latest user message takes priority. Answer the latest request directly unless the user explicitly asked to continue previous work."
	}
	return "Pause any [carry-over] work from earlier context unless the user explicitly asked to continue it. The latest user message takes priority. Answer this latest request directly unless a tool is necessary for this latest request itself:\n" + truncateRunes(latestUser, 320)
}

func buildLatestIntentPauseFallbackReply(latestUser string) string {
	latestUser = strings.TrimSpace(latestUser)
	if latestUser == "" {
		return "I paused the previous unfinished task because the latest user message takes priority. If you want me to continue the earlier task, say so explicitly."
	}
	return "I paused the previous unfinished task because your latest message changes the current priority. If you want me to continue the earlier task, say \"continue that\" explicitly."
}

func normalizeCompressionSourceMessage(msg llm.Message) string {
	text := strings.TrimSpace(flattenLLMMessageForSummary(msg))
	if text == "" {
		return ""
	}
	if msg.Role == llm.RoleTool {
		text = compactToolResultContentForLLM(msg.ToolName, text)
	}
	runes := []rune(text)
	if len(runes) > 1800 {
		text = string(runes[:900]) + "\n...\n" + string(runes[len(runes)-900:])
	}
	return text
}

func splitCompressionUnits(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)*2)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len([]rune(line)) <= 220 {
			out = append(out, line)
			continue
		}
		var current strings.Builder
		for _, r := range line {
			current.WriteRune(r)
			switch r {
			case '.', '?', '!', ';', '。', '？', '！', '；':
				unit := strings.TrimSpace(current.String())
				if unit != "" {
					out = append(out, unit)
				}
				current.Reset()
			}
		}
		if tail := strings.TrimSpace(current.String()); tail != "" {
			out = append(out, tail)
		}
	}
	return out
}

func dedupeCompressionUnits(units []string) []string {
	if len(units) == 0 {
		return nil
	}
	out := make([]string, 0, len(units))
	seen := make(map[string]struct{}, len(units))
	for _, unit := range units {
		unit = normalizeCompressionUnit(unit)
		if unit == "" {
			continue
		}
		key := dedupeCompressionKey(unit)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, unit)
	}
	return out
}

func normalizeCompressionUnit(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	text = strings.Trim(text, "-*•")
	text = strings.TrimSpace(text)
	return text
}

func dedupeCompressionKey(text string) string {
	text = strings.ToLower(text)
	var b strings.Builder
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func compactRepeatedCompressionText(text string) string {
	units := dedupeCompressionUnits(splitCompressionUnits(text))
	switch len(units) {
	case 0:
		return ""
	case 1:
		return units[0]
	default:
		return strings.Join(units, " ")
	}
}

func classifyCompressionUnit(role llm.Role, text string, idx, total int, queryTokens map[string]struct{}) (string, int, bool) {
	lower := strings.ToLower(text)
	if lower == "" {
		return "", 0, false
	}

	score := 4
	if total > 0 {
		score += (idx * 6) / total
	}
	if role == llm.RoleUser {
		score += 4
	}
	if role == llm.RoleSystem {
		score += 3
	}
	if reIdentifierHeavy.MatchString(text) {
		score += 5
	}
	if overlap := keywordOverlapCount(queryTokens, lower); overlap > 0 {
		score += overlap * 4
	}
	if reInstructionLike.MatchString(lower) {
		score += 4
	}
	if reGoalLike.MatchString(lower) {
		score += 3
	}
	if reDiscoveryLike.MatchString(lower) {
		score += 3
	}
	if reAccomplishedLike.MatchString(lower) || reCarryOverItem.MatchString(lower) {
		score += 4
	}
	if looksLikeProcessOnlyItem(lower) && !reIdentifierHeavy.MatchString(text) &&
		!reDiscoveryLike.MatchString(lower) && !reAccomplishedLike.MatchString(lower) &&
		!reCarryOverItem.MatchString(lower) {
		return "", 0, false
	}
	if looksLikeFormattingOnlyInstruction(lower) &&
		!reIdentifierHeavy.MatchString(text) &&
		keywordOverlapCount(queryTokens, lower) == 0 {
		score -= 6
	}
	if score < 4 {
		return "", 0, false
	}

	switch {
	case role == llm.RoleUser && (idx <= 1 || reGoalLike.MatchString(lower)) && !reInstructionLike.MatchString(lower):
		return "Goal", score, true
	case reAccomplishedLike.MatchString(lower) || reCarryOverItem.MatchString(lower):
		return "Accomplished", score, true
	case role != llm.RoleUser && reDiscoveryLike.MatchString(lower):
		return "Discoveries", score, true
	case role == llm.RoleUser || reInstructionLike.MatchString(lower):
		return "Instructions", score, true
	default:
		return "Discoveries", score, true
	}
}

func dedupeCompressionSectionsInPlace(sections map[string][]string, order []string) {
	if len(sections) == 0 || len(order) == 0 {
		return
	}
	seen := make(map[string]struct{})
	for _, section := range order {
		items := sections[section]
		if len(items) == 0 {
			continue
		}
		filtered := items[:0]
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			key := dedupeCompressionKey(item)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			filtered = append(filtered, item)
		}
		sections[section] = filtered
	}
}

func fallbackGoalFromMessages(messages []llm.Message) string {
	for _, msg := range messages {
		if msg.Role != llm.RoleUser {
			continue
		}
		text := compactRepeatedCompressionText(normalizeCompressionSourceMessage(msg))
		for _, unit := range dedupeCompressionUnits(splitCompressionUnits(text)) {
			lower := strings.ToLower(unit)
			if looksLikeProcessOnlyItem(lower) || looksLikeFormattingOnlyInstruction(lower) {
				continue
			}
			if text := truncateRunes(unit, 220); text != "" {
				return text
			}
		}
		if text := truncateRunes(text, 220); text != "" {
			return text
		}
	}
	return ""
}

func fallbackInstructionsFromMessages(messages []llm.Message) string {
	for _, msg := range messages {
		if msg.Role != llm.RoleUser && msg.Role != llm.RoleSystem {
			continue
		}
		text := compactRepeatedCompressionText(normalizeCompressionSourceMessage(msg))
		if text == "" {
			continue
		}
		for _, unit := range dedupeCompressionUnits(splitCompressionUnits(text)) {
			lower := strings.ToLower(unit)
			if !reInstructionLike.MatchString(lower) || looksLikeFormattingOnlyInstruction(lower) {
				continue
			}
			return truncateRunes(unit, 220)
		}
	}
	return ""
}

func buildCompressionTranscript(messages []llm.Message, maxRunes int) string {
	var transcript strings.Builder
	total := 0
	for _, msg := range messages {
		text := normalizeCompressionSourceMessage(msg)
		if text == "" {
			continue
		}
		role := "Context"
		switch msg.Role {
		case llm.RoleUser:
			role = "User"
		case llm.RoleAssistant:
			role = "Assistant"
		case llm.RoleSystem:
			role = "System"
		case llm.RoleTool:
			role = "Tool"
		}
		line := role + ": " + truncateRunes(strings.TrimSpace(text), 280) + "\n"
		lineRunes := len([]rune(line))
		if total+lineRunes > maxRunes {
			break
		}
		transcript.WriteString(line)
		total += lineRunes
	}
	return strings.TrimSpace(transcript.String())
}

func toolCallsContainSideEffects(toolCalls []llm.ToolCall) bool {
	for _, tc := range toolCalls {
		name := strings.ToLower(strings.TrimSpace(tc.Name))
		switch {
		case strings.Contains(name, "write"),
			strings.Contains(name, "edit"),
			strings.Contains(name, "patch"),
			strings.Contains(name, "replace"),
			strings.Contains(name, "update"),
			strings.Contains(name, "create"),
			strings.Contains(name, "exec"),
			strings.Contains(name, "browser"),
			strings.Contains(name, "click"),
			strings.Contains(name, "type"),
			strings.Contains(name, "submit"),
			strings.Contains(name, "send"),
			strings.Contains(name, "email"),
			strings.Contains(name, "calendar"),
			strings.Contains(name, "reminder"),
			strings.Contains(name, "tts"),
			strings.Contains(name, "convert"),
			strings.Contains(name, "delete"):
			return true
		}
	}
	return false
}

func flattenToolCallsForIntent(toolCalls []llm.ToolCall) string {
	var b strings.Builder
	for _, tc := range toolCalls {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.TrimSpace(tc.Name))
		if args := strings.TrimSpace(tc.Arguments); args != "" {
			b.WriteString(": ")
			b.WriteString(args)
		}
	}
	return b.String()
}

func intentTokenOverlapLow(latestUser, toolText string) bool {
	userTokens := extractIntentKeywords(latestUser)
	toolTokens := extractIntentKeywords(toolText)
	if len(userTokens) == 0 || len(toolTokens) == 0 {
		return false
	}
	return keywordOverlapCount(userTokens, toolText) == 0 || keywordOverlapCount(toolTokens, latestUser) == 0
}

func extractIntentKeywords(text string) map[string]struct{} {
	out := make(map[string]struct{})
	if strings.TrimSpace(text) == "" {
		return out
	}
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '/' || r == '.')
	})
	for _, field := range fields {
		field = strings.Trim(field, "._-/")
		if len([]rune(field)) < 2 {
			continue
		}
		switch field {
		case "the", "this", "that", "with", "from", "into", "your", "just", "then", "them", "they", "what", "why", "how", "when",
			"please", "继续", "刚才", "上次", "不要", "先", "解释", "回答":
			continue
		}
		out[field] = struct{}{}
	}
	return out
}

func keywordOverlapCount(tokens map[string]struct{}, text string) int {
	if len(tokens) == 0 || strings.TrimSpace(text) == "" {
		return 0
	}
	text = strings.ToLower(text)
	count := 0
	for token := range tokens {
		if strings.Contains(text, token) {
			count++
		}
	}
	return count
}
