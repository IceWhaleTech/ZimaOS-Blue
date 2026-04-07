package selector

import (
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"
)

const (
	DomainLocalWorkspace = "local_workspace"
	DomainLiveWeb        = "live_web"
	DomainProductivity   = "productivity"
	DomainUIArtifact     = "ui_artifact"
	DomainURLPresent     = "url_present"
	DomainHighRisk       = "high_risk"
)

// QueryIntentSignals captures query-level gating and domain signals shared by
// tool and skill selectors.
type QueryIntentSignals struct {
	Query          string `json:"query,omitempty"`
	Normalized     string `json:"-"`
	QuestionPrefix bool   `json:"question_prefix,omitempty"`
	HowToQuestion  bool   `json:"howto_question,omitempty"`
	MetaIntent     bool   `json:"meta_intent,omitempty"`
	PlainReply     bool   `json:"plain_reply,omitempty"`
	Negated        bool   `json:"negated,omitempty"`
	Smalltalk      bool   `json:"smalltalk,omitempty"`
	LocalWorkspace bool   `json:"local_workspace,omitempty"`
	LiveWeb        bool   `json:"live_web,omitempty"`
	Productivity   bool   `json:"productivity,omitempty"`
	UIArtifact     bool   `json:"ui_artifact,omitempty"`
	URLPresent     bool   `json:"url_present,omitempty"`
	HighRisk       bool   `json:"high_risk,omitempty"`
}

func AnalyzeQuery(query string) QueryIntentSignals {
	q := strings.TrimSpace(query)
	lower := strings.ToLower(q)
	signals := QueryIntentSignals{
		Query:      query,
		Normalized: lower,
	}
	if lower == "" {
		return signals
	}

	signals.QuestionPrefix = staticCueMatchers.questionPrefixMatcher().HasAnyPrefix(lower)
	signals.HowToQuestion = staticCueMatchers.howToPrefixMatcher().HasAnyPrefix(lower) ||
		(signals.QuestionPrefix && staticCueMatchers.usageMetaTermMatcher().ContainsAnyFold(lower))
	signals.MetaIntent = staticCueMatchers.metaIntentTermMatcher().ContainsAnyFold(lower)
	signals.URLPresent = staticCueMatchers.urlPresentTermMatcher().ContainsAnyFold(lower)
	signals.LocalWorkspace = detectLocalWorkspace(lower)
	signals.LiveWeb = signals.URLPresent || staticCueMatchers.liveWebTermMatcher().ContainsAnyFold(lower)
	signals.Productivity = staticCueMatchers.productivityTermMatcher().ContainsAnyFold(lower)
	signals.UIArtifact = staticCueMatchers.uiArtifactTermMatcher().ContainsAnyFold(lower)
	signals.HighRisk = staticCueMatchers.highRiskTermMatcher().ContainsAnyFold(lower)
	if signals.LiveWeb && signals.Productivity && !signals.URLPresent && !staticCueMatchers.liveWebStrongTermMatcher().ContainsAnyFold(lower) {
		signals.LiveWeb = false
	}

	operational := signals.LocalWorkspace || signals.LiveWeb || signals.Productivity ||
		signals.UIArtifact || signals.URLPresent || staticCueMatchers.operationalTermMatcher().ContainsAnyFold(lower)

	signals.PlainReply = staticCueMatchers.plainReplyTermMatcher().ContainsAnyFold(lower) && !operational
	signals.Smalltalk = staticCueMatchers.smalltalkPrefixMatcher().HasAnyPrefix(lower) && !operational
	signals.Negated = detectNegation(lower)

	return signals
}

func (q QueryIntentSignals) BlocksAutomaticSelection() bool {
	return q.PlainReply || q.Negated || q.Smalltalk
}

func (q QueryIntentSignals) GatingFlags() []string {
	var flags []string
	if q.PlainReply {
		flags = append(flags, "plain_reply")
	}
	if q.HowToQuestion {
		flags = append(flags, "definition_or_howto")
	}
	if q.Negated {
		flags = append(flags, "negated")
	}
	if q.Smalltalk {
		flags = append(flags, "smalltalk")
	}
	if q.QuestionPrefix && !q.HowToQuestion {
		flags = append(flags, "question_prefix")
	}
	if q.MetaIntent {
		flags = append(flags, "meta_intent")
	}
	return flags
}

func (q QueryIntentSignals) DomainFlags() []string {
	var flags []string
	if q.LocalWorkspace {
		flags = append(flags, DomainLocalWorkspace)
	}
	if q.LiveWeb {
		flags = append(flags, DomainLiveWeb)
	}
	if q.Productivity {
		flags = append(flags, DomainProductivity)
	}
	if q.UIArtifact {
		flags = append(flags, DomainUIArtifact)
	}
	if q.URLPresent {
		flags = append(flags, DomainURLPresent)
	}
	if q.HighRisk {
		flags = append(flags, DomainHighRisk)
	}
	return flags
}

