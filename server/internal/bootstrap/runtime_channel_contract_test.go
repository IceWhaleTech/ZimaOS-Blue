package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func TestBindRouteRuntimeChannels_RegistersFallbackRoutesWithoutConfigStore(t *testing.T) {
	e := echo.New()
	api := e.Group("/api")

	bindRouteRuntimeChannels(routeRuntimeContractChannelOptions{api: api})

	if !routeExists(e, http.MethodGet, "/api/channels") {
		t.Fatalf("expected channel fallback route to be registered, got %#v", e.Routes())
	}
}

func TestBindRouteRuntimeChannels_RegistersWechatILinkSetupRoutesWithoutConfigStore(t *testing.T) {
	e := echo.New()
	api := e.Group("/api")

	bindRouteRuntimeChannels(routeRuntimeContractChannelOptions{
		api:           api,
		tunnelHandler: stubRuntimeChannelTunnelSetup{},
		logger:        zap.NewNop(),
	})

	for _, path := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/channels/wechat_ilink/setup/session"},
		{method: http.MethodGet, path: "/api/channels/wechat_ilink/setup/session/:id"},
		{method: http.MethodPost, path: "/api/channels/wechat_ilink/setup/session/:id/complete"},
	} {
		if !routeExists(e, path.method, path.path) {
			t.Fatalf("expected route %s %s to be registered without config store, got %#v", path.method, path.path, e.Routes())
		}
	}
}

func TestBindRouteRuntimeChannels_RegistersWechatILinkSetupRoutes(t *testing.T) {
	e := echo.New()
	api := e.Group("/api")

	bindRouteRuntimeChannels(routeRuntimeContractChannelOptions{
		api:                api,
		channelConfigStore: serverpkg.NewChannelConfigStore(kvstore.NewMemoryStore()),
		tunnelHandler:      stubRuntimeChannelTunnelSetup{},
		logger:             zap.NewNop(),
	})

	for _, path := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/channels/wechat_ilink/setup/session"},
		{method: http.MethodGet, path: "/api/channels/wechat_ilink/setup/session/:id"},
		{method: http.MethodPost, path: "/api/channels/wechat_ilink/setup/session/:id/complete"},
	} {
		if !routeExists(e, path.method, path.path) {
			t.Fatalf("expected route %s %s to be registered, got %#v", path.method, path.path, e.Routes())
		}
	}
}

func TestBindRouteRuntimeChannels_RegistersWechatILinkSetupRoutesWithoutTunnelHandler(t *testing.T) {
	e := echo.New()
	api := e.Group("/api")

	bindRouteRuntimeChannels(routeRuntimeContractChannelOptions{
		api:                api,
		channelConfigStore: serverpkg.NewChannelConfigStore(kvstore.NewMemoryStore()),
		logger:             zap.NewNop(),
	})

	for _, path := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/channels/wechat_ilink/setup/session"},
		{method: http.MethodGet, path: "/api/channels/wechat_ilink/setup/session/:id"},
		{method: http.MethodPost, path: "/api/channels/wechat_ilink/setup/session/:id/complete"},
	} {
		if !routeExists(e, path.method, path.path) {
			t.Fatalf("expected route %s %s to be registered without tunnel handler, got %#v", path.method, path.path, e.Routes())
		}
	}
}

