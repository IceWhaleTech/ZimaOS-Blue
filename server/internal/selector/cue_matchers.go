package selector

import (
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"
)

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

type lazyValue[T any] struct {
	once  sync.Once
	value T
}

func (l *lazyValue[T]) get(build func() T) T {
	l.once.Do(func() {
		l.value = build()
	})
	return l.value
}

type lazyCueMatchers struct {
	questionPrefix       lazyValue[*foldedPrefixMatcher]
	howToPrefix          lazyValue[*foldedPrefixMatcher]
	smalltalkPrefix      lazyValue[*foldedPrefixMatcher]
	usageMeta            lazyValue[*textmatch.FoldedTermMatcher]
	metaIntent           lazyValue[*textmatch.FoldedTermMatcher]
	urlPresent           lazyValue[*textmatch.FoldedTermMatcher]
	workspaceContainerTerms lazyValue[[]string]
	workspaceContainer   lazyValue[*textmatch.FoldedTermMatcher]
	workspaceFileExt     lazyValue[*textmatch.FoldedTermMatcher]
	workspaceFileContextTerms lazyValue[[]string]
	workspaceFileContext lazyValue[*textmatch.FoldedTermMatcher]
	liveWebTerms         lazyValue[[]string]
	liveWeb              lazyValue[*textmatch.FoldedTermMatcher]
	liveWebStrongTerms   lazyValue[[]string]
	liveWebStrong        lazyValue[*textmatch.FoldedTermMatcher]
	productivityTerms    lazyValue[[]string]
	productivity         lazyValue[*textmatch.FoldedTermMatcher]
	uiArtifactTerms      lazyValue[[]string]
	uiArtifact           lazyValue[*textmatch.FoldedTermMatcher]
	highRisk             lazyValue[*textmatch.FoldedTermMatcher]
	operational          lazyValue[*textmatch.FoldedTermMatcher]
	plainReply           lazyValue[*textmatch.FoldedTermMatcher]
	negation             lazyValue[*textmatch.FoldedTermMatcher]
	followupAction       lazyValue[*textmatch.FoldedTermMatcher]
}

func newLazyCueMatchers() *lazyCueMatchers {
	return &lazyCueMatchers{}
}

func (m *lazyCueMatchers) questionPrefixMatcher() *foldedPrefixMatcher {
	return m.questionPrefix.get(func() *foldedPrefixMatcher {
		return newFoldedPrefixMatcher(questionPrefixes)
	})
}

func (m *lazyCueMatchers) howToPrefixMatcher() *foldedPrefixMatcher {
	return m.howToPrefix.get(func() *foldedPrefixMatcher {
		return newFoldedPrefixMatcher(howToPrefixes)
	})
}

func (m *lazyCueMatchers) smalltalkPrefixMatcher() *foldedPrefixMatcher {
	return m.smalltalkPrefix.get(func() *foldedPrefixMatcher {
		return newFoldedPrefixMatcher(smalltalkPrefixes)
	})
}

func (m *lazyCueMatchers) usageMetaTermMatcher() *textmatch.FoldedTermMatcher {
	return m.usageMeta.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(usageMetaTerms)
	})
}

func (m *lazyCueMatchers) metaIntentTermMatcher() *textmatch.FoldedTermMatcher {
	return m.metaIntent.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(metaIntentTerms)
	})
}

func (m *lazyCueMatchers) urlPresentTermMatcher() *textmatch.FoldedTermMatcher {
	return m.urlPresent.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher([]string{"http://", "https://", "www."})
	})
}

func (m *lazyCueMatchers) workspaceContainerMatcher() *textmatch.FoldedTermMatcher {
	return m.workspaceContainer.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(m.workspaceContainerTermList())
	})
}

func (m *lazyCueMatchers) workspaceFileExtMatcher() *textmatch.FoldedTermMatcher {
	return m.workspaceFileExt.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(workspaceFileExtTerms)
	})
}

func (m *lazyCueMatchers) workspaceFileContextMatcher() *textmatch.FoldedTermMatcher {
	return m.workspaceFileContext.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(m.workspaceFileContextTermList())
	})
}

func (m *lazyCueMatchers) liveWebTermMatcher() *textmatch.FoldedTermMatcher {
	return m.liveWeb.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(m.liveWebTermList())
	})
}

func (m *lazyCueMatchers) liveWebStrongTermMatcher() *textmatch.FoldedTermMatcher {
	return m.liveWebStrong.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(m.liveWebStrongTermList())
	})
}

func (m *lazyCueMatchers) productivityTermMatcher() *textmatch.FoldedTermMatcher {
	return m.productivity.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(m.productivityTermList())
	})
}

func (m *lazyCueMatchers) uiArtifactTermMatcher() *textmatch.FoldedTermMatcher {
	return m.uiArtifact.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(m.uiArtifactTermList())
	})
}

func (m *lazyCueMatchers) highRiskTermMatcher() *textmatch.FoldedTermMatcher {
	return m.highRisk.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(highRiskTerms)
	})
}

func (m *lazyCueMatchers) operationalTermMatcher() *textmatch.FoldedTermMatcher {
	return m.operational.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(operationalTerms)
	})
}

func (m *lazyCueMatchers) plainReplyTermMatcher() *textmatch.FoldedTermMatcher {
	return m.plainReply.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(plainReplyTerms)
	})
}

func (m *lazyCueMatchers) negationTermMatcher() *textmatch.FoldedTermMatcher {
	return m.negation.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(negationTerms)
	})
}

func (m *lazyCueMatchers) followupActionTermMatcher() *textmatch.FoldedTermMatcher {
	return m.followupAction.get(func() *textmatch.FoldedTermMatcher {
		return textmatch.NewFoldedTermMatcher(followupActionTerms)
	})
}

func (m *lazyCueMatchers) workspaceContainerTermList() []string {
	return m.workspaceContainerTerms.get(func() []string {
		return appendMergedTerms(workspaceContainerBaseTerms, routingcue.LocalWorkspaceTerms())
	})
}

func (m *lazyCueMatchers) workspaceFileContextTermList() []string {
	return m.workspaceFileContextTerms.get(func() []string {
		return appendMergedTerms(workspaceFileContextBaseTerms, routingcue.LocalWorkspaceTerms())
	})
}

func (m *lazyCueMatchers) liveWebTermList() []string {
	return m.liveWebTerms.get(func() []string {
		return appendMergedTerms(liveWebBaseTerms, routingcue.LiveWebTerms())
	})
}

func (m *lazyCueMatchers) liveWebStrongTermList() []string {
	return m.liveWebStrongTerms.get(func() []string {
		webQueryTerms := routingcue.SkillTerms("web_query")
		return appendMergedTerms(liveWebBaseTerms, append(webQueryTerms.Objects, webQueryTerms.Context...))
	})
}

func (m *lazyCueMatchers) productivityTermList() []string {
	return m.productivityTerms.get(func() []string {
		return appendMergedTerms(productivityBaseTerms, routingcue.ProductivityTerms())
	})
}

func (m *lazyCueMatchers) uiArtifactTermList() []string {
	return m.uiArtifactTerms.get(func() []string {
		return appendMergedTerms(uiArtifactBaseTerms, routingcue.UIArtifactTerms())
	})
}

func appendMergedTerms(base, extra []string) []string {
	out := append([]string(nil), base...)
	out = append(out, extra...)
	return out
}

var staticCueMatchers = newLazyCueMatchers()
