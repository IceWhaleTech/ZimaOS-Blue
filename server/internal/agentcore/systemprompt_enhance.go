package agentcore

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

// EnhancementMode determines the level of response guidance injected into the system prompt.
type EnhancementMode int

const (
	// EnhanceMinimal injects only a brief style nudge.
	EnhanceMinimal EnhancementMode = iota
	// EnhanceStandard injects structured response guidelines.
	EnhanceStandard
	// EnhanceDetailed injects comprehensive communication, code, and formatting guidance.
	EnhanceDetailed
)

// ScenarioType classifies the user's intent so we can pick the right enhancement.
type ScenarioType int

const (
	ScenarioGeneral  ScenarioType = iota // default / chitchat
	ScenarioCoding                       // programming, debugging, code review
	ScenarioCreative                     // writing, brainstorming, storytelling
	ScenarioAnalysis                     // data analysis, math, research
	ScenarioSysAdmin                     // system ops, docker, network, shell
	ScenarioExplain                      // explain concept, how-does-X-work
)

// scenarioSignals maps each scenario to its signal words.
// SignalWordScore uses these for IR-based scoring with diminishing returns.
var scenarioSignals = map[ScenarioType][]string{
	ScenarioCoding: {
		// EN
		"code", "function", "class", "compile", "error", "refactor",
		"api", "endpoint", "import", "package", "module", "struct",
		"interface", "implement", "test", "unit test", "lint",
		"git", "commit", "merge", "branch", "pull request",
		"bug", "debug", "fix", "patch", "review",
		"func", "def", "const", "var", "let", "type",
		"sql", "query", "database", "migration", "schema",
		"regex", "algorithm", "recursion", "pointer", "goroutine",
		"```",
		// CN
		"代码", "编程", "函数", "变量", "编译", "报错",
		"接口", "实现", "测试", "重构", "修复",
	},
	ScenarioSysAdmin: {
		// EN
		"server", "deploy", "nginx", "systemd", "cron", "ssh",
		"firewall", "port", "process", "disk", "memory usage",
		"cpu", "load average", "chmod", "chown", "sudo",
		"iptables", "dns", "ssl", "certificate",
		"docker", "container", "kubernetes", "k8s", "pod",
		"network", "ip address", "route", "proxy", "reverse proxy",
		"log", "monitor", "uptime", "restart", "service",
		// CN
		"服务器", "部署", "运维", "网络", "防火墙", "端口", "进程",
		"容器", "集群", "负载", "监控",
	},
	ScenarioAnalysis: {
		// EN
		"analyze", "analysis", "data", "statistics", "calculate",
		"formula", "chart", "graph", "metric", "benchmark",
		"compare", "evaluate", "measure", "probability",
		"dataset", "correlation", "regression", "distribution",
		"average", "median", "percentage", "ratio",
		// CN
		"分析", "数据", "统计", "计算", "公式", "图表",
		"对比", "评估", "指标", "概率",
	},
	ScenarioExplain: {
		// EN
		"explain", "what is", "how does", "why does", "concept",
		"difference between", "meaning", "definition", "principle",
		"how to", "tutorial", "guide", "understand", "overview",
		"pros and cons", "trade-off", "when to use",
		// CN
		"解释", "什么是", "怎么理解", "原理", "为什么",
		"区别", "含义", "定义", "教程", "入门",
	},
	ScenarioCreative: {
		// EN
		"write a story", "creative", "poem", "fiction", "brainstorm",
		"copywriting", "slogan", "narrative", "imagine",
		"draft", "outline", "plot", "character", "dialogue",
		"essay", "blog post", "article",
		// CN
		"写一篇", "写一个故事", "创意", "文案", "诗", "小说", "剧本",
		"草稿", "大纲", "情节", "角色", "对话",
	},
}

// scenarioMinConfidence is the minimum SignalWordScore needed to classify
// a message into a non-general scenario. Below this, we fall back to General.
const scenarioMinConfidence = 0.25

// DetectScenario classifies the user message into a scenario using the pruner's
// IR-based SignalWordScore. Each scenario's signal words are scored with
// diminishing returns, and the highest-scoring scenario wins (if above threshold).
func DetectScenario(userMessage string) ScenarioType {
	if userMessage == "" {
		return ScenarioGeneral
	}

	bestScenario := ScenarioGeneral
	bestScore := 0.0

	for scenario, signals := range scenarioSignals {
		score := pruner.SignalWordScore(userMessage, signals)
		if score > bestScore {
			bestScore = score
			bestScenario = scenario
		}
	}

	if bestScore < scenarioMinConfidence {
		return ScenarioGeneral
	}
	return bestScenario
}

// scenarioToMode maps scenario types to enhancement modes.
func scenarioToMode(s ScenarioType) EnhancementMode {
	switch s {
	case ScenarioCoding, ScenarioSysAdmin:
		return EnhanceDetailed
	case ScenarioAnalysis, ScenarioExplain, ScenarioCreative:
		return EnhanceStandard
	default:
		return EnhanceMinimal
	}
}

// enhancementTemplates holds the response guidelines for each mode.
var enhancementTemplates = map[EnhancementMode]string{
	EnhanceMinimal: `# Response Style
Be clear and concise. Match the user's language.`,

	EnhanceStandard: `# Response Guidelines
- Be clear and concise in your explanations
- Use code blocks with appropriate language tags when showing code
- Structure your response with headings when addressing multiple topics
- Provide practical examples when helpful
- Match the user's language (respond in Chinese if asked in Chinese)`,

	EnhanceDetailed: `# Response Guidelines

## Communication Style
- Be clear, concise, and professional
- Use simple language; avoid unnecessary jargon
- Structure complex responses with headings and bullet points
- Match the user's language

## Code Standards
- Use syntax-highlighted code blocks with appropriate language tags
- Include comments for complex logic
- Show complete, runnable examples when possible
- Mention dependencies or prerequisites

## Problem Solving
- Break down complex problems into steps
- Explain your reasoning when making recommendations
- Offer alternatives when multiple approaches exist
- Highlight potential pitfalls or edge cases`,
}

// buildEnhancement detects the scenario from the last user message and returns
// the appropriate response guidelines block.
func (b *SystemPromptBuilder) buildEnhancement(userMessage string) string {
	if userMessage == "" {
		return enhancementTemplates[EnhanceMinimal]
	}
	scenario := DetectScenario(userMessage)
	mode := scenarioToMode(scenario)
	return enhancementTemplates[mode]
}
