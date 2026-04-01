package server

import (
	_ "embed"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

type featureIntent struct {
	DeepResearch bool
	AgentMode    bool
}

var definitionBoundaryMarkers = []string{
	"?",
	"？",
	"!",
	"！",
	"\n",
	"。",
	";",
	"；",
	" and then ",
	" then ",
	", then ",
	", please ",
	" but ",
	" however ",
	"，然后",
	"然后",
	"，请",
	"请",
}

var leadingCourtesyPrefixes = []string{
	"please ",
	"pls ",
	"请帮我",
	"请你",
	"请",
}

var wrappedLiteralClosers = map[rune]rune{
	'"':  '"',
	'\'': '\'',
	'`':  '`',
	'“':  '”',
	'‘':  '’',
	'「':  '」',
	'『':  '』',
	'《':  '》',
	'〈':  '〉',
	'‹':  '›',
}

var explicitRequestCueSuffixes = []string{
	"use",
	"enable",
	"run in",
	"switch to",
	"turn on",
	"activate",
	"start",
	"apply",
	"keep",
	"enter",
	"开启",
	"打开",
	"启用",
	"切换到",
	"进入",
	"使用",
}

var referentialContextCues = []string{
	"docs",
	"documentation",
	"help text",
	"help copy",
	"ui copy",
	"settings copy",
	"button copy",
	"tooltip",
	"label",
	"menu label",
	"button",
	"title",
	"heading",
	"command",
	"keyword",
	"term",
	"string",
	"word",
	"mention",
	"document",
	"by name",
	"rename",
	"renaming",
	"relabel",
	"relabeling",
	"reword",
	"文档",
	"帮助文案",
	"标签",
	"按钮",
	"按钮文案",
	"命令",
	"关键词",
	"术语",
	"字符串",
	"这个词",
	"这两个词",
	"写上",
	"写进",
	"改成",
	"改为",
	"重命名",
	"提到",
	"提及",
	"文案",
	"说明",
	"设置页文案",
	"标题",
	"移动",
}

//go:embed feature_intent_terms.json
var featureIntentTermsRaw []byte

type featureIntentTerms struct {
	DeepResearchExplicit []string `json:"deep_research_explicit"`
	DeepResearchActions  []string `json:"deep_research_actions"`
	DeepResearchTargets  []string `json:"deep_research_targets"`
	DeepResearchNegation []string `json:"deep_research_negations"`

	AgentModeExplicit []string `json:"agent_mode_explicit"`
	AgentModeActions  []string `json:"agent_mode_actions"`
	AgentModeTargets  []string `json:"agent_mode_targets"`
	AgentModeNegation []string `json:"agent_mode_negations"`

	DefinitionPrefixes []string `json:"definition_prefixes"`
}

var featureIntentDict = loadFeatureIntentTerms()

func classifyFeatureIntent(message string) featureIntent {
	text := resolveFeatureIntentText(strings.ToLower(strings.TrimSpace(message)))
	if text == "" {
		return featureIntent{}
	}

	deepExplicit := containsAnyExplicitFeatureTerm(text, featureIntentDict.DeepResearchExplicit)
	deepComposite := containsAnyFeatureTerm(text, featureIntentDict.DeepResearchActions) && containsAnyFeatureTerm(text, featureIntentDict.DeepResearchTargets)
	deepNegated := containsAnyFeatureTerm(text, featureIntentDict.DeepResearchNegation)
	deepResearch := (deepExplicit || deepComposite) && !deepNegated

	agentExplicit := containsAnyExplicitFeatureTerm(text, featureIntentDict.AgentModeExplicit)
	agentComposite := containsAnyFeatureTerm(text, featureIntentDict.AgentModeActions) && containsAnyFeatureTerm(text, featureIntentDict.AgentModeTargets)
	agentNegated := containsAnyFeatureTerm(text, featureIntentDict.AgentModeNegation)
	agentMode := (agentExplicit || agentComposite) && !agentNegated

	return featureIntent{
		DeepResearch: deepResearch,
		AgentMode:    agentMode,
	}
}

func hasFeatureDefinitionPrefix(text string) bool {
	normalized := stripLeadingCourtesyPrefixes(text)
	for _, prefix := range featureIntentDict.DefinitionPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func stripLeadingCourtesyPrefixes(text string) string {
	current := strings.TrimLeft(text, " \t\r\n")
	for {
		matched := ""
		for _, prefix := range leadingCourtesyPrefixes {
			if strings.HasPrefix(current, prefix) {
				matched = prefix
				break
			}
		}
		if matched == "" {
			return current
		}
		current = strings.TrimLeft(current[len(matched):], " \t\r\n")
	}
}

func findFeatureDefinitionBoundary(text string) (int, int, bool) {
	bestIndex := -1
	bestLen := 0

	for _, marker := range definitionBoundaryMarkers {
		idx := strings.Index(text, marker)
		if idx < 0 {
			continue
		}
		if bestIndex < 0 || idx < bestIndex {
			bestIndex = idx
			bestLen = len(marker)
		}
	}

	if bestIndex < 0 {
		return 0, 0, false
	}

	return bestIndex, bestLen, true
}

func extractFeatureDefinitionTail(text string) (string, bool) {
	normalized := stripLeadingCourtesyPrefixes(text)
	if !hasFeatureDefinitionPrefix(text) {
		return "", false
	}

	mentionsFeature := containsAnyFeatureTerm(normalized, featureIntentDict.DeepResearchExplicit) ||
		containsAnyFeatureTerm(normalized, featureIntentDict.AgentModeExplicit)
	if !mentionsFeature {
		return "", false
	}

	idx, markerLen, ok := findFeatureDefinitionBoundary(normalized)
	if !ok {
		return "", true
	}

	return strings.TrimSpace(normalized[idx+markerLen:]), true
}

func resolveFeatureIntentText(text string) string {
	current := strings.TrimSpace(text)
	for current != "" {
		tail, isDefinition := extractFeatureDefinitionTail(current)
		if !isDefinition {
			return current
		}
		if tail == "" {
			return ""
		}
		current = tail
	}
	return ""
}

func containsAnyFeatureTerm(text string, terms []string) bool {
	for _, term := range terms {
		if containsFeatureTerm(text, term) {
			return true
		}
	}
	return false
}

func containsAnyExplicitFeatureTerm(text string, terms []string) bool {
	for _, term := range terms {
		if containsExplicitFeatureTerm(text, term) {
			return true
		}
	}
	return false
}

func containsFeatureTerm(text, term string) bool {
	if term == "" {
		return false
	}
	if isASCIITerm(term) {
		return containsASCIIWord(text, term)
	}
	return strings.Contains(text, term)
}

func trimTrailingWrappedLiteralOpener(text string) string {
	current := strings.TrimRight(text, " \t\r\n")
	for current != "" {
		last, size := utf8.DecodeLastRuneInString(current)
		if _, ok := wrappedLiteralClosers[last]; !ok {
			return current
		}
		current = strings.TrimRight(current[:len(current)-size], " \t\r\n")
	}
	return current
}

func isWrappedLiteralOccurrence(text string, start, end int) bool {
	if start <= 0 || end >= len(text) {
		return false
	}
	before, _ := utf8.DecodeLastRuneInString(text[:start])
	after, _ := utf8.DecodeRuneInString(text[end:])
	return wrappedLiteralClosers[before] == after
}

func hasExplicitRequestCueBefore(text string, start int) bool {
	prefix := text
	if start > 48 {
		prefix = text[start-48 : start]
	} else {
		prefix = text[:start]
	}
	prefix = trimTrailingWrappedLiteralOpener(prefix)
	for _, cue := range explicitRequestCueSuffixes {
		if strings.HasSuffix(prefix, cue) {
			return true
		}
	}
	return false
}

func hasReferentialContextAround(text string, start, end int) bool {
	prefixStart := 0
	if start > 64 {
		prefixStart = start - 64
	}
	suffixEnd := len(text)
	if end+64 < suffixEnd {
		suffixEnd = end + 64
	}
	prefix := text[prefixStart:start]
	suffix := text[end:suffixEnd]
	for _, cue := range referentialContextCues {
		if strings.Contains(prefix, cue) || strings.Contains(suffix, cue) {
			return true
		}
	}
	return false
}

func containsExplicitFeatureTerm(text, term string) bool {
	if term == "" {
		return false
	}

	asciiTerm := isASCIITerm(term)
	searchFrom := 0

	for searchFrom <= len(text) {
		idx := strings.Index(text[searchFrom:], term)
		if idx < 0 {
			return false
		}
		idx += searchFrom
		end := idx + len(term)

		if asciiTerm {
			beforeOK := idx == 0 || !isASCIIWordByte(text[idx-1])
			afterOK := end >= len(text) || !isASCIIWordByte(text[end])
			if !beforeOK || !afterOK {
				searchFrom = idx + 1
				continue
			}
		}

		hasRequestCue := hasExplicitRequestCueBefore(text, idx)

		if isWrappedLiteralOccurrence(text, idx, end) && !hasRequestCue {
			searchFrom = idx + 1
			continue
		}

		if !hasRequestCue && hasReferentialContextAround(text, idx, end) {
			searchFrom = idx + 1
			continue
		}

		if !isWrappedLiteralOccurrence(text, idx, end) || hasRequestCue {
			return true
		}

		searchFrom = idx + 1
	}

	return false
}

func isASCIITerm(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

func containsASCIIWord(text, term string) bool {
	searchFrom := 0
	for searchFrom <= len(text) {
		idx := strings.Index(text[searchFrom:], term)
		if idx < 0 {
			return false
		}
		idx += searchFrom
		beforeOK := idx == 0 || !isASCIIWordByte(text[idx-1])
		end := idx + len(term)
		afterOK := end >= len(text) || !isASCIIWordByte(text[end])
		if beforeOK && afterOK {
			return true
		}
		searchFrom = idx + 1
	}
	return false
}

func isASCIIWordByte(ch byte) bool {
	return (ch >= '0' && ch <= '9') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= 'a' && ch <= 'z') ||
		ch == '_'
}

func loadFeatureIntentTerms() featureIntentTerms {
	var cfg featureIntentTerms
	if err := json.Unmarshal(featureIntentTermsRaw, &cfg); err != nil {
		return defaultFeatureIntentTerms()
	}
	if len(cfg.DeepResearchExplicit) == 0 || len(cfg.AgentModeExplicit) == 0 {
		return defaultFeatureIntentTerms()
	}
	return cfg
}

func defaultFeatureIntentTerms() featureIntentTerms {
	return featureIntentTerms{
		DeepResearchExplicit: []string{
			"deep research",
			"research mode",
			"research workflow",
			"research capability",
			"harness research",
			"深度搜索",
			"深度研究",
			"深入研究",
			"深度调研",
			"深入调研",
			"研究模式",
			"研究流程",
			"研究能力",
			"人物调研",
			"查阅观点",
			"按时期调研",
		},
		DeepResearchActions: []string{
			"deep dive",
			"in-depth",
			"in depth",
			"comprehensive research",
			"research thoroughly",
			"详细调研",
			"全面调研",
			"深入分析",
			"全面分析",
			"逐一调研",
			"查阅",
			"梳理",
		},
		DeepResearchTargets: []string{
			"资料",
			"来源",
			"引用",
			"证据",
			"sources",
			"citations",
			"evidence",
			"references",
			"观点",
			"文章",
			"不同时期",
		},
		DeepResearchNegation: []string{
			"no deep research",
			"no research mode",
			"without deep research",
			"without research mode",
			"disable deep research",
			"disable research mode",
			"disable research",
			"不要深度搜索",
			"不用深度搜索",
			"关闭深度搜索",
			"不要研究模式",
			"不用研究模式",
			"关闭研究模式",
		},
		AgentModeExplicit: []string{
			"agent mode",
			"agent loop",
			"agent loop mode",
			"agent mode loop",
			"智能体模式",
			"智能体循环",
			"循环智能体",
			"代理模式",
			"自动代理",
			"自主代理",
		},
		AgentModeActions: []string{
			"autonomous",
			"plan and execute",
			"multi-step",
			"自动执行",
			"自主执行",
			"自己完成",
			"自动完成",
			"分步执行",
			"端到端执行",
		},
		AgentModeTargets: []string{
			"task",
			"tasks",
			"workflow",
			"步骤",
			"任务",
			"流程",
			"命令",
		},
		AgentModeNegation: []string{
			"no agent mode",
			"no agent loop",
			"without agent mode",
			"without agent loop",
			"disable agent mode",
			"disable agent loop",
			"不要 agent mode",
			"不要 agent loop",
			"不要智能体模式",
			"不要智能体循环",
			"关闭agent loop",
			"关闭agent mode",
			"关闭智能体模式",
			"不要自动执行",
		},
		DefinitionPrefixes: []string{
			"what is",
			"what's",
			"how to",
			"how do i",
			"explain",
			"tell me about",
			"什么是",
			"啥是",
			"怎么用",
			"如何使用",
			"介绍一下",
			"解释一下",
		},
	}
}
