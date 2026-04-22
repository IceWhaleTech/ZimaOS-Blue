package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ApprovalDecision represents the user's response to an exec approval request.
type ApprovalDecision string

const (
	ApprovalAllowOnce   ApprovalDecision = "allow-once"
	ApprovalAllowAlways ApprovalDecision = "allow-always"
	ApprovalDeny        ApprovalDecision = "deny"
)

const defaultApprovalTimeout = 5 * time.Minute

// ApprovalRequest is the data sent to the frontend via SSE.
type ApprovalRequest struct {
	ID              string                 `json:"id"`
	RunID           string                 `json:"run_id,omitempty"`
	StepIndex       int                    `json:"step_index,omitempty"`
	Type            string                 `json:"type"` // "command" or "directory"
	Command         string                 `json:"command,omitempty"`
	CommandDigest   string                 `json:"command_digest,omitempty"`
	Directory       string                 `json:"directory,omitempty"` // for type=directory
	Workdir         string                 `json:"workdir,omitempty"`
	Host            string                 `json:"host,omitempty"`
	Security        string                 `json:"security,omitempty"`
	UserID          string                 `json:"user_id"`
	SessionID       string                 `json:"session_id,omitempty"`
	PolicySource    string                 `json:"policy_source,omitempty"`
	RiskLevel       string                 `json:"risk_level,omitempty"`
	BindingHash     string                 `json:"binding_hash,omitempty"`
	ReferencedPaths []string               `json:"referenced_paths,omitempty"`
	EnvKeys         []string               `json:"env_keys,omitempty"`
	PathSnapshots   []ApprovalPathSnapshot `json:"path_snapshots,omitempty"`
	Purpose         string                 `json:"purpose,omitempty"`
	RiskSummary     string                 `json:"risk_summary,omitempty"`
	ScopeSummary    string                 `json:"scope_summary,omitempty"`
	ExpectedEffects string                 `json:"expected_effects,omitempty"`
	AffectedTargets []string               `json:"affected_targets,omitempty"`
	ExpiresAt       int64                  `json:"expires_at"` // Unix ms
}

// ApprovalPathSnapshot binds an approval to the state of a target path.
type ApprovalPathSnapshot struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Type    string `json:"type,omitempty"`
	ModTime int64  `json:"mtime,omitempty"`
	Size    int64  `json:"size,omitempty"`
}

type pendingApproval struct {
	ch      chan ApprovalDecision
	request ApprovalRequest
	created time.Time
}

// ApprovalResolveStatus describes the outcome of resolving a pending exec approval.
type ApprovalResolveStatus int

const (
	ApprovalResolveNotFound ApprovalResolveStatus = iota
	ApprovalResolveBindingMismatch
	ApprovalResolveSuccess
)

// ApprovalManager handles the exec approval flow via SSE.
type ApprovalManager struct {
	broker   *sse.Broker
	mu       sync.Mutex
	pending  map[string]*pendingApproval
	timeout  time.Duration
	observer RuntimeEventObserver
}

// NewApprovalManager creates a new approval manager.
func NewApprovalManager(broker *sse.Broker) *ApprovalManager {
	m := &ApprovalManager{
		broker:  broker,
		pending: make(map[string]*pendingApproval),
		timeout: defaultApprovalTimeout,
	}
	return m
}

// SetObserver wires lifecycle notifications for exec approvals.
func (m *ApprovalManager) SetObserver(observer RuntimeEventObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observer = observer
}

