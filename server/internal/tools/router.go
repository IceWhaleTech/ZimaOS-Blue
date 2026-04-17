package tools

import (
	"encoding/json"
	"strings"
	"sync/atomic"
)

const compactToolDescriptionMaxLen = 64

// ToolRouterStats holds cumulative stats for tool routing and schema compression.
type ToolRouterStats struct {
	Requests          int64 `json:"requests"`
	ToolsTotal        int64 `json:"tools_total"`
	ToolsExposed      int64 `json:"tools_exposed"`
	ToolsHidden       int64 `json:"tools_hidden"`
	SchemaBytesBefore int64 `json:"schema_bytes_before"`
	SchemaBytesAfter  int64 `json:"schema_bytes_after"`
}

// ToolRouter applies dynamic tool exposure rules and optional schema compression.
type ToolRouter struct {
	// DynamicExposure enables query-driven tool visibility rules.
	DynamicExposure bool
	// SchemaCompression removes non-essential JSON-schema fields to reduce prompt tokens.
	SchemaCompression bool
	// AlwaysInclude tools that must stay visible regardless of routing rules.
	AlwaysInclude []string

	processKeywords []string

	requests          int64
	toolsTotal        int64
	toolsExposed      int64
	toolsHidden       int64
	schemaBytesBefore int64
	schemaBytesAfter  int64
}

// DefaultToolRouter returns a tool router with conservative defaults.
func DefaultToolRouter() *ToolRouter {
	return &ToolRouter{
		DynamicExposure:   true,
		SchemaCompression: true,
		AlwaysInclude:     []string{"bash"},
		processKeywords: []string{
			"process", "session", "sessions", "poll", "log", "kill", "pid", "background",
			"running", "status", "tail", "terminate", "output",
			"进程", "会话", "后台", "轮询", "日志", "停止", "终止", "状态", "输出",
		},
	}
}

// Stats returns current cumulative router stats.
func (tr *ToolRouter) Stats() ToolRouterStats {
	return ToolRouterStats{
		Requests:          atomic.LoadInt64(&tr.requests),
		ToolsTotal:        atomic.LoadInt64(&tr.toolsTotal),
		ToolsExposed:      atomic.LoadInt64(&tr.toolsExposed),
		ToolsHidden:       atomic.LoadInt64(&tr.toolsHidden),
		SchemaBytesBefore: atomic.LoadInt64(&tr.schemaBytesBefore),
		SchemaBytesAfter:  atomic.LoadInt64(&tr.schemaBytesAfter),
	}
}

// Route returns routed tool definitions for a single request.
func (tr *ToolRouter) Route(query, model string, defs []ToolDefinition) []ToolDefinition {
	if len(defs) == 0 {
		return nil
	}

	routed := defs
	if tr.DynamicExposure {
		routed = tr.routeDynamic(query, model, defs)
	}

	beforeBytes := estimateSchemaBytes(routed)
	out := routed
	if tr.SchemaCompression {
		out = tr.applySchemaCompression(routed)
	}
	afterBytes := estimateSchemaBytes(out)

	total := int64(len(defs))
	exposed := int64(len(out))
	hidden := total - exposed
	if hidden < 0 {
		hidden = 0
	}

	atomic.AddInt64(&tr.requests, 1)
	atomic.AddInt64(&tr.toolsTotal, total)
	atomic.AddInt64(&tr.toolsExposed, exposed)
	atomic.AddInt64(&tr.toolsHidden, hidden)
	atomic.AddInt64(&tr.schemaBytesBefore, int64(beforeBytes))
	atomic.AddInt64(&tr.schemaBytesAfter, int64(afterBytes))

	return out
}

