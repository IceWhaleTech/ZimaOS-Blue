package skillmarket

import (
	"regexp"
	"sync"
)

type labeledRegexpRule struct {
	pattern *regexp.Regexp
	label   string
}

type permissionRegexpRule struct {
	pattern    *regexp.Regexp
	permission string
}

var (
	skillMarketParserOnce sync.Once

	nonSlugChars               *regexp.Regexp
	versionLikeCategoryPattern *regexp.Regexp
	marketplaceCategoryAliases map[string]string
	categoryDisplayOrder       map[string]int

	skillMarketIdentifierOnce sync.Once
	skillIDPattern            *regexp.Regexp

	skillMarketCatalogRegexOnce sync.Once
	skillHubStringRefPattern    *regexp.Regexp
	llmSkillsWebsitePattern     *regexp.Regexp

	skillMarketSecurityRulesOnce sync.Once
	dangerousCommandRules        []labeledRegexpRule
	promptInjectionRules         []labeledRegexpRule
	permissionRules              []permissionRegexpRule
	secretRules                  []labeledRegexpRule
)

func ensureSkillMarketParserGlobals() {
	skillMarketParserOnce.Do(func() {
		nonSlugChars = regexp.MustCompile(`[^a-z0-9._-]+`)
		versionLikeCategoryPattern = regexp.MustCompile(`^(?:v?\d+(?:[._-]\d+){0,4}(?:[-+][a-z0-9._-]+)?|release[-_. ]?v?\d+(?:[._-]\d+){0,4}|latest|stable)$`)
		marketplaceCategoryAliases = map[string]string{
			"ai":                              "ai_intelligence",
			"ai_intelligence":                 "ai_intelligence",
			"ai_skills":                       "ai_intelligence",
			"ai_tools":                        "ai_intelligence",
			"agent":                           "ai_intelligence",
			"agentic":                         "ai_intelligence",
			"agents":                          "ai_intelligence",
			"assistant":                       "ai_intelligence",
			"chatbot":                         "ai_intelligence",
			"claude":                          "ai_intelligence",
			"copilot":                         "ai_intelligence",
			"gpt":                             "ai_intelligence",
			"llm":                             "ai_intelligence",
			"model":                           "ai_intelligence",
			"prompt":                          "ai_intelligence",
			"rag":                             "ai_intelligence",
			"workflow_ai":                     "ai_intelligence",
			"人工智能":                            "ai_intelligence",
			"智能":                              "ai_intelligence",
			"ai智能":                            "ai_intelligence",
			"ai_智能":                           "ai_intelligence",
			"analytics":                       "data_analysis",
			"analysis":                        "data_analysis",
			"browser":                         "data_analysis",
			"data":                            "data_analysis",
			"data_analysis":                   "data_analysis",
			"information":                     "data_analysis",
			"research":                        "data_analysis",
			"search":                          "data_analysis",
			"web":                             "data_analysis",
			"分析":                              "data_analysis",
			"数据":                              "data_analysis",
			"数据分析":                            "data_analysis",
			"开发":                              "development_tools",
			"开发工具":                            "development_tools",
			"api":                             "development_tools",
			"coding":                          "development_tools",
			"connector":                       "development_tools",
			"connectors":                      "development_tools",
			"developer":                       "development_tools",
			"developer_tools":                 "development_tools",
			"developer-tools":                 "development_tools",
			"development":                     "development_tools",
			"development_tools":               "development_tools",
			"dev":                             "development_tools",
			"devops":                          "development_tools",
			"extension":                       "development_tools",
			"github":                          "development_tools",
			"git":                             "development_tools",
			"infrastructure":                  "development_tools",
			"integration":                     "development_tools",
			"integrations":                    "development_tools",
			"plugin":                          "development_tools",
			"plugins":                         "development_tools",
			"sdk":                             "development_tools",
			"tooling":                         "development_tools",
			"webhook":                         "development_tools",
			"communication":                   "communication_collaboration",
			"communication_collaboration":     "communication_collaboration",
			"collaboration":                   "communication_collaboration",
			"crm":                             "communication_collaboration",
			"chat":                            "communication_collaboration",
			"discord":                         "communication_collaboration",
			"email":                           "communication_collaboration",
			"meeting":                         "communication_collaboration",
			"messaging":                       "communication_collaboration",
			"notion":                          "communication_collaboration",
			"slack":                           "communication_collaboration",
			"teams":                           "communication_collaboration",
			"telegram":                        "communication_collaboration",
			"协作":                              "communication_collaboration",
			"通讯协作":                            "communication_collaboration",
			"communication_and_collaboration": "communication_collaboration",
			"content":                         "content_creation",
			"content_creation":                "content_creation",
			"copywriting":                     "content_creation",
			"creative":                        "content_creation",
			"design":                          "content_creation",
			"documentation":                   "content_creation",
			"image":                           "content_creation",
			"marketing":                       "content_creation",
			"media":                           "content_creation",
			"presentation":                    "content_creation",
			"social":                          "content_creation",
			"translation":                     "content_creation",
			"video":                           "content_creation",
			"writing":                         "content_creation",
			"内容创作":                            "content_creation",
			"automation":                      "productivity",
			"efficiency":                      "productivity",
			"general":                         "productivity",
			"productivity":                    "productivity",
			"task":                            "productivity",
			"todo":                            "productivity",
			"tool":                            "productivity",
			"tools":                           "productivity",
			"utility":                         "productivity",
			"utilities":                       "productivity",
			"workflow":                        "productivity",
			"效率":                              "productivity",
			"效率提升":                            "productivity",
			"compliance":                      "security_compliance",
			"governance":                      "security_compliance",
			"ops":                             "security_compliance",
			"risk":                            "security_compliance",
			"security":                        "security_compliance",
			"security_compliance":             "security_compliance",
			"system":                          "security_compliance",
			"安全":                              "security_compliance",
			"安全合规":                            "security_compliance",
			"misc":                            "other",
			"miscellaneous":                   "other",
			"other":                           "other",
		}
		categoryDisplayOrder = map[string]int{
			"ai_intelligence":             10,
			"development_tools":           20,
			"productivity":                30,
			"data_analysis":               40,
			"content_creation":            50,
			"security_compliance":         60,
			"communication_collaboration": 70,
			"other":                       999,
		}
	})
}

