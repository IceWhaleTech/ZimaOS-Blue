package tools

import (
	"context"
	"fmt"
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

const defaultApprovalTimeout = 2 * time.Minute

// ApprovalRequest is the data sent to the frontend via SSE.
type ApprovalRequest struct {
	ID        string `json:"id"`
	Type      string `json:"type"`                // "command" or "directory"
	Command   string `json:"command,omitempty"`
	Directory string `json:"directory,omitempty"`  // for type=directory
	Workdir   string `json:"workdir,omitempty"`
	Host      string `json:"host,omitempty"`
	Security  string `json:"security,omitempty"`
	UserID    string `json:"user_id"`
	ExpiresAt int64  `json:"expires_at"` // Unix ms
}

type pendingApproval struct {
	ch      chan ApprovalDecision
	created time.Time
}

// ApprovalManager handles the exec approval flow via SSE.
type ApprovalManager struct {
	broker  *sse.Broker
	mu      sync.Mutex
	pending map[string]*pendingApproval
	timeout time.Duration
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

// RequestApproval sends an approval request to the user via SSE and blocks
// until the user responds or the timeout expires.
func (m *ApprovalManager) RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	req.ExpiresAt = timeutil.NowMilli() + m.timeout.Milliseconds()

	ch := make(chan ApprovalDecision, 1)
	m.mu.Lock()
	m.pending[req.ID] = &pendingApproval{ch: ch, created: timeutil.NowTime()}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.pending, req.ID)
		m.mu.Unlock()
	}()

	// Publish SSE event to the user.
	userID := req.UserID
	if userID == "" {
		userID = "default"
	}
	m.broker.Publish(userID, "exec:approval-request", req)

	// Wait for response or timeout.
	timer := time.NewTimer(m.timeout)
	defer timer.Stop()

	select {
	case decision := <-ch:
		return decision, nil
	case <-timer.C:
		return ApprovalDeny, fmt.Errorf("approval timed out after %s", m.timeout)
	case <-ctx.Done():
		return ApprovalDeny, ctx.Err()
	}
}

// ResolveApproval is called by the REST endpoint when the user responds.
// Returns false if the approval ID is not found (expired or already resolved).
func (m *ApprovalManager) ResolveApproval(id string, decision ApprovalDecision) bool {
	m.mu.Lock()
	p, ok := m.pending[id]
	if ok {
		delete(m.pending, id)
	}
	m.mu.Unlock()

	if !ok {
		return false
	}

	select {
	case p.ch <- decision:
	default:
	}
	return true
}

// PendingCount returns the number of pending approval requests.
func (m *ApprovalManager) PendingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}
