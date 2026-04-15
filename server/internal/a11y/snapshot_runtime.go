package a11y

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type SnapshotProjectionMode string

const (
	SnapshotProjectionFull        SnapshotProjectionMode = "full"
	SnapshotProjectionInteractive SnapshotProjectionMode = "interactive"
)

type BuildStructuredSnapshotOptions struct {
	WindowID string
	Title    string
	Mode     string
}

type FlatNode struct {
	NodeID           int
	BackendToken     string
	ParentID         int
	Role             string
	Name             string
	Value            string
	Bounds           NormalizedRect
	State            string
	Description      string
	DefaultAction    string
	Visible          bool
	Enabled          bool
	Focused          bool
	Interactive      bool
	Depth            int
	InteractiveDepth int
}

func (n FlatNode) referenceable() bool {
	return n.Interactive || strings.TrimSpace(n.DefaultAction) != "" || strings.TrimSpace(n.BackendToken) != ""
}

func (n FlatNode) label() string {
	for _, candidate := range []string{n.Name, n.Value, n.Description, n.State} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type SnapshotProjection struct {
	Mode      SnapshotProjectionMode
	Tree      string
	RefMap    map[int]string
	NodeToRef map[int]int
}

type Snapshot struct {
	WindowID    string
	Title       string
	Revision    int64
	Mode        string
	Dirty       bool
	DirtyReason string
	Nodes       []FlatNode
	NameIndex   map[string][]int
	RoleIndex   map[string][]int
	TokenIndex  map[string]int

	fullProjection        SnapshotProjection
	interactiveProjection SnapshotProjection
}

func BuildStructuredSnapshot(options BuildStructuredSnapshotOptions, root *Node) *Snapshot {
	if root == nil {
		return nil
	}
	snapshot := &Snapshot{
		WindowID: strings.TrimSpace(options.WindowID),
		Title:    strings.TrimSpace(options.Title),
		Mode:     strings.TrimSpace(options.Mode),
		Nodes:    make([]FlatNode, 0, 32),
		NameIndex: make(map[string][]int),
		RoleIndex: make(map[string][]int),
		TokenIndex: make(map[string]int),
	}
	nextID := 1
	buildStructuredSnapshotNodes(root, 0, 0, 0, &nextID, snapshot)
	snapshot.rebuildDerived()
	return snapshot
}

func buildStructuredSnapshotNodes(node *Node, parentID int, depth int, interactiveDepth int, nextID *int, snapshot *Snapshot) {
	if node == nil || snapshot == nil || nextID == nil {
		return
	}
	nodeID := *nextID
	*nextID = *nextID + 1

	description := strings.TrimSpace(node.Description)
	normalizedDescription := normalizeSnapshotMatchValue(description)
	visible := !snapshotContainsAny(normalizedDescription, "offscreen", "invisible", "hidden")
	enabled := !snapshotContainsAny(normalizedDescription, "disabled", "unavailable")
	focused := snapshotContainsAny(normalizedDescription, "focused", "selected", "active", "editable")

	flat := FlatNode{
		NodeID:           nodeID,
		BackendToken:     strings.TrimSpace(node.Token),
		ParentID:         parentID,
		Role:             strings.TrimSpace(node.Role),
		Name:             strings.TrimSpace(node.Name),
		Value:            strings.TrimSpace(node.Value),
		State:            description,
		Description:      description,
		DefaultAction:    strings.TrimSpace(node.DefaultAction),
		Visible:          visible,
		Enabled:          enabled,
		Focused:          focused,
		Interactive:      node.referenceable(),
		Depth:            max(depth, 0),
		InteractiveDepth: max(interactiveDepth, 0),
	}
	snapshot.Nodes = append(snapshot.Nodes, flat)

	if key := normalizeSnapshotMatchValue(flat.Name); key != "" {
		snapshot.NameIndex[key] = append(snapshot.NameIndex[key], nodeID)
	}
	for _, token := range snapshotSearchTokens(flat.Name) {
		snapshot.NameIndex[token] = append(snapshot.NameIndex[token], nodeID)
	}
	roleKey := normalizeSnapshotMatchValue(flat.Role)
	if roleKey == "" {
		roleKey = "element"
	}
	snapshot.RoleIndex[roleKey] = append(snapshot.RoleIndex[roleKey], nodeID)
	if flat.BackendToken != "" {
		snapshot.TokenIndex[flat.BackendToken] = nodeID
	}

	nextInteractiveDepth := interactiveDepth
	if flat.referenceable() {
		nextInteractiveDepth++
	}
	for _, child := range node.Children {
		buildStructuredSnapshotNodes(child, nodeID, depth+1, nextInteractiveDepth, nextID, snapshot)
	}
}

func (s *Snapshot) LookupToken(token string) int {
	if s == nil {
		return 0
	}
	return s.TokenIndex[strings.TrimSpace(token)]
}

func (s *Snapshot) Projection(mode SnapshotProjectionMode) SnapshotProjection {
	if s == nil {
		return SnapshotProjection{}
	}
	switch mode {
	case SnapshotProjectionInteractive:
		return cloneSnapshotProjection(s.interactiveProjection)
	default:
		return cloneSnapshotProjection(s.fullProjection)
	}
}

func (s *Snapshot) Clone() *Snapshot {
	if s == nil {
		return nil
	}
	out := &Snapshot{
		WindowID:    s.WindowID,
		Title:       s.Title,
		Revision:    s.Revision,
		Mode:        s.Mode,
		Dirty:       s.Dirty,
		DirtyReason: s.DirtyReason,
		Nodes:       append([]FlatNode(nil), s.Nodes...),
		NameIndex:   cloneNodeIndex(s.NameIndex),
		RoleIndex:   cloneNodeIndex(s.RoleIndex),
		TokenIndex:  cloneTokenIndex(s.TokenIndex),
	}
	return out
}

func (s *Snapshot) rebuildDerived() {
	if s == nil {
		return
	}
	s.NameIndex = make(map[string][]int)
	s.RoleIndex = make(map[string][]int)
	s.TokenIndex = make(map[string]int)
	for _, node := range s.Nodes {
		if key := normalizeSnapshotMatchValue(node.Name); key != "" {
			s.NameIndex[key] = append(s.NameIndex[key], node.NodeID)
		}
		for _, token := range snapshotSearchTokens(node.Name) {
			s.NameIndex[token] = append(s.NameIndex[token], node.NodeID)
		}
		roleKey := normalizeSnapshotMatchValue(node.Role)
		if roleKey == "" {
			roleKey = "element"
		}
		s.RoleIndex[roleKey] = append(s.RoleIndex[roleKey], node.NodeID)
		if token := strings.TrimSpace(node.BackendToken); token != "" {
			s.TokenIndex[token] = node.NodeID
		}
	}
	s.fullProjection = buildSnapshotProjection(s.Nodes, SnapshotProjectionFull)
	s.interactiveProjection = buildSnapshotProjection(s.Nodes, SnapshotProjectionInteractive)
}

func buildSnapshotProjection(nodes []FlatNode, mode SnapshotProjectionMode) SnapshotProjection {
	projection := SnapshotProjection{
		Mode:      mode,
		RefMap:    make(map[int]string),
		NodeToRef: make(map[int]int),
	}
	if len(nodes) == 0 {
		projection.RefMap = nil
		projection.NodeToRef = nil
		return projection
	}

	lines := make([]string, 0, len(nodes))
	ref := 1
	if mode == SnapshotProjectionInteractive {
		referenceable := make([]FlatNode, 0, len(nodes))
		for _, node := range nodes {
			if node.referenceable() {
				referenceable = append(referenceable, node)
			}
		}
		if len(referenceable) > interactiveSnapshotMaxRefs {
			ranked := make([]flatScoredNode, 0, len(referenceable))
			for idx, node := range referenceable {
				ranked = append(ranked, flatScoredNode{
					node:  node,
					score: interactiveSnapshotPriorityFlat(node),
					order: idx,
				})
			}
			sort.SliceStable(ranked, func(i int, j int) bool {
				if ranked[i].score == ranked[j].score {
					return ranked[i].order < ranked[j].order
				}
				return ranked[i].score > ranked[j].score
			})
			for _, item := range ranked[:interactiveSnapshotMaxRefs] {
				line := formatFlatSnapshotLine(item.node, 0)
				if line == "" {
					continue
				}
				line = formatSnapshotLineWithRef(ref, line)
				lines = append(lines, line)
				projection.NodeToRef[item.node.NodeID] = ref
				if item.node.BackendToken != "" {
					projection.RefMap[ref] = item.node.BackendToken
				}
				ref++
			}
			projection.Tree = strings.Join(lines, "\n")
			if len(projection.RefMap) == 0 {
				projection.RefMap = nil
			}
			if len(projection.NodeToRef) == 0 {
				projection.NodeToRef = nil
			}
			return projection
		}

		for _, node := range nodes {
			if !node.referenceable() {
				continue
			}
			line := formatFlatSnapshotLine(node, node.InteractiveDepth)
			if line == "" {
				continue
			}
			line = formatSnapshotLineWithRef(ref, line)
			lines = append(lines, line)
			projection.NodeToRef[node.NodeID] = ref
			if node.BackendToken != "" {
				projection.RefMap[ref] = node.BackendToken
			}
			ref++
		}
	} else {
		for _, node := range nodes {
			line := formatFlatSnapshotLine(node, node.Depth)
			if line == "" {
				continue
			}
			if node.referenceable() {
				line = formatSnapshotLineWithRef(ref, line)
				projection.NodeToRef[node.NodeID] = ref
				if node.BackendToken != "" {
					projection.RefMap[ref] = node.BackendToken
				}
				ref++
			}
			lines = append(lines, line)
		}
	}
	if len(lines) > 0 {
		projection.Tree = strings.Join(lines, "\n")
	}
	if len(projection.RefMap) == 0 {
		projection.RefMap = nil
	}
	if len(projection.NodeToRef) == 0 {
		projection.NodeToRef = nil
	}
	return projection
}

type flatScoredNode struct {
	node  FlatNode
	score int
	order int
}

func formatFlatSnapshotLine(node FlatNode, depth int) string {
	role := strings.TrimSpace(node.Role)
	if role == "" {
		role = "element"
	}
	label := node.label()
	indent := strings.Repeat("  ", max(depth, 0))
	if label == "" {
		return indent + "[" + role + "]"
	}
	return fmt.Sprintf("%s[%s] %q", indent, role, label)
}

func interactiveSnapshotPriorityFlat(node FlatNode) int {
	role := normalizeSnapshotMatchValue(node.Role)
	label := normalizeSnapshotMatchValue(node.label())
	description := normalizeSnapshotMatchValue(node.Description)
	isWindowChrome := interactiveSnapshotIsWindowChrome(role, label, description)
	isShellNavigation := !isWindowChrome && interactiveSnapshotIsShellNavigation(role, label, description)

	score := 0
	switch {
	case snapshotContainsAny(role, "editable_text", "text_field", "text_area", "search_field", "combo_box", "document", "search", "editor"):
		score += 110
	case snapshotContainsAny(role, "button", "push_button", "switch", "check_box", "checkbox", "toggle"):
		score += 70
	case snapshotContainsAny(role, "tab", "radio", "link", "menu_item"):
		score += 45
	case snapshotContainsAny(role, "list_item", "tree_item", "row", "cell", "list", "menu", "outline"):
		score += 25
	default:
		score += 15
	}
	if label != "" {
		score += 10
	}
	if !isWindowChrome && !isShellNavigation && snapshotContainsAny(label, "message", "send", "reply", "search", "chat", "conversation", "contact", "compose", "new_chat", "new_message", "消息", "发送", "回复", "搜索", "聊天", "会话", "联系人", "新聊天", "新消息") {
		score += 50
	}
	if node.Focused || snapshotContainsAny(description, "focused", "selected", "active", "editable", "checked", "expanded") {
		score += 20
	}
	if !node.Enabled || !node.Visible || snapshotContainsAny(description, "disabled", "unavailable", "offscreen", "invisible") {
		score -= 20
	}
	if isWindowChrome {
		score -= 90
	} else if isShellNavigation {
		score -= interactiveSnapshotShellNavigationPenalty(role, label)
	}
	return score
}

func snapshotSearchTokens(value string) []string {
	normalized := normalizeSnapshotMatchValue(value)
	if normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, "_")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func cloneSnapshotProjection(in SnapshotProjection) SnapshotProjection {
	return SnapshotProjection{
		Mode:      in.Mode,
		Tree:      in.Tree,
		RefMap:    cloneRefMap(in.RefMap),
		NodeToRef: cloneNodeToRef(in.NodeToRef),
	}
}

func cloneNodeIndex(in map[string][]int) map[string][]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]int, len(in))
	for key, value := range in {
		out[key] = append([]int(nil), value...)
	}
	return out
}

