package rbac

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestUserRoleService_AssignRole(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("user", []string{"read", "write"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	t.Run("assign role successfully", func(t *testing.T) {
		assignment, err := svc.AssignRole(context.Background(), &AssignRoleRequest{
			UserID:     "user-123",
			RoleName:   "admin",
			AssignedBy: "admin-user",
		})
		if err != nil {
			t.Fatalf("failed to assign role: %v", err)
		}

		if assignment.UserID != "user-123" {
			t.Errorf("expected user ID 'user-123', got %s", assignment.UserID)
		}
		if assignment.RoleName != "admin" {
			t.Errorf("expected role 'admin', got %s", assignment.RoleName)
		}
		if assignment.AssignedBy != "admin-user" {
			t.Errorf("expected assigned by 'admin-user', got %s", assignment.AssignedBy)
		}
	})

	t.Run("assign non-existent role should fail", func(t *testing.T) {
		_, err := svc.AssignRole(context.Background(), &AssignRoleRequest{
			UserID:   "user-123",
			RoleName: "non-existent",
		})
		if err != ErrRoleNotFound {
			t.Errorf("expected ErrRoleNotFound, got %v", err)
		}
	})

	t.Run("reassign revoked role", func(t *testing.T) {
		// Assign role
		_, err := svc.AssignRole(context.Background(), &AssignRoleRequest{
			UserID:   "user-456",
			RoleName: "user",
		})
		if err != nil {
			t.Fatalf("failed to assign role: %v", err)
		}

		// Revoke role
		err = svc.RevokeRole(context.Background(), &RevokeRoleRequest{
			UserID:   "user-456",
			RoleName: "user",
		})
		if err != nil {
			t.Fatalf("failed to revoke role: %v", err)
		}

		// Reassign role
		assignment, err := svc.AssignRole(context.Background(), &AssignRoleRequest{
			UserID:   "user-456",
			RoleName: "user",
		})
		if err != nil {
			t.Fatalf("failed to reassign role: %v", err)
		}

		if assignment.Revoked {
			t.Error("reassigned role should not be revoked")
		}
	})
}

func TestUserRoleService_RevokeRole(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	t.Run("revoke role successfully", func(t *testing.T) {
		// Assign role first
		_, err := svc.AssignRole(context.Background(), &AssignRoleRequest{
			UserID:   "user-123",
			RoleName: "admin",
		})
		if err != nil {
			t.Fatalf("failed to assign role: %v", err)
		}

		// Revoke role
		err = svc.RevokeRole(context.Background(), &RevokeRoleRequest{
			UserID:    "user-123",
			RoleName:  "admin",
			RevokedBy: "admin-user",
		})
		if err != nil {
			t.Fatalf("failed to revoke role: %v", err)
		}

		// Check role is revoked
		hasRole, err := svc.HasRole(context.Background(), "user-123", "admin")
		if err != nil {
			t.Fatalf("failed to check role: %v", err)
		}
		if hasRole {
			t.Error("user should not have role after revocation")
		}
	})

	t.Run("revoke non-existent assignment should fail", func(t *testing.T) {
		err := svc.RevokeRole(context.Background(), &RevokeRoleRequest{
			UserID:   "non-existent",
			RoleName: "admin",
		})
		if err != ErrAssignmentNotFound {
			t.Errorf("expected ErrAssignmentNotFound, got %v", err)
		}
	})
}

func TestUserRoleService_GetUserRoles(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("user", []string{"read"})
	rbac.DefineRole("editor", []string{"write"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	// Assign multiple roles
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "admin",
	})
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "user",
	})
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "editor",
	})

	// Revoke one role
	svc.RevokeRole(context.Background(), &RevokeRoleRequest{
		UserID:   "user-123",
		RoleName: "editor",
	})

	t.Run("get active roles", func(t *testing.T) {
		roles, err := svc.GetUserRoles(context.Background(), "user-123")
		if err != nil {
			t.Fatalf("failed to get roles: %v", err)
		}

		if len(roles) != 2 {
			t.Errorf("expected 2 active roles, got %d", len(roles))
		}
	})
}

func TestUserRoleService_HasRole(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "admin",
	})

	t.Run("has assigned role", func(t *testing.T) {
		hasRole, err := svc.HasRole(context.Background(), "user-123", "admin")
		if err != nil {
			t.Fatalf("failed to check role: %v", err)
		}
		if !hasRole {
			t.Error("user should have admin role")
		}
	})

	t.Run("does not have unassigned role", func(t *testing.T) {
		hasRole, err := svc.HasRole(context.Background(), "user-123", "user")
		if err != nil {
			t.Fatalf("failed to check role: %v", err)
		}
		if hasRole {
			t.Error("user should not have user role")
		}
	})
}

func TestUserRoleService_HasPermission(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("user", []string{"read", "write"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "user",
	})

	t.Run("has permission through role", func(t *testing.T) {
		hasPerm, err := svc.HasPermission(context.Background(), "user-123", "read")
		if err != nil {
			t.Fatalf("failed to check permission: %v", err)
		}
		if !hasPerm {
			t.Error("user should have read permission")
		}
	})

	t.Run("does not have ungranted permission", func(t *testing.T) {
		hasPerm, err := svc.HasPermission(context.Background(), "user-123", "delete")
		if err != nil {
			t.Fatalf("failed to check permission: %v", err)
		}
		if hasPerm {
			t.Error("user should not have delete permission")
		}
	})
}

