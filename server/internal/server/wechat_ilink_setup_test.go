package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

type wechatILinkRewriteHostTransport struct {
	t      *testing.T
	target *url.URL
}

func (r wechatILinkRewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = r.target.Scheme
	cloned.URL.Host = r.target.Host
	cloned.Host = req.URL.Host
	return http.DefaultTransport.RoundTrip(cloned)
}

func newWeChatILinkRewriteHostTransport(t *testing.T, server *httptest.Server) http.RoundTripper {
	t.Helper()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(server.URL) error = %v", err)
	}
	return wechatILinkRewriteHostTransport{t: t, target: target}
}

func TestWeChatILinkSetupHandler_CreateSessionReturnsUpstreamScanURLWithoutQRCodeImage(t *testing.T) {
	var qrRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			qrRequests++
			if got := r.URL.Query().Get("bot_type"); got != "3" {
				t.Fatalf("bot_type = %q, want %q", got, "3")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"qrcode":             "qr-key-1",
				"qrcode_img_content": "https://ilink.example.com/scan/abc",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	if err := store.Set(wechatILinkSetupChannelID, &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: false,
		Config: map[string]string{
			"api_base_url": upstream.URL,
		},
	}); err != nil {
		t.Fatalf("store.Set error = %v", err)
	}

	handler := NewWeChatILinkSetupHandler(
		store,
		channel.NewManager(channel.DefaultConfig(), zap.NewNop()),
		NewChannelFactory(zap.NewNop()),
		zap.NewNop(),
	)

	response := callWeChatILinkCreateSession(t, handler)

	if qrRequests != 1 {
		t.Fatalf("qrRequests = %d, want 1", qrRequests)
	}
	if got := strings.TrimSpace(stringValue(response["session_id"])); got == "" {
		t.Fatal("expected session_id")
	}
	if got := stringValue(response["status"]); got != wechatILinkSessionStatusPending {
		t.Fatalf("status = %q, want %q", got, wechatILinkSessionStatusPending)
	}
	if got := stringValue(response["scan_url"]); got != "https://ilink.example.com/scan/abc" {
		t.Fatalf("scan_url = %q, want %q", got, "https://ilink.example.com/scan/abc")
	}
	if got := stringValue(response["mobile_url"]); got != "https://ilink.example.com/scan/abc" {
		t.Fatalf("mobile_url = %q, want %q", got, "https://ilink.example.com/scan/abc")
	}
	if strings.Contains(stringValue(response["mobile_url"]), "/channels/setup/wechat_ilink") {
		t.Fatalf("mobile_url = %q, should not point at Blue setup page", stringValue(response["mobile_url"]))
	}
	if _, ok := response["qrcode"]; ok {
		t.Fatal("response should not include qrcode image data")
	}

	session, ok := handler.getSession(stringValue(response["session_id"]))
	if !ok {
		t.Fatal("expected stored setup session")
	}
	if session.QRKey != "qr-key-1" {
		t.Fatalf("session.QRKey = %q, want %q", session.QRKey, "qr-key-1")
	}
	if session.ScanURL != "https://ilink.example.com/scan/abc" {
		t.Fatalf("session.ScanURL = %q, want %q", session.ScanURL, "https://ilink.example.com/scan/abc")
	}
	if session.ResolvedAPIBaseURL != upstream.URL {
		t.Fatalf("session.ResolvedAPIBaseURL = %q, want %q", session.ResolvedAPIBaseURL, upstream.URL)
	}
}

