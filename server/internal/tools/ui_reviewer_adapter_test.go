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

func TestLeaseAwareRodBrowserAdapterReleasesService(t *testing.T) {
	svc := &browser.RodService{}
	releases := 0
	adapter := NewLeaseAwareRodBrowserAdapter(func() (*browser.RodService, func(), error) {
		return svc, func() { releases++ }, nil
	})

	lease, err := adapter.acquire()
	if err != nil {
		t.Fatalf("acquire() error = %v", err)
	}
	if lease.svc != svc {
		t.Fatalf("acquire() service = %p, want %p", lease.svc, svc)
	}
	if releases != 0 {
		t.Fatalf("release count before close = %d, want 0", releases)
	}

	lease.close()
	if releases != 1 {
		t.Fatalf("release count after close = %d, want 1", releases)
	}
}
