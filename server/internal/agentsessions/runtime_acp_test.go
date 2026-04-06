package agentsessions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
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

func TestACPRuntimeVerifyProfileFallsBackToNpxForClaudeAgentACP(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "claude-npx.log")
	writeACPInitializeStub(t, filepath.Join(tempDir, "npx"), logPath)
	t.Setenv("PATH", tempDir)
	t.Setenv("HOME", tempDir)

	runtime := NewACPRuntime(nil, zap.NewNop(), nil)
	_, err := runtime.VerifyProfile(context.Background(), AgentProfile{
		ID:       "claude",
		Protocol: ProtocolACP,
		Command:  []string{"claude-agent-acp"},
	})
	if err != nil {
		t.Fatalf("VerifyProfile() error = %v, want nil", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", logPath, err)
	}
	if got := strings.TrimSpace(string(data)); got != "-y @zed-industries/claude-agent-acp" {
		t.Fatalf("npx args = %q, want %q", got, "-y @zed-industries/claude-agent-acp")
	}
}

func TestACPRuntimeVerifyProfileFallsBackToNpxForCodexACP(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "codex-npx.log")
	writeACPInitializeStub(t, filepath.Join(tempDir, "npx"), logPath)
	t.Setenv("PATH", tempDir)
	t.Setenv("HOME", tempDir)

	runtime := NewACPRuntime(nil, zap.NewNop(), nil)
	_, err := runtime.VerifyProfile(context.Background(), AgentProfile{
		ID:       "codex",
		Protocol: ProtocolACP,
		Command:  []string{"codex-acp"},
	})
	if err != nil {
		t.Fatalf("VerifyProfile() error = %v, want nil", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", logPath, err)
	}
	if got := strings.TrimSpace(string(data)); got != "@zed-industries/codex-acp" {
		t.Fatalf("npx args = %q, want %q", got, "@zed-industries/codex-acp")
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

func writeACPInitializeStub(t *testing.T, path, logPath string) {
	t.Helper()
	script := fmt.Sprintf(`#!/usr/bin/python3
import json
import sys
import time

with open(%q, "w", encoding="utf-8") as handle:
    handle.write(" ".join(sys.argv[1:]))

line = sys.stdin.readline()
if not line:
    sys.exit(1)
request = json.loads(line)
response = {
    "jsonrpc": "2.0",
    "id": request.get("id"),
    "result": {
        "protocolVersion": 1,
        "agentCapabilities": {"loadSession": False},
        "agentInfo": {"name": "stub"},
    },
}
sys.stdout.write(json.dumps(response) + "\n")
sys.stdout.flush()
sys.stdin.read()
`, logPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func writeACPConversationStub(t *testing.T, path, logPath, replyText string) {
	t.Helper()
	script := fmt.Sprintf(`#!/usr/bin/python3
import json
import sys
import time

with open(%q, "w", encoding="utf-8") as handle:
    handle.write(" ".join(sys.argv[1:]))

session_id = "stub-session"
for line in sys.stdin:
    if not line.strip():
        continue
    request = json.loads(line)
    method = request.get("method")
    request_id = request.get("id")
    if method == "initialize":
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {
                "protocolVersion": 1,
                "agentCapabilities": {"loadSession": False},
                "agentInfo": {"name": "stub"},
            },
        }
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()
    elif method == "session/new":
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {"sessionId": session_id},
        }
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()
    elif method == "session/prompt":
        time.sleep(0.05)
        update = {
            "jsonrpc": "2.0",
            "method": "session/update",
            "params": {
                "sessionId": session_id,
                "update": {
                    "sessionUpdate": "agent_message_chunk",
                    "content": {"text": %q},
                },
            },
        }
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {"stopReason": "end_turn"},
        }
        sys.stdout.write(json.dumps(update) + "\n")
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()
`, logPath, replyText)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
