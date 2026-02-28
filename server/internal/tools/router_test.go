package tools

import "testing"

func TestToolRouterDynamicExposureHidesProcessByDefault(t *testing.T) {
	router := DefaultToolRouter()
	defs := []ToolDefinition{
		{Name: "exec"},
		{Name: "process"},
		{Name: "ask"},
	}

	got := router.Route("请帮我查看当前目录文件", "auto", defs)
	if hasTool(got, "process") {
		t.Fatalf("process should be hidden for non-process query: %+v", got)
	}
	if !hasTool(got, "exec") {
		t.Fatalf("exec must remain exposed: %+v", got)
	}
}

func TestToolRouterDynamicExposureKeepsProcessWhenNeeded(t *testing.T) {
	router := DefaultToolRouter()
	defs := []ToolDefinition{
		{Name: "exec"},
		{Name: "process"},
	}

	got := router.Route("poll process session status", "auto", defs)
	if !hasTool(got, "process") {
		t.Fatalf("process should be exposed for process/session queries: %+v", got)
	}
}

func TestToolRouterSchemaCompression(t *testing.T) {
	router := DefaultToolRouter()
	router.DynamicExposure = false

	defs := []ToolDefinition{
		{
			Name: "exec",
			Parameters: map[string]interface{}{
				"type":        "object",
				"description": "verbose object description",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "very long command description",
						"title":       "Command",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"safe", "raw"},
						"description": "mode description",
					},
				},
				"required": []string{"command"},
			},
		},
	}

	got := router.Route("run command", "auto", defs)
	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}

	schema := got[0].Parameters
	if _, ok := schema["description"]; ok {
		t.Fatalf("schema top-level description should be removed: %+v", schema)
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("compressed schema must keep properties object")
	}
	command, ok := props["command"].(map[string]interface{})
	if !ok {
		t.Fatalf("command property missing after compression")
	}
	if command["type"] != "string" {
		t.Fatalf("command.type should remain string, got: %#v", command["type"])
	}
	if _, ok := command["description"]; ok {
		t.Fatalf("property descriptions should be removed: %+v", command)
	}
	mode, ok := props["mode"].(map[string]interface{})
	if !ok {
		t.Fatalf("mode property missing after compression")
	}
	if _, ok := mode["enum"]; !ok {
		t.Fatalf("mode.enum should be preserved: %+v", mode)
	}
	if _, ok := schema["required"]; !ok {
		t.Fatalf("required should be preserved: %+v", schema)
	}

	// Ensure original schema is not mutated.
	origProps := defs[0].Parameters["properties"].(map[string]interface{})
	origCmd := origProps["command"].(map[string]interface{})
	if _, ok := origCmd["description"]; !ok {
		t.Fatalf("original schema was mutated unexpectedly: %+v", origCmd)
	}

	stats := router.Stats()
	if stats.SchemaBytesAfter >= stats.SchemaBytesBefore {
		t.Fatalf("expected schema bytes to shrink, before=%d after=%d", stats.SchemaBytesBefore, stats.SchemaBytesAfter)
	}
}

func hasTool(defs []ToolDefinition, name string) bool {
	for _, def := range defs {
		if def.Name == name {
			return true
		}
	}
	return false
}
