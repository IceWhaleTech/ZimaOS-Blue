package deepresearch

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	calibrationConflictRiskLow      = "low"
	calibrationConflictRiskMedium   = "medium"
	calibrationConflictRiskBlocking = "blocking"

	calibrationActionPublish      = "publish"
	calibrationActionCaution      = "caution"
	calibrationActionInsufficient = "insufficient"

	calibrationTakeawayTargetFile = "AGENTS.md"
)

func buildCalibration(
	evidence []Evidence,
	reportConfidence float64,
	citationCoverage float64,
	supportCount int,
	conflictCount int,
	hasConflict bool,
	verificationSummary *VerificationSummary,
	stageErrors []string,
) *Calibration {
	coverage := clampCalibrationScore(citationCoverage)
	groundedness := groundednessScore(evidence)
	freshness := freshnessScore(evidence)
	conflictRisk := calibrationConflictRiskLow
	switch {
	case hasConflict && (conflictCount >= maxInt(1, supportCount) || conflictCount >= 2):
		conflictRisk = calibrationConflictRiskBlocking
	case hasConflict || conflictCount > 0:
		conflictRisk = calibrationConflictRiskMedium
	}

	confidence := clampCalibrationScore(
		coverage*0.35 +
			groundedness*0.30 +
			freshness*0.15 +
			clampCalibrationScore(reportConfidence)*0.20,
	)
	switch conflictRisk {
	case calibrationConflictRiskMedium:
		confidence = clampCalibrationScore(confidence - 0.08)
	case calibrationConflictRiskBlocking:
		confidence = clampCalibrationScore(confidence - 0.28)
	}
	if len(stageErrors) > 0 {
		confidence = clampCalibrationScore(confidence - minFloat(0.12, float64(len(stageErrors))*0.04))
	}

	recommendedAction := calibrationActionPublish
	switch {
	case len(evidence) == 0:
		recommendedAction = calibrationActionInsufficient
	case conflictRisk == calibrationConflictRiskBlocking:
		recommendedAction = calibrationActionInsufficient
	case coverage < 0.55 || groundedness < 0.45 || confidence < 0.45:
		recommendedAction = calibrationActionInsufficient
	case coverage < 0.8 || freshness < 0.55 || len(stageErrors) > 0:
		recommendedAction = calibrationActionCaution
	case verificationSummary != nil && verificationSummary.InsufficientCount > 0:
		recommendedAction = calibrationActionCaution
	}

	calibration := &Calibration{
		Coverage:          coverage,
		Groundedness:      groundedness,
		Freshness:         freshness,
		ConflictRisk:      conflictRisk,
		Confidence:        confidence,
		RecommendedAction: recommendedAction,
	}
	calibration.TakeawayCandidates = buildTakeawayCandidates(
		calibration,
		evidence,
		supportCount,
		conflictCount,
		verificationSummary,
		stageErrors,
	)
	return calibration
}

func groundednessScore(evidence []Evidence) float64 {
	if len(evidence) == 0 {
		return 0
	}
	totalWeight := 0.0
	weighted := 0.0
	for _, ev := range evidence {
		weight := maxFloat(0.1, ev.RelevanceScore)
		score := clampCalibrationScore(ev.CredibilityScore*0.75 + boolScore(ev.Quote != "")*0.15 + boolScore(ev.Snippet != "")*0.10)
		totalWeight += weight
		weighted += score * weight
	}
	if totalWeight <= 0 {
		return 0
	}
	return clampCalibrationScore(weighted / totalWeight)
}

func freshnessScore(evidence []Evidence) float64 {
	if len(evidence) == 0 {
		return 0
	}
	now := timeutil.NowTime()
	totalWeight := 0.0
	weighted := 0.0
	for _, ev := range evidence {
		weight := maxFloat(0.1, ev.RelevanceScore)
		sourceTime := ev.FetchedAt
		if ev.PublishedAt != nil && !ev.PublishedAt.IsZero() {
			sourceTime = *ev.PublishedAt
		}
		score := 0.55
		if !sourceTime.IsZero() {
			age := now.Sub(sourceTime)
			switch {
			case age <= 30*24*time.Hour:
				score = 1.0
			case age <= 90*24*time.Hour:
				score = 0.85
			case age <= 365*24*time.Hour:
				score = 0.65
			case age <= 2*365*24*time.Hour:
				score = 0.45
			default:
				score = 0.25
			}
		}
		totalWeight += weight
		weighted += score * weight
	}
	if totalWeight <= 0 {
		return 0
	}
	return clampCalibrationScore(weighted / totalWeight)
}

