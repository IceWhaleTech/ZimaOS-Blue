package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/reclaim"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for workflows.
type Handler struct {
	service *WorkflowService

	// Lazy init support
	initFn          func() *WorkflowService
	serviceInitHook func(*WorkflowService)
	lazy            *reclaim.Managed[*WorkflowService]

	routeMiddlewares []echo.MiddlewareFunc
}

// NewHandler creates a new workflow handler.
func NewHandler(service *WorkflowService) *Handler {
	return &Handler{service: service}
}

// NewLazyHandler creates a handler that defers WorkflowService creation to the first API call.
func NewLazyHandler(initFn func() *WorkflowService) *Handler {
	h := &Handler{initFn: initFn}
	h.lazy = reclaim.NewManaged[*WorkflowService](0, func() (*WorkflowService, error) {
		if h.initFn == nil {
			return nil, nil
		}
		svc := h.initFn()
		if svc == nil {
			return nil, fmt.Errorf("workflow service unavailable")
		}
		return svc, nil
	}, func(_ context.Context, svc *WorkflowService) error {
		if svc == nil {
			return nil
		}
		return svc.Close()
	})
	return h
}

// svc returns the service, initializing lazily if needed.
func (h *Handler) svc() *WorkflowService {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service
	}
	svc, _ := h.lazy.Get()
	return svc
}

// GetService returns the workflow service, triggering lazy init if needed.
func (h *Handler) GetService() *WorkflowService {
	return h.svc()
}

// SetIdleReclaim configures idle reclaim for the lazily initialized workflow service.
func (h *Handler) SetIdleReclaim(idleAfter time.Duration) {
	if h == nil || h.lazy == nil {
		return
	}
	h.lazy.SetIdleAfter(idleAfter)
}

func (h *Handler) withService(fn func(*WorkflowService) error) error {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return fn(h.service)
	}

	svc, release, err := h.lazy.Acquire()
	if err != nil {
		return err
	}
	defer release()
	return fn(svc)
}

// SetServiceInitHook configures a callback that runs once the lazy service is created.
func (h *Handler) SetServiceInitHook(fn func(*WorkflowService)) {
	if h == nil {
		return
	}
	if fn == nil {
		return
	}
	if h.serviceInitHook == nil {
		h.serviceInitHook = fn
	} else {
		prev := h.serviceInitHook
		h.serviceInitHook = func(svc *WorkflowService) {
			prev(svc)
			fn(svc)
		}
	}
	if h.lazy != nil {
		svc, ok := h.lazy.Peek()
		h.lazy.SetOnCreateSilently(h.serviceInitHook)
		if ok && svc != nil {
			fn(svc)
		}
		return
	}
	if h.service != nil {
		fn(h.service)
	}
}

// SetRouteMiddlewares applies security middleware to workflow management routes.
// Webhook routes stay public and are intentionally excluded.
func (h *Handler) SetRouteMiddlewares(middlewares ...echo.MiddlewareFunc) {
	h.routeMiddlewares = append([]echo.MiddlewareFunc(nil), middlewares...)
}

// getContextString safely gets a string value from echo context with a default fallback.
func getContextString(c echo.Context, key, defaultValue string) string {
	if val := c.Get(key); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func requestContext(c echo.Context) context.Context {
	return withWorkflowTenant(c.Request().Context(), getContextString(c, "tenant_id", "default"))
}

// RegisterRoutes registers the workflow routes.
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	// Register under /api/v1/workflows
	g := e.Group("/api/v1/workflows", h.routeMiddlewares...)
	h.registerWorkflowRoutes(g)

	// Also register under /api/workflows for frontend compatibility
	g2 := e.Group("/api/workflows", h.routeMiddlewares...)
	h.registerWorkflowRoutes(g2)

	// Webhooks
	webhookGroup := e.Group("/api/v1/webhooks")
	webhookGroup.Any("/*", h.HandleWebhook)
}

