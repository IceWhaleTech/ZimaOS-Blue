package user

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/password"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestService(t *testing.T) (*Service, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	repo, err := NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	hasher := password.NewHasher(nil)
	policy := password.NewPolicy(nil)
	config := &ServiceConfig{
		LockoutThreshold:     3,
		LockoutDuration:      5 * time.Minute,
		PasswordHistoryCount: 3,
		SessionDuration:      24 * time.Hour,
	}

	service := NewService(repo, hasher, policy, config)

	cleanup := func() {
		db.Close()
	}

	return service, cleanup
}

func TestService_Create(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	email := "test@example.com"

	user, err := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Email:    &email,
		Password: "SecurePass123!",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("Username = %v, want testuser", user.Username)
	}
	if user.Role != RoleUser {
		t.Errorf("Role = %v, want %v", user.Role, RoleUser)
	}
}

func TestService_Create_WeakPassword(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "weak",
	})

	if err != ErrInvalidPassword {
		t.Errorf("Create() error = %v, want %v", err, ErrInvalidPassword)
	}
}

func TestService_Create_DuplicateUsername(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, _ = service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	_, err := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass456!",
	})

	if err != ErrUsernameExists {
		t.Errorf("Create() error = %v, want %v", err, ErrUsernameExists)
	}
}

func TestService_Authenticate(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, _ = service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	user, err := service.Authenticate(ctx, "testuser", "SecurePass123!")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("Username = %v, want testuser", user.Username)
	}
}

func TestService_Authenticate_InvalidPassword(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, _ = service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	_, err := service.Authenticate(ctx, "testuser", "WrongPassword!")
	if err != ErrInvalidCredentials {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestService_Authenticate_UserNotFound(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := service.Authenticate(ctx, "nonexistent", "password")
	if err != ErrInvalidCredentials {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestService_Authenticate_AccountLockout(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, _ = service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Fail 3 times (lockout threshold)
	for i := 0; i < 3; i++ {
		_, _ = service.Authenticate(ctx, "testuser", "WrongPassword!")
	}

	// Should be locked now
	_, err := service.Authenticate(ctx, "testuser", "SecurePass123!")
	if err != ErrAccountLocked {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrAccountLocked)
	}
}

func TestService_Authenticate_MFARequired(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Enable MFA
	user.MFAEnabled = true
	secret := "JBSWY3DPEHPK3PXP"
	user.MFASecret = &secret
	_ = service.repo.Update(ctx, user)

	_, err := service.Authenticate(ctx, "testuser", "SecurePass123!")
	if err != ErrMFARequired {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrMFARequired)
	}
}

func TestService_ChangePassword(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	err := service.ChangePassword(ctx, user.ID, "SecurePass123!", "NewSecurePass456!")
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	// Old password should not work
	_, err = service.Authenticate(ctx, "testuser", "SecurePass123!")
	if err != ErrInvalidCredentials {
		t.Errorf("Authenticate() with old password error = %v, want %v", err, ErrInvalidCredentials)
	}

	// New password should work
	_, err = service.Authenticate(ctx, "testuser", "NewSecurePass456!")
	if err != nil {
		t.Errorf("Authenticate() with new password error = %v", err)
	}
}

func TestService_ChangePassword_WrongCurrentPassword(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	err := service.ChangePassword(ctx, user.ID, "WrongPassword!", "NewSecurePass456!")
	if err != ErrInvalidCredentials {
		t.Errorf("ChangePassword() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestService_ChangePassword_PasswordHistory(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Change password
	_ = service.ChangePassword(ctx, user.ID, "SecurePass123!", "NewSecurePass456!")

	// Try to reuse old password
	err := service.ChangePassword(ctx, user.ID, "NewSecurePass456!", "SecurePass123!")
	if err != ErrInvalidPassword {
		t.Errorf("ChangePassword() error = %v, want %v", err, ErrInvalidPassword)
	}
}

func TestService_Lock_Unlock(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Lock user
	err := service.Lock(ctx, user.ID)
	if err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	// Should not be able to authenticate
	_, err = service.Authenticate(ctx, "testuser", "SecurePass123!")
	if err != ErrAccountLocked {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrAccountLocked)
	}

	// Unlock user
	err = service.Unlock(ctx, user.ID)
	if err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	// Should be able to authenticate
	_, err = service.Authenticate(ctx, "testuser", "SecurePass123!")
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
	}
}

func TestService_Update(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	email := "updated@example.com"
	role := RoleAdmin

	updated, err := service.Update(ctx, user.ID, &UpdateUserRequest{
		Email: &email,
		Role:  &role,
	})

	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if *updated.Email != email {
		t.Errorf("Email = %v, want %v", *updated.Email, email)
	}
	if updated.Role != RoleAdmin {
		t.Errorf("Role = %v, want %v", updated.Role, RoleAdmin)
	}
}

func TestService_Delete(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	err := service.Delete(ctx, user.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = service.GetByID(ctx, user.ID)
	if err != ErrUserNotFound {
		t.Errorf("GetByID() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestService_Sessions(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Create session
	session, err := service.CreateSession(ctx, user.ID, "refresh_token_123", "Mozilla/5.0", "192.168.1.1")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Get session
	found, err := service.GetSession(ctx, "refresh_token_123")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if found.ID != session.ID {
		t.Errorf("Session ID = %v, want %v", found.ID, session.ID)
	}

	// Get user sessions
	sessions, err := service.GetUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserSessions() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("GetUserSessions() returned %d sessions, want 1", len(sessions))
	}

	// Revoke session
	err = service.RevokeSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}

	// Session should be revoked
	_, err = service.GetSession(ctx, "refresh_token_123")
	if err != ErrSessionRevoked {
		t.Errorf("GetSession() error = %v, want %v", err, ErrSessionRevoked)
	}
}

func TestService_RevokeAllSessions(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, _ := service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Create multiple sessions
	_, _ = service.CreateSession(ctx, user.ID, "token1", "Mozilla/5.0", "192.168.1.1")
	_, _ = service.CreateSession(ctx, user.ID, "token2", "Mozilla/5.0", "192.168.1.2")

	// Revoke all
	err := service.RevokeAllSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("RevokeAllSessions() error = %v", err)
	}

	// All sessions should be revoked
	sessions, _ := service.GetUserSessions(ctx, user.ID)
	if len(sessions) != 0 {
		t.Errorf("GetUserSessions() returned %d sessions, want 0", len(sessions))
	}
}

func TestNewService_NilDependencies(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()

	repo, _ := NewSQLiteRepository(db)

	// Should not panic with nil dependencies
	service := NewService(repo, nil, nil, nil)
	if service == nil {
		t.Error("NewService() returned nil")
	}
}

func TestService_GetByID_NotFound(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := service.GetByID(ctx, uuid.New())
	if err != ErrUserNotFound {
		t.Errorf("GetByID() error = %v, want %v", err, ErrUserNotFound)
	}
}
