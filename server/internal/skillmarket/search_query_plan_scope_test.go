package skillmarket

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestServiceSearchPreparedPlanDoesNotReplaceExpansionVector(t *testing.T) {
	provider := &searchQueryEmbedding{}
	svc, cleanup := newTestServiceWithEmbeddingProvider(t, provider)
	defer cleanup()
	plan := SearchQueryPlan{OriginalQuery: "Trouvez une compétence pour rebase", Query: "rebase", SemanticQuery: "变基", Source: "model"}
	ctx := WithSearchQueryPlan(context.Background(), plan)
	for _, query := range []string{plan.OriginalQuery, plan.Query, "offline OCR"} {
		if _, err := svc.Search(ctx, SearchQuery{Query: query, Semantic: true}); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(provider.queries, []string{"变基", "变基", "offline OCR"}) {
		t.Fatalf("prepared plan polluted expansion query: %q", provider.queries)
	}
	if _, ok := PreparedSearchQuery(context.Background(), plan.Query); ok {
		t.Fatal("prepared plan escaped its context")
	}
}

func TestServiceSearchMultilingualRetainsFiltersAndPagination(t *testing.T) {
	for _, mode := range []string{"hybrid", "embedding-error", "empty-embedding", "no-embedding"} {
		t.Run(mode, func(t *testing.T) {
			provider := &searchQueryEmbedding{fail: mode == "embedding-error", empty: mode == "empty-embedding"}
			svc, cleanup := newTestServiceWithEmbeddingProvider(t, provider)
			defer cleanup()
			if mode == "no-embedding" {
				svc.embeddingProvider = nil
			}
			for _, f := range []struct{ id, category, source string }{
				{"alpha", "development_tools", "trusted"}, {"beta", "development_tools", "trusted"},
				{"wrong-source", "development_tools", "other"}, {"wrong-category", "productivity", "trusted"},
			} {
				doc := SkillDocument{ID: f.id, Slug: f.id, Name: f.id, Description: "rebase", Published: true,
					Category: f.category, SourceID: f.source, EmbeddingJSON: `[1,0,0]`, EmbeddingModel: provider.Model(),
					LastUpdated: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
				if f.id == "alpha" {
					doc.Stars = 100
				}
				if err := svc.Store().UpsertSkill(context.Background(), &doc, nil, nil); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			svc.queryPlanner = NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
				calls++
				return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"rebase","semantic_query":"变基"}`}, Usage: llm.Usage{TotalTokens: 19}}, nil
			}), "zh")
			query := SearchQuery{Query: "Finde eine Fähigkeit zum Umbasieren von Git Branches", Semantic: true,
				Categories: []string{"development_tools"}, Sources: []string{"trusted"}, Page: 2, PageSize: 1}
			before := query
			before.Categories, before.Sources = append([]string(nil), query.Categories...), append([]string(nil), query.Sources...)
			for attempt := range 2 {
				got, err := svc.Search(context.Background(), query)
				if err != nil {
					t.Fatal(err)
				}
				if got.Total != 2 || got.Page != 2 || got.PageSize != 1 || got.TotalPages != 2 || len(got.Skills) != 1 || got.Skills[0].Skill.ID != "beta" {
					t.Fatalf("rewrite changed filters/page: %+v", got)
				}
				if got.QueryPlan == nil || calls != 1 || got.QueryPlan.CacheHit != (attempt == 1) || (attempt == 0 && got.QueryPlan.Usage == nil) || (attempt == 1 && got.QueryPlan.Usage != nil) {
					t.Fatalf("cache lost usage/filter identity: calls=%d result=%+v", calls, got)
				}
			}
			if !reflect.DeepEqual(query, before) {
				t.Fatalf("caller filters mutated: %+v", query)
			}
		})
	}
}

func TestServiceSearchMultilingualMetadataRemainsRequestScoped(t *testing.T) {
	for _, mode := range []string{"no-embedding", "embedding-error", "empty-embedding"} {
		t.Run(mode, func(t *testing.T) {
			provider := &searchQueryEmbedding{fail: mode == "embedding-error", empty: mode == "empty-embedding"}
			svc, cleanup := newTestServiceWithEmbeddingProvider(t, provider)
			defer cleanup()
			if mode == "no-embedding" {
				svc.embeddingProvider = nil
			}
			svc.queryPlanner = NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
				return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"rebase","semantic_query":"变基"}`}, Usage: llm.Usage{TotalTokens: 37}}, nil
			}), "zh")
			query := SearchQuery{Query: "Finde eine Fähigkeit zum Umbasieren von Git Branches", Semantic: true}
			first, err := svc.Search(context.Background(), query)
			if err != nil {
				t.Fatal(err)
			}
			second, err := svc.Search(context.Background(), query)
			if err != nil {
				t.Fatal(err)
			}
			if first.QueryPlan == nil || first.QueryPlan.Usage == nil || first.QueryPlan.Usage.TotalTokens != 37 || first.QueryPlan.CacheHit {
				t.Errorf("later search overwrote first response billing: %+v", first.QueryPlan)
			}
			if second.QueryPlan == nil || !second.QueryPlan.CacheHit || second.QueryPlan.Usage != nil {
				t.Errorf("wrong cached response: %+v", second.QueryPlan)
			}
			literal, err := svc.Search(context.Background(), SearchQuery{Query: "rebase", Semantic: false})
			if err != nil {
				t.Fatal(err)
			}
			if literal.QueryPlan != nil {
				t.Errorf("semantic diagnostic leaked into literal search: %+v", literal.QueryPlan)
			}
		})
	}
}

func TestServiceSearchMultilingualConcurrentUsage(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	var calls atomic.Int32
	svc.queryPlanner = NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		calls.Add(1)
		return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"rebase","semantic_query":"变基"}`}, Usage: llm.Usage{TotalTokens: 37}}, nil
	}), "zh")
	start := make(chan struct{})
	results := make(chan *SearchResponse, 16)
	var group sync.WaitGroup
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			got, err := svc.Search(context.Background(), SearchQuery{Query: "Finde eine Fähigkeit zum Umbasieren von Git Branches", Semantic: true})
			if err != nil {
				t.Error(err)
				return
			}
			results <- got
		}()
	}
	close(start)
	group.Wait()
	close(results)
	billed, received := 0, 0
	for got := range results {
		received++
		if got.QueryPlan == nil {
			t.Error("concurrent response lost query plan")
			continue
		}
		if got.QueryPlan.Usage != nil {
			billed++
			if got.QueryPlan.Usage.TotalTokens != 37 || !got.QueryPlan.ModelCalled {
				t.Errorf("invalid usage: %+v", got.QueryPlan)
			}
		} else if !got.QueryPlan.CacheHit || got.QueryPlan.ModelCalled {
			t.Errorf("shared request billed: %+v", got.QueryPlan)
		}
	}
	if received != 16 || billed != 1 || calls.Load() != 1 {
		t.Fatalf("received=%d billed=%d model calls=%d", received, billed, calls.Load())
	}
}
