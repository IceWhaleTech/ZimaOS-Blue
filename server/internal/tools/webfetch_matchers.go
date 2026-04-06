package tools

import (
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"
)

type webFetchCueMatcher struct {
	once     sync.Once
	patterns []string
	inner    *textmatch.FoldedAhoMatcher
}

func newWebFetchCueMatcher(patterns []string) *webFetchCueMatcher {
	return &webFetchCueMatcher{patterns: append([]string(nil), patterns...)}
}

func (m *webFetchCueMatcher) ensureInner() *textmatch.FoldedAhoMatcher {
	if m == nil {
		return nil
	}
	m.once.Do(func() {
		m.inner = textmatch.NewFoldedAhoMatcher(m.patterns)
	})
	return m.inner
}

var (
	webFetchLoginURLMatcher = newWebFetchCueMatcher([]string{
		"/login",
		"/signin",
		"authwall",
	})
	webFetchLoginFieldMatcher = newWebFetchCueMatcher([]string{
		"log in",
		"sign in",
		"create account",
		"create an account",
		"forgot password",
		"continue with google",
		"continue with apple",
		"continue with email",
		"password",
		"username",
	})
	webFetchChallengeMatcher = newWebFetchCueMatcher([]string{
		"captcha",
		"verify you are human",
		"verify you’re human",
		"are you a robot",
		"attention required",
		"access denied",
		"unusual traffic",
		"cloudflare",
		"enable javascript and cookies",
		"human verification",
	})
	webFetchHardChallengeMatcher = newWebFetchCueMatcher([]string{
		"turnstile",
		"g-recaptcha",
		"recaptcha",
		"hcaptcha",
		"geetest",
		"gee test",
		"slide to verify",
		"complete the captcha",
		"security check to continue",
	})
)

func containsAnyWebFetchCue(matcher *webFetchCueMatcher, texts ...string) bool {
	inner := matcher.ensureInner()
	if inner == nil {
		return false
	}
	for _, text := range texts {
		if inner.ContainsAnyFold(text) {
			return true
		}
	}
	return false
}

func countDistinctWebFetchCues(matcher *webFetchCueMatcher, texts ...string) int {
	inner := matcher.ensureInner()
	if inner == nil {
		return 0
	}
	seen := make(map[int]struct{}, 8)
	for _, text := range texts {
		inner.ScanFold(text, func(match textmatch.FoldedMatch) bool {
			seen[match.PatternIndex] = struct{}{}
			return false
		})
	}
	return len(seen)
}