// RequestApproval sends an approval request to the user via SSE and blocks
// until the user responds or the timeout expires.
func (m *ApprovalManager) RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if GetAutoConfirm(ctx) {
		return ApprovalAllowOnce, nil
	}
	userID := req.UserID
	if userID == "" {
		userID = "default"
	}
	req.UserID = userID
	if req.SessionID == "" {
		req.SessionID = GetSessionID(ctx)
	}
	if req.RunID == "" {
		req.RunID = GetRunID(ctx)
	}
	if req.StepIndex == 0 {
		req.StepIndex = GetRunStep(ctx)
	}
	req.Command = strings.TrimSpace(req.Command)
	req.Workdir = strings.TrimSpace(req.Workdir)
	req.Directory = strings.TrimSpace(req.Directory)
	req.PolicySource = nonEmpty(req.PolicySource, "exec_runtime")
	req.RiskLevel = normalizeToolRiskLevel(req.RiskLevel)
	req.ReferencedPaths = normalizeApprovalPathList(req.ReferencedPaths)
	req.EnvKeys = normalizeApprovalStringList(req.EnvKeys)
	if req.Command != "" {
		req.CommandDigest = approvalCommandDigest(req.Command)
	}
	presentation := BuildExecApprovalPresentation(req, GetLang(ctx))
	req.Purpose = strings.TrimSpace(presentation.Purpose)
	req.RiskSummary = strings.TrimSpace(presentation.RiskSummary)
	req.ScopeSummary = strings.TrimSpace(presentation.ScopeSummary)
	req.ExpectedEffects = strings.TrimSpace(presentation.ExpectedEffects)
	req.AffectedTargets = append([]string(nil), presentation.AffectedTargets...)
	req.PathSnapshots = approvalSnapshotsForRequest(req)
	if req.BindingHash == "" {
		req.BindingHash = approvalBindingHash(req)
	}
	req.ExpiresAt = timeutil.NowMilli() + m.timeout.Milliseconds()
	if ctx.Err() != nil {
		code := "exec_approval_aborted"
		switch {
		case context.Canceled == ctx.Err():
			code = "exec_approval_cancelled"
		case context.DeadlineExceeded == ctx.Err():
			code = "exec_approval_timeout"
		}
		return ApprovalDeny, newToolRuntimeError(code, ctx.Err().Error(), ctx.Err(), map[string]interface{}{
			"approval_id": req.ID,
			"kind":        strings.TrimSpace(req.Type),
			"command":     strings.TrimSpace(req.Command),
			"directory":   strings.TrimSpace(req.Directory),
			"policy_mode": "ask",
		})
	}
	if GetAutoConfirm(ctx) {
		return ApprovalAllowOnce, nil
	}
	if m.broker == nil || m.broker.ClientCount(userID) == 0 {
		return ApprovalDeny, newToolRuntimeError(
			"exec_approval_delivery_unavailable",
			fmt.Sprintf("approval cannot be delivered: no active SSE client for user %q", userID),
			nil,
			map[string]interface{}{
				"approval_id":      req.ID,
				"user_id":          userID,
				"session_id":       strings.TrimSpace(req.SessionID),
				"kind":             strings.TrimSpace(req.Type),
				"command":          strings.TrimSpace(req.Command),
				"directory":        strings.TrimSpace(req.Directory),
				"delivery_target":  "sse",
				"required_channel": "web",
			},
		)
	}

	ch := make(chan ApprovalDecision, 1)
	m.mu.Lock()
	m.pending[req.ID] = &pendingApproval{
		ch:      ch,
		request: req,
		created: timeutil.NowTime(),
	}
	observer := m.observer
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.pending, req.ID)
		m.mu.Unlock()
	}()

	// Publish SSE event to the user.
	if m.broker != nil {
		m.broker.Publish(userID, "exec:approval-request", req)
	}
	if observer != nil {
		observer.OnApprovalRequested(execApprovalRuntimeEvent(req))
	}

	// Wait for response or timeout.
	timer := time.NewTimer(m.timeout)
	defer timer.Stop()

	select {
	case decision := <-ch:
		if observer != nil {
			event := execApprovalRuntimeEvent(req)
			event.Decision = string(decision)
			observer.OnApprovalResolved(event)
		}
		return decision, nil
	case <-timer.C:
		err := newToolRuntimeError("exec_approval_timeout", fmt.Sprintf("approval timed out after %s", m.timeout), context.DeadlineExceeded, map[string]interface{}{
			"approval_id": req.ID,
			"kind":        strings.TrimSpace(req.Type),
			"command":     strings.TrimSpace(req.Command),
			"directory":   strings.TrimSpace(req.Directory),
			"policy_mode": "ask",
		})
		if observer != nil {
			event := execApprovalRuntimeEvent(req)
			event.Decision = string(ApprovalDeny)
			event.Error = err.Error()
			observer.OnApprovalResolved(event)
		}
		return ApprovalDeny, err
	case <-ctx.Done():
		code := "exec_approval_aborted"
		switch {
		case context.Canceled == ctx.Err():
			code = "exec_approval_cancelled"
		case context.DeadlineExceeded == ctx.Err():
			code = "exec_approval_timeout"
		}
		err := newToolRuntimeError(code, ctx.Err().Error(), ctx.Err(), map[string]interface{}{
			"approval_id": req.ID,
			"kind":        strings.TrimSpace(req.Type),
			"command":     strings.TrimSpace(req.Command),
			"directory":   strings.TrimSpace(req.Directory),
			"policy_mode": "ask",
		})
		if observer != nil {
			event := execApprovalRuntimeEvent(req)
			event.Decision = string(ApprovalDeny)
			event.Error = err.Error()
			observer.OnApprovalResolved(event)
		}
		return ApprovalDeny, err
	}
}

