package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

const (
	defaultToolSearchMaxResults = 5
	defaultDeferredToolTTL      = 10 * time.Minute
)

type ToolSearchRuntimeInfo struct {
	PromptPolicyHash    string
	WebSearchEnabled    bool
	DeepResearchEnabled bool
}

type ToolSearchRuntimeInfoSource func(sessionID string) ToolSearchRuntimeInfo

type DeferredToolExposureState struct {
	ActivatedTools     []string
	SelectedSkills     []string
	SelectedAgents     []string
	NeedExec           bool
	NeedAgentTools     bool
	RegistryVersion    uint64
	PromptPolicyHash   string
	SkillExposureStamp string
	ExpiresAt          time.Time
}

type DeferredToolExposureUpdate struct {
	ActivatedTools     []string
	SelectedSkills     []string
	SelectedAgents     []string
	NeedExec           bool
	NeedAgentTools     bool
	RegistryVersion    uint64
	PromptPolicyHash   string
	SkillExposureStamp string
}

type DeferredToolExposureStore struct {
	mu         sync.RWMutex
	ttl        time.Duration
	states     map[string]DeferredToolExposureState
	onMutation func(string)
}

func NewDeferredToolExposureStore(ttl time.Duration) *DeferredToolExposureStore {
	if ttl <= 0 {
		ttl = defaultDeferredToolTTL
	}
	return &DeferredToolExposureStore{
		ttl:    ttl,
		states: make(map[string]DeferredToolExposureState),
	}
}

func (s *DeferredToolExposureStore) SetInvalidationCallback(fn func(string)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMutation = fn
}

func (s *DeferredToolExposureStore) Snapshot(sessionID string) (DeferredToolExposureState, bool) {
	if s == nil {
		return DeferredToolExposureState{}, false
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return DeferredToolExposureState{}, false
	}

	s.mu.RLock()
	state, ok := s.states[sessionID]
	s.mu.RUnlock()
	if !ok {
		return DeferredToolExposureState{}, false
	}
	if !state.ExpiresAt.IsZero() && time.Now().After(state.ExpiresAt) {
		s.Delete(sessionID)
		return DeferredToolExposureState{}, false
	}
	return cloneDeferredToolExposureState(state), true
}