func (q QueryIntentSignals) HasDomain(domain string) bool {
	switch strings.ToLower(strings.TrimSpace(domain)) {
	case DomainLocalWorkspace:
		return q.LocalWorkspace
	case DomainLiveWeb:
		return q.LiveWeb
	case DomainProductivity:
		return q.Productivity
	case DomainUIArtifact:
		return q.UIArtifact
	case DomainURLPresent:
		return q.URLPresent
	case DomainHighRisk:
		return q.HighRisk
	default:
		return false
	}
}

func (q QueryIntentSignals) HasAnyDomain(domains []string) bool {
	for _, domain := range domains {
		if q.HasDomain(domain) {
			return true
		}
	}
	return false
}

// SelectorProfile describes the structured trigger surface for one tool or skill.
type SelectorProfile struct {
	Name              string
	ExactAliases      []string
	Actions           []string
	Objects           []string
	ContextCues       []string
	NegativeCues      []string
	PreferredDomains  []string
	RequireAnyDomains []string
	ConflictDomains   []string
}

type compiledSelectorProfile struct {
	raw          SelectorProfile
	exactAliases *textmatch.FoldedTermMatcher
	actions      *textmatch.FoldedTermMatcher
	objects      *textmatch.FoldedTermMatcher
	contextCues  *textmatch.FoldedTermMatcher
	negativeCues *textmatch.FoldedTermMatcher
}

// MatchResult captures structured signal matches for one candidate.
type MatchResult struct {
	Name             string   `json:"name"`
	Eligible         bool     `json:"eligible"`
	Anchored         bool     `json:"anchored"`
	Score            float64  `json:"score"`
	ExactAliasHits   []string `json:"exact_alias_hits,omitempty"`
	ActionHits       []string `json:"action_hits,omitempty"`
	ObjectHits       []string `json:"object_hits,omitempty"`
	ContextHits      []string `json:"context_hits,omitempty"`
	NegativeHits     []string `json:"negative_hits,omitempty"`
	DomainHits       []string `json:"domain_hits,omitempty"`
	ConflictFlags    []string `json:"conflict_flags,omitempty"`
	ConfidenceReason string   `json:"confidence_reason,omitempty"`
	MatchedSignals   []string `json:"matched_signals,omitempty"`
}

func MatchProfile(signals QueryIntentSignals, profile SelectorProfile) MatchResult {
	return matchCompiledProfile(signals, compileSelectorProfile(profile))
}

func matchCompiledProfile(signals QueryIntentSignals, profile compiledSelectorProfile) MatchResult {
	query := signals.Normalized
	result := MatchResult{Name: profile.raw.Name}
	if query == "" {
		return result
	}

	result.ExactAliasHits = compiledProfileMatches(profile.exactAliases, query)
	result.ActionHits = compiledProfileMatches(profile.actions, query)
	result.ObjectHits = compiledProfileMatches(profile.objects, query)
	result.ContextHits = compiledProfileMatches(profile.contextCues, query)
	result.NegativeHits = compiledProfileMatches(profile.negativeCues, query)
	for _, domain := range profile.raw.PreferredDomains {
		if signals.HasDomain(domain) {
			result.DomainHits = append(result.DomainHits, domain)
		}
	}
	for _, domain := range profile.raw.ConflictDomains {
		if signals.HasDomain(domain) {
			result.ConflictFlags = append(result.ConflictFlags, domain)
		}
	}

	if len(result.NegativeHits) > 0 {
		result.ConfidenceReason = "negative_cue"
		result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("negative", result.NegativeHits)...)
		return result
	}

	result.Anchored = len(result.ExactAliasHits) > 0 || (len(result.ActionHits) > 0 && len(result.ObjectHits) > 0)
	if !result.Anchored {
		return result
	}

	if len(profile.raw.RequireAnyDomains) > 0 && len(result.ExactAliasHits) == 0 && !signals.HasAnyDomain(profile.raw.RequireAnyDomains) {
		result.ConfidenceReason = "missing_required_domain"
		result.ConflictFlags = append(result.ConflictFlags, "missing_required_domain")
		result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("action", result.ActionHits)...)
		result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("object", result.ObjectHits)...)
		return result
	}

	score := 0.0
	if len(result.ExactAliasHits) > 0 {
		score += 4.0 + float64(len(result.ExactAliasHits)-1)*0.35
	}
	if len(result.ActionHits) > 0 {
		score += 1.35 + float64(minInt(len(result.ActionHits)-1, 2))*0.2
	}
	if len(result.ObjectHits) > 0 {
		score += 1.35 + float64(minInt(len(result.ObjectHits)-1, 2))*0.2
	}
	if len(result.ContextHits) > 0 {
		score += 0.35 + float64(minInt(len(result.ContextHits)-1, 2))*0.1
	}
	if len(result.DomainHits) > 0 {
		score += float64(len(result.DomainHits)) * 0.45
	}
	if len(result.ConflictFlags) > 0 {
		score -= float64(len(result.ConflictFlags)) * 2.4
	}

	result.Eligible = score > 0
	result.Score = score
	switch {
	case len(result.ExactAliasHits) > 0:
		result.ConfidenceReason = "exact_alias"
	case len(result.ActionHits) > 0 && len(result.ObjectHits) > 0:
		result.ConfidenceReason = "action_object_anchor"
	default:
		result.ConfidenceReason = "weak_anchor"
	}
	if len(result.ConflictFlags) > 0 {
		result.ConfidenceReason += "_conflict_penalized"
	}

	result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("exact_alias", result.ExactAliasHits)...)
	result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("action", result.ActionHits)...)
	result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("object", result.ObjectHits)...)
	result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("context", result.ContextHits)...)
	result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("domain", result.DomainHits)...)
	result.MatchedSignals = append(result.MatchedSignals, prefixedSignals("conflict", result.ConflictFlags)...)
	sort.Strings(result.MatchedSignals)
	sort.Strings(result.ConflictFlags)
	return result
}

