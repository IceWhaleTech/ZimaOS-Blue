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
	Name string
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

type a11yStructuredSnapshotProvider interface {
	CurrentStructuredSnapshot(windowID string) (*a11yruntime.Snapshot, bool)
}

var a11ySubmitConfirmationTimeout = 45 * time.Second
var a11ySubmitConfirmationPollInterval = 1 * time.Second
var a11yMessageConversationSettleDelay = 1 * time.Second
var a11yMessageConversationConfirmationTimeout = 30 * time.Second
var a11yMessageConversationConfirmationPollInterval = 1 * time.Second
var a11yConversationSearchResultResolveAttempts = 3
var a11yConversationSearchResultResolvePollInterval = 250 * time.Millisecond
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
	normalized = strings.Join(fields, "_")
	switch normalized {
	case "search", "searchbox", "search_box", "searchbar", "search_bar":
		return "search_field"
	default:
		return normalized
	}
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
	switch normalizeA11yActIntent(intent) {
	case "message", "select":
	default:
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
	state := getA11yChatExecutionState(ctx)
	ref, refMap, resolvedWindow, err := t.resolveActRefByTarget(ctx, backend, windowID, selector)
	if err != nil {
		if fastWindow, fastErr, handled := t.tryA11yConversationFallbackChain(ctx, backend, args, strings.TrimSpace(valueOrDefault(resolvedWindow, windowID)), selector, holdMS, err); handled {
			if fastErr != nil {
				return "", enrichA11yActionPhaseError(fastErr, "conversation", false)
			}
			return fastWindow, nil
		}
		return "", enrichA11yActionPhaseError(err, "conversation", false)
	}
	if state != nil {
		a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusOK, "structured_match", "", "", nil)
		t.rememberA11yChatStrategy(state, a11yChatStageLocateConversation, "structured_match")
	}
	confirmedWindow, confirmErr := t.activateA11yMessageConversationTarget(ctx, backend, resolvedWindow, ref, refMap, selector, holdMS)
	if confirmErr != nil {
		return "", enrichA11yActionPhaseError(confirmErr, "conversation", false)
	}
	return confirmedWindow, nil
}

func (t *A11yTool) tryA11yConversationFallbackChain(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, selector a11yTargetSelector, holdMS int, originalErr error) (string, error, bool) {
	hadCache := false
	if a11yAllowsConversationVisualFastPath(backend, args, selector) {
		cacheKey := t.a11yConversationClickCacheKeyForArgs(args, selector, strings.TrimSpace(windowID))
		_, hadCache = t.a11yConversationClickCacheGet(cacheKey)
	}
	if fastWindow, fastErr, handled := t.tryA11yConversationVisualCacheHit(ctx, backend, args, windowID, selector, holdMS); handled {
		return fastWindow, fastErr, true
	}
	if hadCache {
		if fastWindow, fastErr, handled := t.tryA11yConversationVisualSearchFallback(ctx, backend, args, windowID, selector, holdMS); handled {
			if fastErr == nil {
				return fastWindow, nil, true
			}
			if !a11yCanContinueConversationFallbackAfterVisualMiss(fastErr) {
				return "", fastErr, true
			}
		}
	}
	preferVisual := false
	if state := getA11yChatExecutionState(ctx); state != nil {
		if memory := t.a11yChatMemory(); memory != nil {
			preferVisual = a11yLocateConversationStrategyPrefersVisual(memory.strategyFor(state.platform, state.appProfile, state.intent, string(a11yChatStageLocateConversation)))
		}
	}
	if preferVisual {
		if fastWindow, fastErr, handled := t.tryA11yConversationVisualFastPath(ctx, backend, args, windowID, selector, holdMS); handled {
			if fastErr == nil {
				return fastWindow, nil, true
			}
			if !a11yCanContinueConversationFallbackAfterVisualMiss(fastErr) {
				return "", fastErr, true
			}
		}
	}
	if fastWindow, fastErr, handled := t.tryA11yConversationSearchFallback(ctx, backend, args, windowID, selector, holdMS, originalErr); handled {
		if fastErr == nil {
			return fastWindow, nil, true
		}
		if !a11yShouldContinueConversationFallbackToVisual(backend, args, selector, fastErr) {
			return "", fastErr, true
		}
	}
	if fastWindow, fastErr, handled := t.tryA11yConversationVisualFastPath(ctx, backend, args, windowID, selector, holdMS); handled {
		return fastWindow, fastErr, true
	}
	return "", nil, false
}

func a11yLocateConversationStrategyPrefersVisual(strategy string) bool {
	switch strings.TrimSpace(strings.ToLower(strategy)) {
	case "visual_cache_hit", "visual_sidebar_hit", "visual_search":
		return true
	default:
		return false
	}
}

func a11yCanContinueConversationFallbackAfterVisualMiss(err error) bool {
	if err == nil {
		return false
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		return false
	}
	switch runtimeErr.Code {
	case "fallback_exhausted", "target_not_found", "confirmation_failed":
		return true
	default:
		return false
	}
}

func a11yShouldContinueConversationFallbackToVisual(backend a11yruntime.Backend, args map[string]interface{}, selector a11yTargetSelector, err error) bool {
	if !a11yAllowsConversationVisualFastPath(backend, args, selector) {
		return false
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		return false
	}
	return runtimeErr.Code == "fallback_exhausted"
}

func (t *A11yTool) tryA11yConversationVisualCacheHit(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, selector a11yTargetSelector, holdMS int) (string, error, bool) {
	if !a11yAllowsConversationVisualFastPath(backend, args, selector) {
		return "", nil, false
	}
	state := getA11yChatExecutionState(ctx)
	resolvedWindow := strings.TrimSpace(windowID)
	cacheKey := t.a11yConversationClickCacheKeyForArgs(args, selector, resolvedWindow)
	cachedPoint, ok := t.a11yConversationClickCacheGet(cacheKey)
	if !ok {
		return "", nil, false
	}
	nextWindow, err := t.tryA11yConversationPointClick(ctx, backend, resolvedWindow, selector, holdMS, cachedPoint)
	if err == nil {
		if state != nil {
			a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusOK, "visual_cache_hit", state.groundingSource, "", nil)
			t.rememberA11yChatStrategy(state, a11yChatStageLocateConversation, "visual_cache_hit")
		}
		return nextWindow, nil, true
	}
	t.a11yConversationClickCacheDelete(cacheKey)
	if !a11yCanRetryConversationActivation(err) {
		if state != nil {
			a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusTerminalFailure, "visual_cache_hit", state.groundingSource, "", nil)
		}
		return "", err, true
	}
	return "", nil, false
}

