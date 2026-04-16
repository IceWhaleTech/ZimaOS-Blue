package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	wechatILinkDefaultAPIBaseURL        = "https://ilinkai.weixin.qq.com"
	wechatILinkBotAPIPath               = "/ilink/bot"
)

type wechatILinkSetupSession struct {
	ID                 string
	UserID             string
	Status             string
	Error              string
	Message            string
	ExpiresAt          time.Time
	Consumed           bool
	QRKey              string
	ScanURL            string
	QRCode             string
	ResolvedAPIBaseURL string
	ActivationStarted  bool
}

type WeChatILinkSetupHandler struct {
	store      *ChannelConfigStore
	manager    *channel.Manager
	factory    *ChannelFactory
	logger     *zap.Logger
	httpClient *http.Client

	mu       sync.RWMutex
	sessions map[string]*wechatILinkSetupSession
	now      func() time.Time
	ttl      time.Duration
}

func NewWeChatILinkSetupHandler(
	store *ChannelConfigStore,
	manager *channel.Manager,
	factory *ChannelFactory,
	logger *zap.Logger,
) *WeChatILinkSetupHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &WeChatILinkSetupHandler{
		store:      store,
		manager:    manager,
		factory:    factory,
		logger:     logger.With(zap.String("component", "wechat_ilink_setup")),
		httpClient: &http.Client{Timeout: 45 * time.Second},
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

	resolvedAPIBaseURL := h.resolveWeChatILinkAPIBaseURL()
	qrResp, err := h.fetchWeChatILinkQRCode(c.Request().Context(), resolvedAPIBaseURL)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	qrcode, err := ngrok.GenerateQRCode(qrResp.ScanURL, 200)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	session := h.newSession(userClaims.UserID)
	session.QRKey = qrResp.QRKey
	session.ScanURL = qrResp.ScanURL
	session.QRCode = qrcode
	session.ResolvedAPIBaseURL = resolvedAPIBaseURL
	h.storeSession(session)

	return c.JSON(http.StatusOK, h.sessionResponse(session))
}

