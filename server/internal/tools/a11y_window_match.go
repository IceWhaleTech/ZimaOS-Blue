package tools

import (
	"context"
	"strings"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func (t *A11yTool) resolveHostWindowID(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (string, error) {
	windowID = strings.TrimSpace(windowID)
	windowTitle := firstCompatString(args, "window_title", "windowTitle", "title")
	appName := firstCompatString(args, "app_name", "appName", "application", "app")

	if windowID == "" && strings.TrimSpace(windowTitle) == "" && strings.TrimSpace(appName) == "" {
		return t.effectiveWindow(""), nil
	}

	windows, err := backend.ListWindows(ctx)
	if err != nil {
		if strings.TrimSpace(windowTitle) != "" || strings.TrimSpace(appName) != "" {
			return "", err
		}
		return windowID, nil
	}

	resolved, err := resolveA11yWindowTarget(windowID, windowTitle, appName, windows)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved) != "" {
		return resolved, nil
	}
	return t.effectiveWindow(""), nil
}

func resolveA11yWindowTarget(windowID string, windowTitle string, appName string, windows []a11yruntime.WindowInfo) (string, error) {
	windowID = strings.TrimSpace(windowID)
	windowTitle = strings.TrimSpace(windowTitle)
	appName = strings.TrimSpace(appName)

	if len(windows) == 0 {
		if windowID != "" {
			return windowID, nil
		}
		if windowTitle == "" && appName == "" {
			return "", nil
		}
		return "", a11yruntime.NewError("backend_unavailable", "target window not found", windowMatchDetails(windowID, windowTitle, appName, 0))
	}

	if windowID != "" && hasExactWindowID(windowID, windows) {
		return windowID, nil
	}

	if windowTitle != "" || appName != "" {
		candidates := windows
		if windowTitle != "" {
			candidates = filterWindowsByResolvedQuery(windowTitle, candidates, func(item a11yruntime.WindowInfo) string {
				return item.Title
			})
		}
		if appName != "" {
			candidates = filterWindowsByResolvedQuery(appName, candidates, func(item a11yruntime.WindowInfo) string {
				return item.AppName
			})
		}
		return resolveUniqueWindowCandidate(candidates, windowID, windowTitle, appName)
	}

	if windowID == "" {
		return "", nil
	}

	candidates := filterWindowsByExactTitle(windowID, windows)
	candidates = appendMissingWindows(candidates, filterWindowsByExactAppName(windowID, windows))
	if len(candidates) == 0 {
		return windowID, nil
	}
	return resolveUniqueWindowCandidate(candidates, windowID, "", "")
}

func resolveUniqueWindowCandidate(candidates []a11yruntime.WindowInfo, windowID string, windowTitle string, appName string) (string, error) {
	switch len(candidates) {
	case 0:
		return "", a11yruntime.NewError("backend_unavailable", "target window not found", windowMatchDetails(windowID, windowTitle, appName, 0))
	case 1:
		return strings.TrimSpace(candidates[0].ID), nil
	default:
		if resolved, ok := resolveFocusedWindowCandidate(candidates); ok {
			return resolved, nil
		}
		return "", a11yruntime.NewError("backend_unavailable", "target window is ambiguous", windowMatchDetails(windowID, windowTitle, appName, len(candidates)))
	}
}

func resolveFocusedWindowCandidate(candidates []a11yruntime.WindowInfo) (string, bool) {
	focusedID := ""
	for _, item := range candidates {
		if !item.Focused {
			continue
		}
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if focusedID != "" {
			return "", false
		}
		focusedID = id
	}
	if focusedID == "" {
		return "", false
	}
	return focusedID, true
}

func hasExactWindowID(windowID string, windows []a11yruntime.WindowInfo) bool {
	for _, item := range windows {
		if strings.TrimSpace(item.ID) == windowID {
			return true
		}
	}
	return false
}

func filterWindowsByExactTitle(query string, windows []a11yruntime.WindowInfo) []a11yruntime.WindowInfo {
	return filterWindowsByExactField(query, windows, func(item a11yruntime.WindowInfo) string {
		return item.Title
	})
}

func filterWindowsByExactAppName(query string, windows []a11yruntime.WindowInfo) []a11yruntime.WindowInfo {
	return filterWindowsByExactField(query, windows, func(item a11yruntime.WindowInfo) string {
		return item.AppName
	})
}

func filterWindowsByResolvedQuery(query string, windows []a11yruntime.WindowInfo, primaryField func(a11yruntime.WindowInfo) string) []a11yruntime.WindowInfo {
	exactMatches := filterWindowsByExactField(query, windows, primaryField)
	if len(exactMatches) > 0 {
		return exactMatches
	}
	return filterWindowsByBestFuzzyField(query, windows, primaryField)
}

func filterWindowsByExactField(query string, windows []a11yruntime.WindowInfo, field func(a11yruntime.WindowInfo) string) []a11yruntime.WindowInfo {
	normalizedQuery := normalizeA11yWindowMatchValue(query)
	if normalizedQuery == "" {
		return nil
	}
	matches := make([]a11yruntime.WindowInfo, 0, len(windows))
	for _, item := range windows {
		if normalizeA11yWindowMatchValue(field(item)) == normalizedQuery {
			matches = append(matches, item)
		}
	}
	return matches
}

func filterWindowsByBestFuzzyField(query string, windows []a11yruntime.WindowInfo, field func(a11yruntime.WindowInfo) string) []a11yruntime.WindowInfo {
	terms := parseA11yWindowMatchTerms(query)
	if len(terms) == 0 {
		return nil
	}
	bestScore := 0
	matches := make([]a11yruntime.WindowInfo, 0, len(windows))
	for _, item := range windows {
		score := windowFieldMatchScore(field(item), terms)
		if score <= 0 {
			continue
		}
		switch {
		case score > bestScore:
			bestScore = score
			matches = matches[:0]
			matches = append(matches, item)
		case score == bestScore:
			matches = append(matches, item)
		}
	}
	return matches
}

func parseA11yWindowMatchTerms(query string) []string {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	replacer := strings.NewReplacer("，", ",", "、", ",", "；", ",", ";", ",", "|", ",", "\n", " ", "\t", " ")
	parts := strings.FieldsFunc(replacer.Replace(query), func(r rune) bool {
		return r == ',' || r == ' '
	})
	seen := make(map[string]struct{}, len(parts))
	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := normalizeA11yWindowMatchValue(part)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		terms = append(terms, normalized)
	}
	return terms
}

func windowFieldMatchScore(value string, terms []string) int {
	normalizedValue := normalizeA11yWindowMatchValue(value)
	score := 0
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(normalizedValue, term) {
			score++
		}
	}
	return score
}

func appendMissingWindows(base []a11yruntime.WindowInfo, extra []a11yruntime.WindowInfo) []a11yruntime.WindowInfo {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base))
	for _, item := range base {
		seen[strings.TrimSpace(item.ID)] = struct{}{}
	}
	out := append([]a11yruntime.WindowInfo(nil), base...)
	for _, item := range extra {
		id := strings.TrimSpace(item.ID)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeA11yWindowMatchValue(value string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	return strings.Join(fields, " ")
}

func windowMatchDetails(windowID string, windowTitle string, appName string, matches int) map[string]interface{} {
	details := map[string]interface{}{}
	if trimmed := strings.TrimSpace(windowID); trimmed != "" {
		details["window_id"] = trimmed
	}
	if trimmed := strings.TrimSpace(windowTitle); trimmed != "" {
		details["window_title"] = trimmed
	}
	if trimmed := strings.TrimSpace(appName); trimmed != "" {
		details["app_name"] = trimmed
	}
	if matches > 0 {
		details["matches"] = matches
	}
	return details
}
