package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIDBPolicy_NoUnexpectedDirectDBOpeners(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(filename)

	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	allowed := map[string]struct{}{
		"context.go": {},
		"main.go":    {},
	}
	patterns := []string{
		`sql.Open(`,
		`openPrimaryDatabaseWithStartupRecovery(`,
		`"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"`,
		`"github.com/mattn/go-sqlite3"`,
		`NewSQLiteStoreWithReadDB(`,
		`MigrateLegacyAnnotations(`,
	}

	for _, path := range files {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		if _, ok := allowed[base]; ok {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(data)
		for _, pattern := range patterns {
			if strings.Contains(text, pattern) {
				t.Fatalf("%s contains disallowed direct DB pattern %q", base, pattern)
			}
		}
	}
}
