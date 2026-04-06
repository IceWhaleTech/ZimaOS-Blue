package tools

import (
	"sync"
	"testing"
)

func resetRiskRulesForTest(t *testing.T) {
	t.Helper()

	originalRiskRules := riskRules

	riskRules = nil
	riskRulesOnce = sync.Once{}

	t.Cleanup(func() {
		riskRules = originalRiskRules
		riskRulesOnce = sync.Once{}
		if len(originalRiskRules) > 0 {
			riskRulesOnce.Do(func() {})
		}
	})
}

func TestAnalyzeRisk_InitializesRiskRulesOnDemand(t *testing.T) {
	resetRiskRulesForTest(t)

	if riskRules != nil {
		t.Fatal("expected risk rules to start uninitialized")
	}

	analysis := AnalyzeRisk("rm -rf /tmp/demo")
	if analysis.Total == 0 {
		t.Fatal("AnalyzeRisk() = zero score, want destructive command to score after lazy init")
	}
	if len(riskRules) == 0 {
		t.Fatal("expected risk rules to initialize on first analysis")
	}
}
