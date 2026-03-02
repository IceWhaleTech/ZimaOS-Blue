package server

import (
	_ "embed"
	"encoding/json"
	"strings"
)

type featureIntent struct {
	DeepResearch bool
	AgentMode    bool
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
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return featureIntent{}
	}
	if isFeatureDefinitionQuestion(text) {
		return featureIntent{}
	}

	deepExplicit := containsAnyFeatureTerm(text, featureIntentDict.DeepResearchExplicit)
	deepComposite := containsAnyFeatureTerm(text, featureIntentDict.DeepResearchActions) && containsAnyFeatureTerm(text, featureIntentDict.DeepResearchTargets)
	deepNegated := containsAnyFeatureTerm(text, featureIntentDict.DeepResearchNegation)
	deepResearch := (deepExplicit || deepComposite) && !deepNegated

	agentExplicit := containsAnyFeatureTerm(text, featureIntentDict.AgentModeExplicit)
	agentComposite := containsAnyFeatureTerm(text, featureIntentDict.AgentModeActions) && containsAnyFeatureTerm(text, featureIntentDict.AgentModeTargets)
	agentNegated := containsAnyFeatureTerm(text, featureIntentDict.AgentModeNegation)
	agentMode := (agentExplicit || agentComposite) && !agentNegated

	return featureIntent{
		DeepResearch: deepResearch,
		AgentMode:    agentMode,
	}
}

func isFeatureDefinitionQuestion(text string) bool {
	asksDefinition := false
	for _, prefix := range featureIntentDict.DefinitionPrefixes {
		if strings.HasPrefix(text, prefix) {
			asksDefinition = true
			break
		}
	}
	if !asksDefinition {
		return false
	}
	return containsAnyFeatureTerm(text, featureIntentDict.DeepResearchExplicit) || containsAnyFeatureTerm(text, featureIntentDict.AgentModeExplicit)
}

func containsAnyFeatureTerm(text string, terms []string) bool {
	for _, term := range terms {
		if containsFeatureTerm(text, term) {
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
			"深度搜索",
			"深度研究",
			"深入研究",
			"深度调研",
			"深入调研",
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
		},
		DeepResearchNegation: []string{
			"no deep research",
			"without deep research",
			"disable deep research",
			"不要深度搜索",
			"不用深度搜索",
			"关闭深度搜索",
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
			"什么是",
			"啥是",
			"怎么用",
			"如何使用",
			"介绍一下",
			"解释一下",
		},
	}
}
