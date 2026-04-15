package a11y

import (
	"sort"
	"strings"
	"time"
)

type TargetSelector struct {
	Name   string
	Role   string
	Intent string
	ActType string
}

type TargetResolution struct {
	WindowID         string
	NodeID           int
	Ref              int
	RefMap           map[int]string
	Tree             string
	Token            string
	SnapshotRevision int64
	CacheHit         bool
	NodeCount        int
	CandidateCount   int
	QueryMS          int64
	Fallbacks        []string
}

type rankedTarget struct {
	node      FlatNode
	score     int
	order     int
	exactName bool
}

func ResolveSnapshotTarget(snapshot *Snapshot, selector TargetSelector) (TargetResolution, error) {
	start := time.Now()
	if snapshot == nil || len(snapshot.Nodes) == 0 {
		return TargetResolution{}, NewError("backend_unavailable", "structured snapshot is unavailable", nil)
	}

	nameKey := normalizeSnapshotMatchValue(selector.Name)
	roleFamily := normalizeQueryRoleFamily(selector.Role)
	candidates := collectQueryCandidates(snapshot, nameKey, roleFamily)
	ranked := rankSnapshotCandidates(snapshot, candidates, selector, nameKey, roleFamily)
	if len(ranked) == 0 {
		fallback, ok := resolveLabelAnchorFallback(snapshot, selector, nameKey, roleFamily)
		if ok {
			fallback.QueryMS = time.Since(start).Milliseconds()
			return fallback, nil
		}
		return TargetResolution{}, NewError("target_not_found", "target selector did not match any interactive element", map[string]interface{}{
			"name": selector.Name,
			"role": selector.Role,
		})
	}

	sort.SliceStable(ranked, func(i int, j int) bool {
		if ranked[i].score == ranked[j].score {
			if ranked[i].exactName == ranked[j].exactName {
				return ranked[i].order < ranked[j].order
			}
			return ranked[i].exactName
		}
		return ranked[i].score > ranked[j].score
	})

	best := ranked[0]
	if len(ranked) > 1 && ranked[1].score == best.score && ranked[1].exactName == best.exactName {
		return TargetResolution{}, NewError("ambiguous_target", "target selector matched multiple interactive elements", map[string]interface{}{
			"name": selector.Name,
			"role": selector.Role,
		})
	}

	result := buildTargetResolution(snapshot, best.node, len(ranked))
	result.QueryMS = time.Since(start).Milliseconds()
	return result, nil
}

