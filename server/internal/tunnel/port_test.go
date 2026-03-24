package tunnel

import "testing"

func TestResolveTargetPort_RequiresConfig(t *testing.T) {
	if _, err := resolveTargetPort(nil); err == nil {
		t.Fatal("resolveTargetPort(nil) should return an error")
	}
}

func TestResolveTargetPort_RequiresPositivePort(t *testing.T) {
	if _, err := resolveTargetPort(&Config{}); err == nil {
		t.Fatal("resolveTargetPort should reject missing port")
	}
}

func TestResolveTargetPort_ReturnsConfiguredPort(t *testing.T) {
	port, err := resolveTargetPort(&Config{Port: 19091})
	if err != nil {
		t.Fatalf("resolveTargetPort returned unexpected error: %v", err)
	}
	if port != 19091 {
		t.Fatalf("resolveTargetPort() = %d, want %d", port, 19091)
	}
}