func cloneTokenIndex(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneRefMap(in map[int]string) map[int]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneNodeToRef(in map[int]int) map[int]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

type snapshotSlot struct {
	current atomic.Pointer[Snapshot]
}

type SnapshotStore struct {
	mu           sync.Mutex
	slots        map[string]*snapshotSlot
	nextRevision atomic.Int64
}

func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{
		slots: make(map[string]*snapshotSlot),
	}
}

func (s *SnapshotStore) Current(windowID string) (*Snapshot, bool) {
	if s == nil {
		return nil, false
	}
	slot := s.slot(strings.TrimSpace(windowID), false)
	if slot == nil {
		return nil, false
	}
	current := slot.current.Load()
	if current == nil {
		return nil, false
	}
	return current, true
}

func (s *SnapshotStore) Swap(snapshot *Snapshot) *Snapshot {
	if s == nil || snapshot == nil || strings.TrimSpace(snapshot.WindowID) == "" {
		return nil
	}
	out := snapshot.Clone()
	out.Dirty = false
	out.DirtyReason = ""
	out.Revision = s.nextRevision.Add(1)
	out.rebuildDerived()
	slot := s.slot(out.WindowID, true)
	slot.current.Store(out)
	return out
}

func (s *SnapshotStore) MarkDirty(windowID string, reason string) (*Snapshot, bool) {
	current, ok := s.Current(windowID)
	if !ok {
		return nil, false
	}
	out := current.Clone()
	out.Dirty = true
	out.DirtyReason = strings.TrimSpace(reason)
	out.Revision = s.nextRevision.Add(1)
	out.rebuildDerived()
	slot := s.slot(out.WindowID, true)
	slot.current.Store(out)
	return out, true
}

func (s *SnapshotStore) ApplyPatch(windowID string, reason string, patch func(existing *Snapshot) *Snapshot) (*Snapshot, bool) {
	current, ok := s.Current(windowID)
	if !ok || patch == nil {
		return nil, false
	}
	out := patch(current)
	if out == nil {
		return nil, false
	}
	if strings.TrimSpace(out.WindowID) == "" {
		out.WindowID = current.WindowID
	}
	out.Dirty = false
	out.DirtyReason = strings.TrimSpace(reason)
	out.Revision = s.nextRevision.Add(1)
	out.rebuildDerived()
	slot := s.slot(out.WindowID, true)
	slot.current.Store(out)
	return out, true
}

func (s *SnapshotStore) slot(windowID string, create bool) *snapshotSlot {
	windowID = strings.TrimSpace(windowID)
	if s == nil || windowID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.slots[windowID]
	if slot == nil && create {
		slot = &snapshotSlot{}
		s.slots[windowID] = slot
	}
	return slot
}
