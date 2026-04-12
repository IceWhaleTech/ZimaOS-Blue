package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

var validJSONSchemaTypes = map[string]struct{}{
	"array":   {},
	"boolean": {},
	"integer": {},
	"null":    {},
	"number":  {},
	"object":  {},
	"string":  {},
}

// ValidateToolSchema validates a tool JSON schema shape conservatively.
func ValidateToolSchema(schema map[string]interface{}) error {
	return validateSchemaNode("$", schema, true)
}

// NormalizeToolSchemaForLLM normalizes a tool schema before exposing it to providers.
func NormalizeToolSchemaForLLM(provider, providerID, model string, schema map[string]interface{}) map[string]interface{} {
	_ = provider
	_ = providerID
	_ = model

	if len(schema) == 0 {
		return map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": true,
		}
	}
	normalized, ok := normalizeSchemaValue(schema, true).(map[string]interface{})
	if !ok || len(normalized) == 0 {
		return map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": true,
		}
	}
	return normalized
}

// ValidateToolArguments validates a parsed argument object against a tool schema.
func ValidateToolArguments(schema map[string]interface{}, args map[string]interface{}) error {
	normalized := NormalizeToolSchemaForLLM("", "", "", schema)
	return validateValueAgainstSchema("$", normalized, args)
}

func validateSchemaNode(path string, schema map[string]interface{}, topLevel bool) error {
	if len(schema) == 0 {
		return nil
	}
	if topLevel {
		if typeName, ok := schema["type"].(string); ok && strings.TrimSpace(typeName) != "" && typeName != "object" {
			return fmt.Errorf("%s: tool parameter schema must be an object, got %q", path, typeName)
		}
	}
	if _, err := parseSchemaTypes(schema["type"]); err != nil {
		return fmt.Errorf("%s.type: %w", path, err)
	}
	if rawRequired, ok := schema["required"]; ok {
		if _, err := normalizeStringSlice(rawRequired); err != nil {
			return fmt.Errorf("%s.required: %w", path, err)
		}
	}
	if props, exists := schema["properties"]; exists {
		propMap, ok := props.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s.properties: must be an object", path)
		}
		for name, rawChild := range propMap {
			childSchema, ok := rawChild.(map[string]interface{})
			if !ok {
				return fmt.Errorf("%s.properties.%s: must be an object", path, name)
			}
			if err := validateSchemaNode(path+".properties."+name, childSchema, false); err != nil {
				return err
			}
		}
	}
	if rawItems, ok := schema["items"]; ok {
		if err := validateSchemaItems(path+".items", rawItems); err != nil {
			return err
		}
	}
	for _, key := range []string{"oneOf", "anyOf", "allOf"} {
		if raw, ok := schema[key]; ok {
			if err := validateSchemaAlternatives(path+"."+key, raw); err != nil {
				return err
			}
		}
	}
	if rawAdditional, ok := schema["additionalProperties"]; ok {
		switch typed := rawAdditional.(type) {
		case bool:
		case map[string]interface{}:
			if err := validateSchemaNode(path+".additionalProperties", typed, false); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s.additionalProperties: must be a boolean or object", path)
		}
	}
	return nil
}

func validateSchemaItems(path string, raw interface{}) error {
	if typed, ok := raw.(map[string]interface{}); ok {
		return validateSchemaNode(path, typed, false)
	}
	items, ok := normalizeSchemaArray(raw)
	if !ok {
		return fmt.Errorf("%s: must be an object or array of objects", path)
	}
	for i, item := range items {
		child, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s[%d]: must be an object", path, i)
		}
		if err := validateSchemaNode(fmt.Sprintf("%s[%d]", path, i), child, false); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaAlternatives(path string, raw interface{}) error {
	items, ok := normalizeSchemaArray(raw)
	if !ok {
		return fmt.Errorf("%s: must be an array", path)
	}
	for i, item := range items {
		child, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s[%d]: must be an object", path, i)
		}
		if err := validateSchemaNode(fmt.Sprintf("%s[%d]", path, i), child, false); err != nil {
			return err
		}
	}
	return nil
}