func (s *DeferredToolExposureStore) Delete(sessionID string) bool {
	if s == nil {
		return false
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	var callback func(string)
	deleted := false
	s.mu.Lock()
	if _, ok := s.states[sessionID]; ok {
		delete(s.states, sessionID)
		deleted = true
		callback = s.onMutation
	}
	s.mu.Unlock()
	if deleted && callback != nil {
		callback(sessionID)
	}
	return deleted
}

func (s *DeferredToolExposureStore) InvalidateIfStale(sessionID string, registryVersion uint64, promptPolicyHash, skillExposureStamp string) (bool, string) {
	if s == nil {
		return false, ""
	}
	state, ok := s.Snapshot(sessionID)
	if !ok {
		return false, ""
	}
	if state.RegistryVersion != registryVersion {
		return s.Delete(sessionID), "registry_version"
	}
	if strings.TrimSpace(state.PromptPolicyHash) != strings.TrimSpace(promptPolicyHash) {
		return s.Delete(sessionID), "prompt_policy"
	}
	if strings.TrimSpace(state.SkillExposureStamp) != strings.TrimSpace(skillExposureStamp) {
		return s.Delete(sessionID), "skill_exposure"
	}
	return false, ""
}

func (s *DeferredToolExposureStore) Apply(sessionID string, update DeferredToolExposureUpdate) (DeferredToolExposureState, bool) {
	if s == nil {
		return DeferredToolExposureState{}, false
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return DeferredToolExposureState{}, false
	}

	var (
		state    DeferredToolExposureState
		changed  bool
		callback func(string)
	)
	s.mu.Lock()
	current, ok := s.states[sessionID]
	if ok && !current.ExpiresAt.IsZero() && time.Now().After(current.ExpiresAt) {
		ok = false
		current = DeferredToolExposureState{}
	}
	state = current
	state.ActivatedTools = mergeToolSearchNames(state.ActivatedTools, update.ActivatedTools)
	state.SelectedSkills = mergeToolSearchNames(state.SelectedSkills, update.SelectedSkills)
	state.SelectedAgents = mergeToolSearchNames(state.SelectedAgents, update.SelectedAgents)
	state.NeedExec = state.NeedExec || update.NeedExec
	state.NeedAgentTools = state.NeedAgentTools || update.NeedAgentTools
	if update.RegistryVersion != 0 {
		state.RegistryVersion = update.RegistryVersion
	}
	state.PromptPolicyHash = strings.TrimSpace(update.PromptPolicyHash)
	state.SkillExposureStamp = strings.TrimSpace(update.SkillExposureStamp)
	state.ExpiresAt = time.Now().Add(s.ttl)

	changed = !ok || !deferredToolExposureStatesEqual(current, state)
	s.states[sessionID] = state
	callback = s.onMutation
	s.mu.Unlock()

	if changed && callback != nil {
		callback(sessionID)
	}
	return cloneDeferredToolExposureState(state), changed
}

func cloneDeferredToolExposureState(state DeferredToolExposureState) DeferredToolExposureState {
	state.ActivatedTools = append([]string(nil), state.ActivatedTools...)
	state.SelectedSkills = append([]string(nil), state.SelectedSkills...)
	state.SelectedAgents = append([]string(nil), state.SelectedAgents...)
	return state
}

func deferredToolExposureStatesEqual(left, right DeferredToolExposureState) bool {
	if left.NeedExec != right.NeedExec || left.NeedAgentTools != right.NeedAgentTools {
		return false
	}
	if left.RegistryVersion != right.RegistryVersion {
		return false
	}
	if strings.TrimSpace(left.PromptPolicyHash) != strings.TrimSpace(right.PromptPolicyHash) {
		return false
	}
	if strings.TrimSpace(left.SkillExposureStamp) != strings.TrimSpace(right.SkillExposureStamp) {
		return false
	}
	if !toolSearchStringSlicesEqual(left.ActivatedTools, right.ActivatedTools) {
		return false
	}
	if !toolSearchStringSlicesEqual(left.SelectedSkills, right.SelectedSkills) {
		return false
	}
	return toolSearchStringSlicesEqual(left.SelectedAgents, right.SelectedAgents)
}

func toolSearchStringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func mergeToolSearchNames(existing, incoming []string) []string {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	out := make([]string, 0, len(existing)+len(incoming))
	for _, raw := range append(append([]string(nil), existing...), incoming...) {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

type ToolSearchTool struct {
	registry          *Registry
	policyResolver    *ToolPolicyResolver
	skillExposure     *skillmanifest.SkillExposureManager
	agentsConfig      *config.AgentsConfig
	deferredExposure  *DeferredToolExposureStore
	runtimeInfoSource ToolSearchRuntimeInfoSource
}

func NewToolSearchTool(registry *Registry) *ToolSearchTool {
	return &ToolSearchTool{registry: registry}
}

func (t *ToolSearchTool) SetToolPolicyResolver(resolver *ToolPolicyResolver) {
	if t == nil {
		return
	}
	t.policyResolver = resolver
}

func (t *ToolSearchTool) SetSkillExposureManager(manager *skillmanifest.SkillExposureManager) {
	if t == nil {
		return
	}
	t.skillExposure = manager
}

func (t *ToolSearchTool) SetAgentsConfig(cfg *config.AgentsConfig) {
	if t == nil {
		return
	}
	t.agentsConfig = cfg
}

func (t *ToolSearchTool) SetDeferredExposureStore(store *DeferredToolExposureStore) {
	if t == nil {
		return
	}
	t.deferredExposure = store
}

func (t *ToolSearchTool) SetRuntimeInfoSource(fn ToolSearchRuntimeInfoSource) {
	if t == nil {
		return
	}
	t.runtimeInfoSource = fn
}

func (t *ToolSearchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "tool_search",
		Description: "Search tools, skills, and agents by capability. Supports Claude-style select:name1,name2 exact activation and +term required-term filtering, then defers the matched callable surface into the next chat turn.",
		Icon:        "search",
		Aliases:     []string{"search_tools", "search_capabilities"},
		SearchHints: []string{"tools", "skills", "agents", "capabilities", "search", "select"},
		AlwaysLoad:  true,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query. Supports `select:Read,Edit` for exact activation and `+term` for required terms.",
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Optional maximum number of matches to return. Defaults to 5.",
				},
				"kinds": map[string]interface{}{
					"type":        "array",
					"description": "Optional capability kinds to search: tool, skill, agent.",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"tool", "skill", "agent"},
					},
				},
				"select": map[string]interface{}{
					"description": "Optional exact capability names/IDs to activate in addition to query matches. Accepts either a string array or a comma-separated string for compatibility.",
					"anyOf": []interface{}{
						map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			"required": []string{"query"},
		},
	}
}

