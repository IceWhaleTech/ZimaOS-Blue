package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	sel "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
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
)

// SelectOptions controls the progressive selector behavior.
type SelectOptions struct {
	Mode                string
	EnableRerank        bool
	ConfidenceThreshold float64
}

// SkillCandidate is one ranked candidate.
type SkillCandidate struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Description string  `json:"description,omitempty"`
}

// Decision is the selector output.
type Decision struct {
	Query            string           `json:"query"`
	SelectedSkill    string           `json:"selected_skill"`
	Confidence       float64          `json:"confidence"`
	NeedClarify      bool             `json:"need_clarify"`
	Reason           string           `json:"reason"`
	Stage            string           `json:"stage"`
	Candidates       []SkillCandidate `json:"candidates,omitempty"`
	MatchedSignals   []string         `json:"matched_signals,omitempty"`
	ConflictFlags    []string         `json:"conflict_flags,omitempty"`
	ConfidenceReason string           `json:"confidence_reason,omitempty"`
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

	mu         sync.RWMutex
	indexStamp string
	indexDocs  []SkillDoc
	indexCache []skillIndexEntry

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
		cache:        make(map[string]cachedSkillDecision),
		cacheTTL:     defaultSkillSelectCacheTTL,
	}
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

	cacheKey := s.buildCacheKey(query, mode, opts.EnableRerank, thres)
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
		s.setCachedDecision(cacheKey, rule)
		return rule, nil
	}

	irDecision := s.stage1IR(query, indexCache, thres)
	if mode == SkillSelectorModeIROnly {
		s.setCachedDecision(cacheKey, irDecision)
		return irDecision, nil
	}

	if mode == SkillSelectorModeHybrid && !shouldTriggerRerank(query, irDecision) {
		s.setCachedDecision(cacheKey, irDecision)
		return irDecision, nil
	}
	if !opts.EnableRerank || s.reranker == nil {
		s.setCachedDecision(cacheKey, irDecision)
		return irDecision, nil
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
		s.setCachedDecision(cacheKey, irDecision)
		return irDecision, nil
	}

	rr, err := s.reranker.Rerank(ctx, query, rankedDocs)
	if err != nil {
		s.setCachedDecision(cacheKey, irDecision)
		return irDecision, nil
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
	out.NeedClarify = rr.NeedClarify || out.Confidence < thres || out.Confidence < skillSelectorNeedClarifyFloor

	s.setCachedDecision(cacheKey, out)
	return out, nil
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
	if strings.EqualFold(strings.TrimSpace(doc.Name), "browser") && signals.URLPresent {
		match.Eligible = true
		match.Anchored = true
		if match.Score < 4.8 {
			match.Score = 4.8
		}
		match.ConfidenceReason = "url_present_rule"
		match.ConflictFlags = nil
		match.MatchedSignals = append(match.MatchedSignals, "rule:url_present")
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
	if hasSelectorTerm(lower, []string{
		"ui", "ux", "interface", "layout", "design", "mockup", "wireframe", "component", "visual",
		"accessibility", "a11y", "界面", "布局", "设计", "设计稿", "组件", "视觉", "无障碍", "可访问性",
	}) {
		return true
	}
	return hasSelectorTerm(lower, []string{
		"analyze", "analysis", "summarize", "summary", "synthesize", "compare", "report", "insight", "insights", "findings", "extract",
		"research", "investigate", "study", "analyze this", "summarize this", "分析", "总结", "提炼", "比较", "报告", "洞察", "研究", "梳理", "评估",
	})
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
	pairs := map[string][]string{
		"web_search":    {"search", "搜索", "检索", "news", "sources", "citations"},
		"browser":       {"url", "网页", "open", "navigate", "visit"},
		"ask":           {"ask", "询问", "clarify", "question"},
		"ui_reviewer":   {"ui", "界面", "review", "评审", "screenshot", "design"},
		"deep_research": {"research", "调研", "深入", "查阅", "观点", "timeline", "sources"},
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
	parts := []string{d.Name, d.Description, d.Category, d.Example, d.Setup, d.Body}
	if len(d.Tags) > 0 {
		parts = append(parts, strings.Join(d.Tags, " "))
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

func (s *SkillSelector) buildCacheKey(query, mode string, rerank bool, threshold float64) string {
	n := strings.ToLower(strings.TrimSpace(query))
	h := sha256.Sum256([]byte(n + "|" + mode + "|" + fmt.Sprintf("%t", rerank) + "|" + fmt.Sprintf("%.2f", threshold)))
	return hex.EncodeToString(h[:])
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
	stamp, err := buildSkillRootsStamp(s.workspaceDir)
	if err != nil {
		return nil, nil, err
	}

	s.mu.RLock()
	if s.indexStamp == stamp && len(s.indexDocs) > 0 {
		out := make([]SkillDoc, len(s.indexDocs))
		copy(out, s.indexDocs)
		cache := make([]skillIndexEntry, len(s.indexCache))
		copy(cache, s.indexCache)
		s.mu.RUnlock()
		return out, cache, nil
	}
	s.mu.RUnlock()

	docs, err := BuildSkillIndex(s.workspaceDir)
	if err != nil {
		return nil, nil, err
	}
	indexCache := make([]skillIndexEntry, 0, len(docs))
	for _, doc := range docs {
		indexCache = append(indexCache, skillIndexEntry{
			doc:    doc,
			bundle: buildSkillSelectorBundle(doc),
		})
	}

	s.mu.Lock()
	s.indexStamp = stamp
	s.indexDocs = make([]SkillDoc, len(docs))
	copy(s.indexDocs, docs)
	s.indexCache = make([]skillIndexEntry, len(indexCache))
	copy(s.indexCache, indexCache)
	s.mu.Unlock()

	out := make([]SkillDoc, len(docs))
	copy(out, docs)
	cache := make([]skillIndexEntry, len(indexCache))
	copy(cache, indexCache)
	return out, cache, nil
}

func buildSkillRootsStamp(workspaceDir string) (string, error) {
	roots := resolveSkillRoots(workspaceDir)
	parts := make([]string, 0, 128)
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			md := filepath.Join(root, e.Name(), "SKILL.md")
			st, err := os.Stat(md)
			if err != nil {
				continue
			}
			parts = append(parts, md+"|"+fmt.Sprintf("%d|%d", st.Size(), st.ModTime().UnixNano()))
		}
	}
	sort.Strings(parts)
	h := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(h[:]), nil
}
