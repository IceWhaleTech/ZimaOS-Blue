package skillstore

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Store provides skill storage and search operations.
type Store struct {
	db   *sql.DB
	zorm *ZormStore
}

// NewStore creates a new skill store.
func NewStore(db *sql.DB) (*Store, error) {
	store := &Store{
		db:   db,
		zorm: NewZormStore(db),
	}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	return store, nil
}

// initSchema creates the necessary tables and indexes.
func (s *Store) initSchema() error {
	coreSchema := `
	CREATE TABLE IF NOT EXISTS skills (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT,
		summary TEXT,
		description TEXT,
		author TEXT,
		category TEXT,
		tags TEXT,
		source_id TEXT NOT NULL,
		source_name TEXT,
		homepage TEXT,
		download_url TEXT,
		stars INTEGER DEFAULT 0,
		downloads INTEGER DEFAULT 0,
		reviews INTEGER DEFAULT 0,
		rating REAL DEFAULT 0.0,
		versions INTEGER DEFAULT 0,
		changelog TEXT,
		readme TEXT,
		readme_hash TEXT,
		dedup_key TEXT,
		installed INTEGER DEFAULT 0,
		enabled INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		search_content TEXT
	);

	CREATE TABLE IF NOT EXISTS skill_sync_status (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id TEXT NOT NULL,
		last_sync_at DATETIME,
		skill_count INTEGER DEFAULT 0,
		sync_duration_ms INTEGER DEFAULT 0,
		status TEXT DEFAULT 'pending',
		error_message TEXT,
		next_sync_at DATETIME,
		UNIQUE(source_id)
	);

	CREATE INDEX IF NOT EXISTS idx_skills_source ON skills(source_id);
	CREATE INDEX IF NOT EXISTS idx_skills_category ON skills(category);
	CREATE INDEX IF NOT EXISTS idx_skills_stars ON skills(stars DESC);
	CREATE INDEX IF NOT EXISTS idx_skills_downloads ON skills(downloads DESC);
	CREATE INDEX IF NOT EXISTS idx_skills_updated ON skills(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_skills_installed ON skills(installed);
	CREATE INDEX IF NOT EXISTS idx_skills_dedup_key ON skills(dedup_key);
	CREATE INDEX IF NOT EXISTS idx_skills_rating ON skills(rating DESC);
	`
	if _, err := s.db.Exec(coreSchema); err != nil {
		return fmt.Errorf("core tables: %w", err)
	}

	_, _ = s.db.Exec("ALTER TABLE skills ADD COLUMN readme_hash TEXT")

	ftsStatements := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS skills_fts USING fts5(
			id, name, summary, description, author, category, tags, readme,
			content='skills', content_rowid='rowid'
		)`,
		`CREATE TRIGGER IF NOT EXISTS skills_ai AFTER INSERT ON skills BEGIN
			INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags, readme)
			VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags, new.readme);
		END`,
		`CREATE TRIGGER IF NOT EXISTS skills_ad AFTER DELETE ON skills BEGIN
			INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags, readme)
			VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags, old.readme);
		END`,
		`CREATE TRIGGER IF NOT EXISTS skills_au AFTER UPDATE ON skills BEGIN
			INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags, readme)
			VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags, old.readme);
			INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags, readme)
			VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags, new.readme);
		END`,
	}
	for _, stmt := range ftsStatements {
		if _, err := s.db.Exec(stmt); err != nil {
			fmt.Printf("[skillstore] FTS5 setup warning: %v\n", err)
			break
		}
	}

	return nil
}

func (s *Store) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "skills")
}

func (s *Store) syncTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "skill_sync_status")
}

func skillToMap(skill *Skill) map[string]interface{} {
	return map[string]interface{}{
		"id": skill.ID, "name": skill.Name, "version": skill.Version,
		"summary": skill.Summary, "description": skill.Description,
		"author": skill.Author, "category": skill.Category, "tags": skill.Tags,
		"source_id": skill.SourceID, "source_name": skill.SourceName,
		"homepage": skill.Homepage, "download_url": skill.DownloadURL,
		"stars": skill.Stars, "downloads": skill.Downloads,
		"reviews": skill.Reviews, "rating": skill.Rating,
		"versions": skill.Versions, "changelog": skill.Changelog,
		"readme": skill.Readme, "dedup_key": skill.DedupKey,
		"installed": skill.Installed, "enabled": skill.Enabled,
		"created_at": skill.CreatedAt, "updated_at": skill.UpdatedAt,
		"synced_at": skill.SyncedAt, "search_content": buildSearchContent(skill),
	}
}

var upsertUpdateFields = []string{
	"name", "version", "summary", "description", "author", "category", "tags",
	"source_name", "homepage", "download_url", "stars", "downloads",
	"reviews", "rating", "versions", "changelog", "readme", "dedup_key",
	"updated_at", "synced_at", "search_content",
}

// UpsertSkill inserts or updates a skill.
func (s *Store) UpsertSkill(ctx context.Context, skill *Skill) error {
	_, err := s.table(ctx).Insert(skillToMap(skill),
		z.OnConflictDoUpdateSet([]string{"id"}, upsertUpdateFields),
	)
	return err
}

// UpsertSkillBatch inserts or updates multiple skills in a transaction.
func (s *Store) UpsertSkillBatch(ctx context.Context, skills []*Skill) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	t := z.TableContext(ctx, tx, "skills")
	for _, skill := range skills {
		_, err := t.Insert(skillToMap(skill),
			z.OnConflictDoUpdateSet([]string{"id"}, upsertUpdateFields),
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// skillRow is the intermediate struct for zorm scanning.
type skillRow struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Summary     string  `json:"summary"`
	Description string  `json:"description"`
	Author      string  `json:"author"`
	Category    string  `json:"category"`
	Tags        string  `json:"tags"`
	SourceID    string  `json:"source_id"`
	SourceName  string  `json:"source_name"`
	Homepage    string  `json:"homepage"`
	DownloadURL string  `json:"download_url"`
	Stars       int     `json:"stars"`
	Downloads   int     `json:"downloads"`
	Reviews     int     `json:"reviews"`
	Rating      float64 `json:"rating"`
	Versions    int     `json:"versions"`
	Changelog   string  `json:"changelog"`
	Readme      *string `json:"readme"`
	DedupKey    *string `json:"dedup_key"`
	Installed   bool    `json:"installed"`
	Enabled     bool    `json:"enabled"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	SyncedAt    string  `json:"synced_at"`
}

