package routingcue

import (
	"sort"
	"strings"
)

// InferSkill returns a high-confidence canonical skill match for a localized
// query using the curated routing cue catalog.
func InferSkill(query string) (string, bool) {
	ensureLocalizedSkillExamples()
	ensureLocalizedURLBypassExamples()

	query = normalizeCueText(query)
	if query == "" {
		return "", false
	}

	bestSkill := ""
	bestScore := 0
	secondScore := 0

	for _, skill := range inferSkillCandidates() {
		examples := localizedSkillExamples[skill]
		score := scoreLocalizedExamples(query, examples)
		if bypass := localizedURLBypassExamples[skill]; len(bypass) > 0 {
			if bypassScore := scoreLocalizedExamples(query, bypass); bypassScore > score {
				score = bypassScore
			}
		}
		if score > bestScore {
			secondScore = bestScore
			bestScore = score
			bestSkill = skill
			continue
		}
		if score > secondScore {
			secondScore = score
		}
	}

	switch {
	case bestScore >= 80:
		return bestSkill, true
	case bestScore >= 3 && bestScore-secondScore >= 2:
		return bestSkill, true
	default:
		return "", false
	}
}

func inferSkillCandidates() []string {
	ensureLocalizedSkillExamples()
	ensureLocalizedURLBypassExamples()

	seen := make(map[string]struct{}, len(localizedSkillExamples)+len(localizedURLBypassExamples))
	out := make([]string, 0, len(seen))
	for skill := range localizedSkillExamples {
		if _, ok := seen[skill]; ok {
			continue
		}
		seen[skill] = struct{}{}
		out = append(out, skill)
	}
	for skill := range localizedURLBypassExamples {
		if _, ok := seen[skill]; ok {
			continue
		}
		seen[skill] = struct{}{}
		out = append(out, skill)
	}
	sort.Strings(out)
	return out
}

func scoreLocalizedExamples(query string, examples []LocalizedSkillExample) int {
	best := 0
	for _, example := range examples {
		if score := scoreLocalizedExample(query, example); score > best {
			best = score
		}
	}
	return best
}

func scoreLocalizedExample(query string, example LocalizedSkillExample) int {
	exampleQuery := normalizeCueText(example.Query)
	switch {
	case query == exampleQuery:
		return 100
	case exampleQuery != "" && (strings.Contains(query, exampleQuery) || strings.Contains(exampleQuery, query)):
		return 80
	}

	score := 0
	for _, term := range example.Actions {
		if containsCueTerm(query, term) {
			score += 3
		}
	}
	for _, term := range example.Objects {
		if containsCueTerm(query, term) {
			score += 2
		}
	}
	for _, term := range example.Context {
		if containsCueTerm(query, term) {
			score++
		}
	}
	return score
}

func containsCueTerm(query, term string) bool {
	term = normalizeCueText(term)
	return term != "" && strings.Contains(query, term)
}

func normalizeCueText(text string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer(
		"\n", " ",
		"\t", " ",
		",", " ",
		".", " ",
		":", " ",
		";", " ",
		"!", " ",
		"?", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		"\"", " ",
		"'", " ",
		"`", " ",
		"’", " ",
		"“", " ",
		"”", " ",
		"-", " ",
		"_", " ",
		"/", " ",
		"\\", " ",
		"，", " ",
		"。", " ",
		"：", " ",
		"；", " ",
		"！", " ",
		"？", " ",
		"（", " ",
		"）", " ",
		"、", " ",
	)
	normalized = replacer.Replace(normalized)
	return strings.Join(strings.Fields(normalized), " ")
}
