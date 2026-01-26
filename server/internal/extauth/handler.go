package extauth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles external auth HTTP requests.
type Handler struct {
	service Service
}

// NewHandler creates a new external auth handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the external auth routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/providers", h.ListProviders)
	g.GET("/oidc/:provider/authorize", h.Authorize)
	g.GET("/oidc/:provider/callback", h.Callback)
	g.POST("/oidc/:provider/token", h.ExchangeToken)
	g.POST("/oidc/:provider/refresh", h.RefreshToken)
	g.GET("/oidc/:provider/userinfo", h.GetUserInfo)

	// Account linking (requires authentication)
	g.POST("/link/:provider", h.LinkAccount)
	g.DELETE("/link/:provider", h.UnlinkAccount)
	g.GET("/linked", h.GetLinkedAccounts)
}

// ListProviders returns all enabled providers.
func (h *Handler) ListProviders(c echo.Context) error {
	providers, err := h.service.ListProviders(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, providers)
}

// Authorize starts the authorization flow.
func (h *Handler) Authorize(c echo.Context) error {
	providerID := c.Param("provider")
	if providerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	req := &AuthorizeRequest{
		ProviderID:  providerID,
		RedirectURI: c.QueryParam("redirect_uri"),
		State:       c.QueryParam("state"),
	}

	resp, err := h.service.Authorize(c.Request().Context(), req)
	if err != nil {
		return mapError(err)
	}

	// Redirect to provider's authorization page
	return c.Redirect(http.StatusFound, resp.AuthURL)
}

// Callback handles the authorization callback.
func (h *Handler) Callback(c echo.Context) error {
	providerID := c.Param("provider")
	if providerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	req := &CallbackRequest{
		ProviderID:       providerID,
		Code:             c.QueryParam("code"),
		State:            c.QueryParam("state"),
		Error:            c.QueryParam("error"),
		ErrorDescription: c.QueryParam("error_description"),
	}

	resp, err := h.service.Callback(c.Request().Context(), req)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, resp)
}

// ExchangeToken exchanges an authorization code for tokens.
func (h *Handler) ExchangeToken(c echo.Context) error {
	providerID := c.Param("provider")
	if providerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	var req struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	callbackReq := &CallbackRequest{
		ProviderID: providerID,
		Code:       req.Code,
		State:      req.State,
	}

	resp, err := h.service.Callback(c.Request().Context(), callbackReq)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, resp)
}

// RefreshToken refreshes an access token.
func (h *Handler) RefreshToken(c echo.Context) error {
	providerID := c.Param("provider")
	if providerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.RefreshToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "refresh_token is required")
	}

	resp, err := h.service.RefreshToken(c.Request().Context(), providerID, req.RefreshToken)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, resp)
}

// GetUserInfo gets user info from the provider.
func (h *Handler) GetUserInfo(c echo.Context) error {
	providerID := c.Param("provider")
	if providerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	accessToken := c.Request().Header.Get("X-Provider-Token")
	if accessToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "X-Provider-Token header is required")
	}

	userInfo, err := h.service.GetUserInfo(c.Request().Context(), providerID, accessToken)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, userInfo)
}

// LinkAccount links an external account to the current user.
func (h *Handler) LinkAccount(c echo.Context) error {
	providerID := c.Param("provider")
	if providerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	// Get user ID from context (set by auth middleware)
	userID := c.Get("user_id")
	if userID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	req := &AuthorizeRequest{
		ProviderID:  providerID,
		RedirectURI: c.QueryParam("redirect_uri"),
	}

	resp, err := h.service.LinkAccount(c.Request().Context(), userID.(string), req)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, resp)
}

// UnlinkAccount unlinks an external account from the current user.
func (h *Handler) UnlinkAccount(c echo.Context) error {
	providerID := c.Param("provider")
	if providerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	// Get user ID from context (set by auth middleware)
	userID := c.Get("user_id")
	if userID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	err := h.service.UnlinkAccount(c.Request().Context(), userID.(string), providerID)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "unlinked"})
}

// GetLinkedAccounts returns all linked accounts for the current user.
func (h *Handler) GetLinkedAccounts(c echo.Context) error {
	// Get user ID from context (set by auth middleware)
	userID := c.Get("user_id")
	if userID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	accounts, err := h.service.GetLinkedAccounts(c.Request().Context(), userID.(string))
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, accounts)
}

// mapError maps domain errors to HTTP errors.
func mapError(err error) *echo.HTTPError {
	switch err {
	case ErrProviderNotFound:
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case ErrProviderDisabled:
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case ErrInvalidState, ErrStateExpired:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case ErrInvalidCode, ErrInvalidToken:
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	case ErrUserNotFound:
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case ErrAccountAlreadyLinked:
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case ErrEmailNotAllowed:
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case ErrDiscoveryFailed:
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}
