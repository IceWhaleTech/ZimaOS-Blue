package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
)

// NetworkHandler handles network-related API requests.
type NetworkHandler struct {
	detector *network.AddressDetector
}

// NewNetworkHandler creates a new network handler.
func NewNetworkHandler(port int) *NetworkHandler {
	return &NetworkHandler{
		detector: network.NewAddressDetector(port),
	}
}

// RegisterRoutes registers network routes.
func (h *NetworkHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/network")
	g.GET("/addresses", h.GetAddresses)
	g.GET("/status", h.GetStatus)
	g.GET("/preferred", h.GetPreferred)
}

// InitializeCORSOrigins detects local network addresses and adds them to CORS allowed origins.
// This should be called during server initialization.
func (h *NetworkHandler) InitializeCORSOrigins() error {
	addresses, err := h.detector.GetAddresses()
	if err != nil {
		return err
	}

	// Add all LAN addresses to CORS allowed origins
	for _, iface := range addresses.LAN {
		security.AddDynamicOriginDefault(iface.Address)
	}

	// Add hostname if available
	if addresses.Hostname != "" {
		security.AddDynamicOriginDefault(addresses.Hostname)
	}

	return nil
}

// GetAddresses returns all available network addresses.
// @Summary Get network addresses
// @Description Returns all available network addresses for accessing the application
// @Tags network
// @Produce json
// @Success 200 {object} network.NetworkAddresses
// @Router /api/v1/network/addresses [get]
func (h *NetworkHandler) GetAddresses(c echo.Context) error {
	addresses, err := h.detector.GetAddresses()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "Failed to get network addresses",
			"details": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, addresses)
}

// GetStatus returns the network connectivity status.
// @Summary Get network status
// @Description Returns the network connectivity status
// @Tags network
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/network/status [get]
func (h *NetworkHandler) GetStatus(c echo.Context) error {
	addresses, err := h.detector.GetAddresses()
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":           "degraded",
			"error":            err.Error(),
			"interfaces_count": 0,
			"has_lan_access":   false,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":           "healthy",
		"interfaces_count": len(addresses.LAN),
		"has_lan_access":   len(addresses.LAN) > 0,
		"has_hostname":     addresses.Hostname != "",
	})
}

// GetPreferred returns the preferred network address for sharing.
// @Summary Get preferred address
// @Description Returns the best network address for sharing with other devices
// @Tags network
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/network/preferred [get]
func (h *NetworkHandler) GetPreferred(c echo.Context) error {
	preferred, err := h.detector.GetPreferredAddress()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "Failed to get preferred address",
			"details": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"address": preferred,
	})
}
