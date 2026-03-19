package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

func TestChannelConfigStore_PersistsSettings(t *testing.T) {
	t.Parallel()

	kv := kvstore.NewMemoryStore()
	store := NewChannelConfigStore(kv)

	settings := ChannelSettings{
		GroupAccess: channel.GroupAccessConfig{
			Policy: channel.GroupPolicyAllowlist,
			AllowedChatIDs: map[string][]string{
				"feishu": {"oc_allowed"},
			},
		},
	}
	if err := store.SetSettings(settings); err != nil {
		t.Fatalf("set settings: %v", err)
	}

	reloaded := NewChannelConfigStore(kv)
	got, ok := reloaded.GetSettings()
	if !ok {
		t.Fatal("expected persisted settings to be present")
	}
	if got.GroupAccess.Policy != channel.GroupPolicyAllowlist {
		t.Fatalf("policy = %q, want %q", got.GroupAccess.Policy, channel.GroupPolicyAllowlist)
	}
	if len(got.GroupAccess.AllowedChatIDs["feishu"]) != 1 || got.GroupAccess.AllowedChatIDs["feishu"][0] != "oc_allowed" {
		t.Fatalf("unexpected allowed chat IDs: %#v", got.GroupAccess.AllowedChatIDs)
	}
}

func TestApplyChannelSettings_OverridesGroupAccess(t *testing.T) {
	t.Parallel()

	cfg := channel.DefaultConfig()
	cfg.GroupAccess.Policy = channel.GroupPolicyDisabled

	settings := ChannelSettings{
		GroupAccess: channel.GroupAccessConfig{
			Policy: channel.GroupPolicyAllowlist,
			AllowedChatIDs: map[string][]string{
				"feishu": {"oc_allowed"},
			},
		},
	}

	got := ApplyChannelSettings(cfg, settings)
	if got.GroupAccess.Policy != channel.GroupPolicyAllowlist {
		t.Fatalf("policy = %q, want %q", got.GroupAccess.Policy, channel.GroupPolicyAllowlist)
	}
	if len(got.GroupAccess.AllowedChatIDs["feishu"]) != 1 || got.GroupAccess.AllowedChatIDs["feishu"][0] != "oc_allowed" {
		t.Fatalf("unexpected allowed chat IDs: %#v", got.GroupAccess.AllowedChatIDs)
	}
}

func TestChannelConfigHandler_UpdateSettings_AppliesRuntimeGroupAccess(t *testing.T) {
	t.Parallel()

	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	handler := NewChannelConfigHandler(store)
	handler.SetManager(manager)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/channels/settings", strings.NewReader(`{"group_access":{"policy":"allowlist","allowed_chat_ids":{"feishu":["oc_allowed"]}}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.UpdateSettings(c); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	gotRuntime := manager.GetGroupAccess()
	if gotRuntime.Policy != channel.GroupPolicyAllowlist {
		t.Fatalf("runtime policy = %q, want %q", gotRuntime.Policy, channel.GroupPolicyAllowlist)
	}
	if len(gotRuntime.AllowedChatIDs["feishu"]) != 1 || gotRuntime.AllowedChatIDs["feishu"][0] != "oc_allowed" {
		t.Fatalf("unexpected runtime allowed chat IDs: %#v", gotRuntime.AllowedChatIDs)
	}

	gotStored, ok := store.GetSettings()
	if !ok {
		t.Fatal("expected settings to be persisted")
	}
	if gotStored.GroupAccess.Policy != channel.GroupPolicyAllowlist {
		t.Fatalf("stored policy = %q, want %q", gotStored.GroupAccess.Policy, channel.GroupPolicyAllowlist)
	}
}

func TestChannelConfigHandler_GetSettings_ReturnsRuntimePolicy(t *testing.T) {
	t.Parallel()

	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	manager.SetGroupAccess(channel.GroupAccessConfig{
		Policy: channel.GroupPolicyDisabled,
	})

	handler := NewChannelConfigHandler(store)
	handler.SetManager(manager)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/channels/settings", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetSettings(c); err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got ChannelSettings
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode settings response: %v", err)
	}
	if got.GroupAccess.Policy != channel.GroupPolicyDisabled {
		t.Fatalf("policy = %q, want %q", got.GroupAccess.Policy, channel.GroupPolicyDisabled)
	}
}

func TestChannelConfigHandler_UpdateSettings_RejectsInvalidPolicy(t *testing.T) {
	t.Parallel()

	handler := NewChannelConfigHandler(NewChannelConfigStore(kvstore.NewMemoryStore()))
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/channels/settings", strings.NewReader(`{"group_access":{"policy":"nope"}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.UpdateSettings(c)
	if err == nil {
		t.Fatal("expected validation error")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTP error, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestChannelConfigStore_GetSettings_DefaultMissing(t *testing.T) {
	t.Parallel()

	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	got, ok := store.GetSettings()
	if ok {
		t.Fatal("expected no stored settings by default")
	}
	if got.GroupAccess.Policy != channel.GroupPolicyOpen {
		t.Fatalf("policy = %q, want %q", got.GroupAccess.Policy, channel.GroupPolicyOpen)
	}
}

func TestChannelConfigStore_RawPersistenceAccessible(t *testing.T) {
	t.Parallel()

	kv := kvstore.NewMemoryStore()
	store := NewChannelConfigStore(kv)
	if err := store.SetSettings(ChannelSettings{
		GroupAccess: channel.GroupAccessConfig{
			Policy: channel.GroupPolicyAllowlist,
			AllowedChatIDs: map[string][]string{
				"feishu": {"oc_allowed"},
			},
		},
	}); err != nil {
		t.Fatalf("set settings: %v", err)
	}

	var raw ChannelSettings
	if err := kv.GetJSON(context.Background(), channelSettingsKVKey, &raw); err != nil {
		t.Fatalf("load raw settings: %v", err)
	}
	if raw.GroupAccess.Policy != channel.GroupPolicyAllowlist {
		t.Fatalf("raw policy = %q, want %q", raw.GroupAccess.Policy, channel.GroupPolicyAllowlist)
	}
}
