package backup

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles backup-related API endpoints
type Handler struct {
	manager *Manager
}

// NewHandler creates a new backup handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes registers backup routes on the given group
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/backup", h.List)
	g.POST("/backup", h.Create)
	g.GET("/backup/:id", h.Get)
	g.DELETE("/backup/:id", h.Delete)
	g.POST("/backup/:id/restore", h.Restore)
	g.POST("/backup/:id/verify", h.Verify)
	g.POST("/backup/:id/repair", h.Repair)
}

// List returns all available backups
func (h *Handler) List(c echo.Context) error {
	backups := h.manager.List()
	return c.JSON(http.StatusOK, backups)
}

// CreateRequest represents a backup creation request
type CreateRequest struct {
	Type string `json:"type"` // full, config, data
}

// Create creates a new backup
func (h *Handler) Create(c echo.Context) error {
	var req CreateRequest
	if err := c.Bind(&req); err != nil {
		req.Type = "full" // Default to full backup
	}

	backupType := BackupType(req.Type)
	if backupType != BackupTypeFull && backupType != BackupTypeConfig && backupType != BackupTypeData {
		backupType = BackupTypeFull
	}

	info, err := h.manager.Create(context.Background(), backupType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to create backup",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, info)
}

// Get returns a specific backup by ID
func (h *Handler) Get(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Backup ID is required",
		})
	}

	info, err := h.manager.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"success": false,
			"message": "Backup not found",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, info)
}

// Delete removes a backup
func (h *Handler) Delete(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Backup ID is required",
		})
	}

	if err := h.manager.Delete(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"success": false,
			"message": "Failed to delete backup",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Backup deleted successfully",
	})
}

// RestoreRequest represents a restore request
type RestoreRequest struct {
	Force bool `json:"force"` // Skip checksum verification
}

// Restore restores from a backup
func (h *Handler) Restore(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Backup ID is required",
		})
	}

	// Parse request body for options
	var req RestoreRequest
	c.Bind(&req) // Ignore error, use defaults if not provided

	// Verify backup exists
	if _, err := h.manager.Get(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"success": false,
			"message": "Backup not found",
			"error":   err.Error(),
		})
	}

	// Perform restore with options
	opts := DefaultRestoreOptions()
	opts.SkipVerify = req.Force

	result, err := h.manager.Restore(context.Background(), id, opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to restore backup",
			"error":   err.Error(),
		})
	}

	if !result.Success {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Restore completed with errors",
			"result":  result,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Backup restored successfully. Service may need to restart.",
		"result":  result,
	})
}

// Verify checks the integrity of a backup
func (h *Handler) Verify(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Backup ID is required",
		})
	}

	if err := h.manager.Verify(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Backup verification failed",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Backup verified successfully",
	})
}

// Repair recalculates and updates the checksum for a backup
func (h *Handler) Repair(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Backup ID is required",
		})
	}

	if err := h.manager.RepairChecksum(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to repair backup checksum",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Backup checksum repaired successfully",
	})
}
