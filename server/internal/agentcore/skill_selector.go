package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
	sel "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

const (
	SkillSelectorModeHybrid  = "hybrid"
	SkillSelectorModeIROnly  = "ir_only"
	SkillSelectorModeLLMOnly = "llm_only"
)

const (
	defaultSkillTopK                 = 8
	defaultSkillSelectCacheTTL       = 10 * time.Minute
	defaultSkillSelectorConfThres    = 0.78
	skillSelectorNeedClarifyFloor    = 0.55
	defaultSkillSelectorHintMaxCands = 3
	defaultSkillBudgetBaseTokens     = 200
	defaultSkillBudgetExampleTokens  = 150
)

var ErrTokenBudgetExhausted = errors.New("skill selector token budget exhausted")

type TokenBudget struct {
	MaxTokens      int
	ReservedTokens int
	PerSkillTokens int
}

// SelectOptions controls the progressive selector behavior.
type SelectOptions struct {
	Mode                string
	EnableRerank        bool
	ConfidenceThreshold float64
	TokenBudget         TokenBudget
	StrictTokenBudget   bool
}

// SkillCandidate is one ranked candidate.
type SkillCandidate struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Description string  `json:"description,omitempty"`
}

// Decision is the selector output.
type Decision struct {
	Query              string           `json:"query"`
	SelectedSkill      string           `json:"selected_skill"`
	Confidence         float64          `json:"confidence"`
	NeedClarify        bool             `json:"need_clarify"`
	Reason             string           `json:"reason"`
	Stage              string           `json:"stage"`
	Candidates         []SkillCandidate `json:"candidates,omitempty"`
	MatchedSignals     []string         `json:"matched_signals,omitempty"`
	ConflictFlags      []string         `json:"conflict_flags,omitempty"`
	ConfidenceReason   string           `json:"confidence_reason,omitempty"`
	LoadedSkills       []string         `json:"loaded_skills,omitempty"`
	SkippedSkills      []string         `json:"skipped_skills,omitempty"`
	TokenBudgetMax     int              `json:"token_budget_max,omitempty"`
	TokenBudgetUsed    int              `json:"token_budget_used,omitempty"`
	TokenBudgetTrimmed bool             `json:"token_budget_trimmed,omitempty"`
}

type skillRankedCandidate struct {
	doc     SkillDoc
	match   sel.MatchResult
	score   float64
	docText string
}

type skillIndexEntry struct {
	doc    SkillDoc
	bundle skillSelectorBundle
}

type SkillSelectorSkillRuntimeView struct {
	ID               string   `json:"id,omitempty"`
	Name             string   `json:"name,omitempty"`
	Paths            []string `json:"paths,omitempty"`
	UserInvocable    bool     `json:"user_invocable"`
	ModelInvocable   bool     `json:"model_invocable"`
	ActivationState  string   `json:"activation_state,omitempty"`
	ActivationSource string   `json:"activation_source,omitempty"`
}

type SkillSelectorDebugState struct {
	ActiveSkillCount           int                                      `json:"active_skill_count"`
	DormantSkillCount          int                                      `json:"dormant_skill_count"`
	CacheInvalidationCount     uint64                                   `json:"cache_invalidation_count"`
	DiscoveredDirs             []string                                 `json:"discovered_dirs,omitempty"`
	ActivatedConditionalSkills []string                                 `json:"activated_conditional_skills,omitempty"`
	Skills                     map[string]SkillSelectorSkillRuntimeView `json:"skills,omitempty"`
}

// PromptHint returns a compact XML block for system prompt injection.
func (d Decision) PromptHint(maxCandidates int) string {
	if maxCandidates <= 0 {
		maxCandidates = defaultSkillSelectorHintMaxCands
	}
	if len(d.Candidates) == 0 {
		return ""
	}
	if maxCandidates > len(d.Candidates) {
		maxCandidates = len(d.Candidates)
	}
	var sb strings.Builder
	sb.WriteString("<selected_skill_candidates query=")
	sb.WriteString(fmt.Sprintf("%q", truncateForIndex(d.Query, 160)))
	sb.WriteString(" confidence=")
	sb.WriteString(fmt.Sprintf("%q", fmt.Sprintf("%.2f", d.Confidence)))
	sb.WriteString(" need_clarify=")
	sb.WriteString(fmt.Sprintf("%q", fmt.Sprintf("%t", d.NeedClarify)))
	sb.WriteString(">")
	for i := 0; i < maxCandidates; i++ {
		c := d.Candidates[i]
		sb.WriteString("<skill name=")
		sb.WriteString(fmt.Sprintf("%q", xmlEscape(c.Name)))
		if c.Description != "" {
			sb.WriteString(" desc=")
			sb.WriteString(fmt.Sprintf("%q", truncateForIndex(c.Description, 120)))
		}
		sb.WriteString(" />")
	}
	if d.SelectedSkill != "" {
		sb.WriteString("<decision selected=")
		sb.WriteString(fmt.Sprintf("%q", xmlEscape(d.SelectedSkill)))
		sb.WriteString(" stage=")
		sb.WriteString(fmt.Sprintf("%q", d.Stage))
		sb.WriteString(" />")
	}
	sb.WriteString("</selected_skill_candidates>")
	return sb.String()
}

