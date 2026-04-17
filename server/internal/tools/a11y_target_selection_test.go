package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestParseA11ySnapshotEntries_ParsesQuotedLabels(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [button] \"Send\"\n  @2 [text_field] \"He said \\\"Hi\\\"\"\n@3 [document]")
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[1].Ref != 2 || entries[1].Role != "text_field" || entries[1].Label != `He said "Hi"` {
		t.Fatalf("entries[1] = %#v, want parsed quoted label", entries[1])
	}
	if entries[2].Label != "" {
		t.Fatalf("entries[2].Label = %q, want empty", entries[2].Label)
	}
}

func TestResolveA11yTargetRef_PrefersLikelyComposerInputOverSearchField(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [search_field] \"Search\"\n@2 [document]\n@3 [button] \"Send\"")

	ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Role: "input"})
	if err != nil {
		t.Fatalf("resolveA11yTargetRef() error = %v", err)
	}
	if ref != 2 {
		t.Fatalf("ref = %d, want 2 for likely composer input", ref)
	}
}

func TestResolveA11yTargetRef_ExplicitSearchRoleMatchesSearchFieldAliases(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [search_field] \"Search\"\n@2 [button] \"Send\"")

	for _, role := range []string{"search", "searchbox"} {
		ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Role: role})
		if err != nil {
			t.Fatalf("resolveA11yTargetRef(%q) error = %v", role, err)
		}
		if ref != 1 {
			t.Fatalf("resolveA11yTargetRef(%q) ref = %d, want 1 for search field alias", role, ref)
		}
	}
}

func TestResolveA11yTargetRef_PrefersInputNearestLikelySubmitControl(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [document]\n@2 [document]\n@3 [button] \"Send\"")

	ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Role: "input"})
	if err != nil {
		t.Fatalf("resolveA11yTargetRef() error = %v", err)
	}
	if ref != 2 {
		t.Fatalf("ref = %d, want 2 for input nearest likely submit control", ref)
	}
}

func TestResolveA11yTargetRef_PrefersExactNameBeforeFuzzyMatches(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [button] \"Send\"\n@2 [button] \"Send Message\"")

	ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Name: "Send", Role: "button"})
	if err != nil {
		t.Fatalf("resolveA11yTargetRef() error = %v", err)
	}
	if ref != 1 {
		t.Fatalf("ref = %d, want 1 for exact name match", ref)
	}
}

func TestResolveA11yTargetRef_UsesUniqueBestFuzzyNameMatch(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [button] \"Open Network Settings\"\n@2 [button] \"Open Bluetooth\"\n@3 [button] \"Close\"")

	ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Name: "Network", Role: "button"})
	if err != nil {
		t.Fatalf("resolveA11yTargetRef() error = %v", err)
	}
	if ref != 1 {
		t.Fatalf("ref = %d, want 1 for unique fuzzy name match", ref)
	}
}

func TestResolveA11yTargetRef_UsesBestAliasFuzzyNameMatch(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [list_item] \"Orca Team\"\n@2 [list_item] \"Orca Ops\"\n@3 [list_item] \"Project Orca Team\"")

	ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Name: "Project Team Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("resolveA11yTargetRef() error = %v", err)
	}
	if ref != 3 {
		t.Fatalf("ref = %d, want 3 for best fuzzy alias match", ref)
	}
}

func TestResolveA11yTargetRef_LeavesFuzzyNameTiesAmbiguous(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [button] \"Open Network\"\n@2 [button] \"Network Details\"")

	_, err := resolveA11yTargetRef(entries, a11yTargetSelector{Name: "Network", Role: "button"})
	if err == nil {
		t.Fatal("resolveA11yTargetRef() error = nil, want ambiguous_target")
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "ambiguous_target" {
		t.Fatalf("code = %q, want ambiguous_target", runtimeErr.Code)
	}
}

func TestResolveA11yTargetRef_PrefersConversationRolePriorityBeforeAmbiguity(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [button] \"Orca\"\n@2 [list_item] \"Orca\"")

	ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("resolveA11yTargetRef() error = %v", err)
	}
	if ref != 2 {
		t.Fatalf("ref = %d, want 2 for preferred conversation role", ref)
	}
}

