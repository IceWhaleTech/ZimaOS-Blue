//go:build !darwin && !windows

package bootstrap

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestBindTooling_DoesNotRegisterHostComputerUseToolOnUnsupportedHost(t *testing.T) {
	registry := tools.NewRegistry()
	binding := &runtimeContractBinding{}

	binding.BindTooling(routeRuntimeContractToolingOptions{
		registry: registry,
	})

	if registry.Get("computer_use") != nil {
		t.Fatal("expected host computer_use tool to stay unregistered on unsupported hosts")
	}
}
