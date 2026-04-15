package config

import "time"

type RuntimeReflectionConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	IntervalToolFinishes int           `yaml:"interval_tool_finishes" json:"interval_tool_finishes"`
	MinReviewGap         time.Duration `yaml:"min_review_gap" json:"min_review_gap"`
	RepeatWindow         int           `yaml:"repeat_window" json:"repeat_window"`
	MaxEvidenceEvents    int           `yaml:"max_evidence_events" json:"max_evidence_events"`
}

type HarnessConfig struct {
	Enabled              bool                    `yaml:"enabled" json:"enabled"`
	StorePath            string                  `yaml:"store_path" json:"store_path"`
	ArtifactRoot         string                  `yaml:"artifact_root" json:"artifact_root"`
	EventRetentionDays   int                     `yaml:"event_retention_days" json:"event_retention_days"`
	DefaultApprovalMode  string                  `yaml:"default_approval_mode" json:"default_approval_mode"`
	DefaultSandboxMode   string                  `yaml:"default_sandbox_mode" json:"default_sandbox_mode"`
	DefaultMaxDuration   time.Duration           `yaml:"default_max_duration" json:"default_max_duration"`
	DefaultMaxSteps      int                     `yaml:"default_max_steps" json:"default_max_steps"`
	DefaultMaxToolRounds int                     `yaml:"default_max_tool_rounds" json:"default_max_tool_rounds"`
	RuntimeReflection    RuntimeReflectionConfig `yaml:"runtime_reflection" json:"runtime_reflection"`
}

func DefaultHarnessConfig() *HarnessConfig {
	return &HarnessConfig{
		Enabled:              true,
		StorePath:            "./data/blue.db",
		ArtifactRoot:         "./data/harness/artifacts",
		EventRetentionDays:   30,
		DefaultApprovalMode:  "ask",
		DefaultSandboxMode:   "inherit",
		DefaultMaxDuration:   30 * time.Minute,
		DefaultMaxSteps:      32,
		DefaultMaxToolRounds: 50,
		RuntimeReflection: RuntimeReflectionConfig{
			Enabled:              true,
			IntervalToolFinishes: 10,
			MinReviewGap:         30 * time.Second,
			RepeatWindow:         8,
			MaxEvidenceEvents:    8,
		},
	}
}