// registerWorkflowRoutes registers workflow routes on a group.
func (h *Handler) registerWorkflowRoutes(g *echo.Group) {
	// Templates (must be before /:id to avoid conflict)
	g.GET("/templates", h.GetTemplates)

	// Workflow CRUD
	g.POST("", h.CreateWorkflow)
	g.GET("", h.ListWorkflows)
	g.GET("/:id", h.GetWorkflow)
	g.PUT("/:id", h.UpdateWorkflow)
	g.DELETE("/:id", h.DeleteWorkflow)

	// Workflow control
	g.POST("/:id/enable", h.EnableWorkflow)
	g.POST("/:id/disable", h.DisableWorkflow)
	g.POST("/:id/validate", h.ValidateWorkflow)

	// Execution
	g.POST("/:id/execute", h.ExecuteWorkflow)
	g.GET("/:id/executions", h.ListExecutions)
	g.GET("/:id/executions/:executionId", h.GetExecution)
	g.POST("/:id/executions/:executionId/cancel", h.CancelExecution)
	g.POST("/:id/executions/:executionId/retry", h.RetryExecution)
	g.POST("/:id/executions/:executionId/resume", h.ResumeExecution)
	g.GET("/:id/executions/:executionId/logs", h.GetExecutionLogs)

	// Import/Export
	g.POST("/import", h.ImportWorkflow)
	g.GET("/:id/export", h.ExportWorkflow)

	// Stats
	g.GET("/stats", h.GetStats)
}

// CreateWorkflowRequest represents a create workflow request.
type CreateWorkflowRequest struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description,omitempty"`
	Nodes       []Node            `json:"nodes,omitempty"`
	Connections []Connection      `json:"connections,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Settings    *WorkflowSettings `json:"settings,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// CreateWorkflow creates a new workflow.
func (h *Handler) CreateWorkflow(c echo.Context) error {
	var req CreateWorkflowRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	tenantID := getContextString(c, "tenant_id", "default")
	userID := getContextString(c, "user_id", "anonymous")

	workflow := &Workflow{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Status:      WorkflowStatusDraft,
		Nodes:       req.Nodes,
		Connections: req.Connections,
		Variables:   req.Variables,
		Settings:    req.Settings,
		Tags:        req.Tags,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}

	var created *Workflow
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		created, err = svc.CreateWorkflow(requestContext(c), workflow)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, created)
}

// GetWorkflow retrieves a workflow by ID.
func (h *Handler) GetWorkflow(c echo.Context) error {
	id := c.Param("id")

	var workflow *Workflow
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		workflow, err = svc.GetWorkflow(requestContext(c), id)
		return err
	})
	if err != nil {
		if err == ErrWorkflowNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, workflow)
}

