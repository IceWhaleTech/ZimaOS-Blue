package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type WebQueryLLMCompactionMode string

const (
	WebQueryLLMCompactionDefault WebQueryLLMCompactionMode = "default"
	WebQueryLLMCompactionSummary WebQueryLLMCompactionMode = "summary"
	WebQueryLLMCompactionMinimal WebQueryLLMCompactionMode = "minimal"
)

type WebQueryLLMCompactionOptions struct {
	ByteBudget           int
	Mode                 WebQueryLLMCompactionMode
	ResultLimit          int
	FactLimit            int
	WarningLimit         int
	Materialize          bool
	ToolCallID           string
	ConversationID       string
	SearchRound          int
	OmittedFromLLM       bool
	OmittedReason        string
	ToolName             string
	ForceMaterialization bool
}

type webQueryLLMResult struct {
	Rank     int
	Title    string
	URL      string
	FinalURL string
	Snippet  string
	Source   string
	Selected bool
}

type webQueryLLMWarning struct {
	Code    string
	Message string
}

type webQueryLLMSummaryData struct {
	Status                string
	Mode                  string
	Provider              string
	Query                 string
	Input                 string
	Route                 string
	TotalCount            int
	HasResults            bool
	SelectedResult        *webQueryLLMResult
	SelectedSourceRank    int
	Results               []webQueryLLMResult
	KeyFacts              []string
	Warnings              []webQueryLLMWarning
	WarningCount          int
	Error                 string
	ArtifactPath          string
	Materialized          bool
	AlreadyCompacted      bool
	SearchCardEmitted     bool
	OriginalAuditContent  string
	ExistingCompactSource map[string]interface{}
}

type webQueryLLMRenderConfig struct {
	maxQueryBytes    int
	maxTitleBytes    int
	maxURLBytes      int
	maxSnippetBytes  int
	maxFactBytes     int
	maxWarningBytes  int
	maxResults       int
	maxFacts         int
	maxWarnings      int
	includeResults   bool
	includeWarnings  bool
	includeQuery     bool
	includeInput     bool
	includeProvider  bool
	includeRoute     bool
	includeError     bool
	includeSnippet   bool
	includeFactList  bool
	includeWarnList  bool
	includeSelected  bool
	includeMaterial  bool
	includeSearchTag bool
}

var (
	webQueryLLMWhitespaceRE     = regexp.MustCompile(`\s+`)
	webQueryLLMURLRE            = regexp.MustCompile(`https?://\S+`)
	webQueryLLMNonWordRE        = regexp.MustCompile(`[^a-z0-9%$#+./:-]+`)
	webQueryLLMVersionRE        = regexp.MustCompile(`(?i)\bv?\d+\.\d+(?:\.\d+)?\b`)
	webQueryLLMDateRE           = regexp.MustCompile(`(?i)\b(?:20\d{2}[-/]\d{1,2}[-/]\d{1,2}|jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:t(?:ember)?)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\b`)
	webQueryLLMMetricRE         = regexp.MustCompile(`(?i)(?:\b\d+(?:\.\d+)?(?:%|x|ms|s|sec|min|minute|hour|day|week|month|year|kb|mb|gb|tb)\b|\$\d+(?:\.\d+)?|\b(?:price|version|release|released|published|updated|fix(?:es)?|improvement(?:s)?|migration(?:s)?|candidate(?:s)?|selected)\b)`)
	webQueryLLMSafeSegmentRE    = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	webQueryLLMCompactFieldKeys = []string{"has_results", "selected_result", "key_facts", "research_artifact_path", "llm_compacted"}
)

