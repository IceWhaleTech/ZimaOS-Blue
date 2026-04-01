package harness

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	defaultSkillCutoverMinMedianSchemaByteReductionRate = 0.80
	defaultSkillCutoverMaxMedianLatencyIncreaseRate     = 0.10
	// Dry-run selector and tiny local harness runs can complete within scheduler jitter.
	// Keep a small floor so sub-50ms differences do not register as a cutover regression.
	skillCutoverLatencyNoiseFloorMs = 50.0
)

var defaultSkillCutoverAllowedFinalNativeTools = []string{"exec"}

type SkillCutoverBudgetThresholds struct {
	MinMedianSchemaByteReductionRate *float64 `json:"min_median_schema_byte_reduction_rate,omitempty"`
	MaxMedianLatencyIncreaseRate     *float64 `json:"max_median_latency_increase_rate,omitempty"`
	AllowedFinalNativeTools          []string `json:"allowed_final_native_tools,omitempty"`
}

type SkillCutoverBudgetRequest struct {
	BaseEvalRunID string                       `json:"base_eval_run_id,omitempty"`
	BaselineID    string                       `json:"baseline_id,omitempty"`
	Thresholds    SkillCutoverBudgetThresholds `json:"thresholds,omitempty"`
}

type SkillCutoverBudgetMetrics struct {
	CaseCount                      int      `json:"case_count"`
	ComparableCaseCount            int      `json:"comparable_case_count"`
	MissingSurfaceCaseCount        int      `json:"missing_surface_case_count"`
	BaseMedianToolCount            float64  `json:"base_median_tool_count"`
	TargetMedianToolCount          float64  `json:"target_median_tool_count"`
	BaseMedianSchemaBytes          float64  `json:"base_median_schema_bytes"`
	TargetMedianSchemaBytes        float64  `json:"target_median_schema_bytes"`
	MedianSchemaByteReductionRate  float64  `json:"median_schema_byte_reduction_rate"`
	BaseMedianLatencyMs            float64  `json:"base_median_latency_ms"`
	TargetMedianLatencyMs          float64  `json:"target_median_latency_ms"`
	MedianLatencyIncreaseRate      float64  `json:"median_latency_increase_rate"`
	AllowedFinalNativeTools        []string `json:"allowed_final_native_tools,omitempty"`
	AllowedFinalNativeToolCases    int      `json:"allowed_final_native_tool_cases"`
	AllowedFinalNativeToolCaseRate float64  `json:"allowed_final_native_tool_case_rate"`
	NonAllowedNativeToolCaseCount  int      `json:"non_allowed_native_tool_case_count"`
}

