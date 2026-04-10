package rbac

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkRBAC benchmarks RBAC operations
func BenchmarkRBAC(b *testing.B) {
	b.Run("DefineRole", func(b *testing.B) {
		rbac := New()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			rbac.DefineRole(fmt.Sprintf("role-%d", i), []string{"read", "write", "delete"})
		}
	})

	b.Run("HasPermission", func(b *testing.B) {
		rbac := New()
		// Pre-define roles
		for i := 0; i < 100; i++ {
			rbac.DefineRole(fmt.Sprintf("role-%d", i), []string{
				fmt.Sprintf("resource-%d.read", i),
				fmt.Sprintf("resource-%d.write", i),
				fmt.Sprintf("resource-%d.delete", i),
			})
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rbac.HasPermission(fmt.Sprintf("role-%d", i%100), fmt.Sprintf("resource-%d.read", i%100))
		}
	})

	b.Run("HasPermissionWildcard", func(b *testing.B) {
		rbac := New()
		rbac.DefineRole("admin", []string{"*"})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rbac.HasPermission("admin", fmt.Sprintf("resource.action.%d", i))
		}
	})

	b.Run("HasPermissionPrefixWildcard", func(b *testing.B) {
		rbac := New()
		rbac.DefineRole("user", []string{"chat.*", "profile.*", "settings.*"})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rbac.HasPermission("user", fmt.Sprintf("chat.action%d", i))
		}
	})

	b.Run("GetAllPermissions", func(b *testing.B) {
		rbac := New()
		// Create role hierarchy
		rbac.DefineRole("base", []string{"read"})
		rbac.DefineRole("user", []string{"write"})
		rbac.SetRoleParent("user", "base")
		rbac.DefineRole("admin", []string{"delete", "admin.*"})
		rbac.SetRoleParent("admin", "user")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rbac.GetAllPermissions("admin")
		}
	})

	b.Run("ListRoles", func(b *testing.B) {
		rbac := New()
		// Pre-define roles
		for i := 0; i < 50; i++ {
			rbac.DefineRole(fmt.Sprintf("role-%d", i), []string{"read", "write"})
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rbac.ListRoles()
		}
	})
}