func BuildCompactWebQueryPayloadForLLM(ctx context.Context, raw interface{}, auditContent string, opts WebQueryLLMCompactionOptions) (map[string]interface{}, bool) {
	compactPayload, compactOK := parseWebQueryLLMMap(raw)
	auditPayload, auditOK := parseWebQueryLLMMap(auditContent)
	if !compactOK && !auditOK {
		return nil, false
	}

	source := compactPayload
	carried := compactPayload
	if auditOK {
		if !compactOK || looksLikeCompactWebQueryPayload(compactPayload) {
			source = auditPayload
		}
		if carried == nil {
			carried = auditPayload
		}
	}
	if source == nil {
		source = compactPayload
	}
	if carried == nil {
		carried = source
	}
	source = unwrapWebQueryPayloadForLLM(source)
	carried = unwrapWebQueryPayloadForLLM(carried)

	data := extractWebQueryLLMSummaryData(source, carried)
	if strings.TrimSpace(auditContent) != "" {
		data.OriginalAuditContent = strings.TrimSpace(auditContent)
	} else {
		data.OriginalAuditContent = serializeCompactWebQueryAuditForLLM(source)
	}

	if data.ArtifactPath == "" && shouldMaterializeWebQuerySummary(ctx, data, opts) {
		if artifactPath, ok := materializeWebQuerySummaryArtifact(ctx, data, opts); ok {
			data.ArtifactPath = artifactPath
			data.Materialized = true
		}
	}

	configs := buildWebQueryLLMRenderConfigs(opts)
	budget := opts.ByteBudget
	if budget <= 0 {
		budget = 8 * 1024
	}
	for _, cfg := range configs {
		summary := renderWebQueryLLMSummary(data, opts, cfg)
		encoded, err := json.Marshal(summary)
		if err == nil && len(encoded) <= budget {
			return summary, true
		}
	}

	emergencyConfigs := []webQueryLLMRenderConfig{
		{
			maxQueryBytes:    80,
			maxTitleBytes:    56,
			maxURLBytes:      96,
			maxSnippetBytes:  0,
			maxFactBytes:     64,
			maxWarningBytes:  64,
			maxResults:       0,
			maxFacts:         0,
			maxWarnings:      0,
			includeQuery:     false,
			includeInput:     false,
			includeProvider:  true,
			includeRoute:     false,
			includeError:     false,
			includeSnippet:   false,
			includeFactList:  false,
			includeWarnList:  false,
			includeSelected:  true,
			includeMaterial:  true,
			includeSearchTag: opts.OmittedFromLLM,
		},
		{
			maxQueryBytes:    48,
			maxTitleBytes:    40,
			maxURLBytes:      72,
			maxSnippetBytes:  0,
			maxFactBytes:     48,
			maxWarningBytes:  48,
			maxResults:       0,
			maxFacts:         0,
			maxWarnings:      0,
			includeQuery:     false,
			includeInput:     false,
			includeProvider:  false,
			includeRoute:     false,
			includeError:     false,
			includeSnippet:   false,
			includeFactList:  false,
			includeWarnList:  false,
			includeSelected:  true,
			includeMaterial:  true,
			includeSearchTag: opts.OmittedFromLLM,
		},
	}
	for _, cfg := range emergencyConfigs {
		summary := renderWebQueryLLMSummary(data, opts, cfg)
		encoded, err := json.Marshal(summary)
		if err == nil && len(encoded) <= budget {
			return summary, true
		}
	}

	fallback := map[string]interface{}{
		"status":        truncateUTF8BytesForWebQueryLLM(firstNonEmpty(data.Status, webQueryStatusOK), 24),
		"has_results":   data.HasResults,
		"llm_compacted": true,
	}
	if data.TotalCount > 0 {
		fallback["total_count"] = data.TotalCount
	}
	if data.SelectedResult != nil {
		fallback["selected_result"] = webQueryResultToSummaryMap(*data.SelectedResult, webQueryLLMRenderConfig{
			maxTitleBytes:   32,
			maxURLBytes:     48,
			maxSnippetBytes: 0,
			includeSnippet:  false,
		})
	}
	if data.ArtifactPath != "" {
		fallback["research_artifact_path"] = truncateUTF8BytesForWebQueryLLM(data.ArtifactPath, 64)
		fallback["materialized"] = true
	}
	return fallback, true
}

func parseWebQueryLLMMap(raw interface{}) (map[string]interface{}, bool) {
	switch typed := raw.(type) {
	case nil:
		return nil, false
	case map[string]interface{}:
		return cloneJSONInterfaceMap(typed), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, false
		}
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
			return nil, false
		}
		return out, true
	case []byte:
		return parseWebQueryLLMMap(string(typed))
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil, false
		}
		return parseWebQueryLLMMap(string(data))
	}
}

func unwrapWebQueryPayloadForLLM(payload map[string]interface{}) map[string]interface{} {
	if len(payload) == 0 {
		return payload
	}
	data, ok := payload["data"].(map[string]interface{})
	if !ok || len(data) == 0 {
		return payload
	}
	if strings.TrimSpace(anyToStringForLLM(payload["query"])) != "" ||
		strings.TrimSpace(anyToStringForLLM(payload["input"])) != "" ||
		strings.TrimSpace(anyToStringForLLM(payload["title"])) != "" ||
		strings.TrimSpace(anyToStringForLLM(payload["target_url"])) != "" ||
		payload["sources"] != nil ||
		payload["selected_result"] != nil {
		return payload
	}
	return cloneJSONInterfaceMap(data)
}

func looksLikeCompactWebQueryPayload(payload map[string]interface{}) bool {
	if len(payload) == 0 {
		return false
	}
	for _, key := range webQueryLLMCompactFieldKeys {
		if _, ok := payload[key]; ok {
			return true
		}
	}
	return false
}

