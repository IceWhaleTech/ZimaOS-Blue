package smallmodel

import (
	"context"
	"strings"
	"testing"
)

func TestManagerDefaults(t *testing.T) {
	m := NewManager(t.TempDir())
	st := m.GetStatus()
	if st.ModelID != ModelID {
		t.Fatalf("model id mismatch: got %q want %q", st.ModelID, ModelID)
	}
	if st.Runtime != RuntimeType {
		t.Fatalf("runtime mismatch: got %q want %q", st.Runtime, RuntimeType)
	}
	if st.Ready {
		t.Fatal("expected model not ready in temp dir")
	}
}

func TestEnsureReadyNoAutoDownload(t *testing.T) {
	m := NewManager(t.TempDir())
	err := m.EnsureReady(context.Background(), false)
	if err == nil {
		t.Fatal("expected error when model missing and auto download disabled")
	}
	if !strings.Contains(err.Error(), "auto download is disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}
