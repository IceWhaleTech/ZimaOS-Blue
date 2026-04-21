package tools

import (
	"context"
	"strconv"
	"strings"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type a11yWindowMatch struct {
	ResolvedID string
	MatchedBy  string
	Exact      bool
	Unique     bool
	Candidates int
}

func (t *A11yTool) resolveHostWindowID(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (string, *a11yWindowMatch, error) {
	windowID = strings.TrimSpace(windowID)
	windowTitle := firstCompatString(args, "window_title", "windowTitle", "title")
	appName := firstCompatString(args, "app_name", "appName", "application", "app")
	cachedWindow, cachedHint := t.effectiveWindowContext()
	currentHint := a11yWindowQueryHint(windowTitle, appName)

	if windowID == "" && strings.TrimSpace(windowTitle) == "" && strings.TrimSpace(appName) == "" {
		return t.effectiveWindow(""), nil, nil
	}

	windows, err := backend.ListWindows(ctx)
	if err != nil {
		if strings.TrimSpace(windowTitle) != "" || strings.TrimSpace(appName) != "" {
			return "", nil, err
		}
		return windowID, nil, nil
	}

	resolved, match, err := resolveA11yWindowTarget(windowID, windowTitle, appName, windows)
	if err != nil {
		if fallback, fallbackMatch, ok := resolveA11yWindowContextFallback(err, cachedWindow, cachedHint, currentHint, windows); ok {
			return fallback, fallbackMatch, nil
		}
		if fallback, fallbackMatch, fallbackErr, ok := resolveA11yWindowAllWindowsFallback(ctx, backend, windowID, windowTitle, appName, err); ok {
			return fallback, fallbackMatch, fallbackErr
		}
		return "", nil, err
	}
	if strings.TrimSpace(resolved) != "" {
		return resolved, match, nil
	}
	return t.effectiveWindow(""), nil, nil
}

func resolveA11yWindowAllWindowsFallback(ctx context.Context, backend a11yruntime.Backend, windowID string, windowTitle string, appName string, resolveErr error) (string, *a11yWindowMatch, error, bool) {
	runtimeErr, ok := resolveErr.(*a11yruntime.RuntimeError)
	if !ok || runtimeErr.Code != "backend_unavailable" || runtimeErr.Message != "target window not found" {
		return "", nil, nil, false
	}
	lister, ok := backend.(hostAllWindowsLister)
	if !ok {
		return "", nil, nil, false
	}
	allWindows, err := lister.ListAllWindows(ctx)
	if err != nil {
		return "", nil, err, true
	}
	if len(allWindows) == 0 {
		return "", nil, nil, false
	}
	resolved, match, err := resolveA11yWindowTarget(windowID, windowTitle, appName, allWindows)
	if err != nil {
		return "", nil, err, true
	}
	if strings.TrimSpace(resolved) == "" {
		return "", nil, nil, false
	}
	return resolved, match, nil, true
}

func resolveA11yWindowContextFallback(err error, cachedWindow string, cachedHint string, currentHint string, windows []a11yruntime.WindowInfo) (string, *a11yWindowMatch, bool) {
	cachedWindow = strings.TrimSpace(cachedWindow)
	cachedHint = strings.TrimSpace(cachedHint)
	currentHint = strings.TrimSpace(currentHint)
	if cachedWindow == "" || cachedHint == "" || currentHint == "" || cachedHint != currentHint || len(windows) == 0 {
		return "", nil, false
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok || runtimeErr.Code != "backend_unavailable" || runtimeErr.Message != "target window not found" {
		return "", nil, false
	}
	contextWindow := rememberedWindowFromList(windows)
	if contextWindow == "" || contextWindow != cachedWindow {
		return "", nil, false
	}
	return cachedWindow, &a11yWindowMatch{
		ResolvedID: cachedWindow,
		MatchedBy:  "cached_window",
		Exact:      false,
		Unique:     true,
		Candidates: 1,
	}, true
}

func resolveA11yWindowTarget(windowID string, windowTitle string, appName string, windows []a11yruntime.WindowInfo) (string, *a11yWindowMatch, error) {
	windowID = strings.TrimSpace(windowID)
	windowTitle = strings.TrimSpace(windowTitle)
	appName = strings.TrimSpace(appName)

	if len(windows) == 0 {
		if windowID != "" {
			return windowID, &a11yWindowMatch{ResolvedID: windowID, MatchedBy: "window_id", Exact: true, Unique: true, Candidates: 1}, nil
		}
		if windowTitle == "" && appName == "" {
			return "", nil, nil
		}
		return "", nil, a11yruntime.NewError("backend_unavailable", "target window not found", windowMatchDetails(windowID, windowTitle, appName, 0))
	}

	if windowID != "" && hasExactWindowID(windowID, windows) {
		return windowID, &a11yWindowMatch{ResolvedID: windowID, MatchedBy: "window_id", Exact: true, Unique: true, Candidates: 1}, nil
	}

	if windowTitle != "" || appName != "" {
		candidates := windows
		exactTitle := true
		exactApp := true
		if windowTitle != "" {
			var exact bool
			candidates, exact = filterWindowsByResolvedQuery(windowTitle, candidates, func(item a11yruntime.WindowInfo) string {
				return item.Title
			})
			exactTitle = exact
		}
		if appName != "" {
			var exact bool
			candidates, exact = filterWindowsByResolvedQuery(appName, candidates, func(item a11yruntime.WindowInfo) string {
				return item.AppName
			})
			exactApp = exact
		}
		matchedBy := ""
		switch {
		case windowTitle != "" && appName != "":
			matchedBy = "window_title+app_name"
		case windowTitle != "":
			matchedBy = "window_title"
		default:
			matchedBy = "app_name"
		}
		return resolveUniqueWindowCandidate(candidates, windowID, windowTitle, appName, exactTitle && exactApp, matchedBy)
	}

	if windowID == "" {
		return "", nil, nil
	}

	candidates := filterWindowsByExactTitle(windowID, windows)
	candidates = appendMissingWindows(candidates, filterWindowsByExactAppName(windowID, windows))
	if len(candidates) == 0 {
		return windowID, &a11yWindowMatch{ResolvedID: windowID, MatchedBy: "window_id", Exact: true, Unique: true, Candidates: 1}, nil
	}
	return resolveUniqueWindowCandidate(candidates, windowID, "", "", true, "window_id")
}

func resolveUniqueWindowCandidate(candidates []a11yruntime.WindowInfo, windowID string, windowTitle string, appName string, exact bool, matchedBy string) (string, *a11yWindowMatch, error) {
	switch len(candidates) {
	case 0:
		return "", nil, a11yruntime.NewError("backend_unavailable", "target window not found", windowMatchDetails(windowID, windowTitle, appName, 0))
	case 1:
		id := strings.TrimSpace(candidates[0].ID)
		return id, &a11yWindowMatch{ResolvedID: id, MatchedBy: matchedBy, Exact: exact, Unique: true, Candidates: 1}, nil
	default:
		if resolved, ok := resolveFocusedWindowCandidate(candidates); ok {
			return resolved, &a11yWindowMatch{ResolvedID: resolved, MatchedBy: matchedBy, Exact: exact, Unique: false, Candidates: len(candidates)}, nil
		}
		// macOS apps like Feishu/Lark can expose multiple CoreGraphics windows (popovers, helpers)
		// under the same app name; pick the most likely main window by size when possible.
		if matchedBy == "app_name" && strings.TrimSpace(windowTitle) == "" && strings.TrimSpace(windowID) == "" {
			if resolved, ok := resolveLargestWindowCandidate(candidates); ok {
				return resolved, &a11yWindowMatch{ResolvedID: resolved, MatchedBy: matchedBy, Exact: exact, Unique: false, Candidates: len(candidates)}, nil
			}
		}
		return "", nil, a11yruntime.NewError("backend_unavailable", "target window is ambiguous", windowMatchDetails(windowID, windowTitle, appName, len(candidates)))
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

func resolveLargestWindowCandidate(candidates []a11yruntime.WindowInfo) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	// Prefer layer 0 windows when present.
	preferred := candidates
	layer0 := make([]a11yruntime.WindowInfo, 0, len(candidates))
	for _, item := range candidates {
		if item.Layer == 0 {
			layer0 = append(layer0, item)
		}
	}
	if len(layer0) > 0 {
		preferred = layer0
	}

	bestID := ""
	bestArea := -1.0
	bestTitleLen := -1

	for _, item := range preferred {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		w := item.Bounds.Width
		h := item.Bounds.Height
		area := -1.0
		if w > 0 && h > 0 {
			area = w * h
		}
		titleLen := len(strings.TrimSpace(item.Title))
		switch {
		case bestID == "":
			bestID = id
			bestArea = area
			bestTitleLen = titleLen
		case area > bestArea:
			bestID = id
			bestArea = area
			bestTitleLen = titleLen
		case area == bestArea:
			if titleLen > bestTitleLen {
				bestID = id
				bestTitleLen = titleLen
			} else if titleLen == bestTitleLen && id != bestID {
				// Deterministic tie-breaker: prefer the lowest numeric window id when possible,
				// otherwise fall back to lexicographic ordering.
				bestInt, bestErr := strconv.Atoi(bestID)
				candInt, candErr := strconv.Atoi(id)
				switch {
				case bestErr == nil && candErr == nil:
					if candInt < bestInt {
						bestID = id
					}
				case id < bestID:
					bestID = id
				}
			}
		}
	}
	// Only use this heuristic when we have concrete size information; using title length
	// alone is too error-prone and can pick an arbitrary background popover.
	if bestArea < 0 {
		return "", false
	}
	if bestID == "" {
		return "", false
	}
	return bestID, true
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

func filterWindowsByResolvedQuery(query string, windows []a11yruntime.WindowInfo, primaryField func(a11yruntime.WindowInfo) string) ([]a11yruntime.WindowInfo, bool) {
	exactMatches := filterWindowsByExactField(query, windows, primaryField)
	if len(exactMatches) > 0 {
		return exactMatches, true
	}
	return filterWindowsByBestFuzzyField(query, windows, primaryField), false
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
	rawTerms := parseA11yWindowMatchAliases(query)
	terms := make([]string, 0, len(rawTerms))
	for _, part := range rawTerms {
		if normalized := normalizeA11yWindowMatchValue(part); normalized != "" {
			terms = append(terms, normalized)
		}
	}
	return terms
}

func parseA11yWindowMatchAliases(query string) []string {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	replacer := strings.NewReplacer("，", ",", "、", ",", "；", ",", ";", ",", "|", ",", "\n", " ", "\t", " ")
	parts := strings.FieldsFunc(replacer.Replace(query), func(r rune) bool {
		return r == ',' || r == ' '
	})
	seen := make(map[string]struct{}, len(parts))
	aliases := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		normalized := normalizeA11yWindowMatchValue(trimmed)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		aliases = append(aliases, trimmed)
	}
	return aliases
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
