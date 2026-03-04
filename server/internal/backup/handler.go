package backup

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Handler handles backup-related API endpoints
type Handler struct {
	manager    *Manager
	restartFn  func() error
}

// NewHandler creates a new backup handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// SetRestartFunc sets the restart callback used for auto-restart after staged restore.
func (h *Handler) SetRestartFunc(fn func() error) {
	h.restartFn = fn
}

// RegisterRoutes registers backup routes on the given group
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/backup", h.List)
	g.POST("/backup", h.Create)
	g.GET("/backup/progress", h.GetProgress)
	g.GET("/backup/:id", h.Get)
	g.DELETE("/backup/:id", h.Delete)
	g.POST("/backup/:id/restore", h.Restore)
	g.POST("/backup/:id/stage-restore", h.StageRestore) // New: stage for restart
	g.POST("/backup/:id/verify", h.Verify)
	g.POST("/backup/:id/repair", h.Repair)
	g.GET("/backup/pending", h.GetPendingRestore)
	g.DELETE("/backup/pending", h.CancelPendingRestore)
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
	Force            bool  `json:"force"`                       // Skip checksum verification
	RequireRestart   *bool `json:"require_restart,omitempty"`   // Default true for DB-safe restore
	CreateCheckpoint *bool `json:"create_checkpoint,omitempty"` // Default true
	AutoRestart      *bool `json:"auto_restart,omitempty"`      // Default true when require_restart=true
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

	requireRestart := true
	if req.RequireRestart != nil {
		requireRestart = *req.RequireRestart
	}
	autoRestart := true
	if req.AutoRestart != nil {
		autoRestart = *req.AutoRestart
	}

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
	if req.CreateCheckpoint != nil {
		opts.CreateCheckpoint = *req.CreateCheckpoint
	}

	if requireRestart {
		pending, err := h.manager.StageRestore(context.Background(), id)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"message": "Failed to stage restore",
				"error":   err.Error(),
			})
		}
		response := map[string]interface{}{
			"success":          true,
			"requires_restart": true,
			"pending_restore":  pending,
		}

		if autoRestart && h.restartFn != nil {
			response["restarting"] = true
			response["message"] = "Restore staged successfully. Service will restart automatically to apply safely."
			c.Response().Header().Set("Connection", "close")
			if err := c.JSON(http.StatusOK, response); err != nil {
				return err
			}
			c.Response().Flush()

			go func() {
				time.Sleep(250 * time.Millisecond)
				if err := h.restartFn(); err != nil {
					log.Printf("[backup] auto restart failed: %v", err)
				}
			}()
			return nil
		}

		response["restarting"] = false
		response["message"] = "Restore staged successfully. Restart service to apply safely (DB files will be replaced while offline)."
		return c.JSON(http.StatusOK, response)
	}

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

// StageRestore stages a backup for restore on next restart.
// This is the recommended approach for hot recovery to avoid database lock issues.
func (h *Handler) StageRestore(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Backup ID is required",
		})
	}

	pending, err := h.manager.StageRestore(context.Background(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to stage restore",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":          true,
		"message":          "Restore staged successfully. Please restart the service to apply.",
		"pending_restore":  pending,
		"requires_restart": true,
	})
}

// GetPendingRestore returns the pending restore info if any
func (h *Handler) GetPendingRestore(c echo.Context) error {
	pending, err := h.manager.GetPendingRestore()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to get pending restore",
			"error":   err.Error(),
		})
	}

	if pending == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":         true,
			"has_pending":     false,
			"pending_restore": nil,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":         true,
		"has_pending":     true,
		"pending_restore": pending,
	})
}

// CancelPendingRestore cancels a pending restore operation
func (h *Handler) CancelPendingRestore(c echo.Context) error {
	if err := h.manager.CancelPendingRestore(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to cancel pending restore",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pending restore cancelled",
	})
}

// ProgressResponse represents the backup/restore progress
type ProgressResponse struct {
	InProgress     bool   `json:"in_progress"`
	Operation      string `json:"operation"` // "backup" or "restore"
	Progress       int    `json:"progress"`  // 0-100
	CurrentFile    string `json:"current_file"`
	FilesProcessed int    `json:"files_processed"`
	TotalFiles     int    `json:"total_files"`
	BytesProcessed int64  `json:"bytes_processed"`
	TotalBytes     int64  `json:"total_bytes"`
	StartedAt      string `json:"started_at"`
	Error          string `json:"error,omitempty"`
}

// GetProgress returns the current backup/restore progress
func (h *Handler) GetProgress(c echo.Context) error {
	progress := h.manager.GetProgress()
	return c.JSON(http.StatusOK, progress)
}
