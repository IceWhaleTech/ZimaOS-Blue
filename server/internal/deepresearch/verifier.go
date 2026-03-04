package deepresearch

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const defaultEntityThreshold = 0.72

var (
	reYearToken = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	reWordToken = regexp.MustCompile(`[a-zA-Z0-9\p{Han}]{2,}`)
)

type claimPolarity int

const (
	claimPolarityNeutral claimPolarity = iota
	claimPolarityPositive
	claimPolarityNegative
)

func applyEntityDisambiguation(query string, strict bool, threshold float64, evidence []Evidence, taskByID map[string]Task) ([]Evidence, EntityDisambiguation) {
	if threshold <= 0 {
		threshold = defaultEntityThreshold
	}
	stats := EntityDisambiguation{
		Enabled:   strict,
		Threshold: threshold,
	}
	if len(evidence) == 0 {
		return evidence, stats
	}

	entityTokens := extractEntityTokens(query)
	out := make([]Evidence, 0, len(evidence))
	for _, ev := range evidence {
		negKeywords := []string(nil)
		if task, ok := taskByID[ev.TaskID]; ok && len(task.NegKeywords) > 0 {
			negKeywords = task.NegKeywords
		}
		score := scoreEntityEvidence(entityTokens, ev, negKeywords)
		ev.EntityScore = score
		if score < threshold {
			stats.AmbiguousCount++
			if strict {
				stats.FilteredCount++
				continue
			}
		}
		out = append(out, ev)
	}
	return out, stats
}

func extractEntityTokens(query string) []string {
	raw := strings.TrimSpace(query)
	if raw == "" {
		return nil
	}
	found := reWordToken.FindAllString(strings.ToLower(raw), -1)
	seen := make(map[string]struct{}, len(found))
	out := make([]string, 0, len(found))
	for _, token := range found {
		token = strings.TrimSpace(token)
		if len(token) < 2 {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i]) > len(out[j])
	})
	return out
}

func scoreEntityEvidence(entityTokens []string, ev Evidence, negKeywords []string) float64 {
	if len(entityTokens) == 0 {
		return 0.5
	}
	text := strings.ToLower(strings.TrimSpace(strings.Join([]string{ev.Title, ev.Snippet, ev.Author, ev.URL, ev.Query}, " ")))
	if text == "" {
		return 0.0
	}

	score := 0.0
	strongHit := false
	for idx, token := range entityTokens {
		if !strings.Contains(text, token) {
			continue
		}
		if idx == 0 {
			strongHit = true
			score += 0.40
			continue
		}
		score += 0.18
		if score >= 0.88 {
			break
		}
	}
	if strongHit && strings.Contains(text, "合伙人") {
		score += 0.08
	}
	if strongHit && (strings.Contains(text, "ventures") || strings.Contains(text, "创投") || strings.Contains(text, "vc")) {
		score += 0.08
	}
	for _, kw := range negKeywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw == "" {
			continue
		}
		if strings.Contains(text, kw) {
			score -= 0.20
		}
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func buildClaimKey(title, snippet string) string {
	raw := strings.ToLower(strings.TrimSpace(title + " " + snippet))
	if raw == "" {
		return ""
	}
	tokens := reWordToken.FindAllString(raw, -1)
	if len(tokens) == 0 {
		return ""
	}
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "from": {}, "that": {}, "this": {}, "have": {}, "been": {}, "will": {},
		"is": {}, "are": {}, "was": {}, "were": {}, "in": {}, "on": {}, "to": {}, "of": {}, "a": {}, "an": {},
		"的": {}, "了": {}, "和": {}, "在": {}, "是": {}, "与": {}, "及": {}, "并": {}, "中": {}, "对": {},
	}
	seen := make(map[string]struct{}, len(tokens))
	key := make([]string, 0, 5)
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if len(token) < 2 {
			continue
		}
		if _, ok := stop[token]; ok {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		key = append(key, token)
		if len(key) >= 5 {
			break
		}
	}
	if len(key) == 0 {
		return ""
	}
	return strings.Join(key, "|")
}

func classifyClaimPolarity(text string) claimPolarity {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return claimPolarityNeutral
	}
	neg := []string{
		"not", "no ", "deny", "denied", "dispute", "conflict", "uncertain", "unconfirmed", "rumor", "rumour", "false",
		"并非", "不是", "否认", "争议", "矛盾", "未证实", "传闻", "不实",
	}
	for _, marker := range neg {
		if strings.Contains(t, marker) {
			return claimPolarityNegative
		}
	}
	if strings.Contains(t, "confirmed") || strings.Contains(t, "official") || strings.Contains(t, "发布") || strings.Contains(t, "宣布") {
		return claimPolarityPositive
	}
	return claimPolarityNeutral
}

func analyzeEvidenceConsistencyByClaim(evidence []Evidence) (supportCount, conflictCount int, hasConflict bool) {
	type claimState struct {
		pos int
		neg int
	}
	claims := map[string]*claimState{}
	for _, ev := range evidence {
		key := strings.TrimSpace(ev.ClaimKey)
		if key == "" {
			key = buildClaimKey(ev.Title, ev.Snippet)
		}
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(ev.Title))
		}
		if key == "" {
			continue
		}
		st := claims[key]
		if st == nil {
			st = &claimState{}
			claims[key] = st
		}
		switch classifyClaimPolarity(ev.Title + " " + ev.Snippet + " " + ev.Quote) {
		case claimPolarityNegative:
			st.neg++
		default:
			st.pos++
		}
	}
	for _, st := range claims {
		if st.neg > 0 && st.pos > 0 {
			conflictCount++
		}
		if st.pos > 0 {
			supportCount++
		}
	}
	hasConflict = conflictCount > 0
	return supportCount, conflictCount, hasConflict
}

func extractEvidenceYear(ev Evidence) int {
	if ev.PublishedAt != nil {
		y := ev.PublishedAt.Year()
		if y >= 1900 && y <= 2100 {
			return y
		}
	}
	text := ev.Title + " " + ev.Snippet + " " + ev.Quote
	raw := reYearToken.FindString(text)
	if raw == "" {
		return 0
	}
	year := 0
	for _, r := range raw {
		if !unicode.IsDigit(r) {
			continue
		}
		year = year*10 + int(r-'0')
	}
	if year < 1900 || year > 2100 {
		return 0
	}
	return year
}