func collectQueryCandidates(snapshot *Snapshot, nameKey string, roleFamily string) []FlatNode {
	seen := make(map[int]struct{}, len(snapshot.Nodes))
	out := make([]FlatNode, 0, len(snapshot.Nodes))

	addNode := func(nodeID int) {
		if nodeID == 0 {
			return
		}
		if _, ok := seen[nodeID]; ok {
			return
		}
		node, ok := snapshotNodeByID(snapshot, nodeID)
		if !ok || !node.referenceable() {
			return
		}
		seen[nodeID] = struct{}{}
		out = append(out, node)
	}

	if nameKey != "" {
		for _, nodeID := range snapshot.NameIndex[nameKey] {
			addNode(nodeID)
		}
		for _, token := range snapshotSearchTokens(selectorNameFromKey(nameKey)) {
			for _, nodeID := range snapshot.NameIndex[token] {
				addNode(nodeID)
			}
		}
	}
	for _, role := range queryRoleAliases(roleFamily) {
		for _, nodeID := range snapshot.RoleIndex[role] {
			addNode(nodeID)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, node := range snapshot.Nodes {
		if node.referenceable() {
			out = append(out, node)
		}
	}
	return out
}

func rankSnapshotCandidates(snapshot *Snapshot, candidates []FlatNode, selector TargetSelector, nameKey string, roleFamily string) []rankedTarget {
	out := make([]rankedTarget, 0, len(candidates))
	for idx, node := range candidates {
		score, exact := snapshotTargetScore(snapshot, node, selector, nameKey, roleFamily)
		if score <= 0 {
			continue
		}
		out = append(out, rankedTarget{
			node:      node,
			score:     score,
			order:     idx,
			exactName: exact,
		})
	}
	if len(out) > 20 {
		sort.SliceStable(out, func(i int, j int) bool {
			if out[i].score == out[j].score {
				if out[i].exactName == out[j].exactName {
					return out[i].order < out[j].order
				}
				return out[i].exactName
			}
			return out[i].score > out[j].score
		})
		return append([]rankedTarget(nil), out[:20]...)
	}
	return out
}

func snapshotTargetScore(snapshot *Snapshot, node FlatNode, selector TargetSelector, nameKey string, roleFamily string) (int, bool) {
	score := 10
	exactName := false
	labelKey := normalizeSnapshotMatchValue(node.label())
	roleKey := normalizeSnapshotMatchValue(node.Role)

	if roleFamily != "" {
		if queryRoleMatches(roleFamily, roleKey) {
			score += 40
		} else {
			score -= 30
		}
	}

	if nameKey != "" {
		switch {
		case labelKey == nameKey:
			score += 120
			exactName = true
		case strings.Contains(labelKey, nameKey):
			score += 45
		case queryTokenOverlap(labelKey, nameKey):
			score += 20
		default:
			score -= 15
		}
		if labelKey == "" {
			score -= 90
		}
	}

	if node.Focused {
		score += 20
	}
	if node.Visible {
		score += 8
	}
	if node.Enabled {
		score += 8
	} else {
		score -= 25
	}

	if roleFamily == "input" {
		if queryLikelyComposer(node) {
			score += 35
		}
		if queryLikelySearch(node) {
			score -= 12
		}
	}
	if roleFamily == "button" || roleFamily == "control" {
		if queryLikelySubmit(node, selector) {
			score += 18
		}
	}
	if strings.EqualFold(strings.TrimSpace(selector.Role), "setting") && queryRoleMatches("setting", roleKey) {
		score += 25
	}
	return score, exactName
}

func resolveLabelAnchorFallback(snapshot *Snapshot, selector TargetSelector, nameKey string, roleFamily string) (TargetResolution, bool) {
	if nameKey == "" {
		return TargetResolution{}, false
	}
	switch roleFamily {
	case "setting", "control", "button", "toggle":
	default:
		return TargetResolution{}, false
	}

	bestScore := 0
	bestNode := FlatNode{}
	tied := false
	for idx, anchor := range snapshot.Nodes {
		anchorKey := normalizeSnapshotMatchValue(anchor.label())
		if anchorKey == "" || (anchorKey != nameKey && !strings.Contains(anchorKey, nameKey)) {
			continue
		}
		for offset := 1; offset <= 3 && idx+offset < len(snapshot.Nodes); offset++ {
			candidate := snapshot.Nodes[idx+offset]
			if !candidate.referenceable() {
				continue
			}
			roleKey := normalizeSnapshotMatchValue(candidate.Role)
			if !queryRoleMatches(roleFamily, roleKey) {
				continue
			}
			score := 60 - (offset * 10)
			if candidate.ParentID == anchor.ParentID {
				score += 10
			}
			if score > bestScore {
				bestScore = score
				bestNode = candidate
				tied = false
				continue
			}
			if score == bestScore {
				tied = true
			}
		}
	}
	if bestScore <= 0 || tied {
		return TargetResolution{}, false
	}
	result := buildTargetResolution(snapshot, bestNode, 1)
	result.Fallbacks = []string{"label_anchor"}
	return result, true
}

func buildTargetResolution(snapshot *Snapshot, node FlatNode, candidateCount int) TargetResolution {
	projection := snapshot.Projection(SnapshotProjectionFull)
	return TargetResolution{
		WindowID:         snapshot.WindowID,
		NodeID:           node.NodeID,
		Ref:              projection.NodeToRef[node.NodeID],
		RefMap:           projection.RefMap,
		Tree:             projection.Tree,
		Token:            node.BackendToken,
		SnapshotRevision: snapshot.Revision,
		NodeCount:        len(snapshot.Nodes),
		CandidateCount:   candidateCount,
	}
}

func snapshotNodeByID(snapshot *Snapshot, nodeID int) (FlatNode, bool) {
	if snapshot == nil || nodeID == 0 {
		return FlatNode{}, false
	}
	for _, node := range snapshot.Nodes {
		if node.NodeID == nodeID {
			return node, true
		}
	}
	return FlatNode{}, false
}

func normalizeQueryRoleFamily(role string) string {
	switch normalizeSnapshotMatchValue(role) {
	case "", "element":
		return ""
	case "input", "field", "text", "text_field", "search", "search_field", "textbox", "composer", "message":
		return "input"
	case "button", "submit":
		return "button"
	case "toggle", "switch", "checkbox", "check_box", "setting":
		return "setting"
	case "control", "click", "click_target", "clickable":
		return "control"
	default:
		return normalizeSnapshotMatchValue(role)
	}
}

func queryRoleAliases(roleFamily string) []string {
	switch roleFamily {
	case "input":
		return []string{"editable_text", "text_field", "text_area", "search_field", "combo_box", "document", "editor"}
	case "button":
		return []string{"button", "push_button", "menu_item", "link"}
	case "setting":
		return []string{"switch", "toggle", "check_box", "checkbox", "button"}
	case "control":
		return []string{"button", "push_button", "switch", "toggle", "check_box", "checkbox", "link", "menu_item", "tab", "radio", "list_item"}
	default:
		if roleFamily == "" {
			return nil
		}
		return []string{roleFamily}
	}
}

func queryRoleMatches(roleFamily string, role string) bool {
	for _, alias := range queryRoleAliases(roleFamily) {
		if role == alias {
			return true
		}
	}
	return false
}

func queryLikelyComposer(node FlatNode) bool {
	text := normalizeSnapshotMatchValue(node.label() + " " + node.Description)
	return snapshotContainsAny(text, "message", "reply", "compose", "chat", "editable", "comment", "send_message")
}

func queryLikelySearch(node FlatNode) bool {
	text := normalizeSnapshotMatchValue(node.label() + " " + node.Description)
	return snapshotContainsAny(text, "search", "find", "lookup", "address_bar")
}

func queryLikelySubmit(node FlatNode, selector TargetSelector) bool {
	text := normalizeSnapshotMatchValue(node.label() + " " + node.Description + " " + selector.Intent + " " + selector.ActType)
	return snapshotContainsAny(text, "send", "submit", "save", "apply", "continue", "confirm", "ok")
}

func queryTokenOverlap(labelKey string, nameKey string) bool {
	if labelKey == "" || nameKey == "" {
		return false
	}
	labelParts := strings.Split(labelKey, "_")
	nameParts := strings.Split(nameKey, "_")
	matches := 0
	for _, want := range nameParts {
		if want == "" {
			continue
		}
		for _, have := range labelParts {
			if have == want {
				matches++
				break
			}
		}
	}
	return matches > 0
}

func selectorNameFromKey(nameKey string) string {
	return strings.ReplaceAll(nameKey, "_", " ")
}
