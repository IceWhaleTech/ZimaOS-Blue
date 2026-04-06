package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tenant"
)

type Handler struct {
	service *Service
	creator interface {
		CreateJob(ctx context.Context, req CreateJobRequest) (*KnowledgeJob, error)
	}
}

const (
	defaultSSERetryMillis = 2000
	sseHeartbeatInterval  = 15 * time.Second
)

func NewHandler(service *Service) *Handler {
	return &Handler{service: service, creator: service}
}

func (h *Handler) SetJobCreator(creator interface {
	CreateJob(ctx context.Context, req CreateJobRequest) (*KnowledgeJob, error)
}) {
	if h == nil || creator == nil {
		return
	}
	h.creator = creator
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	g0 := e.Group("/api/v1/knowledge")
	h.RegisterGroup(g0)

	g00 := e.Group("/api/knowledge")
	h.RegisterGroup(g00)
}

func (h *Handler) RegisterGroup(g *echo.Group) {
	g.POST("/jobs", h.CreateJob)
	g.GET("/jobs", h.ListJobs)
	g.GET("/jobs/:id", h.GetJob)
	g.GET("/jobs/:id/report", h.GetReport)
	g.POST("/jobs/:id/cancel", h.CancelJob)
	g.GET("/jobs/:id/events", h.StreamEvents)
	g.POST("/query/:id/promote", h.PromoteQuery)
	g.GET("/pages", h.ListPages)
	g.GET("/pages/:slug", h.GetPage)
	g.GET("/index", h.GetIndex)
	g.GET("/schema", h.GetSchema)
	g.PUT("/schema", h.UpdateSchema)
	g.GET("/log", h.GetLog)
	g.GET("/lint/latest", h.GetLatestLint)
}

func (h *Handler) CreateJob(c echo.Context) error {
	var req CreateJobRequest
	if err := c.Bind(&req); err != nil {
		return writeServiceError(c, http.StatusBadRequest, "invalid_request", err)
	}
	userID, tenantID := actorFromContext(c)
	req.UserID = userID
	req.TenantID = tenantID

	creator := h.creator
	if creator == nil {
		creator = h.service
	}
	job, err := creator.CreateJob(c.Request().Context(), req)
	if err != nil {
		return mapServiceError(c, err, http.StatusBadRequest)
	}
	return c.JSON(http.StatusCreated, job)
}

func (h *Handler) ListJobs(c echo.Context) error {
	userID, tenantID := actorFromContext(c)
	status := strings.TrimSpace(strings.ToLower(c.QueryParam("status")))
	activeOnly := status == "active"
	jobs, err := h.service.ListJobsForUser(userID, tenantID, activeOnly)
	if err != nil {
		return mapServiceError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, jobs)
}

func (h *Handler) GetJob(c echo.Context) error {
	userID, tenantID := actorFromContext(c)
	job, err := h.service.GetJobForUser(c.Param("id"), userID, tenantID)
	if err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, job)
}

func (h *Handler) GetReport(c echo.Context) error {
	userID, tenantID := actorFromContext(c)
	report, err := h.service.GetReportForUser(c.Param("id"), userID, tenantID)
	if err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) CancelJob(c echo.Context) error {
	userID, tenantID := actorFromContext(c)
	if err := h.service.CancelJobForUser(c.Param("id"), userID, tenantID); err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) StreamEvents(c echo.Context) error {
	jobID := c.Param("id")
	userID, tenantID := actorFromContext(c)
	events, unsubscribe, err := h.service.SubscribeForUser(jobID, userID, tenantID)
	if err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	defer unsubscribe()

	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")
	res.Header().Set("X-Accel-Buffering", "no")
	res.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(res, "retry: %d\n\n", defaultSSERetryMillis); err != nil {
		return nil
	}
	res.Flush()

	ctx := c.Request().Context()
	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeat.C:
			if _, err := fmt.Fprint(res, ": keep-alive\n\n"); err != nil {
				return nil
			}
			res.Flush()
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			raw, _ := json.Marshal(ev.Payload)
			if ev.ID != "" {
				if _, err := fmt.Fprintf(res, "id: %s\n", ev.ID); err != nil {
					return nil
				}
			}
			if _, err := fmt.Fprintf(res, "event: %s\ndata: %s\n\n", ev.Type, raw); err != nil {
				return nil
			}
			res.Flush()
		}
	}
}

func (h *Handler) ListPages(c echo.Context) error {
	pages, err := h.service.ListPages(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err, http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, pages)
}

func (h *Handler) GetPage(c echo.Context) error {
	page, err := h.service.GetPage(c.Request().Context(), c.Param("slug"))
	if err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, page)
}

func (h *Handler) GetIndex(c echo.Context) error {
	content, err := h.service.GetIndexMarkdown(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, map[string]string{"content": content})
}

func (h *Handler) GetSchema(c echo.Context) error {
	doc, err := h.service.GetSchema(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, doc)
}

func (h *Handler) UpdateSchema(c echo.Context) error {
	var req KnowledgeSchemaDocument
	if err := c.Bind(&req); err != nil {
		return writeServiceError(c, http.StatusBadRequest, "invalid_request", err)
	}
	doc, err := h.service.UpdateSchema(c.Request().Context(), req.Content)
	if err != nil {
		return mapServiceError(c, err, http.StatusBadRequest)
	}
	return c.JSON(http.StatusOK, doc)
}

func (h *Handler) GetLog(c echo.Context) error {
	entries, err := h.service.GetLog(c.Request().Context(), 100)
	if err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, entries)
}

func (h *Handler) PromoteQuery(c echo.Context) error {
	userID, tenantID := actorFromContext(c)
	if _, err := h.service.GetJobForUser(c.Param("id"), userID, tenantID); err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	page, err := h.service.PromoteQuery(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapServiceError(c, err, http.StatusBadRequest)
	}
	return c.JSON(http.StatusCreated, PromoteQueryResult{Page: *page})
}

func (h *Handler) GetLatestLint(c echo.Context) error {
	report, err := h.service.GetLatestLint(c.Request().Context())
	if err != nil {
		return mapServiceError(c, err, http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, report)
}

func actorFromContext(c echo.Context) (userID, tenantID string) {
	if claims := auth.GetUserFromContext(c); claims != nil {
		userID = strings.TrimSpace(claims.UserID)
	}
	tid := tenant.GetTenantIDFromContext(c.Request().Context())
	if tid != uuid.Nil {
		tenantID = tid.String()
	}
	if tenantID == "" {
		tenantID = strings.TrimSpace(c.Request().Header.Get("X-Tenant-ID"))
	}
	if tenantID == "" {
		tenantID = strings.TrimSpace(c.QueryParam("tenant_id"))
	}
	return userID, tenantID
}

func mapServiceError(c echo.Context, err error, defaultStatus int) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return writeServiceError(c, http.StatusRequestTimeout, "request_timeout", err)
	}
	switch err {
	case nil:
		return nil
	case ErrJobNotFound, ErrPageNotFound:
		return writeServiceError(c, http.StatusNotFound, "not_found", err)
	case ErrJobForbidden:
		return writeServiceError(c, http.StatusForbidden, "forbidden", err)
	case ErrReportNotReady:
		return writeServiceError(c, http.StatusTooEarly, "not_ready", err)
	case ErrJobAlreadyTerminal:
		return writeServiceError(c, http.StatusConflict, "already_terminal", err)
	default:
		return writeServiceError(c, defaultStatus, "request_failed", err)
	}
}

func writeServiceError(c echo.Context, status int, code string, err error) error {
	return c.JSON(status, map[string]string{
		"code":  code,
		"error": err.Error(),
	})
}
