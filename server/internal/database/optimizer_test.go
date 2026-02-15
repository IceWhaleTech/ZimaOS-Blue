package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create test tables
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE INDEX idx_users_email ON users(email);
		CREATE INDEX idx_users_created_at ON users(created_at);

		CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			content TEXT,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		CREATE INDEX idx_posts_user_id ON posts(user_id);
	`)
	if err != nil {
		db.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Insert test data
	for i := 0; i < 100; i++ {
		_, err = db.Exec(
			"INSERT INTO users (name, email, created_at) VALUES (?, ?, ?)",
			"User "+string(rune('A'+i%26)),
			"user"+string(rune('a'+i%26))+"@example.com",
			time.Now().Add(-time.Duration(i)*time.Hour),
		)
		if err != nil {
			db.Close()
			os.Remove(tmpFile.Name())
			t.Fatalf("Failed to insert user: %v", err)
		}
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestOptimizer_AnalyzeQuery(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	opt := NewOptimizer(db, DefaultOptimizerConfig(), logger)

	ctx := context.Background()

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "simple select",
			query:   "SELECT * FROM users WHERE id = ?",
			wantErr: false,
		},
		{
			name:    "select with index",
			query:   "SELECT * FROM users WHERE email = ?",
			wantErr: false,
		},
		{
			name:    "select all",
			query:   "SELECT * FROM users",
			wantErr: false,
		},
		{
			name:    "join query",
			query:   "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plans, err := opt.AnalyzeQuery(ctx, tt.query, 1)
			if (err != nil) != tt.wantErr {
				t.Errorf("AnalyzeQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(plans) == 0 {
				t.Error("AnalyzeQuery() returned empty plans")
			}
		})
	}
}

func TestOptimizer_IsFullTableScan(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	opt := NewOptimizer(db, DefaultOptimizerConfig(), logger)

	ctx := context.Background()

	tests := []struct {
		name     string
		query    string
		wantScan bool
	}{
		{
			name:     "primary key lookup",
			query:    "SELECT * FROM users WHERE id = ?",
			wantScan: false,
		},
		{
			name:     "indexed column lookup",
			query:    "SELECT * FROM users WHERE email = ?",
			wantScan: false,
		},
		{
			name:     "non-indexed column lookup",
			query:    "SELECT * FROM users WHERE name = ?",
			wantScan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isScan, err := opt.IsFullTableScan(ctx, tt.query, "test")
			if err != nil {
				t.Errorf("IsFullTableScan() error = %v", err)
				return
			}
			if isScan != tt.wantScan {
				t.Errorf("IsFullTableScan() = %v, want %v", isScan, tt.wantScan)
			}
		})
	}
}

func TestOptimizer_GetTableStats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	opt := NewOptimizer(db, DefaultOptimizerConfig(), logger)

	ctx := context.Background()

	stats, err := opt.GetTableStats(ctx)
	if err != nil {
		t.Fatalf("GetTableStats() error = %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("GetTableStats() returned %d tables, want 2", len(stats))
	}

	// Find users table
	var usersStats *TableStats
	for i := range stats {
		if stats[i].Name == "users" {
			usersStats = &stats[i]
			break
		}
	}

	if usersStats == nil {
		t.Fatal("GetTableStats() did not return users table")
	}

	if usersStats.RowCount != 100 {
		t.Errorf("users table row count = %d, want 100", usersStats.RowCount)
	}

	if usersStats.IndexCount < 2 {
		t.Errorf("users table index count = %d, want >= 2", usersStats.IndexCount)
	}
}

func TestOptimizer_GetIndexes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	opt := NewOptimizer(db, DefaultOptimizerConfig(), logger)

	ctx := context.Background()

	indexes, err := opt.GetIndexes(ctx, "users")
	if err != nil {
		t.Fatalf("GetIndexes() error = %v", err)
	}

	if len(indexes) < 2 {
		t.Errorf("GetIndexes() returned %d indexes, want >= 2", len(indexes))
	}

	// Check for email index
	var emailIndex *IndexInfo
	for i := range indexes {
		if indexes[i].Name == "idx_users_email" {
			emailIndex = &indexes[i]
			break
		}
	}

	if emailIndex == nil {
		t.Fatal("GetIndexes() did not return idx_users_email")
	}

	if len(emailIndex.Columns) != 1 || emailIndex.Columns[0] != "email" {
		t.Errorf("idx_users_email columns = %v, want [email]", emailIndex.Columns)
	}
}

func TestOptimizer_ExecuteWithStats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultOptimizerConfig()
	opt := NewOptimizer(db, config, logger)

	ctx := context.Background()

	stats, err := opt.ExecuteWithStats(ctx, "SELECT * FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("ExecuteWithStats() error = %v", err)
	}

	// Duration may be 0 on very fast systems, so we only check it's non-negative
	if stats.Duration < 0 {
		t.Error("ExecuteWithStats() duration < 0")
	}

	if stats.RowsReturned != 1 {
		t.Errorf("ExecuteWithStats() rows = %d, want 1", stats.RowsReturned)
	}

	// Check optimizer stats - TotalQueries should be incremented
	optStats := opt.GetStats()
	if optStats.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", optStats.TotalQueries)
	}
	// SlowQueries depends on actual query duration vs threshold, so we don't assert a specific value
}

func TestOptimizer_Cache(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultOptimizerConfig()
	config.EnableQueryCache = true
	config.QueryCacheTTL = time.Hour
	opt := NewOptimizer(db, config, logger)

	ctx := context.Background()

	// First query - cache miss
	_, err := opt.ExecuteWithStats(ctx, "SELECT * FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("First ExecuteWithStats() error = %v", err)
	}

	stats := opt.GetStats()
	if stats.CacheMisses != 1 {
		t.Errorf("CacheMisses = %d, want 1", stats.CacheMisses)
	}

	// Clear cache and verify
	opt.ClearCache()

	// After clear, cache should be empty
	opt.cacheMu.RLock()
	cacheLen := len(opt.cache)
	opt.cacheMu.RUnlock()

	if cacheLen != 0 {
		t.Errorf("Cache length after clear = %d, want 0", cacheLen)
	}
}

func TestOptimizer_SuggestIndexes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	opt := NewOptimizer(db, DefaultOptimizerConfig(), logger)

	ctx := context.Background()

	queries := []string{
		"SELECT * FROM users WHERE name = ?",  // No index on name
		"SELECT * FROM users WHERE email = ?", // Has index
	}

	suggestions, err := opt.SuggestIndexes(ctx, queries)
	if err != nil {
		t.Fatalf("SuggestIndexes() error = %v", err)
	}

	// Should suggest index for name column
	if len(suggestions) == 0 {
		t.Error("SuggestIndexes() returned no suggestions")
	}

	found := false
	for _, s := range suggestions {
		if contains(s, "users") {
			found = true
			break
		}
	}

	if !found {
		t.Error("SuggestIndexes() did not suggest index for users table")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestParseIndexColumns(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "single column",
			sql:  "CREATE INDEX idx_users_email ON users(email)",
			want: []string{"email"},
		},
		{
			name: "multiple columns",
			sql:  "CREATE INDEX idx_users_name_email ON users(name, email)",
			want: []string{"name", "email"},
		},
		{
			name: "with order",
			sql:  "CREATE INDEX idx_users_created ON users(created_at DESC)",
			want: []string{"created_at"},
		},
		{
			name: "empty",
			sql:  "CREATE INDEX idx_test ON test",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIndexColumns(tt.sql)
			if len(got) != len(tt.want) {
				t.Errorf("parseIndexColumns() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseIndexColumns()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTruncateQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int // max length
	}{
		{
			name:  "short query",
			query: "SELECT * FROM users",
			want:  50,
		},
		{
			name:  "long query",
			query: "SELECT id, name, email, created_at, updated_at, status, role, department, manager_id, hire_date, salary, bonus, commission, address, city, state, country, postal_code, phone, mobile, fax FROM employees WHERE department = 'Engineering' AND status = 'active' AND hire_date > '2020-01-01'",
			want:  203, // 200 + "..."
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateQuery(tt.query)
			if len(got) > tt.want {
				t.Errorf("truncateQuery() length = %d, want <= %d", len(got), tt.want)
			}
		})
	}
}