type SkillCutoverBudgetCheck struct {
	Name     string                 `json:"name"`
	Passed   bool                   `json:"passed"`
	Actual   interface{}            `json:"actual,omitempty"`
	Expected interface{}            `json:"expected,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

type SkillCutoverBudgetReport struct {
	TargetEvalRunID string                    `json:"target_eval_run_id"`
	BaseEvalRunID   string                    `json:"base_eval_run_id,omitempty"`
	BaselineID      string                    `json:"baseline_id,omitempty"`
	Metrics         SkillCutoverBudgetMetrics `json:"metrics"`
	Thresholds      map[string]interface{}    `json:"thresholds,omitempty"`
	Checks          []SkillCutoverBudgetCheck `json:"checks,omitempty"`
	Passed          bool                      `json:"passed"`
	CreatedAt       time.Time                 `json:"created_at"`
}

type normalizedSkillCutoverBudgetThresholds struct {
	minMedianSchemaByteReductionRate float64
	maxMedianLatencyIncreaseRate     float64
	allowedFinalNativeTools          []string
}

type skillCutoverBudgetCaseSnapshot struct {
	Key         string
	Label       string
	RunID       string
	ToolNames   []string
	ToolCount   int
	SchemaBytes int
	LatencyMs   float64
	HasSurface  bool
}

func (c *Controller) EvaluateSkillCutoverBudgetGate(ctx context.Context, targetEvalRunID string, req SkillCutoverBudgetRequest) (*SkillCutoverBudgetReport, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}

	targetReport, err := c.GetEvalRunReport(ctx, strings.TrimSpace(targetEvalRunID))
	if err != nil {
		return nil, err
	}
	if targetReport == nil || targetReport.EvalRun == nil || targetReport.GroupReport == nil {
		return nil, fmt.Errorf("target eval run report is incomplete")
	}

	baseEvalRun, baseline, err := c.resolveComparisonBase(ctx, targetReport.EvalRun, CompareEvalRunRequest{
		BaseEvalRunID: strings.TrimSpace(req.BaseEvalRunID),
		BaselineID:    strings.TrimSpace(req.BaselineID),
	})
	if err != nil {
		return nil, err
	}

	baseReport, err := c.GetEvalRunReport(ctx, baseEvalRun.ID)
	if err != nil {
		return nil, err
	}
	if baseReport == nil || baseReport.EvalRun == nil || baseReport.GroupReport == nil {
		return nil, fmt.Errorf("base eval run report is incomplete")
	}

	thresholds := normalizeSkillCutoverBudgetThresholds(req.Thresholds)
	metrics := buildSkillCutoverBudgetMetrics(baseReport, targetReport, thresholds.allowedFinalNativeTools)
	checks := evaluateSkillCutoverBudgetChecks(metrics, thresholds)

	return &SkillCutoverBudgetReport{
		TargetEvalRunID: targetReport.EvalRun.ID,
		BaseEvalRunID:   strings.TrimSpace(baseEvalRun.ID),
		BaselineID:      baselineID(baseline),
		Metrics:         metrics,
		Thresholds: map[string]interface{}{
			"min_median_schema_byte_reduction_rate": thresholds.minMedianSchemaByteReductionRate,
			"max_median_latency_increase_rate":      thresholds.maxMedianLatencyIncreaseRate,
			"allowed_final_native_tools":            append([]string(nil), thresholds.allowedFinalNativeTools...),
		},
		Checks:    checks,
		Passed:    skillCutoverBudgetChecksPassed(checks),
		CreatedAt: timeutil.NowTime(),
	}, nil
}

func normalizeSkillCutoverBudgetThresholds(raw SkillCutoverBudgetThresholds) normalizedSkillCutoverBudgetThresholds {
	normalized := normalizedSkillCutoverBudgetThresholds{
		minMedianSchemaByteReductionRate: defaultSkillCutoverMinMedianSchemaByteReductionRate,
		maxMedianLatencyIncreaseRate:     defaultSkillCutoverMaxMedianLatencyIncreaseRate,
		allowedFinalNativeTools:          append([]string(nil), defaultSkillCutoverAllowedFinalNativeTools...),
	}
	if raw.MinMedianSchemaByteReductionRate != nil {
		normalized.minMedianSchemaByteReductionRate = *raw.MinMedianSchemaByteReductionRate
	}
	if raw.MaxMedianLatencyIncreaseRate != nil {
		normalized.maxMedianLatencyIncreaseRate = *raw.MaxMedianLatencyIncreaseRate
	}
	if len(raw.AllowedFinalNativeTools) > 0 {
		normalized.allowedFinalNativeTools = normalizeSkillCutoverToolNames(raw.AllowedFinalNativeTools)
	}
	return normalized
}

func buildSkillCutoverBudgetMetrics(baseReport *EvalRunReport, targetReport *EvalRunReport, allowedFinalNativeTools []string) SkillCutoverBudgetMetrics {
	metrics := SkillCutoverBudgetMetrics{
		AllowedFinalNativeTools: append([]string(nil), allowedFinalNativeTools...),
	}
	baseCases := buildSkillCutoverBudgetCaseIndex(baseReport, false)
	targetCases := buildSkillCutoverBudgetCaseIndex(targetReport, true)
	metrics.CaseCount = len(targetCases)

	baseToolCounts := make([]float64, 0, len(targetCases))
	targetToolCounts := make([]float64, 0, len(targetCases))
	baseSchemaBytes := make([]float64, 0, len(targetCases))
	targetSchemaBytes := make([]float64, 0, len(targetCases))
	baseLatencies := make([]float64, 0, len(targetCases))
	targetLatencies := make([]float64, 0, len(targetCases))

	allowedSet := make(map[string]struct{}, len(allowedFinalNativeTools))
	for _, name := range allowedFinalNativeTools {
		if name = strings.TrimSpace(name); name != "" {
			allowedSet[strings.ToLower(name)] = struct{}{}
		}
	}

	keys := make([]string, 0, len(targetCases))
	for key := range targetCases {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		target := targetCases[key]
		if skillCutoverToolsAllowed(target.ToolNames, allowedSet) {
			metrics.AllowedFinalNativeToolCases++
		} else {
			metrics.NonAllowedNativeToolCaseCount++
		}

		base, ok := baseCases[key]
		if !ok || !base.HasSurface || !target.HasSurface {
			metrics.MissingSurfaceCaseCount++
			continue
		}

		metrics.ComparableCaseCount++
		baseToolCounts = append(baseToolCounts, float64(base.ToolCount))
		targetToolCounts = append(targetToolCounts, float64(target.ToolCount))
		baseSchemaBytes = append(baseSchemaBytes, float64(base.SchemaBytes))
		targetSchemaBytes = append(targetSchemaBytes, float64(target.SchemaBytes))
		baseLatencies = append(baseLatencies, base.LatencyMs)
		targetLatencies = append(targetLatencies, target.LatencyMs)
	}

	metrics.BaseMedianToolCount = skillCutoverMedian(baseToolCounts)
	metrics.TargetMedianToolCount = skillCutoverMedian(targetToolCounts)
	metrics.BaseMedianSchemaBytes = skillCutoverMedian(baseSchemaBytes)
	metrics.TargetMedianSchemaBytes = skillCutoverMedian(targetSchemaBytes)
	metrics.MedianSchemaByteReductionRate = skillCutoverReductionRate(metrics.BaseMedianSchemaBytes, metrics.TargetMedianSchemaBytes)
	metrics.BaseMedianLatencyMs = skillCutoverMedian(baseLatencies)
	metrics.TargetMedianLatencyMs = skillCutoverMedian(targetLatencies)
	metrics.MedianLatencyIncreaseRate = skillCutoverIncreaseRate(metrics.BaseMedianLatencyMs, metrics.TargetMedianLatencyMs)
	if metrics.CaseCount > 0 {
		metrics.AllowedFinalNativeToolCaseRate = float64(metrics.AllowedFinalNativeToolCases) / float64(metrics.CaseCount)
	}

	return metrics
}

func evaluateSkillCutoverBudgetChecks(metrics SkillCutoverBudgetMetrics, thresholds normalizedSkillCutoverBudgetThresholds) []SkillCutoverBudgetCheck {
	checks := []SkillCutoverBudgetCheck{
		{
			Name:     "surface_metrics_coverage",
			Passed:   metrics.CaseCount > 0 && metrics.ComparableCaseCount == metrics.CaseCount,
			Actual:   metrics.ComparableCaseCount,
			Expected: metrics.CaseCount,
		},
		{
			Name:     "median_schema_byte_reduction_rate",
			Passed:   metrics.MedianSchemaByteReductionRate >= thresholds.minMedianSchemaByteReductionRate,
			Actual:   metrics.MedianSchemaByteReductionRate,
			Expected: thresholds.minMedianSchemaByteReductionRate,
		},
		{
			Name:     "median_latency_increase_rate",
			Passed:   metrics.MedianLatencyIncreaseRate <= thresholds.maxMedianLatencyIncreaseRate,
			Actual:   metrics.MedianLatencyIncreaseRate,
			Expected: thresholds.maxMedianLatencyIncreaseRate,
		},
		{
			Name:     "final_native_tool_surface",
			Passed:   metrics.NonAllowedNativeToolCaseCount == 0,
			Actual:   metrics.NonAllowedNativeToolCaseCount,
			Expected: 0,
			Details: map[string]interface{}{
				"allowed_final_native_tools": append([]string(nil), thresholds.allowedFinalNativeTools...),
			},
		},
	}
	return checks
}

func skillCutoverBudgetChecksPassed(checks []SkillCutoverBudgetCheck) bool {
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}

func buildSkillCutoverBudgetCaseIndex(report *EvalRunReport, preferNativeSurface bool) map[string]skillCutoverBudgetCaseSnapshot {
	out := make(map[string]skillCutoverBudgetCaseSnapshot)
	if report == nil || report.GroupReport == nil {
		return out
	}

	runByID := make(map[string]Run, len(report.GroupReport.LinkedRuns))
	for _, run := range report.GroupReport.LinkedRuns {
		runByID[run.ID] = run
	}

	for _, item := range report.GroupReport.Items {
		key := comparisonCaseKey(item)
		snapshot := skillCutoverBudgetCaseSnapshot{
			Key:   key,
			Label: comparisonCaseLabel(item, key),
		}

		if run, ok := runByID[strings.TrimSpace(item.LatestRunID)]; ok {
			snapshot.RunID = run.ID
			snapshot.LatencyMs = skillCutoverRunLatencyMs(&run)
			structured := structuredRunResult(&run)
			toolNames := decodeSkillCutoverToolNames(structured, preferNativeSurface)
			snapshot.ToolNames = normalizeSkillCutoverToolNames(toolNames)
			surface := decodeSkillCutoverToolSurface(structured, preferNativeSurface)
			if len(surface) > 0 {
				if toolCount, ok := skillCutoverIntValue(surface["tool_count"]); ok {
					snapshot.ToolCount = toolCount
				}
				if schemaBytes, ok := skillCutoverIntValue(surface["schema_bytes"]); ok {
					snapshot.SchemaBytes = schemaBytes
					snapshot.HasSurface = true
				}
			}
		}
		if snapshot.ToolCount == 0 && len(snapshot.ToolNames) > 0 {
			snapshot.ToolCount = len(snapshot.ToolNames)
		}
		out[snapshot.Key] = snapshot
	}

	return out
}

func decodeSkillCutoverToolNames(structured map[string]interface{}, preferNativeSurface bool) []string {
	if len(structured) == 0 {
		return nil
	}
	if preferNativeSurface {
		if _, ok := structured["selected_native_tools"]; ok {
			return decodeStringSlice(structured["selected_native_tools"])
		}
		return decodeStringSlice(structured["selected_tools"])
	}
	if _, ok := structured["selected_tools"]; ok {
		return decodeStringSlice(structured["selected_tools"])
	}
	return decodeStringSlice(structured["selected_native_tools"])
}

func decodeSkillCutoverToolSurface(structured map[string]interface{}, preferNativeSurface bool) map[string]interface{} {
	if len(structured) == 0 {
		return nil
	}
	if preferNativeSurface {
		if _, ok := structured["selected_native_tool_surface"]; ok {
			return nestedMetadataMap(structured, "selected_native_tool_surface")
		}
		return nestedMetadataMap(structured, "selected_tool_surface")
	}
	if _, ok := structured["selected_tool_surface"]; ok {
		return nestedMetadataMap(structured, "selected_tool_surface")
	}
	return nestedMetadataMap(structured, "selected_native_tool_surface")
}

func skillCutoverRunLatencyMs(run *Run) float64 {
	if run == nil || run.StartedAt == nil || run.FinishedAt == nil {
		return 0
	}
	return float64(run.FinishedAt.Sub(*run.StartedAt)) / float64(time.Millisecond)
}

func skillCutoverIntValue(raw interface{}) (int, bool) {
	switch typed := raw.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func skillCutoverMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func skillCutoverReductionRate(base, target float64) float64 {
	switch {
	case base <= 0 && target <= 0:
		return 1
	case base <= 0:
		return 0
	default:
		return (base - target) / base
	}
}

func skillCutoverIncreaseRate(base, target float64) float64 {
	switch {
	case base <= 0 && target <= 0:
		return 0
	case base < skillCutoverLatencyNoiseFloorMs && target < skillCutoverLatencyNoiseFloorMs:
		return 0
	case base < skillCutoverLatencyNoiseFloorMs:
		return (target - skillCutoverLatencyNoiseFloorMs) / skillCutoverLatencyNoiseFloorMs
	default:
		return (target - base) / base
	}
}

func skillCutoverToolsAllowed(toolNames []string, allowedSet map[string]struct{}) bool {
	if len(toolNames) == 0 {
		return true
	}
	for _, name := range toolNames {
		if _, ok := allowedSet[strings.ToLower(strings.TrimSpace(name))]; !ok {
			return false
		}
	}
	return true
}

func normalizeSkillCutoverToolNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
