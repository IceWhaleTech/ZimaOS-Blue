package tools

import (
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

// ToolPolicyRequest carries runtime selection hints.
type ToolPolicyRequest struct {
	Provider            string
	ProviderID          string
	Model               string
	AgentID             string
	SessionID           string
	RouteKind           ToolRouteKind
	DeepResearchEnabled *bool
}

// ToolPolicyResolver narrows the visible tool surface before selection/routing.
type ToolPolicyResolver struct {
	profiles       map[string][]string
	groups         map[string][]string
	globalPolicy   config.ToolPolicyConfig
	providerPolicy map[string]config.ToolPolicyConfig
	agentDefault   config.ToolPolicyConfig
	agentPolicy    map[string]config.ToolPolicyConfig
}

// defaultChatDirectToolAllowlist keeps the chat-facing first-class tool surface
// intentionally small. Richer capabilities remain available through exec-routed
// skills or compat paths without inflating the default tool list.
var defaultChatDirectToolAllowlist = map[string]struct{}{
	"ask":           {},
	"bash":          {},
	"browser":       {},
	"calendar":      {},
	"convert":       {},
	"edit":          {},
	"email":         {},
	"find":          {},
	"grep":          {},
	"image":         {},
	"ls":            {},
	"memory":        {},
	"pdf":           {},
	"plan_append":   {},
	"plan_create":   {},
	"plan_update":   {},
	"read":          {},
	"deep_research": {},
	"sessions":      {},
	"web":           {},
	"write":         {},
}

// NewToolPolicyResolver creates a resolver from app config.
func NewToolPolicyResolver(cfg *config.Config) *ToolPolicyResolver {
	if cfg == nil {
		return nil
	}
	resolver := &ToolPolicyResolver{
		profiles:       clonePolicyMap(cfg.ToolCalling.Profiles),
		groups:         clonePolicyMap(cfg.ToolCalling.Groups),
		globalPolicy:   config.ToolPolicyConfig{Profile: cfg.ToolCalling.Profile, Allow: append([]string(nil), cfg.ToolCalling.Allow...), Deny: append([]string(nil), cfg.ToolCalling.Deny...)},
		providerPolicy: clonePolicyScopes(cfg.ToolCalling.ByProvider),
		agentDefault:   cfg.Agents.Defaults.ToolPolicy,
		agentPolicy:    make(map[string]config.ToolPolicyConfig),
	}
	for _, agent := range cfg.Agents.List {
		id := strings.TrimSpace(agent.ID)
		if id == "" {
			continue
		}
		resolver.agentPolicy[id] = cloneToolPolicyScope(agent.ToolPolicy)
	}
	return resolver
}

// Filter narrows tool definitions according to configured policy.
func (r *ToolPolicyResolver) Filter(req ToolPolicyRequest, defs []ToolDefinition) []ToolDefinition {
	if r == nil || len(defs) == 0 {
		return defs
	}
	providerScope := r.providerScope(req)
	agentScope, hasAgent := r.agentScope(req)

	baseProfile := "full"
	if hasAgent {
		baseProfile = firstNonEmpty(agentScope.Profile, r.agentDefault.Profile, r.globalPolicy.Profile)
	} else {
		baseProfile = firstNonEmpty(providerScope.Profile, r.globalPolicy.Profile)
	}

	visible := make(map[string]ToolDefinition, len(defs))
	for _, def := range defs {
		visible[def.Name] = def
	}
	if r.shouldApplyDefaultChatDirectToolAllowlist(req, hasAgent, providerScope) {
		r.applyAllowSet(visible, defaultChatDirectToolAllowlist)
	}
	r.applyBaseProfile(visible, baseProfile)
	r.applyAllowDeny(visible, r.globalPolicy)
	r.applyAllowDeny(visible, providerScope)
	if hasAgent {
		r.applyAllowDeny(visible, r.agentDefault)
		r.applyAllowDeny(visible, agentScope)
	}
	if len(visible) == 0 {
		return nil
	}
	names := make([]string, 0, len(visible))
	for name := range visible {
		names = append(names, name)
	}
	sort.Strings(names)
	filtered := make([]ToolDefinition, 0, len(names))
	for _, name := range names {
		filtered = append(filtered, visible[name])
	}
	return filtered
}

func (r *ToolPolicyResolver) providerScope(req ToolPolicyRequest) config.ToolPolicyConfig {
	if r == nil || len(r.providerPolicy) == 0 {
		return config.ToolPolicyConfig{}
	}
	if policy, ok := r.providerPolicy[strings.TrimSpace(req.Provider)]; ok {
		return cloneToolPolicyScope(policy)
	}
	if policy, ok := r.providerPolicy[strings.TrimSpace(req.ProviderID)]; ok {
		return cloneToolPolicyScope(policy)
	}
	if policy, ok := r.providerPolicy[strings.TrimSpace(req.Model)]; ok {
		return cloneToolPolicyScope(policy)
	}
	if providerID := strings.TrimSpace(req.ProviderID); providerID != "" && strings.TrimSpace(req.Model) != "" {
		if policy, ok := r.providerPolicy[providerID+"/"+strings.TrimSpace(req.Model)]; ok {
			return cloneToolPolicyScope(policy)
		}
	}
	if provider := strings.TrimSpace(req.Provider); provider != "" && strings.TrimSpace(req.Model) != "" {
		if policy, ok := r.providerPolicy[provider+"/"+strings.TrimSpace(req.Model)]; ok {
			return cloneToolPolicyScope(policy)
		}
	}
	return config.ToolPolicyConfig{}
}

func (r *ToolPolicyResolver) shouldApplyDefaultChatDirectToolAllowlist(req ToolPolicyRequest, hasAgent bool, providerScope config.ToolPolicyConfig) bool {
	if r == nil || hasAgent || req.RouteKind != ToolRouteKindChat {
		return false
	}
	if toolPolicyConfigured(r.globalPolicy) {
		return false
	}
	if toolPolicyConfigured(providerScope) {
		return false
	}
	return true
}

func (r *ToolPolicyResolver) agentScope(req ToolPolicyRequest) (config.ToolPolicyConfig, bool) {
	if r == nil {
		return config.ToolPolicyConfig{}, false
	}
	id := strings.TrimSpace(req.AgentID)
	if id == "" {
		return config.ToolPolicyConfig{}, false
	}
	policy, ok := r.agentPolicy[id]
	if !ok {
		return config.ToolPolicyConfig{}, false
	}
	return cloneToolPolicyScope(policy), true
}

func (r *ToolPolicyResolver) applyBaseProfile(visible map[string]ToolDefinition, profile string) {
	profile = strings.ToLower(strings.TrimSpace(profile))
	if profile == "" || profile == "full" || len(visible) == 0 {
		return
	}
	expanded := r.expandProfile(profile)
	if len(expanded) == 0 {
		return
	}
	for name := range visible {
		if _, ok := expanded[normalizeToolPolicyName(name)]; !ok {
			delete(visible, name)
		}
	}
}

func (r *ToolPolicyResolver) applyAllowSet(visible map[string]ToolDefinition, allow map[string]struct{}) {
	if len(visible) == 0 || len(allow) == 0 {
		return
	}
	for name := range visible {
		if _, ok := allow[normalizeToolPolicyName(name)]; !ok {
			delete(visible, name)
		}
	}
}

func (r *ToolPolicyResolver) applyAllowDeny(visible map[string]ToolDefinition, scope config.ToolPolicyConfig) {
	if len(visible) == 0 {
		return
	}
	if expanded := r.expandEntries(scope.Allow); len(expanded) > 0 {
		for name := range visible {
			if _, ok := expanded[normalizeToolPolicyName(name)]; !ok {
				delete(visible, name)
			}
		}
	}
	if denied := r.expandEntries(scope.Deny); len(denied) > 0 {
		for name := range visible {
			if _, ok := denied[normalizeToolPolicyName(name)]; ok {
				delete(visible, name)
			}
		}
	}
}

func (r *ToolPolicyResolver) expandProfile(profile string) map[string]struct{} {
	entries, ok := r.profiles[profile]
	if !ok {
		return nil
	}
	return r.expandEntries(entries)
}

func (r *ToolPolicyResolver) expandEntries(entries []string) map[string]struct{} {
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]struct{})
	var visit func(string)
	visit = func(entry string) {
		key := normalizeToolPolicyName(entry)
		if key == "" {
			return
		}
		if groupEntries, ok := r.groups[key]; ok {
			for _, nested := range groupEntries {
				visit(nested)
			}
			return
		}
		out[key] = struct{}{}
	}
	for _, entry := range entries {
		visit(entry)
	}
	return out
}

