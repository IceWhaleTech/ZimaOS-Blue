// Package companion provides real-time Agent monitoring capabilities.
// Echo Companion is a monitoring tool that provides developers and administrators
// with a visual interface to observe AI Agent operations across multiple platforms.
package companion

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionClosed      = errors.New("session is closed")
	ErrEventNotFound      = errors.New("event not found")
	ErrAlertNotFound      = errors.New("alert not found")
	ErrInvalidEventType   = errors.New("invalid event type")
	ErrStorageUnavailable = errors.New("storage unavailable")
	ErrSubscriberClosed   = errors.New("subscriber closed")
)

// SessionEventType represents the type of session event.
type SessionEventType string

const (
	EventSessionStart    SessionEventType = "session_start"
	EventSessionEnd      SessionEventType = "session_end"
	EventMessageReceived SessionEventType = "message_received"
	EventMessageSent     SessionEventType = "message_sent"
	EventToolCall        SessionEventType = "tool_call"
	EventLLMRequest      SessionEventType = "llm_request"
	EventSecurityThreat  SessionEventType = "security_threat"
	EventSandboxExec     SessionEventType = "sandbox_exec"
)

// SessionStatus represents the status of a session.
type SessionStatus string

const (
	SessionStatusActive SessionStatus = "active"
	SessionStatusEnded  SessionStatus = "ended"
)

// ThreatLevel represents the severity of a security threat.
type ThreatLevel string

