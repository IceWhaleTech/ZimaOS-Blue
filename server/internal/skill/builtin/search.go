package builtin

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
)

// Search is a built-in search skill for local content
type Search struct {
	manifest *skill.Manifest
}

// NewSearch creates a new search skill
func NewSearch() *Search {
	return &Search{
		manifest: &skill.Manifest{
			ID:          "search",
			Name:        "Search",
			Version:     "1.0.0",
			Description: "Search and filter text content",
			Category:    "utility",
			Icon:        "search",
			Tags:        []string{"search", "filter", "text", "utility"},
			Inputs: []skill.Parameter{
				{
					Name:        "query",
					Type:        "string",
					Description: "Search query or pattern",
					Required:    true,
				},
				{
					Name:        "content",
					Type:        "string",
					Description: "Content to search in",
					Required:    true,
				},
				{
					Name:        "mode",
					Type:        "string",
					Description: "Search mode: 'contains', 'exact', 'regex', 'fuzzy'",
					Required:    false,
					Default:     "contains",
				},
				{
					Name:        "case_sensitive",
					Type:        "boolean",
					Description: "Whether search is case sensitive",
					Required:    false,
					Default:     false,
				},
				{
					Name:        "max_results",
					Type:        "number",
					Description: "Maximum number of results to return",
					Required:    false,
					Default:     100,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "results",
					Type:        "array",
					Description: "Search results with line numbers and matched content",
				},
				{
					Name:        "count",
					Type:        "number",
					Description: "Total number of matches",
				},
			},
		},
	}
}

// Manifest returns the skill manifest
func (s *Search) Manifest() *skill.Manifest {
	return s.manifest
}

// Validate validates the input parameters
func (s *Search) Validate(input map[string]any) error {
	if _, ok := input["query"]; !ok {
		return fmt.Errorf("query is required")
	}
	if _, ok := input["query"].(string); !ok {
		return fmt.Errorf("query must be a string")
	}

	if _, ok := input["content"]; !ok {
		return fmt.Errorf("content is required")
	}
	if _, ok := input["content"].(string); !ok {
		return fmt.Errorf("content must be a string")
	}

	if mode, ok := input["mode"]; ok {
		modeStr, ok := mode.(string)
		if !ok {
			return fmt.Errorf("mode must be a string")
		}
		validModes := map[string]bool{"contains": true, "exact": true, "regex": true, "fuzzy": true}
		if !validModes[modeStr] {
			return fmt.Errorf("invalid mode: %s", modeStr)
		}
	}

	return nil
}

// Execute executes the search skill
func (s *Search) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	query := input["query"].(string)
	content := input["content"].(string)

	mode := "contains"
	if m, ok := input["mode"].(string); ok {
		mode = m
	}

	caseSensitive := false
	if cs, ok := input["case_sensitive"].(bool); ok {
		caseSensitive = cs
	}

	maxResults := 100
	if mr, ok := input["max_results"].(float64); ok {
		maxResults = int(mr)
	} else if mr, ok := input["max_results"].(int); ok {
		maxResults = mr
	}

	var results []map[string]any
	var err error

	switch mode {
	case "contains":
		results, err = s.searchContains(query, content, caseSensitive, maxResults)
	case "exact":
		results, err = s.searchExact(query, content, caseSensitive, maxResults)
	case "regex":
		results, err = s.searchRegex(query, content, caseSensitive, maxResults)
	case "fuzzy":
		results, err = s.searchFuzzy(query, content, caseSensitive, maxResults)
	default:
		return skill.NewErrorResult(fmt.Errorf("unknown search mode: %s", mode)), nil
	}

	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"results": results,
		"count":   len(results),
		"query":   query,
		"mode":    mode,
	}), nil
}

