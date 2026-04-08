package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDataDir_DefaultsUnderZimaOSBlueHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("UserHomeDir() error = %v, home=%q", err, home)
	}

	want := filepath.Join(home, ".zimaos-blue", "data")
	if got := getDataDir(); filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("getDataDir() = %q, want %q", got, want)
	}
}
