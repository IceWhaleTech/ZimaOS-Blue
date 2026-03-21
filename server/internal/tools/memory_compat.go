package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// MemoryCompatTool exposes legacy memory_* names as native wrappers.
type MemoryCompatTool struct {
	name          string
	description   string
	memoryService MemoryServiceInterface
}

func newMemoryCompatTool(name, description string, memoryService MemoryServiceInterface) *MemoryCompatTool {
	return &MemoryCompatTool{name: name, description: description, memoryService: memoryService}
}

func (t *MemoryCompatTool) Definition() ToolDefinition {
	props := map[string]interface{}{}
	switch t.name {
	case "memory_search":
		props["query"] = map[string]interface{}{"type": "string", "description": "Search query."}
		props["limit"] = map[string]interface{}{"type": "integer", "description": "Optional max results."}
	case "memory_get", "memory_read", "memory_forget", "memory_delete":
		props["id"] = map[string]interface{}{"type": "string", "description": "Memory ID or path."}
		props["path"] = map[string]interface{}{"type": "string", "description": "Alias for id."}
	case "memory_write", "memory_remember", "memory_store":
		props["content"] = map[string]interface{}{"type": "string", "description": "Memory content to store."}
		props["tags"] = map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional tags."}
		props["category"] = map[string]interface{}{"type": "string", "description": "Single-tag alias."}
	}
	return ToolDefinition{
		Name:        t.name,
		Description: t.description,
		Parameters: map[string]interface{}{
			"type":                 "object",
			"properties":           props,
			"additionalProperties": true,
		},
	}
}

func (t *MemoryCompatTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.memoryService == nil {
		return nil, errors.New("memory service not available")
	}
	memoryTool := NewMemoryTool(t.memoryService)
	normalized, err := t.normalizeArgs(args)
	if err != nil {
		return nil, err
	}
	result, err := memoryTool.Execute(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return decodeMemoryCompatResult(result), nil
}

func (t *MemoryCompatTool) normalizeArgs(args map[string]interface{}) (map[string]interface{}, error) {
	normalized := map[string]interface{}{}
	switch t.name {
	case "memory_search":
		query := firstCompatString(args, "query", "q", "search", "vsearch", "text", "input", "content")
		if query == "" {
			return nil, errors.New("query is required for memory_search")
		}
		normalized["action"] = "search"
		normalized["query"] = query
		if limit := compatInt(args, "limit", "max_results", "n"); limit > 0 {
			normalized["limit"] = float64(limit)
		}
	case "memory_get", "memory_read":
		id := firstCompatString(args, "id", "path", "file", "memory_id", "memoryId", "key")
		if id == "" {
			return nil, errors.New("id/path is required for memory_get")
		}
		normalized["action"] = "get"
		normalized["id"] = id
	case "memory_write", "memory_remember", "memory_store":
		content := firstCompatString(args, "content", "text", "input", "memory", "note", "message")
		if content == "" {
			return nil, errors.New("content/text is required for memory_write")
		}
		normalized["action"] = "remember"
		normalized["content"] = content
		if tags, ok := compatArgValue(args, "tags"); ok && tags != nil {
			normalized["tags"] = tags
		} else if category := firstCompatString(args, "category", "tag"); category != "" {
			normalized["category"] = category
		}
	case "memory_forget", "memory_delete":
		id := firstCompatString(args, "id", "path", "file", "memory_id", "memoryId", "key")
		if id == "" {
			return nil, errors.New("id/path is required for memory_forget")
		}
		normalized["action"] = "forget"
		normalized["id"] = id
	default:
		return nil, fmt.Errorf("unsupported memory compat tool %q", t.name)
	}
	return normalized, nil
}

func decodeMemoryCompatResult(result interface{}) interface{} {
	text, ok := result.(string)
	if !ok {
		return result
	}
	var payload interface{}
	if err := json.Unmarshal([]byte(text), &payload); err == nil {
		return payload
	}
	return result
}

// RegisterMemoryCompatTools registers legacy memory_* wrappers backed by the native memory service.
func RegisterMemoryCompatTools(registry *Registry, memoryService MemoryServiceInterface) {
	if registry == nil || memoryService == nil {
		return
	}
	registry.Register(newMemoryCompatTool("memory_search", "Search memory snippets by query.", memoryService))
	registry.Register(newMemoryCompatTool("memory_get", "Read a memory entry by ID or path.", memoryService))
	registry.Register(newMemoryCompatTool("memory_write", "Write a new memory entry.", memoryService))
	registry.Register(newMemoryCompatTool("memory_forget", "Delete a memory entry by ID.", memoryService))
	for _, alias := range []string{"memory_read", "memory_remember", "memory_store", "memory_delete"} {
		registry.Register(newMemoryCompatTool(alias, "Hidden legacy memory alias.", memoryService))
	}
	for _, name := range []string{
		"memory_search", "memory_get", "memory_write", "memory_forget",
		"memory_read", "memory_remember", "memory_store", "memory_delete",
	} {
		registry.Disable(name)
	}
}
