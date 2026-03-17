package skillmarket

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultPageSize    = 20
	DefaultSearchLimit = 100

	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"

	BadgeGreen  = "green"
	BadgeYellow = "yellow"
	BadgeRed    = "red"

	InstallTypeBuiltinCommands = "builtin_commands"
	InstallTypeRawSkill        = "raw_skill"
	InstallTypeGitRepo         = "git_repo"
	InstallTypeSourceArchive   = "source_archive"
	InstallTypeScriptPackage   = "script_package"
	InstallTypeBinaryPackage   = "binary_package"
	InstallTypeManualExternal  = "manual_external"

	ArtifactKindOpenSource   = "open_source"
	ArtifactKindClosedBinary = "closed_binary"
	ArtifactKindMixed        = "mixed"
	ArtifactKindUnknown      = "unknown"

	VulnerabilityStatusNone          = "none"
	VulnerabilityStatusUnknown       = "unknown"
	VulnerabilityStatusSuspected     = "suspected"
	VulnerabilityStatusDetected      = "detected"
	VulnerabilityStatusNotApplicable = "not_applicable"

	ScannerVersion = "skillmarket-detector-v2"
)

type Config struct {
	Enabled                  bool
	GitHubToken              string
	GitHubAPIBaseURL         string
	ClawHubBaseURL           string
	ClawHubMirrorBaseURLs    []string
	SkillHubBaseURL          string
	SkillHubAPIKey           string
	SkillStackBaseURL        string
	SkillsMPBaseURL          string
	SkillsMPAPIKey           string
	LLMSkillsBaseURL         string
	SeedURLs                 []string
	CrawlIncrementalInterval time.Duration
	CrawlFullInterval        time.Duration
	UpdateCheckInterval      time.Duration
	TelemetryRollupInterval  time.Duration
	SemanticRatio            float64
	SearchCandidateLimit     int
	CacheRoot                string
	ActiveSkillsDir          string
	CuratedConfigPath        string
	CuratedConfigURLs        []string
}

func DefaultConfig(dataDir, activeSkillsDir string) Config {
	home, _ := os.UserHomeDir()
	cacheRoot := filepath.Join(home, ".zima", "skills")
	if strings.TrimSpace(activeSkillsDir) == "" {
		activeSkillsDir = filepath.Join(dataDir, "workspace", ".claude", "skills")
	}
	return Config{
		Enabled:                  true,
		GitHubAPIBaseURL:         "https://api.github.com",
		ClawHubBaseURL:           "https://www.clawhub.ai",
		ClawHubMirrorBaseURLs:    nil,
		SkillHubBaseURL:          "https://www.skillhub.club",
		SkillStackBaseURL:        "https://www.skillstack.me",
		SkillsMPBaseURL:          "https://skillsmp.com",
		LLMSkillsBaseURL:         "https://llmskills.org",
		SeedURLs:                 []string{"https://github.com/topics/claude-code", "https://github.com/topics/ai-agent"},
		CrawlIncrementalInterval: 24 * time.Hour,
		CrawlFullInterval:        24 * time.Hour,
		UpdateCheckInterval:      24 * time.Hour,
		TelemetryRollupInterval:  time.Hour,
		SemanticRatio:            0.35,
		SearchCandidateLimit:     100,
		CacheRoot:                cacheRoot,
		ActiveSkillsDir:          activeSkillsDir,
		CuratedConfigPath:        "server/skillmarket_curated.yaml",
		CuratedConfigURLs: []string{
			"https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Blue/main/server/skillmarket_curated.yaml",
			"https://raw.gitmirror.com/IceWhaleTech/ZimaOS-Blue/main/server/skillmarket_curated.yaml",
			"https://cdn.jsdelivr.net/gh/IceWhaleTech/ZimaOS-Blue@main/server/skillmarket_curated.yaml",
			"https://ghproxy.com/https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Blue/main/server/skillmarket_curated.yaml",
		},
	}
}

