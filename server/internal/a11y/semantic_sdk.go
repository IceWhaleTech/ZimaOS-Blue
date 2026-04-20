package a11y

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	defaultExpandLines     = 6
	defaultExpandMaxPasses = 3
)

type RawNode = Node
type SemanticNode = FlatNode

type RawTree struct {
	WindowID string   `json:"window_id,omitempty"`
	Title    string   `json:"title,omitempty"`
	Mode     string   `json:"mode,omitempty"`
	Root     *RawNode `json:"root,omitempty"`
}

type CaptureScope struct {
	WindowID string `json:"window_id,omitempty"`
	AppName  string `json:"app_name,omitempty"`
}

type CompileOptions struct {
	Mode string `json:"mode,omitempty"`
}

type Query struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Role       string `json:"role,omitempty"`
	Capability string `json:"capability,omitempty"`
	Ancestor   string `json:"ancestor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type Action struct {
	Op        string `json:"op,omitempty"`
	TargetID  string `json:"target_id,omitempty"`
	Value     string `json:"value,omitempty"`
	Direction string `json:"direction,omitempty"`
	Lines     int    `json:"lines,omitempty"`
	Query     Query  `json:"query,omitempty"`
}

type RefreshHint struct {
	WindowID string `json:"window_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type ExecutionResult struct {
	Success        bool         `json:"success,omitempty"`
	WindowID       string       `json:"window_id,omitempty"`
	ResolvedNodeID string       `json:"resolved_node_id,omitempty"`
	AppliedAction  string       `json:"applied_action,omitempty"`
	Message        string       `json:"message,omitempty"`
	Snapshot       *Snapshot    `json:"snapshot,omitempty"`
	HostResult     ActionResult `json:"host_result,omitempty"`
}

type semanticSDKHost interface {
	HostOS() string
	Capture(ctx context.Context, scope CaptureScope) (RawTree, error)
	Execute(ctx context.Context, windowID string, node SemanticNode, action Action) (ActionResult, error)
	Scroll(ctx context.Context, windowID string, direction string, lines int) (ActionResult, error)
}

type SemanticSDK struct {
	host semanticSDKHost
}

func NewSemanticSDK(backend Backend) *SemanticSDK {
	host, _ := backend.(semanticSDKHost)
	return newSemanticSDKWithHost(host)
}

func newSemanticSDKWithHost(host semanticSDKHost) *SemanticSDK {
	return &SemanticSDK{host: host}
}

func (s *SemanticSDK) Capture(ctx context.Context, scope CaptureScope) (RawTree, error) {
	if s == nil || s.host == nil {
		return RawTree{}, NewError("backend_unavailable", "semantic capture host is unavailable", nil)
	}
	return s.host.Capture(ctx, scope)
}

func (s *SemanticSDK) Compile(rawTree RawTree, options CompileOptions) (*Snapshot, error) {
	if rawTree.Root == nil {
		return nil, NewError("backend_unavailable", "raw accessibility tree is empty", nil)
	}
	snapshot := &Snapshot{
		WindowID:        strings.TrimSpace(rawTree.WindowID),
		Title:           strings.TrimSpace(rawTree.Title),
		Mode:            firstNonEmpty(strings.TrimSpace(options.Mode), strings.TrimSpace(rawTree.Mode), "ax"),
		Nodes:           make([]FlatNode, 0, 32),
		NameIndex:       make(map[string][]int),
		RoleIndex:       make(map[string][]int),
		TokenIndex:      make(map[string]int),
		StableIndex:     make(map[string]int),
		CapabilityIndex: make(map[string][]int),
	}
	state := semanticCompileState{
		snapshot: snapshot,
		nextID:   1,
	}
	state.walk(rawTree.Root, semanticCompileContext{})
	snapshot.rebuildDerived()
	return snapshot, nil
}

func (s *SemanticSDK) Resolve(snapshot *Snapshot, query Query) ([]SemanticNode, error) {
	if snapshot == nil {
		return nil, NewError("backend_unavailable", "semantic snapshot is unavailable", nil)
	}
	ids := semanticCandidateIDs(snapshot, query)
	candidates := make([]SemanticNode, 0, len(ids))
	for _, nodeID := range ids {
		node, ok := snapshotNodeByID(snapshot, nodeID)
		if !ok {
			continue
		}
		if !semanticQueryMatches(node, query) {
			continue
		}
		candidates = append(candidates, node)
	}
	sort.SliceStable(candidates, func(i int, j int) bool {
		return semanticQueryScore(candidates[i], query) > semanticQueryScore(candidates[j], query)
	})
	if query.Limit > 0 && len(candidates) > query.Limit {
		return append([]SemanticNode(nil), candidates[:query.Limit]...), nil
	}
	return candidates, nil
}

