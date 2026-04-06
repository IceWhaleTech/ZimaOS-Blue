package deepresearch

import (
	"regexp"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"
)

type foldedCueMatcher struct {
	once  sync.Once
	cues  []string
	inner *textmatch.FoldedAhoMatcher
}

func newFoldedCueMatcher(cues []string) *foldedCueMatcher {
	return &foldedCueMatcher{cues: append([]string(nil), cues...)}
}

func (m *foldedCueMatcher) ensureInner() *textmatch.FoldedAhoMatcher {
	if m == nil {
		return nil
	}
	m.once.Do(func() {
		m.inner = textmatch.NewFoldedAhoMatcher(m.cues)
	})
	return m.inner
}

func (m *foldedCueMatcher) Contains(text string) bool {
	inner := m.ensureInner()
	if inner == nil {
		return false
	}
	return inner.ContainsAnyFold(text)
}

var (
	personTimelinePrimaryCueMatcher = newFoldedCueMatcher([]string{
		"不同时期",
		"逐一调研",
		"观点演变",
		"互联网分享",
		"person timeline",
	})
	personTimelineViewCueMatcher     = newFoldedCueMatcher([]string{"观点"})
	personTimelineArticleCueMatcher  = newFoldedCueMatcher([]string{"文章"})
	personTimelineResearchCueMatcher = newFoldedCueMatcher([]string{"调研"})
	personTimelinePartnerCueMatcher  = newFoldedCueMatcher([]string{"合伙人"})

	knowledgeBaseStyleCueMatcher = newFoldedCueMatcher([]string{
		"knowledge base",
		"knowledge-base",
		"kb-style",
		"build a kb",
		"build a knowledge base",
		"construct a knowledge base",
		"documentation pack",
		"知识库",
		"构建完整知识库",
		"整理成知识库",
		"知识库级",
		"资料库",
	})

	officialLikeEvidenceCueMatcher = newFoldedCueMatcher([]string{
		"official",
		"官方",
		"documentation",
		"文档",
	})

	latestQueryCueMatcher = newFoldedCueMatcher([]string{
		"latest",
		"recent",
		"today",
		"current",
		"最新",
		"近期",
		"最近",
		"今年",
	})

	claimValidationPhraseCueMatcher = newFoldedCueMatcher([]string{
		"是否",
		"是不是",
		"真的假的",
		"confirm",
		"confirmed",
		"verify",
		"verification",
	})
	claimValidationQuestionVerbRegex = regexp.MustCompile(`\b(?:did|does|is)\b`)

	entityPartnerCueMatcher = newFoldedCueMatcher([]string{"合伙人"})
	entityVentureCueMatcher = newFoldedCueMatcher([]string{"ventures", "创投", "vc"})

	claimNegativeCueMatcher = newFoldedCueMatcher([]string{
		"not",
		"deny",
		"denied",
		"dispute",
		"conflict",
		"uncertain",
		"unconfirmed",
		"rumor",
		"rumour",
		"false",
		"并非",
		"不是",
		"否认",
		"争议",
		"矛盾",
		"未证实",
		"传闻",
		"不实",
	})
	claimNegativeNoRegex = regexp.MustCompile(`\bno\b`)

	claimPositiveCueMatcher = newFoldedCueMatcher([]string{
		"confirmed",
		"official",
		"发布",
		"宣布",
	})
)
