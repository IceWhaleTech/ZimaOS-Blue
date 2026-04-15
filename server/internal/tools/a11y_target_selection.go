package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type a11ySnapshotEntry struct {
	Ref   int
	Role  string
	Label string
	Token string
	Depth int
}

type a11yTargetSelector struct {
	Name string
	Role string
}

type a11yActSubmitPlan struct {
	Enabled      bool
	Ref          int
	RefMap       map[int]string
	KeySequences [][]string
}

type a11yConversationSearchPlan struct {
	Open [][]string
}

type a11ySubmitConfirmation struct {
	TypedValue string
	InputToken string
}

type a11yConversationClickPoint struct {
	X float64
	Y float64
}

type a11yConversationVisualHit struct {
	Point      a11yruntime.NormalizedPoint
	Confidence float64
}

var a11ySubmitConfirmationTimeout = 45 * time.Second
var a11ySubmitConfirmationPollInterval = 1 * time.Second
var a11yMessageConversationSettleDelay = 1 * time.Second
var a11yMessageConversationConfirmationTimeout = 30 * time.Second
var a11yMessageConversationConfirmationPollInterval = 1 * time.Second
var a11yLocateConversationVisualHit = func(context.Context, string, string) (a11yConversationVisualHit, error) {
	return a11yConversationVisualHit{}, a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", nil)
}

func (s a11yTargetSelector) Provided() bool {
	return strings.TrimSpace(s.Name) != "" || strings.TrimSpace(s.Role) != ""
}

func a11yConversationClickCacheKey(windowHint string, selectorName string) string {
	return normalizeA11yWindowMatchValue(windowHint) + "|" + normalizeA11yTargetName(selectorName)
}

func (t *A11yTool) a11yConversationClickCacheGet(key string) (a11yConversationClickPoint, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.clickCache) == 0 {
		return a11yConversationClickPoint{}, false
	}
	point, ok := t.clickCache[key]
	return point, ok
}

func (t *A11yTool) a11yConversationClickCacheSet(key string, point a11yConversationClickPoint) {
	if strings.TrimSpace(key) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.clickCache == nil {
		t.clickCache = make(map[string]a11yConversationClickPoint)
	}
	t.clickCache[key] = point
}

func (t *A11yTool) a11yConversationClickCacheDelete(key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.clickCache) == 0 {
		return
	}
	delete(t.clickCache, key)
}

func (t *A11yTool) a11yConversationClickCacheKeyForArgs(args map[string]interface{}, selector a11yTargetSelector, fallbackWindow string) string {
	hint := a11yWindowQueryHintFromArgs(args)
	if strings.TrimSpace(hint) == "" {
		_, rememberedHint := t.effectiveWindowContext()
		hint = rememberedHint
	}
	if strings.TrimSpace(hint) == "" {
		hint = strings.TrimSpace(fallbackWindow)
	}
	return a11yConversationClickCacheKey(hint, selector.Name)
}

func (t *A11yTool) resolveActTarget(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector) (a11yruntime.TargetResolution, error) {
	if resolver, ok := backend.(a11yruntime.TargetResolver); ok {
		resolution, err := resolver.ResolveTarget(ctx, windowID, a11yruntime.TargetSelector{
			Name: selector.Name,
			Role: selector.Role,
		})
		if err == nil {
			resolvedWindow := strings.TrimSpace(resolution.WindowID)
			if resolvedWindow == "" {
				resolvedWindow = strings.TrimSpace(windowID)
			}
			if resolution.RefMap == nil && resolution.Ref > 0 && strings.TrimSpace(resolution.Token) != "" {
				resolution.RefMap = map[int]string{resolution.Ref: resolution.Token}
			}
			if resolution.WindowID == "" {
				resolution.WindowID = resolvedWindow
			}
			if resolution.Tree != "" || len(resolution.RefMap) > 0 {
				t.cacheSnapshotContext(resolvedWindow, resolution.RefMap, resolution.Tree)
			}
			return resolution, nil
		}
	}

	ref, refMap, resolvedWindow, err := t.resolveActRefByTargetLegacy(ctx, backend, windowID, selector)
	if err != nil {
		return a11yruntime.TargetResolution{}, err
	}
	return a11yruntime.TargetResolution{
		WindowID: resolvedWindow,
		Ref:      ref,
		RefMap:   refMap,
	}, nil
}

func (t *A11yTool) resolveActRefByTarget(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector) (int, map[int]string, string, error) {
	result, err := t.resolveActTarget(ctx, backend, windowID, selector)
	if err != nil {
		return 0, nil, strings.TrimSpace(windowID), err
	}
	return result.Ref, cloneA11yRefMap(result.RefMap), strings.TrimSpace(result.WindowID), nil
}

func (t *A11yTool) resolveActRefByTargetLegacy(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector) (int, map[int]string, string, error) {
	t.mu.RLock()
	cachedWindow := strings.TrimSpace(t.lastWindow)
	cachedRefMap := cloneA11yRefMap(t.lastRefMap)
	cachedRefs := cloneA11ySnapshotEntries(t.lastRefs)
	t.mu.RUnlock()

	if ref, ok := resolveActRefFromCachedSnapshot(windowID, cachedWindow, cachedRefMap, cachedRefs, selector); ok {
		if strings.TrimSpace(windowID) == "" {
			windowID = cachedWindow
		}
		return ref, cachedRefMap, strings.TrimSpace(windowID), nil
	}

	snapshotTarget := strings.TrimSpace(windowID)
	if snapshotTarget == "" {
		snapshotTarget = cachedWindow
	}
	result, err := backend.SnapshotInteractive(ctx, snapshotTarget)
	if err != nil {
		return 0, nil, snapshotTarget, err
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, snapshotTarget))
	t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)

	ref, err := resolveA11yTargetRef(parseA11ySnapshotEntries(result.Tree), selector)
	if err != nil {
		if fallbackRef, fallbackRefMap, fallbackWindow, fallbackErr, ok := t.resolveActRefFromFullSnapshotByLabel(ctx, backend, resolvedWindow, selector, err); ok {
			if fallbackErr != nil {
				return 0, nil, strings.TrimSpace(valueOrDefault(fallbackWindow, resolvedWindow)), fallbackErr
			}
			return fallbackRef, fallbackRefMap, strings.TrimSpace(valueOrDefault(fallbackWindow, resolvedWindow)), nil
		}
		return 0, nil, resolvedWindow, err
	}
	return ref, cloneA11yRefMap(result.RefMap), resolvedWindow, nil
}

func resolveActRefFromCachedSnapshot(windowID string, cachedWindow string, refMap map[int]string, refs []a11ySnapshotEntry, selector a11yTargetSelector) (int, bool) {
	if len(refMap) == 0 || len(refs) == 0 {
		return 0, false
	}
	if strings.TrimSpace(windowID) != "" && strings.TrimSpace(cachedWindow) != "" && strings.TrimSpace(windowID) != strings.TrimSpace(cachedWindow) {
		return 0, false
	}
	ref, err := resolveA11yTargetRef(refs, selector)
	if err != nil {
		return 0, false
	}
	if _, ok := refMap[ref]; !ok {
		return 0, false
	}
	return ref, true
}

