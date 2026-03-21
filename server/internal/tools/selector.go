package tools

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	sel "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
)

// ToolSelectorStats holds cumulative statistics for smart tool selection.
type ToolSelectorStats struct {
	Requests     int64 `json:"requests"`
	ToolsTotal   int64 `json:"tools_total"`
	ToolsSent    int64 `json:"tools_sent"`
	ToolsSkipped int64 `json:"tools_skipped"`
	TokensSaved  int64 `json:"tokens_saved"`
}

// ToolSelectionCandidateDebug captures selector signals for one candidate.
type ToolSelectionCandidateDebug struct {
	Name             string   `json:"name"`
	Score            float64  `json:"score"`
	MatchedSignals   []string `json:"matched_signals,omitempty"`
	ConflictFlags    []string `json:"conflict_flags,omitempty"`
	ConfidenceReason string   `json:"confidence_reason,omitempty"`
}

// ToolSelectionDebug is returned by dry-run APIs for selector inspection.
type ToolSelectionDebug struct {
	QuerySignals selectorSignals               `json:"query_signals"`
	GatingFlags  []string                      `json:"gating_flags,omitempty"`
	Candidates   []ToolSelectionCandidateDebug `json:"candidates,omitempty"`
}

// ToolSelectionResult returns selected tools plus optional debug details.
type ToolSelectionResult struct {
	Selected []ToolDefinition   `json:"selected"`
	Debug    ToolSelectionDebug `json:"debug"`
}

type toolSelectorCandidate struct {
	def     ToolDefinition
	match   sel.MatchResult
	score   float64
	docText string
}

type selectorSignals struct {
	QuestionPrefix bool `json:"question_prefix,omitempty"`
	HowToQuestion  bool `json:"definition_or_howto,omitempty"`
	PlainReply     bool `json:"plain_reply,omitempty"`
	Negated        bool `json:"negated,omitempty"`
	Smalltalk      bool `json:"smalltalk,omitempty"`
	LocalWorkspace bool `json:"local_workspace,omitempty"`
	LiveWeb        bool `json:"live_web,omitempty"`
	Productivity   bool `json:"productivity,omitempty"`
	UIArtifact     bool `json:"ui_artifact,omitempty"`
	URLPresent     bool `json:"url_present,omitempty"`
	HighRisk       bool `json:"high_risk,omitempty"`
}

func fromSharedSignals(signals sel.QueryIntentSignals) selectorSignals {
	return selectorSignals{
		QuestionPrefix: signals.QuestionPrefix,
		HowToQuestion:  signals.HowToQuestion,
		PlainReply:     signals.PlainReply,
		Negated:        signals.Negated,
		Smalltalk:      signals.Smalltalk,
		LocalWorkspace: signals.LocalWorkspace,
		LiveWeb:        signals.LiveWeb,
		Productivity:   signals.Productivity,
		UIArtifact:     signals.UIArtifact,
		URLPresent:     signals.URLPresent,
		HighRisk:       signals.HighRisk,
	}
}

// ToolSelector performs heuristic tool selection, filtering tool definitions to
// only those relevant to the user's query.
type ToolSelector struct {
	MinScore      float64
	MaxTools      int
	AlwaysInclude []string

	bundleCache  sync.Map
	requests     int64
	toolsTotal   int64
	toolsSent    int64
	toolsSkipped int64
	tokensSaved  int64
}

// DefaultToolSelector returns a ToolSelector with conservative defaults.
func DefaultToolSelector() *ToolSelector {
	return &ToolSelector{
		MinScore:      1.15,
		MaxTools:      10,
		AlwaysInclude: []string{"exec", "ask"},
	}
}

// Stats returns a snapshot of cumulative selection statistics.
func (ts *ToolSelector) Stats() ToolSelectorStats {
	return ToolSelectorStats{
		Requests:     atomic.LoadInt64(&ts.requests),
		ToolsTotal:   atomic.LoadInt64(&ts.toolsTotal),
		ToolsSent:    atomic.LoadInt64(&ts.toolsSent),
		ToolsSkipped: atomic.LoadInt64(&ts.toolsSkipped),
		TokensSaved:  atomic.LoadInt64(&ts.tokensSaved),
	}
}

// Select filters tool definitions to those relevant to the user query.
func (ts *ToolSelector) Select(query string, allDefs []ToolDefinition) []ToolDefinition {
	return ts.SelectDetailed(query, allDefs).Selected
}

