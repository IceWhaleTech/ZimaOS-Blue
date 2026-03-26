package skilladvisor

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

func TestAdviseSkipsStoreSearchForStrongInstalledMatch(t *testing.T) {
	called := 0
	service := NewService(SearchFunc(func(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		called++
		return nil, nil
	}))

	advice, err := service.Advise(context.Background(), "请帮我搜索最新新闻", &agentcore.Decision{
		SelectedSkill: "web_search",
		Confidence:    0.93,
		NeedClarify:   false,
	})
	if err != nil {
		t.Fatalf("Advise error: %v", err)
	}
	if advice.NeedStoreSearch {
		t.Fatalf("expected no store search, got %+v", advice)
	}
	if called != 0 {
		t.Fatalf("expected no searcher call, got %d", called)
	}
}

func TestAdviseUsesKeywordAndHybridSearch(t *testing.T) {
	var captured []skillmarket.SearchQuery
	service := NewService(SearchFunc(func(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		captured = append(captured, query)
		return &skillmarket.SearchResponse{
			Skills: []skillmarket.SearchResult{
				{
					Skill: skillmarket.SkillDocument{
						ID:            "gh-release-bot",
						Name:          "GitHub Release Bot",
						Description:   "Automate GitHub Actions releases, changelog generation, and tag publishing.",
						Installable:   true,
						SecurityBadge: skillmarket.BadgeGreen,
						RiskLevel:     skillmarket.RiskLow,
						CuratedRank:   1,
						Tags:          []string{"github-actions", "release", "changelog"},
					},
					Score: 42,
				},
				{
					Skill: skillmarket.SkillDocument{
						ID:                 "workflow-linter",
						Name:               "Workflow Linter",
						Description:        "Lint GitHub workflow files.",
						Installable:        true,
						SecurityBadge:      skillmarket.BadgeYellow,
						RiskLevel:          skillmarket.RiskMedium,
						HasPromptInjection: true,
						Tags:               []string{"github-actions", "lint"},
					},
					Score: 39,
				},
			},
			Total: 2,
		}, nil
	}))

	advice, err := service.Advise(context.Background(), "帮我做 GitHub Actions 自动发版并生成 changelog", &agentcore.Decision{
		SelectedSkill: "",
		Confidence:    0.31,
		NeedClarify:   true,
	})
	if err != nil {
		t.Fatalf("Advise error: %v", err)
	}
	if !advice.NeedStoreSearch {
		t.Fatalf("expected store search, got %+v", advice)
	}
	if len(captured) == 0 {
		t.Fatalf("expected search calls")
	}
	foundSemantic := false
	foundMappedQuery := false
	for _, query := range captured {
		if query.Semantic {
			foundSemantic = true
		}
		if strings.Contains(strings.ToLower(query.Query), "release automation") || strings.Contains(strings.ToLower(query.Query), "github actions") {
			foundMappedQuery = true
		}
	}
	if !foundSemantic {
		t.Fatalf("expected semantic search to be enabled: %+v", captured)
	}
	if !foundMappedQuery {
		t.Fatalf("expected mapped capability query, got %+v", captured)
	}
	if len(advice.RecommendedIDs) == 0 || advice.RecommendedIDs[0] != "gh-release-bot" {
		t.Fatalf("expected gh-release-bot recommendation, got %+v", advice.RecommendedIDs)
	}
}
