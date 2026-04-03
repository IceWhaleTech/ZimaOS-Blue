package agentsessions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveACPCommandBinaryUsesScenarioPathForClaude(t *testing.T) {
	homeDir := t.TempDir()
	expected := filepath.Join(
		homeDir,
		".vscode",
		"extensions",
		"anthropic.claude-code-2.1.90-darwin-arm64",
		"resources",
		"native-binary",
		"claude",
	)
	writeExecutableForTest(t, expected)

	resolved := resolveACPCommandBinary("claude", func(string) (string, error) {
		return "", errors.New("not found")
	}, homeDir)
	if resolved != expected {
		t.Fatalf("resolveACPCommandBinary(claude) = %q, want %q", resolved, expected)
	}
}

func TestResolveACPCommandBinaryUsesScenarioPathForCodex(t *testing.T) {
	homeDir := t.TempDir()
	expected := filepath.Join(
		homeDir,
		".vscode",
		"extensions",
		"openai.chatgpt-26.325.31654-darwin-arm64",
		"bin",
		"macos-aarch64",
		"codex",
	)
	writeExecutableForTest(t, expected)

	resolved := resolveACPCommandBinary("codex", func(string) (string, error) {
		return "", errors.New("not found")
	}, homeDir)
	if resolved != expected {
		t.Fatalf("resolveACPCommandBinary(codex) = %q, want %q", resolved, expected)
	}
}

func TestResolveACPCommandBinaryUsesScenarioPathForGemini(t *testing.T) {
	homeDir := t.TempDir()
	expected := filepath.Join(
		homeDir,
		".vscode",
		"extensions",
		"google.gemini-cli-1.2.3",
		"bin",
		"macos-aarch64",
		"gemini",
	)
	writeExecutableForTest(t, expected)

	resolved := resolveACPCommandBinary("gemini", func(string) (string, error) {
		return "", errors.New("not found")
	}, homeDir)
	if resolved != expected {
		t.Fatalf("resolveACPCommandBinary(gemini) = %q, want %q", resolved, expected)
	}
}

func TestResolveACPCommandBinaryPrefersLookPathResult(t *testing.T) {
	homeDir := t.TempDir()
	scenarioPath := filepath.Join(
		homeDir,
		".vscode",
		"extensions",
		"anthropic.claude-code-2.1.90-darwin-arm64",
		"resources",
		"native-binary",
		"claude",
	)
	writeExecutableForTest(t, scenarioPath)

	resolved := resolveACPCommandBinary("claude", func(string) (string, error) {
		return "/usr/local/bin/claude", nil
	}, homeDir)
	if resolved != "/usr/local/bin/claude" {
		t.Fatalf("resolveACPCommandBinary(claude) = %q, want %q", resolved, "/usr/local/bin/claude")
	}
}

func writeExecutableForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
