package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	queryMode bool
}

// NewWebSearch creates a new web search skill.
func NewWebSearch() *WebSearch {
	return &WebSearch{
		manifest: &skill.Manifest{
			ID:          "web_search",
			Name:        "Web Search",
			Version:     "1.0.0",
			Description: "Legacy compatibility alias for web_query's keyword-search route. Returns result listings (title, URL, snippet) when older prompts still call web_search directly.",
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
				{
					Name:        "format",
					Type:        "string",
					Description: "Output format: xml (default) or json",
					Required:    false,
				},
			},
		},
	}
}

// NewWebQuery creates the canonical web_query skill backed by the unified
// web_query tool/runtime path rather than the legacy compatibility tool route.
func NewWebQuery() *WebSearch {
	return &WebSearch{
		manifest: &skill.Manifest{
			ID:          "web_query",
			Name:        "Web Query",
			Version:     "1.0.0",
			Description: "Unified public web discovery and reading skill. Accepts a query or URL and returns the best grounded page, title, and content when available.",
			Category:    "system",
			Icon:        "web-search",
			Tags:        []string{"web", "docs", "search", "read"},
			Inputs: []skill.Parameter{
				{
					Name:        "input",
					Type:        "string",
					Description: "A search query or target URL.",
					Required:    true,
				},
				{
					Name:        "max_results",
					Type:        "integer",
					Description: "Optional maximum discovery results.",
					Required:    false,
				},
				{
					Name:        "max_chars",
					Type:        "integer",
					Description: "Optional maximum returned content characters.",
					Required:    false,
				},
				{
					Name:        "depth",
					Type:        "string",
					Description: "Discovery depth: quick, standard, or deep.",
					Required:    false,
				},
			},
		},
		queryMode: true,
	}
}

// SetSearcher injects the web search backend.
func (w *WebSearch) SetSearcher(s WebSearcher) { w.searcher = s }

func (w *WebSearch) Manifest() *skill.Manifest { return w.manifest }

func (w *WebSearch) Validate(input map[string]any) error {
	if w.queryMode {
		normalizeWebQueryInput(input)
		rawInput, ok := input["input"]
		if !ok {
			return fmt.Errorf("input is required")
		}
		value, ok := rawInput.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("input must be a non-empty string")
		}
		input["input"] = strings.TrimSpace(value)
		return nil
	}

	normalizeWebSearchInput(input)

	q, ok := input["query"]
	if !ok {
		return fmt.Errorf("query is required")
	}
	if s, ok := q.(string); !ok || strings.TrimSpace(s) == "" {
		return fmt.Errorf("query must be a non-empty string")
	}
	input["query"] = strings.TrimSpace(q.(string))
	if rawFormat, ok := input["format"]; ok {
		format, ok := rawFormat.(string)
		if !ok {
			return fmt.Errorf("format must be a string")
		}
		normalized, err := normalizeWebSearchSkillFormat(format)
		if err != nil {
			return err
		}
		if normalized == "" {
			delete(input, "format")
		} else {
			input["format"] = normalized
		}
	}
	return nil
}

func normalizeWebSearchInput(input map[string]any) {
	normalizeUniqueStringAlias(input, "query", "q", "search", "input", "text", "content", "message", "prompt", "topic", "question")
}

func normalizeWebQueryInput(input map[string]any) {
	normalizeUniqueStringAlias(input, "input", "query", "q", "search", "url", "href", "target", "text", "content", "message", "prompt", "topic", "question")
}

func normalizeWebSearchSkillFormat(raw string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(raw))
	format = strings.Trim(format, ",.;:!?")
	if idx := strings.Index(format, ";"); idx >= 0 {
		format = strings.TrimSpace(format[:idx])
	}
	switch format {
	case "", "json", "xml":
		return format, nil
	case "md", "markdown", "text", "txt", "plain", "plaintext", "human", "jsonl", "application/json":
		return "json", nil
	case "application/xml", "text/xml":
		return "xml", nil
	default:
		return "", fmt.Errorf("format must be one of: json, xml")
	}
}

func (w *WebSearch) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := w.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}
	if w.searcher == nil {
		if w.queryMode {
			return skill.NewErrorResult(fmt.Errorf("web query not configured")), nil
		}
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

	if w.queryMode {
		rawResult, ok := result.(string)
		if !ok {
			return skill.NewErrorResult(fmt.Errorf("unexpected result type")), nil
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(rawResult), &parsed); err == nil && len(parsed) > 0 {
			return skill.NewResult(parsed), nil
		}
		return skill.NewResult(map[string]any{"result": rawResult}), nil
	}

	// WebSearchTool returns a JSON or XML string based on format.
	rawResult, ok := result.(string)
	if !ok {
		return skill.NewErrorResult(fmt.Errorf("unexpected result type")), nil
	}
	requestedFormat := "xml"
	if v, ok := input["format"].(string); ok {
		trimmed, err := normalizeWebSearchSkillFormat(v)
		if err == nil && trimmed != "" {
			requestedFormat = trimmed
		}
	}
	if requestedFormat == "xml" {
		return &skill.Result{
			Success: true,
			Data: map[string]any{
				"format": "xml",
				"result": rawResult,
			},
		}, nil
	}

	// Parse the JSON so we can emit a structured "search" card via _card hint.
	// The frontend CardSearch.vue expects {query, results[], totalCount, provider}.
	var parsed struct {
		Query   string `json:"query"`
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Source      string `json:"source,omitempty"`
		} `json:"results"`
		TotalCount int    `json:"total_count"`
		Provider   string `json:"provider"`
	}
	if err := json.Unmarshal([]byte(rawResult), &parsed); err != nil {
		// Fallback: return raw JSON
		return &skill.Result{
			Success: true,
			Data:    map[string]any{"results": rawResult},
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
