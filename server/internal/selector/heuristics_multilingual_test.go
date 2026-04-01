package selector

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
)

func TestAnalyzeQuery_MultilingualDomainCoverage(t *testing.T) {
	checkDomain := func(skill string, want func(QueryIntentSignals) bool, label string) {
		t.Helper()
		for _, example := range routingcue.LocalizedExamples(skill) {
			signals := AnalyzeQuery(example.Query)
			if !want(signals) {
				t.Fatalf("%s locale=%s query=%q signals=%+v", label, example.Locale, example.Query, signals)
			}
		}
	}

	checkDomain("web_search", func(signals QueryIntentSignals) bool { return signals.LiveWeb }, "expected live web signal")
	checkDomain("workspace_local", func(signals QueryIntentSignals) bool { return signals.LocalWorkspace }, "expected local workspace signal")
	checkDomain("reminder", func(signals QueryIntentSignals) bool { return signals.Productivity }, "expected productivity signal")
	checkDomain("browser", func(signals QueryIntentSignals) bool { return signals.URLPresent && signals.LiveWeb }, "expected url/live web signal")
	checkDomain("ui_reviewer", func(signals QueryIntentSignals) bool { return signals.UIArtifact }, "expected ui artifact signal")
}