// SkillSelector performs staged selection with index + cache + optional rerank.
type SkillSelector struct {
	workspaceDir string
	reranker     SkillReranker
	exposure     *skillmanifest.SkillExposureManager

	mu                     sync.RWMutex
	indexLoaded            bool
	indexStamp             string
	indexDocs              []SkillDoc
	indexCache             []skillIndexEntry
	cacheInvalidationCount uint64
	debugState             SkillSelectorDebugState

	cacheMu  sync.RWMutex
	cache    map[string]cachedSkillDecision
	cacheTTL time.Duration
}

type cachedSkillDecision struct {
	decision Decision
	expires  time.Time
}

// NewSkillSelector creates a selector with default caching behavior.
func NewSkillSelector(workspaceDir string, reranker SkillReranker) *SkillSelector {
	if reranker == nil {
		reranker = NewHeuristicSkillReranker()
	}
	return &SkillSelector{
		workspaceDir: workspaceDir,
		reranker:     reranker,
		exposure:     skillmanifest.SharedSkillExposureManager(workspaceDir),
		cache:        make(map[string]cachedSkillDecision),
		cacheTTL:     defaultSkillSelectCacheTTL,
	}
}

func (s *SkillSelector) SetDynamicExposureEnabledFunc(fn func() bool) {
	if s == nil || s.exposure == nil {
		return
	}
	s.exposure.SetDynamicExposureEnabledFunc(fn)
}

func (s *SkillSelector) ExposureManager() *skillmanifest.SkillExposureManager {
	if s == nil {
		return nil
	}
	return s.exposure
}

func (s *SkillSelector) DebugState() (SkillSelectorDebugState, error) {
	if s == nil {
		return SkillSelectorDebugState{}, nil
	}
	if _, _, err := s.loadIndex(); err != nil {
		return SkillSelectorDebugState{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSkillSelectorDebugState(s.debugState), nil
}

func (s *SkillSelector) LookupSkillRuntimeView(name string) (SkillSelectorSkillRuntimeView, bool, error) {
	if s == nil {
		return SkillSelectorSkillRuntimeView{}, false, nil
	}
	if _, _, err := s.loadIndex(); err != nil {
		return SkillSelectorSkillRuntimeView{}, false, err
	}

	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return SkillSelectorSkillRuntimeView{}, false, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	view, ok := s.debugState.Skills[key]
	if !ok {
		return SkillSelectorSkillRuntimeView{}, false, nil
	}
	return cloneSkillRuntimeView(view), true, nil
}

func normalizeSkillSelectorMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case SkillSelectorModeIROnly:
		return SkillSelectorModeIROnly
	case SkillSelectorModeLLMOnly:
		return SkillSelectorModeLLMOnly
	default:
		return SkillSelectorModeHybrid
	}
}

