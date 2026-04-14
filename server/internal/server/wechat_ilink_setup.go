package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
)

const (
	wechatILinkSessionStatusPending     = "pending"
	wechatILinkSessionStatusAuthorizing = "authorizing"
	wechatILinkSessionStatusConfiguring = "configuring"
	wechatILinkSessionStatusConnected   = "connected"
	wechatILinkSessionStatusError       = "error"
	wechatILinkSessionStatusExpired     = "expired"
	wechatILinkSetupChannelID           = "wechat_ilink"
	wechatILinkSetupSessionTTL          = 10 * time.Minute
)

type wechatILinkSetupTunnelRuntime interface {
	EnsureTunnelURL(ctx context.Context) (string, error)
}

type wechatILinkSetupSession struct {
	ID        string
	UserID    string
	Status    string
	Error     string
	Message   string
	ExpiresAt time.Time
	Consumed  bool
}

type WeChatILinkSetupHandler struct {
	store      *ChannelConfigStore
	manager    *channel.Manager
	factory    *ChannelFactory
	tunnel     wechatILinkSetupTunnelRuntime
	jwtService *auth.JWTService
	logger     *zap.Logger

	mu       sync.RWMutex
	sessions map[string]*wechatILinkSetupSession
	now      func() time.Time
	ttl      time.Duration
}

func NewWeChatILinkSetupHandler(
	store *ChannelConfigStore,
	manager *channel.Manager,
	factory *ChannelFactory,
	tunnel wechatILinkSetupTunnelRuntime,
	jwtService *auth.JWTService,
	logger *zap.Logger,
) *WeChatILinkSetupHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &WeChatILinkSetupHandler{
		store:      store,
		manager:    manager,
		factory:    factory,
		tunnel:     tunnel,
		jwtService: jwtService,
		logger:     logger.With(zap.String("component", "wechat_ilink_setup")),
		sessions:   make(map[string]*wechatILinkSetupSession),
		now:        time.Now,
		ttl:        wechatILinkSetupSessionTTL,
	}
}

func (h *WeChatILinkSetupHandler) RegisterRoutes(g *echo.Group) {
	setupGroup := g.Group("/channels/wechat_ilink/setup")
	setupGroup.POST("/session", h.CreateSession)
	setupGroup.GET("/session/:id", h.GetSession)
	setupGroup.POST("/session/:id/complete", h.CompleteSession)
}

func (h *WeChatILinkSetupHandler) CreateSession(c echo.Context) error {
	userClaims := auth.GetUserFromContext(c)
	if userClaims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if h.tunnel == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "tunnel runtime not available")
	}

	tunnelURL, err := h.tunnel.EnsureTunnelURL(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	session := h.newSession(userClaims.UserID)
	h.storeSession(session)

	mobileURL, err := h.buildMobileURL(strings.TrimRight(tunnelURL, "/"), session.ID, userClaims)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	qrcode, err := ngrok.GenerateQRCode(mobileURL, 200)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"session_id": session.ID,
		"status":     session.Status,
		"qrcode":     qrcode,
		"mobile_url": mobileURL,
		"expires_at": session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *WeChatILinkSetupHandler) GetSession(c echo.Context) error {
	session, err := h.requireOwnedSession(c, false)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.sessionResponse(session))
}

