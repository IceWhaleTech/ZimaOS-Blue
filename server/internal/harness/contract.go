package harness

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type HarnessContract struct {
	Deliverables          []string                  `json:"deliverables,omitempty"`
	SuccessCriteria       []string                  `json:"success_criteria,omitempty"`
	ExpectedArtifacts     []HarnessExpectedArtifact `json:"expected_artifacts,omitempty"`
	RequiredToolCalls     []string                  `json:"required_tool_calls,omitempty"`
	ForbiddenToolCalls    []string                  `json:"forbidden_tool_calls,omitempty"`
	RequiredChecks        []string                  `json:"required_checks,omitempty"`
	RequiredObservations  []string                  `json:"required_observations,omitempty"`
	ForbiddenObservations []string                  `json:"forbidden_observations,omitempty"`
	BrowserChecks         []HarnessBrowserCheck     `json:"browser_checks,omitempty"`
	APIChecks             []HarnessAPICheck         `json:"api_checks,omitempty"`
	FallbackOrder         []string                  `json:"fallback_order,omitempty"`
	StopConditions        []string                  `json:"stop_conditions,omitempty"`
	EvaluatorHints        []string                  `json:"evaluator_hints,omitempty"`
	RiskLevel             string                    `json:"risk_level,omitempty"`
}

type HarnessExpectedArtifact struct {
	Path      string `json:"path,omitempty"`
	Label     string `json:"label,omitempty"`
	MustExist bool   `json:"must_exist,omitempty"`
}

type HarnessBrowserCheck struct {
	Name                string `json:"name,omitempty"`
	Target              string `json:"target,omitempty"`
	Expectation         string `json:"expectation,omitempty"`
	RequiredObservation string `json:"required_observation,omitempty"`
	RequiredArtifact    string `json:"required_artifact,omitempty"`
	FailureLabel        string `json:"failure_label,omitempty"`
	RequireScreenshot   bool   `json:"require_screenshot,omitempty"`
}

type HarnessAPICheck struct {
	Name          string `json:"name,omitempty"`
	Target        string `json:"target,omitempty"`
	Expectation   string `json:"expectation,omitempty"`
	RequiredCheck string `json:"required_check,omitempty"`
	FailureLabel  string `json:"failure_label,omitempty"`
}

type HarnessAdaptivePolicy struct {
	Profile             string `json:"profile,omitempty"`
	Reason              string `json:"reason,omitempty"`
	EnableExternalQA    bool   `json:"enable_external_qa,omitempty"`
	EnableBrowserQA     bool   `json:"enable_browser_qa,omitempty"`
	EnableCheckpoints   bool   `json:"enable_checkpoints,omitempty"`
	MaxRecoveryAttempts int    `json:"max_recovery_attempts,omitempty"`
	CheckpointInterval  int    `json:"checkpoint_interval,omitempty"`
}