func compiledProfileMatches(matcher *textmatch.FoldedTermMatcher, query string) []string {
	if matcher == nil || query == "" {
		return nil
	}
	return matcher.FindMatchesFold(query)
}

func FindMatches(text string, terms []string) []string {
	if text == "" || len(terms) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(terms))
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if !ContainsTerm(text, term) {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}
	sort.Strings(out)
	return out
}

func ContainsAny(text string, terms []string) bool {
	return len(FindMatches(text, terms)) > 0
}

func ContainsTerm(text, term string) bool {
	if text == "" || term == "" {
		return false
	}
	if isASCIITerm(term) && needsASCIIWordBoundary(term) {
		return containsASCIIWord(text, term)
	}
	return strings.Contains(text, term)
}

func HumanizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return strings.Join(strings.Fields(name), " ")
}

func detectLocalWorkspace(lower string) bool {
	hasContainer := staticCueMatchers.workspaceContainerMatcher().ContainsAnyFold(lower)
	hasFile := staticCueMatchers.workspaceFileExtMatcher().ContainsAnyFold(lower) &&
		staticCueMatchers.workspaceFileContextMatcher().ContainsAnyFold(lower)
	return hasContainer || hasFile
}

func detectNegation(lower string) bool {
	if lower == "" {
		return false
	}
	match, ok := staticCueMatchers.negationTermMatcher().FirstMatchFold(lower)
	if !ok {
		return false
	}
	if match.Start < 12 {
		return true
	}
	after := strings.TrimLeftFunc(string([]rune(lower)[match.Start:]), unicode.IsSpace)
	return staticCueMatchers.followupActionTermMatcher().ContainsAnyFold(after)
}

func prefixedSignals(prefix string, values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		out = append(out, prefix+":"+value)
	}
	return out
}

