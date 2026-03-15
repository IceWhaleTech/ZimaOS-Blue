package security

import "testing"

func TestNewHandler_DefaultPasswordMinLength(t *testing.T) {
	handler := NewHandler(nil)

	if handler.settings.PasswordMinLength != 6 {
		t.Fatalf("PasswordMinLength = %d, want 6", handler.settings.PasswordMinLength)
	}
}
