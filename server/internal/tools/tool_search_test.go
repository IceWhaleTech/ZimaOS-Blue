package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

func TestToolSearchTool_SchemaAcceptsStringSelectCompat(t *testing.T) {
	searchTool := NewToolSearchTool(NewRegistry())
	def := searchTool.Definition()

	err := ValidateToolArguments(def.Parameters, map[string]interface{}{
		"query":  "process helpers",
		"select": "process,read",
	})
	if err != nil {
		t.Fatalf("ValidateToolArguments() error = %v, want nil for string select compat", err)
	}
}

func TestToolSearchTool_SelectAndRequiredTermsActivateDeferredTool(t *testing.T) {
	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.ExposeDefinition(ToolDefinition{
		Name:        "process",
		Description: "Inspect process session status and background jobs.",
	})

	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(cfg))
	store := NewDeferredToolExposureStore(time.Minute)
	searchTool.SetDeferredExposureStore(store)

	ctx := WithSessionID(context.Background(), "conv-tool-search")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{
		"query":       "+process select:process",
		"max_results": 10,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	result, ok := resultAny.(ToolSearchResult)
	if !ok {
		t.Fatalf("Execute result type = %T, want ToolSearchResult", resultAny)
	}
	if len(result.Matches) == 0 {
		t.Fatalf("expected at least one match, got %+v", result)
	}
	if result.Matches[0].Name != "process" || result.Matches[0].Kind != "tool" {
		t.Fatalf("first match = %+v, want tool process", result.Matches[0])
	}
	if len(result.Activated.Tools) != 1 || result.Activated.Tools[0] != "process" {
		t.Fatalf("activated.tools = %+v, want [process]", result.Activated.Tools)
	}

	state, ok := store.Snapshot("conv-tool-search")
	if !ok {
		t.Fatal("expected deferred exposure snapshot")
	}
	if len(state.ActivatedTools) != 1 || state.ActivatedTools[0] != "process" {
		t.Fatalf("state.ActivatedTools = %+v, want [process]", state.ActivatedTools)
	}
}

func TestToolSearchTool_RespectsPolicyAndRouteVisibility(t *testing.T) {
	t.Run("deny_hidden_tool", func(t *testing.T) {
		registry := NewRegistry()
		searchTool := NewToolSearchTool(registry)
		registry.Register(searchTool)
		registry.ExposeDefinition(ToolDefinition{
			Name:        "process",
			Description: "Inspect background process status.",
		})

		cfg := &config.Config{
			ToolCalling: *config.DefaultToolCallingConfig(),
			Agents:      *config.DefaultAgentsConfig(),
		}
		cfg.ToolCalling.Deny = []string{"process"}
		searchTool.SetToolPolicyResolver(NewToolPolicyResolver(cfg))

		ctx := WithSessionID(context.Background(), "conv-deny")
		ctx = WithRouteKind(ctx, ToolRouteKindChat)

		resultAny, err := searchTool.Execute(ctx, map[string]interface{}{"query": "process"})
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		result := resultAny.(ToolSearchResult)
		if len(result.Matches) != 0 {
			t.Fatalf("expected denied tool to stay hidden, got %+v", result.Matches)
		}
	})

	t.Run("route_hidden_tool", func(t *testing.T) {
		registry := NewRegistry()
		searchTool := NewToolSearchTool(registry)
		registry.Register(searchTool)
		registry.ExposeDefinition(ToolDefinition{
			Name:                "agent_only_tool",
			Description:         "Agent-only worker capability",
			VisibilityAllowlist: []string{string(ToolRouteKindAgent)},
		})
		searchTool.SetToolPolicyResolver(NewToolPolicyResolver(&config.Config{
			ToolCalling: *config.DefaultToolCallingConfig(),
			Agents:      *config.DefaultAgentsConfig(),
		}))

		ctx := WithSessionID(context.Background(), "conv-route")
		ctx = WithRouteKind(ctx, ToolRouteKindChat)

		resultAny, err := searchTool.Execute(ctx, map[string]interface{}{"query": "agent_only_tool"})
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		result := resultAny.(ToolSearchResult)
		if len(result.Matches) != 0 {
			t.Fatalf("expected route-hidden tool to stay hidden, got %+v", result.Matches)
		}
	})
}

