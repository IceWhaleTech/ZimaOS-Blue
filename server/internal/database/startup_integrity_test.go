package database

import "testing"

func TestShouldQuickCheckSQLitePathRunsOncePerPathUntilReset(t *testing.T) {
	SetStartupQuickCheckEnabled(true)
	t.Cleanup(func() {
		SetStartupQuickCheckEnabled(true)
	})

	const dbPath = "/tmp/zimaos-blue-startup-integrity.db"

	if !shouldQuickCheckSQLitePath(dbPath) {
		t.Fatal("expected first quick-check decision for path to be true")
	}

	if shouldQuickCheckSQLitePath(dbPath) {
		t.Fatal("expected repeated quick-check decision for same path to be false")
	}

	if !shouldQuickCheckSQLitePath("/tmp/zimaos-blue-startup-integrity-2.db") {
		t.Fatal("expected a different path to still require a quick check")
	}
}

func TestSetStartupQuickCheckEnabledResetsPathTracking(t *testing.T) {
	SetStartupQuickCheckEnabled(true)
	t.Cleanup(func() {
		SetStartupQuickCheckEnabled(true)
	})

	const dbPath = "/tmp/zimaos-blue-startup-integrity-reset.db"

	if !shouldQuickCheckSQLitePath(dbPath) {
		t.Fatal("expected first quick-check decision for path to be true")
	}

	SetStartupQuickCheckEnabled(false)
	if shouldQuickCheckSQLitePath(dbPath) {
		t.Fatal("expected disabled startup quick-check to short-circuit path checks")
	}

	SetStartupQuickCheckEnabled(true)
	if !shouldQuickCheckSQLitePath(dbPath) {
		t.Fatal("expected re-enabling startup quick-check to reset path tracking")
	}
}