func TestWeChatILinkSetupHandler_CreateSessionFallsBackToDefaultAPIBaseURLWhenStoredValueIsRelative(t *testing.T) {
	var qrRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/get_bot_qrcode" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		qrRequests++
		if got := r.Host; got != "ilinkai.weixin.qq.com" {
			t.Fatalf("host = %q, want %q", got, "ilinkai.weixin.qq.com")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"qrcode":             "qr-key-default",
			"qrcode_img_content": "https://ilink.example.com/scan/default",
		})
	}))
	defer upstream.Close()

	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	if err := store.Set(wechatILinkSetupChannelID, &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: false,
		Config: map[string]string{
			"api_base_url": "admin",
		},
	}); err != nil {
		t.Fatalf("store.Set error = %v", err)
	}

	handler := NewWeChatILinkSetupHandler(
		store,
		channel.NewManager(channel.DefaultConfig(), zap.NewNop()),
		NewChannelFactory(zap.NewNop()),
		zap.NewNop(),
	)
	handler.httpClient = &http.Client{Transport: newWeChatILinkRewriteHostTransport(t, upstream)}

	response := callWeChatILinkCreateSession(t, handler)

	if qrRequests != 1 {
		t.Fatalf("qrRequests = %d, want 1", qrRequests)
	}
	if got := stringValue(response["scan_url"]); got != "https://ilink.example.com/scan/default" {
		t.Fatalf("scan_url = %q, want %q", got, "https://ilink.example.com/scan/default")
	}

	session, ok := handler.getSession(stringValue(response["session_id"]))
	if !ok {
		t.Fatal("expected stored setup session")
	}
	if session.ResolvedAPIBaseURL != wechatILinkDefaultAPIBaseURL {
		t.Fatalf("session.ResolvedAPIBaseURL = %q, want %q", session.ResolvedAPIBaseURL, wechatILinkDefaultAPIBaseURL)
	}
}

func TestWeChatILinkSetupHandler_CreateSessionAcceptsStoredBotAPIBaseURL(t *testing.T) {
	var qrRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			qrRequests++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"qrcode":             "qr-key-compat",
				"qrcode_img_content": "https://ilink.example.com/scan/compat",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	if err := store.Set(wechatILinkSetupChannelID, &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: false,
		Config: map[string]string{
			"api_base_url": upstream.URL + "/ilink/bot",
		},
	}); err != nil {
		t.Fatalf("store.Set error = %v", err)
	}

	handler := NewWeChatILinkSetupHandler(
		store,
		channel.NewManager(channel.DefaultConfig(), zap.NewNop()),
		NewChannelFactory(zap.NewNop()),
		zap.NewNop(),
	)

	response := callWeChatILinkCreateSession(t, handler)

	if qrRequests != 1 {
		t.Fatalf("qrRequests = %d, want 1", qrRequests)
	}
	if got := stringValue(response["scan_url"]); got != "https://ilink.example.com/scan/compat" {
		t.Fatalf("scan_url = %q, want %q", got, "https://ilink.example.com/scan/compat")
	}

	session, ok := handler.getSession(stringValue(response["session_id"]))
	if !ok {
		t.Fatal("expected stored setup session")
	}
	if session.ResolvedAPIBaseURL != upstream.URL {
		t.Fatalf("session.ResolvedAPIBaseURL = %q, want %q", session.ResolvedAPIBaseURL, upstream.URL)
	}
}

func TestWeChatILinkSetupHandler_GetSessionMapsWaitToPending(t *testing.T) {
	var pollHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/get_qrcode_status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pollHeader = r.Header.Get("iLink-App-ClientVersion")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "wait",
		})
	}))
	defer upstream.Close()

	handler := newWeChatILinkSetupHandlerForTest()
	session := handler.newSession("user-1")
	session.QRKey = "qr-key-1"
	session.ScanURL = "https://ilink.example.com/scan/abc"
	session.ResolvedAPIBaseURL = upstream.URL
	handler.storeSession(session)

	response := callWeChatILinkGetSession(t, handler, session.ID)

	if pollHeader != "1" {
		t.Fatalf("iLink-App-ClientVersion = %q, want %q", pollHeader, "1")
	}
	if got := stringValue(response["status"]); got != wechatILinkSessionStatusPending {
		t.Fatalf("status = %q, want %q", got, wechatILinkSessionStatusPending)
	}
	if got := stringValue(response["scan_url"]); got != session.ScanURL {
		t.Fatalf("scan_url = %q, want %q", got, session.ScanURL)
	}
	if got := stringValue(response["mobile_url"]); got != session.ScanURL {
		t.Fatalf("mobile_url = %q, want %q", got, session.ScanURL)
	}
}