type toolSearchParsedQuery struct {
	raw           string
	terms         []string
	requiredTerms []string
	selectNames   []string
	kindFilter    map[string]struct{}
	maxResults    int
}

type toolSearchCapability struct {
	ID             string
	Name           string
	Kind           string
	Description    string
	Aliases        []string
	SearchHints    []string
	Tags           []string
	Invocation     string
	Body           string
	Availability   string
	Activation     string
	CallTool       string
	CallHint       string
	CanonicalSkill string
}

type toolSearchScoredMatch struct {
	capability toolSearchCapability
	score      float64
	why        []string
}

type ToolSearchMatch struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Description  string   `json:"description,omitempty"`
	Availability string   `json:"availability,omitempty"`
	Activation   string   `json:"activation,omitempty"`
	CallTool     string   `json:"call_tool,omitempty"`
	CallHint     string   `json:"call_hint,omitempty"`
	WhyMatched   []string `json:"why_matched,omitempty"`
}

type ToolSearchActivationSkip struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type ToolSearchActivated struct {
	Tools      []string                   `json:"tools,omitempty"`
	Exec       bool                       `json:"exec,omitempty"`
	AgentTools bool                       `json:"agent_tools,omitempty"`
	Skipped    []ToolSearchActivationSkip `json:"skipped,omitempty"`
}

type ToolSearchResult struct {
	Matches   []ToolSearchMatch   `json:"matches"`
	Activated ToolSearchActivated `json:"activated"`
}