// Select runs staged selection and returns a stable decision.
func (s *SkillSelector) Select(ctx context.Context, query string, opts SelectOptions) (Decision, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Decision{}, nil
	}

	mode := normalizeSkillSelectorMode(opts.Mode)
	thres := opts.ConfidenceThreshold
	if thres <= 0 || thres > 1 {
		thres = defaultSkillSelectorConfThres
	}

	cacheKey := s.buildCacheKey(query, mode, opts.EnableRerank, thres, opts.TokenBudget, opts.StrictTokenBudget)
	if d, ok := s.getCachedDecision(cacheKey); ok {
		return d, nil
	}

	docs, indexCache, err := s.loadIndex()
	if err != nil {
		return Decision{}, err
	}
	if len(docs) == 0 {
		return Decision{Query: query, NeedClarify: true, Reason: "no_skill_docs", Stage: "empty"}, nil
	}

	if rule := stage0RuleRoute(query); rule.SelectedSkill != "" && mode != SkillSelectorModeLLMOnly {
		rule.Candidates = buildCandidatesFromNames(rule.SelectedSkill, docs)
		return s.finalizeDecision(cacheKey, rule, docs, opts)
	}

	irDecision := s.stage1IR(query, indexCache, thres)
	if mode == SkillSelectorModeIROnly {
		return s.finalizeDecision(cacheKey, irDecision, docs, opts)
	}

	if mode == SkillSelectorModeHybrid && !shouldTriggerRerank(query, irDecision) {
		return s.finalizeDecision(cacheKey, irDecision, docs, opts)
	}
	if !opts.EnableRerank || s.reranker == nil {
		return s.finalizeDecision(cacheKey, irDecision, docs, opts)
	}

	rankedDocs := make([]SkillDoc, 0, minInt(defaultSkillTopK, len(irDecision.Candidates)))
	for i := 0; i < len(irDecision.Candidates) && i < defaultSkillTopK; i++ {
		name := strings.ToLower(irDecision.Candidates[i].Name)
		for _, d := range docs {
			if strings.ToLower(d.Name) == name {
				rankedDocs = append(rankedDocs, d)
				break
			}
		}
	}
	if len(rankedDocs) == 0 {
		return s.finalizeDecision(cacheKey, irDecision, docs, opts)
	}

	rr, err := s.reranker.Rerank(ctx, query, rankedDocs)
	if err != nil {
		return s.finalizeDecision(cacheKey, irDecision, docs, opts)
	}
	out := irDecision
	if rr.SelectedSkill != "" {
		out.SelectedSkill = rr.SelectedSkill
	}
	if rr.Confidence > 0 {
		out.Confidence = rr.Confidence
	}
	out.Stage = "rerank"
	out.Reason = rr.ReasonShort
	out.NeedClarify = rr.NeedClarify || out.Confidence < thres || out.Confidence < skillSelectorNeedClarifyFloor || len(out.ConflictFlags) > 0

	return s.finalizeDecision(cacheKey, out, docs, opts)
}

func (s *SkillSelector) finalizeDecision(cacheKey string, decision Decision, docs []SkillDoc, opts SelectOptions) (Decision, error) {
	decision, budgetErr := applyTokenBudgetToDecision(decision, docs, opts.TokenBudget, opts.StrictTokenBudget)
	s.setCachedDecision(cacheKey, decision)
	return decision, budgetErr
}

func (s *SkillSelector) stage1IR(query string, docs []skillIndexEntry, threshold float64) Decision {
	signals := sel.AnalyzeQuery(query)
	if signals.PlainReply || signals.Smalltalk || signals.Negated {
		return Decision{
			Query:         query,
			NeedClarify:   false,
			Reason:        "ir_suppressed",
			Stage:         "ir",
			ConflictFlags: signals.GatingFlags(),
		}
	}

	ranked := make([]skillRankedCandidate, 0, len(docs))
	for _, entry := range docs {
		match := sel.MatchProfile(signals, entry.bundle.profile)
		applySkillHardAnchors(signals, entry.doc, &match)
		if shouldSuppressDefinitionLikeSkillQuery(signals, match) {
			continue
		}
		if !match.Eligible {
			continue
		}
		ranked = append(ranked, skillRankedCandidate{
			doc:     entry.doc,
			match:   match,
			score:   match.Score,
			docText: entry.bundle.docText,
		})
	}

	if len(ranked) == 0 {
		reason := "ir_no_match"
		needClarify := true
		if signals.HowToQuestion || signals.MetaIntent {
			reason = "ir_definition_like"
			needClarify = false
		}
		return Decision{Query: query, NeedClarify: needClarify, Reason: reason, Stage: "ir", ConflictFlags: signals.GatingFlags()}
	}

	applySkillBM25TieBreak(query, ranked)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].doc.Name < ranked[j].doc.Name
		}
		return ranked[i].score > ranked[j].score
	})
	if len(ranked) > defaultSkillTopK {
		ranked = ranked[:defaultSkillTopK]
	}

	top1 := ranked[0].score
	top2 := 0.0
	if len(ranked) > 1 {
		top2 = ranked[1].score
	}
	scoreGap := top1 - top2

	conf := 0.44
	if len(ranked[0].match.ExactAliasHits) > 0 {
		conf += 0.22
	}
	if len(ranked[0].match.ActionHits) > 0 && len(ranked[0].match.ObjectHits) > 0 {
		conf += 0.18
	}
	if len(ranked[0].match.ContextHits) > 0 {
		conf += 0.05
	}
	if len(ranked[0].match.DomainHits) > 0 {
		conf += 0.06
	}
	switch {
	case scoreGap >= 1.2:
		conf += 0.12
	case scoreGap >= 0.65:
		conf += 0.08
	case scoreGap < 0.35:
		conf -= 0.08
	}
	if len(ranked[0].match.ConflictFlags) > 0 {
		conf -= 0.18
	}
	if signals.HighRisk && conf > 0.72 {
		conf = 0.72
	}
	conf = clampFloat(conf, 0, 1)

	needClarify := conf < threshold || conf < skillSelectorNeedClarifyFloor || len(ranked[0].match.ConflictFlags) > 0
	result := Decision{
		Query:            query,
		SelectedSkill:    ranked[0].doc.Name,
		Confidence:       conf,
		NeedClarify:      needClarify,
		Reason:           "ir_ranked",
		Stage:            "ir",
		MatchedSignals:   append([]string(nil), ranked[0].match.MatchedSignals...),
		ConflictFlags:    append([]string(nil), ranked[0].match.ConflictFlags...),
		ConfidenceReason: ranked[0].match.ConfidenceReason,
		Candidates:       make([]SkillCandidate, 0, minInt(3, len(ranked))),
	}
	for i := 0; i < len(ranked) && i < 3; i++ {
		result.Candidates = append(result.Candidates, SkillCandidate{
			Name:        ranked[i].doc.Name,
			Score:       ranked[i].score,
			Description: ranked[i].doc.Description,
		})
	}
	if scoreGap < 0.35 {
		result.ConfidenceReason += "_gap_small"
	}
	return result
}