func TestWeChatILinkSetupHandler_GetSessionMapsScanedToAuthorizing(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/get_qrcode_status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "scaned",
		})
	}))
	defer upstream.Close()

	handler := newWeChatILinkSetupHandlerForTest()
	session := handler.newSession("user-1")
	session.QRKey = "qr-key-1"
	session.ResolvedAPIBaseURL = upstream.URL
	handler.storeSession(session)

	response := callWeChatILinkGetSession(t, handler, session.ID)

	if got := stringValue(response["status"]); got != wechatILinkSessionStatusAuthorizing {
		t.Fatalf("status = %q, want %q", got, wechatILinkSessionStatusAuthorizing)
	}
}

func TestWeChatILinkSetupHandler_GetSessionConfirmedAutoActivatesChannel(t *testing.T) {
	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	var getUpdatesCalls int
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_qrcode_status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":        "confirmed",
				"bot_token":     "bot-token",
				"ilink_bot_id":  "bot-1",
				"ilink_user_id": "user@im.wechat",
				"baseurl":       upstream.URL,
			})
		case "/ilink/bot/getupdates":
			getUpdatesCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret":             0,
				"msgs":            []any{},
				"get_updates_buf": "cursor-1",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	handler := NewWeChatILinkSetupHandler(
		store,
		manager,
		NewChannelFactory(zap.NewNop()),
		zap.NewNop(),
	)
	session := handler.newSession("user-1")
	session.QRKey = "qr-key-1"
	session.ResolvedAPIBaseURL = upstream.URL
	handler.storeSession(session)

	response := callWeChatILinkGetSession(t, handler, session.ID)

	if getUpdatesCalls != 1 {
		t.Fatalf("getUpdatesCalls = %d, want 1", getUpdatesCalls)
	}
	if got := stringValue(response["status"]); got != wechatILinkSessionStatusConnected {
		t.Fatalf("status = %q, want %q", got, wechatILinkSessionStatusConnected)
	}
	if got := stringValue(response["message"]); got != "configured" {
		t.Fatalf("message = %q, want %q", got, "configured")
	}
	cfg, ok := store.Get(wechatILinkSetupChannelID)
	if !ok {
		t.Fatal("expected persisted wechat_ilink config")
	}
	if cfg.Config["api_base_url"] != upstream.URL {
		t.Fatalf("api_base_url = %q, want %q", cfg.Config["api_base_url"], upstream.URL)
	}
	if cfg.Config["bot_token"] != "bot-token" {
		t.Fatalf("bot_token = %q, want %q", cfg.Config["bot_token"], "bot-token")
	}
	if cfg.Config["user_id"] != "user@im.wechat" {
		t.Fatalf("user_id = %q, want %q", cfg.Config["user_id"], "user@im.wechat")
	}
	if got, exists := manager.Get(wechatILinkSetupChannelID); !exists {
		t.Fatal("expected wechat_ilink channel to be registered")
	} else if got.Info().Status != channel.StatusConnected {
		t.Fatalf("status = %q, want %q", got.Info().Status, channel.StatusConnected)
	}
}

