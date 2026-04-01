package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const verifierEventLimit = 1000

type HarnessVerificationResult struct {
	Passed         bool                     `json:"passed"`
	Retryable      bool                     `json:"retryable"`
	FailureLabel   string                   `json:"failure_label,omitempty"`
	Summary        string                   `json:"summary,omitempty"`
	OutcomeScore   float64                  `json:"outcome_score,omitempty"`
	EvidenceScore  float64                  `json:"evidence_score,omitempty"`
	ExecutionScore float64                  `json:"execution_score,omitempty"`
	Observations   []string                 `json:"observations,omitempty"`
	Checks         []map[string]interface{} `json:"checks,omitempty"`
	Artifacts      []map[string]interface{} `json:"artifacts,omitempty"`
	TraceSummary   map[string]interface{}   `json:"trace_summary,omitempty"`
}

type expectedArtifactContract struct {
	Path      string
	Label     string
	MustExist bool
}

func (c *Controller) verifyGroupRun(ctx context.Context, group *RunGroup, item *RunGroupItem, run *Run) HarnessVerificationResult {
	result := HarnessVerificationResult{
		Passed:         true,
		Retryable:      false,
		Summary:        "verification passed",
		OutcomeScore:   1,
		EvidenceScore:  1,
		ExecutionScore: 1,
	}
	if item == nil {
		result.Passed = false
		result.Retryable = false
		result.FailureLabel = "verification_failed"
		result.Summary = "group item is missing"
		result.OutcomeScore = 0
		result.EvidenceScore = 0
		result.ExecutionScore = 0
		return result
	}
	if run == nil {
		result.Passed = false
		result.Retryable = false
		result.FailureLabel = "run_missing"
		result.Summary = "run is missing"
		result.OutcomeScore = 0
		result.EvidenceScore = 0
		result.ExecutionScore = 0
		return result
	}

	events, eventsErr := c.ListEvents(ctx, run.ID, verifierEventLimit)
	artifacts, artifactsErr := c.ListArtifacts(ctx, run.ID)
	toolNames, eventTypeCounts := summarizeVerifierEvents(events)
	observations := deriveVerificationObservations(run, events, artifacts, toolNames)
	sessionIDs := observedSessionIDs(events)
	if expectedSessionID := strings.TrimSpace(run.SessionID); expectedSessionID != "" {
		if stringSeenFold(sessionIDs, expectedSessionID) {
			observations = appendUniqueObservation(observations, "session_context_propagated")
		} else if len(sessionIDs) > 0 {
			observations = appendUniqueObservation(observations, "session_context_mismatch")
		}
	}
	traceSummary := map[string]interface{}{
		"event_count":       len(events),
		"artifact_count":    len(artifacts),
		"tool_names":        toolNames,
		"event_type_counts": eventTypeCounts,
	}
	if eventsErr != nil {
		traceSummary["events_error"] = strings.TrimSpace(eventsErr.Error())
	}
	if artifactsErr != nil {
		traceSummary["artifacts_error"] = strings.TrimSpace(artifactsErr.Error())
	}
	if len(sessionIDs) > 0 {
		traceSummary["session_ids"] = append([]string(nil), sessionIDs...)
	}
	if len(observations) > 0 {
		traceSummary["observations"] = append([]string(nil), observations...)
	}
	result.Observations = observations
	result.TraceSummary = traceSummary

	checks := make([]map[string]interface{}, 0, 8)
	artifactChecks := make([]map[string]interface{}, 0, 4)
	failureLabel := ""
	failureSummary := ""
	executionPenalty := false
	addCheck := func(name string, expected interface{}, actual interface{}, ok bool, label string, summary string) {
		checks = append(checks, map[string]interface{}{
			"name":     name,
			"expected": expected,
			"actual":   actual,
			"passed":   ok,
		})
		if ok || failureLabel != "" {
			return
		}
		failureLabel = strings.TrimSpace(label)
		failureSummary = strings.TrimSpace(summary)
	}

	runCompleted := run.Status == RunStatusCompleted
	addCheck("run_completed", RunStatusCompleted, run.Status, runCompleted, verificationFailureLabelForRun(run.Status), fmt.Sprintf("run finished with status %s", run.Status))

	contract := DecodeHarnessContract(item.Metadata, item.Expected, run.Metadata)
	requiredTools := dedupeContractStrings(append(append([]string(nil), contract.RequiredToolCalls...), decodeStringSlice(item.Expected["required_tool_calls"])...))
	forbiddenTools := dedupeContractStrings(append(append([]string(nil), contract.ForbiddenToolCalls...), decodeStringSlice(item.Expected["forbidden_tool_calls"])...))
	requiredChecks := dedupeContractStrings(append(append([]string(nil), contract.RequiredChecks...), decodeStringSlice(item.Expected["required_checks"])...))
	requiredCards := dedupeContractStrings(append(expectedStrings(item.Expected, "required_cards"), expectedStrings(item.Metadata, "required_cards")...))
	requiredObservations := dedupeContractStrings(append(append([]string(nil), contract.RequiredObservations...), decodeStringSlice(item.Expected["required_observations"])...))
	forbiddenObservations := dedupeContractStrings(append(append([]string(nil), contract.ForbiddenObservations...), decodeStringSlice(item.Expected["forbidden_observations"])...))
	expectedArtifacts := decodeExpectedArtifactContracts(item.Expected["expected_artifacts"])
	if len(expectedArtifacts) == 0 && len(contract.ExpectedArtifacts) > 0 {
		for _, artifact := range contract.ExpectedArtifacts {
			expectedArtifacts = append(expectedArtifacts, expectedArtifactContract{
				Path:      strings.TrimSpace(artifact.Path),
				Label:     strings.TrimSpace(artifact.Label),
				MustExist: artifact.MustExist,
			})
		}
	}

	for _, contract := range expectedArtifacts {
		ok, actual := verifyExpectedArtifact(contract, run, artifacts)
		artifactChecks = append(artifactChecks, map[string]interface{}{
			"path":       contract.Path,
			"label":      contract.Label,
			"must_exist": contract.MustExist,
			"actual":     actual,
			"passed":     ok,
		})
		target := firstNonEmpty(contract.Path, contract.Label)
		addCheck("expected_artifact", target, actual, ok, "missing_artifact", fmt.Sprintf("expected artifact %q was not produced", target))
	}
	for _, required := range requiredCards {
		ok, actual := verifyRequiredCard(required, run, events, artifacts)
		addCheck(
			"required_card",
			required,
			actual,
			ok,
			"missing_required_card",
			fmt.Sprintf("required card %q was not observed in the structured result", required),
		)
	}

	if shouldVerifyExecutionRoute(item, run) {
		expectedRoute := metadataString(item.Metadata, "primary_route")
		expectedCLIAction := metadataString(item.Metadata, "expected_cli_action")
		ok, actual := verifyExpectedExecutionRoute(expectedRoute, expectedCLIAction, events, toolNames)
		addCheck(
			"primary_route",
			map[string]interface{}{
				"primary_route":       expectedRoute,
				"expected_cli_action": expectedCLIAction,
			},
			actual,
			ok,
			"tool_selection_error",
			fmt.Sprintf("expected primary route %q was not observed in the execution trace", expectedRoute),
		)
	}

	for _, required := range requiredTools {
		ok := toolNameSeen(toolNames, required)
		addCheck("required_tool_call", required, toolNames, ok, "tool_selection_error", fmt.Sprintf("required tool %q was not used", required))
	}
	for _, forbidden := range forbiddenTools {
		ok := !toolNameSeen(toolNames, forbidden)
		addCheck("forbidden_tool_call", forbidden, toolNames, ok, "forbidden_tool_used", fmt.Sprintf("forbidden tool %q was used", forbidden))
		if !ok {
			executionPenalty = true
		}
	}

	corpus := buildVerificationCorpus(run, events, toolNames, artifacts)
	for _, required := range requiredChecks {
		ok := verificationCorpusContains(corpus, required)
		addCheck("required_check", required, corpus["checks"], ok, "required_check_missing", fmt.Sprintf("required check %q was not observed", required))
	}
	for _, required := range requiredObservations {
		ok := observationSeen(observations, required)
		label, summary := requiredObservationFailure(required)
		checks = append(checks, map[string]interface{}{
			"name":     "required_observation",
			"expected": required,
			"actual":   observations,
			"passed":   ok,
		})
		if !ok && shouldPreferObservationFailure(failureLabel, label) {
			failureLabel = strings.TrimSpace(label)
			failureSummary = strings.TrimSpace(summary)
		}
	}
	for _, forbidden := range forbiddenObservations {
		ok := !observationSeen(observations, forbidden)
		addCheck(
			"forbidden_observation",
			forbidden,
			observations,
			ok,
			"verification_failed",
			fmt.Sprintf("forbidden observation %q was observed", strings.TrimSpace(forbidden)),
		)
	}
	for _, browserCheck := range contract.BrowserChecks {
		target := firstNonEmpty(browserCheck.Name, browserCheck.Target, "browser QA")
		requiredObservation := firstNonEmpty(browserCheck.RequiredObservation, "browser_used")
		if requiredObservation != "" {
			ok := observationSeen(observations, requiredObservation)
			label := firstNonEmpty(browserCheck.FailureLabel, "browser_qa_missing")
			addCheck(
				"browser_check",
				target,
				observations,
				ok,
				label,
				fmt.Sprintf("browser QA %q did not produce observation %q", target, requiredObservation),
			)
		}
		requiredArtifact := strings.TrimSpace(browserCheck.RequiredArtifact)
		if browserCheck.RequireScreenshot && requiredArtifact == "" {
			requiredArtifact = "screenshot"
		}
		if requiredArtifact != "" {
			ok, actual := verifyArtifactHint(requiredArtifact, artifacts)
			label := firstNonEmpty(browserCheck.FailureLabel, "ui_regression")
			addCheck(
				"browser_artifact",
				requiredArtifact,
				actual,
				ok,
				label,
				fmt.Sprintf("browser QA %q did not produce artifact %q", target, requiredArtifact),
			)
		}
	}
	for _, apiCheck := range contract.APIChecks {
		expected := firstNonEmpty(apiCheck.RequiredCheck, apiCheck.Expectation, apiCheck.Target, apiCheck.Name)
		if expected == "" {
			continue
		}
		ok := verificationCorpusContains(corpus, expected)
		addCheck(
			"api_check",
			expected,
			corpus["checks"],
			ok,
			firstNonEmpty(apiCheck.FailureLabel, "api_check_missing"),
			fmt.Sprintf("API check %q was not observed", expected),
		)
	}

	passedChecks := 0
	for _, check := range checks {
		if passed, _ := check["passed"].(bool); passed {
			passedChecks++
		}
	}
	if len(checks) > 0 {
		result.EvidenceScore = float64(passedChecks) / float64(len(checks))
	}
	if !runCompleted {
		result.OutcomeScore = 0
		result.ExecutionScore = 0
	} else {
		result.OutcomeScore = 0.7
		result.ExecutionScore = 0.9
	}
	if len(expectedArtifacts) == 0 && len(requiredTools) == 0 && len(requiredChecks) == 0 && len(requiredCards) == 0 && runCompleted {
		result.EvidenceScore = 1
		result.OutcomeScore = 1
	}
	if executionPenalty {
		result.ExecutionScore = 0.1
	}

	result.Checks = checks
	result.Artifacts = artifactChecks
	processLabel, processSummary := processObservationFailureLabel(group, item, run, observations)
	if shouldPreferObservationFailure(failureLabel, processLabel) {
		failureLabel = strings.TrimSpace(processLabel)
		failureSummary = strings.TrimSpace(processSummary)
	}
	result.Passed = failureLabel == ""
	if result.Passed {
		result.Retryable = false
		result.FailureLabel = ""
		result.Summary = "verification passed"
		if runCompleted {
			result.OutcomeScore = 1
			if len(checks) > 0 {
				result.EvidenceScore = 1
			}
			if !executionPenalty {
				result.ExecutionScore = 1
			}
		}
		return result
	}

	result.FailureLabel = firstNonEmpty(failureLabel, "verification_failed")
	result.Retryable = verificationRetryable(item, result.FailureLabel)
	result.Summary = firstNonEmpty(failureSummary, fmt.Sprintf("verification failed: %s", result.FailureLabel))
	return result
}

