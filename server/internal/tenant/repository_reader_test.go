package tenant

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupReaderTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "tenant-reader.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

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
		_ = db.Close()
		t.Fatalf("failed to create users table: %v", err)
	}

	return db, dbPath
}

func TestSQLiteRepository_UsesReaderDBForReads(t *testing.T) {
	writeDB, dbPath := setupReaderTestDB(t)

	repo := NewSQLiteRepository(writeDB)
	if err := repo.InitSchema(context.Background()); err != nil {
		_ = writeDB.Close()
		t.Fatalf("failed to init schema: %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("failed to open read db: %v", err)
	}
	defer readDB.Close()

	repo = NewSQLiteRepositoryWithReadDB(writeDB, readDB)
	if repo.readDB == nil || repo.readDB == repo.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()
	ownerID := uuid.New()
	insertTestUser(t, writeDB, ownerID, "tenant-owner", "owner@example.com")

	tenant := NewTenant("Reader Tenant", "reader-tenant", ownerID)
	if err := repo.Create(ctx, tenant); err != nil {
		_ = writeDB.Close()
		t.Fatalf("failed to create tenant: %v", err)
	}

	member := NewTenantMember(tenant.ID, ownerID, MemberRoleOwner, nil)
	if err := repo.AddMember(ctx, member); err != nil {
		_ = writeDB.Close()
		t.Fatalf("failed to add member: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("failed to close writer db: %v", err)
	}
	writeDB = nil

	gotTenant, err := repo.GetByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("GetByID via reader db: %v", err)
	}
	if gotTenant.Slug != tenant.Slug {
		t.Fatalf("unexpected tenant via reader db: %+v", gotTenant)
	}

	exists, err := repo.ExistsBySlug(ctx, tenant.Slug)
	if err != nil {
		t.Fatalf("ExistsBySlug via reader db: %v", err)
	}
	if !exists {
		t.Fatal("expected tenant slug to exist via reader db")
	}

	isMember, err := repo.IsMember(ctx, tenant.ID, ownerID)
	if err != nil {
		t.Fatalf("IsMember via reader db: %v", err)
	}
	if !isMember {
		t.Fatal("expected tenant membership via reader db")
	}

	members, err := repo.ListMembers(ctx, tenant.ID, &ListMembersQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListMembers via reader db: %v", err)
	}
	if members.Total != 1 || len(members.Members) != 1 || members.Members[0].Username != "tenant-owner" {
		t.Fatalf("unexpected tenant members via reader db: %+v", members)
	}
}
