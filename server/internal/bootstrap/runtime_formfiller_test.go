package bootstrap

import "testing"

func TestNewRuntimeFormfillerHandler_Disabled(t *testing.T) {
	if handler := NewRuntimeFormfillerHandler(); handler != nil {
		t.Fatalf("NewRuntimeFormfillerHandler() = %#v, want nil", handler)
	}
}