var skillFields = z.Fields(
	"id", "name", "version", "summary", "description", "author", "category", "tags",
	"source_id", "source_name", "homepage", "download_url", "stars", "downloads",
	"reviews", "rating", "versions", "changelog", "readme", "dedup_key",
	"installed", "enabled", "created_at", "updated_at", "synced_at",
)

func parseSkillTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z", s)
	}
	return t
}

func rowToSkill(r skillRow) *Skill {
	sk := &Skill{
		ID: r.ID, Name: r.Name, Version: r.Version, Summary: r.Summary,
		Description: r.Description, Author: r.Author, Category: r.Category, Tags: r.Tags,
		SourceID: r.SourceID, SourceName: r.SourceName, Homepage: r.Homepage,
		DownloadURL: r.DownloadURL, Stars: r.Stars, Downloads: r.Downloads,
		Reviews: r.Reviews, Rating: r.Rating, Versions: r.Versions, Changelog: r.Changelog,
		Installed: r.Installed, Enabled: r.Enabled,
		CreatedAt: parseSkillTime(r.CreatedAt), UpdatedAt: parseSkillTime(r.UpdatedAt), SyncedAt: parseSkillTime(r.SyncedAt),
	}
	if r.Readme != nil {
		sk.Readme = *r.Readme
	}
	if r.DedupKey != nil {
		sk.DedupKey = *r.DedupKey
	}
	return sk
}

