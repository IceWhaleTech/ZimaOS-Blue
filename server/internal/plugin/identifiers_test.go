package plugin

import "testing"

func TestValidatePluginID(t *testing.T) {
	t.Run("accepts safe plugin id", func(t *testing.T) {
		id, err := ValidatePluginID("demo-plugin_1")
		if err != nil {
			t.Fatalf("ValidatePluginID returned error: %v", err)
		}
		if id != "demo-plugin_1" {
			t.Fatalf("id = %q, want %q", id, "demo-plugin_1")
		}
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		if _, err := ValidatePluginID("../escape"); err == nil {
			t.Fatal("expected traversal plugin id to be rejected")
		}
	})
}
