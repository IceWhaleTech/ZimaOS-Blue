package config

import "time"

// ResearchConfig controls deep-research routing and experiment backends.
type ResearchConfig struct {
	Router       ResearchRouterConfig       `yaml:"router" json:"router"`
	Autoresearch ResearchAutoresearchConfig `yaml:"autoresearch" json:"autoresearch"`
}

// ResearchRouterConfig controls route selection for deep research.
type ResearchRouterConfig struct {
	DefaultMode     string `yaml:"default_mode" json:"default_mode"`
	AllowExperiment bool   `yaml:"allow_experiment" json:"allow_experiment"`
	AllowHybrid     bool   `yaml:"allow_hybrid" json:"allow_hybrid"`
}

// ResearchAutoresearchConfig controls the external autoresearch backend wrapper.
type ResearchAutoresearchConfig struct {
	Enabled     bool              `yaml:"enabled" json:"enabled"`
	Command     string            `yaml:"command" json:"command"`
	Args        []string          `yaml:"args" json:"args,omitempty"`
	WorkingDir  string            `yaml:"working_dir" json:"working_dir,omitempty"`
	Timeout     time.Duration     `yaml:"timeout" json:"timeout"`
	ArtifactDir string            `yaml:"artifact_dir" json:"artifact_dir,omitempty"`
	Env         map[string]string `yaml:"env" json:"env,omitempty"`
}

// DefaultResearchConfig returns default deep-research routing config.
func DefaultResearchConfig() *ResearchConfig {
	return &ResearchConfig{
		Router: ResearchRouterConfig{
			DefaultMode:     "web",
			AllowExperiment: true,
			AllowHybrid:     true,
		},
		Autoresearch: ResearchAutoresearchConfig{
			Enabled:     false,
			Command:     "",
			Args:        nil,
			WorkingDir:  "",
			Timeout:     20 * time.Minute,
			ArtifactDir: "./data/deep-research/experiments",
			Env:         map[string]string{},
		},
	}
}
