//go:build !darwin && !windows

package a11y

import "testing"

func TestDefaultHostBackend_UnsupportedHostReturnsNil(t *testing.T) {
	if backend := DefaultHostBackend(""); backend != nil {
		t.Fatalf("DefaultHostBackend() = %#v, want nil on unsupported host", backend)
	}
}