func (t *A11yTool) tryA11yConversationVisualFastPath(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, selector a11yTargetSelector, holdMS int) (string, error, bool) {
	if !a11yAllowsConversationVisualFastPath(backend, args, selector) {
		return "", nil, false
	}
	state := getA11yChatExecutionState(ctx)
	resolvedWindow := strings.TrimSpace(windowID)
	cacheKey := t.a11yConversationClickCacheKeyForArgs(args, selector, resolvedWindow)
	if hit, locateErr := t.locateA11yConversationVisualHit(ctx, backend, resolvedWindow, selector); locateErr == nil {
		nextWindow, clickErr := t.tryA11yConversationPointClick(ctx, backend, resolvedWindow, selector, holdMS, a11yConversationClickPoint{
			X: hit.Point.X,
			Y: hit.Point.Y,
		})
		if clickErr == nil {
			t.a11yConversationClickCacheSet(cacheKey, a11yConversationClickPoint{X: hit.Point.X, Y: hit.Point.Y})
			if state != nil {
				a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusOK, "visual_sidebar_hit", state.groundingSource, "", nil)
				t.rememberA11yChatStrategy(state, a11yChatStageLocateConversation, "visual_sidebar_hit")
			}
			return nextWindow, nil, true
		}
		if !a11yCanRetryConversationActivation(clickErr) {
			if state != nil {
				a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusTerminalFailure, "visual_sidebar_hit", state.groundingSource, "", nil)
			}
			return "", clickErr, true
		}
	}
	return t.tryA11yConversationVisualSearchFallback(ctx, backend, args, resolvedWindow, selector, holdMS)
}

func (t *A11yTool) tryA11yConversationVisualSearchFallback(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, selector a11yTargetSelector, holdMS int) (string, error, bool) {
	if !a11yAllowsConversationVisualFastPath(backend, args, selector) {
		return "", nil, false
	}
	state := getA11yChatExecutionState(ctx)
	resolvedWindow := strings.TrimSpace(windowID)
	cacheKey := t.a11yConversationClickCacheKeyForArgs(args, selector, resolvedWindow)
	plans := a11yConversationSearchPlansForArgs(args, backend.HostOS())
	if state != nil {
		plans = t.orderA11yChatConversationPlans(state, plans)
	}
	if len(plans) == 0 {
		return "", a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil)), true
	}
	var lastErr error = a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil))
	for _, plan := range plans {
		nextWindow, point, err := t.executeA11yConversationVisualSearchPlan(ctx, backend, resolvedWindow, selector, holdMS, plan)
		if err == nil {
			if point != nil {
				t.a11yConversationClickCacheSet(cacheKey, *point)
			}
			if state != nil {
				t.rememberA11yChatStrategy(state, a11yChatStageLocateConversation, valueOrDefault(plan.Name, "visual_search"))
				if state.stage != a11yChatStageConfirmConversation {
					a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusOK, valueOrDefault(plan.Name, "visual_search"), state.groundingSource, "", nil)
				}
			}
			return nextWindow, nil, true
		}
		if !a11yIsTargetNotFound(err) {
			if state != nil {
				a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusTerminalFailure, valueOrDefault(plan.Name, "visual_search"), state.groundingSource, "", nil)
			}
			return "", err, true
		}
		if state != nil {
			a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusRetryableFailure, valueOrDefault(plan.Name, "visual_search"), state.groundingSource, "", nil)
		}
		lastErr = err
		if strings.TrimSpace(nextWindow) != "" {
			resolvedWindow = strings.TrimSpace(nextWindow)
		}
	}
	return "", a11yConversationFallbackExhaustedError(lastErr, selector, "visual_confirmation", len(plans)), true
}

func (t *A11yTool) tryA11yConversationSearchFallback(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, selector a11yTargetSelector, holdMS int, originalErr error) (string, error, bool) {
	if !a11yAllowsConversationSearchFallback(args, originalErr) {
		return "", nil, false
	}
	plans := a11yConversationSearchPlansForArgs(args, backend.HostOS())
	if len(plans) == 0 {
		return "", nil, false
	}
	state := getA11yChatExecutionState(ctx)
	if state != nil {
		plans = t.orderA11yChatConversationPlans(state, plans)
	}
	resolvedWindow := strings.TrimSpace(windowID)
	lastErr := originalErr
	for _, plan := range plans {
		nextWindow, err := t.executeA11yConversationSearchPlan(ctx, backend, resolvedWindow, selector, holdMS, plan)
		if err == nil {
			if state != nil {
				t.rememberA11yChatStrategy(state, a11yChatStageLocateConversation, valueOrDefault(plan.Name, "keyboard_search"))
				if state.stage != a11yChatStageConfirmConversation {
					a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusOK, valueOrDefault(plan.Name, "keyboard_search"), "", "", nil)
				}
			}
			return nextWindow, nil, true
		}
		if !a11yIsTargetNotFound(err) {
			if state != nil {
				a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusTerminalFailure, valueOrDefault(plan.Name, "keyboard_search"), "", "", nil)
			}
			return "", err, true
		}
		if state != nil {
			a11yRecordChatStage(ctx, state, a11yChatStageLocateConversation, a11yChatStageStatusRetryableFailure, valueOrDefault(plan.Name, "keyboard_search"), "", "", nil)
		}
		lastErr = err
		if strings.TrimSpace(nextWindow) != "" {
			resolvedWindow = strings.TrimSpace(nextWindow)
		}
	}
	return "", a11yConversationFallbackExhaustedError(lastErr, selector, "keyboard_search", len(plans)), true
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
	typedWithSearchField := false
	if resolvedWindow, typedWithSearchField, err = t.tryTypeA11yConversationSearchQuery(ctx, backend, resolvedWindow, selector, holdMS); err != nil {
		return resolvedWindow, err
	}
	if !typedWithSearchField {
		if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, [][]string{{selector.Name}}, holdMS); err != nil {
			return resolvedWindow, err
		}
	}
	ref, refMap, resolvedWindow, err := t.resolveA11yConversationSearchResultTarget(ctx, backend, resolvedWindow, selector, a11yConversationSearchResultResolveAttempts)
	if err != nil {
		return resolvedWindow, err
	}
	return t.activateA11yMessageConversationTarget(ctx, backend, resolvedWindow, ref, refMap, selector, holdMS)
}

func (t *A11yTool) executeA11yConversationVisualSearchPlan(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, holdMS int, plan a11yConversationSearchPlan) (string, *a11yConversationClickPoint, error) {
	resolvedWindow := strings.TrimSpace(windowID)
	var err error
	if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, plan.Open, holdMS); err != nil {
		return resolvedWindow, nil, err
	}
	clearSequences := a11yConversationSearchClearSequences(backend.HostOS())
	if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, clearSequences, holdMS); err != nil {
		return resolvedWindow, nil, err
	}
	typedWithSearchField := false
	if resolvedWindow, typedWithSearchField, err = t.tryTypeA11yConversationSearchQuery(ctx, backend, resolvedWindow, selector, holdMS); err != nil {
		return resolvedWindow, nil, err
	}
	if !typedWithSearchField {
		if resolvedWindow, err = t.sendA11yConversationSearchKeySequences(ctx, backend, resolvedWindow, [][]string{{selector.Name}}, holdMS); err != nil {
			return resolvedWindow, nil, err
		}
	}
	ref, refMap, resolvedWindow, err := t.resolveA11yConversationSearchResultTarget(ctx, backend, resolvedWindow, selector, 1)
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
		result, err := a11yRunActionResultWithTimeout(ctx, "key", resolvedWindow, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
			return backend.Key(actionCtx, resolvedWindow, keys, holdMS)
		})
		if err != nil {
			return resolvedWindow, err
		}
		resolvedWindow = strings.TrimSpace(valueOrDefault(result.WindowID, resolvedWindow))
		t.syncWindowContext(resolvedWindow)
		t.clearSnapshotRefs()
		nextWindow, err := t.waitForA11yConversationSearchField(ctx, backend, resolvedWindow)
		if err != nil {
			return resolvedWindow, err
		}
		resolvedWindow = nextWindow
	}
	return resolvedWindow, nil
}

