package server

import (
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"
)

type unicodeAhoMatcher struct {
	once     sync.Once
	patterns []string
	inner    *textmatch.FoldedAhoMatcher
}

func newUnicodeAhoMatcher(patterns []string) *unicodeAhoMatcher {
	return &unicodeAhoMatcher{patterns: append([]string(nil), patterns...)}
}

func (m *unicodeAhoMatcher) ensureInner() *textmatch.FoldedAhoMatcher {
	if m == nil {
		return nil
	}
	m.once.Do(func() {
		m.inner = textmatch.NewFoldedAhoMatcher(m.patterns)
	})
	return m.inner
}

func (m *unicodeAhoMatcher) ContainsAnyFold(text string) bool {
	inner := m.ensureInner()
	if inner == nil {
		return false
	}
	return inner.ContainsAnyFold(text)
}

func (m *unicodeAhoMatcher) HasAnyPrefixFold(text string) bool {
	inner := m.ensureInner()
	if inner == nil {
		return false
	}
	return inner.HasAnyPrefixFold(text)
}