func parseA11ySnapshotEntries(tree string) []a11ySnapshotEntry {
	lines := parseA11ySnapshotLines(tree)
	if len(lines) == 0 {
		return nil
	}
	entries := make([]a11ySnapshotEntry, 0, len(lines))
	for _, entry := range lines {
		if entry.Ref == 0 {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func parseA11ySnapshotLines(tree string) []a11ySnapshotEntry {
	tree = strings.TrimSpace(tree)
	if tree == "" {
		return nil
	}
	lines := strings.Split(tree, "\n")
	entries := make([]a11ySnapshotEntry, 0, len(lines))
	for _, line := range lines {
		entry, ok := parseA11ySnapshotLine(line)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func parseA11ySnapshotEntriesWithTokens(tree string, refMap map[int]string) []a11ySnapshotEntry {
	entries := parseA11ySnapshotEntries(tree)
	if len(entries) == 0 || len(refMap) == 0 {
		return entries
	}
	for idx := range entries {
		entries[idx].Token = strings.TrimSpace(refMap[entries[idx].Ref])
	}
	return entries
}

func parseA11ySnapshotEntry(line string) (a11ySnapshotEntry, bool) {
	entry, ok := parseA11ySnapshotLine(line)
	if !ok || entry.Ref == 0 {
		return a11ySnapshotEntry{}, false
	}
	return entry, true
}

func parseA11ySnapshotLine(line string) (a11ySnapshotEntry, bool) {
	if strings.TrimSpace(line) == "" {
		return a11ySnapshotEntry{}, false
	}
	trimmedLeft := strings.TrimLeft(line, " ")
	indentLen := len(line) - len(trimmedLeft)
	trimmed := strings.TrimSpace(line)
	ref := 0
	body := trimmed
	if strings.HasPrefix(trimmed, "@") {
		spaceIdx := strings.IndexByte(trimmed, ' ')
		if spaceIdx <= 1 {
			return a11ySnapshotEntry{}, false
		}
		parsedRef, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trimmed[:spaceIdx], "@")))
		if err != nil {
			return a11ySnapshotEntry{}, false
		}
		ref = parsedRef
		body = strings.TrimSpace(trimmed[spaceIdx+1:])
	}
	if !strings.HasPrefix(body, "[") {
		return a11ySnapshotEntry{}, false
	}
	roleEnd := strings.IndexByte(body, ']')
	if roleEnd <= 1 {
		return a11ySnapshotEntry{}, false
	}
	entry := a11ySnapshotEntry{
		Ref:   ref,
		Role:  strings.TrimSpace(body[1:roleEnd]),
		Depth: indentLen / 2,
	}
	label := strings.TrimSpace(body[roleEnd+1:])
	if label != "" {
		if unquoted, err := strconv.Unquote(label); err == nil {
			entry.Label = unquoted
		} else {
			entry.Label = label
		}
	}
	return entry, true
}

func (t *A11yTool) resolveActRefFromFullSnapshotByLabel(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, originalErr error) (int, map[int]string, string, error, bool) {
	if !a11yAllowsFullSnapshotLabelFallback(selector, originalErr) {
		return 0, nil, "", nil, false
	}
	result, err := backend.Snapshot(ctx, windowID)
	if err != nil {
		return 0, nil, "", nil, false
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
	t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
	ref, resolveErr := resolveA11yLabelAnchoredTargetRef(parseA11ySnapshotLines(result.Tree), selector)
	if resolveErr != nil {
		return 0, nil, resolvedWindow, resolveErr, true
	}
	return ref, cloneA11yRefMap(result.RefMap), resolvedWindow, nil, true
}

func a11yAllowsFullSnapshotLabelFallback(selector a11yTargetSelector, originalErr error) bool {
	if normalizeA11yTargetName(selector.Name) == "" {
		return false
	}
	runtimeErr, ok := originalErr.(*a11yruntime.RuntimeError)
	if !ok || runtimeErr.Code != "target_not_found" {
		return false
	}
	switch normalizeA11yTargetRole(selector.Role) {
	case "conversation", "chat", "thread", "contact", "selectable", "item", "option", "control", "button", "click_target", "clickable", "setting", "toggle":
		return true
	default:
		return false
	}
}

func resolveA11yLabelAnchoredTargetRef(lines []a11ySnapshotEntry, selector a11yTargetSelector) (int, error) {
	if len(lines) == 0 || normalizeA11yTargetName(selector.Name) == "" {
		return 0, a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil))
	}
	anchors := filterA11ySnapshotLabelAnchors(lines, selector.Name)
	if len(anchors) == 0 {
		return 0, a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil))
	}
	bestRef := 0
	bestScore := 0
	tied := false
	bestMatches := make([]a11ySnapshotEntry, 0, 2)
	for _, anchorIdx := range anchors {
		for targetIdx, target := range lines {
			score := a11yLabelAnchorTargetScore(lines, anchorIdx, targetIdx, selector)
			if score <= 0 {
				continue
			}
			switch {
			case score > bestScore:
				bestRef = target.Ref
				bestScore = score
				tied = false
				bestMatches = bestMatches[:0]
				bestMatches = append(bestMatches, target)
			case score == bestScore:
				if target.Ref != bestRef {
					tied = true
					bestMatches = append(bestMatches, target)
				}
			}
		}
	}
	if bestScore <= 0 || bestRef == 0 {
		return 0, a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil))
	}
	if tied {
		return 0, a11yruntime.NewError("ambiguous_target", "target selector matched multiple interactive elements", a11yTargetSelectorDetails(selector, bestMatches))
	}
	return bestRef, nil
}

func filterA11ySnapshotLabelAnchors(lines []a11ySnapshotEntry, query string) []int {
	exact := make([]int, 0, 2)
	normalizedQuery := normalizeA11yTargetName(query)
	for idx, line := range lines {
		if normalizedQuery == "" || normalizeA11yTargetName(line.Label) != normalizedQuery {
			continue
		}
		exact = append(exact, idx)
	}
	if len(exact) > 0 {
		return exact
	}
	terms := parseA11yTargetMatchTerms(query)
	if len(terms) == 0 {
		return nil
	}
	bestScore := 0
	best := make([]int, 0, 2)
	for idx, line := range lines {
		score := a11yTargetNameMatchScore(line.Label, terms)
		if score <= 0 {
			continue
		}
		switch {
		case score > bestScore:
			bestScore = score
			best = best[:0]
			best = append(best, idx)
		case score == bestScore:
			best = append(best, idx)
		}
	}
	return best
}

func a11yLabelAnchorTargetScore(lines []a11ySnapshotEntry, anchorIdx int, targetIdx int, selector a11yTargetSelector) int {
	if anchorIdx < 0 || anchorIdx >= len(lines) || targetIdx < 0 || targetIdx >= len(lines) {
		return 0
	}
	target := lines[targetIdx]
	if target.Ref == 0 || !a11ySnapshotRoleMatches(target.Role, normalizeA11yTargetRole(selector.Role)) {
		return 0
	}
	score := preferredScenarioEntryScore(target, selector.Role)
	if score <= 0 {
		return 0
	}
	distance := targetIdx - anchorIdx
	if distance == 0 {
		return 0
	}
	absDistance := distance
	if absDistance < 0 {
		absDistance = -absDistance
	}
	if absDistance > 6 {
		return 0
	}
	switch absDistance {
	case 1:
		score += 140
	case 2:
		score += 110
	case 3:
		score += 80
	case 4:
		score += 50
	case 5:
		score += 25
	case 6:
		score += 10
	}
	if distance > 0 {
		score += 15
	} else {
		score += 5
	}
	depthDelta := target.Depth - lines[anchorIdx].Depth
	if depthDelta < 0 {
		depthDelta = -depthDelta
	}
	if depthDelta > 2 {
		return 0
	}
	switch depthDelta {
	case 0:
		score += 50
	case 1:
		score += 35
	case 2:
		score += 15
	}
	if a11ySnapshotLineParentIndex(lines, anchorIdx) == a11ySnapshotLineParentIndex(lines, targetIdx) {
		score += 20
	}
	return score
}

func a11ySnapshotLineParentIndex(lines []a11ySnapshotEntry, idx int) int {
	if idx <= 0 || idx >= len(lines) {
		return -1
	}
	depth := lines[idx].Depth
	for i := idx - 1; i >= 0; i-- {
		if lines[i].Depth < depth {
			return i
		}
	}
	return -1
}

