package claudecode

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	sel "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
)

// SkillReranker re-ranks top candidates when IR confidence is ambiguous.
type SkillReranker interface {
	Rerank(ctx context.Context, query string, cands []SkillDoc) (RerankResult, error)
}

// RerankResult is the strict output contract for stage-2 reranking.
type RerankResult struct {
	SelectedSkill string  `json:"selected_skill"`
	Confidence    float64 `json:"confidence"`
	ReasonShort   string  `json:"reason_short"`
	NeedClarify   bool    `json:"need_clarify"`
}

// HeuristicSkillReranker is a fallback reranker that emulates a compact LLM
// decision protocol with deterministic lexical scoring.
type HeuristicSkillReranker struct{}

func NewHeuristicSkillReranker() *HeuristicSkillReranker { return &HeuristicSkillReranker{} }

func (r *HeuristicSkillReranker) Rerank(_ context.Context, query string, cands []SkillDoc) (RerankResult, error) {
	if len(cands) == 0 {
		return RerankResult{}, errors.New("no candidates")
	}
	queryLower := strings.ToLower(strings.TrimSpace(query))
	if queryLower == "" {
		return RerankResult{}, errors.New("empty query")
	}

	bestIdx := 0
	bestScore := -1.0
	second := -1.0
	for i, c := range cands {
		s := rerankScore(queryLower, c)
		if s > bestScore {
			second = bestScore
			bestScore = s
			bestIdx = i
		} else if s > second {
			second = s
		}
	}

	gap := bestScore - second
	conf := 0.55 + minFloat(0.4, bestScore*0.25+gap*0.2)
	needClarify := conf < 0.78
	if conf < 0.55 {
		needClarify = true
	}
	if hasHighRiskIntent(queryLower) {
		needClarify = true
		if conf > 0.72 {
			conf = 0.72
		}
	}

	return RerankResult{
		SelectedSkill: cands[bestIdx].Name,
		Confidence:    clampFloat(conf, 0, 1),
		ReasonShort:   "reranked_by_compact_model",
		NeedClarify:   needClarify,
	}, nil
}

func rerankScore(query string, c SkillDoc) float64 {
	score := 0.0
	name := strings.ToLower(c.Name)
	desc := strings.ToLower(c.Description)
	example := strings.ToLower(c.Example)
	if name != "" && sel.ContainsTerm(query, name) {
		score += 2.8
	}
	for _, t := range c.Tags {
		tl := strings.ToLower(strings.TrimSpace(t))
		if tl != "" && sel.ContainsTerm(query, tl) {
			score += 0.7
		}
	}
	for _, tok := range strings.Fields(query) {
		if len(tok) < 2 {
			continue
		}
		if sel.ContainsTerm(desc, tok) {
			score += 0.25
		}
		if sel.ContainsTerm(example, tok) {
			score += 0.2
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// AutoSkillReranker attempts to prepare a real ONNX reranker model locally and
// gracefully falls back to heuristic reranking when unavailable.
type AutoSkillReranker struct {
	fallback     SkillReranker
	modelManager *SkillRerankerModelManager
	onnxEnabled  func() bool
	autoDownload func() bool
}

type AutoSkillRerankerOptions struct {
	ONNXEnabled  bool
	AutoDownload bool
}

func NewAutoSkillReranker(dataDir, modelRepo string, opts AutoSkillRerankerOptions) *AutoSkillReranker {
	return &AutoSkillReranker{
		fallback:     NewHeuristicSkillReranker(),
		modelManager: NewSkillRerankerModelManager(dataDir, modelRepo),
		onnxEnabled: func() bool {
			return opts.ONNXEnabled
		},
		autoDownload: func() bool {
			return opts.AutoDownload
		},
	}
}

func (r *AutoSkillReranker) SetSwitchFuncs(onnxEnabledFn, autoDownloadFn func() bool) {
	if onnxEnabledFn != nil {
		r.onnxEnabled = onnxEnabledFn
	}
	if autoDownloadFn != nil {
		r.autoDownload = autoDownloadFn
	}
}

func (r *AutoSkillReranker) isONNXEnabled() bool {
	if r.onnxEnabled == nil {
		return false
	}
	return r.onnxEnabled()
}

func (r *AutoSkillReranker) isAutoDownloadEnabled() bool {
	if r.autoDownload == nil {
		return false
	}
	return r.autoDownload()
}

func (r *AutoSkillReranker) Rerank(ctx context.Context, query string, cands []SkillDoc) (RerankResult, error) {
	// ONNX model preparation is switch-controlled. If disabled/not-ready, we keep
	// heuristic fallback to avoid blocking selection.
	if r.modelManager != nil && r.isONNXEnabled() {
		autoDownload := r.isAutoDownloadEnabled()
		if err := r.modelManager.EnsureReady(ctx, autoDownload); err != nil {
			slog.Warn("[skill-reranker] onnx model not ready, fallback to heuristic",
				"error", err,
				"onnx_enabled", true,
				"auto_download", autoDownload)
		}
	}
	return r.fallback.Rerank(ctx, query, cands)
}

func (r *AutoSkillReranker) WarmupAsync() {
	if r.modelManager != nil && r.isONNXEnabled() {
		r.modelManager.WarmupAsync(r.isAutoDownloadEnabled())
	}
}

// ModelManager returns the underlying ONNX model manager.
func (r *AutoSkillReranker) ModelManager() *SkillRerankerModelManager {
	return r.modelManager
}
