package mediagen

import (
	"strconv"
	"strings"
)

func skillCompatRaw(args map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := args[key]; ok && value != nil {
			return value
		}
	}
	for _, containerKey := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := args[containerKey].(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range keys {
			if value, ok := nested[key]; ok && value != nil {
				return value
			}
		}
	}
	return nil
}

func skillCompatString(args map[string]interface{}, keys ...string) string {
	v, ok := skillCompatRaw(args, keys...).(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func skillCompatInt(args map[string]interface{}, keys ...string) int {
	v := skillCompatRaw(args, keys...)
	switch typed := v.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return n
		}
	}
	return 0
}