// SelectDetailed returns selected definitions plus debugging details.
func (ts *ToolSelector) SelectDetailed(query string, allDefs []ToolDefinition) ToolSelectionResult {
	out := ToolSelectionResult{
		Debug: ToolSelectionDebug{},
	}
	if len(allDefs) == 0 {
		ts.recordSelectionStats(allDefs, nil)
		return out
	}
	if strings.TrimSpace(query) == "" {
		out.Selected = append([]ToolDefinition(nil), allDefs...)
		ts.recordSelectionStats(allDefs, out.Selected)
		return out
	}

	signals := sel.AnalyzeQuery(query)
	out.Debug.QuerySignals = fromSharedSignals(signals)
	out.Debug.GatingFlags = signals.GatingFlags()

	if shouldSuppressToolSelection(signals) {
		ts.recordSelectionStats(allDefs, nil)
		return out
	}

	alwaysSet := make(map[string]bool, len(ts.AlwaysInclude))
	for _, name := range ts.AlwaysInclude {
		alwaysSet[strings.ToLower(strings.TrimSpace(name))] = true
	}

	minScore := ts.MinScore
	if minScore <= 0 {
		minScore = 1.15
	}
	maxTools := ts.MaxTools
	if maxTools <= 0 {
		maxTools = 10
	}

	candidates := make([]toolSelectorCandidate, 0, len(allDefs))
	for _, def := range allDefs {
		bundle := ts.cachedToolSelectorBundle(def)
		match := sel.MatchProfile(signals, bundle.profile)
		applyToolHardAnchors(query, signals, def, &match)
		if !match.Eligible {
			continue
		}
		candidates = append(candidates, toolSelectorCandidate{
			def:     def,
			match:   match,
			score:   match.Score,
			docText: bundle.docText,
		})
	}

	applyToolBM25TieBreak(query, candidates)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].def.Name < candidates[j].def.Name
		}
		return candidates[i].score > candidates[j].score
	})

	out.Debug.Candidates = make([]ToolSelectionCandidateDebug, 0, minInt(len(candidates), 6))
	for i := 0; i < len(candidates) && i < 6; i++ {
		out.Debug.Candidates = append(out.Debug.Candidates, ToolSelectionCandidateDebug{
			Name:             candidates[i].def.Name,
			Score:            candidates[i].score,
			MatchedSignals:   append([]string(nil), candidates[i].match.MatchedSignals...),
			ConflictFlags:    append([]string(nil), candidates[i].match.ConflictFlags...),
			ConfidenceReason: candidates[i].match.ConfidenceReason,
		})
	}

	results := make([]ToolDefinition, 0, minInt(maxTools, len(allDefs)))
	included := make(map[string]bool, len(allDefs))
	for _, def := range allDefs {
		if !alwaysSet[strings.ToLower(strings.TrimSpace(def.Name))] {
			continue
		}
		results = append(results, def)
		included[def.Name] = true
		if len(results) >= maxTools {
			out.Selected = results
			ts.recordSelectionStats(allDefs, out.Selected)
			return out
		}
	}

	for _, cand := range candidates {
		if len(results) >= maxTools {
			break
		}
		if included[cand.def.Name] || cand.score < minScore {
			continue
		}
		results = append(results, cand.def)
		included[cand.def.Name] = true
	}

	out.Selected = results
	ts.recordSelectionStats(allDefs, out.Selected)
	return out
}

// estimateToolTokens estimates the token count for a tool definition.
func estimateToolTokens(def ToolDefinition) int {
	words := len(strings.Fields(def.Name + " " + def.Description))
	tokens := int(float64(words) * 1.3)
	if tokens < 10 {
		tokens = 10
	}
	if len(def.Parameters) > 0 {
		b, _ := json.Marshal(def.Parameters)
		tokens += len(b) / 4
	}
	return tokens
}