func TestWeChatILinkSetupHandler_EndToEndCreateThenConfirmedConnectsDespiteInitialGetUpdatesEOF(t *testing.T) {
	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	var getUpdatesCalls atomic.Int32
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"qrcode":             "qr-key-e2e",
				"qrcode_img_content": "https://ilink.example.com/scan/e2e",
			})
		case "/ilink/bot/get_qrcode_status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":        "confirmed",
				"bot_token":     "bot-token-e2e",
				"ilink_bot_id":  "bot-e2e",
				"ilink_user_id": "user@im.wechat",
				"baseurl":       upstream.URL,
			})
		case "/ilink/bot/getupdates":
			call := getUpdatesCalls.Add(1)
			if call == 1 {
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("response writer does not support hijacking")
				}
				conn, _, err := hj.Hijack()
				if err != nil {
					t.Fatalf("Hijack error = %v", err)
				}
				_ = conn.Close()
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret":                    0,
				"msgs":                   []any{},
				"get_updates_buf":        "cursor-e2e",
				"longpolling_timeout_ms": 1,
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	handler := NewWeChatILinkSetupHandler(
		store,
		manager,
		NewChannelFactory(zap.NewNop()),
		zap.NewNop(),
	)
	if err := store.Set(wechatILinkSetupChannelID, &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: false,
		Config: map[string]string{
			"api_base_url": upstream.URL,
		},
	}); err != nil {
		t.Fatalf("store.Set error = %v", err)
	}

	createResp := callWeChatILinkCreateSession(t, handler)
	sessionID := stringValue(createResp["session_id"])
	if sessionID == "" {
		t.Fatal("expected session_id from CreateSession")
	}
	if got := stringValue(createResp["status"]); got != wechatILinkSessionStatusPending {
		t.Fatalf("create status = %q, want %q", got, wechatILinkSessionStatusPending)
	}
	if got := stringValue(createResp["scan_url"]); got != "https://ilink.example.com/scan/e2e" {
		t.Fatalf("scan_url = %q, want %q", got, "https://ilink.example.com/scan/e2e")
	}

	getResp := callWeChatILinkGetSession(t, handler, sessionID)
	if got := stringValue(getResp["status"]); got != wechatILinkSessionStatusConnected {
		t.Fatalf("poll status = %q, want %q", got, wechatILinkSessionStatusConnected)
	}
	if got := stringValue(getResp["message"]); got != "configured" {
		t.Fatalf("message = %q, want %q", got, "configured")
	}

	deadline := time.Now().Add(2 * time.Second)
	for getUpdatesCalls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := getUpdatesCalls.Load(); got < 2 {
		t.Fatalf("getUpdatesCalls = %d, want at least 2 to prove background poller continued after EOF", got)
	}

	cfg, ok := store.Get(wechatILinkSetupChannelID)
	if !ok {
		t.Fatal("expected persisted wechat_ilink config")
	}
	if cfg.Config["api_base_url"] != upstream.URL {
		t.Fatalf("api_base_url = %q, want %q", cfg.Config["api_base_url"], upstream.URL)
	}
	if cfg.Config["bot_token"] != "bot-token-e2e" {
		t.Fatalf("bot_token = %q, want %q", cfg.Config["bot_token"], "bot-token-e2e")
	}
	if cfg.Config["user_id"] != "user@im.wechat" {
		t.Fatalf("user_id = %q, want %q", cfg.Config["user_id"], "user@im.wechat")
	}
	if got, exists := manager.Get(wechatILinkSetupChannelID); !exists {
		t.Fatal("expected wechat_ilink channel to be registered")
	} else {
		defer func() {
			_ = manager.StopChannel(context.Background(), wechatILinkSetupChannelID)
			_ = manager.Unregister(wechatILinkSetupChannelID)
		}()
		if got.Info().Status != channel.StatusConnected {
			t.Fatalf("status = %q, want %q", got.Info().Status, channel.StatusConnected)
		}
	}
}