const (
	ThreatLevelNone     ThreatLevel = "none"
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// AlertSeverity represents the severity of an alert.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Platform represents the messaging platform.
type Platform string

const (
	PlatformWhatsApp Platform = "whatsapp"
	PlatformTelegram Platform = "telegram"
	PlatformDiscord  Platform = "discord"
	PlatformSlack    Platform = "slack"
	PlatformMatrix   Platform = "matrix"
	PlatformFeishu   Platform = "feishu"
	PlatformWeb      Platform = "web"
	PlatformAPI      Platform = "api"
)

// SessionEvent represents an event that occurred during an Agent session.
type SessionEvent struct {
	ID        string           `json:"id"`
	SessionID string           `json:"session_id"`
	Timestamp time.Time        `json:"timestamp"`
	EventType SessionEventType `json:"event_type"`
	Platform  Platform         `json:"platform"`
	UserID    string           `json:"user_id"`
	TenantID  string           `json:"tenant_id,omitempty"`

	// Event-specific data (only one will be set based on EventType)
	Message    *MessageEvent    `json:"message,omitempty"`
	ToolCall   *ToolCallEvent   `json:"tool_call,omitempty"`
	LLMRequest *LLMRequestEvent `json:"llm_request,omitempty"`
	Security   *SecurityEvent   `json:"security,omitempty"`

	// Metadata
	Duration time.Duration `json:"duration,omitempty"`
	Status   string        `json:"status"`
	Error    string        `json:"error,omitempty"`
}

// Session represents an Agent session being monitored.
type Session struct {
	ID          string        `json:"id"`
	Platform    Platform      `json:"platform"`
	UserID      string        `json:"user_id"`
	TenantID    string        `json:"tenant_id,omitempty"`
	Status      SessionStatus `json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	EndedAt     *time.Time    `json:"ended_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	EventCount  int           `json:"event_count"`
	ThreatLevel ThreatLevel   `json:"threat_level"`
	ThreatScore int           `json:"threat_score"`
	Metadata    SessionMeta   `json:"metadata,omitempty"`
}

// SessionMeta contains additional session metadata.
type SessionMeta struct {
	AgentID       string `json:"agent_id,omitempty"`
	ChannelID     string `json:"channel_id,omitempty"`
	ThreadID      string `json:"thread_id,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	ClientIP      string `json:"client_ip,omitempty"`
	MessageCount  int    `json:"message_count"`
	ToolCallCount int    `json:"tool_call_count"`
	LLMCallCount  int    `json:"llm_call_count"`
	TotalTokens   int    `json:"total_tokens"`
}

// MessageEvent represents a message event.
type MessageEvent struct {
	Direction   string `json:"direction"` // "inbound" or "outbound"
	Content     string `json:"content"`
	ContentType string `json:"content_type,omitempty"` // text, image, audio, etc.
	Length      int    `json:"length"`
	Truncated   bool   `json:"truncated,omitempty"`
}

// ToolCallEvent represents a tool/function call event.
type ToolCallEvent struct {
	ToolName    string                 `json:"tool_name"`
	ToolID      string                 `json:"tool_id"`
	Input       map[string]interface{} `json:"input"`
	Output      interface{}            `json:"output,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Status      string                 `json:"status"` // pending, running, completed, failed
	SandboxUsed bool                   `json:"sandbox_used"`
	Resources   *ResourceUsage         `json:"resources,omitempty"`
}

// LLMRequestEvent represents an LLM API request event.
type LLMRequestEvent struct {
	Provider       string        `json:"provider"` // openai, anthropic, ollama, etc.
	Model          string        `json:"model"`
	PromptTokens   int           `json:"prompt_tokens"`
	CompletionTokens int         `json:"completion_tokens"`
	TotalTokens    int           `json:"total_tokens"`
	Duration       time.Duration `json:"duration"`
	Status         string        `json:"status"`
	Error          string        `json:"error,omitempty"`
	Temperature    float64       `json:"temperature,omitempty"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	StopReason     string        `json:"stop_reason,omitempty"`
}

// SecurityEvent represents a security-related event.
type SecurityEvent struct {
	ThreatLevel      ThreatLevel `json:"threat_level"`
	ThreatScore      int         `json:"threat_score"`
	ThreatTypes      []string    `json:"threat_types"`
	DetectedPatterns []string    `json:"detected_patterns"`
	Action           string      `json:"action"` // allowed, blocked, filtered
	FilteredContent  string      `json:"filtered_content,omitempty"`
	Source           string      `json:"source,omitempty"` // prompt_guard, audit, sandbox
}

// ResourceUsage represents resource consumption during execution.
type ResourceUsage struct {
	CPUTime    time.Duration `json:"cpu_time"`
	MemoryPeak int64         `json:"memory_peak"` // bytes
	IORead     int64         `json:"io_read"`     // bytes
	IOWrite    int64         `json:"io_write"`    // bytes
}

// Alert represents a security or operational alert.
type Alert struct {
	ID           string        `json:"id"`
	Severity     AlertSeverity `json:"severity"`
	Title        string        `json:"title"`
	Description  string        `json:"description,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	EventID      string        `json:"event_id,omitempty"`
	ThreatLevel  ThreatLevel   `json:"threat_level,omitempty"`
	Details      interface{}   `json:"details,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
	Acknowledged bool          `json:"acknowledged"`
	AckedAt      *time.Time    `json:"acked_at,omitempty"`
	AckedBy      string        `json:"acked_by,omitempty"`
}

// FlowNode represents a node in the operation flow graph.
type FlowNode struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // message, tool_call, llm_request, security_check
	Label     string                 `json:"label"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Status    string                 `json:"status"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Position  *Position              `json:"position,omitempty"`
}

// FlowEdge represents an edge connecting two nodes in the flow graph.
type FlowEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

// Position represents the visual position of a node.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// FlowGraph represents the complete operation flow for a session.
type FlowGraph struct {
	SessionID string     `json:"session_id"`
	Nodes     []FlowNode `json:"nodes"`
	Edges     []FlowEdge `json:"edges"`
}

// Stats represents companion statistics.
type Stats struct {
	ActiveSessions   int            `json:"active_sessions"`
	TotalSessions    int            `json:"total_sessions"`
	TotalEvents      int            `json:"total_events"`
	TotalAlerts      int            `json:"total_alerts"`
	UnackedAlerts    int            `json:"unacked_alerts"`
	SessionsByPlatform map[Platform]int `json:"sessions_by_platform"`
	ThreatsByLevel   map[ThreatLevel]int `json:"threats_by_level"`
	EventsByType     map[SessionEventType]int `json:"events_by_type"`
	AvgSessionDuration time.Duration `json:"avg_session_duration"`
	LastUpdated      time.Time      `json:"last_updated"`
}

// DailyStats represents daily aggregated statistics.
type DailyStats struct {
	Date             string         `json:"date"` // YYYY-MM-DD
	TotalSessions    int            `json:"total_sessions"`
	TotalEvents      int            `json:"total_events"`
	TotalAlerts      int            `json:"total_alerts"`
	SessionsByPlatform map[Platform]int `json:"sessions_by_platform"`
	ThreatsByLevel   map[ThreatLevel]int `json:"threats_by_level"`
	AvgSessionDuration time.Duration `json:"avg_session_duration"`
}

// ListOptions contains options for listing resources.
type ListOptions struct {
	Offset   int               `json:"offset"`
	Limit    int               `json:"limit"`
	Sort     string            `json:"sort,omitempty"`
	Order    string            `json:"order,omitempty"` // asc, desc
	Filters  map[string]string `json:"filters,omitempty"`
	Platform Platform          `json:"platform,omitempty"`
	UserID   string            `json:"user_id,omitempty"`
	Status   SessionStatus     `json:"status,omitempty"`
	From     *time.Time        `json:"from,omitempty"`
	To       *time.Time        `json:"to,omitempty"`
}

// ExportOptions contains options for exporting data.
type ExportOptions struct {
	Format     string     `json:"format"` // json, csv
	SessionIDs []string   `json:"session_ids,omitempty"`
	From       *time.Time `json:"from,omitempty"`
	To         *time.Time `json:"to,omitempty"`
}

// Config contains companion service configuration.
type Config struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Storage configuration
	Storage StorageConfig `json:"storage" yaml:"storage"`

	// WebSocket configuration
	WebSocket WebSocketConfig `json:"websocket" yaml:"websocket"`

	// Retention configuration
	Retention RetentionConfig `json:"retention" yaml:"retention"`

	// Alert configuration
	Alerts AlertConfig `json:"alerts" yaml:"alerts"`

	// Security integration
	Security SecurityIntegrationConfig `json:"security" yaml:"security"`

	// Performance configuration
	Performance PerformanceConfig `json:"performance" yaml:"performance"`
}

// StorageConfig contains storage configuration.
type StorageConfig struct {
	BasePath string `json:"base_path" yaml:"base_path"`
	Format   string `json:"format" yaml:"format"` // jsonl
}

// WebSocketConfig contains WebSocket configuration.
type WebSocketConfig struct {
	PingInterval    time.Duration `json:"ping_interval" yaml:"ping_interval"`
	WriteTimeout    time.Duration `json:"write_timeout" yaml:"write_timeout"`
	ReadBufferSize  int           `json:"read_buffer_size" yaml:"read_buffer_size"`
	WriteBufferSize int           `json:"write_buffer_size" yaml:"write_buffer_size"`
}

// RetentionConfig contains data retention configuration.
type RetentionConfig struct {
	EventsDays   int `json:"events_days" yaml:"events_days"`
	SessionsDays int `json:"sessions_days" yaml:"sessions_days"`
	AlertsDays   int `json:"alerts_days" yaml:"alerts_days"`
}

// AlertConfig contains alert configuration.
type AlertConfig struct {
	Enabled         bool          `json:"enabled" yaml:"enabled"`
	ThreatThreshold ThreatLevel   `json:"threat_threshold" yaml:"threat_threshold"`
	Channels        []AlertChannel `json:"channels" yaml:"channels"`
}

// AlertChannel represents an alert notification channel.
type AlertChannel struct {
	Type       string            `json:"type" yaml:"type"` // webhook, email
	URL        string            `json:"url,omitempty" yaml:"url,omitempty"`
	Recipients []string          `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	Headers    map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// SecurityIntegrationConfig contains security integration configuration.
type SecurityIntegrationConfig struct {
	PromptGuardIntegration bool `json:"prompt_guard_integration" yaml:"prompt_guard_integration"`
	AuditLogIntegration    bool `json:"audit_log_integration" yaml:"audit_log_integration"`
	SandboxMonitor         bool `json:"sandbox_monitor" yaml:"sandbox_monitor"`
}

// PerformanceConfig contains performance configuration.
type PerformanceConfig struct {
	MaxConcurrentSessions int           `json:"max_concurrent_sessions" yaml:"max_concurrent_sessions"`
	EventBufferSize       int           `json:"event_buffer_size" yaml:"event_buffer_size"`
	BatchWriteInterval    time.Duration `json:"batch_write_interval" yaml:"batch_write_interval"`
}

// DefaultConfig returns the default companion configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Storage: StorageConfig{
			BasePath: "./data/companion",
			Format:   "jsonl",
		},
		WebSocket: WebSocketConfig{
			PingInterval:    30 * time.Second,
			WriteTimeout:    10 * time.Second,
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		Retention: RetentionConfig{
			EventsDays:   7,
			SessionsDays: 30,
			AlertsDays:   90,
		},
		Alerts: AlertConfig{
			Enabled:         true,
			ThreatThreshold: ThreatLevelMedium,
			Channels:        []AlertChannel{},
		},
		Security: SecurityIntegrationConfig{
			PromptGuardIntegration: true,
			AuditLogIntegration:    true,
			SandboxMonitor:         true,
		},
		Performance: PerformanceConfig{
			MaxConcurrentSessions: 1000,
			EventBufferSize:       10000,
			BatchWriteInterval:    time.Second,
		},
	}
}

