package tools

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

type panicAfterFirstDefinitionTool struct {
	def   ToolDefinition
	calls int
}

func (t *panicAfterFirstDefinitionTool) Definition() ToolDefinition {
	t.calls++
	if t.calls > 1 {
		panic("definition should not be called after registration")
	}
	return t.def
}

func (t *panicAfterFirstDefinitionTool) Execute(context.Context, map[string]interface{}) (interface{}, error) {
	return "ok", nil
}

func writeToolSearchConflictSkill(t *testing.T, dir, entryDir, manifestName, description string) string {
	t.Helper()

	skillDir := filepath.Join(dir, entryDir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := `---
name: ` + manifestName + `
version: 1.0.0
description: ` + description + `
invocation: blue ` + manifestName + `
examples:
  - blue ` + manifestName + `
capability_tags:
  - test
interaction_mode: stateless
card_support: none
---
# ` + manifestName + `
`
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return path
}

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

func TestResolveToolSearchLoadedDefinitions_DefaultChatAllowlistUsesLoadedSubset(t *testing.T) {
	resolver := NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	})
	searchDefs := []ToolDefinition{
		{Name: "custom_tool"},
		{Name: "read"},
		{Name: "web_query"},
	}

	loaded := resolveToolSearchLoadedDefinitions(searchDefs, resolver, ToolPolicyRequest{
		RouteKind: ToolRouteKindChat,
	})

	if len(loaded) != 2 {
		t.Fatalf("loaded len = %d, want 2; defs=%+v", len(loaded), loaded)
	}
	if loaded[0].Name != "read" || loaded[1].Name != "web_query" {
		t.Fatalf("loaded names = [%s %s], want [read web_query]", loaded[0].Name, loaded[1].Name)
	}
}

func TestResolveToolSearchLoadedDefinitions_ConfiguredPolicyBypassesDefaultAllowlist(t *testing.T) {
	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.ToolCalling.Allow = []string{"custom_tool", "read", "web_query"}
	resolver := NewToolPolicyResolver(cfg)
	searchDefs := []ToolDefinition{
		{Name: "custom_tool"},
		{Name: "read"},
		{Name: "web_query"},
	}

	loaded := resolveToolSearchLoadedDefinitions(searchDefs, resolver, ToolPolicyRequest{
		RouteKind: ToolRouteKindChat,
	})

	if len(loaded) != len(searchDefs) {
		t.Fatalf("loaded len = %d, want %d", len(loaded), len(searchDefs))
	}
	for idx := range loaded {
		if loaded[idx].Name != searchDefs[idx].Name {
			t.Fatalf("loaded[%d] = %q, want %q", idx, loaded[idx].Name, searchDefs[idx].Name)
		}
	}
}

