package api

import (
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ApprovalConfig holds the tool-call approval policy.
type ApprovalConfig struct {
	Enabled       bool              `json:"enabled"`
	DefaultPolicy string            `json:"default_policy"` // auto, ask, deny
	ToolPolicies  map[string]string `json:"tool_policies"`
}

// PendingRequest is a tool call waiting for user approval.
type PendingRequest struct {
	ID         string         `json:"id"`
	ToolName   string         `json:"tool_name"`
	ToolCallID string         `json:"tool_call_id"`
	Arguments  map[string]any `json:"arguments"`
	CreatedAt  string         `json:"created_at"`
	UserID     string         `json:"-"`
}

// ApprovalHandler serves the /api/v1/approval endpoints.
type ApprovalHandler struct {
	mu      sync.RWMutex
	config  ApprovalConfig
	pending map[string]*PendingRequest // id → request
	broker  *sse.Broker
}

// NewApprovalHandler creates a new handler with sensible defaults.
func NewApprovalHandler(broker *sse.Broker) *ApprovalHandler {
	return &ApprovalHandler{
		config: ApprovalConfig{
			Enabled:       true,
			DefaultPolicy: "auto",
			ToolPolicies:  map[string]string{},
		},
		pending: make(map[string]*PendingRequest),
		broker:  broker,
	}
}

// RegisterRoutes registers approval endpoints on the given group.
func (h *ApprovalHandler) RegisterRoutes(g *echo.Group) {
	ag := g.Group("/approval")
	ag.GET("/config", h.GetConfig)
	ag.PUT("/config", h.UpdateConfig)
	ag.GET("/pending", h.ListPending)
	ag.POST("/resolve", h.Resolve)
}

// GetConfig returns the current approval configuration.
func (h *ApprovalHandler) GetConfig(c echo.Context) error {
	h.mu.RLock()
	cfg := h.config
	h.mu.RUnlock()
	return c.JSON(http.StatusOK, cfg)
}

// UpdateConfig replaces the approval configuration.
func (h *ApprovalHandler) UpdateConfig(c echo.Context) error {
	var cfg ApprovalConfig
	if err := c.Bind(&cfg); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if cfg.ToolPolicies == nil {
		cfg.ToolPolicies = map[string]string{}
	}
	h.mu.Lock()
	h.config = cfg
	h.mu.Unlock()
	return c.JSON(http.StatusOK, cfg)
}

// ListPending returns all pending approval requests.
func (h *ApprovalHandler) ListPending(c echo.Context) error {
	h.mu.RLock()
	out := make([]*PendingRequest, 0, len(h.pending))
	for _, r := range h.pending {
		out = append(out, r)
	}
	h.mu.RUnlock()
	return c.JSON(http.StatusOK, out)
}

// resolveRequest is the JSON body for POST /approval/resolve.
type resolveRequest struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"` // approve or deny
}

// Resolve approves or denies a pending request.
func (h *ApprovalHandler) Resolve(c echo.Context) error {
	var req resolveRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	h.mu.Lock()
	pr, ok := h.pending[req.RequestID]
	if ok {
		delete(h.pending, req.RequestID)
	}
	h.mu.Unlock()
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
	}
	// Notify via SSE so the backend tool executor can pick up the decision
	if h.broker != nil {
		h.broker.Publish(pr.UserID, "tool_approval_resolved", map[string]string{
			"request_id": req.RequestID,
			"decision":   req.Decision,
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": req.Decision})
}

// Enqueue adds a new pending request and pushes an SSE event.
// Called by the tool executor when a tool call needs approval.
func (h *ApprovalHandler) Enqueue(userID string, req *PendingRequest) {
	req.UserID = userID
	if req.CreatedAt == "" {
		req.CreatedAt = timeutil.NowTime().UTC().Format("2006-01-02T15:04:05Z")
	}
	h.mu.Lock()
	h.pending[req.ID] = req
	h.mu.Unlock()
	if h.broker != nil {
		h.broker.Publish(userID, "tool_approval_request", req)
	}
}
