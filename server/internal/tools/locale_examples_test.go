package tools

import (
	"strings"
	"testing"
)

func TestRegistryDefinitionsForLocaleLocalizesLocaleExamples(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&captureArgsTool{def: ToolDefinition{
		Name:        "localized_tool",
		Description: "test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"lang": map[string]interface{}{
					"type":        "string",
					"description": "Output language (default: zh-CN). Examples: zh-CN, en-US",
				},
				"locale": map[string]interface{}{
					"type":        "string",
					"description": "Language/locale code for localized responses (e.g., en-US, zh-CN)",
				},
			},
		},
	}})

	localized := registry.DefinitionsForLocale("fr-FR")
	if len(localized) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(localized))
	}

	properties := localized[0].Parameters["properties"].(map[string]interface{})
	langDesc := properties["lang"].(map[string]interface{})["description"].(string)
	localeDesc := properties["locale"].(map[string]interface{})["description"].(string)

	if !strings.Contains(langDesc, "fr-FR") {
		t.Fatalf("lang description = %q, want locale example fr-FR", langDesc)
	}
	if strings.Contains(langDesc, "en-US") || strings.Contains(langDesc, "zh-CN") {
		t.Fatalf("lang description still contains old examples: %q", langDesc)
	}
	if !strings.Contains(localeDesc, "fr-FR") {
		t.Fatalf("locale description = %q, want locale example fr-FR", localeDesc)
	}
	if strings.Contains(localeDesc, "en-US") || strings.Contains(localeDesc, "zh-CN") {
		t.Fatalf("locale description still contains old examples: %q", localeDesc)
	}

	raw := registry.Definitions()
	rawProps := raw[0].Parameters["properties"].(map[string]interface{})
	rawLangDesc := rawProps["lang"].(map[string]interface{})["description"].(string)
	if !strings.Contains(rawLangDesc, "en-US") {
		t.Fatalf("expected raw definitions to stay unmodified, got %q", rawLangDesc)
	}
}
