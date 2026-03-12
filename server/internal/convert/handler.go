package convert

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	service   *Service
	authorize func(ctx context.Context, userID, conversationID string) error
}

func NewHandler(service *Service, authorize func(ctx context.Context, userID, conversationID string) error) *Handler {
	return &Handler{service: service, authorize: authorize}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/capabilities", h.GetCapabilities)
	g.GET("/tasks", h.ListTasks)
	g.GET("/tasks/:id", h.GetTask)
	g.POST("/tasks/:id/cancel", h.CancelTask)
	g.GET("/tasks/:id/download/:output_id", h.DownloadOutput)
	g.GET("/tasks/:id/outputs/:output_id/location", h.GetOutputLocation)
}

func (h *Handler) GetCapabilities(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.Capabilities(c.Request().Context()))
}

func (h *Handler) ListTasks(c echo.Context) error {
	userID := currentUserID(c)
	conversationID := strings.TrimSpace(c.QueryParam("conversation_id"))
	if conversationID != "" && h.authorize != nil {
		if err := h.authorize(c.Request().Context(), userID, conversationID); err != nil {
			return err
		}
	}
	limit := 50
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	tasks, err := h.service.ListTasks(c.Request().Context(), userID, conversationID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"tasks": tasks})
}

func (h *Handler) GetTask(c echo.Context) error {
	userID := currentUserID(c)
	task, err := h.service.GetTask(c.Request().Context(), userID, "", c.Param("id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "task not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, task)
}

func (h *Handler) CancelTask(c echo.Context) error {
	userID := currentUserID(c)
	task, err := h.service.CancelTask(c.Request().Context(), userID, "", c.Param("id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "task not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, task)
}

func (h *Handler) DownloadOutput(c echo.Context) error {
	userID := currentUserID(c)
	_, output, err := h.service.ResolveDownload(c.Request().Context(), userID, "", c.Param("id"), c.Param("output_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "output not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Attachment(output.Path, output.Name)
}

func (h *Handler) GetOutputLocation(c echo.Context) error {
	userID := currentUserID(c)
	_, output, err := h.service.ResolveDownload(c.Request().Context(), userID, "", c.Param("id"), c.Param("output_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "output not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	absPath, err := filepath.Abs(strings.TrimSpace(output.Path))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{
		"path":        absPath,
		"parent_path": filepath.Dir(absPath),
	})
}

func currentUserID(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return claims.UserID
	}
	return ""
}