func summarizeVerifierEvents(events []RunEvent) ([]string, map[string]int) {
	toolSet := make(map[string]struct{})
	counts := make(map[string]int)
	for _, event := range events {
		counts[event.Type]++
		if name := strings.TrimSpace(event.ToolName); name != "" {
			toolSet[name] = struct{}{}
		}
	}
	toolNames := make([]string, 0, len(toolSet))
	for name := range toolSet {
		toolNames = append(toolNames, name)
	}
	return toolNames, counts
}

func deriveVerificationObservations(run *Run, events []RunEvent, artifacts []ArtifactRef, toolNames []string) []string {
	observed := newObservationSet()
	routes := observedExecutionRoutes(events, toolNames)
	evidenceResults := summarizeEvidenceCollectionResults(events)
	if verificationResponderEmpty(run) {
		observed.add("verification_responder_empty")
	}
	if verificationUnknownEvidence(run) {
		observed.add("verification_unknown_evidence")
	}
	if len(artifacts) > 0 {
		observed.add("artifact_emitted")
	}
	if evidenceToolSeen(toolNames) || routeObserved(routes, "web_search") || routeObserved(routes, "web_query") || routeObserved(routes, "browser") {
		observed.add("evidence_tool_used")
	}
	if evidenceResults.sawFinished && !evidenceResults.usable && evidenceResults.empty {
		observed.add("evidence_collection_empty")
	}
	if browserToolSeen(toolNames) || routeObserved(routes, "browser") {
		observed.add("browser_used")
	}
	for route := range routes {
		observed.add(route + "_routed")
	}
	for _, artifact := range artifacts {
		joined := strings.ToLower(strings.TrimSpace(strings.Join([]string{artifact.Kind, artifact.Label, artifact.PathOrURL, artifact.MIMEType}, " ")))
		if strings.Contains(joined, "screenshot") || strings.Contains(joined, ".png") || strings.Contains(joined, ".jpg") {
			observed.add("screenshot_captured")
		}
		if strings.Contains(joined, "dom") || strings.Contains(joined, "html") {
			observed.add("dom_snapshot_captured")
		}
	}
	sawToolError := false
	for _, event := range events {
		switch event.Type {
		case "question_requested":
			observed.add("clarification_requested")
		case "question_resolved":
			if questionResolvedSuccessfully(event) {
				observed.add("clarification_resolved")
			}
		case "approval_requested":
			observed.add("approval_requested")
		case "approval_resolved":
			observed.add("approval_resolved")
		case "tool_finished":
			if toolFinishedErrored(event) {
				observed.add("tool_error_seen")
				sawToolError = true
				continue
			}
			if sawToolError {
				observed.add("tool_error_recovered")
			}
		case "task_planner_memory_skipped":
			observed.add("planner_memory_skipped")
			if strings.EqualFold(strings.TrimSpace(event.Message), "public_web") {
				observed.add("planner_memory_skipped_public_web")
			}
		case "task_planner_memory_filtered_session_compaction":
			observed.add("planner_memory_session_compaction_filtered")
		case "task_planner_memory_filtered_low_score":
			observed.add("planner_memory_low_score_filtered")
		case "task_planner_memory_used":
			observed.add("planner_memory_used")
		}
	}
	switch classifyProviderInfraFailure(run, events) {
	case "auth":
		observed.add("provider_auth_failed")
		observed.add("provider_infra_blocked")
	case "quota":
		observed.add("provider_quota_exhausted")
		observed.add("provider_infra_blocked")
	case "blocked":
		observed.add("provider_infra_blocked")
	}
	return observed.values()
}

