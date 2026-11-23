package skilladvisor

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

func TestAdviseUsesCapabilityWithoutDiscoveryBoilerplate(t *testing.T) {
	for _, tc := range []struct{ raw, capability string }{
		{"请帮我搜索一个技能，离线 OCR PDF", "离线 OCR PDF"},
		{"Help me find a skill for chroma key", "chroma key"},
		{"Could you please help me find a skill for chroma key", "chroma key"},
		{"帮我找一个技能，离线 OCR PDF，不要上传云端", "离线 OCR PDF，不要上传云端"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			var queries []string
			service := NewService(SearchFunc(func(_ context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
				queries = append(queries, query.Query)
				return &skillmarket.SearchResponse{Skills: []skillmarket.SearchResult{
					{Skill: skillmarket.SkillDocument{ID: "find-skills", Name: "Help me find a skill", Installable: true}, Score: 30},
					{Skill: skillmarket.SkillDocument{ID: "capability", Name: tc.capability, Installable: true}, Score: 30},
				}}, nil
			}))
			advice, err := service.Advise(context.Background(), tc.raw, nil)
			if err != nil {
				t.Fatal(err)
			}
			if advice.Query != tc.raw {
				t.Fatalf("response query = %q, want original %q", advice.Query, tc.raw)
			}
			if len(queries) == 0 || queries[0] != tc.capability {
				t.Errorf("search queries = %q, want capability first: %q", queries, tc.capability)
			}
			for _, tag := range advice.CapabilityTags {
				if tag == "web-search" {
					t.Errorf("discovery verb incorrectly generated web-search capability")
				}
			}
			if len(advice.RecommendedIDs) == 0 || advice.RecommendedIDs[0] != "capability" {
				t.Errorf("recommendations = %q, want requested capability ahead of generic skill finder", advice.RecommendedIDs)
			}
		})
	}
}

func TestAdvisePreservesRequestedSearchCapability(t *testing.T) {
	service := NewService(SearchFunc(func(_ context.Context, _ skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		return &skillmarket.SearchResponse{}, nil
	}))
	advice, err := service.Advise(context.Background(), "帮我找一个技能，搜索新闻并引用网页来源", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(advice.SearchQueries) == 0 || advice.SearchQueries[0] != "搜索新闻并引用网页来源" {
		t.Fatalf("real search capability was removed: %+v", advice)
	}
	for _, tag := range advice.CapabilityTags {
		if tag == "web-search" {
			return
		}
	}
	t.Fatalf("real web-search capability was lost: %+v", advice)
}
