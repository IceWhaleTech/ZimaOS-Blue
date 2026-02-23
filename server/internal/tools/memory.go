package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// MemoryTool is a unified tool for all memory operations: search, get, stats, and progressive search.
type MemoryTool struct {
	memoryService MemoryServiceInterface
}

// NewMemoryTool creates a new unified memory tool.
func NewMemoryTool(memoryService MemoryServiceInterface) *MemoryTool {
	return &MemoryTool{memoryService: memoryService}
}

// Definition returns the tool's definition.
func (m *MemoryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "memory",
		Description: `Search, retrieve, and manage the memory system. Actions:
- search: Find relevant memories by query (returns scored results)
- get: Retrieve a specific memory by ID (full content + metadata)
- remember: Store a new memory with optional tags
- forget: Delete a specific memory by ID
- stats: Get memory system statistics (total count, size, backend)
- progressive_search: Token-efficient multi-depth search (depth 1=index, 2=context, 3=detail)`,
		Icon: "brain",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"search", "get", "remember", "forget", "stats", "progressive_search"},
					"description": "The memory operation to perform",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query (required for 'search' and 'progressive_search')",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Memory ID (required for 'get' and 'forget')",
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
					"description": "Max results (default: 10 for search, 20 for progressive depth 1)",
				},
				"min_score": map[string]interface{}{
					"type":        "number",
					"description": "Minimum similarity score 0.0-1.0 (for 'search', default: 0.0)",
				},
				"depth": map[string]interface{}{
					"type":        "integer",
					"enum":        []int{1, 2, 3},
					"description": "Progressive search depth: 1=index, 2=context, 3=detail (for 'progressive_search')",
				},
				"ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Memory IDs to expand (for progressive_search depth 2/3)",
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
	case "progressive_search":
		return m.executeProgressiveSearch(ctx, args)
	default:
		return nil, fmt.Errorf("unknown action: %s (use search, get, remember, forget, stats, or progressive_search)", action)
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

	minScore := float32(0.0)
	if v, ok := args["min_score"].(float64); ok {
		minScore = float32(v)
		if minScore < 0 {
			minScore = 0
		} else if minScore > 1 {
			minScore = 1
		}
	}

	results, err := m.memoryService.Recall(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("memory search failed: %w", err)
	}

	filteredResults := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		if r.CombinedScore < minScore {
			continue
		}
		filteredResults = append(filteredResults, map[string]interface{}{
			"id":            r.Chunk.ID,
			"content":       r.Chunk.Content,
			"score":         r.CombinedScore,
			"vector_score":  r.VectorScore,
			"keyword_score": r.KeywordScore,
			"match_types":   r.MatchTypes,
			"created_at":    r.Chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"metadata":      r.Chunk.Metadata,
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
		"content":    chunk.Content,
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

func (m *MemoryTool) executeProgressiveSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	ps, ok := m.memoryService.(ProgressiveSearchInterface)
	if !ok || ps == nil {
		return nil, errors.New("progressive search not available")
	}

	query, _ := args["query"].(string)

	depth := 1
	if v, ok := args["depth"].(float64); ok {
		depth = int(v)
	}

	var ids []string
	if rawIDs, ok := args["ids"].([]interface{}); ok {
		for _, id := range rawIDs {
			if s, ok := id.(string); ok {
				ids = append(ids, s)
			}
		}
	}

	limit := 0
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}

	result, err := ps.ProgressiveSearch(ctx, query, depth, ids, limit)
	if err != nil {
		return nil, fmt.Errorf("progressive search failed: %w", err)
	}

	jsonResult, _ := json.Marshal(result)
	return string(jsonResult), nil
}

// ProgressiveSearchInterface is an optional interface for progressive search support.
// The memory service can implement this to enable progressive_search action.
type ProgressiveSearchInterface interface {
	ProgressiveSearch(ctx context.Context, query string, depth int, ids []string, limit int) (interface{}, error)
}