func resolveA11yTargetRef(entries []a11ySnapshotEntry, selector a11yTargetSelector) (int, error) {
	if !selector.Provided() {
		return 0, a11yruntime.NewError("unsupported_action", "target selector requires target_name or target_role", nil)
	}
	matches := filterA11yTargetEntriesByResolvedSelector(entries, selector)
	switch len(matches) {
	case 0:
		return 0, a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil))
	case 1:
		return matches[0].Ref, nil
	default:
		if ref, ok := resolvePreferredA11yTargetRef(entries, matches, selector); ok {
			return ref, nil
		}
		return 0, a11yruntime.NewError("ambiguous_target", "target selector matched multiple interactive elements", a11yTargetSelectorDetails(selector, matches))
	}
}

func filterA11yTargetEntriesByResolvedSelector(entries []a11ySnapshotEntry, selector a11yTargetSelector) []a11ySnapshotEntry {
	exactMatches := filterA11yTargetEntriesByExactSelector(entries, selector)
	if len(exactMatches) > 0 {
		return exactMatches
	}
	if normalizeA11yTargetName(selector.Name) == "" {
		return exactMatches
	}
	return filterA11yTargetEntriesByBestFuzzySelector(entries, selector)
}

func filterA11yTargetEntriesByExactSelector(entries []a11ySnapshotEntry, selector a11yTargetSelector) []a11ySnapshotEntry {
	matches := make([]a11ySnapshotEntry, 0, 4)
	for _, entry := range entries {
		if !a11ySnapshotEntryMatchesExactSelector(entry, selector) {
			continue
		}
		matches = append(matches, entry)
	}
	return matches
}

func filterA11yTargetEntriesByBestFuzzySelector(entries []a11ySnapshotEntry, selector a11yTargetSelector) []a11ySnapshotEntry {
	terms := parseA11yTargetMatchTerms(selector.Name)
	if len(terms) == 0 {
		return nil
	}
	bestScore := 0
	matches := make([]a11ySnapshotEntry, 0, 4)
	for _, entry := range entries {
		score := a11ySnapshotEntryFuzzyNameMatchScore(entry, selector, terms)
		if score <= 0 {
			continue
		}
		switch {
		case score > bestScore:
			bestScore = score
			matches = matches[:0]
			matches = append(matches, entry)
		case score == bestScore:
			matches = append(matches, entry)
		}
	}
	return matches
}

func resolvePreferredA11yTargetRef(entries []a11ySnapshotEntry, matches []a11ySnapshotEntry, selector a11yTargetSelector) (int, bool) {
	if selectorAllowsPreferredInputResolution(selector) {
		return resolvePreferredInputTargetRef(entries, matches)
	}
	return resolvePreferredScenarioTargetRef(matches, selector)
}

func resolvePreferredInputTargetRef(entries []a11ySnapshotEntry, matches []a11ySnapshotEntry) (int, bool) {
	entryOrder := make(map[int]int, len(entries))
	for idx, entry := range entries {
		entryOrder[entry.Ref] = idx
	}
	submitPositions := likelySubmitEntryPositions(entries)
	bestRef := 0
	bestScore := 0
	tied := false
	for idx, entry := range matches {
		score := preferredInputEntryScore(entry) + preferredInputSubmitProximityBonus(entryOrder[entry.Ref], submitPositions)
		if idx == 0 || score > bestScore {
			bestRef = entry.Ref
			bestScore = score
			tied = false
			continue
		}
		if score == bestScore {
			tied = true
		}
	}
	if tied || bestRef == 0 {
		return 0, false
	}
	return bestRef, true
}

func resolvePreferredScenarioTargetRef(matches []a11ySnapshotEntry, selector a11yTargetSelector) (int, bool) {
	bestRef := 0
	bestScore := 0
	tied := false
	for _, entry := range matches {
		score := preferredScenarioEntryScore(entry, selector.Role)
		if score <= 0 {
			return 0, false
		}
		switch {
		case score > bestScore:
			bestRef = entry.Ref
			bestScore = score
			tied = false
		case score == bestScore:
			tied = true
		}
	}
	if tied || bestRef == 0 {
		return 0, false
	}
	return bestRef, true
}

func likelySubmitEntryPositions(entries []a11ySnapshotEntry) []int {
	positions := make([]int, 0, 2)
	for idx, entry := range entries {
		if likelySubmitEntryScore(entry) > 0 {
			positions = append(positions, idx)
		}
	}
	return positions
}

func preferredInputSubmitProximityBonus(entryIndex int, submitPositions []int) int {
	if entryIndex < 0 || len(submitPositions) == 0 {
		return 0
	}
	bestDistance := -1
	for _, submitIndex := range submitPositions {
		distance := submitIndex - entryIndex
		if distance < 0 {
			distance = -distance
		}
		if bestDistance < 0 || distance < bestDistance {
			bestDistance = distance
		}
	}
	switch bestDistance {
	case 1:
		return 40
	case 2:
		return 20
	case 3:
		return 10
	default:
		return 0
	}
}

func a11ySnapshotEntryMatchesExactSelector(entry a11ySnapshotEntry, selector a11yTargetSelector) bool {
	if role := normalizeA11yTargetRole(selector.Role); role != "" && !a11ySnapshotRoleMatches(entry.Role, role) {
		return false
	}
	if name := normalizeA11yTargetName(selector.Name); name != "" && normalizeA11yTargetName(entry.Label) != name {
		return false
	}
	return true
}

func a11ySnapshotEntryFuzzyNameMatchScore(entry a11ySnapshotEntry, selector a11yTargetSelector, terms []string) int {
	if role := normalizeA11yTargetRole(selector.Role); role != "" && !a11ySnapshotRoleMatches(entry.Role, role) {
		return 0
	}
	return a11yTargetNameMatchScore(entry.Label, terms)
}

func selectorAllowsPreferredInputResolution(selector a11yTargetSelector) bool {
	if normalizeA11yTargetName(selector.Name) != "" {
		return false
	}
	switch normalizeA11yTargetRole(selector.Role) {
	case "input", "text_input", "text", "editor":
		return true
	default:
		return false
	}
}

func preferredInputEntryScore(entry a11ySnapshotEntry) int {
	role := normalizeA11yTargetRole(entry.Role)
	label := normalizeA11yTargetName(entry.Label)
	score := 0

	switch role {
	case "document":
		score += 140
	case "editable_text", "text_area", "editor":
		score += 130
	case "text_field", "combo_box":
		score += 110
	case "search_field", "search":
		score += 60
	default:
		score += 40
	}

	if label != "" {
		score += 5
	}
	if a11yTargetLabelContainsAny(label, "message", "reply", "chat", "compose", "write", "comment", "say", "消息", "回复", "聊天", "编写", "输入", "评论") {
		score += 80
	}
	if a11yTargetLabelContainsAny(label, "search", "find", "address", "url", "lookup", "搜索", "查找", "地址", "网址") {
		score -= 80
	}
	return score
}

func preferredScenarioEntryScore(entry a11ySnapshotEntry, selectorRole string) int {
	switch normalizeA11yTargetRole(selectorRole) {
	case "conversation", "chat", "thread", "contact":
		return preferredConversationEntryScore(entry)
	case "control", "button", "click_target", "clickable":
		return preferredControlEntryScore(entry)
	case "selectable", "item", "option":
		return preferredSelectableEntryScore(entry)
	case "setting", "toggle":
		return preferredSettingEntryScore(entry)
	default:
		return 0
	}
}

func preferredConversationEntryScore(entry a11ySnapshotEntry) int {
	switch normalizeA11yTargetRole(entry.Role) {
	case "list_item":
		return 140
	case "tree_item":
		return 135
	case "row":
		return 130
	case "cell":
		return 125
	case "tab":
		return 120
	case "button", "push_button":
		return 90
	case "link":
		return 80
	default:
		return 0
	}
}

func preferredControlEntryScore(entry a11ySnapshotEntry) int {
	switch normalizeA11yTargetRole(entry.Role) {
	case "button", "push_button":
		return 140
	case "link":
		return 125
	case "menu_item":
		return 120
	case "tab":
		return 110
	case "toggle_button":
		return 105
	case "switch", "check_box", "radio_button":
		return 100
	case "list_item", "tree_item":
		return 85
	case "row", "cell":
		return 80
	default:
		return 0
	}
}

