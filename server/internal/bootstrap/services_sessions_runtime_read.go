package bootstrap

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
)

func runtimeSessionProtocol(args map[string]interface{}) (agentsessions.ProtocolKind, error) {
	runtime := strings.ToLower(strings.TrimSpace(runtimeCompatString(args, "runtime")))
	if runtime == "" {
		return "", nil
	}
	return agentsessions.ParseProtocolKind(runtime)
}

func (a sessionListAdapter) handleRuntimeSessionListAction(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	protocol, err := runtimeSessionProtocol(args)
	if err != nil {
		return nil, err
	}
	sessions, err := a.agentSessions.ListSessions(
		runtimeCompatInt(args, "limit", 20),
		runtimeCompatInt(args, "offset", 0),
		runtimeSessionUserID(ctx, args),
		protocol,
	)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	}, nil
}

func (a sessionListAdapter) handleRuntimeSessionHistoryAction(args map[string]interface{}) (interface{}, error) {
	sessionID := runtimeSessionID(args)
	limit := runtimeCompatInt(args, "limit", 50)
	history, err := a.agentSessions.GetSessionHistory(sessionID, limit)
	if err != nil {
		return nil, err
	}
	detail, err := a.agentSessions.GetSessionDetail(sessionID, limit)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"session": detail.Session,
		"profile": detail.Profile,
		"history": history,
		"count":   len(history),
	}, nil
}

func (a sessionListAdapter) handleRuntimeSessionStatusAction(args map[string]interface{}) (interface{}, error) {
	return a.agentSessions.GetSessionDetail(runtimeSessionID(args), 20)
}