func verificationResponderEmpty(run *Run) bool {
	if run == nil {
		return false
	}
	corpus := strings.ToLower(strings.TrimSpace(strings.Join([]string{run.Result, run.Error}, "\n")))
	if corpus == "" {
		return false
	}
	return strings.Contains(corpus, "responder returned empty content")
}

func verificationUnknownEvidence(run *Run) bool {
	if run == nil {
		return false
	}
	result := strings.ToLower(strings.TrimSpace(run.Result))
	if result == "" {
		return false
	}
	if strings.Contains(result, "verification passed on 'unknown' evidence values") {
		return true
	}
	if strings.Contains(result, "criteria results:") && strings.Contains(result, "-- unknown") {
		return true
	}
	return strings.Contains(result, "verified:") && strings.Contains(result, "\n1. unknown")
}

func classifyProviderInfraFailure(run *Run, events []RunEvent) string {
	corpus := strings.ToLower(strings.TrimSpace(providerFailureCorpus(run, events)))
	if corpus == "" {
		return ""
	}
	switch {
	case providerAuthFailureSeen(corpus):
		return "auth"
	case providerQuotaFailureSeen(corpus):
		return "quota"
	case providerInfraFailureSeen(corpus):
		return "blocked"
	default:
		return ""
	}
}

func providerFailureCorpus(run *Run, events []RunEvent) string {
	parts := make([]string, 0, len(events)*4+4)
	if run != nil {
		if value := strings.TrimSpace(run.Error); value != "" {
			parts = append(parts, value)
		}
		if value := strings.TrimSpace(run.Result); value != "" {
			parts = append(parts, value)
		}
	}
	for _, event := range events {
		if value := strings.TrimSpace(event.Type); value != "" {
			parts = append(parts, value)
		}
		if value := strings.TrimSpace(event.ToolName); value != "" {
			parts = append(parts, value)
		}
		if value := strings.TrimSpace(event.Message); value != "" {
			parts = append(parts, value)
		}
		if value := strings.TrimSpace(event.PayloadJSON); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n")
}

func providerAuthFailureSeen(corpus string) bool {
	return containsAny(
		corpus,
		"invalid_api_key",
		"invalid access token",
		"token expired",
		"incorrect api key",
		"api key not valid",
		"authentication_error",
		"authentication error",
		"auth error (401)",
	) || (containsAny(corpus, "unauthorized", "401") && containsAny(corpus, "provider", "proxy returned", "api key", "token", "auth"))
}

func providerQuotaFailureSeen(corpus string) bool {
	return containsAny(
		corpus,
		"insufficient quota",
		"quota exceeded",
		"quota exhausted",
		"out of credit",
		"remaining credit",
		"credit balance",
		"balance is not enough",
		"余额不足",
		"额度不足",
	)
}

func providerInfraFailureSeen(corpus string) bool {
	return containsAny(
		corpus,
		"proxy returned 529",
		"proxy returned 429",
		"proxy returned 500",
		"proxy returned 502",
		"proxy returned 503",
		"proxy returned 504",
		"upstream 503",
		"system_cpu_overloaded",
		"system cpu overloaded",
		"too many requests",
		"rate limit",
		"service unavailable",
		"gateway timeout",
		"bad gateway",
		"upstream connect error",
		"provider unavailable",
		"temporarily unavailable",
		"overloaded",
	)
}

func browserToolSeen(toolNames []string) bool {
	for _, name := range toolNames {
		lower := strings.ToLower(strings.TrimSpace(name))
		switch {
		case strings.Contains(lower, "browser"),
			strings.Contains(lower, "playwright"),
			strings.Contains(lower, "chrome"),
			strings.Contains(lower, "puppeteer"):
			return true
		}
	}
	return false
}

func verifyArtifactHint(expected string, artifacts []ArtifactRef) (bool, string) {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return false, ""
	}
	for _, artifact := range artifacts {
		corpus := strings.ToLower(strings.TrimSpace(strings.Join([]string{artifact.Kind, artifact.Label, artifact.PathOrURL, artifact.MIMEType}, " ")))
		if strings.Contains(corpus, expected) {
			return true, firstNonEmpty(strings.TrimSpace(artifact.PathOrURL), strings.TrimSpace(artifact.Label), strings.TrimSpace(artifact.Kind))
		}
	}
	return false, ""
}