func TestWeChatILinkSetupHandler_HTTPRoutesEndToEndCreateThenConfirmedConnects(t *testing.T) {
	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	var getUpdatesCalls atomic.Int32
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"qrcode":             "qr-key-http-e2e",
				"qrcode_img_content": "https://ilink.example.com/scan/http-e2e",
			})
		case "/ilink/bot/get_qrcode_status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":        "confirmed",
				"bot_token":     "bot-token-http-e2e",
				"ilink_bot_id":  "bot-http-e2e",
				"ilink_user_id": "user@im.wechat",
				"baseurl":       upstream.URL,
			})
		case "/ilink/bot/getupdates":
			call := getUpdatesCalls.Add(1)
			if call == 1 {
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("response writer does not support hijacking")
				}
				conn, _, err := hj.Hijack()
				if err != nil {
					t.Fatalf("Hijack error = %v", err)
				}
				_ = conn.Close()
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret":                    0,
				"msgs":                   []any{},
				"get_updates_buf":        "cursor-http-e2e",
				"longpolling_timeout_ms": 1,
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	if err := store.Set(wechatILinkSetupChannelID, &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: false,
		Config: map[string]string{
			"api_base_url": upstream.URL,
		},
	}); err != nil {
		t.Fatalf("store.Set error = %v", err)
	}

	handler := NewWeChatILinkSetupHandler(
		store,
		manager,
		NewChannelFactory(zap.NewNop()),
		zap.NewNop(),
	)

	e := echo.New()
	api := e.Group("/api/v1")
	api.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request().WithContext(context.WithValue(c.Request().Context(), auth.UserContextKey, &auth.UserClaims{
				UserID:   "user-1",
				Username: "user-1",
			}))
			c.SetRequest(req)
			return next(c)
		}
	})
	handler.RegisterRoutes(api)

	server := httptest.NewServer(e)
	defer server.Close()

	createResp, err := http.Post(server.URL+"/api/v1/channels/wechat_ilink/setup/session", "application/json", nil)
	if err != nil {
		t.Fatalf("create session request error = %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("create status = %d, want %d body=%s", createResp.StatusCode, http.StatusOK, string(body))
	}

	createBody, err := io.ReadAll(createResp.Body)
	if err != nil {
		t.Fatalf("read create response: %v", err)
	}
	createData := decodeWeChatILinkSetupResponse(t, createBody)
	sessionID := stringValue(createData["session_id"])
	if sessionID == "" {
		t.Fatal("expected session_id from create response")
	}
	if got := stringValue(createData["status"]); got != wechatILinkSessionStatusPending {
		t.Fatalf("create status = %q, want %q", got, wechatILinkSessionStatusPending)
	}

	getResp, err := http.Get(server.URL + "/api/v1/channels/wechat_ilink/setup/session/" + sessionID)
	if err != nil {
		t.Fatalf("get session request error = %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("get status = %d, want %d body=%s", getResp.StatusCode, http.StatusOK, string(body))
	}

	getBody, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read get response: %v", err)
	}
	getData := decodeWeChatILinkSetupResponse(t, getBody)
	if got := stringValue(getData["status"]); got != wechatILinkSessionStatusConnected {
		t.Fatalf("poll status = %q, want %q", got, wechatILinkSessionStatusConnected)
	}
	if got := stringValue(getData["message"]); got != "configured" {
		t.Fatalf("message = %q, want %q", got, "configured")
	}

	deadline := time.Now().Add(2 * time.Second)
	for getUpdatesCalls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := getUpdatesCalls.Load(); got < 2 {
		t.Fatalf("getUpdatesCalls = %d, want at least 2 to prove background poller continued after EOF", got)
	}

	if got, exists := manager.Get(wechatILinkSetupChannelID); !exists {
		t.Fatal("expected wechat_ilink channel to be registered")
	} else {
		defer func() {
			_ = manager.StopChannel(context.Background(), wechatILinkSetupChannelID)
			_ = manager.Unregister(wechatILinkSetupChannelID)
		}()
		if got.Info().Status != channel.StatusConnected {
			t.Fatalf("status = %q, want %q", got.Info().Status, channel.StatusConnected)
		}
	}
}