func applySkillHardAnchors(signals sel.QueryIntentSignals, doc SkillDoc, match *sel.MatchResult) {
	if match == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(doc.Name)) {
	case "browser":
		if !signals.URLPresent || shouldBypassURLBrowserRule(signals.Normalized) {
			return
		}
		match.Eligible = true
		match.Anchored = true
		if match.Score < 4.8 {
			match.Score = 4.8
		}
		match.ConfidenceReason = "url_present_rule"
		match.ConflictFlags = nil
		match.MatchedSignals = append(match.MatchedSignals, "rule:url_present")
		sort.Strings(match.MatchedSignals)
	case "analyze":
		if signals.LocalWorkspace || signals.Productivity || (!signals.LiveWeb && !signals.URLPresent) {
			return
		}
		if !hasSelectorTerm(signals.Normalized, uniqueStringTerms(append([]string{
			"analyze", "analysis", "summarize", "summary", "synthesize", "compare", "report", "extract", "inspect", "review",
			"分析", "总结", "提炼", "比较", "报告", "归纳", "评估",
		}, routingcue.URLBypassTermsForSkill("analyze")...))) {
			return
		}
		match.Eligible = true
		match.Anchored = true
		if match.Score < 5.05 {
			match.Score = 5.05
		}
		if match.ConfidenceReason == "" {
			match.ConfidenceReason = "web_analysis_rule"
		}
		match.MatchedSignals = append(match.MatchedSignals, "rule:web_analysis")
		sort.Strings(match.MatchedSignals)
	case "himalaya":
		if signals.LiveWeb || !hasSelectorTerm(signals.Normalized, []string{
			"email", "mail", "inbox", "imap", "smtp", "reply", "forward", "attachment",
			"邮件", "邮箱", "收件箱", "回复", "转发", "附件",
		}) {
			return
		}
		match.Eligible = true
		match.Anchored = true
		if match.Score < 5.15 {
			match.Score = 5.15
		}
		if match.ConfidenceReason == "" {
			match.ConfidenceReason = "email_cli_rule"
		}
		match.MatchedSignals = append(match.MatchedSignals, "rule:email_cli")
		sort.Strings(match.MatchedSignals)
	}
}

func shouldSuppressDefinitionLikeSkillQuery(signals sel.QueryIntentSignals, match sel.MatchResult) bool {
	if len(match.ExactAliasHits) == 0 {
		return false
	}
	if signals.HowToQuestion {
		return true
	}
	return signals.QuestionPrefix && len(match.ObjectHits) == 0
}

func applySkillBM25TieBreak(query string, ranked []skillRankedCandidate) {
	if len(ranked) < 2 {
		return
	}
	scorer := pruner.NewBM25Scorer(1.2, 0.75)
	segments := make([]pruner.Segment, 0, len(ranked))
	for i, cand := range ranked {
		segments = append(segments, pruner.Segment{
			Content:   cand.docText,
			Tokens:    pruner.TextTokenize(cand.docText),
			StartLine: i,
			EndLine:   i,
		})
	}
	scored := scorer.Score(query, segments)
	for rank, segmentScore := range scored {
		if segmentScore.Segment.StartLine < 0 || segmentScore.Segment.StartLine >= len(ranked) {
			continue
		}
		bonus := 0.06
		if rank == 0 {
			bonus = 0.18
		} else if rank == 1 {
			bonus = 0.12
		}
		ranked[segmentScore.Segment.StartLine].score += bonus
	}
}

