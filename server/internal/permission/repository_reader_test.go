package permission

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func TestRepository_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "permissions.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	if _, err := NewRepository(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewRepository(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	repo, err := NewRepositoryWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewRepositoryWithReadDB: %v", err)
	}
	if repo.readDB == nil || repo.readDB == repo.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()
	userID := uuid.New()
	grantedBy := "system"
	if err := repo.AddPermission(ctx, userID, "chat.read", &grantedBy); err != nil {
		_ = writeDB.Close()
		t.Fatalf("AddPermission: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	perms, err := repo.GetUserPermissions(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPermissions via reader db: %v", err)
	}
	if len(perms) != 1 || perms[0] != "chat.read" {
		t.Fatalf("unexpected permissions via reader db: %+v", perms)
	}

	has, err := repo.HasPermission(ctx, userID, "chat.read")
	if err != nil {
		t.Fatalf("HasPermission via reader db: %v", err)
	}
	if !has {
		t.Fatal("expected permission via reader db")
	}

	userIDs, err := repo.GetUsersWithPermission(ctx, "chat.read")
	if err != nil {
		t.Fatalf("GetUsersWithPermission via reader db: %v", err)
	}
	if len(userIDs) != 1 || userIDs[0] != userID {
		t.Fatalf("unexpected user ids via reader db: %+v", userIDs)
	}
}
