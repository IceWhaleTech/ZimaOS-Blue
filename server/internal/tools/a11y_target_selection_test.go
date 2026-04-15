package tools

import (
	"context"
	"encoding/json"
	"testing"

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
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Orca\"\n@2 [list_item] \"Orca\"",
			RefMap: map[int]string{
				1: "token-orca-button",
				2: "token-orca-list-item",
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
		t.Fatalf("keyHistory = %#v, want no submit fallback when submit=false", backend.keyHistory)
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
		t.Fatalf("keyHistory = %#v, want no submit fallback by default", backend.keyHistory)
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
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [list_item] \"Orca\"\n@2 [list_item] \"Team\"",
			RefMap: map[int]string{
				1: "token-orca-conversation",
				2: "token-team-conversation",
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
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [list_item] \"Orca\"\n@2 [list_item] \"Team\"",
			RefMap: map[int]string{
				1: "token-orca-conversation",
				2: "token-team-conversation",
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

func TestA11yToolExecute_ActInfersSelectFromConversationWithoutValue(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [list_item] \"Orca\"\n@2 [list_item] \"Team\"",
			RefMap: map[int]string{
				1: "token-orca-conversation",
				2: "token-team-conversation",
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