func (t *A11yTool) activateA11yMessageConversationTarget(ctx context.Context, backend a11yruntime.Backend, windowID string, ref int, refMap map[int]string, selector a11yTargetSelector, holdMS int) (string, error) {
	result, err := a11yRunActionResultWithTimeout(ctx, "click", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
		return backend.Act(actionCtx, windowID, ref, refMap, "click", "", holdMS)
	})
	if err != nil {
		return "", err
	}
	resolvedWindow := valueOrDefault(result.WindowID, windowID)
	t.syncWindowContext(resolvedWindow)
	t.clearSnapshotRefs()
	return t.confirmA11yMessageConversationActivated(ctx, backend, resolvedWindow, selector)
}

func (t *A11yTool) tryA11yConversationPointClick(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, holdMS int, point a11yConversationClickPoint) (string, error) {
	result, err := a11yRunActionResultWithTimeout(ctx, "point_click", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
		return backend.ClickWindowPoint(actionCtx, windowID, a11yruntime.NormalizedPoint{X: point.X, Y: point.Y}, holdMS)
	})
	if err != nil {
		return "", err
	}
	resolvedWindow := valueOrDefault(result.WindowID, windowID)
	t.syncWindowContext(resolvedWindow)
	t.clearSnapshotRefs()
	return t.confirmA11yMessageConversationActivated(ctx, backend, resolvedWindow, selector)
}

func (t *A11yTool) locateA11yConversationVisualHit(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector) (a11yConversationVisualHit, error) {
	state := getA11yChatExecutionState(ctx)
	preferredStableID := t.preferredA11yChatAnchor(state, a11yChatStageLocateConversation)
	if hit, ok := a11yStructuredConversationVisualHit(backend, windowID, selector, preferredStableID); ok {
		return hit, nil
	}
	if grounding, ok := backend.(a11yruntime.GroundingScreenshotter); ok {
		result, err := grounding.ScreenshotForGrounding(ctx, windowID)
		if err == nil && len(result.ImageBytes) > 0 && a11yLocateConversationVisualHitFromPNG != nil {
			a11yMaybeWriteChatArtifact(ctx, state, a11yChatStageLocateConversation, "conversation_grounding", result.ImageBytes)
			if hit, ok := t.locateA11yConversationGroundingHit(ctx, strings.TrimSpace(windowID), selector, result.ImageBytes, state); ok {
				return hit, nil
			}
			hit, locateErr := a11yLocateConversationVisualHitFromPNG(ctx, result.ImageBytes, selector.Name)
			if locateErr == nil {
				if state != nil && strings.TrimSpace(state.groundingSource) == "" {
					state.groundingSource = "grounding_screenshot"
				}
				return hit, nil
			}
		}
	}
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

func a11yStructuredConversationVisualHit(backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, preferredStableID string) (a11yConversationVisualHit, bool) {
	provider, ok := backend.(a11yStructuredSnapshotProvider)
	if !ok {
		return a11yConversationVisualHit{}, false
	}
	snapshot, ok := provider.CurrentStructuredSnapshot(strings.TrimSpace(windowID))
	if !ok || snapshot == nil {
		return a11yConversationVisualHit{}, false
	}
	return resolveA11yConversationVisualHitFromStructuredSnapshot(snapshot, selector.Name, preferredStableID)
}

func resolveA11yConversationVisualHitFromStructuredSnapshot(snapshot *a11yruntime.Snapshot, selectorName string, preferredStableID string) (a11yConversationVisualHit, bool) {
	if snapshot == nil || len(snapshot.Nodes) == 0 {
		return a11yConversationVisualHit{}, false
	}
	terms := parseA11yTargetMatchTerms(selectorName)
	if len(terms) == 0 {
		return a11yConversationVisualHit{}, false
	}
	preferredStableID = strings.TrimSpace(preferredStableID)
	matches := make([]a11yConversationVisualHit, 0, 2)
	matchStableIDs := make([]string, 0, 2)
	for _, node := range snapshot.Nodes {
		if !a11yStructuredNodeCouldBeConversationTarget(node) {
			continue
		}
		if a11yTargetNameMatchScore(a11yStructuredSnapshotNodeLabel(node), terms) <= 0 {
			continue
		}
		if node.Bounds.Width <= 0 || node.Bounds.Height <= 0 {
			continue
		}
		matches = append(matches, a11yConversationVisualHit{
			Point: a11yruntime.NormalizedPoint{
				X: node.Bounds.X + node.Bounds.Width/2,
				Y: node.Bounds.Y + node.Bounds.Height/2,
			},
			Confidence: 1,
		})
		matchStableIDs = append(matchStableIDs, strings.TrimSpace(node.StableID))
	}
	if preferredStableID != "" {
		for idx, stableID := range matchStableIDs {
			if stableID == preferredStableID {
				return matches[idx], true
			}
		}
	}
	if len(matches) != 1 {
		return a11yConversationVisualHit{}, false
	}
	return matches[0], true
}

func a11yStructuredNodeCouldBeConversationTarget(node a11yruntime.FlatNode) bool {
	if !a11ySnapshotRoleIsConversation(node.Role) {
		return false
	}
	if !node.Interactive && strings.TrimSpace(node.DefaultAction) == "" {
		return false
	}
	return node.Visible && node.Enabled
}

func (t *A11yTool) locateA11yConversationGroundingHit(ctx context.Context, windowID string, selector a11yTargetSelector, screenshot []byte, state *a11yChatExecutionState) (a11yConversationVisualHit, bool) {
	req := a11yChatGroundingRequest{
		WindowID:         strings.TrimSpace(windowID),
		WindowScreenshot: append([]byte(nil), screenshot...),
		TaskHint:         a11yChatGroundingTaskLocateConversation,
		TargetText:       selector.Name,
	}
	if state != nil {
		req.AppProfile = state.appProfile
	}
	result, used, err := t.groundA11yChat(ctx, req)
	if err != nil || !used {
		return a11yConversationVisualHit{}, false
	}
	hit, ok := resolveA11yConversationVisualHitFromGroundingCandidates(result.Candidates, selector.Name)
	if !ok {
		return a11yConversationVisualHit{}, false
	}
	if state != nil && strings.TrimSpace(state.groundingSource) == "" {
		state.groundingSource = strings.TrimSpace(valueOrDefault(result.Source, "grounding_model"))
	}
	return hit, true
}

func resolveA11yConversationVisualHitFromGroundingCandidates(candidates []a11yChatGroundingCandidate, selectorName string) (a11yConversationVisualHit, bool) {
	normalizedSelector := normalizeA11yTargetName(selectorName)
	if normalizedSelector == "" || len(candidates) == 0 {
		return a11yConversationVisualHit{}, false
	}
	terms := parseA11yTargetMatchTerms(selectorName)
	best := a11yConversationVisualHit{}
	bestScore := 0
	tied := false
	for _, candidate := range candidates {
		score := a11yConversationGroundingCandidateScore(candidate, normalizedSelector, terms)
		if score <= 0 {
			continue
		}
		hit := a11yConversationVisualHit{
			Point: a11yruntime.NormalizedPoint{
				X: candidate.Bounds.X + candidate.Bounds.Width/2,
				Y: candidate.Bounds.Y + candidate.Bounds.Height/2,
			},
			Confidence: candidate.Confidence,
		}
		switch {
		case score > bestScore:
			best = hit
			bestScore = score
			tied = false
		case score == bestScore:
			tied = true
		}
	}
	if bestScore <= 0 || tied {
		return a11yConversationVisualHit{}, false
	}
	return best, true
}

func a11yConversationGroundingCandidateScore(candidate a11yChatGroundingCandidate, normalizedSelector string, terms []string) int {
	if !a11yConversationGroundingRoleAllowed(candidate.Role) {
		return 0
	}
	if candidate.Confidence < a11yConversationVisualConfidenceThreshold {
		return 0
	}
	if candidate.Bounds.Width <= 0 || candidate.Bounds.Height <= 0 {
		return 0
	}
	if candidate.Bounds.X < 0 || candidate.Bounds.Y < 0 || candidate.Bounds.X+candidate.Bounds.Width > 1 || candidate.Bounds.Y+candidate.Bounds.Height > 1 {
		return 0
	}
	score := 0
	candidateName := normalizeA11yTargetName(candidate.Label)
	switch {
	case candidateName == normalizedSelector:
		score += 200
	default:
		matchScore := a11yTargetNameMatchScore(candidate.Label, terms)
		if matchScore <= 0 {
			return 0
		}
		score += matchScore
	}
	switch normalizeA11yTargetRole(candidate.Role) {
	case "conversation", "chat", "thread", "contact":
		score += 50
	case "list_item", "item", "option", "selectable":
		score += 25
	case "button", "push_button":
		score += 15
	case "link":
		score += 10
	}
	score += a11yConversationGroundingRationaleBonus(candidate.RationaleTags)
	score += int(candidate.Confidence * 100)
	return score
}

func a11yConversationGroundingRoleAllowed(role string) bool {
	switch normalizeA11yTargetRole(role) {
	case "conversation", "chat", "thread", "contact", "list_item", "item", "option", "selectable", "button", "push_button", "link":
		return true
	default:
		return false
	}
}

func a11yConversationGroundingConfirmsTarget(candidates []a11yChatGroundingCandidate, selectorName string) bool {
	normalizedSelector := normalizeA11yTargetName(selectorName)
	if normalizedSelector == "" || len(candidates) == 0 {
		return false
	}
	terms := parseA11yTargetMatchTerms(selectorName)
	bestScore := 0
	tied := false
	for _, candidate := range candidates {
		score := a11yConversationGroundingConfirmationScore(candidate, normalizedSelector, terms)
		if score <= 0 {
			continue
		}
		switch {
		case score > bestScore:
			bestScore = score
			tied = false
		case score == bestScore:
			tied = true
		}
	}
	return bestScore > 0 && !tied
}

func a11yConversationGroundingConfirmationScore(candidate a11yChatGroundingCandidate, normalizedSelector string, terms []string) int {
	if !a11yConversationGroundingRoleAllowed(candidate.Role) {
		return 0
	}
	if candidate.Confidence < a11yConversationVisualConfidenceThreshold {
		return 0
	}
	score := 0
	candidateName := normalizeA11yTargetName(candidate.Label)
	switch {
	case candidateName == normalizedSelector:
		score += 200
	default:
		matchScore := a11yTargetNameMatchScore(candidate.Label, terms)
		if matchScore <= 0 {
			return 0
		}
		score += matchScore
	}
	switch normalizeA11yTargetRole(candidate.Role) {
	case "conversation", "chat", "thread", "contact":
		score += 50
	case "list_item", "item", "option", "selectable":
		score += 25
	case "button", "push_button":
		score += 15
	case "link":
		score += 10
	}
	score += a11yConversationGroundingRationaleBonus(candidate.RationaleTags)
	score += int(candidate.Confidence * 100)
	return score
}

func a11yConversationGroundingRationaleBonus(tags []string) int {
	bonus := 0
	for _, tag := range tags {
		switch strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(tag)), "-", "_"), " ", "_") {
		case "active", "current", "selected":
			bonus += 30
		case "focused":
			bonus += 20
		}
	}
	return bonus
}

