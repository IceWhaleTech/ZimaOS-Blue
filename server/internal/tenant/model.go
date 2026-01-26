// Package tenant provides multi-tenant functionality.
package tenant

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the tenant status.
type Status string

const (
	// StatusActive indicates an active tenant.
	StatusActive Status = "active"
	// StatusSuspended indicates a suspended tenant.
	StatusSuspended Status = "suspended"
	// StatusDeleted indicates a soft-deleted tenant.
	StatusDeleted Status = "deleted"
)

// MemberRole represents a user's role within a tenant.
type MemberRole string

const (
	// MemberRoleOwner has full control over the tenant.
	MemberRoleOwner MemberRole = "owner"
	// MemberRoleAdmin can manage tenant settings and members.
	MemberRoleAdmin MemberRole = "admin"
	// MemberRoleMember has standard access.
	MemberRoleMember MemberRole = "member"
)

// Tenant represents a tenant (organization) in the system.
type Tenant struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id" db:"id"`
	// Name is the tenant display name.
	Name string `json:"name" db:"name"`
	// Slug is the URL-friendly identifier.
	Slug string `json:"slug" db:"slug"`
	// Description is an optional description.
	Description *string `json:"description,omitempty" db:"description"`
	// Status is the tenant status.
	Status Status `json:"status" db:"status"`
	// Settings contains tenant-specific settings as JSON.
	Settings *string `json:"settings,omitempty" db:"settings"`
	// Limits contains resource limits as JSON.
	Limits *string `json:"limits,omitempty" db:"limits"`
	// OwnerID is the ID of the tenant owner.
	OwnerID uuid.UUID `json:"owner_id" db:"owner_id"`
	// CreatedAt is when the tenant was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// UpdatedAt is when the tenant was last updated.
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	// DeletedAt is when the tenant was soft-deleted.
	DeletedAt *time.Time `json:"-" db:"deleted_at"`
}

