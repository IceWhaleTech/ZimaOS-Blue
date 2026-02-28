package claudecode

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
	Query         string           `json:"query"`
	SelectedSkill string           `json:"selected_skill"`
	Confidence    float64          `json:"confidence"`
	NeedClarify   bool             `json:"need_clarify"`
	Reason        string           `json:"reason"`
	Stage         string           `json:"stage"`
	Candidates    []SkillCandidate `json:"candidates,omitempty"`
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

	docs, err := s.loadIndex()
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

	irDecision := s.stage1IR(query, docs, thres)
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

func (s *SkillSelector) stage1IR(query string, docs []SkillDoc, threshold float64) Decision {
	queryTokens := pruner.TextTokenize(query)
	segments := make([]pruner.Segment, 0, len(docs))
	for i, doc := range docs {
		content := skillDocTextForIR(doc)
		segments = append(segments, pruner.Segment{
			Content:   content,
			Tokens:    pruner.TextTokenize(content),
			StartLine: i,
			EndLine:   i,
		})
	}

	scorer := pruner.NewBM25Scorer(1.2, 0.75)
	scored := scorer.Score(query, segments)

	type cand struct {
		doc      SkillDoc
		score    float64
		exactHit bool
		coverage float64
	}
	cands := make([]cand, 0, len(scored))
	for _, ss := range scored {
		idx := ss.Segment.StartLine
		if idx < 0 || idx >= len(docs) {
			continue
		}
		d := docs[idx]
		docText := strings.ToLower(skillDocTextForIR(d))
		matched := 0
		for _, t := range queryTokens {
			if strings.Contains(docText, t) {
				matched++
			}
		}
		cov := 0.0
		if len(queryTokens) > 0 {
			cov = float64(matched) / float64(len(queryTokens))
		}
		exact := strings.Contains(strings.ToLower(query), strings.ToLower(d.Name))
		boost := 0.0
		if exact {
			boost += 1.5
		}
		if hit := skillAliasKeywordHit(query, d.Name); hit > 0 {
			boost += float64(hit) * 0.3
		}
		cands = append(cands, cand{doc: d, score: ss.Score + boost, exactHit: exact, coverage: cov})
	}

	if len(cands) == 0 {
		return Decision{Query: query, NeedClarify: true, Reason: "ir_no_match", Stage: "ir"}
	}

	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	if len(cands) > defaultSkillTopK {
		cands = cands[:defaultSkillTopK]
	}
	top1 := cands[0].score
	top2 := 0.0
	if len(cands) > 1 {
		top2 = cands[1].score
	}
	scoreGap := top1 - top2
	conf := 0.48 + minFloat(0.2, scoreGap*0.16) + minFloat(0.2, cands[0].coverage*0.2)
	if cands[0].exactHit {
		conf += 0.15
	}
	if conf > 1 {
		conf = 1
	}
	needClarify := conf < threshold || conf < skillSelectorNeedClarifyFloor

	result := Decision{
		Query:         query,
		SelectedSkill: cands[0].doc.Name,
		Confidence:    conf,
		NeedClarify:   needClarify,
		Reason:        "ir_ranked",
		Stage:         "ir",
		Candidates:    make([]SkillCandidate, 0, minInt(3, len(cands))),
	}
	for i := 0; i < len(cands) && i < 3; i++ {
		result.Candidates = append(result.Candidates, SkillCandidate{
			Name:        cands[i].doc.Name,
			Score:       cands[i].score,
			Description: cands[i].doc.Description,
		})
	}
	return result
}