func TestToolSearchTool_ToolKindsSkipSkillConflictSnapshot(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeToolSearchConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "team-browser", "browser", "Browser from workspace agents")
	writeToolSearchConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "browser", "browser", "Browser duplicate from workspace agents")

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
	searchTool.SetSkillExposureManager(skillmanifest.SharedSkillExposureManager(workspaceDir))

	ctx := WithSessionID(context.Background(), "conv-tool-kind-only")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{
		"query": "read",
		"kinds": []string{"tool"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 1 || result.Matches[0].Name != "read" || result.Matches[0].Kind != "tool" {
		t.Fatalf("matches = %+v, want one tool read match", result.Matches)
	}
}

func TestToolSearchTool_AgentKindsSkipSkillConflictSnapshot(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeToolSearchConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "team-browser", "browser", "Browser from workspace agents")
	writeToolSearchConflictSkill(t, filepath.Join(workspaceDir, ".agents", "skills"), "browser", "browser", "Browser duplicate from workspace agents")

	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)

	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.Agents.List = []config.AgentConfig{
		{
			ID:          "delegate_helper",
			Enabled:     true,
			Description: "Delegate helper agent",
			Subagents: config.AgentSubagentPolicyConfig{
				Enabled: true,
			},
		},
	}
	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(cfg))
	searchTool.SetAgentsConfig(&cfg.Agents)
	searchTool.SetSkillExposureManager(skillmanifest.SharedSkillExposureManager(workspaceDir))

	ctx := WithSessionID(context.Background(), "conv-agent-kind-only")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{
		"query": "delegate",
		"kinds": []string{"agent"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 1 || result.Matches[0].Name != "delegate_helper" || result.Matches[0].Kind != "agent" {
		t.Fatalf("matches = %+v, want one agent delegate_helper match", result.Matches)
	}
}

func TestToolSearchTool_AgentKindsWithoutSelectionSkipToolSurfaceResolution(t *testing.T) {
	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.Register(&panicAfterFirstDefinitionTool{
		def: ToolDefinition{
			Name:        "poison_definition",
			Description: "Panics if tool_search tries to resolve full tool surfaces.",
		},
	})

	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.Agents.List = []config.AgentConfig{
		{
			ID:          "delegate_helper",
			Enabled:     true,
			Description: "Delegate helper agent",
			Subagents: config.AgentSubagentPolicyConfig{
				Enabled: true,
			},
		},
	}
	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(cfg))
	searchTool.SetAgentsConfig(&cfg.Agents)

	ctx := WithSessionID(context.Background(), "conv-agent-kind-skip-tool-surface")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Execute panicked while resolving tool surfaces for agent-only query: %v", recovered)
		}
	}()

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{
		"query": "delegate",
		"kinds": []string{"agent"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 1 || result.Matches[0].Name != "delegate_helper" || result.Matches[0].Kind != "agent" {
		t.Fatalf("matches = %+v, want one agent delegate_helper match", result.Matches)
	}
	if result.Activated.AgentTools {
		t.Fatalf("activated.AgentTools = %v, want false without explicit selection", result.Activated.AgentTools)
	}
}

func TestToolSearchTool_AgentSelectionUsesTargetedSupportToolLookup(t *testing.T) {
	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.Register(&panicAfterFirstDefinitionTool{
		def: ToolDefinition{
			Name:        "poison_definition",
			Description: "Panics if tool_search resolves the full tool surface instead of targeted support tools.",
		},
	})
	registry.ExposeDefinition(ToolDefinition{
		Name:        "agents_list",
		Description: "List available agents.",
	})
	registry.ExposeDefinition(ToolDefinition{
		Name:        "subagents",
		Description: "Spawn and manage subagents.",
	})

	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.Agents.List = []config.AgentConfig{
		{
			ID:          "delegate_helper",
			Enabled:     true,
			Description: "Delegate helper agent",
			Subagents: config.AgentSubagentPolicyConfig{
				Enabled: true,
			},
		},
	}
	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(cfg))
	searchTool.SetAgentsConfig(&cfg.Agents)

	ctx := WithSessionID(context.Background(), "conv-agent-kind-targeted-support")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Execute panicked while resolving full tool surface for selected agent query: %v", recovered)
		}
	}()

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{
		"query": "select:delegate_helper",
		"kinds": []string{"agent"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 1 || result.Matches[0].Name != "delegate_helper" || result.Matches[0].Kind != "agent" {
		t.Fatalf("matches = %+v, want one selected agent delegate_helper match", result.Matches)
	}
	if !result.Activated.AgentTools {
		t.Fatalf("activated.AgentTools = %v, want true for selected agent", result.Activated.AgentTools)
	}
}

func TestToolSearchTool_SkillKindsWithoutSelectionSkipToolSurfaceResolution(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	skillDir := filepath.Join(workspaceDir, ".claude", "skills", "pkg_helper")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	skillDoc := `---
name: pkg_helper
description: Inspect packages in the workspace.
invocation: "blue pkg_helper path=pkg"
---

Inspect packages in the workspace.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillDoc), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.Register(&panicAfterFirstDefinitionTool{
		def: ToolDefinition{
			Name:        "poison_definition",
			Description: "Panics if tool_search resolves full tool surfaces for skill-only search.",
		},
	})

	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))
	searchTool.SetSkillExposureManager(skillmanifest.SharedSkillExposureManager(workspaceDir))

	ctx := WithSessionID(context.Background(), "conv-skill-kind-skip-tool-surface")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Execute panicked while resolving tool surfaces for skill-only query: %v", recovered)
		}
	}()

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{
		"query": "pkg",
		"kinds": []string{"skill"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 1 || result.Matches[0].Name != "pkg_helper" || result.Matches[0].Kind != "skill" {
		t.Fatalf("matches = %+v, want one skill pkg_helper match", result.Matches)
	}
	if result.Activated.Exec {
		t.Fatalf("activated.Exec = %v, want false without explicit selection", result.Activated.Exec)
	}
}

func TestToolSearchTool_SkillSelectionUsesTargetedSupportToolLookup(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	skillDir := filepath.Join(workspaceDir, ".claude", "skills", "pkg_helper")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	skillDoc := `---
name: pkg_helper
description: Inspect packages in the workspace.
invocation: "blue pkg_helper path=pkg"
---

Inspect packages in the workspace.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillDoc), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	registry := NewRegistry()
	searchTool := NewToolSearchTool(registry)
	registry.Register(searchTool)
	registry.Register(&panicAfterFirstDefinitionTool{
		def: ToolDefinition{
			Name:        "poison_definition",
			Description: "Panics if tool_search resolves the full tool surface instead of targeted exec lookup.",
		},
	})
	registry.ExposeDefinition(ToolDefinition{
		Name:        "exec",
		Description: "Execute skill and shell commands.",
	})

	searchTool.SetToolPolicyResolver(NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))
	searchTool.SetSkillExposureManager(skillmanifest.SharedSkillExposureManager(workspaceDir))

	ctx := WithSessionID(context.Background(), "conv-skill-kind-targeted-support")
	ctx = WithRouteKind(ctx, ToolRouteKindChat)

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Execute panicked while resolving full tool surface for selected skill query: %v", recovered)
		}
	}()

	resultAny, err := searchTool.Execute(ctx, map[string]interface{}{
		"query": "select:pkg_helper",
		"kinds": []string{"skill"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	result := resultAny.(ToolSearchResult)
	if len(result.Matches) != 1 || result.Matches[0].Name != "pkg_helper" || result.Matches[0].Kind != "skill" {
		t.Fatalf("matches = %+v, want one selected skill pkg_helper match", result.Matches)
	}
	if !result.Activated.Exec {
		t.Fatalf("activated.Exec = %v, want true for selected skill", result.Activated.Exec)
	}
}

func legacyToolSearchCapabilityKeysForTest(capability toolSearchCapability) []string {
	keys := []string{
		normalizeToolSearchKey(capability.ID),
		normalizeToolSearchKey(capability.Name),
	}
	for _, alias := range capability.Aliases {
		if key := normalizeToolSearchKey(alias); key != "" {
			keys = append(keys, key)
		}
	}
	if key := normalizeToolSearchKey(capability.CanonicalSkill); key != "" {
		keys = append(keys, key)
	}
	return uniqueNormalizedToolSearchNames(keys)
}

func legacyMatchedToolSearchSelectionsForTest(capability toolSearchCapability, selected map[string]struct{}) []string {
	if len(selected) == 0 {
		return nil
	}
	keys := legacyToolSearchCapabilityKeysForTest(capability)
	matched := make([]string, 0, 2)
	for _, key := range keys {
		if _, ok := selected[key]; ok {
			matched = append(matched, "selected:"+key)
		}
	}
	return matched
}

func legacyScoreToolSearchCapabilityForTest(capability toolSearchCapability, parsed toolSearchParsedQuery, selectReasons []string) (float64, []string, bool) {
	searchText := strings.ToLower(strings.Join([]string{
		capability.ID,
		capability.Name,
		strings.Join(capability.Aliases, " "),
		strings.Join(capability.SearchHints, " "),
		strings.Join(capability.Tags, " "),
		capability.Description,
		capability.Invocation,
		capability.Body,
	}, " "))

	for _, required := range parsed.requiredTerms {
		if required == "" {
			continue
		}
		if !strings.Contains(searchText, required) {
			return 0, nil, false
		}
	}

	reasons := make([]string, 0, len(selectReasons)+len(parsed.terms))
	score := 0.0
	if len(selectReasons) > 0 {
		score += 100
		reasons = append(reasons, selectReasons...)
	}

	for _, term := range parsed.terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		termScore := 0.0
		termReasons := make([]string, 0, 2)
		switch {
		case strings.EqualFold(capability.Name, term), strings.EqualFold(capability.ID, term):
			termScore += 12
			termReasons = append(termReasons, "exact_name")
		case strings.HasPrefix(strings.ToLower(capability.Name), term), strings.HasPrefix(strings.ToLower(capability.ID), term):
			termScore += 8
			termReasons = append(termReasons, "prefix_name")
		case strings.Contains(strings.ToLower(capability.Name), term), strings.Contains(strings.ToLower(capability.ID), term):
			termScore += 6
			termReasons = append(termReasons, "name")
		}

		for _, alias := range capability.Aliases {
			if strings.Contains(strings.ToLower(alias), term) {
				termScore += 4
				termReasons = append(termReasons, "alias")
				break
			}
		}
		for _, hint := range capability.SearchHints {
			if strings.Contains(strings.ToLower(hint), term) {
				termScore += 3
				termReasons = append(termReasons, "hint")
				break
			}
		}
		if strings.Contains(strings.ToLower(capability.Description), term) {
			termScore += 2
			termReasons = append(termReasons, "description")
		}
		if strings.Contains(strings.ToLower(capability.Invocation), term) || strings.Contains(strings.ToLower(capability.Body), term) {
			termScore += 1
			termReasons = append(termReasons, "body")
		}
		if termScore == 0 {
			return 0, nil, false
		}
		score += termScore
		reasons = append(reasons, term+":"+strings.Join(uniqueNormalizedToolSearchNames(termReasons), "+"))
	}

	if score == 0 {
		return 0, nil, false
	}
	return score, reasons, true
}

func TestPrepareToolSearchCapability_PreservesLegacySelectionAndScoring(t *testing.T) {
	raw := toolSearchCapability{
		ID:             "web_query",
		Name:           "Web Query",
		Kind:           "tool",
		Description:    "Search the web for official latest references.",
		Aliases:        []string{"web-search", "browse"},
		SearchHints:    []string{"official docs", "latest sources"},
		Tags:           []string{"web", "reference"},
		Invocation:     "blue web_query query=openai",
		Body:           "Use when you need latest official references.",
		CanonicalSkill: "browser",
	}
	parsed := toolSearchParsedQuery{
		terms:         []string{"web", "official"},
		requiredTerms: []string{"latest"},
	}
	selected := map[string]struct{}{
		"webquery": {},
		"browser":  {},
	}

	legacySelectReasons := legacyMatchedToolSearchSelectionsForTest(raw, selected)
	legacyScore, legacyWhy, legacyOK := legacyScoreToolSearchCapabilityForTest(raw, parsed, legacySelectReasons)

	prepared := prepareToolSearchCapability(raw)
	gotKeys := toolSearchCapabilityKeys(prepared)
	wantKeys := legacyToolSearchCapabilityKeysForTest(raw)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("keys = %+v, want %+v", gotKeys, wantKeys)
	}

	gotSelectReasons := matchedToolSearchSelections(prepared, selected)
	if !reflect.DeepEqual(gotSelectReasons, legacySelectReasons) {
		t.Fatalf("select reasons = %+v, want %+v", gotSelectReasons, legacySelectReasons)
	}

	gotScore, gotWhy, gotOK := scoreToolSearchCapability(prepared, parsed, gotSelectReasons)
	if gotOK != legacyOK {
		t.Fatalf("score ok = %v, want %v", gotOK, legacyOK)
	}
	if gotScore != legacyScore {
		t.Fatalf("score = %v, want %v", gotScore, legacyScore)
	}
	if !reflect.DeepEqual(gotWhy, legacyWhy) {
		t.Fatalf("why = %+v, want %+v", gotWhy, legacyWhy)
	}
}

