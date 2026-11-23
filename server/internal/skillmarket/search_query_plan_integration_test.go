package skillmarket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestServiceSearchUsesSeparateMultilingualKeywordAndVectorQueries(t *testing.T) {
	for _, mode := range []string{"hybrid", "embedding-error", "no-embedding", "literal"} {
		t.Run(mode, func(t *testing.T) {
			provider := &searchQueryEmbedding{fail: mode == "embedding-error"}
			svc, cleanup := newTestServiceWithEmbeddingProvider(t, provider)
			defer cleanup()
			if mode == "no-embedding" {
				svc.embeddingProvider = nil
			}
			calls := 0
			svc.queryPlanner = NewSearchQueryPlanner(queryPlanChatFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
				calls++
				return &llm.ChatResponse{Message: llm.Message{Content: `{"query":"rebase","semantic_query":"变基"}`}, Usage: llm.Usage{TotalTokens: 42}}, nil
			}), "zh")
			for _, id := range []string{"rebase-helper", "find-skills"} {
				doc := SkillDocument{ID: id, Slug: id, Name: id, Description: id, Published: true, EmbeddingJSON: `[1,0,0]`, EmbeddingModel: provider.Model()}
				if id == "rebase-helper" {
					vector, _ := json.Marshal(provider.semanticTestProvider.embedText("变基"))
					doc.EmbeddingJSON = string(vector)
				}
				if err := svc.Store().UpsertSkill(context.Background(), &doc, nil, nil); err != nil {
					t.Fatal(err)
				}
			}
			const raw = "Finde eine Fähigkeit zum Umbasieren von Git Branches"
			result, err := svc.Search(context.Background(), SearchQuery{Query: raw, Semantic: mode != "literal"})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "literal" {
				if calls != 0 || result.Total != 0 || result.QueryPlan != nil {
					t.Fatalf("literal search invoked rewrite: calls=%d result=%+v", calls, result)
				}
				return
			}
			if calls != 1 || len(result.Skills) == 0 || result.Skills[0].Skill.ID != "rebase-helper" || result.QueryPlan == nil || result.QueryPlan.OriginalQuery != raw || result.QueryPlan.Usage.TotalTokens != 42 {
				t.Fatalf("multilingual search failed: calls=%d result=%+v", calls, result)
			}
			if mode != "no-embedding" && (len(provider.queries) != 1 || provider.queries[0] != "变基") {
				t.Fatalf("vector query=%q", provider.queries)
			}
		})
	}
}
