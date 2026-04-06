package companion

import "testing"

func TestDefaultConfig_EventBufferSizeIsColdStartBounded(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Performance.EventBufferSize > 1024 {
		t.Fatalf("default companion event buffer size = %d, want <= 1024 for cold-start idle memory", cfg.Performance.EventBufferSize)
	}
}
