package harness

import "time"

type Batch1ExecutionAssets struct {
	Dataset        *Dataset        `json:"dataset,omitempty"`
	DatasetVersion *DatasetVersion `json:"dataset_version,omitempty"`
	EvalSpec       *EvalSpec       `json:"eval_spec,omitempty"`
}

type ExecutionEquivalenceThresholds struct {
	MaxPassRateDrop               *float64 `json:"max_pass_rate_drop,omitempty"`
	MaxCriticalRegressionCount    *int     `json:"max_critical_regression_count,omitempty"`
	MaxVerificationPassRateDrop   *float64 `json:"max_verification_pass_rate_drop,omitempty"`
	MaxEvidenceBackedPassRateDrop *float64 `json:"max_evidence_backed_pass_rate_drop,omitempty"`
}

type ExecutionEquivalenceRequest struct {
	BaseEvalRunID string                         `json:"base_eval_run_id,omitempty"`
	BaselineID    string                         `json:"baseline_id,omitempty"`
	Thresholds    ExecutionEquivalenceThresholds `json:"thresholds,omitempty"`
}

type ExecutionEquivalenceSegmentMetrics struct {
	CaseCount               int     `json:"case_count"`
	PassedCount             int     `json:"passed_count"`
	PassRate                float64 `json:"pass_rate"`
	CriticalCaseCount       int     `json:"critical_case_count"`
	CriticalPassedCount     int     `json:"critical_passed_count"`
	CriticalPassRate        float64 `json:"critical_pass_rate"`
	InfraBlockedCount       int     `json:"infra_blocked_count"`
	RegressionCount         int     `json:"regression_count"`
	ImprovementCount        int     `json:"improvement_count"`
	NewFailureCount         int     `json:"new_failure_count"`
	ResolvedFailureCount    int     `json:"resolved_failure_count"`
	CriticalRegressionCount int     `json:"critical_regression_count"`
}

type ExecutionEquivalenceMetrics struct {
	CaseCount                    int                                           `json:"case_count"`
	PassedCount                  int                                           `json:"passed_count"`
	PassRate                     float64                                       `json:"pass_rate"`
	CriticalCaseCount            int                                           `json:"critical_case_count"`
	CriticalPassedCount          int                                           `json:"critical_passed_count"`
	CriticalPassRate             float64                                       `json:"critical_pass_rate"`
	InfraBlockedCount            int                                           `json:"infra_blocked_count"`
	BaseInfraBlockedCount        int                                           `json:"base_infra_blocked_count"`
	TargetInfraBlockedCount      int                                           `json:"target_infra_blocked_count"`
	InfraBlockedDelta            int                                           `json:"infra_blocked_delta"`
	BasePassRate                 float64                                       `json:"base_pass_rate"`
	TargetPassRate               float64                                       `json:"target_pass_rate"`
	PassRateDelta                float64                                       `json:"pass_rate_delta"`
	BaseVerificationPassRate     float64                                       `json:"base_verification_pass_rate"`
	TargetVerificationPassRate   float64                                       `json:"target_verification_pass_rate"`
	VerificationPassRateDelta    float64                                       `json:"verification_pass_rate_delta"`
	BaseEvidenceBackedPassRate   float64                                       `json:"base_evidence_backed_pass_rate"`
	TargetEvidenceBackedPassRate float64                                       `json:"target_evidence_backed_pass_rate"`
	EvidenceBackedPassRateDelta  float64                                       `json:"evidence_backed_pass_rate_delta"`
	RegressionCount              int                                           `json:"regression_count"`
	ImprovementCount             int                                           `json:"improvement_count"`
	NewFailureCount              int                                           `json:"new_failure_count"`
	ResolvedFailureCount         int                                           `json:"resolved_failure_count"`
	CriticalRegressionCount      int                                           `json:"critical_regression_count"`
	LocaleBreakdown              map[string]ExecutionEquivalenceSegmentMetrics `json:"locale_breakdown,omitempty"`
	PrimaryRouteBreakdown        map[string]ExecutionEquivalenceSegmentMetrics `json:"primary_route_breakdown,omitempty"`
}

type ExecutionEquivalenceCheck struct {
	Name     string                 `json:"name"`
	Passed   bool                   `json:"passed"`
	Actual   interface{}            `json:"actual,omitempty"`
	Expected interface{}            `json:"expected,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

type ExecutionEquivalenceReport struct {
	TargetEvalRunID    string                      `json:"target_eval_run_id"`
	BaseEvalRunID      string                      `json:"base_eval_run_id,omitempty"`
	BaselineID         string                      `json:"baseline_id,omitempty"`
	ComparisonReportID string                      `json:"comparison_report_id,omitempty"`
	Metrics            ExecutionEquivalenceMetrics `json:"metrics"`
	Thresholds         map[string]interface{}      `json:"thresholds,omitempty"`
	Checks             []ExecutionEquivalenceCheck `json:"checks,omitempty"`
	Passed             bool                        `json:"passed"`
	CreatedAt          time.Time                   `json:"created_at"`
}