func TestWeChatILinkSetupHandler_GetSessionConfirmedDoesNotReactivate(t *testing.T) {
	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	var getUpdatesCalls int
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_qrcode_status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":       "confirmed",
				"bot_token":    "bot-token",
				"ilink_bot_id": "bot-1",
				"baseurl":      upstream.URL,
			})
		case "/ilink/bot/getupdates":
			getUpdatesCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret":             0,
				"msgs":            []any{},
				"get_updates_buf": "cursor-1",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	handler := NewWeChatILinkSetupHandler(
		store,
		manager,
		NewChannelFactory(zap.NewNop()),
		zap.NewNop(),
	)
	session := handler.newSession("user-1")
	session.QRKey = "qr-key-1"
	session.ResolvedAPIBaseURL = upstream.URL
	handler.storeSession(session)

	first := callWeChatILinkGetSession(t, handler, session.ID)
	second := callWeChatILinkGetSession(t, handler, session.ID)

	if getUpdatesCalls != 1 {
		t.Fatalf("getUpdatesCalls = %d, want 1", getUpdatesCalls)
	}
	if got := stringValue(first["status"]); got != wechatILinkSessionStatusConnected {
		t.Fatalf("first status = %q, want %q", got, wechatILinkSessionStatusConnected)
	}
	if got := stringValue(second["status"]); got != wechatILinkSessionStatusConnected {
		t.Fatalf("second status = %q, want %q", got, wechatILinkSessionStatusConnected)
	}
}

func TestWeChatILinkSetupHandler_GetSessionMapsExpiredToExpired(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/get_qrcode_status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "expired",
		})
	}))
	defer upstream.Close()

	handler := newWeChatILinkSetupHandlerForTest()
	session := handler.newSession("user-1")
	session.QRKey = "qr-key-1"
	session.ResolvedAPIBaseURL = upstream.URL
	handler.storeSession(session)

	response := callWeChatILinkGetSession(t, handler, session.ID)

	if got := stringValue(response["status"]); got != wechatILinkSessionStatusExpired {
		t.Fatalf("status = %q, want %q", got, wechatILinkSessionStatusExpired)
	}
	if got := stringValue(response["error"]); !strings.Contains(got, "expired") {
		t.Fatalf("error = %q, want expired message", got)
	}
}

func TestWeChatILinkSetupHandler_GetSessionMarksErrorWhenConfirmedPayloadMissingBotToken(t *testing.T) {
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/get_qrcode_status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "confirmed",
			"baseurl": upstream.URL,
		})
	}))
	defer upstream.Close()

	handler := newWeChatILinkSetupHandlerForTest()
	session := handler.newSession("user-1")
	session.QRKey = "qr-key-1"
	session.ResolvedAPIBaseURL = upstream.URL
	handler.storeSession(session)

	response := callWeChatILinkGetSession(t, handler, session.ID)

	if got := stringValue(response["status"]); got != wechatILinkSessionStatusError {
		t.Fatalf("status = %q, want %q", got, wechatILinkSessionStatusError)
	}
	if got := stringValue(response["error"]); !strings.Contains(got, "bot_token") {
		t.Fatalf("error = %q, want bot_token error", got)
	}
}

func TestWeChatILinkSetupHandler_GetSessionMarksErrorOnUpstreamHTTPFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer upstream.Close()

	handler := newWeChatILinkSetupHandlerForTest()
	session := handler.newSession("user-1")
	session.QRKey = "qr-key-1"
	session.ResolvedAPIBaseURL = upstream.URL
	handler.storeSession(session)

	response := callWeChatILinkGetSession(t, handler, session.ID)

	if got := stringValue(response["status"]); got != wechatILinkSessionStatusError {
		t.Fatalf("status = %q, want %q", got, wechatILinkSessionStatusError)
	}
	if got := stringValue(response["error"]); !strings.Contains(got, "http 502") {
		t.Fatalf("error = %q, want upstream http error", got)
	}
}

func TestWeChatILinkSetupHandler_GetSessionMarksErrorOnInvalidJSON(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer upstream.Close()

	handler := newWeChatILinkSetupHandlerForTest()
	session := handler.newSession("user-1")
	session.QRKey = "qr-key-1"
	session.ResolvedAPIBaseURL = upstream.URL
	handler.storeSession(session)

	response := callWeChatILinkGetSession(t, handler, session.ID)

	if got := stringValue(response["status"]); got != wechatILinkSessionStatusError {
		t.Fatalf("status = %q, want %q", got, wechatILinkSessionStatusError)
	}
	if got := stringValue(response["error"]); !strings.Contains(got, "decode") {
		t.Fatalf("error = %q, want decode error", got)
	}
}

