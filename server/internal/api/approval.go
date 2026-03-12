package api

import (
	"net/http"
	"strings"
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

// ExecApprovalResolver resolves exec-specific approval requests.
// Implemented by a wrapper around tools.ApprovalManager to avoid circular imports.
type ExecApprovalResolver interface {
	ResolveApproval(id string, decision string) bool
}

// PendingRequest is a tool call waiting for user approval.
type PendingRequest struct {
	ID         string         `json:"id"`
	ToolName   string         `json:"tool_name"`
	ToolCallID string         `json:"tool_call_id"`
	Arguments  map[string]any `json:"arguments"`
	SessionID  string         `json:"session_id,omitempty"`
	CreatedAt  string         `json:"created_at"`
	UserID     string         `json:"-"`
}

// ApprovalHandler serves the /api/v1/approval endpoints.
type ApprovalHandler struct {
	mu           sync.RWMutex
	config       ApprovalConfig
	pending      map[string]*PendingRequest // id → request
	broker       *sse.Broker
	execResolver ExecApprovalResolver // optional, for exec tool approvals
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

// SetExecResolver wires the exec approval manager so that
// POST /approval/resolve can also resolve exec tool approvals.
func (h *ApprovalHandler) SetExecResolver(r ExecApprovalResolver) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.execResolver = r
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
	sessionID := strings.TrimSpace(c.QueryParam("session_id"))
	h.mu.RLock()
	out := make([]*PendingRequest, 0, len(h.pending))
	for _, r := range h.pending {
		if sessionID != "" && strings.TrimSpace(r.SessionID) != sessionID {
			continue
		}
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
// Falls back to the exec approval resolver if the request is not found locally.
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
	resolver := h.execResolver
	h.mu.Unlock()

	if ok {
		// Resolve a generic tool approval.
		if h.broker != nil {
			h.broker.Publish(pr.UserID, "tool_approval_resolved", map[string]string{
				"request_id": req.RequestID,
				"decision":   req.Decision,
			})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": req.Decision})
	}

	// Try exec approval resolver as fallback.
	if resolver != nil && resolver.ResolveApproval(req.RequestID, req.Decision) {
		return c.JSON(http.StatusOK, map[string]string{"status": req.Decision})
	}

	return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
}

// Enqueue adds a new pending request and pushes an SSE event.
// Called by the tool executor when a tool call needs approval.
func (h *ApprovalHandler) Enqueue(userID string, req *PendingRequest) {
	req.UserID = userID
	req.SessionID = strings.TrimSpace(req.SessionID)
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