type SkillDocument struct {
	ID                  string    `json:"id"`
	Slug                string    `json:"slug"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	Author              string    `json:"author,omitempty"`
	RepoURL             string    `json:"repo_url,omitempty"`
	Homepage            string    `json:"homepage,omitempty"`
	DownloadURL         string    `json:"download_url,omitempty"`
	Stars               int       `json:"stars"`
	Downloads           int       `json:"downloads"`
	Tags                []string  `json:"tags,omitempty"`
	Category            string    `json:"category,omitempty"`
	SecurityScore       int       `json:"security_score"`
	Permissions         []string  `json:"permissions,omitempty"`
	LatestVersion       string    `json:"latest_version,omitempty"`
	RiskLevel           string    `json:"risk_level"`
	SecurityBadge       string    `json:"security_badge"`
	Installable         bool      `json:"installable"`
	InstallType         string    `json:"install_type,omitempty"`
	ArtifactKind        string    `json:"artifact_kind,omitempty"`
	VulnerabilityStatus string    `json:"vulnerability_status"`
	HasVulnerabilities  bool      `json:"has_vulnerabilities"`
	HasPromptInjection  bool      `json:"has_prompt_injection"`
	HasShellInjection   bool      `json:"has_shell_injection"`
	HasDataExfiltration bool      `json:"has_data_exfiltration"`
	HasBinary           bool      `json:"has_binary"`
	HasScripts          bool      `json:"has_scripts"`
	PopularityScore     float64   `json:"popularity_score"`
	TrendingScore       float64   `json:"trending_score"`
	ScanStatus          string    `json:"scan_status,omitempty"`
	ContentSHA256       string    `json:"content_sha256,omitempty"`
	Published           bool      `json:"published"`
	SourceID            string    `json:"source_id,omitempty"`
	SourceName          string    `json:"source_name,omitempty"`
	SourceGroup         string    `json:"source_group,omitempty"`
	SourceType          string    `json:"source_type,omitempty"`
	SkillPath           string    `json:"skill_path,omitempty"`
	SkillContent        string    `json:"skill_content,omitempty"`
	EmbeddingJSON       string    `json:"embedding_json,omitempty"`
	EmbeddingModel      string    `json:"embedding_model,omitempty"`
	CuratedRank         int       `json:"curated_rank,omitempty"`
	CuratedBoost        float64   `json:"curated_boost,omitempty"`
	CuratedLabel        string    `json:"curated_label,omitempty"`
	CuratedReason       string    `json:"curated_reason,omitempty"`
	LastUpdated         time.Time `json:"last_updated"`
	LastCrawledAt       time.Time `json:"last_crawled_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type SkillVersion struct {
	ID           string    `json:"id"`
	SkillID      string    `json:"skill_id"`
	Version      string    `json:"version"`
	CommitHash   string    `json:"commit_hash,omitempty"`
	SourceURL    string    `json:"source_url,omitempty"`
	Checksum     string    `json:"checksum,omitempty"`
	SkillPath    string    `json:"skill_path,omitempty"`
	RawSkillMD   string    `json:"raw_skill_md,omitempty"`
	ManifestJSON string    `json:"manifest_json,omitempty"`
	ReleasedAt   time.Time `json:"released_at"`
	ScannedAt    time.Time `json:"scanned_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SecurityFinding struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Pattern    string `json:"pattern,omitempty"`
	Message    string `json:"message"`
	Command    string `json:"command,omitempty"`
	Permission string `json:"permission,omitempty"`
}

type SecurityEvidence struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
}

type SourceSecuritySignals struct {
	Score               *int               `json:"score,omitempty"`
	RiskLevel           string             `json:"risk_level,omitempty"`
	SecurityBadge       string             `json:"security_badge,omitempty"`
	VulnerabilityStatus string             `json:"vulnerability_status,omitempty"`
	Permissions         []string           `json:"permissions,omitempty"`
	Vulnerabilities     []string           `json:"vulnerabilities,omitempty"`
	HasVulnerabilities  bool               `json:"has_vulnerabilities,omitempty"`
	HasPromptInjection  bool               `json:"has_prompt_injection,omitempty"`
	HasShellInjection   bool               `json:"has_shell_injection,omitempty"`
	HasDataExfiltration bool               `json:"has_data_exfiltration,omitempty"`
	HasBinary           bool               `json:"has_binary,omitempty"`
	HasScripts          bool               `json:"has_scripts,omitempty"`
	InstallType         string             `json:"install_type,omitempty"`
	ArtifactKind        string             `json:"artifact_kind,omitempty"`
	Installable         *bool              `json:"installable,omitempty"`
	Findings            []SecurityFinding  `json:"findings,omitempty"`
	Evidence            []SecurityEvidence `json:"evidence,omitempty"`
	ScannerVersion      string             `json:"scanner_version,omitempty"`
}

type InstallSurface struct {
	InstallType         string   `json:"install_type"`
	ArtifactKind        string   `json:"artifact_kind"`
	Installable         bool     `json:"installable"`
	HasBinary           bool     `json:"has_binary"`
	HasScripts          bool     `json:"has_scripts"`
	DependencyManifests []string `json:"dependency_manifests,omitempty"`
}

type SecurityReport struct {
	ID                  string             `json:"id"`
	SkillVersionID      string             `json:"skill_version_id"`
	SkillID             string             `json:"skill_id"`
	Version             string             `json:"version"`
	Score               int                `json:"score"`
	RiskLevel           string             `json:"risk_level"`
	SecurityBadge       string             `json:"security_badge"`
	VulnerabilityStatus string             `json:"vulnerability_status"`
	Risks               []SecurityFinding  `json:"risks"`
	Permissions         []string           `json:"permissions"`
	Secrets             []string           `json:"secrets"`
	Vulnerabilities     []string           `json:"vulnerabilities,omitempty"`
	InstallSurface      InstallSurface     `json:"install_surface"`
	Evidence            []SecurityEvidence `json:"evidence,omitempty"`
	HasVulnerabilities  bool               `json:"has_vulnerabilities"`
	HasPromptInjection  bool               `json:"has_prompt_injection"`
	HasShellInjection   bool               `json:"has_shell_injection"`
	HasDataExfiltration bool               `json:"has_data_exfiltration"`
	HasBinary           bool               `json:"has_binary"`
	ScannerVersion      string             `json:"scanner_version"`
	LLMStatus           string             `json:"llm_status"`
	LLMVerdictJSON      string             `json:"llm_verdict_json,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

type Source struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"`
	BaseURL            string            `json:"base_url"`
	DisplayName        string            `json:"display_name,omitempty"`
	SourceGroup        string            `json:"source_group,omitempty"`
	MirrorOf           string            `json:"mirror_of,omitempty"`
	AuthMode           string            `json:"auth_mode,omitempty"`
	Headers            map[string]string `json:"headers,omitempty"`
	Enabled            bool              `json:"enabled"`
	LastCursor         string            `json:"last_cursor,omitempty"`
	ETag               string            `json:"etag,omitempty"`
	RateLimitPerMinute int               `json:"rate_limit_per_minute"`
	Priority           int               `json:"priority"`
	LastSuccessAt      time.Time         `json:"last_success_at"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type CrawlRun struct {
	ID         string    `json:"id"`
	SourceID   string    `json:"source_id"`
	Status     string    `json:"status"`
	Discovered int       `json:"discovered"`
	Updated    int       `json:"updated"`
	Failed     int       `json:"failed"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	ErrorText  string    `json:"error_text,omitempty"`
}

