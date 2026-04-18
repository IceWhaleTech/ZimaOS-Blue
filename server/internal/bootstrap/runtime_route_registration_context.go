package bootstrap

func routeRegistrationContextCanceled(state *routeRegistrationState) bool {
	return state != nil && state.deps != nil && state.deps.Ctx != nil && state.deps.Ctx.Err() != nil
}