type HarnessCheckpoint struct {
	Version           string          `json:"version,omitempty"`
	RunID             string          `json:"run_id,omitempty"`
	GroupID           string          `json:"group_id,omitempty"`
	GroupItemID       string          `json:"group_item_id,omitempty"`
	AttemptIndex      int             `json:"attempt_index,omitempty"`
	Goal              string          `json:"goal,omitempty"`
	Summary           string          `json:"summary,omitempty"`
	VerifiedEvidence  []string        `json:"verified_evidence,omitempty"`
	UnresolvedRisks   []string        `json:"unresolved_risks,omitempty"`
	FailureLabels     []string        `json:"failure_labels,omitempty"`
	NextContract      HarnessContract `json:"next_contract,omitempty"`
	EvaluatorInput    string          `json:"evaluator_input,omitempty"`
	RecommendedResume string          `json:"recommended_resume,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

func DecodeHarnessContract(sources ...map[string]interface{}) HarnessContract {
	contract := HarnessContract{}
	for _, source := range sources {
		contract = mergeHarnessContracts(contract, decodeHarnessContractMap(source))
	}
	return normalizeHarnessContract(contract)
}

func HarnessContractMetadata(contract HarnessContract) map[string]interface{} {
	contract = normalizeHarnessContract(contract)
	if len(contract.Deliverables) == 0 &&
		len(contract.SuccessCriteria) == 0 &&
		len(contract.ExpectedArtifacts) == 0 &&
		len(contract.RequiredToolCalls) == 0 &&
		len(contract.ForbiddenToolCalls) == 0 &&
		len(contract.RequiredChecks) == 0 &&
		len(contract.RequiredObservations) == 0 &&
		len(contract.ForbiddenObservations) == 0 &&
		len(contract.BrowserChecks) == 0 &&
		len(contract.APIChecks) == 0 &&
		len(contract.FallbackOrder) == 0 &&
		len(contract.StopConditions) == 0 &&
		len(contract.EvaluatorHints) == 0 &&
		strings.TrimSpace(contract.RiskLevel) == "" {
		return nil
	}
	raw, err := json.Marshal(contract)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func ApplyHarnessContractToExpected(expected map[string]interface{}, contract HarnessContract) map[string]interface{} {
	expected = cloneMetadataMap(expected)
	if expected == nil {
		expected = map[string]interface{}{}
	}
	contractMeta := HarnessContractMetadata(contract)
	if len(contract.RequiredToolCalls) > 0 {
		expected["required_tool_calls"] = contractMeta["required_tool_calls"]
	}
	if len(contract.ForbiddenToolCalls) > 0 {
		expected["forbidden_tool_calls"] = contractMeta["forbidden_tool_calls"]
	}
	if len(contract.RequiredChecks) > 0 {
		expected["required_checks"] = contractMeta["required_checks"]
	}
	if len(contract.RequiredObservations) > 0 {
		expected["required_observations"] = contractMeta["required_observations"]
	}
	if len(contract.ForbiddenObservations) > 0 {
		expected["forbidden_observations"] = contractMeta["forbidden_observations"]
	}
	if len(contract.ExpectedArtifacts) > 0 {
		expected["expected_artifacts"] = contractMeta["expected_artifacts"]
	}
	if len(contract.BrowserChecks) > 0 {
		expected["browser_checks"] = contractMeta["browser_checks"]
	}
	if len(contract.APIChecks) > 0 {
		expected["api_checks"] = contractMeta["api_checks"]
	}
	return expected
}

func HarnessContractSuccessCriteria(contract HarnessContract) []string {
	if len(contract.SuccessCriteria) > 0 {
		return append([]string(nil), contract.SuccessCriteria...)
	}
	return append([]string(nil), contract.Deliverables...)
}

func HarnessContractFallbackPlan(contract HarnessContract) []string {
	return append([]string(nil), contract.FallbackOrder...)
}

func HarnessAdaptivePolicyMetadata(policy HarnessAdaptivePolicy) map[string]interface{} {
	if strings.TrimSpace(policy.Profile) == "" && strings.TrimSpace(policy.Reason) == "" &&
		!policy.EnableExternalQA && !policy.EnableBrowserQA && !policy.EnableCheckpoints &&
		policy.MaxRecoveryAttempts == 0 && policy.CheckpointInterval == 0 {
		return nil
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func DeriveHarnessAdaptivePolicy(model string, kind RunKind, contract HarnessContract) HarnessAdaptivePolicy {
	contract = normalizeHarnessContract(contract)
	strongModel := isStrongHarnessModel(model)
	lightweight := isLightweightHarnessContract(kind, contract)
	highRisk := strings.EqualFold(contract.RiskLevel, "high") ||
		len(contract.BrowserChecks) > 0 ||
		len(contract.APIChecks) > 0 ||
		len(contract.StopConditions) > 0

	policy := HarnessAdaptivePolicy{
		Profile:             "standard",
		Reason:              "default harness path",
		EnableExternalQA:    true,
		EnableBrowserQA:     len(contract.BrowserChecks) > 0,
		EnableCheckpoints:   true,
		MaxRecoveryAttempts: 1,
		CheckpointInterval:  1,
	}
	switch {
	case strongModel && lightweight && !highRisk:
		policy.Profile = "light"
		policy.Reason = "strong model with a lightweight low-risk contract"
		policy.EnableExternalQA = false
		policy.EnableCheckpoints = false
		policy.MaxRecoveryAttempts = 0
		policy.CheckpointInterval = 0
	case highRisk:
		policy.Profile = "strict"
		policy.Reason = "browser, API, or high-risk contract requires the full harness loop"
	default:
		policy.Profile = "standard"
		policy.Reason = "task benefits from skeptical QA and checkpoint visibility"
	}
	if !policy.EnableBrowserQA {
		policy.EnableBrowserQA = len(contract.BrowserChecks) > 0
	}
	if policy.EnableBrowserQA {
		policy.EnableExternalQA = true
	}
	return policy
}

func BuildHarnessContractContext(contract HarnessContract) string {
	contract = normalizeHarnessContract(contract)
	parts := make([]string, 0, 8)
	if len(contract.Deliverables) > 0 {
		parts = append(parts, "Deliverables:\n- "+strings.Join(contract.Deliverables, "\n- "))
	}
	if len(contract.ExpectedArtifacts) > 0 {
		items := make([]string, 0, len(contract.ExpectedArtifacts))
		for _, artifact := range contract.ExpectedArtifacts {
			target := firstNonEmpty(strings.TrimSpace(artifact.Path), strings.TrimSpace(artifact.Label))
			if target == "" {
				continue
			}
			items = append(items, target)
		}
		if len(items) > 0 {
			parts = append(parts, "Expected artifacts:\n- "+strings.Join(items, "\n- "))
		}
	}
	if len(contract.RequiredToolCalls) > 0 {
		parts = append(parts, "Required tool calls:\n- "+strings.Join(contract.RequiredToolCalls, "\n- "))
	}
	if len(contract.RequiredObservations) > 0 {
		parts = append(parts, "Required observations:\n- "+strings.Join(contract.RequiredObservations, "\n- "))
	}
	if len(contract.FallbackOrder) > 0 {
		parts = append(parts, "Fallback order:\n- "+strings.Join(contract.FallbackOrder, "\n- "))
	}
	if len(contract.StopConditions) > 0 {
		parts = append(parts, "Stop conditions:\n- "+strings.Join(contract.StopConditions, "\n- "))
	}
	if len(contract.BrowserChecks) > 0 {
		items := make([]string, 0, len(contract.BrowserChecks))
		for _, check := range contract.BrowserChecks {
			label := firstNonEmpty(check.Name, check.Target, "browser QA")
			expectation := strings.TrimSpace(check.Expectation)
			if expectation != "" {
				label = fmt.Sprintf("%s (%s)", label, expectation)
			}
			items = append(items, label)
		}
		parts = append(parts, "Browser QA:\n- "+strings.Join(items, "\n- "))
	}
	if len(contract.APIChecks) > 0 {
		items := make([]string, 0, len(contract.APIChecks))
		for _, check := range contract.APIChecks {
			label := firstNonEmpty(check.Name, check.Target, "API check")
			expectation := strings.TrimSpace(check.Expectation)
			if expectation != "" {
				label = fmt.Sprintf("%s (%s)", label, expectation)
			}
			items = append(items, label)
		}
		parts = append(parts, "API checks:\n- "+strings.Join(items, "\n- "))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func BuildHarnessCheckpointContext(raw map[string]interface{}) string {
	if len(raw) == 0 {
		return ""
	}
	summary := strings.TrimSpace(fmt.Sprint(raw["summary"]))
	goal := strings.TrimSpace(fmt.Sprint(raw["goal"]))
	verifiedEvidence := decodeStringSlice(raw["verified_evidence"])
	unresolvedRisks := decodeStringSlice(raw["unresolved_risks"])
	recommendedResume := strings.TrimSpace(fmt.Sprint(raw["recommended_resume"]))

	var parts []string
	if goal != "" {
		parts = append(parts, "Checkpoint goal: "+goal)
	}
	if summary != "" {
		parts = append(parts, "Checkpoint summary: "+summary)
	}
	if len(verifiedEvidence) > 0 {
		parts = append(parts, "Verified evidence:\n- "+strings.Join(verifiedEvidence, "\n- "))
	}
	if len(unresolvedRisks) > 0 {
		parts = append(parts, "Unresolved risks:\n- "+strings.Join(unresolvedRisks, "\n- "))
	}
	if recommendedResume != "" {
		parts = append(parts, "Recommended next move: "+recommendedResume)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func decodeHarnessContractMap(source map[string]interface{}) HarnessContract {
	if len(source) == 0 {
		return HarnessContract{}
	}
	contract := decodeHarnessContractFields(source)
	for _, key := range []string{"harness_contract", "contract"} {
		if nested := nestedMetadataMap(source, key); len(nested) > 0 {
			contract = mergeHarnessContracts(contract, decodeHarnessContractFields(nested))
		}
	}
	return contract
}

func decodeHarnessContractFields(source map[string]interface{}) HarnessContract {
	return HarnessContract{
		Deliverables:          decodeStringSlice(source["deliverables"]),
		SuccessCriteria:       decodeStringSlice(source["success_criteria"]),
		ExpectedArtifacts:     decodeHarnessExpectedArtifacts(source["expected_artifacts"]),
		RequiredToolCalls:     decodeStringSlice(source["required_tool_calls"]),
		ForbiddenToolCalls:    decodeStringSlice(source["forbidden_tool_calls"]),
		RequiredChecks:        decodeStringSlice(source["required_checks"]),
		RequiredObservations:  decodeStringSlice(source["required_observations"]),
		ForbiddenObservations: decodeStringSlice(source["forbidden_observations"]),
		BrowserChecks:         decodeHarnessBrowserChecks(source["browser_checks"]),
		APIChecks:             decodeHarnessAPIChecks(source["api_checks"]),
		FallbackOrder:         decodeStringSlice(firstNonNil(source["fallback_order"], source["fallback_plan"])),
		StopConditions:        decodeStringSlice(source["stop_conditions"]),
		EvaluatorHints:        decodeStringSlice(source["evaluator_hints"]),
		RiskLevel:             strings.TrimSpace(fmt.Sprint(source["risk_level"])),
	}
}

func normalizeHarnessContract(contract HarnessContract) HarnessContract {
	contract.Deliverables = dedupeContractStrings(contract.Deliverables)
	contract.SuccessCriteria = dedupeContractStrings(contract.SuccessCriteria)
	contract.RequiredToolCalls = dedupeContractStrings(contract.RequiredToolCalls)
	contract.ForbiddenToolCalls = dedupeContractStrings(contract.ForbiddenToolCalls)
	contract.RequiredChecks = dedupeContractStrings(contract.RequiredChecks)
	contract.RequiredObservations = dedupeContractStrings(contract.RequiredObservations)
	contract.ForbiddenObservations = dedupeContractStrings(contract.ForbiddenObservations)
	contract.FallbackOrder = dedupeContractStrings(contract.FallbackOrder)
	contract.StopConditions = dedupeContractStrings(contract.StopConditions)
	contract.EvaluatorHints = dedupeContractStrings(contract.EvaluatorHints)
	contract.ExpectedArtifacts = dedupeExpectedArtifacts(contract.ExpectedArtifacts)
	contract.BrowserChecks = dedupeBrowserChecks(contract.BrowserChecks)
	contract.APIChecks = dedupeAPIChecks(contract.APIChecks)
	contract.RiskLevel = strings.TrimSpace(contract.RiskLevel)
	return contract
}

func mergeHarnessContracts(base HarnessContract, override HarnessContract) HarnessContract {
	base.Deliverables = append(base.Deliverables, override.Deliverables...)
	base.SuccessCriteria = append(base.SuccessCriteria, override.SuccessCriteria...)
	base.ExpectedArtifacts = append(base.ExpectedArtifacts, override.ExpectedArtifacts...)
	base.RequiredToolCalls = append(base.RequiredToolCalls, override.RequiredToolCalls...)
	base.ForbiddenToolCalls = append(base.ForbiddenToolCalls, override.ForbiddenToolCalls...)
	base.RequiredChecks = append(base.RequiredChecks, override.RequiredChecks...)
	base.RequiredObservations = append(base.RequiredObservations, override.RequiredObservations...)
	base.ForbiddenObservations = append(base.ForbiddenObservations, override.ForbiddenObservations...)
	base.BrowserChecks = append(base.BrowserChecks, override.BrowserChecks...)
	base.APIChecks = append(base.APIChecks, override.APIChecks...)
	base.FallbackOrder = append(base.FallbackOrder, override.FallbackOrder...)
	base.StopConditions = append(base.StopConditions, override.StopConditions...)
	base.EvaluatorHints = append(base.EvaluatorHints, override.EvaluatorHints...)
	if strings.TrimSpace(override.RiskLevel) != "" {
		base.RiskLevel = strings.TrimSpace(override.RiskLevel)
	}
	return normalizeHarnessContract(base)
}

func decodeHarnessExpectedArtifacts(raw interface{}) []HarnessExpectedArtifact {
	contracts := decodeExpectedArtifactContracts(raw)
	out := make([]HarnessExpectedArtifact, 0, len(contracts))
	for _, contract := range contracts {
		out = append(out, HarnessExpectedArtifact{
			Path:      strings.TrimSpace(contract.Path),
			Label:     strings.TrimSpace(contract.Label),
			MustExist: contract.MustExist,
		})
	}
	return out
}

func decodeHarnessBrowserChecks(raw interface{}) []HarnessBrowserCheck {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]HarnessBrowserCheck, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, HarnessBrowserCheck{
			Name:                strings.TrimSpace(fmt.Sprint(record["name"])),
			Target:              strings.TrimSpace(fmt.Sprint(record["target"])),
			Expectation:         strings.TrimSpace(fmt.Sprint(record["expectation"])),
			RequiredObservation: strings.TrimSpace(fmt.Sprint(record["required_observation"])),
			RequiredArtifact:    strings.TrimSpace(fmt.Sprint(record["required_artifact"])),
			FailureLabel:        strings.TrimSpace(fmt.Sprint(record["failure_label"])),
			RequireScreenshot:   record["require_screenshot"] == true,
		})
	}
	return out
}

func decodeHarnessAPIChecks(raw interface{}) []HarnessAPICheck {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]HarnessAPICheck, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, HarnessAPICheck{
			Name:          strings.TrimSpace(fmt.Sprint(record["name"])),
			Target:        strings.TrimSpace(fmt.Sprint(record["target"])),
			Expectation:   strings.TrimSpace(fmt.Sprint(record["expectation"])),
			RequiredCheck: strings.TrimSpace(fmt.Sprint(record["required_check"])),
			FailureLabel:  strings.TrimSpace(fmt.Sprint(record["failure_label"])),
		})
	}
	return out
}

func dedupeContractStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
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

func dedupeExpectedArtifacts(values []HarnessExpectedArtifact) []HarnessExpectedArtifact {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]HarnessExpectedArtifact, 0, len(values))
	for _, value := range values {
		value.Path = strings.TrimSpace(value.Path)
		value.Label = strings.TrimSpace(value.Label)
		key := fmt.Sprintf("%s|%s|%t", value.Path, value.Label, value.MustExist)
		if value.Path == "" && value.Label == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func dedupeBrowserChecks(values []HarnessBrowserCheck) []HarnessBrowserCheck {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]HarnessBrowserCheck, 0, len(values))
	for _, value := range values {
		value.Name = strings.TrimSpace(value.Name)
		value.Target = strings.TrimSpace(value.Target)
		value.Expectation = strings.TrimSpace(value.Expectation)
		value.RequiredObservation = strings.TrimSpace(value.RequiredObservation)
		value.RequiredArtifact = strings.TrimSpace(value.RequiredArtifact)
		value.FailureLabel = strings.TrimSpace(value.FailureLabel)
		key := fmt.Sprintf("%s|%s|%s|%s|%s|%t", value.Name, value.Target, value.Expectation, value.RequiredObservation, value.RequiredArtifact, value.RequireScreenshot)
		if key == "|||||false" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func dedupeAPIChecks(values []HarnessAPICheck) []HarnessAPICheck {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]HarnessAPICheck, 0, len(values))
	for _, value := range values {
		value.Name = strings.TrimSpace(value.Name)
		value.Target = strings.TrimSpace(value.Target)
		value.Expectation = strings.TrimSpace(value.Expectation)
		value.RequiredCheck = strings.TrimSpace(value.RequiredCheck)
		value.FailureLabel = strings.TrimSpace(value.FailureLabel)
		key := fmt.Sprintf("%s|%s|%s|%s", value.Name, value.Target, value.Expectation, value.RequiredCheck)
		if key == "|||" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func isStrongHarnessModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(model, "gpt-5"),
		strings.Contains(model, "claude-4"),
		strings.Contains(model, "claude-opus"),
		strings.Contains(model, "gemini-2.5"),
		strings.Contains(model, "o3"),
		strings.Contains(model, "o4"):
		return true
	default:
		return false
	}
}

func isLightweightHarnessContract(kind RunKind, contract HarnessContract) bool {
	if len(contract.BrowserChecks) > 0 || len(contract.APIChecks) > 0 || len(contract.ExpectedArtifacts) > 0 {
		return false
	}
	if len(contract.RequiredChecks) > 1 || len(contract.RequiredObservations) > 1 {
		return false
	}
	if len(contract.StopConditions) > 0 {
		return false
	}
	switch kind {
	case RunKindResearch:
		return false
	default:
		return len(contract.Deliverables) <= 1 && len(contract.SuccessCriteria) <= 2
	}
}

func firstNonNil(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
