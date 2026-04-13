package plugin

import (
	"sync"
	"testing"
)

func TestPluginSemverRegex_InitializeOnDemand(t *testing.T) {
	originalPattern := semverRegex
	originalOnce := semverRegexOnce

	semverRegex = nil
	semverRegexOnce = sync.Once{}
	t.Cleanup(func() {
		semverRegex = originalPattern
		semverRegexOnce = originalOnce
	})

	if semverRegex != nil {
		t.Fatal("expected plugin semver regex to start nil")
	}

	v, err := parseVersion("v1.2.3-beta.1")
	if err != nil {
		t.Fatalf("parseVersion returned error: %v", err)
	}
	if v.major != 1 || v.minor != 2 || v.patch != 3 || v.prerelease != "beta.1" {
		t.Fatalf("parseVersion() = %#v, want v1.2.3-beta.1", v)
	}
	if semverRegex == nil {
		t.Fatal("expected plugin semver regex to initialize on first parse")
	}
}
