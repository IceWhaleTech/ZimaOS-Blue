package tools

import (
	"context"
	"testing"
)

func TestExecutorExecute_FuzzyActionCompatUsesSchemaEnumCandidate(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "docx",
			Description: "Docx test tool",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type": "string",
						"enum": []string{"read", "create", "apply_template", "validate"},
					},
				},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "docx", map[string]interface{}{
		"action": "apply template to the report",
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := tool.args["action"]; got != "apply_template" {
		t.Fatalf("action = %v, want apply_template", got)
	}
}

func TestExecutorExecute_FuzzyActionCompatPromotesOperationAliasToAction(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "docx",
			Description: "Docx test tool",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type": "string",
						"enum": []string{"read", "create", "apply_template", "validate"},
					},
				},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "docx", map[string]interface{}{
		"operation": "make a new document",
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := tool.args["action"]; got != "create" {
		t.Fatalf("action = %v, want create", got)
	}
}

func TestExecutorExecute_FuzzyActionCompatSupportsChineseIntentPhrase(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "docx",
			Description: "Docx test tool",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type": "string",
						"enum": []string{"read", "create", "apply_template", "validate"},
					},
				},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "docx", map[string]interface{}{
		"action": "套用模板",
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := tool.args["action"]; got != "apply_template" {
		t.Fatalf("action = %v, want apply_template", got)
	}
}

func TestExecutorExecute_FuzzyActionCompatLeavesAmbiguousPhraseUntouched(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "ambiguous_tool",
			Description: "Ambiguous action test tool",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type": "string",
						"enum": []string{"read", "list"},
					},
				},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "ambiguous_tool", map[string]interface{}{
		"action": "show",
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := tool.args["action"]; got != "show" {
		t.Fatalf("action = %v, want original ambiguous phrase to remain untouched", got)
	}
}

func TestExecutorExecute_FuzzyEnumCompatUsesSchemaEnumCandidate(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "theme_tool",
			Description: "Theme enum test tool",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"theme": map[string]interface{}{
						"type": "string",
						"enum": []string{"analysis", "editorial", "midnight"},
					},
				},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "theme_tool", map[string]interface{}{
		"theme": "editorial style",
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := tool.args["theme"]; got != "editorial" {
		t.Fatalf("theme = %v, want editorial", got)
	}
}

func TestExecutorExecute_FuzzyEnumCompatPromotesNestedEnumField(t *testing.T) {
	registry := NewRegistry()
	tool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "theme_tool",
			Description: "Theme enum test tool",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"theme": map[string]interface{}{
						"type": "string",
						"enum": []string{"analysis", "editorial", "midnight"},
					},
				},
			},
		},
	}
	registry.Register(tool)

	executor := NewExecutor(registry)
	if _, err := executor.Execute(context.Background(), "theme_tool", map[string]interface{}{
		"input": map[string]interface{}{
			"theme": "midnight theme",
		},
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := tool.args["theme"]; got != "midnight" {
		t.Fatalf("theme = %v, want midnight", got)
	}
}