func preferredSelectableEntryScore(entry a11ySnapshotEntry) int {
	switch normalizeA11yTargetRole(entry.Role) {
	case "list_item":
		return 140
	case "tree_item":
		return 135
	case "row":
		return 130
	case "cell":
		return 125
	case "tab":
		return 120
	case "radio_button":
		return 115
	case "menu_item":
		return 110
	case "button", "push_button":
		return 90
	case "link":
		return 80
	default:
		return 0
	}
}

func preferredSettingEntryScore(entry a11ySnapshotEntry) int {
	switch normalizeA11yTargetRole(entry.Role) {
	case "switch":
		return 140
	case "check_box":
		return 130
	case "toggle_button":
		return 125
	case "radio_button":
		return 120
	case "menu_item":
		return 90
	default:
		return 0
	}
}

func normalizeA11yTargetName(value string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	return strings.Join(fields, " ")
}

func parseA11yTargetMatchTerms(query string) []string {
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
		normalized := normalizeA11yTargetName(part)
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

func a11yTargetNameMatchScore(value string, terms []string) int {
	normalizedValue := normalizeA11yTargetName(value)
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

func normalizeA11yTargetRole(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")
	fields := strings.Fields(normalized)
	return strings.Join(fields, "_")
}

func a11ySnapshotRoleMatches(role string, selectorRole string) bool {
	role = normalizeA11yTargetRole(role)
	if selectorRole == "" {
		return true
	}
	switch selectorRole {
	case "input", "text_input", "text", "editor":
		return a11ySnapshotRoleIsInput(role)
	case "control", "button", "click_target", "clickable":
		return a11ySnapshotRoleIsControl(role)
	case "selectable", "item", "option":
		return a11ySnapshotRoleIsSelectable(role)
	case "setting", "toggle":
		return a11ySnapshotRoleIsSetting(role)
	case "conversation", "chat", "thread", "contact":
		return a11ySnapshotRoleIsConversation(role)
	}
	return role == selectorRole
}

func a11ySnapshotRoleIsInput(role string) bool {
	switch normalizeA11yTargetRole(role) {
	case "editable_text", "text_field", "text_area", "search_field", "combo_box", "document", "search", "editor":
		return true
	default:
		return false
	}
}

func a11ySnapshotRoleIsControl(role string) bool {
	switch normalizeA11yTargetRole(role) {
	case "button", "push_button", "link", "menu_item", "list_item", "tree_item", "row", "cell", "tab", "check_box", "radio_button", "switch", "toggle_button":
		return true
	default:
		return false
	}
}

func a11ySnapshotRoleIsSelectable(role string) bool {
	switch normalizeA11yTargetRole(role) {
	case "list_item", "tree_item", "row", "cell", "tab", "radio_button", "menu_item", "button", "push_button", "link":
		return true
	default:
		return false
	}
}

func a11ySnapshotRoleIsSetting(role string) bool {
	switch normalizeA11yTargetRole(role) {
	case "switch", "check_box", "radio_button", "toggle_button", "menu_item":
		return true
	default:
		return false
	}
}

func a11ySnapshotRoleIsConversation(role string) bool {
	switch normalizeA11yTargetRole(role) {
	case "list_item", "tree_item", "row", "cell", "tab":
		return true
	default:
		return false
	}
}

func normalizeA11yActIntent(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.NewReplacer(" ", "", "_", "", "-", "").Replace(normalized)
	switch normalized {
	case "":
		return ""
	case "select", "choose", "pick", "activateitem", "switchto", "切换到", "选择":
		return "select"
	case "click", "tap", "press", "open", "activate", "点击":
		return "click"
	case "toggle", "switch", "setting", "settingswitch", "切换", "开关":
		return "toggle"
	case "message", "chat", "thread", "conversation", "contact", "reply", "sendmessage", "sayhello", "消息", "聊天", "会话", "对话":
		return "message"
	default:
		return ""
	}
}

func inferA11yActIntent(args map[string]interface{}, explicitIntent string, value string) string {
	if intent := normalizeA11yActIntent(explicitIntent); intent != "" {
		return intent
	}
	if firstCompatString(args, "conversation", "thread", "chat", "contact") != "" && strings.TrimSpace(value) != "" {
		return "message"
	}
	if firstCompatString(args, "conversation", "thread", "chat", "contact", "item", "option", "choice") != "" {
		return "select"
	}
	if firstCompatString(args, "setting", "switch", "toggle") != "" {
		return "toggle"
	}
	if firstCompatString(args, "control", "button") != "" {
		return "click"
	}
	return ""
}

func a11yIntentDefaultActType(intent string) string {
	switch normalizeA11yActIntent(intent) {
	case "select":
		return "select"
	case "click":
		return "click"
	case "toggle":
		return "toggle"
	case "message":
		return "type"
	default:
		return ""
	}
}

func resolveA11yActTargetSelector(args map[string]interface{}, actType string, intent string) a11yTargetSelector {
	selector := a11yTargetSelector{
		Name: firstCompatString(args, "target_name", "targetName"),
		Role: firstCompatString(args, "target_role", "targetRole"),
	}
	if selector.Provided() {
		return selector
	}
	switch normalizeA11yActIntent(intent) {
	case "select":
		if name := firstCompatString(args, "conversation", "thread", "chat", "contact"); name != "" {
			return a11yTargetSelector{Name: name, Role: "conversation"}
		}
		if name := firstCompatString(args, "item", "option", "choice", "target", "element"); name != "" {
			return a11yTargetSelector{Name: name, Role: "selectable"}
		}
	case "click":
		if name := firstCompatString(args, "control", "button", "target", "element", "item"); name != "" {
			return a11yTargetSelector{Name: name, Role: "control"}
		}
	case "toggle":
		if name := firstCompatString(args, "setting", "switch", "toggle", "target", "element", "item"); name != "" {
			return a11yTargetSelector{Name: name, Role: "setting"}
		}
	case "message":
		if name := firstCompatString(args, "input", "editor", "composer", "field"); name != "" {
			return a11yTargetSelector{Name: name, Role: "input"}
		}
	}
	if strings.EqualFold(strings.TrimSpace(actType), "type") {
		return a11yTargetSelector{Role: "input"}
	}
	return a11yTargetSelector{}
}

func resolveA11yConversationSelector(args map[string]interface{}, intent string) a11yTargetSelector {
	if normalizeA11yActIntent(intent) != "message" {
		return a11yTargetSelector{}
	}
	name := firstCompatString(args, "conversation", "thread", "chat", "contact")
	if name == "" {
		return a11yTargetSelector{}
	}
	return a11yTargetSelector{Name: name, Role: "conversation"}
}

func (t *A11yTool) maybeActivateA11yMessageConversation(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, holdMS int, intent string) (string, error) {
	selector := resolveA11yConversationSelector(args, intent)
	if !selector.Provided() {
		return strings.TrimSpace(windowID), nil
	}
	if fastWindow, fastErr, handled := t.tryA11yOrcaConversationFastPath(ctx, backend, args, strings.TrimSpace(windowID), selector, holdMS); handled {
		if fastErr != nil {
			return "", enrichA11yActionPhaseError(fastErr, "conversation", false)
		}
		return fastWindow, nil
	}
	ref, refMap, resolvedWindow, err := t.resolveActRefByTarget(ctx, backend, windowID, selector)
	if err != nil {
		if fastWindow, fastErr, handled := t.tryA11yConversationSearchFallback(ctx, backend, args, strings.TrimSpace(valueOrDefault(resolvedWindow, windowID)), selector, holdMS, err); handled {
			if fastErr != nil {
				return "", enrichA11yActionPhaseError(fastErr, "conversation", false)
			}
			return fastWindow, nil
		}
		return "", enrichA11yActionPhaseError(err, "conversation", false)
	}
	confirmedWindow, confirmErr := t.activateA11yMessageConversationTarget(ctx, backend, resolvedWindow, ref, refMap, selector, holdMS)
	if confirmErr != nil {
		return "", enrichA11yActionPhaseError(confirmErr, "conversation", false)
	}
	return confirmedWindow, nil
}

func (t *A11yTool) tryA11yOrcaConversationFastPath(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, selector a11yTargetSelector, holdMS int) (string, error, bool) {
	if !a11yAllowsOrcaConversationFastPath(backend, args, selector) {
		return "", nil, false
	}
	resolvedWindow := strings.TrimSpace(windowID)
	cacheKey := t.a11yConversationClickCacheKeyForArgs(args, selector, resolvedWindow)
	if cachedPoint, ok := t.a11yConversationClickCacheGet(cacheKey); ok {
		nextWindow, err := t.tryA11yConversationPointClick(ctx, backend, resolvedWindow, selector, holdMS, cachedPoint)
		if err == nil {
			return nextWindow, nil, true
		}
		t.a11yConversationClickCacheDelete(cacheKey)
		if strings.TrimSpace(nextWindow) != "" {
			resolvedWindow = strings.TrimSpace(nextWindow)
		}
	}
	plans := a11yConversationSearchPlans(backend.HostOS())
	if len(plans) == 0 {
		return "", a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil)), true
	}
	var lastErr error = a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil))
	for _, plan := range plans {
		nextWindow, point, err := t.executeA11yOrcaConversationSearchPlan(ctx, backend, resolvedWindow, selector, holdMS, plan)
		if err == nil {
			if point != nil {
				t.a11yConversationClickCacheSet(cacheKey, *point)
			}
			return nextWindow, nil, true
		}
		if !a11yIsTargetNotFound(err) {
			return "", err, true
		}
		lastErr = err
		if strings.TrimSpace(nextWindow) != "" {
			resolvedWindow = strings.TrimSpace(nextWindow)
		}
	}
	return "", lastErr, true
}

