package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestApplyPrimaryDatabasePoolConfig(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "pool.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	applyPrimaryDatabasePoolConfig(db, config.DatabasePerfConfig{
		PoolSize:        2,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 50 * time.Millisecond,
	})

	if got := db.Stats().MaxOpenConnections; got != 2 {
		t.Fatalf("MaxOpenConnections = %d, want 2", got)
	}

	conn1, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn() first error = %v", err)
	}
	conn2, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn() second error = %v", err)
	}
	conn1.Close()
	conn2.Close()

	if got := db.Stats().Idle; got < 1 {
		t.Fatalf("Idle after returning conns = %d, want >= 1", got)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := db.Ping(); err != nil {
			t.Fatalf("db.Ping() error = %v", err)
		}
		if got := db.Stats().Idle; got < 2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("idle connections did not shrink before deadline; idle=%d", db.Stats().Idle)
}