func (tr *ToolRouter) routeDynamic(query, _ string, defs []ToolDefinition) []ToolDefinition {
	if len(defs) == 0 {
		return defs
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		// Empty query: keep full visibility to avoid over-filtering.
		return defs
	}

	always := make(map[string]struct{}, len(tr.AlwaysInclude))
	for _, name := range tr.AlwaysInclude {
		always[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}

	needProcess := strings.Contains(q, "process") || strings.Contains(q, "session")
	if !needProcess {
		for _, kw := range tr.processKeywords {
			if strings.Contains(q, strings.ToLower(kw)) {
				needProcess = true
				break
			}
		}
	}

	out := make([]ToolDefinition, 0, len(defs))
	for _, def := range defs {
		name := strings.ToLower(strings.TrimSpace(def.Name))
		if _, ok := always[name]; ok {
			out = append(out, def)
			continue
		}

		if name == "process" && !needProcess {
			continue
		}
		out = append(out, def)
	}

	// Safety fallback: never return empty toolset if original had tools.
	if len(out) == 0 {
		return defs
	}
	return out
}

func (tr *ToolRouter) applySchemaCompression(defs []ToolDefinition) []ToolDefinition {
	if len(defs) == 0 {
		return nil
	}

	out := make([]ToolDefinition, len(defs))
	for i, def := range defs {
		out[i] = ToolDefinition{
			Name:        def.Name,
			Description: compressToolDescription(def.Description),
			Parameters:  compressSchemaMap(def.Parameters),
		}
	}
	return out
}

func compressToolDescription(raw string) string {
	desc := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if len(desc) <= compactToolDescriptionMaxLen {
		return desc
	}

	for _, sep := range []string{". ", "。", "; ", "；", ": "} {
		if idx := strings.Index(desc, sep); idx > 0 && idx <= compactToolDescriptionMaxLen {
			return strings.TrimSpace(desc[:idx])
		}
	}

	cut := desc[:compactToolDescriptionMaxLen]
	if idx := strings.LastIndexAny(cut, " ,;:"); idx >= compactToolDescriptionMaxLen/2 {
		cut = cut[:idx]
	}
	return strings.TrimSpace(cut)
}

func estimateSchemaBytes(defs []ToolDefinition) int {
	total := 0
	for _, def := range defs {
		if len(def.Parameters) == 0 {
			continue
		}
		b, err := json.Marshal(def.Parameters)
		if err != nil {
			continue
		}
		total += len(b)
	}
	return total
}

var schemaAllowedKeys = map[string]struct{}{
	"$ref":                 {},
	"additionalProperties": {},
	"allOf":                {},
	"anyOf":                {},
	"const":                {},
	"default":              {},
	"enum":                 {},
	"exclusiveMaximum":     {},
	"exclusiveMinimum":     {},
	"format":               {},
	"items":                {},
	"maxItems":             {},
	"maxLength":            {},
	"maximum":              {},
	"minItems":             {},
	"minLength":            {},
	"minimum":              {},
	"nullable":             {},
	"oneOf":                {},
	"pattern":              {},
	"properties":           {},
	"required":             {},
	"type":                 {},
	"uniqueItems":          {},
}

func compressSchemaMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return in
	}

	out := make(map[string]interface{}, len(in))
	for key, raw := range in {
		if _, ok := schemaAllowedKeys[key]; !ok {
			continue
		}

		switch key {
		case "properties":
			props, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			compressedProps := make(map[string]interface{}, len(props))
			for propName, propRaw := range props {
				if propMap, ok := propRaw.(map[string]interface{}); ok {
					compressedProps[propName] = compressSchemaMap(propMap)
					continue
				}
				compressedProps[propName] = cloneSchemaValue(propRaw)
			}
			out[key] = compressedProps
		case "items":
			out[key] = compressSchemaValue(raw)
		case "oneOf", "anyOf", "allOf":
			out[key] = compressSchemaValue(raw)
		case "additionalProperties":
			out[key] = compressSchemaValue(raw)
		default:
			out[key] = cloneSchemaValue(raw)
		}
	}

	// Normalize object schema shape for providers that expect properties to exist.
	if typ, ok := out["type"].(string); ok && typ == "object" {
		if _, exists := out["properties"]; !exists {
			out["properties"] = map[string]interface{}{}
		}
	}

	return out
}

func compressSchemaValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return compressSchemaMap(t)
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, item := range t {
			out = append(out, compressSchemaValue(item))
		}
		return out
	case []string:
		out := make([]string, len(t))
		copy(out, t)
		return out
	default:
		return t
	}
}

func cloneSchemaValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, value := range t {
			out[k] = cloneSchemaValue(value)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i := range t {
			out[i] = cloneSchemaValue(t[i])
		}
		return out
	case []string:
		out := make([]string, len(t))
		copy(out, t)
		return out
	default:
		return t
	}
}
