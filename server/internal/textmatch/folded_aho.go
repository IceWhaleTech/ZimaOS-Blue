package textmatch

import "strings"

type FoldedMatch struct {
	PatternIndex int
	Start        int
	End          int
}

type foldedACNode struct {
	children map[rune]int
	fail     int
	outputs  []int
	terminal bool
}

type FoldedAhoMatcher struct {
	patterns []string
	nodes    []foldedACNode
}

func NewFoldedAhoMatcher(patterns []string) *FoldedAhoMatcher {
	matcher := &FoldedAhoMatcher{
		patterns: dedupeFoldedPatterns(patterns),
		nodes:    []foldedACNode{{children: make(map[rune]int)}},
	}
	for idx, pattern := range matcher.patterns {
		node := 0
		for _, r := range pattern {
			next, ok := matcher.nodes[node].children[r]
			if !ok {
				next = len(matcher.nodes)
				matcher.nodes = append(matcher.nodes, foldedACNode{children: make(map[rune]int)})
				matcher.nodes[node].children[r] = next
			}
			node = next
		}
		matcher.nodes[node].terminal = true
		matcher.nodes[node].outputs = append(matcher.nodes[node].outputs, idx)
	}
	matcher.buildFailures()
	return matcher
}

func (m *FoldedAhoMatcher) ContainsAnyFold(text string) bool {
	contains := false
	m.ScanFold(text, func(match FoldedMatch) bool {
		contains = true
		return true
	})
	return contains
}

func (m *FoldedAhoMatcher) HasAnyPrefixFold(text string) bool {
	if m == nil || len(m.nodes) == 0 {
		return false
	}
	node := 0
	for _, r := range normalizeFoldedMatcherText(text) {
		next, ok := m.nodes[node].children[r]
		if !ok {
			return false
		}
		node = next
		if m.nodes[node].terminal {
			return true
		}
	}
	return false
}

func (m *FoldedAhoMatcher) FindAllFold(text string) []FoldedMatch {
	matches := make([]FoldedMatch, 0)
	m.ScanFold(text, func(match FoldedMatch) bool {
		matches = append(matches, match)
		return false
	})
	return matches
}

func (m *FoldedAhoMatcher) ScanFold(text string, onMatch func(FoldedMatch) bool) {
	if m == nil || len(m.nodes) == 0 || onMatch == nil {
		return
	}
	textRunes := []rune(normalizeFoldedMatcherText(text))
	if len(textRunes) == 0 {
		return
	}
	node := 0
	for idx, r := range textRunes {
		for node != 0 {
			if next, ok := m.nodes[node].children[r]; ok {
				node = next
				goto advanced
			}
			node = m.nodes[node].fail
		}
		if next, ok := m.nodes[0].children[r]; ok {
			node = next
		} else {
			node = 0
		}

	advanced:
		if len(m.nodes[node].outputs) == 0 {
			continue
		}
		for _, patternIndex := range m.nodes[node].outputs {
			pattern := m.patterns[patternIndex]
			end := idx + 1
			start := end - len([]rune(pattern))
			if start < 0 {
				continue
			}
			if onMatch(FoldedMatch{PatternIndex: patternIndex, Start: start, End: end}) {
				return
			}
		}
	}
}

func (m *FoldedAhoMatcher) buildFailures() {
	if m == nil || len(m.nodes) == 0 {
		return
	}
	queue := make([]int, 0, len(m.nodes))
	for _, child := range m.nodes[0].children {
		m.nodes[child].fail = 0
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		nodeIndex := queue[0]
		queue = queue[1:]

		for r, childIndex := range m.nodes[nodeIndex].children {
			queue = append(queue, childIndex)

			fail := m.nodes[nodeIndex].fail
			for fail != 0 {
				if next, ok := m.nodes[fail].children[r]; ok {
					fail = next
					goto linked
				}
				fail = m.nodes[fail].fail
			}
			if next, ok := m.nodes[0].children[r]; ok && next != childIndex {
				fail = next
			} else {
				fail = 0
			}

		linked:
			m.nodes[childIndex].fail = fail
			if len(m.nodes[fail].outputs) > 0 {
				m.nodes[childIndex].outputs = append(m.nodes[childIndex].outputs, m.nodes[fail].outputs...)
			}
		}
	}
}

func dedupeFoldedPatterns(patterns []string) []string {
	deduped := make([]string, 0, len(patterns))
	seen := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		pattern = normalizeFoldedMatcherText(pattern)
		if pattern == "" {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		deduped = append(deduped, pattern)
	}
	return deduped
}

func normalizeFoldedMatcherText(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}
