package browser

import (
	"strings"
	"testing"
)

func TestLightpandaBinaryManagerUsesThreeNonJsdelivrSources(t *testing.T) {
	manager := NewLightpandaBinaryManager(DefaultConfig())
	sources := manager.downloadSources()
	if len(sources) != 3 {
		t.Fatalf("downloadSources() = %d sources, want 3", len(sources))
	}
	for _, source := range sources {
		if strings.Contains(strings.ToLower(source), "jsdelivr") {
			t.Fatalf("download source %q should not use jsdelivr", source)
		}
	}
}
