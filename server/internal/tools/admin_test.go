package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// --- Mock services for testing ---

type mockProviderService struct {
	providers []AdminProviderInfo
	models    []map[string]interface{}
}

func (m *mockProviderService) ListProviders(_ context.Context) ([]AdminProviderInfo, error) {
	return m.providers, nil
}
func (m *mockProviderService) AddProvider(_ context.Context, name, providerType, baseURL, apiKey, location string) (*AdminProviderInfo, error) {
	p := &AdminProviderInfo{ID: "new-1", Name: name, Type: providerType, Location: location, BaseURL: baseURL, Enabled: true}
	m.providers = append(m.providers, *p)
	return p, nil
}
func (m *mockProviderService) AddKey(_ context.Context, providerID, apiKey string) (*AdminProviderInfo, error) {
	for i, p := range m.providers {
		if p.ID == providerID {
			m.providers[i].APIKeys = append(m.providers[i].APIKeys, AdminProviderKey{ID: "key-new", KeyHash: "sk-ab...yz"})
			return &m.providers[i], nil
		}
	}
	return nil, fmt.Errorf("provider %q not found", providerID)
}
func (m *mockProviderService) RemoveProvider(_ context.Context, id string) error  { return nil }
func (m *mockProviderService) EnableProvider(_ context.Context, id string) error  { return nil }
func (m *mockProviderService) DisableProvider(_ context.Context, id string) error { return nil }
func (m *mockProviderService) TestProvider(_ context.Context, id string) (map[string]interface{}, error) {
	return map[string]interface{}{"id": id, "status": "active"}, nil
}
func (m *mockProviderService) ListModels(_ context.Context) ([]map[string]interface{}, error) {
	return m.models, nil
}

type mockSettingsService struct {
	settings map[string]interface{}
}

func (m *mockSettingsService) GetAll(_ context.Context) (map[string]interface{}, error) {
	return m.settings, nil
}
func (m *mockSettingsService) Set(_ context.Context, key, value string) error {
	m.settings[key] = value
	return nil
}

type mockSkillService struct {
	skills []AdminSkillInfo
}

func (m *mockSkillService) ListSkills(_ context.Context) ([]AdminSkillInfo, error) {
	return m.skills, nil
}
func (m *mockSkillService) EnableSkill(_ context.Context, id string) error  { return nil }
func (m *mockSkillService) DisableSkill(_ context.Context, id string) error { return nil }

type mockToolService struct {
	tools []AdminToolInfo
}

func (m *mockToolService) ListTools(_ context.Context) ([]AdminToolInfo, error) {
	return m.tools, nil
}
func (m *mockToolService) EnableTool(_ context.Context, name string) error  { return nil }
func (m *mockToolService) DisableTool(_ context.Context, name string) error { return nil }

type mockSystemService struct{}

func (m *mockSystemService) Health(_ context.Context) (*AdminSystemInfo, error) {
	return &AdminSystemInfo{
		Version:    "0.10.33",
		Uptime:     "1h 30m",
		GoVersion:  "go1.22.0",
		NumCPU:     8,
		Goroutines: 42,
		MemAllocMB: 64.5,
		MemRSSMB:   128.0,
	}, nil
}

type mockUserService struct {
	users []AdminUserInfo
}

func (m *mockUserService) ListUsers(_ context.Context) ([]AdminUserInfo, error) {
	return m.users, nil
}
func (m *mockUserService) LockUser(_ context.Context, id string) error   { return nil }
func (m *mockUserService) UnlockUser(_ context.Context, id string) error { return nil }

type mockAPIKeyService struct {
	keys []AdminAPIKeyInfo
}

func (m *mockAPIKeyService) ListKeys(_ context.Context) ([]AdminAPIKeyInfo, error) {
	return m.keys, nil
}
func (m *mockAPIKeyService) CreateKey(_ context.Context, name string) (*AdminAPIKeyCreateResult, error) {
	return &AdminAPIKeyCreateResult{ID: "key-1", Name: name, Key: "sk-test-123", Prefix: "sk-test-"}, nil
}
func (m *mockAPIKeyService) RevokeKey(_ context.Context, id string) error { return nil }

// --- Helper ---