func TestNewToolSearchNameSet_NormalizesCaseAndWhitespace(t *testing.T) {
	set := newToolSearchNameSet([]string{" Exec ", "pkg_helper", "EXEC", "", " Delegate_Helper "})

	if len(set) != 3 {
		t.Fatalf("set len = %d, want 3", len(set))
	}
	for _, want := range []string{"exec", "pkg_helper", "delegate_helper"} {
		if _, ok := set[want]; !ok {
			t.Fatalf("set missing %q: %+v", want, set)
		}
	}
	if !toolSearchNameSetContains(set, " exec ") {
		t.Fatal("expected set to match exec case-insensitively")
	}
	if !toolSearchNameSetContains(set, "delegate_helper") {
		t.Fatal("expected set to preserve exact punctuation-sensitive matching")
	}
	if toolSearchNameSetContains(set, "delegate-helper") {
		t.Fatal("expected set not to normalize punctuation beyond legacy behavior")
	}
	if toolSearchNameSetContains(set, "missing") {
		t.Fatal("expected set to exclude missing name")
	}
}

func legacyToolSearchDedupeKeysForTest(name, id, canonical string) []string {
	keys := []string{
		"name:" + normalizeToolSearchKey(name),
	}
	if normalizedID := normalizeToolSearchKey(id); normalizedID != "" {
		keys = append(keys, "id:"+normalizedID)
	}
	if normalizedCanonical := normalizeToolSearchKey(canonical); normalizedCanonical != "" {
		keys = append(keys, "canonical:"+normalizedCanonical)
	}
	return keys
}