func normalizeSchemaValue(raw interface{}, topLevel bool) interface{} {
	node, ok := raw.(map[string]interface{})
	if !ok {
		return raw
	}
	out := make(map[string]interface{}, len(node)+3)
	for key, value := range node {
		switch key {
		case "properties":
			props, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			normalizedProps := make(map[string]interface{}, len(props))
			for name, child := range props {
				normalizedProps[name] = normalizeSchemaValue(child, false)
			}
			out[key] = normalizedProps
		case "items":
			switch typed := value.(type) {
			case map[string]interface{}:
				out[key] = normalizeSchemaValue(typed, false)
			default:
				items, ok := normalizeSchemaArray(typed)
				if !ok {
					continue
				}
				normalizedItems := make([]interface{}, 0, len(items))
				for _, child := range items {
					normalizedItems = append(normalizedItems, normalizeSchemaValue(child, false))
				}
				out[key] = normalizedItems
			}
		case "oneOf", "anyOf", "allOf":
			items, ok := normalizeSchemaArray(value)
			if !ok {
				continue
			}
			normalized := make([]interface{}, 0, len(items))
			for _, child := range items {
				normalized = append(normalized, normalizeSchemaValue(child, false))
			}
			if len(normalized) == 1 {
				if only, ok := normalized[0].(map[string]interface{}); ok {
					for nestedKey, nestedValue := range only {
						out[nestedKey] = nestedValue
					}
					continue
				}
			}
			out[key] = normalized
		case "additionalProperties":
			switch typed := value.(type) {
			case bool:
				out[key] = typed
			case map[string]interface{}:
				out[key] = normalizeSchemaValue(typed, false)
			}
		case "required":
			required, err := normalizeStringSlice(value)
			if err == nil && len(required) > 0 {
				out[key] = required
			}
		case "nullable":
			// Providers differ on nullable support. Keep runtime validation strict
			// and expose a provider-safe schema by dropping the nullable marker.
			continue
		default:
			out[key] = cloneJSONCompatibleValue(value)
		}
	}

	if _, ok := out["type"]; !ok {
		switch {
		case out["properties"] != nil || topLevel:
			out["type"] = "object"
		case out["items"] != nil:
			out["type"] = "array"
		}
	}
	if typeName, ok := out["type"].(string); ok && typeName == "array" {
		if _, exists := out["items"]; !exists {
			// OpenAI-compatible tool validators reject array schemas that omit
			// `items`, even when the runtime schema intentionally accepts mixed
			// item shapes. An empty schema keeps runtime validation permissive
			// while producing a provider-safe array definition.
			out["items"] = map[string]interface{}{}
		}
	}
	if typeName, ok := out["type"].(string); ok && typeName == "object" {
		if _, exists := out["properties"]; !exists {
			out["properties"] = map[string]interface{}{}
		}
		if _, exists := out["required"]; !exists {
			// Some OpenAI-compatible relays reject object schemas when `required`
			// is omitted, even though plain JSON Schema treats it as optional.
			out["required"] = []string{}
		}
		if props, ok := out["properties"].(map[string]interface{}); ok && len(props) > 0 {
			if _, exists := out["additionalProperties"]; !exists {
				out["additionalProperties"] = false
			}
		} else if topLevel {
			if _, exists := out["additionalProperties"]; exists {
				return out
			}
			out["additionalProperties"] = false
		}
	}
	return out
}

func validateValueAgainstSchema(path string, schema map[string]interface{}, value interface{}) error {
	if len(schema) == 0 {
		if m, ok := value.(map[string]interface{}); ok && len(m) == 0 {
			return nil
		}
	}

	if rawOneOf, ok := schema["oneOf"]; ok {
		return validateCompositeSchema(path, "oneOf", rawOneOf, value, true)
	}
	if rawAnyOf, ok := schema["anyOf"]; ok {
		return validateCompositeSchema(path, "anyOf", rawAnyOf, value, false)
	}
	if rawAllOf, ok := schema["allOf"]; ok {
		items, _ := rawAllOf.([]interface{})
		for i, item := range items {
			child, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if err := validateValueAgainstSchema(fmt.Sprintf("%s.allOf[%d]", path, i), child, value); err != nil {
				return err
			}
		}
	}

	allowedTypes, err := parseSchemaTypes(schema["type"])
	if err != nil {
		return err
	}
	if value == nil {
		if typeAllowed("null", allowedTypes) || truthy(schema["nullable"]) {
			return nil
		}
		return fmt.Errorf("%s: value is required", path)
	}

	if rawEnum, ok := schema["enum"]; ok {
		items, ok := normalizeSchemaArray(rawEnum)
		if ok {
			matched := false
			for _, item := range items {
				if reflect.DeepEqual(item, value) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("%s: value %v is not in enum", path, value)
			}
		}
	}

	switch {
	case typeAllowed("object", allowedTypes) || inferSchemaType(schema) == "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		return validateObjectValue(path, schema, obj)
	case typeAllowed("array", allowedTypes) || inferSchemaType(schema) == "array":
		items, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("%s: expected array", path)
		}
		return validateArrayValue(path, schema, items)
	case typeAllowed("string", allowedTypes):
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected string", path)
		}
	case typeAllowed("boolean", allowedTypes):
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean", path)
		}
	case typeAllowed("integer", allowedTypes):
		if !isIntegerValue(value) {
			return fmt.Errorf("%s: expected integer", path)
		}
	case typeAllowed("number", allowedTypes):
		if !isNumericValue(value) {
			return fmt.Errorf("%s: expected number", path)
		}
	case typeAllowed("null", allowedTypes):
		return fmt.Errorf("%s: expected null", path)
	}

	return nil
}

