package tools

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestBrowserSiteAllowlistStoreMatchFallsBackToDefaultApprover(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	store, err := NewBrowserSiteAllowlistStore(db)
	if err != nil {
		t.Fatalf("NewBrowserSiteAllowlistStore() error = %v", err)
	}
	if err := store.Add("https://www.zhihu.com", ""); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	entry := store.Match("https://www.zhihu.com/question/1", "alice")
	if entry == nil {
		t.Fatal("Match() = nil, want default/global entry")
	}
	if entry.Origin != "https://www.zhihu.com" {
		t.Fatalf("Match().Origin = %q, want https://www.zhihu.com", entry.Origin)
	}
}
