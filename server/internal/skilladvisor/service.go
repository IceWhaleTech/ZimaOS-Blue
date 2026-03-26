package skilladvisor

import (
	"context"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

const (
	defaultInstalledSufficientConfidence = 0.82
	defaultSearchPageSize                = 8
	defaultMaxMergedResults              = 10
)

type Advice struct {
	Query             string                     `json:"query"`
	InstalledDecision *agentcore.Decision        `json:"installed_decision,omitempty"`
	NeedStoreSearch   bool                       `json:"need_store_search"`
	Reason            string                     `json:"reason,omitempty"`
	SearchQueries     []string                   `json:"search_queries,omitempty"`
	CapabilityTags    []string                   `json:"capability_tags,omitempty"`
	Results           []skillmarket.SearchResult `json:"results,omitempty"`
	RecommendedIDs    []string                   `json:"recommended_ids,omitempty"`
	InstallMode       string                     `json:"install_mode,omitempty"`
}

type Plan struct {
	NeedNewSkill     bool
	Reason           string
	SearchQueries    []string
	CapabilityTags   []string
	AllowImplicitUse bool
}

type Planner interface {
	Plan(query string, installed *agentcore.Decision) Plan
}

type Reranker interface {
	Recommend(query string, results []skillmarket.SearchResult) []string
}

type Searcher interface {
	Search(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error)
}

type SearchFunc func(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error)

func (fn SearchFunc) Search(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
	return fn(ctx, query)
}

type Service struct {
	searcher         Searcher
	planner          Planner
	reranker         Reranker
	searchPageSize   int
	maxMergedResults int
}

func NewService(searcher Searcher) *Service {
	return &Service{
		searcher:         searcher,
		planner:          NewHeuristicPlanner(),
		reranker:         NewHeuristicReranker(),
		searchPageSize:   defaultSearchPageSize,
		maxMergedResults: defaultMaxMergedResults,
	}
}

func (s *Service) Advise(ctx context.Context, query string, installed *agentcore.Decision) (*Advice, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return &Advice{}, nil
	}

	advice := &Advice{
		Query:             query,
		InstalledDecision: installed,
		InstallMode:       "explicit_only",
	}

	if s.planner == nil {
		s.planner = NewHeuristicPlanner()
	}
	if s.reranker == nil {
		s.reranker = NewHeuristicReranker()
	}

	if !needsStoreSearch(installed) {
		advice.Reason = "installed_skill_is_sufficient"
		return advice, nil
	}

	plan := s.planner.Plan(query, installed)
	advice.NeedStoreSearch = plan.NeedNewSkill
	advice.Reason = plan.Reason
	advice.SearchQueries = append(advice.SearchQueries, plan.SearchQueries...)
	advice.CapabilityTags = append(advice.CapabilityTags, plan.CapabilityTags...)
	if !plan.NeedNewSkill {
		return advice, nil
	}
	if plan.AllowImplicitUse {
		advice.InstallMode = "allow_implicit"
	}
	if s.searcher == nil {
		advice.Reason = "skill_store_search_unavailable"
		return advice, nil
	}

	results, err := s.searchQueries(ctx, advice.SearchQueries)
	if err != nil {
		return advice, err
	}
	advice.Results = results
	advice.RecommendedIDs = s.reranker.Recommend(query, results)
	return advice, nil
}

func needsStoreSearch(installed *agentcore.Decision) bool {
	if installed == nil {
		return true
	}
	if strings.TrimSpace(installed.SelectedSkill) == "" {
		return true
	}
	if installed.NeedClarify {
		return true
	}
	return installed.Confidence < defaultInstalledSufficientConfidence
}

func (s *Service) searchQueries(ctx context.Context, queries []string) ([]skillmarket.SearchResult, error) {
	results, err := s.searchQueriesWithInstallable(ctx, queries, true)
	if err == nil && len(results) > 0 {
		return results, nil
	}
	fallback, fallbackErr := s.searchQueriesWithInstallable(ctx, queries, false)
	if len(fallback) > 0 || fallbackErr == nil {
		return fallback, fallbackErr
	}
	if err != nil {
		return nil, err
	}
	return nil, fallbackErr
}

func (s *Service) searchQueriesWithInstallable(ctx context.Context, queries []string, installableOnly bool) ([]skillmarket.SearchResult, error) {
	querySets := make([][]skillmarket.SearchResult, 0, len(queries))
	var firstErr error
	for _, raw := range queries {
		q := strings.TrimSpace(raw)
		if q == "" {
			continue
		}
		searchQuery := skillmarket.SearchQuery{
			Query:    q,
			Page:     1,
			PageSize: s.searchPageSize,
			Semantic: true,
		}
		if installableOnly {
			installable := true
			searchQuery.Installable = &installable
		}
		resp, err := s.searcher.Search(ctx, searchQuery)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if resp != nil && len(resp.Skills) > 0 {
			querySets = append(querySets, resp.Skills)
		}
	}
	if len(querySets) == 0 {
		return nil, firstErr
	}
	return mergeByRRF(querySets, s.maxMergedResults), nil
}

func mergeByRRF(querySets [][]skillmarket.SearchResult, limit int) []skillmarket.SearchResult {
	const k = 60.0
	type aggregate struct {
		item  skillmarket.SearchResult
		score float64
	}

	merged := make(map[string]*aggregate, len(querySets)*4)
	for _, results := range querySets {
		for rank, item := range results {
			if strings.TrimSpace(item.Skill.ID) == "" {
				continue
			}
			score := 1.0 / (k + float64(rank+1))
			if existing, ok := merged[item.Skill.ID]; ok {
				existing.score += score
				if item.Score > existing.item.Score {
					existing.item = item
				}
				continue
			}
			copied := item
			merged[item.Skill.ID] = &aggregate{item: copied, score: score}
		}
	}

	out := make([]skillmarket.SearchResult, 0, len(merged))
	for _, item := range merged {
		item.item.Score = item.score
		out = append(out, item.item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Skill.Name < out[j].Skill.Name
		}
		return out[i].Score > out[j].Score
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
