package security

import "testing"

func TestNewHandler_DefaultPasswordMinLength(t *testing.T) {
	handler := NewHandler(nil)

	if handler.settings.PasswordMinLength != 8 {
		t.Fatalf("PasswordMinLength = %d, want 8", handler.settings.PasswordMinLength)
	}
}