func TestUserRoleService_GetUserPermissions(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"admin.*"})
	rbac.DefineRole("user", []string{"read", "write"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "admin",
	})
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "user",
	})

	t.Run("get all permissions", func(t *testing.T) {
		perms, err := svc.GetUserPermissions(context.Background(), "user-123")
		if err != nil {
			t.Fatalf("failed to get permissions: %v", err)
		}

		// Should have permissions from both roles
		if len(perms) < 3 {
			t.Errorf("expected at least 3 permissions, got %d", len(perms))
		}
	})
}

func TestUserRoleService_ExpiredAssignments(t *testing.T) {
	rbac := New()
	rbac.DefineRole("temp", []string{"read"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	// Assign role with past expiration
	expiredTime := time.Now().Add(-time.Hour)
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:    "user-123",
		RoleName:  "temp",
		ExpiresAt: &expiredTime,
	})

	t.Run("expired role not returned", func(t *testing.T) {
		roles, err := svc.GetUserRoles(context.Background(), "user-123")
		if err != nil {
			t.Fatalf("failed to get roles: %v", err)
		}

		if len(roles) != 0 {
			t.Errorf("expected 0 active roles, got %d", len(roles))
		}
	})

	t.Run("cleanup expired assignments", func(t *testing.T) {
		count, err := svc.CleanupExpiredAssignments(context.Background())
		if err != nil {
			t.Fatalf("failed to cleanup: %v", err)
		}

		if count != 1 {
			t.Errorf("expected 1 assignment cleaned up, got %d", count)
		}
	})
}

func TestUserRoleService_BulkOperations(t *testing.T) {
	rbac := New()
	rbac.DefineRole("user", []string{"read"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	userIDs := []string{"user-1", "user-2", "user-3"}

	t.Run("bulk assign role", func(t *testing.T) {
		err := svc.BulkAssignRole(context.Background(), userIDs, "user", "admin")
		if err != nil {
			t.Fatalf("failed to bulk assign: %v", err)
		}

		// Verify all users have the role
		for _, userID := range userIDs {
			hasRole, err := svc.HasRole(context.Background(), userID, "user")
			if err != nil {
				t.Fatalf("failed to check role: %v", err)
			}
			if !hasRole {
				t.Errorf("user %s should have role", userID)
			}
		}
	})

	t.Run("bulk revoke role", func(t *testing.T) {
		err := svc.BulkRevokeRole(context.Background(), userIDs[:2], "user", "admin")
		if err != nil {
			t.Fatalf("failed to bulk revoke: %v", err)
		}

		// First two users should not have role
		for _, userID := range userIDs[:2] {
			hasRole, err := svc.HasRole(context.Background(), userID, "user")
			if err != nil {
				t.Fatalf("failed to check role: %v", err)
			}
			if hasRole {
				t.Errorf("user %s should not have role", userID)
			}
		}

		// Third user should still have role
		hasRole, err := svc.HasRole(context.Background(), userIDs[2], "user")
		if err != nil {
			t.Fatalf("failed to check role: %v", err)
		}
		if !hasRole {
			t.Error("user-3 should still have role")
		}
	})
}

func TestUserRoleService_GetAssignmentHistory(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("user", []string{"read"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	// Create some history
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "admin",
	})
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-123",
		RoleName: "user",
	})
	svc.RevokeRole(context.Background(), &RevokeRoleRequest{
		UserID:   "user-123",
		RoleName: "admin",
	})

	t.Run("get full history", func(t *testing.T) {
		history, err := svc.GetAssignmentHistory(context.Background(), "user-123")
		if err != nil {
			t.Fatalf("failed to get history: %v", err)
		}

		if len(history) != 2 {
			t.Errorf("expected 2 history entries, got %d", len(history))
		}

		// Check that revoked assignment is in history
		foundRevoked := false
		for _, h := range history {
			if h.RoleName == "admin" && h.Revoked {
				foundRevoked = true
				break
			}
		}
		if !foundRevoked {
			t.Error("revoked admin assignment should be in history")
		}
	})
}

func TestUserRoleService_GetRoleUsers(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})

	dbPath := filepath.Join(t.TempDir(), "user_roles_test.db")
	svc, err := NewUserRoleService(dbPath, rbac)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	// Assign role to multiple users
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-1",
		RoleName: "admin",
	})
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-2",
		RoleName: "admin",
	})
	svc.AssignRole(context.Background(), &AssignRoleRequest{
		UserID:   "user-3",
		RoleName: "admin",
	})

	// Revoke from one user
	svc.RevokeRole(context.Background(), &RevokeRoleRequest{
		UserID:   "user-2",
		RoleName: "admin",
	})

	t.Run("get users with role", func(t *testing.T) {
		users, err := svc.GetRoleUsers(context.Background(), "admin")
		if err != nil {
			t.Fatalf("failed to get role users: %v", err)
		}

		if len(users) != 2 {
			t.Errorf("expected 2 users with admin role, got %d", len(users))
		}
	})
}
