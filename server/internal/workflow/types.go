// Package workflow provides n8n-style workflow automation capabilities.
package workflow

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrWorkflowNotFound     = errors.New("workflow not found")
	ErrWorkflowDisabled     = errors.New("workflow is disabled")
	ErrExecutionNotFound    = errors.New("execution not found")
	ErrNodeNotFound         = errors.New("node not found")
	ErrInvalidWorkflow      = errors.New("invalid workflow definition")
	ErrInvalidNode          = errors.New("invalid node configuration")
	ErrInvalidTrigger       = errors.New("invalid trigger configuration")
	ErrCyclicDependency     = errors.New("cyclic dependency detected")
	ErrMaxExecutionsReached = errors.New("maximum concurrent executions reached")
	ErrExecutionTimeout     = errors.New("execution timed out")
	ErrNodeExecutionFailed  = errors.New("node execution failed")
)

// WorkflowStatus represents the status of a workflow.
type WorkflowStatus string

const (
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusInactive WorkflowStatus = "inactive"
	WorkflowStatusDraft    WorkflowStatus = "draft"
)

// ExecutionStatus represents the status of a workflow execution.
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusPaused    ExecutionStatus = "paused"
)

// NodeStatus represents the status of a node execution.
type NodeStatus string

const (
	NodeStatusPending   NodeStatus = "pending"
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusCompleted NodeStatus = "completed"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusSkipped   NodeStatus = "skipped"
)

// TriggerType represents the type of workflow trigger.
type TriggerType string

const (
	TriggerTypeManual       TriggerType = "manual"
	TriggerTypeSchedule     TriggerType = "schedule"
	TriggerTypeWebhook      TriggerType = "webhook"
	TriggerTypeFileChange   TriggerType = "file_change"
	TriggerTypeChannelMsg   TriggerType = "channel_message"
	TriggerTypeHAEvent      TriggerType = "ha_event"
	TriggerTypeHAState      TriggerType = "ha_state"
	TriggerTypeSystemEvent  TriggerType = "system_event"
)

// NodeType represents the type of workflow node.
type NodeType string

const (
	NodeTypeTrigger   NodeType = "trigger"
	NodeTypeAction    NodeType = "action"
	NodeTypeCondition NodeType = "condition"
	NodeTypeLoop      NodeType = "loop"
	NodeTypeSwitch    NodeType = "switch"
	NodeTypeMerge     NodeType = "merge"
	NodeTypeDelay     NodeType = "delay"
	NodeTypeSubflow   NodeType = "subflow"
)

// ActionType represents the type of action node.
type ActionType string

const (
	ActionTypeHTTP        ActionType = "http"
	ActionTypeLLM         ActionType = "llm"
	ActionTypeSkill       ActionType = "skill"
	ActionTypeChannelSend ActionType = "channel_send"
	ActionTypeHAService   ActionType = "ha_service"
	ActionTypeBrowser     ActionType = "browser"
	ActionTypeFileOps     ActionType = "file_ops"
	ActionTypeJavaScript  ActionType = "javascript"
	ActionTypeSetVariable ActionType = "set_variable"
	ActionTypeEmail       ActionType = "email"
	ActionTypeNotify      ActionType = "notify"
)

// Workflow represents a workflow definition.
type Workflow struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Status      WorkflowStatus    `json:"status"`
	Nodes       []Node            `json:"nodes"`
	Connections []Connection      `json:"connections"`
	Variables   map[string]string `json:"variables,omitempty"`
	Settings    *WorkflowSettings `json:"settings,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CreatedBy   string            `json:"created_by,omitempty"`
	UpdatedBy   string            `json:"updated_by,omitempty"`
}

// WorkflowSettings contains workflow-level settings.
type WorkflowSettings struct {
	Timeout            int  `json:"timeout,omitempty"`              // Execution timeout in seconds
	MaxRetries         int  `json:"max_retries,omitempty"`          // Max retries on failure
	RetryDelay         int  `json:"retry_delay,omitempty"`          // Delay between retries in seconds
	ContinueOnError    bool `json:"continue_on_error,omitempty"`    // Continue execution on node error
	SaveExecutionData  bool `json:"save_execution_data,omitempty"`  // Save execution data for debugging
	MaxConcurrent      int  `json:"max_concurrent,omitempty"`       // Max concurrent executions
	NotifyOnCompletion bool `json:"notify_on_completion,omitempty"` // Send notification on completion
	NotifyOnError      bool `json:"notify_on_error,omitempty"`      // Send notification on error
}

// Node represents a workflow node.
type Node struct {
	ID          string                 `json:"id"`
	Type        NodeType               `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Position    *Position              `json:"position,omitempty"`
	Disabled    bool                   `json:"disabled,omitempty"`
}