func buildVerificationCorpus(run *Run, events []RunEvent, toolNames []string, artifacts []ArtifactRef) map[string]string {
	parts := make([]string, 0, len(events)+len(toolNames)+len(artifacts)+4)
	checks := make([]string, 0, len(events))
	if run != nil {
		if run.Result != "" {
			parts = append(parts, run.Result)
		}
		if run.Error != "" {
			parts = append(parts, run.Error)
		}
	}
	for _, name := range toolNames {
		parts = append(parts, name)
	}
	for _, event := range events {
		if event.Type != "" {
			parts = append(parts, event.Type)
		}
		if event.Message != "" {
			parts = append(parts, event.Message)
			checks = append(checks, event.Message)
		}
		if event.PayloadJSON != "" {
			parts = append(parts, event.PayloadJSON)
		}
	}
	for _, artifact := range artifacts {
		if artifact.Label != "" {
			parts = append(parts, artifact.Label)
		}
		if artifact.PathOrURL != "" {
			parts = append(parts, artifact.PathOrURL)
		}
		if artifact.MetadataJSON != "" {
			parts = append(parts, artifact.MetadataJSON)
		}
	}
	return map[string]string{
		"all":    strings.Join(parts, "\n"),
		"checks": strings.Join(checks, "\n"),
	}
}

func verificationCorpusContains(corpus map[string]string, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true
	}
	if containsText(corpus["all"], expected) {
		return true
	}
	return false
}

func observationSeen(observations []string, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true
	}
	for _, observation := range observations {
		if strings.EqualFold(strings.TrimSpace(observation), expected) {
			return true
		}
	}
	return false
}

func verificationFailureLabelForRun(status RunStatus) string {
	switch status {
	case RunStatusCompleted:
		return ""
	case RunStatusCancelled:
		return "run_cancelled"
	case RunStatusAborted:
		return "run_aborted"
	case RunStatusFailed:
		return "run_failed"
	default:
		return "run_not_completed"
	}
}

func isRunStatusFailureLabel(label string) bool {
	switch strings.TrimSpace(label) {
	case "run_missing", "run_failed", "run_cancelled", "run_aborted", "run_not_completed":
		return true
	default:
		return false
	}
}

func isObservationFailureLabel(label string) bool {
	switch strings.TrimSpace(label) {
	case "missing_clarification", "question_left_unresolved", "approval_blocked_without_replan", "tool_failed_without_fallback", "missing_evidence_collection", "missing_session_context", "session_context_mismatch", "infra_provider_auth", "infra_provider_quota", "infra_provider_blocked":
		return true
	default:
		return false
	}
}

