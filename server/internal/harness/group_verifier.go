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
	observations := deriveVerificationObservations(events, artifacts, toolNames)
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
	if len(expectedArtifacts) == 0 && len(requiredTools) == 0 && len(requiredChecks) == 0 && runCompleted {
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

func deriveVerificationObservations(events []RunEvent, artifacts []ArtifactRef, toolNames []string) []string {
	observed := newObservationSet()
	if len(artifacts) > 0 {
		observed.add("artifact_emitted")
	}
	if evidenceToolSeen(toolNames) {
		observed.add("evidence_tool_used")
	}
	if browserToolSeen(toolNames) {
		observed.add("browser_used")
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
		}
	}
	return observed.values()
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
	case "missing_clarification", "question_left_unresolved", "approval_blocked_without_replan", "tool_failed_without_fallback", "missing_evidence_collection":
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
	case "missing_artifact", "required_check_missing", "tool_selection_error", "run_failed", "run_not_completed", "verification_failed", "timeout", "missing_clarification", "question_left_unresolved", "approval_blocked_without_replan", "tool_failed_without_fallback", "missing_evidence_collection":
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
	default:
		normalized := strings.TrimSpace(observation)
		return "verification_failed", fmt.Sprintf("required observation %q was not observed", normalized)
	}
}

func processObservationFailureLabel(group *RunGroup, item *RunGroupItem, run *Run, observations []string) (string, string) {
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
	if strings.TrimSpace(fmt.Sprint(payload["error"])) != "" {
		return false
	}
	return true
}

func toolFinishedErrored(event RunEvent) bool {
	payload := eventPayload(event)
	if payload == nil {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(payload["error"])) != ""
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