func buildTakeawayCandidates(
	calibration *Calibration,
	evidence []Evidence,
	supportCount int,
	conflictCount int,
	verificationSummary *VerificationSummary,
	stageErrors []string,
) []TakeawayCandidate {
	if calibration == nil {
		return nil
	}
	candidates := make([]TakeawayCandidate, 0, 4)
	evidenceIDs := topCalibrationEvidenceIDs(evidence, 3)
	addCandidate := func(lesson string, when string, details string, confidence float64) {
		lesson = strings.TrimSpace(lesson)
		when = strings.TrimSpace(when)
		details = strings.TrimSpace(details)
		if lesson == "" || details == "" {
			return
		}
		candidates = append(candidates, TakeawayCandidate{
			Lesson:      lesson,
			WhenToApply: when,
			Evidence:    details,
			EvidenceIDs: append([]string(nil), evidenceIDs...),
			Confidence:  clampCalibrationScore(confidence),
			TargetFile:  calibrationTakeawayTargetFile,
		})
	}

	if calibration.Coverage < 0.8 {
		addCandidate(
			"Keep the final conclusion explicitly cautious when citation coverage stays below 80%.",
			"When only part of the supporting evidence is directly cited in the final report.",
			fmt.Sprintf("Citation coverage was %.0f%% across %d collected source(s).", calibration.Coverage*100, len(evidence)),
			maxFloat(0.62, calibration.Confidence),
		)
	}
	if calibration.ConflictRisk == calibrationConflictRiskMedium || calibration.ConflictRisk == calibrationConflictRiskBlocking {
		addCandidate(
			"Do not collapse conflicting sources into a single definitive answer before the disagreement is bounded.",
			"When multiple sources disagree on the same claim or timeline.",
			fmt.Sprintf("Conflict risk was %s with %d conflicting signal(s) against %d supporting source(s).", calibration.ConflictRisk, conflictCount, supportCount),
			maxFloat(0.72, calibration.Confidence),
		)
	}
	if calibration.Freshness < 0.65 {
		addCandidate(
			"Call out freshness limits instead of presenting the answer as current when most support is old or undated.",
			"When the source set is materially older than the user’s recency needs.",
			fmt.Sprintf("Freshness scored %.0f%% across %d source(s).", calibration.Freshness*100, len(evidence)),
			maxFloat(0.6, calibration.Confidence),
		)
	}
	if verificationSummary != nil && verificationSummary.InsufficientCount > 0 {
		addCandidate(
			"Carry unresolved verification gaps into the final report instead of filling them with uncited inference.",
			"When verification still marks part of the answer as insufficient.",
			fmt.Sprintf("Verification left %d insufficient item(s) unresolved.", verificationSummary.InsufficientCount),
			maxFloat(0.7, calibration.Confidence),
		)
	}
	if len(candidates) == 0 && calibration.RecommendedAction == calibrationActionPublish {
		addCandidate(
			"Prefer a concise citation-backed summary over uncited synthesis when coverage and grounding are both strong.",
			"When the source set is broad enough to support a clean publish-ready answer.",
			fmt.Sprintf("Coverage %.0f%%, groundedness %.0f%%, freshness %.0f%% across %d source(s).", calibration.Coverage*100, calibration.Groundedness*100, calibration.Freshness*100, len(evidence)),
			maxFloat(0.75, calibration.Confidence),
		)
	}
	if len(stageErrors) > 0 && len(candidates) < 3 {
		addCandidate(
			"Expose stage-level research failures in the report whenever retrieval or verification had material gaps.",
			"When the pipeline reports stage errors that could change how much the user should trust the result.",
			fmt.Sprintf("The run recorded %d stage error(s): %s", len(stageErrors), strings.Join(dedupeStrings(stageErrors), "; ")),
			maxFloat(0.58, calibration.Confidence),
		)
	}

	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates
}

func topCalibrationEvidenceIDs(evidence []Evidence, limit int) []string {
	if len(evidence) == 0 || limit <= 0 {
		return nil
	}
	copied := append([]Evidence(nil), evidence...)
	sort.SliceStable(copied, func(i, j int) bool {
		left := copied[i].RelevanceScore*0.6 + copied[i].CredibilityScore*0.4
		right := copied[j].RelevanceScore*0.6 + copied[j].CredibilityScore*0.4
		return left > right
	})
	if len(copied) > limit {
		copied = copied[:limit]
	}
	out := make([]string, 0, len(copied))
	for _, ev := range copied {
		if strings.TrimSpace(ev.ID) != "" {
			out = append(out, ev.ID)
		}
	}
	return out
}

func clampCalibrationScore(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func boolScore(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}
