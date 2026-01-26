package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TestUser is a test entity for zorm tests.
type TestUser struct {
	ID        int64     `zorm:"id,auto_incr"`
	Name      string    `zorm:"name"`
	Email     string    `zorm:"email"`
	Age       int       `zorm:"age"`
	CreatedAt time.Time `zorm:"created_at"`
}

func setupZormTestDB(t *testing.T) (*ZormDB, func()) {
	tmpFile, err := os.CreateTemp("", "zorm-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	config := ZormConfig{
		DSN:             tmpFile.Name(),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	db, err := NewZormDB(config)
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create ZormDB: %v", err)
	}

	// Create test table
	_, err = db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE,
			age INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create test table: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestZormDB_NewZormDB(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	if db == nil {
		t.Fatal("Expected non-nil ZormDB")
	}

	if db.DB() == nil {
		t.Fatal("Expected non-nil underlying sql.DB")
	}
}

func TestZormDB_DefaultConfig(t *testing.T) {
	config := DefaultZormConfig()

	if config.MaxOpenConns != 10 {
		t.Errorf("MaxOpenConns = %d, want 10", config.MaxOpenConns)
	}
	if config.MaxIdleConns != 5 {
		t.Errorf("MaxIdleConns = %d, want 5", config.MaxIdleConns)
	}
	if config.ConnMaxLifetime != time.Hour {
		t.Errorf("ConnMaxLifetime = %v, want 1h", config.ConnMaxLifetime)
	}
}

func TestZormRepository_Insert(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	user := &TestUser{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	id, err := repo.Insert(ctx, user)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if id <= 0 {
		t.Errorf("Insert() returned id = %d, want > 0", id)
	}
}

func TestZormRepository_FindByID(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert a user first
	user := &TestUser{
		Name:  "Jane Doe",
		Email: "jane@example.com",
		Age:   25,
	}
	id, _ := repo.Insert(ctx, user)

	// Find by ID
	var found TestUser
	err := repo.FindByID(ctx, id, &found)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.Name != "Jane Doe" {
		t.Errorf("FindByID() name = %s, want Jane Doe", found.Name)
	}
	if found.Email != "jane@example.com" {
		t.Errorf("FindByID() email = %s, want jane@example.com", found.Email)
	}
}

func TestZormRepository_FindAll(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert multiple users
	for i := 0; i < 3; i++ {
		user := &TestUser{
			Name:  fmt.Sprintf("User%d", i+1),
			Email: fmt.Sprintf("user%d@example.com", i+1),
			Age:   20 + i*5,
		}
		repo.Insert(ctx, user)
	}

	// Find all
	var found []TestUser
	count, err := repo.FindAll(ctx, &found)
	if err != nil {
		t.Fatalf("FindAll() error = %v", err)
	}

	if count != 3 {
		t.Errorf("FindAll() count = %d, want 3", count)
	}
	if len(found) != 3 {
		t.Errorf("FindAll() len = %d, want 3", len(found))
	}
}

func TestZormRepository_FindWithPagination(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert 10 users
	for i := 0; i < 10; i++ {
		user := &TestUser{
			Name:  "User",
			Email: fmt.Sprintf("page%d@example.com", i),
			Age:   20 + i,
		}
		repo.Insert(ctx, user)
	}

	// Get page 1 with size 3
	var page1 []TestUser
	count, err := repo.FindWithPagination(ctx, &page1, 1, 3)
	if err != nil {
		t.Fatalf("FindWithPagination() error = %v", err)
	}

	if count != 3 {
		t.Errorf("FindWithPagination() count = %d, want 3", count)
	}

	// Get page 2 with size 3
	var page2 []TestUser
	count, err = repo.FindWithPagination(ctx, &page2, 2, 3)
	if err != nil {
		t.Fatalf("FindWithPagination() page 2 error = %v", err)
	}

	if count != 3 {
		t.Errorf("FindWithPagination() page 2 count = %d, want 3", count)
	}
}

func TestZormRepository_Update(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert a user
	user := &TestUser{
		Name:  "Original Name",
		Email: "original@example.com",
		Age:   30,
	}
	id, _ := repo.Insert(ctx, user)

	// Update the user
	user.ID = id
	user.Name = "Updated Name"
	user.Age = 35

	affected, err := repo.UpdateByID(ctx, id, user)
	if err != nil {
		t.Fatalf("UpdateByID() error = %v", err)
	}

	if affected != 1 {
		t.Errorf("UpdateByID() affected = %d, want 1", affected)
	}

	// Verify update
	var found TestUser
	repo.FindByID(ctx, id, &found)
	if found.Name != "Updated Name" {
		t.Errorf("After update, name = %s, want Updated Name", found.Name)
	}
	if found.Age != 35 {
		t.Errorf("After update, age = %d, want 35", found.Age)
	}
}

func TestZormRepository_Delete(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert a user
	user := &TestUser{
		Name:  "To Delete",
		Email: "delete@example.com",
		Age:   25,
	}
	id, _ := repo.Insert(ctx, user)

	// Delete the user
	affected, err := repo.DeleteByID(ctx, id)
	if err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}

	if affected != 1 {
		t.Errorf("DeleteByID() affected = %d, want 1", affected)
	}

	// Verify deletion
	exists, _ := repo.Exists(ctx, Where(Eq("id", id)))
	if exists {
		t.Error("User should not exist after deletion")
	}
}

func TestZormRepository_Count(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert users
	for i := 0; i < 5; i++ {
		user := &TestUser{
			Name:  "User",
			Email: fmt.Sprintf("count%d@example.com", i),
			Age:   20 + i,
		}
		repo.Insert(ctx, user)
	}

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 5 {
		t.Errorf("Count() = %d, want 5", count)
	}
}

func TestZormRepository_Exists(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert a user
	user := &TestUser{
		Name:  "Exists Test",
		Email: "exists@example.com",
		Age:   30,
	}
	repo.Insert(ctx, user)

	// Check exists
	exists, err := repo.Exists(ctx, Where(Eq("email", "exists@example.com")))
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}

	// Check not exists
	exists, err = repo.Exists(ctx, Where(Eq("email", "notexists@example.com")))
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}
}

