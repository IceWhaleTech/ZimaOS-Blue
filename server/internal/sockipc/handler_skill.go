package sockipc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"go.uber.org/zap"
)

const (
	defaultSkillFallbackTimeout = 2 * time.Minute
	analyzeSkillFallbackTimeout = 10 * time.Minute
)

// SkillExecutor is the interface for executing skills by ID.
// Matches skill.Executor.Execute signature without importing the skill package.
type SkillExecutor interface {
	Execute(ctx context.Context, skillID string, input map[string]any) (map[string]string, error)
}

// RegisterSkillFallback sets a fallback handler that forwards unmatched IPC
// commands to the skill executor. This enables `blue <skill_name> key=value`
// to invoke any registered skill via IPC without explicit handler registration.
func RegisterSkillFallback(srv *Server, executor SkillExecutor, log *zap.Logger) {
	srv.HandleFallback(func(ctx context.Context, req *Request) *Response {
		if executor == nil {
			return ErrResponse("unknown cmd: " + req.Cmd)
		}

		if userID := strings.TrimSpace(req.Params["__blue_user_id"]); userID != "" {
			ctx = skill.WithUserID(ctx, userID)
		}

		// Convert params from map[string]string to map[string]any
		input := make(map[string]any, len(req.Params))
		for k, v := range req.Params {
			input[k] = v
		}

		log.Info("skill fallback", zap.String("cmd", req.Cmd), zap.Any("params", req.Params))

		execSkillID := req.Cmd
		execInput := input

		// Support dotted command aliases (e.g. `reminder.add message=...`)
		// by mapping to skill `reminder` with implicit `action=add`.
		if dot := strings.Index(req.Cmd, "."); dot > 0 && dot < len(req.Cmd)-1 {
			base := req.Cmd[:dot]
			action := req.Cmd[dot+1:]
			aliased := make(map[string]any, len(input)+1)
			for k, v := range input {
				aliased[k] = v
			}
			if _, hasAction := aliased["action"]; !hasAction {
				aliased["action"] = action
			}
			execSkillID = base
			execInput = aliased
			log.Info("skill fallback alias mapped",
				zap.String("cmd", req.Cmd),
				zap.String("skill_id", execSkillID),
				zap.String("action", action))
		}

		timeout := defaultSkillFallbackTimeout
		if strings.EqualFold(strings.TrimSpace(execSkillID), "analyze") {
			timeout = analyzeSkillFallbackTimeout
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		data, err := executor.Execute(ctx, execSkillID, execInput)
		if err != nil {
			return ErrResponse(fmt.Sprintf("skill %s: %v", req.Cmd, err))
		}
		return OkResponse(data)
	})
}

// SkillExecutorFunc adapts a skill.Executor into the SkillExecutor interface.
// The adapter converts skill.Result into map[string]string for IPC transport.
type SkillExecutorFunc func(ctx context.Context, skillID string, input map[string]any) (map[string]string, error)

func (f SkillExecutorFunc) Execute(ctx context.Context, skillID string, input map[string]any) (map[string]string, error) {
	return f(ctx, skillID, input)
}

// SkillResultToMap converts a skill result (with Data as any) to map[string]string.
// Prefers flat key-value pairs over nested JSON for readability.
func SkillResultToMap(data any, success bool, errMsg string) map[string]string {
	result := map[string]string{
		"success": fmt.Sprintf("%v", success),
	}
	if errMsg != "" {
		result["error"] = errMsg
	}
	if data == nil {
		return result
	}
	// Try to flatten data into top-level keys
	switch v := data.(type) {
	case map[string]string:
		for k, val := range v {
			result[k] = val
		}
	case map[string]interface{}:
		for k, val := range v {
			switch tv := val.(type) {
			case string:
				result[k] = tv
			case fmt.Stringer:
				result[k] = tv.String()
			default:
				if b, err := json.Marshal(tv); err == nil {
					result[k] = string(b)
				}
			}
		}
	default:
		// Fallback: serialize entire data as JSON under "data" key
		if b, err := json.Marshal(v); err == nil {
			result["data"] = string(b)
		}
	}
	return result
}