func stage0RuleRoute(query string) Decision {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return Decision{}
	}
	signals := sel.AnalyzeQuery(query)
	selectSkill := func(name, reason string) Decision {
		return Decision{
			Query:            query,
			SelectedSkill:    name,
			Confidence:       0.95,
			NeedClarify:      false,
			Reason:           reason,
			Stage:            "rule",
			ConfidenceReason: "explicit_rule",
		}
	}

	if strings.HasPrefix(lower, "ask ") || strings.HasPrefix(lower, "blue ask ") {
		return selectSkill("ask", "rule_ask")
	}
	if (strings.Contains(lower, "http://") || strings.Contains(lower, "https://")) && !shouldBypassURLBrowserRule(lower) {
		return selectSkill("browser", "rule_url")
	}
	if (strings.Contains(lower, "http://") || strings.Contains(lower, "https://")) && shouldBypassURLBrowserRule(lower) {
		if shouldRouteURLBypassToUIReviewer(lower) {
			return selectSkill("ui_reviewer", "rule_url_ui_review")
		}
		if shouldRouteURLBypassToDeepResearch(lower) {
			return selectSkill("deep_research", "rule_url_deep_research")
		}
		if shouldRouteURLBypassToAnalyze(lower) {
			return selectSkill("analyze", "rule_url_analyze")
		}
	}
	if signals.LocalWorkspace && !signals.LiveWeb && !signals.Productivity &&
		hasSelectorTerm(lower, []string{
			"workspace", "repo", "repository", "readme", "file", "files", "report", "reports", "document", "documents",
			"analyze", "analysis", "summarize", "summary", "compare", "inspect",
			"工作区", "仓库", "readme", "文件", "报告", "文档", "分析", "总结", "比较", "查看",
		}) {
		return selectSkill("exec", "rule_workspace_local_exec")
	}
	if !signals.LiveWeb && signals.Productivity &&
		hasSelectorTerm(lower, []string{
			"email", "mail", "inbox", "imap", "smtp", "reply", "forward", "attachment",
			"邮件", "邮箱", "收件箱", "回复", "转发", "附件",
		}) {
		return selectSkill("himalaya", "rule_email_cli")
	}
	if strings.Contains(lower, "plan_create") || strings.Contains(lower, "plan_update") || strings.Contains(lower, "plan_append") {
		return selectSkill("plan_create", "rule_plan")
	}
	if strings.HasPrefix(lower, "mgmt") || strings.Contains(lower, "settings.") || strings.Contains(lower, "providers.") {
		return selectSkill("mgmt", "rule_mgmt")
	}
	return Decision{}
}

func shouldBypassURLBrowserRule(lower string) bool {
	if lower == "" {
		return false
	}
	if hasSelectorTerm(lower, uniqueStringTerms(append([]string{
		"ui", "ux", "interface", "layout", "design", "mockup", "wireframe", "component", "visual",
		"accessibility", "a11y", "界面", "布局", "设计", "设计稿", "组件", "视觉", "无障碍", "可访问性",
	}, routingcue.URLBypassTerms()...))) {
		return true
	}
	return hasSelectorTerm(lower, []string{
		"analyze", "analysis", "summarize", "summary", "synthesize", "compare", "report", "insight", "insights", "findings", "extract",
		"research", "investigate", "study", "citations", "citation", "evidence", "sources", "source", "timeline", "tradeoff", "benchmark", "analyze this", "summarize this", "分析", "总结", "提炼", "比较", "报告", "洞察", "研究", "梳理", "评估", "引用", "证据", "来源", "时间线", "权衡", "基准",
	})
}

func shouldRouteURLBypassToDeepResearch(lower string) bool {
	return hasSelectorTerm(lower, uniqueStringTerms(append([]string{
		"research", "investigate", "study", "citations", "citation", "evidence", "sources", "source", "timeline", "tradeoff", "benchmark", "multi-source",
		"research this", "investigate this", "with citations", "with sources", "分析证据", "研究", "调研", "查阅", "引用", "证据", "来源", "时间线", "权衡", "基准", "多来源",
	}, routingcue.URLBypassTermsForSkill("deep_research")...)))
}

func shouldRouteURLBypassToAnalyze(lower string) bool {
	return hasSelectorTerm(lower, uniqueStringTerms(append([]string{
		"analyze", "analysis", "summarize", "summary", "synthesize", "compare", "report", "insight", "insights", "findings", "extract",
		"analyze this", "summarize this", "分析", "总结", "提炼", "比较", "报告", "洞察", "归纳", "评估",
	}, routingcue.URLBypassTermsForSkill("analyze")...)))
}

func shouldRouteURLBypassToUIReviewer(lower string) bool {
	return hasSelectorTerm(lower, uniqueStringTerms(append([]string{
		"ui", "ux", "interface", "layout", "design", "mockup", "wireframe", "component", "visual",
		"accessibility", "a11y", "review", "audit", "inspect", "evaluate", "critique", "score", "rate", "assess",
		"界面", "布局", "设计", "设计稿", "组件", "视觉", "无障碍", "可访问性", "评审", "审查", "检查", "点评", "打分", "评分",
	}, routingcue.URLBypassTermsForSkill("ui_reviewer")...)))
}

