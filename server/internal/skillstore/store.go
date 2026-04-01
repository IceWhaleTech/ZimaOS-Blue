package skillstore

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

// Store provides skill storage and search operations.
type Store struct {
	db         *sql.DB
	readDB     *sql.DB
	zorm       *ZormStore
	ftsEnabled bool
}

// NewStore creates a new skill store.
func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

// NewStoreWithReadDB creates a new skill store with separate write and read
// database handles.
func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("db is required")
	}
	if readDB == nil {
		readDB = writeDB
	}
	store := &Store{
		db:     writeDB,
		readDB: readDB,
		zorm:   NewZormStore(writeDB),
	}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	return store, nil
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

	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS temp.skills_fts5_probe USING fts5(content)`); err != nil {
		return false
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS temp.skills_fts5_probe`)
	return true
}

func dropFTSTriggers(db *sql.DB) error {
	if db == nil {
		return nil
	}
	for _, stmt := range []string{
		`DROP TRIGGER IF EXISTS skills_ai`,
		`DROP TRIGGER IF EXISTS skills_ad`,
		`DROP TRIGGER IF EXISTS skills_au`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func dropAllSkillFTSTriggers(db *sql.DB) error {
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

func isFTSUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return (strings.Contains(msg, "no such module") && strings.Contains(msg, "fts5")) ||
		(strings.Contains(msg, "no such table") && strings.Contains(msg, "skills_fts"))
}

func searchTerms(query string) []string {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(words) == 0 {
		return nil
	}
	terms := make([]string, 0, len(words))
	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		word = strings.Trim(word, `"'*+-():,.;!?[]{} `)
		if word == "" {
			continue
		}
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		terms = append(terms, word)
	}
	return terms
}

func (s *Store) disableFTS() {
	s.ftsEnabled = false
	_ = dropFTSTriggers(s.db)
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

	ftsSupported := supportsFTS5(s.db)
	if !ftsSupported {
		if err := dropAllSkillFTSTriggers(s.db); err != nil {
			return fmt.Errorf("drop legacy skill FTS triggers: %w", err)
		}
		s.ftsEnabled = false
		return nil
	}

	if err := dropFTSTriggers(s.db); err != nil {
		return fmt.Errorf("drop skill FTS triggers: %w", err)
	}
	s.ftsEnabled = true

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
			if isFTSUnavailableError(err) {
				s.disableFTS()
				return nil
			}
			return fmt.Errorf("setup skill FTS schema: %w", err)
		}
	}

	return nil
}

func (s *Store) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "skills")
}

func (s *Store) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "skills")
}

func (s *Store) searchReadTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "skills s")
}

func (s *Store) syncTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "skill_sync_status")
}

func (s *Store) syncReadTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "skill_sync_status")
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
	ID          string  `json:"id" zorm:"id"`
	Name        string  `json:"name" zorm:"name"`
	Version     string  `json:"version" zorm:"version"`
	Summary     string  `json:"summary" zorm:"summary"`
	Description string  `json:"description" zorm:"description"`
	Author      string  `json:"author" zorm:"author"`
	Category    string  `json:"category" zorm:"category"`
	Tags        string  `json:"tags" zorm:"tags"`
	SourceID    string  `json:"source_id" zorm:"source_id"`
	SourceName  string  `json:"source_name" zorm:"source_name"`
	Homepage    string  `json:"homepage" zorm:"homepage"`
	DownloadURL string  `json:"download_url" zorm:"download_url"`
	Stars       int     `json:"stars" zorm:"stars"`
	Downloads   int     `json:"downloads" zorm:"downloads"`
	Reviews     int     `json:"reviews" zorm:"reviews"`
	Rating      float64 `json:"rating" zorm:"rating"`
	Versions    int     `json:"versions" zorm:"versions"`
	Changelog   string  `json:"changelog" zorm:"changelog"`
	Readme      *string `json:"readme" zorm:"readme"`
	DedupKey    *string `json:"dedup_key" zorm:"dedup_key"`
	Installed   bool    `json:"installed" zorm:"installed"`
	Enabled     bool    `json:"enabled" zorm:"enabled"`
	CreatedAt   string  `json:"created_at" zorm:"created_at"`
	UpdatedAt   string  `json:"updated_at" zorm:"updated_at"`
	SyncedAt    string  `json:"synced_at" zorm:"synced_at"`
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