func (t *A11yTool) tryA11yConversationSearchFallback(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, selector a11yTargetSelector, holdMS int, originalErr error) (string, error, bool) {
	if !a11yAllowsConversationSearchFallback(args, originalErr) {
		return "", nil, false
	}
	plans := a11yConversationSearchPlans(backend.HostOS())
	if len(plans) == 0 {
		return "", nil, false
	}
	resolvedWindow := strings.TrimSpace(windowID)
	lastErr := originalErr
	for _, plan := range plans {
		nextWindow, err := t.executeA11yConversationSearchPlan(ctx, backend, resolvedWindow, selector, holdMS, plan)
		if err == nil {
			return nextWindow, nil, true
		}
		lastErr = err
		if strings.TrimSpace(nextWindow) != "" {
			resolvedWindow = strings.TrimSpace(nextWindow)
		}
	}
	return "", lastErr, true
}

func (t *A11yTool) executeA11yConversationSearchPlan(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, holdMS int, plan a11yConversationSearchPlan) (string, error) {
	resolvedWindow := strings.TrimSpace(windowID)
	var err error
	if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, plan.Open, holdMS); err != nil {
		return resolvedWindow, err
	}
	clearSequences := a11yConversationSearchClearSequences(backend.HostOS())
	if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, clearSequences, holdMS); err != nil {
		return resolvedWindow, err
	}
	if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, [][]string{{selector.Name}}, holdMS); err != nil {
		return resolvedWindow, err
	}
	ref, refMap, resolvedWindow, err := t.resolveActRefByTarget(ctx, backend, resolvedWindow, selector)
	if err != nil {
		return resolvedWindow, err
	}
	return t.activateA11yMessageConversationTarget(ctx, backend, resolvedWindow, ref, refMap, selector, holdMS)
}

func (t *A11yTool) executeA11yOrcaConversationSearchPlan(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, holdMS int, plan a11yConversationSearchPlan) (string, *a11yConversationClickPoint, error) {
	resolvedWindow := strings.TrimSpace(windowID)
	var err error
	if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, plan.Open, holdMS); err != nil {
		return resolvedWindow, nil, err
	}
	clearSequences := a11yConversationSearchClearSequences(backend.HostOS())
	if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, clearSequences, holdMS); err != nil {
		return resolvedWindow, nil, err
	}
	if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, [][]string{{selector.Name}}, holdMS); err != nil {
		return resolvedWindow, nil, err
	}
	ref, refMap, resolvedWindow, err := t.resolveActRefByTarget(ctx, backend, resolvedWindow, selector)
	if err == nil {
		nextWindow, confirmErr := t.activateA11yMessageConversationTarget(ctx, backend, resolvedWindow, ref, refMap, selector, holdMS)
		return nextWindow, nil, confirmErr
	}
	if !a11yIsTargetNotFound(err) {
		return resolvedWindow, nil, err
	}
	hit, locateErr := t.locateA11yConversationVisualHit(ctx, backend, resolvedWindow, selector)
	if locateErr != nil {
		return resolvedWindow, nil, locateErr
	}
	nextWindow, clickErr := t.tryA11yConversationPointClick(ctx, backend, resolvedWindow, selector, holdMS, a11yConversationClickPoint{
		X: hit.Point.X,
		Y: hit.Point.Y,
	})
	if clickErr != nil {
		return nextWindow, nil, clickErr
	}
	return nextWindow, &a11yConversationClickPoint{X: hit.Point.X, Y: hit.Point.Y}, nil
}

func (t *A11yTool) sendA11yConversationSearchKeySequences(ctx context.Context, backend a11yruntime.Backend, windowID string, sequences [][]string, holdMS int) (string, error) {
	resolvedWindow := strings.TrimSpace(windowID)
	for _, keys := range sequences {
		if len(keys) == 0 {
			continue
		}
		result, err := backend.Key(ctx, resolvedWindow, keys, holdMS)
		if err != nil {
			return resolvedWindow, err
		}
		resolvedWindow = strings.TrimSpace(valueOrDefault(result.WindowID, resolvedWindow))
		t.syncWindowContext(resolvedWindow)
		t.clearSnapshotRefs()
		if err := a11yWaitForConversationSettle(ctx); err != nil {
			return resolvedWindow, err
		}
	}
	return resolvedWindow, nil
}

func (t *A11yTool) activateA11yMessageConversationTarget(ctx context.Context, backend a11yruntime.Backend, windowID string, ref int, refMap map[int]string, selector a11yTargetSelector, holdMS int) (string, error) {
	result, err := backend.Act(ctx, windowID, ref, refMap, "click", "", holdMS)
	if err != nil {
		return "", err
	}
	resolvedWindow := valueOrDefault(result.WindowID, windowID)
	t.syncWindowContext(resolvedWindow)
	t.clearSnapshotRefs()
	if err := a11yWaitForConversationSettle(ctx); err != nil {
		return resolvedWindow, err
	}
	return t.confirmA11yMessageConversationActivated(ctx, backend, resolvedWindow, selector)
}

func (t *A11yTool) tryA11yConversationPointClick(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, holdMS int, point a11yConversationClickPoint) (string, error) {
	result, err := backend.ClickWindowPoint(ctx, windowID, a11yruntime.NormalizedPoint{X: point.X, Y: point.Y}, holdMS)
	if err != nil {
		return "", err
	}
	resolvedWindow := valueOrDefault(result.WindowID, windowID)
	t.syncWindowContext(resolvedWindow)
	t.clearSnapshotRefs()
	if err := a11yWaitForConversationSettle(ctx); err != nil {
		return resolvedWindow, err
	}
	return t.confirmA11yMessageConversationActivated(ctx, backend, resolvedWindow, selector)
}

