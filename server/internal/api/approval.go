package api

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
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
	ResolveApproval(id string, decision string, bindingHash string) ExecApprovalResolveResult
}

// ExecApprovalResolveResult describes the outcome of resolving an exec approval.
type ExecApprovalResolveResult struct {
	Resolved        bool
	BindingMismatch bool
}

// PendingRequest is a tool call waiting for user approval.
type PendingRequest struct {
	ID           string         `json:"id"`
	RunID        string         `json:"run_id,omitempty"`
	StepIndex    int            `json:"step_index,omitempty"`
	ToolName     string         `json:"tool_name"`
	ToolCallID   string         `json:"tool_call_id"`
	Arguments    map[string]any `json:"arguments"`
	SessionID    string         `json:"session_id,omitempty"`
	RouteKind    string         `json:"route_kind,omitempty"`
	Provider     string         `json:"provider,omitempty"`
	ProviderID   string         `json:"provider_id,omitempty"`
	Model        string         `json:"model,omitempty"`
	AgentID      string         `json:"agent_id,omitempty"`
	PolicySource string         `json:"policy_source,omitempty"`
	RiskLevel    string         `json:"risk_level,omitempty"`
	BindingHash  string         `json:"binding_hash,omitempty"`
	CreatedAt    string         `json:"created_at"`
	ExpiresAt    int64          `json:"expires_at,omitempty"`
	UserID       string         `json:"-"`
}

// ApprovalHandler serves the /api/v1/approval endpoints.
type ApprovalHandler struct {
	mu           sync.RWMutex
	config       ApprovalConfig
	pending      map[string]*PendingRequest // id → request
	waiters      map[string]chan string
	broker       *sse.Broker
	execResolver ExecApprovalResolver // optional, for exec tool approvals
	timeout      time.Duration
	observer     tools.RuntimeEventObserver
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
		waiters: make(map[string]chan string),
		broker:  broker,
		timeout: 2 * time.Minute,
	}
}

// SetExecResolver wires the exec approval manager so that
// POST /approval/resolve can also resolve exec tool approvals.
func (h *ApprovalHandler) SetExecResolver(r ExecApprovalResolver) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.execResolver = r
}

// SetObserver wires lifecycle notifications for tool approval requests.
func (h *ApprovalHandler) SetObserver(observer tools.RuntimeEventObserver) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.observer = observer
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

// GetPendingByRun returns the first pending tool approval request for a harness run.
func (h *ApprovalHandler) GetPendingByRun(runID string) *PendingRequest {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, req := range h.pending {
		if strings.TrimSpace(req.RunID) == runID {
			out := *req
			return &out
		}
	}
	return nil
}

