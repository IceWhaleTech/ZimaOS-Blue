package browser

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/reclaim"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/labstack/echo/v4"
)

// Handler handles browser automation HTTP requests.
type Handler struct {
	service         Service
	serviceFactory  func() Service
	lazy            *reclaim.Managed[Service]
	relayInfo       func() RelayInfo
	sessionRoutes   SessionRouteProvider
	taskProjections BrowserOverviewTaskProvider
}

const (
	browserSessionCreateTimeout   = 60 * time.Second
	browserSessionNavigateTimeout = 60 * time.Second
)

type browserOverviewResponse struct {
	Tasks    []map[string]any `json:"tasks"`
	Sessions []SessionInfo    `json:"sessions"`
}

type browserViewportScreenshoter interface {
	ScreenshotViewport(ctx context.Context, targetID string) (string, error)
}

type browserTabScreenshoter interface {
	ScreenshotTab(ctx context.Context, targetID string) (string, error)
}

type browserSessionScreenshotHistoryProvider interface {
	SessionScreenshotHistory(targetID string) []SessionScreenshot
}

type browserPageInfoProvider interface {
	PageInfo(ctx context.Context, targetID string) (string, string, error)
}

type browserElementExistsProvider interface {
	ElementExists(ctx context.Context, targetID, selector string) (bool, error)
}

type browserExtractProvider interface {
	ExtractFirstFromTab(ctx context.Context, targetID, selector, attribute string) (string, error)
}

type browserSecurityRuntime interface {
	currentBrowserSecurityConfig() BrowserSecurityConfig
	replaceBrowserSecurityConfig(config BrowserSecurityConfig) BrowserSecurityConfig
	validateBrowserURL(rawURL string) error
}

// NewHandler creates a new browser handler.
func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

// NewLazyHandler creates a browser handler whose service is initialized on demand.
func NewLazyHandler(factory func() Service) *Handler {
	h := &Handler{
		serviceFactory: factory,
	}
	h.lazy = reclaim.NewManaged[Service](0, func() (Service, error) {
		if h.serviceFactory == nil {
			return nil, nil
		}
		svc := h.serviceFactory()
		if svc == nil {
			return nil, ErrBrowserNotAvailable
		}
		return svc, nil
	}, func(_ context.Context, svc Service) error {
		if svc == nil {
			return nil
		}
		return svc.Close()
	})
	return h
}

func (h *Handler) getService() Service {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service
	}
	svc, _ := h.lazy.Get()
	return svc
}

// GetService returns the current service, creating it if needed.
func (h *Handler) GetService() Service {
	return h.getService()
}

// PeekService returns the current browser service without creating it.
func (h *Handler) PeekService() Service {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service
	}
	svc, ok := h.lazy.Peek()
	if !ok {
		return nil
	}
	return svc
}

// SetIdleReclaim configures idle reclaim for lazily created browser services.
func (h *Handler) SetIdleReclaim(idleAfter time.Duration) {
	if h == nil || h.lazy == nil {
		return
	}
	h.lazy.SetIdleAfter(idleAfter)
}

func (h *Handler) peekService() Service {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service
	}
	svc, _ := h.lazy.Peek()
	return svc
}

func (h *Handler) acquireService() (Service, func(), error) {
	if h == nil {
		return nil, nil, nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service, func() {}, nil
	}
	svc, release, err := h.lazy.Acquire()
	if err != nil {
		return nil, nil, err
	}
	return svc, release, nil
}

func (h *Handler) acquireStartedService(ctx context.Context) (Service, func(), error) {
	service, release, err := h.acquireService()
	if err != nil {
		return nil, nil, err
	}
	if service == nil {
		return nil, release, nil
	}
	if err := service.Start(ctx); err != nil {
		release()
		return nil, nil, err
	}
	return service, release, nil
}

// SetRelayInfoProvider configures how relay runtime info is exposed over HTTP.
func (h *Handler) SetRelayInfoProvider(provider func() RelayInfo) {
	h.relayInfo = provider
}

// SetSessionRouteProvider configures a multi-engine session provider for
// session and monitor HTTP routes.
func (h *Handler) SetSessionRouteProvider(provider SessionRouteProvider) {
	h.sessionRoutes = provider
}

