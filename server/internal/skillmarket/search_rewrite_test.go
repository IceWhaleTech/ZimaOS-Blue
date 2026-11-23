package skillmarket

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
)

type searchQueryEmbedding struct {
	semanticTestProvider
	queries []string
	fail    bool
	empty   bool
}

func (p *searchQueryEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	p.queries = append(p.queries, text)
	if p.fail {
		return nil, errors.New("embedding unavailable")
	}
	if p.empty {
		return nil, nil
	}
	return p.semanticTestProvider.Embed(ctx, text)
}

func TestServiceSemanticSearchRewritesDiscoveryQuery(t *testing.T) {
	for _, mode := range []string{"hybrid", "embedding-error", "empty-embedding", "no-embedding"} {
		t.Run(mode, func(t *testing.T) {
			provider := &searchQueryEmbedding{fail: mode == "embedding-error", empty: mode == "empty-embedding"}
			var embedder embedding.Provider = provider
			if mode == "no-embedding" {
				embedder = nil
			}
			svc, cleanup := newTestServiceWithEmbeddingProvider(t, embedder)
			defer cleanup()
			for _, doc := range []SkillDocument{
				{ID: "rebase-helper", Name: "Rebase Helper", Description: "rebase", Category: "development_tools", Published: true, EmbeddingJSON: `[1,0,0]`, EmbeddingModel: provider.Model()},
				{ID: "find-skills", Name: "Find Skills", Description: "Help me find a skill 帮我找一个技能", Category: "development_tools", Published: true, EmbeddingJSON: `[0,0,1]`, EmbeddingModel: provider.Model()},
				{ID: "other-category", Name: "Other Rebase", Description: "rebase", Category: "other", Published: true, EmbeddingJSON: `[1,0,0]`, EmbeddingModel: provider.Model()},
			} {
				doc := doc
				doc.Slug = doc.ID
				if err := svc.Store().UpsertSkill(context.Background(), &doc, nil, nil); err != nil {
					t.Fatal(err)
				}
			}
			query := SearchQuery{Query: "rebase", Semantic: true, Category: "development_tools", Page: 1, PageSize: 1}
			baseline, err := svc.Search(context.Background(), query)
			if err != nil {
				t.Fatal(err)
			}
			if len(baseline.Skills) != 1 || baseline.Skills[0].Skill.ID != "rebase-helper" {
				t.Fatalf("plain capability search = %+v", baseline)
			}
			for _, wrapped := range []string{
				"帮我找一个技能，rebase",
				"请帮我搜索一个技能：rebase",
				"帮我找一个rebase的技能",
				"Help me find a skill for rebase",
				"Could you recommend a skill to rebase",
			} {
				provider.queries = nil
				query.Query = wrapped
				got, err := svc.Search(context.Background(), query)
				if err != nil {
					t.Fatal(err)
				}
				if mode != "no-embedding" && !reflect.DeepEqual(provider.queries, []string{"rebase"}) {
					t.Errorf("Embed query for %q = %q, want capability only", wrapped, provider.queries)
				}
				if !reflect.DeepEqual(got, baseline) {
					t.Errorf("Search(%q) = %+v, want same ranking, filters and pagination as plain rebase: %+v", wrapped, got, baseline)
				}
			}
			provider.queries = nil
			query.Semantic = false
			query.Query = "帮我找一个技能，rebase"
			literal, err := svc.Search(context.Background(), query)
			if err != nil {
				t.Fatal(err)
			}
			if literal.Total != 0 || len(provider.queries) != 0 {
				t.Errorf("non-semantic search must retain literal query, got %+v and embedding calls %q", literal, provider.queries)
			}
		})
	}
}

func TestServiceSemanticSearchPreservesCapabilityContent(t *testing.T) {
	provider := &searchQueryEmbedding{}
	svc, cleanup := newTestServiceWithEmbeddingProvider(t, provider)
	defer cleanup()
	for _, tc := range []struct{ raw, want string }{
		{"帮我找一个技能，查找重复文件", "查找重复文件"},
		{"请帮我搜索一个技能：本地搜索 PDF，不要上传云端", "本地搜索 PDF，不要上传云端"},
		{"帮我找一个能读取扫描 PDF 的技能", "能读取扫描 PDF"},
		{"我想找一个技能，C++ AST", "C++ AST"},
		{"帮我找一个技能，\"find-skills\"", "\"find-skills\""},
		{"Help me find a skill for finding duplicate files without uploading data", "finding duplicate files without uploading data"},
		{"Please recommend a skill that can search local PDFs", "search local PDFs"},
		{"find duplicate files", "find duplicate files"},
		{"查找重复文件", "查找重复文件"},
		{"搜索引擎优化", "搜索引擎优化"},
		{"find-skills", "find-skills"},
		{"search.py", "search.py"},
		{"技能搜索", "技能搜索"},
		{"\"帮我找一个技能，PDF\"", "\"帮我找一个技能，PDF\""},
		{"帮我找一个技能", "帮我找一个技能"},
		{"帮我找一个技能，", "帮我找一个技能，"},
		{"find a skill", "find a skill"},
		{"find skills", "find skills"},
		{"find a skill for", "find a skill for"},
		{"find a skill that can", "find a skill that can"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			provider.queries = nil
			if _, err := svc.Search(context.Background(), SearchQuery{Query: tc.raw, Semantic: true}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(provider.queries, []string{tc.want}) {
				t.Errorf("Embed received %q, want %q", provider.queries, tc.want)
			}
		})
	}
}
