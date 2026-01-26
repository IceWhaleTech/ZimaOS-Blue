// Package builtin provides built-in skills for the skill hub.
package builtin

import (
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
)

// RegisterAll registers all built-in skills with the registry
func RegisterAll(registry *skill.Registry) error {
	return RegisterAllWithConfig(registry, nil)
}

// RegisterAllWithConfig registers all built-in skills with optional configuration
func RegisterAllWithConfig(registry *skill.Registry, weatherConfig *WeatherConfig) error {
	skills := []skill.Skill{
		NewCalculator(),
		NewSystemInfo(),
		NewDateTime(),
		NewWeather(weatherConfig),
		NewSearch(),
	}

	for _, s := range skills {
		if err := registry.Register(s, true); err != nil {
			return err
		}
	}

	return nil
}
