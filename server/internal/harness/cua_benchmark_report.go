package harness

import "sort"

type CUABenchmarkCaseResult struct {
	ID                 string `json:"id,omitempty"`
	Passed             bool   `json:"passed"`
	Critical           bool   `json:"critical,omitempty"`
	VerificationBacked bool   `json:"verification_backed,omitempty"`
	FailureLabel       string `json:"failure_label,omitempty"`
	LatencyMS          int    `json:"latency_ms,omitempty"`
}

type CUABenchmarkReport struct {
	CaseCount                     int            `json:"case_count"`
	PassedCount                   int            `json:"passed_count"`
	CriticalCount                 int            `json:"critical_count"`
	CriticalPassedCount           int            `json:"critical_passed_count"`
	PassRate                      float64        `json:"pass_rate"`
	CriticalPassRate              float64        `json:"critical_pass_rate"`
	UnsafeSendCount               int            `json:"unsafe_send_count"`
	TypedBodyIntoSearchFieldCount int            `json:"typed_body_into_search_field_count"`
	VerificationBackedRate        float64        `json:"verification_backed_rate"`
	LatencyP50MS                  int            `json:"latency_p50_ms"`
	LatencyP90MS                  int            `json:"latency_p90_ms"`
	FailureLabelCounts            map[string]int `json:"failure_label_counts,omitempty"`
}

func EvaluateCUABenchmarkReport(results []CUABenchmarkCaseResult) CUABenchmarkReport {
	report := CUABenchmarkReport{
		CaseCount:          len(results),
		FailureLabelCounts: make(map[string]int),
	}
	latencies := make([]int, 0, len(results))
	verificationEligible := 0
	verificationBacked := 0
	for _, result := range results {
		if result.Passed {
			report.PassedCount++
		}
		if result.Critical {
			report.CriticalCount++
			if result.Passed {
				report.CriticalPassedCount++
			}
		}
		if result.Passed {
			verificationEligible++
			if result.VerificationBacked {
				verificationBacked++
			}
		}
		if result.FailureLabel != "" {
			report.FailureLabelCounts[result.FailureLabel]++
		}
		if result.LatencyMS > 0 {
			latencies = append(latencies, result.LatencyMS)
		}
	}
	report.UnsafeSendCount = report.FailureLabelCounts["unsafe_send"] + report.FailureLabelCounts["send_in_wrong_conversation"]
	report.TypedBodyIntoSearchFieldCount = report.FailureLabelCounts["typed_body_into_search_field"] + report.FailureLabelCounts["body_typed_into_search_field"]
	if report.CaseCount > 0 {
		report.PassRate = float64(report.PassedCount) / float64(report.CaseCount)
	}
	if report.CriticalCount > 0 {
		report.CriticalPassRate = float64(report.CriticalPassedCount) / float64(report.CriticalCount)
	}
	if verificationEligible > 0 {
		report.VerificationBackedRate = float64(verificationBacked) / float64(verificationEligible)
	}
	report.LatencyP50MS = cuaPercentileLatency(latencies, 0.50)
	report.LatencyP90MS = cuaPercentileLatency(latencies, 0.90)
	if len(report.FailureLabelCounts) == 0 {
		report.FailureLabelCounts = nil
	}
	return report
}

func cuaPercentileLatency(values []int, percentile float64) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	if percentile <= 0 {
		return sorted[0]
	}
	if percentile >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(float64(len(sorted)-1)*percentile + 0.5)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