func extractWebQueryLLMSummaryData(source, carried map[string]interface{}) webQueryLLMSummaryData {
	results := parseWebQueryLLMResults(source["sources"])
	if len(results) == 0 {
		results = parseWebQueryLLMResults(source["results"])
	}
	if len(results) == 0 {
		results = parseWebQueryLLMResults(carried["sources"])
	}
	if len(results) == 0 {
		results = parseWebQueryLLMResults(carried["results"])
	}
	if len(results) > 1 {
		sortWebQueryLLMResultsByRank(results)
	}

	selected, selectedRank := extractSelectedWebQueryLLMResult(source, results)
	if selected == nil {
		selected, selectedRank = extractSelectedWebQueryLLMResult(carried, results)
	}
	if selected != nil && selectedRank <= 0 {
		selectedRank = selected.Rank
	}

	warnings := parseWebQueryLLMWarnings(source["warnings"])
	if len(warnings) == 0 {
		warnings = parseWebQueryLLMWarnings(carried["warnings"])
	}
	warningCount := anyToIntForWebQueryLLM(source["warning_count"])
	if warningCount <= 0 {
		warningCount = len(warnings)
	}
	if warningCount <= 0 {
		warningCount = anyToIntForWebQueryLLM(carried["warning_count"])
	}

	query := firstNonEmpty(
		anyToStringForLLM(source["query"]),
		anyToStringForLLM(source["input"]),
		anyToStringForLLM(carried["query"]),
		anyToStringForLLM(carried["input"]),
	)
	input := firstNonEmpty(
		anyToStringForLLM(source["input"]),
		anyToStringForLLM(carried["input"]),
		query,
	)
	provider := firstNonEmpty(
		anyToStringForLLM(source["provider"]),
		anyToStringForLLM(carried["provider"]),
	)
	if provider == "" {
		for _, item := range results {
			if strings.TrimSpace(item.Source) != "" {
				provider = strings.TrimSpace(item.Source)
				break
			}
		}
	}

	status := firstNonEmpty(anyToStringForLLM(source["status"]), anyToStringForLLM(carried["status"]), webQueryStatusOK)
	mode := firstNonEmpty(anyToStringForLLM(source["mode"]), anyToStringForLLM(carried["mode"]))
	route := firstNonEmpty(anyToStringForLLM(source["route"]), routeFromDiagnostics(source["diagnostics"]), anyToStringForLLM(carried["route"]), routeFromDiagnostics(carried["diagnostics"]))
	totalCount := anyToIntForWebQueryLLM(source["total_count"])
	if totalCount <= 0 {
		totalCount = anyToIntForWebQueryLLM(source["candidate_count"])
	}
	if totalCount <= 0 {
		totalCount = candidateCountFromDiagnostics(source["diagnostics"])
	}
	if totalCount <= 0 {
		totalCount = anyToIntForWebQueryLLM(carried["total_count"])
	}
	if totalCount <= 0 {
		totalCount = anyToIntForWebQueryLLM(carried["candidate_count"])
	}
	if totalCount <= 0 {
		totalCount = candidateCountFromDiagnostics(carried["diagnostics"])
	}
	if totalCount <= 0 && len(results) > 0 {
		totalCount = len(results)
	}

	factCandidates := extractExistingWebQueryFacts(carried["key_facts"])
	if len(factCandidates) == 0 {
		factCandidates = extractExistingWebQueryFacts(source["key_facts"])
	}
	factCandidates = append(factCandidates, buildWebQueryFactCandidates(source, selected, results, warnings)...)
	if len(factCandidates) == 0 {
		factCandidates = append(factCandidates, buildWebQueryFactCandidates(carried, selected, results, warnings)...)
	}
	keyFacts := dedupeWebQueryFacts(factCandidates)

	hasResults := totalCount > 0 || len(results) > 0 || selected != nil
	artifactPath := firstNonEmpty(anyToStringForLLM(source["research_artifact_path"]), anyToStringForLLM(carried["research_artifact_path"]))
	materialized := anyToBoolForWebQueryLLM(source["materialized"]) || anyToBoolForWebQueryLLM(carried["materialized"])
	if artifactPath != "" {
		materialized = true
	}

	if selectedRank <= 0 {
		selectedRank = anyToIntForWebQueryLLM(source["selected_source_rank"])
	}
	if selectedRank <= 0 {
		selectedRank = anyToIntForWebQueryLLM(carried["selected_source_rank"])
	}
	if selectedRank <= 0 {
		selectedRank = selectedSourceFromDiagnostics(source["diagnostics"])
	}
	if selectedRank <= 0 {
		selectedRank = selectedSourceFromDiagnostics(carried["diagnostics"])
	}

	return webQueryLLMSummaryData{
		Status:                status,
		Mode:                  mode,
		Provider:              provider,
		Query:                 query,
		Input:                 input,
		Route:                 route,
		TotalCount:            totalCount,
		HasResults:            hasResults,
		SelectedResult:        selected,
		SelectedSourceRank:    selectedRank,
		Results:               results,
		KeyFacts:              keyFacts,
		Warnings:              warnings,
		WarningCount:          warningCount,
		Error:                 firstNonEmpty(anyToStringForLLM(source["error"]), anyToStringForLLM(carried["error"])),
		ArtifactPath:          artifactPath,
		Materialized:          materialized,
		AlreadyCompacted:      looksLikeCompactWebQueryPayload(source) || looksLikeCompactWebQueryPayload(carried) || anyToBoolForWebQueryLLM(source["llm_compacted"]) || anyToBoolForWebQueryLLM(carried["llm_compacted"]),
		SearchCardEmitted:     anyToBoolForWebQueryLLM(source["search_card_emitted"]) || anyToBoolForWebQueryLLM(carried["search_card_emitted"]),
		ExistingCompactSource: carried,
	}
}

func parseWebQueryLLMResults(raw interface{}) []webQueryLLMResult {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}
	results := make([]webQueryLLMResult, 0, len(items))
	for idx, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		result := webQueryLLMResult{
			Rank:     anyToIntForWebQueryLLM(row["rank"]),
			Title:    firstNonEmpty(anyToStringForLLM(row["title"]), anyToStringForLLM(row["name"])),
			URL:      anyToStringForLLM(row["url"]),
			FinalURL: anyToStringForLLM(row["final_url"]),
			Snippet:  firstNonEmpty(anyToStringForLLM(row["snippet"]), anyToStringForLLM(row["description"])),
			Source:   anyToStringForLLM(row["source"]),
			Selected: anyToBoolForWebQueryLLM(row["selected"]),
		}
		if result.Rank <= 0 {
			result.Rank = idx + 1
		}
		if result.URL == "" && result.FinalURL != "" {
			result.URL = result.FinalURL
		}
		if result.FinalURL == "" && result.URL != "" {
			result.FinalURL = result.URL
		}
		if result.URL == "" && result.Title == "" && result.Snippet == "" {
			continue
		}
		results = append(results, result)
	}
	return results
}

