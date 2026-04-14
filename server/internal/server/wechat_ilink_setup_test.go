package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

type stubWechatILinkSetupTunnelRuntime struct {
	url   string
	err   error
	calls int
}

func (s *stubWechatILinkSetupTunnelRuntime) EnsureTunnelURL(_ context.Context) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	return s.url, nil
}

func TestWeChatILinkSetupHandler_CreateSessionReturnsQRCode(t *testing.T) {
	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	handler := NewWeChatILinkSetupHandler(
		store,
		channel.NewManager(channel.DefaultConfig(), zap.NewNop()),
		NewChannelFactory(zap.NewNop()),
		&stubWechatILinkSetupTunnelRuntime{url: "https://blue.example.com"},
		auth.NewJWTService(&auth.JWTConfig{
			Secret:     "01234567890123456789012345678901",
			Expiration: time.Hour,
			Issuer:     "test",
		}),
		zap.NewNop(),
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/wechat_ilink/setup/session", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID:   "user-1",
		Username: "user-1",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.CreateSession(c); err != nil {
		t.Fatalf("CreateSession error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "pending" {
		t.Fatalf("status = %v, want pending", resp["status"])
	}
	if sessionID, _ := resp["session_id"].(string); strings.TrimSpace(sessionID) == "" {
		t.Fatal("expected session_id")
	}
	if mobileURL, _ := resp["mobile_url"].(string); !strings.Contains(mobileURL, "/channels/setup/wechat_ilink") {
		t.Fatalf("mobile_url = %q, want setup path", mobileURL)
	}
	if qrcode, _ := resp["qrcode"].(string); strings.TrimSpace(qrcode) == "" {
		t.Fatal("expected qrcode")
	}
}

func TestWeChatILinkSetupHandler_CompletePersistsAndEnablesChannel(t *testing.T) {
	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	factory := NewChannelFactory(zap.NewNop())
	handler := NewWeChatILinkSetupHandler(
		store,
		manager,
		factory,
		&stubWechatILinkSetupTunnelRuntime{url: "https://blue.example.com"},
		auth.NewJWTService(&auth.JWTConfig{
			Secret:     "01234567890123456789012345678901",
			Expiration: time.Hour,
			Issuer:     "test",
		}),
		zap.NewNop(),
	)
	handler.now = func() time.Time { return time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC) }

	session := handler.newSession("user-1")
	handler.storeSession(session)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getupdates":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret":             0,
				"msgs":            []any{},
				"get_updates_buf": "cursor-1",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	body := `{"pairing_payload":{"api_base_url":"` + server.URL + `","bot_token":"bot-token"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/wechat_ilink/setup/session/"+session.ID+"/complete", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID:   "user-1",
		Username: "user-1",
	}))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(session.ID)

	if err := handler.CompleteSession(c); err != nil {
		t.Fatalf("CompleteSession error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, ok := store.Get("wechat_ilink")
	if !ok {
		t.Fatal("expected persisted wechat_ilink config")
	}
	if !cfg.Enabled {
		t.Fatal("expected wechat_ilink config to be enabled")
	}
	if cfg.Config["api_base_url"] != server.URL {
		t.Fatalf("api_base_url = %q, want %q", cfg.Config["api_base_url"], server.URL)
	}
	if cfg.Config["bot_token"] != "bot-token" {
		t.Fatalf("bot_token = %q, want %q", cfg.Config["bot_token"], "bot-token")
	}
	if got, exists := manager.Get("wechat_ilink"); !exists {
		t.Fatal("expected wechat_ilink channel to be registered")
	} else if got.Info().Status != channel.StatusConnected {
		t.Fatalf("status = %q, want %q", got.Info().Status, channel.StatusConnected)
	}
}