func hasSelectorTerm(text string, terms []string) bool {
	for _, term := range terms {
		if sel.ContainsTerm(text, term) {
			return true
		}
	}
	return false
}

func shouldTriggerRerank(query string, d Decision) bool {
	if d.SelectedSkill == "" {
		return false
	}
	if hasHighRiskIntent(strings.ToLower(query)) || d.NeedClarify || len(d.ConflictFlags) > 0 {
		return true
	}
	if len(d.Candidates) >= 2 {
		gap := d.Candidates[0].Score - d.Candidates[1].Score
		if gap < 0.65 {
			return true
		}
	}
	if !hasActionAlignment(query, d.SelectedSkill) {
		return true
	}
	return false
}

func hasActionAlignment(query, skill string) bool {
	q := strings.ToLower(query)
	s := strings.ToLower(skill)
	for _, term := range routingcue.SkillTerms(s).All() {
		if sel.ContainsTerm(q, term) {
			return true
		}
	}
	pairs := map[string][]string{
		"web_query":     {"search", "搜索", "检索", "news", "sources", "citations", "docs", "documentation", "manual", "文档", "官方文档"},
		"web_search":    {"search", "搜索", "检索", "news", "sources", "citations", "docs", "documentation", "manual", "文档", "官方文档"},
		"browser":       {"url", "网页", "open", "navigate", "visit"},
		"exec":          {"workspace", "repo", "repository", "readme", "file", "files", "folder", "directory", "local", "shell", "terminal", "工作区", "仓库", "文件", "目录", "本地", "终端"},
		"ask":           {"ask", "询问", "clarify", "question"},
		"ui_reviewer":   {"ui", "界面", "review", "评审", "screenshot", "design"},
		"deep_research": {"research", "investigate", "citations", "evidence", "sources", "source", "timeline", "benchmark", "tradeoff", "调研", "深入", "查阅", "引用", "证据", "来源", "时间线", "基准", "权衡"},
		"analyze":       {"analyze", "analysis", "summarize", "summary", "report", "url", "urls", "link", "links", "text", "article", "articles", "网页", "链接", "文本", "文章", "分析", "总结", "报告", "文档"},
		"himalaya":      {"email", "mail", "inbox", "imap", "smtp", "reply", "forward", "attachment", "terminal", "邮件", "邮箱", "收件箱", "回复", "转发", "附件"},
		"reminder":      {"remind", "reminder", "notify", "提醒", "通知", "tomorrow", "明天", "明早", "later", "稍后", "时间"},
	}
	if kws, ok := pairs[s]; ok {
		for _, kw := range kws {
			if sel.ContainsTerm(q, kw) {
				return true
			}
		}
	}
	return sel.ContainsTerm(q, s) || sel.ContainsTerm(q, sel.HumanizeName(s))
}

