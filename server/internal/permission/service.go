package permission

import (
	"context"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

// Service handles permission business logic
type Service struct {
	repo     *Repository
	userRepo user.Repository
}

// NewService creates a new permission service
func NewService(repo *Repository, userRepo user.Repository) *Service {
	return &Service{
		repo:     repo,
		userRepo: userRepo,
	}
}

// GetEffectivePermissions returns all effective permissions for a user
// This combines role-based defaults with custom permissions
func (s *Service) GetEffectivePermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	// Guard against nil userRepo (typed nil interface can still dispatch and panic)
	if s.userRepo == nil {
		return s.GetDefaultPermissionsForRole("guest"), nil
	}

	// Get user to check role
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get custom permissions from database
	if s.repo == nil {
		return s.GetDefaultPermissionsForRole(string(u.Role)), nil
	}
	customPerms, err := s.repo.GetUserPermissions(ctx, userID)
	if err != nil {
		// If error fetching permissions (e.g., table doesn't exist in old versions),
		// fallback to role defaults for backward compatibility
		return s.GetDefaultPermissionsForRole(string(u.Role)), nil
	}

	// Migration logic: If admin has no permissions in DB, populate them
	if u.Role == user.RoleAdmin && len(customPerms) == 0 {
		// Automatically initialize admin permissions for migration
		allPerms := AllPagePermissions()
		systemGranter := "system-migration"
		_ = s.repo.SetUserPermissions(ctx, userID, allPerms, &systemGranter)
		return allPerms, nil
	}

	// Admin gets all permissions
	if u.Role == user.RoleAdmin {
		return AllPagePermissions(), nil
	}

	// If user has custom permissions, use those
	if len(customPerms) > 0 {
		return customPerms, nil
	}

	// Migration logic: If non-admin user has no permissions, populate defaults
	defaultPerms := s.GetDefaultPermissionsForRole(string(u.Role))
	if len(defaultPerms) > 0 {
		systemGranter := "system-migration"
		_ = s.repo.SetUserPermissions(ctx, userID, defaultPerms, &systemGranter)
	}

	// Return role defaults (for users created before permission system)
	return defaultPerms, nil
}

// GetDefaultPermissionsForRole returns the default permissions for a role
func (s *Service) GetDefaultPermissionsForRole(role string) []string {
	switch user.Role(role) {
	case user.RoleAdmin:
		return DefaultAdminPermissions()
	case user.RoleUser:
		return DefaultUserPermissions()
	case user.RoleGuest:
		return DefaultGuestPermissions()
	default:
		return DefaultGuestPermissions()
	}
}

// HasPermission checks if a user has a specific permission
func (s *Service) HasPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	perms, err := s.GetEffectivePermissions(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, p := range perms {
		if p == permission {
			return true, nil
		}
	}

	return false, nil
}

// SetUserPermissions sets custom permissions for a user
func (s *Service) SetUserPermissions(ctx context.Context, userID uuid.UUID, permissions []string, grantedBy *string) error {
	// Validate permissions
	validPerms := make(map[string]bool)
	for _, p := range AllPagePermissions() {
		validPerms[p] = true
	}

	for _, p := range permissions {
		if !validPerms[p] {
			continue // Skip invalid permissions
		}
	}

	return s.repo.SetUserPermissions(ctx, userID, permissions, grantedBy)
}

// AddPermission adds a single permission to a user
func (s *Service) AddPermission(ctx context.Context, userID uuid.UUID, permission string, grantedBy *string) error {
	return s.repo.AddPermission(ctx, userID, permission, grantedBy)
}

// RemovePermission removes a single permission from a user
func (s *Service) RemovePermission(ctx context.Context, userID uuid.UUID, permission string) error {
	return s.repo.RemovePermission(ctx, userID, permission)
}

// GetAvailablePermissions returns all available permissions with info
func (s *Service) GetAvailablePermissions() []PermissionInfo {
	return GetPermissionInfo()
}

// InitializeUserPermissions sets default permissions for a new user based on role
func (s *Service) InitializeUserPermissions(ctx context.Context, userID uuid.UUID, role string, grantedBy *string) error {
	// Admin doesn't need explicit permissions (they get all by default)
	if user.Role(role) == user.RoleAdmin {
		return nil
	}

	defaultPerms := s.GetDefaultPermissionsForRole(role)
	return s.repo.SetUserPermissions(ctx, userID, defaultPerms, grantedBy)
}

// DeleteUserPermissions removes all permissions for a user (used when deleting user)
func (s *Service) DeleteUserPermissions(ctx context.Context, userID uuid.UUID) error {
	return s.repo.DeleteUserPermissions(ctx, userID)
}
