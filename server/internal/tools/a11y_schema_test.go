package tools

import "testing"

func TestComputerUseSchema_AcceptsPreferVisualLocateStrategy(t *testing.T) {
	tool := NewA11yTool()
	def := tool.Definition()
	if err := ValidateToolSchema(def.Parameters); err != nil {
		t.Fatalf("ValidateToolSchema() error = %v", err)
	}
	args := map[string]interface{}{
		"action":          "message",
		"prefer_visual":   true,
		"locate_strategy": "visual_first",
		"app_name":        "飞书",
		"conversation":    "test_group",
		"value":           "hi",
		"submit":          false,
	}
	if err := ValidateToolArguments(def.Parameters, args); err != nil {
		t.Fatalf("ValidateToolArguments() error = %v", err)
	}
}
