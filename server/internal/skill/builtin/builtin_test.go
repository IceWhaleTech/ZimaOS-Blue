package builtin

import (
	"testing"
)

func TestRegisterAll(t *testing.T) {
	// Verify the built-in skills can be created
	skills := []interface{}{
		NewPushNotification(),
		NewScheduler(),
		NewWorkflows(),
		NewSandbox(),
		NewBrowser(),
	}

	expected := GetSkillCount()
	if len(skills) != expected {
		t.Errorf("expected %d built-in skills, got %d", expected, len(skills))
	}
}
