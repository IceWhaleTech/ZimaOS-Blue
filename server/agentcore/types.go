package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type ProtocolKind string

const (
	ProtocolACP ProtocolKind = "acp"
	ProtocolA2A ProtocolKind = "a2a"
)

func (k ProtocolKind) String() string {
	return string(k)
}

var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol kind")
	ErrProfileNotFound     = errors.New("agent profile not found")
	ErrSessionNotFound     = errors.New("agent session not found")
	ErrRunNotFound         = errors.New("agent run not found")
)

func ParseProtocolKind(raw string) (ProtocolKind, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ProtocolACP):
		return ProtocolACP, nil
	case string(ProtocolA2A):
		return ProtocolA2A, nil
	default:
		return "", ErrUnsupportedProtocol
	}
}

type SessionStatus string

const (
	SessionStatusCreating   SessionStatus = "creating"
	SessionStatusIdle       SessionStatus = "idle"
	SessionStatusRunning    SessionStatus = "running"
	SessionStatusCancelling SessionStatus = "cancelling"
	SessionStatusClosed     SessionStatus = "closed"
	SessionStatusError      SessionStatus = "error"
)

type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

type AgentProfile struct {
	ID                   string                 `json:"id"`
	Protocol             ProtocolKind           `json:"protocol"`
	Name                 string                 `json:"name"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Builtin              bool                   `json:"builtin,omitempty"`
	TemplateOnly         bool                   `json:"template_only,omitempty"`
	Command              []string               `json:"command,omitempty"`
	Env                  map[string]string      `json:"env,omitempty"`
	CWD                  string                 `json:"cwd,omitempty"`
	CardURL              string                 `json:"card_url,omitempty"`
	EndpointURL          string                 `json:"endpoint_url,omitempty"`
	Headers              map[string]string      `json:"headers,omitempty"`
	CredentialProviderID string                 `json:"credential_provider_id,omitempty"`
	AuthMethodID         string                 `json:"auth_method_id,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	HealthStatus         string                 `json:"health_status,omitempty"`
	HealthMessage        string                 `json:"health_message,omitempty"`
	LastVerifiedAt       time.Time              `json:"last_verified_at,omitempty"`
	LastHealthAt         time.Time              `json:"last_health_at,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

type ExternalSession struct {
	ID              string                 `json:"id"`
	ProfileID       string                 `json:"profile_id"`
	Protocol        ProtocolKind           `json:"protocol"`
	UserID          string                 `json:"user_id,omitempty"`
	Name            string                 `json:"name"`
	CWD             string                 `json:"cwd,omitempty"`
	Status          SessionStatus          `json:"status"`
	RemoteSessionID string                 `json:"remote_session_id,omitempty"`
	LastError       string                 `json:"last_error,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	ClosedAt        time.Time              `json:"closed_at,omitempty"`
}

type ExternalRun struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id"`
	Status      RunStatus              `json:"status"`
	Prompt      string                 `json:"prompt"`
	RemoteRunID string                 `json:"remote_run_id,omitempty"`
	StopReason  string                 `json:"stop_reason,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	StartedAt   time.Time              `json:"started_at,omitempty"`
	CompletedAt time.Time              `json:"completed_at,omitempty"`
}

type RunEvent struct {
	ID        int64           `json:"id"`
	SessionID string          `json:"session_id"`
	RunID     string          `json:"run_id"`
	Index     int             `json:"index"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

type SessionHistoryItem struct {
	ID        int64                  `json:"id"`
	RunID     string                 `json:"run_id"`
	Type      string                 `json:"type"`
	Role      string                 `json:"role,omitempty"`
	Content   string                 `json:"content,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

const (
	ProfileVerifyMessageCodeProfileVerified = "profile_verified"
	ProfileHealthMessageCodeRuntimeHealthy  = "runtime_healthy"
)

type ProfileVerifyResult struct {
	OK           bool                   `json:"ok"`
	MessageCode  string                 `json:"message_code,omitempty"`
	Message      string                 `json:"message,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

type ProfileHealthResult struct {
	Healthy     bool                   `json:"healthy"`
	MessageCode string                 `json:"message_code,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

type EnsureSessionRequest struct {
	Profile AgentProfile
	Session ExternalSession
}

type EnsureSessionResult struct {
	RemoteSessionID string                 `json:"remote_session_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type SubmitRunRequest struct {
	Profile AgentProfile
	Session ExternalSession
	Run     ExternalRun
	Prompt  string
}

type SubmitRunResult struct {
	RemoteRunID string `json:"remote_run_id,omitempty"`
}

type StreamRunRequest struct {
	Profile AgentProfile
	Session ExternalSession
	Run     ExternalRun
}

type RuntimeEvent struct {
	Type    string                 `json:"type"`
	Role    string                 `json:"role,omitempty"`
	Text    string                 `json:"text,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Status  string                 `json:"status,omitempty"`
	Remote  string                 `json:"remote,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type RuntimeStream interface {
	Events() <-chan RuntimeEvent
	Wait() error
}

type ProtocolAdapter interface {
	VerifyProfile(ctx context.Context, profile AgentProfile) (*ProfileVerifyResult, error)
	EnsureSession(ctx context.Context, req EnsureSessionRequest) (*EnsureSessionResult, error)
	SubmitRun(ctx context.Context, req SubmitRunRequest) (*SubmitRunResult, error)
	StreamRun(ctx context.Context, req StreamRunRequest) (RuntimeStream, error)
	CancelRun(ctx context.Context, session ExternalSession, run ExternalRun) error
	CloseSession(ctx context.Context, profile AgentProfile, session ExternalSession) error
	Health(ctx context.Context, profile AgentProfile) (*ProfileHealthResult, error)
}

type ClientAuthority interface {
	ReadTextFile(ctx context.Context, path string, startLine, endLine int) (map[string]interface{}, error)
	WriteTextFile(ctx context.Context, path, content string, appendMode bool) (map[string]interface{}, error)
	CreateTerminal(ctx context.Context, command, workdir string, env map[string]string) (map[string]interface{}, error)
	TerminalOutput(ctx context.Context, terminalID string) (map[string]interface{}, error)
	TerminalWaitForExit(ctx context.Context, terminalID string, timeout time.Duration) (map[string]interface{}, error)
	TerminalKill(ctx context.Context, terminalID string) (map[string]interface{}, error)
	TerminalRelease(ctx context.Context, terminalID string) (map[string]interface{}, error)
}