func (t *A11yTool) tryTypeA11yConversationSearchQuery(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, holdMS int) (string, bool, error) {
	ref, refMap, resolvedWindow, err := t.resolveA11yConversationSearchField(ctx, backend, windowID)
	if err != nil {
		return resolvedWindow, false, nil
	}
	result, err := a11yRunActionResultWithTimeout(ctx, "type", resolvedWindow, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
		return backend.Act(actionCtx, resolvedWindow, ref, refMap, "type", selector.Name, holdMS)
	})
	if err != nil {
		return strings.TrimSpace(valueOrDefault(result.WindowID, resolvedWindow)), true, err
	}
	resolvedWindow = strings.TrimSpace(valueOrDefault(result.WindowID, resolvedWindow))
	t.syncWindowContext(resolvedWindow)
	t.clearSnapshotRefs()
	if err := a11yWaitForConversationSettle(ctx); err != nil {
		return resolvedWindow, true, err
	}
	return resolvedWindow, true, nil
}

func (t *A11yTool) resolveA11yConversationSearchResultTarget(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector, maxAttempts int) (int, map[int]string, string, error) {
	resolvedWindow := strings.TrimSpace(windowID)
	attempts := maxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	state := getA11yChatExecutionState(ctx)
	profile := a11yConversationAppProfileFromExecutionState(state)
	candidates := a11yConversationSearchResultSelectors(profile, selector)
	var lastErr error = a11yruntime.NewError("target_not_found", "target selector did not match any interactive element", a11yTargetSelectorDetails(selector, nil))
	for attempt := 0; attempt < attempts; attempt++ {
		result, err := backend.SnapshotInteractive(ctx, resolvedWindow)
		if err != nil {
			return 0, nil, resolvedWindow, err
		}
		if strings.TrimSpace(result.WindowID) != "" {
			resolvedWindow = strings.TrimSpace(result.WindowID)
		}
		t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
		entries := parseA11ySnapshotEntries(result.Tree)
		for idx, candidate := range candidates {
			if idx > 0 && !a11ySnapshotHasConversationSearch(entries) {
				break
			}
			ref, resolveErr := resolveA11yTargetRef(entries, candidate)
			if resolveErr == nil {
				return ref, cloneA11yRefMap(result.RefMap), resolvedWindow, nil
			}
			if !a11yIsTargetNotFound(resolveErr) {
				return 0, nil, resolvedWindow, resolveErr
			}
			if idx == 0 {
				lastErr = resolveErr
			}
		}
		if fallbackRef, fallbackRefMap, fallbackWindow, fallbackErr, ok := t.resolveA11yConversationSearchResultFromFullSnapshot(ctx, backend, resolvedWindow, candidates, lastErr); ok {
			if fallbackErr == nil {
				return fallbackRef, fallbackRefMap, fallbackWindow, nil
			}
			resolvedWindow = strings.TrimSpace(valueOrDefault(fallbackWindow, resolvedWindow))
			if !a11yIsTargetNotFound(fallbackErr) {
				return 0, nil, resolvedWindow, fallbackErr
			}
			lastErr = fallbackErr
		}
		if !a11ySnapshotHasConversationSearch(entries) {
			return 0, nil, resolvedWindow, lastErr
		}
		if attempt+1 >= attempts {
			break
		}
		if err := a11yWaitForPollInterval(ctx, a11yConversationSearchResultResolvePollInterval); err != nil {
			return 0, nil, resolvedWindow, err
		}
	}
	return 0, nil, resolvedWindow, lastErr
}

