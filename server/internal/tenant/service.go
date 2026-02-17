package tenant

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// Service provides tenant management operations.
type Service struct {
	repo              Repository
	invitationExpiry  time.Duration
	defaultLimits     *TenantLimits
	defaultSettings   *TenantSettings
}

// ServiceConfig contains configuration for the tenant service.
type ServiceConfig struct {
	InvitationExpiry time.Duration
	DefaultLimits    *TenantLimits
	DefaultSettings  *TenantSettings
}

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		InvitationExpiry: 7 * 24 * time.Hour, // 7 days
		DefaultLimits:    DefaultTenantLimits(),
		DefaultSettings:  DefaultTenantSettings(),
	}
}

// NewService creates a new tenant service.
func NewService(repo Repository, config *ServiceConfig) *Service {
	if config == nil {
		config = DefaultServiceConfig()
	}
	return &Service{
		repo:             repo,
		invitationExpiry: config.InvitationExpiry,
		defaultLimits:    config.DefaultLimits,
		defaultSettings:  config.DefaultSettings,
	}
}

// Create creates a new tenant and adds the creator as owner.
func (s *Service) Create(ctx context.Context, req *CreateTenantRequest, ownerID uuid.UUID) (*Tenant, error) {
	// Check if slug exists
	exists, err := s.repo.ExistsBySlug(ctx, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("failed to check slug: %w", err)
	}
	if exists {
		return nil, ErrSlugExists
	}

	// Create tenant
	tenant := NewTenant(req.Name, req.Slug, ownerID)
	tenant.Description = req.Description

	// Set default limits
	limitsJSON, err := json.Marshal(s.defaultLimits)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal limits: %w", err)
	}
	limitsStr := string(limitsJSON)
	tenant.Limits = &limitsStr

	// Set default settings
	settingsJSON, err := json.Marshal(s.defaultSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal settings: %w", err)
	}
	settingsStr := string(settingsJSON)
	tenant.Settings = &settingsStr

	if err := s.repo.Create(ctx, tenant); err != nil {
		return nil, err
	}

	// Add owner as member
	member := NewTenantMember(tenant.ID, ownerID, MemberRoleOwner, nil)
	if err := s.repo.AddMember(ctx, member); err != nil {
		// Rollback tenant creation
		_ = s.repo.Delete(ctx, tenant.ID)
		return nil, fmt.Errorf("failed to add owner as member: %w", err)
	}

	return tenant, nil
}

// GetByID retrieves a tenant by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	return s.repo.GetByID(ctx, id)
}

// GetBySlug retrieves a tenant by slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// Update updates a tenant.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *UpdateTenantRequest) (*Tenant, error) {
	tenant, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.Description != nil {
		tenant.Description = req.Description
	}
	if req.Status != nil {
		tenant.Status = *req.Status
	}

	if err := s.repo.Update(ctx, tenant); err != nil {
		return nil, err
	}

	return tenant, nil
}

// Delete soft-deletes a tenant.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List retrieves tenants with pagination and filtering.
func (s *Service) List(ctx context.Context, query *ListTenantsQuery) (*ListTenantsResponse, error) {
	return s.repo.List(ctx, query)
}

// GetUserTenants retrieves all tenants a user belongs to.
func (s *Service) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*Tenant, error) {
	return s.repo.GetUserTenants(ctx, userID)
}

// GetMember retrieves a member by tenant and user ID.
func (s *Service) GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*TenantMember, error) {
	return s.repo.GetMember(ctx, tenantID, userID)
}

// ListMembers retrieves members of a tenant with pagination.
func (s *Service) ListMembers(ctx context.Context, tenantID uuid.UUID, query *ListMembersQuery) (*ListMembersResponse, error) {
	return s.repo.ListMembers(ctx, tenantID, query)
}

// UpdateMemberRole updates a member's role.
func (s *Service) UpdateMemberRole(ctx context.Context, tenantID, userID uuid.UUID, role MemberRole) error {
	// Get tenant to check owner
	tenant, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}

	// Cannot change owner's role
	if tenant.OwnerID == userID && role != MemberRoleOwner {
		return ErrCannotChangeOwnerRole
	}

	member, err := s.repo.GetMember(ctx, tenantID, userID)
	if err != nil {
		return err
	}

	member.Role = role
	return s.repo.UpdateMember(ctx, member)
}

// RemoveMember removes a member from a tenant.
func (s *Service) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	// Get tenant to check owner
	tenant, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}

	// Cannot remove owner
	if tenant.OwnerID == userID {
		return ErrCannotRemoveOwner
	}

	return s.repo.RemoveMember(ctx, tenantID, userID)
}