func validateCompositeSchema(path, kind string, raw interface{}, value interface{}, requireExactlyOne bool) error {
	items, _ := normalizeSchemaArray(raw)
	matchCount := 0
	var lastErr error
	for i, item := range items {
		child, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if err := validateValueAgainstSchema(fmt.Sprintf("%s.%s[%d]", path, kind, i), child, value); err == nil {
			matchCount++
			lastErr = nil
		} else if lastErr == nil {
			lastErr = err
		}
	}
	if requireExactlyOne {
		if matchCount == 1 {
			return nil
		}
		if matchCount > 1 {
			return fmt.Errorf("%s: value matches multiple %s branches", path, kind)
		}
	} else if matchCount > 0 {
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("%s: value does not satisfy %s", path, kind)
}

func validateObjectValue(path string, schema map[string]interface{}, obj map[string]interface{}) error {
	props, _ := schema["properties"].(map[string]interface{})
	required, _ := normalizeStringSlice(schema["required"])
	for _, field := range required {
		if _, ok := obj[field]; !ok {
			return fmt.Errorf("%s.%s: missing required field", path, field)
		}
	}

	for key, value := range obj {
		if childRaw, ok := props[key]; ok {
			childSchema, _ := childRaw.(map[string]interface{})
			if err := validateValueAgainstSchema(path+"."+key, childSchema, value); err != nil {
				return err
			}
			continue
		}
		switch typed := schema["additionalProperties"].(type) {
		case nil:
			if len(props) > 0 {
				return fmt.Errorf("%s.%s: unexpected field", path, key)
			}
		case bool:
			if !typed {
				return fmt.Errorf("%s.%s: unexpected field", path, key)
			}
		case map[string]interface{}:
			if err := validateValueAgainstSchema(path+"."+key, typed, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateArrayValue(path string, schema map[string]interface{}, items []interface{}) error {
	rawItems, hasItems := schema["items"]
	if !hasItems {
		return nil
	}
	switch typed := rawItems.(type) {
	case map[string]interface{}:
		for i, item := range items {
			if err := validateValueAgainstSchema(fmt.Sprintf("%s[%d]", path, i), typed, item); err != nil {
				return err
			}
		}
	default:
		schemaItems, ok := normalizeSchemaArray(typed)
		if !ok {
			return nil
		}
		for i, item := range items {
			if i >= len(schemaItems) {
				return nil
			}
			child, ok := schemaItems[i].(map[string]interface{})
			if !ok {
				continue
			}
			if err := validateValueAgainstSchema(fmt.Sprintf("%s[%d]", path, i), child, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseSchemaTypes(raw interface{}) ([]string, error) {
	switch typed := raw.(type) {
	case nil:
		return nil, nil
	case string:
		typeName := strings.TrimSpace(typed)
		if typeName == "" {
			return nil, nil
		}
		if _, ok := validJSONSchemaTypes[typeName]; !ok {
			return nil, fmt.Errorf("unsupported type %q", typeName)
		}
		return []string{typeName}, nil
	default:
		items, ok := normalizeSchemaArray(raw)
		if !ok {
			return nil, fmt.Errorf("must be a string or array of strings")
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			typeName, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("type array items must be strings")
			}
			if _, ok := validJSONSchemaTypes[typeName]; !ok {
				return nil, fmt.Errorf("unsupported type %q", typeName)
			}
			out = append(out, typeName)
		}
		return out, nil
	}
}

func inferSchemaType(schema map[string]interface{}) string {
	if typeName, ok := schema["type"].(string); ok {
		return strings.TrimSpace(typeName)
	}
	if schema["properties"] != nil {
		return "object"
	}
	if schema["items"] != nil {
		return "array"
	}
	return ""
}

func typeAllowed(typeName string, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, item := range allowed {
		if item == typeName {
			return true
		}
	}
	return false
}

func normalizeStringSlice(raw interface{}) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := normalizeSchemaArray(raw)
	if !ok {
		return nil, fmt.Errorf("must be an array")
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		str, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("must contain only strings")
		}
		trimmed := strings.TrimSpace(str)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out, nil
}

func normalizeSchemaArray(raw interface{}) ([]interface{}, bool) {
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

func cloneJSONCompatibleValue(raw interface{}) interface{} {
	switch typed := raw.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = cloneJSONCompatibleValue(value)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = cloneJSONCompatibleValue(value)
		}
		return out
	case []string:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = value
		}
		return out
	case json.Number:
		return typed.String()
	default:
		return typed
	}
}

func truthy(raw interface{}) bool {
	v, ok := raw.(bool)
	return ok && v
}

func isNumericValue(value interface{}) bool {
	switch typed := value.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case json.Number:
		_, err := typed.Float64()
		return err == nil
	default:
		return false
	}
}

func isIntegerValue(value interface{}) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return math.Mod(typed, 1) == 0
	case float32:
		return math.Mod(float64(typed), 1) == 0
	case json.Number:
		if strings.Contains(typed.String(), ".") {
			return false
		}
		_, err := typed.Int64()
		return err == nil
	default:
		return false
	}
}