func legacyToolSearchDedupeTakenForTest(taken map[string]struct{}, name, id, canonical string) bool {
	for _, key := range legacyToolSearchDedupeKeysForTest(name, id, canonical) {
		if key == "" {
			continue
		}
		if _, ok := taken[key]; ok {
			return true
		}
	}
	return false
}

func legacyToolSearchMarkDedupeForTest(taken map[string]struct{}, name, id, canonical string) {
	for _, key := range legacyToolSearchDedupeKeysForTest(name, id, canonical) {
		if key == "" {
			continue
		}
		taken[key] = struct{}{}
	}
}

func TestToolSearchDedupeState_MatchesLegacyBehavior(t *testing.T) {
	legacyTaken := map[string]struct{}{}
	gotTaken := map[string]struct{}{}

	cases := []struct {
		name      string
		id        string
		canonical string
	}{
		{name: "Web Query", id: "web_query", canonical: "browser"},
		{name: "web-query", id: "web-query", canonical: ""},
		{name: "Deep Research", id: "deep_research", canonical: "research"},
	}

	for idx, tc := range cases {
		legacyHas := legacyToolSearchDedupeTakenForTest(legacyTaken, tc.name, tc.id, tc.canonical)
		gotHas := toolSearchDedupeContainsAny(gotTaken, tc.name, tc.id, tc.canonical)
		if gotHas != legacyHas {
			t.Fatalf("case %d contains = %v, want %v", idx, gotHas, legacyHas)
		}
		legacyToolSearchMarkDedupeForTest(legacyTaken, tc.name, tc.id, tc.canonical)
		toolSearchMarkDedupeKeys(gotTaken, tc.name, tc.id, tc.canonical)
	}

	if !reflect.DeepEqual(gotTaken, legacyTaken) {
		t.Fatalf("taken = %+v, want %+v", gotTaken, legacyTaken)
	}

	if !toolSearchDedupeContainsAny(gotTaken, "web query", "different", "") {
		t.Fatal("expected normalized name match to be treated as duplicate")
	}
	if !toolSearchDedupeContainsAny(gotTaken, "", "deep_research", "") {
		t.Fatal("expected normalized id match to be treated as duplicate")
	}
	if !toolSearchDedupeContainsAny(gotTaken, "", "", "research") {
		t.Fatal("expected canonical match to be treated as duplicate")
	}
}