// Service defines the companion service interface.
type Service interface {
	// Session management
	CreateSession(ctx context.Context, session *Session) (*Session, error)
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessions(ctx context.Context, opts *ListOptions) ([]*Session, int, error)
	EndSession(ctx context.Context, id string) error

	// Event management
	EmitEvent(ctx context.Context, event *SessionEvent) error
	GetSessionEvents(ctx context.Context, sessionID string, opts *ListOptions) ([]*SessionEvent, int, error)

	// Flow graph
	GetSessionFlow(ctx context.Context, sessionID string) (*FlowGraph, error)

	// Alerts
	GetAlerts(ctx context.Context, opts *ListOptions) ([]*Alert, int, error)
	AcknowledgeAlert(ctx context.Context, id string, userID string) error

	// Stats
	GetStats(ctx context.Context) (*Stats, error)

	// Export
	Export(ctx context.Context, opts *ExportOptions) ([]byte, error)

	// Subscription
	Subscribe(ctx context.Context, sessionID string) (<-chan *SessionEvent, func(), error)

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Storage defines the storage interface for companion data.
type Storage interface {
	// Session storage
	SaveSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessions(ctx context.Context, opts *ListOptions) ([]*Session, int, error)
	UpdateSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, id string) error

	// Event storage
	AppendEvent(ctx context.Context, event *SessionEvent) error
	GetSessionEvents(ctx context.Context, sessionID string, opts *ListOptions) ([]*SessionEvent, int, error)

	// Alert storage
	SaveAlert(ctx context.Context, alert *Alert) error
	GetAlert(ctx context.Context, id string) (*Alert, error)
	ListAlerts(ctx context.Context, opts *ListOptions) ([]*Alert, int, error)
	UpdateAlert(ctx context.Context, alert *Alert) error

	// Stats storage
	SaveDailyStats(ctx context.Context, stats *DailyStats) error
	GetDailyStats(ctx context.Context, date string) (*DailyStats, error)

	// Cleanup
	CleanupExpired(ctx context.Context, retention *RetentionConfig) error

	// Lifecycle
	Close() error
}

// Streamer defines the event streaming interface.
type Streamer interface {
	// Emit an event to all subscribers
	Emit(event *SessionEvent)

	// Subscribe to events (optionally filtered by session ID)
	Subscribe(sessionID string) (<-chan *SessionEvent, func())

	// Start the streamer
	Start(ctx context.Context) error

	// Stop the streamer
	Stop() error
}

// AlertEngine defines the alert engine interface.
type AlertEngine interface {
	// Trigger an alert
	Trigger(ctx context.Context, alert *Alert) error

	// Check if an event should trigger an alert
	CheckEvent(ctx context.Context, event *SessionEvent) error

	// Start the alert engine
	Start(ctx context.Context) error

	// Stop the alert engine
	Stop() error
}
