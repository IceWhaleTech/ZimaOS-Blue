package bootstrap

import (
	"context"
	"fmt"
	"strings"
)

func (a sessionListAdapter) HandleRuntimeSessionAction(ctx context.Context, action string, args map[string]interface{}) (interface{}, error) {
	if a.agentSessions == nil {
		return nil, fmt.Errorf("runtime-aware sessions are not configured")
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "list":
		return a.handleRuntimeSessionListAction(ctx, args)
	case "history":
		return a.handleRuntimeSessionHistoryAction(args)
	case "status":
		return a.handleRuntimeSessionStatusAction(args)
	case "spawn":
		return a.handleRuntimeSessionSpawnAction(ctx, args)
	case "send":
		return a.handleRuntimeSessionSendAction(ctx, args)
	default:
		return nil, fmt.Errorf("unknown runtime-aware sessions action %q", action)
	}
}
