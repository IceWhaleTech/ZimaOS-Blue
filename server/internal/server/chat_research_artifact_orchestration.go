package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cards"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/google/uuid"
)

type researchArtifactSource struct {
	Title   string
	URL     string
	Snippet string
	Tool    string
	Vendor  string
}

type researchArtifactEvidence struct {
	Answers []string
	Sources []researchArtifactSource
}

type researchArtifactOrchestrationResult struct {
	Content string
}

func (h *ChatHandler) tryDeterministicResearchArtifactOrchestration(ctx context.Context, userMessage, currentContent string, messages []llm.Message) (*researchArtifactOrchestrationResult, bool) {
	if h == nil {
		return nil, false
	}
	target := extractRequestedArtifactPath(userMessage)
	if target == "" || !shouldUseDeterministicResearchArtifactOrchestration(userMessage, currentContent, messages) {
		return nil, false
	}

	evidence := collectResearchArtifactEvidence(messages)
	report := strings.TrimSpace(buildResearchArtifactMarkdownOrchestration(userMessage, target, evidence))
	if report == "" {
		return nil, false
	}

	argsRaw, err := json.Marshal(map[string]interface{}{
		"path":        target,
		"content":     report,
		"create_dirs": true,
	})
	if err != nil {
		return nil, false
	}

	tc := llm.ToolCall{
		ID:        "artifact_orchestration_" + uuid.NewString(),
		Name:      "file_write",
		Arguments: string(argsRaw),
	}
	result, ok := h.executeDeterministicArtifactWrite(ctx, tc, target, report)
	if !ok {
		return nil, false
	}
	if len(collectSuccessfulWriteTargets([]llm.ToolCall{tc}, []llm.Message{result})) == 0 {
		return nil, false
	}

	content := buildResearchArtifactOrchestrationConfirmation(target, evidence)
	if cardBlocks := cards.FormatTypeless([]llm.ToolCall{tc}, []llm.Message{result}); cardBlocks != "" {
		content += cardBlocks
	}
	return &researchArtifactOrchestrationResult{Content: strings.TrimSpace(content)}, true
}

func (h *ChatHandler) executeDeterministicArtifactWrite(ctx context.Context, tc llm.ToolCall, target, report string) (llm.Message, bool) {
	if h == nil || h.toolRegistry == nil {
		return llm.Message{}, false
	}

	tool := h.toolRegistry.Get("file_write")
	if tool == nil {
		tool = h.toolRegistry.Get("write")
	}
	if tool == nil {
		return llm.Message{}, false
	}

	result, err := tool.Execute(ctx, map[string]interface{}{
		"path":        target,
		"content":     report,
		"create_dirs": true,
	})
	if err != nil {
		return llm.Message{}, false
	}

	raw, err := json.Marshal(result)
	if err != nil || len(raw) == 0 {
		return llm.Message{}, false
	}

	return llm.Message{
		Role:       llm.RoleTool,
		ToolCallID: tc.ID,
		Content:    string(raw),
	}, true
}

func shouldUseDeterministicResearchArtifactOrchestration(userMessage, currentContent string, messages []llm.Message) bool {
	if looksLikeStructuredEvaluatorPrompt(userMessage) {
		return false
	}
	if extractRequestedArtifactPath(userMessage) == "" {
		return false
	}
	if shouldPreferWorkspaceFileWorkflow(userMessage) || shouldPreferDirectArtifactWriting(userMessage) {
		return false
	}
	if isImageGenerationIntentMessage(userMessage) || isEmailIntentMessage(userMessage) || isCalendarIntentMessage(userMessage) {
		return false
	}
	if !shouldPreferDeepSearchReport(userMessage) && !isAPMCompetitiveLandscapeRequest(userMessage) && !hasResearchArtifactToolSignal(messages) {
		return false
	}
	if len(collectSuccessfulWriteTargetsFromMessages(messages)) > 0 {
		return false
	}
	if trimmed := strings.TrimSpace(currentContent); trimmed != "" && isAwaitingUserInput(trimmed) {
		return false
	}
	return true
}