func (h *WeChatILinkSetupHandler) GetSession(c echo.Context) error {
	session, err := h.requireOwnedSession(c, false)
	if err != nil {
		return err
	}

	if session.Status != wechatILinkSessionStatusConnected &&
		session.Status != wechatILinkSessionStatusError &&
		session.Status != wechatILinkSessionStatusExpired {
		h.refreshWeChatILinkSetupSession(c.Request().Context(), session)
	}

	current, _ := h.getSession(session.ID)
	return c.JSON(http.StatusOK, h.sessionResponse(current))
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

	_, botToken, parseErr := parseWechatILinkPairingPayload(req.PairingPayload)
	if parseErr != nil {
		h.markSessionError(session.ID, parseErr.Error())
		return echo.NewHTTPError(http.StatusBadRequest, parseErr.Error())
	}
	apiBaseURL := h.resolveWeChatILinkAPIBaseURL()

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
	if session.QRCode != "" {
		resp["qrcode"] = session.QRCode
	}
	if session.ScanURL != "" {
		resp["scan_url"] = session.ScanURL
		resp["mobile_url"] = session.ScanURL
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

func (h *WeChatILinkSetupHandler) resolveWeChatILinkAPIBaseURL() string {
	if h.store != nil {
		if cfg, ok := h.store.Get(wechatILinkSetupChannelID); ok && cfg != nil {
			if raw := strings.TrimSpace(cfg.Config["api_base_url"]); raw != "" {
				normalized := normalizeWeChatILinkAPIBaseURL(raw)
				if isValidWeChatILinkAPIBaseURL(normalized) {
					return normalized
				}
			}
		}
	}
	return normalizeWeChatILinkAPIBaseURL(wechatILinkDefaultAPIBaseURL)
}

func (h *WeChatILinkSetupHandler) refreshWeChatILinkSetupSession(ctx context.Context, session *wechatILinkSetupSession) {
	if strings.TrimSpace(session.QRKey) == "" {
		h.markSessionError(session.ID, "setup session missing qr_key")
		return
	}
	if strings.TrimSpace(session.ResolvedAPIBaseURL) == "" {
		h.markSessionError(session.ID, "setup session missing api_base_url")
		return
	}

	status, err := h.pollWeChatILinkQRCodeStatus(ctx, session.ResolvedAPIBaseURL, session.QRKey)
	if err != nil {
		h.markSessionError(session.ID, err.Error())
		return
	}

	switch strings.TrimSpace(status.Status) {
	case "", "wait":
		h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
			current.Status = wechatILinkSessionStatusPending
			current.Error = ""
			current.Message = ""
		})
	case "scaned":
		h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
			current.Status = wechatILinkSessionStatusAuthorizing
			current.Error = ""
			current.Message = ""
		})
	case "confirmed":
		if h.beginWeChatILinkActivation(session.ID) {
			apiBaseURL := strings.TrimSpace(status.BaseURL)
			if apiBaseURL == "" {
				apiBaseURL = strings.TrimSpace(session.ResolvedAPIBaseURL)
			}
			if strings.TrimSpace(status.BotToken) == "" {
				h.markSessionError(session.ID, "confirmed QR status missing bot_token")
				return
			}
			if apiBaseURL == "" {
				h.markSessionError(session.ID, "confirmed QR status missing api_base_url")
				return
			}
			if err := h.activateChannel(ctx, apiBaseURL, strings.TrimSpace(status.BotToken)); err != nil {
				h.markSessionError(session.ID, err.Error())
				return
			}
			resolvedAPIBaseURL := normalizeWeChatILinkAPIBaseURL(apiBaseURL)
			if !isValidWeChatILinkAPIBaseURL(resolvedAPIBaseURL) {
				resolvedAPIBaseURL = wechatILinkDefaultAPIBaseURL
			}
			h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
				current.Status = wechatILinkSessionStatusConnected
				current.Error = ""
				current.Message = "configured"
				current.ResolvedAPIBaseURL = resolvedAPIBaseURL
			})
			return
		}

		h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
			if current.Status != wechatILinkSessionStatusConnected &&
				current.Status != wechatILinkSessionStatusError {
				current.Status = wechatILinkSessionStatusConfiguring
				current.Error = ""
				current.Message = ""
			}
		})
	case "expired":
		h.updateSession(session.ID, func(current *wechatILinkSetupSession) {
			current.Status = wechatILinkSessionStatusExpired
			current.Error = "setup session expired"
			current.Message = ""
		})
	default:
		h.markSessionError(session.ID, fmt.Sprintf("unexpected QR status: %s", strings.TrimSpace(status.Status)))
	}
}

func (h *WeChatILinkSetupHandler) beginWeChatILinkActivation(id string) bool {
	started := false
	h.updateSession(id, func(session *wechatILinkSetupSession) {
		if session.ActivationStarted {
			return
		}
		session.ActivationStarted = true
		session.Status = wechatILinkSessionStatusConfiguring
		session.Error = ""
		session.Message = ""
		started = true
	})
	return started
}

type wechatILinkQRCodeResponse struct {
	QRKey   string
	ScanURL string
}

type wechatILinkQRCodeStatus struct {
	Status      string `json:"status"`
	BotToken    string `json:"bot_token"`
	IlinkBotID  string `json:"ilink_bot_id"`
	BaseURL     string `json:"baseurl"`
	IlinkUserID string `json:"ilink_user_id"`
}