func TestBindRouteRuntimeChannelTargets_WiresChatAndWatcherCallbacks(t *testing.T) {
	manager := &stubRuntimeChannelSender{sendWithIDResult: "msg-1"}
	chat := &stubRuntimeChannelChatTarget{}
	watcher := &stubRuntimeChannelWatcherTarget{}
	resolver := stubRuntimeChannelURLResolver("https://example.com/result")

	bindRouteRuntimeChannelChatTarget(chat, manager)
	bindRouteRuntimeChannelWatcherTarget(watcher, manager, resolver)

	if chat.sender == nil || chat.senderWithID == nil || chat.updater == nil {
		t.Fatalf("expected chat callbacks to be wired, got %#v", chat)
	}
	if watcher.notifier == nil || watcher.resolver == nil {
		t.Fatalf("expected watcher callbacks to be wired, got %#v", watcher)
	}

	if err := chat.sender(context.Background(), "telegram", channel.OutgoingMessage{Content: "hello"}); err != nil {
		t.Fatalf("sender callback error = %v", err)
	}
	if _, err := chat.senderWithID(context.Background(), "telegram", channel.OutgoingMessage{Content: "hello"}); err != nil {
		t.Fatalf("senderWithID callback error = %v", err)
	}
	if err := chat.updater(context.Background(), "telegram", "chat-1", "msg-1", channel.OutgoingMessage{Content: "updated"}); err != nil {
		t.Fatalf("updater callback error = %v", err)
	}
	if err := watcher.notifier(context.Background(), "telegram", channel.OutgoingMessage{Content: "notify"}); err != nil {
		t.Fatalf("watcher notifier error = %v", err)
	}

	if manager.sendCalls != 2 || manager.sendWithIDCalls != 1 || manager.updateCalls != 1 {
		t.Fatalf("expected manager callbacks to be invoked, got %#v", manager)
	}
	if got := watcher.resolver.ResolveExternalURL("/api/media/out.png"); got != "https://example.com/result" {
		t.Fatalf("resolver output = %q, want %q", got, "https://example.com/result")
	}
}

func TestBindRouteRuntimeChannelHandler_PrefersAutoreplyThenFallsBackToChat(t *testing.T) {
	t.Run("prefers autoreply match", func(t *testing.T) {
		target := &stubRuntimeChannelHandlerTarget{}
		chat := &stubRuntimeChannelProcessor{response: "assistant"}
		autoreplySvc := &stubRuntimeChannelAutoreplyTarget{
			response: "rule reply",
			rule:     &autoreply.Rule{Name: "hello"},
		}

		bindRouteRuntimeChannelHandler(target, chat, autoreplySvc)
		resp, err := target.handler(context.Background(), channel.Message{
			ChannelName: "telegram",
			ChatID:      "chat-1",
			UserID:      "user-1",
			Content:     "hello",
			Metadata: map[string]interface{}{
				"context_token":  "ctx-123",
				"session_id":     "session-456",
				"target_user_id": "wxid-peer-123",
			},
		})
		if err != nil {
			t.Fatalf("handler error = %v", err)
		}
		if resp == nil || resp.Content != "rule reply" {
			t.Fatalf("response = %#v, want autoreply response", resp)
		}
		if got := resp.Metadata["context_token"]; got != "ctx-123" {
			t.Fatalf("context_token = %v, want %q", got, "ctx-123")
		}
		if got := resp.Metadata["session_id"]; got != "session-456" {
			t.Fatalf("session_id = %v, want %q", got, "session-456")
		}
		if got := resp.Metadata["target_user_id"]; got != "wxid-peer-123" {
			t.Fatalf("target_user_id = %v, want %q", got, "wxid-peer-123")
		}
		if chat.calls != 0 {
			t.Fatalf("chat calls = %d, want 0", chat.calls)
		}
	})

	t.Run("falls back to chat", func(t *testing.T) {
		target := &stubRuntimeChannelHandlerTarget{}
		chat := &stubRuntimeChannelProcessor{response: "assistant"}
		autoreplySvc := &stubRuntimeChannelAutoreplyTarget{}

		bindRouteRuntimeChannelHandler(target, chat, autoreplySvc)
		resp, err := target.handler(context.Background(), channel.Message{
			ChannelName: "telegram",
			ChatID:      "chat-1",
			UserID:      "user-1",
			Content:     "need help",
			Metadata: map[string]interface{}{
				"context_token":  "ctx-789",
				"session_id":     "session-999",
				"target_user_id": "wxid-peer-789",
			},
		})
		if err != nil {
			t.Fatalf("handler error = %v", err)
		}
		if resp == nil || resp.Content != "assistant" {
			t.Fatalf("response = %#v, want chat response", resp)
		}
		if got := resp.Metadata["context_token"]; got != "ctx-789" {
			t.Fatalf("context_token = %v, want %q", got, "ctx-789")
		}
		if got := resp.Metadata["session_id"]; got != "session-999" {
			t.Fatalf("session_id = %v, want %q", got, "session-999")
		}
		if got := resp.Metadata["target_user_id"]; got != "wxid-peer-789" {
			t.Fatalf("target_user_id = %v, want %q", got, "wxid-peer-789")
		}
		if chat.calls != 1 {
			t.Fatalf("chat calls = %d, want 1", chat.calls)
		}
	})
}

