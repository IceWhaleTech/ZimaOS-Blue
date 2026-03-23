package skillmarket

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type Store struct {
	db         *sql.DB
	ftsEnabled bool
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

func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}
	if err := s.normalizeLegacyCategories(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
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

	if err := dropFTSTriggers(s.db); err != nil {
		return err
	}
	s.ftsEnabled = supportsFTS5(s.db)
	if !s.ftsEnabled {
		return nil
	}
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
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(category, ''), COALESCE(tags, '[]'), COALESCE(skill_content, '') FROM skills`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type update struct {
		id       string
		category string
	}
	updates := make([]update, 0)
	for rows.Next() {
		var id, category, tagsJSON, content string
		if err := rows.Scan(&id, &category, &tagsJSON, &content); err != nil {
			return err
		}
		normalized := normalizeCategory(category, content, decodeStrings(tagsJSON))
		if normalized == "" || normalized == category {
			continue
		}
		updates = append(updates, update{id: id, category: normalized})
	}
	if err := rows.Err(); err != nil {
		return err
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
		if _, err := tx.ExecContext(ctx, `UPDATE skills SET category = ?, updated_at = ? WHERE id = ?`, item.category, timeutil.NowTime(), item.id); err != nil {
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
	var hidden int
	var featuredRank int
	var boostWeight float64
	var label, reason string
	switch err := tx.QueryRowContext(ctx, `
		SELECT hidden, featured_rank, boost_weight, COALESCE(label, ''), COALESCE(reason, '')
		FROM skill_curations
		WHERE skill_id = ?
	`, doc.ID).Scan(&hidden, &featuredRank, &boostWeight, &label, &reason); err {
	case nil:
		doc.CuratedRank = featuredRank
		doc.CuratedBoost = boostWeight
		doc.CuratedLabel = label
		doc.CuratedReason = reason
		if hidden == 1 {
			doc.Published = false
		}
	case sql.ErrNoRows:
		return nil
	default:
		return err
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
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT id
			FROM skills
			WHERE id IN (`+placeholders(len(chunk))+`)`,
			args...,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			result[id] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
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
	const columnsPerRow = 48
	for start := 0; start < len(records); start += maxRowsPerInsert(columnsPerRow) {
		end := start + maxRowsPerInsert(columnsPerRow)
		if end > len(records) {
			end = len(records)
		}
		chunk := records[start:end]
		args := make([]interface{}, 0, len(chunk)*columnsPerRow)
		for _, record := range chunk {
			doc := record.Doc
			args = append(args,
				doc.ID, doc.Slug, doc.Name, doc.Description, doc.Author, doc.RepoURL, doc.Homepage,
				doc.DownloadURL, doc.Stars, doc.Downloads, encodeStrings(doc.Tags), doc.Category,
				doc.SecurityScore, encodeStrings(doc.Permissions), doc.LatestVersion, doc.RiskLevel,
				doc.SecurityBadge, boolToInt(doc.Installable), doc.InstallType, doc.ArtifactKind,
				doc.VulnerabilityStatus, boolToInt(doc.HasVulnerabilities), boolToInt(doc.HasPromptInjection),
				boolToInt(doc.HasShellInjection), boolToInt(doc.HasDataExfiltration), boolToInt(doc.HasBinary),
				boolToInt(doc.HasScripts), doc.PopularityScore, doc.TrendingScore, doc.ScanStatus,
				doc.ContentSHA256, boolToInt(doc.Published), doc.SourceID, doc.SourceName, doc.SourceGroup,
				doc.SourceType, doc.SkillPath, doc.SkillContent, doc.EmbeddingJSON, doc.EmbeddingModel,
				doc.CuratedRank, doc.CuratedBoost, doc.CuratedLabel, doc.CuratedReason, doc.LastUpdated,
				doc.LastCrawledAt, doc.CreatedAt, doc.UpdatedAt,
			)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO skills (
				id, slug, name, description, author, repo_url, homepage, download_url, stars, downloads, tags, category,
				security_score, permissions, latest_version, risk_level, security_badge, installable, install_type,
				artifact_kind, vulnerability_status, has_vulnerabilities, has_prompt_injection, has_shell_injection,
				has_data_exfiltration, has_binary, has_scripts, popularity_score, trending_score, scan_status,
				content_sha256, published, source_id, source_name, source_group, source_type, skill_path,
				skill_content, embedding_json, embedding_model, curated_rank, curated_boost, curated_label,
				curated_reason, last_updated, last_crawled_at, created_at, updated_at
			) VALUES `+groupedPlaceholders(len(chunk), columnsPerRow),
			args...,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) updateSkillDocs(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE skills SET
			slug=?, name=?, description=?, author=?, repo_url=?, homepage=?, download_url=?, stars=?, downloads=?, tags=?, category=?,
			security_score=?, permissions=?, latest_version=?, risk_level=?, security_badge=?, installable=?, install_type=?,
			artifact_kind=?, vulnerability_status=?, has_vulnerabilities=?, has_prompt_injection=?, has_shell_injection=?,
			has_data_exfiltration=?, has_binary=?, has_scripts=?, popularity_score=?, trending_score=?, scan_status=?,
			content_sha256=?, published=?, source_id=?, source_name=?, source_group=?, source_type=?, skill_path=?,
			skill_content=?, embedding_json=?, embedding_model=?, curated_rank=?, curated_boost=?, curated_label=?,
			curated_reason=?, last_updated=?, last_crawled_at=?, updated_at=?
		WHERE id = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, record := range records {
		doc := record.Doc
		if _, err := stmt.ExecContext(ctx,
			doc.Slug, doc.Name, doc.Description, doc.Author, doc.RepoURL, doc.Homepage, doc.DownloadURL,
			doc.Stars, doc.Downloads, encodeStrings(doc.Tags), doc.Category, doc.SecurityScore,
			encodeStrings(doc.Permissions), doc.LatestVersion, doc.RiskLevel, doc.SecurityBadge,
			boolToInt(doc.Installable), doc.InstallType, doc.ArtifactKind, doc.VulnerabilityStatus,
			boolToInt(doc.HasVulnerabilities), boolToInt(doc.HasPromptInjection), boolToInt(doc.HasShellInjection),
			boolToInt(doc.HasDataExfiltration), boolToInt(doc.HasBinary), boolToInt(doc.HasScripts),
			doc.PopularityScore, doc.TrendingScore, doc.ScanStatus, doc.ContentSHA256, boolToInt(doc.Published),
			doc.SourceID, doc.SourceName, doc.SourceGroup, doc.SourceType, doc.SkillPath, doc.SkillContent,
			doc.EmbeddingJSON, doc.EmbeddingModel, doc.CuratedRank, doc.CuratedBoost, doc.CuratedLabel,
			doc.CuratedReason, doc.LastUpdated, doc.LastCrawledAt, doc.UpdatedAt, doc.ID,
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
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT id, skill_id, version
			FROM skill_versions
			WHERE skill_id IN (`+placeholders(len(chunk))+`)`,
			args...,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, skillID, version string
			if err := rows.Scan(&id, &skillID, &version); err != nil {
				rows.Close()
				return nil, err
			}
			result[skillVersionKey(skillID, version)] = id
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return result, nil
}

func (s *Store) insertSkillVersionsIgnore(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	const columnsPerRow = 13
	for start := 0; start < len(records); start += maxRowsPerInsert(columnsPerRow) {
		end := start + maxRowsPerInsert(columnsPerRow)
		if end > len(records) {
			end = len(records)
		}
		chunk := records[start:end]
		args := make([]interface{}, 0, len(chunk)*columnsPerRow)
		for _, record := range chunk {
			version := record.Version
			args = append(args,
				version.ID, version.SkillID, version.Version, version.CommitHash, version.SourceURL,
				version.Checksum, version.SkillPath, version.RawSkillMD, version.ManifestJSON,
				version.ReleasedAt, version.ScannedAt, version.CreatedAt, version.UpdatedAt,
			)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO skill_versions (
				id, skill_id, version, commit_hash, source_url, checksum, skill_path,
				raw_skill_md, manifest_json, released_at, scanned_at, created_at, updated_at
			) VALUES `+groupedPlaceholders(len(chunk), columnsPerRow),
			args...,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) updateSkillVersions(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE skill_versions SET
			commit_hash=?, source_url=?, checksum=?, skill_path=?, raw_skill_md=?, manifest_json=?,
			released_at=?, scanned_at=?, updated_at=?
		WHERE skill_id = ? AND version = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, record := range records {
		version := record.Version
		if _, err := stmt.ExecContext(ctx,
			version.CommitHash, version.SourceURL, version.Checksum, version.SkillPath,
			version.RawSkillMD, version.ManifestJSON, version.ReleasedAt, version.ScannedAt,
			version.UpdatedAt, version.SkillID, version.Version,
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
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT id, skill_version_id
			FROM skill_security_reports
			WHERE skill_version_id IN (`+placeholders(len(chunk))+`)`,
			args...,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, skillVersionID string
			if err := rows.Scan(&id, &skillVersionID); err != nil {
				rows.Close()
				return nil, err
			}
			result[skillVersionID] = id
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return result, nil
}

func (s *Store) insertSkillReportsIgnore(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	const columnsPerRow = 23
	for start := 0; start < len(records); start += maxRowsPerInsert(columnsPerRow) {
		end := start + maxRowsPerInsert(columnsPerRow)
		if end > len(records) {
			end = len(records)
		}
		chunk := records[start:end]
		args := make([]interface{}, 0, len(chunk)*columnsPerRow)
		for _, record := range chunk {
			report := record.Report
			args = append(args,
				report.ID, report.SkillVersionID, report.SkillID, report.Version, report.Score,
				report.RiskLevel, report.SecurityBadge, report.VulnerabilityStatus, encodeJSON(report.Risks),
				encodeStrings(report.Permissions), encodeStrings(report.Secrets), encodeStrings(report.Vulnerabilities),
				encodeJSON(report.InstallSurface), encodeJSON(report.Evidence), report.InstallSurface.ArtifactKind,
				boolToInt(report.HasPromptInjection), boolToInt(report.HasShellInjection), boolToInt(report.HasDataExfiltration),
				report.ScannerVersion, report.LLMStatus, report.LLMVerdictJSON, report.CreatedAt, report.UpdatedAt,
			)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO skill_security_reports (
				id, skill_version_id, skill_id, version, score, risk_level, security_badge,
				vulnerability_status, risks_json, permissions_json, secrets_json, vulnerabilities_json,
				install_surface_json, evidence_json, artifact_kind, has_prompt_injection,
				has_shell_injection, has_data_exfiltration, scanner_version,
				llm_status, llm_verdict_json, created_at, updated_at
			) VALUES `+groupedPlaceholders(len(chunk), columnsPerRow),
			args...,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) updateSkillReports(ctx context.Context, tx *sql.Tx, records []*SkillUpsertRecord) error {
	if len(records) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE skill_security_reports SET
			score=?, risk_level=?, security_badge=?, vulnerability_status=?, risks_json=?, permissions_json=?,
			secrets_json=?, vulnerabilities_json=?, install_surface_json=?, evidence_json=?, artifact_kind=?,
			has_prompt_injection=?, has_shell_injection=?, has_data_exfiltration=?, scanner_version=?,
			llm_status=?, llm_verdict_json=?, updated_at=?
		WHERE skill_version_id = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, record := range records {
		report := record.Report
		if _, err := stmt.ExecContext(ctx,
			report.Score, report.RiskLevel, report.SecurityBadge, report.VulnerabilityStatus,
			encodeJSON(report.Risks), encodeStrings(report.Permissions), encodeStrings(report.Secrets),
			encodeStrings(report.Vulnerabilities), encodeJSON(report.InstallSurface), encodeJSON(report.Evidence),
			report.InstallSurface.ArtifactKind, boolToInt(report.HasPromptInjection), boolToInt(report.HasShellInjection),
			boolToInt(report.HasDataExfiltration), report.ScannerVersion, report.LLMStatus,
			report.LLMVerdictJSON, report.UpdatedAt, report.SkillVersionID,
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
	var existingSourceID string
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(source_id, '')
		FROM skills
		WHERE id = ?
		LIMIT 1
	`, skillID).Scan(&existingSourceID)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	existingSourceID = strings.TrimSpace(existingSourceID)
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
	var priority int
	err := tx.QueryRowContext(ctx, `
		SELECT priority
		FROM skill_sources
		WHERE id = ?
		LIMIT 1
	`, sourceID).Scan(&priority)
	if err == sql.ErrNoRows {
		return math.MaxInt32, nil
	}
	if err != nil {
		return 0, err
	}
	return priority, nil
}

func (s *Store) UpsertSource(ctx context.Context, source Source) error {
	now := timeutil.NowTime()
	if source.CreatedAt.IsZero() {
		source.CreatedAt = now
	}
	source.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO skill_sources (
			id, type, base_url, display_name, source_group, mirror_of, auth_mode, headers_json, enabled, last_cursor, etag,
			rate_limit_per_minute, priority, last_success_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type=excluded.type,
			base_url=excluded.base_url,
			display_name=excluded.display_name,
			source_group=excluded.source_group,
			mirror_of=excluded.mirror_of,
			auth_mode=excluded.auth_mode,
			headers_json=excluded.headers_json,
			enabled=excluded.enabled,
			last_cursor=excluded.last_cursor,
			etag=excluded.etag,
			rate_limit_per_minute=excluded.rate_limit_per_minute,
			priority=excluded.priority,
			last_success_at=excluded.last_success_at,
			updated_at=excluded.updated_at
	`, source.ID, source.Type, source.BaseURL, source.DisplayName, source.SourceGroup, source.MirrorOf,
		source.AuthMode, encodeJSON(source.Headers), boolToInt(source.Enabled), source.LastCursor, source.ETag,
		source.RateLimitPerMinute, source.Priority, nullTime(source.LastSuccessAt), source.CreatedAt, source.UpdatedAt,
	)
	return err
}

func (s *Store) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, base_url, COALESCE(display_name, ''), COALESCE(source_group, ''), COALESCE(mirror_of, ''),
			auth_mode, COALESCE(headers_json, '{}'), enabled, last_cursor, etag,
			rate_limit_per_minute, priority, COALESCE(last_success_at, ''), created_at, updated_at
		FROM skill_sources
		WHERE enabled = 1
		ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Source
	for rows.Next() {
		var src Source
		var enabled int
		var headersJSON, lastSuccess, createdAt, updatedAt string
		if err := rows.Scan(&src.ID, &src.Type, &src.BaseURL, &src.DisplayName, &src.SourceGroup, &src.MirrorOf,
			&src.AuthMode, &headersJSON, &enabled, &src.LastCursor, &src.ETag, &src.RateLimitPerMinute, &src.Priority,
			&lastSuccess, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		src.Enabled = enabled == 1
		_ = json.Unmarshal([]byte(headersJSON), &src.Headers)
		src.LastSuccessAt = parseTime(lastSuccess)
		src.CreatedAt = parseTime(createdAt)
		src.UpdatedAt = parseTime(updatedAt)
		result = append(result, src)
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
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO crawl_runs (id, source_id, status, discovered, updated, failed, started_at)
		VALUES (?, ?, ?, 0, 0, 0, ?)
	`, run.ID, run.SourceID, run.Status, run.StartedAt)
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Store) CompleteCrawlRun(ctx context.Context, run *CrawlRun) error {
	run.FinishedAt = timeutil.NowTime()
	_, err := s.db.ExecContext(ctx, `
		UPDATE crawl_runs
		SET status = ?, discovered = ?, updated = ?, failed = ?, finished_at = ?, error_text = ?
		WHERE id = ?
	`, run.Status, run.Discovered, run.Updated, run.Failed, run.FinishedAt, run.ErrorText, run.ID)
	if err != nil {
		return err
	}
	if run.Status == "success" {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE skill_sources
			SET last_success_at = ?, updated_at = ?
			WHERE id = ?
		`, run.FinishedAt, run.FinishedAt, run.SourceID)
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
	searching := strings.TrimSpace(query.Query) != ""
	whereClause, args := buildSkillFilters("s", query)

	var total int
	switch {
	case searching && s.ftsEnabled:
		countArgs := []interface{}{escapeFTSQuery(query.Query)}
		countArgs = append(countArgs, args...)
		countSQL := `SELECT COUNT(*) FROM skills s JOIN skillmarket_fts ON skillmarket_fts.rowid = s.rowid WHERE skillmarket_fts MATCH ?` + strings.TrimPrefix(whereClause, " WHERE s.published = 1")
		if err := s.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
			return nil, err
		}
	case searching:
		countArgs := fallbackSearchFilterArgs(args, query.Query)
		countSQL := `SELECT COUNT(*) FROM skills s` + whereClause + fallbackSearchFilter("s")
		if err := s.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
			return nil, err
		}
	default:
		countSQL := `SELECT COUNT(*) FROM skills s` + whereClause
		if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
			return nil, err
		}
	}

	sortBy := skillSortClause(query.Sort)

	offset := (query.Page - 1) * query.PageSize
	results := make([]SearchResult, 0, query.PageSize)

	if !searching || !s.ftsEnabled {
		rows, err := s.db.QueryContext(ctx, `
			SELECT `+skillSelectColumns("s")+`
			FROM skills s`+whereClause+fallbackSearchFilterMaybe("s", searching)+` ORDER BY `+fallbackSearchOrder("s", searching, sortBy)+` LIMIT ? OFFSET ?`,
			append(fallbackSearchArgs(args, query.Query), query.PageSize, offset)...,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			doc, err := scanSkillDocument(rows)
			if err != nil {
				return nil, err
			}
			results = append(results, SearchResult{Skill: *doc, Score: doc.TrendingScore, MatchSource: "rank"})
		}
	} else {
		ftsQuery := escapeFTSQuery(query.Query)
		ftsArgs := []interface{}{ftsQuery}
		ftsArgs = append(ftsArgs, args...)
		ftsArgs = append(ftsArgs, query.PageSize, offset)
		rows, err := s.db.QueryContext(ctx, `
			SELECT `+skillSelectColumns("s")+`,
				-bm25(skillmarket_fts, 10.0, 6.0, 2.0, 1.0, 1.0, 1.0, 0.5) +
				((s.trending_score + s.curated_boost) * 0.05) AS score
			FROM skills s
			JOIN skillmarket_fts ON skillmarket_fts.rowid = s.rowid
			WHERE skillmarket_fts MATCH ?`+strings.TrimPrefix(whereClause, " WHERE s.published = 1")+`
			ORDER BY score DESC LIMIT ? OFFSET ?`,
			ftsArgs...,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			doc, score, err := scanSkillDocumentWithScore(rows)
			if err != nil {
				return nil, err
			}
			results = append(results, SearchResult{Skill: *doc, Score: score, KeywordScore: score, MatchSource: "fts5"})
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

func fallbackSearchFilterMaybe(alias string, searching bool) string {
	if !searching {
		return ""
	}
	return fallbackSearchFilter(alias)
}

func fallbackSearchOrder(alias string, searching bool, defaultOrder string) string {
	if !searching {
		return defaultOrder
	}
	return `(
		CASE WHEN LOWER(` + alias + `.name) LIKE ? THEN 40 ELSE 0 END +
		CASE WHEN LOWER(` + alias + `.description) LIKE ? THEN 20 ELSE 0 END +
		CASE WHEN LOWER(` + alias + `.skill_content) LIKE ? THEN 25 ELSE 0 END +
		CASE WHEN LOWER(` + alias + `.tags) LIKE ? THEN 15 ELSE 0 END +
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

func fallbackSearchArgs(base []interface{}, rawQuery string) []interface{} {
	args := append([]interface{}{}, base...)
	if strings.TrimSpace(rawQuery) == "" {
		return args
	}
	pattern := "%" + strings.ToLower(strings.TrimSpace(rawQuery)) + "%"
	args = append(args, pattern, pattern, pattern, pattern)
	args = append(args, pattern, pattern, pattern, pattern)
	return args
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
	row := s.db.QueryRowContext(ctx, `
		SELECT `+skillSelectColumns("skills")+`
		FROM skills WHERE id = ?
	`, id)
	doc, err := scanSkillDocument(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	detail := &SkillDetail{Skill: *doc}
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
	row := s.db.QueryRowContext(ctx, `
		SELECT id, skill_id, version, commit_hash, source_url, checksum, skill_path,
			raw_skill_md, manifest_json, released_at, scanned_at, created_at, updated_at
		FROM skill_versions
		WHERE skill_id = ?
		ORDER BY CASE
			WHEN version = COALESCE((SELECT latest_version FROM skills WHERE id = ?), '') THEN 0
			ELSE 1
		END,
		released_at DESC, created_at DESC
		LIMIT 1
	`, skillID, skillID)
	return scanSkillVersion(row)
}

func (s *Store) GetSkillVersion(ctx context.Context, skillID, version string) (*SkillVersion, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, skill_id, version, commit_hash, source_url, checksum, skill_path,
			raw_skill_md, manifest_json, released_at, scanned_at, created_at, updated_at
		FROM skill_versions
		WHERE skill_id = ? AND version = ?
		LIMIT 1
	`, skillID, version)
	return scanSkillVersion(row)
}

func (s *Store) GetSecurityReport(ctx context.Context, skillID, version string) (*SecurityReport, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, skill_version_id, skill_id, version, score, risk_level, security_badge,
			vulnerability_status, risks_json, permissions_json, secrets_json, vulnerabilities_json,
			install_surface_json, evidence_json, artifact_kind, has_prompt_injection,
			has_shell_injection, has_data_exfiltration, scanner_version,
			llm_status, llm_verdict_json, created_at, updated_at
		FROM skill_security_reports
		WHERE skill_id = ? AND version = ?
		LIMIT 1
	`, skillID, version)
	var report SecurityReport
	var risksJSON, permissionsJSON, secretsJSON, vulnerabilitiesJSON, installSurfaceJSON, evidenceJSON, artifactKind, createdAt, updatedAt string
	var hasPromptInjection, hasShellInjection, hasDataExfiltration int
	if err := row.Scan(&report.ID, &report.SkillVersionID, &report.SkillID, &report.Version,
		&report.Score, &report.RiskLevel, &report.SecurityBadge, &report.VulnerabilityStatus,
		&risksJSON, &permissionsJSON, &secretsJSON, &vulnerabilitiesJSON, &installSurfaceJSON,
		&evidenceJSON, &artifactKind, &hasPromptInjection, &hasShellInjection, &hasDataExfiltration,
		&report.ScannerVersion, &report.LLMStatus, &report.LLMVerdictJSON, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	report.Risks = decodeFindings(risksJSON)
	report.Permissions = decodeStrings(permissionsJSON)
	report.Secrets = decodeStrings(secretsJSON)
	report.Vulnerabilities = decodeStrings(vulnerabilitiesJSON)
	_ = json.Unmarshal([]byte(installSurfaceJSON), &report.InstallSurface)
	_ = json.Unmarshal([]byte(evidenceJSON), &report.Evidence)
	if report.InstallSurface.ArtifactKind == "" {
		report.InstallSurface.ArtifactKind = artifactKind
	}
	report.HasPromptInjection = hasPromptInjection == 1
	report.HasShellInjection = hasShellInjection == 1
	report.HasDataExfiltration = hasDataExfiltration == 1
	report.CreatedAt = parseTime(createdAt)
	report.UpdatedAt = parseTime(updatedAt)
	return &report, nil
}

func (s *Store) SetInstalledSkill(ctx context.Context, installed InstalledSkill) error {
	now := timeutil.NowTime()
	if installed.InstalledAt.IsZero() {
		installed.InstalledAt = now
	}
	installed.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO installed_skills (
			skill_id, installed_version, checksum, source_url, enabled, auto_update,
			installed_at, updated_at, last_security_score
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(skill_id) DO UPDATE SET
			installed_version=excluded.installed_version,
			checksum=excluded.checksum,
			source_url=excluded.source_url,
			enabled=excluded.enabled,
			auto_update=excluded.auto_update,
			updated_at=excluded.updated_at,
			last_security_score=excluded.last_security_score
	`, installed.SkillID, installed.InstalledVersion, installed.Checksum, installed.SourceURL,
		boolToInt(installed.Enabled), boolToInt(installed.AutoUpdate), installed.InstalledAt,
		installed.UpdatedAt, installed.LastSecurityScore,
	)
	return err
}

func (s *Store) RemoveInstalledSkill(ctx context.Context, skillID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM installed_skills WHERE skill_id = ?`, skillID)
	return err
}

func (s *Store) GetInstalledSkill(ctx context.Context, skillID string) (*InstalledSkill, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT skill_id, installed_version, checksum, source_url, enabled, auto_update,
			installed_at, updated_at, last_security_score
		FROM installed_skills WHERE skill_id = ?
	`, skillID)
	return scanInstalledSkill(row)
}

func (s *Store) ListInstalledSkills(ctx context.Context) ([]InstalledSkill, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.skill_id, i.installed_version, i.checksum, i.source_url, i.enabled, i.auto_update,
			i.installed_at, i.updated_at, i.last_security_score,
			COALESCE(sk.name, ''), COALESCE(sk.latest_version, ''), COALESCE(u.latest_checksum, ''),
			COALESCE(u.action, ''), COALESCE(sk.security_badge, '')
		FROM installed_skills i
		LEFT JOIN skills sk ON sk.id = i.skill_id
		LEFT JOIN skill_update_checks u ON u.skill_id = i.skill_id
		ORDER BY i.updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []InstalledSkill
	for rows.Next() {
		var skill InstalledSkill
		var enabled, autoUpdate int
		var installedAt, updatedAt string
		if err := rows.Scan(&skill.SkillID, &skill.InstalledVersion, &skill.Checksum, &skill.SourceURL,
			&enabled, &autoUpdate, &installedAt, &updatedAt, &skill.LastSecurityScore,
			&skill.Name, &skill.LatestVersion, &skill.LatestChecksum, &skill.PendingUpdateState, &skill.SecurityBadge,
		); err != nil {
			return nil, err
		}
		skill.Enabled = enabled == 1
		skill.AutoUpdate = autoUpdate == 1
		skill.InstalledAt = parseTime(installedAt)
		skill.UpdatedAt = parseTime(updatedAt)
		skill.UpdateAvailable = skill.LatestVersion != "" && skill.LatestVersion != skill.InstalledVersion
		result = append(result, skill)
	}
	return result, nil
}

func (s *Store) RecordUpdateCheck(ctx context.Context, update AvailableUpdate) error {
	now := timeutil.NowTime()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO skill_update_checks (skill_id, checked_at, latest_version, latest_checksum, action)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(skill_id) DO UPDATE SET
			checked_at=excluded.checked_at,
			latest_version=excluded.latest_version,
			latest_checksum=excluded.latest_checksum,
			action=excluded.action
	`, update.SkillID, now, update.LatestVersion, update.LatestChecksum, update.Action)
	return err
}

func (s *Store) ListUpdates(ctx context.Context) ([]AvailableUpdate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.skill_id, i.installed_version, COALESCE(u.latest_version, ''), i.checksum,
			COALESCE(u.latest_checksum, ''), COALESCE(u.action, ''), COALESCE(u.checked_at, '')
		FROM installed_skills i
		LEFT JOIN skill_update_checks u ON u.skill_id = i.skill_id
		WHERE COALESCE(u.latest_version, '') != '' AND u.latest_version != i.installed_version
		ORDER BY u.checked_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []AvailableUpdate
	for rows.Next() {
		var item AvailableUpdate
		var checkedAt string
		if err := rows.Scan(&item.SkillID, &item.CurrentVersion, &item.LatestVersion,
			&item.CurrentChecksum, &item.LatestChecksum, &item.Action, &checkedAt,
		); err != nil {
			return nil, err
		}
		item.CheckedAt = parseTime(checkedAt)
		result = append(result, item)
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
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO skill_telemetry_daily (id, skill_id, day, downloads, installs, active_skills, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(skill_id, day) DO UPDATE SET
			downloads = skill_telemetry_daily.downloads + excluded.downloads,
			installs = skill_telemetry_daily.installs + excluded.installs,
			active_skills = skill_telemetry_daily.active_skills + excluded.active_skills,
			updated_at = excluded.updated_at
	`, uuid.NewString(), skillID, day,
		boolCount(field == "downloads"), boolCount(field == "installs"), boolCount(field == "active_skills"),
		now, now,
	)
	return err
}

func (s *Store) ListTrending(ctx context.Context, category string, limit int) ([]SkillDocument, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT ` + skillSelectColumns("skills") + ` FROM skills`
	args := []interface{}{}
	if strings.TrimSpace(category) != "" {
		query += ` WHERE category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY CASE WHEN curated_rank > 0 THEN 0 ELSE 1 END ASC, curated_rank ASC, (trending_score + curated_boost) DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []SkillDocument
	for rows.Next() {
		doc, err := scanSkillDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, *doc)
	}
	return docs, nil
}

func (s *Store) ListFeatured(ctx context.Context, category string, source string, limit int) ([]SkillDocument, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT ` + skillSelectColumns("skills") + ` FROM skills WHERE published = 1`
	args := []interface{}{}
	if strings.TrimSpace(category) != "" {
		query += ` AND category = ?`
		args = append(args, category)
	}
	if strings.TrimSpace(source) != "" {
		query += ` AND (source_group = ? OR source_name = ? OR source_id = ?)`
		args = append(args, source, source, source)
	}
	query += ` AND (curated_rank > 0 OR curated_boost > 0)`
	query += ` ORDER BY CASE WHEN curated_rank > 0 THEN 0 ELSE 1 END ASC, curated_rank ASC, (trending_score + curated_boost) DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []SkillDocument
	for rows.Next() {
		doc, err := scanSkillDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, *doc)
	}
	return docs, nil
}

func (s *Store) GetFilters(ctx context.Context) (*SkillFilters, error) {
	result := &SkillFilters{
		Installable:     map[string]int{},
		SecuritySignals: map[string]int{},
	}
	buildBuckets := func(query string, dest *[]FilterOption) error {
		rows, err := s.db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var value string
			var count int
			if err := rows.Scan(&value, &count); err != nil {
				return err
			}
			if strings.TrimSpace(value) == "" {
				continue
			}
			*dest = append(*dest, FilterOption{Value: value, Label: value, Count: count})
		}
		return nil
	}
	if err := buildBuckets(`SELECT category, COUNT(*) FROM skills WHERE published = 1 AND category != '' GROUP BY category ORDER BY category`, &result.Categories); err != nil {
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
	if err := buildBuckets(`SELECT COALESCE(source_name, source_group, source_id, ''), COUNT(*) FROM skills WHERE published = 1 GROUP BY COALESCE(source_name, source_group, source_id, '') ORDER BY COUNT(*) DESC, 1 ASC`, &result.Sources); err != nil {
		return nil, err
	}
	if err := buildBuckets(`SELECT security_badge, COUNT(*) FROM skills WHERE published = 1 GROUP BY security_badge ORDER BY security_badge`, &result.RiskBadges); err != nil {
		return nil, err
	}
	if err := buildBuckets(`SELECT install_type, COUNT(*) FROM skills WHERE published = 1 GROUP BY install_type ORDER BY install_type`, &result.InstallTypes); err != nil {
		return nil, err
	}
	if err := buildBuckets(`SELECT artifact_kind, COUNT(*) FROM skills WHERE published = 1 GROUP BY artifact_kind ORDER BY artifact_kind`, &result.ArtifactKinds); err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN installable = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN installable = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN has_vulnerabilities = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN has_prompt_injection = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN has_shell_injection = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN has_data_exfiltration = 1 THEN 1 ELSE 0 END), 0)
		FROM skills
		WHERE published = 1
	`)
	var installable, manualOnly, vulnerabilities, prompt, shell, exfil int
	if err := row.Scan(&installable, &manualOnly, &vulnerabilities, &prompt, &shell, &exfil); err != nil {
		return nil, err
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

	if _, err := tx.ExecContext(ctx, `DELETE FROM skill_curations`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE skills
		SET curated_rank = 0, curated_boost = 0, curated_label = NULL, curated_reason = NULL, published = 1
	`); err != nil {
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
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO skill_curations (id, skill_id, hidden, featured_rank, boost_weight, label, reason, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, entry.ID, entry.SkillID, boolToInt(entry.Hidden), entry.FeaturedRank, entry.BoostWeight, entry.Label, entry.Reason, entry.CreatedAt, entry.UpdatedAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE skills
			SET curated_rank = ?, curated_boost = ?, curated_label = ?, curated_reason = ?, published = CASE WHEN ? = 1 THEN 0 ELSE published END
			WHERE id = ?
		`, entry.FeaturedRank, entry.BoostWeight, entry.Label, entry.Reason, boolToInt(entry.Hidden), entry.SkillID); err != nil {
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
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO curation_sync_state (id, source_url, checksum, last_success_at, last_error, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_url=excluded.source_url,
			checksum=excluded.checksum,
			last_success_at=excluded.last_success_at,
			last_error=excluded.last_error,
			updated_at=excluded.updated_at
	`, state.ID, state.SourceURL, state.Checksum, nullTime(state.LastSuccessAt), state.LastError, state.UpdatedAt)
	return err
}

func (s *Store) GetCurationSyncState(ctx context.Context, id string) (*CurationSyncState, error) {
	if strings.TrimSpace(id) == "" {
		id = "default"
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(source_url, ''), COALESCE(checksum, ''), COALESCE(last_success_at, ''), COALESCE(last_error, ''), COALESCE(updated_at, '')
		FROM curation_sync_state
		WHERE id = ?
	`, id)
	var state CurationSyncState
	var lastSuccess, updatedAt string
	if err := row.Scan(&state.ID, &state.SourceURL, &state.Checksum, &lastSuccess, &state.LastError, &updatedAt); err != nil {
		return nil, err
	}
	state.LastSuccessAt = parseTime(lastSuccess)
	state.UpdatedAt = parseTime(updatedAt)
	return &state, nil
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
