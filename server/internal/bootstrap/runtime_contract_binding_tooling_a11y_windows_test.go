//go:build windows

package bootstrap

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestBindTooling_RegistersHostA11yToolOnWindows(t *testing.T) {
	registry := tools.NewRegistry()
	binding := &runtimeContractBinding{}

	binding.BindTooling(routeRuntimeContractToolingOptions{
		registry: registry,
	})

	if registry.Get("a11y") == nil {
		t.Fatal("expected host a11y tool to be registered on windows")
	}
}
