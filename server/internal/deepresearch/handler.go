package deepresearch

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	g0 := e.Group("/api/v1/deep-research")
	h.RegisterGroup(g0)

	g00 := e.Group("/api/deep-research")
	h.RegisterGroup(g00)
}

func (h *Handler) RegisterGroup(g *echo.Group) {
	g.POST("/jobs", h.CreateJob)
	g.GET("/jobs/:id", h.GetJob)
	g.GET("/jobs/:id/report", h.GetReport)
	g.POST("/jobs/:id/cancel", h.CancelJob)
	g.GET("/jobs/:id/events", h.StreamEvents)
}

func (h *Handler) CreateJob(c echo.Context) error {
	var req CreateJobRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	job, err := h.service.CreateJob(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, job)
}

func (h *Handler) GetJob(c echo.Context) error {
	job, err := h.service.GetJob(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, job)
}

func (h *Handler) GetReport(c echo.Context) error {
	report, err := h.service.GetReport(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) CancelJob(c echo.Context) error {
	if err := h.service.CancelJob(c.Param("id")); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) StreamEvents(c echo.Context) error {
	jobID := c.Param("id")
	events, unsubscribe, err := h.service.Subscribe(jobID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	defer unsubscribe()

	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")
	res.Header().Set("X-Accel-Buffering", "no")
	res.WriteHeader(http.StatusOK)
	res.Flush()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			raw, _ := json.Marshal(ev.Payload)
			if _, err := fmt.Fprintf(res, "event: %s\ndata: %s\n\n", ev.Type, raw); err != nil {
				return nil
			}
			res.Flush()
		}
	}
}