func isInfraFailureLabel(label string) bool {
	switch strings.TrimSpace(label) {
	case "infra_provider_auth", "infra_provider_quota", "infra_provider_blocked":
		return true
	default:
		return false
	}
}

func shouldPreferObservationFailure(current string, next string) bool {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if next == "" {
		return false
	}
	if isInfraFailureLabel(next) {
		return !isInfraFailureLabel(current)
	}
	if current == "" {
		return true
	}
	if isRunStatusFailureLabel(current) {
		return true
	}
	return current == "verification_failed" && isObservationFailureLabel(next)
}

func verificationRetryable(item *RunGroupItem, failureLabel string) bool {
	failureLabel = strings.TrimSpace(failureLabel)
	if failureLabel == "" {
		return false
	}
	if item != nil {
		if policy, ok := item.Expected["retry_policy"].(map[string]interface{}); ok {
			retryOn := decodeStringSlice(policy["retry_on"])
			if len(retryOn) > 0 {
				for _, candidate := range retryOn {
					if strings.EqualFold(strings.TrimSpace(candidate), failureLabel) {
						return true
					}
				}
				return false
			}
		}
	}
	switch failureLabel {
	case "missing_artifact", "missing_required_card", "required_check_missing", "tool_selection_error", "run_failed", "run_not_completed", "verification_failed", "timeout", "missing_clarification", "question_left_unresolved", "approval_blocked_without_replan", "tool_failed_without_fallback", "missing_evidence_collection", "missing_session_context", "session_context_mismatch", "infra_provider_auth", "infra_provider_quota", "infra_provider_blocked":
		return true
	default:
		return false
	}
}

func decodeExpectedArtifactContracts(raw interface{}) []expectedArtifactContract {
	switch typed := raw.(type) {
	case []interface{}:
		out := make([]expectedArtifactContract, 0, len(typed))
		for _, item := range typed {
			switch value := item.(type) {
			case string:
				if path := strings.TrimSpace(value); path != "" {
					out = append(out, expectedArtifactContract{Path: path, MustExist: true})
				}
			case map[string]interface{}:
				contract := expectedArtifactContract{
					Path:      firstNonEmpty(metadataString(value, "path"), metadataString(value, "path_or_url")),
					Label:     metadataString(value, "label"),
					MustExist: true,
				}
				if mustExist, ok := mapBool(value, "must_exist", "exists"); ok {
					contract.MustExist = mustExist
				}
				if contract.Path != "" || contract.Label != "" {
					out = append(out, contract)
				}
			}
		}
		return out
	case []string:
		out := make([]expectedArtifactContract, 0, len(typed))
		for _, path := range typed {
			if path = strings.TrimSpace(path); path != "" {
				out = append(out, expectedArtifactContract{Path: path, MustExist: true})
			}
		}
		return out
	default:
		return nil
	}
}

func verifyExpectedArtifact(contract expectedArtifactContract, run *Run, artifacts []ArtifactRef) (bool, string) {
	if !contract.MustExist {
		return true, "not required"
	}
	for _, artifact := range artifacts {
		if contract.Label != "" && strings.EqualFold(strings.TrimSpace(artifact.Label), contract.Label) {
			return true, firstNonEmpty(strings.TrimSpace(artifact.PathOrURL), strings.TrimSpace(artifact.Label))
		}
		if contract.Path != "" && strings.EqualFold(strings.TrimSpace(artifact.PathOrURL), contract.Path) {
			return true, strings.TrimSpace(artifact.PathOrURL)
		}
	}
	if run != nil && contract.Path != "" {
		candidate := strings.TrimSpace(contract.Path)
		if candidate != "" {
			if !filepath.IsAbs(candidate) && strings.TrimSpace(run.WorkspaceRoot) != "" {
				candidate = filepath.Join(run.WorkspaceRoot, candidate)
			}
			if stat, err := os.Stat(candidate); err == nil {
				if stat.IsDir() {
					return true, candidate + " (dir)"
				}
				return true, candidate
			}
		}
	}
	return false, "missing"
}

func toolNameSeen(toolNames []string, expected string) bool {
	expected = normalizeText(expected)
	if expected == "" {
		return true
	}
	for _, name := range toolNames {
		normalized := normalizeText(name)
		if normalized == expected || strings.Contains(normalized, expected) || strings.Contains(expected, normalized) {
			return true
		}
	}
	return false
}

func shouldVerifyExecutionRoute(item *RunGroupItem, run *Run) bool {
	if item == nil && run == nil {
		return false
	}
	if run != nil && strings.EqualFold(metadataString(run.Metadata, "gate_type"), "execution_equivalence") {
		return true
	}
	return strings.TrimSpace(metadataString(item.Metadata, "execution_case_type")) != ""
}

func verifyExpectedExecutionRoute(primaryRoute string, expectedCLIAction string, events []RunEvent, toolNames []string) (bool, map[string]interface{}) {
	primaryRoute = normalizeExecutionRoute(primaryRoute)
	expectedCLIAction = strings.TrimSpace(expectedCLIAction)
	execCommands := observedExecCommands(events)
	actual := map[string]interface{}{
		"tool_names":      append([]string(nil), toolNames...),
		"exec_commands":   execCommands,
		"observed_routes": observedExecutionRouteList(events, toolNames),
	}
	if primaryRoute == "" && expectedCLIAction == "" {
		return true, actual
	}
	if primaryRoute != "" && directExecutionRouteSeen(toolNames, primaryRoute) {
		actual["matched_via"] = "direct_tool"
		return true, actual
	}
	for _, command := range execCommands {
		if executionCommandMatchesExpectedRoute(command, expectedCLIAction, primaryRoute) {
			actual["matched_via"] = "exec_command"
			actual["matched_command"] = command
			return true, actual
		}
	}
	return false, actual
}

