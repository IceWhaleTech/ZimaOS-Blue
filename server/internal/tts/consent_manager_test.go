package tts

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestConsentManager_SetGetHasConsent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	manager := NewConsentManager(db)
	ctx := context.Background()

	if err := manager.SetConsent(ctx, "user-1", "edge-tts", true, "v1"); err != nil {
		t.Fatalf("set consent: %v", err)
	}

	consent, err := manager.GetConsent(ctx, "user-1", "edge-tts")
	if err != nil {
		t.Fatalf("get consent: %v", err)
	}
	if consent == nil {
		t.Fatal("expected consent record")
	}
	if !consent.ConsentGiven {
		t.Fatal("expected consent to be granted")
	}
	if consent.ConsentDate == nil {
		t.Fatal("expected consent date for granted consent")
	}
	if consent.ConsentVersion != "v1" {
		t.Fatalf("expected consent version v1, got %q", consent.ConsentVersion)
	}

	hasConsent, err := manager.HasConsent(ctx, "user-1", "edge-tts")
	if err != nil {
		t.Fatalf("has consent: %v", err)
	}
	if !hasConsent {
		t.Fatal("expected HasConsent to return true")
	}

	if err := manager.SetConsent(ctx, "user-1", "edge-tts", false, "v2"); err != nil {
		t.Fatalf("revoke consent: %v", err)
	}

	updated, err := manager.GetConsent(ctx, "user-1", "edge-tts")
	if err != nil {
		t.Fatalf("get updated consent: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated consent record")
	}
	if updated.ConsentGiven {
		t.Fatal("expected consent to be revoked")
	}
	if updated.ConsentDate != nil {
		t.Fatal("expected consent date to be cleared when revoked")
	}
	if updated.ConsentVersion != "v2" {
		t.Fatalf("expected consent version v2, got %q", updated.ConsentVersion)
	}
}

func TestConsentManager_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tts-consent.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open writer db: %v", err)
	}
	defer writeDB.Close()

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("open reader db: %v", err)
	}
	defer readDB.Close()

	manager := NewConsentManagerWithReadDB(writeDB, readDB)
	if manager.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if manager.readDB == manager.db {
		t.Fatal("expected separate read db")
	}

	ctx := context.Background()
	if err := manager.SetConsent(ctx, "user-2", "espeak", true, "v1"); err != nil {
		t.Fatalf("set consent: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	consent, err := manager.GetConsent(ctx, "user-2", "espeak")
	if err != nil {
		t.Fatalf("get consent via reader: %v", err)
	}
	if consent == nil || consent.UserID != "user-2" || consent.Service != "espeak" {
		t.Fatalf("unexpected consent via reader: %+v", consent)
	}

	hasConsent, err := manager.HasConsent(ctx, "user-2", "espeak")
	if err != nil {
		t.Fatalf("has consent via reader: %v", err)
	}
	if !hasConsent {
		t.Fatal("expected consent lookup to work via reader db")
	}
}