// SetTaskProjectionService configures the task provider used by the browser overview route.
func (h *Handler) SetTaskProjectionService(provider BrowserOverviewTaskProvider) {
	h.taskProjections = provider
}

// RegisterRoutes registers the browser routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Status and control
	g.GET("/relay/info", h.RelayInfo)
	g.GET("/status", h.Status)
	g.POST("/start", h.Start)
	g.POST("/stop", h.Stop)

	// Tab management
	g.GET("/tabs", h.Tabs)
	g.POST("/tabs", h.OpenTab)
	g.POST("/tabs/:id/focus", h.FocusTab)
	g.DELETE("/tabs/:id", h.CloseTab)

	// Navigation
	g.POST("/navigate", h.Navigate)

	// Content capture
	g.POST("/screenshot", h.Screenshot)
	g.POST("/pdf", h.PDF)
	g.POST("/snapshot", h.Snapshot)

	// Data extraction
	g.POST("/scrape", h.Scrape)

	// Actions
	g.POST("/act", h.Act)
	g.POST("/automate", h.Automate)

	// Console
	g.GET("/console", h.Console)

	// Recipes
	g.GET("/recipes", h.ListRecipes)
	g.POST("/recipe", h.ExecuteRecipe)

	// Session management routes used by the browser monitor UI.
	g.GET("/overview", h.Overview)
	g.POST("/sessions", h.CreateSession)
	g.DELETE("/sessions/:id", h.CloseSession)
	g.POST("/sessions/:id/monitor", h.SessionMonitor)
	g.POST("/sessions/:id/screenshot", h.SessionScreenshot)
	g.POST("/sessions/:id/navigate", h.SessionNavigate)
	g.POST("/sessions/:id/execute", h.SessionExecute)

	// Security configuration routes for browser session policies.
	g.GET("/security", h.GetSecurityConfig)
	g.PUT("/security", h.UpdateSecurityConfig)
	g.POST("/security/allowed", h.AddAllowedDomain)
	g.DELETE("/security/allowed/:domain", h.RemoveAllowedDomain)
	g.POST("/security/blocked", h.AddBlockedDomain)
	g.DELETE("/security/blocked/:domain", h.RemoveBlockedDomain)
	g.POST("/security/test", h.TestURL)
}

// RelayInfo returns built-in browser relay runtime details.
func (h *Handler) RelayInfo(c echo.Context) error {
	if h.relayInfo == nil {
		return c.JSON(http.StatusOK, RelayInfo{})
	}
	return c.JSON(http.StatusOK, h.relayInfo())
}

func newBrowserSessionResponse(tab *Tab, now time.Time) SessionInfo {
	session := SessionInfo{
		CreatedAt:    now.Format(time.RFC3339),
		LastActivity: now.Format(time.RFC3339),
		Engine:       SessionEngineChromiumManaged,
		MonitorKind:  SessionMonitorKindImage,
	}
	if tab == nil {
		session.Status = "idle"
		return session
	}
	session.ID = tab.TargetID
	session.CurrentURL = tab.URL
	session.PageTitle = tab.Title
	if tab.Active {
		session.Status = "active"
	} else {
		session.Status = "idle"
	}
	return session
}

func browserSessionResponsesFromTabs(tabs []*Tab, now time.Time) []SessionInfo {
	sessions := make([]SessionInfo, 0, len(tabs))
	for _, tab := range tabs {
		if tab == nil {
			continue
		}
		sessions = append(sessions, newBrowserSessionResponse(tab, now))
	}
	return sessions
}

func findTabByTargetID(tabs []*Tab, id string) *Tab {
	for _, tab := range tabs {
		if tab != nil && tab.TargetID == id {
			return tab
		}
	}
	return nil
}

// Overview returns the Browser Monitor overview payload in one request.
func (h *Handler) Overview(c echo.Context) error {
	sessions, err := h.listSessionInfo(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	tasks, err := h.listOverviewTasks(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, browserOverviewResponse{
		Tasks:    tasks,
		Sessions: sessions,
	})
}

func (h *Handler) listSessionInfo(parent context.Context) ([]SessionInfo, error) {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(parent, 3*time.Second)
		defer cancel()
		sessions, err := h.sessionRoutes.ListBrowserSessions(ctx)
		if err != nil {
			return nil, err
		}
		if sessions == nil {
			return []SessionInfo{}, nil
		}
		return sessions, nil
	}

	service := h.peekService()
	if service == nil {
		return []SessionInfo{}, nil
	}

	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	status, err := service.Status(ctx)
	if err != nil || status == nil || !status.Running {
		return []SessionInfo{}, nil
	}

	tabs, err := service.Tabs(ctx)
	if err != nil {
		return []SessionInfo{}, nil
	}
	return browserSessionResponsesFromTabs(tabs, timeutil.NowTime()), nil
}

