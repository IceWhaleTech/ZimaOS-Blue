package config

import "time"

type SkillMarketConfig struct {
	Enabled                  bool          `yaml:"enabled"`
	SeedURLs                 []string      `yaml:"seed_urls"`
	ClawHubMirrorBaseURLs    []string      `yaml:"clawhub_mirror_base_urls"`
	SkillHubBaseURL          string        `yaml:"skillhub_base_url"`
	SkillHubAPIKey           string        `yaml:"skillhub_api_key"`
	SkillStackBaseURL        string        `yaml:"skillstack_base_url"`
	SkillsMPBaseURL          string        `yaml:"skillsmp_base_url"`
	SkillsMPAPIKey           string        `yaml:"skillsmp_api_key"`
	LLMSkillsBaseURL         string        `yaml:"llmskills_base_url"`
	CuratedConfigPath        string        `yaml:"curated_config_path"`
	CuratedConfigURLs        []string      `yaml:"curated_config_urls"`
	CrawlIncrementalInterval time.Duration `yaml:"crawl_incremental_interval"`
	CrawlFullInterval        time.Duration `yaml:"crawl_full_interval"`
	UpdateCheckInterval      time.Duration `yaml:"update_check_interval"`
	TelemetryRollupInterval  time.Duration `yaml:"telemetry_rollup_interval"`
	SemanticRatio            float64       `yaml:"semantic_ratio"`
	SearchCandidateLimit     int           `yaml:"search_candidate_limit"`
}

func DefaultSkillMarketConfig() *SkillMarketConfig {
	return &SkillMarketConfig{
		Enabled:               true,
		SeedURLs:              []string{"https://github.com/topics/claude-code", "https://github.com/topics/ai-agent"},
		ClawHubMirrorBaseURLs: nil,
		SkillHubBaseURL:       "https://www.skillhub.club",
		SkillStackBaseURL:     "https://www.skillstack.me",
		SkillsMPBaseURL:       "https://skillsmp.com",
		LLMSkillsBaseURL:      "https://llmskills.org",
		CuratedConfigPath:     "server/skillmarket_curated.yaml",
		CuratedConfigURLs: []string{
			"https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Blue/main/server/skillmarket_curated.yaml",
			"https://raw.gitmirror.com/IceWhaleTech/ZimaOS-Blue/main/server/skillmarket_curated.yaml",
			"https://cdn.jsdelivr.net/gh/IceWhaleTech/ZimaOS-Blue@main/server/skillmarket_curated.yaml",
			"https://ghproxy.com/https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Blue/main/server/skillmarket_curated.yaml",
		},
		CrawlIncrementalInterval: 24 * time.Hour,
		CrawlFullInterval:        24 * time.Hour,
		UpdateCheckInterval:      24 * time.Hour,
		TelemetryRollupInterval:  time.Hour,
		SemanticRatio:            0.35,
		SearchCandidateLimit:     100,
	}
}