// resolveRequest is the JSON body for POST /approval/resolve.
type resolveRequest struct {
	RequestID   string `json:"request_id"`
	Decision    string `json:"decision"`     // approve or deny
	BindingHash string `json:"binding_hash"` // binds UI resolution to the original request
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
	waiter := h.waiters[req.RequestID]
	if ok || waiter != nil {
		delete(h.waiters, req.RequestID)
	}
	resolver := h.execResolver
	h.mu.Unlock()

	if ok {
		expectedBinding := strings.TrimSpace(pr.BindingHash)
		if expectedBinding != "" && strings.TrimSpace(req.BindingHash) != expectedBinding {
			h.mu.Lock()
			h.pending[req.RequestID] = pr
			if waiter != nil {
				h.waiters[req.RequestID] = waiter
			}
			h.mu.Unlock()
			return c.JSON(http.StatusConflict, map[string]string{"error": "binding hash mismatch"})
		}
		// Resolve a generic tool approval.
		if waiter != nil {
			select {
			case waiter <- req.Decision:
			default:
			}
		}
		if h.broker != nil {
			h.broker.Publish(pr.UserID, "tool_approval_resolved", map[string]string{
				"request_id":   req.RequestID,
				"decision":     req.Decision,
				"binding_hash": expectedBinding,
			})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": req.Decision})
	}

	// Try exec approval resolver as fallback.
	if resolver != nil {
		result := resolver.ResolveApproval(req.RequestID, req.Decision, req.BindingHash)
		if result.Resolved {
			return c.JSON(http.StatusOK, map[string]string{"status": req.Decision})
		}
		if result.BindingMismatch {
			return c.JSON(http.StatusConflict, map[string]string{"error": "binding hash mismatch"})
		}
	}

	return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
}

// Enqueue adds a new pending request and pushes an SSE event.
// Called by the tool executor when a tool call needs approval.
func (h *ApprovalHandler) Enqueue(userID string, req *PendingRequest) {
	if req == nil {
		return
	}
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	req.UserID = userID
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.CreatedAt == "" {
		req.CreatedAt = timeutil.NowTime().UTC().Format("2006-01-02T15:04:05Z")
	}
	if req.ExpiresAt == 0 {
		req.ExpiresAt = timeutil.NowMilli() + h.timeout.Milliseconds()
	}
	h.mu.Lock()
	h.pending[req.ID] = req
	h.mu.Unlock()
	if h.broker != nil {
		h.broker.Publish(userID, "tool_approval_request", req)
	}
}

// AuthorizeToolCall enforces generic tool approval policies at runtime.
func (h *ApprovalHandler) AuthorizeToolCall(ctx context.Context, req tools.ToolApprovalRequest) (tools.ToolApprovalDecision, error) {
	mode, source := h.policyMode(req.ToolName)
	riskLevel := normalizeRiskLevel(req.RiskLevel)
	if overrideMode, overrideSource, overrideRisk, ok := highRiskToolApprovalOverride(ctx, req, mode); ok {
		mode = overrideMode
		source = overrideSource
		riskLevel = maxApprovalRiskLevel(riskLevel, overrideRisk)
	}
	approval := tools.ToolApprovalEnvelope{
		Mode:         mode,
		PolicySource: nonEmpty(strings.TrimSpace(req.PolicySource), source),
		RiskLevel:    riskLevel,
		BindingHash:  strings.TrimSpace(req.BindingHash),
	}
	decision := tools.ToolApprovalDecision{
		Allowed:  true,
		Approval: approval,
	}
	switch mode {
	case "deny":
		decision.Allowed = false
		decision.Approval.Required = true
		decision.Approval.Reason = fmt.Sprintf("tool %q blocked by approval policy", strings.TrimSpace(req.ToolName))
		return decision, nil
	case "ask":
		pending := &PendingRequest{
			ID:           uuid.NewString(),
			RunID:        tools.GetRunID(ctx),
			StepIndex:    tools.GetRunStep(ctx),
			ToolName:     strings.TrimSpace(req.ToolName),
			ToolCallID:   strings.TrimSpace(req.ToolCallID),
			Arguments:    cloneApprovalArgs(req.Arguments),
			SessionID:    strings.TrimSpace(req.SessionID),
			RouteKind:    strings.TrimSpace(string(req.RouteKind)),
			Provider:     strings.TrimSpace(req.Provider),
			ProviderID:   strings.TrimSpace(req.ProviderID),
			Model:        strings.TrimSpace(req.Model),
			AgentID:      strings.TrimSpace(req.AgentID),
			PolicySource: decision.Approval.PolicySource,
			RiskLevel:    decision.Approval.RiskLevel,
			BindingHash:  decision.Approval.BindingHash,
		}
		resolution, waitErr := h.waitForApproval(ctx, nonEmpty(strings.TrimSpace(req.UserID), tools.GetUserID(ctx)), pending)
		decision.Approval.Required = true
		decision.Approval.ID = pending.ID
		decision.Approval.ExpiresAt = pending.ExpiresAt
		if waitErr != nil {
			decision.Allowed = false
			decision.Approval.Reason = waitErr.Error()
			return decision, waitErr
		}
		allowed := resolution == "approve" || resolution == "allow" || resolution == "allow-once" || resolution == "allow-always"
		decision.Allowed = allowed
		if !allowed {
			decision.Approval.Reason = "tool approval denied"
		}
		return decision, nil
	default:
		return decision, nil
	}
}

func (h *ApprovalHandler) waitForApproval(ctx context.Context, userID string, req *PendingRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("approval request is required")
	}
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	req.UserID = nonEmpty(strings.TrimSpace(userID), "default")
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.RunID == "" {
		req.RunID = tools.GetRunID(ctx)
	}
	if req.StepIndex == 0 {
		req.StepIndex = tools.GetRunStep(ctx)
	}
	req.CreatedAt = timeutil.NowTime().UTC().Format("2006-01-02T15:04:05Z")
	req.ExpiresAt = timeutil.NowMilli() + h.timeout.Milliseconds()

	ch := make(chan string, 1)
	h.mu.Lock()
	h.pending[req.ID] = req
	h.waiters[req.ID] = ch
	observer := h.observer
	h.mu.Unlock()

	if h.broker != nil {
		h.broker.Publish(req.UserID, "tool_approval_request", req)
	}
	if observer != nil {
		observer.OnApprovalRequested(toolApprovalRuntimeEvent(req))
	}

	timer := time.NewTimer(h.timeout)
	defer timer.Stop()
	defer func() {
		h.mu.Lock()
		delete(h.pending, req.ID)
		delete(h.waiters, req.ID)
		h.mu.Unlock()
	}()

	select {
	case resolution := <-ch:
		if observer != nil {
			event := toolApprovalRuntimeEvent(req)
			event.Decision = strings.ToLower(strings.TrimSpace(resolution))
			observer.OnApprovalResolved(event)
		}
		return strings.ToLower(strings.TrimSpace(resolution)), nil
	case <-timer.C:
		err := fmt.Errorf("tool approval timed out after %s", h.timeout)
		if observer != nil {
			event := toolApprovalRuntimeEvent(req)
			event.Decision = "deny"
			event.Error = err.Error()
			observer.OnApprovalResolved(event)
		}
		return "deny", err
	case <-ctx.Done():
		err := ctx.Err()
		if observer != nil {
			event := toolApprovalRuntimeEvent(req)
			event.Decision = "deny"
			event.Error = err.Error()
			observer.OnApprovalResolved(event)
		}
		return "deny", err
	}
}

