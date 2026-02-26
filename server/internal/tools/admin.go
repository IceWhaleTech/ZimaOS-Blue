package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MgmtTool is a native tool for system administration via chat.
// It provides a single entry point with action-based dispatch using dot notation
// (e.g. "providers.list", "settings.get").
type MgmtTool struct {
	providers AdminProviderService
	settings  AdminSettingsService
	channels  AdminChannelService
	skills    AdminSkillService
	tools     AdminToolService
	system    AdminSystemService
	proxy     AdminProxyService
	users     AdminUserService
	apiKeys   AdminAPIKeyService
	memory    *MemoryTool
}

// NewMgmtTool creates a new management tool. Services are injected later via Set* methods.
func NewMgmtTool() *MgmtTool {
	return &MgmtTool{}
}

// --- Setters for deferred wiring ---

func (t *MgmtTool) SetProviders(svc AdminProviderService) { t.providers = svc }
func (t *MgmtTool) SetSettings(svc AdminSettingsService)   { t.settings = svc }
func (t *MgmtTool) SetChannels(svc AdminChannelService)    { t.channels = svc }
func (t *MgmtTool) SetSkills(svc AdminSkillService)        { t.skills = svc }
func (t *MgmtTool) SetTools(svc AdminToolService)          { t.tools = svc }
func (t *MgmtTool) SetSystem(svc AdminSystemService)       { t.system = svc }
func (t *MgmtTool) SetProxy(svc AdminProxyService)         { t.proxy = svc }
func (t *MgmtTool) SetUsers(svc AdminUserService)          { t.users = svc }
func (t *MgmtTool) SetAPIKeys(svc AdminAPIKeyService)      { t.apiKeys = svc }
func (t *MgmtTool) SetMemory(mem *MemoryTool)              { t.memory = mem }

// Definition returns the tool definition.
func (t *MgmtTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "mgmt",
		Description: `System management tool. Use {domain}.{action} format. Domains: providers, settings, channels, skills, tools, system, proxy, users, apikeys. Call with action="providers.list" first to explore available operations. Common: providers.list, settings.get, system.health, tools.list, users.list.`,
		Icon: "settings",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform (dot notation, e.g. providers.list, settings.set)",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Resource identifier (provider ID, user ID, API key ID, etc.)",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Resource name (provider name, API key name, channel name, etc.)",
				},
				"key": map[string]interface{}{
					"type":        "string",
					"description": "Setting key (for settings.set)",
				},
				"value": map[string]interface{}{
					"type":        "string",
					"description": "Setting value (for settings.set)",
				},
				"provider_type": map[string]interface{}{
					"type":        "string",
					"description": "Provider type: openai, anthropic, google, ollama, etc. (for providers.add)",
				},
				"base_url": map[string]interface{}{
					"type":        "string",
					"description": "Provider base URL (for providers.add)",
				},
				"api_key": map[string]interface{}{
					"type":        "string",
					"description": "Provider API key (for providers.add, providers.add_key)",
				},
				"location": map[string]interface{}{
					"type":        "string",
					"description": "Provider location: cloud or local (for providers.add, default: cloud)",
				},
			},
			"required": []string{"action"},
		},
	}
}

// Execute dispatches to the appropriate handler based on the action parameter.
func (t *MgmtTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errJSON("action is required"), nil
	}

	parts := strings.SplitN(action, ".", 2)
	if len(parts) != 2 {
		return errJSON("invalid action format, use dot notation (e.g. providers.list)"), nil
	}

	domain, op := parts[0], parts[1]

	switch domain {
	case "providers":
		return t.handleProviders(ctx, op, args)
	case "settings":
		return t.handleSettings(ctx, op, args)
	case "channels":
		return t.handleChannels(ctx, op, args)
	case "skills":
		return t.handleSkills(ctx, op, args)
	case "tools":
		return t.handleTools(ctx, op, args)
	case "system":
		return t.handleSystem(ctx, op)
	case "proxy":
		return t.handleProxy(ctx, op)
	case "users":
		return t.handleUsers(ctx, op, args)
	case "apikeys":
		return t.handleAPIKeys(ctx, op, args)
	case "memory":
		return t.handleMemory(ctx, op, args)
	default:
		return errJSON(fmt.Sprintf("unknown domain: %s", domain)), nil
	}
}

