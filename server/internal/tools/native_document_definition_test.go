package tools

import (
	"strings"
	"testing"
)

func TestNativeDocumentCreateDefinitionsAdvertiseSingleInputWorkflow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		def  ToolDefinition
	}{
		{name: "docx", def: NewDOCXTool(nil, nil, nil).Definition()},
		{name: "xlsx", def: NewXLSXTool(nil, nil, nil).Definition()},
		{name: "pptx", def: NewPPTXTool(nil, nil, nil).Definition()},
		{name: "pdf", def: NewPDFTool(nil).Definition()},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			properties := definitionPropertiesForNativeDocumentTest(t, tc.def)

			inputPath := definitionPropertyDescriptionForNativeDocumentTest(t, properties, "input_path")
			if !strings.Contains(strings.ToLower(inputPath), "create") {
				t.Fatalf("input_path description = %q, want create workflow guidance", inputPath)
			}
			if !strings.Contains(strings.ToLower(inputPath), "derive") {
				t.Fatalf("input_path description = %q, want derived output guidance", inputPath)
			}

			outputPath := definitionPropertyDescriptionForNativeDocumentTest(t, properties, "output_path")
			if !strings.Contains(strings.ToLower(outputPath), "create") {
				t.Fatalf("output_path description = %q, want create workflow guidance", outputPath)
			}
			if !strings.Contains(strings.ToLower(outputPath), "omit") && !strings.Contains(strings.ToLower(outputPath), "derived") {
				t.Fatalf("output_path description = %q, want omit/derived guidance", outputPath)
			}

			if required := definitionRequiredFieldsForNativeDocumentTest(tc.def); len(required) > 0 {
				for _, field := range required {
					if field == "path" {
						t.Fatalf("required fields = %#v, want path optional so create can use input_path-only workflow", required)
					}
				}
			}
		})
	}
}

func definitionPropertiesForNativeDocumentTest(t *testing.T, def ToolDefinition) map[string]interface{} {
	t.Helper()

	properties, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties type = %T, want object", def.Parameters["properties"])
	}
	return properties
}

func definitionPropertyDescriptionForNativeDocumentTest(t *testing.T, properties map[string]interface{}, key string) string {
	t.Helper()

	property, ok := properties[key].(map[string]interface{})
	if !ok {
		t.Fatalf("properties[%q] type = %T, want object", key, properties[key])
	}
	description, _ := property["description"].(string)
	if strings.TrimSpace(description) == "" {
		t.Fatalf("properties[%q].description is empty", key)
	}
	return description
}

func definitionRequiredFieldsForNativeDocumentTest(def ToolDefinition) []string {
	raw, ok := def.Parameters["required"]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		fields := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok {
				fields = append(fields, value)
			}
		}
		return fields
	default:
		return nil
	}
}
