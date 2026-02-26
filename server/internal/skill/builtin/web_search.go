package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// WebSearcher is the interface for performing web searches.
// Decouples skill/builtin from tools package to avoid import cycles.
type WebSearcher interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// WebSearch is a built-in skill that wraps a web search backend for IPC access.
type WebSearch struct {
	manifest *skill.Manifest
	searcher WebSearcher
}

// NewWebSearch creates a new web search skill.
func NewWebSearch() *WebSearch {
	return &WebSearch{
		manifest: &skill.Manifest{
			ID:          "web_search",
			Name:        "Web Search",
			Version:     "1.0.0",
			Description: "Keyword web search. Returns result listings (title, URL, snippet). Does NOT open or read pages — use browser for that.",
			Category:    "system",
			Icon:        "web-search",
			Tags:        []string{"search", "web", "lookup"},
			Inputs: []skill.Parameter{
				{
					Name:        "query",
					Type:        "string",
					Description: "The search query",
					Required:    true,
				},
				{
					Name:        "max_results",
					Type:        "integer",
					Description: "Maximum results (default 10, max 20)",
					Required:    false,
				},
				{
					Name:        "region",
					Type:        "string",
					Description: "Search region (e.g. us-en, wt-wt)",
					Required:    false,
				},
			},
		},
	}
}

// SetSearcher injects the web search backend.
func (w *WebSearch) SetSearcher(s WebSearcher) { w.searcher = s }

func (w *WebSearch) Manifest() *skill.Manifest { return w.manifest }

func (w *WebSearch) Validate(input map[string]any) error {
	q, ok := input["query"]
	if !ok {
		return fmt.Errorf("query is required")
	}
	if s, ok := q.(string); !ok || s == "" {
		return fmt.Errorf("query must be a non-empty string")
	}
	return nil
}

func (w *WebSearch) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if w.searcher == nil {
		return skill.NewErrorResult(fmt.Errorf("web search not configured")), nil
	}

	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
	}

	result, err := w.searcher.Execute(ctx, args)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	// WebSearchTool returns a JSON string of WebSearchResponse.
	jsonStr, ok := result.(string)
	if !ok {
		return skill.NewErrorResult(fmt.Errorf("unexpected result type")), nil
	}

	// Parse the JSON so we can emit a structured "search" card via _card hint.
	// The frontend CardSearch.vue expects {query, results[], totalCount, provider}.
	var parsed struct {
		Query      string `json:"query"`
		Results    []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Source      string `json:"source,omitempty"`
		} `json:"results"`
		TotalCount int    `json:"total_count"`
		Provider   string `json:"provider"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		// Fallback: return raw JSON
		return &skill.Result{
			Success: true,
			Data:    map[string]any{"results": jsonStr},
		}, nil
	}

	// Build results array for the card
	results := make([]map[string]any, len(parsed.Results))
	for i, r := range parsed.Results {
		results[i] = map[string]any{
			"title":       r.Title,
			"url":         r.URL,
			"description": r.Description,
		}
	}

	return &skill.Result{
		Success: true,
		Data: map[string]any{
			"_card":      "search",
			"query":      parsed.Query,
			"results":    results,
			"totalCount": parsed.TotalCount,
			"provider":   parsed.Provider,
		},
	}, nil
}