func (t *A11yTool) resolveA11yConversationSearchResultFromFullSnapshot(ctx context.Context, backend a11yruntime.Backend, windowID string, candidates []a11yTargetSelector, originalErr error) (int, map[int]string, string, error, bool) {
	if len(candidates) == 0 {
		return 0, nil, strings.TrimSpace(windowID), nil, false
	}
	eligible := make([]a11yTargetSelector, 0, len(candidates))
	for _, candidate := range candidates {
		if a11yAllowsFullSnapshotLabelFallback(candidate, originalErr) {
			eligible = append(eligible, candidate)
		}
	}
	if len(eligible) == 0 {
		return 0, nil, strings.TrimSpace(windowID), nil, false
	}
	result, err := backend.Snapshot(ctx, windowID)
	if err != nil {
		return 0, nil, strings.TrimSpace(windowID), nil, false
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
	t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
	lines := parseA11ySnapshotLines(result.Tree)
	lastErr := originalErr
	for idx, candidate := range eligible {
		ref, resolveErr := resolveA11yLabelAnchoredTargetRef(lines, candidate)
		if resolveErr == nil {
			return ref, cloneA11yRefMap(result.RefMap), resolvedWindow, nil, true
		}
		if !a11yIsTargetNotFound(resolveErr) {
			return 0, nil, resolvedWindow, resolveErr, true
		}
		if idx == 0 {
			lastErr = resolveErr
		}
	}
	return 0, nil, resolvedWindow, lastErr, true
}

func (t *A11yTool) resolveA11yConversationSearchField(ctx context.Context, backend a11yruntime.Backend, windowID string) (int, map[int]string, string, error) {
	snapshotTarget := strings.TrimSpace(windowID)
	t.mu.RLock()
	cachedWindow := strings.TrimSpace(t.lastWindow)
	cachedRefMap := cloneA11yRefMap(t.lastRefMap)
	cachedRefs := cloneA11ySnapshotEntries(t.lastRefs)
	t.mu.RUnlock()
	if len(cachedRefMap) > 0 && len(cachedRefs) > 0 {
		if snapshotTarget == "" || cachedWindow == "" || cachedWindow == snapshotTarget {
			if ref, err := resolveA11yConversationSearchRef(cachedRefs); err == nil {
				if snapshotTarget == "" {
					snapshotTarget = cachedWindow
				}
				return ref, cachedRefMap, strings.TrimSpace(valueOrDefault(snapshotTarget, cachedWindow)), nil
			}
		}
	}
	result, err := backend.SnapshotInteractive(ctx, snapshotTarget)
	if err != nil {
		return 0, nil, snapshotTarget, err
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, snapshotTarget))
	t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
	ref, err := resolveA11yConversationSearchRef(parseA11ySnapshotEntries(result.Tree))
	if err != nil {
		return 0, nil, resolvedWindow, err
	}
	return ref, cloneA11yRefMap(result.RefMap), resolvedWindow, nil
}

func resolveA11yConversationSearchRef(entries []a11ySnapshotEntry) (int, error) {
	matches := make([]a11ySnapshotEntry, 0, 2)
	bestRef := 0
	bestScore := 0
	tied := false
	for _, entry := range entries {
		if !a11ySnapshotRoleLooksLikeConversationSearch(entry.Role, entry.Label) {
			continue
		}
		matches = append(matches, entry)
		score := a11yConversationSearchEntryScore(entry)
		switch {
		case score > bestScore:
			bestRef = entry.Ref
			bestScore = score
			tied = false
		case score == bestScore:
			tied = true
		}
	}
	selector := a11yTargetSelector{Role: "search_field"}
	if len(matches) == 0 || bestRef == 0 {
		return 0, a11yruntime.NewError("target_not_found", "conversation search field not found", a11yTargetSelectorDetails(selector, nil))
	}
	if tied {
		return 0, a11yruntime.NewError("ambiguous_target", "conversation search field matched multiple interactive elements", a11yTargetSelectorDetails(selector, matches))
	}
	return bestRef, nil
}

func a11yConversationSearchEntryScore(entry a11ySnapshotEntry) int {
	score := 0
	switch normalizeA11yTargetRole(entry.Role) {
	case "search_field", "search":
		score += 200
	case "combo_box":
		score += 140
	case "text_field":
		score += 120
	default:
		score += 80
	}
	if a11yTargetLabelContainsAny(normalizeA11yTargetName(entry.Label), "search", "find", "lookup", "搜索", "查找") {
		score += 40
	}
	return score
}

func a11yConversationAppProfileFromExecutionState(state *a11yChatExecutionState) *a11yConversationAppProfile {
	if state == nil {
		return nil
	}
	if profile := lookupA11yConversationAppProfileByID(state.appProfile); profile != nil {
		return profile
	}
	return lookupA11yConversationAppProfileValues(state.appProfile)
}

func a11yConversationSearchResultSelectors(profile *a11yConversationAppProfile, selector a11yTargetSelector) []a11yTargetSelector {
	selectors := []a11yTargetSelector{selector}
	if !a11yAllowsConversationSearchResultRoleFallback(profile, selector) {
		return selectors
	}
	return append(selectors, a11yTargetSelector{Name: selector.Name, Role: "selectable"})
}

func a11yAllowsConversationSearchResultRoleFallback(profile *a11yConversationAppProfile, selector a11yTargetSelector) bool {
	if profile == nil || !profile.EnableSearchResultRoleFallback {
		return false
	}
	if normalizeA11yTargetName(selector.Name) == "" {
		return false
	}
	switch normalizeA11yTargetRole(selector.Role) {
	case "conversation", "chat", "thread", "contact":
		return true
	default:
		return false
	}
}

func a11yAllowsConversationSearchFallback(args map[string]interface{}, err error) bool {
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok || runtimeErr.Code != "target_not_found" {
		return false
	}
	profile := lookupA11yConversationAppProfile(args)
	return profile != nil && profile.EnableSearchFallback
}

