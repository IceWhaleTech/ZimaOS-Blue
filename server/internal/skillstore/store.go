package skillstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Store provides skill storage and search operations.
type Store struct {
	db *sql.DB
}

// NewStore creates a new skill store.
func NewStore(db *sql.DB) (*Store, error) {
	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	return store, nil
}

// initSchema creates the necessary tables and indexes.
func (s *Store) initSchema() error {
	schema := `
	-- Skills table
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
		versions INTEGER DEFAULT 0,
		changelog TEXT,
		installed INTEGER DEFAULT 0,
		enabled INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		search_content TEXT
	);

	-- Full-text search virtual table
	CREATE VIRTUAL TABLE IF NOT EXISTS skills_fts USING fts5(
		id,
		name,
		summary,
		description,
		author,
		category,
		tags,
		content='skills',
		content_rowid='rowid'
	);

	-- Triggers to keep FTS in sync
	CREATE TRIGGER IF NOT EXISTS skills_ai AFTER INSERT ON skills BEGIN
		INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags)
		VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS skills_ad AFTER DELETE ON skills BEGIN
		INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags)
		VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS skills_au AFTER UPDATE ON skills BEGIN
		INSERT INTO skills_fts(skills_fts, rowid, id, name, summary, description, author, category, tags)
		VALUES ('delete', old.rowid, old.id, old.name, old.summary, old.description, old.author, old.category, old.tags);
		INSERT INTO skills_fts(rowid, id, name, summary, description, author, category, tags)
		VALUES (new.rowid, new.id, new.name, new.summary, new.description, new.author, new.category, new.tags);
	END;

	-- Sync status table
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

	-- Indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_skills_source ON skills(source_id);
	CREATE INDEX IF NOT EXISTS idx_skills_category ON skills(category);
	CREATE INDEX IF NOT EXISTS idx_skills_stars ON skills(stars DESC);
	CREATE INDEX IF NOT EXISTS idx_skills_downloads ON skills(downloads DESC);
	CREATE INDEX IF NOT EXISTS idx_skills_updated ON skills(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_skills_installed ON skills(installed);
	`

	_, err := s.db.Exec(schema)
	return err
}

