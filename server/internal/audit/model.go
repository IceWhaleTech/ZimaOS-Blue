// Package audit provides audit logging functionality.
package audit

import (
	"encoding/json"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// Action represents the type of action being audited.
type Action string

const (
	// Authentication actions
	ActionLogin         Action = "login"
	ActionLoginFailed   Action = "login_failed"
	ActionLogout        Action = "logout"
	ActionPasswordChange Action = "password_change"
	ActionPasswordReset Action = "password_reset"
	ActionMFASetup      Action = "mfa_setup"
	ActionMFADisable    Action = "mfa_disable"
	ActionMFAVerify     Action = "mfa_verify"

	// User management actions
	ActionUserCreate  Action = "user_create"
	ActionUserUpdate  Action = "user_update"
	ActionUserDelete  Action = "user_delete"
	ActionUserLock    Action = "user_lock"
	ActionUserUnlock  Action = "user_unlock"
	ActionRoleChange  Action = "role_change"

	// API access actions
	ActionAPIAccess   Action = "api_access"
	ActionConfigChange Action = "config_change"

	// System actions
	ActionSystemStart  Action = "system_start"
	ActionSystemStop   Action = "system_stop"
	ActionConfigReload Action = "config_reload"
)

// Status represents the outcome of an action.
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
)

// Entry represents a single audit log entry.
type Entry struct {
	// ID is the unique identifier for this entry.
	ID uuid.UUID `json:"id" db:"id"`
	// Timestamp is when the action occurred.
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
	// UserID is the user who performed the action (nil for anonymous).
	UserID *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	// Username is the username (for display purposes).
	Username string `json:"username,omitempty" db:"username"`
	// Action is the type of action performed.
	Action Action `json:"action" db:"action"`
	// ResourceType is the type of resource affected.
	ResourceType string `json:"resource_type,omitempty" db:"resource_type"`
	// ResourceID is the ID of the resource affected.
	ResourceID string `json:"resource_id,omitempty" db:"resource_id"`
	// IPAddress is the client's IP address.
	IPAddress string `json:"ip_address,omitempty" db:"ip_address"`
	// UserAgent is the client's user agent string.
	UserAgent string `json:"user_agent,omitempty" db:"user_agent"`
	// RequestID is the unique request identifier.
	RequestID string `json:"request_id,omitempty" db:"request_id"`
	// Status is the outcome of the action.
	Status Status `json:"status" db:"status"`
	// Details contains additional action-specific information.
	Details json.RawMessage `json:"details,omitempty" db:"details"`
	// OldValue contains the previous value (for updates).
	OldValue json.RawMessage `json:"old_value,omitempty" db:"old_value"`
	// NewValue contains the new value (for updates).
	NewValue json.RawMessage `json:"new_value,omitempty" db:"new_value"`
}

// NewEntry creates a new audit entry with default values.
func NewEntry(action Action, status Status) *Entry {
	return &Entry{
		ID:        uuid.New(),
		Timestamp: timeutil.NowTime().UTC(),
		Action:    action,
		Status:    status,
	}
}

// WithUser sets the user information.
func (e *Entry) WithUser(userID uuid.UUID, username string) *Entry {
	e.UserID = &userID
	e.Username = username
	return e
}

// WithResource sets the resource information.
func (e *Entry) WithResource(resourceType, resourceID string) *Entry {
	e.ResourceType = resourceType
	e.ResourceID = resourceID
	return e
}

// WithRequest sets the request information.
func (e *Entry) WithRequest(ipAddress, userAgent, requestID string) *Entry {
	e.IPAddress = ipAddress
	e.UserAgent = userAgent
	e.RequestID = requestID
	return e
}

// WithDetails sets the details field.
func (e *Entry) WithDetails(details interface{}) *Entry {
	if details != nil {
		data, _ := json.Marshal(details)
		e.Details = data
	}
	return e
}

// WithChange sets the old and new values for update actions.
func (e *Entry) WithChange(oldValue, newValue interface{}) *Entry {
	if oldValue != nil {
		data, _ := json.Marshal(oldValue)
		e.OldValue = data
	}
	if newValue != nil {
		data, _ := json.Marshal(newValue)
		e.NewValue = data
	}
	return e
}

// QueryParams represents parameters for querying audit logs.
type QueryParams struct {
	// UserID filters by user.
	UserID *uuid.UUID
	// Action filters by action type.
	Action *Action
	// ResourceType filters by resource type.
	ResourceType string
	// ResourceID filters by resource ID.
	ResourceID string
	// Status filters by status.
	Status *Status
	// StartTime filters entries after this time.
	StartTime *time.Time
	// EndTime filters entries before this time.
	EndTime *time.Time
	// IPAddress filters by IP address.
	IPAddress string
	// Page is the page number (1-indexed).
	Page int
	// PageSize is the number of entries per page.
	PageSize int
	// SortBy is the field to sort by.
	SortBy string
	// SortDir is the sort direction (asc/desc).
	SortDir string
}

// QueryResult represents the result of a query.
type QueryResult struct {
	Entries    []*Entry `json:"entries"`
	Total      int64    `json:"total"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	TotalPages int      `json:"total_pages"`
}
