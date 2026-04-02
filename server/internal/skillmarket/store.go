package skillmarket

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

type searchCacheEntry struct {
	key       string
	results   *SearchResponse
	timestamp time.Time
}

type Store struct {
	db         *sql.DB
	readDB     *sql.DB
	ftsEnabled bool

	// Search result caching
	searchCacheMu  sync.RWMutex
	searchCache    map[string]searchCacheEntry
	searchCacheTTL time.Duration
}

const sqliteSafeMaxBindVars = 900

type SkillUpsertRecord struct {
	Doc     *SkillDocument
	Version *SkillVersion
	Report  *SecurityReport
}

type SkillBatchUpsertResult struct {
	Inserted int
	Updated  int
	Skipped  int
}

func newStoreWithDB(writeDB, readDB *sql.DB) (*Store, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("skillmarket db is nil")
	}
	if readDB == nil {
		readDB = writeDB
	}
	s := &Store{db: writeDB, readDB: readDB, searchCache: make(map[string]searchCacheEntry), searchCacheTTL: 5 * time.Second}
	if err := s.initSchema(); err != nil {
		return nil, err
	}
	if err := s.normalizeLegacyCategories(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	return newStoreWithDB(writeDB, readDB)
}

func (s *Store) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) table(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, s.db, name)
}

func (s *Store) readTable(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), name)
}

type skillCategoryNormalizationRow struct {
	ID           string  `zorm:"id"`
	Category     *string `zorm:"category"`
	Tags         *string `zorm:"tags"`
	SkillContent *string `zorm:"skill_content"`
}

type skillCurationRow struct {
	Hidden       int     `zorm:"hidden"`
	FeaturedRank int     `zorm:"featured_rank"`
	BoostWeight  float64 `zorm:"boost_weight"`
	Label        *string `zorm:"label"`
	Reason       *string `zorm:"reason"`
}

type skillIDRow struct {
	ID string `zorm:"id"`
}

type skillVersionLookupRow struct {
	ID      string `zorm:"id"`
	SkillID string `zorm:"skill_id"`
	Version string `zorm:"version"`
}

type skillReportLookupRow struct {
	ID             string `zorm:"id"`
	SkillVersionID string `zorm:"skill_version_id"`
}

type skillSourceIDRow struct {
	SourceID *string `zorm:"source_id"`
}

type sourcePriorityRow struct {
	Priority int `zorm:"priority"`
}

type skillLatestVersionRow struct {
	LatestVersion *string `zorm:"latest_version"`
}

type skillDocumentRow struct {
	ID                  string  `zorm:"id"`
	Slug                string  `zorm:"slug"`
	Name                string  `zorm:"name"`
	Description         *string `zorm:"description"`
	Author              *string `zorm:"author"`
	RepoURL             *string `zorm:"repo_url"`
	Homepage            *string `zorm:"homepage"`
	DownloadURL         *string `zorm:"download_url"`
	Stars               int     `zorm:"stars"`
	Downloads           int     `zorm:"downloads"`
	Tags                *string `zorm:"tags"`
	Category            *string `zorm:"category"`
	SecurityScore       int     `zorm:"security_score"`
	Permissions         *string `zorm:"permissions"`
	LatestVersion       *string `zorm:"latest_version"`
	RiskLevel           *string `zorm:"risk_level"`
	SecurityBadge       *string `zorm:"security_badge"`
	Installable         int     `zorm:"installable"`
	InstallType         *string `zorm:"install_type"`
	ArtifactKind        *string `zorm:"artifact_kind"`
	VulnerabilityStatus *string `zorm:"vulnerability_status"`
	HasVulnerabilities  int     `zorm:"has_vulnerabilities"`
	HasPromptInjection  int     `zorm:"has_prompt_injection"`
	HasShellInjection   int     `zorm:"has_shell_injection"`
	HasDataExfiltration int     `zorm:"has_data_exfiltration"`
	HasBinary           int     `zorm:"has_binary"`
	HasScripts          int     `zorm:"has_scripts"`
	PopularityScore     float64 `zorm:"popularity_score"`
	TrendingScore       float64 `zorm:"trending_score"`
	ScanStatus          *string `zorm:"scan_status"`
	ContentSHA256       *string `zorm:"content_sha256"`
	Published           int     `zorm:"published"`
	SourceID            *string `zorm:"source_id"`
	SourceName          *string `zorm:"source_name"`
	SourceGroup         *string `zorm:"source_group"`
	SourceType          *string `zorm:"source_type"`
	SkillPath           *string `zorm:"skill_path"`
	SkillContent        *string `zorm:"skill_content"`
	EmbeddingJSON       *string `zorm:"embedding_json"`
	EmbeddingModel      *string `zorm:"embedding_model"`
	CuratedRank         int     `zorm:"curated_rank"`
	CuratedBoost        float64 `zorm:"curated_boost"`
	CuratedLabel        *string `zorm:"curated_label"`
	CuratedReason       *string `zorm:"curated_reason"`
	LastUpdated         *string `zorm:"last_updated"`
	LastCrawledAt       *string `zorm:"last_crawled_at"`
	CreatedAt           *string `zorm:"created_at"`
	UpdatedAt           *string `zorm:"updated_at"`
}

type skillDocumentWriteRow struct {
	ID                  string    `zorm:"id"`
	Slug                string    `zorm:"slug"`
	Name                string    `zorm:"name"`
	Description         string    `zorm:"description"`
	Author              string    `zorm:"author"`
	RepoURL             string    `zorm:"repo_url"`
	Homepage            string    `zorm:"homepage"`
	DownloadURL         string    `zorm:"download_url"`
	Stars               int       `zorm:"stars"`
	Downloads           int       `zorm:"downloads"`
	Tags                string    `zorm:"tags"`
	Category            string    `zorm:"category"`
	SecurityScore       int       `zorm:"security_score"`
	Permissions         string    `zorm:"permissions"`
	LatestVersion       string    `zorm:"latest_version"`
	RiskLevel           string    `zorm:"risk_level"`
	SecurityBadge       string    `zorm:"security_badge"`
	Installable         int       `zorm:"installable"`
	InstallType         string    `zorm:"install_type"`
	ArtifactKind        string    `zorm:"artifact_kind"`
	VulnerabilityStatus string    `zorm:"vulnerability_status"`
	HasVulnerabilities  int       `zorm:"has_vulnerabilities"`
	HasPromptInjection  int       `zorm:"has_prompt_injection"`
	HasShellInjection   int       `zorm:"has_shell_injection"`
	HasDataExfiltration int       `zorm:"has_data_exfiltration"`
	HasBinary           int       `zorm:"has_binary"`
	HasScripts          int       `zorm:"has_scripts"`
	PopularityScore     float64   `zorm:"popularity_score"`
	TrendingScore       float64   `zorm:"trending_score"`
	ScanStatus          string    `zorm:"scan_status"`
	ContentSHA256       string    `zorm:"content_sha256"`
	Published           int       `zorm:"published"`
	SourceID            string    `zorm:"source_id"`
	SourceName          string    `zorm:"source_name"`
	SourceGroup         string    `zorm:"source_group"`
	SourceType          string    `zorm:"source_type"`
	SkillPath           string    `zorm:"skill_path"`
	SkillContent        string    `zorm:"skill_content"`
	EmbeddingJSON       string    `zorm:"embedding_json"`
	EmbeddingModel      string    `zorm:"embedding_model"`
	CuratedRank         int       `zorm:"curated_rank"`
	CuratedBoost        float64   `zorm:"curated_boost"`
	CuratedLabel        string    `zorm:"curated_label"`
	CuratedReason       string    `zorm:"curated_reason"`
	LastUpdated         time.Time `zorm:"last_updated"`
	LastCrawledAt       time.Time `zorm:"last_crawled_at"`
	CreatedAt           time.Time `zorm:"created_at"`
	UpdatedAt           time.Time `zorm:"updated_at"`
}

type skillVersionRow struct {
	ID           string  `zorm:"id"`
	SkillID      string  `zorm:"skill_id"`
	Version      string  `zorm:"version"`
	CommitHash   *string `zorm:"commit_hash"`
	SourceURL    *string `zorm:"source_url"`
	Checksum     *string `zorm:"checksum"`
	SkillPath    *string `zorm:"skill_path"`
	RawSkillMD   *string `zorm:"raw_skill_md"`
	ManifestJSON *string `zorm:"manifest_json"`
	ReleasedAt   *string `zorm:"released_at"`
	ScannedAt    *string `zorm:"scanned_at"`
	CreatedAt    *string `zorm:"created_at"`
	UpdatedAt    *string `zorm:"updated_at"`
}

type skillVersionWriteRow struct {
	ID           string    `zorm:"id"`
	SkillID      string    `zorm:"skill_id"`
	Version      string    `zorm:"version"`
	CommitHash   string    `zorm:"commit_hash"`
	SourceURL    string    `zorm:"source_url"`
	Checksum     string    `zorm:"checksum"`
	SkillPath    string    `zorm:"skill_path"`
	RawSkillMD   string    `zorm:"raw_skill_md"`
	ManifestJSON string    `zorm:"manifest_json"`
	ReleasedAt   time.Time `zorm:"released_at"`
	ScannedAt    time.Time `zorm:"scanned_at"`
	CreatedAt    time.Time `zorm:"created_at"`
	UpdatedAt    time.Time `zorm:"updated_at"`
}

type securityReportRow struct {
	ID                  string  `zorm:"id"`
	SkillVersionID      string  `zorm:"skill_version_id"`
	SkillID             string  `zorm:"skill_id"`
	Version             string  `zorm:"version"`
	Score               int     `zorm:"score"`
	RiskLevel           *string `zorm:"risk_level"`
	SecurityBadge       *string `zorm:"security_badge"`
	VulnerabilityStatus *string `zorm:"vulnerability_status"`
	RisksJSON           *string `zorm:"risks_json"`
	PermissionsJSON     *string `zorm:"permissions_json"`
	SecretsJSON         *string `zorm:"secrets_json"`
	VulnerabilitiesJSON *string `zorm:"vulnerabilities_json"`
	InstallSurfaceJSON  *string `zorm:"install_surface_json"`
	EvidenceJSON        *string `zorm:"evidence_json"`
	ArtifactKind        *string `zorm:"artifact_kind"`
	HasPromptInjection  int     `zorm:"has_prompt_injection"`
	HasShellInjection   int     `zorm:"has_shell_injection"`
	HasDataExfiltration int     `zorm:"has_data_exfiltration"`
	ScannerVersion      *string `zorm:"scanner_version"`
	LLMStatus           *string `zorm:"llm_status"`
	LLMVerdictJSON      *string `zorm:"llm_verdict_json"`
	CreatedAt           *string `zorm:"created_at"`
	UpdatedAt           *string `zorm:"updated_at"`
}

type securityReportWriteRow struct {
	ID                  string    `zorm:"id"`
	SkillVersionID      string    `zorm:"skill_version_id"`
	SkillID             string    `zorm:"skill_id"`
	Version             string    `zorm:"version"`
	Score               int       `zorm:"score"`
	RiskLevel           string    `zorm:"risk_level"`
	SecurityBadge       string    `zorm:"security_badge"`
	VulnerabilityStatus string    `zorm:"vulnerability_status"`
	RisksJSON           string    `zorm:"risks_json"`
	PermissionsJSON     string    `zorm:"permissions_json"`
	SecretsJSON         string    `zorm:"secrets_json"`
	VulnerabilitiesJSON string    `zorm:"vulnerabilities_json"`
	InstallSurfaceJSON  string    `zorm:"install_surface_json"`
	EvidenceJSON        string    `zorm:"evidence_json"`
	ArtifactKind        string    `zorm:"artifact_kind"`
	HasPromptInjection  int       `zorm:"has_prompt_injection"`
	HasShellInjection   int       `zorm:"has_shell_injection"`
	HasDataExfiltration int       `zorm:"has_data_exfiltration"`
	ScannerVersion      string    `zorm:"scanner_version"`
	LLMStatus           string    `zorm:"llm_status"`
	LLMVerdictJSON      string    `zorm:"llm_verdict_json"`
	CreatedAt           time.Time `zorm:"created_at"`
	UpdatedAt           time.Time `zorm:"updated_at"`
}

type sourceRow struct {
	ID                 string  `zorm:"id"`
	Type               string  `zorm:"type"`
	BaseURL            string  `zorm:"base_url"`
	DisplayName        *string `zorm:"display_name"`
	SourceGroup        *string `zorm:"source_group"`
	MirrorOf           *string `zorm:"mirror_of"`
	AuthMode           *string `zorm:"auth_mode"`
	HeadersJSON        *string `zorm:"headers_json"`
	Enabled            int     `zorm:"enabled"`
	LastCursor         *string `zorm:"last_cursor"`
	ETag               *string `zorm:"etag"`
	RateLimitPerMinute int     `zorm:"rate_limit_per_minute"`
	Priority           int     `zorm:"priority"`
	LastSuccessAt      *string `zorm:"last_success_at"`
	CreatedAt          *string `zorm:"created_at"`
	UpdatedAt          *string `zorm:"updated_at"`
}

type installedSkillRow struct {
	SkillID           string  `zorm:"skill_id"`
	InstalledVersion  string  `zorm:"installed_version"`
	Checksum          *string `zorm:"checksum"`
	SourceURL         *string `zorm:"source_url"`
	Enabled           int     `zorm:"enabled"`
	AutoUpdate        int     `zorm:"auto_update"`
	InstalledAt       *string `zorm:"installed_at"`
	UpdatedAt         *string `zorm:"updated_at"`
	LastSecurityScore int     `zorm:"last_security_score"`
}

type installedSkillListRow struct {
	SkillID            string  `zorm:"skill_id"`
	InstalledVersion   string  `zorm:"installed_version"`
	Checksum           *string `zorm:"checksum"`
	SourceURL          *string `zorm:"source_url"`
	Enabled            int     `zorm:"enabled"`
	AutoUpdate         int     `zorm:"auto_update"`
	InstalledAt        *string `zorm:"installed_at"`
	UpdatedAt          *string `zorm:"updated_at"`
	LastSecurityScore  int     `zorm:"last_security_score"`
	Name               *string `zorm:"name"`
	LatestVersion      *string `zorm:"latest_version"`
	LatestChecksum     *string `zorm:"latest_checksum"`
	PendingUpdateState *string `zorm:"pending_update_state"`
	SecurityBadge      *string `zorm:"security_badge"`
}

type availableUpdateRow struct {
	SkillID         string  `zorm:"skill_id"`
	CurrentVersion  string  `zorm:"current_version"`
	LatestVersion   *string `zorm:"latest_version"`
	CurrentChecksum *string `zorm:"current_checksum"`
	LatestChecksum  *string `zorm:"latest_checksum"`
	Action          *string `zorm:"action"`
	CheckedAt       *string `zorm:"checked_at"`
}

type filterBucketRow struct {
	Value string `zorm:"value"`
	Count int    `zorm:"count"`
}

type categoryBucketRow struct {
	Category string `zorm:"category"`
	Count    int    `zorm:"count"`
}

type securityBadgeBucketRow struct {
	SecurityBadge string `zorm:"security_badge"`
	Count         int    `zorm:"count"`
}

type installTypeBucketRow struct {
	InstallType string `zorm:"install_type"`
	Count       int    `zorm:"count"`
}