// --- Provider handlers ---

func (t *MgmtTool) handleProviders(ctx context.Context, op string, args map[string]interface{}) (interface{}, error) {
	if t.providers == nil {
		return errJSON("provider management not available"), nil
	}
	switch op {
	case "list":
		result, err := t.providers.ListProviders(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "add":
		name, _ := args["name"].(string)
		pType, _ := args["provider_type"].(string)
		baseURL, _ := args["base_url"].(string)
		apiKey, _ := args["api_key"].(string)
		location, _ := args["location"].(string)
		if name == "" {
			return errJSON("name is required for providers.add"), nil
		}
		result, err := t.providers.AddProvider(ctx, name, pType, baseURL, apiKey, location)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "add_key":
		id, _ := args["id"].(string)
		apiKey, _ := args["api_key"].(string)
		if id == "" {
			return errJSON("id is required for providers.add_key"), nil
		}
		if apiKey == "" {
			return errJSON("api_key is required for providers.add_key"), nil
		}
		result, err := t.providers.AddKey(ctx, id, apiKey)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "remove":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for providers.remove"), nil
		}
		if err := t.providers.RemoveProvider(ctx, id); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON("provider removed")
	case "enable":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for providers.enable"), nil
		}
		if err := t.providers.EnableProvider(ctx, id); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON("provider enabled")
	case "disable":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for providers.disable"), nil
		}
		if err := t.providers.DisableProvider(ctx, id); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON("provider disabled")
	case "test":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for providers.test"), nil
		}
		result, err := t.providers.TestProvider(ctx, id)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "models":
		result, err := t.providers.ListModels(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	default:
		return errJSON(fmt.Sprintf("unknown provider operation: %s", op)), nil
	}
}

// --- Settings handlers ---