func uniqueStringTerms(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func hasHighRiskIntent(q string) bool {
	return sel.AnalyzeQuery(q).HighRisk
}

func buildCandidatesFromNames(names string, docs []SkillDoc) []SkillCandidate {
	parts := strings.Split(names, ",")
	out := make([]SkillCandidate, 0, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		desc := ""
		for _, d := range docs {
			if strings.EqualFold(d.Name, name) {
				desc = d.Description
				break
			}
		}
		out = append(out, SkillCandidate{Name: name, Score: 1, Description: desc})
	}
	return out
}

func skillDocTextForIR(d SkillDoc) string {
	parts := []string{d.ID, d.Name, d.Description, d.Category, d.Example, d.Invocation, d.InteractionMode, d.CardSupport, d.Setup, d.Body}
	if len(d.Tags) > 0 {
		parts = append(parts, strings.Join(d.Tags, " "))
	}
	if len(d.Examples) > 0 {
		parts = append(parts, strings.Join(d.Examples, " "))
	}
	if len(d.Environment) > 0 {
		parts = append(parts, strings.Join(d.Environment, " "))
	}
	if len(d.ScriptPaths) > 0 {
		parts = append(parts, strings.Join(d.ScriptPaths, " "))
	}
	if len(d.InstallSteps) > 0 {
		parts = append(parts, strings.Join(d.InstallSteps, " "))
	}
	if len(d.UsageSteps) > 0 {
		parts = append(parts, strings.Join(d.UsageSteps, " "))
	}
	for _, r := range d.TaskRoutes {
		parts = append(parts, r.Intent, r.Action)
	}
	for _, er := range d.ErrorRules {
		parts = append(parts, er.Error, er.Resolution)
	}
	return strings.Join(parts, " ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *SkillSelector) buildCacheKey(query, mode string, rerank bool, threshold float64, budget TokenBudget, strict bool) string {
	n := strings.ToLower(strings.TrimSpace(query))
	payload := strings.Join([]string{
		n,
		mode,
		fmt.Sprintf("%t", rerank),
		fmt.Sprintf("%.2f", threshold),
		fmt.Sprintf("%d", budget.MaxTokens),
		fmt.Sprintf("%d", budget.ReservedTokens),
		fmt.Sprintf("%d", budget.PerSkillTokens),
		fmt.Sprintf("%t", strict),
	}, "|")
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

func applyTokenBudgetToDecision(decision Decision, docs []SkillDoc, budget TokenBudget, strict bool) (Decision, error) {
	budget = normalizeTokenBudget(budget)
	if budget.MaxTokens <= 0 {
		return decision, nil
	}

	docByName := make(map[string]SkillDoc, len(docs)*2)
	for _, doc := range docs {
		if key := strings.ToLower(strings.TrimSpace(doc.Name)); key != "" {
			docByName[key] = doc
		}
		if key := strings.ToLower(strings.TrimSpace(doc.ID)); key != "" {
			docByName[key] = doc
		}
	}

	maxTokens := budget.MaxTokens - budget.ReservedTokens
	if maxTokens < 0 {
		maxTokens = 0
	}

	loaded := make([]string, 0, len(PinnedSkills())+len(decision.Candidates))
	skipped := make([]string, 0, len(decision.Candidates))
	seen := make(map[string]struct{}, len(docs))
	skippedSeen := make(map[string]struct{}, len(docs))
	used := 0

	addLoaded := func(name string) {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		loaded = append(loaded, strings.TrimSpace(name))
		if doc, ok := docByName[key]; ok {
			used += estimateSkillDocTokens(doc, budget)
		}
	}

	for _, pinned := range PinnedSkills() {
		if _, ok := docByName[strings.ToLower(strings.TrimSpace(pinned))]; ok {
			addLoaded(pinned)
		}
	}

	queue := make([]string, 0, len(decision.Candidates)+1)
	if strings.TrimSpace(decision.SelectedSkill) != "" {
		queue = append(queue, decision.SelectedSkill)
	}
	for _, candidate := range decision.Candidates {
		queue = append(queue, candidate.Name)
	}
	for _, name := range queue {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		doc, ok := docByName[key]
		if !ok {
			continue
		}
		estimate := estimateSkillDocTokens(doc, budget)
		if maxTokens > 0 && used+estimate > maxTokens {
			if _, seenSkip := skippedSeen[key]; !seenSkip {
				skippedSeen[key] = struct{}{}
				skipped = append(skipped, doc.Name)
			}
			continue
		}
		addLoaded(doc.Name)
	}

	decision.LoadedSkills = loaded
	decision.SkippedSkills = skipped
	decision.TokenBudgetMax = budget.MaxTokens
	decision.TokenBudgetUsed = used
	decision.TokenBudgetTrimmed = len(skipped) > 0
	if strict && len(skipped) > 0 {
		return decision, fmt.Errorf("%w: loaded=%d skipped=%d", ErrTokenBudgetExhausted, len(loaded), len(skipped))
	}
	return decision, nil
}

func normalizeTokenBudget(budget TokenBudget) TokenBudget {
	if budget.PerSkillTokens <= 0 {
		budget.PerSkillTokens = defaultSkillBudgetBaseTokens
	}
	if budget.ReservedTokens < 0 {
		budget.ReservedTokens = 0
	}
	if budget.MaxTokens < 0 {
		budget.MaxTokens = 0
	}
	return budget
}

func estimateSkillDocTokens(doc SkillDoc, budget TokenBudget) int {
	base := budget.PerSkillTokens
	if base <= 0 {
		base = defaultSkillBudgetBaseTokens
	}
	exampleCount := len(doc.Examples)
	if exampleCount == 0 && (strings.TrimSpace(doc.Example) != "" || strings.TrimSpace(doc.Invocation) != "") {
		exampleCount = 1
	}
	return base + (exampleCount * defaultSkillBudgetExampleTokens)
}

func (s *SkillSelector) getCachedDecision(key string) (Decision, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	entry, ok := s.cache[key]
	if !ok || time.Now().After(entry.expires) {
		return Decision{}, false
	}
	return entry.decision, true
}

func (s *SkillSelector) setCachedDecision(key string, d Decision) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[key] = cachedSkillDecision{decision: d, expires: time.Now().Add(s.cacheTTL)}
}

func (s *SkillSelector) loadIndex() ([]SkillDoc, []skillIndexEntry, error) {
	snapshot, err := s.exposure.Snapshot()
	if err != nil {
		return nil, nil, err
	}
	activeViews := make([]skillmanifest.SkillExposureView, 0, len(snapshot.ActiveSkills))
	for _, view := range snapshot.ActiveSkills {
		if !view.ModelInvocable {
			continue
		}
		activeViews = append(activeViews, view)
	}
	stamp := strings.TrimSpace(snapshot.Stamp)

	s.mu.RLock()
	if s.indexLoaded && s.indexStamp == stamp {
		out := make([]SkillDoc, len(s.indexDocs))
		copy(out, s.indexDocs)
		cache := make([]skillIndexEntry, len(s.indexCache))
		copy(cache, s.indexCache)
		s.mu.RUnlock()
		return out, cache, nil
	}
	s.mu.RUnlock()

	docs := BuildSkillIndexFromViews(activeViews)
	indexCache := make([]skillIndexEntry, 0, len(docs))
	for _, doc := range docs {
		indexCache = append(indexCache, skillIndexEntry{
			doc:    doc,
			bundle: buildSkillSelectorBundle(doc),
		})
	}

	s.mu.Lock()
	cacheInvalidations := s.cacheInvalidationCount
	if s.indexStamp != "" && s.indexStamp != stamp {
		cacheInvalidations++
		s.cacheInvalidationCount = cacheInvalidations
		s.invalidateDecisionCache()
	}
	s.indexLoaded = true
	s.indexStamp = stamp
	s.indexDocs = make([]SkillDoc, len(docs))
	copy(s.indexDocs, docs)
	s.indexCache = make([]skillIndexEntry, len(indexCache))
	copy(s.indexCache, indexCache)
	s.debugState = buildSkillSelectorDebugState(snapshot, cacheInvalidations)
	s.mu.Unlock()

	out := make([]SkillDoc, len(docs))
	copy(out, docs)
	cache := make([]skillIndexEntry, len(indexCache))
	copy(cache, indexCache)
	return out, cache, nil
}

func (s *SkillSelector) invalidateDecisionCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache = make(map[string]cachedSkillDecision)
}

func buildSkillSelectorDebugState(snapshot skillmanifest.SkillExposureSnapshot, cacheInvalidations uint64) SkillSelectorDebugState {
	state := SkillSelectorDebugState{
		ActiveSkillCount:           snapshot.ActiveSkillCount,
		DormantSkillCount:          snapshot.DormantSkillCount,
		CacheInvalidationCount:     cacheInvalidations,
		DiscoveredDirs:             append([]string(nil), snapshot.DiscoveredDirs...),
		ActivatedConditionalSkills: append([]string(nil), snapshot.ActivatedConditionalSkills...),
		Skills:                     make(map[string]SkillSelectorSkillRuntimeView, len(snapshot.VisibleSkills)*2),
	}

	for _, skillView := range snapshot.VisibleSkills {
		view := SkillSelectorSkillRuntimeView{
			ID:               strings.TrimSpace(skillView.Document.ID),
			Name:             strings.TrimSpace(firstNonBlank(skillView.Document.Name, skillView.Document.ID)),
			Paths:            append([]string(nil), skillView.Paths...),
			UserInvocable:    skillView.UserInvocable,
			ModelInvocable:   skillView.ModelInvocable,
			ActivationState:  strings.TrimSpace(skillView.ActivationState),
			ActivationSource: strings.TrimSpace(skillView.ActivationSource),
		}
		if key := strings.ToLower(strings.TrimSpace(view.Name)); key != "" {
			state.Skills[key] = view
		}
		if key := strings.ToLower(strings.TrimSpace(view.ID)); key != "" {
			state.Skills[key] = view
		}
	}
	return state
}

func cloneSkillSelectorDebugState(state SkillSelectorDebugState) SkillSelectorDebugState {
	out := SkillSelectorDebugState{
		ActiveSkillCount:           state.ActiveSkillCount,
		DormantSkillCount:          state.DormantSkillCount,
		CacheInvalidationCount:     state.CacheInvalidationCount,
		DiscoveredDirs:             append([]string(nil), state.DiscoveredDirs...),
		ActivatedConditionalSkills: append([]string(nil), state.ActivatedConditionalSkills...),
	}
	if len(state.Skills) == 0 {
		return out
	}
	out.Skills = make(map[string]SkillSelectorSkillRuntimeView, len(state.Skills))
	for key, value := range state.Skills {
		out.Skills[key] = cloneSkillRuntimeView(value)
	}
	return out
}

func cloneSkillRuntimeView(view SkillSelectorSkillRuntimeView) SkillSelectorSkillRuntimeView {
	view.Paths = append([]string(nil), view.Paths...)
	return view
}
