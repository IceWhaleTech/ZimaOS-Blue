package update

import (
	"sync"
	"testing"
)

func TestVersionTagRegex_InitializeOnDemand(t *testing.T) {
	originalPattern := tagsRE
	originalOnce := tagsREOnce

	tagsRE = nil
	tagsREOnce = sync.Once{}
	t.Cleanup(func() {
		tagsRE = originalPattern
		tagsREOnce = originalOnce
	})

	if tagsRE != nil {
		t.Fatal("expected version tag regex to start nil")
	}

	v, err := ParseVersion("1.2.3-beta1")
	if err != nil {
		t.Fatalf("ParseVersion returned error: %v", err)
	}
	if v.Prerelease != "beta" || v.PreNum != 1 {
		t.Fatalf("ParseVersion() = %#v, want prerelease beta1", v)
	}
	if tagsRE == nil {
		t.Fatal("expected version tag regex to initialize on first parse")
	}
}
