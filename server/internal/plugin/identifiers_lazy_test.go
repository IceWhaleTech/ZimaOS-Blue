package plugin

import (
	"sync"
	"testing"
)

func TestPluginIDRegex_InitializeOnDemand(t *testing.T) {
	originalPattern := pluginIDPattern
	originalOnce := pluginIDPatternOnce

	pluginIDPattern = nil
	pluginIDPatternOnce = sync.Once{}
	t.Cleanup(func() {
		pluginIDPattern = originalPattern
		pluginIDPatternOnce = originalOnce
	})

	if pluginIDPattern != nil {
		t.Fatal("expected plugin id regex to start nil")
	}

	id, err := ValidatePluginID("demo-plugin_2")
	if err != nil {
		t.Fatalf("ValidatePluginID returned error: %v", err)
	}
	if id != "demo-plugin_2" {
		t.Fatalf("id = %q, want %q", id, "demo-plugin_2")
	}
	if pluginIDPattern == nil {
		t.Fatal("expected plugin id regex to initialize on first validation")
	}
}
