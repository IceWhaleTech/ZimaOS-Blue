// Package builtin provides built-in skills for the skill hub.
package builtin

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// RegisterAll registers all built-in skills with the registry
func RegisterAll(registry *skill.Registry) error {
	skills := []skill.Skill{
		NewAsk(),
		NewPlanCreate(),
		NewPlanUpdate(),
		NewPlanAppend(),
		NewSelfReflect(),
		NewReminder(),
		NewScheduler(),
		NewEmail(),
		NewCalendar(),
		NewBrowser(),
		NewAnalyze(),
		NewDeepResearch(),
		NewWebQuery(),
		NewWebSearch(),
		NewUIReviewer(),
		NewHumanizer(),
	}

	for _, s := range skills {
		if err := registry.Register(s, true); err != nil {
			return err
		}
	}

	return nil
}

// GetSkillCount returns the number of built-in skills
func GetSkillCount() int {
	return 16
}