func applyToolHardAnchors(query string, signals sel.QueryIntentSignals, def ToolDefinition, match *sel.MatchResult) {
	if match == nil {
		return
	}
	name := strings.ToLower(strings.TrimSpace(def.Name))
	if looksLikeStructuredWorkspaceArtifactTask(query, signals) {
		allowed := make(map[string]struct{}, 8)
		for _, toolName := range StructuredWorkspaceArtifactWorkflowToolNames(query) {
			allowed[toolName] = struct{}{}
		}
		if _, ok := allowed[name]; ok {
			match.Eligible = true
			match.Anchored = true
			if match.Score < 4.8 {
				match.Score = 4.8
			}
			match.ConfidenceReason = "structured_workspace_artifact"
			match.ConflictFlags = nil
			match.MatchedSignals = append(match.MatchedSignals, "rule:structured_workspace_artifact")
			if !containsString(match.DomainHits, sel.DomainLocalWorkspace) {
				match.DomainHits = append(match.DomainHits, sel.DomainLocalWorkspace)
			}
			sort.Strings(match.MatchedSignals)
			return
		}
		switch name {
		case "file_delete", "edit", "grep", "pdf", "image":
			match.Eligible = false
			match.Score = 0
			match.Anchored = false
			match.ConfidenceReason = "structured_workspace_artifact_pruned"
			return
		}
	}
	if signals.LocalWorkspace {
		switch name {
		case "file_read", "file_write", "file_delete", "edit", "ls", "find", "grep", "convert", "pdf":
			match.Eligible = true
			match.Anchored = true
			if match.Score < 4.2 {
				match.Score = 4.2
			}
			match.ConfidenceReason = "local_workspace_file_workflow"
			match.ConflictFlags = nil
			match.MatchedSignals = append(match.MatchedSignals, "rule:local_workspace_file_workflow")
			if !containsString(match.DomainHits, sel.DomainLocalWorkspace) {
				match.DomainHits = append(match.DomainHits, sel.DomainLocalWorkspace)
			}
			sort.Strings(match.MatchedSignals)
		}
	}
	if name == "browser" && signals.URLPresent {
		match.Eligible = true
		match.Anchored = true
		if match.Score < 4.8 {
			match.Score = 4.8
		}
		match.ConfidenceReason = "url_present_rule"
		match.ConflictFlags = nil
		match.MatchedSignals = append(match.MatchedSignals, "rule:url_present")
		if !containsString(match.DomainHits, sel.DomainLiveWeb) {
			match.DomainHits = append(match.DomainHits, sel.DomainLiveWeb)
		}
		if !containsString(match.DomainHits, sel.DomainURLPresent) {
			match.DomainHits = append(match.DomainHits, sel.DomainURLPresent)
		}
		sort.Strings(match.MatchedSignals)
	}
}

func applyToolBM25TieBreak(query string, candidates []toolSelectorCandidate) {
	if len(candidates) < 2 {
		return
	}
	scorer := pruner.NewBM25Scorer(1.2, 0.75)
	segments := make([]pruner.Segment, 0, len(candidates))
	for i, cand := range candidates {
		segments = append(segments, pruner.Segment{
			Content:   cand.docText,
			Tokens:    pruner.TextTokenize(cand.docText),
			StartLine: i,
			EndLine:   i,
		})
	}
	scored := scorer.Score(query, segments)
	for rank, segmentScore := range scored {
		if segmentScore.Segment.StartLine < 0 || segmentScore.Segment.StartLine >= len(candidates) {
			continue
		}
		bonus := 0.06
		if rank == 0 {
			bonus = 0.18
		} else if rank == 1 {
			bonus = 0.12
		}
		candidates[segmentScore.Segment.StartLine].score += bonus
	}
}

func (ts *ToolSelector) recordSelectionStats(allDefs, selected []ToolDefinition) {
	total := int64(len(allDefs))
	sent := int64(len(selected))
	skipped := total - sent
	if skipped < 0 {
		skipped = 0
	}
	atomic.AddInt64(&ts.requests, 1)
	atomic.AddInt64(&ts.toolsTotal, total)
	atomic.AddInt64(&ts.toolsSent, sent)
	atomic.AddInt64(&ts.toolsSkipped, skipped)

	included := make(map[string]struct{}, len(selected))
	for _, def := range selected {
		included[def.Name] = struct{}{}
	}

	var savedTokens int64
	for _, def := range allDefs {
		if _, ok := included[def.Name]; ok {
			continue
		}
		savedTokens += int64(estimateToolTokens(def))
	}
	atomic.AddInt64(&ts.tokensSaved, savedTokens)
}

func shouldSuppressToolSelection(signals sel.QueryIntentSignals) bool {
	if signals.PlainReply || signals.Smalltalk || signals.Negated {
		return true
	}
	return signals.HowToQuestion && signals.MetaIntent
}

// LooksLikeWorkspaceFileTask reports whether the user is asking the model to
// work from files already provided in the workspace.
func LooksLikeWorkspaceFileTask(query string) bool {
	signals := sel.AnalyzeQuery(query)
	return signals.LocalWorkspace
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func (ts *ToolSelector) cachedToolSelectorBundle(def ToolDefinition) toolSelectorBundle {
	key := toolSelectorBundleCacheKey(def)
	if cached, ok := ts.bundleCache.Load(key); ok {
		return cached.(toolSelectorBundle)
	}
	bundle := buildToolSelectorBundle(def)
	actual, _ := ts.bundleCache.LoadOrStore(key, bundle)
	return actual.(toolSelectorBundle)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SelectFromRegistry is a convenience method that gets definitions from a registry
// and filters them based on the query.
func (ts *ToolSelector) SelectFromRegistry(query string, registry *Registry) []ToolDefinition {
	return ts.Select(query, registry.Definitions())
}