// UpsertSkill inserts or updates a skill.
func (s *Store) UpsertSkill(ctx context.Context, skill *Skill) error {
	query := `
	INSERT INTO skills (
		id, name, version, summary, description, author, category, tags,
		source_id, source_name, homepage, download_url, stars, downloads,
		versions, changelog, installed, enabled, created_at, updated_at, synced_at, search_content
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		version = excluded.version,
		summary = excluded.summary,
		description = excluded.description,
		author = excluded.author,
		category = excluded.category,
		tags = excluded.tags,
		source_name = excluded.source_name,
		homepage = excluded.homepage,
		download_url = excluded.download_url,
		stars = excluded.stars,
		downloads = excluded.downloads,
		versions = excluded.versions,
		changelog = excluded.changelog,
		updated_at = excluded.updated_at,
		synced_at = excluded.synced_at,
		search_content = excluded.search_content
	`

	// Build search content for full-text search
	searchContent := buildSearchContent(skill)

	_, err := s.db.ExecContext(ctx, query,
		skill.ID, skill.Name, skill.Version, skill.Summary, skill.Description,
		skill.Author, skill.Category, skill.Tags, skill.SourceID, skill.SourceName,
		skill.Homepage, skill.DownloadURL, skill.Stars, skill.Downloads,
		skill.Versions, skill.Changelog, skill.Installed, skill.Enabled,
		skill.CreatedAt, skill.UpdatedAt, skill.SyncedAt, searchContent,
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

	stmt, err := tx.PrepareContext(ctx, `
	INSERT INTO skills (
		id, name, version, summary, description, author, category, tags,
		source_id, source_name, homepage, download_url, stars, downloads,
		versions, changelog, installed, enabled, created_at, updated_at, synced_at, search_content
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		version = excluded.version,
		summary = excluded.summary,
		description = excluded.description,
		author = excluded.author,
		category = excluded.category,
		tags = excluded.tags,
		source_name = excluded.source_name,
		homepage = excluded.homepage,
		download_url = excluded.download_url,
		stars = excluded.stars,
		downloads = excluded.downloads,
		versions = excluded.versions,
		changelog = excluded.changelog,
		updated_at = excluded.updated_at,
		synced_at = excluded.synced_at,
		search_content = excluded.search_content
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, skill := range skills {
		searchContent := buildSearchContent(skill)
		_, err := stmt.ExecContext(ctx,
			skill.ID, skill.Name, skill.Version, skill.Summary, skill.Description,
			skill.Author, skill.Category, skill.Tags, skill.SourceID, skill.SourceName,
			skill.Homepage, skill.DownloadURL, skill.Stars, skill.Downloads,
			skill.Versions, skill.Changelog, skill.Installed, skill.Enabled,
			skill.CreatedAt, skill.UpdatedAt, skill.SyncedAt, searchContent,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetSkill retrieves a skill by ID.
func (s *Store) GetSkill(ctx context.Context, id string) (*Skill, error) {
	query := `
	SELECT id, name, version, summary, description, author, category, tags,
		source_id, source_name, homepage, download_url, stars, downloads,
		versions, changelog, installed, enabled, created_at, updated_at, synced_at
	FROM skills WHERE id = ?
	`

	skill := &Skill{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&skill.ID, &skill.Name, &skill.Version, &skill.Summary, &skill.Description,
		&skill.Author, &skill.Category, &skill.Tags, &skill.SourceID, &skill.SourceName,
		&skill.Homepage, &skill.DownloadURL, &skill.Stars, &skill.Downloads,
		&skill.Versions, &skill.Changelog, &skill.Installed, &skill.Enabled,
		&skill.CreatedAt, &skill.UpdatedAt, &skill.SyncedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return skill, err
}

// Search performs a full-text search on skills with relevance scoring.
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

	// Build base query
	baseQuery := `FROM skills s`
	scoreSelect := "0.0 as score"

	// Full-text search with relevance scoring
	if hasQuery {
		// Use FTS5 for full-text search with BM25 relevance scoring
		baseQuery = `FROM skills s
			INNER JOIN skills_fts fts ON s.rowid = fts.rowid`
		conditions = append(conditions, "skills_fts MATCH ?")
		// Escape special FTS5 characters and add prefix matching
		searchQuery := escapeFTS5Query(opts.Query)
		args = append(args, searchQuery)
		// BM25 returns negative values (more negative = more relevant), so we negate it
		// Also add boost for exact name matches and prefix matches
		scoreSelect = `(
			-bm25(skills_fts, 10.0, 5.0, 3.0, 2.0, 1.0, 1.0, 1.0) +
			CASE WHEN LOWER(s.name) = LOWER(?) THEN 100.0 ELSE 0.0 END +
			CASE WHEN LOWER(s.name) LIKE LOWER(?) || '%' THEN 50.0 ELSE 0.0 END +
			CASE WHEN LOWER(s.id) = LOWER(?) THEN 80.0 ELSE 0.0 END +
			CASE WHEN LOWER(s.id) LIKE LOWER(?) || '%' THEN 40.0 ELSE 0.0 END +
			(s.stars * 0.01) + (s.downloads * 0.001)
		) as score`
	}

	// Filter by categories
	if len(opts.Categories) > 0 {
		placeholders := make([]string, len(opts.Categories))
		for i, cat := range opts.Categories {
			placeholders[i] = "?"
			args = append(args, cat)
		}
		conditions = append(conditions, fmt.Sprintf("s.category IN (%s)", strings.Join(placeholders, ",")))
	}

	// Filter by sources
	if len(opts.Sources) > 0 {
		placeholders := make([]string, len(opts.Sources))
		for i, src := range opts.Sources {
			placeholders[i] = "?"
			args = append(args, src)
		}
		conditions = append(conditions, fmt.Sprintf("s.source_id IN (%s)", strings.Join(placeholders, ",")))
	}

	// Filter by minimum stars
	if opts.MinStars > 0 {
		conditions = append(conditions, "s.stars >= ?")
		args = append(args, opts.MinStars)
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Build ORDER BY clause
	switch opts.SortBy {
	case "stars":
		orderBy = "s.stars"
	case "downloads":
		orderBy = "s.downloads"
	case "updated":
		orderBy = "s.updated_at"
	case "name":
		orderBy = "s.name"
	case "relevance":
		if hasQuery {
			orderBy = "score"
		} else {
			orderBy = "s.downloads"
		}
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

	// Count total results (without score calculation for efficiency)
	countQuery := "SELECT COUNT(*) " + baseQuery + whereClause
	countArgs := args
	var total int64
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	// Calculate pagination
	offset := (opts.Page - 1) * opts.PageSize
	totalPages := int((total + int64(opts.PageSize) - 1) / int64(opts.PageSize))

	// Build select query with score
	var selectQuery string
	var selectArgs []interface{}

	if hasQuery {
		// Add query parameters for score calculation (exact match, prefix match for name and id)
		selectArgs = append(selectArgs, opts.Query, opts.Query, opts.Query, opts.Query)
		selectArgs = append(selectArgs, args...)
		selectQuery = fmt.Sprintf(`
			SELECT s.id, s.name, s.version, s.summary, s.description, s.author, s.category, s.tags,
				s.source_id, s.source_name, s.homepage, s.download_url, s.stars, s.downloads,
				s.versions, s.changelog, s.installed, s.enabled, s.created_at, s.updated_at, s.synced_at,
				%s
			%s %s
			ORDER BY %s
			LIMIT ? OFFSET ?
		`, scoreSelect, baseQuery, whereClause, orderBy)
	} else {
		selectArgs = args
		selectQuery = fmt.Sprintf(`
			SELECT s.id, s.name, s.version, s.summary, s.description, s.author, s.category, s.tags,
				s.source_id, s.source_name, s.homepage, s.download_url, s.stars, s.downloads,
				s.versions, s.changelog, s.installed, s.enabled, s.created_at, s.updated_at, s.synced_at,
				%s
			%s %s
			ORDER BY %s
			LIMIT ? OFFSET ?
		`, scoreSelect, baseQuery, whereClause, orderBy)
	}

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
			&skill.Versions, &skill.Changelog, &skill.Installed, &skill.Enabled,
			&skill.CreatedAt, &skill.UpdatedAt, &skill.SyncedAt, &score,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, SearchResult{Skill: skill, Score: score})
	}

	return &SearchResponse{
		Skills:     results,
		Total:      total,
		Page:       opts.Page,
		PageSize:   opts.PageSize,
		TotalPages: totalPages,
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

	// Count total
	var total int64
	countQuery := "SELECT COUNT(*) FROM skills WHERE source_id = ?"
	if err := s.db.QueryRowContext(ctx, countQuery, sourceID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch results
	offset := (page - 1) * pageSize
	query := `
	SELECT id, name, version, summary, description, author, category, tags,
		source_id, source_name, homepage, download_url, stars, downloads,
		versions, changelog, installed, enabled, created_at, updated_at, synced_at
	FROM skills WHERE source_id = ?
	ORDER BY downloads DESC
	LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, query, sourceID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var skills []*Skill
	for rows.Next() {
		skill := &Skill{}
		err := rows.Scan(
			&skill.ID, &skill.Name, &skill.Version, &skill.Summary, &skill.Description,
			&skill.Author, &skill.Category, &skill.Tags, &skill.SourceID, &skill.SourceName,
			&skill.Homepage, &skill.DownloadURL, &skill.Stars, &skill.Downloads,
			&skill.Versions, &skill.Changelog, &skill.Installed, &skill.Enabled,
			&skill.CreatedAt, &skill.UpdatedAt, &skill.SyncedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		skills = append(skills, skill)
	}

	return skills, total, nil
}

// GetCategories returns all unique categories.
func (s *Store) GetCategories(ctx context.Context) ([]string, error) {
	query := "SELECT DISTINCT category FROM skills WHERE category != '' ORDER BY category"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

// GetStats returns skill statistics.
func (s *Store) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total skills
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM skills").Scan(&total); err != nil {
		return nil, err
	}
	stats["total_skills"] = total

	// Skills by source
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

	// Installed count
	var installed int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM skills WHERE installed = 1").Scan(&installed); err != nil {
		return nil, err
	}
	stats["installed"] = installed

	return stats, nil
}

// UpdateSyncStatus updates the sync status for a source.
func (s *Store) UpdateSyncStatus(ctx context.Context, status *SyncStatus) error {
	query := `
	INSERT INTO skill_sync_status (source_id, last_sync_at, skill_count, sync_duration_ms, status, error_message, next_sync_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(source_id) DO UPDATE SET
		last_sync_at = excluded.last_sync_at,
		skill_count = excluded.skill_count,
		sync_duration_ms = excluded.sync_duration_ms,
		status = excluded.status,
		error_message = excluded.error_message,
		next_sync_at = excluded.next_sync_at
	`
	_, err := s.db.ExecContext(ctx, query,
		status.SourceID, status.LastSyncAt, status.SkillCount,
		status.SyncDuration, status.Status, status.ErrorMessage, status.NextSyncAt,
	)
	return err
}

// GetSyncStatus retrieves the sync status for a source.
func (s *Store) GetSyncStatus(ctx context.Context, sourceID string) (*SyncStatus, error) {
	query := `
	SELECT id, source_id, last_sync_at, skill_count, sync_duration_ms, status, error_message, next_sync_at
	FROM skill_sync_status WHERE source_id = ?
	`
	status := &SyncStatus{}
	err := s.db.QueryRowContext(ctx, query, sourceID).Scan(
		&status.ID, &status.SourceID, &status.LastSyncAt, &status.SkillCount,
		&status.SyncDuration, &status.Status, &status.ErrorMessage, &status.NextSyncAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return status, err
}

// SetInstalled marks a skill as installed or not.
func (s *Store) SetInstalled(ctx context.Context, id string, installed bool) error {
	query := "UPDATE skills SET installed = ?, updated_at = ? WHERE id = ?"
	_, err := s.db.ExecContext(ctx, query, installed, time.Now(), id)
	return err
}

// SetEnabled marks a skill as enabled or not.
func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) error {
	query := "UPDATE skills SET enabled = ?, updated_at = ? WHERE id = ?"
	_, err := s.db.ExecContext(ctx, query, enabled, time.Now(), id)
	return err
}

// DeleteBySource deletes all skills from a source.
func (s *Store) DeleteBySource(ctx context.Context, sourceID string) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM skills WHERE source_id = ?", sourceID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// buildSearchContent creates searchable content from a skill.
func buildSearchContent(skill *Skill) string {
	parts := []string{
		skill.ID,
		skill.Name,
		skill.Summary,
		skill.Description,
		skill.Author,
		skill.Category,
		skill.Tags,
	}
	return strings.Join(parts, " ")
}

// escapeFTS5Query escapes special characters for FTS5 queries.
func escapeFTS5Query(query string) string {
	// Remove special FTS5 operators and add prefix matching
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// Split into words and add prefix matching
	words := strings.Fields(query)
	for i, word := range words {
		// Escape quotes
		word = strings.ReplaceAll(word, "\"", "\"\"")
		// Add prefix matching for partial word search
		words[i] = "\"" + word + "\"*"
	}

	return strings.Join(words, " OR ")
}
