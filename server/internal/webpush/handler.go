package webpush

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

// Handler provides HTTP endpoints for Web Push subscription management.
type Handler struct {
	store     *Store
	publicKey string
}

// NewHandler creates a new Web Push handler.
func NewHandler(store *Store, publicKey string) *Handler {
	return &Handler{store: store, publicKey: publicKey}
}

// RegisterRoutes registers Web Push routes on the given echo group.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/vapid-key", h.GetVAPIDKey)
	g.POST("/subscribe", h.Subscribe)
	g.DELETE("/subscribe", h.Unsubscribe)
}

// GetVAPIDKey returns the VAPID public key for browser subscription.
func (h *Handler) GetVAPIDKey(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"public_key": h.publicKey,
	})
}

type subscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Subscribe saves a push subscription for the authenticated user.
func (h *Handler) Subscribe(c echo.Context) error {
	userID := getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing user")
	}

	var req subscribeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing subscription fields")
	}

	sub := &Subscription{
		Endpoint:  req.Endpoint,
		KeyP256dh: req.Keys.P256dh,
		KeyAuth:   req.Keys.Auth,
	}
	if err := h.store.Subscribe(c.Request().Context(), userID, sub); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save subscription")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "subscribed"})
}

type unsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// Unsubscribe removes a push subscription.
func (h *Handler) Unsubscribe(c echo.Context) error {
	var req unsubscribeRequest
	if err := c.Bind(&req); err != nil || req.Endpoint == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing endpoint")
	}

	if err := h.store.Unsubscribe(c.Request().Context(), req.Endpoint); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove subscription")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "unsubscribed"})
}

func getUserID(c echo.Context) string {
	if claims, ok := c.Get("user").(*auth.Claims); ok && claims != nil {
		return claims.UserID
	}
	return ""
}