func a11yAllowsConversationVisualFastPath(backend a11yruntime.Backend, args map[string]interface{}, selector a11yTargetSelector) bool {
	if backend == nil || !strings.EqualFold(strings.TrimSpace(backend.HostOS()), "darwin") {
		return false
	}
	if !selector.Provided() || strings.TrimSpace(selector.Name) == "" {
		return false
	}
	profile := lookupA11yConversationAppProfile(args)
	return profile != nil && profile.EnableVisualFastPath
}

func a11yIsTargetNotFound(err error) bool {
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	return ok && runtimeErr.Code == "target_not_found"
}

func a11yCanRetryConversationActivation(err error) bool {
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		return false
	}
	switch runtimeErr.Code {
	case "target_not_found", "confirmation_failed":
		return true
	default:
		return false
	}
}

func a11yConversationSearchPlans(hostOS string) []a11yConversationSearchPlan {
	modifier := a11yConversationShortcutModifier(hostOS)
	return []a11yConversationSearchPlan{
		{Name: "structured_search", Open: [][]string{{modifier, "f"}, {modifier, "f"}}},
		{Name: "quick_switcher", Open: [][]string{{modifier, "k"}}},
	}
}

func a11yConversationSearchClearSequences(hostOS string) [][]string {
	modifier := a11yConversationShortcutModifier(hostOS)
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

func (t *A11yTool) waitForA11yConversationSearchField(ctx context.Context, backend a11yruntime.Backend, windowID string) (string, error) {
	resolvedWindow := strings.TrimSpace(windowID)
	timeout := a11yMessageConversationConfirmationTimeout
	deadline := time.Now().Add(timeout)
	for {
		_, _, nextWindow, err := t.resolveA11yConversationSearchField(ctx, backend, resolvedWindow)
		if err == nil {
			if strings.TrimSpace(nextWindow) != "" {
				return strings.TrimSpace(nextWindow), nil
			}
			return resolvedWindow, nil
		}
		if !a11yIsTargetNotFound(err) {
			return strings.TrimSpace(valueOrDefault(nextWindow, resolvedWindow)), err
		}
		resolvedWindow = strings.TrimSpace(valueOrDefault(nextWindow, resolvedWindow))
		if timeout <= 0 || time.Now().After(deadline) {
			return resolvedWindow, err
		}
		if err := a11yWaitForPollInterval(ctx, a11yConversationSearchResultResolvePollInterval); err != nil {
			return resolvedWindow, err
		}
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
	state := getA11yChatExecutionState(ctx)
	resolvedWindow := strings.TrimSpace(windowID)
	for {
		if snapshot, structuredWindow, ok := t.currentA11yChatStructuredSnapshot(backend, resolvedWindow); ok {
			if verification, ok := a11yStructuredSnapshotSelectedConversationVerification(snapshot, selector); ok {
				resolvedWindow = valueOrDefault(structuredWindow, resolvedWindow)
				if state != nil {
					if stableID := strings.TrimSpace(a11yFirstNonEmptyString(verification["element_stable_id"])); stableID != "" {
						t.rememberA11yChatAnchor(state, a11yChatStageLocateConversation, stableID)
					}
					a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusOK, "structured_selected_conversation", "", "", verification)
				}
				return resolvedWindow, nil
			}
		}
		result, err := backend.SnapshotInteractive(ctx, resolvedWindow)
		if err != nil {
			return "", err
		}
		resolvedWindow = strings.TrimSpace(valueOrDefault(result.WindowID, resolvedWindow))
		t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
		entries := parseA11ySnapshotEntriesWithTokens(result.Tree, result.RefMap)
		pendingSearch := a11ySnapshotHasPendingConversationSearch(entries, selector.Name)
		composerReady := a11ySnapshotHasComposer(entries)
		if composerReady {
			if !pendingSearch {
				if verification, ok := a11yStructuredSnapshotConfirmsSelectedConversation(backend, resolvedWindow, selector); ok {
					if state != nil {
						if stableID := strings.TrimSpace(a11yFirstNonEmptyString(verification["element_stable_id"])); stableID != "" {
							t.rememberA11yChatAnchor(state, a11yChatStageLocateConversation, stableID)
						}
						a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusOK, "structured_selected_conversation", "", "", verification)
					}
					return resolvedWindow, nil
				}
			}
			if groundingSource, verification, grounded, retryable, groundErr := t.confirmA11yConversationWithGrounding(ctx, backend, resolvedWindow, selector); groundErr != nil {
				return "", groundErr
			} else if retryable {
				if timeout <= 0 || time.Now().After(deadline) {
					if state != nil {
						a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusTerminalFailure, "visual_grounding_check", groundingSource, "search_box_still_active", verification)
					}
					return "", a11yruntime.NewError("confirmation_failed", "conversation switch could not be confirmed", map[string]interface{}{
						"confirmation":            "search_box_still_active",
						"search_box_still_active": true,
						"grounding_source":        groundingSource,
						"verification":            verification,
					})
				}
				if state != nil {
					a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusRetryableFailure, "visual_grounding_check", groundingSource, "", verification)
				}
				if err := a11yWaitForPollInterval(ctx, a11yMessageConversationConfirmationPollInterval); err != nil {
					return "", err
				}
				continue
			} else if grounded {
				if pendingSearch && !compatBoolValue(verification["conversation_confirmed"], false) {
					if state != nil {
						a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusTerminalFailure, "visual_grounding_check", groundingSource, "search_box_still_active", verification)
					}
					return "", a11yruntime.NewError("confirmation_failed", "conversation switch could not be confirmed", map[string]interface{}{
						"confirmation":            "composer_not_ready",
						"search_box_still_active": true,
						"grounding_source":        groundingSource,
						"verification":            verification,
					})
				}
				if state != nil {
					a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusOK, "visual_grounding_check", groundingSource, "", verification)
				}
				return resolvedWindow, nil
			} else if !pendingSearch {
				if state != nil {
					a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusOK, "post_click_confirmation", "", "", nil)
				}
				return resolvedWindow, nil
			}
		}
		if pendingSearch && composerReady {
			if state != nil {
				a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusTerminalFailure, "post_click_confirmation", "", "search_box_still_active", nil)
			}
			return "", a11yruntime.NewError("confirmation_failed", "conversation switch could not be confirmed", map[string]interface{}{
				"confirmation":            "composer_not_ready",
				"search_box_still_active": true,
			})
		}
		if pendingSearch {
			if timeout <= 0 || time.Now().After(deadline) {
				if state != nil {
					a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusTerminalFailure, "post_click_confirmation", "", "search_box_still_active", nil)
				}
				return "", a11yruntime.NewError("confirmation_failed", "conversation switch could not be confirmed", map[string]interface{}{
					"confirmation":            "composer_not_ready",
					"search_box_still_active": true,
				})
			}
			if state != nil {
				a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusRetryableFailure, "post_click_confirmation", "", "", nil)
			}
			if err := a11yWaitForPollInterval(ctx, a11yMessageConversationConfirmationPollInterval); err != nil {
				return "", err
			}
			continue
		}
		if timeout <= 0 || time.Now().After(deadline) {
			if state != nil {
				a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusTerminalFailure, "post_click_confirmation", "", "conversation_not_confirmed", nil)
			}
			return "", a11yConversationConfirmationError(selector)
		}
		if err := a11yWaitForPollInterval(ctx, a11yMessageConversationConfirmationPollInterval); err != nil {
			return "", err
		}
	}
}

