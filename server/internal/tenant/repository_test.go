package tenant

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create users table for foreign key tests
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		email TEXT,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		deleted_at DATETIME
	)`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	return db
}

func TestSQLiteRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	ownerID := uuid.New()
	tenant := NewTenant("Test Tenant", "test-tenant", ownerID)

	err := repo.Create(context.Background(), tenant)
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Verify tenant was created
	retrieved, err := repo.GetByID(context.Background(), tenant.ID)
	if err != nil {
		t.Fatalf("failed to get tenant: %v", err)
	}

	if retrieved.Name != tenant.Name {
		t.Errorf("expected name %s, got %s", tenant.Name, retrieved.Name)
	}
	if retrieved.Slug != tenant.Slug {
		t.Errorf("expected slug %s, got %s", tenant.Slug, retrieved.Slug)
	}
}

func TestSQLiteRepository_Create_DuplicateSlug(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	ownerID := uuid.New()
	tenant1 := NewTenant("Test Tenant 1", "test-tenant", ownerID)
	tenant2 := NewTenant("Test Tenant 2", "test-tenant", ownerID)

	if err := repo.Create(context.Background(), tenant1); err != nil {
		t.Fatalf("failed to create first tenant: %v", err)
	}

	err := repo.Create(context.Background(), tenant2)
	if err != ErrSlugExists {
		t.Errorf("expected ErrSlugExists, got %v", err)
	}
}

func TestSQLiteRepository_GetBySlug(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	ownerID := uuid.New()
	tenant := NewTenant("Test Tenant", "test-tenant", ownerID)

	if err := repo.Create(context.Background(), tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	retrieved, err := repo.GetBySlug(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("failed to get tenant by slug: %v", err)
	}

	if retrieved.ID != tenant.ID {
		t.Errorf("expected ID %s, got %s", tenant.ID, retrieved.ID)
	}
}

func TestSQLiteRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	ownerID := uuid.New()
	tenant := NewTenant("Test Tenant", "test-tenant", ownerID)

	if err := repo.Create(context.Background(), tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	tenant.Name = "Updated Tenant"
	if err := repo.Update(context.Background(), tenant); err != nil {
		t.Fatalf("failed to update tenant: %v", err)
	}

	retrieved, err := repo.GetByID(context.Background(), tenant.ID)
	if err != nil {
		t.Fatalf("failed to get tenant: %v", err)
	}

	if retrieved.Name != "Updated Tenant" {
		t.Errorf("expected name 'Updated Tenant', got %s", retrieved.Name)
	}
}

func TestSQLiteRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	ownerID := uuid.New()
	tenant := NewTenant("Test Tenant", "test-tenant", ownerID)

	if err := repo.Create(context.Background(), tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	if err := repo.Delete(context.Background(), tenant.ID); err != nil {
		t.Fatalf("failed to delete tenant: %v", err)
	}

	_, err := repo.GetByID(context.Background(), tenant.ID)
	if err != ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestSQLiteRepository_Members(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	ownerID := uuid.New()
	tenant := NewTenant("Test Tenant", "test-tenant", ownerID)

	if err := repo.Create(context.Background(), tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Add member
	member := NewTenantMember(tenant.ID, ownerID, MemberRoleOwner, nil)
	if err := repo.AddMember(context.Background(), member); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// Check membership
	isMember, err := repo.IsMember(context.Background(), tenant.ID, ownerID)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("expected user to be a member")
	}

	// Get member
	retrieved, err := repo.GetMember(context.Background(), tenant.ID, ownerID)
	if err != nil {
		t.Fatalf("failed to get member: %v", err)
	}
	if retrieved.Role != MemberRoleOwner {
		t.Errorf("expected role %s, got %s", MemberRoleOwner, retrieved.Role)
	}

	// Update member role
	retrieved.Role = MemberRoleAdmin
	if err := repo.UpdateMember(context.Background(), retrieved); err != nil {
		t.Fatalf("failed to update member: %v", err)
	}

	// Verify update
	retrieved, err = repo.GetMember(context.Background(), tenant.ID, ownerID)
	if err != nil {
		t.Fatalf("failed to get member: %v", err)
	}
	if retrieved.Role != MemberRoleAdmin {
		t.Errorf("expected role %s, got %s", MemberRoleAdmin, retrieved.Role)
	}

	// Count members
	count, err := repo.CountMembers(context.Background(), tenant.ID)
	if err != nil {
		t.Fatalf("failed to count members: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 member, got %d", count)
	}

	// Remove member
	if err := repo.RemoveMember(context.Background(), tenant.ID, ownerID); err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}

	isMember, err = repo.IsMember(context.Background(), tenant.ID, ownerID)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if isMember {
		t.Error("expected user to not be a member")
	}
}

func TestSQLiteRepository_Invitations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	ownerID := uuid.New()
	tenant := NewTenant("Test Tenant", "test-tenant", ownerID)

	if err := repo.Create(context.Background(), tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Create invitation
	invitation := NewTenantInvitation(
		tenant.ID,
		"test@example.com",
		MemberRoleMember,
		ownerID,
		"test-token-123",
		time.Now().Add(24*time.Hour),
	)

	if err := repo.CreateInvitation(context.Background(), invitation); err != nil {
		t.Fatalf("failed to create invitation: %v", err)
	}

	// Get by token
	retrieved, err := repo.GetInvitationByToken(context.Background(), "test-token-123")
	if err != nil {
		t.Fatalf("failed to get invitation by token: %v", err)
	}
	if retrieved.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", retrieved.Email)
	}

	// Get pending by email
	retrieved, err = repo.GetPendingInvitationByEmail(context.Background(), tenant.ID, "test@example.com")
	if err != nil {
		t.Fatalf("failed to get pending invitation: %v", err)
	}
	if retrieved.Token != "test-token-123" {
		t.Errorf("expected token test-token-123, got %s", retrieved.Token)
	}

	// List pending
	invitations, err := repo.ListPendingInvitations(context.Background(), tenant.ID)
	if err != nil {
		t.Fatalf("failed to list invitations: %v", err)
	}
	if len(invitations) != 1 {
		t.Errorf("expected 1 invitation, got %d", len(invitations))
	}

	// Accept invitation
	if err := repo.AcceptInvitation(context.Background(), invitation.ID); err != nil {
		t.Fatalf("failed to accept invitation: %v", err)
	}

	// Verify accepted
	retrieved, err = repo.GetInvitationByID(context.Background(), invitation.ID)
	if err != nil {
		t.Fatalf("failed to get invitation: %v", err)
	}
	if retrieved.AcceptedAt == nil {
		t.Error("expected invitation to be accepted")
	}

	// List pending should be empty now
	invitations, err = repo.ListPendingInvitations(context.Background(), tenant.ID)
	if err != nil {
		t.Fatalf("failed to list invitations: %v", err)
	}
	if len(invitations) != 0 {
		t.Errorf("expected 0 invitations, got %d", len(invitations))
	}
}

func TestService_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	service := NewService(repo, nil)

	ownerID := uuid.New()
	req := &CreateTenantRequest{
		Name: "Test Tenant",
		Slug: "test-tenant",
	}

	tenant, err := service.Create(context.Background(), req, ownerID)
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	if tenant.Name != req.Name {
		t.Errorf("expected name %s, got %s", req.Name, tenant.Name)
	}

	// Verify owner is a member
	isMember, err := service.IsMember(context.Background(), tenant.ID, ownerID)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("expected owner to be a member")
	}

	// Verify owner has owner role
	member, err := service.GetMember(context.Background(), tenant.ID, ownerID)
	if err != nil {
		t.Fatalf("failed to get member: %v", err)
	}
	if member.Role != MemberRoleOwner {
		t.Errorf("expected role %s, got %s", MemberRoleOwner, member.Role)
	}
}

func TestService_HasPermission(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	service := NewService(repo, nil)

	ownerID := uuid.New()
	req := &CreateTenantRequest{
		Name: "Test Tenant",
		Slug: "test-tenant",
	}

	tenant, err := service.Create(context.Background(), req, ownerID)
	if err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	// Owner should have all permissions
	tests := []struct {
		role     MemberRole
		expected bool
	}{
		{MemberRoleOwner, true},
		{MemberRoleAdmin, true},
		{MemberRoleMember, true},
	}

	for _, tt := range tests {
		hasPermission, err := service.HasPermission(context.Background(), tenant.ID, ownerID, tt.role)
		if err != nil {
			t.Fatalf("failed to check permission: %v", err)
		}
		if hasPermission != tt.expected {
			t.Errorf("expected permission %v for role %s, got %v", tt.expected, tt.role, hasPermission)
		}
	}

	// Non-member should have no permissions
	nonMemberID := uuid.New()
	hasPermission, err := service.HasPermission(context.Background(), tenant.ID, nonMemberID, MemberRoleMember)
	if err != nil {
		t.Fatalf("failed to check permission: %v", err)
	}
	if hasPermission {
		t.Error("expected non-member to have no permissions")
	}
}

func TestHasRolePermission(t *testing.T) {
	tests := []struct {
		role         MemberRole
		requiredRole MemberRole
		expected     bool
	}{
		{MemberRoleOwner, MemberRoleOwner, true},
		{MemberRoleOwner, MemberRoleAdmin, true},
		{MemberRoleOwner, MemberRoleMember, true},
		{MemberRoleAdmin, MemberRoleOwner, false},
		{MemberRoleAdmin, MemberRoleAdmin, true},
		{MemberRoleAdmin, MemberRoleMember, true},
		{MemberRoleMember, MemberRoleOwner, false},
		{MemberRoleMember, MemberRoleAdmin, false},
		{MemberRoleMember, MemberRoleMember, true},
	}

	for _, tt := range tests {
		result := hasRolePermission(tt.role, tt.requiredRole)
		if result != tt.expected {
			t.Errorf("hasRolePermission(%s, %s) = %v, expected %v", tt.role, tt.requiredRole, result, tt.expected)
		}
	}
}
