package rbac

import (
	"testing"
)

func TestRBAC_HasPermission(t *testing.T) {
	rbac := New()

	// Define roles
	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("user", []string{"chat", "chat.read", "skills.execute", "skills.list"})
	rbac.DefineRole("readonly", []string{"chat.read", "skills.list"})

	t.Run("admin has all permissions", func(t *testing.T) {
		if !rbac.HasPermission("admin", "chat") {
			t.Error("admin should have 'chat' permission")
		}
		if !rbac.HasPermission("admin", "admin.users") {
			t.Error("admin should have 'admin.users' permission")
		}
		if !rbac.HasPermission("admin", "anything.else") {
			t.Error("admin should have any permission")
		}
	})

	t.Run("user has specific permissions", func(t *testing.T) {
		if !rbac.HasPermission("user", "chat") {
			t.Error("user should have 'chat' permission")
		}
		if !rbac.HasPermission("user", "skills.execute") {
			t.Error("user should have 'skills.execute' permission")
		}
		if rbac.HasPermission("user", "admin.users") {
			t.Error("user should not have 'admin.users' permission")
		}
	})

	t.Run("readonly has limited permissions", func(t *testing.T) {
		if !rbac.HasPermission("readonly", "chat.read") {
			t.Error("readonly should have 'chat.read' permission")
		}
		if rbac.HasPermission("readonly", "chat") {
			t.Error("readonly should not have 'chat' permission")
		}
		if rbac.HasPermission("readonly", "skills.execute") {
			t.Error("readonly should not have 'skills.execute' permission")
		}
	})

	t.Run("undefined role has no permissions", func(t *testing.T) {
		if rbac.HasPermission("unknown", "chat") {
			t.Error("unknown role should not have any permission")
		}
	})
}

func TestRBAC_WildcardPermissions(t *testing.T) {
	rbac := New()

	// Define role with wildcard permissions
	rbac.DefineRole("chat_admin", []string{"chat.*"})
	rbac.DefineRole("skills_admin", []string{"skills.*"})

	t.Run("chat_admin has chat wildcard", func(t *testing.T) {
		if !rbac.HasPermission("chat_admin", "chat.read") {
			t.Error("chat_admin should have 'chat.read' permission")
		}
		if !rbac.HasPermission("chat_admin", "chat.write") {
			t.Error("chat_admin should have 'chat.write' permission")
		}
		if !rbac.HasPermission("chat_admin", "chat.delete") {
			t.Error("chat_admin should have 'chat.delete' permission")
		}
		if rbac.HasPermission("chat_admin", "skills.execute") {
			t.Error("chat_admin should not have 'skills.execute' permission")
		}
	})

	t.Run("skills_admin has skills wildcard", func(t *testing.T) {
		if !rbac.HasPermission("skills_admin", "skills.execute") {
			t.Error("skills_admin should have 'skills.execute' permission")
		}
		if !rbac.HasPermission("skills_admin", "skills.list") {
			t.Error("skills_admin should have 'skills.list' permission")
		}
		if rbac.HasPermission("skills_admin", "chat.read") {
			t.Error("skills_admin should not have 'chat.read' permission")
		}
	})
}

func TestRBAC_RoleInheritance(t *testing.T) {
	rbac := New()

	// Define roles with inheritance
	rbac.DefineRole("readonly", []string{"chat.read", "skills.list"})
	rbac.DefineRole("user", []string{"chat", "skills.execute"})
	rbac.DefineRole("admin", []string{"*"})

	// Set up inheritance: admin inherits from user, user inherits from readonly
	rbac.SetRoleParent("user", "readonly")
	rbac.SetRoleParent("admin", "user")

	t.Run("user inherits from readonly", func(t *testing.T) {
		// User's own permissions
		if !rbac.HasPermission("user", "chat") {
			t.Error("user should have 'chat' permission")
		}
		// Inherited from readonly
		if !rbac.HasPermission("user", "chat.read") {
			t.Error("user should inherit 'chat.read' from readonly")
		}
		if !rbac.HasPermission("user", "skills.list") {
			t.Error("user should inherit 'skills.list' from readonly")
		}
	})

	t.Run("admin inherits from user and readonly", func(t *testing.T) {
		// Admin has wildcard, so everything should work
		if !rbac.HasPermission("admin", "chat") {
			t.Error("admin should have 'chat' permission")
		}
		if !rbac.HasPermission("admin", "chat.read") {
			t.Error("admin should have 'chat.read' permission")
		}
		if !rbac.HasPermission("admin", "admin.special") {
			t.Error("admin should have any permission")
		}
	})
}

func TestRBAC_GetRolePermissions(t *testing.T) {
	rbac := New()

	rbac.DefineRole("user", []string{"chat", "skills.execute"})

	t.Run("get role permissions", func(t *testing.T) {
		perms := rbac.GetRolePermissions("user")
		if len(perms) != 2 {
			t.Errorf("expected 2 permissions, got %d", len(perms))
		}
	})

	t.Run("get undefined role permissions", func(t *testing.T) {
		perms := rbac.GetRolePermissions("unknown")
		if len(perms) != 0 {
			t.Errorf("expected 0 permissions, got %d", len(perms))
		}
	})
}

func TestRBAC_ListRoles(t *testing.T) {
	rbac := New()

	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("user", []string{"chat"})
	rbac.DefineRole("readonly", []string{"chat.read"})

	roles := rbac.ListRoles()
	if len(roles) != 3 {
		t.Errorf("expected 3 roles, got %d", len(roles))
	}
}

func TestRBAC_RemoveRole(t *testing.T) {
	rbac := New()

	rbac.DefineRole("temp", []string{"chat"})

	if !rbac.HasPermission("temp", "chat") {
		t.Error("temp should have 'chat' permission")
	}

	rbac.RemoveRole("temp")

	if rbac.HasPermission("temp", "chat") {
		t.Error("temp should not have any permission after removal")
	}
}