func a11yStructuredSnapshotConfirmsSelectedConversation(backend a11yruntime.Backend, windowID string, selector a11yTargetSelector) (map[string]interface{}, bool) {
	provider, ok := backend.(a11yStructuredSnapshotProvider)
	if !ok {
		return nil, false
	}
	snapshot, ok := provider.CurrentStructuredSnapshot(strings.TrimSpace(windowID))
	if !ok || snapshot == nil {
		return nil, false
	}
	return a11yStructuredSnapshotSelectedConversationVerification(snapshot, selector)
}

func a11yStructuredSnapshotSelectedConversationVerification(snapshot *a11yruntime.Snapshot, selector a11yTargetSelector) (map[string]interface{}, bool) {
	if snapshot == nil {
		return nil, false
	}
	if a11yStructuredSnapshotHasFocusedSearchField(snapshot) {
		return nil, false
	}
	if node, ok := a11yStructuredSnapshotSelectedConversationNode(snapshot, selector.Name); ok {
		verification := map[string]interface{}{
			"conversation_confirmed": true,
			"confirmation":           "structured_selected_conversation",
		}
		if stableID := strings.TrimSpace(node.StableID); stableID != "" {
			verification["element_stable_id"] = stableID
		}
		return verification, true
	}
	if a11yStructuredSnapshotHasVisibleConversationContext(snapshot, selector.Name) {
		return map[string]interface{}{
			"conversation_confirmed": true,
			"confirmation":           "structured_conversation_context_visible",
		}, true
	}
	return nil, false
}

func a11yStructuredSnapshotHasSelectedConversation(snapshot *a11yruntime.Snapshot, selectorName string) bool {
	_, ok := a11yStructuredSnapshotSelectedConversationNode(snapshot, selectorName)
	return ok
}

func a11yStructuredSnapshotSelectedConversationNode(snapshot *a11yruntime.Snapshot, selectorName string) (a11yruntime.FlatNode, bool) {
	if snapshot == nil || len(snapshot.Nodes) == 0 {
		return a11yruntime.FlatNode{}, false
	}
	normalizedSelector := normalizeA11yTargetName(selectorName)
	if normalizedSelector == "" {
		return a11yruntime.FlatNode{}, false
	}
	terms := parseA11yTargetMatchTerms(normalizedSelector)
	matches := make([]a11yruntime.FlatNode, 0, 2)
	for _, node := range snapshot.Nodes {
		if !a11ySnapshotRoleIsConversation(node.Role) {
			continue
		}
		label := normalizeA11yTargetName(strings.TrimSpace(node.Name))
		if label == "" {
			label = normalizeA11yTargetName(strings.TrimSpace(node.Value))
		}
		if !a11yStructuredConversationLabelMatches(label, normalizedSelector, terms) {
			continue
		}
		if a11yStructuredConversationNodeIsSelected(node) {
			matches = append(matches, node)
		}
	}
	if len(matches) != 1 {
		return a11yruntime.FlatNode{}, false
	}
	return matches[0], true
}

func a11yStructuredConversationLabelMatches(label string, normalizedSelector string, terms []string) bool {
	if label == "" || normalizedSelector == "" {
		return false
	}
	if label == normalizedSelector || strings.Contains(label, normalizedSelector) {
		return true
	}
	if len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		if !strings.Contains(label, term) {
			return false
		}
	}
	return true
}

func a11yStructuredConversationNodeIsSelected(node a11yruntime.FlatNode) bool {
	if node.Focused {
		return true
	}
	state := normalizeA11yTargetName(strings.TrimSpace(node.Description + " " + node.State))
	return a11yTargetLabelContainsAny(state, "selected", "focused", "active", "current")
}

func a11yStructuredSnapshotHasVisibleConversationContext(snapshot *a11yruntime.Snapshot, selectorName string) bool {
	if snapshot == nil || len(snapshot.Nodes) == 0 {
		return false
	}
	normalizedSelector := normalizeA11yTargetName(selectorName)
	if normalizedSelector == "" {
		return false
	}
	terms := parseA11yTargetMatchTerms(normalizedSelector)
	nodesByID := make(map[int]a11yruntime.FlatNode, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		nodesByID[node.NodeID] = node
	}
	for _, node := range snapshot.Nodes {
		if !a11yStructuredNodeCouldBeConversationContextText(node) {
			continue
		}
		label := normalizeA11yTargetName(a11yStructuredSnapshotNodeLabel(node))
		if !a11yStructuredConversationLabelMatches(label, normalizedSelector, terms) {
			continue
		}
		if a11yStructuredNodeMatchesConversationSidebar(snapshot, nodesByID, node, selectorName) {
			continue
		}
		return true
	}
	return false
}

func a11yStructuredNodeCouldBeConversationContextText(node a11yruntime.FlatNode) bool {
	role := normalizeA11yTargetRole(node.Role)
	label := a11yStructuredSnapshotNodeLabel(node)
	if strings.TrimSpace(label) == "" {
		return false
	}
	if node.Interactive || a11ySnapshotRoleIsConversation(role) {
		return false
	}
	if a11ySnapshotRoleCouldBeComposer(role, label) || a11ySnapshotRoleLooksLikeConversationSearch(role, label) {
		return false
	}
	switch role {
	case "static_text", "text", "label", "heading":
		return true
	default:
		return false
	}
}