type SearchQuery struct {
	Query               string
	Category            string
	Categories          []string
	Sources             []string
	RiskBadges          []string
	InstallTypes        []string
	ArtifactKinds       []string
	Sort                string
	Page                int
	PageSize            int
	Semantic            bool
	Installable         *bool
	Curated             *bool
	OpenSourceOnly      bool
	HasVulnerabilities  *bool
	HasPromptInjection  *bool
	HasShellInjection   *bool
	HasDataExfiltration *bool
}

type SearchResult struct {
	Skill         SkillDocument `json:"skill"`
	Score         float64       `json:"score"`
	KeywordScore  float64       `json:"keyword_score,omitempty"`
	SemanticScore float64       `json:"semantic_score,omitempty"`
	MatchSource   string        `json:"match_source,omitempty"`
}

type SearchResponse struct {
	Skills     []SearchResult `json:"skills"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
}

type FilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type SkillFilters struct {
	Categories      []FilterOption `json:"categories"`
	Sources         []FilterOption `json:"sources"`
	RiskBadges      []FilterOption `json:"risk_badges"`
	InstallTypes    []FilterOption `json:"install_types"`
	ArtifactKinds   []FilterOption `json:"artifact_kinds"`
	Installable     map[string]int `json:"installable"`
	SecuritySignals map[string]int `json:"security_signals"`
}

type SkillDetail struct {
	Skill     SkillDocument   `json:"skill"`
	Version   *SkillVersion   `json:"version,omitempty"`
	Security  *SecurityReport `json:"security,omitempty"`
	Installed bool            `json:"installed"`
	Enabled   bool            `json:"enabled"`
}

type InstallRequest struct {
	ID      string `json:"id,omitempty"`
	Version string `json:"version,omitempty"`
	GitHub  string `json:"github,omitempty"`
	AckRisk bool   `json:"ack_risk,omitempty"`
}

type InstallResult struct {
	SkillID     string          `json:"skill_id"`
	Version     string          `json:"version"`
	Path        string          `json:"path"`
	CachePath   string          `json:"cache_path"`
	Warnings    []string        `json:"warnings,omitempty"`
	Security    *SecurityReport `json:"security,omitempty"`
	InstalledAt time.Time       `json:"installed_at"`
}

type InstalledSkill struct {
	SkillID            string    `json:"skill_id"`
	Name               string    `json:"name,omitempty"`
	InstalledVersion   string    `json:"installed_version"`
	Checksum           string    `json:"checksum"`
	SourceURL          string    `json:"source_url,omitempty"`
	Enabled            bool      `json:"enabled"`
	AutoUpdate         bool      `json:"auto_update"`
	InstalledAt        time.Time `json:"installed_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	LastSecurityScore  int       `json:"last_security_score"`
	LatestVersion      string    `json:"latest_version,omitempty"`
	UpdateAvailable    bool      `json:"update_available"`
	LatestChecksum     string    `json:"latest_checksum,omitempty"`
	PendingUpdateState string    `json:"pending_update_state,omitempty"`
	SecurityBadge      string    `json:"security_badge,omitempty"`
}

