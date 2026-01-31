package preview

import (
	"context"
	"database/sql"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/user"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestUserService(t *testing.T) (*user.Service, func()) {
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
	service := user.NewService(repo, hasher, policy, nil)

	cleanup := func() {
		db.Close()
	}

	return service, cleanup
}

func TestModeService_IsPreviewMode_NoAdmin(t *testing.T) {
	userService, cleanup := setupTestUserService(t)
	defer cleanup()

	modeService := NewModeService(userService)

	// No admin exists, should be preview mode
	isPreview, err := modeService.IsPreviewMode(context.Background())
	if err != nil {
		t.Fatalf("IsPreviewMode() error = %v", err)
	}
	if !isPreview {
		t.Error("IsPreviewMode() = false, want true when no admin exists")
	}
}

func TestModeService_IsPreviewMode_WithAdmin(t *testing.T) {
	userService, cleanup := setupTestUserService(t)
	defer cleanup()

	ctx := context.Background()

	// Create an admin user
	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "admin",
		Password: "SecurePass123!",
		Role:     user.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	modeService := NewModeService(userService)

	// Admin exists, should not be preview mode
	isPreview, err := modeService.IsPreviewMode(ctx)
	if err != nil {
		t.Fatalf("IsPreviewMode() error = %v", err)
	}
	if isPreview {
		t.Error("IsPreviewMode() = true, want false when admin exists")
	}
}

func TestModeService_IsPreviewMode_WithRegularUser(t *testing.T) {
	userService, cleanup := setupTestUserService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a regular user (not admin)
	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "regularuser",
		Password: "SecurePass123!",
		Role:     user.RoleUser,
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	modeService := NewModeService(userService)

	// Only regular user exists, should still be preview mode
	isPreview, err := modeService.IsPreviewMode(ctx)
	if err != nil {
		t.Fatalf("IsPreviewMode() error = %v", err)
	}
	if !isPreview {
		t.Error("IsPreviewMode() = false, want true when only regular user exists")
	}
}

func TestModeService_GetSystemMode_Preview(t *testing.T) {
	userService, cleanup := setupTestUserService(t)
	defer cleanup()

	modeService := NewModeService(userService)

	mode, err := modeService.GetSystemMode(context.Background())
	if err != nil {
		t.Fatalf("GetSystemMode() error = %v", err)
	}
	if mode.Mode != "preview" {
		t.Errorf("GetSystemMode().Mode = %v, want preview", mode.Mode)
	}
	if !mode.Features["chat"] {
		t.Error("GetSystemMode().Features[chat] = false, want true")
	}
	if mode.Features["admin"] {
		t.Error("GetSystemMode().Features[admin] = true, want false in preview mode")
	}
	if mode.Features["user_management"] {
		t.Error("GetSystemMode().Features[user_management] = true, want false in preview mode")
	}
}

func TestModeService_GetSystemMode_Normal(t *testing.T) {
	userService, cleanup := setupTestUserService(t)
	defer cleanup()

	ctx := context.Background()

	// Create an admin user
	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "admin",
		Password: "SecurePass123!",
		Role:     user.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	modeService := NewModeService(userService)

	mode, err := modeService.GetSystemMode(ctx)
	if err != nil {
		t.Fatalf("GetSystemMode() error = %v", err)
	}
	if mode.Mode != "normal" {
		t.Errorf("GetSystemMode().Mode = %v, want normal", mode.Mode)
	}
	if !mode.Features["admin"] {
		t.Error("GetSystemMode().Features[admin] = false, want true in normal mode")
	}
	if !mode.Features["user_management"] {
		t.Error("GetSystemMode().Features[user_management] = false, want true in normal mode")
	}
}