// Position represents the visual position of a node.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Connection represents a connection between nodes.
type Connection struct {
	ID         string `json:"id"`
	SourceNode string `json:"source_node"`
	SourcePort string `json:"source_port,omitempty"`
	TargetNode string `json:"target_node"`
	TargetPort string `json:"target_port,omitempty"`
	Condition  string `json:"condition,omitempty"` // For conditional connections
}

// TriggerConfig contains trigger-specific configuration.
type TriggerConfig struct {
	Type     TriggerType            `json:"type"`
	Schedule *ScheduleConfig        `json:"schedule,omitempty"`
	Webhook  *WebhookConfig         `json:"webhook,omitempty"`
	File     *FileChangeConfig      `json:"file,omitempty"`
	Channel  *ChannelTriggerConfig  `json:"channel,omitempty"`
	HA       *HATriggerConfig       `json:"ha,omitempty"`
	System   *SystemTriggerConfig   `json:"system,omitempty"`
}

// ScheduleConfig contains schedule trigger configuration.
type ScheduleConfig struct {
	Cron     string `json:"cron,omitempty"`     // Cron expression
	Interval int    `json:"interval,omitempty"` // Interval in seconds
	Timezone string `json:"timezone,omitempty"` // Timezone for cron
}

// WebhookConfig contains webhook trigger configuration.
type WebhookConfig struct {
	Path       string            `json:"path"`
	Method     string            `json:"method,omitempty"` // GET, POST, etc.
	Headers    map[string]string `json:"headers,omitempty"`
	AuthType   string            `json:"auth_type,omitempty"` // none, basic, bearer, api_key
	AuthConfig map[string]string `json:"auth_config,omitempty"`
}

// FileChangeConfig contains file change trigger configuration.
type FileChangeConfig struct {
	Path      string   `json:"path"`
	Patterns  []string `json:"patterns,omitempty"`  // File patterns to watch
	Events    []string `json:"events,omitempty"`    // create, modify, delete
	Recursive bool     `json:"recursive,omitempty"` // Watch subdirectories
}

// ChannelTriggerConfig contains channel message trigger configuration.
type ChannelTriggerConfig struct {
	ChannelType string   `json:"channel_type"` // telegram, discord, etc.
	ChannelID   string   `json:"channel_id,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`    // Trigger on keywords
	Regex       string   `json:"regex,omitempty"`       // Trigger on regex match
	FromUsers   []string `json:"from_users,omitempty"`  // Filter by user
}

// HATriggerConfig contains Home Assistant trigger configuration.
type HATriggerConfig struct {
	EventType   string            `json:"event_type,omitempty"`   // For event triggers
	EntityID    string            `json:"entity_id,omitempty"`    // For state triggers
	FromState   string            `json:"from_state,omitempty"`   // Previous state
	ToState     string            `json:"to_state,omitempty"`     // New state
	Attribute   string            `json:"attribute,omitempty"`    // Attribute to watch
	Condition   string            `json:"condition,omitempty"`    // Additional condition
}

// SystemTriggerConfig contains system event trigger configuration.
type SystemTriggerConfig struct {
	EventType string `json:"event_type"` // startup, shutdown, etc.
}

// ActionConfig contains action-specific configuration.
type ActionConfig struct {
	Type       ActionType             `json:"type"`
	HTTP       *HTTPActionConfig      `json:"http,omitempty"`
	LLM        *LLMActionConfig       `json:"llm,omitempty"`
	Skill      *SkillActionConfig     `json:"skill,omitempty"`
	Channel    *ChannelActionConfig   `json:"channel,omitempty"`
	HA         *HAActionConfig        `json:"ha,omitempty"`
	Browser    *BrowserActionConfig   `json:"browser,omitempty"`
	File       *FileActionConfig      `json:"file,omitempty"`
	JavaScript *JavaScriptConfig      `json:"javascript,omitempty"`
	Variable   *VariableActionConfig  `json:"variable,omitempty"`
	Email      *EmailActionConfig     `json:"email,omitempty"`
	Notify     *NotifyActionConfig    `json:"notify,omitempty"`
}