func (h *WeChatILinkSetupHandler) CompleteSession(c echo.Context) error {
	session, err := h.requireOwnedSession(c, true)
	if err != nil {
		return err
	}

	var req struct {
		PairingPayload json.RawMessage `json:"pairing_payload"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if len(strings.TrimSpace(string(req.PairingPayload))) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "pairing_payload is required")
	}

	h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
		current.Consumed = true
		current.Status = wechatILinkSessionStatusAuthorizing
		current.Error = ""
		current.Message = ""
	})

	apiBaseURL, botToken, parseErr := parseWechatILinkPairingPayload(req.PairingPayload)
	if parseErr != nil {
		h.markSessionError(session.ID, parseErr.Error())
		return echo.NewHTTPError(http.StatusBadRequest, parseErr.Error())
	}

	h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
		current.Status = wechatILinkSessionStatusConfiguring
	})

	if err := h.activateChannel(c.Request().Context(), apiBaseURL, botToken); err != nil {
		h.markSessionError(session.ID, err.Error())
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
		current.Status = wechatILinkSessionStatusConnected
		current.Message = "configured"
		current.Error = ""
	})

	current, _ := h.getSession(session.ID)
	return c.JSON(http.StatusOK, h.sessionResponse(current))
}

func (h *WeChatILinkSetupHandler) newSession(userID string) *wechatILinkSetupSession {
	now := h.now().UTC()
	return &wechatILinkSetupSession{
		ID:        fmt.Sprintf("wechat-ilink-%d", now.UnixNano()),
		UserID:    userID,
		Status:    wechatILinkSessionStatusPending,
		ExpiresAt: now.Add(h.ttl),
	}
}

func (h *WeChatILinkSetupHandler) storeSession(session *wechatILinkSetupSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[session.ID] = session
}

func (h *WeChatILinkSetupHandler) getSession(id string) (*wechatILinkSetupSession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	session, ok := h.sessions[id]
	return session, ok
}

func (h *WeChatILinkSetupHandler) updateSession(id string, apply func(*wechatILinkSetupSession)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if session, ok := h.sessions[id]; ok {
		apply(session)
	}
}

func (h *WeChatILinkSetupHandler) markSessionError(id, errMsg string) {
	h.updateSession(id, func(session *wechatILinkSetupSession) {
		session.Status = wechatILinkSessionStatusError
		session.Error = errMsg
		session.Message = ""
	})
}

func (h *WeChatILinkSetupHandler) sessionResponse(session *wechatILinkSetupSession) map[string]interface{} {
	resp := map[string]interface{}{
		"session_id": session.ID,
		"status":     session.Status,
		"expires_at": session.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if session.Error != "" {
		resp["error"] = session.Error
	}
	if session.Message != "" {
		resp["message"] = session.Message
	}
	return resp
}

func (h *WeChatILinkSetupHandler) requireOwnedSession(c echo.Context, forCompletion bool) (*wechatILinkSetupSession, error) {
	userClaims := auth.GetUserFromContext(c)
	if userClaims == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	sessionID := c.Param("id")
	session, ok := h.getSession(sessionID)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusNotFound, "setup session not found")
	}
	if session.UserID != userClaims.UserID {
		return nil, echo.NewHTTPError(http.StatusNotFound, "setup session not found")
	}
	if h.now().UTC().After(session.ExpiresAt) {
		h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
			current.Status = wechatILinkSessionStatusExpired
			current.Error = "setup session expired"
		})
		return nil, echo.NewHTTPError(http.StatusGone, "setup session expired")
	}
	if forCompletion && session.Consumed {
		return nil, echo.NewHTTPError(http.StatusConflict, "setup session already consumed")
	}
	return session, nil
}

func (h *WeChatILinkSetupHandler) buildMobileURL(tunnelURL, sessionID string, userClaims *auth.UserClaims) (string, error) {
	values := url.Values{}
	values.Set("session_id", sessionID)
	if h.jwtService != nil && userClaims != nil {
		token, err := h.jwtService.GenerateAccessToken(userClaims)
		if err != nil {
			return "", err
		}
		values.Set("access_token", token)
	}
	return tunnelURL + "/channels/setup/wechat_ilink?" + values.Encode(), nil
}

func (h *WeChatILinkSetupHandler) activateChannel(ctx context.Context, apiBaseURL, botToken string) error {
	if h.store == nil || h.manager == nil || h.factory == nil {
		return fmt.Errorf("wechat_ilink setup runtime is not available")
	}

	newCfg := &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: true,
		Status:  string(channel.StatusConnecting),
		Config: map[string]string{
			"api_base_url": apiBaseURL,
			"bot_token":    botToken,
		},
	}

	previousCfg, _ := h.store.Get(wechatILinkSetupChannelID)
	previousSnapshot := cloneChannelConfig(previousCfg)

	if ch, exists := h.manager.Get(wechatILinkSetupChannelID); exists {
		if ch.IsConnected() {
			_ = h.manager.StopChannel(ctx, wechatILinkSetupChannelID)
		}
		_ = h.manager.Unregister(wechatILinkSetupChannelID)
	}

	restorePrevious := func() {
		if previousSnapshot == nil || !previousSnapshot.Enabled {
			return
		}
		oldChannel, err := h.factory.CreateChannel(previousSnapshot)
		if err != nil || oldChannel == nil {
			if err != nil {
				h.logger.Warn("failed to recreate previous wechat_ilink channel", zap.Error(err))
			}
			return
		}
		if err := h.manager.Register(oldChannel); err != nil {
			h.logger.Warn("failed to re-register previous wechat_ilink channel", zap.Error(err))
			return
		}
		if err := h.manager.StartChannel(ctx, wechatILinkSetupChannelID); err != nil {
			h.logger.Warn("failed to restart previous wechat_ilink channel", zap.Error(err))
		}
	}

	ch, err := h.factory.CreateChannel(newCfg)
	if err != nil {
		restorePrevious()
		return err
	}
	if ch == nil {
		restorePrevious()
		return fmt.Errorf("unsupported channel type: %s", wechatILinkSetupChannelID)
	}
	if err := h.manager.Register(ch); err != nil {
		restorePrevious()
		return err
	}
	if err := h.manager.StartChannel(ctx, wechatILinkSetupChannelID); err != nil {
		_ = h.manager.Unregister(wechatILinkSetupChannelID)
		restorePrevious()
		return err
	}

	if current, exists := h.manager.Get(wechatILinkSetupChannelID); exists {
		newCfg.Status = string(current.Info().Status)
	}

	if err := h.store.Set(wechatILinkSetupChannelID, newCfg); err != nil {
		if current, exists := h.manager.Get(wechatILinkSetupChannelID); exists && current.IsConnected() {
			_ = h.manager.StopChannel(ctx, wechatILinkSetupChannelID)
		}
		_ = h.manager.Unregister(wechatILinkSetupChannelID)
		restorePrevious()
		return err
	}

	return nil
}

func cloneChannelConfig(cfg *ChannelConfig) *ChannelConfig {
	if cfg == nil {
		return nil
	}
	cloned := &ChannelConfig{
		ID:               cfg.ID,
		Enabled:          cfg.Enabled,
		Status:           cfg.Status,
		LastError:        cfg.LastError,
		LastErrorKey:     cfg.LastErrorKey,
		MessagesReceived: cfg.MessagesReceived,
		MessagesSent:     cfg.MessagesSent,
		LastMessageAt:    cfg.LastMessageAt,
		LastReplyAt:      cfg.LastReplyAt,
		Config:           make(map[string]string, len(cfg.Config)),
	}
	for key, value := range cfg.Config {
		cloned.Config[key] = value
	}
	return cloned
}

func parseWechatILinkPairingPayload(raw json.RawMessage) (string, string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "", "", fmt.Errorf("pairing_payload is required")
	}

	if strings.HasPrefix(trimmed, `"`) {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return "", "", fmt.Errorf("invalid pairing_payload")
		}
		trimmed = strings.TrimSpace(decoded)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return "", "", fmt.Errorf("invalid pairing_payload")
	}

	getString := func(source map[string]interface{}, keys ...string) string {
		for _, key := range keys {
			if rawValue, ok := source[key]; ok {
				if value, ok := rawValue.(string); ok && strings.TrimSpace(value) != "" {
					return strings.TrimSpace(value)
				}
			}
		}
		return ""
	}

	apiBaseURL := getString(payload, "api_base_url", "apiBaseURL")
	botToken := getString(payload, "bot_token", "botToken")
	if nested, ok := payload["config"].(map[string]interface{}); ok {
		if apiBaseURL == "" {
			apiBaseURL = getString(nested, "api_base_url", "apiBaseURL")
		}
		if botToken == "" {
			botToken = getString(nested, "bot_token", "botToken")
		}
	}

	if apiBaseURL == "" {
		return "", "", fmt.Errorf("pairing_payload missing api_base_url")
	}
	if botToken == "" {
		return "", "", fmt.Errorf("pairing_payload missing bot_token")
	}
	return apiBaseURL, botToken, nil
}
