package tools

import (
	"context"
	"strings"
)

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

type GenerateImageTool struct {
	native *ImageTool
}

func newGenerateImageTool(native *ImageTool) *GenerateImageTool {
	return &GenerateImageTool{native: native}
}

func (t *GenerateImageTool) Definition() ToolDefinition {
	if t == nil || t.native == nil {
		return ToolDefinition{Name: "generate_image", Description: "Generate an image from a prompt and optionally save it to a workspace path."}
	}
	def := t.native.Definition()
	def.Name = "generate_image"
	def.Description = "Generate an image from a prompt and optionally save it to a workspace path."
	def.SearchHints = []string{"generate image", "image generation", "text to image", "draw", "render", "illustrate"}
	return def
}

func (t *GenerateImageTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.native == nil {
		return nil, ErrToolNotFound
	}
	forwarded := cloneImageCompatArgs(args)
	switch strings.ToLower(strings.TrimSpace(firstCompatString(forwarded, "action", "op", "operation", "command"))) {
	case "status", "get", "task", "progress":
		return t.native.Execute(ctx, forwarded)
	}
	forwarded["action"] = "generate"
	return t.native.Execute(ctx, forwarded)
}

type OCRTool struct {
	native *ImageTool
}

func newOCRTool(native *ImageTool) *OCRTool {
	return &OCRTool{native: native}
}

func (t *OCRTool) Definition() ToolDefinition {
	if t == nil || t.native == nil {
		return ToolDefinition{Name: "ocr", Description: "Extract text from images, screenshots, or scanned files using OCR."}
	}
	return ToolDefinition{
		Name:        "ocr",
		Description: "Extract text from images, screenshots, or scanned files using OCR.",
		Icon:        "image",
		SearchHints: []string{"ocr", "extract text", "read text in image", "transcribe screenshot", "scan text"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image":        map[string]interface{}{"type": "string", "description": "Base64 image content or an inline image value."},
				"url":          map[string]interface{}{"type": "string", "description": "Remote image URL to OCR."},
				"image_path":   map[string]interface{}{"type": "string", "description": "Workspace image path to OCR."},
				"image_paths":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional list of workspace image paths."},
				"images":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional list of inline images."},
				"prompt":       map[string]interface{}{"type": "string", "description": "Optional extraction instruction. Defaults to extracting visible text."},
				"lang":         map[string]interface{}{"type": "string", "description": "Optional OCR language hint."},
				"language":     map[string]interface{}{"type": "string", "description": "Optional OCR language hint."},
				"analysisMode": map[string]interface{}{"type": "string", "description": "Compatibility field. OCR tool always uses OCR-only mode."},
			},
		},
	}
}

func (t *OCRTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.native == nil {
		return nil, ErrToolNotFound
	}
	forwarded := cloneImageCompatArgs(args)
	forwarded["action"] = "review"
	forwarded["analysis_mode"] = "ocr_only"
	if _, ok := forwarded["prompt"]; !ok {
		forwarded["prompt"] = "Extract all visible text from this image."
	}
	return t.native.Execute(ctx, forwarded)
}

func cloneImageCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(args)+2)
	for key, value := range args {
		cloned[key] = value
	}
	return cloned
}
