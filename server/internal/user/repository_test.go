package user

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestNewSQLiteRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("NewSQLiteRepository() error = %v", err)
	}
	if repo == nil {
		t.Fatal("NewSQLiteRepository() returned nil")
	}
}

func TestSQLiteRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	email := "test@example.com"
	user.Email = &email

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify user was created
	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if found.Username != user.Username {
		t.Errorf("Username = %v, want %v", found.Username, user.Username)
	}
}

func TestSQLiteRepository_Create_DuplicateUsername(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user1 := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user1)

	user2 := NewUser("testuser", "hashedpassword2")
	err := repo.Create(ctx, user2)
	if err != ErrUsernameExists {
		t.Errorf("Create() error = %v, want %v", err, ErrUsernameExists)
	}
}

func TestSQLiteRepository_Create_DuplicateEmail(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	email := "test@example.com"
	user1 := NewUser("testuser1", "hashedpassword")
	user1.Email = &email
	_ = repo.Create(ctx, user1)

	user2 := NewUser("testuser2", "hashedpassword2")
	user2.Email = &email
	err := repo.Create(ctx, user2)
	if err != ErrEmailExists {
		t.Errorf("Create() error = %v, want %v", err, ErrEmailExists)
	}
}

func TestSQLiteRepository_GetByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("ID = %v, want %v", found.ID, user.ID)
	}
}

func TestSQLiteRepository_GetByID_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if err != ErrUserNotFound {
		t.Errorf("GetByID() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestSQLiteRepository_GetByUsername(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	found, err := repo.GetByUsername(ctx, "testuser")
	if err != nil {
		t.Fatalf("GetByUsername() error = %v", err)
	}
	if found.Username != "testuser" {
		t.Errorf("Username = %v, want %v", found.Username, "testuser")
	}
}

func TestSQLiteRepository_GetByEmail(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	email := "test@example.com"
	user := NewUser("testuser", "hashedpassword")
	user.Email = &email
	_ = repo.Create(ctx, user)

	found, err := repo.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if *found.Email != email {
		t.Errorf("Email = %v, want %v", *found.Email, email)
	}
}

func TestSQLiteRepository_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	// Update user
	email := "updated@example.com"
	user.Email = &email
	user.Role = RoleAdmin

	err := repo.Update(ctx, user)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	found, _ := repo.GetByID(ctx, user.ID)
	if *found.Email != email {
		t.Errorf("Email = %v, want %v", *found.Email, email)
	}
	if found.Role != RoleAdmin {
		t.Errorf("Role = %v, want %v", found.Role, RoleAdmin)
	}
}