func (t *A11yTool) locateA11yConversationVisualHit(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector) (a11yConversationVisualHit, error) {
	result, err := backend.Screenshot(ctx, windowID)
	if err != nil {
		return a11yConversationVisualHit{}, a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", a11yTargetSelectorDetails(selector, nil))
	}
	imagePath := strings.TrimSpace(result.ImagePath)
	if imagePath == "" {
		return a11yConversationVisualHit{}, a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", a11yTargetSelectorDetails(selector, nil))
	}
	hit, err := a11yLocateConversationVisualHit(ctx, imagePath, selector.Name)
	if err != nil {
		if runtimeErr, ok := err.(*a11yruntime.RuntimeError); ok {
			return a11yConversationVisualHit{}, runtimeErr
		}
		return a11yConversationVisualHit{}, a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", a11yTargetSelectorDetails(selector, nil))
	}
	return hit, nil
}

func a11yAllowsConversationSearchFallback(args map[string]interface{}, err error) bool {
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok || runtimeErr.Code != "target_not_found" {
		return false
	}
	return a11yLooksLikeFeishuWindowQuery(firstCompatString(args, "app_name", "appName", "application", "app")) ||
		a11yLooksLikeFeishuWindowQuery(firstCompatString(args, "window_title", "windowTitle", "title"))
}

func a11yLooksLikeFeishuWindowQuery(value string) bool {
	normalizedValue := normalizeA11yWindowMatchValue(value)
	if normalizedValue == "" {
		return false
	}
	for _, alias := range []string{"feishu", "飞书", "lark"} {
		if strings.Contains(normalizedValue, normalizeA11yWindowMatchValue(alias)) {
			return true
		}
	}
	return false
}

func a11yAllowsOrcaConversationFastPath(backend a11yruntime.Backend, args map[string]interface{}, selector a11yTargetSelector) bool {
	if backend == nil || !strings.EqualFold(strings.TrimSpace(backend.HostOS()), "darwin") {
		return false
	}
	if normalizeA11yTargetName(selector.Name) != normalizeA11yTargetName("Orca") {
		return false
	}
	return a11yLooksLikeFeishuWindowQuery(firstCompatString(args, "app_name", "appName", "application", "app")) ||
		a11yLooksLikeFeishuWindowQuery(firstCompatString(args, "window_title", "windowTitle", "title"))
}

func a11yIsTargetNotFound(err error) bool {
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	return ok && runtimeErr.Code == "target_not_found"
}

func a11yConversationSearchPlans(hostOS string) []a11yConversationSearchPlan {
	modifier := "command"
	if strings.EqualFold(strings.TrimSpace(hostOS), "windows") {
		modifier = "ctrl"
	}
	return []a11yConversationSearchPlan{
		{Open: [][]string{{modifier, "k"}}},
		{Open: [][]string{{modifier, "f"}, {modifier, "f"}}},
	}
}

func a11yConversationSearchClearSequences(hostOS string) [][]string {
	modifier := "command"
	if strings.EqualFold(strings.TrimSpace(hostOS), "windows") {
		modifier = "ctrl"
	}
	return [][]string{
		{modifier, "a"},
		{"delete"},
	}
}

func a11yWaitForConversationSettle(ctx context.Context) error {
	if a11yMessageConversationSettleDelay <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(a11yMessageConversationSettleDelay):
		return nil
	}
}

func a11yWaitForPollInterval(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(interval):
		return nil
	}
}

func (t *A11yTool) confirmA11yMessageConversationActivated(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector) (string, error) {
	timeout := a11yMessageConversationConfirmationTimeout
	deadline := time.Now().Add(timeout)
	for {
		result, err := backend.SnapshotInteractive(ctx, windowID)
		if err != nil {
			return "", err
		}
		resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
		t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
		entries := parseA11ySnapshotEntriesWithTokens(result.Tree, result.RefMap)
		if !a11ySnapshotHasPendingConversationSearch(entries, selector.Name) && a11ySnapshotHasComposer(entries) {
			return resolvedWindow, nil
		}
		if timeout <= 0 || time.Now().After(deadline) {
			return "", a11yConversationConfirmationError(selector)
		}
		if err := a11yWaitForPollInterval(ctx, a11yMessageConversationConfirmationPollInterval); err != nil {
			return "", err
		}
	}
}

func a11ySnapshotHasComposer(entries []a11ySnapshotEntry) bool {
	for _, entry := range entries {
		if a11ySnapshotRoleCouldBeComposer(entry.Role, entry.Label) {
			return true
		}
	}
	return false
}

func a11ySnapshotHasPendingConversationSearch(entries []a11ySnapshotEntry, selectorName string) bool {
	terms := parseA11yTargetMatchTerms(selectorName)
	if len(entries) == 0 || len(terms) == 0 {
		return false
	}
	for _, entry := range entries {
		if !a11ySnapshotRoleLooksLikeConversationSearch(entry.Role, entry.Label) {
			continue
		}
		if a11yTargetNameMatchScore(entry.Label, terms) > 0 {
			return true
		}
	}
	return false
}

func a11ySnapshotRoleLooksLikeConversationSearch(role string, label string) bool {
	switch normalizeA11yTargetRole(role) {
	case "search_field", "search":
		return true
	case "text_field", "combo_box":
		return a11yTargetLabelContainsAny(normalizeA11yTargetName(label), "search", "find", "lookup", "搜索", "查找")
	default:
		return false
	}
}

func a11yConversationConfirmationError(selector a11yTargetSelector) error {
	details := a11yTargetSelectorDetails(selector, nil)
	details["confirmation"] = "composer_not_ready"
	return a11yruntime.NewError("confirmation_failed", "conversation switch could not be confirmed", details)
}

func a11yTargetLabelContainsAny(value string, terms ...string) bool {
	if value == "" {
		return false
	}
	for _, term := range terms {
		normalized := normalizeA11yTargetName(term)
		if normalized == "" {
			continue
		}
		if strings.Contains(value, normalized) {
			return true
		}
	}
	return false
}

func a11yTargetSelectorDetails(selector a11yTargetSelector, matches []a11ySnapshotEntry) map[string]interface{} {
	details := map[string]interface{}{}
	if name := strings.TrimSpace(selector.Name); name != "" {
		details["target_name"] = name
	}
	if role := strings.TrimSpace(selector.Role); role != "" {
		details["target_role"] = role
	}
	if len(matches) == 0 {
		return details
	}
	details["matches"] = len(matches)
	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		if strings.TrimSpace(match.Label) != "" {
			refs = append(refs, fmt.Sprintf("@%d [%s] %q", match.Ref, strings.TrimSpace(match.Role), strings.TrimSpace(match.Label)))
			continue
		}
		refs = append(refs, fmt.Sprintf("@%d [%s]", match.Ref, strings.TrimSpace(match.Role)))
	}
	details["matching_refs"] = refs
	return details
}

func (t *A11yTool) resolveLikelySubmitRef(ctx context.Context, backend a11yruntime.Backend, windowID string, preferredInputRef int) (int, map[int]string, bool) {
	t.mu.RLock()
	cachedWindow := strings.TrimSpace(t.lastWindow)
	cachedRefMap := cloneA11yRefMap(t.lastRefMap)
	cachedRefs := cloneA11ySnapshotEntries(t.lastRefs)
	t.mu.RUnlock()

	if ref, ok := resolveLikelySubmitRefFromSnapshot(windowID, cachedWindow, cachedRefMap, cachedRefs, preferredInputRef); ok {
		return ref, cachedRefMap, true
	}

	snapshotTarget := strings.TrimSpace(windowID)
	if snapshotTarget == "" {
		snapshotTarget = cachedWindow
	}
	result, err := backend.SnapshotInteractive(ctx, snapshotTarget)
	if err != nil {
		return 0, nil, false
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, snapshotTarget))
	t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
	ref, ok := resolveLikelySubmitRefFromSnapshot(resolvedWindow, resolvedWindow, result.RefMap, parseA11ySnapshotEntries(result.Tree), preferredInputRef)
	if !ok {
		return 0, nil, false
	}
	return ref, cloneA11yRefMap(result.RefMap), true
}