func (t *ToolSearchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.registry == nil {
		return nil, fmt.Errorf("tool_search is not configured")
	}

	parsed := parseToolSearchArgs(args)
	if strings.TrimSpace(parsed.raw) == "" {
		return nil, fmt.Errorf("query is required")
	}

	runtimeInfo := t.currentRuntimeInfo(strings.TrimSpace(GetSessionID(ctx)))
	currentState := DeferredToolExposureState{}
	if t.deferredExposure != nil {
		if snapshot, ok := t.deferredExposure.Snapshot(strings.TrimSpace(GetSessionID(ctx))); ok {
			currentState = snapshot
		}
	}

	searchToolDefs := t.visibleToolDefinitions(ctx, runtimeInfo, true)
	loadedToolDefs := t.visibleToolDefinitions(ctx, runtimeInfo, false)
	loadedTools := make(map[string]struct{}, len(loadedToolDefs)+len(currentState.ActivatedTools)+4)
	for _, def := range loadedToolDefs {
		loadedTools[strings.ToLower(strings.TrimSpace(def.Name))] = struct{}{}
	}
	for _, name := range currentState.ActivatedTools {
		loadedTools[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	if currentState.NeedExec {
		loadedTools["exec"] = struct{}{}
	}
	if currentState.NeedAgentTools {
		loadedTools["agents_list"] = struct{}{}
		loadedTools["subagents"] = struct{}{}
	}

	snapshot, err := t.skillExposureSnapshot()
	if err != nil {
		return nil, err
	}

	capabilities := t.buildCapabilities(searchToolDefs, loadedTools, currentState, runtimeInfo, snapshot)
	matches, selected := scoreToolSearchCapabilities(capabilities, parsed)
	limit := parsed.maxResults
	if len(selected) > limit {
		limit = len(selected)
	}
	if limit <= 0 {
		limit = defaultToolSearchMaxResults
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}

	result := ToolSearchResult{
		Matches: make([]ToolSearchMatch, 0, len(matches)),
	}
	for _, match := range matches {
		result.Matches = append(result.Matches, ToolSearchMatch{
			ID:           match.capability.ID,
			Name:         match.capability.Name,
			Kind:         match.capability.Kind,
			Description:  match.capability.Description,
			Availability: match.capability.Availability,
			Activation:   match.capability.Activation,
			CallTool:     match.capability.CallTool,
			CallHint:     match.capability.CallHint,
			WhyMatched:   append([]string(nil), match.why...),
		})
	}
	result.Activated = t.applySelectedCapabilities(ctx, selected, searchToolDefs, runtimeInfo, snapshot.Stamp, loadedTools)

	return result, nil
}

func (t *ToolSearchTool) currentRuntimeInfo(sessionID string) ToolSearchRuntimeInfo {
	info := ToolSearchRuntimeInfo{
		WebSearchEnabled:    true,
		DeepResearchEnabled: true,
	}
	if t == nil || t.runtimeInfoSource == nil {
		return info
	}
	runtimeInfo := t.runtimeInfoSource(strings.TrimSpace(sessionID))
	if strings.TrimSpace(runtimeInfo.PromptPolicyHash) != "" {
		info.PromptPolicyHash = strings.TrimSpace(runtimeInfo.PromptPolicyHash)
	}
	info.WebSearchEnabled = runtimeInfo.WebSearchEnabled
	info.DeepResearchEnabled = runtimeInfo.DeepResearchEnabled
	return info
}

func (t *ToolSearchTool) visibleToolDefinitions(ctx context.Context, runtimeInfo ToolSearchRuntimeInfo, skipDefaultAllowlist bool) []ToolDefinition {
	if t == nil || t.registry == nil {
		return nil
	}
	routeKind := GetRouteKind(ctx)
	if routeKind == ToolRouteKindUnknown {
		routeKind = ToolRouteKindChat
	}
	req := ToolPolicyRequest{
		Provider:                       GetProvider(ctx),
		ProviderID:                     GetProviderID(ctx),
		Model:                          GetModel(ctx),
		AgentID:                        GetAgentID(ctx),
		SessionID:                      GetSessionID(ctx),
		RouteKind:                      routeKind,
		SkipDefaultChatDirectAllowlist: skipDefaultAllowlist,
	}
	if runtimeInfo.DeepResearchEnabled {
		deepResearchEnabled := true
		req.DeepResearchEnabled = &deepResearchEnabled
	} else {
		deepResearchEnabled := false
		req.DeepResearchEnabled = &deepResearchEnabled
	}

	defs := t.registry.DefinitionsForRouteAndLocale(routeKind, GetLang(ctx))
	if t.policyResolver != nil {
		defs = t.policyResolver.Filter(req, defs)
	}

	filtered := make([]ToolDefinition, 0, len(defs))
	for _, def := range defs {
		if strings.EqualFold(strings.TrimSpace(def.Name), "tool_search") {
			continue
		}
		if !toolSearchCapabilityAllowedByToggle(def.Name, runtimeInfo) {
			continue
		}
		filtered = append(filtered, def)
	}
	return filtered
}

func toolSearchCapabilityAllowedByToggle(name string, runtimeInfo ToolSearchRuntimeInfo) bool {
	switch normalizeCompatToolName(name) {
	case "analyze":
		return false
	case "web_query":
		return runtimeInfo.WebSearchEnabled
	case "deep_research":
		return runtimeInfo.DeepResearchEnabled
	default:
		return true
	}
}

func (t *ToolSearchTool) skillExposureSnapshot() (skillmanifest.SkillExposureSnapshot, error) {
	if t == nil || t.skillExposure == nil {
		return skillmanifest.SkillExposureSnapshot{}, nil
	}
	return t.skillExposure.Snapshot()
}

func (t *ToolSearchTool) buildCapabilities(
	searchToolDefs []ToolDefinition,
	loadedTools map[string]struct{},
	currentState DeferredToolExposureState,
	runtimeInfo ToolSearchRuntimeInfo,
	snapshot skillmanifest.SkillExposureSnapshot,
) []toolSearchCapability {
	capabilities := make([]toolSearchCapability, 0, len(searchToolDefs)+len(snapshot.VisibleSkills)+8)
	toolNames := make(map[string]ToolDefinition, len(searchToolDefs))
	taken := make(map[string]struct{}, len(searchToolDefs)*2)

	for _, def := range searchToolDefs {
		toolNames[strings.ToLower(strings.TrimSpace(def.Name))] = def
		availability := "deferred"
		if def.AlwaysLoad {
			availability = "loaded"
		}
		if _, ok := loadedTools[strings.ToLower(strings.TrimSpace(def.Name))]; ok {
			availability = "loaded"
		}
		capability := toolSearchCapability{
			ID:           strings.TrimSpace(def.Name),
			Name:         strings.TrimSpace(def.Name),
			Kind:         "tool",
			Description:  strings.TrimSpace(def.Description),
			Aliases:      append([]string(nil), def.Aliases...),
			SearchHints:  append([]string(nil), def.SearchHints...),
			Availability: availability,
			Activation:   "available",
			CallTool:     strings.TrimSpace(def.Name),
			CallHint:     fmt.Sprintf("Call the `%s` tool directly.", strings.TrimSpace(def.Name)),
		}
		capabilities = append(capabilities, capability)
		for _, key := range toolSearchDedupeKeys(capability.Name, capability.ID, capability.CanonicalSkill) {
			taken[key] = struct{}{}
		}
	}

	if len(snapshot.VisibleSkills) > 0 {
		for _, view := range snapshot.VisibleSkills {
			doc := view.Document
			if !view.ModelInvocable || !doc.ModelInvocable {
				continue
			}
			if !toolSearchCapabilityAllowedByToggle(doc.ID, runtimeInfo) && !toolSearchCapabilityAllowedByToggle(doc.Name, runtimeInfo) {
				continue
			}
			canonical := ""
			if toolSearchDedupeTaken(taken, toolSearchDedupeKeys(doc.Name, doc.ID, canonical)) {
				continue
			}
			availability := "deferred"
			if currentState.NeedExec || containsToolSearchName(currentState.SelectedSkills, doc.ID) || containsToolSearchName(currentState.SelectedSkills, doc.Name) {
				availability = "loaded"
			}
			activation := strings.TrimSpace(view.ActivationState)
			if activation == "" {
				activation = "active"
			}
			callHint := strings.TrimSpace(doc.Invocation)
			if callHint == "" {
				callHint = fmt.Sprintf("blue %s", strings.TrimSpace(firstNonBlank(doc.ID, doc.Name)))
			}
			capabilities = append(capabilities, toolSearchCapability{
				ID:             strings.TrimSpace(firstNonBlank(doc.ID, doc.Name)),
				Name:           strings.TrimSpace(firstNonBlank(doc.Name, doc.ID)),
				Kind:           "skill",
				Description:    strings.TrimSpace(doc.Description),
				Aliases:        nil,
				SearchHints:    append(append(append([]string(nil), doc.CapabilityTags...), doc.Tags...), doc.Environment...),
				Tags:           append(append([]string(nil), doc.CapabilityTags...), doc.Tags...),
				Invocation:     strings.TrimSpace(doc.Invocation),
				Body:           strings.TrimSpace(doc.Body),
				Availability:   availability,
				Activation:     activation,
				CallTool:       "exec",
				CallHint:       callHint,
				CanonicalSkill: canonical,
			})
		}
	}

	if t != nil && t.agentsConfig != nil {
		for _, agent := range t.agentsConfig.List {
			effective := effectiveAgentConfig(t.agentsConfig.Defaults, agent)
			available := effective.Enabled && effective.Subagents.Enabled
			availability := "deferred"
			if currentState.NeedAgentTools || containsToolSearchName(currentState.SelectedAgents, agent.ID) {
				availability = "loaded"
			}
			activation := "available"
			if !available {
				activation = "unavailable"
			}
			capabilities = append(capabilities, toolSearchCapability{
				ID:           strings.TrimSpace(agent.ID),
				Name:         strings.TrimSpace(agent.ID),
				Kind:         "agent",
				Description:  strings.TrimSpace(firstNonEmptyAgent(agent.Description, effective.Description)),
				Aliases:      nil,
				SearchHints:  []string{"agent", "subagent", "worker", "delegate"},
				Availability: availability,
				Activation:   activation,
				CallTool:     "subagents",
				CallHint:     fmt.Sprintf("Call `subagents` with `action=spawn`, `agent_id=%s`, and a concrete `goal`.", strings.TrimSpace(agent.ID)),
			})
		}
	}

	_ = toolNames
	return capabilities
}

func toolSearchDedupeKeys(name, id, canonical string) []string {
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

func toolSearchDedupeTaken(taken map[string]struct{}, keys []string) bool {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := taken[key]; ok {
			return true
		}
	}
	return false
}

func parseToolSearchArgs(args map[string]interface{}) toolSearchParsedQuery {
	parsed := toolSearchParsedQuery{
		raw:        strings.TrimSpace(firstCompatString(args, "query", "q", "input")),
		maxResults: defaultToolSearchMaxResults,
	}
	if maxResults, ok := firstCompatIntDeep(args, "max_results", "maxResults", "limit"); ok && maxResults > 0 {
		parsed.maxResults = maxResults
	}

	kindFilter := make(map[string]struct{})
	if rawKinds, ok := firstCompatValueDeep(args, "kinds", "types"); ok {
		switch typed := rawKinds.(type) {
		case []string:
			for _, kind := range typed {
				if normalized := normalizeToolSearchKind(kind); normalized != "" {
					kindFilter[normalized] = struct{}{}
				}
			}
		case []interface{}:
			for _, item := range typed {
				if normalized := normalizeToolSearchKind(fmt.Sprint(item)); normalized != "" {
					kindFilter[normalized] = struct{}{}
				}
			}
		case string:
			for _, kind := range strings.Split(typed, ",") {
				if normalized := normalizeToolSearchKind(kind); normalized != "" {
					kindFilter[normalized] = struct{}{}
				}
			}
		}
	}
	if len(kindFilter) > 0 {
		parsed.kindFilter = kindFilter
	}

	selectNames := make([]string, 0, 4)
	if rawSelect, ok := firstCompatValueDeep(args, "select", "selects"); ok {
		switch typed := rawSelect.(type) {
		case []string:
			selectNames = append(selectNames, typed...)
		case []interface{}:
			for _, item := range typed {
				selectNames = append(selectNames, fmt.Sprint(item))
			}
		case string:
			selectNames = append(selectNames, strings.Split(typed, ",")...)
		}
	}

	for _, field := range strings.Fields(parsed.raw) {
		lower := strings.ToLower(field)
		if strings.HasPrefix(lower, "select:") {
			for _, item := range strings.Split(field[len("select:"):], ",") {
				selectNames = append(selectNames, item)
			}
			continue
		}
		if strings.HasPrefix(field, "+") && len(field) > 1 {
			term := normalizeToolSearchTerm(field[1:])
			if term != "" {
				parsed.requiredTerms = append(parsed.requiredTerms, term)
				parsed.terms = append(parsed.terms, term)
			}
			continue
		}
		if term := normalizeToolSearchTerm(field); term != "" {
			parsed.terms = append(parsed.terms, term)
		}
	}

	parsed.selectNames = uniqueNormalizedToolSearchNames(selectNames)
	return parsed
}

func normalizeToolSearchKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tool", "tools":
		return "tool"
	case "skill", "skills":
		return "skill"
	case "agent", "agents", "subagent", "subagents":
		return "agent"
	default:
		return ""
	}
}

