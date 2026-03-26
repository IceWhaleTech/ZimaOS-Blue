package skilladvisor

import (
	"regexp"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
)

type capabilityHint struct {
	Needle string
	Query  string
	Tags   []string
}

var (
	asciiWordRegexp = regexp.MustCompile(`[a-z0-9][a-z0-9+._/-]*`)
	capabilityHints = []capabilityHint{
		{Needle: "github actions", Query: "github actions workflow automation", Tags: []string{"github-actions", "workflow"}},
		{Needle: "github action", Query: "github actions workflow automation", Tags: []string{"github-actions", "workflow"}},
		{Needle: "发版", Query: "release automation changelog", Tags: []string{"release", "automation", "changelog"}},
		{Needle: "release", Query: "release automation changelog", Tags: []string{"release", "automation", "changelog"}},
		{Needle: "changelog", Query: "changelog generation release notes", Tags: []string{"changelog", "release-notes"}},
		{Needle: "搜索", Query: "web search research citations", Tags: []string{"web-search", "research", "citations"}},
		{Needle: "news", Query: "news research citations", Tags: []string{"news", "research", "citations"}},
		{Needle: "新闻", Query: "news research citations", Tags: []string{"news", "research", "citations"}},
		{Needle: "文档", Query: "documentation reference", Tags: []string{"docs", "reference"}},
		{Needle: "docs", Query: "documentation reference", Tags: []string{"docs", "reference"}},
		{Needle: "openai", Query: "openai api docs model selection", Tags: []string{"openai", "api", "docs", "models"}},
		{Needle: "browser", Query: "browser automation web extraction", Tags: []string{"browser", "automation", "web"}},
		{Needle: "网页", Query: "browser automation web extraction", Tags: []string{"browser", "automation", "web"}},
		{Needle: "代码评审", Query: "code review pull request", Tags: []string{"code-review", "pull-request"}},
		{Needle: "review", Query: "code review pull request", Tags: []string{"code-review", "pull-request"}},
		{Needle: "数据库", Query: "database migration schema rollback", Tags: []string{"database", "migration", "schema"}},
		{Needle: "migration", Query: "database migration schema rollback", Tags: []string{"database", "migration", "schema"}},
		{Needle: "workflow", Query: "workflow automation ci cd", Tags: []string{"workflow", "automation", "ci-cd"}},
		{Needle: "自动化", Query: "workflow automation", Tags: []string{"automation"}},
		{Needle: "提醒", Query: "reminder scheduling notification", Tags: []string{"reminder", "scheduling", "notification"}},
		{Needle: "schedule", Query: "task scheduling reminder", Tags: []string{"scheduling", "reminder"}},
	}
)

type HeuristicPlanner struct{}

func NewHeuristicPlanner() *HeuristicPlanner { return &HeuristicPlanner{} }

func (p *HeuristicPlanner) Plan(query string, installed *agentcore.Decision) Plan {
	query = strings.TrimSpace(query)
	if query == "" {
		return Plan{}
	}

	queries := []string{query}
	tags := make([]string, 0, 8)
	lower := strings.ToLower(query)

	for _, hint := range capabilityHints {
		if strings.Contains(lower, hint.Needle) {
			queries = appendUnique(queries, hint.Query)
			tags = appendUnique(tags, hint.Tags...)
		}
	}

	if englishQuery := extractASCIIKeywords(lower); englishQuery != "" {
		queries = appendUnique(queries, englishQuery)
	}

	if len(tags) > 1 {
		queries = appendUnique(queries, strings.Join(tags[:minInt(len(tags), 6)], " "))
	}

	return Plan{
		NeedNewSkill:   true,
		Reason:         advisorReason(installed),
		SearchQueries:  limitStrings(queries, 4),
		CapabilityTags: limitStrings(tags, 8),
	}
}

func advisorReason(installed *agentcore.Decision) string {
	if installed == nil {
		return "no_installed_skill_match_context"
	}
	if strings.TrimSpace(installed.SelectedSkill) == "" {
		return "no_installed_skill_match"
	}
	if installed.NeedClarify {
		return "installed_skill_uncertain"
	}
	if installed.Confidence < defaultInstalledSufficientConfidence {
		return "installed_skill_low_confidence"
	}
	return "installed_skill_partial"
}

func extractASCIIKeywords(query string) string {
	words := asciiWordRegexp.FindAllString(strings.ToLower(query), -1)
	if len(words) == 0 {
		return ""
	}
	words = limitStrings(appendUnique(nil, words...), 6)
	return strings.Join(words, " ")
}

func derivedTerms(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	terms := append([]string{}, extractASCIIList(query)...)
	for _, hint := range capabilityHints {
		if strings.Contains(query, hint.Needle) {
			terms = appendUnique(terms, hint.Tags...)
		}
	}
	return limitStrings(terms, 12)
}

func extractASCIIList(query string) []string {
	words := asciiWordRegexp.FindAllString(strings.ToLower(query), -1)
	return appendUnique(nil, words...)
}

func appendUnique(base []string, values ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(values))
	out := make([]string, 0, len(base)+len(values))
	for _, item := range base {
		normalized := strings.TrimSpace(strings.ToLower(item))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, strings.TrimSpace(item))
	}
	for _, item := range values {
		normalized := strings.TrimSpace(strings.ToLower(item))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, strings.TrimSpace(item))
	}
	return out
}

func limitStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
