package tools

import "encoding/json"

// ToolSurfaceMetrics describes the LLM-visible native tool surface after
// routing/localization/schema normalization.
type ToolSurfaceMetrics struct {
	ToolCount   int `json:"tool_count"`
	SchemaBytes int `json:"schema_bytes"`
}

// MeasureLLMToolSurface returns the normalized schema footprint for the given
// tool definitions using the same schema normalization path the chat layer uses
// before exposing tools to providers.
func MeasureLLMToolSurface(defs []ToolDefinition) ToolSurfaceMetrics {
	metrics := ToolSurfaceMetrics{ToolCount: len(defs)}
	for _, def := range defs {
		schema := NormalizeToolSchemaForLLM("", "", "", def.Parameters)
		if len(schema) == 0 {
			continue
		}
		raw, err := json.Marshal(schema)
		if err != nil {
			continue
		}
		metrics.SchemaBytes += len(raw)
	}
	return metrics
}