func TestResolveA11yTargetRef_ConversationRoleDoesNotTreatGenericButtonAsConversation(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [button] \"Orca\"")

	_, err := resolveA11yTargetRef(entries, a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err == nil {
		t.Fatal("resolveA11yTargetRef() error = nil, want target_not_found")
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "target_not_found" {
		t.Fatalf("code = %q, want target_not_found", runtimeErr.Code)
	}
}

func TestA11yConversationSearchPlans_PrefersPlatformShortcuts(t *testing.T) {
	darwinPlans := a11yConversationSearchPlans("darwin")
	if len(darwinPlans) != 2 {
		t.Fatalf("len(darwinPlans) = %d, want 2", len(darwinPlans))
	}
	if got := darwinPlans[0].Open; len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 2 || got[0][0] != "command" || got[0][1] != "f" || got[1][0] != "command" || got[1][1] != "f" {
		t.Fatalf("darwinPlans[0].Open = %#v, want [[command f] [command f]]", got)
	}
	if got := darwinPlans[1].Open; len(got) != 1 || len(got[0]) != 2 || got[0][0] != "command" || got[0][1] != "k" {
		t.Fatalf("darwinPlans[1].Open = %#v, want [[command k]]", got)
	}

	windowsPlans := a11yConversationSearchPlans("windows")
	if len(windowsPlans) != 2 {
		t.Fatalf("len(windowsPlans) = %d, want 2", len(windowsPlans))
	}
	if got := windowsPlans[0].Open; len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 2 || got[0][0] != "ctrl" || got[0][1] != "f" || got[1][0] != "ctrl" || got[1][1] != "f" {
		t.Fatalf("windowsPlans[0].Open = %#v, want [[ctrl f] [ctrl f]]", got)
	}
	if got := windowsPlans[1].Open; len(got) != 1 || len(got[0]) != 2 || got[0][0] != "ctrl" || got[0][1] != "k" {
		t.Fatalf("windowsPlans[1].Open = %#v, want [[ctrl k]]", got)
	}
}

func TestA11yAllowsConversationSearchFallback_UsesRegisteredProfilesAndTargetNotFound(t *testing.T) {
	notFoundErr := a11yruntime.NewError("target_not_found", "missing", nil)
	if !a11yAllowsConversationSearchFallback(map[string]interface{}{"app_name": "Feishu,飞书,Lark"}, notFoundErr) {
		t.Fatal("expected Feishu aliases to enable conversation search fallback")
	}
	if !a11yAllowsConversationSearchFallback(map[string]interface{}{"app_name": "Slack"}, notFoundErr) {
		t.Fatal("expected Slack to enable conversation search fallback")
	}
	if a11yAllowsConversationSearchFallback(map[string]interface{}{"app_name": "Messages"}, notFoundErr) {
		t.Fatal("expected unknown app aliases to skip conversation search fallback")
	}
	if a11yAllowsConversationSearchFallback(map[string]interface{}{"app_name": "Feishu"}, a11yruntime.NewError("ambiguous_target", "ambiguous", nil)) {
		t.Fatal("expected non-target_not_found errors to skip conversation search fallback")
	}
}

func TestExecuteA11yConversationSearchPlan_TypesQueryViaSearchFieldAction(t *testing.T) {
	backend := &a11yCompatBackend{
		hostOS: "darwin",
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"\n@2 [list_item] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-team-ops",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	windowID, err := tool.executeA11yConversationSearchPlan(
		context.Background(),
		backend,
		"win-feishu",
		a11yTargetSelector{Name: "Team Ops", Role: "conversation"},
		0,
		a11yConversationSearchPlan{Open: [][]string{{"command", "k"}}},
	)
	if err != nil {
		t.Fatalf("executeA11yConversationSearchPlan() error = %v", err)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
	if len(backend.keyHistory) != 3 {
		t.Fatalf("keyHistory = %#v, want open + clear-only key steps", backend.keyHistory)
	}
	if got := backend.keyHistory[2]; len(got) != 1 || got[0] != "delete" {
		t.Fatalf("keyHistory[2] = %#v, want [delete]", got)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "click" {
		t.Fatalf("actTypeHistory = %#v, want [type click]", backend.actTypeHistory)
	}
	if len(backend.actRefHistory) != 2 || backend.actRefHistory[0] != 1 || backend.actRefHistory[1] != 2 {
		t.Fatalf("actRefHistory = %#v, want [1 2]", backend.actRefHistory)
	}
	if backend.lastActValue != "" {
		t.Fatalf("lastActValue = %q, want empty after final click activation", backend.lastActValue)
	}
}

func TestResolveA11yConversationSearchResultTarget_UsesFullSnapshotLabelFallbackWhenInteractiveMisses(t *testing.T) {
	backend := &a11yCompatBackend{
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [search_field] \"Orca\"\n@2 [group] \"Results\"",
			RefMap: map[int]string{
				1: "token-search",
				2: "token-results",
			},
		},
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [search_field] \"Orca\"\n@2 [static_text] \"Orca\"\n@3 [row]",
			RefMap: map[int]string{
				1: "token-search",
				2: "token-label",
				3: "token-orca-result",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	ref, refMap, windowID, err := tool.resolveA11yConversationSearchResultTarget(
		context.Background(),
		backend,
		"win-feishu",
		a11yTargetSelector{Name: "Orca", Role: "conversation"},
		1,
	)
	if err != nil {
		t.Fatalf("resolveA11yConversationSearchResultTarget() error = %v", err)
	}
	if ref != 3 {
		t.Fatalf("ref = %d, want 3 from full snapshot label fallback", ref)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
	if got := refMap[3]; got != "token-orca-result" {
		t.Fatalf("refMap[3] = %q, want token-orca-result", got)
	}
}

func TestResolveA11yTargetRef_PrefersControlRolePriorityBeforeAmbiguity(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [row] \"Open Network\"\n@2 [button] \"Open Network\"")

	ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Name: "Open Network", Role: "control"})
	if err != nil {
		t.Fatalf("resolveA11yTargetRef() error = %v", err)
	}
	if ref != 2 {
		t.Fatalf("ref = %d, want 2 for preferred control role", ref)
	}
}

func TestResolveA11yTargetRef_PrefersSettingRolePriorityBeforeAmbiguity(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [menu_item] \"Enable notifications\"\n@2 [switch] \"Enable notifications\"")

	ref, err := resolveA11yTargetRef(entries, a11yTargetSelector{Name: "Enable notifications", Role: "setting"})
	if err != nil {
		t.Fatalf("resolveA11yTargetRef() error = %v", err)
	}
	if ref != 2 {
		t.Fatalf("ref = %d, want 2 for preferred setting role", ref)
	}
}

func TestResolveA11yTargetRef_LeavesAmbiguousInputsUnresolvedWhenProximityStillTies(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [document]\n@2 [button] \"Send\"\n@3 [document]")

	_, err := resolveA11yTargetRef(entries, a11yTargetSelector{Role: "input"})
	if err == nil {
		t.Fatal("resolveA11yTargetRef() error = nil, want ambiguous_target")
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "ambiguous_target" {
		t.Fatalf("code = %q, want ambiguous_target", runtimeErr.Code)
	}
}

func TestA11yToolExecute_ActAutoResolvesUniqueTargetSelector(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-code", Title: "Code", AppName: "Code"},
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"\n@2 [text_field] \"Message\"\n@3 [list_item] \"Orca\"",
			RefMap: map[int]string{
				1: "token-send",
				2: "token-message",
				3: "token-orca",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_name": "Message",
			"target_role": "text field",
			"value":       "你好，Orca",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.interactiveCalls != 1 {
		t.Fatalf("interactiveCalls = %d, want 1", backend.interactiveCalls)
	}
	if backend.lastActWindowID != "win-feishu" {
		t.Fatalf("lastActWindowID = %q, want win-feishu", backend.lastActWindowID)
	}
	if backend.lastActRef != 2 {
		t.Fatalf("lastActRef = %d, want 2", backend.lastActRef)
	}
	if backend.lastActValue != "你好，Orca" {
		t.Fatalf("lastActValue = %q, want 你好，Orca", backend.lastActValue)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["execution_mode"] != "semantic" {
		t.Fatalf("execution_mode = %v, want semantic", out["execution_mode"])
	}
}

func TestA11yToolExecute_ActIntentClickUsesControlAliasAndDefaultsToClick(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [button] \"Open Network\"",
			RefMap: map[int]string{
				1: "token-open-network",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Settings",
		"params": map[string]interface{}{
			"intent":  "click",
			"control": "Open Network",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActType != "click" {
		t.Fatalf("lastActType = %q, want click", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActControlAliasUsesUniqueBestFuzzyNameMatch(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [button] \"Open Network Settings\"\n@2 [button] \"Open Bluetooth\"",
			RefMap: map[int]string{
				1: "token-open-network",
				2: "token-open-bluetooth",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Settings",
		"params": map[string]interface{}{
			"control": "Network",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActType != "click" {
		t.Fatalf("lastActType = %q, want click", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActionSelectAliasPrefersConversationRolePriority(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [button] \"Orca\"\n@2 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-orca-button",
					2: "token-orca-list-item",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"window_title": "Feishu",
		"conversation": "Orca",
	}); err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if backend.lastActRef != 2 {
		t.Fatalf("lastActRef = %d, want 2 for preferred conversation role", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActionSelectReusesFocusedWindowWhenAppAliasDisappearsAfterFocus(t *testing.T) {
	backend := &a11yCompatBackend{
		windowsResults: [][]a11yruntime.WindowInfo{
			{
				{ID: "win-feishu", Title: "Lark - Orca", AppName: "Lark", Focused: false},
			},
			{
				{ID: "win-feishu", Title: "Orca", AppName: "", Focused: true},
			},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Orca",
			Tree:     "@1 [list_item] \"Orca\"",
			RefMap: map[int]string{
				1: "token-orca-conversation",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "focus",
		"app_name": "Feishu,飞书,Lark",
	}); err != nil {
		t.Fatalf("focus Execute() error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
	}); err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if backend.lastFocusWindowID != "win-feishu" {
		t.Fatalf("lastFocusWindowID = %q, want win-feishu", backend.lastFocusWindowID)
	}
	if backend.lastActWindowID != "win-feishu" {
		t.Fatalf("lastActWindowID = %q, want win-feishu", backend.lastActWindowID)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
	if backend.listWindowsCalls != 2 {
		t.Fatalf("listWindowsCalls = %d, want 2", backend.listWindowsCalls)
	}
}

func TestA11yToolExecute_FocusActivatesAppWhenWindowIsNotYetVisible(t *testing.T) {
	backend := &a11yCompatBackend{
		windowsResults: [][]a11yruntime.WindowInfo{
			{},
			{
				{ID: "win-feishu", Title: "Lark - Orca", AppName: "Lark", Focused: true},
			},
		},
		focusResultWindowID: "win-feishu",
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "focus",
		"app_name": "Feishu,飞书,Lark",
	})
	if err != nil {
		t.Fatalf("focus Execute() error = %v", err)
	}
	if len(backend.activateAppCalls) == 0 {
		t.Fatal("activateAppCalls = 0, want activation fallback")
	}
	if backend.lastFocusWindowID != "win-feishu" {
		t.Fatalf("lastFocusWindowID = %q, want win-feishu", backend.lastFocusWindowID)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["window_id"] != "win-feishu" {
		t.Fatalf("window_id = %v, want win-feishu", out["window_id"])
	}
}

func TestA11yToolExecute_ActionClickAliasFallsBackToFullSnapshotLabelProximity(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [button]",
			RefMap: map[int]string{
				1: "token-open-network",
			},
		},
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree: "[window] \"Settings\"\n" +
				"  [group] \"Network\"\n" +
				"    [static_text] \"Open Network\"\n" +
				"    @1 [button]",
			RefMap: map[int]string{
				1: "token-open-network",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "click",
		"window_title": "Settings",
		"control":      "Open Network",
	}); err != nil {
		t.Fatalf("click Execute() error = %v", err)
	}
	if backend.lastActType != "click" {
		t.Fatalf("lastActType = %q, want click", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1 from full snapshot label proximity", backend.lastActRef)
	}
	if backend.interactiveCalls != 1 {
		t.Fatalf("interactiveCalls = %d, want 1", backend.interactiveCalls)
	}
	if len(backend.snapshotWindowHistory) != 2 {
		t.Fatalf("snapshotWindowHistory = %#v, want interactive + full snapshot", backend.snapshotWindowHistory)
	}
}

func TestA11yToolExecute_ActIntentToggleUsesSettingAliasAndDefaultsToToggle(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [switch] \"Enable notifications\"",
			RefMap: map[int]string{
				1: "token-enable-notifications",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Settings",
		"params": map[string]interface{}{
			"intent":  "toggle",
			"setting": "Enable notifications",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActType != "toggle" {
		t.Fatalf("lastActType = %q, want toggle", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActIntentMessageSelectsConversationThenTypesAndSubmits(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"intent":       "message",
			"conversation": "Orca",
			"value":        "你好，Orca",
			"submit":       true,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 3 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" || backend.actTypeHistory[2] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [click type submit]", backend.actTypeHistory)
	}
	if len(backend.actRefHistory) != 3 || backend.actRefHistory[0] != 1 || backend.actRefHistory[1] != 1 || backend.actRefHistory[2] != 2 {
		t.Fatalf("actRefHistory = %#v, want [1 1 2]", backend.actRefHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActInfersClickFromControlAliasWithoutIntent(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [button] \"Open Network\"",
			RefMap: map[int]string{
				1: "token-open-network",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Settings",
		"params": map[string]interface{}{
			"control": "Open Network",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActType != "click" {
		t.Fatalf("lastActType = %q, want click", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActInfersToggleFromSettingAliasWithoutIntent(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [switch] \"Enable notifications\"",
			RefMap: map[int]string{
				1: "token-enable-notifications",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Settings",
		"params": map[string]interface{}{
			"setting": "Enable notifications",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActType != "toggle" {
		t.Fatalf("lastActType = %q, want toggle", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActionToggleAliasFallsBackToFullSnapshotLabelProximity(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [switch]",
			RefMap: map[int]string{
				1: "token-enable-notifications",
			},
		},
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree: "[window] \"Settings\"\n" +
				"  [group] \"Notifications\"\n" +
				"    [static_text] \"Enable notifications\"\n" +
				"    @1 [switch]",
			RefMap: map[int]string{
				1: "token-enable-notifications",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "toggle",
		"window_title": "Settings",
		"setting":      "Enable notifications",
	}); err != nil {
		t.Fatalf("toggle Execute() error = %v", err)
	}
	if backend.lastActType != "toggle" {
		t.Fatalf("lastActType = %q, want toggle", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1 from full snapshot label proximity", backend.lastActRef)
	}
	if backend.interactiveCalls != 1 {
		t.Fatalf("interactiveCalls = %d, want 1", backend.interactiveCalls)
	}
	if len(backend.snapshotWindowHistory) != 2 {
		t.Fatalf("snapshotWindowHistory = %#v, want interactive + full snapshot", backend.snapshotWindowHistory)
	}
}

func TestA11yToolExecute_ActInfersToggleFallsBackToClickWhenUnsupported(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [switch] \"Enable notifications\"",
			RefMap: map[int]string{
				1: "token-enable-notifications",
			},
		},
		actErrorsByType: map[string]error{
			"toggle": a11yruntime.NewError("unsupported_action", "element does not support toggle", map[string]interface{}{"act_type": "toggle"}),
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Settings",
		"params": map[string]interface{}{
			"setting": "Enable notifications",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "toggle" || backend.actTypeHistory[1] != "click" {
		t.Fatalf("actTypeHistory = %#v, want [toggle click]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
}

func TestA11yToolExecute_ActionToggleAliasFullSnapshotLabelProximityFailsClosedOnAmbiguousPairs(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [switch]\n@2 [switch]",
			RefMap: map[int]string{
				1: "token-enable-notifications-1",
				2: "token-enable-notifications-2",
			},
		},
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree: "[window] \"Settings\"\n" +
				"  [group] \"Notifications A\"\n" +
				"    [static_text] \"Enable notifications\"\n" +
				"    @1 [switch]\n" +
				"  [group] \"Notifications B\"\n" +
				"    [static_text] \"Enable notifications\"\n" +
				"    @2 [switch]",
			RefMap: map[int]string{
				1: "token-enable-notifications-1",
				2: "token-enable-notifications-2",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "toggle",
		"window_title": "Settings",
		"setting":      "Enable notifications",
	})
	if err != nil {
		t.Fatalf("toggle Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "ambiguous_target" {
		t.Fatalf("error_code = %v, want ambiguous_target", out["error_code"])
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 on ambiguous full snapshot fallback", backend.actCalls)
	}
}

func TestA11yToolExecute_ActInfersMessageFromConversationAliasAndDefaultsToSubmit(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"conversation": "Orca",
			"value":        "你好，Orca",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 3 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" || backend.actTypeHistory[2] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [click type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActIntentMessageHonorsExplicitSubmitFalse(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"intent":       "message",
			"conversation": "Orca",
			"value":        "你好，Orca",
			"submit":       false,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" {
		t.Fatalf("actTypeHistory = %#v, want [click type]", backend.actTypeHistory)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no shortcut search when direct conversation resolve succeeds", backend.keyHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
}

func TestA11yToolExecute_ActionClickAliasUsesTopLevelControl(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [button] \"Open Network\"",
			RefMap: map[int]string{
				1: "token-open-network",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "click",
		"window_title": "Settings",
		"control":      "Open Network",
	}); err != nil {
		t.Fatalf("click Execute() error = %v", err)
	}
	if backend.lastActType != "click" {
		t.Fatalf("lastActType = %q, want click", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActionToggleAliasUsesTopLevelSetting(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [switch] \"Enable notifications\"",
			RefMap: map[int]string{
				1: "token-enable-notifications",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "toggle",
		"window_title": "Settings",
		"setting":      "Enable notifications",
	}); err != nil {
		t.Fatalf("toggle Execute() error = %v", err)
	}
	if backend.lastActType != "toggle" {
		t.Fatalf("lastActType = %q, want toggle", backend.lastActType)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActionToggleAliasFallsBackToClickWhenToggleUnsupported(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-settings", Title: "Settings", AppName: "Settings"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-settings",
			Title:    "Settings",
			Tree:     "@1 [switch] \"Enable notifications\"",
			RefMap: map[int]string{
				1: "token-enable-notifications",
			},
		},
		actErrorsByType: map[string]error{
			"toggle": a11yruntime.NewError("unsupported_action", "element does not support toggle", map[string]interface{}{"act_type": "toggle"}),
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "toggle",
		"window_title": "Settings",
		"setting":      "Enable notifications",
	})
	if err != nil {
		t.Fatalf("toggle Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "toggle" || backend.actTypeHistory[1] != "click" {
		t.Fatalf("actTypeHistory = %#v, want [toggle click]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
}

func TestA11yToolExecute_ActionMessageAliasUsesTopLevelConversationAndDefaultsToSubmit(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "你好，Orca",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 3 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" || backend.actTypeHistory[2] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [click type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionMessageAliasStopsWhenConversationSearchRemainsFocused(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-orca-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-orca-conversation",
				},
			},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [search_field] \"Orca\"\n@2 [list_item] \"Orca\"",
			RefMap: map[int]string{
				1: "token-search",
				2: "token-orca-conversation",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "啊啊，Orca！啊啊Blueaaa.aa",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 1 || backend.actTypeHistory[0] != "click" {
		t.Fatalf("actTypeHistory = %#v, want [click]", backend.actTypeHistory)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no shortcut search when direct conversation resolve succeeds", backend.keyHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error"] != "conversation switch could not be confirmed" {
		t.Fatalf("error = %v, want conversation switch could not be confirmed", out["error"])
	}
	if out["error_code"] != "confirmation_failed" {
		t.Fatalf("error_code = %v, want confirmation_failed", out["error_code"])
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
	if out["confirmation"] != "composer_not_ready" {
		t.Fatalf("confirmation = %v, want composer_not_ready", out["confirmation"])
	}
}

func TestA11yToolExecute_ActionMessageAliasStopsWhenConversationSearchStillContainsTarget(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-editor",
					3: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "你好，Orca",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 1 || backend.actTypeHistory[0] != "click" {
		t.Fatalf("actTypeHistory = %#v, want [click]", backend.actTypeHistory)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no shortcut search when direct conversation resolve succeeds", backend.keyHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error"] != "conversation switch could not be confirmed" {
		t.Fatalf("error = %v, want conversation switch could not be confirmed", out["error"])
	}
	if out["error_code"] != "confirmation_failed" {
		t.Fatalf("error_code = %v, want confirmation_failed", out["error_code"])
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
	if out["confirmation"] != "composer_not_ready" {
		t.Fatalf("confirmation = %v, want composer_not_ready", out["confirmation"])
	}
}

func TestA11yToolExecute_ActionMessageAliasDoesNotSendWhenConversationOnlyMatchesButton(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-orca-button",
				2: "token-editor",
				3: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "你好，Orca",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no click/type/submit when conversation is not found", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "fallback_exhausted" {
		t.Fatalf("error_code = %v, want fallback_exhausted", out["error_code"])
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
	if out["fallback_stage"] != "visual_confirmation" {
		t.Fatalf("fallback_stage = %v, want visual_confirmation", out["fallback_stage"])
	}
	if out["original_error_code"] != "target_not_found" {
		t.Fatalf("original_error_code = %v, want target_not_found", out["original_error_code"])
	}
}

func TestA11yToolExecute_ActionMessageAliasFallsBackToFeishuShortcutSearchWhenDirectConversationMatchIsMissing(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Echo\"\n@2 [list_item] \"Echo\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-echo-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Echo",
		"value":        "你好，Echo",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 4 {
		t.Fatalf("keyHistory = %#v, want structured search open + clear-only key steps before search-field typing", backend.keyHistory)
	}
	if want := []string{"command", "f"}; len(backend.keyHistory[0]) != len(want) || backend.keyHistory[0][0] != want[0] || backend.keyHistory[0][1] != want[1] {
		t.Fatalf("keyHistory[0] = %#v, want %v", backend.keyHistory[0], want)
	}
	if want := []string{"command", "f"}; len(backend.keyHistory[1]) != len(want) || backend.keyHistory[1][0] != want[0] || backend.keyHistory[1][1] != want[1] {
		t.Fatalf("keyHistory[1] = %#v, want %v", backend.keyHistory[1], want)
	}
	if want := []string{"command", "a"}; len(backend.keyHistory[2]) != len(want) || backend.keyHistory[2][0] != want[0] || backend.keyHistory[2][1] != want[1] {
		t.Fatalf("keyHistory[2] = %#v, want %v", backend.keyHistory[2], want)
	}
	if want := []string{"delete"}; len(backend.keyHistory[3]) != len(want) || backend.keyHistory[3][0] != want[0] {
		t.Fatalf("keyHistory[3] = %#v, want %v", backend.keyHistory[3], want)
	}
	if len(backend.actTypeHistory) != 4 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "click" || backend.actTypeHistory[2] != "type" || backend.actTypeHistory[3] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type click type submit]", backend.actTypeHistory)
	}
	if len(backend.actRefHistory) != 4 || backend.actRefHistory[0] != 1 || backend.actRefHistory[1] != 2 || backend.actRefHistory[2] != 1 || backend.actRefHistory[3] != 2 {
		t.Fatalf("actRefHistory = %#v, want [1 2 1 2]", backend.actRefHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionMessageAliasShortcutSearchWaitsForDelayedConversationResult(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Echo\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Echo\"\n@2 [list_item] \"Echo\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-echo-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Echo",
		"value":        "你好，Echo",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 4 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "click" || backend.actTypeHistory[2] != "type" || backend.actTypeHistory[3] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type click type submit]", backend.actTypeHistory)
	}
	if len(backend.actRefHistory) != 4 || backend.actRefHistory[0] != 1 || backend.actRefHistory[1] != 2 || backend.actRefHistory[2] != 1 || backend.actRefHistory[3] != 2 {
		t.Fatalf("actRefHistory = %#v, want [1 2 1 2]", backend.actRefHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionMessageAliasShortcutSearchAcceptsButtonConversationResultAfterQuery(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Echo\"\n@2 [button] \"Echo\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-echo-conversation-button",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Echo",
		"value":        "你好，Echo",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 4 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "click" || backend.actTypeHistory[2] != "type" || backend.actTypeHistory[3] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type click type submit]", backend.actTypeHistory)
	}
	if len(backend.actRefHistory) != 4 || backend.actRefHistory[0] != 1 || backend.actRefHistory[1] != 2 || backend.actRefHistory[2] != 1 || backend.actRefHistory[3] != 2 {
		t.Fatalf("actRefHistory = %#v, want [1 2 1 2]", backend.actRefHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionMessageAliasFailsClosedWhenShortcutSearchKeyInjectionHangs(t *testing.T) {
	prevTimeout := a11yHostInputActionTimeout
	a11yHostInputActionTimeout = 10 * time.Millisecond
	defer func() { a11yHostInputActionTimeout = prevTimeout }()

	blockCh := make(chan struct{})
	defer close(blockCh)

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
		},
		keyBlockCh: blockCh,
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
		"value":        "你好，Orca。",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "backend_timeout" {
		t.Fatalf("error_code = %v, want backend_timeout", out["error_code"])
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
}

func TestA11yToolExecute_ActionMessageAliasSlackQuickSwitcherFailsClosedAfterSingleProfilePlan(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-slack", Title: "Slack", AppName: "Slack"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-slack",
				Title:    "Slack",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-slack",
				Title:    "Slack",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-slack",
				Title:    "Slack",
				Tree:     "@1 [search_field] \"Echo\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Slack",
		"conversation": "Echo",
		"value":        "你好，Echo",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 3 {
		t.Fatalf("keyHistory = %#v, want only one quick-switcher plan", backend.keyHistory)
	}
	if want := []string{"command", "k"}; len(backend.keyHistory[0]) != len(want) || backend.keyHistory[0][0] != want[0] || backend.keyHistory[0][1] != want[1] {
		t.Fatalf("keyHistory[0] = %#v, want %v", backend.keyHistory[0], want)
	}
	if len(backend.actTypeHistory) != 1 || backend.actTypeHistory[0] != "type" {
		t.Fatalf("actTypeHistory = %#v, want only search typing", backend.actTypeHistory)
	}
	if backend.lastGroundingScreenshotWindow != "" || backend.lastScreenshotWindow != "" {
		t.Fatalf("unexpected visual fallback screenshots: grounding=%q screenshot=%q", backend.lastGroundingScreenshotWindow, backend.lastScreenshotWindow)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "fallback_exhausted" {
		t.Fatalf("error_code = %v, want fallback_exhausted", out["error_code"])
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
	if out["fallback_stage"] != "keyboard_search" {
		t.Fatalf("fallback_stage = %v, want keyboard_search", out["fallback_stage"])
	}
}

func TestA11yToolExecute_ActionMessageAliasFallsBackToSlackQuickSwitcherWhenDirectConversationMatchIsMissing(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-slack", Title: "Slack", AppName: "Slack"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-slack",
				Title:    "Slack",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-slack",
				Title:    "Slack",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-slack",
				Title:    "Slack",
				Tree:     "@1 [search_field] \"Echo\"\n@2 [list_item] \"Echo\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-echo-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-slack",
				Title:    "Slack",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Slack",
		"conversation": "Echo",
		"value":        "你好，Echo",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 3 {
		t.Fatalf("keyHistory = %#v, want only one quick-switcher plan", backend.keyHistory)
	}
	if len(backend.actTypeHistory) != 4 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "click" || backend.actTypeHistory[2] != "type" || backend.actTypeHistory[3] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type click type submit]", backend.actTypeHistory)
	}
	if backend.lastGroundingScreenshotWindow != "" || backend.lastScreenshotWindow != "" {
		t.Fatalf("unexpected visual fallback screenshots: grounding=%q screenshot=%q", backend.lastGroundingScreenshotWindow, backend.lastScreenshotWindow)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionMessageAliasShortcutSearchKeepsSearchQueryAndBodySeparate(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-orca-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
		"value":        "啊啊，Orca！",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 4 {
		t.Fatalf("keyHistory = %#v, want structured search open + clear-only key steps before search-field typing", backend.keyHistory)
	}
	if len(backend.actTypeHistory) != 4 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "click" || backend.actTypeHistory[2] != "type" || backend.actTypeHistory[3] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type click type submit]", backend.actTypeHistory)
	}
	if len(backend.actRefHistory) != 4 || backend.actRefHistory[0] != 1 || backend.actRefHistory[1] != 2 || backend.actRefHistory[2] != 1 || backend.actRefHistory[3] != 2 {
		t.Fatalf("actRefHistory = %#v, want [1 2 1 2]", backend.actRefHistory)
	}
	if len(backend.actValueHistory) != 4 {
		t.Fatalf("actValueHistory = %#v, want 4 values", backend.actValueHistory)
	}
	if backend.actValueHistory[0] != "Orca" {
		t.Fatalf("actValueHistory[0] = %q, want search query Orca", backend.actValueHistory[0])
	}
	if backend.actValueHistory[2] != "啊啊，Orca！" {
		t.Fatalf("actValueHistory[2] = %q, want exact body text", backend.actValueHistory[2])
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionMessageAliasShortcutSearchDoesNotTypeBodyWhenConversationStillMissing(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Echo\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Echo\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Echo",
		"value":        "你好，Echo",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	for _, actType := range backend.actTypeHistory {
		if actType != "type" {
			t.Fatalf("actTypeHistory = %#v, want only search-field type attempts before failing closed", backend.actTypeHistory)
		}
	}
	if len(backend.keyHistory) == 0 {
		t.Fatal("keyHistory = 0, want shortcut search attempts before failing closed")
	}
	if backend.lastActValue == "你好，Echo" {
		t.Fatalf("lastActValue = %q, want search query attempts only and never the body text", backend.lastActValue)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "fallback_exhausted" {
		t.Fatalf("error_code = %v, want fallback_exhausted", out["error_code"])
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
	if out["fallback_stage"] != "visual_confirmation" {
		t.Fatalf("fallback_stage = %v, want visual_confirmation", out["fallback_stage"])
	}
	if out["original_error_code"] != "target_not_found" {
		t.Fatalf("original_error_code = %v, want target_not_found", out["original_error_code"])
	}
}

func TestA11yToolExecute_ActionMessageAliasUsesCachedConversationPointBeforeShortcutSearch(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	key := a11yConversationClickCacheKey(a11yWindowQueryHint("", "Feishu,飞书,Lark"), "Team Ops")
	tool.clickCache = map[string]a11yConversationClickPoint{
		key: {X: 0.18, Y: 0.27},
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Team Ops",
		"value":        "你好，Team Ops",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want one cached click", backend.pointClickHistory)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no shortcut search when cache is valid", backend.keyHistory)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionMessageAliasInvalidatesStaleConversationPointCacheAndRefreshesViaVisualFallback(t *testing.T) {
	prevLocate := a11yLocateConversationVisualHit
	prevTimeout := a11yMessageConversationConfirmationTimeout
	a11yMessageConversationConfirmationTimeout = 0
	a11yLocateConversationVisualHit = func(context.Context, string, string) (a11yConversationVisualHit, error) {
		return a11yConversationVisualHit{
			Point:      a11yruntime.NormalizedPoint{X: 0.61, Y: 0.34},
			Confidence: 0.93,
		}, nil
	}
	defer func() {
		a11yLocateConversationVisualHit = prevLocate
		a11yMessageConversationConfirmationTimeout = prevTimeout
	}()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		screenshotImagePath: "/tmp/feishu-team-ops-search.png",
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	key := a11yConversationClickCacheKey(a11yWindowQueryHint("", "Feishu,飞书,Lark"), "Team Ops")
	tool.clickCache = map[string]a11yConversationClickPoint{
		key: {X: 0.12, Y: 0.21},
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Team Ops",
		"value":        "你好，Team Ops",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.pointClickHistory) != 2 {
		t.Fatalf("pointClickHistory = %#v, want cached click then visual click", backend.pointClickHistory)
	}
	if backend.pointClickHistory[1].X != 0.61 || backend.pointClickHistory[1].Y != 0.34 {
		t.Fatalf("visual click = %#v, want refreshed point", backend.pointClickHistory[1])
	}
	if len(backend.keyHistory) != 4 {
		t.Fatalf("keyHistory = %#v, want structured search open + clear-only key steps before search-field typing", backend.keyHistory)
	}
	if len(backend.actTypeHistory) != 3 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "type" || backend.actTypeHistory[2] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type type submit]", backend.actTypeHistory)
	}
	if got, ok := tool.clickCache[key]; !ok || got.X != 0.61 || got.Y != 0.34 {
		t.Fatalf("clickCache[%q] = %#v, want refreshed visual point", key, got)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionSelectAliasRecoversViaGroundingButtonConversationCandidate(t *testing.T) {
	prevLocate := a11yLocateConversationVisualHitFromPNG
	a11yLocateConversationVisualHitFromPNG = func(context.Context, []byte, string) (a11yConversationVisualHit, error) {
		return a11yConversationVisualHit{}, a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", nil)
	}
	defer func() { a11yLocateConversationVisualHitFromPNG = prevLocate }()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
		screenshotGroundingBytes: []byte("conversation-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{
						Role:       "button",
						Label:      "Echo",
						Confidence: 0.98,
						Bounds: a11yruntime.NormalizedRect{
							X:      0.22,
							Y:      0.18,
							Width:  0.24,
							Height: 0.08,
						},
					},
				},
			},
		},
	})

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Echo",
	})
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want one grounding-driven point click", backend.pointClickHistory)
	}
	if len(backend.keyHistory) == 0 {
		t.Fatalf("keyHistory = %#v, want structured search fallback attempts before grounding recovery", backend.keyHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
	if out["grounding_source"] != "vision_model" {
		t.Fatalf("grounding_source = %v, want vision_model", out["grounding_source"])
	}
	stages, ok := out["task_stages"].([]interface{})
	if !ok || len(stages) == 0 {
		t.Fatalf("task_stages = %#v, want non-empty stage trace", out["task_stages"])
	}
	confirmed := false
	for _, rawStage := range stages {
		stage, _ := rawStage.(map[string]interface{})
		if stage["stage"] == "confirm_conversation" && stage["status"] == "ok" {
			confirmed = true
			break
		}
	}
	if !confirmed {
		t.Fatalf("task_stages = %#v, want confirm_conversation ok entry", out["task_stages"])
	}
}

func TestA11yToolExecute_ActionMessageAliasFailsClosedWhenConversationVisualFallbackIsAmbiguous(t *testing.T) {
	prevLocate := a11yLocateConversationVisualHit
	a11yLocateConversationVisualHit = func(context.Context, string, string) (a11yConversationVisualHit, error) {
		return a11yConversationVisualHit{}, a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", map[string]interface{}{
			"target_name": "Team Ops",
		})
	}
	defer func() { a11yLocateConversationVisualHit = prevLocate }()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		screenshotImagePath: "/tmp/feishu-team-ops-search.png",
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Team Ops",
		"value":        "你好，Team Ops",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.pointClickHistory) != 0 {
		t.Fatalf("pointClickHistory = %#v, want no click when visual fallback is ambiguous", backend.pointClickHistory)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no body typing when visual fallback is ambiguous", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "fallback_exhausted" {
		t.Fatalf("error_code = %v, want fallback_exhausted", out["error_code"])
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
	if out["fallback_stage"] != "visual_confirmation" {
		t.Fatalf("fallback_stage = %v, want visual_confirmation", out["fallback_stage"])
	}
	if out["original_error_code"] != "target_not_found" {
		t.Fatalf("original_error_code = %v, want target_not_found", out["original_error_code"])
	}
}

func TestA11yToolExecute_ActionMessageAliasConversationVisualFastPathOnlyRunsOnDarwin(t *testing.T) {
	visualCalls := 0
	prevLocate := a11yLocateConversationVisualHit
	a11yLocateConversationVisualHit = func(context.Context, string, string) (a11yConversationVisualHit, error) {
		visualCalls++
		return a11yConversationVisualHit{
			Point:      a11yruntime.NormalizedPoint{X: 0.61, Y: 0.34},
			Confidence: 0.93,
		}, nil
	}
	defer func() { a11yLocateConversationVisualHit = prevLocate }()

	backend := &a11yCompatBackend{
		hostOS: "windows",
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "windows",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "windows",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "windows",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "windows",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Team Ops\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Team Ops",
		"value":        "你好，Team Ops",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if visualCalls != 0 {
		t.Fatalf("visualCalls = %d, want 0 on non-darwin hosts", visualCalls)
	}
	if len(backend.pointClickHistory) != 0 {
		t.Fatalf("pointClickHistory = %#v, want no visual point click on non-darwin hosts", backend.pointClickHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "fallback_exhausted" {
		t.Fatalf("error_code = %v, want fallback_exhausted", out["error_code"])
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
	if out["fallback_stage"] != "keyboard_search" {
		t.Fatalf("fallback_stage = %v, want keyboard_search", out["fallback_stage"])
	}
	if out["original_error_code"] != "target_not_found" {
		t.Fatalf("original_error_code = %v, want target_not_found", out["original_error_code"])
	}
}

func TestA11yToolExecute_SelectConversationPrefersVisualFallbackWhenMemoryRemembersVisualStrategy(t *testing.T) {
	backend := &a11yCompatBackend{
		hostOS: "darwin",
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [group] \"Results\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-results",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [group] \"Results\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-results",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [group] \"Results\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-results",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [group] \"Results\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-results",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [group] \"Results\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-results",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [group] \"Results\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-results",
				},
			},
		},
		screenshotGroundingBytes: []byte("conversation-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{
						Role:       "conversation",
						Label:      "Orca",
						Confidence: 0.98,
						Bounds: a11yruntime.NormalizedRect{
							X:      0.12,
							Y:      0.22,
							Width:  0.25,
							Height: 0.08,
						},
						RationaleTags: []string{"current", "selected"},
					},
				},
			},
		},
	})
	tool.chatMemory.Remember("darwin", "feishu_lark", "select", string(a11yChatStageLocateConversation), "visual_sidebar_hit")

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
	})
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no keyboard-search fallback before remembered visual path", backend.keyHistory)
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want single visual point click", backend.pointClickHistory)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no search typing or body typing in select flow", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["stage"] != "locate_conversation" {
		t.Fatalf("stage = %v, want locate_conversation after remembered visual fallback success", out["stage"])
	}
}

func TestA11yToolExecute_SelectConversationContinuesToKeyboardSearchAfterStaleVisualCacheMiss(t *testing.T) {
	prevSettle := a11yMessageConversationSettleDelay
	a11yMessageConversationSettleDelay = 0
	defer func() {
		a11yMessageConversationSettleDelay = prevSettle
	}()

	args := map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
	}
	backend := &a11yCompatBackend{
		hostOS: "darwin",
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-orca",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		screenshotGroundingBytes: []byte("conversation-grounding"),
		pointClickErr: a11yruntime.NewError("confirmation_failed", "cached point no longer maps to the conversation", nil),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	cacheKey := tool.a11yConversationClickCacheKeyForArgs(args, a11yTargetSelector{Name: "Orca", Role: "conversation"}, "win-feishu")
	tool.a11yConversationClickCacheSet(cacheKey, a11yConversationClickPoint{X: 0.20, Y: 0.30})

	raw, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want one stale cached click attempt", backend.pointClickHistory)
	}
	if len(backend.keyHistory) == 0 {
		t.Fatalf("keyHistory = %#v, want keyboard-search fallback after stale visual cache miss", backend.keyHistory)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "click" {
		t.Fatalf("actTypeHistory = %#v, want [type click] after keyboard-search recovery", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil after keyboard-search recovery", out["error_code"])
	}
}

func TestA11yToolExecute_SelectConversationFallsBackToKeyboardSearchAfterRememberedVisualConfirmationFailure(t *testing.T) {
	prevLocate := a11yLocateConversationVisualHit
	a11yLocateConversationVisualHit = func(context.Context, string, string) (a11yConversationVisualHit, error) {
		return a11yConversationVisualHit{
			Point:      a11yruntime.NormalizedPoint{X: 0.61, Y: 0.34},
			Confidence: 0.93,
		}, nil
	}
	defer func() { a11yLocateConversationVisualHit = prevLocate }()

	backend := &a11yCompatBackend{
		hostOS: "darwin",
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [group] \"Results\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-results",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-orca",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		screenshotImagePath: "/tmp/example.png",
		pointClickErr:       a11yruntime.NewError("confirmation_failed", "conversation switch could not be confirmed", nil),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.chatMemory.Remember("darwin", "feishu_lark", "select", string(a11yChatStageLocateConversation), "visual_sidebar_hit")

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
	})
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if len(backend.pointClickHistory) == 0 {
		t.Fatalf("pointClickHistory = %#v, want remembered visual attempt before keyboard fallback", backend.pointClickHistory)
	}
	if len(backend.actTypeHistory) < 2 || backend.actTypeHistory[len(backend.actTypeHistory)-1] != "click" {
		t.Fatalf("actTypeHistory = %#v, want keyboard-search recovery ending with click", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil after keyboard-search fallback", out["error_code"])
	}
}

func TestA11yToolExecute_ActionTypeAliasUsesTopLevelConversationAndDefaultsToDraftOnly(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "你好，Orca",
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" {
		t.Fatalf("actTypeHistory = %#v, want [click type]", backend.actTypeHistory)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no shortcut search when direct conversation resolve succeeds", backend.keyHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
}

func TestA11yToolExecute_ActionTypeAliasHonorsExplicitSubmitTrue(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "你好，Orca",
		"submit":       true,
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 3 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" || backend.actTypeHistory[2] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [click type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActionSelectAliasUsesTopLevelConversation(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [list_item] \"Team\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-team-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu",
		"conversation": "Orca",
	})
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 1 || backend.actTypeHistory[0] != "select" {
		t.Fatalf("actTypeHistory = %#v, want [select]", backend.actTypeHistory)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
}

func TestA11yToolExecute_ActionSelectAliasFallsBackToClickWhenSelectUnsupported(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [list_item] \"Team\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-team-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		actErrorsByType: map[string]error{
			"select": a11yruntime.NewError("unsupported_action", "element does not expose a selectable semantic action", map[string]interface{}{"act_type": "select"}),
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu",
		"conversation": "Orca",
	})
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "select" || backend.actTypeHistory[1] != "click" {
		t.Fatalf("actTypeHistory = %#v, want [select click]", backend.actTypeHistory)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
}

func TestA11yToolExecute_ActionSelectAliasUsesConversationSearchFallbackWhenStructuredMiss(t *testing.T) {
	prevSettle := a11yMessageConversationSettleDelay
	a11yMessageConversationSettleDelay = 0
	defer func() {
		a11yMessageConversationSettleDelay = prevSettle
	}()

	backend := &a11yCompatBackend{
		hostOS: "darwin",
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-orca",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
	})
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "click" {
		t.Fatalf("actTypeHistory = %#v, want [type click] from conversation search fallback", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil on recovered select conversation fallback", out["error_code"])
	}
	if out["stage"] != "confirm_conversation" {
		t.Fatalf("stage = %v, want confirm_conversation", out["stage"])
	}
}

func TestA11yToolExecute_ActInfersSelectFromConversationWithoutValue(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [list_item] \"Team\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-team-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"app_name":     "Feishu",
		"conversation": "Orca",
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 1 || backend.actTypeHistory[0] != "select" {
		t.Fatalf("actTypeHistory = %#v, want [select]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
}

func TestA11yToolExecute_ActAutoResolvesInputRoleAlias(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1 for input role alias", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActTypeWithoutSelectorDefaultsToUniqueInput(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type": "type",
			"value":    "你好，Orca",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1 for default unique input resolution", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActTypeWithoutSelectorPrefersLikelyComposerInput(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [search_field] \"Search\"\n@2 [document]\n@3 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-search",
				2: "token-editor",
				3: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type": "type",
			"value":    "你好，Orca",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActRef != 2 {
		t.Fatalf("lastActRef = %d, want 2 for likely composer input", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActTypeWithoutSelectorPrefersInputNearestLikelySubmitControl(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [document]\n@3 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-history",
				2: "token-editor",
				3: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type": "type",
			"value":    "你好，Orca",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActRef != 2 {
		t.Fatalf("lastActRef = %d, want 2 for input nearest likely submit control", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActWithoutActTypeDefaultsValueToType(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"value":  "你好，Orca",
			"submit": true,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActWithoutWindowSelectorUsesFocusedWindowByDefault(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-code", Title: "Code", AppName: "Code"},
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu", Focused: true},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"value":  "你好，Orca",
			"submit": true,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.snapshotWindowHistory) == 0 || backend.snapshotWindowHistory[0] != "" {
		t.Fatalf("snapshotWindowHistory = %#v, want first snapshot to target focused window implicitly", backend.snapshotWindowHistory)
	}
	if backend.lastActWindowID != "win-feishu" {
		t.Fatalf("lastActWindowID = %q, want win-feishu", backend.lastActWindowID)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_ActTargetSelectorUsesCachedSnapshot(t *testing.T) {
	backend := &a11yCompatBackend{
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-9",
			Title:    "Example",
			Tree:     "@1 [button] \"Send\"\n@2 [text_field] \"Message\"",
			RefMap: map[int]string{
				1: "token-send",
				2: "token-message",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "snapshot_interactive",
		"window_id": "win-9",
	}); err != nil {
		t.Fatalf("snapshot_interactive error = %v", err)
	}
	if backend.interactiveCalls != 1 {
		t.Fatalf("interactiveCalls after snapshot = %d, want 1", backend.interactiveCalls)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "act",
		"window_id": "win-9",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_name": "Message",
			"value":       "你好，Orca",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.interactiveCalls != 1 {
		t.Fatalf("interactiveCalls after cached act = %d, want 1", backend.interactiveCalls)
	}
	if backend.lastActRef != 2 {
		t.Fatalf("lastActRef = %d, want 2", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActTargetSelectorRejectsAmbiguousMatches(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Open\"\n@2 [button] \"Open\"",
			RefMap: map[int]string{
				1: "token-open-1",
				2: "token-open-2",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "click",
			"target_name": "Open",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 on ambiguous target selector", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "ambiguous_target" {
		t.Fatalf("error_code = %v, want ambiguous_target", out["error_code"])
	}
	if _, ok := out["matching_refs"].([]interface{}); !ok {
		t.Fatalf("matching_refs = %#v, want JSON array", out["matching_refs"])
	}
}

func TestA11yToolExecute_TypeWithSubmitUsesEnterKeyByDefault(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]",
			RefMap: map[int]string{
				1: "token-editor",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
			"submit":      true,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 1 {
		t.Fatalf("actCalls = %d, want 1", backend.actCalls)
	}
	if len(backend.actTypeHistory) != 1 || backend.actTypeHistory[0] != "type" {
		t.Fatalf("actTypeHistory = %#v, want [type]", backend.actTypeHistory)
	}
	if backend.lastKeyWindowID != "win-feishu" {
		t.Fatalf("lastKeyWindowID = %q, want win-feishu", backend.lastKeyWindowID)
	}
	if len(backend.lastKeys) != 1 || backend.lastKeys[0] != "enter" {
		t.Fatalf("lastKeys = %#v, want [enter]", backend.lastKeys)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_TypeWithSubmitPrefersLikelySendButton(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
			"submit":      true,
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 2 {
		t.Fatalf("actCalls = %d, want 2", backend.actCalls)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit]", backend.actTypeHistory)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no key fallback when likely send button is available", backend.keyHistory)
	}
}

func TestResolveLikelySubmitRefFromEntries_PrefersSubmitNearestTypedInput(t *testing.T) {
	entries := parseA11ySnapshotEntries("@1 [button] \"Send\"\n@2 [list_item] \"Workspace\"\n@3 [document]\n@4 [button] \"Send\"")

	ref, ok := resolveLikelySubmitRefFromEntries(entries, 3)
	if !ok {
		t.Fatal("resolveLikelySubmitRefFromEntries() ok = false, want true")
	}
	if ref != 4 {
		t.Fatalf("ref = %d, want 4 for submit nearest typed input", ref)
	}
}

func TestA11yToolExecute_TypeWithSubmitPrefersLikelySendButtonNearestTypedInput(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"\n@2 [list_item] \"Workspace\"\n@3 [document]\n@4 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-toolbar-send",
				2: "token-workspace",
				3: "token-editor",
				4: "token-composer-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
			"submit":      true,
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 2 {
		t.Fatalf("actCalls = %d, want 2", backend.actCalls)
	}
	if len(backend.actRefHistory) != 2 || backend.actRefHistory[0] != 3 || backend.actRefHistory[1] != 4 {
		t.Fatalf("actRefHistory = %#v, want [3 4]", backend.actRefHistory)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no key fallback when nearest likely send button is available", backend.keyHistory)
	}
}

func TestA11yToolExecute_TypeWithSubmitFallsBackToEnterWhenLikelySendButtonFails(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
		actErrorsByType: map[string]error{
			"submit": a11yruntime.NewError("backend_unavailable", "submit failed", nil),
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
			"submit":      true,
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 2 {
		t.Fatalf("actCalls = %d, want 2", backend.actCalls)
	}
	if len(backend.keyHistory) != 1 || len(backend.keyHistory[0]) != 1 || backend.keyHistory[0][0] != "enter" {
		t.Fatalf("keyHistory = %#v, want [[enter]]", backend.keyHistory)
	}
}

func TestA11yToolExecute_TypeWithSubmitFallsBackWhenPostSubmitSnapshotStillShowsTypedValue(t *testing.T) {
	prevTimeout := a11ySubmitConfirmationTimeout
	prevPoll := a11ySubmitConfirmationPollInterval
	a11ySubmitConfirmationTimeout = time.Second
	a11ySubmitConfirmationPollInterval = time.Second
	defer func() {
		a11ySubmitConfirmationTimeout = prevTimeout
		a11ySubmitConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
			"submit":      true,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 2 {
		t.Fatalf("actCalls = %d, want 2", backend.actCalls)
	}
	if len(backend.keyHistory) != 1 || len(backend.keyHistory[0]) != 1 || backend.keyHistory[0][0] != "enter" {
		t.Fatalf("keyHistory = %#v, want [[enter]] after unconfirmed submit click", backend.keyHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_TypeWithSubmitSkipsFallbackWhenPendingTextClearsOnRetryCheck(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
			"submit":      true,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 2 {
		t.Fatalf("actCalls = %d, want 2", backend.actCalls)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no key fallback when retry check clears pending text", backend.keyHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestA11yToolExecute_TypeWithSubmitFallsBackToPlatformChordAfterEnterFailure(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]",
			RefMap: map[int]string{
				1: "token-editor",
			},
		},
		keyErrorsByChord: map[string]error{
			"enter": a11yruntime.NewError("backend_unavailable", "enter failed", nil),
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
			"submit":      true,
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 2 {
		t.Fatalf("keyHistory = %#v, want two key attempts", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 1 || got[0] != "enter" {
		t.Fatalf("first key attempt = %#v, want [enter]", got)
	}
	if got := backend.keyHistory[1]; len(got) != 2 || got[0] != "cmd" || got[1] != "enter" {
		t.Fatalf("second key attempt = %#v, want [cmd enter]", got)
	}
}

func TestA11yToolExecute_TypeWithSubmitFailsWhenAllFallbacksLeaveTypedValuePending(t *testing.T) {
	prevTimeout := a11ySubmitConfirmationTimeout
	prevPoll := a11ySubmitConfirmationPollInterval
	a11ySubmitConfirmationTimeout = time.Second
	a11ySubmitConfirmationPollInterval = time.Second
	defer func() {
		a11ySubmitConfirmationTimeout = prevTimeout
		a11ySubmitConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":    "type",
			"target_role": "input",
			"value":       "你好，Orca",
			"submit":      true,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 2 {
		t.Fatalf("keyHistory = %#v, want two key fallbacks before failure", backend.keyHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "backend_unavailable" {
		t.Fatalf("error_code = %v, want backend_unavailable", out["error_code"])
	}
	if out["error"] != "submit could not be confirmed" {
		t.Fatalf("error = %v, want submit could not be confirmed", out["error"])
	}
}

func TestA11yToolExecute_TypeWithSubmitTargetSelectorUsesSubmitAction(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type":           "type",
			"target_role":        "input",
			"value":              "你好，Orca",
			"submit":             true,
			"submit_target_name": "Send",
			"submit_target_role": "button",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 2 {
		t.Fatalf("actCalls = %d, want 2", backend.actCalls)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit]", backend.actTypeHistory)
	}
	if len(backend.actRefHistory) != 2 || backend.actRefHistory[0] != 1 || backend.actRefHistory[1] != 2 {
		t.Fatalf("actRefHistory = %#v, want [1 2]", backend.actRefHistory)
	}
	if len(backend.lastKeys) != 0 {
		t.Fatalf("lastKeys = %#v, want no key fallback when submit target selector is used", backend.lastKeys)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message = %v, want Host action completed and submitted", out["message"])
	}
}

func TestConfirmA11yMessageConversationActivated_PollsUntilComposerReady(t *testing.T) {
	prevTimeout := a11yMessageConversationConfirmationTimeout
	prevPoll := a11yMessageConversationConfirmationPollInterval
	a11yMessageConversationConfirmationTimeout = 2 * time.Second
	a11yMessageConversationConfirmationPollInterval = time.Second
	defer func() {
		a11yMessageConversationConfirmationTimeout = prevTimeout
		a11yMessageConversationConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [list_item] \"Orca\"",
				RefMap:   map[int]string{1: "token-search", 2: "token-orca"},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	windowID, err := tool.confirmA11yMessageConversationActivated(context.Background(), backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("confirmA11yMessageConversationActivated() error = %v", err)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
	if backend.interactiveCalls != 2 {
		t.Fatalf("interactiveCalls = %d, want 2 polls", backend.interactiveCalls)
	}
}

func TestConfirmA11yMessageConversationActivated_WaitsForTransientVisualConversationMismatch(t *testing.T) {
	prevTimeout := a11yMessageConversationConfirmationTimeout
	prevPoll := a11yMessageConversationConfirmationPollInterval
	a11yMessageConversationConfirmationTimeout = 100 * time.Millisecond
	a11yMessageConversationConfirmationPollInterval = time.Millisecond
	defer func() {
		a11yMessageConversationConfirmationTimeout = prevTimeout
		a11yMessageConversationConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
		},
		screenshotGroundingBytes: []byte("conversation-confirm-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		resultSequences: map[string][]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				{
					Source: "vision_model",
					Candidates: []a11yChatGroundingCandidate{
						{Role: "search_field", Label: "Search", Confidence: 0.99, RationaleTags: []string{"search_field"}},
					},
				},
				{
					Source: "vision_model",
					Candidates: []a11yChatGroundingCandidate{
						{Role: "conversation", Label: "Orca", Confidence: 0.98},
					},
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)
	ctx := withA11yChatExecutionState(context.Background(), newA11yChatExecutionState("darwin", "feishu_lark", "select", "Orca"))

	windowID, err := tool.confirmA11yMessageConversationActivated(ctx, backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("confirmA11yMessageConversationActivated() error = %v", err)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
	if backend.interactiveCalls < 2 {
		t.Fatalf("interactiveCalls = %d, want retry after transient visual mismatch", backend.interactiveCalls)
	}
	locateConversationCalls := 0
	for _, call := range grounder.calls {
		if call.TaskHint == a11yChatGroundingTaskLocateConversation {
			locateConversationCalls++
		}
	}
	if locateConversationCalls < 2 {
		t.Fatalf("grounder.calls = %#v, want repeated locate_conversation grounding attempts", grounder.calls)
	}
}

func TestConfirmA11yMessageConversationActivated_UsesGroundingToOverrideStaleSearchField(t *testing.T) {
	prevTimeout := a11yMessageConversationConfirmationTimeout
	prevPoll := a11yMessageConversationConfirmationPollInterval
	a11yMessageConversationConfirmationTimeout = 0
	a11yMessageConversationConfirmationPollInterval = time.Millisecond
	defer func() {
		a11yMessageConversationConfirmationTimeout = prevTimeout
		a11yMessageConversationConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [document] \"Type a message\"\n@3 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-search", 2: "token-editor", 3: "token-send"},
			},
		},
		screenshotGroundingBytes: []byte("conversation-confirm-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "conversation", Label: "Orca", Confidence: 0.98},
				},
			},
		},
	})
	ctx := withA11yChatExecutionState(context.Background(), newA11yChatExecutionState("darwin", "feishu_lark", "select", "Orca"))

	windowID, err := tool.confirmA11yMessageConversationActivated(ctx, backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("confirmA11yMessageConversationActivated() error = %v", err)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
}

func TestConfirmA11yMessageConversationActivated_DoesNotOverrideStaleSearchFieldForMismatchedGroundingCandidate(t *testing.T) {
	prevTimeout := a11yMessageConversationConfirmationTimeout
	prevPoll := a11yMessageConversationConfirmationPollInterval
	a11yMessageConversationConfirmationTimeout = 0
	a11yMessageConversationConfirmationPollInterval = time.Millisecond
	defer func() {
		a11yMessageConversationConfirmationTimeout = prevTimeout
		a11yMessageConversationConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [document] \"Type a message\"\n@3 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-search", 2: "token-editor", 3: "token-send"},
			},
		},
		screenshotGroundingBytes: []byte("conversation-confirm-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "conversation", Label: "Team Ops", Confidence: 0.98},
				},
			},
		},
	})
	ctx := withA11yChatExecutionState(context.Background(), newA11yChatExecutionState("darwin", "feishu_lark", "select", "Orca"))

	_, err := tool.confirmA11yMessageConversationActivated(ctx, backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err == nil {
		t.Fatal("confirmA11yMessageConversationActivated() error = nil, want confirmation_failed")
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "confirmation_failed" {
		t.Fatalf("code = %q, want confirmation_failed", runtimeErr.Code)
	}
}

func TestConfirmA11yMessageConversationActivated_UsesActiveGroundingRationaleTagToBreakConfirmationTie(t *testing.T) {
	prevTimeout := a11yMessageConversationConfirmationTimeout
	prevPoll := a11yMessageConversationConfirmationPollInterval
	a11yMessageConversationConfirmationTimeout = 0
	a11yMessageConversationConfirmationPollInterval = time.Millisecond
	defer func() {
		a11yMessageConversationConfirmationTimeout = prevTimeout
		a11yMessageConversationConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [document] \"Type a message\"\n@3 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-search", 2: "token-editor", 3: "token-send"},
			},
		},
		screenshotGroundingBytes: []byte("conversation-confirm-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "conversation", Label: "Orca", Confidence: 0.98},
					{Role: "conversation", Label: "Orca", Confidence: 0.98, RationaleTags: []string{"current", "selected"}},
				},
			},
		},
	})
	ctx := withA11yChatExecutionState(context.Background(), newA11yChatExecutionState("darwin", "feishu_lark", "select", "Orca"))

	windowID, err := tool.confirmA11yMessageConversationActivated(ctx, backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("confirmA11yMessageConversationActivated() error = %v", err)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
}

func TestConfirmA11yMessageConversationActivated_DoesNotRetryWhenSearchFieldAndConfirmedConversationAreBothGrounded(t *testing.T) {
	prevTimeout := a11yMessageConversationConfirmationTimeout
	prevPoll := a11yMessageConversationConfirmationPollInterval
	a11yMessageConversationConfirmationTimeout = 0
	a11yMessageConversationConfirmationPollInterval = time.Millisecond
	defer func() {
		a11yMessageConversationConfirmationTimeout = prevTimeout
		a11yMessageConversationConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [document] \"Type a message\"\n@3 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-search", 2: "token-editor", 3: "token-send"},
			},
		},
		screenshotGroundingBytes: []byte("conversation-confirm-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "search_field", Label: "Search", Confidence: 0.99, RationaleTags: []string{"search_field"}},
					{Role: "conversation", Label: "Orca", Confidence: 0.98, RationaleTags: []string{"current", "selected"}},
				},
			},
		},
	})
	ctx := withA11yChatExecutionState(context.Background(), newA11yChatExecutionState("darwin", "feishu_lark", "select", "Orca"))

	windowID, err := tool.confirmA11yMessageConversationActivated(ctx, backend, "win-feishu", a11yTargetSelector{Name: "Orca", Role: "conversation"})
	if err != nil {
		t.Fatalf("confirmA11yMessageConversationActivated() error = %v", err)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
}

func TestA11ySubmitNeedsRetry_PollsUntilPendingTextClears(t *testing.T) {
	prevTimeout := a11ySubmitConfirmationTimeout
	prevPoll := a11ySubmitConfirmationPollInterval
	a11ySubmitConfirmationTimeout = 3 * time.Second
	a11ySubmitConfirmationPollInterval = time.Second
	defer func() {
		a11ySubmitConfirmationTimeout = prevTimeout
		a11ySubmitConfirmationPollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [document] \"你好，Orca\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	needsRetry := tool.a11ySubmitNeedsRetry(context.Background(), backend, "win-feishu", a11ySubmitConfirmation{
		TypedValue: "你好，Orca",
		InputToken: "token-editor",
	})
	if needsRetry {
		t.Fatal("a11ySubmitNeedsRetry() = true, want false after polling clears pending text")
	}
	if backend.interactiveCalls != 3 {
		t.Fatalf("interactiveCalls = %d, want 3 polls", backend.interactiveCalls)
	}
}
