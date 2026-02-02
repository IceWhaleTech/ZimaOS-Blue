// Package permission provides page-level permission management.
package permission

import (
	"time"

	"github.com/google/uuid"
)

// Page permission constants
const (
	// PageChat allows access to the chat page
	PageChat = "page.chat"
	// PageHome allows access to the home/dashboard page
	PageHome = "page.home"
	// PageChannels allows access to the channels page
	PageChannels = "page.channels"
	// PageSettings allows access to the settings page
	PageSettings = "page.settings"
	// PageSecurity allows access to the security page
	PageSecurity = "page.security"
	// PageUsers allows access to the user management page
	PageUsers = "page.users"
	// PageProfile allows access to the profile page
	PageProfile = "page.profile"
	// PageProviders allows access to the auth providers page
	PageProviders = "page.providers"
	// PageAutomation allows access to the automation page
	PageAutomation = "page.automation"
	// PagePlugins allows access to the plugins page
	PagePlugins = "page.plugins"
	// PageTools allows access to the tool store page
	PageTools = "page.tools"
	// PageSkills allows access to the skill store page
	PageSkills = "page.skills"
)

// AllPagePermissions returns all available page permissions
func AllPagePermissions() []string {
	return []string{
		PageChat,
		PageHome,
		PageChannels,
		PageSettings,
		PageSecurity,
		PageUsers,
		PageProfile,
		PageProviders,
		PageAutomation,
		PagePlugins,
		PageTools,
		PageSkills,
	}
}

// DefaultGuestPermissions returns the default permissions for guest role
func DefaultGuestPermissions() []string {
	return []string{
		PageHome,
		PageChat,
	}
}

// DefaultUserPermissions returns the default permissions for user role
func DefaultUserPermissions() []string {
	return []string{
		PageChat,
		PageProfile,
		PageHome,
	}
}

// DefaultAdminPermissions returns the default permissions for admin role (all)
func DefaultAdminPermissions() []string {
	return AllPagePermissions()
}

// UserPermission represents a permission assigned to a user
type UserPermission struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Permission string    `json:"permission" db:"permission"`
	GrantedBy  *string   `json:"granted_by,omitempty" db:"granted_by"`
	GrantedAt  time.Time `json:"granted_at" db:"granted_at"`
}

// NewUserPermission creates a new user permission
func NewUserPermission(userID uuid.UUID, permission string, grantedBy *string) *UserPermission {
	return &UserPermission{
		ID:         uuid.New(),
		UserID:     userID,
		Permission: permission,
		GrantedBy:  grantedBy,
		GrantedAt:  time.Now().UTC(),
	}
}

// PermissionInfo provides information about a permission
type PermissionInfo struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// GetPermissionInfo returns information about all available permissions
func GetPermissionInfo() []PermissionInfo {
	return []PermissionInfo{
		{Key: PageChat, Name: "Chat", Description: "Access to chat functionality", Category: "core"},
		{Key: PageHome, Name: "Dashboard", Description: "Access to home dashboard", Category: "core"},
		{Key: PageChannels, Name: "Channels", Description: "Access to channels management", Category: "communication"},
		{Key: PageSettings, Name: "Settings", Description: "Access to system settings", Category: "admin"},
		{Key: PageSecurity, Name: "Security", Description: "Access to security settings", Category: "admin"},
		{Key: PageUsers, Name: "User Management", Description: "Access to user management", Category: "admin"},
		{Key: PageProfile, Name: "Profile", Description: "Access to own profile", Category: "core"},
		{Key: PageProviders, Name: "Auth Providers", Description: "Access to authentication providers", Category: "admin"},
		{Key: PageAutomation, Name: "Automation", Description: "Access to automation features", Category: "advanced"},
		{Key: PagePlugins, Name: "Plugins", Description: "Access to plugin management", Category: "advanced"},
		{Key: PageTools, Name: "Tool Store", Description: "Access to tool store", Category: "advanced"},
		{Key: PageSkills, Name: "Skill Store", Description: "Access to skill store", Category: "advanced"},
	}
}

// SetPermissionsRequest represents a request to set user permissions
type SetPermissionsRequest struct {
	Permissions []string `json:"permissions" validate:"required"`
}

// PermissionsResponse represents a response containing user permissions
type PermissionsResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	Role        string    `json:"role"`
	Permissions []string  `json:"permissions"`
}
