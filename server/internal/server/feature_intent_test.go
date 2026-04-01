package server

import "testing"

func TestClassifyFeatureIntent_Empty(t *testing.T) {
	intent := classifyFeatureIntent("   ")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected empty input not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_DeepResearch(t *testing.T) {
	intent := classifyFeatureIntent("请帮我做一次深度研究并给出引用来源")
	if !intent.DeepResearch {
		t.Fatalf("expected deep research intent to be true")
	}
	if intent.AgentMode {
		t.Fatalf("expected agent mode intent to be false")
	}
}

func TestClassifyFeatureIntent_ResearchMode(t *testing.T) {
	intent := classifyFeatureIntent("Please use research mode and verify the sources before answering")
	if !intent.DeepResearch {
		t.Fatalf("expected research mode intent to map to research capability")
	}
	if intent.AgentMode {
		t.Fatalf("expected research mode intent not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_ResearchModePunctuationBoundary(t *testing.T) {
	intent := classifyFeatureIntent("(RESEARCH MODE), please verify sources first.")
	if !intent.DeepResearch {
		t.Fatalf("expected research mode with punctuation and casing to trigger research intent")
	}
	if intent.AgentMode {
		t.Fatalf("expected research mode punctuation case not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_ResearchModelBoundary(t *testing.T) {
	intent := classifyFeatureIntent("Please use the research model selector in settings")
	if intent.DeepResearch {
		t.Fatalf("expected research model wording not to match research mode")
	}
	if intent.AgentMode {
		t.Fatalf("expected research model wording not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_QuotedDocumentationMentions(t *testing.T) {
	intent := classifyFeatureIntent(`The docs should mention "research mode" and "agent mode" by name.`)
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected quoted documentation mentions not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_BacktickedDocumentationMentions(t *testing.T) {
	intent := classifyFeatureIntent("Document the `/research` command and the `research mode` label.")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected backticked command and label mentions not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_QuotedDocumentationMentionsChinese(t *testing.T) {
	intent := classifyFeatureIntent("文档里写上“研究模式”和“智能体模式”这两个词")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected chinese quoted documentation mentions not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_UnquotedDocumentationMentions(t *testing.T) {
	intent := classifyFeatureIntent("The docs should mention research mode and agent mode by name.")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected unquoted documentation mentions not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_UnquotedHelpCopyMentions(t *testing.T) {
	intent := classifyFeatureIntent("Please add research mode to the help text for settings.")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected unquoted help-copy mentions not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_UnquotedDocumentationMentionsChinese(t *testing.T) {
	intent := classifyFeatureIntent("把研究模式和智能体模式写进帮助文案里")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected chinese unquoted documentation mentions not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_RenameStyleUICopyEdit(t *testing.T) {
	intent := classifyFeatureIntent("Rename research mode to Research in the settings copy.")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected rename-style ui copy edits not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_AgentModeButtonLayoutEdit(t *testing.T) {
	intent := classifyFeatureIntent("Move the agent mode button below the composer.")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected button layout edits not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_LegacyDeepResearchRenameMenuLabel(t *testing.T) {
	intent := classifyFeatureIntent("Rename Deep Research to Research in the menu label.")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected legacy deep research rename menu label edits not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_AgentModeHeadingRelabel(t *testing.T) {
	intent := classifyFeatureIntent("Relabel agent mode as Agents in the heading.")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected heading relabel edits not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_RenameStyleUICopyEditChinese(t *testing.T) {
	intent := classifyFeatureIntent("把研究模式改成研究，并更新设置页文案")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected chinese rename-style ui copy edits not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_AgentModeButtonCopyEditChinese(t *testing.T) {
	intent := classifyFeatureIntent("把智能体模式按钮移动到底部，并调整按钮文案")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected chinese button copy edits not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_AgentModeTitleRelabelChinese(t *testing.T) {
	intent := classifyFeatureIntent("把智能体模式重命名为智能体，并更新标题")
	if intent.DeepResearch || intent.AgentMode {
		t.Fatalf("expected chinese title relabel edits not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_QuotedResearchModeRequest(t *testing.T) {
	intent := classifyFeatureIntent(`Please use "research mode" for this answer`)
	if !intent.DeepResearch {
		t.Fatalf("expected directly requested quoted research mode to trigger research intent")
	}
	if intent.AgentMode {
		t.Fatalf("expected directly requested quoted research mode not to trigger agent mode")
	}
}

func TestClassifyFeatureIntent_QuotedAgentModeRequest(t *testing.T) {
	intent := classifyFeatureIntent(`Please run in "agent mode" for this task`)
	if intent.DeepResearch {
		t.Fatalf("expected directly requested quoted agent mode not to trigger research intent")
	}
	if !intent.AgentMode {
		t.Fatalf("expected directly requested quoted agent mode to trigger agent mode")
	}
}

func TestClassifyFeatureIntent_UnquotedAgentModeRequest(t *testing.T) {
	intent := classifyFeatureIntent("Please run in agent mode for this task")
	if intent.DeepResearch {
		t.Fatalf("expected directly requested unquoted agent mode not to trigger research intent")
	}
	if !intent.AgentMode {
		t.Fatalf("expected directly requested unquoted agent mode to trigger agent mode")
	}
}

func TestClassifyFeatureIntent_RequestWithUICopyMention(t *testing.T) {
	intent := classifyFeatureIntent("Use research mode for this answer, then update the button copy later.")
	if !intent.DeepResearch {
		t.Fatalf("expected direct request with extra ui copy mention to keep research intent enabled")
	}
	if intent.AgentMode {
		t.Fatalf("expected direct request with extra ui copy mention not to trigger agent mode")
	}
}

func TestClassifyFeatureIntent_ResearchComposite(t *testing.T) {
	intent := classifyFeatureIntent("Please research thoroughly and include citations plus evidence")
	if !intent.DeepResearch {
		t.Fatalf("expected research action-plus-target combination to trigger research intent")
	}
	if intent.AgentMode {
		t.Fatalf("expected composite research intent not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_ResearchCompositeChinese(t *testing.T) {
	intent := classifyFeatureIntent("请帮我查阅来源和证据，并做个梳理")
	if !intent.DeepResearch {
		t.Fatalf("expected chinese research action-plus-target combination to trigger research intent")
	}
	if intent.AgentMode {
		t.Fatalf("expected chinese composite research intent not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_ResearchCompositeNeedsTargets(t *testing.T) {
	intent := classifyFeatureIntent("Please research thoroughly before answering")
	if intent.DeepResearch {
		t.Fatalf("expected composite research cue without targets not to trigger research intent")
	}
}

func TestClassifyFeatureIntent_ResearchCompositeChineseNeedsTargets(t *testing.T) {
	intent := classifyFeatureIntent("请帮我查阅并梳理一下")
	if intent.DeepResearch {
		t.Fatalf("expected chinese composite research cue without targets not to trigger research intent")
	}
}

func TestClassifyFeatureIntent_AgentMode(t *testing.T) {
	intent := classifyFeatureIntent("Please run in agent mode and execute this task end-to-end")
	if !intent.AgentMode {
		t.Fatalf("expected agent mode intent to be true")
	}
}

func TestClassifyFeatureIntent_AgentModelBoundary(t *testing.T) {
	intent := classifyFeatureIntent("The agent model should stay on auto routing")
	if intent.AgentMode {
		t.Fatalf("expected agent model wording not to match agent mode")
	}
	if intent.DeepResearch {
		t.Fatalf("expected agent model wording not to enable research intent")
	}
}

func TestClassifyFeatureIntent_AgentLoop(t *testing.T) {
	intent := classifyFeatureIntent("Please switch to agent loop and keep iterating improvements")
	if !intent.AgentMode {
		t.Fatalf("expected agent loop intent to map to agent mode")
	}
}

func TestClassifyFeatureIntent_AgentCompositeChinese(t *testing.T) {
	intent := classifyFeatureIntent("请自动执行这个任务并分步完成")
	if !intent.AgentMode {
		t.Fatalf("expected chinese agent action-plus-target combination to trigger agent mode")
	}
	if intent.DeepResearch {
		t.Fatalf("expected chinese composite agent intent not to enable research intent")
	}
}

func TestClassifyFeatureIntent_AgentCompositeNeedsTargets(t *testing.T) {
	intent := classifyFeatureIntent("Please be autonomous and careful")
	if intent.AgentMode {
		t.Fatalf("expected composite agent cue without task targets not to trigger agent mode")
	}
}

func TestClassifyFeatureIntent_AgentCompositeChineseNeedsTargets(t *testing.T) {
	intent := classifyFeatureIntent("请自动执行一下")
	if intent.AgentMode {
		t.Fatalf("expected chinese composite agent cue without targets not to trigger agent mode")
	}
}

func TestClassifyFeatureIntent_Negation(t *testing.T) {
	intent := classifyFeatureIntent("不要深度搜索，直接简单回答")
	if intent.DeepResearch {
		t.Fatalf("expected deep research intent to be false for negation")
	}
}

func TestClassifyFeatureIntent_ResearchModeChineseNegation(t *testing.T) {
	intent := classifyFeatureIntent("这次不用研究模式，直接给我简短回答")
	if intent.DeepResearch {
		t.Fatalf("expected chinese research mode negation to disable research intent")
	}
}

func TestClassifyFeatureIntent_ResearchModeNegation(t *testing.T) {
	intent := classifyFeatureIntent("disable research mode and just answer directly")
	if intent.DeepResearch {
		t.Fatalf("expected research mode intent to be false for negation")
	}
}

func TestClassifyFeatureIntent_ResearchNegationOverridesComposite(t *testing.T) {
	intent := classifyFeatureIntent("Please research thoroughly with citations and evidence, but without research mode")
	if intent.DeepResearch {
		t.Fatalf("expected research negation to override composite research cues")
	}
}

func TestClassifyFeatureIntent_ResearchMixedLanguageNegation(t *testing.T) {
	intent := classifyFeatureIntent("Please use research mode，但这次不用研究模式")
	if intent.DeepResearch {
		t.Fatalf("expected mixed-language research negation to override explicit research mode")
	}
}

func TestClassifyFeatureIntent_AgentLoopNegation(t *testing.T) {
	intent := classifyFeatureIntent("disable agent loop and just answer directly")
	if intent.AgentMode {
		t.Fatalf("expected agent mode intent to be false for agent loop negation")
	}
}

func TestClassifyFeatureIntent_ResearchAndAgentTogether(t *testing.T) {
	intent := classifyFeatureIntent("Use research mode first, then run in agent mode to execute the workflow")
	if !intent.DeepResearch {
		t.Fatalf("expected research intent to remain true when combined with agent mode")
	}
	if !intent.AgentMode {
		t.Fatalf("expected agent mode intent to remain true when combined with research mode")
	}
}

func TestClassifyFeatureIntent_ResearchEnabledAgentDisabled(t *testing.T) {
	intent := classifyFeatureIntent("Use research mode for sources, but disable agent mode for execution")
	if !intent.DeepResearch {
		t.Fatalf("expected research intent to stay enabled when agent mode is negated")
	}
	if intent.AgentMode {
		t.Fatalf("expected agent mode negation to keep agent mode disabled")
	}
}

func TestClassifyFeatureIntent_AgentEnabledResearchDisabled(t *testing.T) {
	intent := classifyFeatureIntent("Disable research mode this time, but run in agent mode to execute the task")
	if intent.DeepResearch {
		t.Fatalf("expected research mode negation to keep research intent disabled")
	}
	if !intent.AgentMode {
		t.Fatalf("expected agent mode to stay enabled when research mode is negated")
	}
}

func TestClassifyFeatureIntent_ResearchEnabledAgentDisabledChinese(t *testing.T) {
	intent := classifyFeatureIntent("请开启研究模式，但不要智能体模式")
	if !intent.DeepResearch {
		t.Fatalf("expected chinese research intent to stay enabled when agent mode is negated")
	}
	if intent.AgentMode {
		t.Fatalf("expected chinese agent mode negation to keep agent mode disabled")
	}
}

func TestClassifyFeatureIntent_AgentEnabledResearchDisabledChinese(t *testing.T) {
	intent := classifyFeatureIntent("不要研究模式，但请自动执行这个任务")
	if intent.DeepResearch {
		t.Fatalf("expected chinese research mode negation to keep research intent disabled")
	}
	if !intent.AgentMode {
		t.Fatalf("expected chinese agent intent to stay enabled when research mode is negated")
	}
}

func TestClassifyFeatureIntent_DefinitionQuestion(t *testing.T) {
	intent := classifyFeatureIntent("什么是 agent mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected definition question not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_ResearchDefinitionQuestion(t *testing.T) {
	intent := classifyFeatureIntent("what is research mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected research mode definition question not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_ResearchDefinitionQuestionChinese(t *testing.T) {
	intent := classifyFeatureIntent("什么是研究模式")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected chinese research mode definition question not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_ResearchDefinitionQuestionMixedLanguage(t *testing.T) {
	intent := classifyFeatureIntent("介绍一下 research mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected mixed-language research mode definition question not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_ResearchDefinitionQuestionCourtesyPrefix(t *testing.T) {
	intent := classifyFeatureIntent("请介绍一下 research mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected courtesy-prefixed research mode definition question not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_AgentDefinitionQuestionCourtesyPrefix(t *testing.T) {
	intent := classifyFeatureIntent("请介绍一下 agent mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected courtesy-prefixed agent mode definition question not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_ResearchDefinitionQuestionExplainPrefix(t *testing.T) {
	intent := classifyFeatureIntent("please explain research mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected explain-style research mode definition question not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_AgentDefinitionQuestionTellMeAboutPrefix(t *testing.T) {
	intent := classifyFeatureIntent("tell me about agent mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected tell-me-about agent mode definition question not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_ResearchHowToQuestion(t *testing.T) {
	intent := classifyFeatureIntent("how do i enable research mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected how-to question about research mode not to trigger feature intent")
	}
}

func TestClassifyFeatureIntent_DefinitionQuestionThenResearchRequest(t *testing.T) {
	intent := classifyFeatureIntent("what is research mode? please use research mode for this answer")
	if !intent.DeepResearch {
		t.Fatalf("expected trailing research request after definition question to trigger research intent")
	}
	if intent.AgentMode {
		t.Fatalf("expected trailing research request after definition question not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_DefinitionQuestionThenResearchRequestChinese(t *testing.T) {
	intent := classifyFeatureIntent("什么是研究模式？然后开启研究模式")
	if !intent.DeepResearch {
		t.Fatalf("expected trailing chinese research request after definition question to trigger research intent")
	}
	if intent.AgentMode {
		t.Fatalf("expected trailing chinese research request after definition question not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_DefinitionQuestionThenAgentRequest(t *testing.T) {
	intent := classifyFeatureIntent("what is research mode? then run in agent mode for this task")
	if intent.DeepResearch {
		t.Fatalf("expected definition clause not to keep research intent enabled once only agent mode is requested")
	}
	if !intent.AgentMode {
		t.Fatalf("expected trailing agent request after definition question to trigger agent mode")
	}
}

func TestClassifyFeatureIntent_DefinitionQuestionThenResearchRequestCourtesyPrefix(t *testing.T) {
	intent := classifyFeatureIntent("请解释一下 research mode，然后开启研究模式")
	if !intent.DeepResearch {
		t.Fatalf("expected trailing courtesy-prefixed chinese research request after definition question to trigger research intent")
	}
	if intent.AgentMode {
		t.Fatalf("expected trailing courtesy-prefixed chinese research request after definition question not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_ChainedDefinitionQuestionsThenRequest(t *testing.T) {
	intent := classifyFeatureIntent("what is research mode? what is agent mode? then use research mode")
	if !intent.DeepResearch {
		t.Fatalf("expected trailing request after chained definition clauses to trigger research intent")
	}
	if intent.AgentMode {
		t.Fatalf("expected trailing request after chained definition clauses not to enable agent mode")
	}
}

func TestClassifyFeatureIntent_ExplainDefinitionThenAgentRequest(t *testing.T) {
	intent := classifyFeatureIntent("please explain research mode, then run in agent mode for this task")
	if intent.DeepResearch {
		t.Fatalf("expected explain-style definition clause not to keep research intent enabled once only agent mode is requested")
	}
	if !intent.AgentMode {
		t.Fatalf("expected trailing agent request after explain-style definition clause to trigger agent mode")
	}
}

func TestClassifyFeatureIntent_DualDefinitionQuestion(t *testing.T) {
	intent := classifyFeatureIntent("what is research mode and what is agent mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected dual definition question not to trigger feature intent")
	}
}

func TestFeatureIntentDictionaryLoaded(t *testing.T) {
	if len(featureIntentDict.DeepResearchExplicit) == 0 {
		t.Fatalf("expected deep research terms to be loaded")
	}
	if len(featureIntentDict.AgentModeExplicit) == 0 {
		t.Fatalf("expected agent mode terms to be loaded")
	}
	if len(featureIntentDict.DefinitionPrefixes) == 0 {
		t.Fatalf("expected definition prefixes to be loaded")
	}
}
