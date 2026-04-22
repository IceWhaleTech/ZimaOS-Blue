package tools

import "testing"

func TestCanonicalizeBrowserAction_ExplicitAliasCoverage(t *testing.T) {
	tests := []struct {
		name        string
		action      string
		actType     string
		wantAction  string
		wantActType string
	}{
		{name: "open", action: "open", wantAction: "navigate"},
		{name: "inspect", action: "inspect", wantAction: "snapshot"},
		{name: "interactive", action: "interactive", wantAction: "snapshot_interactive"},
		{name: "read", action: "read", wantAction: "snapshot_auto"},
		{name: "status", action: "status", wantAction: "tabs"},
		{name: "remove", action: "remove", wantAction: "close"},
		{name: "run recipe", action: "run_recipe", wantAction: "recipe"},
		{name: "list recipes", action: "list_recipes", wantAction: "recipes"},
		{name: "click", action: "click", wantAction: "act", wantActType: "click"},
		{name: "scroll down", action: "scroll_down", wantAction: "scroll_page", wantActType: "down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAction, gotActType := CanonicalizeBrowserAction(tt.action, tt.actType)
			if gotAction != tt.wantAction {
				t.Fatalf("action = %q, want %q", gotAction, tt.wantAction)
			}
			if gotActType != tt.wantActType {
				t.Fatalf("act_type = %q, want %q", gotActType, tt.wantActType)
			}
		})
	}
}

func TestBrowserLegacyPageScrollDelta(t *testing.T) {
	direction, x, y, ok := BrowserLegacyPageScrollDelta("scroll_down", "")
	if !ok {
		t.Fatal("expected scroll_down to map to a page scroll delta")
	}
	if direction != "down" {
		t.Fatalf("direction = %q, want down", direction)
	}
	if x != 0 {
		t.Fatalf("x = %d, want 0", x)
	}
	if y <= 0 {
		t.Fatalf("y = %d, want > 0", y)
	}
}

func TestCanonicalizeBrowserAction_FuzzyIntentPhrases(t *testing.T) {
	tests := []struct {
		name        string
		action      string
		wantAction  string
		wantActType string
	}{
		{name: "open website phrase", action: "open website", wantAction: "navigate"},
		{name: "take screenshot phrase", action: "take screenshot of the page", wantAction: "screenshot"},
		{name: "chinese interactive phrase", action: "查看交互元素", wantAction: "snapshot_interactive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAction, gotActType := CanonicalizeBrowserAction(tt.action, "")
			if gotAction != tt.wantAction {
				t.Fatalf("action = %q, want %q", gotAction, tt.wantAction)
			}
			if gotActType != tt.wantActType {
				t.Fatalf("act_type = %q, want %q", gotActType, tt.wantActType)
			}
		})
	}
}

func TestBrowserToolDefinition_DeclaresActionEnum(t *testing.T) {
	def := NewBrowserTool().Definition()
	properties, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties type = %T, want map[string]interface{}", def.Parameters["properties"])
	}
	actionSchema, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("action schema type = %T, want map[string]interface{}", properties["action"])
	}
	enumValues, ok := actionSchema["enum"].([]string)
	if !ok {
		t.Fatalf("action enum type = %T, want []string", actionSchema["enum"])
	}
	want := []string{"navigate", "snapshot", "snapshot_interactive", "snapshot_auto", "act", "screenshot", "tabs", "close", "recipe", "recipes"}
	for _, candidate := range want {
		found := false
		for _, value := range enumValues {
			if value == candidate {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing action enum %q in %v", candidate, enumValues)
		}
	}
}