func (t *MgmtTool) handleSettings(ctx context.Context, op string, args map[string]interface{}) (interface{}, error) {
	if t.settings == nil {
		return errJSON("settings management not available"), nil
	}
	switch op {
	case "get":
		result, err := t.settings.GetAll(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "set":
		key, _ := args["key"].(string)
		value, _ := args["value"].(string)
		if key == "" {
			return errJSON("key is required for settings.set"), nil
		}
		if err := t.settings.Set(ctx, key, value); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON(fmt.Sprintf("setting %s updated", key))
	default:
		return errJSON(fmt.Sprintf("unknown settings operation: %s", op)), nil
	}
}

// --- Channel handlers ---

func (t *MgmtTool) handleChannels(ctx context.Context, op string, args map[string]interface{}) (interface{}, error) {
	if t.channels == nil {
		return errJSON("channel management not available"), nil
	}
	switch op {
	case "list":
		result, err := t.channels.ListChannels(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "status":
		name, _ := args["name"].(string)
		if name == "" {
			return errJSON("name is required for channels.status"), nil
		}
		result, err := t.channels.GetStatus(ctx, name)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	default:
		return errJSON(fmt.Sprintf("unknown channel operation: %s", op)), nil
	}
}

// --- Skill handlers ---

func (t *MgmtTool) handleSkills(ctx context.Context, op string, args map[string]interface{}) (interface{}, error) {
	if t.skills == nil {
		return errJSON("skill management not available"), nil
	}
	switch op {
	case "list":
		result, err := t.skills.ListSkills(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "enable":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for skills.enable"), nil
		}
		if err := t.skills.EnableSkill(ctx, id); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON(fmt.Sprintf("skill %s enabled", id))
	case "disable":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for skills.disable"), nil
		}
		if err := t.skills.DisableSkill(ctx, id); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON(fmt.Sprintf("skill %s disabled", id))
	default:
		return errJSON(fmt.Sprintf("unknown skill operation: %s", op)), nil
	}
}

// --- Tool handlers ---

func (t *MgmtTool) handleTools(ctx context.Context, op string, args map[string]interface{}) (interface{}, error) {
	if t.tools == nil {
		return errJSON("tool management not available"), nil
	}
	switch op {
	case "list":
		result, err := t.tools.ListTools(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "enable":
		name, _ := args["name"].(string)
		if name == "" {
			return errJSON("name is required for tools.enable"), nil
		}
		if err := t.tools.EnableTool(ctx, name); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON(fmt.Sprintf("tool %s enabled", name))
	case "disable":
		name, _ := args["name"].(string)
		if name == "" {
			return errJSON("name is required for tools.disable"), nil
		}
		if err := t.tools.DisableTool(ctx, name); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON(fmt.Sprintf("tool %s disabled", name))
	default:
		return errJSON(fmt.Sprintf("unknown tool operation: %s", op)), nil
	}
}

// --- System handlers ---

func (t *MgmtTool) handleSystem(ctx context.Context, op string) (interface{}, error) {
	if t.system == nil {
		return errJSON("system info not available"), nil
	}
	switch op {
	case "health", "info":
		result, err := t.system.Health(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "version":
		result, err := t.system.Health(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(map[string]string{"version": result.Version})
	default:
		return errJSON(fmt.Sprintf("unknown system operation: %s", op)), nil
	}
}

// --- Proxy handlers ---

func (t *MgmtTool) handleProxy(ctx context.Context, op string) (interface{}, error) {
	if t.proxy == nil {
		return errJSON("proxy stats not available"), nil
	}
	switch op {
	case "stats":
		result, err := t.proxy.Stats(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "cache_stats":
		result, err := t.proxy.CacheStats(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	default:
		return errJSON(fmt.Sprintf("unknown proxy operation: %s", op)), nil
	}
}

// --- User handlers ---

func (t *MgmtTool) handleUsers(ctx context.Context, op string, args map[string]interface{}) (interface{}, error) {
	if t.users == nil {
		return errJSON("user management not available"), nil
	}
	switch op {
	case "list":
		result, err := t.users.ListUsers(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "lock":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for users.lock"), nil
		}
		if err := t.users.LockUser(ctx, id); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON(fmt.Sprintf("user %s locked", id))
	case "unlock":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for users.unlock"), nil
		}
		if err := t.users.UnlockUser(ctx, id); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON(fmt.Sprintf("user %s unlocked", id))
	default:
		return errJSON(fmt.Sprintf("unknown user operation: %s", op)), nil
	}
}

// --- API Key handlers ---

func (t *MgmtTool) handleAPIKeys(ctx context.Context, op string, args map[string]interface{}) (interface{}, error) {
	if t.apiKeys == nil {
		return errJSON("API key management not available"), nil
	}
	switch op {
	case "list":
		result, err := t.apiKeys.ListKeys(ctx)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "create":
		name, _ := args["name"].(string)
		if name == "" {
			return errJSON("name is required for apikeys.create"), nil
		}
		result, err := t.apiKeys.CreateKey(ctx, name)
		if err != nil {
			return errJSON(err.Error()), nil
		}
		return toJSON(result)
	case "revoke":
		id, _ := args["id"].(string)
		if id == "" {
			return errJSON("id is required for apikeys.revoke"), nil
		}
		if err := t.apiKeys.RevokeKey(ctx, id); err != nil {
			return errJSON(err.Error()), nil
		}
		return okJSON("API key revoked")
	default:
		return errJSON(fmt.Sprintf("unknown apikeys operation: %s", op)), nil
	}
}

// --- Memory handlers ---

func (t *MgmtTool) handleMemory(ctx context.Context, op string, args map[string]interface{}) (interface{}, error) {
	if t.memory == nil {
		return errJSON("memory service not available"), nil
	}
	// Proxy to MemoryTool: map mgmt op to memory action
	memArgs := map[string]interface{}{"action": op}
	for _, key := range []string{"query", "id", "content", "category", "limit"} {
		if v, ok := args[key]; ok {
			memArgs[key] = v
		}
	}
	return t.memory.Execute(ctx, memArgs)
}

// --- Helpers ---

func errJSON(msg string) string {
	b, _ := json.Marshal(map[string]interface{}{"error": msg})
	return string(b)
}

func okJSON(msg string) (interface{}, error) {
	b, _ := json.Marshal(map[string]interface{}{"success": true, "message": msg})
	return string(b), nil
}

func toJSON(v interface{}) (interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return errJSON(err.Error()), nil
	}
	return string(b), nil
}

// RegisterMgmtTool registers the management tool with the registry.
func RegisterMgmtTool(registry *Registry) *MgmtTool {
	t := NewMgmtTool()
	registry.Register(t)
	return t
}
