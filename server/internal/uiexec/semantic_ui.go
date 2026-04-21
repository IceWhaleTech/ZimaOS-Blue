package uiexec

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type SemanticUI struct {
	Snapshot *a11y.Snapshot
	Nodes    []Node
	Finder   *Finder
}

func BuildSemanticUI(snapshot *a11y.Snapshot) *SemanticUI {
	if snapshot == nil {
		return &SemanticUI{}
	}
	nodes := make([]Node, 0, len(snapshot.Nodes))
	for _, flat := range snapshot.Nodes {
		n, ok := nodeFromFlat(flat)
		if !ok {
			continue
		}
		nodes = append(nodes, n)
	}
	return &SemanticUI{
		Snapshot: snapshot,
		Nodes:    nodes,
		Finder:   NewFinder(nodes),
	}
}

func BuildSemanticUIFromRawTree(sdk *a11y.SemanticSDK, raw a11y.RawTree) (*SemanticUI, error) {
	if sdk == nil {
		sdk = &a11y.SemanticSDK{}
	}
	snapshot, err := sdk.Compile(raw, a11y.CompileOptions{Mode: raw.Mode})
	if err != nil {
		return nil, err
	}
	return BuildSemanticUI(snapshot), nil
}

func nodeFromFlat(flat a11y.FlatNode) (Node, bool) {
	roleKey := strings.TrimSpace(strings.ToLower(flat.Role))
	name := strings.TrimSpace(flat.Name)

	// Spec-driven denoising: drop layout-ish nodes and nameless nodes.
	if isLayoutRole(roleKey) {
		return Node{}, false
	}
	if name == "" {
		return Node{}, false
	}

	return Node{
		ID:      strings.TrimSpace(flat.StableID),
		Role:    strings.TrimSpace(flat.Role),
		Name:    name,
		Path:    strings.TrimSpace(flat.Path),
		Actions: primitiveActionsFromCapabilities(flat.Capabilities),
		Visible: flat.Visible,
		Enabled: flat.Enabled,
		Focused: flat.Focused,
	}, true
}

func isLayoutRole(role string) bool {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "group", "layout", "pane", "splitter", "scroll_area", "section", "toolbar":
		return true
	default:
		return false
	}
}

func primitiveActionsFromCapabilities(capabilities []string) []string {
	if len(capabilities) == 0 {
		return nil
	}
	out := make([]string, 0, 4)
	add := func(action string) {
		key := normalizeText(action)
		if key == "" {
			return
		}
		for _, existing := range out {
			if existing == key {
				return
			}
		}
		out = append(out, key)
	}

	for _, cap := range capabilities {
		switch normalizeText(cap) {
		case "press", "click":
			add("click")
			add("invoke")
		case "set_value", "type":
			add("type")
			add("focus")
		case "focus":
			add("focus")
		case "show_menu":
			add("invoke")
		}
	}
	return normalizeActions(out)
}
