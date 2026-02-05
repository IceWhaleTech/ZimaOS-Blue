package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// MemorySearchTool performs semantic search on memories.
type MemorySearchTool struct {
	memoryService MemoryServiceInterface
}

// NewMemorySearchToolWithInterface creates a new memory search tool with interface.
func NewMemorySearchToolWithInterface(memoryService MemoryServiceInterface) *MemorySearchTool {
	return &MemorySearchTool{
		memoryService: memoryService,
	}
}

// Definition returns the tool's definition.
func (m *MemorySearchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "memory_search",
		Description: "Searches memories using semantic/hybrid search. Returns relevant memories based on the query with similarity scores.",
		Icon:        "brain",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query to find relevant memories",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return (default: 10, max: 50)",
				},
				"min_score": map[string]interface{}{
					"type":        "number",
					"description": "Minimum similarity score threshold (0.0-1.0, default: 0.0)",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Execute performs the memory search.
func (m *MemorySearchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if m.memoryService == nil {
		return nil, errors.New("memory service not available")
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, errors.New("query is required")
	}

	// Parse limit (default: 10, max: 50)
	limit := 10
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
		if limit <= 0 {
			limit = 10
		} else if limit > 50 {
			limit = 50
		}
	}

	// Parse min_score (default: 0.0)
	minScore := float32(0.0)
	if v, ok := args["min_score"].(float64); ok {
		minScore = float32(v)
		if minScore < 0 {
			minScore = 0
		} else if minScore > 1 {
			minScore = 1
		}
	}

	// Perform search
	results, err := m.memoryService.Recall(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("memory search failed: %w", err)
	}

	// Filter by min_score and format results
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

// MemoryGetTool retrieves a specific memory by ID.
type MemoryGetTool struct {
	memoryService MemoryServiceInterface
}

// NewMemoryGetToolWithInterface creates a new memory get tool with interface.
func NewMemoryGetToolWithInterface(memoryService MemoryServiceInterface) *MemoryGetTool {
	return &MemoryGetTool{
		memoryService: memoryService,
	}
}

// Definition returns the tool's definition.
func (m *MemoryGetTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "memory_get",
		Description: "Retrieves a specific memory by its ID. Returns the full content and metadata of the memory.",
		Icon:        "brain",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "The unique ID of the memory to retrieve",
				},
			},
			"required": []string{"id"},
		},
	}
}

// Execute retrieves the memory by ID.
func (m *MemoryGetTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if m.memoryService == nil {
		return nil, errors.New("memory service not available")
	}

	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, errors.New("id is required")
	}

	// Get memory
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

// MemoryStatsTool returns memory statistics.
type MemoryStatsTool struct {
	memoryService MemoryServiceInterface
}

// NewMemoryStatsToolWithInterface creates a new memory stats tool with interface.
func NewMemoryStatsToolWithInterface(memoryService MemoryServiceInterface) *MemoryStatsTool {
	return &MemoryStatsTool{
		memoryService: memoryService,
	}
}

// Definition returns the tool's definition.
func (m *MemoryStatsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "memory_stats",
		Description: "Returns statistics about the memory system including total memories, storage size, and backend information.",
		Icon:        "brain",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

// Execute returns memory statistics.
func (m *MemoryStatsTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if m.memoryService == nil {
		return nil, errors.New("memory service not available")
	}

	stats, err := m.memoryService.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	response := map[string]interface{}{
		"total_chunks":          stats.TotalChunks,
		"total_size_bytes":      stats.TotalSizeBytes,
		"oldest_chunk":          stats.OldestChunk,
		"newest_chunk":          stats.NewestChunk,
		"backend":               stats.Backend,
		"supermemory_available": m.memoryService.IsSupermemoryAvailable(),
	}

	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}
