package user

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteRepository_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "users.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	if _, err := NewSQLiteRepository(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewSQLiteRepository(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	repo, err := NewSQLiteRepositoryWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewSQLiteRepositoryWithReadDB: %v", err)
	}
	if repo.readDB == nil || repo.readDB == repo.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()
	user := NewUser("reader-user", "hashedpassword")
	email := "reader@example.com"
	user.Email = &email
	if err := repo.Create(ctx, user); err != nil {
		_ = writeDB.Close()
		t.Fatalf("Create: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	gotByID, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID via reader db: %v", err)
	}
	if gotByID.Username != user.Username {
		t.Fatalf("unexpected user via reader id lookup: %+v", gotByID)
	}

	gotByUsername, err := repo.GetByUsername(ctx, user.Username)
	if err != nil {
		t.Fatalf("GetByUsername via reader db: %v", err)
	}
	if gotByUsername.ID != user.ID {
		t.Fatalf("unexpected user via reader username lookup: %+v", gotByUsername)
	}

	gotByEmail, err := repo.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetByEmail via reader db: %v", err)
	}
	if gotByEmail.ID != user.ID {
		t.Fatalf("unexpected user via reader email lookup: %+v", gotByEmail)
	}
}