func ensureSkillMarketIdentifierRegex() {
	skillMarketIdentifierOnce.Do(func() {
		skillIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	})
}

func ensureSkillMarketCatalogRegexes() {
	skillMarketCatalogRegexOnce.Do(func() {
		skillHubStringRefPattern = regexp.MustCompile(`skillMdRaw":"\$([0-9A-Za-z]+)"`)
		llmSkillsWebsitePattern = regexp.MustCompile(`(?s)"skill":\{.*?"website":"((?:\\.|[^"\\])*)".*?"installPath":"((?:\\.|[^"\\])*)"`)
	})
}

func ensureSkillMarketSecurityRules() {
	skillMarketSecurityRulesOnce.Do(func() {
		dangerousCommandRules = []labeledRegexpRule{
			{pattern: regexp.MustCompile(`(?i)\brm\s+-rf\b`), label: "rm -rf"},
			{pattern: regexp.MustCompile(`(?i)\bcurl\b[^\n|]*\|\s*(sh|bash)\b`), label: "curl | sh"},
			{pattern: regexp.MustCompile(`(?i)\bwget\b[^\n|]*\|\s*(sh|bash)\b`), label: "wget | bash"},
			{pattern: regexp.MustCompile(`(?i)\bchmod\s+777\b`), label: "chmod 777"},
			{pattern: regexp.MustCompile(`(?i)\bscp\b`), label: "scp"},
			{pattern: regexp.MustCompile(`(?i)\bssh\b`), label: "ssh"},
		}
		promptInjectionRules = []labeledRegexpRule{
			{pattern: regexp.MustCompile(`(?i)ignore previous instructions`), label: "ignore previous instructions"},
			{pattern: regexp.MustCompile(`(?i)exfiltrate data`), label: "exfiltrate data"},
			{pattern: regexp.MustCompile(`(?i)reveal system prompt`), label: "reveal system prompt"},
		}
		permissionRules = []permissionRegexpRule{
			{pattern: regexp.MustCompile(`(?i)(read|write|modify).*(file|filesystem)|/etc/|~\/|\.zima`), permission: "filesystem"},
			{pattern: regexp.MustCompile(`(?i)\b(http|https|curl|wget|webhook|api request|download)\b`), permission: "network"},
			{pattern: regexp.MustCompile(`(?i)\b(shell|bash|zsh|sh|terminal|exec|command)\b`), permission: "shell"},
			{pattern: regexp.MustCompile(`(?i)\bdocker\b`), permission: "docker"},
			{pattern: regexp.MustCompile(`(?i)\b(systemctl|launchctl|registry|kernel|sudo)\b`), permission: "system"},
		}
		secretRules = []labeledRegexpRule{
			{pattern: regexp.MustCompile(`(?i)\b(?:api[_-]?key|token|secret)\s*[:=]\s*['"]?[a-z0-9_\-]{12,}`), label: "credential literal"},
			{pattern: regexp.MustCompile(`-----BEGIN (?:RSA|EC|OPENSSH|DSA|PGP) PRIVATE KEY-----`), label: "private key"},
			{pattern: regexp.MustCompile(`(?i)\bgh[pousr]_[A-Za-z0-9]{20,}\b`), label: "github token"},
		}
	})
}
