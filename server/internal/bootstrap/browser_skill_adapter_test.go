package bootstrap

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

func TestLazyBrowserSkillAdapterDoesNotCacheResolvedService(t *testing.T) {
	svcs := []*browser.RodService{{}, {}}
	resolveCalls := 0
	adapter := newLazyBrowserSkillAdapter(func() *browser.RodService {
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

func TestLeaseAwareBrowserSkillAdapterReleasesService(t *testing.T) {
	svc := &browser.RodService{}
	releases := 0
	adapter := newLeaseAwareBrowserSkillAdapter(func() (*browser.RodService, func(), error) {
		return svc, func() { releases++ }, nil
	})

	acquired, release, err := adapter.acquire()
	if err != nil {
		t.Fatalf("acquire() error = %v", err)
	}
	if acquired != svc {
		t.Fatalf("acquire() service = %p, want %p", acquired, svc)
	}
	if releases != 0 {
		t.Fatalf("release count before release = %d, want 0", releases)
	}

	release()
	if releases != 1 {
		t.Fatalf("release count after release = %d, want 1", releases)
	}
}