func observedExecutionRoutes(events []RunEvent, toolNames []string) map[string]struct{} {
	routes := make(map[string]struct{}, len(toolNames)+2)
	for _, name := range toolNames {
		if route := normalizeExecutionRoute(name); route != "" && route != "exec" && route != "bash" {
			routes[route] = struct{}{}
		}
	}
	for _, command := range observedExecCommands(events) {
		if route := executionRouteFromCommand(command); route != "" {
			routes[route] = struct{}{}
		}
	}
	return routes
}

func observedExecutionRouteList(events []RunEvent, toolNames []string) []string {
	routes := observedExecutionRoutes(events, toolNames)
	if len(routes) == 0 {
		return nil
	}
	out := make([]string, 0, len(routes))
	for route := range routes {
		out = append(out, route)
	}
	return out
}

func routeObserved(routes map[string]struct{}, route string) bool {
	if len(routes) == 0 {
		return false
	}
	_, ok := routes[normalizeExecutionRoute(route)]
	return ok
}

func directExecutionRouteSeen(toolNames []string, route string) bool {
	route = normalizeExecutionRoute(route)
	if route == "" {
		return false
	}
	for _, name := range toolNames {
		if normalizeExecutionRoute(name) == route {
			return true
		}
	}
	return false
}

func observedExecCommands(events []RunEvent) []string {
	commands := make([]string, 0, len(events))
	seen := make(map[string]struct{}, len(events))
	for _, event := range events {
		payload := eventPayload(event)
		if payload == nil {
			continue
		}
		toolName := firstNonEmpty(strings.TrimSpace(event.ToolName), metadataString(payload, "tool_name"))
		if normalizeExecutionRoute(toolName) != "exec" && normalizeExecutionRoute(toolName) != "bash" {
			continue
		}
		arguments, _ := payload["arguments"].(map[string]interface{})
		command := strings.TrimSpace(firstNonEmpty(metadataString(arguments, "command"), metadataString(payload, "command")))
		if command == "" {
			continue
		}
		if _, ok := seen[command]; ok {
			continue
		}
		seen[command] = struct{}{}
		commands = append(commands, command)
	}
	return commands
}

func executionRouteFromCommand(command string) string {
	command = normalizeCLICommand(command)
	if !strings.HasPrefix(command, "blue ") {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(command, "blue "))
	if rest == "" {
		return ""
	}
	token := rest
	if idx := strings.IndexByte(token, ' '); idx >= 0 {
		token = token[:idx]
	}
	if dot := strings.IndexByte(token, '.'); dot >= 0 {
		token = token[:dot]
	}
	switch token {
	case "", "help", "version":
		return ""
	default:
		return normalizeExecutionRoute(token)
	}
}

func executionCommandMatchesExpectedRoute(command string, expectedCLIAction string, primaryRoute string) bool {
	command = normalizeCLICommand(command)
	if command == "" {
		return false
	}
	for _, prefix := range expectedExecutionCommandPrefixes(expectedCLIAction, primaryRoute) {
		if command == prefix || strings.HasPrefix(command, prefix+" ") {
			return true
		}
	}
	return false
}

func expectedExecutionCommandPrefixes(expectedCLIAction string, primaryRoute string) []string {
	set := make(map[string]struct{}, 4)
	add := func(value string) {
		value = normalizeCLICommand(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	add(expectedCLIAction)
	if normalizedRoute := normalizeExecutionRoute(primaryRoute); normalizedRoute != "" {
		add("blue " + normalizedRoute)
		add("blue " + normalizedRoute + ".")
	}
	if action := normalizeCLICommand(expectedCLIAction); strings.Contains(action, ".") {
		add(strings.Replace(action, ".", " ", 1))
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	return out
}

func normalizeCLICommand(command string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(command))), " ")
}

func normalizeExecutionRoute(route string) string {
	route = strings.ToLower(strings.TrimSpace(route))
	if route == "" {
		return ""
	}
	return strings.ReplaceAll(route, "-", "_")
}

func verificationPayload(result *HarnessVerificationResult) map[string]interface{} {
	if result == nil {
		return nil
	}
	payload := map[string]interface{}{
		"passed":          result.Passed,
		"retryable":       result.Retryable,
		"failure_label":   strings.TrimSpace(result.FailureLabel),
		"summary":         strings.TrimSpace(result.Summary),
		"outcome_score":   normalizeScore(result.OutcomeScore),
		"evidence_score":  normalizeScore(result.EvidenceScore),
		"execution_score": normalizeScore(result.ExecutionScore),
	}
	if len(result.Checks) > 0 {
		payload["checks"] = result.Checks
	}
	if len(result.Observations) > 0 {
		payload["observations"] = append([]string(nil), result.Observations...)
	}
	if len(result.Artifacts) > 0 {
		payload["artifacts"] = result.Artifacts
	}
	if len(result.TraceSummary) > 0 {
		payload["trace_summary"] = result.TraceSummary
	}
	return payload
}

type observationSet struct {
	order []string
	seen  map[string]struct{}
}

func newObservationSet() *observationSet {
	return &observationSet{seen: make(map[string]struct{}, 8)}
}

func (s *observationSet) add(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if _, ok := s.seen[value]; ok {
		return
	}
	s.seen[value] = struct{}{}
	s.order = append(s.order, value)
}

func (s *observationSet) values() []string {
	if s == nil || len(s.order) == 0 {
		return nil
	}
	return append([]string(nil), s.order...)
}