// NewTenant creates a new Tenant with default values.
func NewTenant(name, slug string, ownerID uuid.UUID) *Tenant {
	now := time.Now().UTC()
	return &Tenant{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		Status:    StatusActive,
		OwnerID:   ownerID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsActive returns true if the tenant is active.
func (t *Tenant) IsActive() bool {
	return t.Status == StatusActive && t.DeletedAt == nil
}

// TenantMember represents a user's membership in a tenant.
type TenantMember struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id" db:"id"`
	// TenantID is the tenant ID.
	TenantID uuid.UUID `json:"tenant_id" db:"tenant_id"`
	// UserID is the user ID.
	UserID uuid.UUID `json:"user_id" db:"user_id"`
	// Role is the member's role within the tenant.
	Role MemberRole `json:"role" db:"role"`
	// JoinedAt is when the user joined the tenant.
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
	// InvitedBy is the ID of the user who invited this member.
	InvitedBy *uuid.UUID `json:"invited_by,omitempty" db:"invited_by"`
}

// NewTenantMember creates a new TenantMember.
func NewTenantMember(tenantID, userID uuid.UUID, role MemberRole, invitedBy *uuid.UUID) *TenantMember {
	return &TenantMember{
		ID:        uuid.New(),
		TenantID:  tenantID,
		UserID:    userID,
		Role:      role,
		JoinedAt:  time.Now().UTC(),
		InvitedBy: invitedBy,
	}
}

// TenantInvitation represents an invitation to join a tenant.
type TenantInvitation struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id" db:"id"`
	// TenantID is the tenant ID.
	TenantID uuid.UUID `json:"tenant_id" db:"tenant_id"`
	// Email is the invited email address.
	Email string `json:"email" db:"email"`
	// Role is the role to assign upon acceptance.
	Role MemberRole `json:"role" db:"role"`
	// Token is the invitation token.
	Token string `json:"-" db:"token"`
	// InvitedBy is the ID of the user who created the invitation.
	InvitedBy uuid.UUID `json:"invited_by" db:"invited_by"`
	// ExpiresAt is when the invitation expires.
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	// AcceptedAt is when the invitation was accepted.
	AcceptedAt *time.Time `json:"accepted_at,omitempty" db:"accepted_at"`
	// CreatedAt is when the invitation was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NewTenantInvitation creates a new TenantInvitation.
func NewTenantInvitation(tenantID uuid.UUID, email string, role MemberRole, invitedBy uuid.UUID, token string, expiresAt time.Time) *TenantInvitation {
	return &TenantInvitation{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		Token:     token,
		InvitedBy: invitedBy,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}
}

// IsValid returns true if the invitation is not expired and not accepted.
func (i *TenantInvitation) IsValid() bool {
	if i.AcceptedAt != nil {
		return false
	}
	return time.Now().Before(i.ExpiresAt)
}

// TenantLimits represents resource limits for a tenant.
type TenantLimits struct {
	// MaxUsers is the maximum number of users.
	MaxUsers int `json:"max_users"`
	// MaxStorage is the maximum storage in bytes.
	MaxStorage int64 `json:"max_storage"`
	// MaxAPIRequests is the maximum API requests per day.
	MaxAPIRequests int `json:"max_api_requests"`
	// MaxWorkflows is the maximum number of workflows.
	MaxWorkflows int `json:"max_workflows"`
	// MaxChannels is the maximum number of channels.
	MaxChannels int `json:"max_channels"`
}

// DefaultTenantLimits returns the default limits for a new tenant.
func DefaultTenantLimits() *TenantLimits {
	return &TenantLimits{
		MaxUsers:       10,
		MaxStorage:     10 * 1024 * 1024 * 1024, // 10 GB
		MaxAPIRequests: 10000,
		MaxWorkflows:   50,
		MaxChannels:    5,
	}
}

// TenantSettings represents tenant-specific settings.
type TenantSettings struct {
	// DefaultLanguage is the default language for the tenant.
	DefaultLanguage string `json:"default_language"`
	// Timezone is the default timezone.
	Timezone string `json:"timezone"`
	// Features contains enabled feature flags.
	Features map[string]bool `json:"features"`
}

// DefaultTenantSettings returns the default settings for a new tenant.
func DefaultTenantSettings() *TenantSettings {
	return &TenantSettings{
		DefaultLanguage: "en",
		Timezone:        "UTC",
		Features:        make(map[string]bool),
	}
}

// CreateTenantRequest represents a request to create a tenant.
type CreateTenantRequest struct {
	Name        string  `json:"name" validate:"required,min=2,max=100"`
	Slug        string  `json:"slug" validate:"required,min=2,max=50,alphanum"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
}

// UpdateTenantRequest represents a request to update a tenant.
type UpdateTenantRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
	Status      *Status `json:"status,omitempty"`
}

// InviteMemberRequest represents a request to invite a member.
type InviteMemberRequest struct {
	Email string     `json:"email" validate:"required,email"`
	Role  MemberRole `json:"role" validate:"required,oneof=admin member"`
}

// UpdateMemberRequest represents a request to update a member's role.
type UpdateMemberRequest struct {
	Role MemberRole `json:"role" validate:"required,oneof=owner admin member"`
}

// ListTenantsQuery represents query parameters for listing tenants.
type ListTenantsQuery struct {
	Page     int     `query:"page"`
	PageSize int     `query:"page_size"`
	Search   string  `query:"search"`
	Status   *Status `query:"status"`
	SortBy   string  `query:"sort_by"`
	SortDir  string  `query:"sort_dir"`
}

// ListTenantsResponse represents a paginated list of tenants.
type ListTenantsResponse struct {
	Tenants    []*Tenant `json:"tenants"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalPages int       `json:"total_pages"`
}

// ListMembersQuery represents query parameters for listing members.
type ListMembersQuery struct {
	Page     int         `query:"page"`
	PageSize int         `query:"page_size"`
	Role     *MemberRole `query:"role"`
}

// ListMembersResponse represents a paginated list of members.
type ListMembersResponse struct {
	Members    []*TenantMemberWithUser `json:"members"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"page_size"`
	TotalPages int                     `json:"total_pages"`
}

// TenantMemberWithUser represents a member with user details.
type TenantMemberWithUser struct {
	TenantMember
	Username string  `json:"username"`
	Email    *string `json:"email,omitempty"`
}
