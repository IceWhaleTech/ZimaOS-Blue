package rbac

import (
	"strings"
	"sync"
)

// Role represents a role with permissions
type Role struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	Parent      string   `json:"parent,omitempty"` // Parent role for inheritance
}

// RBAC handles role-based access control
type RBAC struct {
	roles map[string]*Role
	mu    sync.RWMutex
}

// New creates a new RBAC instance
func New() *RBAC {
	return &RBAC{
		roles: make(map[string]*Role),
	}
}

// DefineRole defines a new role with the given permissions
func (r *RBAC) DefineRole(name string, permissions []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.roles[name] = &Role{
		Name:        name,
		Permissions: permissions,
	}
}

// SetRoleParent sets the parent role for inheritance
func (r *RBAC) SetRoleParent(roleName, parentName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if role, exists := r.roles[roleName]; exists {
		role.Parent = parentName
	}
}

// RemoveRole removes a role
func (r *RBAC) RemoveRole(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.roles, name)
}

// HasPermission checks if a role has a specific permission
func (r *RBAC) HasPermission(roleName, permission string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.hasPermissionInternal(roleName, permission, make(map[string]bool))
}

// hasPermissionInternal checks permission with cycle detection
func (r *RBAC) hasPermissionInternal(roleName, permission string, visited map[string]bool) bool {
	// Prevent infinite loops in case of circular inheritance
	if visited[roleName] {
		return false
	}
	visited[roleName] = true

	role, exists := r.roles[roleName]
	if !exists {
		return false
	}

	// Check direct permissions
	for _, perm := range role.Permissions {
		if matchPermission(perm, permission) {
			return true
		}
	}

	// Check inherited permissions from parent
	if role.Parent != "" {
		if r.hasPermissionInternal(role.Parent, permission, visited) {
			return true
		}
	}

	return false
}

// matchPermission checks if a permission pattern matches the requested permission
func matchPermission(pattern, permission string) bool {
	// Exact match
	if pattern == permission {
		return true
	}

	// Wildcard match for all permissions
	if pattern == "*" {
		return true
	}

	// Wildcard match for prefix (e.g., "chat.*" matches "chat.read")
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		if strings.HasPrefix(permission, prefix+".") {
			return true
		}
	}

	// Wildcard match for suffix (e.g., "*.read" matches "chat.read")
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		if strings.HasSuffix(permission, "."+suffix) {
			return true
		}
	}

	return false
}

// GetRolePermissions returns the permissions for a role
func (r *RBAC) GetRolePermissions(roleName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	role, exists := r.roles[roleName]
	if !exists {
		return []string{}
	}

	return role.Permissions
}

// GetRole returns a role by name
func (r *RBAC) GetRole(name string) *Role {
	r.mu.RLock()
	defer r.mu.RUnlock()

	role, exists := r.roles[name]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	return &Role{
		Name:        role.Name,
		Permissions: append([]string{}, role.Permissions...),
		Parent:      role.Parent,
	}
}

// ListRoles returns all defined roles
func (r *RBAC) ListRoles() []*Role {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roles := make([]*Role, 0, len(r.roles))
	for _, role := range r.roles {
		roles = append(roles, &Role{
			Name:        role.Name,
			Permissions: append([]string{}, role.Permissions...),
			Parent:      role.Parent,
		})
	}

	return roles
}

// GetAllPermissions returns all permissions for a role including inherited ones
func (r *RBAC) GetAllPermissions(roleName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	perms := make(map[string]bool)
	r.collectPermissions(roleName, perms, make(map[string]bool))

	result := make([]string, 0, len(perms))
	for perm := range perms {
		result = append(result, perm)
	}

	return result
}

// collectPermissions recursively collects permissions from a role and its parents
func (r *RBAC) collectPermissions(roleName string, perms map[string]bool, visited map[string]bool) {
	if visited[roleName] {
		return
	}
	visited[roleName] = true

	role, exists := r.roles[roleName]
	if !exists {
		return
	}

	for _, perm := range role.Permissions {
		perms[perm] = true
	}

	if role.Parent != "" {
		r.collectPermissions(role.Parent, perms, visited)
	}
}