func normalizeToolSearchKey(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	trimmed = strings.Trim(trimmed, ",.;:()[]{}\"'`")
	replacer := strings.NewReplacer("_", "", "-", "", " ", "", ".", "", "/", "", ":", "")
	return replacer.Replace(trimmed)
}

func normalizeToolSearchTerm(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	trimmed = strings.Trim(trimmed, ",.;:()[]{}\"'`")
	return trimmed
}

func uniqueNormalizedToolSearchNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeToolSearchKey(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func scoreToolSearchCapabilities(capabilities []toolSearchCapability, parsed toolSearchParsedQuery) ([]toolSearchScoredMatch, []toolSearchCapability) {
	if len(capabilities) == 0 {
		return nil, nil
	}
	selectedByKey := make(map[string]struct{}, len(parsed.selectNames))
	for _, key := range parsed.selectNames {
		selectedByKey[key] = struct{}{}
	}

	matches := make([]toolSearchScoredMatch, 0, len(capabilities))
	selected := make([]toolSearchCapability, 0, len(parsed.selectNames))
	selectedSeen := make(map[string]struct{}, len(parsed.selectNames))

	for _, capability := range capabilities {
		if len(parsed.kindFilter) > 0 {
			if _, ok := parsed.kindFilter[capability.Kind]; !ok {
				continue
			}
		}

		selectReasons := matchedToolSearchSelections(capability, selectedByKey)
		if len(selectReasons) > 0 {
			if _, ok := selectedSeen[strings.ToLower(capability.Kind)+":"+strings.ToLower(capability.ID)]; !ok {
				selected = append(selected, capability)
				selectedSeen[strings.ToLower(capability.Kind)+":"+strings.ToLower(capability.ID)] = struct{}{}
			}
		}

		score, why, ok := scoreToolSearchCapability(capability, parsed, selectReasons)
		if !ok {
			continue
		}
		matches = append(matches, toolSearchScoredMatch{
			capability: capability,
			score:      score,
			why:        why,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			if matches[i].capability.Kind == matches[j].capability.Kind {
				return matches[i].capability.Name < matches[j].capability.Name
			}
			return matches[i].capability.Kind < matches[j].capability.Kind
		}
		return matches[i].score > matches[j].score
	})
	return matches, selected
}

func matchedToolSearchSelections(capability toolSearchCapability, selected map[string]struct{}) []string {
	if len(selected) == 0 {
		return nil
	}
	keys := toolSearchCapabilityKeys(capability)
	matched := make([]string, 0, 2)
	for _, key := range keys {
		if _, ok := selected[key]; ok {
			matched = append(matched, "selected:"+key)
		}
	}
	return matched
}

func toolSearchCapabilityKeys(capability toolSearchCapability) []string {
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

func scoreToolSearchCapability(capability toolSearchCapability, parsed toolSearchParsedQuery, selectReasons []string) (float64, []string, bool) {
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
		reasons = append(reasons, fmt.Sprintf("%s:%s", term, strings.Join(uniqueNormalizedToolSearchNames(termReasons), "+")))
	}

	if score == 0 {
		return 0, nil, false
	}
	return score, reasons, true
}

func (t *ToolSearchTool) applySelectedCapabilities(
	ctx context.Context,
	selected []toolSearchCapability,
	searchToolDefs []ToolDefinition,
	runtimeInfo ToolSearchRuntimeInfo,
	skillExposureStamp string,
	loadedTools map[string]struct{},
) ToolSearchActivated {
	activated := ToolSearchActivated{}
	if len(selected) == 0 {
		return activated
	}

	searchToolNames := make(map[string]ToolDefinition, len(searchToolDefs))
	for _, def := range searchToolDefs {
		searchToolNames[strings.ToLower(strings.TrimSpace(def.Name))] = def
	}

	update := DeferredToolExposureUpdate{
		RegistryVersion:    t.registry.Version(),
		PromptPolicyHash:   strings.TrimSpace(runtimeInfo.PromptPolicyHash),
		SkillExposureStamp: strings.TrimSpace(skillExposureStamp),
	}

	for _, capability := range selected {
		switch capability.Kind {
		case "tool":
			name := strings.TrimSpace(capability.Name)
			if name == "" {
				continue
			}
			if _, ok := loadedTools[strings.ToLower(name)]; ok {
				activated.Skipped = append(activated.Skipped, ToolSearchActivationSkip{
					ID: capability.ID, Name: capability.Name, Kind: capability.Kind, Reason: "already_loaded",
				})
				continue
			}
			if _, ok := searchToolNames[strings.ToLower(name)]; !ok {
				activated.Skipped = append(activated.Skipped, ToolSearchActivationSkip{
					ID: capability.ID, Name: capability.Name, Kind: capability.Kind, Reason: "tool_not_visible",
				})
				continue
			}
			update.ActivatedTools = append(update.ActivatedTools, name)
			activated.Tools = mergeToolSearchNames(activated.Tools, []string{name})
		case "skill":
			if strings.EqualFold(strings.TrimSpace(capability.Activation), "dormant") {
				activated.Skipped = append(activated.Skipped, ToolSearchActivationSkip{
					ID: capability.ID, Name: capability.Name, Kind: capability.Kind, Reason: "skill_dormant",
				})
				continue
			}
			if _, ok := searchToolNames["exec"]; !ok {
				activated.Skipped = append(activated.Skipped, ToolSearchActivationSkip{
					ID: capability.ID, Name: capability.Name, Kind: capability.Kind, Reason: "exec_not_visible",
				})
				continue
			}
			if _, ok := loadedTools["exec"]; ok {
				activated.Skipped = append(activated.Skipped, ToolSearchActivationSkip{
					ID: capability.ID, Name: capability.Name, Kind: capability.Kind, Reason: "already_loaded",
				})
			}
			update.NeedExec = true
			update.SelectedSkills = append(update.SelectedSkills, capability.ID)
			activated.Exec = true
		case "agent":
			if !strings.EqualFold(strings.TrimSpace(capability.Activation), "available") {
				activated.Skipped = append(activated.Skipped, ToolSearchActivationSkip{
					ID: capability.ID, Name: capability.Name, Kind: capability.Kind, Reason: "agent_unavailable",
				})
				continue
			}
			if _, ok := searchToolNames["agents_list"]; !ok {
				activated.Skipped = append(activated.Skipped, ToolSearchActivationSkip{
					ID: capability.ID, Name: capability.Name, Kind: capability.Kind, Reason: "agents_list_not_visible",
				})
				continue
			}
			if _, ok := searchToolNames["subagents"]; !ok {
				activated.Skipped = append(activated.Skipped, ToolSearchActivationSkip{
					ID: capability.ID, Name: capability.Name, Kind: capability.Kind, Reason: "subagents_not_visible",
				})
				continue
			}
			update.NeedAgentTools = true
			update.SelectedAgents = append(update.SelectedAgents, capability.ID)
			activated.AgentTools = true
		}
	}

	if t == nil || t.deferredExposure == nil || strings.TrimSpace(GetSessionID(ctx)) == "" {
		return activated
	}
	if len(update.ActivatedTools) == 0 && len(update.SelectedSkills) == 0 && len(update.SelectedAgents) == 0 && !update.NeedExec && !update.NeedAgentTools {
		return activated
	}
	_, _ = t.deferredExposure.Apply(strings.TrimSpace(GetSessionID(ctx)), update)
	return activated
}

func containsToolSearchName(values []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