// UpdateWorkflowRequest represents an update workflow request.
type UpdateWorkflowRequest struct {
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Status      WorkflowStatus    `json:"status,omitempty"`
	Nodes       []Node            `json:"nodes,omitempty"`
	Connections []Connection      `json:"connections,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Settings    *WorkflowSettings `json:"settings,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// ResumeExecutionRequest represents a resume execution request.
type ResumeExecutionRequest struct {
	Decision string                 `json:"decision,omitempty"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
}

// UpdateWorkflow updates an existing workflow.
func (h *Handler) UpdateWorkflow(c echo.Context) error {
	id := c.Param("id")

	var req UpdateWorkflowRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Get existing workflow
	var workflow *Workflow
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		workflow, err = svc.GetWorkflow(requestContext(c), id)
		return err
	})
	if err != nil {
		if err == ErrWorkflowNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Update fields
	if req.Name != "" {
		workflow.Name = req.Name
	}
	if req.Description != "" {
		workflow.Description = req.Description
	}
	if req.Status != "" {
		workflow.Status = req.Status
	}
	if req.Nodes != nil {
		workflow.Nodes = req.Nodes
	}
	if req.Connections != nil {
		workflow.Connections = req.Connections
	}
	if req.Variables != nil {
		workflow.Variables = req.Variables
	}
	if req.Settings != nil {
		workflow.Settings = req.Settings
	}
	if req.Tags != nil {
		workflow.Tags = req.Tags
	}

	userID := getContextString(c, "user_id", "anonymous")
	workflow.UpdatedBy = userID

	var updated *Workflow
	err = h.withService(func(svc *WorkflowService) error {
		var err error
		updated, err = svc.UpdateWorkflow(requestContext(c), workflow)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, updated)
}

// DeleteWorkflow deletes a workflow.
func (h *Handler) DeleteWorkflow(c echo.Context) error {
	id := c.Param("id")

	err := h.withService(func(svc *WorkflowService) error {
		return svc.DeleteWorkflow(requestContext(c), id)
	})
	if err != nil {
		if err == ErrWorkflowNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

// ListWorkflows lists workflows with pagination.
func (h *Handler) ListWorkflows(c echo.Context) error {
	tenantID := getContextString(c, "tenant_id", "default")

	opts := &ListOptions{
		Offset:  0,
		Limit:   20,
		Filters: make(map[string]string),
	}

	if offset := c.QueryParam("offset"); offset != "" {
		opts.Offset, _ = strconv.Atoi(offset)
	}
	if limit := c.QueryParam("limit"); limit != "" {
		opts.Limit, _ = strconv.Atoi(limit)
	}
	if sort := c.QueryParam("sort"); sort != "" {
		opts.Sort = sort
	}
	if order := c.QueryParam("order"); order != "" {
		opts.Order = order
	}
	if status := c.QueryParam("status"); status != "" {
		opts.Filters["status"] = status
	}
	if name := c.QueryParam("name"); name != "" {
		opts.Filters["name"] = name
	}

	var (
		workflows []*Workflow
		total     int
	)
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		workflows, total, err = svc.ListWorkflows(requestContext(c), tenantID, opts)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"workflows": workflows,
		"total":     total,
		"offset":    opts.Offset,
		"limit":     opts.Limit,
	})
}

// EnableWorkflow enables a workflow.
func (h *Handler) EnableWorkflow(c echo.Context) error {
	id := c.Param("id")

	err := h.withService(func(svc *WorkflowService) error {
		return svc.EnableWorkflow(requestContext(c), id)
	})
	if err != nil {
		if err == ErrWorkflowNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "enabled"})
}

// DisableWorkflow disables a workflow.
func (h *Handler) DisableWorkflow(c echo.Context) error {
	id := c.Param("id")

	err := h.withService(func(svc *WorkflowService) error {
		return svc.DisableWorkflow(requestContext(c), id)
	})
	if err != nil {
		if err == ErrWorkflowNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

// ValidateWorkflow validates a workflow definition.
func (h *Handler) ValidateWorkflow(c echo.Context) error {
	var workflow Workflow
	if err := c.Bind(&workflow); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	err := h.withService(func(svc *WorkflowService) error {
		return svc.ValidateWorkflow(requestContext(c), &workflow)
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
	})
}

// ExecuteWorkflowRequest represents an execute workflow request.
type ExecuteWorkflowRequest struct {
	TriggerData map[string]interface{} `json:"trigger_data,omitempty"`
}

// ExecuteWorkflow manually executes a workflow.
func (h *Handler) ExecuteWorkflow(c echo.Context) error {
	id := c.Param("id")

	var req ExecuteWorkflowRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var execution *Execution
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		execution, err = svc.ExecuteWorkflow(requestContext(c), id, req.TriggerData)
		return err
	})
	if err != nil {
		if err == ErrWorkflowNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
		}
		if err == ErrMaxExecutionsReached {
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "max concurrent executions reached"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusAccepted, execution)
}

// GetExecution retrieves an execution by ID.
func (h *Handler) GetExecution(c echo.Context) error {
	executionID := c.Param("executionId")

	var execution *Execution
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		execution, err = svc.GetExecution(requestContext(c), executionID)
		return err
	})
	if err != nil {
		if err == ErrExecutionNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "execution not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, execution)
}

// ListExecutions lists executions for a workflow.
func (h *Handler) ListExecutions(c echo.Context) error {
	workflowID := c.Param("id")

	opts := &ListOptions{
		Offset: 0,
		Limit:  20,
	}

	if offset := c.QueryParam("offset"); offset != "" {
		opts.Offset, _ = strconv.Atoi(offset)
	}
	if limit := c.QueryParam("limit"); limit != "" {
		opts.Limit, _ = strconv.Atoi(limit)
	}

	var (
		executions []*Execution
		total      int
	)
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		executions, total, err = svc.ListExecutions(requestContext(c), workflowID, opts)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"executions": executions,
		"total":      total,
		"offset":     opts.Offset,
		"limit":      opts.Limit,
	})
}

// CancelExecution cancels a running execution.
func (h *Handler) CancelExecution(c echo.Context) error {
	executionID := c.Param("executionId")

	err := h.withService(func(svc *WorkflowService) error {
		return svc.CancelExecution(requestContext(c), executionID)
	})
	if err != nil {
		if err == ErrExecutionNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "execution not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

// RetryExecution retries a failed execution.
func (h *Handler) RetryExecution(c echo.Context) error {
	executionID := c.Param("executionId")

	var execution *Execution
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		execution, err = svc.RetryExecution(requestContext(c), executionID)
		return err
	})
	if err != nil {
		if err == ErrExecutionNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "execution not found"})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusAccepted, execution)
}

// ResumeExecution resumes a paused execution from its current checkpoint.
func (h *Handler) ResumeExecution(c echo.Context) error {
	executionID := c.Param("executionId")

	var req ResumeExecutionRequest
	if err := c.Bind(&req); err != nil && err != io.EOF {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	var execution *Execution
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		execution, err = svc.ResumeExecution(requestContext(c), executionID, ExecutionResumeInput{
			Decision: req.Decision,
			Payload:  req.Payload,
		})
		return err
	})
	if err != nil {
		if err == ErrExecutionNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "execution not found"})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, execution)
}

// GetExecutionLogs retrieves logs for an execution.
func (h *Handler) GetExecutionLogs(c echo.Context) error {
	executionID := c.Param("executionId")

	opts := &ListOptions{
		Offset: 0,
		Limit:  100,
	}

	if offset := c.QueryParam("offset"); offset != "" {
		opts.Offset, _ = strconv.Atoi(offset)
	}
	if limit := c.QueryParam("limit"); limit != "" {
		opts.Limit, _ = strconv.Atoi(limit)
	}

	var (
		logs  []*ExecutionLog
		total int
	)
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		logs, total, err = svc.GetExecutionLogs(requestContext(c), executionID, opts)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"logs":   logs,
		"total":  total,
		"offset": opts.Offset,
		"limit":  opts.Limit,
	})
}

// GetStats returns workflow statistics.
func (h *Handler) GetStats(c echo.Context) error {
	tenantID := getContextString(c, "tenant_id", "default")

	var stats *Stats
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		stats, err = svc.GetStats(requestContext(c), tenantID)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, stats)
}

// HandleWebhook handles incoming webhook requests.
func (h *Handler) HandleWebhook(c echo.Context) error {
	path := c.Param("*")
	method := c.Request().Method

	// Read headers
	headers := make(map[string]string)
	for key, values := range c.Request().Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Read body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read body"})
	}

	var execution *Execution
	err = h.withService(func(svc *WorkflowService) error {
		var err error
		execution, err = svc.HandleWebhook(c.Request().Context(), path, method, headers, body)
		return err
	})
	if err != nil {
		if err == ErrWorkflowNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "webhook not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusAccepted, map[string]interface{}{
		"execution_id": execution.ID,
		"status":       execution.Status,
	})
}

// WorkflowTemplateResponse represents a workflow template.
type WorkflowTemplateResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Workflow    Workflow `json:"workflow"`
}

// GetTemplates returns available workflow templates.
func (h *Handler) GetTemplates(c echo.Context) error {
	return c.JSON(http.StatusOK, DefaultTemplates())
}

// ImportWorkflow imports a workflow from JSON.
func (h *Handler) ImportWorkflow(c echo.Context) error {
	var workflow Workflow
	if err := json.NewDecoder(c.Request().Body).Decode(&workflow); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
	}

	tenantID := getContextString(c, "tenant_id", "default")
	userID := getContextString(c, "user_id", "anonymous")

	// Reset IDs and metadata
	workflow.ID = ""
	workflow.TenantID = tenantID
	workflow.Status = WorkflowStatusDraft
	workflow.CreatedBy = userID
	workflow.UpdatedBy = userID

	var created *Workflow
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		created, err = svc.CreateWorkflow(requestContext(c), &workflow)
		return err
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, created)
}

// ExportWorkflow exports a workflow as JSON.
func (h *Handler) ExportWorkflow(c echo.Context) error {
	id := c.Param("id")

	var workflow *Workflow
	err := h.withService(func(svc *WorkflowService) error {
		var err error
		workflow, err = svc.GetWorkflow(requestContext(c), id)
		return err
	})
	if err != nil {
		if err == ErrWorkflowNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Remove sensitive/internal fields
	exportWorkflow := *workflow
	exportWorkflow.TenantID = ""
	exportWorkflow.CreatedBy = ""
	exportWorkflow.UpdatedBy = ""

	c.Response().Header().Set("Content-Disposition", "attachment; filename=workflow-"+id+".json")
	return c.JSON(http.StatusOK, exportWorkflow)
}
