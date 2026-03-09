package whatsapp

import (
	"context"
	"testing"
)

func TestValidator_Validate(t *testing.T) {
	v := NewValidator()

	if result := v.Validate(context.Background(), map[string]string{}); result.Success {
		t.Fatal("expected missing phone_number to fail")
	}

	ok := v.Validate(context.Background(), map[string]string{
		"phone_number": "+1807890",
		"session_path": t.TempDir(),
		"cli_path":     "/bin/echo",
	})
	if !ok.Success {
		t.Fatalf("expected validator success, got %#v", ok)
	}

	missingCLI := v.Validate(context.Background(), map[string]string{
		"phone_number": "+1807890",
		"session_path": t.TempDir(),
		"cli_path":     "/definitely/missing/wacli",
	})
	if missingCLI.Success {
		t.Fatal("expected missing cli_path to fail")
	}
}
