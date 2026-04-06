//go:build !linux

package onnx

import "testing"

func TestRuntimeTgzMetadataUnsupportedOnNonLinux(t *testing.T) {
	if got := RuntimeTgzFilename(); got != "" {
		t.Fatalf("RuntimeTgzFilename() = %q, want empty on non-Linux", got)
	}
	if got := RuntimeTgzURL(); got != "" {
		t.Fatalf("RuntimeTgzURL() = %q, want empty on non-Linux", got)
	}
	if got := RuntimeTgzMirrors(); len(got) != 0 {
		t.Fatalf("RuntimeTgzMirrors() = %v, want empty on non-Linux", got)
	}
}