func (h *Handler) listOverviewTasks(c echo.Context) ([]map[string]any, error) {
	if h == nil || h.taskProjections == nil {
		return []map[string]any{}, nil
	}

	scope := normalizedBrowserOverviewTaskScope(c.QueryParam("scope"))
	limit := normalizedBrowserOverviewTaskLimit(c.QueryParam("limit"))
	query := BrowserOverviewTaskQuery{
		UserID:         browserUserID(c),
		ConversationID: strings.TrimSpace(c.QueryParam("conversation_id")),
		Scope:          scope,
		Limit:          limit,
	}
	tasks, err := h.taskProjections.List(c.Request().Context(), query)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		return []map[string]any{}, nil
	}
	return tasks, nil
}

func (h *Handler) browserSecurityRuntime() browserSecurityRuntime {
	service := h.getService()
	if runtime, ok := service.(browserSecurityRuntime); ok {
		return runtime
	}
	return nil
}

func browserUserID(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return strings.TrimSpace(claims.UserID)
	}
	raw, _ := c.Get("user_id").(string)
	return strings.TrimSpace(raw)
}

func normalizedBrowserOverviewTaskScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "background":
		return "background"
	case "all":
		return "all"
	default:
		return "current"
	}
}

func normalizedBrowserOverviewTaskLimit(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 12
	}
	return value
}

// CreateSession creates a new browser session.
func (h *Handler) CreateSession(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), browserSessionCreateTimeout)
		defer cancel()
		session, err := h.sessionRoutes.CreateBrowserSession(ctx)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if session == nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "browser session unavailable")
		}
		return c.JSON(http.StatusOK, session)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return mapError(err)
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return mapError(ErrBrowserNotAvailable)
	}

	// Create a context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	// Try to create a new tab
	tab, err := service.OpenTab(ctx, "about:blank")
	if err != nil {
		return mapError(err)
	}
	if tab == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser session unavailable")
	}
	now := timeutil.NowTime()
	return c.JSON(http.StatusOK, SessionInfo{
		ID:           tab.TargetID,
		Status:       "active",
		CurrentURL:   tab.URL,
		PageTitle:    tab.Title,
		CreatedAt:    now.Format(time.RFC3339),
		LastActivity: now.Format(time.RFC3339),
		Engine:       SessionEngineChromiumManaged,
		MonitorKind:  SessionMonitorKindImage,
	})
}

// CloseSession closes a browser session.
func (h *Handler) CloseSession(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		if err := h.sessionRoutes.CloseBrowserSession(ctx, c.Param("id")); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusNoContent)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return c.NoContent(http.StatusNoContent)
	}

	id := c.Param("id")
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	// Try to close the tab
	_ = service.CloseTab(ctx, id)
	return c.NoContent(http.StatusNoContent)
}