// GetSkill retrieves a skill by ID.
func (s *Store) GetSkill(ctx context.Context, id string) (*Skill, error) {
	var rows []skillRow
	_, err := s.table(ctx).Select(&rows, skillFields,
		z.Where(z.Eq("id", id)), z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rowToSkill(rows[0]), nil
}

// Search performs a full-text search on skills with relevance scoring.
// Uses raw SQL because FTS5 JOINs and BM25 scoring don't map to zorm's query builder.
func (s *Store) Search(ctx context.Context, opts SearchOptions) (*SearchResponse, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 24
	}

	var args []interface{}
	var conditions []string
	var orderBy string
	hasQuery := opts.Query != ""

	baseQuery := `FROM skills s`
	scoreSelect := "0.0 as score"

	if hasQuery {
		baseQuery = `FROM skills s INNER JOIN skills_fts fts ON s.rowid = fts.rowid`
		conditions = append(conditions, "skills_fts MATCH ?")
		searchQuery := escapeFTS5Query(opts.Query)
		args = append(args, searchQuery)
		scoreSelect = `(
			-bm25(skills_fts, 10.0, 5.0, 3.0, 2.0, 1.0, 1.0, 1.0) +
			CASE WHEN LOWER(s.name) = LOWER(?) THEN 100.0 ELSE 0.0 END +
			CASE WHEN LOWER(s.name) LIKE LOWER(?) || '%' THEN 50.0 ELSE 0.0 END +
			CASE WHEN LOWER(s.id) = LOWER(?) THEN 80.0 ELSE 0.0 END +
			CASE WHEN LOWER(s.id) LIKE LOWER(?) || '%' THEN 40.0 ELSE 0.0 END +
			(s.stars * 0.01) + (s.downloads * 0.001)
		) as score`
	}

	if len(opts.Categories) > 0 {
		catConditions := make([]string, len(opts.Categories))
		for i, cat := range opts.Categories {
			catConditions[i] = "(s.category = ? OR s.category LIKE ? OR s.category LIKE ? OR s.category LIKE ?)"
			args = append(args, cat, cat+",%", "%,"+cat+",%", "%,"+cat)
		}
		conditions = append(conditions, "("+strings.Join(catConditions, " OR ")+")")
	}

	if len(opts.Sources) > 0 {
		placeholders := make([]string, len(opts.Sources))
		for i, src := range opts.Sources {
			placeholders[i] = "?"
			args = append(args, src)
		}
		conditions = append(conditions, fmt.Sprintf("s.source_id IN (%s)", strings.Join(placeholders, ",")))
	}

	if opts.MinStars > 0 {
		conditions = append(conditions, "s.stars >= ?")
		args = append(args, opts.MinStars)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	switch opts.SortBy {
	case "stars":
		orderBy = "s.stars"
	case "downloads":
		orderBy = "s.downloads"
	case "updated":
		orderBy = "s.updated_at"
	case "name":
		orderBy = "s.name"
	default:
		if hasQuery {
			orderBy = "score"
		} else {
			orderBy = "s.downloads"
		}
	}
	if opts.SortOrder == "asc" {
		orderBy += " ASC"
	} else {
		orderBy += " DESC"
	}

	countQuery := "SELECT COUNT(*) " + baseQuery + whereClause
	var total int64
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	totalPages := int((total + int64(opts.PageSize) - 1) / int64(opts.PageSize))

	var selectQuery string
	var selectArgs []interface{}

	if hasQuery {
		selectArgs = append(selectArgs, opts.Query, opts.Query, opts.Query, opts.Query)
		selectArgs = append(selectArgs, args...)
	} else {
		selectArgs = args
	}

	selectQuery = fmt.Sprintf(`
		SELECT s.id, s.name, s.version, s.summary, s.description, s.author, s.category, s.tags,
			s.source_id, s.source_name, s.homepage, s.download_url, s.stars, s.downloads,
			s.reviews, s.rating, s.versions, s.changelog, s.installed, s.enabled,
			s.created_at, s.updated_at, s.synced_at, %s
		%s %s ORDER BY %s LIMIT ? OFFSET ?
	`, scoreSelect, baseQuery, whereClause, orderBy)
	selectArgs = append(selectArgs, opts.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var skill Skill
		var score float64
		err := rows.Scan(
			&skill.ID, &skill.Name, &skill.Version, &skill.Summary, &skill.Description,
			&skill.Author, &skill.Category, &skill.Tags, &skill.SourceID, &skill.SourceName,
			&skill.Homepage, &skill.DownloadURL, &skill.Stars, &skill.Downloads,
			&skill.Reviews, &skill.Rating, &skill.Versions, &skill.Changelog, &skill.Installed, &skill.Enabled,
			&skill.CreatedAt, &skill.UpdatedAt, &skill.SyncedAt, &score,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, SearchResult{Skill: skill, Score: score})
	}

	var nextCursor string
	hasMore := false
	if len(results) > 0 {
		hasMore = int64(offset+len(results)) < total
		if hasMore {
			nextCursor = results[len(results)-1].Skill.ID
		}
	}

	return &SearchResponse{
		Skills: results, Total: total, Page: opts.Page,
		PageSize: opts.PageSize, TotalPages: totalPages,
		NextCursor: nextCursor, HasMore: hasMore,
	}, nil
}

// ListBySource lists all skills from a specific source.
func (s *Store) ListBySource(ctx context.Context, sourceID string, page, pageSize int) ([]*Skill, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 24
	}

	var total int64
	_, err := s.table(ctx).Select(&total,
		z.Fields("count(1)"),
		z.Where(z.Eq("source_id", sourceID)),
	)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var rows []skillRow
	_, err = s.table(ctx).Select(&rows, skillFields,
		z.Where(z.Eq("source_id", sourceID)),
		z.OrderBy("downloads DESC"),
		z.Limit(pageSize, offset),
	)
	if err != nil {
		return nil, 0, err
	}

	skills := make([]*Skill, len(rows))
	for i, r := range rows {
		skills[i] = rowToSkill(r)
	}
	return skills, total, nil
}

