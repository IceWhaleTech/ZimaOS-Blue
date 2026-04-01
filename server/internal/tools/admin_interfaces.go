package tools

import "context"

// --- Provider Management ---

// AdminProviderInfo is a simplified provider representation for the admin tool.
type AdminProviderInfo struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Type     string             `json:"type"`
	Location string             `json:"location,omitempty"` // cloud or local
	BaseURL  string             `json:"base_url,omitempty"`
	Enabled  bool               `json:"enabled"`
	Models   []string           `json:"models,omitempty"`
	Priority int                `json:"priority"`
	APIKeys  []AdminProviderKey `json:"api_keys,omitempty"`
}

// AdminProviderKey is a simplified API key representation (no raw key).
type AdminProviderKey struct {
	ID      string `json:"id"`
	KeyHash string `json:"key_hash"` // e.g. "sk-ab...wxyz"
	Label   string `json:"label,omitempty"`
	Enabled bool   `json:"enabled"`
}

// AdminProviderService provides provider management operations.
type AdminProviderService interface {
	ListProviders(ctx context.Context) ([]AdminProviderInfo, error)
	AddProvider(ctx context.Context, name, providerType, baseURL, apiKey, location string) (*AdminProviderInfo, error)
	AddKey(ctx context.Context, providerID, apiKey string) (*AdminProviderInfo, error)
	RemoveProvider(ctx context.Context, id string) error
	EnableProvider(ctx context.Context, id string) error
	DisableProvider(ctx context.Context, id string) error
	TestProvider(ctx context.Context, id string) (map[string]interface{}, error)
	ListModels(ctx context.Context) ([]map[string]interface{}, error)
}

// --- Settings ---

// AdminSettingsService provides settings management.
type AdminSettingsService interface {
	GetAll(ctx context.Context) (map[string]interface{}, error)
	Set(ctx context.Context, key, value string) error
}

// --- Channel Management ---

// AdminChannelInfo is a simplified channel representation.
type AdminChannelInfo struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// AdminChannelService provides channel management.
type AdminChannelService interface {
	ListChannels(ctx context.Context) ([]AdminChannelInfo, error)
	GetStatus(ctx context.Context, name string) (*AdminChannelInfo, error)
}

// --- Skill Management ---

// AdminSkillInfo is a simplified skill representation.
type AdminSkillInfo struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Category         string   `json:"category,omitempty"`
	Enabled          bool     `json:"enabled"`
	Builtin          bool     `json:"builtin"`
	Paths            []string `json:"paths,omitempty"`
	UserInvocable    bool     `json:"user_invocable"`
	ModelInvocable   bool     `json:"model_invocable"`
	ActivationState  string   `json:"activation_state,omitempty"`
	ActivationSource string   `json:"activation_source,omitempty"`
}

// AdminSkillService provides skill management.
type AdminSkillService interface {
	ListSkills(ctx context.Context) ([]AdminSkillInfo, error)
	EnableSkill(ctx context.Context, id string) error
	DisableSkill(ctx context.Context, id string) error
}

// --- Tool Management ---

// AdminToolInfo is a simplified tool representation.
type AdminToolInfo struct {
	Name                string   `json:"name"`
	Description         string   `json:"description,omitempty"`
	Disabled            bool     `json:"disabled,omitempty"`
	RiskLevel           string   `json:"risk_level,omitempty"`
	VisibilityAllowlist []string `json:"visibility_allowlist,omitempty"`
}

// AdminToolService provides tool management.
type AdminToolService interface {
	ListTools(ctx context.Context) ([]AdminToolInfo, error)
	EnableTool(ctx context.Context, name string) error
	DisableTool(ctx context.Context, name string) error
}

// --- System Info ---

// AdminSystemInfo holds system health information.
type AdminSystemInfo struct {
	Version       string  `json:"version"`
	Uptime        string  `json:"uptime"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	GoVersion     string  `json:"go_version"`
	NumCPU        int     `json:"num_cpu"`
	Goroutines    int     `json:"goroutines"`
	MemAllocMB    float64 `json:"mem_alloc_mb"`
	MemRSSMB      float64 `json:"mem_rss_mb"`
}

// AdminSystemService provides system information.
type AdminSystemService interface {
	Health(ctx context.Context) (*AdminSystemInfo, error)
}

// --- Proxy Stats ---

// AdminProxyService provides proxy/pipeline statistics.
type AdminProxyService interface {
	Stats(ctx context.Context) (map[string]interface{}, error)
	CacheStats(ctx context.Context) (map[string]interface{}, error)
}

// --- User Management ---

// AdminUserInfo is a simplified user representation.
type AdminUserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role,omitempty"`
	Locked   bool   `json:"locked"`
}

// AdminUserService provides user management.
type AdminUserService interface {
	ListUsers(ctx context.Context) ([]AdminUserInfo, error)
	LockUser(ctx context.Context, id string) error
	UnlockUser(ctx context.Context, id string) error
}

// --- API Key Management ---

// AdminAPIKeyInfo is a simplified API key representation.
type AdminAPIKeyInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"` // first 8 chars
	CreatedAt string `json:"created_at,omitempty"`
}

// AdminAPIKeyService provides API key management.
type AdminAPIKeyService interface {
	ListKeys(ctx context.Context) ([]AdminAPIKeyInfo, error)
	CreateKey(ctx context.Context, name string) (*AdminAPIKeyCreateResult, error)
	RevokeKey(ctx context.Context, id string) error
}

// AdminAPIKeyCreateResult holds the result of creating a new API key.
type AdminAPIKeyCreateResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Key    string `json:"key"` // full key, shown only once
	Prefix string `json:"prefix"`
}

// --- Upgrade / OTA ---

// AdminUpgradeInfo holds upgrade/OTA information.
type AdminUpgradeInfo struct {
	CurrentVersion  string   `json:"current_version"`
	UpdateAvailable bool     `json:"update_available"`
	LatestVersion   string   `json:"latest_version,omitempty"`
	DownloadURLs    []string `json:"download_urls,omitempty"`
	ReleaseNoteURL  string   `json:"release_note_url,omitempty"`
	Delay           int      `json:"delay,omitempty"`
	State           string   `json:"state,omitempty"`           // idle, checking, downloading, applying, restarting, failed
	Progress        float64  `json:"progress,omitempty"`        // 0-100
	Error           string   `json:"error,omitempty"`           // error message if failed
	DownloadedPath  string   `json:"downloaded_path,omitempty"` // path to downloaded update
}

// AdminUpgradeService provides upgrade/OTA management.
type AdminUpgradeService interface {
	GetOTAStatus(ctx context.Context) (*AdminUpgradeInfo, error)
	CheckForUpdate(ctx context.Context) (*AdminUpgradeInfo, error)
	GetUpdateStatus(ctx context.Context) (*AdminUpgradeInfo, error)
	StartDownload(ctx context.Context) (*AdminUpgradeInfo, error)
	ApplyUpdate(ctx context.Context) (*AdminUpgradeInfo, error)
}