type artifactKindBucketRow struct {
	ArtifactKind string `zorm:"artifact_kind"`
	Count        int    `zorm:"count"`
}

type filterSignalsRow struct {
	Installable     int `zorm:"installable"`
	ManualOnly      int `zorm:"manual_only"`
	Vulnerabilities int `zorm:"vulnerabilities"`
	Prompt          int `zorm:"prompt"`
	Shell           int `zorm:"shell"`
	Exfil           int `zorm:"exfil"`
}

type curationSyncStateRow struct {
	ID            string  `zorm:"id"`
	SourceURL     *string `zorm:"source_url"`
	Checksum      *string `zorm:"checksum"`
	LastSuccessAt *string `zorm:"last_success_at"`
	LastError     *string `zorm:"last_error"`
	UpdatedAt     *string `zorm:"updated_at"`
}

func nullableStringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func sourceFromRow(row sourceRow) Source {
	src := Source{
		ID:                 row.ID,
		Type:               row.Type,
		BaseURL:            row.BaseURL,
		DisplayName:        nullableStringValue(row.DisplayName),
		SourceGroup:        nullableStringValue(row.SourceGroup),
		MirrorOf:           nullableStringValue(row.MirrorOf),
		AuthMode:           nullableStringValue(row.AuthMode),
		Enabled:            row.Enabled == 1,
		LastCursor:         nullableStringValue(row.LastCursor),
		ETag:               nullableStringValue(row.ETag),
		RateLimitPerMinute: row.RateLimitPerMinute,
		Priority:           row.Priority,
		LastSuccessAt:      parseTime(nullableStringValue(row.LastSuccessAt)),
		CreatedAt:          parseTime(nullableStringValue(row.CreatedAt)),
		UpdatedAt:          parseTime(nullableStringValue(row.UpdatedAt)),
	}
	if raw := strings.TrimSpace(nullableStringValue(row.HeadersJSON)); raw != "" {
		_ = json.Unmarshal([]byte(raw), &src.Headers)
	}
	return src
}

func skillDocumentFromRow(row skillDocumentRow) SkillDocument {
	return SkillDocument{
		ID:                  row.ID,
		Slug:                row.Slug,
		Name:                row.Name,
		Description:         nullableStringValue(row.Description),
		Author:              nullableStringValue(row.Author),
		RepoURL:             nullableStringValue(row.RepoURL),
		Homepage:            nullableStringValue(row.Homepage),
		DownloadURL:         nullableStringValue(row.DownloadURL),
		Stars:               row.Stars,
		Downloads:           row.Downloads,
		Tags:                decodeStrings(nullableStringValue(row.Tags)),
		Category:            nullableStringValue(row.Category),
		SecurityScore:       row.SecurityScore,
		Permissions:         decodeStrings(nullableStringValue(row.Permissions)),
		LatestVersion:       nullableStringValue(row.LatestVersion),
		RiskLevel:           nullableStringValue(row.RiskLevel),
		SecurityBadge:       nullableStringValue(row.SecurityBadge),
		Installable:         row.Installable == 1,
		InstallType:         nullableStringValue(row.InstallType),
		ArtifactKind:        nullableStringValue(row.ArtifactKind),
		VulnerabilityStatus: nullableStringValue(row.VulnerabilityStatus),
		HasVulnerabilities:  row.HasVulnerabilities == 1,
		HasPromptInjection:  row.HasPromptInjection == 1,
		HasShellInjection:   row.HasShellInjection == 1,
		HasDataExfiltration: row.HasDataExfiltration == 1,
		HasBinary:           row.HasBinary == 1,
		HasScripts:          row.HasScripts == 1,
		PopularityScore:     row.PopularityScore,
		TrendingScore:       row.TrendingScore,
		ScanStatus:          nullableStringValue(row.ScanStatus),
		ContentSHA256:       nullableStringValue(row.ContentSHA256),
		Published:           row.Published == 1,
		SourceID:            nullableStringValue(row.SourceID),
		SourceName:          nullableStringValue(row.SourceName),
		SourceGroup:         nullableStringValue(row.SourceGroup),
		SourceType:          nullableStringValue(row.SourceType),
		SkillPath:           nullableStringValue(row.SkillPath),
		SkillContent:        nullableStringValue(row.SkillContent),
		EmbeddingJSON:       nullableStringValue(row.EmbeddingJSON),
		EmbeddingModel:      nullableStringValue(row.EmbeddingModel),
		CuratedRank:         row.CuratedRank,
		CuratedBoost:        row.CuratedBoost,
		CuratedLabel:        nullableStringValue(row.CuratedLabel),
		CuratedReason:       nullableStringValue(row.CuratedReason),
		LastUpdated:         parseTime(nullableStringValue(row.LastUpdated)),
		LastCrawledAt:       parseTime(nullableStringValue(row.LastCrawledAt)),
		CreatedAt:           parseTime(nullableStringValue(row.CreatedAt)),
		UpdatedAt:           parseTime(nullableStringValue(row.UpdatedAt)),
	}
}

func skillDocumentWriteRowFromDoc(doc *SkillDocument) skillDocumentWriteRow {
	return skillDocumentWriteRow{
		ID:                  doc.ID,
		Slug:                doc.Slug,
		Name:                doc.Name,
		Description:         doc.Description,
		Author:              doc.Author,
		RepoURL:             doc.RepoURL,
		Homepage:            doc.Homepage,
		DownloadURL:         doc.DownloadURL,
		Stars:               doc.Stars,
		Downloads:           doc.Downloads,
		Tags:                encodeStrings(doc.Tags),
		Category:            doc.Category,
		SecurityScore:       doc.SecurityScore,
		Permissions:         encodeStrings(doc.Permissions),
		LatestVersion:       doc.LatestVersion,
		RiskLevel:           doc.RiskLevel,
		SecurityBadge:       doc.SecurityBadge,
		Installable:         boolToInt(doc.Installable),
		InstallType:         doc.InstallType,
		ArtifactKind:        doc.ArtifactKind,
		VulnerabilityStatus: doc.VulnerabilityStatus,
		HasVulnerabilities:  boolToInt(doc.HasVulnerabilities),
		HasPromptInjection:  boolToInt(doc.HasPromptInjection),
		HasShellInjection:   boolToInt(doc.HasShellInjection),
		HasDataExfiltration: boolToInt(doc.HasDataExfiltration),
		HasBinary:           boolToInt(doc.HasBinary),
		HasScripts:          boolToInt(doc.HasScripts),
		PopularityScore:     doc.PopularityScore,
		TrendingScore:       doc.TrendingScore,
		ScanStatus:          doc.ScanStatus,
		ContentSHA256:       doc.ContentSHA256,
		Published:           boolToInt(doc.Published),
		SourceID:            doc.SourceID,
		SourceName:          doc.SourceName,
		SourceGroup:         doc.SourceGroup,
		SourceType:          doc.SourceType,
		SkillPath:           doc.SkillPath,
		SkillContent:        doc.SkillContent,
		EmbeddingJSON:       doc.EmbeddingJSON,
		EmbeddingModel:      doc.EmbeddingModel,
		CuratedRank:         doc.CuratedRank,
		CuratedBoost:        doc.CuratedBoost,
		CuratedLabel:        doc.CuratedLabel,
		CuratedReason:       doc.CuratedReason,
		LastUpdated:         doc.LastUpdated,
		LastCrawledAt:       doc.LastCrawledAt,
		CreatedAt:           doc.CreatedAt,
		UpdatedAt:           doc.UpdatedAt,
	}
}

func skillVersionFromRow(row skillVersionRow) SkillVersion {
	return SkillVersion{
		ID:           row.ID,
		SkillID:      row.SkillID,
		Version:      row.Version,
		CommitHash:   nullableStringValue(row.CommitHash),
		SourceURL:    nullableStringValue(row.SourceURL),
		Checksum:     nullableStringValue(row.Checksum),
		SkillPath:    nullableStringValue(row.SkillPath),
		RawSkillMD:   nullableStringValue(row.RawSkillMD),
		ManifestJSON: nullableStringValue(row.ManifestJSON),
		ReleasedAt:   parseTime(nullableStringValue(row.ReleasedAt)),
		ScannedAt:    parseTime(nullableStringValue(row.ScannedAt)),
		CreatedAt:    parseTime(nullableStringValue(row.CreatedAt)),
		UpdatedAt:    parseTime(nullableStringValue(row.UpdatedAt)),
	}
}

func skillVersionWriteRowFromVersion(version *SkillVersion) skillVersionWriteRow {
	return skillVersionWriteRow{
		ID:           version.ID,
		SkillID:      version.SkillID,
		Version:      version.Version,
		CommitHash:   version.CommitHash,
		SourceURL:    version.SourceURL,
		Checksum:     version.Checksum,
		SkillPath:    version.SkillPath,
		RawSkillMD:   version.RawSkillMD,
		ManifestJSON: version.ManifestJSON,
		ReleasedAt:   version.ReleasedAt,
		ScannedAt:    version.ScannedAt,
		CreatedAt:    version.CreatedAt,
		UpdatedAt:    version.UpdatedAt,
	}
}

func securityReportFromRow(row securityReportRow) SecurityReport {
	report := SecurityReport{
		ID:                  row.ID,
		SkillVersionID:      row.SkillVersionID,
		SkillID:             row.SkillID,
		Version:             row.Version,
		Score:               row.Score,
		RiskLevel:           nullableStringValue(row.RiskLevel),
		SecurityBadge:       nullableStringValue(row.SecurityBadge),
		VulnerabilityStatus: nullableStringValue(row.VulnerabilityStatus),
		Risks:               decodeFindings(nullableStringValue(row.RisksJSON)),
		Permissions:         decodeStrings(nullableStringValue(row.PermissionsJSON)),
		Secrets:             decodeStrings(nullableStringValue(row.SecretsJSON)),
		Vulnerabilities:     decodeStrings(nullableStringValue(row.VulnerabilitiesJSON)),
		HasPromptInjection:  row.HasPromptInjection == 1,
		HasShellInjection:   row.HasShellInjection == 1,
		HasDataExfiltration: row.HasDataExfiltration == 1,
		ScannerVersion:      nullableStringValue(row.ScannerVersion),
		LLMStatus:           nullableStringValue(row.LLMStatus),
		LLMVerdictJSON:      nullableStringValue(row.LLMVerdictJSON),
		CreatedAt:           parseTime(nullableStringValue(row.CreatedAt)),
		UpdatedAt:           parseTime(nullableStringValue(row.UpdatedAt)),
	}
	_ = json.Unmarshal([]byte(nullableStringValue(row.InstallSurfaceJSON)), &report.InstallSurface)
	_ = json.Unmarshal([]byte(nullableStringValue(row.EvidenceJSON)), &report.Evidence)
	if report.InstallSurface.ArtifactKind == "" {
		report.InstallSurface.ArtifactKind = nullableStringValue(row.ArtifactKind)
	}
	return report
}

func securityReportWriteRowFromReport(report *SecurityReport) securityReportWriteRow {
	return securityReportWriteRow{
		ID:                  report.ID,
		SkillVersionID:      report.SkillVersionID,
		SkillID:             report.SkillID,
		Version:             report.Version,
		Score:               report.Score,
		RiskLevel:           report.RiskLevel,
		SecurityBadge:       report.SecurityBadge,
		VulnerabilityStatus: report.VulnerabilityStatus,
		RisksJSON:           encodeJSON(report.Risks),
		PermissionsJSON:     encodeStrings(report.Permissions),
		SecretsJSON:         encodeStrings(report.Secrets),
		VulnerabilitiesJSON: encodeStrings(report.Vulnerabilities),
		InstallSurfaceJSON:  encodeJSON(report.InstallSurface),
		EvidenceJSON:        encodeJSON(report.Evidence),
		ArtifactKind:        report.InstallSurface.ArtifactKind,
		HasPromptInjection:  boolToInt(report.HasPromptInjection),
		HasShellInjection:   boolToInt(report.HasShellInjection),
		HasDataExfiltration: boolToInt(report.HasDataExfiltration),
		ScannerVersion:      report.ScannerVersion,
		LLMStatus:           report.LLMStatus,
		LLMVerdictJSON:      report.LLMVerdictJSON,
		CreatedAt:           report.CreatedAt,
		UpdatedAt:           report.UpdatedAt,
	}
}

func installedSkillFromRow(row installedSkillRow) InstalledSkill {
	return InstalledSkill{
		SkillID:           row.SkillID,
		InstalledVersion:  row.InstalledVersion,
		Checksum:          nullableStringValue(row.Checksum),
		SourceURL:         nullableStringValue(row.SourceURL),
		Enabled:           row.Enabled == 1,
		AutoUpdate:        row.AutoUpdate == 1,
		InstalledAt:       parseTime(nullableStringValue(row.InstalledAt)),
		UpdatedAt:         parseTime(nullableStringValue(row.UpdatedAt)),
		LastSecurityScore: row.LastSecurityScore,
	}
}

func installedSkillListFromRow(row installedSkillListRow) InstalledSkill {
	skill := InstalledSkill{
		SkillID:            row.SkillID,
		Name:               nullableStringValue(row.Name),
		InstalledVersion:   row.InstalledVersion,
		Checksum:           nullableStringValue(row.Checksum),
		SourceURL:          nullableStringValue(row.SourceURL),
		Enabled:            row.Enabled == 1,
		AutoUpdate:         row.AutoUpdate == 1,
		InstalledAt:        parseTime(nullableStringValue(row.InstalledAt)),
		UpdatedAt:          parseTime(nullableStringValue(row.UpdatedAt)),
		LastSecurityScore:  row.LastSecurityScore,
		LatestVersion:      nullableStringValue(row.LatestVersion),
		LatestChecksum:     nullableStringValue(row.LatestChecksum),
		PendingUpdateState: nullableStringValue(row.PendingUpdateState),
		SecurityBadge:      nullableStringValue(row.SecurityBadge),
	}
	skill.UpdateAvailable = skill.LatestVersion != "" && skill.LatestVersion != skill.InstalledVersion
	return skill
}

func installedSkillListFromMapRow(row z.V) InstalledSkill {
	skill := InstalledSkill{
		SkillID:            skillmarketStringFromMapValue(row, "skill_id"),
		Name:               skillmarketStringFromMapValue(row, "name"),
		InstalledVersion:   skillmarketStringFromMapValue(row, "installed_version"),
		Checksum:           skillmarketStringFromMapValue(row, "checksum"),
		SourceURL:          skillmarketStringFromMapValue(row, "source_url"),
		Enabled:            skillmarketIntFromMapValue(row, "enabled") == 1,
		AutoUpdate:         skillmarketIntFromMapValue(row, "auto_update") == 1,
		InstalledAt:        parseTime(skillmarketStringFromMapValue(row, "installed_at")),
		UpdatedAt:          parseTime(skillmarketStringFromMapValue(row, "updated_at")),
		LastSecurityScore:  skillmarketIntFromMapValue(row, "last_security_score"),
		LatestVersion:      skillmarketStringFromMapValue(row, "latest_version"),
		LatestChecksum:     skillmarketStringFromMapValue(row, "latest_checksum"),
		PendingUpdateState: skillmarketStringFromMapValue(row, "pending_update_state"),
		SecurityBadge:      skillmarketStringFromMapValue(row, "security_badge"),
	}
	skill.UpdateAvailable = skill.LatestVersion != "" && skill.LatestVersion != skill.InstalledVersion
	return skill
}

