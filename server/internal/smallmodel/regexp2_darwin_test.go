//go:build darwin

package smallmodel

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSmallModelPackageOnDarwinAvoidsRegexp2StartupDependency(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate current test file")
	}

	serverDir := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	cmd := exec.Command("go", "list", "-deps", "./internal/smallmodel")
	cmd.Dir = serverDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, out)
	}

	if strings.Contains(string(out), "github.com/dlclark/regexp2") {
		t.Fatalf("expected darwin smallmodel package deps to avoid regexp2, got:\n%s", out)
	}
}
