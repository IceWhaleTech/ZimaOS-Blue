package bootstrap

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func tryExecuteToolFallback(ctx context.Context, registry *tools.Registry, toolName string, input map[string]any) (map[string]string, bool, error) {
	if registry == nil {
		return nil, false, nil
	}
	tool := registry.Get(strings.TrimSpace(toolName))
	if tool == nil {
		return nil, false, nil
	}

	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		return nil, true, err
	}
	return toolResultToIPCData(result), true, nil
}

func toolResultToIPCData(result any) map[string]string {
	switch typed := result.(type) {
	case nil:
		return map[string]string{}
	case map[string]string:
		out := make(map[string]string, len(typed))
		for k, v := range typed {
			out[k] = v
		}
		return out
	case map[string]interface{}:
		return flattenToolResultMap(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return map[string]string{}
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return flattenToolResultMap(parsed)
		}
		return map[string]string{"result": typed}
	default:
		sanitized := tools.SafeToolPayloadValue(typed, 64*1024)
		if parsed, ok := sanitized.(map[string]interface{}); ok {
			return flattenToolResultMap(parsed)
		}
		return map[string]string{"result": tools.SafeToolPayloadString(sanitized, 64*1024)}
	}
}

func flattenToolResultMap(data map[string]interface{}) map[string]string {
	out := make(map[string]string, len(data))
	for k, v := range data {
		switch typed := v.(type) {
		case nil:
			out[k] = "null"
		case string:
			out[k] = typed
		default:
			out[k] = tools.SafeToolPayloadString(typed, 64*1024)
		}
	}
	return out
}