// searchContains performs a simple contains search
func (s *Search) searchContains(query, content string, caseSensitive bool, maxResults int) ([]map[string]any, error) {
	var results []map[string]any
	lines := strings.Split(content, "\n")

	searchQuery := query
	if !caseSensitive {
		searchQuery = strings.ToLower(query)
	}

	for i, line := range lines {
		if len(results) >= maxResults {
			break
		}

		searchLine := line
		if !caseSensitive {
			searchLine = strings.ToLower(line)
		}

		if strings.Contains(searchLine, searchQuery) {
			results = append(results, map[string]any{
				"line_number": i + 1,
				"content":     line,
				"match_type":  "contains",
			})
		}
	}

	return results, nil
}

// searchExact performs an exact match search
func (s *Search) searchExact(query, content string, caseSensitive bool, maxResults int) ([]map[string]any, error) {
	var results []map[string]any
	lines := strings.Split(content, "\n")

	searchQuery := query
	if !caseSensitive {
		searchQuery = strings.ToLower(query)
	}

	for i, line := range lines {
		if len(results) >= maxResults {
			break
		}

		searchLine := line
		if !caseSensitive {
			searchLine = strings.ToLower(line)
		}

		if strings.TrimSpace(searchLine) == strings.TrimSpace(searchQuery) {
			results = append(results, map[string]any{
				"line_number": i + 1,
				"content":     line,
				"match_type":  "exact",
			})
		}
	}

	return results, nil
}

// searchRegex performs a regex search
func (s *Search) searchRegex(query, content string, caseSensitive bool, maxResults int) ([]map[string]any, error) {
	pattern := query
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var results []map[string]any
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if len(results) >= maxResults {
			break
		}

		matches := re.FindAllStringIndex(line, -1)
		if len(matches) > 0 {
			matchPositions := make([]map[string]int, len(matches))
			for j, match := range matches {
				matchPositions[j] = map[string]int{
					"start": match[0],
					"end":   match[1],
				}
			}

			results = append(results, map[string]any{
				"line_number": i + 1,
				"content":     line,
				"match_type":  "regex",
				"matches":     matchPositions,
			})
		}
	}

	return results, nil
}

// searchFuzzy performs a fuzzy search using simple similarity
func (s *Search) searchFuzzy(query, content string, caseSensitive bool, maxResults int) ([]map[string]any, error) {
	var results []map[string]any
	lines := strings.Split(content, "\n")

	searchQuery := query
	if !caseSensitive {
		searchQuery = strings.ToLower(query)
	}

	type scoredResult struct {
		lineNumber int
		content    string
		score      float64
	}

	var scored []scoredResult

	for i, line := range lines {
		searchLine := line
		if !caseSensitive {
			searchLine = strings.ToLower(line)
		}

		score := s.fuzzyScore(searchQuery, searchLine)
		if score > 0.3 { // Minimum threshold
			scored = append(scored, scoredResult{
				lineNumber: i + 1,
				content:    line,
				score:      score,
			})
		}
	}

	// Sort by score (simple bubble sort for small datasets)
	for i := 0; i < len(scored)-1; i++ {
		for j := 0; j < len(scored)-i-1; j++ {
			if scored[j].score < scored[j+1].score {
				scored[j], scored[j+1] = scored[j+1], scored[j]
			}
		}
	}

	// Take top results
	for i := 0; i < len(scored) && i < maxResults; i++ {
		results = append(results, map[string]any{
			"line_number": scored[i].lineNumber,
			"content":     scored[i].content,
			"match_type":  "fuzzy",
			"score":       scored[i].score,
		})
	}

	return results, nil
}

// fuzzyScore calculates a simple fuzzy match score
func (s *Search) fuzzyScore(query, text string) float64 {
	if len(query) == 0 || len(text) == 0 {
		return 0
	}

	// Check for exact substring match first
	if strings.Contains(text, query) {
		return 1.0
	}

	// Calculate character overlap score
	queryChars := make(map[rune]int)
	for _, c := range query {
		queryChars[c]++
	}

	matches := 0
	for _, c := range text {
		if queryChars[c] > 0 {
			matches++
			queryChars[c]--
		}
	}

	// Score based on how many query characters were found
	return float64(matches) / float64(len(query))
}
