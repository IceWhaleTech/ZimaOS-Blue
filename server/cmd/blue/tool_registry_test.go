package main

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRegisterServerToolRegistry_IncludesShellSurface(t *testing.T) {
	registry := tools.NewRegistry()
	cfg := &config.Config{}

	registerServerToolRegistry(registry, cfg, t.TempDir())

	if tools.GetExecTool(registry) == nil {
		t.Fatalf("expected exec tool to be registered, got %v", registry.List())
	}
	if def, ok := registry.LookupDefinition("exec"); !ok || def.Name != "exec" {
		t.Fatalf("expected visible exec overlay definition, got ok=%v def=%+v", ok, def)
	}
	if def, ok := registry.LookupDefinitionForRoute("bash", tools.ToolRouteKindChat); !ok || def.Name != "bash" {
		t.Fatalf("expected public bash definition for chat route, got ok=%v def=%+v", ok, def)
	}
}