func extractSelectedWebQueryLLMResult(payload map[string]interface{}, results []webQueryLLMResult) (*webQueryLLMResult, int) {
	if row, ok := payload["selected_result"].(map[string]interface{}); ok {
		selected := webQueryLLMResult{
			Rank:     anyToIntForWebQueryLLM(row["rank"]),
			Title:    anyToStringForLLM(row["title"]),
			URL:      anyToStringForLLM(row["url"]),
			FinalURL: anyToStringForLLM(row["final_url"]),
			Snippet:  firstNonEmpty(anyToStringForLLM(row["snippet"]), anyToStringForLLM(row["description"])),
			Source:   anyToStringForLLM(row["source"]),
			Selected: true,
		}
		if selected.URL == "" && selected.FinalURL != "" {
			selected.URL = selected.FinalURL
		}
		if selected.FinalURL == "" && selected.URL != "" {
			selected.FinalURL = selected.URL
		}
		rank := anyToIntForWebQueryLLM(payload["selected_source_rank"])
		if rank <= 0 {
			rank = selected.Rank
		}
		return &selected, rank
	}

	for _, item := range results {
		if item.Selected {
			selected := item
			selected.Selected = true
			return &selected, item.Rank
		}
	}

	selectedRank := anyToIntForWebQueryLLM(payload["selected_source_rank"])
	if selectedRank <= 0 {
		selectedRank = selectedSourceFromDiagnostics(payload["diagnostics"])
	}
	if selectedRank > 0 {
		for _, item := range results {
			if item.Rank == selectedRank {
				selected := item
				selected.Selected = true
				return &selected, selectedRank
			}
		}
	}

	title := anyToStringForLLM(payload["title"])
	targetURL := firstNonEmpty(anyToStringForLLM(payload["final_url"]), anyToStringForLLM(payload["target_url"]), anyToStringForLLM(payload["url"]))
	description := firstNonEmpty(anyToStringForLLM(payload["content"]), contentFromNestedWebQueryPayload(payload), anyToStringForLLM(payload["summary"]))
	if title == "" && targetURL == "" && description == "" {
		return nil, 0
	}
	selected := &webQueryLLMResult{
		Rank:     selectedRank,
		Title:    title,
		URL:      targetURL,
		FinalURL: targetURL,
		Snippet:  description,
		Source:   anyToStringForLLM(payload["provider"]),
		Selected: true,
	}
	if selected.Rank <= 0 && len(results) > 0 {
		selected.Rank = 1
	}
	return selected, selected.Rank
}

func parseWebQueryLLMWarnings(raw interface{}) []webQueryLLMWarning {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		if single := anyToStringForLLM(raw); single != "" {
			return []webQueryLLMWarning{{Message: single}}
		}
		return nil
	}
	warnings := make([]webQueryLLMWarning, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]interface{}:
			warning := webQueryLLMWarning{
				Code:    anyToStringForLLM(typed["code"]),
				Message: firstNonEmpty(anyToStringForLLM(typed["message"]), anyToStringForLLM(typed["detail"])),
			}
			if warning.Code == "" && warning.Message == "" {
				continue
			}
			warnings = append(warnings, warning)
		default:
			if message := anyToStringForLLM(item); message != "" {
				warnings = append(warnings, webQueryLLMWarning{Message: message})
			}
		}
	}
	return warnings
}

func extractExistingWebQueryFacts(raw interface{}) []string {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		if single := anyToStringForLLM(raw); single != "" {
			return []string{single}
		}
		return nil
	}
	facts := make([]string, 0, len(items))
	for _, item := range items {
		if fact := anyToStringForLLM(item); fact != "" {
			facts = append(facts, fact)
		}
	}
	return facts
}

func buildWebQueryFactCandidates(payload map[string]interface{}, selected *webQueryLLMResult, results []webQueryLLMResult, warnings []webQueryLLMWarning) []string {
	candidates := make([]string, 0, 16)
	if selected != nil {
		if selected.Title != "" {
			candidates = append(candidates, selected.Title)
		}
		if selected.Snippet != "" {
			candidates = append(candidates, extractFactSentencesForWebQueryLLM(selected.Snippet)...)
		}
	}
	if content := firstNonEmpty(anyToStringForLLM(payload["content"]), contentFromNestedWebQueryPayload(payload)); content != "" {
		candidates = append(candidates, extractFactSentencesForWebQueryLLM(content)...)
	}
	for _, item := range results {
		if item.Snippet != "" {
			candidates = append(candidates, extractFactSentencesForWebQueryLLM(item.Snippet)...)
		}
	}
	for _, warning := range warnings {
		text := strings.TrimSpace(strings.TrimSpace(warning.Code) + ": " + strings.TrimSpace(warning.Message))
		text = strings.TrimPrefix(text, ": ")
		if text != "" {
			candidates = append(candidates, text)
		}
	}
	if route := routeFromDiagnostics(payload["diagnostics"]); route != "" {
		candidateCount := candidateCountFromDiagnostics(payload["diagnostics"])
		selectedSource := selectedSourceFromDiagnostics(payload["diagnostics"])
		fact := "Route " + route
		if candidateCount > 0 {
			fact += fmt.Sprintf(" considered %d candidates", candidateCount)
		}
		if selectedSource > 0 {
			fact += fmt.Sprintf(" and selected source %d", selectedSource)
		}
		candidates = append(candidates, fact)
	}
	return candidates
}

