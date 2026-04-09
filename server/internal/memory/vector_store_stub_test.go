//go:build !cgo || windows

package memory

import (
	"errors"
	"strings"
	"testing"
)

func TestNewVectorStoreStubReturnsUnavailableError(t *testing.T) {
	store, err := NewVectorStore(VectorStoreConfig{})
	if store != nil {
		t.Fatalf("NewVectorStore() store = %#v, want nil", store)
	}
	if !errors.Is(err, errVectorStoreNoCGO) {
		t.Fatalf("NewVectorStore() error = %v, want %v", err, errVectorStoreNoCGO)
	}
	if !strings.Contains(err.Error(), "unavailable in this build") {
		t.Fatalf("NewVectorStore() error = %q, want unavailable-build guidance", err.Error())
	}
}
