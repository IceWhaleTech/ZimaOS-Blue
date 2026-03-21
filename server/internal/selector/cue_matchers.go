package selector

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"

type foldedPrefixMatcher struct {
	inner *textmatch.FoldedAhoMatcher
}

func newFoldedPrefixMatcher(terms []string) *foldedPrefixMatcher {
	return &foldedPrefixMatcher{inner: textmatch.NewFoldedAhoMatcher(terms)}
}

func (m *foldedPrefixMatcher) HasAnyPrefix(text string) bool {
	if m == nil || m.inner == nil {
		return false
	}
	return m.inner.HasAnyPrefixFold(text)
}

var (
	questionPrefixMatcher       = newFoldedPrefixMatcher(questionPrefixes)
	howToPrefixMatcher          = newFoldedPrefixMatcher(howToPrefixes)
	smalltalkPrefixMatcher      = newFoldedPrefixMatcher(smalltalkPrefixes)
	usageMetaTermMatcher        = textmatch.NewFoldedTermMatcher(usageMetaTerms)
	metaIntentTermMatcher       = textmatch.NewFoldedTermMatcher(metaIntentTerms)
	urlPresentTermMatcher       = textmatch.NewFoldedTermMatcher([]string{"http://", "https://", "www."})
	workspaceContainerMatcher   = textmatch.NewFoldedTermMatcher(workspaceContainerTerms)
	workspaceFileExtMatcher     = textmatch.NewFoldedTermMatcher(workspaceFileExtTerms)
	workspaceFileContextMatcher = textmatch.NewFoldedTermMatcher(workspaceFileContextTerms)
	liveWebTermMatcher          = textmatch.NewFoldedTermMatcher(liveWebTerms)
	productivityTermMatcher     = textmatch.NewFoldedTermMatcher(productivityTerms)
	uiArtifactTermMatcher       = textmatch.NewFoldedTermMatcher(uiArtifactTerms)
	highRiskTermMatcher         = textmatch.NewFoldedTermMatcher(highRiskTerms)
	operationalTermMatcher      = textmatch.NewFoldedTermMatcher(operationalTerms)
	plainReplyTermMatcher       = textmatch.NewFoldedTermMatcher(plainReplyTerms)
	negationTermMatcher         = textmatch.NewFoldedTermMatcher(negationTerms)
	followupActionTermMatcher   = textmatch.NewFoldedTermMatcher(followupActionTerms)
)