func extractFactSentencesForWebQueryLLM(text string) []string {
	normalized := compactWhitespaceForWebQueryLLM(text)
	if normalized == "" {
		return nil
	}
	segments := splitWebQueryFactSegments(normalized)
	facts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if keepWebQueryFactSegment(segment) {
			facts = append(facts, segment)
		}
	}
	if len(facts) == 0 && keepWebQueryFactSegment(normalized) {
		facts = append(facts, normalized)
	}
	return facts
}

func splitWebQueryFactSegments(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	segments := make([]string, 0, 8)
	var current []rune
	runes := []rune(text)
	for i, r := range runes {
		current = append(current, r)
		boundary := false
		switch r {
		case '\n', ';', '。', '！', '？', '!', '?':
			boundary = true
		case '.':
			prevDigit := i > 0 && runes[i-1] >= '0' && runes[i-1] <= '9'
			nextSpace := i+1 >= len(runes) || runes[i+1] == ' ' || runes[i+1] == '\n'
			if !prevDigit && nextSpace {
				boundary = true
			}
		}
		if !boundary {
			continue
		}
		segment := strings.TrimSpace(string(current))
		if segment != "" {
			segments = append(segments, segment)
		}
		current = current[:0]
	}
	if len(current) > 0 {
		segment := strings.TrimSpace(string(current))
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func keepWebQueryFactSegment(segment string) bool {
	segment = compactWhitespaceForWebQueryLLM(segment)
	if segment == "" {
		return false
	}
	runes := utf8.RuneCountInString(segment)
	if runes < 8 {
		return false
	}
	score := 0
	if containsDigitForWebQueryLLM(segment) {
		score += 2
	}
	if webQueryLLMVersionRE.MatchString(segment) {
		score += 2
	}
	if webQueryLLMDateRE.MatchString(segment) {
		score += 2
	}
	if webQueryLLMMetricRE.MatchString(segment) {
		score++
	}
	return score > 0
}

func dedupeWebQueryFacts(candidates []string) []string {
	if len(candidates) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(candidates))
	facts := make([]string, 0, len(candidates))
	for _, raw := range candidates {
		fact := compactWhitespaceForWebQueryLLM(raw)
		if fact == "" {
			continue
		}
		key := webQueryFactFingerprint(fact)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		facts = append(facts, fact)
	}
	return facts
}

func webQueryFactFingerprint(text string) string {
	normalized := strings.ToLower(compactWhitespaceForWebQueryLLM(text))
	if normalized == "" {
		return ""
	}
	normalized = webQueryLLMURLRE.ReplaceAllString(normalized, "")
	normalized = webQueryLLMNonWordRE.ReplaceAllString(normalized, " ")
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

func buildWebQueryLLMRenderConfigs(opts WebQueryLLMCompactionOptions) []webQueryLLMRenderConfig {
	switch opts.Mode {
	case WebQueryLLMCompactionMinimal:
		return []webQueryLLMRenderConfig{
			{
				maxQueryBytes:    120,
				maxTitleBytes:    96,
				maxURLBytes:      180,
				maxSnippetBytes:  96,
				maxFactBytes:     110,
				maxWarningBytes:  110,
				maxResults:       1,
				maxFacts:         firstPositive(opts.FactLimit, 4),
				maxWarnings:      firstPositive(opts.WarningLimit, 1),
				includeResults:   false,
				includeWarnings:  true,
				includeQuery:     true,
				includeInput:     true,
				includeProvider:  true,
				includeRoute:     true,
				includeError:     true,
				includeSnippet:   true,
				includeFactList:  true,
				includeWarnList:  true,
				includeSelected:  true,
				includeMaterial:  true,
				includeSearchTag: opts.OmittedFromLLM,
			},
		}
	case WebQueryLLMCompactionSummary:
		return []webQueryLLMRenderConfig{
			{
				maxQueryBytes:    192,
				maxTitleBytes:    140,
				maxURLBytes:      240,
				maxSnippetBytes:  150,
				maxFactBytes:     180,
				maxWarningBytes:  140,
				maxResults:       firstPositive(opts.ResultLimit, 2),
				maxFacts:         firstPositive(opts.FactLimit, 8),
				maxWarnings:      firstPositive(opts.WarningLimit, 2),
				includeResults:   true,
				includeWarnings:  true,
				includeQuery:     true,
				includeInput:     true,
				includeProvider:  true,
				includeRoute:     true,
				includeError:     true,
				includeSnippet:   true,
				includeFactList:  true,
				includeWarnList:  true,
				includeSelected:  true,
				includeMaterial:  true,
				includeSearchTag: opts.OmittedFromLLM,
			},
			{
				maxQueryBytes:    156,
				maxTitleBytes:    120,
				maxURLBytes:      200,
				maxSnippetBytes:  110,
				maxFactBytes:     140,
				maxWarningBytes:  120,
				maxResults:       1,
				maxFacts:         6,
				maxWarnings:      1,
				includeResults:   true,
				includeWarnings:  true,
				includeQuery:     true,
				includeInput:     false,
				includeProvider:  true,
				includeRoute:     true,
				includeError:     true,
				includeSnippet:   true,
				includeFactList:  true,
				includeWarnList:  true,
				includeSelected:  true,
				includeMaterial:  true,
				includeSearchTag: opts.OmittedFromLLM,
			},
		}
	default:
		return []webQueryLLMRenderConfig{
			{
				maxQueryBytes:    256,
				maxTitleBytes:    180,
				maxURLBytes:      320,
				maxSnippetBytes:  220,
				maxFactBytes:     220,
				maxWarningBytes:  180,
				maxResults:       firstPositive(opts.ResultLimit, 3),
				maxFacts:         firstPositive(opts.FactLimit, 10),
				maxWarnings:      firstPositive(opts.WarningLimit, 3),
				includeResults:   true,
				includeWarnings:  true,
				includeQuery:     true,
				includeInput:     true,
				includeProvider:  true,
				includeRoute:     true,
				includeError:     true,
				includeSnippet:   true,
				includeFactList:  true,
				includeWarnList:  true,
				includeSelected:  true,
				includeMaterial:  true,
				includeSearchTag: opts.OmittedFromLLM,
			},
			{
				maxQueryBytes:    220,
				maxTitleBytes:    156,
				maxURLBytes:      280,
				maxSnippetBytes:  180,
				maxFactBytes:     180,
				maxWarningBytes:  160,
				maxResults:       2,
				maxFacts:         8,
				maxWarnings:      2,
				includeResults:   true,
				includeWarnings:  true,
				includeQuery:     true,
				includeInput:     true,
				includeProvider:  true,
				includeRoute:     true,
				includeError:     true,
				includeSnippet:   true,
				includeFactList:  true,
				includeWarnList:  true,
				includeSelected:  true,
				includeMaterial:  true,
				includeSearchTag: opts.OmittedFromLLM,
			},
			{
				maxQueryBytes:    160,
				maxTitleBytes:    120,
				maxURLBytes:      220,
				maxSnippetBytes:  120,
				maxFactBytes:     150,
				maxWarningBytes:  120,
				maxResults:       1,
				maxFacts:         6,
				maxWarnings:      1,
				includeResults:   false,
				includeWarnings:  true,
				includeQuery:     true,
				includeInput:     false,
				includeProvider:  true,
				includeRoute:     true,
				includeError:     true,
				includeSnippet:   true,
				includeFactList:  true,
				includeWarnList:  true,
				includeSelected:  true,
				includeMaterial:  true,
				includeSearchTag: opts.OmittedFromLLM,
			},
		}
	}
}

func renderWebQueryLLMSummary(data webQueryLLMSummaryData, opts WebQueryLLMCompactionOptions, cfg webQueryLLMRenderConfig) map[string]interface{} {
	out := map[string]interface{}{
		"status":        truncateUTF8BytesForWebQueryLLM(firstNonEmpty(data.Status, webQueryStatusOK), 48),
		"mode":          truncateUTF8BytesForWebQueryLLM(data.Mode, 64),
		"total_count":   data.TotalCount,
		"has_results":   data.HasResults,
		"llm_compacted": true,
	}
	if cfg.includeQuery {
		if query := truncateUTF8BytesForWebQueryLLM(data.Query, cfg.maxQueryBytes); query != "" {
			out["query"] = query
		}
	}
	if cfg.includeInput {
		if input := truncateUTF8BytesForWebQueryLLM(data.Input, cfg.maxQueryBytes); input != "" {
			out["input"] = input
		}
	}
	if cfg.includeProvider {
		if provider := truncateUTF8BytesForWebQueryLLM(data.Provider, 96); provider != "" {
			out["provider"] = provider
		}
	}
	if cfg.includeRoute {
		if route := truncateUTF8BytesForWebQueryLLM(data.Route, 96); route != "" {
			out["route"] = route
		}
	}
	if cfg.includeError {
		if errMsg := truncateUTF8BytesForWebQueryLLM(data.Error, 180); errMsg != "" {
			out["error"] = errMsg
		}
	}
	if cfg.includeSelected && data.SelectedResult != nil {
		out["selected_result"] = webQueryResultToSummaryMap(*data.SelectedResult, cfg)
	}
	if data.SelectedSourceRank > 0 {
		out["selected_source_rank"] = data.SelectedSourceRank
	}
	if cfg.includeResults {
		preview := buildWebQueryPreviewResults(data, cfg)
		if len(preview) > 0 {
			out["results"] = preview
			previewCount := len(preview)
			if data.TotalCount > previewCount {
				out["omitted_results"] = data.TotalCount - previewCount
			}
		} else if data.TotalCount > 0 {
			out["omitted_results"] = data.TotalCount
		}
	} else if data.TotalCount > 0 && data.SelectedResult != nil {
		out["omitted_results"] = maxInt(data.TotalCount-1, 0)
	}
	if cfg.includeFactList {
		facts := buildRenderedWebQueryFacts(data.KeyFacts, cfg)
		if len(facts) > 0 {
			out["key_facts"] = facts
		}
	}
	if data.WarningCount > 0 {
		out["warning_count"] = data.WarningCount
	}
	if cfg.includeWarnings && cfg.includeWarnList {
		warnings := buildRenderedWebQueryWarnings(data.Warnings, cfg)
		if len(warnings) > 0 {
			out["warnings"] = warnings
		}
	}
	if cfg.includeMaterial {
		if data.ArtifactPath != "" {
			out["research_artifact_path"] = truncateUTF8BytesForWebQueryLLM(data.ArtifactPath, 240)
		}
		if data.Materialized || data.ArtifactPath != "" {
			out["materialized"] = true
		}
	}
	if data.SearchCardEmitted {
		out["search_card_emitted"] = true
	}
	if cfg.includeSearchTag && opts.OmittedFromLLM {
		out["omitted_from_llm_context"] = true
		out["tool"] = firstNonEmpty(strings.TrimSpace(opts.ToolName), "web_query")
		if reason := firstNonEmpty(strings.TrimSpace(opts.OmittedReason), "additional search outputs compacted to preserve multi-round evidence"); reason != "" {
			out["reason"] = truncateUTF8BytesForWebQueryLLM(reason, 160)
		}
	}
	return out
}

func buildWebQueryPreviewResults(data webQueryLLMSummaryData, cfg webQueryLLMRenderConfig) []map[string]interface{} {
	if cfg.maxResults <= 0 {
		return nil
	}
	preview := make([]map[string]interface{}, 0, cfg.maxResults)
	seen := make(map[string]struct{}, cfg.maxResults+1)
	appendResult := func(result webQueryLLMResult) {
		if len(preview) >= cfg.maxResults {
			return
		}
		key := webQueryResultIdentity(result)
		if key != "" {
			if _, exists := seen[key]; exists {
				return
			}
			seen[key] = struct{}{}
		}
		preview = append(preview, webQueryResultToSummaryMap(result, cfg))
	}
	if data.SelectedResult != nil {
		appendResult(*data.SelectedResult)
	}
	for _, result := range data.Results {
		appendResult(result)
		if len(preview) >= cfg.maxResults {
			break
		}
	}
	return preview
}

func buildRenderedWebQueryFacts(facts []string, cfg webQueryLLMRenderConfig) []string {
	if cfg.maxFacts <= 0 || len(facts) == 0 {
		return nil
	}
	limit := cfg.maxFacts
	if len(facts) < limit {
		limit = len(facts)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		fact := truncateUTF8BytesForWebQueryLLM(facts[i], cfg.maxFactBytes)
		if fact == "" {
			continue
		}
		out = append(out, fact)
	}
	return out
}

func buildRenderedWebQueryWarnings(warnings []webQueryLLMWarning, cfg webQueryLLMRenderConfig) []string {
	if cfg.maxWarnings <= 0 || len(warnings) == 0 {
		return nil
	}
	limit := cfg.maxWarnings
	if len(warnings) < limit {
		limit = len(warnings)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		text := strings.TrimSpace(strings.TrimSpace(warnings[i].Code) + ": " + strings.TrimSpace(warnings[i].Message))
		text = strings.TrimPrefix(text, ": ")
		if text == "" {
			continue
		}
		out = append(out, truncateUTF8BytesForWebQueryLLM(text, cfg.maxWarningBytes))
	}
	return out
}

func webQueryResultToSummaryMap(result webQueryLLMResult, cfg webQueryLLMRenderConfig) map[string]interface{} {
	out := map[string]interface{}{}
	if result.Rank > 0 {
		out["rank"] = result.Rank
	}
	if title := truncateUTF8BytesForWebQueryLLM(firstNonEmpty(result.Title, result.URL, result.FinalURL), cfg.maxTitleBytes); title != "" {
		out["title"] = title
	}
	urlValue := firstNonEmpty(result.FinalURL, result.URL)
	if trimmedURL := truncateUTF8BytesForWebQueryLLM(urlValue, cfg.maxURLBytes); trimmedURL != "" {
		out["url"] = trimmedURL
	}
	if cfg.includeSnippet {
		if snippet := truncateUTF8BytesForWebQueryLLM(result.Snippet, cfg.maxSnippetBytes); snippet != "" {
			out["description"] = snippet
		}
	}
	if source := truncateUTF8BytesForWebQueryLLM(result.Source, 80); source != "" {
		out["source"] = source
	}
	if result.Selected {
		out["selected"] = true
	}
	return out
}

func webQueryResultIdentity(result webQueryLLMResult) string {
	target := strings.ToLower(strings.TrimSpace(firstNonEmpty(result.FinalURL, result.URL)))
	if target != "" {
		return target
	}
	return strings.ToLower(strings.TrimSpace(result.Title + "|" + result.Snippet))
}

func shouldMaterializeWebQuerySummary(ctx context.Context, data webQueryLLMSummaryData, opts WebQueryLLMCompactionOptions) bool {
	if ctx == nil || !opts.Materialize || !data.HasResults {
		return false
	}
	if strings.TrimSpace(data.ArtifactPath) != "" {
		return false
	}
	conversationID := firstNonEmpty(strings.TrimSpace(opts.ConversationID), strings.TrimSpace(GetSessionID(ctx)))
	toolCallID := strings.TrimSpace(opts.ToolCallID)
	if conversationID == "" || toolCallID == "" {
		return false
	}
	if opts.ForceMaterialization {
		return true
	}
	if len(data.OriginalAuditContent) > 8*1024 {
		return true
	}
	if opts.SearchRound >= 4 {
		return true
	}
	return data.AlreadyCompacted
}

func materializeWebQuerySummaryArtifact(ctx context.Context, data webQueryLLMSummaryData, opts WebQueryLLMCompactionOptions) (string, bool) {
	conversationID := safeWebQueryPathSegment(firstNonEmpty(strings.TrimSpace(opts.ConversationID), strings.TrimSpace(GetSessionID(ctx))))
	toolCallID := safeWebQueryPathSegment(strings.TrimSpace(opts.ToolCallID))
	if conversationID == "" || toolCallID == "" {
		return "", false
	}
	artifactPath := filepath.ToSlash(filepath.Join(".research", "web-query", conversationID, toolCallID+".md"))
	body, err := buildWebQueryArtifactMarkdown(data)
	if err != nil {
		return "", false
	}
	writtenPath, err := WriteBinaryArtifact(ctx, artifactPath, body)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(writtenPath), true
}

func buildWebQueryArtifactMarkdown(data webQueryLLMSummaryData) ([]byte, error) {
	metadata := map[string]interface{}{
		"status":               data.Status,
		"mode":                 data.Mode,
		"provider":             data.Provider,
		"query":                data.Query,
		"input":                data.Input,
		"route":                data.Route,
		"total_count":          data.TotalCount,
		"has_results":          data.HasResults,
		"selected_source_rank": data.SelectedSourceRank,
	}
	selected := map[string]interface{}{}
	if data.SelectedResult != nil {
		selected = webQueryResultToSummaryMap(*data.SelectedResult, webQueryLLMRenderConfig{
			maxTitleBytes:   220,
			maxURLBytes:     380,
			maxSnippetBytes: 280,
			includeSnippet:  true,
		})
	}
	results := buildWebQueryPreviewResults(data, webQueryLLMRenderConfig{
		maxTitleBytes:   220,
		maxURLBytes:     380,
		maxSnippetBytes: 240,
		maxResults:      minInt(maxInt(len(data.Results), 4), 8),
		includeSnippet:  true,
	})
	keyFacts := data.KeyFacts
	if len(keyFacts) > 12 {
		keyFacts = keyFacts[:12]
	}

	var buf bytes.Buffer
	buf.WriteString("# Web Query Research Artifact\n\n")
	buf.WriteString("## Query Metadata\n\n```json\n")
	buf.Write(mustMarshalIndentForWebQueryLLM(metadata))
	buf.WriteString("\n```\n\n")
	buf.WriteString("## Selected Result\n\n```json\n")
	buf.Write(mustMarshalIndentForWebQueryLLM(selected))
	buf.WriteString("\n```\n\n")
	buf.WriteString("## Key Facts\n")
	if len(keyFacts) == 0 {
		buf.WriteString("\n- (none)\n")
	} else {
		buf.WriteByte('\n')
		for _, fact := range keyFacts {
			buf.WriteString("- ")
			buf.WriteString(fact)
			buf.WriteByte('\n')
		}
	}
	buf.WriteString("\n## Result Preview\n\n```json\n")
	buf.Write(mustMarshalIndentForWebQueryLLM(results))
	buf.WriteString("\n```\n\n")
	buf.WriteString("## Audit JSON\n\n```json\n")
	if strings.TrimSpace(data.OriginalAuditContent) != "" {
		buf.WriteString(strings.TrimSpace(data.OriginalAuditContent))
	} else {
		buf.WriteString("{}")
	}
	buf.WriteString("\n```\n")
	return buf.Bytes(), nil
}

func mustMarshalIndentForWebQueryLLM(v interface{}) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return []byte("{}")
	}
	return data
}

