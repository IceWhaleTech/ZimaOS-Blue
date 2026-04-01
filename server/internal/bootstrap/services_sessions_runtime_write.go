package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
)

func runtimeSessionProfileID(args map[string]interface{}) string {
	profileID := strings.TrimSpace(runtimeCompatString(args, "profile_id", "profileId", "provider", "provider_id", "providerId"))
	if profileID != "" {
		return profileID
	}
	switch strings.ToLower(strings.TrimSpace(runtimeCompatString(args, "runtime"))) {
	case "a2a":
		return "generic-a2a"
	default:
		return runtimeCompatString(args, "agent", "agent_id", "agentId")
	}
}

func runtimeSessionTitle(args map[string]interface{}) string {
	title := runtimeCompatString(args, "title", "name")
	if title != "" {
		return title
	}
	return runtimeCompatTitle(runtimeCompatString(args, "input", "prompt", "message", "content", "text"))
}

func (a sessionListAdapter) handleRuntimeSessionSpawnAction(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	profileID := runtimeSessionProfileID(args)
	if profileID == "" {
		return nil, fmt.Errorf("profile_id is required for runtime-aware spawn")
	}
	detail, err := a.agentSessions.CreateSession(ctx, agentsessions.CreateSessionParams{
		ProfileID:      profileID,
		UserID:         runtimeSessionUserID(ctx, args),
		Name:           runtimeSessionTitle(args),
		CWD:            runtimeCompatString(args, "cwd", "workdir", "work_dir", "working_dir"),
		InitialMessage: runtimeCompatString(args, "initial_message", "initialMessage"),
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"session": detail.Session,
		"profile": detail.Profile,
		"id":      detail.Session.ID,
		"created": true,
	}, nil
}

func (a sessionListAdapter) handleRuntimeSessionSendAction(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID := runtimeSessionID(args)
	run, err := a.agentSessions.SendMessage(ctx, agentsessions.SendMessageParams{
		SessionID: sessionID,
		Message:   runtimeCompatString(args, "message", "content", "text", "input", "prompt"),
	})
	if err != nil {
		return nil, err
	}
	detail, err := a.agentSessions.GetSessionDetail(sessionID, 10)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"session": detail.Session,
		"profile": detail.Profile,
		"run":     run,
		"sent":    true,
	}, nil
}