func (h *WeChatILinkSetupHandler) fetchWeChatILinkQRCode(ctx context.Context, apiBaseURL string) (*wechatILinkQRCodeResponse, error) {
	normalizedAPIBaseURL := normalizeWeChatILinkAPIBaseURL(apiBaseURL)
	if !isValidWeChatILinkAPIBaseURL(normalizedAPIBaseURL) {
		return nil, fmt.Errorf("invalid iLink api_base_url")
	}

	endpoint, err := url.Parse(normalizedAPIBaseURL + "/")
	if err != nil {
		return nil, fmt.Errorf("invalid iLink api_base_url: %w", err)
	}
	endpoint = endpoint.JoinPath("ilink", "bot", "get_bot_qrcode")
	query := endpoint.Query()
	query.Set("bot_type", "3")
	endpoint.RawQuery = query.Encode()

	var raw struct {
		QRKey   string `json:"qrcode"`
		ScanURL string `json:"qrcode_img_content"`
	}
	if err := h.getWeChatILinkJSON(ctx, endpoint.String(), nil, &raw); err != nil {
		return nil, fmt.Errorf("get_bot_qrcode: %w", err)
	}

	raw.QRKey = strings.TrimSpace(raw.QRKey)
	raw.ScanURL = strings.TrimSpace(raw.ScanURL)
	if raw.QRKey == "" {
		return nil, fmt.Errorf("get_bot_qrcode: missing qrcode")
	}
	if raw.ScanURL == "" {
		return nil, fmt.Errorf("get_bot_qrcode: missing qrcode_img_content")
	}
	return &wechatILinkQRCodeResponse{
		QRKey:   raw.QRKey,
		ScanURL: raw.ScanURL,
	}, nil
}

func (h *WeChatILinkSetupHandler) pollWeChatILinkQRCodeStatus(ctx context.Context, apiBaseURL, qrKey string) (*wechatILinkQRCodeStatus, error) {
	normalizedAPIBaseURL := normalizeWeChatILinkAPIBaseURL(apiBaseURL)
	if !isValidWeChatILinkAPIBaseURL(normalizedAPIBaseURL) {
		return nil, fmt.Errorf("invalid iLink api_base_url")
	}

	endpoint, err := url.Parse(normalizedAPIBaseURL + "/")
	if err != nil {
		return nil, fmt.Errorf("invalid iLink api_base_url: %w", err)
	}
	endpoint = endpoint.JoinPath("ilink", "bot", "get_qrcode_status")
	query := endpoint.Query()
	query.Set("qrcode", qrKey)
	endpoint.RawQuery = query.Encode()

	var status wechatILinkQRCodeStatus
	if err := h.getWeChatILinkJSON(ctx, endpoint.String(), map[string]string{
		"iLink-App-ClientVersion": "1",
	}, &status); err != nil {
		return nil, fmt.Errorf("get_qrcode_status: %w", err)
	}
	return &status, nil
}

func (h *WeChatILinkSetupHandler) getWeChatILinkJSON(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
	out any,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}

	client := h.httpClient
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d: %s", resp.StatusCode, truncateWeChatILinkBody(body, 256))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func truncateWeChatILinkBody(body []byte, max int) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func (h *WeChatILinkSetupHandler) activateChannel(ctx context.Context, apiBaseURL, botToken string) error {
	if h.store == nil || h.manager == nil || h.factory == nil {
		return fmt.Errorf("wechat_ilink setup runtime is not available")
	}

	normalizedAPIBaseURL := normalizeWeChatILinkAPIBaseURL(apiBaseURL)
	if !isValidWeChatILinkAPIBaseURL(normalizedAPIBaseURL) {
		normalizedAPIBaseURL = wechatILinkDefaultAPIBaseURL
	}

	newCfg := &ChannelConfig{
		ID:      wechatILinkSetupChannelID,
		Enabled: true,
		Status:  string(channel.StatusConnecting),
		Config: map[string]string{
			"api_base_url": normalizedAPIBaseURL,
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

func normalizeWeChatILinkAPIBaseURL(raw string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		return ""
	}
	if strings.HasSuffix(baseURL, wechatILinkBotAPIPath) {
		return strings.TrimSuffix(baseURL, wechatILinkBotAPIPath)
	}
	return baseURL
}

func isValidWeChatILinkAPIBaseURL(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return strings.TrimSpace(parsed.Host) != ""
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

	botToken := getString(payload, "bot_token", "botToken")
	if nested, ok := payload["config"].(map[string]interface{}); ok {
		if botToken == "" {
			botToken = getString(nested, "bot_token", "botToken")
		}
	}

	if botToken == "" {
		return "", "", fmt.Errorf("pairing_payload missing bot_token")
	}
	return "", botToken, nil
}
