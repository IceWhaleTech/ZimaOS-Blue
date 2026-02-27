package builtin

import (
	"testing"
)

func TestRegisterAll(t *testing.T) {
	// Verify the built-in skills can be created
	skills := []interface{}{
		NewAsk(),
		NewPlanCreate(),
		NewPlanUpdate(),
		NewPlanAppend(),
		NewReminder(),
		NewScheduler(),
		NewWorkflows(),
		NewSandbox(),
		NewBrowser(),
		NewAnalyze(),
		NewDeepSearch(),
		NewWebSearch(),
		NewUIReviewer(),
	}

	expected := GetSkillCount()
	if len(skills) != expected {
		t.Errorf("expected %d built-in skills, got %d", expected, len(skills))
	}
}
