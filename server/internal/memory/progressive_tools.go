package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// MemoryProgressiveSearchTool provides the memory_search_progressive LLM tool.
type MemoryProgressiveSearchTool struct {
	searcher *ProgressiveSearcher
}

// NewMemoryProgressiveSearchTool creates a new progressive search tool.
func NewMemoryProgressiveSearchTool(searcher *ProgressiveSearcher) *MemoryProgressiveSearchTool {
	return &MemoryProgressiveSearchTool{searcher: searcher}
}

// Definition returns the tool's definition.
func (m *MemoryProgressiveSearchTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name: "memory_search_progressive",
		Description: `Search memories with progressive disclosure for token efficiency.
Use depth=1 (index) first to scan broadly (~50 tokens/result): returns ID, title, date, type, score.
Use depth=2 (context) with specific IDs to get snippets (~150 tokens/result).
Use depth=3 (detail) to get full content for specific IDs (~500+ tokens/result).
This saves ~10x tokens vs loading full content upfront.
Workflow: depth=1 to find relevant IDs → depth=2 to preview → depth=3 for full content.`,
		Icon: "brain",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query text",
				},
				"depth": map[string]interface{}{
					"type":        "integer",
					"enum":        []int{1, 2, 3},
					"description": "1=index (titles only), 2=context (snippets), 3=detail (full content)",
				},
				"ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Specific memory IDs to expand (for depth 2 and 3)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Max results (default: 20 for depth 1, 5 for depth 2/3)",
				},
			},
			"required": []string{"query", "depth"},
		},
	}
}

// Execute performs the progressive search.
func (m *MemoryProgressiveSearchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if m.searcher == nil {
		return nil, errors.New("progressive searcher not available")
	}

	query, _ := args["query"].(string)

	depth := SearchDepthIndex
	if v, ok := args["depth"].(float64); ok {
		depth = SearchDepth(int(v))
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

	req := ProgressiveSearchRequest{
		Query: query,
		Depth: depth,
		IDs:   ids,
		Limit: limit,
	}

	resp, err := m.searcher.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("progressive search failed: %w", err)
	}

	jsonResult, _ := json.Marshal(resp)
	return string(jsonResult), nil
}

// Ensure it implements Tool interface.
var _ tools.Tool = (*MemoryProgressiveSearchTool)(nil)