// SessionMonitor returns the unified text/image monitor payload for a session.
func (h *Handler) SessionMonitor(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		payload, err := h.sessionRoutes.CaptureBrowserSessionMonitor(ctx, c.Param("id"))
		if err != nil {
			if errors.Is(err, ErrTabNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "session not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, payload)
	}

	screenshotPayload, err := h.captureLegacySessionScreenshot(c, c.Param("id"))
	if err != nil {
		return err
	}
	updatedAt := ""
	if len(screenshotPayload.History) > 0 {
		updatedAt = screenshotPayload.History[0].CapturedAt
	}
	return c.JSON(http.StatusOK, &SessionMonitorResponse{
		Kind: SessionMonitorKindImage,
		Image: &SessionImageMonitor{
			Screenshot: screenshotPayload.Screenshot,
			History:    screenshotPayload.History,
			UpdatedAt:  updatedAt,
			Status:     "active",
		},
		Error: screenshotPayload.Error,
	})
}

func (h *Handler) SessionScreenshot(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		payload, err := h.sessionRoutes.CaptureBrowserSessionScreenshot(ctx, c.Param("id"))
		if err != nil {
			if errors.Is(err, ErrTabNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "session not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, payload)
	}
	payload, err := h.captureLegacySessionScreenshot(c, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) captureLegacySessionScreenshot(c echo.Context, targetID string) (*SessionScreenshotResponse, error) {
	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return &SessionScreenshotResponse{
			Error: "browser service unavailable",
		}, nil
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	var captureErrors []string

	if screenshoter, ok := service.(browserViewportScreenshoter); ok {
		screenshot, err := screenshoter.ScreenshotViewport(ctx, targetID)
		if err == nil && strings.TrimSpace(screenshot) != "" {
			return &SessionScreenshotResponse{
				Screenshot: screenshot,
				History:    sessionScreenshotHistory(service, targetID),
			}, nil
		}
		if err != nil {
			captureErrors = append(captureErrors, err.Error())
		}
	}

	if screenshoter, ok := service.(browserTabScreenshoter); ok {
		screenshot, err := screenshoter.ScreenshotTab(ctx, targetID)
		if err == nil && strings.TrimSpace(screenshot) != "" {
			return &SessionScreenshotResponse{
				Screenshot: screenshot,
				History:    sessionScreenshotHistory(service, targetID),
			}, nil
		}
		if err != nil {
			captureErrors = append(captureErrors, err.Error())
		}
	}

	history := sessionScreenshotHistory(service, targetID)
	latest := ""
	if len(history) > 0 {
		latest = history[0].Data
	}

	errorMessage := "preview unavailable"
	if len(captureErrors) > 0 {
		errorMessage = strings.Join(captureErrors, "; ")
	}

	return &SessionScreenshotResponse{
		Screenshot: latest,
		History:    history,
		Error:      errorMessage,
	}, nil
}

func sessionScreenshotHistory(service Service, targetID string) []SessionScreenshot {
	provider, ok := service.(browserSessionScreenshotHistoryProvider)
	if !ok || provider == nil {
		return nil
	}
	return provider.SessionScreenshotHistory(targetID)
}

func (h *Handler) SessionNavigate(c echo.Context) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), browserSessionNavigateTimeout)
		defer cancel()
		resp, err := h.sessionRoutes.NavigateBrowserSession(ctx, c.Param("id"), req.URL)
		if err != nil {
			if errors.Is(err, ErrTabNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "session not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if resp == nil {
			return c.JSON(http.StatusOK, map[string]string{"status": "navigated"})
		}
		return c.JSON(http.StatusOK, resp)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), browserSessionNavigateTimeout)
	defer cancel()

	resp, err := service.Navigate(ctx, &NavigateRequest{
		URL:      req.URL,
		TargetID: c.Param("id"),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if resp == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "navigated"})
	}
	return c.JSON(http.StatusOK, resp)
}

type browserSessionExecuteRequest struct {
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params"`
}

func normalizeBrowserSessionStepType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "click", "type", "select", "scroll", "hover", "wait", "extract", "navigate", "screenshot":
		return strings.ToLower(strings.TrimSpace(raw))
	case "press", "press_key":
		return "press_key"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func browserStepString(params map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := params[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok {
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func browserStepBool(params map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		value, ok := params[key]
		if !ok {
			continue
		}
		if flag, ok := value.(bool); ok {
			return flag
		}
	}
	return false
}

func browserStepInt(params map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		value, ok := params[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case float32:
			return int(typed)
		case int:
			return typed
		case int64:
			return int(typed)
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed)
			}
		}
	}
	return 0
}

