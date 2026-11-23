package llm

import (
	"encoding/json"
	"strings"
)

// ProviderContent leaves stored tool JSON intact while presenting shell streams as text.
func (m Message) ProviderContent() string {
	if m.Role == RoleTool {
		switch strings.ToLower(strings.TrimSpace(m.ToolName)) {
		case "bash", "exec":
			return ExecOutputText(m.Content)
		}
	}
	return m.Content
}

// ExecOutputText decodes only shell stream fields; execution and recovery metadata stay JSON.
func ExecOutputText(content string) string {
	var metadata map[string]json.RawMessage
	if json.Unmarshal([]byte(content), &metadata) != nil || metadata == nil {
		return content
	}
	var streams strings.Builder
	for _, key := range []string{"stdout", "stderr", "output"} {
		raw, ok := metadata[key]
		if !ok || len(raw) == 0 || raw[0] != '"' {
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			continue
		}
		delete(metadata, key)
		streams.WriteString("\n" + key + ":\n")
		streams.WriteString(value)
	}
	if streams.Len() == 0 {
		return content
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return content
	}
	return string(encoded) + streams.String()
}
