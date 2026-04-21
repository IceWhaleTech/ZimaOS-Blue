package uiexec

import (
	"sort"
	"strings"
)

func TaskPatternFromTask(task string) string {
	tokens := tokenizeNormalized(task)
	if len(tokens) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(tokens))
	out := make([]string, 0, 6)
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
		if len(out) >= 6 {
			break
		}
	}
	return strings.Join(out, " ")
}

func StatePatternFromState(state State) string {
	entities := make([]string, 0, 5)
	for _, e := range state.VisibleEntities {
		if key := normalizeText(e); key != "" {
			entities = append(entities, key)
		}
		if len(entities) >= 5 {
			break
		}
	}
	sort.Strings(entities)
	return strings.Join([]string{
		normalizeText(state.PageHint),
		normalizeText(state.FocusedRole),
		strings.Join(entities, ","),
	}, "|")
}
