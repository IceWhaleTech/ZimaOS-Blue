package server

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"

type unicodeAhoMatcher struct {
	inner *textmatch.FoldedAhoMatcher
}

func newUnicodeAhoMatcher(patterns []string) *unicodeAhoMatcher {
	return &unicodeAhoMatcher{inner: textmatch.NewFoldedAhoMatcher(patterns)}
}

func (m *unicodeAhoMatcher) ContainsAnyFold(text string) bool {
	if m == nil || m.inner == nil {
		return false
	}
	return m.inner.ContainsAnyFold(text)
}

func (m *unicodeAhoMatcher) HasAnyPrefixFold(text string) bool {
	if m == nil || m.inner == nil {
		return false
	}
	return m.inner.HasAnyPrefixFold(text)
}
