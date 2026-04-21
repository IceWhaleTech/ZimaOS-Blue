package uiexec

import (
	"regexp"
	"sort"
	"strings"
)

var tokenRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

func normalizeText(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	tokens := tokenRe.FindAllString(s, -1)
	return strings.Join(tokens, " ")
}

func tokenizeNormalized(s string) []string {
	s = normalizeText(s)
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

func tokenSet(tokens []string) map[string]struct{} {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		if t == "" {
			continue
		}
		out[t] = struct{}{}
	}
	return out
}

func tokenOverlapScore(a string, b string) float64 {
	at := tokenizeNormalized(a)
	bt := tokenizeNormalized(b)
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	as := tokenSet(at)
	bs := tokenSet(bt)
	inter := 0
	for t := range as {
		if _, ok := bs[t]; ok {
			inter++
		}
	}
	union := len(as) + len(bs) - inter
	if union <= 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func normalizeActions(actions []string) []string {
	if len(actions) == 0 {
		return nil
	}
	out := make([]string, 0, len(actions))
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		key := normalizeText(action)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func hasAction(actions []string, required string) bool {
	required = normalizeText(required)
	if required == "" {
		return true
	}
	for _, action := range actions {
		if normalizeText(action) == required {
			return true
		}
	}
	return false
}