// GetCategories returns all unique categories.
func (s *Store) GetCategories(ctx context.Context) ([]string, error) {
	var cats []string
	_, err := s.table(ctx).Select(&cats,
		z.Fields("category"),
		z.Where(z.Neq("category", "")),
	)
	if err != nil {
		return nil, err
	}

	categorySet := make(map[string]struct{})
	for _, cat := range cats {
		for _, c := range strings.Split(cat, ",") {
			trimmed := strings.TrimSpace(c)
			if trimmed != "" {
				categorySet[trimmed] = struct{}{}
			}
		}
	}

	categories := make([]string, 0, len(categorySet))
	for cat := range categorySet {
		categories = append(categories, cat)
	}
	sort.Strings(categories)
	return categories, nil
}

// GetStats returns skill statistics.
func (s *Store) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var total int64
	_, err := s.table(ctx).Select(&total, z.Fields("count(1)"))
	if err != nil {
		return nil, err
	}
	stats["total_skills"] = total

	// by_source uses GROUP BY which zorm doesn't support well — keep raw SQL
	rows, err := s.db.QueryContext(ctx, "SELECT source_id, COUNT(*) FROM skills GROUP BY source_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySource := make(map[string]int64)
	for rows.Next() {
		var sourceID string
		var count int64
		if err := rows.Scan(&sourceID, &count); err != nil {
			return nil, err
		}
		bySource[sourceID] = count
	}
	stats["by_source"] = bySource

	var installed int64
	_, err = s.table(ctx).Select(&installed,
		z.Fields("count(1)"),
		z.Where(z.Eq("installed", 1)),
	)
	if err != nil {
		return nil, err
	}
	stats["installed"] = installed

	return stats, nil
}

// UpdateSyncStatus updates the sync status for a source.
func (s *Store) UpdateSyncStatus(ctx context.Context, status *SyncStatus) error {
	_, err := s.syncTable(ctx).Insert(
		map[string]interface{}{
			"source_id":        status.SourceID,
			"last_sync_at":     status.LastSyncAt,
			"skill_count":      status.SkillCount,
			"sync_duration_ms": status.SyncDuration,
			"status":           status.Status,
			"error_message":    status.ErrorMessage,
			"next_sync_at":     status.NextSyncAt,
		},
		z.OnConflictDoUpdateSet(
			[]string{"source_id"},
			[]string{"last_sync_at", "skill_count", "sync_duration_ms", "status", "error_message", "next_sync_at"},
		),
	)
	return err
}

// syncStatusRow for zorm scanning.
type syncStatusRow struct {
	ID           int64   `json:"id"`
	SourceID     string  `json:"source_id"`
	LastSyncAt   *string `json:"last_sync_at"`
	SkillCount   int     `json:"skill_count"`
	SyncDuration int64   `json:"sync_duration_ms"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message"`
	NextSyncAt   *string `json:"next_sync_at"`
}