func availableUpdateFromRow(row availableUpdateRow) AvailableUpdate {
	return AvailableUpdate{
		SkillID:         row.SkillID,
		CurrentVersion:  row.CurrentVersion,
		LatestVersion:   nullableStringValue(row.LatestVersion),
		CurrentChecksum: nullableStringValue(row.CurrentChecksum),
		LatestChecksum:  nullableStringValue(row.LatestChecksum),
		Action:          nullableStringValue(row.Action),
		CheckedAt:       parseTime(nullableStringValue(row.CheckedAt)),
	}
}

func availableUpdateFromMapRow(row z.V) AvailableUpdate {
	return AvailableUpdate{
		SkillID:         skillmarketStringFromMapValue(row, "skill_id"),
		CurrentVersion:  skillmarketStringFromMapValue(row, "current_version"),
		LatestVersion:   skillmarketStringFromMapValue(row, "latest_version"),
		CurrentChecksum: skillmarketStringFromMapValue(row, "current_checksum"),
		LatestChecksum:  skillmarketStringFromMapValue(row, "latest_checksum"),
		Action:          skillmarketStringFromMapValue(row, "action"),
		CheckedAt:       parseTime(skillmarketStringFromMapValue(row, "checked_at")),
	}
}

func supportsFTS5(db *sql.DB) bool {
	if db == nil {
		return false
	}
	rows, err := db.Query(`SELECT name FROM pragma_module_list WHERE name = 'fts5'`)
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			return true
		}
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS temp.skillmarket_fts_probe USING fts5(content)`); err != nil {
		return false
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS temp.skillmarket_fts_probe`)
	return true
}

