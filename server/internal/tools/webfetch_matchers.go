package tools

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"

var (
	webFetchLoginURLMatcher = textmatch.NewFoldedAhoMatcher([]string{
		"/login",
		"/signin",
		"authwall",
	})
	webFetchLoginFieldMatcher = textmatch.NewFoldedAhoMatcher([]string{
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
	webFetchChallengeMatcher = textmatch.NewFoldedAhoMatcher([]string{
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
	webFetchHardChallengeMatcher = textmatch.NewFoldedAhoMatcher([]string{
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

func containsAnyWebFetchCue(matcher *textmatch.FoldedAhoMatcher, texts ...string) bool {
	if matcher == nil {
		return false
	}
	for _, text := range texts {
		if matcher.ContainsAnyFold(text) {
			return true
		}
	}
	return false
}

func countDistinctWebFetchCues(matcher *textmatch.FoldedAhoMatcher, texts ...string) int {
	if matcher == nil {
		return 0
	}
	seen := make(map[int]struct{}, 8)
	for _, text := range texts {
		matcher.ScanFold(text, func(match textmatch.FoldedMatch) bool {
			seen[match.PatternIndex] = struct{}{}
			return false
		})
	}
	return len(seen)
}