func searchSkillFields(scoreExpr string) z.ZormItem {
	return z.Fields(
		"s.id id",
		"s.name name",
		"COALESCE(s.version, '') version",
		"COALESCE(s.summary, '') summary",
		"COALESCE(s.description, '') description",
		"COALESCE(s.author, '') author",
		"COALESCE(s.category, '') category",
		"COALESCE(s.tags, '') tags",
		"s.source_id source_id",
		"COALESCE(s.source_name, '') source_name",
		"COALESCE(s.homepage, '') homepage",
		"COALESCE(s.download_url, '') download_url",
		"COALESCE(s.stars, 0) stars",
		"COALESCE(s.downloads, 0) downloads",
		"COALESCE(s.reviews, 0) reviews",
		"COALESCE(s.rating, 0) rating",
		"COALESCE(s.versions, 0) versions",
		"COALESCE(s.changelog, '') changelog",
		"COALESCE(s.installed, 0) installed",
		"COALESCE(s.enabled, 0) enabled",
		"COALESCE(s.created_at, '') created_at",
		"COALESCE(s.updated_at, '') updated_at",
		"COALESCE(s.synced_at, '') synced_at",
		fmt.Sprintf("%s score", scoreExpr),
	)
}

func searchResultFromMapRow(row z.V) SearchResult {
	return SearchResult{
		Skill: Skill{
			ID:          stringFromMapValue(row, "id"),
			Name:        stringFromMapValue(row, "name"),
			Version:     stringFromMapValue(row, "version"),
			Summary:     stringFromMapValue(row, "summary"),
			Description: stringFromMapValue(row, "description"),
			Author:      stringFromMapValue(row, "author"),
			Category:    stringFromMapValue(row, "category"),
			Tags:        stringFromMapValue(row, "tags"),
			SourceID:    stringFromMapValue(row, "source_id"),
			SourceName:  stringFromMapValue(row, "source_name"),
			Homepage:    stringFromMapValue(row, "homepage"),
			DownloadURL: stringFromMapValue(row, "download_url"),
			Stars:       intFromMapValue(row, "stars"),
			Downloads:   intFromMapValue(row, "downloads"),
			Reviews:     intFromMapValue(row, "reviews"),
			Rating:      float64FromMapValue(row, "rating"),
			Versions:    intFromMapValue(row, "versions"),
			Changelog:   stringFromMapValue(row, "changelog"),
			Installed:   boolFromMapValue(row, "installed"),
			Enabled:     boolFromMapValue(row, "enabled"),
			CreatedAt:   parseSkillTime(stringFromMapValue(row, "created_at")),
			UpdatedAt:   parseSkillTime(stringFromMapValue(row, "updated_at")),
			SyncedAt:    parseSkillTime(stringFromMapValue(row, "synced_at")),
		},
		Score: float64FromMapValue(row, "score"),
	}
}

