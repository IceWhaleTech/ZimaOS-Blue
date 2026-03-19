package bootstrap

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type bootstrapPluginTool struct {
	name                string
	riskLevel           string
	visibilityAllowlist []string
	result              interface{}
}

type bootstrapNativePlugin struct {
	id   string
	tool bootstrapPluginTool
}

func (p *bootstrapNativePlugin) ID() string { return p.id }

func (p *bootstrapNativePlugin) Manifest() *plugin.Manifest {
	return &plugin.Manifest{
		ID:           p.id,
		Name:         p.id,
		Description:  "bootstrap test plugin",
		Version:      "1.0.0",
		ConfigSchema: map[string]interface{}{},
	}
}

func (p *bootstrapNativePlugin) Init(_ context.Context, api plugin.PluginAPI) error {
	result := p.tool.result
	if result == nil {
		result = map[string]interface{}{"ok": true}
	}
	return api.RegisterTool(plugin.Tool{
		Name:                p.tool.name,
		Description:         "plugin tool",
		Parameters:          map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		RiskLevel:           p.tool.riskLevel,
		VisibilityAllowlist: append([]string(nil), p.tool.visibilityAllowlist...),
		Handler: func(context.Context, map[string]interface{}) (interface{}, error) {
			return result, nil
		},
	})
}

func (p *bootstrapNativePlugin) Start(context.Context) error { return nil }

func (p *bootstrapNativePlugin) Stop(context.Context) error { return nil }

func (p *bootstrapNativePlugin) IsNative() bool { return true }

func TestRegisterPluginToolsPreservesRiskAndVisibility(t *testing.T) {
	pluginRegistry := plugin.NewRegistry()
	if err := pluginRegistry.RegisterNativePlugin(&bootstrapNativePlugin{
		id: "plugin-risk-test",
		tool: bootstrapPluginTool{
			name:                "plugin_tool",
			riskLevel:           "high",
			visibilityAllowlist: []string{string(tools.ToolRouteKindAgent)},
		},
	}); err != nil {
		t.Fatalf("RegisterNativePlugin() error = %v", err)
	}
	if err := pluginRegistry.InitializePlugins(context.Background()); err != nil {
		t.Fatalf("InitializePlugins() error = %v", err)
	}

	registry := tools.NewRegistry()
	registerPluginTools(registry, pluginRegistry)

	if _, ok := registry.LookupDefinitionForRoute("plugin_tool", tools.ToolRouteKindChat); ok {
		t.Fatal("plugin tool should be hidden from chat route")
	}

	def, ok := registry.LookupDefinitionForRoute("plugin_tool", tools.ToolRouteKindAgent)
	if !ok {
		t.Fatal("plugin tool should be visible for agent route")
	}
	if def.RiskLevel != "high" {
		t.Fatalf("risk_level = %q, want high", def.RiskLevel)
	}
	if len(def.VisibilityAllowlist) != 1 || def.VisibilityAllowlist[0] != string(tools.ToolRouteKindAgent) {
		t.Fatalf("visibility_allowlist = %+v, want [agent]", def.VisibilityAllowlist)
	}

	adapter := &mgmtToolAdapter{registry: registry}
	listed, err := adapter.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed tools = %d, want 1", len(listed))
	}
	if listed[0].RiskLevel != "high" {
		t.Fatalf("listed risk_level = %q, want high", listed[0].RiskLevel)
	}
	if len(listed[0].VisibilityAllowlist) != 1 || listed[0].VisibilityAllowlist[0] != string(tools.ToolRouteKindAgent) {
		t.Fatalf("listed visibility_allowlist = %+v, want [agent]", listed[0].VisibilityAllowlist)
	}
}

type recursiveBootstrapPluginPayload struct {
	Name string                           `json:"name"`
	Self *recursiveBootstrapPluginPayload `json:"self,omitempty"`
}

func TestRegisterPluginToolsSanitizesRecursiveResults(t *testing.T) {
	pluginRegistry := plugin.NewRegistry()
	payload := &recursiveBootstrapPluginPayload{Name: "root"}
	payload.Self = payload
	if err := pluginRegistry.RegisterNativePlugin(&bootstrapNativePlugin{
		id: "plugin-recursive-test",
		tool: bootstrapPluginTool{
			name:   "plugin_recursive",
			result: payload,
		},
	}); err != nil {
		t.Fatalf("RegisterNativePlugin() error = %v", err)
	}
	if err := pluginRegistry.InitializePlugins(context.Background()); err != nil {
		t.Fatalf("InitializePlugins() error = %v", err)
	}

	registry := tools.NewRegistry()
	registerPluginTools(registry, pluginRegistry)
	tool := registry.Get("plugin_recursive")
	if tool == nil {
		t.Fatal("plugin_recursive tool not registered")
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	asMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result = %#v, want sanitized map", result)
	}
	if got := asMap["name"]; got != "root" {
		t.Fatalf("name = %v, want root", got)
	}
	if got := asMap["self"]; got != "[circular payload omitted]" {
		t.Fatalf("self = %v, want circular marker", got)
	}
}
