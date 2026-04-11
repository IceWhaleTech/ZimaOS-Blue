package config

import (
	"encoding/json"
	"time"

	"gopkg.in/yaml.v3"
)

var defaultSkillMarketDiscoveryPageURLs = []string{
	"https://github.com/topics/claude-code",
	"https://github.com/topics/ai-agent",
}

type SkillMarketConfig struct {
	Enabled                   bool          `yaml:"enabled"`
	DiscoveryPageURLs         []string      `yaml:"discovery_page_urls"`
	ClawHubMirrorBaseURLs     []string      `yaml:"clawhub_mirror_base_urls"`
	TencentSkillHubAPIBaseURL string        `yaml:"tencent_skillhub_api_base_url"`
	SkillHubBaseURL           string        `yaml:"skillhub_base_url"`
	SkillHubAPIKey            string        `yaml:"skillhub_api_key"`
	LLMSkillsBaseURL          string        `yaml:"llmskills_base_url"`
	CuratedConfigPath         string        `yaml:"curated_config_path"`
	CuratedConfigURLs         []string      `yaml:"curated_config_urls"`
	CrawlIncrementalInterval  time.Duration `yaml:"crawl_incremental_interval"`
	CrawlFullInterval         time.Duration `yaml:"crawl_full_interval"`
	UpdateCheckInterval       time.Duration `yaml:"update_check_interval"`
	TelemetryRollupInterval   time.Duration `yaml:"telemetry_rollup_interval"`
	SemanticRatio             float64       `yaml:"semantic_ratio"`
	SearchCandidateLimit      int           `yaml:"search_candidate_limit"`
}

func DefaultSkillMarketConfig() *SkillMarketConfig {
	return &SkillMarketConfig{
		Enabled:                   true,
		DiscoveryPageURLs:         append([]string(nil), defaultSkillMarketDiscoveryPageURLs...),
		ClawHubMirrorBaseURLs:     nil,
		TencentSkillHubAPIBaseURL: "https://lightmake.site",
		SkillHubBaseURL:           "https://www.skillhub.club",
		LLMSkillsBaseURL:          "https://llmskills.org",
		CuratedConfigPath:         "server/skillmarket_curated.yaml",
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

type skillMarketConfigAlias SkillMarketConfig

func hasYAMLMappingKey(node *yaml.Node, key string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

func hasJSONKey(data []byte, key string) (bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, err
	}
	_, ok := raw[key]
	return ok, nil
}

func (c *SkillMarketConfig) applyDiscoveryPageURLs(urls []string) {
	c.DiscoveryPageURLs = append([]string(nil), urls...)
}

func (c *SkillMarketConfig) applyLegacyDiscoveryPageURLs(legacySets ...[]string) {
	for _, legacy := range legacySets {
		if len(legacy) > 0 {
			c.applyDiscoveryPageURLs(legacy)
			return
		}
	}
}

func (c *SkillMarketConfig) UnmarshalYAML(value *yaml.Node) error {
	type yamlSkillMarketConfig struct {
		skillMarketConfigAlias `yaml:",inline"`
		LegacySeedURLs         []string `yaml:"seed_urls"`
	}

	aux := yamlSkillMarketConfig{
		skillMarketConfigAlias: skillMarketConfigAlias(*c),
	}
	if err := value.Decode(&aux); err != nil {
		return err
	}
	*c = SkillMarketConfig(aux.skillMarketConfigAlias)
	if !hasYAMLMappingKey(value, "discovery_page_urls") {
		c.applyLegacyDiscoveryPageURLs(aux.LegacySeedURLs)
	}
	return nil
}

func (c *SkillMarketConfig) UnmarshalJSON(data []byte) error {
	type jsonSkillMarketConfig struct {
		skillMarketConfigAlias
		DiscoveryPageURLsSnake []string `json:"discovery_page_urls"`
		LegacySeedURLsSnake    []string `json:"seed_urls"`
		LegacySeedURLs         []string `json:"SeedURLs"`
	}

	hasDiscoveryPageURLsSnake, err := hasJSONKey(data, "discovery_page_urls")
	if err != nil {
		return err
	}
	hasDiscoveryPageURLs, err := hasJSONKey(data, "DiscoveryPageURLs")
	if err != nil {
		return err
	}

	aux := jsonSkillMarketConfig{
		skillMarketConfigAlias: skillMarketConfigAlias(*c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*c = SkillMarketConfig(aux.skillMarketConfigAlias)
	if hasDiscoveryPageURLsSnake {
		c.applyDiscoveryPageURLs(aux.DiscoveryPageURLsSnake)
		return nil
	}
	if hasDiscoveryPageURLs {
		return nil
	}
	c.applyLegacyDiscoveryPageURLs(aux.LegacySeedURLsSnake, aux.LegacySeedURLs)
	return nil
}
