package bootstrap

import (
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
)

func bindRouteRuntimeHTTPBootstrap(state *routeRegistrationState) {
	connManager := connection.NewManager(10000, 5*time.Second)
	state.e.Use(connManager.Middleware())
	state.setHTTPBootstrap(connManager, state.e.Group("/api/v1"), state.e.Group("/api"))
}