func newTestMgmtTool() *MgmtTool {
	t := NewMgmtTool()
	t.SetProviders(&mockProviderService{
		providers: []AdminProviderInfo{
			{ID: "openai", Name: "OpenAI", Type: "builtin", Enabled: true, Priority: 100},
			{ID: "ollama", Name: "Ollama", Type: "custom", BaseURL: "http://localhost:11434", Enabled: true},
		},
		models: []map[string]interface{}{
			{"id": "gpt-4", "provider": "OpenAI"},
		},
	})
	t.SetSettings(&mockSettingsService{
		settings: map[string]interface{}{"locale": "zh-CN", "agent_mode": true},
	})
	t.SetSkills(&mockSkillService{
		skills: []AdminSkillInfo{
			{ID: "push_notification", Name: "Push", Enabled: true, Builtin: true},
		},
	})
	t.SetTools(&mockToolService{
		tools: []AdminToolInfo{
			{Name: "exec", Description: "Execute commands"},
			{Name: "memory", Description: "Memory operations"},
		},
	})
	t.SetSystem(&mockSystemService{})
	t.SetUsers(&mockUserService{
		users: []AdminUserInfo{
			{ID: "u1", Username: "admin", Role: "admin"},
		},
	})
	t.SetAPIKeys(&mockAPIKeyService{
		keys: []AdminAPIKeyInfo{
			{ID: "k1", Name: "default", Prefix: "sk-abc1"},
		},
	})
	return t
}

func parseResult(t *testing.T, result interface{}) map[string]interface{} {
	t.Helper()
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		// Try array
		var arr []interface{}
		if err2 := json.Unmarshal([]byte(s), &arr); err2 != nil {
			t.Fatalf("failed to parse result JSON: %v (raw: %s)", err, s)
		}
		return map[string]interface{}{"_array": arr}
	}
	return m
}

// --- Tests ---

func TestMgmtTool_Definition(t *testing.T) {
	tool := NewMgmtTool()
	def := tool.Definition()
	if def.Name != "mgmt" {
		t.Errorf("expected name 'mgmt', got %q", def.Name)
	}
	if def.Icon != "settings" {
		t.Errorf("expected icon 'settings', got %q", def.Icon)
	}
	params := def.Parameters
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties in parameters")
	}
	if _, ok := props["action"]; !ok {
		t.Error("expected 'action' in properties")
	}
}

func TestMgmtTool_MissingAction(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing action")
	}
}

func TestMgmtTool_InvalidActionFormat(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "nope"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for invalid action format")
	}
}

func TestMgmtTool_ProvidersListAction(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "providers.list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	arr, ok := m["_array"].([]interface{})
	if !ok {
		t.Fatal("expected array result for providers.list")
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 providers, got %d", len(arr))
	}
}

func TestMgmtTool_ProvidersAdd(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":        "providers.add",
		"name":          "MyProvider",
		"provider_type": "openai",
		"base_url":      "https://api.example.com",
		"api_key":       "sk-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["name"] != "MyProvider" {
		t.Errorf("expected name 'MyProvider', got %v", m["name"])
	}
}

func TestMgmtTool_SupportsNestedCamelCaseArgs(t *testing.T) {
	tool := newTestMgmtTool()

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":       "providers.add",
			"name":         "MyProvider",
			"providerType": "openai",
			"baseUrl":      "https://api.example.com",
			"apiKey":       "sk-test",
			"location":     "local",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["location"] != "local" || m["name"] != "MyProvider" {
		t.Fatalf("unexpected provider payload: %#v", m)
	}

	result, err = tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":     "providers.add_key",
			"providerId": "openai",
			"apiKey":     "sk-added",
		},
	})
	if err != nil {
		t.Fatalf("unexpected add_key error: %v", err)
	}
	m = parseResult(t, result)
	if _, ok := m["api_keys"]; !ok {
		t.Fatalf("expected api_keys in result, got %#v", m)
	}

	result, err = tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":       "settings.set",
			"settingKey":   "site_name",
			"settingValue": "Blue",
		},
	})
	if err != nil {
		t.Fatalf("unexpected settings.set error: %v", err)
	}
	m = parseResult(t, result)
	if m["success"] != true {
		t.Fatalf("expected success response, got %#v", m)
	}
}

func TestMgmtTool_ProvidersAddMissingParams(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "providers.add",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing params")
	}
}

func TestMgmtTool_SettingsGet(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "settings.get"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["locale"] != "zh-CN" {
		t.Errorf("expected locale 'zh-CN', got %v", m["locale"])
	}
}

func TestMgmtTool_SettingsSet(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "settings.set",
		"key":    "locale",
		"value":  "en-US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["success"] != true {
		t.Errorf("expected success=true, got %v", m["success"])
	}
}

func TestMgmtTool_SkillsList(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "skills.list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	arr, ok := m["_array"].([]interface{})
	if !ok {
		t.Fatal("expected array result")
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 skill, got %d", len(arr))
	}
}

func TestMgmtTool_ToolsList(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "tools.list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	arr, ok := m["_array"].([]interface{})
	if !ok {
		t.Fatal("expected array result")
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 tools, got %d", len(arr))
	}
}

