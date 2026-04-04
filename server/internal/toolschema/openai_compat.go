package toolschema

import (
	"reflect"
	"strings"
)

// NormalizeForOpenAICompat rewrites a tool parameter schema into a shape that
// is accepted more consistently by OpenAI-compatible tool-calling endpoints.
//
// In particular, some relays reject top-level composition keywords like anyOf
// even when nested composition remains acceptable. We therefore strip these
// keywords only at the root while preserving nested schema detail.
func NormalizeForOpenAICompat(schema map[string]interface{}) map[string]interface{} {
	return normalizeOpenAICompatNode(schema, true)
}

func normalizeOpenAICompatNode(schema map[string]interface{}, topLevel bool) map[string]interface{} {
	if len(schema) == 0 {
		return map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"required":             []string{},
			"additionalProperties": false,
		}
	}

	out := make(map[string]interface{}, len(schema)+3)
	for key, value := range schema {
		switch key {
		case "properties":
			props, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			normalized := make(map[string]interface{}, len(props))
			for name, child := range props {
				childMap, _ := child.(map[string]interface{})
				normalized[name] = normalizeOpenAICompatNode(childMap, false)
			}
			out[key] = normalized
		case "items":
			switch typed := value.(type) {
			case map[string]interface{}:
				out[key] = normalizeOpenAICompatNode(typed, false)
			default:
				items, ok := normalizeSchemaArrayValue(typed)
				if !ok {
					continue
				}
				normalizedItems := make([]interface{}, 0, len(items))
				for _, child := range items {
					if childMap, ok := child.(map[string]interface{}); ok {
						normalizedItems = append(normalizedItems, normalizeOpenAICompatNode(childMap, false))
						continue
					}
					normalizedItems = append(normalizedItems, cloneJSONValue(child))
				}
				out[key] = normalizedItems
			}
		case "oneOf", "anyOf", "allOf":
			items, ok := normalizeSchemaArrayValue(value)
			if !ok {
				continue
			}
			normalized := make([]interface{}, 0, len(items))
			for _, child := range items {
				if childMap, ok := child.(map[string]interface{}); ok {
					normalized = append(normalized, normalizeOpenAICompatNode(childMap, false))
					continue
				}
				normalized = append(normalized, cloneJSONValue(child))
			}
			out[key] = normalized
		case "additionalProperties":
			switch typed := value.(type) {
			case bool:
				out[key] = typed
			case map[string]interface{}:
				out[key] = normalizeOpenAICompatNode(typed, false)
			}
		case "required":
			if normalized := normalizeStringList(value); normalized != nil {
				out[key] = normalized
			}
		case "nullable":
			continue
		default:
			out[key] = cloneJSONValue(value)
		}
	}

	if _, ok := out["type"]; !ok {
		if _, hasProps := out["properties"]; hasProps {
			out["type"] = "object"
		} else if _, hasItems := out["items"]; hasItems {
			out["type"] = "array"
		}
	}
	if typeName, ok := out["type"].(string); ok && typeName == "object" {
		if _, exists := out["properties"]; !exists {
			out["properties"] = map[string]interface{}{}
		}
		if _, exists := out["required"]; !exists {
			out["required"] = []string{}
		}
		if _, exists := out["additionalProperties"]; !exists {
			out["additionalProperties"] = false
		}
	}
	if topLevel {
		delete(out, "oneOf")
		delete(out, "anyOf")
		delete(out, "allOf")
		delete(out, "enum")
		delete(out, "not")
	}
	return out
}

func cloneJSONValue(raw interface{}) interface{} {
	switch typed := raw.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = cloneJSONValue(value)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = cloneJSONValue(value)
		}
		return out
	default:
		return typed
	}
}

func normalizeStringList(raw interface{}) []string {
	items, ok := normalizeSchemaArrayValue(raw)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeSchemaArrayValue(raw interface{}) ([]interface{}, bool) {
	switch typed := raw.(type) {
	case nil:
		return nil, false
	case []interface{}:
		return typed, true
	}
	value := reflect.ValueOf(raw)
	if !value.IsValid() {
		return nil, false
	}
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]interface{}, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			out = append(out, value.Index(i).Interface())
		}
		return out, true
	default:
		return nil, false
	}
}
