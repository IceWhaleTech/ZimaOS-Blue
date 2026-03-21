package mediagen

import (
	"strings"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"
)

type cueMatch struct {
	start int
	end   int
}

type keywordCueMatcher interface {
	Contains(text string) bool
	CountDistinct(text string) int
	FindMatches(text string) []cueMatch
}

type keywordCueMatcherFactory func(cues []string) keywordCueMatcher

type scanKeywordCueMatcher struct {
	cues []string
}

func newIRKeywordCueMatcher(cues []string) keywordCueMatcher {
	return newAhoKeywordCueMatcher(cues)
}

func newScanKeywordCueMatcher(cues []string) keywordCueMatcher {
	patterns := dedupeCuePatterns(cues)
	deduped := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		deduped = append(deduped, pattern.cue)
	}
	return &scanKeywordCueMatcher{cues: deduped}
}

func (m *scanKeywordCueMatcher) Contains(text string) bool {
	if m == nil {
		return false
	}
	text = strings.ToLower(text)
	for _, cue := range m.cues {
		if hasCueMatch(text, cue) {
			return true
		}
	}
	return false
}

func (m *scanKeywordCueMatcher) CountDistinct(text string) int {
	if m == nil {
		return 0
	}
	text = strings.ToLower(text)
	hits := 0
	for _, cue := range m.cues {
		if hasCueMatch(text, cue) {
			hits++
		}
	}
	return hits
}

func (m *scanKeywordCueMatcher) FindMatches(text string) []cueMatch {
	if m == nil {
		return nil
	}
	text = strings.ToLower(text)
	textRunes := []rune(text)
	matches := make([]cueMatch, 0)
	for _, cue := range m.cues {
		cueRunes := []rune(cue)
		if len(cueRunes) == 0 || len(cueRunes) > len(textRunes) {
			continue
		}
		requireWordBoundary := len(cue) <= 4 && isASCII(cue)
		for i := 0; i <= len(textRunes)-len(cueRunes); i++ {
			if string(textRunes[i:i+len(cueRunes)]) != cue {
				continue
			}
			if requireWordBoundary && !isRuneWordMatch(textRunes, i, len(cueRunes)) {
				continue
			}
			matches = append(matches, cueMatch{start: i, end: i + len(cueRunes)})
		}
	}
	return matches
}

type compiledLangKeywords struct {
	raw        *langKeywords
	actions    keywordCueMatcher
	imageNouns keywordCueMatcher
	videoNouns keywordCueMatcher
	editVerbs  keywordCueMatcher
	animVerbs  keywordCueMatcher
	metaCues   keywordCueMatcher
	metaStrong keywordCueMatcher
}

func compileLangKeywords(raw map[string]*langKeywords) map[string]*compiledLangKeywords {
	return compileLangKeywordsWithFactory(raw, newIRKeywordCueMatcher)
}

func compileLangKeywordsWithFactory(raw map[string]*langKeywords, factory keywordCueMatcherFactory) map[string]*compiledLangKeywords {
	compiled := make(map[string]*compiledLangKeywords, len(raw))
	for key, kw := range raw {
		compiled[key] = &compiledLangKeywords{
			raw:        kw,
			actions:    factory(kw.actions),
			imageNouns: factory(kw.imageNouns),
			videoNouns: factory(kw.videoNouns),
			editVerbs:  factory(kw.editVerbs),
			animVerbs:  factory(kw.animVerbs),
			metaCues:   factory(kw.metaCues),
			metaStrong: factory(kw.metaStrong),
		}
	}
	return compiled
}

type cuePattern struct {
	cue                 string
	runes               []rune
	requireWordBoundary bool
}

type ahoKeywordCueMatcher struct {
	patterns []cuePattern
	matcher  *textmatch.FoldedAhoMatcher
}

func newAhoKeywordCueMatcher(cues []string) keywordCueMatcher {
	patterns := dedupeCuePatterns(cues)
	return &ahoKeywordCueMatcher{
		patterns: patterns,
		matcher:  textmatch.NewFoldedAhoMatcher(patternCueStrings(patterns)),
	}
}

func (m *ahoKeywordCueMatcher) Contains(text string) bool {
	if m == nil || m.matcher == nil {
		return false
	}
	textRunes := []rune(strings.ToLower(strings.TrimSpace(text)))
	contains := false
	m.matcher.ScanFold(text, func(match textmatch.FoldedMatch) bool {
		pattern := m.patterns[match.PatternIndex]
		if pattern.requireWordBoundary && !isRuneWordMatch(textRunes, match.Start, len(pattern.runes)) {
			return false
		}
		contains = true
		return true
	})
	return contains
}

func (m *ahoKeywordCueMatcher) CountDistinct(text string) int {
	if m == nil || m.matcher == nil {
		return 0
	}
	textRunes := []rune(strings.ToLower(strings.TrimSpace(text)))
	seen := make(map[int]struct{}, len(m.patterns))
	m.matcher.ScanFold(text, func(match textmatch.FoldedMatch) bool {
		pattern := m.patterns[match.PatternIndex]
		if pattern.requireWordBoundary && !isRuneWordMatch(textRunes, match.Start, len(pattern.runes)) {
			return false
		}
		seen[match.PatternIndex] = struct{}{}
		return false
	})
	return len(seen)
}

func (m *ahoKeywordCueMatcher) FindMatches(text string) []cueMatch {
	if m == nil || m.matcher == nil {
		return nil
	}
	textRunes := []rune(strings.ToLower(strings.TrimSpace(text)))
	matches := make([]cueMatch, 0)
	m.matcher.ScanFold(text, func(match textmatch.FoldedMatch) bool {
		pattern := m.patterns[match.PatternIndex]
		if pattern.requireWordBoundary && !isRuneWordMatch(textRunes, match.Start, len(pattern.runes)) {
			return false
		}
		matches = append(matches, cueMatch{start: match.Start, end: match.End})
		return false
	})
	return matches
}

func dedupeCuePatterns(cues []string) []cuePattern {
	patterns := make([]cuePattern, 0, len(cues))
	seen := make(map[string]struct{}, len(cues))
	for _, cue := range cues {
		cue = strings.ToLower(strings.TrimSpace(cue))
		if cue == "" {
			continue
		}
		if _, ok := seen[cue]; ok {
			continue
		}
		seen[cue] = struct{}{}
		runes := []rune(cue)
		patterns = append(patterns, cuePattern{
			cue:                 cue,
			runes:               runes,
			requireWordBoundary: len(cue) <= 4 && isASCII(cue),
		})
	}
	return patterns
}

func patternCueStrings(patterns []cuePattern) []string {
	values := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		values = append(values, pattern.cue)
	}
	return values
}

func hasCueMatch(text, cue string) bool {
	if cue == "" {
		return false
	}
	textRunes := []rune(text)
	cueRunes := []rune(cue)
	if len(cueRunes) == 0 || len(cueRunes) > len(textRunes) {
		return false
	}
	requireWordBoundary := len(cue) <= 4 && isASCII(cue)
	for i := 0; i <= len(textRunes)-len(cueRunes); i++ {
		if string(textRunes[i:i+len(cueRunes)]) != cue {
			continue
		}
		if requireWordBoundary && !isRuneWordMatch(textRunes, i, len(cueRunes)) {
			continue
		}
		return true
	}
	return false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

func isRuneWordMatch(text []rune, start, length int) bool {
	if start > 0 && unicode.IsLetter(text[start-1]) {
		return false
	}
	end := start + length
	if end < len(text) && unicode.IsLetter(text[end]) {
		return false
	}
	return true
}
