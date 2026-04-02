package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

type sqliteFTSRepairPlan struct {
	virtualTable     string
	contentTable     string
	triggerNames     []string
	createStatements []string
	rebuildStatement string
}

var sqliteKnownFTSRepairPlans = []sqliteFTSRepairPlan{
	{
		virtualTable: "skills_fts",
		contentTable: "skills",
		triggerNames: []string{"skills_ai", "skills_ad", "skills_au"},
		createStatements: []string{
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
		},
		rebuildStatement: `INSERT INTO skills_fts(skills_fts) VALUES('rebuild')`,
	},
	{
		virtualTable: "skillmarket_fts",
		contentTable: "skills",
		triggerNames: []string{"skillmarket_ai", "skillmarket_ad", "skillmarket_au"},
		createStatements: []string{
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
		},
		rebuildStatement: `INSERT INTO skillmarket_fts(skillmarket_fts) VALUES('rebuild')`,
	},
	{
		virtualTable: "memory_fts",
		contentTable: "memory_chunks",
		triggerNames: []string{"memory_chunks_ai", "memory_chunks_ad", "memory_chunks_au"},
		createStatements: []string{
			`CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
				content,
				tags,
				content=memory_chunks,
				content_rowid=id,
				tokenize='porter unicode61'
			)`,
			`CREATE TRIGGER IF NOT EXISTS memory_chunks_ai AFTER INSERT ON memory_chunks BEGIN
				INSERT INTO memory_fts(rowid, content, tags) VALUES (new.id, new.content, new.tags);
			END`,
			`CREATE TRIGGER IF NOT EXISTS memory_chunks_ad AFTER DELETE ON memory_chunks BEGIN
				INSERT INTO memory_fts(memory_fts, rowid, content, tags) VALUES('delete', old.id, old.content, old.tags);
			END`,
			`CREATE TRIGGER IF NOT EXISTS memory_chunks_au AFTER UPDATE ON memory_chunks BEGIN
				INSERT INTO memory_fts(memory_fts, rowid, content, tags) VALUES('delete', old.id, old.content, old.tags);
				INSERT INTO memory_fts(rowid, content, tags) VALUES (new.id, new.content, new.tags);
			END`,
		},
		rebuildStatement: `INSERT INTO memory_fts(memory_fts) VALUES('rebuild')`,
	},
}

// RepairKnownFTSIndexes rebuilds application-owned FTS virtual tables when their
// shadow tables are corrupted but the source tables are still intact.
func RepairKnownFTSIndexes(dbPath string) (repaired []string, err error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("database path is empty")
	}
	if dbPath == ":memory:" {
		return nil, nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("failed to stat database %s: %w", dbPath, err)
	}
	startedAt := time.Now()
	sqliteLogf("fts repair started db_path=%s", dbPath)
	defer func() {
		if err != nil {
			sqliteLogf("fts repair failed db_path=%s duration=%s error=%v", dbPath, time.Since(startedAt), err)
			return
		}
		if len(repaired) > 0 {
			sqliteLogf("fts repair completed db_path=%s duration=%s tables=%v", dbPath, time.Since(startedAt), repaired)
		}
	}()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %s for FTS repair: %w", dbPath, err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		return nil, fmt.Errorf("set busy timeout for %s: %w", dbPath, err)
	}

	repaired = make([]string, 0)
	for _, plan := range sqliteKnownFTSRepairPlans {
		present, contentExists, err := sqliteFTSPlanPresence(db, plan)
		if err != nil {
			return repaired, fmt.Errorf("inspect FTS plan %s: %w", plan.virtualTable, err)
		}
		if !present {
			continue
		}
		if err := rebuildSQLiteFTSPlan(db, plan, contentExists); err != nil {
			return repaired, fmt.Errorf("rebuild FTS table %s: %w", plan.virtualTable, err)
		}
		repaired = append(repaired, plan.virtualTable)
	}

	return repaired, nil
}

func sqliteFTSPlanPresence(db *sql.DB, plan sqliteFTSRepairPlan) (present bool, contentExists bool, err error) {
	contentExists, err = sqliteObjectExists(db, "table", plan.contentTable)
	if err != nil {
		return false, false, err
	}

	virtualExists, err := sqliteObjectExists(db, "table", plan.virtualTable)
	if err != nil {
		return false, false, err
	}
	if virtualExists {
		return true, contentExists, nil
	}

	for _, triggerName := range plan.triggerNames {
		triggerExists, err := sqliteObjectExists(db, "trigger", triggerName)
		if err != nil {
			return false, false, err
		}
		if triggerExists {
			return true, contentExists, nil
		}
	}

	return false, contentExists, nil
}

func sqliteObjectExists(db *sql.DB, objectType, name string) (bool, error) {
	var exists int
	err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = ? AND name = ?)`,
		objectType,
		name,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

func rebuildSQLiteFTSPlan(db *sql.DB, plan sqliteFTSRepairPlan, contentExists bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, triggerName := range plan.triggerNames {
		if _, err := tx.Exec(`DROP TRIGGER IF EXISTS ` + triggerName); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS ` + plan.virtualTable); err != nil {
		return err
	}

	if contentExists {
		for _, stmt := range plan.createStatements {
			if _, err := tx.Exec(stmt); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(plan.rebuildStatement); err != nil {
			return err
		}
	}

	return tx.Commit()
}