// InviteMember creates an invitation to join a tenant.
func (s *Service) InviteMember(ctx context.Context, tenantID uuid.UUID, req *InviteMemberRequest, invitedBy uuid.UUID) (*TenantInvitation, error) {
	// Check if tenant exists
	tenant, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Check member limit
	limits, err := s.GetTenantLimits(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	memberCount, err := s.repo.CountMembers(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if int(memberCount) >= limits.MaxUsers {
		return nil, ErrLimitExceeded
	}

	// Check if there's already a pending invitation
	existing, err := s.repo.GetPendingInvitationByEmail(ctx, tenantID, req.Email)
	if err == nil && existing != nil {
		return existing, nil // Return existing invitation
	}

	// Generate token
	token, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	invitation := NewTenantInvitation(
		tenant.ID,
		req.Email,
		req.Role,
		invitedBy,
		token,
		timeutil.NowTime().Add(s.invitationExpiry),
	)

	if err := s.repo.CreateInvitation(ctx, invitation); err != nil {
		return nil, err
	}

	return invitation, nil
}

// AcceptInvitation accepts an invitation and adds the user to the tenant.
func (s *Service) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (*Tenant, error) {
	invitation, err := s.repo.GetInvitationByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if invitation.AcceptedAt != nil {
		return nil, ErrInvitationAlreadyAccepted
	}

	if timeutil.NowNano() > invitation.ExpiresAt.UnixNano() {
		return nil, ErrInvitationExpired
	}

	// Check if already a member
	isMember, err := s.repo.IsMember(ctx, invitation.TenantID, userID)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, ErrMemberExists
	}

	// Add member
	member := NewTenantMember(invitation.TenantID, userID, invitation.Role, &invitation.InvitedBy)
	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	// Mark invitation as accepted
	if err := s.repo.AcceptInvitation(ctx, invitation.ID); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, invitation.TenantID)
}

// CancelInvitation cancels a pending invitation.
func (s *Service) CancelInvitation(ctx context.Context, invitationID uuid.UUID) error {
	return s.repo.DeleteInvitation(ctx, invitationID)
}

// ListPendingInvitations retrieves pending invitations for a tenant.
func (s *Service) ListPendingInvitations(ctx context.Context, tenantID uuid.UUID) ([]*TenantInvitation, error) {
	return s.repo.ListPendingInvitations(ctx, tenantID)
}

// IsMember checks if a user is a member of a tenant.
func (s *Service) IsMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	return s.repo.IsMember(ctx, tenantID, userID)
}

// HasPermission checks if a user has a specific role or higher in a tenant.
func (s *Service) HasPermission(ctx context.Context, tenantID, userID uuid.UUID, requiredRole MemberRole) (bool, error) {
	member, err := s.repo.GetMember(ctx, tenantID, userID)
	if err != nil {
		if err == ErrMemberNotFound {
			return false, nil
		}
		return false, err
	}

	return hasRolePermission(member.Role, requiredRole), nil
}

// GetTenantLimits retrieves the limits for a tenant.
func (s *Service) GetTenantLimits(ctx context.Context, tenantID uuid.UUID) (*TenantLimits, error) {
	tenant, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if tenant.Limits == nil {
		return s.defaultLimits, nil
	}

	var limits TenantLimits
	if err := json.Unmarshal([]byte(*tenant.Limits), &limits); err != nil {
		return nil, fmt.Errorf("failed to unmarshal limits: %w", err)
	}

	return &limits, nil
}

// UpdateTenantLimits updates the limits for a tenant.
func (s *Service) UpdateTenantLimits(ctx context.Context, tenantID uuid.UUID, limits *TenantLimits) error {
	tenant, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}

	limitsJSON, err := json.Marshal(limits)
	if err != nil {
		return fmt.Errorf("failed to marshal limits: %w", err)
	}
	limitsStr := string(limitsJSON)
	tenant.Limits = &limitsStr

	return s.repo.Update(ctx, tenant)
}

// GetTenantSettings retrieves the settings for a tenant.
func (s *Service) GetTenantSettings(ctx context.Context, tenantID uuid.UUID) (*TenantSettings, error) {
	tenant, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if tenant.Settings == nil {
		return s.defaultSettings, nil
	}

	var settings TenantSettings
	if err := json.Unmarshal([]byte(*tenant.Settings), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &settings, nil
}

// UpdateTenantSettings updates the settings for a tenant.
func (s *Service) UpdateTenantSettings(ctx context.Context, tenantID uuid.UUID, settings *TenantSettings) error {
	tenant, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	settingsStr := string(settingsJSON)
	tenant.Settings = &settingsStr

	return s.repo.Update(ctx, tenant)
}

// TransferOwnership transfers tenant ownership to another member.
func (s *Service) TransferOwnership(ctx context.Context, tenantID, currentOwnerID, newOwnerID uuid.UUID) error {
	tenant, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}

	// Verify current owner
	if tenant.OwnerID != currentOwnerID {
		return fmt.Errorf("only the current owner can transfer ownership")
	}

	// Verify new owner is a member
	newOwnerMember, err := s.repo.GetMember(ctx, tenantID, newOwnerID)
	if err != nil {
		return err
	}

	// Update new owner's role
	newOwnerMember.Role = MemberRoleOwner
	if err := s.repo.UpdateMember(ctx, newOwnerMember); err != nil {
		return err
	}

	// Update old owner's role to admin
	oldOwnerMember, err := s.repo.GetMember(ctx, tenantID, currentOwnerID)
	if err != nil {
		return err
	}
	oldOwnerMember.Role = MemberRoleAdmin
	if err := s.repo.UpdateMember(ctx, oldOwnerMember); err != nil {
		return err
	}

	// Update tenant owner
	tenant.OwnerID = newOwnerID
	return s.repo.Update(ctx, tenant)
}

// CleanupExpiredInvitations removes expired invitations.
func (s *Service) CleanupExpiredInvitations(ctx context.Context) error {
	return s.repo.CleanupExpiredInvitations(ctx)
}

// hasRolePermission checks if a role has permission for a required role.
func hasRolePermission(role, requiredRole MemberRole) bool {
	roleHierarchy := map[MemberRole]int{
		MemberRoleOwner:  3,
		MemberRoleAdmin:  2,
		MemberRoleMember: 1,
	}

	return roleHierarchy[role] >= roleHierarchy[requiredRole]
}

// generateToken generates a random token.
func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
