package server

import "regexp"

type researchVendorMatcher struct {
	name           string
	mentionMatcher *unicodeAhoMatcher
	domainMatcher  *unicodeAhoMatcher
}

var (
	apmCompetitiveDomainCueMatcher = newUnicodeAhoMatcher([]string{
		"observability",
		"application performance monitoring",
		"enterprise apm",
	})

	apmCompetitiveCompetitionCueMatcher = newUnicodeAhoMatcher([]string{
		"competitive",
		"competitor",
		"landscape",
		"market segment",
		"market research",
	})

	apmStandaloneAcronymRegex = regexp.MustCompile(`(^|[^a-z])apm([^a-z]|$)`)

	lowQualityResearchSourceHostMatcher = newUnicodeAhoMatcher([]string{
		"zhihu.com",
		"baidu.com",
		"reddit.com",
		"quora.com",
		"medium.com",
	})

	researchVendorMatchers = []researchVendorMatcher{
		{
			name:           "Datadog",
			mentionMatcher: newUnicodeAhoMatcher([]string{"datadog"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"datadoghq.com"}),
		},
		{
			name:           "Dynatrace",
			mentionMatcher: newUnicodeAhoMatcher([]string{"dynatrace"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"dynatrace.com"}),
		},
		{
			name:           "New Relic",
			mentionMatcher: newUnicodeAhoMatcher([]string{"new relic", "newrelic"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"newrelic.com"}),
		},
		{
			name:           "Elastic",
			mentionMatcher: newUnicodeAhoMatcher([]string{"elastic"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"elastic.co"}),
		},
		{
			name:           "Splunk",
			mentionMatcher: newUnicodeAhoMatcher([]string{"splunk"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"splunk.com"}),
		},
		{
			name:           "Grafana",
			mentionMatcher: newUnicodeAhoMatcher([]string{"grafana"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"grafana.com"}),
		},
		{
			name:           "AppDynamics",
			mentionMatcher: newUnicodeAhoMatcher([]string{"appdynamics", "app dynamics"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"appdynamics.com", "cisco.com"}),
		},
		{
			name:           "Honeycomb",
			mentionMatcher: newUnicodeAhoMatcher([]string{"honeycomb"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"honeycomb.io"}),
		},
		{
			name:           "SigNoz",
			mentionMatcher: newUnicodeAhoMatcher([]string{"signoz"}),
			domainMatcher:  newUnicodeAhoMatcher([]string{"signoz.io"}),
		},
	}
)