func browserStepStringSlice(params map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		value, ok := params[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []string:
			return append([]string(nil), typed...)
		case []interface{}:
			result := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok {
					text = strings.TrimSpace(text)
					if text != "" {
						result = append(result, text)
					}
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func browserSessionPageInfo(ctx context.Context, service Service, targetID string) (string, string) {
	provider, ok := service.(browserPageInfoProvider)
	if !ok {
		return "", ""
	}
	url, title, err := provider.PageInfo(ctx, targetID)
	if err != nil {
		return "", ""
	}
	return url, title
}

func browserSessionExecuteResult(ctx context.Context, service Service, targetID string) map[string]interface{} {
	url, title := browserSessionPageInfo(ctx, service, targetID)
	return map[string]interface{}{
		"page_title": title,
		"page_url":   url,
	}
}

func browserActRequestFromSessionStep(stepType, targetID string, params map[string]interface{}) (*ActRequest, error) {
	req := &ActRequest{
		TargetID: targetID,
		Selector: browserStepString(params, "selector"),
		Ref:      browserStepString(params, "ref"),
		Value:    browserStepString(params, "value"),
		Text:     browserStepString(params, "text"),
		Key:      browserStepString(params, "key", "value", "text"),
		X:        browserStepInt(params, "x"),
		Y:        browserStepInt(params, "y"),
		Double:   browserStepBool(params, "double"),
		Submit:   browserStepBool(params, "submit"),
		Options:  browserStepStringSlice(params, "options", "values"),
		ToRef:    browserStepString(params, "to_ref", "toRef"),
		Duration: browserStepInt(params, "duration", "wait_for", "waitFor"),
		Timeout:  browserStepInt(params, "timeout"),
	}

	switch stepType {
	case "click":
		req.Kind = "click"
	case "type":
		req.Kind = "type"
	case "select":
		req.Kind = "select"
		if len(req.Options) == 0 && req.Value != "" {
			req.Options = []string{req.Value}
		}
	case "scroll":
		req.Kind = "scroll"
	case "hover":
		req.Kind = "hover"
	case "press_key":
		req.Kind = "press"
	case "wait":
		req.Kind = "wait"
	default:
		return nil, errors.New("unsupported browser session step")
	}

	return req, nil
}

// SessionExecute executes a step in a session.
func (h *Handler) SessionExecute(c echo.Context) error {
	var req browserSessionExecuteRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	stepType := normalizeBrowserSessionStepType(req.Type)
	if stepType == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "step type is required")
	}

	if h.sessionRoutes != nil {
		return echo.NewHTTPError(http.StatusNotImplemented, "browser session step execution unavailable")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return mapError(err)
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return mapError(ErrBrowserNotAvailable)
	}

	targetID := c.Param("id")
	ctx := c.Request().Context()
	result := browserSessionExecuteResult(ctx, service, targetID)

	switch stepType {
	case "navigate":
		rawURL := browserStepString(req.Params, "url", "value")
		if rawURL == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "url is required")
		}
		if _, err := service.Navigate(ctx, &NavigateRequest{
			URL:      rawURL,
			TargetID: targetID,
		}); err != nil {
			return mapError(err)
		}
		return c.JSON(http.StatusOK, browserSessionExecuteResult(ctx, service, targetID))
	case "extract":
		selector := browserStepString(req.Params, "selector")
		if selector == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "selector is required")
		}
		existsProvider, ok := service.(browserElementExistsProvider)
		if !ok {
			return echo.NewHTTPError(http.StatusNotImplemented, "browser extract execution unavailable")
		}
		found, err := existsProvider.ElementExists(ctx, targetID, selector)
		if err != nil {
			return mapError(err)
		}
		result["element_found"] = found
		if found {
			extractor, ok := service.(browserExtractProvider)
			if !ok {
				return echo.NewHTTPError(http.StatusNotImplemented, "browser extract execution unavailable")
			}
			value, err := extractor.ExtractFirstFromTab(ctx, targetID, selector, browserStepString(req.Params, "attribute"))
			if err != nil {
				return mapError(err)
			}
			result["extracted_data"] = value
		} else {
			result["extracted_data"] = nil
		}
		return c.JSON(http.StatusOK, result)
	case "screenshot":
		if screenshoter, ok := service.(browserViewportScreenshoter); ok {
			screenshot, err := screenshoter.ScreenshotViewport(ctx, targetID)
			if err != nil {
				return mapError(err)
			}
			result["screenshot"] = screenshot
			return c.JSON(http.StatusOK, result)
		}
		if screenshoter, ok := service.(browserTabScreenshoter); ok {
			screenshot, err := screenshoter.ScreenshotTab(ctx, targetID)
			if err != nil {
				return mapError(err)
			}
			result["screenshot"] = screenshot
			return c.JSON(http.StatusOK, result)
		}
		return echo.NewHTTPError(http.StatusNotImplemented, "browser screenshot execution unavailable")
	default:
		actReq, err := browserActRequestFromSessionStep(stepType, targetID, req.Params)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		resp, err := service.Act(ctx, actReq)
		if err != nil {
			return mapError(err)
		}
		if resp != nil && resp.Data != nil {
			result["extracted_data"] = resp.Data
		}
		for key, value := range browserSessionExecuteResult(ctx, service, targetID) {
			result[key] = value
		}
		return c.JSON(http.StatusOK, result)
	}
}