func TestMgmtTool_SystemHealth(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "system.health"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["version"] != "0.10.33" {
		t.Errorf("expected version '0.10.33', got %v", m["version"])
	}
	if m["goroutines"] != float64(42) {
		t.Errorf("expected goroutines=42, got %v", m["goroutines"])
	}
}

func TestMgmtTool_SystemVersion(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "system.version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["version"] != "0.10.33" {
		t.Errorf("expected version '0.10.33', got %v", m["version"])
	}
}

func TestMgmtTool_UsersList(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "users.list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	arr, ok := m["_array"].([]interface{})
	if !ok {
		t.Fatal("expected array result")
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 user, got %d", len(arr))
	}
}

func TestMgmtTool_APIKeysList(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "apikeys.list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	arr, ok := m["_array"].([]interface{})
	if !ok {
		t.Fatal("expected array result")
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 key, got %d", len(arr))
	}
}

func TestMgmtTool_APIKeysCreate(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "apikeys.create",
		"name":   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["name"] != "test-key" {
		t.Errorf("expected name 'test-key', got %v", m["name"])
	}
	if m["key"] == nil || m["key"] == "" {
		t.Error("expected key to be returned on create")
	}
}

func TestMgmtTool_UnknownDomain(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "foo.bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for unknown domain")
	}
}

func TestMgmtTool_NilService(t *testing.T) {
	tool := NewMgmtTool() // no services wired
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "providers.list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error when service is nil")
	}
}

func TestMgmtTool_ProvidersModels(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "providers.models"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	arr, ok := m["_array"].([]interface{})
	if !ok {
		t.Fatal("expected array result")
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 model, got %d", len(arr))
	}
}

func TestMgmtTool_ProvidersAddWithLocation(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":        "providers.add",
		"name":          "LocalLLM",
		"provider_type": "ollama",
		"base_url":      "http://localhost:11434",
		"location":      "local",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["name"] != "LocalLLM" {
		t.Errorf("expected name 'LocalLLM', got %v", m["name"])
	}
	if m["location"] != "local" {
		t.Errorf("expected location 'local', got %v", m["location"])
	}
}

func TestMgmtTool_ProvidersAddKey(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "providers.add_key",
		"id":      "openai",
		"api_key": "sk-new-key-12345",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["id"] != "openai" {
		t.Errorf("expected id 'openai', got %v", m["id"])
	}
	keys, ok := m["api_keys"].([]interface{})
	if !ok {
		t.Fatal("expected api_keys array in result")
	}
	if len(keys) == 0 {
		t.Error("expected at least one API key after add_key")
	}
}

func TestMgmtTool_ProvidersAddKeyMissingID(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "providers.add_key",
		"api_key": "sk-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing id")
	}
}

func TestMgmtTool_ProvidersAddKeyMissingKey(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "providers.add_key",
		"id":     "openai",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing api_key")
	}
}

func TestMgmtTool_ProvidersAddKeyNotFound(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "providers.add_key",
		"id":      "nonexistent",
		"api_key": "sk-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for nonexistent provider")
	}
}

func TestMgmtTool_ProvidersTest(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "providers.test",
		"id":     "openai",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["status"] != "active" {
		t.Errorf("expected status 'active', got %v", m["status"])
	}
}

func TestMgmtTool_ProvidersTestMissingID(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "providers.test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing id")
	}
}

func TestMgmtTool_ProvidersEnableDisable(t *testing.T) {
	tool := newTestMgmtTool()
	for _, op := range []string{"enable", "disable"} {
		t.Run(op, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]interface{}{
				"action": "providers." + op,
				"id":     "openai",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := parseResult(t, result)
			if m["success"] != true {
				t.Errorf("expected success=true for providers.%s", op)
			}
		})
	}
}

func TestMgmtTool_ProvidersRemove(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "providers.remove",
		"id":     "openai",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["success"] != true {
		t.Error("expected success=true for providers.remove")
	}
}

func TestMgmtTool_ProvidersRemoveMissingID(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "providers.remove",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing id")
	}
}

func TestMgmtTool_UnknownOperations(t *testing.T) {
	tool := newTestMgmtTool()
	actions := []string{
		"providers.unknown",
		"settings.unknown",
		"channels.unknown",
		"skills.unknown",
		"tools.unknown",
		"system.unknown",
		"proxy.unknown",
		"users.unknown",
		"apikeys.unknown",
	}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]interface{}{"action": action})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := parseResult(t, result)
			if _, ok := m["error"]; !ok {
				t.Errorf("expected error for unknown operation %s", action)
			}
		})
	}
}