func resolveLikelySubmitRefFromSnapshot(windowID string, cachedWindow string, refMap map[int]string, refs []a11ySnapshotEntry, preferredInputRef int) (int, bool) {
	if len(refMap) == 0 || len(refs) == 0 {
		return 0, false
	}
	if strings.TrimSpace(windowID) != "" && strings.TrimSpace(cachedWindow) != "" && strings.TrimSpace(windowID) != strings.TrimSpace(cachedWindow) {
		return 0, false
	}
	ref, ok := resolveLikelySubmitRefFromEntries(refs, preferredInputRef)
	if !ok {
		return 0, false
	}
	if _, ok := refMap[ref]; !ok {
		return 0, false
	}
	return ref, true
}

func resolveLikelySubmitRefFromEntries(entries []a11ySnapshotEntry, preferredInputRef int) (int, bool) {
	entryOrder := make(map[int]int, len(entries))
	for idx, entry := range entries {
		entryOrder[entry.Ref] = idx
	}
	preferredInputIndex, hasPreferredInput := entryOrder[preferredInputRef]
	bestRef := 0
	bestScore := 0
	bestCount := 0
	for _, entry := range entries {
		score := likelySubmitEntryScore(entry)
		if score > 0 && hasPreferredInput {
			score += preferredInputSubmitProximityBonus(entryOrder[entry.Ref], []int{preferredInputIndex})
		}
		if score <= 0 {
			continue
		}
		switch {
		case score > bestScore:
			bestRef = entry.Ref
			bestScore = score
			bestCount = 1
		case score == bestScore:
			bestCount++
		}
	}
	if bestScore <= 0 || bestCount != 1 {
		return 0, false
	}
	return bestRef, true
}

func likelySubmitEntryScore(entry a11ySnapshotEntry) int {
	role := normalizeA11yTargetRole(entry.Role)
	label := normalizeA11yTargetName(entry.Label)
	if label == "" {
		return 0
	}
	if !a11ySnapshotRoleIsLikelySubmit(role) {
		return 0
	}
	score := 0
	switch {
	case a11yTargetNameEqualsAny(label, "send", "发送", "reply", "回复", "submit", "提交"):
		score += 100
	case a11yTargetNameContainsAny(label, "send", "发送", "reply", "回复", "submit", "提交", "send message", "发送消息"):
		score += 70
	default:
		return 0
	}
	switch role {
	case "button", "push_button":
		score += 20
	case "link", "menu_item":
		score += 10
	}
	return score
}

func a11ySnapshotRoleIsLikelySubmit(role string) bool {
	switch normalizeA11yTargetRole(role) {
	case "button", "push_button", "link", "menu_item":
		return true
	default:
		return false
	}
}

func a11yTargetNameEqualsAny(value string, options ...string) bool {
	value = normalizeA11yTargetName(value)
	for _, option := range options {
		if value == normalizeA11yTargetName(option) {
			return true
		}
	}
	return false
}

func a11yTargetNameContainsAny(value string, options ...string) bool {
	value = normalizeA11yTargetName(value)
	for _, option := range options {
		if strings.Contains(value, normalizeA11yTargetName(option)) {
			return true
		}
	}
	return false
}

func (t *A11yTool) resolveActSubmitPlan(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, actType string, preferredInputRef int, intent string) (a11yActSubmitPlan, error) {
	submitRequested, submitProvided := compatBoolArg(args, "submit")
	submitKeys, hasSubmitKeys := compatStringSlice(args, "submit_keys", "submitKeys")
	selector := a11yTargetSelector{
		Name: firstCompatString(args, "submit_target_name", "submitTargetName"),
		Role: firstCompatString(args, "submit_target_role", "submitTargetRole"),
	}
	if !submitProvided && normalizeA11yActIntent(intent) == "message" {
		submitRequested = true
	}
	if strings.TrimSpace(strings.ToLower(actType)) != "type" {
		if submitRequested || hasSubmitKeys || selector.Provided() {
			return a11yActSubmitPlan{}, a11yruntime.NewError("unsupported_action", "submit options are only supported with act_type=type", map[string]interface{}{"act_type": actType})
		}
		return a11yActSubmitPlan{}, nil
	}
	if !submitRequested && !hasSubmitKeys && !selector.Provided() {
		return a11yActSubmitPlan{}, nil
	}
	if selector.Provided() {
		ref, refMap, _, err := t.resolveActRefByTarget(ctx, backend, windowID, selector)
		if err != nil {
			return a11yActSubmitPlan{}, enrichA11ySubmitPhaseError(err)
		}
		return a11yActSubmitPlan{
			Enabled:      true,
			Ref:          ref,
			RefMap:       refMap,
			KeySequences: defaultA11ySubmitKeySequences(backend.HostOS()),
		}, nil
	}
	if hasSubmitKeys {
		return a11yActSubmitPlan{
			Enabled:      true,
			KeySequences: [][]string{append([]string(nil), submitKeys...)},
		}, nil
	}
	if ref, refMap, ok := t.resolveLikelySubmitRef(ctx, backend, windowID, preferredInputRef); ok {
		return a11yActSubmitPlan{
			Enabled:      true,
			Ref:          ref,
			RefMap:       refMap,
			KeySequences: defaultA11ySubmitKeySequences(backend.HostOS()),
		}, nil
	}
	return a11yActSubmitPlan{
		Enabled:      true,
		KeySequences: defaultA11ySubmitKeySequences(backend.HostOS()),
	}, nil
}

func (t *A11yTool) executeActSubmitPlan(ctx context.Context, backend a11yruntime.Backend, windowID string, holdMS int, plan a11yActSubmitPlan) (a11yruntime.ActionResult, error) {
	return t.executeActSubmitPlanWithConfirmation(ctx, backend, windowID, holdMS, plan, a11ySubmitConfirmation{})
}

func (t *A11yTool) executeActSubmitPlanWithConfirmation(ctx context.Context, backend a11yruntime.Backend, windowID string, holdMS int, plan a11yActSubmitPlan, confirmation a11ySubmitConfirmation) (a11yruntime.ActionResult, error) {
	if !plan.Enabled {
		return a11yruntime.ActionResult{}, nil
	}
	var lastErr error
	if plan.Ref != 0 && len(plan.RefMap) > 0 {
		result, err := backend.Act(ctx, windowID, plan.Ref, plan.RefMap, "submit", "", holdMS)
		if err == nil && !t.a11ySubmitNeedsRetry(ctx, backend, windowID, confirmation) {
			return result, nil
		}
		if err != nil {
			lastErr = enrichA11ySubmitPhaseError(err)
		} else {
			lastErr = a11ySubmitConfirmationError()
		}
	}
	for _, keys := range plan.KeySequences {
		if len(keys) == 0 {
			continue
		}
		result, err := backend.Key(ctx, windowID, keys, holdMS)
		if err == nil && !t.a11ySubmitNeedsRetry(ctx, backend, windowID, confirmation) {
			return result, nil
		}
		if err != nil {
			lastErr = enrichA11ySubmitPhaseError(err)
		} else {
			lastErr = a11ySubmitConfirmationError()
		}
	}
	if lastErr != nil {
		return a11yruntime.ActionResult{}, lastErr
	}
	return a11yruntime.ActionResult{}, a11yruntime.NewError("unsupported_action", "submit fallback is unavailable", map[string]interface{}{"phase": "submit", "typed": true})
}

func buildA11ySubmitConfirmation(value string, ref int, refMap map[int]string) a11ySubmitConfirmation {
	confirmation := a11ySubmitConfirmation{
		TypedValue: strings.TrimSpace(value),
	}
	if ref != 0 && len(refMap) > 0 {
		confirmation.InputToken = strings.TrimSpace(refMap[ref])
	}
	return confirmation
}