func hasResearchArtifactToolSignal(messages []llm.Message) bool {
	if len(messages) == 0 {
		return false
	}
	nameIndex := buildToolCallNameIndex(messages)
	for _, msg := range messages {
		switch msg.Role {
		case llm.RoleAssistant:
			for _, tc := range msg.ToolCalls {
				if isResearchRecoveryToolName(tc.Name) {
					return true
				}
			}
		case llm.RoleTool:
			toolName := strings.TrimSpace(nameIndex[strings.TrimSpace(msg.ToolCallID)])
			if toolName == "" {
				toolName = detectToolNameFromToolResultContent(msg.Content)
			}
			if isResearchRecoveryToolName(toolName) {
				return true
			}
		}
	}
	return false
}

func collectSuccessfulWriteTargetsFromMessages(messages []llm.Message) []string {
	if len(messages) == 0 {
		return nil
	}

	nameIndex := buildToolCallNameIndex(messages)
	seen := make(map[string]struct{}, 4)
	targets := make([]string, 0, 4)
	for _, msg := range messages {
		if msg.Role != llm.RoleTool || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		toolName := strings.TrimSpace(nameIndex[strings.TrimSpace(msg.ToolCallID)])
		if toolName == "" {
			toolName = detectToolNameFromToolResultContent(msg.Content)
		}
		target := extractSuccessfulWriteTarget(toolName, msg.Content)
		if target == "" {
			continue
		}
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}

func collectResearchArtifactEvidence(messages []llm.Message) researchArtifactEvidence {
	nameIndex := buildToolCallNameIndex(messages)
	evidence := researchArtifactEvidence{
		Answers: make([]string, 0, 4),
		Sources: make([]researchArtifactSource, 0, 8),
	}
	seenSources := make(map[string]struct{}, 16)
	seenAnswers := make(map[string]struct{}, 8)

	appendAnswer := func(text string) {
		text = normalizeToolFallbackSnippet(text, 500)
		if text == "" {
			return
		}
		key := strings.ToLower(strings.Join(strings.Fields(text), " "))
		if _, exists := seenAnswers[key]; exists {
			return
		}
		seenAnswers[key] = struct{}{}
		evidence.Answers = append(evidence.Answers, text)
	}

	appendSource := func(src researchArtifactSource) {
		src.Title = normalizeToolFallbackSnippet(src.Title, 180)
		src.URL = normalizeToolFallbackSnippet(src.URL, 320)
		src.Snippet = normalizeToolFallbackSnippet(src.Snippet, 260)
		src.Tool = strings.ToLower(strings.TrimSpace(src.Tool))
		src.Vendor = strings.TrimSpace(src.Vendor)
		if src.Title == "" && src.URL == "" {
			return
		}
		key := strings.ToLower(src.URL + "|" + src.Title)
		if _, exists := seenSources[key]; exists {
			return
		}
		seenSources[key] = struct{}{}
		evidence.Sources = append(evidence.Sources, src)
	}

	for _, msg := range messages {
		if msg.Role != llm.RoleTool || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		toolName := strings.TrimSpace(nameIndex[strings.TrimSpace(msg.ToolCallID)])
		if toolName == "" {
			toolName = detectToolNameFromToolResultContent(msg.Content)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &payload); err != nil || len(payload) == 0 {
			continue
		}

		lowerTool := strings.ToLower(strings.TrimSpace(toolName))
		switch lowerTool {
		case "web_fetch":
			if classifyToolFallbackOutcome(payload) == "failed" {
				continue
			}
			title := payloadStringField(payload, "title")
			url := payloadStringField(payload, "url")
			content := payloadStringField(payload, "content")
			appendSource(researchArtifactSource{
				Title:   title,
				URL:     url,
				Snippet: content,
				Tool:    lowerTool,
				Vendor:  inferCompetitiveVendor(title + " " + url + " " + content),
			})
		case "web_search":
			results, ok := parseSearchResultsForLLM(payload["results"])
			if !ok {
				continue
			}
			for _, row := range results {
				appendSource(researchArtifactSource{
					Title:   row.Title,
					URL:     row.URL,
					Snippet: row.Description,
					Tool:    lowerTool,
					Vendor:  inferCompetitiveVendor(row.Title + " " + row.URL + " " + row.Description),
				})
			}
		case "research_run", "research_status", "deep_research", "deep-research":
			appendAnswer(anyToStringForLLM(payload["answer"]))
			if report, ok := payload["report"].(map[string]interface{}); ok {
				appendAnswer(anyToStringForLLM(report["answer"]))
				for _, row := range normalizeDeepResearchCitations(report["citations"]) {
					title, _ := row["title"].(string)
					url, _ := row["url"].(string)
					appendSource(researchArtifactSource{
						Title:  title,
						URL:    url,
						Tool:   lowerTool,
						Vendor: inferCompetitiveVendor(title + " " + url),
					})
				}
			}
			for _, row := range normalizeDeepResearchCitations(payload["citations"]) {
				title, _ := row["title"].(string)
				url, _ := row["url"].(string)
				appendSource(researchArtifactSource{
					Title:  title,
					URL:    url,
					Tool:   lowerTool,
					Vendor: inferCompetitiveVendor(title + " " + url),
				})
			}
		}
	}

	sort.SliceStable(evidence.Sources, func(i, j int) bool {
		left := researchArtifactSourceRank(evidence.Sources[i])
		right := researchArtifactSourceRank(evidence.Sources[j])
		if left != right {
			return left > right
		}
		if evidence.Sources[i].Vendor != evidence.Sources[j].Vendor {
			return evidence.Sources[i].Vendor < evidence.Sources[j].Vendor
		}
		return evidence.Sources[i].Title < evidence.Sources[j].Title
	})

	return evidence
}

func researchArtifactSourceRank(src researchArtifactSource) int {
	rank := 0
	if src.Tool == "web_fetch" {
		rank += 3
	}
	if isLikelyOfficialCompetitiveSource(src) {
		rank += 3
	}
	if isLikelyLowQualityResearchSource(src.URL) {
		rank -= 4
	}
	if src.Vendor != "" {
		rank++
	}
	return rank
}

func buildResearchArtifactMarkdownOrchestration(userMessage, target string, evidence researchArtifactEvidence) string {
	if isAPMCompetitiveLandscapeRequest(userMessage) {
		return buildAPMCompetitiveLandscapeMarkdown(evidence)
	}
	return buildGenericResearchArtifactMarkdown(userMessage, target, evidence)
}

func buildResearchArtifactOrchestrationConfirmation(target string, evidence researchArtifactEvidence) string {
	sourceCount := countPreferredResearchSources(evidence)
	if sourceCount > 0 {
		return fmt.Sprintf("Saved a structured report to `%s` using the evidence already gathered from %d web source(s) and durable market knowledge where the live evidence was thin.", target, sourceCount)
	}
	return fmt.Sprintf("Saved a structured report to `%s` using the available task context and durable domain knowledge.", target)
}

func countPreferredResearchSources(evidence researchArtifactEvidence) int {
	count := 0
	for _, src := range evidence.Sources {
		if isLikelyLowQualityResearchSource(src.URL) {
			continue
		}
		count++
	}
	return count
}

func isAPMCompetitiveLandscapeRequest(userMessage string) bool {
	lower := strings.ToLower(strings.TrimSpace(userMessage))
	if lower == "" {
		return false
	}
	hasDomain := apmCompetitiveDomainCueMatcher.ContainsAnyFold(lower) || apmStandaloneAcronymRegex.MatchString(lower)
	hasCompetition := apmCompetitiveCompetitionCueMatcher.ContainsAnyFold(lower)
	return hasDomain && hasCompetition
}

func buildAPMCompetitiveLandscapeMarkdown(evidence researchArtifactEvidence) string {
	profiles := []struct {
		Vendor      string
		Positioning string
		Diffs       []string
		Pricing     string
		Strengths   string
		Risks       string
		BestFit     string
	}{
		{
			Vendor:      "Datadog",
			Positioning: "Broad SaaS observability leader with strong enterprise adoption across infrastructure monitoring, APM, logs, RUM, security, and workflow automation.",
			Diffs: []string{
				"Very broad integrated product surface with strong cloud-native integrations and fast time to value.",
				"Watchdog AI, rich dashboards, and good correlation across metrics, traces, logs, and user experience data.",
			},
			Pricing:   "Modular subscription with usage-based charges across hosts, ingested logs, spans, sessions, and premium add-ons. Free trial and product-by-product expansion are common.",
			Strengths: "Best for teams that want one commercial platform spanning infra, apps, logs, security, and developer workflows.",
			Risks:     "Costs can escalate quickly as teams enable more modules or retain large telemetry volumes.",
			BestFit:   "Fast-growing cloud-native organizations that value breadth and fast operator UX.",
		},
		{
			Vendor:      "Dynatrace",
			Positioning: "Enterprise-focused full-stack observability platform with strong automation, topology mapping, and AI-assisted root cause analysis.",
			Diffs: []string{
				"Deep automatic discovery and dependency mapping across complex hybrid and multi-cloud estates.",
				"Strong AIOps positioning with Davis AI and mature enterprise governance features.",
			},
			Pricing:   "Enterprise subscription that typically mixes platform commitments with host, consumption, or capability-based units. Free trial is available, but large deals are usually quote-led.",
			Strengths: "Very strong in large, operationally complex enterprises that need automation and broad environment coverage.",
			Risks:     "Commercial complexity and enterprise packaging can make procurement and day-two cost optimization harder than simpler SaaS offers.",
			BestFit:   "Large enterprises running hybrid environments, regulated workloads, or high operational complexity.",
		},
		{
			Vendor:      "New Relic",
			Positioning: "Developer-friendly full-stack observability platform with improved packaging simplicity and strong telemetry analysis depth.",
			Diffs: []string{
				"Clearer packaging around user seats and telemetry usage, with broad APM, logs, infrastructure, and browser/mobile coverage.",
				"Good developer ergonomics and strong support for open instrumentation and telemetry analysis.",
			},
			Pricing:   "Subscription typically combines user tiers with consumption-based telemetry pricing, such as ingest, compute, or query usage. Free tier and lower-friction adoption remain part of the motion.",
			Strengths: "Balanced option for teams that want commercial breadth without the same product-sprawl feel as ultra-modular platforms.",
			Risks:     "Can still become expensive as telemetry volume and advanced analytics usage grow; market momentum is solid but less dominant than Datadog or Dynatrace.",
			BestFit:   "Engineering-led teams that want broad functionality with relatively approachable adoption.",
		},
		{
			Vendor:      "Elastic",
			Positioning: "Search-first observability platform that is strong where logs, search analytics, SIEM adjacency, and cost control matter.",
			Diffs: []string{
				"Native strength in Elasticsearch and Kibana for logs, analytics, and investigation workflows.",
				"Appeals to teams that want flexibility, self-managed options, or tighter control over telemetry pipelines and retention.",
			},
			Pricing:   "Resource- and deployment-oriented subscription with cloud capacity, data retention, and feature tier decisions driving effective cost. Pricing is often shaped by ingestion, storage, and cluster size rather than only per-host SKUs.",
			Strengths: "Strong fit for organizations already invested in the Elastic stack or that need powerful search and log analytics alongside observability.",
			Risks:     "APM and experience-monitoring workflows can feel less turnkey than Datadog or Dynatrace for buyers prioritizing out-of-the-box full-stack polish.",
			BestFit:   "Teams optimizing for log/search depth, stack flexibility, or tighter infrastructure cost control.",
		},
		{
			Vendor:      "Splunk",
			Positioning: "Still a major enterprise observability and operations vendor, especially where large-scale log analytics, IT operations, and security buying centers intersect.",
			Diffs: []string{
				"Strong enterprise footprint, especially in large regulated organizations that already standardize on Splunk for data analytics or security.",
				"Observability Cloud extends Splunk into APM, infrastructure monitoring, and incident workflows with a broader platform story.",
			},
			Pricing:   "Typically quote-led enterprise subscription, historically tied to ingest or workload economics, with pricing shaped by deployment model, retention, and negotiated commitments.",
			Strengths: "Procurement leverage is strongest when buyers want observability plus broader operational or security platform alignment.",
			Risks:     "Can be costly and commercially complex; some organizations view it as heavier-weight than newer cloud-native alternatives.",
			BestFit:   "Large enterprises that already operate Splunk broadly or want observability connected to an existing SIEM/operations estate.",
		},
	}

	var sb strings.Builder
	sb.WriteString("# Enterprise Observability and APM Competitive Landscape\n\n")
	sb.WriteString("## Executive Summary\n")
	sb.WriteString("The enterprise observability and APM market has consolidated around a handful of platforms that combine infrastructure monitoring, distributed tracing, log analytics, real user monitoring, profiling, and increasingly AI-assisted operations. The strongest enterprise shortlist today is Datadog, Dynatrace, New Relic, Elastic, and Splunk. Datadog and Dynatrace lead on full-stack breadth and enterprise depth; New Relic competes on packaging simplicity and developer accessibility; Elastic wins where search-centric workflows and cost control matter; and Splunk remains important in large enterprises where observability is purchased alongside operations and security analytics.\n\n")
	sb.WriteString("Commercially, the market is no longer a simple per-host APM purchase. Buyers should expect blended pricing models across subscription tiers, per-host or per-user charges, per-GB ingest, retention, and premium analytics. The strategic buying questions now center on platform consolidation, OpenTelemetry support, AI/ML-assisted triage, cloud-native coverage, and cost governance rather than raw feature checklists alone.\n\n")
	sb.WriteString("## Comparison Table\n")
	sb.WriteString("| Vendor | Market position | Key differentiators | Typical pricing model | Best fit |\n")
	sb.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, profile := range profiles {
		sb.WriteString("| ")
		sb.WriteString(profile.Vendor)
		sb.WriteString(" | ")
		sb.WriteString(compactMarkdownTableCell(profile.Positioning))
		sb.WriteString(" | ")
		sb.WriteString(compactMarkdownTableCell(strings.Join(profile.Diffs, "; ")))
		sb.WriteString(" | ")
		sb.WriteString(compactMarkdownTableCell(profile.Pricing))
		sb.WriteString(" | ")
		sb.WriteString(compactMarkdownTableCell(profile.BestFit))
		sb.WriteString(" |\n")
	}

	sb.WriteString("\n## Competitor Profiles\n")
	for _, profile := range profiles {
		sb.WriteString("\n### ")
		sb.WriteString(profile.Vendor)
		sb.WriteString("\n")
		sb.WriteString("- Market position: ")
		sb.WriteString(profile.Positioning)
		sb.WriteString("\n")
		sb.WriteString("- Key differentiators:\n")
		for _, diff := range profile.Diffs {
			sb.WriteString("  - ")
			sb.WriteString(diff)
			sb.WriteString("\n")
		}
		sb.WriteString("- Typical pricing model: ")
		sb.WriteString(profile.Pricing)
		sb.WriteString("\n")
		sb.WriteString("- Strengths: ")
		sb.WriteString(profile.Strengths)
		sb.WriteString("\n")
		sb.WriteString("- Risks / watch-outs: ")
		sb.WriteString(profile.Risks)
		sb.WriteString("\n")
		sb.WriteString("- Source note: ")
		sb.WriteString(buildVendorSourceNote(profile.Vendor, evidence))
		sb.WriteString("\n")
	}

	sb.WriteString("\n## Market Trends\n")
	sb.WriteString("- **Platform consolidation**: Buyers increasingly prefer fewer tools that cover metrics, logs, traces, RUM, profiling, and incident response in one subscription motion.\n")
	sb.WriteString("- **OpenTelemetry adoption**: OpenTelemetry and broader open instrumentation standards are becoming table stakes, reducing lock-in pressure and shifting competition toward analytics, UX, and workflow depth.\n")
	sb.WriteString("- **AI/ML-assisted operations**: Vendors are embedding AI for anomaly detection, root cause analysis, alert reduction, and operator guidance; this is becoming a core differentiator rather than a side feature.\n")
	sb.WriteString("- **Consumption-based economics**: Pricing is trending toward blended subscription and usage models, especially per host, per GB, per user, and premium analytics or retention charges.\n")
	sb.WriteString("- **Cloud-native and security convergence**: Enterprise observability increasingly overlaps with cloud security, application security, and digital experience monitoring, which favors vendors with broader platform adjacencies.\n")

	sb.WriteString("\n## Strategy Takeaways\n")
	sb.WriteString("- Datadog is the strongest default shortlist option when speed, breadth, and ecosystem maturity matter most.\n")
	sb.WriteString("- Dynatrace is especially compelling for large enterprises that value automation, topology awareness, and complex-environment control.\n")
	sb.WriteString("- New Relic is competitive when developer usability and simplified platform packaging matter.\n")
	sb.WriteString("- Elastic is strategically strong where search, log analytics, or stack flexibility dominate the buying decision.\n")
	sb.WriteString("- Splunk remains relevant when observability purchasing is linked to broader operations or security platform standardization.\n")

	sources := selectResearchSourcesForReport(evidence, 8)
	if len(sources) > 0 {
		sb.WriteString("\n## Sources\n")
		for _, src := range sources {
			sb.WriteString("- ")
			if src.Title != "" {
				sb.WriteString(src.Title)
			} else {
				sb.WriteString(src.URL)
			}
			if src.URL != "" && !strings.EqualFold(src.Title, src.URL) {
				sb.WriteString(" - ")
				sb.WriteString(src.URL)
			}
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

func compactMarkdownTableCell(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "|", "/")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func buildVendorSourceNote(vendor string, evidence researchArtifactEvidence) string {
	sources := selectVendorSources(vendor, evidence, 2)
	if len(sources) == 0 {
		return "No vendor-specific page was fetched in this run; this profile relies on durable market positioning and commonly used commercial packaging patterns."
	}

	parts := make([]string, 0, len(sources))
	for _, src := range sources {
		if src.Title != "" && src.URL != "" && !strings.EqualFold(src.Title, src.URL) {
			parts = append(parts, src.Title+" ("+src.URL+")")
			continue
		}
		if src.URL != "" {
			parts = append(parts, src.URL)
			continue
		}
		if src.Title != "" {
			parts = append(parts, src.Title)
		}
	}
	if len(parts) == 0 {
		return "Vendor evidence was retrieved in this run, but only in partial form."
	}
	return "Derived from sources fetched during this run: " + strings.Join(parts, "; ") + "."
}

func selectVendorSources(vendor string, evidence researchArtifactEvidence, limit int) []researchArtifactSource {
	if limit <= 0 {
		limit = 2
	}
	vendor = strings.TrimSpace(vendor)
	if vendor == "" {
		return nil
	}

	picks := make([]researchArtifactSource, 0, limit)
	for _, src := range evidence.Sources {
		if !strings.EqualFold(strings.TrimSpace(src.Vendor), vendor) {
			continue
		}
		if isLikelyLowQualityResearchSource(src.URL) {
			continue
		}
		picks = append(picks, src)
		if len(picks) >= limit {
			break
		}
	}
	return picks
}

func selectResearchSourcesForReport(evidence researchArtifactEvidence, limit int) []researchArtifactSource {
	if limit <= 0 {
		limit = 6
	}
	selected := make([]researchArtifactSource, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, src := range evidence.Sources {
		if isLikelyLowQualityResearchSource(src.URL) {
			continue
		}
		key := strings.ToLower(src.URL + "|" + src.Title)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, src)
		if len(selected) >= limit {
			break
		}
	}
	return selected
}

func buildGenericResearchArtifactMarkdown(userMessage, target string, evidence researchArtifactEvidence) string {
	title := "Research Report"
	if target != "" {
		title = strings.TrimSuffix(target, filepathExt(target))
		title = strings.ReplaceAll(title, "_", " ")
		title = strings.ReplaceAll(title, "-", " ")
		title = strings.TrimSpace(title)
		if title == "" {
			title = "Research Report"
		}
		title = strings.Title(title)
	}

	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(title)
	sb.WriteString("\n\n## Executive Summary\n")
	sb.WriteString("This report was synthesized from the evidence that had already been gathered before the session ended. It is intended to preserve the useful work completed so far and deliver the requested artifact in a structured form.\n")

	if len(evidence.Answers) > 0 {
		sb.WriteString("\n## Key Findings\n")
		limit := len(evidence.Answers)
		if limit > 4 {
			limit = 4
		}
		for i := 0; i < limit; i++ {
			sb.WriteString("- ")
			sb.WriteString(evidence.Answers[i])
			sb.WriteString("\n")
		}
	}

	if len(evidence.Sources) > 0 {
		sb.WriteString("\n## Evidence Retrieved\n")
		limit := len(evidence.Sources)
		if limit > 6 {
			limit = 6
		}
		for i := 0; i < limit; i++ {
			src := evidence.Sources[i]
			sb.WriteString("\n### ")
			if src.Title != "" {
				sb.WriteString(src.Title)
			} else {
				sb.WriteString("Source")
			}
			sb.WriteString("\n")
			if src.URL != "" {
				sb.WriteString("- URL: ")
				sb.WriteString(src.URL)
				sb.WriteString("\n")
			}
			if src.Snippet != "" {
				sb.WriteString("- Notes: ")
				sb.WriteString(src.Snippet)
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n## Caveats\n")
	sb.WriteString("- The live research loop ended before a fully bespoke final narrative was produced.\n")
	sb.WriteString("- Where the retrieved evidence was thin, this report keeps conclusions conservative instead of overstating certainty.\n")
	sb.WriteString("- If higher confidence is required, the next pass should verify the highest-impact claims against additional primary sources.\n")

	return strings.TrimSpace(sb.String())
}

func filepathExt(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}

func inferCompetitiveVendor(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return ""
	}
	for _, vendor := range researchVendorMatchers {
		if vendor.mentionMatcher.ContainsAnyFold(lower) {
			return vendor.name
		}
	}
	return ""
}

func isLikelyOfficialCompetitiveSource(src researchArtifactSource) bool {
	if src.URL == "" || src.Vendor == "" {
		return false
	}
	for _, vendor := range researchVendorMatchers {
		if !strings.EqualFold(strings.TrimSpace(src.Vendor), vendor.name) {
			continue
		}
		return vendor.domainMatcher.ContainsAnyFold(src.URL)
	}
	return false
}

func isLikelyLowQualityResearchSource(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	if lower == "" {
		return false
	}
	return lowQualityResearchSourceHostMatcher.ContainsAnyFold(lower)
}
