package tools

import (
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestRegisterHostA11yTool_SkipsWhenFactoryReturnsNil(t *testing.T) {
	restore := setA11yBackendFactoryForTest(func(string) a11yruntime.Backend {
		return nil
	})
	defer restore()

	registry := NewRegistry()
	tool := RegisterHostA11yTool(registry, "")
	if tool != nil {
		t.Fatalf("RegisterHostA11yTool() = %#v, want nil when backend is unavailable", tool)
	}
	if got := registry.Get("computer_use"); got != nil {
		t.Fatalf("registry.Get(computer_use) = %#v, want nil", got)
	}
}