func serializeCompactWebQueryAuditForLLM(payload map[string]interface{}) string {
	if len(payload) == 0 {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func routeFromDiagnostics(raw interface{}) string {
	row, ok := raw.(map[string]interface{})
	if !ok || len(row) == 0 {
		return ""
	}
	return anyToStringForLLM(row["route"])
}

func candidateCountFromDiagnostics(raw interface{}) int {
	row, ok := raw.(map[string]interface{})
	if !ok || len(row) == 0 {
		return 0
	}
	return anyToIntForWebQueryLLM(row["candidate_count"])
}

func selectedSourceFromDiagnostics(raw interface{}) int {
	row, ok := raw.(map[string]interface{})
	if !ok || len(row) == 0 {
		return 0
	}
	return anyToIntForWebQueryLLM(row["selected_source"])
}

func contentFromNestedWebQueryPayload(payload map[string]interface{}) string {
	if page, ok := payload["page"].(map[string]interface{}); ok {
		if content := anyToStringForLLM(page["content"]); content != "" {
			return content
		}
	}
	if transcript, ok := payload["transcript"].(map[string]interface{}); ok {
		if text := anyToStringForLLM(transcript["text"]); text != "" {
			return text
		}
	}
	return ""
}

func anyToIntForWebQueryLLM(v interface{}) int {
	switch typed := v.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		if n, err := json.Number(trimmed).Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}

func anyToBoolForWebQueryLLM(v interface{}) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y":
			return true
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}
	return false
}

func truncateUTF8BytesForWebQueryLLM(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	cut := 0
	for _, r := range s {
		size := utf8.RuneLen(r)
		if size <= 0 {
			size = 1
		}
		if cut+size > maxBytes {
			break
		}
		cut += size
	}
	if cut <= 0 {
		return ""
	}
	return s[:cut]
}

func compactWhitespaceForWebQueryLLM(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	return webQueryLLMWhitespaceRE.ReplaceAllString(trimmed, " ")
}

func containsDigitForWebQueryLLM(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func safeWebQueryPathSegment(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	normalized := webQueryLLMSafeSegmentRE.ReplaceAllString(trimmed, "-")
	normalized = strings.Trim(normalized, "-.")
	if normalized == "" {
		return ""
	}
	return normalized
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func sortWebQueryLLMResultsByRank(results []webQueryLLMResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Rank == results[j].Rank {
			return webQueryResultIdentity(results[i]) < webQueryResultIdentity(results[j])
		}
		if results[i].Rank <= 0 {
			return false
		}
		if results[j].Rank <= 0 {
			return true
		}
		return results[i].Rank < results[j].Rank
	})
}
