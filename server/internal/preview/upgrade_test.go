package preview

import (
	"context"
	"database/sql"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/user"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestUpgradeService(t *testing.T) (*UpgradeService, *user.Service, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	repo, err := user.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	hasher := password.NewHasher(nil)
	policy := password.NewPolicy(nil)
	userService := user.NewService(repo, hasher, policy, nil)

	upgradeService := NewUpgradeService(userService, db)

	cleanup := func() {
		db.Close()
	}

	return upgradeService, userService, cleanup
}

func TestUpgradeService_Upgrade_Success(t *testing.T) {
	upgradeService, userService, cleanup := setupTestUpgradeService(t)
	defer cleanup()

	ctx := context.Background()

	// Verify we're in preview mode
	adminExists, _ := userService.AdminExists(ctx)
	if adminExists {
		t.Fatal("expected no admin to exist initially")
	}

	// Upgrade
	resp, err := upgradeService.Upgrade(ctx, &UpgradeRequest{
		Username: "admin",
		Password: "SecurePass123!",
	})
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Verify response
	if !resp.Success {
		t.Error("Upgrade() Success = false, want true")
	}
	if resp.User.Username != "admin" {
		t.Errorf("Upgrade() User.Username = %v, want admin", resp.User.Username)
	}
	if resp.User.Role != user.RoleAdmin {
		t.Errorf("Upgrade() User.Role = %v, want admin", resp.User.Role)
	}

	// Verify admin now exists
	adminExists, _ = userService.AdminExists(ctx)
	if !adminExists {
		t.Error("expected admin to exist after upgrade")
	}
}

func TestUpgradeService_Upgrade_EmptyUsername(t *testing.T) {
	upgradeService, _, cleanup := setupTestUpgradeService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := upgradeService.Upgrade(ctx, &UpgradeRequest{
		Username: "",
		Password: "SecurePass123!",
	})
	if err != ErrUsernameRequired {
		t.Errorf("Upgrade() error = %v, want %v", err, ErrUsernameRequired)
	}
}

func TestUpgradeService_Upgrade_EmptyPassword(t *testing.T) {
	upgradeService, _, cleanup := setupTestUpgradeService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := upgradeService.Upgrade(ctx, &UpgradeRequest{
		Username: "admin",
		Password: "",
	})
	if err != ErrPasswordRequired {
		t.Errorf("Upgrade() error = %v, want %v", err, ErrPasswordRequired)
	}
}

func TestUpgradeService_Upgrade_AdminAlreadyExists(t *testing.T) {
	upgradeService, userService, cleanup := setupTestUpgradeService(t)
	defer cleanup()

	ctx := context.Background()

	// Create an admin first
	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "existingadmin",
		Password: "SecurePass123!",
		Role:     user.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to create existing admin: %v", err)
	}

	// Try to upgrade again
	_, err = upgradeService.Upgrade(ctx, &UpgradeRequest{
		Username: "newadmin",
		Password: "SecurePass456!",
	})
	if err != ErrAdminAlreadyExists {
		t.Errorf("Upgrade() error = %v, want %v", err, ErrAdminAlreadyExists)
	}
}

func TestUpgradeService_Upgrade_WeakPassword(t *testing.T) {
	upgradeService, _, cleanup := setupTestUpgradeService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := upgradeService.Upgrade(ctx, &UpgradeRequest{
		Username: "admin",
		Password: "weak",
	})
	// Should fail due to password policy
	if err == nil {
		t.Error("Upgrade() expected error for weak password, got nil")
	}
}

func TestUpgradeService_Upgrade_DuplicateUsername(t *testing.T) {
	upgradeService, userService, cleanup := setupTestUpgradeService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a regular user with the same username
	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "admin",
		Password: "SecurePass123!",
		Role:     user.RoleUser,
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Try to upgrade with the same username
	_, err = upgradeService.Upgrade(ctx, &UpgradeRequest{
		Username: "admin",
		Password: "SecurePass456!",
	})
	if err != user.ErrUsernameExists {
		t.Errorf("Upgrade() error = %v, want %v", err, user.ErrUsernameExists)
	}
}
