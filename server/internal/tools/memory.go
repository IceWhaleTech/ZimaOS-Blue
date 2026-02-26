package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// MemoryTool is a unified tool for memory operations: search, get, remember, forget, stats.
type MemoryTool struct {
	memoryService MemoryServiceInterface
}

// reThink strips <think>...</think> blocks from memory content.
var reThink = regexp.MustCompile(`<think>[\s\S]*?</think>`)

// cleanMemoryContent removes LLM artifacts (<think> blocks) from stored memory content.
func cleanMemoryContent(s string) string {
	s = reThink.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// NewMemoryTool creates a new unified memory tool.
func NewMemoryTool(memoryService MemoryServiceInterface) *MemoryTool {
	return &MemoryTool{memoryService: memoryService}
}

// Definition returns the tool's definition.
func (m *MemoryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "memory",
		Description: `Search and manage the user's personal memory store. Only use when the question is about prior conversations, saved notes, personal preferences, or past decisions. Do NOT use for general knowledge questions, greetings, or casual chat. Actions:
- search: Find relevant memories by keyword query (returns scored snippets with path + lines)
- remember: Store a new memory (use when the user says "remember", "note this", "don't forget")
- get: Read a specific memory file by path (use after search to pull only the needed lines)
- forget: Delete a specific memory by ID
- stats: Get memory system statistics`,
		Icon: "memory",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"search", "get", "remember", "forget", "stats"},
					"description": "The memory operation to perform",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query (required for 'search')",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Memory file path or ID (required for 'get' and 'forget')",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Memory content to store (required for 'remember')",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tags for the memory (optional, for 'remember')",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Max results (default: 10)",
				},
			},
			"required": []string{"action"},
		},
	}
}

// Execute dispatches to the appropriate memory operation.
func (m *MemoryTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if m.memoryService == nil {
		return nil, errors.New("memory service not available")
	}

	action, _ := args["action"].(string)
	switch action {
	case "search":
		return m.executeSearch(ctx, args)
	case "get":
		return m.executeGet(ctx, args)
	case "remember":
		return m.executeRemember(ctx, args)
	case "forget":
		return m.executeForget(ctx, args)
	case "stats":
		return m.executeStats(ctx)
	default:
		return nil, fmt.Errorf("unknown action: %s (use search, get, remember, forget, or stats)", action)
	}
}

func (m *MemoryTool) executeSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required for search action")
	}

	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
		if limit <= 0 {
			limit = 10
		} else if limit > 50 {
			limit = 50
		}
	}

	results, err := m.memoryService.Recall(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("memory search failed: %w", err)
	}

	filteredResults := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		content := cleanMemoryContent(r.Chunk.Content)
		if content == "" {
			continue
		}
		filteredResults = append(filteredResults, map[string]interface{}{
			"id":         r.Chunk.ID,
			"content":    content,
			"score":      r.CombinedScore,
			"match_types": r.MatchTypes,
			"created_at": r.Chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"metadata":   r.Chunk.Metadata,
		})
	}

	response := map[string]interface{}{
		"query":   query,
		"count":   len(filteredResults),
		"results": filteredResults,
		"backend": m.memoryService.GetActiveBackend(),
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func (m *MemoryTool) executeGet(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, errors.New("id is required for get action")
	}

	chunk, err := m.memoryService.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}

	response := map[string]interface{}{
		"id":         chunk.ID,
		"content":    cleanMemoryContent(chunk.Content),
		"metadata":   chunk.Metadata,
		"created_at": chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at": chunk.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"backend":    m.memoryService.GetActiveBackend(),
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func (m *MemoryTool) executeRemember(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return nil, errors.New("content is required for remember action")
	}

	var tags []string
	if rawTags, ok := args["tags"].([]interface{}); ok {
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	chunk, err := m.memoryService.Remember(ctx, content, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to store memory: %w", err)
	}

	response := map[string]interface{}{
		"id":         chunk.ID,
		"content":    chunk.Content,
		"created_at": chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"backend":    m.memoryService.GetActiveBackend(),
		"success":    true,
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func (m *MemoryTool) executeForget(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, errors.New("id is required for forget action")
	}

	if err := m.memoryService.Forget(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to forget memory: %w", err)
	}

	response := map[string]interface{}{
		"id":      id,
		"success": true,
		"backend": m.memoryService.GetActiveBackend(),
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func (m *MemoryTool) executeStats(ctx context.Context) (interface{}, error) {
	stats, err := m.memoryService.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	response := map[string]interface{}{
		"total_chunks":     stats.TotalChunks,
		"total_size_bytes": stats.TotalSizeBytes,
		"oldest_chunk":     stats.OldestChunk,
		"newest_chunk":     stats.NewestChunk,
		"backend":          stats.Backend,
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}
