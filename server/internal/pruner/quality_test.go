package pruner

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestPruningQuality_RealisticCode(t *testing.T) {
	code := `package auth

import (
	"crypto/bcrypt"
	"database/sql"
	"errors"
	"time"
	"fmt"
)

// User represents an authenticated user.
type User struct {
	ID        int64
	Email     string
	Password  string
	CreatedAt time.Time
}

// AuthService handles user authentication.
type AuthService struct {
	db *sql.DB
}

// Authenticate verifies user credentials and returns a session token.
func (s *AuthService) Authenticate(email, password string) (string, error) {
	var user User
	err := s.db.QueryRow("SELECT id, email, password FROM users WHERE email = ?", email).
		Scan(&user.ID, &user.Email, &user.Password)
	if err != nil {
		return "", errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	return generateToken(user.ID), nil
}

// generateToken creates a JWT token for the user.
func generateToken(userID int64) string {
	return fmt.Sprintf("token-%d-%d", userID, time.Now().Unix())
}

// ListUsers returns all users (admin only).
func (s *AuthService) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, email, created_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Email, &u.CreatedAt)
		users = append(users, u)
	}
	return users, nil
}

// DeleteUser removes a user by ID.
func (s *AuthService) DeleteUser(id int64) error {
	_, err := s.db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// UpdatePassword changes a user's password.
func (s *AuthService) UpdatePassword(id int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE users SET password = ? WHERE id = ?", hash, id)
	return err
}

// GetUserByID retrieves a user by their ID.
func (s *AuthService) GetUserByID(id int64) (*User, error) {
	var u User
	err := s.db.QueryRow("SELECT id, email, created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Email, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
`

	backend := NewIRPruner(Config{Threshold: 0.5, CacheCapacity: 64, MinLines: 5})
	resp, err := backend.Prune(context.Background(), PruneRequest{
		Content:   code,
		Query:     "authenticate login password",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Original: %d lines, %d tokens", resp.OriginalLines, resp.OriginalTokens)
	t.Logf("Pruned:   %d lines, %d tokens (%.1f%% reduction)", resp.KeptLines, resp.PrunedTokens, (1.0-resp.CompressionRate)*100)
	t.Logf("\n=== PRUNED OUTPUT ===\n%s", resp.PrunedContent)

	kept := resp.PrunedContent

	// Relevant functions should be kept
	checks := []struct {
		name     string
		substr   string
		wantKept bool
	}{
		{"Authenticate()", "Authenticate", true},
		{"generateToken()", "generateToken", true},
		{"UpdatePassword()", "UpdatePassword", true},
		{"imports", "import", true},
		{"User struct", "type User struct", true},
		{"ListUsers() [irrelevant]", "ListUsers", false},
		{"DeleteUser() [irrelevant]", "DeleteUser", false},
	}

	for _, c := range checks {
		found := strings.Contains(kept, c.substr)
		if c.wantKept && !found {
			t.Errorf("❌ %s: LOST (should be kept)", c.name)
		} else if c.wantKept && found {
			t.Logf("✅ %s: KEPT", c.name)
		} else if !c.wantKept && !found {
			t.Logf("✅ %s: PRUNED (correctly removed)", c.name)
		} else {
			t.Logf("⚠️  %s: KEPT (ideally would be pruned, but acceptable)", c.name)
		}
	}

	// Check filtered markers exist
	if !strings.Contains(kept, "(filtered") {
		t.Error("expected (filtered N lines) markers in output")
	} else {
		count := strings.Count(kept, "(filtered")
		t.Logf("📋 %d filtered markers in output", count)
	}

	// Verify the LLM can still understand the code structure
	if !strings.Contains(kept, "func") {
		t.Error("no function signatures in pruned output — LLM would lose context")
	}

	reduction := 1.0 - resp.CompressionRate
	t.Logf("\n📊 Summary: %.0f%% tokens saved, Recall of relevant functions: check above", reduction*100)
}

func TestPruningQuality_RealisticDoc(t *testing.T) {
	doc := `## Authentication System

The authentication system provides secure user login and session management.
It supports OAuth2, JWT tokens, and traditional username/password authentication.
Sessions are stored in Redis with configurable TTL.

## Database Architecture

The database layer uses PostgreSQL with connection pooling via pgxpool.
Migrations are managed through golang-migrate.
Read replicas are supported for horizontal scaling.

## Caching Strategy

The caching layer implements a two-level cache:
- L1: In-memory LRU cache (256 slots)
- L2: Redis-backed distributed cache
Cache invalidation uses pub/sub for cross-node consistency.

## Deployment

The application is deployed on Kubernetes using Helm charts.
CI/CD pipeline runs on GitHub Actions.
Monitoring uses Prometheus + Grafana stack.

## API Rate Limiting

Rate limiting is implemented per-user using a token bucket algorithm.
Default limits: 100 requests/minute for free tier, 1000 for paid.
Rate limit headers are included in all API responses.

## Logging

Structured logging uses zerolog with JSON output.
Log levels: debug, info, warn, error, fatal.
Logs are shipped to Elasticsearch via Filebeat.
`

	backend := NewIRPruner(Config{Threshold: 0.5, CacheCapacity: 64, MinLines: 5})
	resp, err := backend.Prune(context.Background(), PruneRequest{
		Content:   doc,
		Query:     "authentication login session",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Original: %d lines, %d tokens", resp.OriginalLines, resp.OriginalTokens)
	t.Logf("Pruned:   %d lines, %d tokens (%.1f%% reduction)", resp.KeptLines, resp.PrunedTokens, (1.0-resp.CompressionRate)*100)
	t.Logf("\n=== PRUNED OUTPUT ===\n%s", resp.PrunedContent)

	kept := resp.PrunedContent

	if !strings.Contains(kept, "Authentication") {
		t.Error("❌ Authentication section: LOST")
	} else {
		t.Log("✅ Authentication section: KEPT")
	}

	fmt.Println()
	reduction := 1.0 - resp.CompressionRate
	t.Logf("📊 Summary: %.0f%% tokens saved", reduction*100)
}