func TestSQLiteRepository_Update_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	err := repo.Update(ctx, user)
	if err != ErrUserNotFound {
		t.Errorf("Update() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestSQLiteRepository_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	err := repo.Delete(ctx, user.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify soft delete
	_, err = repo.GetByID(ctx, user.ID)
	if err != ErrUserNotFound {
		t.Errorf("GetByID() after delete error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestSQLiteRepository_Delete_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())
	if err != ErrUserNotFound {
		t.Errorf("Delete() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestSQLiteRepository_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	// Create test users
	for i := 0; i < 25; i++ {
		user := NewUser("testuser"+string(rune('a'+i)), "hashedpassword")
		_ = repo.Create(ctx, user)
	}

	// Test pagination
	result, err := repo.List(ctx, &ListUsersQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Users) != 10 {
		t.Errorf("List() returned %d users, want 10", len(result.Users))
	}
	if result.Total != 25 {
		t.Errorf("List() total = %d, want 25", result.Total)
	}
	if result.TotalPages != 3 {
		t.Errorf("List() totalPages = %d, want 3", result.TotalPages)
	}
}

func TestSQLiteRepository_List_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user1 := NewUser("alice", "hashedpassword")
	user2 := NewUser("bob", "hashedpassword")
	_ = repo.Create(ctx, user1)
	_ = repo.Create(ctx, user2)

	result, err := repo.List(ctx, &ListUsersQuery{Search: "alice"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Users) != 1 {
		t.Errorf("List() returned %d users, want 1", len(result.Users))
	}
	if result.Users[0].Username != "alice" {
		t.Errorf("List() username = %v, want alice", result.Users[0].Username)
	}
}

func TestSQLiteRepository_List_FilterByRole(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user1 := NewUser("admin1", "hashedpassword")
	user1.Role = RoleAdmin
	user2 := NewUser("user1", "hashedpassword")
	user2.Role = RoleUser
	_ = repo.Create(ctx, user1)
	_ = repo.Create(ctx, user2)

	role := RoleAdmin
	result, err := repo.List(ctx, &ListUsersQuery{Role: &role})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Users) != 1 {
		t.Errorf("List() returned %d users, want 1", len(result.Users))
	}
}

func TestSQLiteRepository_ExistsByUsername(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	exists, err := repo.ExistsByUsername(ctx, "testuser")
	if err != nil {
		t.Fatalf("ExistsByUsername() error = %v", err)
	}
	if !exists {
		t.Error("ExistsByUsername() = false, want true")
	}

	exists, err = repo.ExistsByUsername(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ExistsByUsername() error = %v", err)
	}
	if exists {
		t.Error("ExistsByUsername() = true, want false")
	}
}

func TestSQLiteRepository_PasswordHistory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	// Add password history with small delays to ensure ordering
	_ = repo.AddPasswordHistory(ctx, user.ID, "hash1")
	time.Sleep(10 * time.Millisecond)
	_ = repo.AddPasswordHistory(ctx, user.ID, "hash2")
	time.Sleep(10 * time.Millisecond)
	_ = repo.AddPasswordHistory(ctx, user.ID, "hash3")

	// Get password history
	history, err := repo.GetPasswordHistory(ctx, user.ID, 2)
	if err != nil {
		t.Fatalf("GetPasswordHistory() error = %v", err)
	}
	if len(history) != 2 {
		t.Errorf("GetPasswordHistory() returned %d hashes, want 2", len(history))
	}
	// Most recent should be first
	if history[0] != "hash3" {
		t.Errorf("GetPasswordHistory()[0] = %v, want hash3", history[0])
	}
}

func TestSQLiteRepository_Sessions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	// Create session
	session := NewSession(user.ID, "refresh_token_123", "Mozilla/5.0", "192.168.1.1", time.Now().Add(24*time.Hour))
	err := repo.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Get session by ID
	found, err := repo.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSessionByID() error = %v", err)
	}
	if found.RefreshToken != session.RefreshToken {
		t.Errorf("RefreshToken = %v, want %v", found.RefreshToken, session.RefreshToken)
	}

	// Get session by refresh token
	found, err = repo.GetSessionByRefreshToken(ctx, "refresh_token_123")
	if err != nil {
		t.Fatalf("GetSessionByRefreshToken() error = %v", err)
	}
	if found.ID != session.ID {
		t.Errorf("ID = %v, want %v", found.ID, session.ID)
	}

	// Get user sessions
	sessions, err := repo.GetUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserSessions() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("GetUserSessions() returned %d sessions, want 1", len(sessions))
	}
}

func TestSQLiteRepository_RevokeSession(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	session := NewSession(user.ID, "refresh_token_123", "Mozilla/5.0", "192.168.1.1", time.Now().Add(24*time.Hour))
	_ = repo.CreateSession(ctx, session)

	// Revoke session
	err := repo.RevokeSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}

	// Verify session is revoked
	found, _ := repo.GetSessionByID(ctx, session.ID)
	if found.RevokedAt == nil {
		t.Error("Session should be revoked")
	}

	// User sessions should not include revoked session
	sessions, _ := repo.GetUserSessions(ctx, user.ID)
	if len(sessions) != 0 {
		t.Errorf("GetUserSessions() returned %d sessions, want 0", len(sessions))
	}
}

func TestSQLiteRepository_RevokeUserSessions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewSQLiteRepository(db)
	ctx := context.Background()

	user := NewUser("testuser", "hashedpassword")
	_ = repo.Create(ctx, user)

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		session := NewSession(user.ID, "refresh_token_"+string(rune('0'+i)), "Mozilla/5.0", "192.168.1.1", time.Now().Add(24*time.Hour))
		_ = repo.CreateSession(ctx, session)
	}

	// Revoke all sessions
	err := repo.RevokeUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("RevokeUserSessions() error = %v", err)
	}

	// Verify all sessions are revoked
	sessions, _ := repo.GetUserSessions(ctx, user.ID)
	if len(sessions) != 0 {
		t.Errorf("GetUserSessions() returned %d sessions, want 0", len(sessions))
	}
}