func (s *SemanticSDK) Refresh(ctx context.Context, priorSnapshot *Snapshot, hint RefreshHint) (*Snapshot, error) {
	if s == nil || s.host == nil {
		return nil, NewError("backend_unavailable", "semantic refresh host is unavailable", nil)
	}
	scope := CaptureScope{WindowID: strings.TrimSpace(hint.WindowID)}
	if scope.WindowID == "" && priorSnapshot != nil {
		scope.WindowID = strings.TrimSpace(priorSnapshot.WindowID)
	}
	rawTree, err := s.host.Capture(ctx, scope)
	if err != nil {
		return nil, err
	}
	return s.Compile(rawTree, CompileOptions{Mode: rawTree.Mode})
}

func (s *SemanticSDK) Expand(ctx context.Context, snapshot *Snapshot, unresolved Query) (*Snapshot, error) {
	if snapshot == nil {
		return nil, NewError("backend_unavailable", "semantic snapshot is unavailable", nil)
	}
	if matches, err := s.Resolve(snapshot, unresolved); err == nil && len(matches) > 0 {
		return snapshot, nil
	}
	if s == nil || s.host == nil {
		return snapshot, NewError("backend_unavailable", "semantic expansion host is unavailable", nil)
	}
	current := snapshot
	previousSignature := semanticSnapshotSignature(current)
	for attempt := 0; attempt < defaultExpandMaxPasses; attempt++ {
		if _, err := s.host.Scroll(ctx, current.WindowID, "down", defaultExpandLines); err != nil {
			return current, err
		}
		refreshed, err := s.Refresh(ctx, current, RefreshHint{
			WindowID: current.WindowID,
			Reason:   "lazy_expand",
		})
		if err != nil {
			return current, err
		}
		current = mergeSemanticSnapshots(current, refreshed)
		if matches, err := s.Resolve(current, unresolved); err == nil && len(matches) > 0 {
			return current, nil
		}
		signature := semanticSnapshotSignature(current)
		if signature == previousSignature {
			break
		}
		previousSignature = signature
	}
	return current, nil
}

func (s *SemanticSDK) Execute(ctx context.Context, snapshot *Snapshot, action Action) (ExecutionResult, error) {
	if snapshot == nil {
		return ExecutionResult{}, NewError("backend_unavailable", "semantic snapshot is unavailable", nil)
	}
	if s == nil || s.host == nil {
		return ExecutionResult{}, NewError("backend_unavailable", "semantic execution host is unavailable", nil)
	}
	action.Op = semanticNormalizeAction(action.Op)
	if action.Op == "" {
		return ExecutionResult{}, NewError("unsupported_action", "action op is required", nil)
	}
	if action.Op == "scroll" {
		hostResult, err := s.host.Scroll(ctx, snapshot.WindowID, firstNonEmpty(action.Direction, "down"), normalizeScrollLines(action.Lines))
		if err != nil {
			return ExecutionResult{}, err
		}
		refreshed, refreshErr := s.Refresh(ctx, snapshot, RefreshHint{WindowID: snapshot.WindowID, Reason: "post_scroll"})
		if refreshErr != nil {
			refreshed = snapshot
		}
		return ExecutionResult{
			Success:       true,
			WindowID:      snapshot.WindowID,
			AppliedAction: "scroll",
			Message:       hostResult.Message,
			Snapshot:      refreshed,
			HostResult:    hostResult,
		}, nil
	}

	query := action.Query
	if query.ID == "" && strings.TrimSpace(action.TargetID) != "" {
		query.ID = strings.TrimSpace(action.TargetID)
	}
	if query.Capability == "" {
		query.Capability = semanticRequiredCapability(action.Op)
	}
	matches, err := s.Resolve(snapshot, query)
	if err != nil {
		return ExecutionResult{}, err
	}
	switch len(matches) {
	case 0:
		return ExecutionResult{}, NewError("target_not_found", "semantic query did not match any element", map[string]interface{}{
			"query": query,
		})
	case 1:
	default:
		return ExecutionResult{}, NewError("ambiguous_target", "semantic query matched multiple elements", map[string]interface{}{
			"query":           query,
			"candidate_count": len(matches),
		})
	}
	node := matches[0]
	required := semanticRequiredCapability(action.Op)
	if required != "" && !semanticNodeHasCapability(node, required) {
		return ExecutionResult{}, NewError("unsupported_action", fmt.Sprintf("element does not support %s", required), map[string]interface{}{
			"node_id":    node.StableID,
			"capability": required,
		})
	}
	hostResult, err := s.host.Execute(ctx, snapshot.WindowID, node, action)
	if err != nil {
		return ExecutionResult{}, err
	}
	refreshed, refreshErr := s.Refresh(ctx, snapshot, RefreshHint{
		WindowID: snapshot.WindowID,
		Reason:   "post_action",
	})
	if refreshErr != nil {
		refreshed = snapshot
	}
	return ExecutionResult{
		Success:        true,
		WindowID:       snapshot.WindowID,
		ResolvedNodeID: node.StableID,
		AppliedAction:  action.Op,
		Message:        hostResult.Message,
		Snapshot:       refreshed,
		HostResult:     hostResult,
	}, nil
}