func TestStartRouteRuntimeEnabledChannels_UpdatesStatusAndSkipsUnsupportedPlatform(t *testing.T) {
	t.Run("marks connected and errors", func(t *testing.T) {
		store := &stubRuntimeChannelConfigUpdater{configs: map[string]*serverpkg.ChannelConfig{}}
		manager := &stubRuntimeChannelLifecycleManager{
			startErrByName: map[string]error{"discord": errors.New("dial failed")},
		}
		factory := &stubRuntimeChannelFactory{
			create: func(cfg *serverpkg.ChannelConfig) (channel.Channel, error) {
				return &stubRuntimeChannel{name: cfg.ID, typ: cfg.ID}, nil
			},
		}

		startRouteRuntimeEnabledChannels([]*serverpkg.ChannelConfig{
			{ID: "telegram", Enabled: true},
			{ID: "discord", Enabled: true},
		}, store, factory, manager, zap.NewNop(), "darwin")

		if got := store.configs["telegram"]; got == nil || got.Status != "connected" || got.LastError != "" {
			t.Fatalf("telegram status = %#v, want connected", got)
		}
		if got := store.configs["discord"]; got == nil || got.Status != "error" || got.LastError != "dial failed" {
			t.Fatalf("discord status = %#v, want error", got)
		}
		if len(manager.registered) != 2 || len(manager.started) != 2 {
			t.Fatalf("expected manager to register/start both channels, got %#v", manager)
		}
	})

	t.Run("skips imessage on unsupported platform", func(t *testing.T) {
		store := &stubRuntimeChannelConfigUpdater{configs: map[string]*serverpkg.ChannelConfig{}}
		manager := &stubRuntimeChannelLifecycleManager{}
		factory := &stubRuntimeChannelFactory{
			create: func(cfg *serverpkg.ChannelConfig) (channel.Channel, error) {
				return &stubRuntimeChannel{name: cfg.ID, typ: cfg.ID}, nil
			},
		}

		startRouteRuntimeEnabledChannels([]*serverpkg.ChannelConfig{
			{ID: "imessage", Enabled: true},
		}, store, factory, manager, zap.NewNop(), "linux")

		if len(factory.created) != 0 || len(manager.registered) != 0 || len(manager.started) != 0 {
			t.Fatalf("expected unsupported channel to be skipped, got factory=%#v manager=%#v", factory, manager)
		}
		if len(store.configs) != 0 {
			t.Fatalf("expected no persisted status for skipped channel, got %#v", store.configs)
		}
	})
}

type stubRuntimeChannelSender struct {
	sendCalls        int
	sendWithIDCalls  int
	updateCalls      int
	sendWithIDResult string
}

func (s *stubRuntimeChannelSender) Send(_ context.Context, _ string, _ channel.OutgoingMessage) error {
	s.sendCalls++
	return nil
}

func (s *stubRuntimeChannelSender) SendWithID(_ context.Context, _ string, _ channel.OutgoingMessage) (string, error) {
	s.sendWithIDCalls++
	return s.sendWithIDResult, nil
}

func (s *stubRuntimeChannelSender) UpdateMessage(_ context.Context, _ string, _ string, _ string, _ channel.OutgoingMessage) error {
	s.updateCalls++
	return nil
}

type stubRuntimeChannelChatTarget struct {
	sender       func(ctx context.Context, channelName string, out channel.OutgoingMessage) error
	senderWithID func(ctx context.Context, channelName string, out channel.OutgoingMessage) (string, error)
	updater      func(ctx context.Context, channelName string, chatID string, messageID string, out channel.OutgoingMessage) error
}

func (s *stubRuntimeChannelChatTarget) SetChannelSender(sender func(ctx context.Context, channelName string, out channel.OutgoingMessage) error) {
	s.sender = sender
}

func (s *stubRuntimeChannelChatTarget) SetChannelSenderWithID(sender func(ctx context.Context, channelName string, out channel.OutgoingMessage) (string, error)) {
	s.senderWithID = sender
}