func TestMgmtTool_NilServiceAllDomains(t *testing.T) {
	tool := NewMgmtTool() // no services wired
	actions := []string{
		"providers.list", "settings.get", "channels.list",
		"skills.list", "tools.list", "system.health",
		"proxy.stats", "users.list", "apikeys.list",
	}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]interface{}{"action": action})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := parseResult(t, result)
			if _, ok := m["error"]; !ok {
				t.Errorf("expected error for nil service on %s", action)
			}
		})
	}
}

func TestMgmtTool_SettingsSetMissingKey(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "settings.set",
		"value":  "en-US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing key")
	}
}

func TestMgmtTool_ChannelsStatus(t *testing.T) {
	tool := newTestMgmtTool()
	// channels service is nil in default test tool, so expect error
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "channels.list",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for nil channels service")
	}
}

func TestMgmtTool_ChannelsStatusMissingName(t *testing.T) {
	tool := NewMgmtTool()
	tool.SetChannels(&mockChannelService{})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "channels.status",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing name")
	}
}

func TestMgmtTool_SkillsEnableDisable(t *testing.T) {
	tool := newTestMgmtTool()
	for _, op := range []string{"enable", "disable"} {
		t.Run(op, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]interface{}{
				"action": "skills." + op,
				"id":     "push_notification",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := parseResult(t, result)
			if m["success"] != true {
				t.Errorf("expected success=true for skills.%s", op)
			}
		})
	}
}

func TestMgmtTool_SkillsEnableMissingID(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "skills.enable",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing id")
	}
}

func TestMgmtTool_ToolsEnableDisable(t *testing.T) {
	tool := newTestMgmtTool()
	for _, op := range []string{"enable", "disable"} {
		t.Run(op, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]interface{}{
				"action": "tools." + op,
				"name":   "exec",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := parseResult(t, result)
			if m["success"] != true {
				t.Errorf("expected success=true for tools.%s", op)
			}
		})
	}
}

func TestMgmtTool_ToolsEnableMissingName(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "tools.enable",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing name")
	}
}

func TestMgmtTool_UsersLockUnlock(t *testing.T) {
	tool := newTestMgmtTool()
	for _, op := range []string{"lock", "unlock"} {
		t.Run(op, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]interface{}{
				"action": "users." + op,
				"id":     "u1",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := parseResult(t, result)
			if m["success"] != true {
				t.Errorf("expected success=true for users.%s", op)
			}
		})
	}
}

func TestMgmtTool_UsersLockMissingID(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "users.lock",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing id")
	}
}

func TestMgmtTool_APIKeysCreateMissingName(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "apikeys.create",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing name")
	}
}

func TestMgmtTool_APIKeysRevoke(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "apikeys.revoke",
		"id":     "k1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["success"] != true {
		t.Error("expected success=true for apikeys.revoke")
	}
}

func TestMgmtTool_APIKeysRevokeMissingID(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "apikeys.revoke",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if _, ok := m["error"]; !ok {
		t.Error("expected error for missing id")
	}
}

func TestMgmtTool_ProxyStats(t *testing.T) {
	tool := NewMgmtTool()
	tool.SetProxy(&mockProxyService{})
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "proxy.stats"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["total_requests"] != float64(100) {
		t.Errorf("expected total_requests=100, got %v", m["total_requests"])
	}
}

func TestMgmtTool_ProxyCacheStats(t *testing.T) {
	tool := NewMgmtTool()
	tool.SetProxy(&mockProxyService{})
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "proxy.cache_stats"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["cache_hits"] != float64(50) {
		t.Errorf("expected cache_hits=50, got %v", m["cache_hits"])
	}
}

func TestMgmtTool_SystemInfo(t *testing.T) {
	tool := newTestMgmtTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "system.info"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := parseResult(t, result)
	if m["version"] != "0.10.33" {
		t.Errorf("expected version '0.10.33', got %v", m["version"])
	}
}

// --- Additional mock services ---

type mockChannelService struct{}

func (m *mockChannelService) ListChannels(_ context.Context) ([]AdminChannelInfo, error) {
	return []AdminChannelInfo{{Name: "imessage", Type: "imessage", Status: "connected"}}, nil
}
func (m *mockChannelService) GetStatus(_ context.Context, name string) (*AdminChannelInfo, error) {
	return &AdminChannelInfo{Name: name, Type: "imessage", Status: "connected"}, nil
}

type mockProxyService struct{}

func (m *mockProxyService) Stats(_ context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{"total_requests": 100, "active_connections": 5}, nil
}
func (m *mockProxyService) CacheStats(_ context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{"cache_hits": 50, "cache_misses": 10}, nil
}
