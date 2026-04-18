package bootstrap

import (
	"context"
	"testing"
)

func TestBindRouteRuntimeCorePhase_SkipsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := &routeRegistrationState{
		deps: &RoutesDeps{Ctx: ctx},
	}

	bindRouteRuntimeCorePhase(state)
}