func execApprovalRuntimeEvent(req ApprovalRequest) ApprovalRuntimeEvent {
	return ApprovalRuntimeEvent{
		RunID:        req.RunID,
		StepIndex:    req.StepIndex,
		Kind:         "exec",
		ID:           req.ID,
		ToolName:     "exec",
		Command:      req.Command,
		Directory:    req.Directory,
		SessionID:    req.SessionID,
		UserID:       req.UserID,
		PolicySource: req.PolicySource,
		RiskLevel:    req.RiskLevel,
		BindingHash:  req.BindingHash,
		ExpiresAt:    req.ExpiresAt,
	}
}

// ResolveApproval is called by the REST endpoint when the user responds.
// Returns false if the approval ID is not found (expired or already resolved).
func (m *ApprovalManager) ResolveApproval(id string, decision ApprovalDecision) bool {
	return m.resolveApproval(id, decision, "", false) == ApprovalResolveSuccess
}

// ResolveApprovalWithBinding resolves a pending approval and validates its binding hash.
func (m *ApprovalManager) ResolveApprovalWithBinding(id string, decision ApprovalDecision, bindingHash string) bool {
	return m.resolveApproval(id, decision, bindingHash, true) == ApprovalResolveSuccess
}

// ResolveApprovalWithBindingStatus resolves a pending approval and reports why a resolution failed.
func (m *ApprovalManager) ResolveApprovalWithBindingStatus(id string, decision ApprovalDecision, bindingHash string) ApprovalResolveStatus {
	return m.resolveApproval(id, decision, bindingHash, true)
}

func (m *ApprovalManager) resolveApproval(id string, decision ApprovalDecision, bindingHash string, requireBinding bool) ApprovalResolveStatus {
	m.mu.Lock()
	p, ok := m.pending[id]
	if ok {
		expected := strings.TrimSpace(p.request.BindingHash)
		got := strings.TrimSpace(bindingHash)
		if requireBinding && expected != "" && got != "" && expected != got {
			m.mu.Unlock()
			return ApprovalResolveBindingMismatch
		}
		if requireBinding && expected != "" && got == "" {
			m.mu.Unlock()
			return ApprovalResolveBindingMismatch
		}
		delete(m.pending, id)
	}
	m.mu.Unlock()

	if !ok {
		return ApprovalResolveNotFound
	}

	select {
	case p.ch <- decision:
	default:
	}
	return ApprovalResolveSuccess
}

// PendingCount returns the number of pending approval requests.
func (m *ApprovalManager) PendingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}