func TestZormRepository_InsertBatch(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	users := []TestUser{
		{Name: "Batch1", Email: "batch1@example.com", Age: 20},
		{Name: "Batch2", Email: "batch2@example.com", Age: 25},
		{Name: "Batch3", Email: "batch3@example.com", Age: 30},
	}

	affected, err := repo.InsertBatch(ctx, &users)
	if err != nil {
		t.Fatalf("InsertBatch() error = %v", err)
	}

	if affected != 3 {
		t.Errorf("InsertBatch() affected = %d, want 3", affected)
	}

	count, _ := repo.Count(ctx)
	if count != 3 {
		t.Errorf("After InsertBatch, count = %d, want 3", count)
	}
}

func TestZormQueryBuilder_Basic(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert users with different ages
	users := []TestUser{
		{Name: "Young", Email: "young@example.com", Age: 20},
		{Name: "Middle", Email: "middle@example.com", Age: 30},
		{Name: "Old", Email: "old@example.com", Age: 40},
	}
	for i := range users {
		repo.Insert(ctx, &users[i])
	}

	// Query with builder
	var found []TestUser
	qb := repo.NewQueryBuilder(ctx)
	count, err := qb.Where(Gt("age", 25)).OrderBy("age").Select(&found)
	if err != nil {
		t.Fatalf("QueryBuilder.Select() error = %v", err)
	}

	if count != 2 {
		t.Errorf("QueryBuilder.Select() count = %d, want 2", count)
	}

	if len(found) > 0 && found[0].Name != "Middle" {
		t.Errorf("First result name = %s, want Middle", found[0].Name)
	}
}

func TestZormQueryBuilder_Limit(t *testing.T) {
	db, cleanup := setupZormTestDB(t)
	defer cleanup()

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert 5 users
	for i := 0; i < 5; i++ {
		user := &TestUser{
			Name:  "User",
			Email: fmt.Sprintf("limit%d@example.com", i),
			Age:   20 + i,
		}
		repo.Insert(ctx, user)
	}

	var found []TestUser
	qb := repo.NewQueryBuilder(ctx)
	count, err := qb.Limit(2).Select(&found)
	if err != nil {
		t.Fatalf("QueryBuilder.Limit().Select() error = %v", err)
	}

	if count != 2 {
		t.Errorf("QueryBuilder.Limit().Select() count = %d, want 2", count)
	}
}

func TestZormConditionHelpers(t *testing.T) {
	// Test that condition helpers are properly exported
	if Eq == nil {
		t.Error("Eq should not be nil")
	}
	if Neq == nil {
		t.Error("Neq should not be nil")
	}
	if Gt == nil {
		t.Error("Gt should not be nil")
	}
	if Lt == nil {
		t.Error("Lt should not be nil")
	}
	if In == nil {
		t.Error("In should not be nil")
	}
	if Like == nil {
		t.Error("Like should not be nil")
	}
	if Where == nil {
		t.Error("Where should not be nil")
	}
	if OrderBy == nil {
		t.Error("OrderBy should not be nil")
	}
	if Limit == nil {
		t.Error("Limit should not be nil")
	}
}

func BenchmarkZormRepository_Insert(b *testing.B) {
	tmpFile, _ := os.CreateTemp("", "zorm-bench-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	config := ZormConfig{
		DSN:             tmpFile.Name(),
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	}

	db, _ := NewZormDB(config)
	defer db.Close()

	db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT,
			age INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := &TestUser{
			Name:  "Benchmark User",
			Email: fmt.Sprintf("bench%d@example.com", i),
			Age:   30,
		}
		repo.Insert(ctx, user)
	}
}

func BenchmarkZormRepository_FindByID(b *testing.B) {
	tmpFile, _ := os.CreateTemp("", "zorm-bench-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	config := ZormConfig{
		DSN:             tmpFile.Name(),
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	}

	db, _ := NewZormDB(config)
	defer db.Close()

	db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT,
			age INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)

	repo := NewRepository[TestUser](db, "users")
	ctx := context.Background()

	// Insert test data
	for i := 0; i < 1000; i++ {
		user := &TestUser{
			Name:  "User",
			Email: fmt.Sprintf("find%d@example.com", i),
			Age:   30,
		}
		repo.Insert(ctx, user)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var user TestUser
		repo.FindByID(ctx, int64(i%1000+1), &user)
	}
}
