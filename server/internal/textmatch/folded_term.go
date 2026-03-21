package textmatch

import (
	"sort"
	"strings"
)

type FoldedTermMatcher struct {
	inner         *FoldedAhoMatcher
	boundaryCheck []bool
}

func NewFoldedTermMatcher(patterns []string) *FoldedTermMatcher {
	inner := NewFoldedAhoMatcher(patterns)
	boundaryCheck := make([]bool, len(inner.patterns))
	for idx, pattern := range inner.patterns {
		boundaryCheck[idx] = foldedPatternNeedsASCIIWordBoundary(pattern)
	}
	return &FoldedTermMatcher{
		inner:         inner,
		boundaryCheck: boundaryCheck,
	}
}

func (m *FoldedTermMatcher) ContainsAnyFold(text string) bool {
	found := false
	m.ScanFold(text, func(FoldedMatch) bool {
		found = true
		return true
	})
	return found
}

func (m *FoldedTermMatcher) ScanFold(text string, onMatch func(FoldedMatch) bool) {
	if m == nil || m.inner == nil || onMatch == nil {
		return
	}
	normalized := normalizeFoldedMatcherText(text)
	if normalized == "" {
		return
	}
	textRunes := []rune(normalized)
	m.inner.ScanFold(normalized, func(match FoldedMatch) bool {
		if !m.acceptsMatch(textRunes, match) {
			return false
		}
		return onMatch(match)
	})
}

func (m *FoldedTermMatcher) FirstMatchFold(text string) (FoldedMatch, bool) {
	first := FoldedMatch{}
	found := false
	m.ScanFold(text, func(match FoldedMatch) bool {
		first = match
		found = true
		return true
	})
	return first, found
}

func (m *FoldedTermMatcher) FindMatchesFold(text string) []string {
	if m == nil || m.inner == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(m.inner.patterns))
	out := make([]string, 0, len(m.inner.patterns))
	m.ScanFold(text, func(match FoldedMatch) bool {
		pattern := m.inner.patterns[match.PatternIndex]
		if _, ok := seen[pattern]; ok {
			return false
		}
		seen[pattern] = struct{}{}
		out = append(out, pattern)
		return false
	})
	sort.Strings(out)
	return out
}

func (m *FoldedTermMatcher) acceptsMatch(textRunes []rune, match FoldedMatch) bool {
	if match.PatternIndex < 0 || match.PatternIndex >= len(m.boundaryCheck) {
		return false
	}
	if !m.boundaryCheck[match.PatternIndex] {
		return true
	}
	beforeOK := match.Start == 0 || !isASCIIWordRune(textRunes[match.Start-1])
	afterOK := match.End >= len(textRunes) || !isASCIIWordRune(textRunes[match.End])
	return beforeOK && afterOK
}

func foldedPatternNeedsASCIIWordBoundary(pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	runes := []rune(pattern)
	first := rune(0)
	last := rune(0)
	for _, r := range runes {
		if r == ' ' {
			continue
		}
		first = r
		break
	}
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			continue
		}
		last = runes[i]
		break
	}
	return isASCIIWordRune(first) && isASCIIWordRune(last)
}

func isASCIIWordRune(r rune) bool {
	return (r >= '0' && r <= '9') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		r == '_'
}
