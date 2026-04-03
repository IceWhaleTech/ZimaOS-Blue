package agentcore

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseProtocolKindSupportsACPAndA2A(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  ProtocolKind
	}{
		{input: "acp", want: ProtocolACP},
		{input: "ACP", want: ProtocolACP},
		{input: "a2a", want: ProtocolA2A},
		{input: "A2A", want: ProtocolA2A},
	} {
		got, err := ParseProtocolKind(tc.input)
		if err != nil {
			t.Fatalf("ParseProtocolKind(%q) error = %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("ParseProtocolKind(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseProtocolKindRejectsUnsupportedValues(t *testing.T) {
	if _, err := ParseProtocolKind("grpc"); err == nil {
		t.Fatal("ParseProtocolKind(grpc) succeeded, want error")
	}
}

func TestPackageDoesNotImportInternalPackages(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "github.com/IceWhaleTech/ZimaOS-Blue/server/agentcore")
	cmd.Dir = moduleRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, output)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "/internal/") {
			t.Fatalf("agentcore package depends on internal package %q", line)
		}
	}
}