func toolApprovalRuntimeEvent(req *PendingRequest) tools.ApprovalRuntimeEvent {
	if req == nil {
		return tools.ApprovalRuntimeEvent{}
	}
	return tools.ApprovalRuntimeEvent{
		RunID:        req.RunID,
		StepIndex:    req.StepIndex,
		Kind:         "tool",
		ID:           req.ID,
		ToolName:     req.ToolName,
		ToolCallID:   req.ToolCallID,
		SessionID:    req.SessionID,
		UserID:       req.UserID,
		PolicySource: req.PolicySource,
		RiskLevel:    req.RiskLevel,
		BindingHash:  req.BindingHash,
		ExpiresAt:    req.ExpiresAt,
	}
}

func (h *ApprovalHandler) policyMode(toolName string) (mode string, source string) {
	h.mu.RLock()
	cfg := h.config
	h.mu.RUnlock()

	if !cfg.Enabled {
		return "auto", "approval.disabled"
	}
	toolName = strings.TrimSpace(toolName)
	if toolName != "" {
		if rawMode, ok := cfg.ToolPolicies[toolName]; ok {
			return normalizeApprovalMode(rawMode), "approval.tool_policies." + toolName
		}
	}
	return normalizeApprovalMode(cfg.DefaultPolicy), "approval.default_policy"
}

func normalizeApprovalMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ask":
		return "ask"
	case "deny":
		return "deny"
	default:
		return "auto"
	}
}

func normalizeRiskLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

func highRiskToolApprovalOverride(ctx context.Context, req tools.ToolApprovalRequest, baseMode string) (mode string, source string, risk string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(baseMode)) {
	case "ask", "deny":
		return "", "", "", false
	}
	if !strings.EqualFold(strings.TrimSpace(req.ToolName), "file_delete") {
		return "", "", "", false
	}
	if !shouldRequireFileDeleteApproval(ctx, req.Arguments) {
		return "", "", "", false
	}
	return "deny", "approval.tool_risk.file_delete", "critical", true
}

func shouldRequireFileDeleteApproval(ctx context.Context, args map[string]interface{}) bool {
	path := approvalStringArg(args, "path")
	if isRootLikeDeletePath(ctx, path) {
		return true
	}
	if approvalBoolArg(args, "recursive") {
		return true
	}
	return looksLikeDirectoryDeletePath(path)
}

func approvalStringArg(args map[string]interface{}, key string) string {
	if len(args) == 0 {
		return ""
	}
	raw, ok := args[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func approvalBoolArg(args map[string]interface{}, key string) bool {
	if len(args) == 0 {
		return false
	}
	raw, ok := args[key]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "t", "true", "yes", "y", "on":
			return true
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	}
	return false
}

func isRootLikeDeletePath(ctx context.Context, raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	switch trimmed {
	case ".", "./", "/", "\\":
		return true
	}

	normalized := filepath.Clean(strings.ReplaceAll(trimmed, "\\", "/"))
	switch normalized {
	case ".", "/", "/workspace":
		return true
	}

	roots, aliases := tools.GetFSScope(ctx)
	for _, root := range roots {
		cleanRoot := filepath.Clean(root)
		if cleanRoot != "" && normalized == cleanRoot {
			return true
		}
	}
	for alias, target := range aliases {
		cleanAlias := strings.ToLower(strings.TrimSpace(alias))
		if cleanAlias == "" {
			continue
		}
		if normalized == cleanAlias || normalized == "@"+cleanAlias || normalized == cleanAlias+":" || normalized == "/"+cleanAlias {
			return true
		}
		cleanTarget := filepath.Clean(target)
		if cleanTarget != "" && normalized == cleanTarget {
			return true
		}
	}
	return false
}

func looksLikeDirectoryDeletePath(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.HasSuffix(trimmed, "/") || strings.HasSuffix(trimmed, "\\") {
		return true
	}
	base := filepath.Base(trimmed)
	return base == "." || base == ".."
}

func maxApprovalRiskLevel(a, b string) string {
	if approvalRiskRank(b) > approvalRiskRank(a) {
		return normalizeRiskLevel(b)
	}
	return normalizeRiskLevel(a)
}

func approvalRiskRank(raw string) int {
	switch normalizeRiskLevel(raw) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func cloneApprovalArgs(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
