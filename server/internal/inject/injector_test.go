package inject

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryStoreInjectorInjectMessageNilStore(t *testing.T) {
	injector := NewMemoryStoreInjector(nil)

	_, err := injector.InjectMessage(context.Background(), "user-1", "conv-1", "hello")
	if err == nil {
		t.Fatal("expected error for nil memory store")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}
