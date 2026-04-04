package server

import (
	"context"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (h *ChatHandler) GetDeferredToolExposureStore() *tools.DeferredToolExposureStore {
	if h == nil {
		return nil
	}
	return h.deferredToolExposure
}

func (h *ChatHandler) ConfigureToolSearchRuntime(workspaceDir string, cfg *config.Config) {
	if h == nil || h.toolRegistry == nil {
		return
	}
	searchTool := tools.GetToolSearchTool(h.toolRegistry)
	if searchTool == nil {
		return
	}
	searchTool.SetToolPolicyResolver(h.toolPolicyResolver)
	searchTool.SetDeferredExposureStore(h.deferredToolExposure)
	searchTool.SetRuntimeInfoSource(func(sessionID string) tools.ToolSearchRuntimeInfo {
		return h.toolSearchRuntimeInfo(sessionID)
	})
	if cfg != nil {
		searchTool.SetAgentsConfig(&cfg.Agents)
	}
	if strings.TrimSpace(workspaceDir) == "" {
		h.toolSearchSkillExposureStamp = nil
		return
	}
	exposure := skillmanifest.SharedSkillExposureManager(workspaceDir)
	if h.settingsHandler != nil {
		exposure.SetDynamicExposureEnabledFunc(h.settingsHandler.GetSkillDynamicExposure)
	}
	searchTool.SetSkillExposureManager(exposure)
	h.toolSearchSkillExposureStamp = func() string {
		snapshot, err := exposure.Snapshot()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(snapshot.Stamp)
	}
}

func (h *ChatHandler) toolSearchRuntimeInfo(sessionID string) tools.ToolSearchRuntimeInfo {
	state := memory.ConversationCommandState{}
	if h != nil && h.store != nil && strings.TrimSpace(sessionID) != "" {
		state = h.conversationCommandStateOrDefault(context.Background(), sessionID)
	}
	return tools.ToolSearchRuntimeInfo{
		PromptPolicyHash:    h.resolvePromptPolicy().Hash,
		WebSearchEnabled:    state.WebSearchEnabled,
		DeepResearchEnabled: state.DeepResearchEnabled,
	}
}

func (h *ChatHandler) applyToolSearchSurfaceSelection(policyReq tools.ToolPolicyRequest, webSearchEnabled, deepResearchEnabled *bool, selection chatToolSurfaceSelection) chatToolSurfaceSelection {
	if h == nil || selection.NativeMode == chatNativeToolSurfaceModeClarifyNone {
		return selection
	}
	selection.NativeDefs = h.ensureToolSearchVisible(policyReq, selection.NativeDefs)

	sessionID := strings.TrimSpace(policyReq.SessionID)
	if sessionID == "" || h.deferredToolExposure == nil {
		return selection
	}

	registryVersion := uint64(0)
	if h.toolRegistry != nil {
		registryVersion = h.toolRegistry.Version()
	}
	skillStamp := ""
	if h.toolSearchSkillExposureStamp != nil {
		skillStamp = h.toolSearchSkillExposureStamp()
	}
	if h.deferredToolExposure.InvalidateIfStale(sessionID, registryVersion, h.resolvePromptPolicy().Hash, skillStamp) {
		selection.PromptCacheUnsafe = true
		return selection
	}

	state, ok := h.deferredToolExposure.Snapshot(sessionID)
	if !ok {
		return selection
	}
	selection.PromptCacheUnsafe = true
	selection.NativeDefs = h.overlayDeferredToolExposure(policyReq, selection.NativeDefs, state, webSearchEnabled, deepResearchEnabled)
	return selection
}

func (h *ChatHandler) ensureToolSearchVisible(policyReq tools.ToolPolicyRequest, defs []tools.ToolDefinition) []tools.ToolDefinition {
	if h == nil {
		return defs
	}
	available := h.toolDefinitionsForPolicy(policyReq)
	toolSearch := filterToolDefsToNames(available, "tool_search")
	if len(toolSearch) == 0 {
		return defs
	}
	return mergeToolDefsByName(defs, toolSearch)
}

func (h *ChatHandler) overlayDeferredToolExposure(policyReq tools.ToolPolicyRequest, defs []tools.ToolDefinition, state tools.DeferredToolExposureState, webSearchEnabled, deepResearchEnabled *bool) []tools.ToolDefinition {
	searchReq := policyReq
	searchReq.SkipDefaultChatDirectAllowlist = true

	available := h.toolDefinitionsForPolicy(searchReq)
	available = applyWebSearchPreference(available, webSearchEnabled)
	available = applyDeepResearchPreference(available, deepResearchEnabled)
	byName := make(map[string]tools.ToolDefinition, len(available))
	for _, def := range available {
		byName[strings.ToLower(strings.TrimSpace(def.Name))] = def
	}

	merged := cloneToolDefs(defs)
	if state.NeedExec {
		filtered := make([]tools.ToolDefinition, 0, len(merged))
		for _, def := range merged {
			if strings.EqualFold(strings.TrimSpace(def.Name), "bash") {
				continue
			}
			filtered = append(filtered, def)
		}
		merged = filtered
		if execDef, ok := byName["exec"]; ok {
			merged = mergeToolDefsByName(merged, []tools.ToolDefinition{execDef})
		}
	}
	if state.NeedAgentTools {
		if def, ok := byName["agents_list"]; ok {
			merged = mergeToolDefsByName(merged, []tools.ToolDefinition{def})
		}
		if def, ok := byName["subagents"]; ok {
			merged = mergeToolDefsByName(merged, []tools.ToolDefinition{def})
		}
	}
	for _, name := range state.ActivatedTools {
		if def, ok := byName[strings.ToLower(strings.TrimSpace(name))]; ok {
			merged = mergeToolDefsByName(merged, []tools.ToolDefinition{def})
		}
	}
	merged = h.ensureToolSearchVisible(policyReq, merged)
	return sortToolDefsByName(merged)
}

func sameToolDefNames(left, right []tools.ToolDefinition) bool {
	if len(left) != len(right) {
		return false
	}
	leftNames := make([]string, 0, len(left))
	rightNames := make([]string, 0, len(right))
	for _, def := range left {
		leftNames = append(leftNames, strings.ToLower(strings.TrimSpace(def.Name)))
	}
	for _, def := range right {
		rightNames = append(rightNames, strings.ToLower(strings.TrimSpace(def.Name)))
	}
	sort.Strings(leftNames)
	sort.Strings(rightNames)
	for i := range leftNames {
		if leftNames[i] != rightNames[i] {
			return false
		}
	}
	return true
}