func (t *A11yTool) a11ySubmitNeedsRetry(ctx context.Context, backend a11yruntime.Backend, windowID string, confirmation a11ySubmitConfirmation) bool {
	expected := normalizeA11ySubmitConfirmationText(confirmation.TypedValue)
	if expected == "" {
		return false
	}
	if !t.a11ySubmitSnapshotShowsPendingTypedValue(ctx, backend, windowID, confirmation, expected) {
		return false
	}
	timeout := a11ySubmitConfirmationTimeout
	deadline := time.Now().Add(timeout)
	for {
		if timeout <= 0 || time.Now().After(deadline) {
			return true
		}
		if err := a11yWaitForPollInterval(ctx, a11ySubmitConfirmationPollInterval); err != nil {
			return true
		}
		if !t.a11ySubmitSnapshotShowsPendingTypedValue(ctx, backend, windowID, confirmation, expected) {
			return false
		}
	}
}

func (t *A11yTool) a11ySubmitSnapshotShowsPendingTypedValue(ctx context.Context, backend a11yruntime.Backend, windowID string, confirmation a11ySubmitConfirmation, expected string) bool {
	result, err := backend.SnapshotInteractive(ctx, windowID)
	if err != nil {
		return false
	}
	entries := parseA11ySnapshotEntriesWithTokens(result.Tree, result.RefMap)
	return a11ySubmitSnapshotShowsPendingTypedValue(entries, confirmation, expected)
}

func a11ySubmitSnapshotShowsPendingTypedValue(entries []a11ySnapshotEntry, confirmation a11ySubmitConfirmation, expected string) bool {
	if expected == "" || len(entries) == 0 {
		return false
	}
	if token := strings.TrimSpace(confirmation.InputToken); token != "" {
		for _, entry := range entries {
			if strings.TrimSpace(entry.Token) != token || !a11ySnapshotRoleCouldBeComposer(entry.Role, entry.Label) {
				continue
			}
			return a11ySubmitObservedTextMatches(entry.Label, expected)
		}
	}
	for _, entry := range entries {
		if !a11ySnapshotRoleCouldBeComposer(entry.Role, entry.Label) {
			continue
		}
		if a11ySubmitObservedTextMatches(entry.Label, expected) {
			return true
		}
	}
	return false
}

func a11ySnapshotRoleCouldBeComposer(role string, label string) bool {
	role = normalizeA11yTargetRole(role)
	switch role {
	case "document", "editable_text", "text_area", "text_field", "combo_box", "editor":
	default:
		return false
	}
	if a11yTargetLabelContainsAny(normalizeA11yTargetName(label), "search", "find", "address", "url", "lookup", "搜索", "查找", "地址", "网址") {
		return false
	}
	return true
}

func a11ySubmitObservedTextMatches(observed string, expected string) bool {
	normalizedObserved := normalizeA11ySubmitConfirmationText(observed)
	if normalizedObserved == "" || expected == "" {
		return false
	}
	if len([]rune(expected)) < 4 {
		return normalizedObserved == expected
	}
	return strings.Contains(normalizedObserved, expected)
}

func normalizeA11ySubmitConfirmationText(value string) string {
	var out []rune
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			out = append(out, r)
		}
	}
	return string(out)
}

func a11ySubmitConfirmationError() error {
	return a11yruntime.NewError("backend_unavailable", "submit could not be confirmed", map[string]interface{}{
		"phase":        "submit",
		"typed":        true,
		"confirmation": "input_still_contains_text",
	})
}

func enrichA11ySubmitPhaseError(err error) error {
	return enrichA11yActionPhaseError(err, "submit", true)
}

func enrichA11yActionPhaseError(err error, phase string, typed bool) error {
	if err == nil {
		return nil
	}
	phase = strings.TrimSpace(strings.ToLower(phase))
	if phase == "" {
		phase = "act"
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		return a11yruntime.NewError("backend_unavailable", err.Error(), map[string]interface{}{"phase": phase, "typed": typed})
	}
	details := make(map[string]interface{}, len(runtimeErr.Details)+2)
	for key, value := range runtimeErr.Details {
		details[key] = value
	}
	details["phase"] = phase
	details["typed"] = typed
	return a11yruntime.NewError(runtimeErr.Code, runtimeErr.Message, details)
}

func a11yFallbackActTypeOnUnsupported(actType string, intent string, err error) (string, bool) {
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok || runtimeErr.Code != "unsupported_action" {
		return "", false
	}
	switch {
	case strings.TrimSpace(strings.ToLower(actType)) == "select" && normalizeA11yActIntent(intent) == "select":
		return "click", true
	case strings.TrimSpace(strings.ToLower(actType)) == "toggle" && normalizeA11yActIntent(intent) == "toggle":
		return "click", true
	default:
		return "", false
	}
}

func mergeA11yActionResults(primary a11yruntime.ActionResult, followup a11yruntime.ActionResult) a11yruntime.ActionResult {
	merged := primary
	if strings.TrimSpace(followup.HostOS) != "" {
		merged.HostOS = followup.HostOS
	}
	if strings.TrimSpace(followup.WindowID) != "" {
		merged.WindowID = followup.WindowID
	}
	if strings.TrimSpace(followup.Intent) != "" {
		merged.Intent = followup.Intent
	}
	if followup.TargetHit {
		merged.TargetHit = true
	}
	if followup.VerificationPassed {
		merged.VerificationPassed = true
	}
	if strings.TrimSpace(followup.VerificationMethod) != "" {
		merged.VerificationMethod = followup.VerificationMethod
	}
	if strings.TrimSpace(followup.InputMethod) != "" {
		merged.InputMethod = followup.InputMethod
	}
	if len(followup.Fallbacks) > 0 {
		merged.Fallbacks = mergeA11yFallbacks(merged.Fallbacks, followup.Fallbacks)
	}
	merged.ActionTelemetry = mergeA11yActionTelemetry(merged.ActionTelemetry, followup.ActionTelemetry)
	if strings.TrimSpace(followup.OverlayMode) != "" {
		merged.OverlayMode = followup.OverlayMode
	}
	primaryMode := strings.TrimSpace(primary.ExecutionMode)
	followupMode := strings.TrimSpace(followup.ExecutionMode)
	switch {
	case primaryMode == "":
		merged.ExecutionMode = followupMode
	case followupMode == "" || strings.EqualFold(primaryMode, followupMode):
		merged.ExecutionMode = primaryMode
	default:
		merged.ExecutionMode = "composed"
	}
	if strings.TrimSpace(followup.Message) != "" {
		merged.Message = "Host action completed and submitted"
	}
	return merged
}

func mergeA11yActionTelemetry(primary a11yruntime.ActionTelemetry, followup a11yruntime.ActionTelemetry) a11yruntime.ActionTelemetry {
	merged := primary
	if followup.SnapshotRevision > 0 {
		merged.SnapshotRevision = followup.SnapshotRevision
	}
	if followup.CacheHit {
		merged.CacheHit = true
	}
	if followup.NodeCount > 0 {
		merged.NodeCount = followup.NodeCount
	}
	if followup.CandidateCount > 0 {
		merged.CandidateCount = followup.CandidateCount
	}
	merged.TreeFetchMS += followup.TreeFetchMS
	merged.TreeSerializeMS += followup.TreeSerializeMS
	merged.QueryMS += followup.QueryMS
	merged.ActionMS += followup.ActionMS
	merged.VerificationMS += followup.VerificationMS
	merged.EndToEndMS += followup.EndToEndMS
	merged.Fallbacks = mergeA11yFallbacks(merged.Fallbacks, followup.Fallbacks)
	return merged
}

func defaultA11ySubmitKeySequences(hostOS string) [][]string {
	sequences := [][]string{{"enter"}}
	switch strings.TrimSpace(strings.ToLower(hostOS)) {
	case "darwin":
		sequences = append(sequences, []string{"cmd", "enter"})
	case "windows":
		sequences = append(sequences, []string{"ctrl", "enter"})
	}
	return sequences
}