type semanticCompileState struct {
	snapshot *Snapshot
	nextID   int
}

type semanticCompileContext struct {
	parentID       int
	parentStableID string
	depth          int
	pathHints      []string
	section        string
}

func (s *semanticCompileState) walk(node *RawNode, ctx semanticCompileContext) {
	if node == nil || s == nil || s.snapshot == nil {
		return
	}

	role := strings.TrimSpace(node.Role)
	label := semanticNodeLabel(node)
	defaultAction := semanticDefaultAction(node)
	emit := semanticShouldEmitNode(node, ctx.depth == 0)
	pathHints := append([]string(nil), ctx.pathHints...)
	nextContext := ctx

	if emit {
		flat := FlatNode{
			NodeID:           s.nextID,
			ParentID:         ctx.parentID,
			Role:             role,
			Name:             label,
			Value:            strings.TrimSpace(node.Value),
			Path:             strings.Join(pathHints, " > "),
			Section:          ctx.section,
			Capabilities:     semanticCapabilities(node, role, defaultAction),
			Bounds:           node.Bounds,
			State:            strings.TrimSpace(node.Description),
			Description:      strings.TrimSpace(node.Description),
			DefaultAction:    defaultAction,
			Visible:          !snapshotContainsAny(normalizeSnapshotMatchValue(node.Description), "offscreen", "invisible", "hidden"),
			Enabled:          semanticNodeEnabled(node),
			Focused:          node.Focused || snapshotContainsAny(normalizeSnapshotMatchValue(node.Description), "focused", "active", "editable"),
			Interactive:      node.Interactive || len(node.Actions) > 0 || node.ValueSettable,
			Depth:            ctx.depth,
			InteractiveDepth: ctx.depth,
		}
		if flat.Role == "" {
			flat.Role = "element"
		}
		if flat.Section == "" && len(pathHints) > 0 {
			flat.Section = pathHints[len(pathHints)-1]
		}
		flat.BackendToken = strings.TrimSpace(node.Token)
		flat.StableID = snapshotStableID(ctx.parentStableID, flat)
		s.snapshot.Nodes = append(s.snapshot.Nodes, flat)
		s.nextID++

		nextContext.parentID = flat.NodeID
		nextContext.parentStableID = flat.StableID
		nextContext.depth = ctx.depth + 1
		nextContext.section = semanticNextSection(flat, ctx.section)
		if semanticNodeCarriesPath(node, emit) {
			nextContext.pathHints = semanticAppendPathHint(pathHints, semanticPathHint(role, label))
		}
	} else {
		if semanticNodeCarriesPath(node, emit) {
			hint := semanticPathHint(role, firstNonEmpty(label, node.Name))
			if hint != "" {
				nextContext.pathHints = semanticAppendPathHint(pathHints, hint)
				nextContext.section = hint
			}
		}
	}

	for _, child := range node.Children {
		s.walk(child, nextContext)
	}
}

func semanticNodeEnabled(node *RawNode) bool {
	if node == nil {
		return false
	}
	if node.Enabled {
		return true
	}
	return !snapshotContainsAny(normalizeSnapshotMatchValue(node.Description), "disabled", "unavailable")
}

