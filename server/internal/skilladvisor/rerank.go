package skilladvisor

import (
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

type HeuristicReranker struct {
	maxRecommendations int
}

func NewHeuristicReranker() *HeuristicReranker {
	return &HeuristicReranker{maxRecommendations: 3}
}

func (r *HeuristicReranker) Recommend(query string, results []skillmarket.SearchResult) []string {
	if len(results) == 0 {
		return nil
	}

	terms := derivedTerms(query)
	type scoredResult struct {
		id    string
		score float64
	}
	scored := make([]scoredResult, 0, len(results))
	for _, result := range results {
		doc := result.Skill
		if strings.TrimSpace(doc.ID) == "" {
			continue
		}
		score := result.Score
		if doc.Installable {
			score += 15
		} else {
			score -= 20
		}
		switch strings.ToLower(doc.SecurityBadge) {
		case skillmarket.BadgeGreen:
			score += 8
		case skillmarket.BadgeYellow:
			score += 2
		case skillmarket.BadgeRed:
			score -= 15
		}
		switch strings.ToLower(doc.RiskLevel) {
		case skillmarket.RiskLow:
			score += 6
		case skillmarket.RiskMedium:
			score += 1
		case skillmarket.RiskHigh:
			score -= 10
		case skillmarket.RiskCritical:
			score -= 20
		}
		if doc.CuratedRank > 0 {
			score += 10 - float64(minInt(doc.CuratedRank, 10))
		}
		if doc.HasPromptInjection {
			score -= 20
		}
		if doc.HasShellInjection {
			score -= 20
		}
		if doc.HasDataExfiltration {
			score -= 25
		}
		if doc.HasVulnerabilities {
			score -= 10
		}

		searchable := strings.ToLower(doc.Name + " " + doc.Description + " " + strings.Join(doc.Tags, " "))
		matches := 0
		for _, term := range terms {
			if term != "" && strings.Contains(searchable, term) {
				score += 4
				matches++
			}
		}
		if matches == 0 {
			score -= 4
		}
		scored = append(scored, scoredResult{id: doc.ID, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].id < scored[j].id
		}
		return scored[i].score > scored[j].score
	})

	out := make([]string, 0, r.maxRecommendations)
	for _, item := range scored {
		if item.score <= 0 {
			continue
		}
		out = append(out, item.id)
		if len(out) >= r.maxRecommendations {
			break
		}
	}
	return out
}
