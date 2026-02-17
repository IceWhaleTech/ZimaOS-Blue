package audit

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles audit log API endpoints.
type Handler struct {
	logger Logger
}

// NewHandler creates a new audit handler.
func NewHandler(logger Logger) *Handler {
	return &Handler{logger: logger}
}

// ListRequest represents a request to list audit logs.
type ListRequest struct {
	UserID       string `query:"user_id"`
	Action       string `query:"action"`
	ResourceType string `query:"resource_type"`
	ResourceID   string `query:"resource_id"`
	Status       string `query:"status"`
	StartTime    string `query:"start_time"`
	EndTime      string `query:"end_time"`
	IPAddress    string `query:"ip_address"`
	Page         int    `query:"page"`
	PageSize     int    `query:"page_size"`
	SortBy       string `query:"sort_by"`
	SortDir      string `query:"sort_dir"`
}

// List handles GET /api/v1/audit
func (h *Handler) List(c echo.Context) error {
	var req ListRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request parameters")
	}

	params := &QueryParams{
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		IPAddress:    req.IPAddress,
		Page:         req.Page,
		PageSize:     req.PageSize,
		SortBy:       req.SortBy,
		SortDir:      req.SortDir,
	}

	// Parse user ID
	if req.UserID != "" {
		userID, err := uuid.Parse(req.UserID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
		}
		params.UserID = &userID
	}

	// Parse action
	if req.Action != "" {
		action := Action(req.Action)
		params.Action = &action
	}

	// Parse status
	if req.Status != "" {
		status := Status(req.Status)
		params.Status = &status
	}

	// Parse start time
	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid start_time format (use RFC3339)")
		}
		params.StartTime = &t
	}

	// Parse end time
	if req.EndTime != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid end_time format (use RFC3339)")
		}
		params.EndTime = &t
	}

	result, err := h.logger.Query(c.Request().Context(), params)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to query audit logs")
	}

	return c.JSON(http.StatusOK, result)
}

// GetByID handles GET /api/v1/audit/:id
func (h *Handler) GetByID(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid audit log ID")
	}

	entry, err := h.logger.GetByID(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "audit log not found")
	}

	return c.JSON(http.StatusOK, entry)
}

// ExportRequest represents a request to export audit logs.
type ExportRequest struct {
	Format       string `query:"format"` // json or csv
	UserID       string `query:"user_id"`
	Action       string `query:"action"`
	ResourceType string `query:"resource_type"`
	ResourceID   string `query:"resource_id"`
	Status       string `query:"status"`
	StartTime    string `query:"start_time"`
	EndTime      string `query:"end_time"`
	IPAddress    string `query:"ip_address"`
}

// Export handles GET /api/v1/audit/export
func (h *Handler) Export(c echo.Context) error {
	var req ExportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request parameters")
	}

	format := req.Format
	if format == "" {
		format = "json"
	}

	if format != "json" && format != "csv" {
		return echo.NewHTTPError(http.StatusBadRequest, "format must be 'json' or 'csv'")
	}

	params := &QueryParams{
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		IPAddress:    req.IPAddress,
		Page:         1,
		PageSize:     10000, // Export up to 10000 entries
	}

	// Parse filters (same as List)
	if req.UserID != "" {
		userID, err := uuid.Parse(req.UserID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
		}
		params.UserID = &userID
	}

	if req.Action != "" {
		action := Action(req.Action)
		params.Action = &action
	}

	if req.Status != "" {
		status := Status(req.Status)
		params.Status = &status
	}

	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid start_time format")
		}
		params.StartTime = &t
	}

	if req.EndTime != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid end_time format")
		}
		params.EndTime = &t
	}

	result, err := h.logger.Query(c.Request().Context(), params)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to query audit logs")
	}

	filename := fmt.Sprintf("audit_logs_%s.%s", timeutil.NowTime().Format("20060102_150405"), format)

	switch format {
	case "json":
		c.Response().Header().Set("Content-Type", "application/json")
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		return json.NewEncoder(c.Response()).Encode(result.Entries)

	case "csv":
		c.Response().Header().Set("Content-Type", "text/csv")
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		writer := csv.NewWriter(c.Response())
		defer writer.Flush()

		// Write header
		header := []string{
			"ID", "Timestamp", "UserID", "Username", "Action",
			"ResourceType", "ResourceID", "IPAddress", "UserAgent",
			"RequestID", "Status", "Details",
		}
		if err := writer.Write(header); err != nil {
			return err
		}

		// Write entries
		for _, entry := range result.Entries {
			userID := ""
			if entry.UserID != nil {
				userID = entry.UserID.String()
			}

			details := ""
			if entry.Details != nil {
				details = string(entry.Details)
			}

			row := []string{
				entry.ID.String(),
				entry.Timestamp.Format(time.RFC3339),
				userID,
				entry.Username,
				string(entry.Action),
				entry.ResourceType,
				entry.ResourceID,
				entry.IPAddress,
				entry.UserAgent,
				entry.RequestID,
				string(entry.Status),
				details,
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}

		return nil
	}

	return echo.NewHTTPError(http.StatusBadRequest, "unsupported format")
}

// Stats handles GET /api/v1/audit/stats
func (h *Handler) Stats(c echo.Context) error {
	// Get time range from query params
	startTimeStr := c.QueryParam("start_time")
	endTimeStr := c.QueryParam("end_time")

	var startTime, endTime *time.Time

	if startTimeStr != "" {
		t, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid start_time format")
		}
		startTime = &t
	} else {
		// Default to last 24 hours
		t := timeutil.NowTime().Add(-24 * time.Hour)
		startTime = &t
	}

	if endTimeStr != "" {
		t, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid end_time format")
		}
		endTime = &t
	}

	// Get counts by action
	actions := []Action{
		ActionLogin, ActionLoginFailed, ActionLogout,
		ActionUserCreate, ActionUserUpdate, ActionUserDelete,
		ActionMFASetup, ActionMFADisable,
	}

	stats := make(map[string]int64)
	for _, action := range actions {
		params := &QueryParams{
			Action:    &action,
			StartTime: startTime,
			EndTime:   endTime,
			Page:      1,
			PageSize:  1,
		}
		result, err := h.logger.Query(c.Request().Context(), params)
		if err != nil {
			continue
		}
		stats[string(action)] = result.Total
	}

	// Get success/failure counts
	successStatus := StatusSuccess
	failureStatus := StatusFailure

	successParams := &QueryParams{
		Status:    &successStatus,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      1,
		PageSize:  1,
	}
	successResult, _ := h.logger.Query(c.Request().Context(), successParams)

	failureParams := &QueryParams{
		Status:    &failureStatus,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      1,
		PageSize:  1,
	}
	failureResult, _ := h.logger.Query(c.Request().Context(), failureParams)

	response := map[string]interface{}{
		"by_action": stats,
		"by_status": map[string]int64{
			"success": successResult.Total,
			"failure": failureResult.Total,
		},
		"time_range": map[string]interface{}{
			"start": startTime,
			"end":   endTime,
		},
	}

	return c.JSON(http.StatusOK, response)
}

// RegisterRoutes registers the audit routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("", h.List)
	g.GET("/export", h.Export)
	g.GET("/stats", h.Stats)
	g.GET("/:id", h.GetByID)
}

// parseIntParam parses an integer query parameter with a default value.
func parseIntParam(c echo.Context, name string, defaultValue int) int {
	str := c.QueryParam(name)
	if str == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue
	}
	return val
}
