package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Dataset struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description,omitempty"`
	OwnerUserID     string                 `json:"owner_user_id,omitempty"`
	Subject         string                 `json:"subject,omitempty"`
	DefaultRunKind  RunKind                `json:"default_run_kind,omitempty"`
	DefaultProfile  string                 `json:"default_profile,omitempty"`
	ActiveVersionID string                 `json:"active_version_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type DatasetSpec struct {
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	OwnerUserID    string                 `json:"owner_user_id,omitempty"`
	Subject        string                 `json:"subject,omitempty"`
	DefaultRunKind RunKind                `json:"default_run_kind,omitempty"`
	DefaultProfile string                 `json:"default_profile,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type DatasetFilter struct {
	OwnerUserID string
	Limit       int
}

type DatasetVersion struct {
	ID             string                 `json:"id"`
	DatasetID      string                 `json:"dataset_id"`
	Version        string                 `json:"version"`
	ManifestSHA256 string                 `json:"manifest_sha256,omitempty"`
	ItemCount      int                    `json:"item_count"`
	SourceType     string                 `json:"source_type,omitempty"`
	SourceRef      string                 `json:"source_ref,omitempty"`
	Manifest       map[string]interface{} `json:"manifest,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy      string                 `json:"created_by,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

type DatasetVersionSpec struct {
	Version    string                 `json:"version,omitempty"`
	SourceType string                 `json:"source_type,omitempty"`
	SourceRef  string                 `json:"source_ref,omitempty"`
	Manifest   map[string]interface{} `json:"manifest"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy  string                 `json:"created_by,omitempty"`
}

type EvalSpec struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	OwnerUserID      string                 `json:"owner_user_id,omitempty"`
	Subject          string                 `json:"subject,omitempty"`
	RunKind          RunKind                `json:"run_kind"`
	Profile          string                 `json:"profile,omitempty"`
	DatasetID        string                 `json:"dataset_id,omitempty"`
	DatasetVersionID string                 `json:"dataset_version_id,omitempty"`
	SchedulerConfig  GroupSchedulerConfig   `json:"scheduler_config,omitempty"`
	ScoringConfig    GroupScoringConfig     `json:"scoring_config,omitempty"`
	RuntimePolicy    map[string]interface{} `json:"runtime_policy,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type EvalSpecSpec struct {
	Name             string                 `json:"name"`
	OwnerUserID      string                 `json:"owner_user_id,omitempty"`
	Subject          string                 `json:"subject,omitempty"`
	RunKind          RunKind                `json:"run_kind,omitempty"`
	Profile          string                 `json:"profile,omitempty"`
	DatasetID        string                 `json:"dataset_id"`
	DatasetVersionID string                 `json:"dataset_version_id,omitempty"`
	SchedulerConfig  GroupSchedulerConfig   `json:"scheduler"`
	ScoringConfig    GroupScoringConfig     `json:"scoring"`
	RuntimePolicy    map[string]interface{} `json:"runtime_policy,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type EvalSpecFilter struct {
	OwnerUserID string
	DatasetID   string
	Limit       int
}

type EvalRun struct {
	ID                string                 `json:"id"`
	EvalSpecID        string                 `json:"eval_spec_id"`
	GroupID           string                 `json:"group_id"`
	DatasetVersionID  string                 `json:"dataset_version_id,omitempty"`
	BaselineEvalRunID string                 `json:"baseline_eval_run_id,omitempty"`
	Title             string                 `json:"title,omitempty"`
	OwnerUserID       string                 `json:"owner_user_id,omitempty"`
	Status            RunGroupStatus         `json:"status"`
	TriggerKind       string                 `json:"trigger_kind,omitempty"`
	TriggerRef        string                 `json:"trigger_ref,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Summary           map[string]interface{} `json:"summary,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	StartedAt         *time.Time             `json:"started_at,omitempty"`
	FinishedAt        *time.Time             `json:"finished_at,omitempty"`
}

type EvalRunSpec struct {
	EvalSpecID        string                 `json:"eval_spec_id"`
	BaselineEvalRunID string                 `json:"baseline_eval_run_id,omitempty"`
	Title             string                 `json:"title,omitempty"`
	OwnerUserID       string                 `json:"owner_user_id,omitempty"`
	TriggerKind       string                 `json:"trigger_kind,omitempty"`
	TriggerRef        string                 `json:"trigger_ref,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type EvalRunFilter struct {
	OwnerUserID string
	EvalSpecID  string
	Statuses    []RunGroupStatus
	Limit       int
}

type EvalRunReport struct {
	EvalRun        *EvalRun        `json:"eval_run"`
	EvalSpec       *EvalSpec       `json:"eval_spec,omitempty"`
	Dataset        *Dataset        `json:"dataset,omitempty"`
	DatasetVersion *DatasetVersion `json:"dataset_version,omitempty"`
	GroupReport    *RunGroupReport `json:"group_report,omitempty"`
}

type Baseline struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Subject     string                 `json:"subject,omitempty"`
	OwnerUserID string                 `json:"owner_user_id,omitempty"`
	EvalSpecID  string                 `json:"eval_spec_id"`
	EvalRunID   string                 `json:"eval_run_id"`
	IsDefault   bool                   `json:"is_default"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type BaselineSpec struct {
	Name        string                 `json:"name"`
	Subject     string                 `json:"subject,omitempty"`
	OwnerUserID string                 `json:"owner_user_id,omitempty"`
	EvalSpecID  string                 `json:"eval_spec_id,omitempty"`
	EvalRunID   string                 `json:"eval_run_id"`
	IsDefault   bool                   `json:"is_default,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type BaselineFilter struct {
	OwnerUserID string
	EvalSpecID  string
	Limit       int
}

type SelectorCuratedAssets struct {
	Dataset        *Dataset        `json:"dataset,omitempty"`
	DatasetVersion *DatasetVersion `json:"dataset_version,omitempty"`
	EvalSpec       *EvalSpec       `json:"eval_spec,omitempty"`
}

type CompareEvalRunRequest struct {
	BaseEvalRunID string `json:"base_eval_run_id,omitempty"`
	BaselineID    string `json:"baseline_id,omitempty"`
}

type SelectorGateThresholds struct {
	MinPassRate                *float64 `json:"min_pass_rate,omitempty"`
	MinCriticalPassRate        *float64 `json:"min_critical_pass_rate,omitempty"`
	MinRouteAgreementRate      *float64 `json:"min_route_agreement_rate,omitempty"`
	MinRouteCompatibleRate     *float64 `json:"min_route_compatible_rate,omitempty"`
	MaxClarifyRateDelta        *float64 `json:"max_clarify_rate_delta,omitempty"`
	MaxCriticalRegressionCount *int     `json:"max_critical_regression_count,omitempty"`
}

type SelectorGateRequest struct {
	BaseEvalRunID string                 `json:"base_eval_run_id,omitempty"`
	BaselineID    string                 `json:"baseline_id,omitempty"`
	Thresholds    SelectorGateThresholds `json:"thresholds,omitempty"`
}

type SelectorGateSegmentMetrics struct {
	CaseCount               int     `json:"case_count"`
	RouteAgreementCount     int     `json:"route_agreement_count"`
	RouteAgreementRate      float64 `json:"route_agreement_rate"`
	RouteCompatibleCount    int     `json:"route_compatible_count"`
	RouteCompatibleRate     float64 `json:"route_compatible_rate"`
	RouteImprovementCount   int     `json:"route_improvement_count"`
	RouteDisagreementCount  int     `json:"route_disagreement_count"`
	CriticalCaseCount       int     `json:"critical_case_count"`
	CriticalRegressionCount int     `json:"critical_regression_count"`
	BaseClarifyCount        int     `json:"base_clarify_count"`
	BaseClarifyRate         float64 `json:"base_clarify_rate"`
	TargetClarifyCount      int     `json:"target_clarify_count"`
	TargetClarifyRate       float64 `json:"target_clarify_rate"`
	ClarifyRateDelta        float64 `json:"clarify_rate_delta"`
}

type SelectorGateMetrics struct {
	CaseCount               int                                   `json:"case_count"`
	PassedCount             int                                   `json:"passed_count"`
	PassRate                float64                               `json:"pass_rate"`
	CriticalCaseCount       int                                   `json:"critical_case_count"`
	CriticalPassedCount     int                                   `json:"critical_passed_count"`
	CriticalPassRate        float64                               `json:"critical_pass_rate"`
	RouteCaseCount          int                                   `json:"route_case_count"`
	RouteAgreementCount     int                                   `json:"route_agreement_count"`
	RouteAgreementRate      float64                               `json:"route_agreement_rate"`
	RouteCompatibleCount    int                                   `json:"route_compatible_count"`
	RouteCompatibleRate     float64                               `json:"route_compatible_rate"`
	RouteImprovementCount   int                                   `json:"route_improvement_count"`
	RouteDisagreementCount  int                                   `json:"route_disagreement_count"`
	CriticalRegressionCount int                                   `json:"critical_regression_count"`
	BaseClarifyRate         float64                               `json:"base_clarify_rate"`
	TargetClarifyRate       float64                               `json:"target_clarify_rate"`
	ClarifyRateDelta        float64                               `json:"clarify_rate_delta"`
	LocaleBreakdown         map[string]SelectorGateSegmentMetrics `json:"locale_breakdown,omitempty"`
	PrimaryRouteBreakdown   map[string]SelectorGateSegmentMetrics `json:"primary_route_breakdown,omitempty"`
}

type SelectorGateCheck struct {
	Name     string                 `json:"name"`
	Passed   bool                   `json:"passed"`
	Actual   interface{}            `json:"actual,omitempty"`
	Expected interface{}            `json:"expected,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

type SelectorGateReport struct {
	TargetEvalRunID    string                 `json:"target_eval_run_id"`
	BaseEvalRunID      string                 `json:"base_eval_run_id,omitempty"`
	BaselineID         string                 `json:"baseline_id,omitempty"`
	ComparisonReportID string                 `json:"comparison_report_id,omitempty"`
	Metrics            SelectorGateMetrics    `json:"metrics"`
	Thresholds         map[string]interface{} `json:"thresholds,omitempty"`
	Checks             []SelectorGateCheck    `json:"checks,omitempty"`
	Passed             bool                   `json:"passed"`
	CreatedAt          time.Time              `json:"created_at"`
}

type ComparisonCaseDelta struct {
	Key                 string  `json:"key"`
	Label               string  `json:"label,omitempty"`
	ItemIndex           int     `json:"item_index"`
	Profile             string  `json:"profile,omitempty"`
	BaseVerdict         string  `json:"base_verdict,omitempty"`
	TargetVerdict       string  `json:"target_verdict,omitempty"`
	BaseStatus          string  `json:"base_status,omitempty"`
	TargetStatus        string  `json:"target_status,omitempty"`
	BaseScore           float64 `json:"base_score,omitempty"`
	TargetScore         float64 `json:"target_score,omitempty"`
	DeltaScore          float64 `json:"delta_score,omitempty"`
	BaseRunID           string  `json:"base_run_id,omitempty"`
	TargetRunID         string  `json:"target_run_id,omitempty"`
	BaseReason          string  `json:"base_reason,omitempty"`
	TargetReason        string  `json:"target_reason,omitempty"`
	BaseFailureLabel    string  `json:"base_failure_label,omitempty"`
	TargetFailureLabel  string  `json:"target_failure_label,omitempty"`
	BaseVerification    string  `json:"base_verification,omitempty"`
	TargetVerification  string  `json:"target_verification,omitempty"`
	BaseEvidenceScore   float64 `json:"base_evidence_score,omitempty"`
	TargetEvidenceScore float64 `json:"target_evidence_score,omitempty"`
}

type ComparisonReport struct {
	ID              string                 `json:"id"`
	OwnerUserID     string                 `json:"owner_user_id,omitempty"`
	BaselineID      string                 `json:"baseline_id,omitempty"`
	EvalSpecID      string                 `json:"eval_spec_id"`
	BaseEvalRunID   string                 `json:"base_eval_run_id"`
	TargetEvalRunID string                 `json:"target_eval_run_id"`
	Summary         map[string]interface{} `json:"summary,omitempty"`
	Regressions     []ComparisonCaseDelta  `json:"regressions,omitempty"`
	Improvements    []ComparisonCaseDelta  `json:"improvements,omitempty"`
	ScorerDelta     map[string]interface{} `json:"scorer_delta,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

type GroupPromotionSpec struct {
	DatasetName string `json:"dataset_name"`
	Description string `json:"description,omitempty"`
	Subject     string `json:"subject,omitempty"`
	EvalName    string `json:"eval_name"`
}

type GroupPromotionResult struct {
	Dataset        *Dataset        `json:"dataset,omitempty"`
	DatasetVersion *DatasetVersion `json:"dataset_version,omitempty"`
	EvalSpec       *EvalSpec       `json:"eval_spec,omitempty"`
}

type DatasetManifest struct {
	Dataset  DatasetManifestMeta     `json:"dataset,omitempty"`
	Defaults DatasetManifestDefaults `json:"defaults,omitempty"`
	Items    []DatasetManifestItem   `json:"items,omitempty"`
}

type DatasetManifestMeta struct {
	Name    string `json:"name,omitempty"`
	Subject string `json:"subject,omitempty"`
}

type DatasetManifestDefaults struct {
	RunKind       RunKind                `json:"run_kind,omitempty"`
	Profile       string                 `json:"profile,omitempty"`
	Scheduler     GroupSchedulerConfig   `json:"scheduler,omitempty"`
	Scoring       GroupScoringConfig     `json:"scoring,omitempty"`
	RuntimePolicy map[string]interface{} `json:"runtime_policy,omitempty"`
}

type DatasetManifestItem struct {
	ID       string                 `json:"id,omitempty"`
	RunKind  RunKind                `json:"run_kind,omitempty"`
	Profile  string                 `json:"profile,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Expected map[string]interface{} `json:"expected,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func (v *DatasetVersion) DecodeManifest() (*DatasetManifest, error) {
	if v == nil {
		return nil, fmt.Errorf("dataset version is required")
	}
	return decodeDatasetManifest(v.Manifest)
}

func decodeDatasetManifest(raw map[string]interface{}) (*DatasetManifest, error) {
	if len(raw) == 0 {
		return &DatasetManifest{}, nil
	}
	blob, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var manifest DatasetManifest
	if err := json.Unmarshal(blob, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func manifestItemCount(raw map[string]interface{}) (int, error) {
	manifest, err := decodeDatasetManifest(raw)
	if err != nil {
		return 0, err
	}
	return len(manifest.Items), nil
}

func manifestSHA256(raw map[string]interface{}) string {
	sum := sha256.Sum256([]byte(marshalMetadata(raw)))
	return hex.EncodeToString(sum[:])
}

func normalizeDatasetVersion(version string) string {
	version = strings.TrimSpace(version)
	if version != "" {
		return version
	}
	return time.Now().UTC().Format("20060102-150405")
}