func TestWeChatILinkSetupHandler_CompletePersistsAndEnablesChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getupdates":
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

	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	if err := store.Set(wechatILinkSetupChannelID, &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: false,
		Config: map[string]string{
			"api_base_url": server.URL,
		},
	}); err != nil {
		t.Fatalf("store.Set error = %v", err)
	}
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	factory := NewChannelFactory(zap.NewNop())
	handler := NewWeChatILinkSetupHandler(
		store,
		manager,
		factory,
		zap.NewNop(),
	)
	handler.now = func() time.Time { return time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC) }

	session := handler.newSession("user-1")
	handler.storeSession(session)

	body := `{"pairing_payload":{"bot_token":"bot-token"}}`
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

	cfg, ok := store.Get(wechatILinkSetupChannelID)
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
	if got, exists := manager.Get(wechatILinkSetupChannelID); !exists {
		t.Fatal("expected wechat_ilink channel to be registered")
	} else if got.Info().Status != channel.StatusConnected {
		t.Fatalf("status = %q, want %q", got.Info().Status, channel.StatusConnected)
	}
}

func TestWeChatILinkSetupHandler_CompleteSessionPersistsConfiguredUserIDFromPairingPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":             0,
			"msgs":            []any{},
			"get_updates_buf": "cursor-1",
		})
	}))
	defer server.Close()

	store := NewChannelConfigStore(kvstore.NewMemoryStore())
	if err := store.Set(wechatILinkSetupChannelID, &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: false,
		Config: map[string]string{
			"api_base_url": server.URL,
		},
	}); err != nil {
		t.Fatalf("store.Set error = %v", err)
	}
	manager := channel.NewManager(channel.DefaultConfig(), zap.NewNop())
	factory := NewChannelFactory(zap.NewNop())
	handler := NewWeChatILinkSetupHandler(
		store,
		manager,
		factory,
		zap.NewNop(),
	)

	session := handler.newSession("user-1")
	handler.storeSession(session)

	body := `{"pairing_payload":{"bot_token":"bot-token","user_id":"pairing-user-id"}}`
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

	cfg, ok := store.Get(wechatILinkSetupChannelID)
	if !ok {
		t.Fatal("expected persisted wechat_ilink config")
	}
	if cfg.Config["user_id"] != "pairing-user-id" {
		t.Fatalf("user_id = %q, want %q", cfg.Config["user_id"], "pairing-user-id")
	}
}

func newWeChatILinkSetupHandlerForTest() *WeChatILinkSetupHandler {
	return NewWeChatILinkSetupHandler(
		NewChannelConfigStore(kvstore.NewMemoryStore()),
		channel.NewManager(channel.DefaultConfig(), zap.NewNop()),
		NewChannelFactory(zap.NewNop()),
		zap.NewNop(),
	)
}

func callWeChatILinkCreateSession(t *testing.T, handler *WeChatILinkSetupHandler) map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/wechat_ilink/setup/session", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID:   "user-1",
		Username: "user-1",
	}))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	if err := handler.CreateSession(c); err != nil {
		t.Fatalf("CreateSession error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	return decodeWeChatILinkSetupResponse(t, rec.Body.Bytes())
}

func callWeChatILinkGetSession(t *testing.T, handler *WeChatILinkSetupHandler, sessionID string) map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/channels/wechat_ilink/setup/session/"+sessionID, nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID:   "user-1",
		Username: "user-1",
	}))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(sessionID)

	if err := handler.GetSession(c); err != nil {
		t.Fatalf("GetSession error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	return decodeWeChatILinkSetupResponse(t, rec.Body.Bytes())
}

func decodeWeChatILinkSetupResponse(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var resp map[string]any
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func stringValue(value any) string {
	s, _ := value.(string)
	return s
}