func semanticShouldEmitNode(node *RawNode, isRoot bool) bool {
	if node == nil {
		return false
	}
	if isRoot {
		return true
	}
	role := normalizeSnapshotMatchValue(node.Role)
	if semanticLayoutRole(role) && strings.TrimSpace(node.Token) == "" && !node.Interactive && !node.ValueSettable && len(node.Actions) == 0 {
		return false
	}
	if strings.TrimSpace(node.Token) != "" {
		return true
	}
	if node.Interactive || node.ValueSettable || len(node.Actions) > 0 || strings.TrimSpace(node.DefaultAction) != "" {
		return true
	}
	if role == "text" || role == "static_text" {
		return false
	}
	if semanticNodeLabel(node) != "" && !semanticLayoutRole(role) {
		return true
	}
	return false
}

func semanticLayoutRole(role string) bool {
	return snapshotContainsAny(role, "group", "pane", "layout", "splitter", "scroll_area", "section", "toolbar", "list", "outline", "table")
}

func semanticNodeCarriesPath(node *RawNode, emitted bool) bool {
	if node == nil {
		return false
	}
	role := normalizeSnapshotMatchValue(node.Role)
	if emitted {
		return snapshotContainsAny(role, "window", "toolbar", "list", "outline", "table", "menu", "tab_group")
	}
	return semanticLayoutRole(role) && strings.TrimSpace(node.Name) != ""
}

