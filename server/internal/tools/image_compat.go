package tools

import "context"

// ImageCompatTool exposes legacy image-generation aliases backed by the
// native image tool.
type ImageCompatTool struct {
	name        string
	description string
	native      *ImageTool
}

func newImageCompatTool(name, description string, native *ImageTool) *ImageCompatTool {
	return &ImageCompatTool{name: name, description: description, native: native}
}

func (t *ImageCompatTool) Definition() ToolDefinition {
	if t == nil || t.native == nil {
		return ToolDefinition{Name: t.name, Description: t.description}
	}
	def := t.native.Definition()
	def.Name = t.name
	if t.description != "" {
		def.Description = t.description
	}
	return def
}

func (t *ImageCompatTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.native == nil {
		return nil, ErrToolNotFound
	}
	return t.native.Execute(ctx, args)
}
