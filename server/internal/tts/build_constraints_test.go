package tts

import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseBuildConstraint(t *testing.T, filename string) constraint.Expr {
	t.Helper()

	path := filepath.Join(filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//go:build ") {
			expr, err := constraint.Parse(line)
			if err != nil {
				t.Fatalf("parse %s build constraint: %v", filename, err)
			}
			return expr
		}
		if line == "" || !strings.HasPrefix(line, "//") {
			break
		}
	}

	t.Fatalf("no //go:build line found in %s", filename)
	return nil
}

func testOK(tags ...string) func(tag string) bool {
	set := map[string]struct{}{}
	for _, tag := range tags {
		set[tag] = struct{}{}
	}
	return func(tag string) bool {
		_, ok := set[tag]
		return ok
	}
}

func TestEspeakBuildConstraintsDoNotOverlapOnWindows(t *testing.T) {
	realExpr := parseBuildConstraint(t, "espeak_ng.go")
	stubExpr := parseBuildConstraint(t, "espeak_ng_stub.go")

	ok := testOK("windows", "espeak", "cgo", "amd64")
	if realExpr.Eval(ok) {
		t.Fatalf("espeak_ng.go must not build for windows+espeak")
	}
	if !stubExpr.Eval(ok) {
		t.Fatalf("espeak_ng_stub.go should build for windows+espeak")
	}
}