func semanticPathHint(role string, label string) string {
	parts := []string{normalizeSnapshotMatchValue(label)}
	if parts[0] == "" {
		parts[0] = normalizeSnapshotMatchValue(role)
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func semanticAppendPathHint(pathHints []string, hint string) []string {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return append([]string(nil), pathHints...)
	}
	out := append([]string(nil), pathHints...)
	if len(out) > 0 && out[len(out)-1] == hint {
		return out
	}
	return append(out, hint)
}

func semanticNextSection(node FlatNode, current string) string {
	if strings.TrimSpace(node.Name) == "" {
		return current
	}
	role := normalizeSnapshotMatchValue(node.Role)
	if semanticLayoutRole(role) || snapshotContainsAny(role, "window", "list", "outline", "table", "menu") {
		return semanticPathHint(node.Role, node.Name)
	}
	return current
}

func semanticNodeLabel(node *RawNode) string {
	if node == nil {
		return ""
	}
	for _, candidate := range []string{
		strings.TrimSpace(node.Name),
		strings.TrimSpace(node.Value),
		semanticDescendantLabel(node, 0),
		strings.TrimSpace(node.Description),
	} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func semanticDescendantLabel(node *RawNode, depth int) string {
	if node == nil || depth > 2 {
		return ""
	}
	for _, child := range node.Children {
		for _, candidate := range []string{strings.TrimSpace(child.Name), strings.TrimSpace(child.Value)} {
			if candidate != "" && !child.Interactive {
				return candidate
			}
		}
		if nested := semanticDescendantLabel(child, depth+1); nested != "" {
			return nested
		}
	}
	return ""
}

func semanticDefaultAction(node *RawNode) string {
	if node == nil {
		return ""
	}
	if action := strings.TrimSpace(node.DefaultAction); action != "" {
		return action
	}
	for _, action := range node.Actions {
		switch strings.TrimSpace(strings.ToLower(action)) {
		case "axpress":
			return "press"
		case "axconfirm":
			return "confirm"
		case "axraise":
			return "raise"
		case "axshowmenu":
			return "show_menu"
		}
	}
	return ""
}

func semanticCapabilities(node *RawNode, role string, defaultAction string) []string {
	normalizedRole := normalizeSnapshotMatchValue(role)
	normalizedAction := normalizeSnapshotMatchValue(defaultAction)
	seen := make(map[string]struct{}, 8)
	add := func(values ...string) {
		for _, value := range values {
			key := normalizeSnapshotMatchValue(value)
			if key == "" {
				continue
			}
			seen[key] = struct{}{}
		}
	}

	if node != nil && node.ValueSettable {
		add("set_value", "focus")
	}
	if containsAny(normalizedAction, "press", "confirm") || semanticNodeHasAction(node, "axpress", "axconfirm") {
		add("press")
	}
	if containsAny(normalizedAction, "raise") || semanticNodeHasAction(node, "axraise") {
		add("focus")
	}
	if containsAny(normalizedAction, "show_menu") || semanticNodeHasAction(node, "axshowmenu") {
		add("show_menu")
	}
	if containsAny(normalizedRole, "text_field", "text_area", "editable_text", "document", "editor") {
		add("focus")
	}
	if containsAny(normalizedRole, "scroll", "list", "outline", "table") {
		add("scroll")
	}
	if semanticNodeHasCapabilityHint(normalizedRole, normalizedAction, "select") {
		add("select")
	}
	if node != nil && node.Expanded != nil {
		if *node.Expanded {
			add("collapse")
		} else {
			add("expand")
		}
	}

	out := make([]string, 0, len(seen))
	for capability := range seen {
		out = append(out, capability)
	}
	sort.Strings(out)
	return out
}

func semanticNodeHasAction(node *RawNode, expected ...string) bool {
	if node == nil {
		return false
	}
	for _, action := range node.Actions {
		for _, want := range expected {
			if strings.EqualFold(strings.TrimSpace(action), strings.TrimSpace(want)) {
				return true
			}
		}
	}
	return false
}

func semanticNodeHasCapabilityHint(role string, defaultAction string, capability string) bool {
	switch capability {
	case "select":
		return supportsDarwinSelection(role, defaultAction) || supportsWindowsSelection(role, defaultAction, "")
	default:
		return false
	}
}

func semanticCandidateIDs(snapshot *Snapshot, query Query) []int {
	if snapshot == nil {
		return nil
	}
	seen := make(map[int]struct{}, len(snapshot.Nodes))
	out := make([]int, 0, len(snapshot.Nodes))
	addID := func(nodeID int) {
		if nodeID == 0 {
			return
		}
		if _, ok := seen[nodeID]; ok {
			return
		}
		seen[nodeID] = struct{}{}
		out = append(out, nodeID)
	}

	if query.ID != "" {
		addID(snapshot.StableIndex[strings.TrimSpace(query.ID)])
	}
	for _, key := range append([]string{normalizeSnapshotMatchValue(query.Name)}, snapshotSearchTokens(query.Name)...) {
		for _, nodeID := range snapshot.NameIndex[key] {
			addID(nodeID)
		}
	}
	for _, role := range queryRoleAliases(normalizeQueryRoleFamily(query.Role)) {
		for _, nodeID := range snapshot.RoleIndex[role] {
			addID(nodeID)
		}
	}
	if capability := normalizeSnapshotMatchValue(query.Capability); capability != "" {
		for _, nodeID := range snapshot.CapabilityIndex[capability] {
			addID(nodeID)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, node := range snapshot.Nodes {
		addID(node.NodeID)
	}
	return out
}

func semanticQueryMatches(node SemanticNode, query Query) bool {
	if query.ID != "" && strings.TrimSpace(node.StableID) != strings.TrimSpace(query.ID) {
		return false
	}
	roleFamily := normalizeQueryRoleFamily(query.Role)
	if roleFamily != "" && !queryRoleMatches(roleFamily, normalizeSnapshotMatchValue(node.Role)) {
		return false
	}
	nameKey := normalizeSnapshotMatchValue(query.Name)
	if nameKey != "" {
		labelKey := normalizeSnapshotMatchValue(node.label())
		if labelKey != nameKey && !strings.Contains(labelKey, nameKey) && !queryTokenOverlap(labelKey, nameKey) {
			return false
		}
	}
	capabilityKey := normalizeSnapshotMatchValue(query.Capability)
	if capabilityKey != "" && !semanticNodeHasCapability(node, capabilityKey) {
		return false
	}
	ancestorKey := normalizeSnapshotMatchValue(query.Ancestor)
	if ancestorKey != "" {
		pathKey := normalizeSnapshotMatchValue(node.Path + " " + node.Section)
		if !strings.Contains(pathKey, ancestorKey) {
			return false
		}
	}
	return true
}

func semanticQueryScore(node SemanticNode, query Query) int {
	score := 0
	if query.ID != "" && strings.TrimSpace(node.StableID) == strings.TrimSpace(query.ID) {
		score += 200
	}
	if nameKey := normalizeSnapshotMatchValue(query.Name); nameKey != "" {
		labelKey := normalizeSnapshotMatchValue(node.label())
		switch {
		case labelKey == nameKey:
			score += 100
		case strings.Contains(labelKey, nameKey):
			score += 40
		case queryTokenOverlap(labelKey, nameKey):
			score += 20
		}
	}
	if roleFamily := normalizeQueryRoleFamily(query.Role); roleFamily != "" && queryRoleMatches(roleFamily, normalizeSnapshotMatchValue(node.Role)) {
		score += 30
	}
	if capabilityKey := normalizeSnapshotMatchValue(query.Capability); capabilityKey != "" && semanticNodeHasCapability(node, capabilityKey) {
		score += 25
	}
	if ancestorKey := normalizeSnapshotMatchValue(query.Ancestor); ancestorKey != "" {
		pathKey := normalizeSnapshotMatchValue(node.Path + " " + node.Section)
		if strings.Contains(pathKey, ancestorKey) {
			score += 20
		}
	}
	if node.Focused {
		score += 10
	}
	if node.Enabled {
		score += 4
	}
	if node.Visible {
		score += 4
	}
	return score
}

func semanticNodeHasCapability(node SemanticNode, capability string) bool {
	capability = normalizeSnapshotMatchValue(capability)
	if capability == "" {
		return false
	}
	for _, candidate := range node.Capabilities {
		if normalizeSnapshotMatchValue(candidate) == capability {
			return true
		}
	}
	return false
}

func semanticRequiredCapability(op string) string {
	switch semanticNormalizeAction(op) {
	case "press":
		return "press"
	case "set_value":
		return "set_value"
	case "select":
		return "select"
	case "focus":
		return "focus"
	case "show_menu":
		return "show_menu"
	case "expand":
		return "expand"
	case "collapse":
		return "collapse"
	default:
		return ""
	}
}

func semanticNormalizeAction(op string) string {
	switch normalizeSnapshotMatchValue(op) {
	case "click", "submit", "press":
		return "press"
	case "type", "set_value", "setvalue":
		return "set_value"
	case "show_menu", "menu":
		return "show_menu"
	case "focus":
		return "focus"
	case "select":
		return "select"
	case "expand":
		return "expand"
	case "collapse":
		return "collapse"
	case "scroll":
		return "scroll"
	default:
		return normalizeSnapshotMatchValue(op)
	}
}

func mergeSemanticSnapshots(base *Snapshot, next *Snapshot) *Snapshot {
	if base == nil {
		return next
	}
	if next == nil {
		return base
	}
	out := next.Clone()
	if out == nil {
		return base
	}
	out.WindowID = firstNonEmpty(next.WindowID, base.WindowID)
	out.Title = firstNonEmpty(next.Title, base.Title)

	type nodeWithParent struct {
		node      FlatNode
		parentKey string
	}

	baseParentKeys := semanticParentKeys(base)
	nextParentKeys := semanticParentKeys(next)
	seen := make(map[string]struct{}, len(base.Nodes)+len(next.Nodes))
	merged := make([]nodeWithParent, 0, len(base.Nodes)+len(next.Nodes))
	appendNode := func(node FlatNode, parentKey string) {
		key := semanticNodeMergeKey(node)
		if key == "" {
			key = fmt.Sprintf("node:%d", len(merged)+1)
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, nodeWithParent{
			node:      node,
			parentKey: parentKey,
		})
	}
	for _, node := range base.Nodes {
		appendNode(node, baseParentKeys[node.NodeID])
	}
	for _, node := range next.Nodes {
		appendNode(node, nextParentKeys[node.NodeID])
	}
	reindexed := make([]FlatNode, 0, len(merged))
	keyToID := make(map[string]int, len(merged))
	for idx := range merged {
		merged[idx].node.NodeID = idx + 1
		merged[idx].node.ParentID = 0
		keyToID[semanticNodeMergeKey(merged[idx].node)] = merged[idx].node.NodeID
		reindexed = append(reindexed, merged[idx].node)
	}
	for idx := range merged {
		parentID := keyToID[merged[idx].parentKey]
		reindexed[idx].ParentID = parentID
	}
	out.Nodes = reindexed
	out.rebuildDerived()
	return out
}

func semanticNodeMergeKey(node FlatNode) string {
	for _, value := range []string{
		strings.TrimSpace(node.StableID),
		strings.TrimSpace(node.BackendToken),
		normalizeSnapshotMatchValue(node.Role + "|" + node.Name + "|" + node.Path),
	} {
		if value != "" {
			return value
		}
	}
	return ""
}

func semanticParentKeys(snapshot *Snapshot) map[int]string {
	if snapshot == nil {
		return nil
	}
	byID := make(map[int]FlatNode, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		byID[node.NodeID] = node
	}
	out := make(map[int]string, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		parent, ok := byID[node.ParentID]
		if !ok {
			continue
		}
		out[node.NodeID] = semanticNodeMergeKey(parent)
	}
	return out
}

func semanticSnapshotSignature(snapshot *Snapshot) string {
	if snapshot == nil {
		return ""
	}
	keys := make([]string, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		if key := semanticNodeMergeKey(node); key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return strings.Join(keys, "|")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