func hasPrefixTerm(text string, terms []string) bool {
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if strings.HasPrefix(text, term) {
			return true
		}
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

func needsASCIIWordBoundary(term string) bool {
	term = strings.TrimSpace(term)
	if term == "" {
		return false
	}
	return isASCIIWordByte(term[0]) && isASCIIWordByte(term[len(term)-1])
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func compileSelectorProfile(profile SelectorProfile) compiledSelectorProfile {
	key := selectorProfileCacheKey(profile)
	if cached, ok := selectorProfileCache.Load(key); ok {
		return cached.(compiledSelectorProfile)
	}
	compiled := compiledSelectorProfile{
		raw:          profile,
		exactAliases: textmatch.NewFoldedTermMatcher(profile.ExactAliases),
		actions:      textmatch.NewFoldedTermMatcher(profile.Actions),
		objects:      textmatch.NewFoldedTermMatcher(profile.Objects),
		contextCues:  textmatch.NewFoldedTermMatcher(profile.ContextCues),
		negativeCues: textmatch.NewFoldedTermMatcher(profile.NegativeCues),
	}
	actual, _ := selectorProfileCache.LoadOrStore(key, compiled)
	return actual.(compiledSelectorProfile)
}

func selectorProfileCacheKey(profile SelectorProfile) string {
	var b strings.Builder
	writeField := func(name string, values []string) {
		b.WriteString(name)
		b.WriteByte('=')
		for _, value := range values {
			b.WriteString(value)
			b.WriteByte('\x1f')
		}
		b.WriteByte('\x1e')
	}
	b.WriteString(strings.ToLower(strings.TrimSpace(profile.Name)))
	b.WriteByte('\x1d')
	writeField("exact", profile.ExactAliases)
	writeField("action", profile.Actions)
	writeField("object", profile.Objects)
	writeField("context", profile.ContextCues)
	writeField("negative", profile.NegativeCues)
	writeField("preferred", profile.PreferredDomains)
	writeField("required", profile.RequireAnyDomains)
	writeField("conflict", profile.ConflictDomains)
	return b.String()
}

var (
	selectorProfileCache sync.Map

	questionPrefixes = []string{
		"what is", "what's", "how to", "how do i", "how do", "what does", "什么是", "啥是", "怎么用", "如何使用", "介绍一下", "解释一下",
	}
	howToPrefixes = []string{
		"how to", "how do i", "how do", "怎么用", "如何使用",
	}
	usageMetaTerms = []string{
		"use", "using", "usage", "skill", "tool", "feature", "mode", "功能", "使用", "怎么用", "介绍", "解释",
	}
	metaIntentTerms = []string{
		"tool", "skill", "feature", "mode", "selector", "route", "routing", "功能", "技能", "模式", "路由",
	}
	plainReplyTerms = []string{
		"say ", "reply ", "reply with", "respond ", "just say", "only say", "just reply", "simply reply", "output exactly",
		"只回复", "只回答", "只说", "照着说", "直接回复",
	}
	smalltalkPrefixes = []string{
		"hi", "hello", "hey", "thanks", "thank you", "你好", "嗨", "谢谢", "多谢",
	}
	negationTerms = []string{
		"don't", "do not", "without", "stop", "cancel", "not", "不要", "别", "不用", "停止", "取消",
	}
	followupActionTerms = []string{
		"search", "open", "run", "use", "create", "delete", "generate", "执行", "搜索", "打开", "运行", "使用", "创建", "删除",
	}
	workspaceContainerBaseTerms = []string{
		"workspace", "repo", "repository", "folder", "directory", "path", "branch", "commit",
		"working tree", "working copy", "project", "in my workspace", "in your workspace",
		"工作区", "仓库", "目录", "文件夹", "路径", "分支", "提交",
	}
	workspaceFileExtTerms = []string{
		".csv", ".xlsx", ".xls", ".txt", ".md", ".pdf", ".json", ".yaml", ".yml", ".go", ".ts", ".vue",
	}
	workspaceFileContextBaseTerms = []string{
		"workspace", "repo", "repository", "folder", "directory", "provided", "read", "review", "analyze", "summarize", "source", "code",
		"工作区", "仓库", "目录", "文件夹", "提供", "读取", "查看", "分析", "总结", "代码", "源码",
	}
	liveWebBaseTerms = []string{
		"latest", "news", "source", "sources", "citation", "citations", "reference", "references",
		"doc", "docs", "documentation", "manual",
		"search the web", "web", "website", "web page", "webpage", "browser", "url", "internet", "online",
		"最新", "新闻", "来源", "引用", "参考", "文档", "官方文档", "网页", "网站", "浏览器", "网址", "联网", "在线",
	}
	productivityBaseTerms = []string{
		"email", "mail", "inbox", "calendar", "agenda", "schedule", "meeting", "event", "appointment",
		"remind", "reminder", "task", "todo", "notification", "邮件", "邮箱", "收件箱", "日历", "日程", "会议", "事件", "提醒", "任务", "待办", "通知",
	}
	uiArtifactBaseTerms = []string{
		"ui", "ux", "interface", "screen", "screenshot", "design", "figma", "mockup", "wireframe", "layout", "component", "page",
		"accessibility", "a11y", "界面", "截图", "设计稿", "设计", "页面", "布局", "组件", "原型", "无障碍", "可访问性",
	}
	highRiskTerms = []string{
		"delete", "remove", "drop", "overwrite", "reset", "destroy", "truncate", "wipe", "生产", "线上", "删", "覆盖", "重置", "清空", "销毁",
	}
	operationalTerms = []string{
		"search", "open", "browse", "run", "execute", "read", "write", "review", "compare", "analyze", "archive",
		"create", "update", "list", "find", "show", "check", "fix", "implement", "搜索", "打开", "浏览", "运行", "执行", "读取", "写入", "查看", "比较", "分析", "归档", "创建", "更新", "列出", "查找", "展示", "检查", "修复", "实现",
	}
)