func (s *stubRuntimeChannelChatTarget) SetChannelMessageUpdater(updater func(ctx context.Context, channelName string, chatID string, messageID string, out channel.OutgoingMessage) error) {
	s.updater = updater
}

type stubRuntimeChannelWatcherTarget struct {
	notifier mediagen.ChannelNotifier
	resolver mediagen.URLResolver
}

func (s *stubRuntimeChannelWatcherTarget) SetNotifier(notifier mediagen.ChannelNotifier) {
	s.notifier = notifier
}

func (s *stubRuntimeChannelWatcherTarget) SetURLResolver(resolver mediagen.URLResolver) {
	s.resolver = resolver
}

type stubRuntimeChannelHandlerTarget struct {
	handler channel.MessageHandler
}

func (s *stubRuntimeChannelHandlerTarget) SetHandler(handler channel.MessageHandler) {
	s.handler = handler
}

type stubRuntimeChannelProcessor struct {
	response string
	err      error
	calls    int
}

func (s *stubRuntimeChannelProcessor) ProcessChannelMessage(_ context.Context, _ channel.Message) (string, error) {
	s.calls++
	return s.response, s.err
}

type stubRuntimeChannelAutoreplyTarget struct {
	response string
	rule     *autoreply.Rule
	err      error
	calls    int
}

func (s *stubRuntimeChannelAutoreplyTarget) Match(_ context.Context, _, _, _, _, _ string) (string, *autoreply.Rule, error) {
	s.calls++
	return s.response, s.rule, s.err
}

type stubRuntimeChannelFactory struct {
	created []*serverpkg.ChannelConfig
	create  func(cfg *serverpkg.ChannelConfig) (channel.Channel, error)
}

func (s *stubRuntimeChannelFactory) CreateChannel(cfg *serverpkg.ChannelConfig) (channel.Channel, error) {
	s.created = append(s.created, cfg)
	if s.create != nil {
		return s.create(cfg)
	}
	return nil, nil
}

type stubRuntimeChannelLifecycleManager struct {
	registered     []string
	started        []string
	startErrByName map[string]error
}

func (s *stubRuntimeChannelLifecycleManager) Register(ch channel.Channel) error {
	s.registered = append(s.registered, ch.Name())
	return nil
}

func (s *stubRuntimeChannelLifecycleManager) StartChannel(_ context.Context, name string) error {
	s.started = append(s.started, name)
	if s.startErrByName != nil {
		return s.startErrByName[name]
	}
	return nil
}

type stubRuntimeChannelConfigUpdater struct {
	configs map[string]*serverpkg.ChannelConfig
}

type stubRuntimeChannelTunnelSetup struct{}

func (stubRuntimeChannelTunnelSetup) EnsureTunnelURL(context.Context) (string, error) {
	return "https://blue.example.com", nil
}

func (s *stubRuntimeChannelConfigUpdater) Set(id string, cfg *serverpkg.ChannelConfig) error {
	if s.configs == nil {
		s.configs = map[string]*serverpkg.ChannelConfig{}
	}
	copyCfg := *cfg
	s.configs[id] = &copyCfg
	return nil
}

type stubRuntimeChannel struct {
	name string
	typ  string
}

func (s *stubRuntimeChannel) Name() string                                        { return s.name }
func (s *stubRuntimeChannel) Type() string                                        { return s.typ }
func (s *stubRuntimeChannel) Start(context.Context) error                         { return nil }
func (s *stubRuntimeChannel) Stop(context.Context) error                          { return nil }
func (s *stubRuntimeChannel) Send(context.Context, channel.OutgoingMessage) error { return nil }
func (s *stubRuntimeChannel) SendStreaming(context.Context, string, string, <-chan string, chan<- struct{}) error {
	return nil
}
func (s *stubRuntimeChannel) Info() channel.Info               { return channel.Info{Name: s.name, Type: s.typ} }
func (s *stubRuntimeChannel) IsConnected() bool                { return true }
func (s *stubRuntimeChannel) Messages() <-chan channel.Message { return nil }

type stubRuntimeChannelURLResolver string

func (s stubRuntimeChannelURLResolver) ResolveExternalURL(string) string {
	return string(s)
}
