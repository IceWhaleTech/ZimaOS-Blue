package server

import (
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestShouldUseDeterministicResearchArtifactOrchestration_WhenReportStillMissing(t *testing.T) {
	messages := []llm.Message{
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call-1", Name: "web_fetch"},
			},
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"title":"Datadog Pricing","url":"https://www.datadoghq.com/pricing/","content":"usage-based pricing by host and GB","status":"success"}`,
		},
	}

	if !shouldUseDeterministicResearchArtifactOrchestration(
		"Create a competitive landscape analysis for the enterprise observability and APM market and save it to market_research.md. Use web search if available to gather current information and sources.",
		"Completed tools: web_fetch. Ask me to retry summarizing for a fuller report.",
		messages,
	) {
		t.Fatal("expected deterministic artifact orchestration to trigger when the report is still missing")
	}
}

func TestShouldUseDeterministicResearchArtifactOrchestration_SkipsAfterSuccessfulWrite(t *testing.T) {
	messages := []llm.Message{
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call-1", Name: "write"},
			},
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"market_research.md","success":true,"append":false}`,
		},
	}

	if shouldUseDeterministicResearchArtifactOrchestration(
		"Create a competitive landscape analysis for the enterprise observability and APM market and save it to market_research.md. Use web search if available to gather current information and sources.",
		"Saved the report.",
		messages,
	) {
		t.Fatal("expected deterministic artifact orchestration to skip after a successful write")
	}
}

func TestShouldUseDeterministicResearchArtifactOrchestration_SkipsStructuredEvaluatorPrompt(t *testing.T) {
	messages := []llm.Message{
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call-1", Name: "web_fetch"},
			},
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"title":"Datadog Pricing","url":"https://www.datadoghq.com/pricing/","content":"usage-based pricing by host and GB","status":"success"}`,
		},
	}

	if shouldUseDeterministicResearchArtifactOrchestration(
		structuredEvaluatorPromptFixture(),
		"",
		messages,
	) {
		t.Fatal("expected deterministic artifact orchestration to skip judge prompts")
	}
}

func TestShouldUseDeterministicResearchArtifactOrchestration_SkipsWorkspaceFileTaskEvenWithResearchSignals(t *testing.T) {
	messages := []llm.Message{
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call-1", Name: "web_fetch"},
			},
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"title":"Market News","url":"https://example.com/news","content":"latest updates","status":"success"}`,
		},
	}

	if shouldUseDeterministicResearchArtifactOrchestration(
		"Review all files in the research/ folder and write a daily summary to daily_briefing.md.",
		"",
		messages,
	) {
		t.Fatal("expected deterministic artifact orchestration to skip local workspace synthesis tasks")
	}
}