// HTTPActionConfig contains HTTP action configuration.
type HTTPActionConfig struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
	FollowRedirects bool          `json:"follow_redirects,omitempty"`
	ValidateSSL bool              `json:"validate_ssl,omitempty"`
}

// LLMActionConfig contains LLM action configuration.
type LLMActionConfig struct {
	Provider    string   `json:"provider"` // openai, claude, ollama
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	SystemPrompt string  `json:"system_prompt,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Tools       []string `json:"tools,omitempty"` // Tool names to enable
}

// SkillActionConfig contains skill action configuration.
type SkillActionConfig struct {
	SkillID    string                 `json:"skill_id"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// ChannelActionConfig contains channel send action configuration.
type ChannelActionConfig struct {
	ChannelType string `json:"channel_type"`
	ChannelID   string `json:"channel_id"`
	Message     string `json:"message"`
	Format      string `json:"format,omitempty"` // text, markdown, html
}

// HAActionConfig contains Home Assistant action configuration.
type HAActionConfig struct {
	Domain    string                 `json:"domain"`
	Service   string                 `json:"service"`
	EntityID  string                 `json:"entity_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// BrowserActionConfig contains browser action configuration.
type BrowserActionConfig struct {
	Action   string                 `json:"action"` // navigate, click, type, screenshot, etc.
	URL      string                 `json:"url,omitempty"`
	Selector string                 `json:"selector,omitempty"`
	Value    string                 `json:"value,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// FileActionConfig contains file operation configuration.
type FileActionConfig struct {
	Operation   string `json:"operation"` // read, write, copy, move, delete
	SourcePath  string `json:"source_path"`
	DestPath    string `json:"dest_path,omitempty"`
	Content     string `json:"content,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	CreateDirs  bool   `json:"create_dirs,omitempty"`
}

// JavaScriptConfig contains JavaScript execution configuration.
type JavaScriptConfig struct {
	Code    string `json:"code"`
	Timeout int    `json:"timeout,omitempty"`
}

// VariableActionConfig contains variable set action configuration.
type VariableActionConfig struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Scope string `json:"scope,omitempty"` // execution, workflow
}

// EmailActionConfig contains email action configuration.
type EmailActionConfig struct {
	To          []string `json:"to"`
	Cc          []string `json:"cc,omitempty"`
	Bcc         []string `json:"bcc,omitempty"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	ContentType string   `json:"content_type,omitempty"` // text, html
	Attachments []string `json:"attachments,omitempty"`
}

// NotifyActionConfig contains notification action configuration.
type NotifyActionConfig struct {
	Title    string `json:"title,omitempty"`
	Message  string `json:"message"`
	Priority string `json:"priority,omitempty"` // low, normal, high
	Channel  string `json:"channel,omitempty"`  // Notification channel
}

// ConditionConfig contains condition node configuration.
type ConditionConfig struct {
	Expression string `json:"expression"` // JavaScript expression
	TruePort   string `json:"true_port,omitempty"`
	FalsePort  string `json:"false_port,omitempty"`
}

// LoopConfig contains loop node configuration.
type LoopConfig struct {
	Type       string `json:"type"`        // for_each, while, count
	Items      string `json:"items,omitempty"`      // Expression for items to iterate
	Condition  string `json:"condition,omitempty"`  // While condition
	Count      int    `json:"count,omitempty"`      // Fixed count
	MaxIterations int `json:"max_iterations,omitempty"` // Safety limit
}

// SwitchConfig contains switch node configuration.
type SwitchConfig struct {
	Expression string            `json:"expression"` // Expression to evaluate
	Cases      map[string]string `json:"cases"`      // Value -> port mapping
	Default    string            `json:"default,omitempty"` // Default port
}

// DelayConfig contains delay node configuration.
type DelayConfig struct {
	Duration int    `json:"duration"` // Delay in seconds
	Until    string `json:"until,omitempty"` // Wait until specific time
}

// Execution represents a workflow execution instance.
type Execution struct {
	ID           string                 `json:"id"`
	WorkflowID   string                 `json:"workflow_id"`
	WorkflowName string                 `json:"workflow_name"`
	TenantID     string                 `json:"tenant_id"`
	Status       ExecutionStatus        `json:"status"`
	TriggerType  TriggerType            `json:"trigger_type"`
	TriggerData  map[string]interface{} `json:"trigger_data,omitempty"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	NodeResults  map[string]*NodeResult `json:"node_results,omitempty"`
	Error        string                 `json:"error,omitempty"`
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Duration     int64                  `json:"duration,omitempty"` // Duration in milliseconds
}

// NodeResult represents the result of a node execution.
type NodeResult struct {
	NodeID      string                 `json:"node_id"`
	NodeName    string                 `json:"node_name"`
	Status      NodeStatus             `json:"status"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    int64                  `json:"duration,omitempty"` // Duration in milliseconds
	RetryCount  int                    `json:"retry_count,omitempty"`
}

// ExecutionLog represents a log entry for an execution.
type ExecutionLog struct {
	ID          string    `json:"id"`
	ExecutionID string    `json:"execution_id"`
	NodeID      string    `json:"node_id,omitempty"`
	Level       string    `json:"level"` // debug, info, warn, error
	Message     string    `json:"message"`
	Data        string    `json:"data,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Service defines the workflow service interface.
type Service interface {
	// Workflow CRUD
	CreateWorkflow(ctx context.Context, workflow *Workflow) (*Workflow, error)
	GetWorkflow(ctx context.Context, id string) (*Workflow, error)
	UpdateWorkflow(ctx context.Context, workflow *Workflow) (*Workflow, error)
	DeleteWorkflow(ctx context.Context, id string) error
	ListWorkflows(ctx context.Context, tenantID string, opts *ListOptions) ([]*Workflow, int, error)

	// Workflow control
	EnableWorkflow(ctx context.Context, id string) error
	DisableWorkflow(ctx context.Context, id string) error
	ValidateWorkflow(ctx context.Context, workflow *Workflow) error

	// Execution
	ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (*Execution, error)
	GetExecution(ctx context.Context, id string) (*Execution, error)
	ListExecutions(ctx context.Context, workflowID string, opts *ListOptions) ([]*Execution, int, error)
	CancelExecution(ctx context.Context, id string) error
	RetryExecution(ctx context.Context, id string) (*Execution, error)

	// Logs
	GetExecutionLogs(ctx context.Context, executionID string, opts *ListOptions) ([]*ExecutionLog, int, error)

	// Triggers
	RegisterTrigger(ctx context.Context, workflowID string, trigger *TriggerConfig) error
	UnregisterTrigger(ctx context.Context, workflowID string) error

	// Webhook
	HandleWebhook(ctx context.Context, path string, method string, headers map[string]string, body []byte) (*Execution, error)

	// Stats
	GetStats(ctx context.Context, tenantID string) (*Stats, error)
}

// ListOptions contains options for listing resources.
type ListOptions struct {
	Offset  int               `json:"offset"`
	Limit   int               `json:"limit"`
	Sort    string            `json:"sort,omitempty"`
	Order   string            `json:"order,omitempty"` // asc, desc
	Filters map[string]string `json:"filters,omitempty"`
}

// Stats contains workflow statistics.
type Stats struct {
	TotalWorkflows     int `json:"total_workflows"`
	ActiveWorkflows    int `json:"active_workflows"`
	TotalExecutions    int `json:"total_executions"`
	RunningExecutions  int `json:"running_executions"`
	SuccessfulExecutions int `json:"successful_executions"`
	FailedExecutions   int `json:"failed_executions"`
}

// Config contains workflow engine configuration.
type Config struct {
	MaxConcurrentExecutions int           `json:"max_concurrent_executions" yaml:"max_concurrent_executions"`
	DefaultTimeout          int           `json:"default_timeout" yaml:"default_timeout"` // seconds
	MaxTimeout              int           `json:"max_timeout" yaml:"max_timeout"`         // seconds
	RetryDelay              int           `json:"retry_delay" yaml:"retry_delay"`         // seconds
	MaxRetries              int           `json:"max_retries" yaml:"max_retries"`
	SaveExecutionData       bool          `json:"save_execution_data" yaml:"save_execution_data"`
	ExecutionDataRetention  int           `json:"execution_data_retention" yaml:"execution_data_retention"` // days
	WebhookBasePath         string        `json:"webhook_base_path" yaml:"webhook_base_path"`
	Enabled                 bool          `json:"enabled" yaml:"enabled"`
}

// DefaultConfig returns the default workflow configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxConcurrentExecutions: 10,
		DefaultTimeout:          300,  // 5 minutes
		MaxTimeout:              3600, // 1 hour
		RetryDelay:              5,
		MaxRetries:              3,
		SaveExecutionData:       true,
		ExecutionDataRetention:  30, // 30 days
		WebhookBasePath:         "/api/v1/webhooks",
		Enabled:                 true,
	}
}