type AvailableUpdate struct {
	SkillID         string    `json:"skill_id"`
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	CurrentChecksum string    `json:"current_checksum"`
	LatestChecksum  string    `json:"latest_checksum"`
	Action          string    `json:"action"`
	CheckedAt       time.Time `json:"checked_at"`
}

type DiscoverResult struct {
	SourcesProcessed int `json:"sources_processed"`
	Discovered       int `json:"discovered"`
	Updated          int `json:"updated"`
	Failed           int `json:"failed"`
}

type CurationEntry struct {
	SkillID string  `yaml:"skill_id" json:"skill_id"`
	Rank    int     `yaml:"rank" json:"rank,omitempty"`
	Label   string  `yaml:"label" json:"label,omitempty"`
	Reason  string  `yaml:"reason" json:"reason,omitempty"`
	Weight  float64 `yaml:"weight" json:"weight,omitempty"`
}

type AdditionalSeed struct {
	Type  string `yaml:"type" json:"type"`
	Value string `yaml:"value" json:"value"`
}

type CuratedConfig struct {
	Version         string           `yaml:"version" json:"version"`
	Featured        []CurationEntry  `yaml:"featured" json:"featured"`
	Boosts          []CurationEntry  `yaml:"boosts" json:"boosts"`
	Hidden          []string         `yaml:"hidden" json:"hidden"`
	AdditionalSeeds []AdditionalSeed `yaml:"additional_seeds" json:"additional_seeds"`
}

type SkillCuration struct {
	ID           string    `json:"id"`
	SkillID      string    `json:"skill_id"`
	Hidden       bool      `json:"hidden"`
	FeaturedRank int       `json:"featured_rank"`
	BoostWeight  float64   `json:"boost_weight"`
	Label        string    `json:"label,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CurationSyncState struct {
	ID            string    `json:"id"`
	SourceURL     string    `json:"source_url,omitempty"`
	Checksum      string    `json:"checksum,omitempty"`
	LastSuccessAt time.Time `json:"last_success_at"`
	LastError     string    `json:"last_error,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}
