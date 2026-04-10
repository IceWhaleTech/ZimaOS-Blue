package knowledge

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const archivedAnswerPageType = "archived_answer"

type loopContextCandidate struct {
	score   int
	snippet LoopContextSnippet
	body    string
}

func (s *Service) ResolveLoopContext(ctx context.Context, req LoopContextRequest) (*LoopContextResult, error) {
	result := &LoopContextResult{}
	query := loopContextQuery(req)
	if query == "" {
		result.SkipReason = "empty_query"
		return result, nil
	}
	if skipReason := loopContextSkipReason(req); skipReason != "" {
		result.SkipReason = skipReason
		return result, nil
	}
	pages, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		result.SkipReason = "no_compiled_knowledge"
		return result, nil
	}
	answers, err := s.loadAllArchivedAnswers()
	if err != nil {
		return nil, err
	}

	candidates := make([]loopContextCandidate, 0, len(pages)+len(answers))
	candidates = append(candidates, rankLoopContextPages(pages, req)...)
	candidates = append(candidates, rankLoopContextAnswers(answers, req)...)
	if len(candidates) == 0 {
		result.SkipReason = "no_relevant_knowledge"
		return result, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].snippet.Slug < candidates[j].snippet.Slug
		}
		return candidates[i].score > candidates[j].score
	})

	budget := loopContextBudget(req.Stage, req.TokenBudget)
	maxSnippets := loopContextMaxSnippets(req.Stage)
	remaining := budget
	snippets := make([]LoopContextSnippet, 0, maxSnippets)
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(snippets) >= maxSnippets {
			break
		}
		snippet := candidate.snippet
		snippet.Summary = loopContextSnippetSummary(snippet.Summary, candidate.body, remaining, req.Stage)
		if strings.TrimSpace(snippet.Summary) == "" {
			continue
		}
		cost := estimateLoopContextTokens(renderLoopContextLine(snippet))
		if len(snippets) > 0 && cost > remaining {
			continue
		}
		snippets = append(snippets, snippet)
		remaining -= cost
		if remaining <= 0 {
			break
		}
	}
	if len(snippets) == 0 {
		result.SkipReason = "budget_exhausted"
		return result, nil
	}
	result.Snippets = snippets
	result.Context = buildLoopKnowledgeBlock(snippets)
	result.UsedCount = len(snippets)
	result.UsedSlugs = make([]string, 0, len(snippets))
	for _, snippet := range snippets {
		result.UsedSlugs = append(result.UsedSlugs, snippet.Slug)
	}
	return result, nil
}