func TestShouldUseDeterministicResearchArtifactOrchestration_SkipsWorkspacePDFTask(t *testing.T) {
	messages := []llm.Message{
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call-1", Name: "web_fetch"},
			},
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"title":"OpenClaw Notes","url":"https://example.com/openclaw","content":"report notes","status":"success"}`,
		},
	}

	if shouldUseDeterministicResearchArtifactOrchestration(
		"I have a report in openclaw_report.pdf in my workspace. Extract the answers and write them one per line to answer.txt.",
		"",
		messages,
	) {
		t.Fatal("expected deterministic artifact orchestration to skip local PDF extraction tasks")
	}
}

func TestBuildResearchArtifactMarkdownOrchestration_APMCompetitiveLandscapeContainsRequiredSections(t *testing.T) {
	evidence := researchArtifactEvidence{
		Sources: []researchArtifactSource{
			{Title: "Datadog Pricing", URL: "https://www.datadoghq.com/pricing/", Tool: "web_fetch", Vendor: "Datadog"},
			{Title: "Application Observability", URL: "https://www.dynatrace.com/platform/application-observability/", Tool: "web_fetch", Vendor: "Dynatrace"},
			{Title: "New Relic Pricing", URL: "https://newrelic.com/pricing", Tool: "web_fetch", Vendor: "New Relic"},
			{Title: "Elastic Pricing", URL: "https://www.elastic.co/pricing", Tool: "web_fetch", Vendor: "Elastic"},
		},
	}

	report := buildResearchArtifactMarkdownOrchestration(
		"Create a competitive landscape analysis for the enterprise observability and APM market segment. Save it to market_research.md.",
		"market_research.md",
		evidence,
	)

	for _, needle := range []string{
		"# Enterprise Observability and APM Competitive Landscape",
		"## Executive Summary",
		"## Comparison Table",
		"## Competitor Profiles",
		"## Market Trends",
		"## Sources",
		"Datadog",
		"Dynatrace",
		"New Relic",
		"Elastic",
		"Splunk",
		"OpenTelemetry",
		"AI/ML-assisted operations",
		"| Vendor | Market position |",
	} {
		if !strings.Contains(report, needle) {
			t.Fatalf("expected report to contain %q\nreport=%s", needle, report)
		}
	}
}

func TestAPMCompetitiveLandscapeRequest_UsesCompiledCueMatchers(t *testing.T) {
	positive := []string{
		"Create a competitive landscape analysis for the enterprise observability and APM market segment.",
		"Need market research on **APM** competitors.",
	}
	for _, msg := range positive {
		if !isAPMCompetitiveLandscapeRequest(msg) {
			t.Fatalf("expected APM competitive request for %q", msg)
		}
	}

	negative := []string{
		"Summarize the latest observability news.",
		"List APM setup steps for my service.",
	}
	for _, msg := range negative {
		if isAPMCompetitiveLandscapeRequest(msg) {
			t.Fatalf("expected non-competitive request for %q", msg)
		}
	}
}

func TestResearchVendorMatchers_RecognizeMentionsAndOfficialDomains(t *testing.T) {
	cases := []struct {
		text       string
		vendor     string
		official   string
		unofficial string
	}{
		{text: "Datadog pricing and platform overview", vendor: "Datadog", official: "https://www.datadoghq.com/pricing/", unofficial: "https://example.com/datadog-review"},
		{text: "Dynatrace application observability", vendor: "Dynatrace", official: "https://www.dynatrace.com/platform/application-observability/", unofficial: "https://example.com/dynatrace"},
		{text: "New Relic telemetry pricing", vendor: "New Relic", official: "https://newrelic.com/pricing", unofficial: "https://example.com/new-relic"},
		{text: "Elastic observability", vendor: "Elastic", official: "https://www.elastic.co/pricing", unofficial: "https://example.com/elastic"},
		{text: "App Dynamics enterprise monitoring", vendor: "AppDynamics", official: "https://www.cisco.com/site/us/en/products/observability/appdynamics/index.html", unofficial: "https://example.com/appdynamics"},
		{text: "SigNoz self-hosted observability", vendor: "SigNoz", official: "https://signoz.io/pricing", unofficial: "https://example.com/signoz"},
	}
	for _, tc := range cases {
		if got := inferCompetitiveVendor(tc.text); got != tc.vendor {
			t.Fatalf("inferCompetitiveVendor(%q) = %q, want %q", tc.text, got, tc.vendor)
		}
		if !isLikelyOfficialCompetitiveSource(researchArtifactSource{Vendor: tc.vendor, URL: tc.official}) {
			t.Fatalf("expected official source match for %s", tc.vendor)
		}
		if isLikelyOfficialCompetitiveSource(researchArtifactSource{Vendor: tc.vendor, URL: tc.unofficial}) {
			t.Fatalf("expected unofficial source miss for %s", tc.vendor)
		}
	}
}

func TestIsLikelyLowQualityResearchSource_UsesCompiledHostMatcher(t *testing.T) {
	if !isLikelyLowQualityResearchSource("https://www.reddit.com/r/observability/comments/example") {
		t.Fatal("expected reddit source to be treated as low quality")
	}
	if isLikelyLowQualityResearchSource("https://www.datadoghq.com/pricing/") {
		t.Fatal("expected official vendor site not to be treated as low quality")
	}
}
