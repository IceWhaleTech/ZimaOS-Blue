package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func toolResearchReport(report *deepresearch.Report) map[string]interface{} {
	if report == nil {
		return nil
	}
	out := map[string]interface{}{
		"answer":               report.Answer,
		"confidence":           report.Confidence,
		"citations":            append([]deepresearch.Citation(nil), report.Citations...),
		"open_questions":       append([]string(nil), report.OpenQuestions...),
		"support_count":        report.SupportCount,
		"conflict_count":       report.ConflictCount,
		"has_conflict":         report.HasConflict,
		"iterations":           report.Iterations,
		"stop_reason":          report.StopReason,
		"citation_coverage":    report.CitationCoverage,
		"stage_errors":         append([]string(nil), report.StageErrors...),
		"timeline_sections":    append([]deepresearch.TimelineSection(nil), report.TimelineSections...),
		"research_trace":       append([]deepresearch.ResearchTraceEntry(nil), report.ResearchTrace...),
		"verification_summary": report.VerificationSummary,
		"calibration":          cloneCalibrationForTool(report.Calibration),
		"items_by_source":      cloneToolReportItemsBySource(report.ItemsBySource),
		"errors_by_source":     cloneToolReportErrorsBySource(report.ErrorsBySource),
		"clusters":             cloneToolReportClusters(report.Clusters),
		"lookback_days":        report.LookbackDays,
		"browser_assisted":     report.BrowserAssisted,
		"retrieval_profile":    report.RetrievalProfile,
	}
	if report.Calibration != nil {
		out["takeaway_candidates"] = append([]deepresearch.TakeawayCandidate(nil), report.Calibration.TakeawayCandidates...)
	}
	return out
}

func cloneToolReportItemsBySource(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneToolReportErrorsBySource(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneToolReportClusters(in []map[string]interface{}) []map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, len(in))
	for i := range in {
		out[i] = cloneToolReportItemsBySource(in[i])
	}
	return out
}

func toolResearchJobWithRunMetadata(job *tools.ResearchJob, run *harness.Run) *tools.ResearchJob {
	if job == nil || run == nil || run.Metadata == nil || job.RetrievalProfile != "" {
		return job
	}
	if value, ok := run.Metadata["retrieval_profile"].(string); ok {
		job.RetrievalProfile = value
	}
	return job
}

func toolResearchRequestMode(req tools.ResearchCreateJobRequest) string {
	if req.ResearchDepth != "" {
		return req.ResearchDepth
	}
	return req.Mode
}
