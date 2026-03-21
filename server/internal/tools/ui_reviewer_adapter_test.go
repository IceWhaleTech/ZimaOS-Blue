package tools

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

func TestLazyRodBrowserAdapterDoesNotCacheResolvedService(t *testing.T) {
	svcs := []*browser.RodService{{}, {}}
	resolveCalls := 0
	adapter := NewLazyRodBrowserAdapter(func() *browser.RodService {
		defer func() { resolveCalls++ }()
		return svcs[resolveCalls]
	})

	first, err := adapter.get()
	if err != nil {
		t.Fatalf("get() first error = %v", err)
	}
	if first != svcs[0] {
		t.Fatalf("get() first = %p, want %p", first, svcs[0])
	}

	second, err := adapter.get()
	if err != nil {
		t.Fatalf("get() second error = %v", err)
	}
	if second != svcs[1] {
		t.Fatalf("get() second = %p, want %p", second, svcs[1])
	}
}
