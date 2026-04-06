package main

import (
	"strings"
	"testing"
)

func TestRootCommandDoesNotExposePluginManagement(t *testing.T) {
	t.Helper()

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "plugins" {
			t.Fatalf("unexpected plugins command still registered")
		}
	}

	longDescription := strings.ToLower(rootCmd.Long)
	if strings.Contains(longDescription, "plugins") {
		t.Fatalf("root command long description still mentions plugins: %q", rootCmd.Long)
	}
}