// BenchmarkUserRoleService benchmarks user role service operations
func BenchmarkUserRoleService(b *testing.B) {
	b.Run("AssignRole", func(b *testing.B) {
		rbac := New()
		rbac.DefineRole("user", []string{"read"})

		dbPath := filepath.Join(b.TempDir(), "bench_assign.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			b.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = svc.AssignRole(ctx, &AssignRoleRequest{
				UserID:   fmt.Sprintf("user-%d", i),
				RoleName: "user",
			})
		}
	})

	b.Run("HasRole", func(b *testing.B) {
		rbac := New()
		rbac.DefineRole("user", []string{"read"})

		dbPath := filepath.Join(b.TempDir(), "bench_hasrole.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			b.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		// Pre-assign roles
		for i := 0; i < 100; i++ {
			svc.AssignRole(ctx, &AssignRoleRequest{
				UserID:   fmt.Sprintf("user-%d", i),
				RoleName: "user",
			})
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = svc.HasRole(ctx, fmt.Sprintf("user-%d", i%100), "user")
		}
	})

	b.Run("HasPermission", func(b *testing.B) {
		rbac := New()
		rbac.DefineRole("user", []string{"read", "write", "chat.*"})

		dbPath := filepath.Join(b.TempDir(), "bench_hasperm.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			b.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		// Pre-assign roles
		for i := 0; i < 100; i++ {
			svc.AssignRole(ctx, &AssignRoleRequest{
				UserID:   fmt.Sprintf("user-%d", i),
				RoleName: "user",
			})
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = svc.HasPermission(ctx, fmt.Sprintf("user-%d", i%100), "read")
		}
	})

	b.Run("GetUserRoles", func(b *testing.B) {
		rbac := New()
		rbac.DefineRole("admin", []string{"*"})
		rbac.DefineRole("user", []string{"read"})
		rbac.DefineRole("editor", []string{"write"})

		dbPath := filepath.Join(b.TempDir(), "bench_getroles.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			b.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		// Pre-assign multiple roles to users
		for i := 0; i < 100; i++ {
			userID := fmt.Sprintf("user-%d", i)
			svc.AssignRole(ctx, &AssignRoleRequest{UserID: userID, RoleName: "admin"})
			svc.AssignRole(ctx, &AssignRoleRequest{UserID: userID, RoleName: "user"})
			svc.AssignRole(ctx, &AssignRoleRequest{UserID: userID, RoleName: "editor"})
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = svc.GetUserRoles(ctx, fmt.Sprintf("user-%d", i%100))
		}
	})

	b.Run("BulkAssignRole", func(b *testing.B) {
		rbac := New()
		rbac.DefineRole("user", []string{"read"})

		dbPath := filepath.Join(b.TempDir(), "bench_bulk.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			b.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		userIDs := make([]string, 10)
		for i := 0; i < 10; i++ {
			userIDs[i] = fmt.Sprintf("bulk-user-%d", i)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Use different role names to avoid conflicts
			rbac.DefineRole(fmt.Sprintf("role-%d", i), []string{"read"})
			_ = svc.BulkAssignRole(ctx, userIDs, fmt.Sprintf("role-%d", i), "admin")
		}
	})
}

// RBACPerformanceBaseline captures baseline performance metrics for RBAC
type RBACPerformanceBaseline struct {
	DefineRoleNs       int64
	HasPermissionNs    int64
	GetAllPermsNs      int64
	AssignRoleNs       int64
	HasRoleNs          int64
	HasPermissionDBNs  int64
	GetUserRolesNs     int64
}

// GetRBACPerformanceBaseline returns expected baseline performance
func GetRBACPerformanceBaseline() *RBACPerformanceBaseline {
	return &RBACPerformanceBaseline{
		DefineRoleNs:       1000,      // 1µs
		HasPermissionNs:    500,       // 500ns
		GetAllPermsNs:      2000,      // 2µs
		AssignRoleNs:       10000000,  // 10ms (database write with WAL)
		HasRoleNs:          100000,    // 100µs (database query)
		HasPermissionDBNs:  200000,    // 200µs (database query + RBAC check)
		GetUserRolesNs:     150000,    // 150µs (database query)
	}
}

// TestRBACPerformanceRegression tests for RBAC performance regressions
func TestRBACPerformanceRegression(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("disabled in CI: nanosecond-scale RBAC performance thresholds are host-load sensitive")
	}

	baseline := GetRBACPerformanceBaseline()
	tolerance := 2.0 // Allow 2x baseline

	t.Run("DefineRole", func(t *testing.T) {
		rbac := New()
		iterations := 10000

		start := time.Now()
		for i := 0; i < iterations; i++ {
			rbac.DefineRole(fmt.Sprintf("role-%d", i), []string{"read", "write", "delete"})
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.DefineRoleNs)*tolerance) {
			t.Errorf("DefineRole performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.DefineRoleNs, tolerance)
		}
		t.Logf("DefineRole: %dns/op (baseline: %dns)", avgNs, baseline.DefineRoleNs)
	})

	t.Run("HasPermission", func(t *testing.T) {
		rbac := New()
		// Pre-define roles
		for i := 0; i < 100; i++ {
			rbac.DefineRole(fmt.Sprintf("role-%d", i), []string{
				fmt.Sprintf("resource-%d.read", i),
				fmt.Sprintf("resource-%d.write", i),
				fmt.Sprintf("resource-%d.delete", i),
			})
		}

		iterations := 100000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_ = rbac.HasPermission(fmt.Sprintf("role-%d", i%100), fmt.Sprintf("resource-%d.read", i%100))
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.HasPermissionNs)*tolerance) {
			t.Errorf("HasPermission performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.HasPermissionNs, tolerance)
		}
		t.Logf("HasPermission: %dns/op (baseline: %dns)", avgNs, baseline.HasPermissionNs)
	})

	t.Run("GetAllPermissions", func(t *testing.T) {
		rbac := New()
		// Create role hierarchy
		rbac.DefineRole("base", []string{"read"})
		rbac.DefineRole("user", []string{"write"})
		rbac.SetRoleParent("user", "base")
		rbac.DefineRole("admin", []string{"delete", "admin.*"})
		rbac.SetRoleParent("admin", "user")

		iterations := 10000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_ = rbac.GetAllPermissions("admin")
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.GetAllPermsNs)*tolerance) {
			t.Errorf("GetAllPermissions performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.GetAllPermsNs, tolerance)
		}
		t.Logf("GetAllPermissions: %dns/op (baseline: %dns)", avgNs, baseline.GetAllPermsNs)
	})

	t.Run("UserRoleService_AssignRole", func(t *testing.T) {
		rbac := New()
		rbac.DefineRole("user", []string{"read"})

		dbPath := filepath.Join(t.TempDir(), "perf_assign.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		iterations := 100

		start := time.Now()
		for i := 0; i < iterations; i++ {
			_, _ = svc.AssignRole(ctx, &AssignRoleRequest{
				UserID:   fmt.Sprintf("user-%d", i),
				RoleName: "user",
			})
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.AssignRoleNs)*tolerance) {
			t.Errorf("AssignRole performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.AssignRoleNs, tolerance)
		}
		t.Logf("AssignRole: %dns/op (baseline: %dns)", avgNs, baseline.AssignRoleNs)
	})

	t.Run("UserRoleService_HasRole", func(t *testing.T) {
		rbac := New()
		rbac.DefineRole("user", []string{"read"})

		dbPath := filepath.Join(t.TempDir(), "perf_hasrole.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		// Pre-assign roles
		for i := 0; i < 100; i++ {
			svc.AssignRole(ctx, &AssignRoleRequest{
				UserID:   fmt.Sprintf("user-%d", i),
				RoleName: "user",
			})
		}

		iterations := 1000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_, _ = svc.HasRole(ctx, fmt.Sprintf("user-%d", i%100), "user")
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.HasRoleNs)*tolerance) {
			t.Errorf("HasRole performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.HasRoleNs, tolerance)
		}
		t.Logf("HasRole: %dns/op (baseline: %dns)", avgNs, baseline.HasRoleNs)
	})

	t.Run("UserRoleService_HasPermission", func(t *testing.T) {
		rbac := New()
		rbac.DefineRole("user", []string{"read", "write", "chat.*"})

		dbPath := filepath.Join(t.TempDir(), "perf_hasperm.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		// Pre-assign roles
		for i := 0; i < 100; i++ {
			svc.AssignRole(ctx, &AssignRoleRequest{
				UserID:   fmt.Sprintf("user-%d", i),
				RoleName: "user",
			})
		}

		iterations := 1000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_, _ = svc.HasPermission(ctx, fmt.Sprintf("user-%d", i%100), "read")
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.HasPermissionDBNs)*tolerance) {
			t.Errorf("HasPermission performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.HasPermissionDBNs, tolerance)
		}
		t.Logf("HasPermission: %dns/op (baseline: %dns)", avgNs, baseline.HasPermissionDBNs)
	})

	t.Run("UserRoleService_GetUserRoles", func(t *testing.T) {
		rbac := New()
		rbac.DefineRole("admin", []string{"*"})
		rbac.DefineRole("user", []string{"read"})
		rbac.DefineRole("editor", []string{"write"})

		dbPath := filepath.Join(t.TempDir(), "perf_getroles.db")
		svc, err := NewUserRoleService(dbPath, rbac)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		defer svc.Close()

		ctx := context.Background()
		// Pre-assign multiple roles to users
		for i := 0; i < 100; i++ {
			userID := fmt.Sprintf("user-%d", i)
			svc.AssignRole(ctx, &AssignRoleRequest{UserID: userID, RoleName: "admin"})
			svc.AssignRole(ctx, &AssignRoleRequest{UserID: userID, RoleName: "user"})
			svc.AssignRole(ctx, &AssignRoleRequest{UserID: userID, RoleName: "editor"})
		}

		iterations := 1000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_, _ = svc.GetUserRoles(ctx, fmt.Sprintf("user-%d", i%100))
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.GetUserRolesNs)*tolerance) {
			t.Errorf("GetUserRoles performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.GetUserRolesNs, tolerance)
		}
		t.Logf("GetUserRoles: %dns/op (baseline: %dns)", avgNs, baseline.GetUserRolesNs)
	})
}
