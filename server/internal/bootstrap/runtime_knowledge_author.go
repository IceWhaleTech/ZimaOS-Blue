package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

type runtimeKnowledgeAuthor struct {
	caller llm.Provider
}

type runtimeKnowledgeAuthorPayload struct {
	Pages        []runtimeKnowledgeAuthorPage `json:"pages"`
	Conflicts    []string                     `json:"conflicts"`
	Gaps         []string                     `json:"gaps"`
	FallbackMode bool                         `json:"fallback_mode"`
}

type runtimeKnowledgeAuthorPage struct {
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	PageType   string   `json:"page_type"`
	Summary    string   `json:"summary"`
	Keywords   []string `json:"keywords"`
	Confidence string   `json:"confidence"`
	Content    string   `json:"content"`
}

func newRuntimeKnowledgeAuthor(caller llm.Provider) knowledge.KnowledgeAuthor {
	if caller == nil {
		return nil
	}
	return &runtimeKnowledgeAuthor{caller: caller}
}

func newRuntimeKnowledgeAuthorCaller(runtimeLLM llm.Provider, smallModelManager *smallmodel.Manager) llm.Provider {
	if runtimeLLM == nil && smallModelManager == nil {
		return nil
	}
	caller := newAuxiliaryLLMCaller()
	if smallModelManager != nil {
		caller.SetSmallModel(smallmodel.NewLlamaCppRuntime(smallModelManager))
	}
	if runtimeLLM != nil {
		caller.SetFallback(runtimeLLM)
	}
	return caller
}

func (a *runtimeKnowledgeAuthor) AuthorDelta(ctx context.Context, req knowledge.KnowledgeAuthorRequest) (*knowledge.KnowledgeAuthorResult, error) {
	if a == nil || a.caller == nil {
		return nil, fmt.Errorf("knowledge author llm unavailable")
	}
	chatReq, err := buildRuntimeKnowledgeAuthorRequest(req)
	if err != nil {
		return nil, err
	}
	resp, err := a.caller.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	return parseRuntimeKnowledgeAuthorPayload(resp, req)
}

func buildRuntimeKnowledgeAuthorRequest(req knowledge.KnowledgeAuthorRequest) (llm.ChatRequest, error) {
	relevantPages := selectRuntimeKnowledgeExistingPages(req.ExistingPages, req.Source)
	existingJSON, err := json.MarshalIndent(relevantPages, "", "  ")
	if err != nil {
		return llm.ChatRequest{}, err
	}
	sourceJSON, err := json.MarshalIndent(req.Source, "", "  ")
	if err != nil {
		return llm.ChatRequest{}, err
	}
	content := strings.TrimSpace(strings.Join([]string{
		"Schema:\n" + strings.TrimSpace(req.Schema),
		"Source Snapshot:\n" + string(sourceJSON),
		"Relevant Existing Pages:\n" + string(existingJSON),
		`Return strict JSON with shape {"pages":[{"title":"","slug":"","page_type":"","summary":"","keywords":[],"confidence":"","content":""}],"conflicts":[],"gaps":[],"fallback_mode":false}.`,
		"Rules:",
		"- Return 1 to 5 grounded pages.",
		"- Every page must be grounded only in the provided source snapshot.",
		"- Use page_type from: source_summary, entity, concept, comparison, synthesis, decision.",
		"- Source summary pages should reuse the source slug when possible.",
		"- Content must start with a markdown H1 heading.",
		"- Do not wrap the JSON in markdown fences.",
	}, "\n\n"))
	return llm.ChatRequest{
		Model:       "auto",
		MaxTokens:   1400,
		Temperature: 0.1,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You author concise grounded knowledge pages and return only valid JSON."},
			{Role: llm.RoleUser, Content: content},
		},
	}, nil
}

func parseRuntimeKnowledgeAuthorPayload(resp *llm.ChatResponse, req knowledge.KnowledgeAuthorRequest) (*knowledge.KnowledgeAuthorResult, error) {
	if resp == nil {
		return nil, fmt.Errorf("knowledge author returned no response")
	}
	raw := extractRuntimeKnowledgeJSON(resp.Message.Content)
	var payload runtimeKnowledgeAuthorPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	pages := normalizeRuntimeKnowledgePages(payload.Pages, req)
	if len(pages) == 0 {
		return nil, fmt.Errorf("knowledge author returned no valid pages")
	}
	return &knowledge.KnowledgeAuthorResult{
		Pages:        pages,
		Conflicts:    trimRuntimeKnowledgeStrings(payload.Conflicts),
		Gaps:         trimRuntimeKnowledgeStrings(payload.Gaps),
		FallbackMode: payload.FallbackMode,
	}, nil
}