func requiredObservationFailure(observation string) (string, string) {
	switch strings.TrimSpace(observation) {
	case "clarification_requested":
		return "missing_clarification", "required clarification was not requested"
	case "evidence_tool_used":
		return "missing_evidence_collection", "required evidence collection was not observed"
	case "planner_memory_skipped":
		return "planner_memory_not_skipped", "planner memory should have been skipped for this case"
	case "planner_memory_used":
		return "planner_memory_not_used", "planner memory should have been used for this case"
	case "planner_memory_session_compaction_filtered":
		return "planner_memory_filter_missing", "session-compaction planner memory should have been filtered for this case"
	case "session_context_propagated":
		return "missing_session_context", "required session context was not propagated"
	default:
		normalized := strings.TrimSpace(observation)
		return "verification_failed", fmt.Sprintf("required observation %q was not observed", normalized)
	}
}

func processObservationFailureLabel(group *RunGroup, item *RunGroupItem, run *Run, observations []string) (string, string) {
	if observationSeen(observations, "provider_auth_failed") {
		return "infra_provider_auth", "provider authentication failed while executing the case"
	}
	if observationSeen(observations, "provider_quota_exhausted") {
		return "infra_provider_quota", "provider quota or balance was exhausted while executing the case"
	}
	if observationSeen(observations, "provider_infra_blocked") {
		return "infra_provider_blocked", "provider-side infrastructure blocked this case before it could complete"
	}
	if observationSeen(observations, "session_context_mismatch") {
		return "session_context_mismatch", "session context was propagated with a mismatched session id"
	}
	if observationSeen(observations, "clarification_requested") && !observationSeen(observations, "clarification_resolved") {
		return "question_left_unresolved", "clarification was requested but never resolved"
	}
	if observationSeen(observations, "approval_requested") &&
		!observationSeen(observations, "approval_resolved") &&
		run != nil &&
		run.Status != RunStatusCompleted {
		return "approval_blocked_without_replan", "approval was requested but the run did not replan or resolve it before ending"
	}
	if observationSeen(observations, "tool_error_seen") && !observationSeen(observations, "tool_error_recovered") {
		return "tool_failed_without_fallback", "a tool failed and the run did not recover with a successful fallback"
	}
	if wantsEvidenceCollection(group, item, run) && observationSeen(observations, "verification_responder_empty") {
		return "missing_evidence_collection", "verification ended with empty grounded content instead of concrete evidence"
	}
	if wantsEvidenceCollection(group, item, run) && observationSeen(observations, "verification_unknown_evidence") {
		return "missing_evidence_collection", "verification relied on unknown evidence placeholders instead of concrete web evidence"
	}
	if wantsEvidenceCollection(group, item, run) && observationSeen(observations, "evidence_collection_empty") {
		return "missing_evidence_collection", "an evidence tool ran but returned no usable sources or page content"
	}
	if wantsEvidenceCollection(group, item, run) && !observationSeen(observations, "evidence_tool_used") {
		return "missing_evidence_collection", "expected evidence collection was not observed"
	}
	return "", ""
}

func wantsEvidenceCollection(group *RunGroup, item *RunGroupItem, run *Run) bool {
	if item != nil {
		for _, observation := range decodeStringSlice(item.Expected["required_observations"]) {
			if strings.EqualFold(strings.TrimSpace(observation), "evidence_tool_used") {
				return true
			}
		}
		if strings.EqualFold(strings.TrimSpace(item.Profile), "research") {
			return true
		}
	}
	if group != nil && strings.EqualFold(strings.TrimSpace(group.Subject), "research") {
		return true
	}
	return run != nil && run.Kind == RunKindResearch
}

func evidenceToolSeen(toolNames []string) bool {
	for _, toolName := range toolNames {
		if isEvidenceToolName(toolName) {
			return true
		}
	}
	return false
}

type evidenceCollectionResults struct {
	sawFinished bool
	usable      bool
	empty       bool
}

func summarizeEvidenceCollectionResults(events []RunEvent) evidenceCollectionResults {
	callIDs := make(map[string]struct{}, 4)
	for _, event := range events {
		switch strings.TrimSpace(event.Type) {
		case "tool_requested", "tool_call":
			if !eventInvokesEvidenceCommand(event) {
				continue
			}
			if callID := eventToolCallID(event); callID != "" {
				callIDs[callID] = struct{}{}
			}
		}
	}

	var out evidenceCollectionResults
	for _, event := range events {
		if strings.TrimSpace(event.Type) != "tool_finished" {
			continue
		}
		if !eventBelongsToEvidenceCollection(event, callIDs) {
			continue
		}
		out.sawFinished = true
		usable, empty := classifyEvidenceResultPayload(event)
		if usable {
			out.usable = true
		}
		if empty {
			out.empty = true
		}
	}
	return out
}

func eventInvokesEvidenceCommand(event RunEvent) bool {
	if isEvidenceToolName(strings.TrimSpace(event.ToolName)) {
		return true
	}
	corpus := strings.ToLower(strings.TrimSpace(strings.Join([]string{event.PayloadJSON, event.Message}, "\n")))
	return containsAny(corpus,
		"blue web_query ",
		"blue web_search ",
		"blue browser",
		"\"tool_name\":\"web_query\"",
		"\"tool\":\"web_query\"",
		"\"tool_name\":\"web_search\"",
		"\"tool\":\"web_search\"",
		"\"tool_name\":\"browser\"",
		"\"tool\":\"browser\"",
	)
}

func eventBelongsToEvidenceCollection(event RunEvent, callIDs map[string]struct{}) bool {
	if isEvidenceToolName(strings.TrimSpace(event.ToolName)) {
		return true
	}
	if callID := eventToolCallID(event); callID != "" {
		if _, ok := callIDs[callID]; ok {
			return true
		}
	}
	return eventInvokesEvidenceCommand(event)
}