func normalizeToolPolicyName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "read", "read_file", "file_read":
		return "read"
	case "write", "write_file", "file_write":
		return "write"
	case "bash", "exec":
		return "bash"
	case "delete", "remove", "rm", "unlink", "file_delete":
		return "file_delete"
	case "rg":
		return "grep"
	case "web", "web_query", "web_search", "web_fetch", "web_read", "web_extract", "web_crawl":
		return "web"
	case "image", "image_generation", "generate_image", "generateimage":
		return "image"
	case "deep_research", "deep-research", "research_run", "research_status":
		return "deep_research"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func clonePolicyMap(input map[string][]string) map[string][]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string][]string, len(input))
	for key, values := range input {
		normalized := strings.ToLower(strings.TrimSpace(key))
		out[normalized] = append([]string(nil), values...)
	}
	return out
}

func clonePolicyScopes(input map[string]config.ToolPolicyConfig) map[string]config.ToolPolicyConfig {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]config.ToolPolicyConfig, len(input))
	for key, scope := range input {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		out[normalized] = cloneToolPolicyScope(scope)
	}
	return out
}

func cloneToolPolicyScope(scope config.ToolPolicyConfig) config.ToolPolicyConfig {
	cloned := scope
	cloned.Profile = strings.TrimSpace(scope.Profile)
	cloned.Allow = append([]string(nil), scope.Allow...)
	cloned.Deny = append([]string(nil), scope.Deny...)
	cloned.ByProvider = clonePolicyScopes(scope.ByProvider)
	return cloned
}

func toolPolicyConfigured(scope config.ToolPolicyConfig) bool {
	if strings.TrimSpace(scope.Profile) != "" {
		return true
	}
	return len(scope.Allow) > 0 || len(scope.Deny) > 0 || len(scope.ByProvider) > 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