func selectRuntimeKnowledgeExistingPages(existing []knowledge.KnowledgePageSummary, source knowledge.SourceSnapshot) []knowledge.KnowledgePageSummary {
	if len(existing) <= 8 {
		return append([]knowledge.KnowledgePageSummary(nil), existing...)
	}
	type scoredPage struct {
		page  knowledge.KnowledgePageSummary
		score int
	}
	sourceTerms := make(map[string]struct{}, len(source.Keywords)+4)
	for _, value := range append(append([]string(nil), source.Keywords...), source.Title, source.Slug, source.Ref) {
		for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		}) {
			if token != "" {
				sourceTerms[token] = struct{}{}
			}
		}
	}
	scored := make([]scoredPage, 0, len(existing))
	for _, page := range existing {
		score := 0
		for _, value := range append(append([]string(nil), page.Keywords...), page.Title, page.Slug) {
			for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsNumber(r)
			}) {
				if _, ok := sourceTerms[token]; ok {
					score++
				}
			}
		}
		scored = append(scored, scoredPage{page: page, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].page.Title < scored[j].page.Title
		}
		return scored[i].score > scored[j].score
	})
	selected := make([]knowledge.KnowledgePageSummary, 0, 8)
	for idx, page := range scored {
		if idx == 8 {
			break
		}
		selected = append(selected, page.page)
	}
	return selected
}

func extractRuntimeKnowledgeJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}

func normalizeRuntimeKnowledgePages(rawPages []runtimeKnowledgeAuthorPage, req knowledge.KnowledgeAuthorRequest) []knowledge.KnowledgePage {
	seen := make(map[string]struct{}, len(rawPages))
	pages := make([]knowledge.KnowledgePage, 0, len(rawPages))
	for _, raw := range rawPages {
		pageType := normalizeRuntimeKnowledgePageType(raw.PageType)
		title := strings.TrimSpace(raw.Title)
		if pageType == knowledge.PageTypeSourceSummary && title == "" {
			title = strings.TrimSpace(req.Source.Title)
		}
		content := normalizeRuntimeKnowledgeContent(title, raw.Content)
		if title == "" || content == "" {
			continue
		}
		slug := normalizeRuntimeKnowledgeSlug(raw.Slug, title, pageType, req.Source.Slug)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		generatedAt := req.GeneratedAt.UTC()
		if generatedAt.IsZero() {
			generatedAt = time.Now().UTC()
		}
		pages = append(pages, knowledge.KnowledgePage{
			KnowledgePageSummary: knowledge.KnowledgePageSummary{
				Title:       title,
				Slug:        slug,
				PageType:    pageType,
				Summary:     normalizeRuntimeKnowledgeSummary(raw.Summary, content),
				SourceRefs:  []string{strings.TrimSpace(req.Source.Ref)},
				Keywords:    normalizeRuntimeKnowledgeKeywords(raw.Keywords, req.Source.Keywords),
				GeneratedAt: generatedAt,
				UpdatedAt:   generatedAt,
				SourceHash:  strings.TrimSpace(req.Source.SourceHash),
				Status:      knowledge.KnowledgeStatusActive,
				Confidence:  normalizeRuntimeKnowledgeConfidence(raw.Confidence),
			},
			Content: content,
		})
	}
	return pages
}

func normalizeRuntimeKnowledgePageType(pageType string) string {
	switch strings.TrimSpace(pageType) {
	case knowledge.PageTypeSourceSummary, knowledge.PageTypeEntity, knowledge.PageTypeConcept, knowledge.PageTypeComparison, knowledge.PageTypeSynthesis, knowledge.PageTypeDecision:
		return strings.TrimSpace(pageType)
	default:
		return knowledge.PageTypeSourceSummary
	}
}

func normalizeRuntimeKnowledgeConfidence(confidence string) knowledge.KnowledgeConfidence {
	switch strings.TrimSpace(strings.ToLower(confidence)) {
	case string(knowledge.KnowledgeConfidenceLow):
		return knowledge.KnowledgeConfidenceLow
	case string(knowledge.KnowledgeConfidenceHigh):
		return knowledge.KnowledgeConfidenceHigh
	default:
		return knowledge.KnowledgeConfidenceMedium
	}
}

func normalizeRuntimeKnowledgeKeywords(keywords []string, fallback []string) []string {
	values := append([]string(nil), keywords...)
	if len(values) == 0 {
		values = append(values, fallback...)
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, keyword := range values {
		keyword = strings.TrimSpace(strings.ToLower(keyword))
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		out = append(out, keyword)
	}
	sort.Strings(out)
	return out
}

func normalizeRuntimeKnowledgeSummary(summary, content string) string {
	summary = strings.TrimSpace(summary)
	if summary != "" {
		return summary
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if line != "" {
			if len(line) > 220 {
				return strings.TrimSpace(line[:220])
			}
			return line
		}
	}
	return ""
}

func normalizeRuntimeKnowledgeContent(title, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if strings.HasPrefix(content, "# ") {
		return content
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return content
	}
	return "# " + title + "\n\n" + content
}

func normalizeRuntimeKnowledgeSlug(slug, title, pageType, sourceSlug string) string {
	slug = strings.TrimSpace(slug)
	if pageType == knowledge.PageTypeSourceSummary && strings.TrimSpace(sourceSlug) != "" {
		return strings.TrimSpace(sourceSlug)
	}
	if slug != "" {
		return runtimeKnowledgeSlugify(slug)
	}
	base := runtimeKnowledgeSlugify(title)
	if base == "" {
		return ""
	}
	switch pageType {
	case knowledge.PageTypeEntity:
		return "entity-" + base
	case knowledge.PageTypeConcept:
		return "concept-" + base
	default:
		return base
	}
}

func runtimeKnowledgeSlugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			builder.WriteRune(r)
			lastDash = false
		case !lastDash:
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func trimRuntimeKnowledgeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
