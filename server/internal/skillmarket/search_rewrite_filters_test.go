package skillmarket

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestServiceSearchRewritePreservesSourceCategoryAndPage(t *testing.T) {
	provider := &searchQueryEmbedding{}
	svc, cleanup := newTestServiceWithEmbeddingProvider(t, provider)
	defer cleanup()
	for _, fixture := range []struct{ id, category, source string }{
		{"alpha", "development_tools", "trusted-source"},
		{"beta", "development_tools", "trusted-source"},
		{"wrong-source", "development_tools", "other-source"},
		{"wrong-category", "productivity", "trusted-source"},
	} {
		doc := SkillDocument{
			ID: fixture.id, Slug: fixture.id, Name: fixture.id, Description: "rebase", Published: true,
			Category: fixture.category, SourceID: fixture.source,
			EmbeddingJSON: `[1,0,0]`, EmbeddingModel: provider.Model(),
			LastUpdated: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		if err := svc.Store().UpsertSkill(context.Background(), &doc, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	query := SearchQuery{
		Query: "rebase", Semantic: true, Categories: []string{"development_tools"},
		Sources: []string{"trusted-source"}, Page: 2, PageSize: 1,
	}
	baseline, err := svc.Search(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Total != 2 || baseline.Page != 2 || baseline.TotalPages != 2 || len(baseline.Skills) != 1 || baseline.Skills[0].Skill.ID != "beta" {
		t.Fatalf("invalid filter/page fixture: %+v", baseline)
	}
	query.Query = "请帮我找一个技能，rebase"
	before := query
	wrapped, err := svc.Search(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wrapped, baseline) {
		t.Errorf("rewrite changed filtered page: got %+v, want %+v", wrapped, baseline)
	}
	if !reflect.DeepEqual(query, before) {
		t.Errorf("search mutated caller query: got %+v, want %+v", query, before)
	}
}