// GetPending returns the first pending approval request (if any).
// Used by REST endpoint so frontend can restore approval dialog after refresh.
func (m *ApprovalManager) GetPending(userID string) *ApprovalRequest {
	if userID == "" {
		userID = "default"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.pending {
		if p.request.UserID == userID || p.request.UserID == "default" || userID == "default" {
			req := p.request
			return &req
		}
	}
	return nil
}

// GetPendingBySession returns the first pending approval request for a session.
func (m *ApprovalManager) GetPendingBySession(sessionID string) *ApprovalRequest {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.pending {
		if strings.TrimSpace(p.request.SessionID) == sessionID {
			req := p.request
			return &req
		}
	}
	return nil
}

// GetPendingByRun returns the first pending approval request for a harness run.
func (m *ApprovalManager) GetPendingByRun(runID string) *ApprovalRequest {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.pending {
		if strings.TrimSpace(p.request.RunID) == runID {
			req := p.request
			return &req
		}
	}
	return nil
}

func approvalCommandDigest(command string) string {
	sum := sha256.Sum256([]byte(NormalizeCommand(command)))
	return hex.EncodeToString(sum[:])
}

func approvalBindingHash(req ApprovalRequest) string {
	payload := map[string]interface{}{
		"type":             strings.TrimSpace(req.Type),
		"command_digest":   strings.TrimSpace(req.CommandDigest),
		"directory":        strings.TrimSpace(req.Directory),
		"workdir":          strings.TrimSpace(req.Workdir),
		"host":             strings.TrimSpace(req.Host),
		"security":         strings.TrimSpace(req.Security),
		"session_id":       strings.TrimSpace(req.SessionID),
		"policy_source":    strings.TrimSpace(req.PolicySource),
		"risk_level":       strings.TrimSpace(req.RiskLevel),
		"referenced_paths": normalizeApprovalPathList(req.ReferencedPaths),
		"env_keys":         normalizeApprovalStringList(req.EnvKeys),
		"path_snapshots":   req.PathSnapshots,
	}
	data, _ := jsonMarshalNoEscape(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func approvalSnapshotsForRequest(req ApprovalRequest) []ApprovalPathSnapshot {
	paths := make([]string, 0, len(req.ReferencedPaths)+2)
	if req.Directory != "" {
		paths = append(paths, req.Directory)
	}
	if req.Workdir != "" {
		paths = append(paths, req.Workdir)
	}
	paths = append(paths, req.ReferencedPaths...)
	paths = normalizeApprovalPathList(paths)
	if len(paths) == 0 {
		return nil
	}
	snapshots := make([]ApprovalPathSnapshot, 0, len(paths))
	for _, path := range paths {
		snapshots = append(snapshots, buildApprovalPathSnapshot(path))
	}
	return snapshots
}

func buildApprovalPathSnapshot(path string) ApprovalPathSnapshot {
	snapshot := ApprovalPathSnapshot{Path: strings.TrimSpace(path)}
	if snapshot.Path == "" {
		return snapshot
	}
	clean := snapshot.Path
	if !filepath.IsAbs(clean) {
		if abs, err := filepath.Abs(clean); err == nil {
			clean = abs
		}
	}
	clean = filepath.Clean(clean)
	snapshot.Path = clean
	info, err := os.Stat(clean)
	if err != nil {
		return snapshot
	}
	snapshot.Exists = true
	snapshot.Size = info.Size()
	snapshot.ModTime = info.ModTime().UnixMilli()
	if info.IsDir() {
		snapshot.Type = "dir"
	} else {
		snapshot.Type = "file"
	}
	return snapshot
}

func normalizeApprovalPathList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if !filepath.IsAbs(trimmed) {
			if abs, err := filepath.Abs(trimmed); err == nil {
				trimmed = abs
			}
		}
		trimmed = filepath.Clean(trimmed)
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func normalizeApprovalStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func jsonMarshalNoEscape(v interface{}) ([]byte, error) {
	var (
		builder strings.Builder
		enc     = json.NewEncoder(&builder)
	)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(builder.String())), nil
}
