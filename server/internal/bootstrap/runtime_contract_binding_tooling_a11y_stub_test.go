//go:build !darwin && !windows

package bootstrap

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestBindTooling_DoesNotRegisterHostA11yToolOnUnsupportedHost(t *testing.T) {
	registry := tools.NewRegistry()
	binding := &runtimeContractBinding{}

	binding.BindTooling(routeRuntimeContractToolingOptions{
		registry: registry,
	})

	if registry.Get("a11y") != nil {
		t.Fatal("expected host a11y tool to stay unregistered on unsupported hosts")
	}
}
