package a11y

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSemanticSDK_CompilePrunesLayoutOnlyBranches(t *testing.T) {
	sdk := newSemanticSDKWithHost(nil)
	raw := RawTree{
		WindowID: "win-1",
		Title:    "Finder",
		Mode:     "ax",
		Root: &RawNode{
			Role: "window",
			Name: "Finder",
			Children: []*RawNode{
				{
					Role: "group",
					Name: "Sidebar",
					Children: []*RawNode{
						{
							Token:       "token-open",
							Role:        "button",
							Name:        "Open",
							Interactive: true,
							Actions:     []string{"AXPress"},
						},
					},
				},
			},
		},
	}

	snapshot, err := sdk.Compile(raw, CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if semanticRoleCount(snapshot, "group") != 0 {
		t.Fatalf("semantic group count = %d, want 0", semanticRoleCount(snapshot, "group"))
	}
	openNode := semanticNodeByToken(t, snapshot, "token-open")
	if openNode.Role != "button" {
		t.Fatalf("open node role = %q, want button", openNode.Role)
	}
}

func TestSemanticSDK_CompileFlattensWrappersAndKeepsPathHints(t *testing.T) {
	sdk := newSemanticSDKWithHost(nil)
	raw := RawTree{
		WindowID: "win-1",
		Title:    "Finder",
		Mode:     "ax",
		Root: &RawNode{
			Role: "window",
			Name: "Finder",
			Children: []*RawNode{
				{
					Role: "group",
					Name: "Sidebar",
					Children: []*RawNode{
						{
							Role: "group",
							Name: "Favorites",
							Children: []*RawNode{
								{
									Token:       "token-orca",
									Role:        "list_item",
									Name:        "Orca Team",
									Interactive: true,
									Actions:     []string{"AXPress"},
								},
							},
						},
					},
				},
			},
		},
	}

	snapshot, err := sdk.Compile(raw, CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	node := semanticNodeByToken(t, snapshot, "token-orca")
	if node.Path == "" {
		t.Fatal("node.Path = empty, want ancestry hint")
	}
	if !containsAll(node.Path, "sidebar", "favorites") {
		t.Fatalf("node.Path = %q, want sidebar and favorites ancestry", node.Path)
	}
}

func TestSemanticSDK_CompileMergesInteractiveRowsWithStaticTextLabels(t *testing.T) {
	sdk := newSemanticSDKWithHost(nil)
	raw := RawTree{
		WindowID: "win-1",
		Title:    "Finder",
		Mode:     "ax",
		Root: &RawNode{
			Role: "window",
			Name: "Finder",
			Children: []*RawNode{
				{
					Role: "list",
					Name: "Files",
					Children: []*RawNode{
						{
							Token:       "token-row-test",
							Role:        "row",
							Interactive: true,
							Actions:     []string{"AXPress"},
							Children: []*RawNode{
								{Role: "text", Name: "test.txt"},
							},
						},
					},
				},
			},
		},
	}

	snapshot, err := sdk.Compile(raw, CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	row := semanticNodeByToken(t, snapshot, "token-row-test")
	if row.Name != "test.txt" {
		t.Fatalf("row.Name = %q, want test.txt", row.Name)
	}
	if len(row.Capabilities) == 0 {
		t.Fatal("row.Capabilities = empty, want actionable capabilities")
	}
}

func TestSemanticSDK_CompileKeepsStableIDAcrossWrapperReshapes(t *testing.T) {
	sdk := newSemanticSDKWithHost(nil)

	build := func(withExtraWrapper bool) *Snapshot {
		children := []*RawNode{
			{
				Token:       "token-delete",
				Role:        "button",
				Name:        "Delete",
				Interactive: true,
				Actions:     []string{"AXPress"},
				Bounds:      NormalizedRect{X: 0.78, Y: 0.08, Width: 0.12, Height: 0.06},
			},
		}
		if withExtraWrapper {
			children = []*RawNode{{
				Role:     "group",
				Name:     "Toolbar",
				Children: children,
			}}
		}
		snapshot, err := sdk.Compile(RawTree{
			WindowID: "win-1",
			Title:    "Finder",
			Mode:     "ax",
			Root: &RawNode{
				Role: "window",
				Name: "Finder",
				Children: []*RawNode{
					{
						Role:     "group",
						Name:     "Toolbar",
						Children: children,
					},
				},
			},
		}, CompileOptions{})
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		return snapshot
	}

	base := build(false)
	reshaped := build(true)
	if semanticNodeByToken(t, base, "token-delete").StableID != semanticNodeByToken(t, reshaped, "token-delete").StableID {
		t.Fatalf("StableID changed across harmless wrapper reshape: %q vs %q", semanticNodeByToken(t, base, "token-delete").StableID, semanticNodeByToken(t, reshaped, "token-delete").StableID)
	}
}

func TestSemanticSDK_ResolveSupportsStableIDCapabilityAndAncestorFilters(t *testing.T) {
	sdk := newSemanticSDKWithHost(nil)
	snapshot, err := sdk.Compile(RawTree{
		WindowID: "win-1",
		Title:    "Finder",
		Mode:     "ax",
		Root: &RawNode{
			Role: "window",
			Name: "Finder",
			Children: []*RawNode{
				{
					Role: "group",
					Name: "Toolbar",
					Children: []*RawNode{
						{Token: "token-toolbar-delete", Role: "button", Name: "Delete", Interactive: true, Actions: []string{"AXPress"}},
					},
				},
				{
					Role: "group",
					Name: "File List",
					Children: []*RawNode{
						{Token: "token-row-delete", Role: "button", Name: "Delete", Interactive: true, Actions: []string{"AXPress"}},
					},
				},
			},
		},
	}, CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	toolbarDelete := semanticNodeByToken(t, snapshot, "token-toolbar-delete")
	byID, err := sdk.Resolve(snapshot, Query{ID: toolbarDelete.StableID})
	if err != nil {
		t.Fatalf("Resolve(by id) error = %v", err)
	}
	if len(byID) != 1 || byID[0].StableID != toolbarDelete.StableID {
		t.Fatalf("Resolve(by id) = %#v, want toolbar delete node", byID)
	}

	filtered, err := sdk.Resolve(snapshot, Query{Name: "Delete", Role: "button", Capability: "press", Ancestor: "Toolbar"})
	if err != nil {
		t.Fatalf("Resolve(filtered) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].BackendToken != "token-toolbar-delete" {
		t.Fatalf("Resolve(filtered) = %#v, want toolbar delete node", filtered)
	}
}

func TestSemanticSDK_ResolveUsesUpdatedIndexesAfterSnapshotPatch(t *testing.T) {
	sdk := newSemanticSDKWithHost(nil)
	store := NewSnapshotStore()
	snapshot, err := sdk.Compile(RawTree{
		WindowID: "win-1",
		Title:    "Finder",
		Mode:     "ax",
		Root: &RawNode{
			Role: "window",
			Name: "Finder",
			Children: []*RawNode{
				{Token: "token-row", Role: "row", Name: "old.txt", Interactive: true, Actions: []string{"AXPress"}},
			},
		},
	}, CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	current := store.Swap(snapshot)
	if current == nil {
		t.Fatal("Swap() = nil")
	}

	updated, ok := store.ApplyPatch("win-1", "rename", func(existing *Snapshot) *Snapshot {
		out := existing.Clone()
		for idx := range out.Nodes {
			if out.Nodes[idx].BackendToken == "token-row" {
				out.Nodes[idx].Name = "new.txt"
				break
			}
		}
		return out
	})
	if !ok {
		t.Fatal("ApplyPatch() = false, want true")
	}

	matches, err := sdk.Resolve(updated, Query{Name: "new.txt", Role: "row"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(matches) != 1 || matches[0].BackendToken != "token-row" {
		t.Fatalf("Resolve() = %#v, want renamed row", matches)
	}
}

func TestSemanticSDK_ExpandScrollsUntilTargetAppearsAndDeduplicates(t *testing.T) {
	host := &fakeSemanticHost{
		captures: []RawTree{
			rawTreeWithRows("win-1", "Finder", "token-row-1:file-1.txt", "token-row-2:file-2.txt"),
			rawTreeWithRows("win-1", "Finder", "token-row-2:file-2.txt", "token-row-3:file-3.txt"),
		},
	}
	sdk := newSemanticSDKWithHost(host)
	initial, err := sdk.Compile(host.captures[0], CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	host.captureIdx = 1

	expanded, err := sdk.Expand(context.Background(), initial, Query{Name: "file-3.txt", Role: "row"})
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(host.scrollCalls) != 1 {
		t.Fatalf("scroll calls = %d, want 1", len(host.scrollCalls))
	}
	if semanticTokenCount(expanded, "token-row-2") != 1 {
		t.Fatalf("token-row-2 count = %d, want 1 after dedupe", semanticTokenCount(expanded, "token-row-2"))
	}
	if len(mustResolve(t, sdk, expanded, Query{Name: "file-3.txt", Role: "row"})) != 1 {
		t.Fatal("expanded snapshot does not resolve file-3.txt")
	}
}

func TestSemanticSDK_ExpandStopsAfterRepeatedDuplicatePages(t *testing.T) {
	host := &fakeSemanticHost{
		captures: []RawTree{
			rawTreeWithRows("win-1", "Finder", "token-row-1:file-1.txt", "token-row-2:file-2.txt"),
			rawTreeWithRows("win-1", "Finder", "token-row-1:file-1.txt", "token-row-2:file-2.txt"),
		},
	}
	sdk := newSemanticSDKWithHost(host)
	initial, err := sdk.Compile(host.captures[0], CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	host.captureIdx = 1

	expanded, err := sdk.Expand(context.Background(), initial, Query{Name: "missing.txt", Role: "row"})
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(host.scrollCalls) != 1 {
		t.Fatalf("scroll calls = %d, want stop after first duplicate page", len(host.scrollCalls))
	}
	if len(mustResolve(t, sdk, expanded, Query{Name: "missing.txt", Role: "row"})) != 0 {
		t.Fatal("missing row unexpectedly resolved after duplicate expansion")
	}
}

func TestSemanticSDK_ExecuteSetValueRefreshesSnapshotState(t *testing.T) {
	host := &fakeSemanticHost{
		captures: []RawTree{
			rawTreeWithTextField("win-1", "Messages", "token-message", "Message", ""),
			rawTreeWithTextField("win-1", "Messages", "token-message", "Message", "hello world"),
		},
	}
	sdk := newSemanticSDKWithHost(host)
	snapshot, err := sdk.Compile(host.captures[0], CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	host.captureIdx = 1

	result, err := sdk.Execute(context.Background(), snapshot, Action{
		Op:    "set_value",
		Query: Query{Name: "Message", Role: "text_field"},
		Value: "hello world",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("result.Success = false, want true: %#v", result)
	}
	if result.AppliedAction != "set_value" {
		t.Fatalf("AppliedAction = %q, want set_value", result.AppliedAction)
	}
	if len(host.executed) != 1 || host.executed[0].action.Op != "set_value" {
		t.Fatalf("executed actions = %#v, want one set_value action", host.executed)
	}
	postNode := semanticNodeByToken(t, result.Snapshot, "token-message")
	if postNode.Value != "hello world" {
		t.Fatalf("post-execute node value = %q, want hello world", postNode.Value)
	}
}

func TestSemanticSDK_ExecuteFailsClosedOnAmbiguousOrUnsupportedTargets(t *testing.T) {
	host := &fakeSemanticHost{
		captures: []RawTree{
			{
				WindowID: "win-1",
				Title:    "Finder",
				Mode:     "ax",
				Root: &RawNode{
					Role: "window",
					Name: "Finder",
					Children: []*RawNode{
						{Token: "token-delete-a", Role: "button", Name: "Delete", Interactive: true, Actions: []string{"AXPress"}},
						{Token: "token-delete-b", Role: "button", Name: "Delete", Interactive: true, Actions: []string{"AXPress"}},
						{Token: "token-text", Role: "text", Name: "Status"},
					},
				},
			},
		},
	}
	sdk := newSemanticSDKWithHost(host)
	snapshot, err := sdk.Compile(host.captures[0], CompileOptions{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if _, err := sdk.Execute(context.Background(), snapshot, Action{
		Op:    "press",
		Query: Query{Name: "Delete", Role: "button"},
	}); err == nil {
		t.Fatal("Execute(ambiguous) error = nil, want ambiguity failure")
	}

	textNode := semanticNodeByToken(t, snapshot, "token-text")
	if _, err := sdk.Execute(context.Background(), snapshot, Action{
		Op:    "press",
		Query: Query{ID: textNode.StableID},
	}); err == nil {
		t.Fatal("Execute(unsupported) error = nil, want unsupported action failure")
	}

	if len(host.executed) != 0 {
		t.Fatalf("executed actions = %#v, want none when executor fails closed", host.executed)
	}
}

type fakeSemanticHost struct {
	captures    []RawTree
	captureIdx  int
	scrollCalls []scrollCall
	executed    []executedAction
	executeErr  error
}

type scrollCall struct {
	windowID  string
	direction string
	lines     int
}

type executedAction struct {
	windowID string
	node     SemanticNode
	action   Action
}

func (f *fakeSemanticHost) HostOS() string {
	return "darwin"
}

func (f *fakeSemanticHost) Capture(context.Context, CaptureScope) (RawTree, error) {
	if len(f.captures) == 0 {
		return RawTree{}, errors.New("no captures queued")
	}
	idx := f.captureIdx
	if idx >= len(f.captures) {
		idx = len(f.captures) - 1
	}
	out := f.captures[idx]
	if f.captureIdx < len(f.captures)-1 {
		f.captureIdx++
	}
	return out, nil
}

func (f *fakeSemanticHost) Execute(_ context.Context, windowID string, node SemanticNode, action Action) (ActionResult, error) {
	if f.executeErr != nil {
		return ActionResult{}, f.executeErr
	}
	f.executed = append(f.executed, executedAction{
		windowID: windowID,
		node:     node,
		action:   action,
	})
	return ActionResult{
		HostOS:        f.HostOS(),
		WindowID:      windowID,
		ExecutionMode: "semantic",
		Message:       "fake execute completed",
	}, nil
}

func (f *fakeSemanticHost) Scroll(_ context.Context, windowID string, direction string, lines int) (ActionResult, error) {
	f.scrollCalls = append(f.scrollCalls, scrollCall{windowID: windowID, direction: direction, lines: lines})
	return ActionResult{HostOS: f.HostOS(), WindowID: windowID, Message: "scroll queued"}, nil
}

func semanticNodeByToken(t *testing.T, snapshot *Snapshot, token string) SemanticNode {
	t.Helper()
	for _, node := range snapshot.Nodes {
		if node.BackendToken == token {
			return node
		}
	}
	t.Fatalf("semantic node with token %q not found in %#v", token, snapshot.Nodes)
	return SemanticNode{}
}

func semanticRoleCount(snapshot *Snapshot, role string) int {
	count := 0
	for _, node := range snapshot.Nodes {
		if node.Role == role {
			count++
		}
	}
	return count
}

func semanticTokenCount(snapshot *Snapshot, token string) int {
	count := 0
	for _, node := range snapshot.Nodes {
		if node.BackendToken == token {
			count++
		}
	}
	return count
}

func mustResolve(t *testing.T, sdk *SemanticSDK, snapshot *Snapshot, query Query) []SemanticNode {
	t.Helper()
	nodes, err := sdk.Resolve(snapshot, query)
	if err != nil {
		t.Fatalf("Resolve(%#v) error = %v", query, err)
	}
	return nodes
}

func rawTreeWithRows(windowID string, title string, specs ...string) RawTree {
	children := make([]*RawNode, 0, len(specs))
	for _, spec := range specs {
		token, name := splitSpec(spec)
		children = append(children, &RawNode{
			Token:       token,
			Role:        "row",
			Name:        name,
			Interactive: true,
			Actions:     []string{"AXPress"},
		})
	}
	return RawTree{
		WindowID: windowID,
		Title:    title,
		Mode:     "ax",
		Root: &RawNode{
			Role:     "window",
			Name:     title,
			Children: children,
		},
	}
}

func rawTreeWithTextField(windowID string, title string, token string, name string, value string) RawTree {
	return RawTree{
		WindowID: windowID,
		Title:    title,
		Mode:     "ax",
		Root: &RawNode{
			Role: "window",
			Name: title,
			Children: []*RawNode{
				{
					Token:         token,
					Role:          "text_field",
					Name:          name,
					Value:         value,
					Interactive:   true,
					ValueSettable: true,
					Actions:       []string{"AXSetValue"},
				},
			},
		},
	}
}

func splitSpec(spec string) (string, string) {
	for idx := 0; idx < len(spec); idx++ {
		if spec[idx] == ':' {
			return spec[:idx], spec[idx+1:]
		}
	}
	return spec, spec
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !containsFold(value, part) {
			return false
		}
	}
	return true
}

func containsFold(value string, part string) bool {
	return strings.Contains(normalizeSnapshotMatchValue(value), normalizeSnapshotMatchValue(part))
}