func eventToolCallID(event RunEvent) string {
	payload := decodeVerifierEventPayload(event.PayloadJSON)
	return firstNonEmpty(
		mapString(payload, "tool_call_id"),
		metadataString(payload, "tool_call_id"),
	)
}

func decodeVerifierEventPayload(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload
}

func mapString(value map[string]interface{}, key string) string {
	if value == nil {
		return ""
	}
	raw, ok := value[key]
	if !ok {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func classifyEvidenceResultPayload(event RunEvent) (usable bool, empty bool) {
	corpus := strings.ToLower(strings.TrimSpace(strings.Join([]string{event.PayloadJSON, event.Message}, "\n")))
	if corpus == "" {
		return false, false
	}
	usable = containsAny(corpus,
		"\"status\":\"ok\"",
		"status: ok",
		"\"final_url\":\"http",
		"final_url: http",
		"\"target_url\":\"http",
		"target_url: http",
		"\"sources\":[{",
		"sources: [{",
		"\"selected_source\":1",
		"\"selected_source\":2",
		"\"selected\":true",
	)
	if usable {
		return true, false
	}
	partial := containsAny(corpus, "\"status\":\"partial\"", "status: partial")
	noResults := containsAny(corpus, "\"code\":\"no_results\"", "\"next_action\":\"refine_query\"", "next_action: refine_query")
	emptySources := containsAny(corpus, "\"sources\":[]", "\"sources\":\"[]\"", "sources: []")
	noURL := containsAny(corpus, "\"final_url\":\"\"", "\"target_url\":\"\"", "final_url: \\n", "target_url: \\n")
	if partial && noResults && (emptySources || noURL) {
		return false, true
	}
	return false, false
}

func isEvidenceToolName(toolName string) bool {
	name := strings.ToLower(strings.TrimSpace(toolName))
	switch name {
	case "web_search", "web_fetch", "web_read", "web_extract", "web_crawl", "web_query", "research_run", "research_status", "deep_research", "deep-research":
		return true
	}
	return name == "browser" || strings.HasPrefix(name, "browser.")
}

func questionResolvedSuccessfully(event RunEvent) bool {
	payload := eventPayload(event)
	if payload == nil {
		message := strings.TrimSpace(event.Message)
		return message != "" && message != "timed out"
	}
	if timedOut, ok := payload["timed_out"].(bool); ok && timedOut {
		return false
	}
	if raw, ok := payload["error"]; ok && strings.TrimSpace(fmt.Sprint(raw)) != "" {
		return false
	}
	return true
}

func toolFinishedErrored(event RunEvent) bool {
	payload := eventPayload(event)
	if payload == nil {
		return false
	}
	raw, ok := payload["error"]
	return ok && strings.TrimSpace(fmt.Sprint(raw)) != ""
}

func eventPayload(event RunEvent) map[string]interface{} {
	if strings.TrimSpace(event.PayloadJSON) == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		return nil
	}
	return payload
}

func verifyRequiredCard(required string, run *Run, events []RunEvent, artifacts []ArtifactRef) (bool, string) {
	required = strings.TrimSpace(required)
	if required == "" {
		return true, ""
	}
	if value, ok := verificationStructuredValue(required, structuredRunResult(run)); ok {
		return true, formatVerificationValue(value)
	}
	if run != nil {
		if value, ok := verificationStructuredValue(required, decodeJSONMap(run.Result)); ok {
			return true, formatVerificationValue(value)
		}
	}
	for _, event := range events {
		if value, ok := verificationStructuredValue(required, eventPayload(event)); ok {
			return true, formatVerificationValue(value)
		}
	}
	for _, artifact := range artifacts {
		if value, ok := verificationStructuredValue(required, decodeJSONMap(artifact.MetadataJSON)); ok {
			return true, formatVerificationValue(value)
		}
	}
	return false, "missing"
}

func verificationStructuredValue(key string, payload map[string]interface{}) (interface{}, bool) {
	if len(payload) == 0 {
		return nil, false
	}
	return structuredValueForKeys(payload, key)
}

func structuredValueForKeys(payload map[string]interface{}, keys ...string) (interface{}, bool) {
	if len(payload) == 0 || len(keys) == 0 {
		return nil, false
	}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if value, ok := findStructuredValue(payload, key); ok && meaningfulStructuredValue(value) {
			return value, true
		}
	}
	return nil, false
}

func findStructuredValue(raw interface{}, key string) (interface{}, bool) {
	switch typed := raw.(type) {
	case map[string]interface{}:
		for candidate, value := range typed {
			if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(key)) {
				return value, true
			}
		}
		for _, value := range typed {
			if found, ok := findStructuredValue(value, key); ok {
				return found, true
			}
		}
	case []interface{}:
		for _, value := range typed {
			if found, ok := findStructuredValue(value, key); ok {
				return found, true
			}
		}
	}
	return nil, false
}

func meaningfulStructuredValue(value interface{}) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []interface{}:
		return len(typed) > 0
	case []string:
		return len(typed) > 0
	case map[string]interface{}:
		return len(typed) > 0
	default:
		return true
	}
}

func formatVerificationValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return "<empty>"
		}
		return text
	}
}

func observedSessionIDs(events []RunEvent) []string {
	out := make([]string, 0, len(events))
	seen := make(map[string]struct{}, len(events))
	for _, event := range events {
		sessionID, ok := structuredValueForKeys(eventPayload(event), "session_id", "sessionId")
		if !ok {
			continue
		}
		value := strings.TrimSpace(fmt.Sprint(sessionID))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func stringSeenFold(values []string, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), expected) {
			return true
		}
	}
	return false
}

func appendUniqueObservation(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || stringSeenFold(values, value) {
		return values
	}
	return append(values, value)
}