func dropFTSTriggers(db *sql.DB) error {
	for _, stmt := range []string{
		`DROP TRIGGER IF EXISTS skillmarket_ai`,
		`DROP TRIGGER IF EXISTS skillmarket_ad`,
		`DROP TRIGGER IF EXISTS skillmarket_au`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func dropLegacySkillTableFTSTriggers(db *sql.DB) error {
	if db == nil {
		return nil
	}
	for _, stmt := range []string{
		`DROP TRIGGER IF EXISTS skills_ai`,
		`DROP TRIGGER IF EXISTS skills_ad`,
		`DROP TRIGGER IF EXISTS skills_au`,
		`DROP TRIGGER IF EXISTS skillmarket_ai`,
		`DROP TRIGGER IF EXISTS skillmarket_ad`,
		`DROP TRIGGER IF EXISTS skillmarket_au`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func (s *Store) initSchema() error {
	coreSchema := `
	CREATE TABLE IF NOT EXISTS skills (
		id TEXT PRIMARY KEY,
		slug TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		author TEXT,
		repo_url TEXT,
		homepage TEXT,
		download_url TEXT,
		stars INTEGER DEFAULT 0,
		downloads INTEGER DEFAULT 0,
		tags TEXT,
		category TEXT,
		security_score INTEGER DEFAULT 100,
		permissions TEXT,
		latest_version TEXT,
		risk_level TEXT,
		security_badge TEXT DEFAULT 'yellow',
		installable INTEGER DEFAULT 0,
		install_type TEXT DEFAULT 'manual_external',
		artifact_kind TEXT DEFAULT 'unknown',
		vulnerability_status TEXT DEFAULT 'unknown',
		has_vulnerabilities INTEGER DEFAULT 0,
		has_prompt_injection INTEGER DEFAULT 0,
		has_shell_injection INTEGER DEFAULT 0,
		has_data_exfiltration INTEGER DEFAULT 0,
		has_binary INTEGER DEFAULT 0,
		has_scripts INTEGER DEFAULT 0,
		popularity_score REAL DEFAULT 0,
		trending_score REAL DEFAULT 0,
		scan_status TEXT,
		content_sha256 TEXT,
		published INTEGER DEFAULT 1,
		source_id TEXT,
		source_name TEXT,
		source_group TEXT,
		source_type TEXT,
		skill_path TEXT,
		skill_content TEXT,
		embedding_json TEXT,
		embedding_model TEXT,
		curated_rank INTEGER DEFAULT 0,
		curated_boost REAL DEFAULT 0,
		curated_label TEXT,
		curated_reason TEXT,
		last_updated DATETIME,
		last_crawled_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS skill_versions (
		id TEXT PRIMARY KEY,
		skill_id TEXT NOT NULL,
		version TEXT NOT NULL,
		commit_hash TEXT,
		source_url TEXT,
		checksum TEXT,
		skill_path TEXT,
		raw_skill_md TEXT,
		manifest_json TEXT,
		released_at DATETIME,
		scanned_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME,
		UNIQUE(skill_id, version)
	);
	CREATE TABLE IF NOT EXISTS skill_security_reports (
		id TEXT PRIMARY KEY,
		skill_version_id TEXT NOT NULL UNIQUE,
		skill_id TEXT NOT NULL,
		version TEXT NOT NULL,
		score INTEGER NOT NULL,
		risk_level TEXT NOT NULL,
		security_badge TEXT DEFAULT 'yellow',
		vulnerability_status TEXT DEFAULT 'unknown',
		risks_json TEXT,
		permissions_json TEXT,
		secrets_json TEXT,
		vulnerabilities_json TEXT,
		install_surface_json TEXT,
		evidence_json TEXT,
		artifact_kind TEXT,
		has_prompt_injection INTEGER DEFAULT 0,
		has_shell_injection INTEGER DEFAULT 0,
		has_data_exfiltration INTEGER DEFAULT 0,
		scanner_version TEXT,
		llm_status TEXT,
		llm_verdict_json TEXT,
		created_at DATETIME,
		updated_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS skill_sources (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		base_url TEXT NOT NULL,
		display_name TEXT,
		source_group TEXT,
		mirror_of TEXT,
		auth_mode TEXT,
		headers_json TEXT,
		enabled INTEGER DEFAULT 1,
		last_cursor TEXT,
		etag TEXT,
		rate_limit_per_minute INTEGER DEFAULT 30,
		priority INTEGER DEFAULT 100,
		last_success_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS crawl_runs (
		id TEXT PRIMARY KEY,
		source_id TEXT NOT NULL,
		status TEXT NOT NULL,
		discovered INTEGER DEFAULT 0,
		updated INTEGER DEFAULT 0,
		failed INTEGER DEFAULT 0,
		started_at DATETIME,
		finished_at DATETIME,
		error_text TEXT
	);
	CREATE TABLE IF NOT EXISTS skill_telemetry_daily (
		id TEXT PRIMARY KEY,
		skill_id TEXT NOT NULL,
		day TEXT NOT NULL,
		downloads INTEGER DEFAULT 0,
		installs INTEGER DEFAULT 0,
		active_skills INTEGER DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME,
		UNIQUE(skill_id, day)
	);
	CREATE TABLE IF NOT EXISTS installed_skills (
		skill_id TEXT PRIMARY KEY,
		installed_version TEXT NOT NULL,
		checksum TEXT,
		source_url TEXT,
		enabled INTEGER DEFAULT 1,
		auto_update INTEGER DEFAULT 0,
		installed_at DATETIME,
		updated_at DATETIME,
		last_security_score INTEGER DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS skill_update_checks (
		skill_id TEXT PRIMARY KEY,
		checked_at DATETIME,
		latest_version TEXT,
		latest_checksum TEXT,
		action TEXT
	);
	CREATE TABLE IF NOT EXISTS skill_curations (
		id TEXT PRIMARY KEY,
		skill_id TEXT NOT NULL UNIQUE,
		hidden INTEGER DEFAULT 0,
		featured_rank INTEGER DEFAULT 0,
		boost_weight REAL DEFAULT 0,
		label TEXT,
		reason TEXT,
		created_at DATETIME,
		updated_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS curation_sync_state (
		id TEXT PRIMARY KEY,
		source_url TEXT,
		checksum TEXT,
		last_success_at DATETIME,
		last_error TEXT,
		updated_at DATETIME
	)
	`
	if _, err := s.db.Exec(coreSchema); err != nil {
		return fmt.Errorf("init skillmarket schema: %w", err)
	}

	ftsSupported := supportsFTS5(s.db)

	columnDefs := map[string]map[string]string{
		"skills": {
			"slug":                  "TEXT DEFAULT ''",
			"repo_url":              "TEXT DEFAULT ''",
			"homepage":              "TEXT",
			"download_url":          "TEXT",
			"security_score":        "INTEGER DEFAULT 100",
			"permissions":           "TEXT DEFAULT '[]'",
			"latest_version":        "TEXT DEFAULT ''",
			"risk_level":            "TEXT DEFAULT 'unknown'",
			"source_name":           "TEXT",
			"source_group":          "TEXT",
			"source_type":           "TEXT DEFAULT ''",
			"security_badge":        "TEXT DEFAULT 'yellow'",
			"installable":           "INTEGER DEFAULT 0",
			"install_type":          "TEXT DEFAULT 'manual_external'",
			"artifact_kind":         "TEXT DEFAULT 'unknown'",
			"vulnerability_status":  "TEXT DEFAULT 'unknown'",
			"has_vulnerabilities":   "INTEGER DEFAULT 0",
			"has_prompt_injection":  "INTEGER DEFAULT 0",
			"has_shell_injection":   "INTEGER DEFAULT 0",
			"has_data_exfiltration": "INTEGER DEFAULT 0",
			"has_binary":            "INTEGER DEFAULT 0",
			"has_scripts":           "INTEGER DEFAULT 0",
			"popularity_score":      "REAL DEFAULT 0",
			"trending_score":        "REAL DEFAULT 0",
			"scan_status":           "TEXT DEFAULT ''",
			"content_sha256":        "TEXT DEFAULT ''",
			"published":             "INTEGER DEFAULT 1",
			"skill_path":            "TEXT DEFAULT ''",
			"skill_content":         "TEXT DEFAULT ''",
			"embedding_json":        "TEXT DEFAULT ''",
			"embedding_model":       "TEXT DEFAULT ''",
			"curated_rank":          "INTEGER DEFAULT 0",
			"curated_boost":         "REAL DEFAULT 0",
			"curated_label":         "TEXT",
			"curated_reason":        "TEXT",
			"last_updated":          "DATETIME",
			"last_crawled_at":       "DATETIME",
		},
		"skill_security_reports": {
			"security_badge":        "TEXT DEFAULT 'yellow'",
			"vulnerability_status":  "TEXT DEFAULT 'unknown'",
			"vulnerabilities_json":  "TEXT",
			"install_surface_json":  "TEXT",
			"evidence_json":         "TEXT",
			"artifact_kind":         "TEXT",
			"has_prompt_injection":  "INTEGER DEFAULT 0",
			"has_shell_injection":   "INTEGER DEFAULT 0",
			"has_data_exfiltration": "INTEGER DEFAULT 0",
		},
		"skill_sources": {
			"display_name": "TEXT",
			"source_group": "TEXT",
			"mirror_of":    "TEXT",
			"headers_json": "TEXT",
			"priority":     "INTEGER DEFAULT 100",
		},
	}
	for table, defs := range columnDefs {
		for column, definition := range defs {
			if err := ensureColumn(s.db, table, column, definition); err != nil {
				return err
			}
		}
	}

	if !ftsSupported {
		if err := dropLegacySkillTableFTSTriggers(s.db); err != nil {
			return fmt.Errorf("drop legacy skill FTS triggers: %w", err)
		}
	}

	if _, err := s.db.Exec(`UPDATE skills SET slug = id WHERE COALESCE(slug, '') = ''`); err != nil {
		return err
	}

	indexSchema := `
	CREATE INDEX IF NOT EXISTS idx_skillmarket_category ON skills(category);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_trending ON skills(trending_score DESC);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_updated ON skills(last_updated DESC);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_downloads ON skills(downloads DESC);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_latest_version ON skills(latest_version);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_source_group ON skills(source_group);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_source_name ON skills(source_name);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_badge ON skills(security_badge);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_install_type ON skills(install_type);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_artifact_kind ON skills(artifact_kind);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_curated_rank ON skills(curated_rank DESC);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_versions_skill ON skill_versions(skill_id);
	CREATE INDEX IF NOT EXISTS idx_skillmarket_reports_skill ON skill_security_reports(skill_id);
	`
	if _, err := s.db.Exec(indexSchema); err != nil {
		return fmt.Errorf("init skillmarket indexes: %w", err)
	}

	if !ftsSupported {
		s.ftsEnabled = false
		return nil
	}

	if err := dropFTSTriggers(s.db); err != nil {
		return err
	}
	s.ftsEnabled = true
	ftsStatements := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS skillmarket_fts USING fts5(
			id UNINDEXED,
			name,
			description,
			author,
			category,
			tags,
			skill_content,
			content='skills',
			content_rowid='rowid'
		)`,
		`CREATE TRIGGER IF NOT EXISTS skillmarket_ai AFTER INSERT ON skills BEGIN
			INSERT INTO skillmarket_fts(rowid, id, name, description, author, category, tags, skill_content)
			VALUES (new.rowid, new.id, new.name, new.description, new.author, new.category, new.tags, new.skill_content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS skillmarket_ad AFTER DELETE ON skills BEGIN
			INSERT INTO skillmarket_fts(skillmarket_fts, rowid, id, name, description, author, category, tags, skill_content)
			VALUES ('delete', old.rowid, old.id, old.name, old.description, old.author, old.category, old.tags, old.skill_content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS skillmarket_au AFTER UPDATE ON skills BEGIN
			INSERT INTO skillmarket_fts(skillmarket_fts, rowid, id, name, description, author, category, tags, skill_content)
			VALUES ('delete', old.rowid, old.id, old.name, old.description, old.author, old.category, old.tags, old.skill_content);
			INSERT INTO skillmarket_fts(rowid, id, name, description, author, category, tags, skill_content)
			VALUES (new.rowid, new.id, new.name, new.description, new.author, new.category, new.tags, new.skill_content);
		END`,
	}
	for _, stmt := range ftsStatements {
		if _, err := s.db.Exec(stmt); err != nil {
			s.ftsEnabled = false
			return nil
		}
	}
	return nil
}

func (s *Store) normalizeLegacyCategories(ctx context.Context) error {
	var rows []skillCategoryNormalizationRow
	if _, err := s.readTable(ctx, "skills").Select(&rows,
		z.Fields("id", "category", "tags", "skill_content"),
	); err != nil {
		return err
	}
	type update struct {
		id       string
		category string
	}
	updates := make([]update, 0, len(rows))
	for i := range rows {
		normalized := normalizeCategory(
			nullableStringValue(rows[i].Category),
			nullableStringValue(rows[i].SkillContent),
			decodeStrings(nullableStringValue(rows[i].Tags)),
		)
		currentCategory := nullableStringValue(rows[i].Category)
		if normalized == "" || normalized == currentCategory {
			continue
		}
		updates = append(updates, update{id: rows[i].ID, category: normalized})
	}
	if len(updates) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range updates {
		if _, err := z.TableContext(ctx, tx, "skills").Update(
			z.V{
				"category":   item.category,
				"updated_at": timeutil.NowTime(),
			},
			z.Fields("category", "updated_at"),
			z.Where(z.Eq("id", item.id)),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func encodeStrings(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func decodeStrings(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if json.Unmarshal([]byte(raw), &values) == nil {
		return values
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func encodeJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func decodeFindings(raw string) []SecurityFinding {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []SecurityFinding
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func (s *Store) UpsertSkill(ctx context.Context, doc *SkillDocument, version *SkillVersion, report *SecurityReport) error {
	_, err := s.UpsertSkillBatch(ctx, []*SkillUpsertRecord{{
		Doc:     doc,
		Version: version,
		Report:  report,
	}})
	return err
}

func (s *Store) UpsertSkillBatch(ctx context.Context, records []*SkillUpsertRecord) (*SkillBatchUpsertResult, error) {
	records = dedupeSkillUpsertRecords(records)
	if len(records) == 0 {
		return &SkillBatchUpsertResult{}, nil
	}

	now := timeutil.NowTime()
	for _, record := range records {
		normalizeSkillUpsertRecord(record, now)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	existingSkillIDs, err := s.fetchExistingSkillIDs(ctx, tx, records)
	if err != nil {
		return nil, err
	}
	result := &SkillBatchUpsertResult{}
	newRecords := make([]*SkillUpsertRecord, 0, len(records))
	updateRecords := make([]*SkillUpsertRecord, 0, len(records))
	appliedRecords := make([]*SkillUpsertRecord, 0, len(records))
	for _, record := range records {
		if record == nil || record.Doc == nil {
			continue
		}
		if err := s.applySkillCuration(ctx, tx, record.Doc); err != nil {
			return nil, err
		}
		if _, exists := existingSkillIDs[record.Doc.ID]; !exists {
			newRecords = append(newRecords, record)
			appliedRecords = append(appliedRecords, record)
			result.Inserted++
			continue
		}
		shouldReplaceDoc, err := shouldReplaceSkillDocument(ctx, tx, record.Doc.ID, record.Doc.SourceID)
		if err != nil {
			return nil, err
		}
		if !shouldReplaceDoc {
			result.Skipped++
			continue
		}
		updateRecords = append(updateRecords, record)
		appliedRecords = append(appliedRecords, record)
		result.Updated++
	}

	if err := s.insertSkillDocsIgnore(ctx, tx, newRecords); err != nil {
		return nil, err
	}
	if err := s.updateSkillDocs(ctx, tx, updateRecords); err != nil {
		return nil, err
	}

	existingVersionIDs, err := s.fetchExistingVersionIDs(ctx, tx, appliedRecords)
	if err != nil {
		return nil, err
	}
	newVersionRecords := make([]*SkillUpsertRecord, 0, len(appliedRecords))
	updateVersionRecords := make([]*SkillUpsertRecord, 0, len(appliedRecords))
	for _, record := range appliedRecords {
		if record == nil || record.Doc == nil || record.Version == nil {
			continue
		}
		key := skillVersionKey(record.Version.SkillID, record.Version.Version)
		if existingID, exists := existingVersionIDs[key]; exists {
			record.Version.ID = existingID
			updateVersionRecords = append(updateVersionRecords, record)
			continue
		}
		newVersionRecords = append(newVersionRecords, record)
	}
	if err := s.insertSkillVersionsIgnore(ctx, tx, newVersionRecords); err != nil {
		return nil, err
	}
	if err := s.updateSkillVersions(ctx, tx, updateVersionRecords); err != nil {
		return nil, err
	}

	existingReportIDs, err := s.fetchExistingReportIDs(ctx, tx, appliedRecords)
	if err != nil {
		return nil, err
	}
	newReportRecords := make([]*SkillUpsertRecord, 0, len(appliedRecords))
	updateReportRecords := make([]*SkillUpsertRecord, 0, len(appliedRecords))
	for _, record := range appliedRecords {
		if record == nil || record.Report == nil || record.Version == nil {
			continue
		}
		record.Report.SkillVersionID = record.Version.ID
		record.Report.SkillID = record.Version.SkillID
		record.Report.Version = record.Version.Version
		if existingID, exists := existingReportIDs[record.Version.ID]; exists {
			record.Report.ID = existingID
			updateReportRecords = append(updateReportRecords, record)
			continue
		}
		newReportRecords = append(newReportRecords, record)
	}
	if err := s.insertSkillReportsIgnore(ctx, tx, newReportRecords); err != nil {
		return nil, err
	}
	if err := s.updateSkillReports(ctx, tx, updateReportRecords); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func dedupeSkillUpsertRecords(records []*SkillUpsertRecord) []*SkillUpsertRecord {
	if len(records) <= 1 {
		return records
	}
	indexByID := make(map[string]int, len(records))
	deduped := make([]*SkillUpsertRecord, 0, len(records))
	for _, record := range records {
		if record == nil || record.Doc == nil {
			continue
		}
		id := strings.TrimSpace(record.Doc.ID)
		if id == "" {
			deduped = append(deduped, record)
			continue
		}
		if existingIndex, exists := indexByID[id]; exists {
			deduped[existingIndex] = record
			continue
		}
		indexByID[id] = len(deduped)
		deduped = append(deduped, record)
	}
	return deduped
}

func normalizeSkillUpsertRecord(record *SkillUpsertRecord, now time.Time) {
	if record == nil || record.Doc == nil {
		return
	}
	doc := record.Doc
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	if doc.LastUpdated.IsZero() {
		doc.LastUpdated = now
	}
	if doc.LastCrawledAt.IsZero() {
		doc.LastCrawledAt = now
	}
	doc.PopularityScore = computePopularityScore(doc.Stars, doc.Downloads)
	doc.TrendingScore = computeTrendingScore(doc.Stars, doc.Downloads, doc.LastUpdated, doc.SecurityScore)
	if strings.TrimSpace(doc.InstallType) == "" {
		if strings.TrimSpace(doc.SkillContent) != "" {
			doc.InstallType = InstallTypeRawSkill
			doc.Installable = true
		} else {
			doc.InstallType = InstallTypeManualExternal
		}
	}
	if strings.TrimSpace(doc.ArtifactKind) == "" {
		if doc.InstallType == InstallTypeManualExternal {
			doc.ArtifactKind = ArtifactKindUnknown
		} else {
			doc.ArtifactKind = ArtifactKindOpenSource
		}
	}
	if strings.TrimSpace(doc.SecurityBadge) == "" {
		switch doc.RiskLevel {
		case RiskHigh, RiskCritical:
			doc.SecurityBadge = BadgeRed
		case RiskMedium:
			doc.SecurityBadge = BadgeYellow
		default:
			doc.SecurityBadge = BadgeGreen
		}
	}
	if strings.TrimSpace(doc.VulnerabilityStatus) == "" {
		doc.VulnerabilityStatus = VulnerabilityStatusNotApplicable
	}

	if record.Version != nil {
		if record.Version.ID == "" {
			record.Version.ID = uuid.NewString()
		}
		if record.Version.SkillID == "" {
			record.Version.SkillID = doc.ID
		}
		if record.Version.CreatedAt.IsZero() {
			record.Version.CreatedAt = now
		}
		record.Version.UpdatedAt = now
		if record.Version.ReleasedAt.IsZero() {
			record.Version.ReleasedAt = doc.LastUpdated
		}
		if record.Version.ScannedAt.IsZero() {
			record.Version.ScannedAt = now
		}
	}

	if record.Report != nil {
		if record.Report.ID == "" {
			record.Report.ID = uuid.NewString()
		}
		if strings.TrimSpace(record.Report.RiskLevel) == "" {
			record.Report.RiskLevel = doc.RiskLevel
		}
		if strings.TrimSpace(record.Report.SecurityBadge) == "" {
			record.Report.SecurityBadge = doc.SecurityBadge
		}
		if strings.TrimSpace(record.Report.VulnerabilityStatus) == "" {
			record.Report.VulnerabilityStatus = doc.VulnerabilityStatus
		}
		if strings.TrimSpace(record.Report.InstallSurface.InstallType) == "" {
			record.Report.InstallSurface.InstallType = doc.InstallType
		}
		if strings.TrimSpace(record.Report.InstallSurface.ArtifactKind) == "" {
			record.Report.InstallSurface.ArtifactKind = doc.ArtifactKind
		}
		if !record.Report.InstallSurface.Installable {
			record.Report.InstallSurface.Installable = doc.Installable
		}
		if !record.Report.InstallSurface.HasBinary {
			record.Report.InstallSurface.HasBinary = doc.HasBinary
		}
		if !record.Report.InstallSurface.HasScripts {
			record.Report.InstallSurface.HasScripts = doc.HasScripts
		}
		if record.Report.CreatedAt.IsZero() {
			record.Report.CreatedAt = now
		}
		record.Report.UpdatedAt = now
	}
}

func (s *Store) applySkillCuration(ctx context.Context, tx *sql.Tx, doc *SkillDocument) error {
	if doc == nil {
		return nil
	}
	var rows []skillCurationRow
	if _, err := z.TableContext(ctx, tx, "skill_curations").Select(&rows,
		z.Fields("hidden", "featured_rank", "boost_weight", "label", "reason"),
		z.Where(z.Eq("skill_id", doc.ID)),
		z.Limit(1),
	); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	doc.CuratedRank = rows[0].FeaturedRank
	doc.CuratedBoost = rows[0].BoostWeight
	doc.CuratedLabel = nullableStringValue(rows[0].Label)
	doc.CuratedReason = nullableStringValue(rows[0].Reason)
	if rows[0].Hidden == 1 {
		doc.Published = false
	}
	return nil
}

func collectSkillIDs(records []*SkillUpsertRecord) []string {
	seen := make(map[string]struct{}, len(records))
	ids := make([]string, 0, len(records))
	for _, record := range records {
		if record == nil || record.Doc == nil {
			continue
		}
		id := strings.TrimSpace(record.Doc.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func (s *Store) fetchExistingSkillIDs(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) (map[string]struct{}, error) {
	ids := collectSkillIDs(records)
	result := make(map[string]struct{}, len(ids))
	for start := 0; start < len(ids); start += sqliteSafeMaxBindVars {
		end := start + sqliteSafeMaxBindVars
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		var rows []skillIDRow
		if _, err := z.TableContext(ctx, tx, "skills").Select(&rows,
			z.Fields("id"),
			z.Where(z.In("id", chunk)),
		); err != nil {
			return nil, err
		}
		for i := range rows {
			result[rows[i].ID] = struct{}{}
		}
	}
	return result, nil
}

func maxRowsPerInsert(columnsPerRow int) int {
	if columnsPerRow <= 0 {
		return 1
	}
	rows := sqliteSafeMaxBindVars / columnsPerRow
	if rows < 1 {
		return 1
	}
	return rows
}

func groupedPlaceholders(rows, columns int) string {
	if rows <= 0 || columns <= 0 {
		return ""
	}
	group := "(" + placeholders(columns) + ")"
	return strings.TrimSuffix(strings.Repeat(group+",", rows), ",")
}

func (s *Store) insertSkillDocsIgnore(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	rows := make([]skillDocumentWriteRow, 0, len(records))
	for _, record := range records {
		if record == nil || record.Doc == nil {
			continue
		}
		rows = append(rows, skillDocumentWriteRowFromDoc(record.Doc))
	}
	if len(rows) == 0 {
		return nil
	}
	_, err := z.TableContext(ctx, tx, "skills").InsertIgnore(&rows)
	return err
}

func (s *Store) updateSkillDocs(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	for _, record := range records {
		if record == nil || record.Doc == nil {
			continue
		}
		doc := record.Doc
		if _, err := z.TableContext(ctx, tx, "skills").Update(
			z.V{
				"slug":                  doc.Slug,
				"name":                  doc.Name,
				"description":           doc.Description,
				"author":                doc.Author,
				"repo_url":              doc.RepoURL,
				"homepage":              doc.Homepage,
				"download_url":          doc.DownloadURL,
				"stars":                 doc.Stars,
				"downloads":             doc.Downloads,
				"tags":                  encodeStrings(doc.Tags),
				"category":              doc.Category,
				"security_score":        doc.SecurityScore,
				"permissions":           encodeStrings(doc.Permissions),
				"latest_version":        doc.LatestVersion,
				"risk_level":            doc.RiskLevel,
				"security_badge":        doc.SecurityBadge,
				"installable":           boolToInt(doc.Installable),
				"install_type":          doc.InstallType,
				"artifact_kind":         doc.ArtifactKind,
				"vulnerability_status":  doc.VulnerabilityStatus,
				"has_vulnerabilities":   boolToInt(doc.HasVulnerabilities),
				"has_prompt_injection":  boolToInt(doc.HasPromptInjection),
				"has_shell_injection":   boolToInt(doc.HasShellInjection),
				"has_data_exfiltration": boolToInt(doc.HasDataExfiltration),
				"has_binary":            boolToInt(doc.HasBinary),
				"has_scripts":           boolToInt(doc.HasScripts),
				"popularity_score":      doc.PopularityScore,
				"trending_score":        doc.TrendingScore,
				"scan_status":           doc.ScanStatus,
				"content_sha256":        doc.ContentSHA256,
				"published":             boolToInt(doc.Published),
				"source_id":             doc.SourceID,
				"source_name":           doc.SourceName,
				"source_group":          doc.SourceGroup,
				"source_type":           doc.SourceType,
				"skill_path":            doc.SkillPath,
				"skill_content":         doc.SkillContent,
				"embedding_json":        doc.EmbeddingJSON,
				"embedding_model":       doc.EmbeddingModel,
				"curated_rank":          doc.CuratedRank,
				"curated_boost":         doc.CuratedBoost,
				"curated_label":         doc.CuratedLabel,
				"curated_reason":        doc.CuratedReason,
				"last_updated":          doc.LastUpdated,
				"last_crawled_at":       doc.LastCrawledAt,
				"updated_at":            doc.UpdatedAt,
			},
			z.Fields(
				"slug",
				"name",
				"description",
				"author",
				"repo_url",
				"homepage",
				"download_url",
				"stars",
				"downloads",
				"tags",
				"category",
				"security_score",
				"permissions",
				"latest_version",
				"risk_level",
				"security_badge",
				"installable",
				"install_type",
				"artifact_kind",
				"vulnerability_status",
				"has_vulnerabilities",
				"has_prompt_injection",
				"has_shell_injection",
				"has_data_exfiltration",
				"has_binary",
				"has_scripts",
				"popularity_score",
				"trending_score",
				"scan_status",
				"content_sha256",
				"published",
				"source_id",
				"source_name",
				"source_group",
				"source_type",
				"skill_path",
				"skill_content",
				"embedding_json",
				"embedding_model",
				"curated_rank",
				"curated_boost",
				"curated_label",
				"curated_reason",
				"last_updated",
				"last_crawled_at",
				"updated_at",
			),
			z.Where(z.Eq("id", doc.ID)),
		); err != nil {
			return err
		}
	}
	return nil
}

func skillVersionKey(skillID, version string) string {
	return skillID + "\x00" + version
}

func (s *Store) fetchExistingVersionIDs(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) (map[string]string, error) {
	skillIDs := make([]string, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record == nil || record.Version == nil {
			continue
		}
		id := strings.TrimSpace(record.Version.SkillID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		skillIDs = append(skillIDs, id)
	}
	result := make(map[string]string, len(skillIDs))
	for start := 0; start < len(skillIDs); start += sqliteSafeMaxBindVars {
		end := start + sqliteSafeMaxBindVars
		if end > len(skillIDs) {
			end = len(skillIDs)
		}
		chunk := skillIDs[start:end]
		var rows []skillVersionLookupRow
		if _, err := z.TableContext(ctx, tx, "skill_versions").Select(&rows,
			z.Fields("id", "skill_id", "version"),
			z.Where(z.In("skill_id", chunk)),
		); err != nil {
			return nil, err
		}
		for i := range rows {
			result[skillVersionKey(rows[i].SkillID, rows[i].Version)] = rows[i].ID
		}
	}
	return result, nil
}

func (s *Store) insertSkillVersionsIgnore(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	rows := make([]skillVersionWriteRow, 0, len(records))
	for _, record := range records {
		if record == nil || record.Version == nil {
			continue
		}
		rows = append(rows, skillVersionWriteRowFromVersion(record.Version))
	}
	if len(rows) == 0 {
		return nil
	}
	_, err := z.TableContext(ctx, tx, "skill_versions").InsertIgnore(&rows)
	return err
}

func (s *Store) updateSkillVersions(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	for _, record := range records {
		if record == nil || record.Version == nil {
			continue
		}
		version := record.Version
		if _, err := z.TableContext(ctx, tx, "skill_versions").Update(
			z.V{
				"commit_hash":   version.CommitHash,
				"source_url":    version.SourceURL,
				"checksum":      version.Checksum,
				"skill_path":    version.SkillPath,
				"raw_skill_md":  version.RawSkillMD,
				"manifest_json": version.ManifestJSON,
				"released_at":   version.ReleasedAt,
				"scanned_at":    version.ScannedAt,
				"updated_at":    version.UpdatedAt,
			},
			z.Fields(
				"commit_hash",
				"source_url",
				"checksum",
				"skill_path",
				"raw_skill_md",
				"manifest_json",
				"released_at",
				"scanned_at",
				"updated_at",
			),
			z.Where(z.Eq("skill_id", version.SkillID), z.Eq("version", version.Version)),
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) fetchExistingReportIDs(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) (map[string]string, error) {
	versionIDs := make([]string, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record == nil || record.Version == nil {
			continue
		}
		id := strings.TrimSpace(record.Version.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		versionIDs = append(versionIDs, id)
	}
	result := make(map[string]string, len(versionIDs))
	for start := 0; start < len(versionIDs); start += sqliteSafeMaxBindVars {
		end := start + sqliteSafeMaxBindVars
		if end > len(versionIDs) {
			end = len(versionIDs)
		}
		chunk := versionIDs[start:end]
		var rows []skillReportLookupRow
		if _, err := z.TableContext(ctx, tx, "skill_security_reports").Select(&rows,
			z.Fields("id", "skill_version_id"),
			z.Where(z.In("skill_version_id", chunk)),
		); err != nil {
			return nil, err
		}
		for i := range rows {
			result[rows[i].SkillVersionID] = rows[i].ID
		}
	}
	return result, nil
}

func (s *Store) insertSkillReportsIgnore(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	rows := make([]securityReportWriteRow, 0, len(records))
	for _, record := range records {
		if record == nil || record.Report == nil {
			continue
		}
		rows = append(rows, securityReportWriteRowFromReport(record.Report))
	}
	if len(rows) == 0 {
		return nil
	}
	_, err := z.TableContext(ctx, tx, "skill_security_reports").InsertIgnore(&rows)
	return err
}

func (s *Store) updateSkillReports(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	for _, record := range records {
		if record == nil || record.Report == nil {
			continue
		}
		report := record.Report
		if _, err := z.TableContext(ctx, tx, "skill_security_reports").Update(
			z.V{
				"score":                 report.Score,
				"risk_level":            report.RiskLevel,
				"security_badge":        report.SecurityBadge,
				"vulnerability_status":  report.VulnerabilityStatus,
				"risks_json":            encodeJSON(report.Risks),
				"permissions_json":      encodeStrings(report.Permissions),
				"secrets_json":          encodeStrings(report.Secrets),
				"vulnerabilities_json":  encodeStrings(report.Vulnerabilities),
				"install_surface_json":  encodeJSON(report.InstallSurface),
				"evidence_json":         encodeJSON(report.Evidence),
				"artifact_kind":         report.InstallSurface.ArtifactKind,
				"has_prompt_injection":  boolToInt(report.HasPromptInjection),
				"has_shell_injection":   boolToInt(report.HasShellInjection),
				"has_data_exfiltration": boolToInt(report.HasDataExfiltration),
				"scanner_version":       report.ScannerVersion,
				"llm_status":            report.LLMStatus,
				"llm_verdict_json":      report.LLMVerdictJSON,
				"updated_at":            report.UpdatedAt,
			},
			z.Fields(
				"score",
				"risk_level",
				"security_badge",
				"vulnerability_status",
				"risks_json",
				"permissions_json",
				"secrets_json",
				"vulnerabilities_json",
				"install_surface_json",
				"evidence_json",
				"artifact_kind",
				"has_prompt_injection",
				"has_shell_injection",
				"has_data_exfiltration",
				"scanner_version",
				"llm_status",
				"llm_verdict_json",
				"updated_at",
			),
			z.Where(z.Eq("skill_version_id", report.SkillVersionID)),
		); err != nil {
			return err
		}
	}
	return nil
}

func shouldReplaceSkillDocument(ctx context.Context, tx *sql.Tx, skillID, incomingSourceID string) (bool, error) {
	if strings.TrimSpace(skillID) == "" {
		return true, nil
	}
	var rows []skillSourceIDRow
	if _, err := z.TableContext(ctx, tx, "skills").Select(&rows,
		z.Fields("source_id"),
		z.Where(z.Eq("id", skillID)),
		z.Limit(1),
	); err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return true, nil
	}
	existingSourceID := strings.TrimSpace(nullableStringValue(rows[0].SourceID))
	incomingSourceID = strings.TrimSpace(incomingSourceID)
	if existingSourceID == "" || incomingSourceID == "" || existingSourceID == incomingSourceID {
		return true, nil
	}

	existingPriority, err := lookupSourcePriority(ctx, tx, existingSourceID)
	if err != nil {
		return false, err
	}
	incomingPriority, err := lookupSourcePriority(ctx, tx, incomingSourceID)
	if err != nil {
		return false, err
	}
	return incomingPriority <= existingPriority, nil
}

func lookupSourcePriority(ctx context.Context, tx *sql.Tx, sourceID string) (int, error) {
	if strings.TrimSpace(sourceID) == "" {
		return math.MaxInt32, nil
	}
	var rows []sourcePriorityRow
	if _, err := z.TableContext(ctx, tx, "skill_sources").Select(&rows,
		z.Fields("priority"),
		z.Where(z.Eq("id", sourceID)),
		z.Limit(1),
	); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return math.MaxInt32, nil
	}
	return rows[0].Priority, nil
}

func (s *Store) UpsertSource(ctx context.Context, source Source) error {
	now := timeutil.NowTime()
	if source.CreatedAt.IsZero() {
		source.CreatedAt = now
	}
	source.UpdatedAt = now
	_, err := s.table(ctx, "skill_sources").Insert(
		z.V{
			"id":                    source.ID,
			"type":                  source.Type,
			"base_url":              source.BaseURL,
			"display_name":          source.DisplayName,
			"source_group":          source.SourceGroup,
			"mirror_of":             source.MirrorOf,
			"auth_mode":             source.AuthMode,
			"headers_json":          encodeJSON(source.Headers),
			"enabled":               boolToInt(source.Enabled),
			"last_cursor":           source.LastCursor,
			"etag":                  source.ETag,
			"rate_limit_per_minute": source.RateLimitPerMinute,
			"priority":              source.Priority,
			"last_success_at":       nullTime(source.LastSuccessAt),
			"created_at":            source.CreatedAt,
			"updated_at":            source.UpdatedAt,
		},
		z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{
				"type",
				"base_url",
				"display_name",
				"source_group",
				"mirror_of",
				"auth_mode",
				"headers_json",
				"enabled",
				"last_cursor",
				"etag",
				"rate_limit_per_minute",
				"priority",
				"last_success_at",
				"updated_at",
			},
		),
	)
	return err
}

func (s *Store) SetSourceEnabledByIdentity(ctx context.Context, sourceID, sourceType, baseURL string, enabled bool) error {
	_, err := s.table(ctx, "skill_sources").Update(
		z.V{
			"enabled":    boolToInt(enabled),
			"updated_at": timeutil.NowTime(),
		},
		z.Fields("enabled", "updated_at"),
		z.Where(
			z.Eq("id", sourceID),
			z.Eq("type", sourceType),
			z.Eq("base_url", baseURL),
		),
	)
	return err
}

func (s *Store) ListSources(ctx context.Context) ([]Source, error) {
	var rows []sourceRow
	if _, err := s.readTable(ctx, "skill_sources").Select(&rows,
		z.Fields(
			"id",
			"type",
			"base_url",
			"display_name",
			"source_group",
			"mirror_of",
			"auth_mode",
			"headers_json",
			"enabled",
			"last_cursor",
			"etag",
			"rate_limit_per_minute",
			"priority",
			"last_success_at",
			"created_at",
			"updated_at",
		),
		z.Where(z.Eq("enabled", 1)),
		z.OrderBy("priority ASC", "id ASC"),
	); err != nil {
		return nil, err
	}
	result := make([]Source, 0, len(rows))
	for i := range rows {
		result = append(result, sourceFromRow(rows[i]))
	}
	return result, nil
}

func (s *Store) BeginCrawlRun(ctx context.Context, sourceID string) (*CrawlRun, error) {
	run := &CrawlRun{
		ID:        uuid.NewString(),
		SourceID:  sourceID,
		Status:    "in_progress",
		StartedAt: timeutil.NowTime(),
	}
	_, err := s.table(ctx, "crawl_runs").Insert(z.V{
		"id":         run.ID,
		"source_id":  run.SourceID,
		"status":     run.Status,
		"discovered": 0,
		"updated":    0,
		"failed":     0,
		"started_at": run.StartedAt,
	})
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Store) CompleteCrawlRun(ctx context.Context, run *CrawlRun) error {
	run.FinishedAt = timeutil.NowTime()
	_, err := s.table(ctx, "crawl_runs").Update(
		z.V{
			"status":      run.Status,
			"discovered":  run.Discovered,
			"updated":     run.Updated,
			"failed":      run.Failed,
			"finished_at": run.FinishedAt,
			"error_text":  run.ErrorText,
		},
		z.Fields("status", "discovered", "updated", "failed", "finished_at", "error_text"),
		z.Where(z.Eq("id", run.ID)),
	)
	if err != nil {
		return err
	}
	if run.Status == "success" {
		_, _ = s.table(ctx, "skill_sources").Update(
			z.V{
				"last_success_at": run.FinishedAt,
				"updated_at":      run.FinishedAt,
			},
			z.Fields("last_success_at", "updated_at"),
			z.Where(z.Eq("id", run.SourceID)),
		)
	}
	return nil
}

func (s *Store) Search(ctx context.Context, query SearchQuery) (*SearchResponse, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > DefaultSearchLimit {
		query.PageSize = DefaultPageSize
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%s:%s:%d:%d", query.Query, query.Category, query.Sort, strings.Join(query.Sources, ","), query.Page, query.PageSize)
	s.searchCacheMu.RLock()
	if entry, ok := s.searchCache[cacheKey]; ok && time.Since(entry.timestamp) < s.searchCacheTTL {
		s.searchCacheMu.RUnlock()
		return entry.results, nil
	}
	s.searchCacheMu.RUnlock()
	searching := strings.TrimSpace(query.Query) != ""
	whereClause, args := buildSkillFilters("s", query)

	var total int
	switch {
	case searching && s.ftsEnabled:
		var rows []z.V
		if _, err := s.readTable(ctx, "skills s").Select(&rows,
			z.Fields("COUNT(*) AS total"),
			z.InnerJoin("skillmarket_fts", z.Expr("skillmarket_fts.rowid = s.rowid")),
			buildSkillFTSWhere(whereClause, args, escapeFTSQuery(query.Query)),
			z.Limit(1),
		); err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			total = skillmarketIntFromMapValue(rows[0], "total")
		}
	case searching:
		countWhere, countArgs := buildSearchWhereClause("s", whereClause, args, true, query.Query)
		countOpts := []z.ZormItem{z.Fields("count(1)")}
		if countWhere != "" {
			countOpts = append(countOpts, z.Where(append([]interface{}{countWhere}, countArgs...)...))
		}
		if _, err := s.readTable(ctx, "skills s").Select(&total, countOpts...); err != nil {
			return nil, err
		}
	default:
		countWhere, countArgs := buildSearchWhereClause("s", whereClause, args, false, query.Query)
		countOpts := []z.ZormItem{z.Fields("count(1)")}
		if countWhere != "" {
			countOpts = append(countOpts, z.Where(append([]interface{}{countWhere}, countArgs...)...))
		}
		if _, err := s.readTable(ctx, "skills s").Select(&total, countOpts...); err != nil {
			return nil, err
		}
	}

	sortBy := skillSortClause(query.Sort)

	offset := (query.Page - 1) * query.PageSize
	results := make([]SearchResult, 0, query.PageSize)

	if !searching || !s.ftsEnabled {
		searchWhere, searchArgs := buildSearchWhereClause("s", whereClause, args, searching, query.Query)
		searchOpts := []z.ZormItem{
			z.OrderBy(fallbackSearchOrder("s", searching, sortBy, query.Query)),
			z.Limit(query.PageSize, offset),
		}
		if searchWhere != "" {
			searchOpts = append([]z.ZormItem{z.Where(append([]interface{}{searchWhere}, searchArgs...)...)}, searchOpts...)
		}
		var rows []skillDocumentRow
		if _, err := s.readTable(ctx, "skills s").Select(&rows, searchOpts...); err != nil {
			return nil, err
		}
		for i := range rows {
			doc := skillDocumentFromRow(rows[i])
			results = append(results, SearchResult{Skill: doc, Score: doc.TrendingScore, MatchSource: "rank"})
		}
	} else {
		ftsQuery := escapeFTSQuery(query.Query)
		var rows []z.V
		if _, err := s.readTable(ctx, "skills s").Select(&rows,
			z.Fields(append(skillSelectMapFields("s"), skillFTSScoreExpr("s")+" AS score")...),
			z.InnerJoin("skillmarket_fts", z.Expr("skillmarket_fts.rowid = s.rowid")),
			buildSkillFTSWhere(whereClause, args, ftsQuery),
			z.OrderBy("score DESC"),
			z.Limit(query.PageSize, offset),
		); err != nil {
			return nil, err
		}
		for _, row := range rows {
			results = append(results, searchResultFromMapScoreRow(row))
		}
	}

	return &SearchResponse{
		Skills:     results,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: int(math.Ceil(float64(total) / float64(query.PageSize))),
	}, nil
}

func (s *Store) SearchKeywordCandidates(ctx context.Context, query SearchQuery) ([]SearchResult, error) {
	if strings.TrimSpace(query.Query) == "" {
		return nil, nil
	}

	whereClause, args := buildSkillFilters("s", query)
	results := make([]SearchResult, 0, DefaultSearchLimit)

	if !s.ftsEnabled {
		sortBy := skillSortClause(query.Sort)
		searchWhere, searchArgs := buildSearchWhereClause("s", whereClause, args, true, query.Query)
		searchOpts := []z.ZormItem{
			z.OrderBy(fallbackSearchOrder("s", true, sortBy, query.Query)),
		}
		if searchWhere != "" {
			searchOpts = append([]z.ZormItem{z.Where(append([]interface{}{searchWhere}, searchArgs...)...)}, searchOpts...)
		}
		var rows []skillDocumentRow
		if _, err := s.readTable(ctx, "skills s").Select(&rows, searchOpts...); err != nil {
			return nil, err
		}
		for i := range rows {
			doc := skillDocumentFromRow(rows[i])
			results = append(results, SearchResult{
				Skill:        doc,
				Score:        doc.TrendingScore,
				KeywordScore: doc.TrendingScore,
				MatchSource:  "rank",
			})
		}
		return results, nil
	}

	ftsQuery := escapeFTSQuery(query.Query)
	var rows []z.V
	if _, err := s.readTable(ctx, "skills s").Select(&rows,
		z.Fields(append(skillSelectMapFields("s"), skillFTSScoreExpr("s")+" AS score")...),
		z.InnerJoin("skillmarket_fts", z.Expr("skillmarket_fts.rowid = s.rowid")),
		buildSkillFTSWhere(whereClause, args, ftsQuery),
		z.OrderBy("score DESC"),
	); err != nil {
		return nil, err
	}
	for _, row := range rows {
		results = append(results, searchResultFromMapScoreRow(row))
	}
	return results, nil
}

func (s *Store) ListFilteredSkills(ctx context.Context, query SearchQuery) ([]SkillDocument, error) {
	whereClause, args := buildSkillFilters("skills", query)
	whereClause = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(whereClause), "WHERE"))
	opts := []z.ZormItem{
		z.OrderBy(skillSortClause(query.Sort)),
	}
	if whereClause != "" {
		whereArgs := append([]interface{}{whereClause}, args...)
		opts = append([]z.ZormItem{z.Where(whereArgs...)}, opts...)
	}
	var rows []skillDocumentRow
	if _, err := s.readTable(ctx, "skills").Select(&rows, opts...); err != nil {
		return nil, err
	}
	result := make([]SkillDocument, 0, len(rows))
	for i := range rows {
		result = append(result, skillDocumentFromRow(rows[i]))
	}
	return result, nil
}

func (s *Store) UpdateSkillEmbedding(ctx context.Context, skillID, embeddingJSON, embeddingModel string) error {
	_, err := s.table(ctx, "skills").Update(
		z.V{
			"embedding_json":  embeddingJSON,
			"embedding_model": embeddingModel,
			"updated_at":      timeutil.NowTime(),
		},
		z.Fields("embedding_json", "embedding_model", "updated_at"),
		z.Where(z.Eq("id", skillID)),
	)
	return err
}

func buildSkillFilters(alias string, query SearchQuery) (string, []interface{}) {
	prefix := alias + "."
	where := []string{prefix + "published = 1"}
	args := []interface{}{}

	categories := append([]string{}, query.Categories...)
	if strings.TrimSpace(query.Category) != "" {
		categories = append(categories, query.Category)
	}
	categories = normalizeUniqueStrings(categories)
	if len(categories) > 0 {
		where = append(where, prefix+`category IN (`+placeholders(len(categories))+`)`)
		for _, category := range categories {
			args = append(args, category)
		}
	}

	if len(query.Sources) > 0 {
		sourceValues := normalizeUniqueStrings(query.Sources)
		groupClause := prefix + `source_group IN (` + placeholders(len(sourceValues)) + `)`
		nameClause := prefix + `source_name IN (` + placeholders(len(sourceValues)) + `)`
		idClause := prefix + `source_id IN (` + placeholders(len(sourceValues)) + `)`
		where = append(where, `(`+groupClause+` OR `+nameClause+` OR `+idClause+`)`)
		for i := 0; i < 3; i++ {
			for _, value := range sourceValues {
				args = append(args, value)
			}
		}
	}

	if len(query.RiskBadges) > 0 {
		values := normalizeUniqueStrings(query.RiskBadges)
		where = append(where, prefix+`security_badge IN (`+placeholders(len(values))+`)`)
		for _, value := range values {
			args = append(args, value)
		}
	}
	if len(query.InstallTypes) > 0 {
		values := normalizeUniqueStrings(query.InstallTypes)
		where = append(where, prefix+`install_type IN (`+placeholders(len(values))+`)`)
		for _, value := range values {
			args = append(args, value)
		}
	}
	if len(query.ArtifactKinds) > 0 {
		values := normalizeUniqueStrings(query.ArtifactKinds)
		where = append(where, prefix+`artifact_kind IN (`+placeholders(len(values))+`)`)
		for _, value := range values {
			args = append(args, value)
		}
	}
	if query.Installable != nil {
		where = append(where, prefix+`installable = ?`)
		args = append(args, boolToInt(*query.Installable))
	}
	if query.Curated != nil {
		if *query.Curated {
			where = append(where, `(`+prefix+`curated_rank > 0 OR `+prefix+`curated_boost > 0)`)
		} else {
			where = append(where, prefix+`curated_rank = 0 AND `+prefix+`curated_boost = 0`)
		}
	}
	if query.OpenSourceOnly {
		where = append(where, prefix+`artifact_kind = ?`)
		args = append(args, ArtifactKindOpenSource)
	}
	if query.HasVulnerabilities != nil {
		where = append(where, prefix+`has_vulnerabilities = ?`)
		args = append(args, boolToInt(*query.HasVulnerabilities))
	}
	if query.HasPromptInjection != nil {
		where = append(where, prefix+`has_prompt_injection = ?`)
		args = append(args, boolToInt(*query.HasPromptInjection))
	}
	if query.HasShellInjection != nil {
		where = append(where, prefix+`has_shell_injection = ?`)
		args = append(args, boolToInt(*query.HasShellInjection))
	}
	if query.HasDataExfiltration != nil {
		where = append(where, prefix+`has_data_exfiltration = ?`)
		args = append(args, boolToInt(*query.HasDataExfiltration))
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func trimSQLClausePrefix(clause, prefix string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(clause), prefix))
}

func appendSQLClause(base, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	switch {
	case base == "":
		return extra
	case extra == "":
		return base
	default:
		return base + " AND " + extra
	}
}

func buildSearchWhereClause(alias, whereClause string, args []interface{}, searching bool, rawQuery string) (string, []interface{}) {
	clause := trimSQLClausePrefix(whereClause, "WHERE")
	queryArgs := append([]interface{}{}, args...)
	if !searching {
		return clause, queryArgs
	}
	clause = appendSQLClause(clause, trimSQLClausePrefix(fallbackSearchFilter(alias), "AND"))
	return clause, fallbackSearchFilterArgs(queryArgs, rawQuery)
}

func buildSkillFTSWhere(whereClause string, args []interface{}, ftsQuery string) z.ZormItem {
	trimmedWhere := trimSQLClausePrefix(whereClause, "WHERE")
	conds := []interface{}{z.Expr("skillmarket_fts MATCH ?", ftsQuery)}
	if trimmedWhere != "" {
		conds = append(conds, z.Expr(trimmedWhere, args...))
	}
	return z.Where(conds...)
}

func skillFTSScoreExpr(alias string) string {
	return `-bm25(skillmarket_fts, 10.0, 6.0, 2.0, 1.0, 1.0, 1.0, 0.5) + ((` + alias + `.trending_score + ` + alias + `.curated_boost) * 0.05)`
}

func skillSelectMapFields(alias string) []string {
	fields := []string{
		"id",
		"slug",
		"name",
		"description",
		"author",
		"repo_url",
		"homepage",
		"download_url",
		"stars",
		"downloads",
		"tags",
		"category",
		"security_score",
		"permissions",
		"latest_version",
		"risk_level",
		"security_badge",
		"installable",
		"install_type",
		"artifact_kind",
		"vulnerability_status",
		"has_vulnerabilities",
		"has_prompt_injection",
		"has_shell_injection",
		"has_data_exfiltration",
		"has_binary",
		"has_scripts",
		"popularity_score",
		"trending_score",
		"scan_status",
		"content_sha256",
		"published",
		"source_id",
		"source_name",
		"source_group",
		"source_type",
		"skill_path",
		"skill_content",
		"embedding_json",
		"embedding_model",
		"curated_rank",
		"curated_boost",
		"curated_label",
		"curated_reason",
		"last_updated",
		"last_crawled_at",
		"created_at",
		"updated_at",
	}
	if strings.TrimSpace(alias) == "" {
		return fields
	}
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		result = append(result, alias+"."+field+" AS "+field)
	}
	return result
}

func skillSelectColumns(alias string) string {
	return strings.Join([]string{
		`COALESCE(` + alias + `.id, '')`,
		`COALESCE(` + alias + `.slug, '')`,
		`COALESCE(` + alias + `.name, '')`,
		`COALESCE(` + alias + `.description, '')`,
		`COALESCE(` + alias + `.author, '')`,
		`COALESCE(` + alias + `.repo_url, '')`,
		`COALESCE(` + alias + `.homepage, '')`,
		`COALESCE(` + alias + `.download_url, '')`,
		`COALESCE(` + alias + `.stars, 0)`,
		`COALESCE(` + alias + `.downloads, 0)`,
		`COALESCE(` + alias + `.tags, '')`,
		`COALESCE(` + alias + `.category, '')`,
		`COALESCE(` + alias + `.security_score, 100)`,
		`COALESCE(` + alias + `.permissions, '[]')`,
		`COALESCE(` + alias + `.latest_version, '')`,
		`COALESCE(` + alias + `.risk_level, 'unknown')`,
		`COALESCE(` + alias + `.security_badge, 'yellow')`,
		`COALESCE(` + alias + `.installable, 0)`,
		`COALESCE(` + alias + `.install_type, 'manual_external')`,
		`COALESCE(` + alias + `.artifact_kind, 'unknown')`,
		`COALESCE(` + alias + `.vulnerability_status, 'unknown')`,
		`COALESCE(` + alias + `.has_vulnerabilities, 0)`,
		`COALESCE(` + alias + `.has_prompt_injection, 0)`,
		`COALESCE(` + alias + `.has_shell_injection, 0)`,
		`COALESCE(` + alias + `.has_data_exfiltration, 0)`,
		`COALESCE(` + alias + `.has_binary, 0)`,
		`COALESCE(` + alias + `.has_scripts, 0)`,
		`COALESCE(` + alias + `.popularity_score, 0)`,
		`COALESCE(` + alias + `.trending_score, 0)`,
		`COALESCE(` + alias + `.scan_status, '')`,
		`COALESCE(` + alias + `.content_sha256, '')`,
		`COALESCE(` + alias + `.published, 1)`,
		`COALESCE(` + alias + `.source_id, '')`,
		`COALESCE(` + alias + `.source_name, '')`,
		`COALESCE(` + alias + `.source_group, '')`,
		`COALESCE(` + alias + `.source_type, '')`,
		`COALESCE(` + alias + `.skill_path, '')`,
		`COALESCE(` + alias + `.skill_content, '')`,
		`COALESCE(` + alias + `.embedding_json, '')`,
		`COALESCE(` + alias + `.embedding_model, '')`,
		`COALESCE(` + alias + `.curated_rank, 0)`,
		`COALESCE(` + alias + `.curated_boost, 0)`,
		`COALESCE(` + alias + `.curated_label, '')`,
		`COALESCE(` + alias + `.curated_reason, '')`,
		`COALESCE(` + alias + `.last_updated, '')`,
		`COALESCE(` + alias + `.last_crawled_at, '')`,
		`COALESCE(` + alias + `.created_at, '')`,
		`COALESCE(` + alias + `.updated_at, '')`,
	}, ", ")
}

func fallbackSearchFilter(alias string) string {
	return ` AND (
		LOWER(` + alias + `.name) LIKE ? OR LOWER(` + alias + `.description) LIKE ? OR LOWER(` + alias + `.skill_content) LIKE ? OR LOWER(` + alias + `.tags) LIKE ?
	)`
}

func fallbackSearchOrder(alias string, searching bool, defaultOrder, rawQuery string) string {
	if !searching {
		return defaultOrder
	}
	pattern := sqlStringLiteral("%" + strings.ToLower(strings.TrimSpace(rawQuery)) + "%")
	return `(
		CASE WHEN LOWER(` + alias + `.name) LIKE ` + pattern + ` THEN 40 ELSE 0 END +
		CASE WHEN LOWER(` + alias + `.description) LIKE ` + pattern + ` THEN 20 ELSE 0 END +
		CASE WHEN LOWER(` + alias + `.skill_content) LIKE ` + pattern + ` THEN 25 ELSE 0 END +
		CASE WHEN LOWER(` + alias + `.tags) LIKE ` + pattern + ` THEN 15 ELSE 0 END +
		((` + alias + `.trending_score + ` + alias + `.curated_boost) * 0.05)
	) DESC`
}

func skillSortClause(sort string) string {
	switch sort {
	case "newest":
		return `last_updated DESC, curated_rank ASC, curated_boost DESC, trending_score DESC`
	case "most_used":
		return `downloads DESC, curated_rank ASC, curated_boost DESC, trending_score DESC`
	case "name":
		return `name ASC`
	case "featured":
		return `CASE WHEN curated_rank > 0 THEN 0 ELSE 1 END ASC, curated_rank ASC, (trending_score + curated_boost) DESC`
	default:
		return `CASE WHEN curated_rank > 0 THEN 0 ELSE 1 END ASC, curated_rank ASC, (trending_score + curated_boost) DESC`
	}
}

func normalizeUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	items := make([]string, n)
	for i := range items {
		items[i] = "?"
	}
	return strings.Join(items, ",")
}

func fallbackSearchFilterArgs(base []interface{}, rawQuery string) []interface{} {
	args := append([]interface{}{}, base...)
	if strings.TrimSpace(rawQuery) == "" {
		return args
	}
	pattern := "%" + strings.ToLower(strings.TrimSpace(rawQuery)) + "%"
	args = append(args, pattern, pattern, pattern, pattern)
	return args
}

func sqlStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func escapeFTSQuery(raw string) string {
	terms := strings.Fields(strings.TrimSpace(raw))
	if len(terms) == 0 {
		return raw
	}
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.Trim(term, `"`)
		if term == "" {
			continue
		}
		quoted = append(quoted, fmt.Sprintf(`"%s"*`, term))
	}
	return strings.Join(quoted, " OR ")
}

func (s *Store) GetSkill(ctx context.Context, id string) (*SkillDetail, error) {
	var rows []skillDocumentRow
	if _, err := s.readTable(ctx, "skills").Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	doc := skillDocumentFromRow(rows[0])
	detail := &SkillDetail{Skill: doc}
	version, _ := s.GetLatestVersion(ctx, id)
	detail.Version = version
	if version != nil {
		report, _ := s.GetSecurityReport(ctx, version.SkillID, version.Version)
		detail.Security = report
	}
	installed, _ := s.GetInstalledSkill(ctx, id)
	if installed != nil {
		detail.Installed = true
		detail.Enabled = installed.Enabled
	}
	return detail, nil
}

func (s *Store) GetLatestVersion(ctx context.Context, skillID string) (*SkillVersion, error) {
	var preferredRows []skillLatestVersionRow
	if _, err := s.readTable(ctx, "skills").Select(&preferredRows,
		z.Fields("latest_version"),
		z.Where(z.Eq("id", skillID)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(preferredRows) > 0 {
		preferredVersion := strings.TrimSpace(nullableStringValue(preferredRows[0].LatestVersion))
		if preferredVersion != "" {
			version, err := s.GetSkillVersion(ctx, skillID, preferredVersion)
			if err == nil {
				return version, nil
			}
			if err != sql.ErrNoRows {
				return nil, err
			}
		}
	}

	var rows []skillVersionRow
	if _, err := s.readTable(ctx, "skill_versions").Select(&rows,
		z.Where(z.Eq("skill_id", skillID)),
		z.OrderBy("released_at DESC", "created_at DESC"),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	version := skillVersionFromRow(rows[0])
	return &version, nil
}

func (s *Store) GetSkillVersion(ctx context.Context, skillID, version string) (*SkillVersion, error) {
	var rows []skillVersionRow
	if _, err := s.readTable(ctx, "skill_versions").Select(&rows,
		z.Where(z.Eq("skill_id", skillID), z.Eq("version", version)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	skillVersion := skillVersionFromRow(rows[0])
	return &skillVersion, nil
}

func (s *Store) GetSecurityReport(ctx context.Context, skillID, version string) (*SecurityReport, error) {
	var rows []securityReportRow
	if _, err := s.readTable(ctx, "skill_security_reports").Select(&rows,
		z.Where(z.Eq("skill_id", skillID), z.Eq("version", version)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	report := securityReportFromRow(rows[0])
	return &report, nil
}

func (s *Store) SetInstalledSkill(ctx context.Context, installed InstalledSkill) error {
	now := timeutil.NowTime()
	if installed.InstalledAt.IsZero() {
		installed.InstalledAt = now
	}
	installed.UpdatedAt = now
	_, err := s.table(ctx, "installed_skills").Insert(
		z.V{
			"skill_id":            installed.SkillID,
			"installed_version":   installed.InstalledVersion,
			"checksum":            installed.Checksum,
			"source_url":          installed.SourceURL,
			"enabled":             boolToInt(installed.Enabled),
			"auto_update":         boolToInt(installed.AutoUpdate),
			"installed_at":        installed.InstalledAt,
			"updated_at":          installed.UpdatedAt,
			"last_security_score": installed.LastSecurityScore,
		},
		z.OnConflictDoUpdateSet(
			[]string{"skill_id"},
			[]string{
				"installed_version",
				"checksum",
				"source_url",
				"enabled",
				"auto_update",
				"updated_at",
				"last_security_score",
			},
		),
	)
	return err
}

func (s *Store) RemoveInstalledSkill(ctx context.Context, skillID string) error {
	_, err := s.table(ctx, "installed_skills").Delete(z.Where(z.Eq("skill_id", skillID)))
	return err
}

func (s *Store) GetInstalledSkill(ctx context.Context, skillID string) (*InstalledSkill, error) {
	var rows []installedSkillRow
	if _, err := s.readTable(ctx, "installed_skills").Select(&rows,
		z.Fields(
			"skill_id",
			"installed_version",
			"checksum",
			"source_url",
			"enabled",
			"auto_update",
			"installed_at",
			"updated_at",
			"last_security_score",
		),
		z.Where(z.Eq("skill_id", skillID)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	skill := installedSkillFromRow(rows[0])
	return &skill, nil
}

func (s *Store) ListInstalledSkills(ctx context.Context) ([]InstalledSkill, error) {
	var rows []z.V
	if _, err := s.readTable(ctx, "installed_skills i").Select(&rows,
		z.Fields(
			"i.skill_id as skill_id",
			"i.installed_version as installed_version",
			"i.checksum as checksum",
			"i.source_url as source_url",
			"i.enabled as enabled",
			"i.auto_update as auto_update",
			"i.installed_at as installed_at",
			"i.updated_at as updated_at",
			"i.last_security_score as last_security_score",
			"COALESCE(sk.name, '') as name",
			"COALESCE(sk.latest_version, '') as latest_version",
			"COALESCE(u.latest_checksum, '') as latest_checksum",
			"COALESCE(u.action, '') as pending_update_state",
			"COALESCE(sk.security_badge, '') as security_badge",
		),
		z.LeftJoin("skills sk", z.Expr("sk.id = i.skill_id")),
		z.LeftJoin("skill_update_checks u", z.Expr("u.skill_id = i.skill_id")),
		z.OrderBy("i.updated_at DESC"),
	); err != nil {
		return nil, err
	}
	result := make([]InstalledSkill, 0, len(rows))
	for _, row := range rows {
		result = append(result, installedSkillListFromMapRow(row))
	}
	return result, nil
}

func (s *Store) RecordUpdateCheck(ctx context.Context, update AvailableUpdate) error {
	now := timeutil.NowTime()
	_, err := s.table(ctx, "skill_update_checks").Insert(
		z.V{
			"skill_id":        update.SkillID,
			"checked_at":      now,
			"latest_version":  update.LatestVersion,
			"latest_checksum": update.LatestChecksum,
			"action":          update.Action,
		},
		z.OnConflictDoUpdateSet(
			[]string{"skill_id"},
			[]string{"checked_at", "latest_version", "latest_checksum", "action"},
		),
	)
	return err
}

func (s *Store) ListUpdates(ctx context.Context) ([]AvailableUpdate, error) {
	var rows []z.V
	if _, err := s.readTable(ctx, "installed_skills i").Select(&rows,
		z.Fields(
			"i.skill_id as skill_id",
			"i.installed_version as current_version",
			"u.latest_version as latest_version",
			"i.checksum as current_checksum",
			"u.latest_checksum as latest_checksum",
			"u.action as action",
			"u.checked_at as checked_at",
		),
		z.LeftJoin("skill_update_checks u", z.Expr("u.skill_id = i.skill_id")),
		z.Where(
			z.Expr("COALESCE(u.latest_version, '') != ''"),
			z.Expr("u.latest_version != i.installed_version"),
		),
		z.OrderBy("u.checked_at DESC"),
	); err != nil {
		return nil, err
	}
	result := make([]AvailableUpdate, 0, len(rows))
	for _, row := range rows {
		result = append(result, availableUpdateFromMapRow(row))
	}
	return result, nil
}

func (s *Store) RecordTelemetry(ctx context.Context, skillID, event string) error {
	now := timeutil.NowTime()
	day := now.Format("2006-01-02")
	field := "downloads"
	switch event {
	case "install":
		field = "installs"
	case "active":
		field = "active_skills"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := z.TableContext(ctx, tx, "skill_telemetry_daily").Insert(
		z.V{
			"id":            uuid.NewString(),
			"skill_id":      skillID,
			"day":           day,
			"downloads":     0,
			"installs":      0,
			"active_skills": 0,
			"created_at":    now,
			"updated_at":    now,
		},
		z.OnConflictDoUpdateSet([]string{"skill_id", "day"}, []string{"updated_at"}),
	); err != nil {
		return err
	}
	if _, err := z.TableContext(ctx, tx, "skill_telemetry_daily").Update(
		z.V{
			field:        z.U(field + " + 1"),
			"updated_at": now,
		},
		z.Fields(field, "updated_at"),
		z.Where(z.Eq("skill_id", skillID), z.Eq("day", day)),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListTrending(ctx context.Context, category string, limit int) ([]SkillDocument, error) {
	if limit <= 0 {
		limit = 20
	}
	opts := []z.ZormItem{
		z.OrderBy("CASE WHEN curated_rank > 0 THEN 0 ELSE 1 END ASC", "curated_rank ASC", "(trending_score + curated_boost) DESC"),
		z.Limit(limit),
	}
	if strings.TrimSpace(category) != "" {
		opts = append([]z.ZormItem{z.Where(z.Eq("category", category))}, opts...)
	}
	var rows []skillDocumentRow
	if _, err := s.readTable(ctx, "skills").Select(&rows, opts...); err != nil {
		return nil, err
	}
	docs := make([]SkillDocument, 0, len(rows))
	for i := range rows {
		docs = append(docs, skillDocumentFromRow(rows[i]))
	}
	return docs, nil
}

func (s *Store) ListFeatured(ctx context.Context, category string, source string, limit int) ([]SkillDocument, error) {
	if limit <= 0 {
		limit = 20
	}
	conds := []interface{}{
		z.Eq("published", 1),
		z.Or(z.Gt("curated_rank", 0), z.Gt("curated_boost", 0)),
	}
	if strings.TrimSpace(category) != "" {
		conds = append(conds, z.Eq("category", category))
	}
	if strings.TrimSpace(source) != "" {
		conds = append(conds, z.Or(
			z.Eq("source_group", source),
			z.Eq("source_name", source),
			z.Eq("source_id", source),
		))
	}
	var rows []skillDocumentRow
	if _, err := s.readTable(ctx, "skills").Select(&rows,
		z.Where(conds...),
		z.OrderBy("CASE WHEN curated_rank > 0 THEN 0 ELSE 1 END ASC", "curated_rank ASC", "(trending_score + curated_boost) DESC"),
		z.Limit(limit),
	); err != nil {
		return nil, err
	}
	docs := make([]SkillDocument, 0, len(rows))
	for i := range rows {
		docs = append(docs, skillDocumentFromRow(rows[i]))
	}
	return docs, nil
}

func (s *Store) GetFilters(ctx context.Context) (*SkillFilters, error) {
	result := &SkillFilters{
		Installable:     map[string]int{},
		SecuritySignals: map[string]int{},
	}
	buildBuckets := func(valueExpr, groupExpr string, extraConds []interface{}, orders []string, dest *[]FilterOption) error {
		conds := []interface{}{z.Eq("published", 1)}
		conds = append(conds, extraConds...)
		var rows []z.V
		opts := []z.ZormItem{
			z.Fields(valueExpr+" as value", "COUNT(*) as count"),
			z.Where(conds...),
			z.GroupBy(groupExpr),
		}
		if len(orders) > 0 {
			opts = append(opts, z.OrderBy(orders...))
		}
		if _, err := s.readTable(ctx, "skills").Select(&rows, opts...); err != nil {
			return err
		}
		for _, row := range rows {
			value := skillmarketStringFromMapValue(row, "value")
			count := skillmarketIntFromMapValue(row, "count")
			if strings.TrimSpace(value) == "" {
				continue
			}
			*dest = append(*dest, FilterOption{Value: value, Label: value, Count: count})
		}
		return nil
	}
	if err := buildBuckets("category", "category", []interface{}{z.Neq("category", "")}, []string{"category"}, &result.Categories); err != nil {
		return nil, err
	}
	sort.Slice(result.Categories, func(i, j int) bool {
		left := MarketplaceCategoryOrder(result.Categories[i].Value)
		right := MarketplaceCategoryOrder(result.Categories[j].Value)
		if left != right {
			return left < right
		}
		return result.Categories[i].Value < result.Categories[j].Value
	})
	sourceBucketExpr := "COALESCE(NULLIF(source_group, ''), NULLIF(source_id, ''), NULLIF(source_name, ''), '')"
	if err := buildBuckets(sourceBucketExpr, sourceBucketExpr, nil, []string{"COUNT(*) DESC", "value ASC"}, &result.Sources); err != nil {
		return nil, err
	}
	if err := buildBuckets("security_badge", "security_badge", nil, []string{"security_badge"}, &result.RiskBadges); err != nil {
		return nil, err
	}
	if err := buildBuckets("install_type", "install_type", nil, []string{"install_type"}, &result.InstallTypes); err != nil {
		return nil, err
	}
	if err := buildBuckets("artifact_kind", "artifact_kind", nil, []string{"artifact_kind"}, &result.ArtifactKinds); err != nil {
		return nil, err
	}
	var signalRows []z.V
	if _, err := s.readTable(ctx, "skills").Select(&signalRows,
		z.Fields(
			"COALESCE(SUM(CASE WHEN installable = 1 THEN 1 ELSE 0 END), 0) as installable",
			"COALESCE(SUM(CASE WHEN installable = 0 THEN 1 ELSE 0 END), 0) as manual_only",
			"COALESCE(SUM(CASE WHEN has_vulnerabilities = 1 THEN 1 ELSE 0 END), 0) as vulnerabilities",
			"COALESCE(SUM(CASE WHEN has_prompt_injection = 1 THEN 1 ELSE 0 END), 0) as prompt",
			"COALESCE(SUM(CASE WHEN has_shell_injection = 1 THEN 1 ELSE 0 END), 0) as shell",
			"COALESCE(SUM(CASE WHEN has_data_exfiltration = 1 THEN 1 ELSE 0 END), 0) as exfil",
		),
		z.Where(z.Eq("published", 1)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	installable, manualOnly, vulnerabilities, prompt, shell, exfil := 0, 0, 0, 0, 0, 0
	if len(signalRows) > 0 {
		installable = skillmarketIntFromMapValue(signalRows[0], "installable")
		manualOnly = skillmarketIntFromMapValue(signalRows[0], "manual_only")
		vulnerabilities = skillmarketIntFromMapValue(signalRows[0], "vulnerabilities")
		prompt = skillmarketIntFromMapValue(signalRows[0], "prompt")
		shell = skillmarketIntFromMapValue(signalRows[0], "shell")
		exfil = skillmarketIntFromMapValue(signalRows[0], "exfil")
	}
	result.Installable["true"] = installable
	result.Installable["false"] = manualOnly
	result.SecuritySignals["vulnerabilities"] = vulnerabilities
	result.SecuritySignals["prompt_injection"] = prompt
	result.SecuritySignals["shell_injection"] = shell
	result.SecuritySignals["data_exfiltration"] = exfil
	return result, nil
}

func (s *Store) ReplaceCurations(ctx context.Context, entries []SkillCuration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	curationsTable := z.TableContext(ctx, tx, "skill_curations")
	skillsTable := z.TableContext(ctx, tx, "skills")

	if _, err := curationsTable.Delete(z.Where(z.Expr("1=1"))); err != nil {
		return err
	}
	if _, err := skillsTable.Update(
		z.V{
			"curated_rank":   0,
			"curated_boost":  0,
			"curated_label":  nil,
			"curated_reason": nil,
			"published":      1,
		},
		z.Fields("curated_rank", "curated_boost", "curated_label", "curated_reason", "published"),
		z.Where(z.Expr("1=1")),
	); err != nil {
		return err
	}
	now := timeutil.NowTime()
	for _, entry := range entries {
		if entry.ID == "" {
			entry.ID = uuid.NewString()
		}
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = now
		}
		entry.UpdatedAt = now
		if _, err := curationsTable.Insert(z.V{
			"id":            entry.ID,
			"skill_id":      entry.SkillID,
			"hidden":        boolToInt(entry.Hidden),
			"featured_rank": entry.FeaturedRank,
			"boost_weight":  entry.BoostWeight,
			"label":         entry.Label,
			"reason":        entry.Reason,
			"created_at":    entry.CreatedAt,
			"updated_at":    entry.UpdatedAt,
		}); err != nil {
			return err
		}
		if _, err := skillsTable.Update(
			z.V{
				"curated_rank":   entry.FeaturedRank,
				"curated_boost":  entry.BoostWeight,
				"curated_label":  entry.Label,
				"curated_reason": entry.Reason,
				"published":      boolToInt(!entry.Hidden),
			},
			z.Fields("curated_rank", "curated_boost", "curated_label", "curated_reason", "published"),
			z.Where(z.Eq("id", entry.SkillID)),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertCurationSyncState(ctx context.Context, state CurationSyncState) error {
	if strings.TrimSpace(state.ID) == "" {
		state.ID = "default"
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = timeutil.NowTime()
	}
	_, err := s.table(ctx, "curation_sync_state").Insert(
		z.V{
			"id":              state.ID,
			"source_url":      state.SourceURL,
			"checksum":        state.Checksum,
			"last_success_at": nullTime(state.LastSuccessAt),
			"last_error":      state.LastError,
			"updated_at":      state.UpdatedAt,
		},
		z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{"source_url", "checksum", "last_success_at", "last_error", "updated_at"},
		),
	)
	return err
}

func (s *Store) GetCurationSyncState(ctx context.Context, id string) (*CurationSyncState, error) {
	if strings.TrimSpace(id) == "" {
		id = "default"
	}
	var rows []curationSyncStateRow
	if _, err := s.readTable(ctx, "curation_sync_state").Select(&rows,
		z.Fields("id", "source_url", "checksum", "last_success_at", "last_error", "updated_at"),
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	state := &CurationSyncState{
		ID:            rows[0].ID,
		SourceURL:     nullableStringValue(rows[0].SourceURL),
		Checksum:      nullableStringValue(rows[0].Checksum),
		LastSuccessAt: parseTime(nullableStringValue(rows[0].LastSuccessAt)),
		LastError:     nullableStringValue(rows[0].LastError),
		UpdatedAt:     parseTime(nullableStringValue(rows[0].UpdatedAt)),
	}
	return state, nil
}

func (s *Store) RerankSemantic(queryVector []float32, results []SearchResult) []SearchResult {
	if len(queryVector) == 0 {
		return results
	}
	reranked := make([]SearchResult, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.Skill.EmbeddingJSON) == "" {
			reranked = append(reranked, result)
			continue
		}
		var candidate []float32
		if err := json.Unmarshal([]byte(result.Skill.EmbeddingJSON), &candidate); err != nil || len(candidate) == 0 {
			reranked = append(reranked, result)
			continue
		}
		score, err := embedding.CosineSimilarity(queryVector, candidate)
		if err != nil {
			reranked = append(reranked, result)
			continue
		}
		result.SemanticScore = float64(score)
		result.Score = (result.KeywordScore * 0.65) + (float64(score) * 100 * 0.35)
		result.MatchSource = "hybrid"
		reranked = append(reranked, result)
	}
	sort.SliceStable(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})
	return reranked
}

func scanSkillDocument(scanner interface {
	Scan(dest ...interface{}) error
}) (*SkillDocument, error) {
	var doc SkillDocument
	var tagsJSON, permissionsJSON string
	var published, installable, hasVulnerabilities, hasPromptInjection, hasShellInjection, hasDataExfiltration, hasBinary, hasScripts int
	var lastUpdated, lastCrawledAt, createdAt, updatedAt string
	if err := scanner.Scan(&doc.ID, &doc.Slug, &doc.Name, &doc.Description, &doc.Author,
		&doc.RepoURL, &doc.Homepage, &doc.DownloadURL, &doc.Stars, &doc.Downloads, &tagsJSON, &doc.Category,
		&doc.SecurityScore, &permissionsJSON, &doc.LatestVersion, &doc.RiskLevel, &doc.SecurityBadge, &installable,
		&doc.InstallType, &doc.ArtifactKind, &doc.VulnerabilityStatus, &hasVulnerabilities, &hasPromptInjection,
		&hasShellInjection, &hasDataExfiltration, &hasBinary, &hasScripts, &doc.PopularityScore, &doc.TrendingScore,
		&doc.ScanStatus, &doc.ContentSHA256, &published, &doc.SourceID, &doc.SourceName, &doc.SourceGroup,
		&doc.SourceType, &doc.SkillPath, &doc.SkillContent, &doc.EmbeddingJSON, &doc.EmbeddingModel,
		&doc.CuratedRank, &doc.CuratedBoost, &doc.CuratedLabel, &doc.CuratedReason,
		&lastUpdated, &lastCrawledAt, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	doc.Tags = decodeStrings(tagsJSON)
	doc.Permissions = decodeStrings(permissionsJSON)
	doc.Published = published == 1
	doc.Installable = installable == 1
	doc.HasVulnerabilities = hasVulnerabilities == 1
	doc.HasPromptInjection = hasPromptInjection == 1
	doc.HasShellInjection = hasShellInjection == 1
	doc.HasDataExfiltration = hasDataExfiltration == 1
	doc.HasBinary = hasBinary == 1
	doc.HasScripts = hasScripts == 1
	doc.LastUpdated = parseTime(lastUpdated)
	doc.LastCrawledAt = parseTime(lastCrawledAt)
	doc.CreatedAt = parseTime(createdAt)
	doc.UpdatedAt = parseTime(updatedAt)
	return &doc, nil
}

func scanSkillDocumentWithScore(scanner interface {
	Scan(dest ...interface{}) error
}) (*SkillDocument, float64, error) {
	var doc SkillDocument
	var tagsJSON, permissionsJSON string
	var published, installable, hasVulnerabilities, hasPromptInjection, hasShellInjection, hasDataExfiltration, hasBinary, hasScripts int
	var lastUpdated, lastCrawledAt, createdAt, updatedAt string
	var score float64
	if err := scanner.Scan(&doc.ID, &doc.Slug, &doc.Name, &doc.Description, &doc.Author,
		&doc.RepoURL, &doc.Homepage, &doc.DownloadURL, &doc.Stars, &doc.Downloads, &tagsJSON, &doc.Category,
		&doc.SecurityScore, &permissionsJSON, &doc.LatestVersion, &doc.RiskLevel, &doc.SecurityBadge, &installable,
		&doc.InstallType, &doc.ArtifactKind, &doc.VulnerabilityStatus, &hasVulnerabilities, &hasPromptInjection,
		&hasShellInjection, &hasDataExfiltration, &hasBinary, &hasScripts, &doc.PopularityScore, &doc.TrendingScore,
		&doc.ScanStatus, &doc.ContentSHA256, &published, &doc.SourceID, &doc.SourceName, &doc.SourceGroup,
		&doc.SourceType, &doc.SkillPath, &doc.SkillContent, &doc.EmbeddingJSON, &doc.EmbeddingModel,
		&doc.CuratedRank, &doc.CuratedBoost, &doc.CuratedLabel, &doc.CuratedReason,
		&lastUpdated, &lastCrawledAt, &createdAt, &updatedAt, &score,
	); err != nil {
		return nil, 0, err
	}
	doc.Tags = decodeStrings(tagsJSON)
	doc.Permissions = decodeStrings(permissionsJSON)
	doc.Published = published == 1
	doc.Installable = installable == 1
	doc.HasVulnerabilities = hasVulnerabilities == 1
	doc.HasPromptInjection = hasPromptInjection == 1
	doc.HasShellInjection = hasShellInjection == 1
	doc.HasDataExfiltration = hasDataExfiltration == 1
	doc.HasBinary = hasBinary == 1
	doc.HasScripts = hasScripts == 1
	doc.LastUpdated = parseTime(lastUpdated)
	doc.LastCrawledAt = parseTime(lastCrawledAt)
	doc.CreatedAt = parseTime(createdAt)
	doc.UpdatedAt = parseTime(updatedAt)
	return &doc, score, nil
}

func scanSkillVersion(scanner interface {
	Scan(dest ...interface{}) error
}) (*SkillVersion, error) {
	var version SkillVersion
	var releasedAt, scannedAt, createdAt, updatedAt string
	if err := scanner.Scan(&version.ID, &version.SkillID, &version.Version, &version.CommitHash,
		&version.SourceURL, &version.Checksum, &version.SkillPath, &version.RawSkillMD,
		&version.ManifestJSON, &releasedAt, &scannedAt, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	version.ReleasedAt = parseTime(releasedAt)
	version.ScannedAt = parseTime(scannedAt)
	version.CreatedAt = parseTime(createdAt)
	version.UpdatedAt = parseTime(updatedAt)
	return &version, nil
}

func scanInstalledSkill(scanner interface {
	Scan(dest ...interface{}) error
}) (*InstalledSkill, error) {
	var skill InstalledSkill
	var enabled, autoUpdate int
	var installedAt, updatedAt string
	if err := scanner.Scan(&skill.SkillID, &skill.InstalledVersion, &skill.Checksum,
		&skill.SourceURL, &enabled, &autoUpdate, &installedAt, &updatedAt, &skill.LastSecurityScore,
	); err != nil {
		return nil, err
	}
	skill.Enabled = enabled == 1
	skill.AutoUpdate = autoUpdate == 1
	skill.InstalledAt = parseTime(installedAt)
	skill.UpdatedAt = parseTime(updatedAt)
	return &skill, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func boolCount(v bool) int {
	if v {
		return 1
	}
	return 0
}

func skillmarketStringFromMapValue(row z.V, key string) string {
	value, ok := skillmarketValueFromMapKey(row, key)
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339)
	default:
		return fmt.Sprint(typed)
	}
}

func skillmarketIntFromMapValue(row z.V, key string) int {
	value, ok := skillmarketValueFromMapKey(row, key)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case []byte:
		n, _ := strconv.Atoi(string(typed))
		return n
	case string:
		n, _ := strconv.Atoi(typed)
		return n
	default:
		return 0
	}
}

func skillmarketFloat64FromMapValue(row z.V, key string) float64 {
	value, ok := skillmarketValueFromMapKey(row, key)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float32:
		return float64(typed)
	case float64:
		return typed
	case int:
		return float64(typed)
	case int8:
		return float64(typed)
	case int16:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint8:
		return float64(typed)
	case uint16:
		return float64(typed)
	case uint32:
		return float64(typed)
	case uint64:
		return float64(typed)
	case []byte:
		n, _ := strconv.ParseFloat(string(typed), 64)
		return n
	case string:
		n, _ := strconv.ParseFloat(typed, 64)
		return n
	default:
		return 0
	}
}

func skillDocumentFromMapRow(row z.V) SkillDocument {
	return SkillDocument{
		ID:                  skillmarketStringFromMapValue(row, "id"),
		Slug:                skillmarketStringFromMapValue(row, "slug"),
		Name:                skillmarketStringFromMapValue(row, "name"),
		Description:         skillmarketStringFromMapValue(row, "description"),
		Author:              skillmarketStringFromMapValue(row, "author"),
		RepoURL:             skillmarketStringFromMapValue(row, "repo_url"),
		Homepage:            skillmarketStringFromMapValue(row, "homepage"),
		DownloadURL:         skillmarketStringFromMapValue(row, "download_url"),
		Stars:               skillmarketIntFromMapValue(row, "stars"),
		Downloads:           skillmarketIntFromMapValue(row, "downloads"),
		Tags:                decodeStrings(skillmarketStringFromMapValue(row, "tags")),
		Category:            skillmarketStringFromMapValue(row, "category"),
		SecurityScore:       skillmarketIntFromMapValue(row, "security_score"),
		Permissions:         decodeStrings(skillmarketStringFromMapValue(row, "permissions")),
		LatestVersion:       skillmarketStringFromMapValue(row, "latest_version"),
		RiskLevel:           skillmarketStringFromMapValue(row, "risk_level"),
		SecurityBadge:       skillmarketStringFromMapValue(row, "security_badge"),
		Installable:         skillmarketIntFromMapValue(row, "installable") == 1,
		InstallType:         skillmarketStringFromMapValue(row, "install_type"),
		ArtifactKind:        skillmarketStringFromMapValue(row, "artifact_kind"),
		VulnerabilityStatus: skillmarketStringFromMapValue(row, "vulnerability_status"),
		HasVulnerabilities:  skillmarketIntFromMapValue(row, "has_vulnerabilities") == 1,
		HasPromptInjection:  skillmarketIntFromMapValue(row, "has_prompt_injection") == 1,
		HasShellInjection:   skillmarketIntFromMapValue(row, "has_shell_injection") == 1,
		HasDataExfiltration: skillmarketIntFromMapValue(row, "has_data_exfiltration") == 1,
		HasBinary:           skillmarketIntFromMapValue(row, "has_binary") == 1,
		HasScripts:          skillmarketIntFromMapValue(row, "has_scripts") == 1,
		PopularityScore:     skillmarketFloat64FromMapValue(row, "popularity_score"),
		TrendingScore:       skillmarketFloat64FromMapValue(row, "trending_score"),
		ScanStatus:          skillmarketStringFromMapValue(row, "scan_status"),
		ContentSHA256:       skillmarketStringFromMapValue(row, "content_sha256"),
		Published:           skillmarketIntFromMapValue(row, "published") == 1,
		SourceID:            skillmarketStringFromMapValue(row, "source_id"),
		SourceName:          skillmarketStringFromMapValue(row, "source_name"),
		SourceGroup:         skillmarketStringFromMapValue(row, "source_group"),
		SourceType:          skillmarketStringFromMapValue(row, "source_type"),
		SkillPath:           skillmarketStringFromMapValue(row, "skill_path"),
		SkillContent:        skillmarketStringFromMapValue(row, "skill_content"),
		EmbeddingJSON:       skillmarketStringFromMapValue(row, "embedding_json"),
		EmbeddingModel:      skillmarketStringFromMapValue(row, "embedding_model"),
		CuratedRank:         skillmarketIntFromMapValue(row, "curated_rank"),
		CuratedBoost:        skillmarketFloat64FromMapValue(row, "curated_boost"),
		CuratedLabel:        skillmarketStringFromMapValue(row, "curated_label"),
		CuratedReason:       skillmarketStringFromMapValue(row, "curated_reason"),
		LastUpdated:         parseTime(skillmarketStringFromMapValue(row, "last_updated")),
		LastCrawledAt:       parseTime(skillmarketStringFromMapValue(row, "last_crawled_at")),
		CreatedAt:           parseTime(skillmarketStringFromMapValue(row, "created_at")),
		UpdatedAt:           parseTime(skillmarketStringFromMapValue(row, "updated_at")),
	}
}

func searchResultFromMapScoreRow(row z.V) SearchResult {
	doc := skillDocumentFromMapRow(row)
	score := skillmarketFloat64FromMapValue(row, "score")
	return SearchResult{
		Skill:        doc,
		Score:        score,
		KeywordScore: score,
		MatchSource:  "fts5",
	}
}

func skillmarketValueFromMapKey(row z.V, key string) (interface{}, bool) {
	if row == nil {
		return nil, false
	}
	if value, ok := row[key]; ok {
		return value, true
	}
	for rawKey, value := range row {
		if skillmarketNormalizeMapKey(rawKey) == key {
			return value, true
		}
	}
	return nil, false
}

func skillmarketNormalizeMapKey(key string) string {
	key = strings.TrimSpace(strings.Trim(key, "`"))
	if key == "" {
		return ""
	}
	fields := strings.Fields(key)
	if len(fields) >= 3 && strings.EqualFold(fields[len(fields)-2], "as") {
		return strings.Trim(fields[len(fields)-1], "`")
	}
	if len(fields) >= 2 {
		return strings.Trim(fields[len(fields)-1], "`")
	}
	if dot := strings.LastIndex(key, "."); dot >= 0 {
		return strings.Trim(key[dot+1:], "`")
	}
	return key
}

func parseTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func nullTime(value time.Time) interface{} {
	if value.IsZero() {
		return nil
	}
	return value
}
