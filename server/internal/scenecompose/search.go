package scenecompose

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
)

type AssetSearcher struct {
	searcher Searcher
	resolver Resolver
}

func NewAssetSearcher(searcher Searcher, resolver Resolver) *AssetSearcher {
	return &AssetSearcher{searcher: searcher, resolver: resolver}
}

func (s *AssetSearcher) FindBackground(ctx context.Context, plan *ScenePlan) (*ResolvedImage, string, AssetRef, error) {
	if s == nil || s.searcher == nil || s.resolver == nil {
		return nil, "", AssetRef{}, fmt.Errorf("asset searcher is not configured")
	}
	query := buildBackgroundQuery(plan)
	results, err := s.searcher.Search(ctx, query, 3)
	if err != nil {
		return nil, query, AssetRef{}, err
	}
	var best *ResolvedImage
	bestScore := math.Inf(-1)
	var bestRef AssetRef
	for _, item := range results {
		resolved, resolveErr := s.resolver.Resolve(ctx, item)
		if resolveErr != nil || resolved == nil || resolved.Image == nil {
			continue
		}
		score := backgroundScore(item, resolved)
		if score > bestScore {
			bestScore = score
			best = resolved
			bestRef = AssetRef{
				Kind:      "background",
				Query:     query,
				SourceURL: firstNonEmpty(resolved.SourceURL, item.URL),
				Title:     firstNonEmpty(resolved.Title, item.Title),
			}
		}
	}
	if best == nil {
		return nil, query, AssetRef{}, fmt.Errorf("no background asset found")
	}
	return best, query, bestRef, nil
}

func (s *AssetSearcher) FindForeground(ctx context.Context, plan *ScenePlan, fg ForegroundPlan) (*ResolvedImage, string, AssetRef, error) {
	if s == nil || s.searcher == nil || s.resolver == nil {
		return nil, "", AssetRef{}, fmt.Errorf("asset searcher is not configured")
	}
	queries := []string{
		buildForegroundQuery(plan, fg, true),
		buildForegroundFallbackQuery(plan, fg),
	}
	var best *ResolvedImage
	bestScore := math.Inf(-1)
	bestQuery := ""
	var bestRef AssetRef
	for _, query := range queries {
		results, err := s.searcher.Search(ctx, query, 3)
		if err != nil {
			continue
		}
		for _, item := range results {
			resolved, resolveErr := s.resolver.Resolve(ctx, item)
			if resolveErr != nil || resolved == nil || resolved.Image == nil {
				continue
			}
			score := foregroundScore(item, resolved)
			if score > bestScore {
				bestScore = score
				best = resolved
				bestQuery = query
				bestRef = AssetRef{
					Kind:      "foreground",
					Query:     query,
					SourceURL: firstNonEmpty(resolved.SourceURL, item.URL),
					Title:     firstNonEmpty(resolved.Title, item.Title),
				}
			}
		}
		if best != nil {
			break
		}
	}
	if best == nil {
		return nil, "", AssetRef{}, fmt.Errorf("no foreground asset found for %s", fg.Type)
	}
	return best, bestQuery, bestRef, nil
}

func buildBackgroundQuery(plan *ScenePlan) string {
	parts := []string{
		strings.TrimSpace(plan.Background),
		strings.TrimSpace(plan.Style),
		strings.TrimSpace(plan.TimeOfDay),
		"landscape background photo",
	}
	return compactQuery(parts...)
}

func buildForegroundQuery(plan *ScenePlan, fg ForegroundPlan, preferTransparent bool) string {
	parts := []string{
		fg.Type,
		strings.Join(fg.Attributes, " "),
		plan.Style,
	}
	if preferTransparent {
		parts = append(parts, "isolated png transparent")
	}
	return compactQuery(parts...)
}

func buildForegroundFallbackQuery(plan *ScenePlan, fg ForegroundPlan) string {
	return compactQuery(fg.Type, strings.Join(fg.Attributes, " "), plan.Style, "white background photo")
}

func compactQuery(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, " ")
}

func backgroundScore(item SearchResult, resolved *ResolvedImage) float64 {
	score := 0.0
	if resolved.Width > resolved.Height {
		score += 3
	}
	text := strings.ToLower(item.Title + " " + item.Description + " " + resolved.SourceURL)
	if backgroundSearchPrimaryCueMatcher.Contains(text) {
		score += 2
	}
	if backgroundSearchWallpaperCueMatcher.Contains(text) {
		score += 0.5
	}
	score -= float64(backgroundSearchNegativeCueMatcher.Count(text)) * 2
	return score
}

func foregroundScore(item SearchResult, resolved *ResolvedImage) float64 {
	score := 0.0
	text := strings.ToLower(item.Title + " " + item.Description + " " + firstNonEmpty(resolved.SourceURL, item.URL))
	if resolved.HasAlpha {
		score += 6
	}
	score += float64(foregroundSearchPositiveCueMatcher.Count(text)) * 2
	score -= float64(foregroundSearchNegativeCueMatcher.Count(text)) * 3
	if resolved.Width >= 512 && resolved.Height >= 512 {
		score += 1
	}
	return score
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func hostOf(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Host
}