// Status returns the browser status.
func (h *Handler) Status(c echo.Context) error {
	service := h.peekService()
	if service == nil {
		return c.JSON(http.StatusOK, &StatusResponse{Running: false})
	}

	status, err := service.Status(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, status)
}

// Start starts the browser.
func (h *Handler) Start(c echo.Context) error {
	service, release, err := h.acquireService()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	err = service.Start(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "started"})
}

// Stop stops the browser.
func (h *Handler) Stop(c echo.Context) error {
	service, release, err := h.acquireService()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "stopped"})
	}

	err = service.Stop(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "stopped"})
}

// Tabs returns all open tabs.
func (h *Handler) Tabs(c echo.Context) error {
	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	tabs, err := service.Tabs(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, tabs)
}

// OpenTab opens a new tab.
func (h *Handler) OpenTab(c echo.Context) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	tab, err := service.OpenTab(c.Request().Context(), req.URL)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, tab)
}

// FocusTab focuses a tab.
func (h *Handler) FocusTab(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "tab id is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	err = service.FocusTab(c.Request().Context(), id)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "focused"})
}

// CloseTab closes a tab.
func (h *Handler) CloseTab(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "tab id is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	err = service.CloseTab(c.Request().Context(), id)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "closed"})
}

// Navigate navigates to a URL.
func (h *Handler) Navigate(c echo.Context) error {
	var req NavigateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Navigate(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Screenshot captures a screenshot.
func (h *Handler) Screenshot(c echo.Context) error {
	var req ScreenshotRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Screenshot(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// PDF generates a PDF.
func (h *Handler) PDF(c echo.Context) error {
	var req PDFRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.PDF(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Snapshot returns a page snapshot.
func (h *Handler) Snapshot(c echo.Context) error {
	var req SnapshotRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Snapshot(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Scrape extracts data from a page.
func (h *Handler) Scrape(c echo.Context) error {
	var req ScrapeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	if len(req.Selectors) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "selectors is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Scrape(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Act performs an action on the page.
func (h *Handler) Act(c echo.Context) error {
	var req ActRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Kind == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "kind is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Act(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Automate runs an automation task.
func (h *Handler) Automate(c echo.Context) error {
	var req AutomateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	if len(req.Steps) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "steps is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Automate(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Console returns console messages.
func (h *Handler) Console(c echo.Context) error {
	var req ConsoleRequest
	req.TargetID = c.QueryParam("target_id")
	req.Level = c.QueryParam("level")
	req.Clear = c.QueryParam("clear") == "true"

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Console(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// ListRecipes returns all available recipes.
func (h *Handler) ListRecipes(c echo.Context) error {
	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	recipes := service.Recipes()
	return c.JSON(http.StatusOK, recipes.List())
}

// ExecuteRecipe runs a browser recipe.
func (h *Handler) ExecuteRecipe(c echo.Context) error {
	var req RecipeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Recipe == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "recipe is required")
	}
	if req.Params == nil {
		req.Params = make(map[string]string)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.ExecuteRecipe(c.Request().Context(), &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

// mapError maps domain errors to HTTP errors.
func mapError(err error) *echo.HTTPError {
	switch err {
	case ErrBrowserNotAvailable, ErrBrowserNotRunning:
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	case ErrPageNotFound, ErrTabNotFound, ErrElementNotFound:
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case ErrURLNotAllowed, ErrURLBlocked:
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case ErrTimeout:
		return echo.NewHTTPError(http.StatusGatewayTimeout, err.Error())
	case ErrResourceLimitExceeded:
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	case ErrInvalidSelector, ErrInvalidAction:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}

// BrowserSecurityConfig represents the browser security configuration.
type BrowserSecurityConfig struct {
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
}

func browserSecurityUnavailableError() *echo.HTTPError {
	return echo.NewHTTPError(http.StatusServiceUnavailable, "browser security unavailable")
}

func normalizeBrowserDomainList(domains []string) []string {
	if len(domains) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(domains))
	result := make([]string, 0, len(domains))
	for _, domain := range domains {
		normalized := strings.ToLower(strings.TrimSpace(domain))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

// GetSecurityConfig returns the configured browser security settings.
func (h *Handler) GetSecurityConfig(c echo.Context) error {
	runtime := h.browserSecurityRuntime()
	if runtime == nil {
		return browserSecurityUnavailableError()
	}
	return c.JSON(http.StatusOK, runtime.currentBrowserSecurityConfig())
}

// UpdateSecurityConfig updates the configured browser security settings.
func (h *Handler) UpdateSecurityConfig(c echo.Context) error {
	runtime := h.browserSecurityRuntime()
	if runtime == nil {
		return browserSecurityUnavailableError()
	}
	var config BrowserSecurityConfig
	if err := c.Bind(&config); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, runtime.replaceBrowserSecurityConfig(config))
}

// AddAllowedDomain adds a domain to the allowed list.
func (h *Handler) AddAllowedDomain(c echo.Context) error {
	runtime := h.browserSecurityRuntime()
	if runtime == nil {
		return browserSecurityUnavailableError()
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Domain == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "domain is required")
	}
	config := runtime.currentBrowserSecurityConfig()
	config.AllowedDomains = append(config.AllowedDomains, req.Domain)
	return c.JSON(http.StatusOK, runtime.replaceBrowserSecurityConfig(config))
}

// RemoveAllowedDomain removes a domain from the allowed list.
func (h *Handler) RemoveAllowedDomain(c echo.Context) error {
	runtime := h.browserSecurityRuntime()
	if runtime == nil {
		return browserSecurityUnavailableError()
	}
	domain := c.Param("domain")
	if domain == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "domain is required")
	}
	config := runtime.currentBrowserSecurityConfig()
	newList := make([]string, 0, len(config.AllowedDomains))
	for _, d := range config.AllowedDomains {
		if !strings.EqualFold(d, domain) {
			newList = append(newList, d)
		}
	}
	config.AllowedDomains = newList
	return c.JSON(http.StatusOK, runtime.replaceBrowserSecurityConfig(config))
}

// AddBlockedDomain adds a domain to the blocked list.
func (h *Handler) AddBlockedDomain(c echo.Context) error {
	runtime := h.browserSecurityRuntime()
	if runtime == nil {
		return browserSecurityUnavailableError()
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Domain == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "domain is required")
	}
	config := runtime.currentBrowserSecurityConfig()
	config.BlockedDomains = append(config.BlockedDomains, req.Domain)
	return c.JSON(http.StatusOK, runtime.replaceBrowserSecurityConfig(config))
}

// RemoveBlockedDomain removes a domain from the blocked list.
func (h *Handler) RemoveBlockedDomain(c echo.Context) error {
	runtime := h.browserSecurityRuntime()
	if runtime == nil {
		return browserSecurityUnavailableError()
	}
	domain := c.Param("domain")
	if domain == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "domain is required")
	}
	config := runtime.currentBrowserSecurityConfig()
	newList := make([]string, 0, len(config.BlockedDomains))
	for _, d := range config.BlockedDomains {
		if !strings.EqualFold(d, domain) {
			newList = append(newList, d)
		}
	}
	config.BlockedDomains = newList
	return c.JSON(http.StatusOK, runtime.replaceBrowserSecurityConfig(config))
}

// TestURL tests whether a URL is allowed by the configured browser security rules.
func (h *Handler) TestURL(c echo.Context) error {
	runtime := h.browserSecurityRuntime()
	if runtime == nil {
		return browserSecurityUnavailableError()
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	if err := runtime.validateBrowserURL(req.URL); err != nil {
		switch err {
		case ErrURLNotAllowed, ErrURLBlocked:
			return c.JSON(http.StatusOK, map[string]interface{}{
				"allowed": false,
				"reason":  err.Error(),
			})
		default:
			return mapError(err)
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"allowed": true,
		"reason":  "",
	})
}