func TestToolSearchTool_DedupesToolAndSkillByName(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	skillDir := filepath.Join(workspaceDir, ".claude", "skills", "read")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	skillDoc := `---
name: read
description: Read project files through a skill wrapper.
invocation: "blue read path=README.md"
---

Read files from the workspace.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillDoc), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.ExposeDefinition(ToolDefinition{
		Name:        "read",
		Description: "Read workspace files directly.",
	})

	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))
	exposure := skillmanifest.SharedSkillExposureManager(workspaceDir)
	searchTool.SetSkillExposureManager(exposure)

	ctx := WithSessionID(context.Background(), "conv-dedupe")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{"query": "select:read", "max_results": 10})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 1 {
		t.Fatalf("expected one deduped match, got %+v", result.Matches)
	}
	if result.Matches[0].Kind != "tool" || result.Matches[0].Name != "read" {
		t.Fatalf("deduped match = %+v, want tool read", result.Matches[0])
	}
}

func TestToolSearchTool_DormantSkillMatchesWithoutActivatingExec(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	skillDir := filepath.Join(workspaceDir, ".claude", "skills", "pkg_helper")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	skillDoc := `---
name: pkg_helper
description: Inspect files under pkg when the workspace path is active.
paths:
  - pkg/**
os:
  - ` + runtime.GOOS + `
invocation: "blue pkg_helper path=pkg"
---

Inspect files under pkg when activated.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillDoc), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.ExposeDefinition(ToolDefinition{
		Name:        "exec",
		Description: "Execute skill and shell commands.",
	})

	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))
	exposure := skillmanifest.SharedSkillExposureManager(workspaceDir)
	exposure.SetDynamicExposureEnabledFunc(func() bool { return true })
	searchTool.SetSkillExposureManager(exposure)
	store := NewDeferredToolExposureStore(time.Minute)
	searchTool.SetDeferredExposureStore(store)

	ctx := WithSessionID(context.Background(), "conv-dormant")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{
		"query":       "select:pkg_helper",
		"max_results": 10,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 1 {
		t.Fatalf("expected one dormant skill match, got %+v", result.Matches)
	}
	match := result.Matches[0]
	if match.Kind != "skill" || match.Name != "pkg_helper" {
		t.Fatalf("match = %+v, want dormant pkg_helper skill", match)
	}
	if match.Activation != "dormant" {
		t.Fatalf("activation = %q, want dormant", match.Activation)
	}
	if result.Activated.Exec {
		t.Fatalf("activated.exec = %v, want false for dormant skill", result.Activated.Exec)
	}
	if len(result.Activated.Skipped) != 1 || result.Activated.Skipped[0].Reason != "skill_dormant" {
		t.Fatalf("activated.skipped = %+v, want skill_dormant", result.Activated.Skipped)
	}
	if _, ok := store.Snapshot("conv-dormant"); ok {
		t.Fatal("expected dormant skill selection to avoid deferred activation state")
	}
}

func TestToolSearchTool_HidesAnalyzeFromUserFacingMatches(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	skillDir := filepath.Join(workspaceDir, ".claude", "skills", "analyze")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	skillDoc := `---
name: analyze
description: Analyze multiple links and synthesize a report.
invocation: "blue analyze topic=report --json"
---

Analyze links and produce a report.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillDoc), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.ExposeDefinition(ToolDefinition{
		Name:        "analyze",
		Description: "Analyze URLs, files, and reports.",
	})
	registry.ExposeDefinition(ToolDefinition{
		Name:        "exec",
		Description: "Execute skill and shell commands.",
	})

	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))
	exposure := skillmanifest.SharedSkillExposureManager(workspaceDir)
	searchTool.SetSkillExposureManager(exposure)

	ctx := WithSessionID(context.Background(), "conv-hide-analyze")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{"query": "select:analyze", "max_results": 10})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 0 {
		t.Fatalf("expected analyze to stay hidden from tool_search, got %+v", result.Matches)
	}
	if len(result.Activated.Tools) != 0 || result.Activated.Exec {
		t.Fatalf("expected no analyze activation, got %+v", result.Activated)
	}
}

func TestToolSearchTool_HidesLegacyWebAliasFromUserFacingMatches(t *testing.T) {
	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.ExposeDefinition(ToolDefinition{
		Name:        "web",
		Description: "Legacy alias for web_query.",
	})
	registry.ExposeDefinition(ToolDefinition{
		Name:        "web_query",
		Description: "Search the web for latest sources.",
	})

	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))
	searchTool.SetRuntimeInfoSource(func(string) ToolSearchRuntimeInfo {
		return ToolSearchRuntimeInfo{WebSearchEnabled: true}
	})

	ctx := WithSessionID(context.Background(), "conv-hide-web-alias")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{"query": "select:web", "max_results": 10})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 0 {
		t.Fatalf("expected legacy web alias to stay hidden from tool_search, got %+v", result.Matches)
	}
	if len(result.Activated.Tools) != 0 || result.Activated.Exec {
		t.Fatalf("expected no legacy web activation, got %+v", result.Activated)
	}

	canonicalAny, err := searchTool.Execute(ctx, map[string]interface{}{"query": "select:web_query", "max_results": 10})
	if err != nil {
		t.Fatalf("Execute canonical returned error: %v", err)
	}
	canonical := canonicalAny.(ToolSearchResult)
	if len(canonical.Matches) != 1 {
		t.Fatalf("expected canonical web_query match, got %+v", canonical.Matches)
	}
	if canonical.Matches[0].Name != "web_query" || canonical.Matches[0].Kind != "tool" {
		t.Fatalf("canonical match = %+v, want tool web_query", canonical.Matches[0])
	}
	if canonical.Matches[0].Availability != "loaded" {
		t.Fatalf("canonical availability = %q, want loaded", canonical.Matches[0].Availability)
	}
	if len(canonical.Activated.Tools) != 0 {
		t.Fatalf("canonical activated.tools = %+v, want no deferred activation for loaded web_query", canonical.Activated.Tools)
	}
	if len(canonical.Activated.Skipped) != 1 || canonical.Activated.Skipped[0].Reason != "already_loaded" {
		t.Fatalf("canonical activated.skipped = %+v, want already_loaded", canonical.Activated.Skipped)
	}
}