// GetSkill retrieves a skill by ID.
func (s *Store) GetSkill(ctx context.Context, id string) (*Skill, error) {
	var rows []skillRow
	_, err := s.readTable(ctx).Select(&rows, skillFields,
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
	hasQuery := strings.TrimSpace(opts.Query) != ""
	useFTS := hasQuery && s.ftsEnabled

	scoreExpr := "0.0"

	if useFTS {
		conditions = append(conditions, "skills_fts MATCH ?")
		args = append(args, escapeFTS5Query(opts.Query))
		scoreExpr = ftsSearchScoreExpr("s", opts.Query)
	} else if hasQuery {
		terms := searchTerms(opts.Query)
		if len(terms) == 0 {
			hasQuery = false
		} else {
			likeConditions := make([]string, 0, len(terms))
			for _, term := range terms {
				pattern := "%" + term + "%"
				likeConditions = append(likeConditions, "LOWER(s.search_content) LIKE ?")
				args = append(args, pattern)
			}
			conditions = append(conditions, "("+strings.Join(likeConditions, " OR ")+")")
			scoreExpr = fallbackSearchScoreExpr("s", opts.Query)
		}
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

	var total int64
	if useFTS {
		trimmedWhere := trimClausePrefix(whereClause, "WHERE")
		countOpts := []z.ZormItem{
			z.Fields("COUNT(*) AS total"),
			z.InnerJoin("skills_fts", z.Expr("s.rowid = skills_fts.rowid")),
		}
		if trimmedWhere != "" {
			countOpts = append(countOpts, z.Where(append([]interface{}{trimmedWhere}, args...)...))
		}
		var rows []z.V
		if _, err := s.searchReadTable(ctx).Select(&rows, countOpts...); err != nil {
			if isFTSUnavailableError(err) {
				s.disableFTS()
				return s.Search(ctx, opts)
			}
			return nil, err
		}
		if len(rows) > 0 {
			total = int64FromMapValue(rows[0], "total")
		}
	} else {
		countOpts := []z.ZormItem{z.Fields("count(1)")}
		trimmedWhere := trimClausePrefix(whereClause, "WHERE")
		if trimmedWhere != "" {
			countOpts = append(countOpts, z.Where(append([]interface{}{trimmedWhere}, args...)...))
		}
		if _, err := s.searchReadTable(ctx).Select(&total, countOpts...); err != nil {
			return nil, err
		}
	}

	offset := (opts.Page - 1) * opts.PageSize
	totalPages := int((total + int64(opts.PageSize) - 1) / int64(opts.PageSize))

	var results []SearchResult
	if useFTS {
		trimmedWhere := trimClausePrefix(whereClause, "WHERE")
		selectOpts := []z.ZormItem{
			searchSkillFields(scoreExpr),
			z.InnerJoin("skills_fts", z.Expr("s.rowid = skills_fts.rowid")),
		}
		if trimmedWhere != "" {
			selectOpts = append(selectOpts, z.Where(append([]interface{}{trimmedWhere}, args...)...))
		}
		selectOpts = append(selectOpts, z.OrderBy(orderBy), z.Limit(opts.PageSize, offset))
		var rows []z.V
		if _, err := s.searchReadTable(ctx).Select(&rows, selectOpts...); err != nil {
			if isFTSUnavailableError(err) {
				s.disableFTS()
				return s.Search(ctx, opts)
			}
			return nil, err
		}
		results = make([]SearchResult, 0, len(rows))
		for _, row := range rows {
			results = append(results, searchResultFromMapRow(row))
		}
	} else {
		trimmedWhere := trimClausePrefix(whereClause, "WHERE")
		selectOpts := []z.ZormItem{
			searchSkillFields(scoreExpr),
		}
		if trimmedWhere != "" {
			selectOpts = append(selectOpts, z.Where(append([]interface{}{trimmedWhere}, args...)...))
		}
		selectOpts = append(selectOpts, z.OrderBy(orderBy), z.Limit(opts.PageSize, offset))
		var rows []z.V
		if _, err := s.searchReadTable(ctx).Select(&rows, selectOpts...); err != nil {
			return nil, err
		}
		results = make([]SearchResult, 0, len(rows))
		for _, row := range rows {
			results = append(results, searchResultFromMapRow(row))
		}
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
	_, err := s.readTable(ctx).Select(&total,
		z.Fields("count(1)"),
		z.Where(z.Eq("source_id", sourceID)),
	)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var rows []skillRow
	_, err = s.readTable(ctx).Select(&rows, skillFields,
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
	_, err := s.readTable(ctx).Select(&cats,
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
	_, err := s.readTable(ctx).Select(&total, z.Fields("count(1)"))
	if err != nil {
		return nil, err
	}
	stats["total_skills"] = total

	var sourceRows []z.V
	_, err = s.readTable(ctx).Select(&sourceRows,
		z.Fields("source_id", "COUNT(*) as count"),
		z.GroupBy("source_id"),
	)
	if err != nil {
		return nil, err
	}

	bySource := make(map[string]int64, len(sourceRows))
	for _, row := range sourceRows {
		bySource[stringFromMapValue(row, "source_id")] = int64FromMapValue(row, "count")
	}
	stats["by_source"] = bySource

	var installed int64
	_, err = s.readTable(ctx).Select(&installed,
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
	ID           int64   `json:"id" zorm:"id"`
	SourceID     string  `json:"source_id" zorm:"source_id"`
	LastSyncAt   *string `json:"last_sync_at" zorm:"last_sync_at"`
	SkillCount   int     `json:"skill_count" zorm:"skill_count"`
	SyncDuration int64   `json:"sync_duration_ms" zorm:"sync_duration_ms"`
	Status       string  `json:"status" zorm:"status"`
	ErrorMessage *string `json:"error_message" zorm:"error_message"`
	NextSyncAt   *string `json:"next_sync_at" zorm:"next_sync_at"`
}

// GetSyncStatus retrieves the sync status for a source.
func (s *Store) GetSyncStatus(ctx context.Context, sourceID string) (*SyncStatus, error) {
	var rows []syncStatusRow
	_, err := s.syncReadTable(ctx).Select(&rows,
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
		z.V{"installed": installed, "updated_at": timeutil.NowTime()},
		z.Where(z.Eq("id", id)),
	)
	return err
}

// SetEnabled marks a skill as enabled or not.
func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.table(ctx).Update(
		z.V{"enabled": enabled, "updated_at": timeutil.NowTime()},
		z.Where(z.Eq("id", id)),
	)
	return err
}

// UpdateReadme updates the readme content for a skill.
func (s *Store) UpdateReadme(ctx context.Context, id string, readme string) error {
	_, err := s.table(ctx).Update(
		z.V{"readme": readme, "updated_at": timeutil.NowTime()},
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
	_, err := s.readTable(ctx).Select(&rows,
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
	_, err := s.readTable(ctx).Select(&rows,
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
	_, err := s.readTable(ctx).Select(&rows, skillFields,
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

func trimClausePrefix(clause, prefix string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(clause), prefix))
}

func sqlStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func fallbackSearchScoreExpr(alias, rawQuery string) string {
	rawQuery = strings.TrimSpace(rawQuery)
	queryLiteral := sqlStringLiteral(rawQuery)
	scoreParts := []string{
		"CASE WHEN LOWER(" + alias + ".name) = LOWER(" + queryLiteral + ") THEN 100.0 ELSE 0.0 END",
		"CASE WHEN LOWER(" + alias + ".name) LIKE LOWER(" + queryLiteral + ") || '%' THEN 50.0 ELSE 0.0 END",
		"CASE WHEN LOWER(" + alias + ".id) = LOWER(" + queryLiteral + ") THEN 80.0 ELSE 0.0 END",
		"CASE WHEN LOWER(" + alias + ".id) LIKE LOWER(" + queryLiteral + ") || '%' THEN 40.0 ELSE 0.0 END",
	}
	for _, term := range searchTerms(rawQuery) {
		scoreParts = append(scoreParts, "CASE WHEN LOWER(COALESCE("+alias+".search_content, '')) LIKE "+sqlStringLiteral("%"+term+"%")+" THEN 5.0 ELSE 0.0 END")
	}
	scoreParts = append(scoreParts, "("+alias+".stars * 0.01)", "("+alias+".downloads * 0.001)")
	return "(" + strings.Join(scoreParts, " + ") + ")"
}

func ftsSearchScoreExpr(alias, rawQuery string) string {
	rawQuery = strings.TrimSpace(rawQuery)
	queryLiteral := sqlStringLiteral(rawQuery)
	return `(
		-bm25(skills_fts, 10.0, 5.0, 3.0, 2.0, 1.0, 1.0, 1.0) +
		CASE WHEN LOWER(` + alias + `.name) = LOWER(` + queryLiteral + `) THEN 100.0 ELSE 0.0 END +
		CASE WHEN LOWER(` + alias + `.name) LIKE LOWER(` + queryLiteral + `) || '%' THEN 50.0 ELSE 0.0 END +
		CASE WHEN LOWER(` + alias + `.id) = LOWER(` + queryLiteral + `) THEN 80.0 ELSE 0.0 END +
		CASE WHEN LOWER(` + alias + `.id) LIKE LOWER(` + queryLiteral + `) || '%' THEN 40.0 ELSE 0.0 END +
		(` + alias + `.stars * 0.01) + (` + alias + `.downloads * 0.001)
	)`
}

func stringFromMapValue(row z.V, key string) string {
	value, ok := valueFromMapKey(row, key)
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

func intFromMapValue(row z.V, key string) int {
	return int(int64FromMapValue(row, key))
}

func int64FromMapValue(row z.V, key string) int64 {
	value, ok := valueFromMapKey(row, key)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		return int64(typed)
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	case bool:
		if typed {
			return 1
		}
		return 0
	case []byte:
		n, _ := strconv.ParseInt(string(typed), 10, 64)
		return n
	case string:
		n, _ := strconv.ParseInt(typed, 10, 64)
		return n
	default:
		return 0
	}
}

func float64FromMapValue(row z.V, key string) float64 {
	value, ok := valueFromMapKey(row, key)
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

func boolFromMapValue(row z.V, key string) bool {
	value, ok := valueFromMapKey(row, key)
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int8:
		return typed != 0
	case int16:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case uint:
		return typed != 0
	case uint8:
		return typed != 0
	case uint16:
		return typed != 0
	case uint32:
		return typed != 0
	case uint64:
		return typed != 0
	case float32:
		return typed != 0
	case float64:
		return typed != 0
	case []byte:
		return string(typed) == "1" || strings.EqualFold(string(typed), "true")
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func valueFromMapKey(row z.V, key string) (interface{}, bool) {
	if row == nil {
		return nil, false
	}
	if value, ok := row[key]; ok {
		return value, true
	}
	for rawKey, value := range row {
		if normalizeMapKey(rawKey) == key {
			return value, true
		}
	}
	return nil, false
}

func normalizeMapKey(key string) string {
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
