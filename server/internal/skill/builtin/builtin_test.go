package builtin

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

func TestRegisterAll(t *testing.T) {
	// Verify the built-in skills can be created
	skills := []interface{}{
		NewAsk(),
		NewPlanCreate(),
		NewPlanUpdate(),
		NewPlanAppend(),
		NewSelfReflect(),
		NewReminder(),
		NewScheduler(),
		NewEmail(),
		NewCalendar(),
		NewContacts(),
		NewBrowser(),
		NewAnalyze(),
		NewDeepResearch(),
		NewWebQuery(),
		NewUIReviewer(),
		NewHumanizer(),
	}

	expected := GetSkillCount()
	if len(skills) != expected {
		t.Errorf("expected %d built-in skills, got %d", expected, len(skills))
	}
}

func TestRegisterAll_RegistersCanonicalWebQueryOnly(t *testing.T) {
	registry := skill.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatalf("RegisterAll error: %v", err)
	}

	if registry.Get("web_query") == nil {
		t.Fatal("expected web_query builtin skill to be registered")
	}
	if registry.Get("web_search") != nil {
		t.Fatal("expected legacy web_search builtin skill to stay unregistered by default")
	}
	if registry.Get("contacts") == nil {
		t.Fatal("expected contacts builtin skill to be registered")
	}
}
