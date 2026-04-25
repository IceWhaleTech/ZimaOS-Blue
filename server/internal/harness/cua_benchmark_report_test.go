package harness

import "testing"

func TestEvaluateCUABenchmarkReportComputesRatesAndLatency(t *testing.T) {
	report := EvaluateCUABenchmarkReport([]CUABenchmarkCaseResult{
		{ID: "chat-send", Passed: true, Critical: true, VerificationBacked: true, LatencyMS: 1000},
		{ID: "settings", Passed: false, Critical: true, FailureLabel: "unsafe_send", LatencyMS: 3000},
		{ID: "browser", Passed: true, VerificationBacked: false, FailureLabel: "typed_body_into_search_field", LatencyMS: 2000},
	})

	if report.CaseCount != 3 || report.PassedCount != 2 {
		t.Fatalf("counts = %d/%d, want 3/2", report.CaseCount, report.PassedCount)
	}
	if report.PassRate != 2.0/3.0 {
		t.Fatalf("PassRate = %v, want 2/3", report.PassRate)
	}
	if report.CriticalPassRate != 0.5 {
		t.Fatalf("CriticalPassRate = %v, want 0.5", report.CriticalPassRate)
	}
	if report.UnsafeSendCount != 1 || report.TypedBodyIntoSearchFieldCount != 1 {
		t.Fatalf("failure counts = unsafe %d typed %d", report.UnsafeSendCount, report.TypedBodyIntoSearchFieldCount)
	}
	if report.VerificationBackedRate != 0.5 {
		t.Fatalf("VerificationBackedRate = %v, want 0.5", report.VerificationBackedRate)
	}
	if report.LatencyP50MS != 2000 || report.LatencyP90MS != 3000 {
		t.Fatalf("latency = p50 %d p90 %d, want 2000/3000", report.LatencyP50MS, report.LatencyP90MS)
	}
	if report.FailureLabelCounts["unsafe_send"] != 1 {
		t.Fatalf("FailureLabelCounts = %#v", report.FailureLabelCounts)
	}
}