func stage0RuleRoute(query string) Decision {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return Decision{}
	}
	selectSkill := func(name, reason string) Decision {
		return Decision{
			Query:         query,
			SelectedSkill: name,
			Confidence:    0.95,
			NeedClarify:   false,
			Reason:        reason,
			Stage:         "rule",
		}
	}

	if strings.HasPrefix(lower, "ask ") || strings.Contains(lower, "澄清") || strings.Contains(lower, "confirm") {
		return selectSkill("ask", "rule_ask")
	}
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "url") || strings.Contains(lower, "网页") {
		return selectSkill("browser", "rule_url")
	}
	if strings.Contains(lower, "搜索") || strings.Contains(lower, "search") || strings.Contains(lower, "news") || strings.Contains(lower, "检索") {
		return selectSkill("web_search", "rule_search")
	}
	if strings.Contains(lower, "ui") || strings.Contains(lower, "界面") || strings.Contains(lower, "review") || strings.Contains(lower, "评审") {
		return selectSkill("ui_reviewer", "rule_ui")
	}
	if strings.Contains(lower, "research") || strings.Contains(lower, "调研") || strings.Contains(lower, "深入") {
		return selectSkill("deep_research", "rule_research")
	}
	if strings.Contains(lower, "计划") || strings.Contains(lower, "todo") || strings.Contains(lower, "checklist") || strings.Contains(lower, "plan") {
		return selectSkill("plan_create", "rule_plan")
	}
	if strings.HasPrefix(lower, "mgmt") || strings.Contains(lower, "settings.") || strings.Contains(lower, "providers.") {
		return selectSkill("mgmt", "rule_mgmt")
	}
	return Decision{}
}

func shouldTriggerRerank(query string, d Decision) bool {
	if d.SelectedSkill == "" {
		return true
	}
	if hasHighRiskIntent(strings.ToLower(query)) {
		return true
	}
	if len(d.Candidates) >= 2 {
		gap := d.Candidates[0].Score - d.Candidates[1].Score
		if gap < 0.18 {
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
		"web_search":    {"search", "搜索", "检索", "news", "查询"},
		"browser":       {"url", "网页", "open", "navigate"},
		"ask":           {"ask", "询问", "确认", "clarify"},
		"ui_reviewer":   {"ui", "界面", "review", "评审"},
		"deep_research": {"research", "调研", "深入"},
	}
	if kws, ok := pairs[s]; ok {
		for _, kw := range kws {
			if strings.Contains(q, kw) {
				return true
			}
		}
	}
	return strings.Contains(q, s)
}

func hasHighRiskIntent(q string) bool {
	riskWords := []string{"delete", "remove", "drop", "overwrite", "reset", "生产", "线上", "删", "覆盖", "卸载"}
	for _, w := range riskWords {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}

func skillAliasKeywordHit(query, skillName string) int {
	aliases := map[string][]string{
		"web_search":    {"search", "搜索", "检索", "news"},
		"browser":       {"browser", "url", "网页", "navigate", "打开"},
		"ask":           {"ask", "clarify", "确认", "询问"},
		"analyze":       {"analyze", "分析"},
		"ui_reviewer":   {"ui", "review", "评审", "界面"},
		"deep_research": {"deep", "research", "深入", "调研"},
		"plan_create":   {"plan", "todo", "计划", "清单"},
		"mgmt":          {"admin", "管理", "settings", "providers"},
	}
	lower := strings.ToLower(query)
	hits := 0
	for _, kw := range aliases[strings.ToLower(skillName)] {
		if strings.Contains(lower, kw) {
			hits++
		}
	}
	return hits
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

func (s *SkillSelector) loadIndex() ([]SkillDoc, error) {
	stamp, err := buildSkillRootsStamp(s.workspaceDir)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	if s.indexStamp == stamp && len(s.indexDocs) > 0 {
		out := make([]SkillDoc, len(s.indexDocs))
		copy(out, s.indexDocs)
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()

	docs, err := BuildSkillIndex(s.workspaceDir)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.indexStamp = stamp
	s.indexDocs = make([]SkillDoc, len(docs))
	copy(s.indexDocs, docs)
	s.mu.Unlock()

	out := make([]SkillDoc, len(docs))
	copy(out, docs)
	return out, nil
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