func (t *A11yTool) confirmA11yConversationWithGrounding(ctx context.Context, backend a11yruntime.Backend, windowID string, selector a11yTargetSelector) (string, map[string]interface{}, bool, bool, error) {
	state := getA11yChatExecutionState(ctx)
	if state == nil || t.a11yChatGrounder() == nil {
		return "", nil, false, false, nil
	}
	req := a11yChatGroundingRequest{
		WindowID:   strings.TrimSpace(windowID),
		TaskHint:   a11yChatGroundingTaskLocateConversation,
		AppProfile: state.appProfile,
		TargetText: selector.Name,
	}
	if grounding, ok := backend.(a11yruntime.GroundingScreenshotter); ok {
		if shot, shotErr := grounding.ScreenshotForGrounding(ctx, windowID); shotErr == nil {
			req.WindowScreenshot = append([]byte(nil), shot.ImageBytes...)
			a11yMaybeWriteChatArtifact(ctx, state, a11yChatStageConfirmConversation, "conversation_confirm_grounding", shot.ImageBytes)
		}
	}
	groundingResult, used, groundErr := t.groundA11yChat(ctx, req)
	if !used || groundErr != nil {
		return "", nil, false, false, nil
	}
	source := strings.TrimSpace(valueOrDefault(groundingResult.Source, "grounding_model"))
	verification := cloneA11yJSONMap(groundingResult.Verification)
	if verification == nil {
		verification = map[string]interface{}{}
	}
	searchFieldDetected := false
	for _, candidate := range groundingResult.Candidates {
		role := normalizeA11yTargetRole(candidate.Role)
		if role == "search_field" || role == "search" {
			searchFieldDetected = true
			break
		}
	}
	explicitReject := false
	if value, ok := groundingResult.Verification["rejected"].(bool); ok && value {
		explicitReject = true
	}
	if reason := strings.TrimSpace(a11yFirstNonEmptyString(groundingResult.Verification["reason"])); strings.Contains(reason, "search_field") {
		searchFieldDetected = true
	}
	if a11yConversationGroundingConfirmsTarget(groundingResult.Candidates, selector.Name) {
		verification["conversation_confirmed"] = true
	}
	conversationConfirmed := compatBoolValue(verification["conversation_confirmed"], false)
	if explicitReject {
		details := map[string]interface{}{
			"grounding_source": source,
			"verification":     verification,
		}
		if searchFieldDetected {
			details["confirmation"] = "search_box_still_active"
			details["search_box_still_active"] = true
		} else {
			details["confirmation"] = "visual_not_confirmed"
		}
		if state != nil {
			failureCode := "conversation_not_confirmed"
			if searchFieldDetected {
				failureCode = "search_box_still_active"
			}
			a11yRecordChatStage(ctx, state, a11yChatStageConfirmConversation, a11yChatStageStatusTerminalFailure, "visual_grounding_check", source, failureCode, verification)
		}
		return source, verification, true, false, a11yruntime.NewError("confirmation_failed", "conversation switch could not be confirmed", details)
	}
	if searchFieldDetected && !conversationConfirmed {
		return source, verification, true, true, nil
	}
	return source, verification, true, false, nil
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

func a11ySnapshotHasConversationSearch(entries []a11ySnapshotEntry) bool {
	for _, entry := range entries {
		if a11ySnapshotRoleLooksLikeConversationSearch(entry.Role, entry.Label) {
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

func a11yConversationFallbackExhaustedError(err error, selector a11yTargetSelector, stage string, attempts int) error {
	details := a11yTargetSelectorDetails(selector, nil)
	if runtimeErr, ok := err.(*a11yruntime.RuntimeError); ok {
		for key, value := range runtimeErr.Details {
			details[key] = value
		}
		if strings.TrimSpace(runtimeErr.Code) != "" {
			details["original_error_code"] = runtimeErr.Code
		}
		if strings.TrimSpace(runtimeErr.Message) != "" {
			details["original_error"] = runtimeErr.Message
		}
	} else if err != nil {
		details["original_error"] = err.Error()
	}
	if strings.TrimSpace(stage) != "" {
		details["fallback_stage"] = stage
	}
	if attempts > 0 {
		details["fallback_attempts"] = attempts
	}
	return a11yruntime.NewError("fallback_exhausted", "conversation fallback chain exhausted without a confirmed target", details)
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
		submitKeys = normalizeA11yShortcutLiteralKeys(submitKeys)
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
	submitToken := ""
	if plan.Ref != 0 && len(plan.RefMap) > 0 {
		submitToken = strings.TrimSpace(plan.RefMap[plan.Ref])
	}
	var lastErr error
	if plan.Ref != 0 && len(plan.RefMap) > 0 {
		result, err := a11yRunActionResultWithTimeout(ctx, "submit", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
			return backend.Act(actionCtx, windowID, plan.Ref, plan.RefMap, "submit", "", holdMS)
		})
		attemptWindow := strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
		if err == nil {
			if runtime, ok := backend.(a11yruntime.SnapshotRuntime); ok {
				runtime.UpdateSnapshotAfterAction(attemptWindow, submitToken, "submit", "")
			}
		}
		if err == nil && !t.a11ySubmitNeedsRetry(ctx, backend, attemptWindow, confirmation) {
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
		result, err := a11yRunActionResultWithTimeout(ctx, "key", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
			return backend.Key(actionCtx, windowID, keys, holdMS)
		})
		attemptWindow := strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
		if err == nil {
			if runtime, ok := backend.(a11yruntime.SnapshotRuntime); ok {
				runtime.UpdateSnapshotAfterAction(attemptWindow, submitToken, "submit", "")
			}
		}
		if err == nil && !t.a11ySubmitNeedsRetry(ctx, backend, attemptWindow, confirmation) {
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
	state := getA11yChatExecutionState(ctx)
	if state != nil {
		state.rememberSubmitEvidence(nil)
	}
	if verification, pending, confirmed := t.a11ySubmitPositiveVerification(ctx, backend, windowID, confirmation, expected); confirmed {
		if state != nil && state.intent == "message" {
			state.rememberSubmitEvidence(verification)
		}
		return false
	} else if !pending {
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
		if verification, pending, confirmed := t.a11ySubmitPositiveVerification(ctx, backend, windowID, confirmation, expected); confirmed {
			if state != nil && state.intent == "message" {
				state.rememberSubmitEvidence(verification)
			}
			return false
		} else if !pending {
			return false
		}
	}
}

func (t *A11yTool) a11ySubmitPositiveVerification(ctx context.Context, backend a11yruntime.Backend, windowID string, confirmation a11ySubmitConfirmation, expected string) (map[string]interface{}, bool, bool) {
	if expected == "" {
		return nil, false, false
	}
	state := getA11yChatExecutionState(ctx)
	conversation := ""
	preferredStableID := ""
	resolvedWindow := strings.TrimSpace(windowID)
	if state != nil && state.intent == "message" {
		conversation = strings.TrimSpace(state.conversation)
		preferredStableID = t.preferredA11yChatAnchor(state, a11yChatStageLocateComposer)
		if verification, ok := t.awaitA11yStructuredPostSubmitVerification(ctx, backend, resolvedWindow, confirmation.TypedValue); ok {
			return verification, false, true
		}
		if snapshot, structuredWindow, ok := t.currentA11yChatStructuredSnapshot(backend, resolvedWindow); ok {
			resolvedWindow = valueOrDefault(structuredWindow, resolvedWindow)
			if verification, ok := a11yStructuredPostSubmitVerificationFromStructuredSnapshot(snapshot, expected, conversation, preferredStableID); ok {
				return verification, false, true
			}
		}
	}
	result, err := backend.SnapshotInteractive(ctx, resolvedWindow)
	if err != nil {
		return nil, false, false
	}
	resolvedWindow = strings.TrimSpace(valueOrDefault(result.WindowID, resolvedWindow))
	t.cacheSnapshotContext(resolvedWindow, result.RefMap, result.Tree)
	if state != nil && state.intent == "message" {
		if verification, _, ok := t.currentA11yChatStructuredPostSubmitVerification(backend, resolvedWindow, expected, conversation, preferredStableID); ok {
			return verification, false, true
		}
	}
	entries := parseA11ySnapshotEntriesWithTokens(result.Tree, result.RefMap)
	if len(entries) == 0 || !a11ySnapshotHasComposer(entries) {
		return nil, false, false
	}
	if a11ySubmitSnapshotShowsPendingTypedValue(entries, confirmation, expected) {
		return nil, true, false
	}
	return map[string]interface{}{
		"status":            "sent",
		"composer_cleared":  true,
		"confirmation":      "interactive_post_submit_confirmation",
		"grounding_skipped": true,
	}, false, true
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
