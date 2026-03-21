package deepresearch

import (
	"strings"
)

type RoutePolicy struct{}

func normalizeRouteMode(raw RouteMode) (RouteMode, bool) {
	switch RouteMode(strings.ToLower(strings.TrimSpace(string(raw)))) {
	case "", RouteModeWeb:
		return RouteModeWeb, true
	default:
		return "", false
	}
}

func resolveRouteMode(query string, requested RouteMode, _ RoutePolicy) (RouteMode, RouteMode, string, error) {
	requestedMode, ok := normalizeRouteMode(requested)
	if !ok {
		return "", "", "", ErrInvalidRouteMode
	}
	if strings.TrimSpace(query) == "" {
		return requestedMode, RouteModeWeb, "web research is the only supported route", nil
	}
	if requestedMode == RouteModeWeb {
		return RouteModeWeb, RouteModeWeb, "explicit web route requested by caller", nil
	}
	return RouteModeWeb, RouteModeWeb, "web research is the only supported route", nil
}