// GetSyncStatus retrieves the sync status for a source.
func (s *Store) GetSyncStatus(ctx context.Context, sourceID string) (*SyncStatus, error) {
	var rows []syncStatusRow
	_, err := s.syncTable(ctx).Select(&rows,
		z.Where(z.Eq("source_id", sourceID)),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	r := rows[0]
	st := &SyncStatus{
		ID: r.ID, SourceID: r.SourceID, SkillCount: r.SkillCount,
		SyncDuration: r.SyncDuration, Status: r.Status,
	}
	if r.LastSyncAt != nil {
		st.LastSyncAt = parseSkillTime(*r.LastSyncAt)
	}
	if r.ErrorMessage != nil {
		st.ErrorMessage = *r.ErrorMessage
	}
	if r.NextSyncAt != nil {
		st.NextSyncAt = parseSkillTime(*r.NextSyncAt)
	}
	return st, nil
}

// SetInstalled marks a skill as installed or not.
func (s *Store) SetInstalled(ctx context.Context, id string, installed bool) error {
	_, err := s.table(ctx).Update(
		map[string]interface{}{"installed": installed, "updated_at": timeutil.NowTime()},
		z.Where(z.Eq("id", id)),
	)
	return err
}

// SetEnabled marks a skill as enabled or not.
func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.table(ctx).Update(
		map[string]interface{}{"enabled": enabled, "updated_at": timeutil.NowTime()},
		z.Where(z.Eq("id", id)),
	)
	return err
}

// UpdateReadme updates the readme content for a skill.
func (s *Store) UpdateReadme(ctx context.Context, id string, readme string) error {
	_, err := s.table(ctx).Update(
		map[string]interface{}{"readme": readme, "updated_at": timeutil.NowTime()},
		z.Where(z.Eq("id", id)),
	)
	return err
}

// UpdateReadmeBatch updates readme content for multiple skills using zorm.
func (s *Store) UpdateReadmeBatch(ctx context.Context, updates []readmeUpdate) error {
	return s.zorm.UpdateReadmeBatchZorm(ctx, updates)
}

// DeleteBySource deletes all skills from a source.
func (s *Store) DeleteBySource(ctx context.Context, sourceID string) (int64, error) {
	n, err := s.table(ctx).Delete(z.Where(z.Eq("source_id", sourceID)))
	return int64(n), err
}

// GetPopular returns the most popular skills by downloads.
func (s *Store) GetPopular(ctx context.Context, limit int) ([]*Skill, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var rows []skillRow
	_, err := s.table(ctx).Select(&rows,
		z.Fields("id", "name", "version", "summary", "description", "author", "category", "tags",
			"source_id", "source_name", "homepage", "download_url", "stars", "downloads",
			"reviews", "rating", "versions", "changelog", "installed", "enabled",
			"created_at", "updated_at", "synced_at"),
		z.OrderBy("downloads DESC"),
		z.Limit(limit),
	)
	if err != nil {
		return nil, err
	}
	skills := make([]*Skill, len(rows))
	for i, r := range rows {
		skills[i] = rowToSkill(r)
	}
	return skills, nil
}

// GetRecent returns the most recently updated skills.
func (s *Store) GetRecent(ctx context.Context, limit int) ([]*Skill, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var rows []skillRow
	_, err := s.table(ctx).Select(&rows,
		z.Fields("id", "name", "version", "summary", "description", "author", "category", "tags",
			"source_id", "source_name", "homepage", "download_url", "stars", "downloads",
			"reviews", "rating", "versions", "changelog", "installed", "enabled",
			"created_at", "updated_at", "synced_at"),
		z.OrderBy("updated_at DESC"),
		z.Limit(limit),
	)
	if err != nil {
		return nil, err
	}
	skills := make([]*Skill, len(rows))
	for i, r := range rows {
		skills[i] = rowToSkill(r)
	}
	return skills, nil
}

// buildSearchContent creates searchable content from a skill.
func buildSearchContent(skill *Skill) string {
	parts := []string{skill.ID, skill.Name, skill.Summary, skill.Description, skill.Author, skill.Category, skill.Tags}
	return strings.Join(parts, " ")
}

// GetInstalledSkills retrieves all installed skills from the database.
func (s *Store) GetInstalledSkills(ctx context.Context) ([]*Skill, error) {
	var rows []skillRow
	_, err := s.table(ctx).Select(&rows, skillFields,
		z.Where(z.Eq("installed", 1)),
		z.OrderBy("name ASC"),
	)
	if err != nil {
		return nil, err
	}
	skills := make([]*Skill, len(rows))
	for i, r := range rows {
		skills[i] = rowToSkill(r)
	}
	return skills, nil
}

// escapeFTS5Query escapes special characters for FTS5 queries.
func escapeFTS5Query(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	words := strings.Fields(query)
	for i, word := range words {
		word = strings.ReplaceAll(word, "\"", "\"\"")
		words[i] = "\"" + word + "\"*"
	}
	return strings.Join(words, " OR ")
}
