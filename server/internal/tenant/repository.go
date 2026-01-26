package tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	// ErrTenantNotFound is returned when a tenant is not found.
	ErrTenantNotFound = errors.New("tenant not found")
	// ErrTenantExists is returned when a tenant already exists.
	ErrTenantExists = errors.New("tenant already exists")
	// ErrSlugExists is returned when a slug is already taken.
	ErrSlugExists = errors.New("slug already exists")
	// ErrMemberNotFound is returned when a member is not found.
	ErrMemberNotFound = errors.New("member not found")
	// ErrMemberExists is returned when a member already exists.
	ErrMemberExists = errors.New("member already exists in tenant")
	// ErrInvitationNotFound is returned when an invitation is not found.
	ErrInvitationNotFound = errors.New("invitation not found")
	// ErrInvitationExpired is returned when an invitation has expired.
	ErrInvitationExpired = errors.New("invitation expired")
	// ErrInvitationAlreadyAccepted is returned when an invitation was already accepted.
	ErrInvitationAlreadyAccepted = errors.New("invitation already accepted")
	// ErrCannotRemoveOwner is returned when trying to remove the tenant owner.
	ErrCannotRemoveOwner = errors.New("cannot remove tenant owner")
	// ErrCannotChangeOwnerRole is returned when trying to change owner's role.
	ErrCannotChangeOwnerRole = errors.New("cannot change owner role")
	// ErrLimitExceeded is returned when a tenant limit is exceeded.
	ErrLimitExceeded = errors.New("tenant limit exceeded")
)

// Repository defines the interface for tenant data access.
type Repository interface {
	// Tenant operations
	// Create creates a new tenant.
	Create(ctx context.Context, tenant *Tenant) error
	// GetByID retrieves a tenant by ID.
	GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
	// GetBySlug retrieves a tenant by slug.
	GetBySlug(ctx context.Context, slug string) (*Tenant, error)
	// Update updates a tenant.
	Update(ctx context.Context, tenant *Tenant) error
	// Delete soft-deletes a tenant.
	Delete(ctx context.Context, id uuid.UUID) error
	// List retrieves tenants with pagination and filtering.
	List(ctx context.Context, query *ListTenantsQuery) (*ListTenantsResponse, error)
	// ExistsBySlug checks if a slug exists.
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	// GetUserTenants retrieves all tenants a user belongs to.
	GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*Tenant, error)

	// Member operations
	// AddMember adds a member to a tenant.
	AddMember(ctx context.Context, member *TenantMember) error
	// GetMember retrieves a member by tenant and user ID.
	GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*TenantMember, error)
	// UpdateMember updates a member's role.
	UpdateMember(ctx context.Context, member *TenantMember) error
	// RemoveMember removes a member from a tenant.
	RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error
	// ListMembers retrieves members of a tenant with pagination.
	ListMembers(ctx context.Context, tenantID uuid.UUID, query *ListMembersQuery) (*ListMembersResponse, error)
	// CountMembers returns the number of members in a tenant.
	CountMembers(ctx context.Context, tenantID uuid.UUID) (int64, error)
	// IsMember checks if a user is a member of a tenant.
	IsMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error)

	// Invitation operations
	// CreateInvitation creates a new invitation.
	CreateInvitation(ctx context.Context, invitation *TenantInvitation) error
	// GetInvitationByID retrieves an invitation by ID.
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*TenantInvitation, error)
	// GetInvitationByToken retrieves an invitation by token.
	GetInvitationByToken(ctx context.Context, token string) (*TenantInvitation, error)
	// GetPendingInvitationByEmail retrieves a pending invitation by email for a tenant.
	GetPendingInvitationByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*TenantInvitation, error)
	// AcceptInvitation marks an invitation as accepted.
	AcceptInvitation(ctx context.Context, id uuid.UUID) error
	// DeleteInvitation deletes an invitation.
	DeleteInvitation(ctx context.Context, id uuid.UUID) error
	// ListPendingInvitations retrieves pending invitations for a tenant.
	ListPendingInvitations(ctx context.Context, tenantID uuid.UUID) ([]*TenantInvitation, error)
	// CleanupExpiredInvitations removes expired invitations.
	CleanupExpiredInvitations(ctx context.Context) error
}
