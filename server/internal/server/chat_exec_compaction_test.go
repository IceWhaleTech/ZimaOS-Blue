package server

import (
	"strings"
	"testing"
)

func TestCompactExecPayloadForLLM_PreservesSafeFileReadStdout(t *testing.T) {
	stdout := strings.Repeat("A", 4559)

	got := compactExecPayloadForLLM(map[string]interface{}{
		"status":  "completed",
		"command": "cat ai_blog.txt",
		"stdout":  stdout,
	})

	out, ok := got["stdout"].(string)
	if !ok {
		t.Fatalf("stdout type = %T, want string", got["stdout"])
	}
	if out != stdout {
		t.Fatalf("stdout len = %d, want %d", len(out), len(stdout))
	}
}

func TestCompactExecPayloadForLLM_StillTruncatesGenericExecStdout(t *testing.T) {
	stdout := strings.Repeat("B", maxLLMToolStdoutBytes+600)

	got := compactExecPayloadForLLM(map[string]interface{}{
		"status":  "completed",
		"command": "python3 -c \"print('hello')\"",
		"stdout":  stdout,
	})

	out, ok := got["stdout"].(string)
	if !ok {
		t.Fatalf("stdout type = %T, want string", got["stdout"])
	}
	if len(out) >= len(stdout) {
		t.Fatalf("stdout len = %d, want truncation below %d", len(out), len(stdout))
	}
	if !strings.HasSuffix(out, "\n[truncated]") {
		t.Fatalf("stdout = %q, want truncated suffix", out[len(out)-min(len(out), 32):])
	}
}
