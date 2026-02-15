package personality

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRepositoryCreate(t *testing.T) {
	// Setup in-memory database
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	p := NewPersonality("Test", "Test desc", "Test prompt")

	// Act
	err := repo.Create(p)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if p.ID == "" {
		t.Error("Expected ID to be set")
	}
}

func TestRepositoryGetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	p := NewPersonality("Test", "Test desc", "Test prompt")
	repo.Create(p)

	// Act
	retrieved, err := repo.GetByID(p.ID)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved.Name != p.Name {
		t.Errorf("Expected name %s, got %s", p.Name, retrieved.Name)
	}
}

func TestRepositoryList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	p1 := NewPersonality("P1", "Desc1", "Prompt1")
	p2 := NewPersonality("P2", "Desc2", "Prompt2")
	repo.Create(p1)
	repo.Create(p2)

	// Act
	personalities, err := repo.List()

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(personalities) != 2 {
		t.Errorf("Expected 2 personalities, got %d", len(personalities))
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create schema
	schema := `
	CREATE TABLE personalities (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		system_prompt TEXT NOT NULL,
		is_active BOOLEAN DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	);
	CREATE TABLE personality_traits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		personality_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		weight REAL,
		FOREIGN KEY(personality_id) REFERENCES personalities(id)
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}