func loopContextQuery(req LoopContextRequest) string {
	parts := make([]string, 0, 3+len(req.PriorFailureHints))
	if goal := strings.TrimSpace(req.Goal); goal != "" {
		parts = append(parts, goal)
	}
	if step := strings.TrimSpace(req.CurrentStep); step != "" {
		parts = append(parts, step)
	}
	if summary := strings.TrimSpace(req.PlanSummary); summary != "" {
		parts = append(parts, summary)
	}
	for _, hint := range req.PriorFailureHints {
		if trimmed := strings.TrimSpace(hint); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func loopContextSkipReason(req LoopContextRequest) string {
	normalized := strings.ToLower(loopContextQuery(req))
	if normalized == "" {
		return "empty_query"
	}
	if strings.Contains(normalized, "http://") || strings.Contains(normalized, "https://") || strings.Contains(normalized, "www.") {
		return "direct_url"
	}
	webSignals := []string{
		"search the web", "web search", "browse", "browser", "website", "url", "look up online",
		"search docs", "search documentation", "official docs", "official documentation",
		"搜索", "搜尋", "网页", "網頁", "网站", "網站", "浏览器", "瀏覽器", "官网", "官方文档", "官方文件",
	}
	freshPublicSignals := []string{
		"latest", "newest", "recent", "current", "today", "news", "release notes", "documentation", "docs",
		"最新", "当前", "今天", "新闻", "更新", "文档", "文件",
	}
	if containsLoopSignal(normalized, webSignals) && containsLoopSignal(normalized, freshPublicSignals) {
		return "latest/live-web-first task"
	}
	return ""
}

func containsLoopSignal(text string, signals []string) bool {
	for _, signal := range signals {
		if strings.Contains(text, signal) {
			return true
		}
	}
	return false
}

func rankLoopContextPages(pages []pageDocument, req LoopContextRequest) []loopContextCandidate {
	terms := extractKeywords(loopContextQuery(req))
	candidates := make([]loopContextCandidate, 0, len(pages))
	for _, page := range pages {
		if loopContextDuplicatesProjectContext(page) {
			continue
		}
		searchSpace := strings.ToLower(strings.Join([]string{
			page.Summary.Title,
			page.Summary.Summary,
			strings.Join(page.Summary.Keywords, " "),
			page.Summary.DerivedFromQuery,
			page.Content,
		}, " "))
		hits := loopContextTermHits(terms, searchSpace)
		if hits == 0 {
			continue
		}
		score := loopContextPagePriority(page.Summary) + hits*10 + loopContextConfidenceWeight(page.Summary.Confidence)
		riskLabel := ""
		switch page.Summary.Status {
		case KnowledgeStatusConflicted:
			score -= 18
			riskLabel = "conflicted evidence"
		case KnowledgeStatusSuperseded:
			score -= 16
			riskLabel = "superseded evidence"
		}
		if score < 18 {
			continue
		}
		candidates = append(candidates, loopContextCandidate{
			score: score,
			snippet: LoopContextSnippet{
				Slug:       page.Summary.Slug,
				PageType:   page.Summary.PageType,
				Status:     page.Summary.Status,
				Confidence: page.Summary.Confidence,
				Summary:    strings.TrimSpace(page.Summary.Summary),
				SourceRefs: uniqueStrings(page.Summary.SourceRefs),
				RiskLabel:  riskLabel,
			},
			body: page.Content,
		})
	}
	return candidates
}

func rankLoopContextAnswers(answers []KnowledgeArchivedAnswer, req LoopContextRequest) []loopContextCandidate {
	terms := extractKeywords(loopContextQuery(req))
	candidates := make([]loopContextCandidate, 0, len(answers))
	for _, answer := range answers {
		searchSpace := strings.ToLower(strings.Join([]string{answer.Title, answer.Query, answer.Summary}, " "))
		hits := loopContextTermHits(terms, searchSpace)
		if hits == 0 {
			continue
		}
		score := 24 + hits*9 + loopContextConfidenceWeight(KnowledgeConfidenceMedium)
		candidates = append(candidates, loopContextCandidate{
			score: score,
			snippet: LoopContextSnippet{
				Slug:       firstNonEmptyLoopString(answer.PageSlug, slugify(answer.Title)),
				PageType:   archivedAnswerPageType,
				Status:     KnowledgeStatusActive,
				Confidence: KnowledgeConfidenceMedium,
				Summary:    strings.TrimSpace(answer.Summary),
				SourceRefs: uniqueStrings([]string{answer.Path}),
			},
		})
	}
	return candidates
}

func loopContextTermHits(terms []string, searchSpace string) int {
	hits := 0
	for _, term := range terms {
		if strings.Contains(searchSpace, term) {
			hits++
		}
	}
	return hits
}

func loopContextPagePriority(summary KnowledgePageSummary) int {
	switch summary.PageType {
	case PageTypeSynthesis:
		return 40
	case PageTypeDecision:
		return 38
	case PageTypeComparison:
		return 30
	case PageTypeConcept:
		return 28
	case PageTypeEntity:
		return 26
	default:
		return 18
	}
}

func loopContextConfidenceWeight(confidence KnowledgeConfidence) int {
	switch confidence {
	case KnowledgeConfidenceHigh:
		return 6
	case KnowledgeConfidenceMedium:
		return 3
	default:
		return 0
	}
}

func loopContextBudget(stage LoopContextStage, explicit int) int {
	if explicit > 0 {
		return explicit
	}
	switch stage {
	case LoopContextStagePlanning:
		return 220
	case LoopContextStageRecovery, LoopContextStageVerification:
		return 140
	default:
		return 120
	}
}

func loopContextMaxSnippets(stage LoopContextStage) int {
	switch stage {
	case LoopContextStagePlanning:
		return 3
	default:
		return 2
	}
}

func loopContextSnippetSummary(summary, body string, remaining int, stage LoopContextStage) string {
	text := strings.TrimSpace(summary)
	if len([]rune(text)) < 48 {
		excerpt := loopContextBodyExcerpt(body, stage)
		switch {
		case text == "":
			text = excerpt
		case excerpt != "":
			text = text + " Evidence: " + excerpt
		}
	}
	if text == "" {
		return ""
	}
	return truncateLoopContextRunes(text, loopContextSummaryRuneLimit(stage, remaining))
}

func loopContextBodyExcerpt(body string, stage LoopContextStage) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	limit := 180
	if stage == LoopContextStagePlanning {
		limit = 220
	}
	return truncateLoopContextRunes(body, limit)
}

func loopContextSummaryRuneLimit(stage LoopContextStage, remaining int) int {
	limit := 220
	if stage != LoopContextStagePlanning {
		limit = 160
	}
	if remaining > 0 {
		remainingLimit := remaining * 4
		if remainingLimit < limit {
			limit = remainingLimit
		}
	}
	if limit < 80 {
		limit = 80
	}
	return limit
}

func renderLoopContextLine(snippet LoopContextSnippet) string {
	meta := []string{
		fmt.Sprintf("slug=%s", strings.TrimSpace(snippet.Slug)),
		fmt.Sprintf("page_type=%s", strings.TrimSpace(snippet.PageType)),
		fmt.Sprintf("status=%s", strings.TrimSpace(string(snippet.Status))),
		fmt.Sprintf("confidence=%s", strings.TrimSpace(string(snippet.Confidence))),
	}
	if risk := strings.TrimSpace(snippet.RiskLabel); risk != "" {
		meta = append(meta, fmt.Sprintf("risk=%q", risk))
	}
	line := fmt.Sprintf("- [knowledge %s]", strings.Join(meta, " "))
	if len(snippet.SourceRefs) > 0 {
		line += " refs=" + strings.Join(snippet.SourceRefs, ", ")
	}
	if summary := strings.TrimSpace(snippet.Summary); summary != "" {
		line += " " + summary
	}
	return line
}

func buildLoopKnowledgeBlock(snippets []LoopContextSnippet) string {
	var builder strings.Builder
	builder.WriteString("<loop_knowledge>\n")
	for _, snippet := range snippets {
		builder.WriteString(renderLoopContextLine(snippet))
		builder.WriteByte('\n')
	}
	builder.WriteString("</loop_knowledge>")
	return builder.String()
}

func estimateLoopContextTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	tokenEstimate := len([]rune(text)) / 4
	if tokenEstimate < 24 {
		tokenEstimate = 24
	}
	return tokenEstimate
}

func truncateLoopContextRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-3])) + "..."
}

func loopContextDuplicatesProjectContext(page pageDocument) bool {
	identity := strings.ToLower(strings.Join([]string{
		page.Summary.Slug,
		page.Summary.Title,
		strings.Join(page.Summary.SourceRefs, " "),
	}, " "))
	return strings.Contains(identity, "agents.md") || strings.Contains(identity, "memory.md")
}

func (s *Service) loadAllArchivedAnswers() ([]KnowledgeArchivedAnswer, error) {
	entries, err := os.ReadDir(s.answersDir())
	if err != nil {
		if errorsIs(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	answers := make([]KnowledgeArchivedAnswer, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		path := filepath.Join(s.answersDir(), entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		doc, err := parseArchivedAnswerDocument(string(body))
		if err != nil {
			continue
		}
		if doc.Answer.Path == "" {
			doc.Answer.Path = path
		}
		answers = append(answers, doc.Answer)
	}
	sort.SliceStable(answers, func(i, j int) bool {
		return answers[i].GeneratedAt.After(answers[j].GeneratedAt)
	})
	return answers, nil
}

func firstNonEmptyLoopString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
