package harness

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type SkillRevisionStatus string

const (
	SkillRevisionStatusCandidate SkillRevisionStatus = "candidate"
	SkillRevisionStatusAccepted  SkillRevisionStatus = "accepted"
	SkillRevisionStatusRejected  SkillRevisionStatus = "rejected"
	SkillRevisionStatusPromoted  SkillRevisionStatus = "promoted"
	SkillRevisionStatusBackup    SkillRevisionStatus = "backup"
)

type SkillRevision struct {
	ID                 string              `json:"id"`
	SkillID            string              `json:"skill_id"`
	Status             SkillRevisionStatus `json:"status"`
	SourcePath         string              `json:"source_path,omitempty"`
	CandidateID        string              `json:"candidate_id,omitempty"`
	ParentRevisionID   string              `json:"parent_revision_id,omitempty"`
	BackupOfRevisionID string              `json:"backup_of_revision_id,omitempty"`
	EvalRunID          string              `json:"eval_run_id,omitempty"`
	OptimizationRunID  string              `json:"optimization_run_id,omitempty"`
	Content            string              `json:"content,omitempty"`
	ContentSHA256      string              `json:"content_sha256,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	PromotedAt         *time.Time          `json:"promoted_at,omitempty"`
}

type SkillRevisionFilter struct {
	SkillID  string                `json:"skill_id,omitempty"`
	Statuses []SkillRevisionStatus `json:"statuses,omitempty"`
	Limit    int                   `json:"limit,omitempty"`
}

type SkillOptimizeRequest struct {
	EvalRunID   string `json:"eval_run_id"`
	CandidateID string `json:"candidate_id,omitempty"`
	SourcePath  string `json:"source_path,omitempty"`
}

type SkillPromoteResult struct {
	PromotedRevisionID string `json:"promoted_revision_id"`
	BackupRevisionID   string `json:"backup_revision_id"`
	WrittenSourcePath  string `json:"written_source_path"`
}

type SkillRevisionPromotionRecorder interface {
	RecordSkillRevisionPromotion(ctx context.Context, promotedRevision *SkillRevision, backupRevision *SkillRevision, writtenSourcePath string) error
}

type skillRevisionRow struct {
	ID                 string         `zorm:"id"`
	SkillID            string         `zorm:"skill_id"`
	Status             string         `zorm:"status"`
	SourcePath         string         `zorm:"source_path"`
	CandidateID        string         `zorm:"candidate_id"`
	ParentRevisionID   string         `zorm:"parent_revision_id"`
	BackupOfRevisionID string         `zorm:"backup_of_revision_id"`
	EvalRunID          string         `zorm:"eval_run_id"`
	OptimizationRunID  string         `zorm:"optimization_run_id"`
	Content            string         `zorm:"content"`
	ContentSHA256      string         `zorm:"content_sha256"`
	CreatedAt          time.Time      `zorm:"created_at"`
	PromotedAt         sql.NullString `zorm:"promoted_at"`
}

func skillRevisionFromRow(row skillRevisionRow) SkillRevision {
	revision := SkillRevision{
		ID:                 row.ID,
		SkillID:            row.SkillID,
		Status:             SkillRevisionStatus(row.Status),
		SourcePath:         row.SourcePath,
		CandidateID:        row.CandidateID,
		ParentRevisionID:   row.ParentRevisionID,
		BackupOfRevisionID: row.BackupOfRevisionID,
		EvalRunID:          row.EvalRunID,
		OptimizationRunID:  row.OptimizationRunID,
		Content:            row.Content,
		ContentSHA256:      row.ContentSHA256,
		CreatedAt:          row.CreatedAt,
	}
	if row.PromotedAt.Valid {
		ts, ok := parseSkillRevisionTime(row.PromotedAt.String)
		if ok {
			revision.PromotedAt = &ts
		}
	}
	return revision
}

func parseSkillRevisionTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		ts, err := time.Parse(format, raw)
		if err == nil {
			return ts, true
		}
	}
	if unixNS, err := time.ParseDuration(raw + "ns"); err == nil {
		return time.Unix(0, unixNS.Nanoseconds()).UTC(), true
	}
	return time.Time{}, false
}

func (s *SQLiteStore) CreateSkillRevision(ctx context.Context, revision *SkillRevision) error {
	if err := normalizeSkillRevisionForWrite(revision); err != nil {
		return err
	}
	_, err := s.execContext(ctx, `INSERT INTO harness_skill_revisions (
		id, skill_id, status, source_path, candidate_id, parent_revision_id, backup_of_revision_id,
		eval_run_id, optimization_run_id, content, content_sha256, created_at, promoted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		revision.ID, revision.SkillID, string(revision.Status), revision.SourcePath, revision.CandidateID,
		revision.ParentRevisionID, revision.BackupOfRevisionID, revision.EvalRunID, revision.OptimizationRunID,
		revision.Content, revision.ContentSHA256, revision.CreatedAt, nullableSkillRevisionTime(revision.PromotedAt),
	)
	return err
}

func (c *Controller) CreateSkillRevision(ctx context.Context, revision SkillRevision) (*SkillRevision, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(revision.ID) == "" {
		revision.ID = uuid.NewString()
	}
	if err := c.store.CreateSkillRevision(ctx, &revision); err != nil {
		return nil, err
	}
	return c.store.GetSkillRevision(ctx, revision.ID)
}

func (s *SQLiteStore) UpdateSkillRevision(ctx context.Context, revision *SkillRevision) error {
	if err := normalizeSkillRevisionForWrite(revision); err != nil {
		return err
	}
	_, err := s.execContext(ctx, `UPDATE harness_skill_revisions SET
		skill_id=?, status=?, source_path=?, candidate_id=?, parent_revision_id=?, backup_of_revision_id=?,
		eval_run_id=?, optimization_run_id=?, content=?, content_sha256=?, created_at=?, promoted_at=?
		WHERE id=?`,
		revision.SkillID, string(revision.Status), revision.SourcePath, revision.CandidateID,
		revision.ParentRevisionID, revision.BackupOfRevisionID, revision.EvalRunID, revision.OptimizationRunID,
		revision.Content, revision.ContentSHA256, revision.CreatedAt, nullableSkillRevisionTime(revision.PromotedAt),
		revision.ID,
	)
	return err
}

func (c *Controller) UpdateSkillRevision(ctx context.Context, revision SkillRevision) (*SkillRevision, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if err := c.store.UpdateSkillRevision(ctx, &revision); err != nil {
		return nil, err
	}
	return c.store.GetSkillRevision(ctx, revision.ID)
}

func (s *SQLiteStore) GetSkillRevision(ctx context.Context, id string) (*SkillRevision, error) {
	rows, err := s.selectSkillRevisionRows(ctx, s.reader(), strings.TrimSpace(id))
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.selectSkillRevisionRows(ctx, s.db, strings.TrimSpace(id))
		if err != nil {
			return nil, err
		}
	} else if len(rows) == 0 && s.hasSeparateReader() {
		rows, err = s.selectSkillRevisionRows(ctx, s.db, strings.TrimSpace(id))
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	revision := skillRevisionFromRow(rows[0])
	return &revision, nil
}

func (c *Controller) GetSkillRevision(ctx context.Context, id string) (*SkillRevision, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.GetSkillRevision(ctx, strings.TrimSpace(id))
}

func (s *SQLiteStore) selectSkillRevisionRows(ctx context.Context, db *sql.DB, id string) ([]skillRevisionRow, error) {
	return querySkillRevisionRows(ctx, db, `SELECT
		id, skill_id, status, source_path, candidate_id, parent_revision_id, backup_of_revision_id,
		eval_run_id, optimization_run_id, content, content_sha256, created_at, promoted_at
		FROM harness_skill_revisions
		WHERE id = ?
		LIMIT 1`, strings.TrimSpace(id))
}

func (s *SQLiteStore) ListSkillRevisions(ctx context.Context, filter SkillRevisionFilter) ([]SkillRevision, error) {
	rows, err := s.listSkillRevisionRows(ctx, s.reader(), filter)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.listSkillRevisionRows(ctx, s.db, filter)
		if err != nil {
			return nil, err
		}
	} else if s.hasSeparateReader() {
		writerRows, writerErr := s.listSkillRevisionRows(ctx, s.db, filter)
		if writerErr == nil {
			rows = mergeUniqueRowsByKey(rows, writerRows, filter.Limit, func(row skillRevisionRow) string { return row.ID })
		}
	}
	out := make([]SkillRevision, 0, len(rows))
	for i := range rows {
		out = append(out, skillRevisionFromRow(rows[i]))
	}
	return out, nil
}

func (c *Controller) ListSkillRevisions(ctx context.Context, filter SkillRevisionFilter) ([]SkillRevision, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.ListSkillRevisions(ctx, filter)
}

func (s *SQLiteStore) listSkillRevisionRows(ctx context.Context, db *sql.DB, filter SkillRevisionFilter) ([]skillRevisionRow, error) {
	var (
		conds []string
		args  []interface{}
	)
	if v := strings.TrimSpace(filter.SkillID); v != "" {
		conds = append(conds, "skill_id = ?")
		args = append(args, v)
	}
	if len(filter.Statuses) > 0 {
		statuses := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			if status == "" {
				continue
			}
			statuses = append(statuses, string(status))
		}
		if len(statuses) > 0 {
			placeholders := make([]string, 0, len(statuses))
			for _, status := range statuses {
				placeholders = append(placeholders, "?")
				args = append(args, status)
			}
			conds = append(conds, "status IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(conds) > 0 {
		args = append(args, limit)
		return querySkillRevisionRows(ctx, db, `SELECT
			id, skill_id, status, source_path, candidate_id, parent_revision_id, backup_of_revision_id,
			eval_run_id, optimization_run_id, content, content_sha256, created_at, promoted_at
			FROM harness_skill_revisions
			WHERE `+strings.Join(conds, " AND ")+`
			ORDER BY created_at DESC, rowid DESC
			LIMIT ?`, args...)
	}
	return querySkillRevisionRows(ctx, db, `SELECT
		id, skill_id, status, source_path, candidate_id, parent_revision_id, backup_of_revision_id,
		eval_run_id, optimization_run_id, content, content_sha256, created_at, promoted_at
		FROM harness_skill_revisions
		ORDER BY created_at DESC, rowid DESC
		LIMIT ?`, limit)
}

func normalizeSkillRevisionForWrite(revision *SkillRevision) error {
	if revision == nil {
		return fmt.Errorf("skill revision is required")
	}
	revision.ID = strings.TrimSpace(revision.ID)
	revision.SkillID = strings.TrimSpace(revision.SkillID)
	revision.SourcePath = normalizeSkillSourcePath(revision.SourcePath)
	revision.CandidateID = strings.TrimSpace(revision.CandidateID)
	revision.ParentRevisionID = strings.TrimSpace(revision.ParentRevisionID)
	revision.BackupOfRevisionID = strings.TrimSpace(revision.BackupOfRevisionID)
	revision.EvalRunID = strings.TrimSpace(revision.EvalRunID)
	revision.OptimizationRunID = strings.TrimSpace(revision.OptimizationRunID)
	if revision.ID == "" {
		return fmt.Errorf("skill revision id is required")
	}
	if revision.SkillID == "" {
		return fmt.Errorf("skill revision skill_id is required")
	}
	if revision.Status == "" {
		return fmt.Errorf("skill revision status is required")
	}
	if revision.CreatedAt.IsZero() {
		revision.CreatedAt = timeutil.NowTime()
	}
	revision.ContentSHA256 = skillRevisionSHA256(revision.Content)
	return nil
}

func nullableSkillRevisionTime(ts *time.Time) interface{} {
	if ts == nil || ts.IsZero() {
		return nil
	}
	return *ts
}

func (c *Controller) PromoteSkillRevision(ctx context.Context, revisionID string) (*SkillPromoteResult, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	revision, err := c.store.GetSkillRevision(ctx, strings.TrimSpace(revisionID))
	if err != nil {
		return nil, err
	}
	if revision.Status != SkillRevisionStatusAccepted {
		return nil, fmt.Errorf("skill revision must be accepted before promote")
	}
	absPath, normalizedSourcePath, err := resolveCanonicalSkillSourcePath(revision.SkillID, revision.SourcePath)
	if err != nil {
		return nil, err
	}
	currentContent, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	now := timeutil.NowTime()
	backup := &SkillRevision{
		ID:                 uuid.NewString(),
		SkillID:            revision.SkillID,
		Status:             SkillRevisionStatusBackup,
		SourcePath:         normalizedSourcePath,
		CandidateID:        revision.CandidateID,
		BackupOfRevisionID: revision.ID,
		EvalRunID:          revision.EvalRunID,
		OptimizationRunID:  revision.OptimizationRunID,
		Content:            string(currentContent),
		CreatedAt:          now,
	}
	if err := c.store.CreateSkillRevision(ctx, backup); err != nil {
		return nil, err
	}
	if err := os.WriteFile(absPath, []byte(revision.Content), 0o644); err != nil {
		return nil, err
	}
	revision.Status = SkillRevisionStatusPromoted
	revision.SourcePath = normalizedSourcePath
	revision.PromotedAt = &now
	if err := c.store.UpdateSkillRevision(ctx, revision); err != nil {
		return nil, err
	}
	c.mu.RLock()
	triggerer := c.optimization
	c.mu.RUnlock()
	if recorder, ok := triggerer.(SkillRevisionPromotionRecorder); ok {
		if err := recorder.RecordSkillRevisionPromotion(ctx, revision, backup, absPath); err != nil {
			return nil, err
		}
	}
	return &SkillPromoteResult{
		PromotedRevisionID: revision.ID,
		BackupRevisionID:   backup.ID,
		WrittenSourcePath:  absPath,
	}, nil
}

func (c *Controller) OptimizeSkill(ctx context.Context, skillID string, req SkillOptimizeRequest) (*OptimizationTrigger, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return nil, fmt.Errorf("skill_id is required")
	}
	evalRunID := strings.TrimSpace(req.EvalRunID)
	if evalRunID == "" {
		return nil, fmt.Errorf("eval_run_id is required")
	}
	evalRun, err := c.store.GetEvalRun(ctx, evalRunID)
	if err != nil {
		return nil, err
	}
	if evalRun == nil {
		return nil, fmt.Errorf("eval run %q not found", evalRunID)
	}
	if evalRun.Status != RunGroupStatusCompleted {
		return nil, fmt.Errorf("manual skill optimize requires a completed eval run")
	}
	evalSpec, err := c.store.GetEvalSpec(ctx, strings.TrimSpace(evalRun.EvalSpecID))
	if err != nil {
		return nil, err
	}
	followupGate, err := inferSkillOptimizeFollowupGate(evalRun, evalSpec)
	if err != nil {
		return nil, err
	}

	sourceCandidate := nestedMetadataMap(evalRun.Metadata, "skill_candidate")
	sourcePath := firstNonEmptySkillRevisionValue(
		strings.TrimSpace(req.SourcePath),
		metadataString(sourceCandidate, "source_path"),
		normalizeSkillSourcePath(filepath.Join("assets", "skills", skillID, "SKILL.md")),
	)
	if _, normalizedSourcePath, err := resolveCanonicalSkillSourcePath(skillID, sourcePath); err != nil {
		return nil, err
	} else {
		sourcePath = normalizedSourcePath
	}
	if existingSkillID := strings.TrimSpace(metadataString(sourceCandidate, "skill_id")); existingSkillID != "" && existingSkillID != skillID {
		return nil, fmt.Errorf("skill candidate skill_id %q does not match requested skill %q", existingSkillID, skillID)
	}

	candidateID := firstNonEmptySkillRevisionValue(
		strings.TrimSpace(req.CandidateID),
		evalRunCandidateID(evalRun),
		metadataString(sourceCandidate, "candidate_id"),
	)
	metadata := evalRunOptimizationMetadata(evalRun)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["followup_gate"] = followupGate
	metadata["optimization_surface"] = string(OptimizationSurfaceSkillDefinition)
	if candidateID != "" {
		metadata["candidate_id"] = candidateID
	}
	skillCandidate := cloneMetadataMap(sourceCandidate)
	if skillCandidate == nil {
		skillCandidate = map[string]interface{}{}
	}
	skillCandidate["skill_id"] = skillID
	skillCandidate["source_path"] = sourcePath
	if candidateID != "" {
		skillCandidate["candidate_id"] = candidateID
	}
	metadata["skill_candidate"] = skillCandidate

	event := OptimizationTrigger{
		Reason:              OptimizationReasonManualSkillOptimize,
		CandidateID:         candidateID,
		EvalRunID:           evalRun.ID,
		BaseEvalRunID:       strings.TrimSpace(evalRun.BaselineEvalRunID),
		OptimizationSurface: OptimizationSurfaceSkillDefinition,
		Metadata:            metadata,
	}
	c.mu.RLock()
	triggerer := c.optimization
	c.mu.RUnlock()
	if triggerer == nil {
		return nil, fmt.Errorf("optimization triggerer is not configured")
	}
	if err := triggerer.TriggerOptimization(ctx, event); err != nil {
		return nil, err
	}
	return &event, nil
}

func resolveCanonicalSkillSourcePath(skillID string, sourcePath string) (string, string, error) {
	skillID = strings.TrimSpace(skillID)
	normalized := normalizeSkillSourcePath(sourcePath)
	if normalized == "" {
		return "", "", fmt.Errorf("skill revision source_path is required")
	}
	if strings.HasPrefix(normalized, "server/internal/skill/embedded/skills/") {
		return "", "", fmt.Errorf("embedded skill paths are not promotable: %s", normalized)
	}
	repoRoot, err := findSkillRepoRoot()
	if err != nil {
		return "", "", err
	}
	allowedRoot := filepath.Join(repoRoot, "assets", "skills")

	var absPath string
	switch {
	case filepath.IsAbs(sourcePath):
		absPath = filepath.Clean(sourcePath)
	default:
		if !strings.HasPrefix(normalized, "assets/skills/") {
			return "", "", fmt.Errorf("skill revision source_path must stay under assets/skills: %s", normalized)
		}
		absPath = filepath.Join(repoRoot, filepath.FromSlash(normalized))
	}
	absPath = filepath.Clean(absPath)
	if !pathWithinRoot(allowedRoot, absPath) {
		return "", "", fmt.Errorf("skill revision source_path must stay under %s", allowedRoot)
	}

	relPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return "", "", err
	}
	normalizedRel := normalizeSkillSourcePath(relPath)
	if skillID != "" {
		expected := normalizeSkillSourcePath(filepath.Join("assets", "skills", skillID, "SKILL.md"))
		if normalizedRel != expected {
			return "", "", fmt.Errorf("skill revision source_path must match %s", expected)
		}
	}
	if filepath.Base(absPath) != "SKILL.md" {
		return "", "", fmt.Errorf("skill revision source_path must point to SKILL.md")
	}
	return absPath, normalizedRel, nil
}

func inferSkillOptimizeFollowupGate(evalRun *EvalRun, evalSpec *EvalSpec) (string, error) {
	for _, raw := range []string{
		metadataString(evalRun.Metadata, "followup_gate"),
		metadataString(evalRun.Metadata, "gate_type"),
		metadataString(evalSpec.Metadata, "followup_gate"),
		metadataString(evalSpec.Metadata, "gate_type"),
	} {
		switch strings.TrimSpace(raw) {
		case "selector", "selection":
			return "selector", nil
		case "execution", "execution_equivalence":
			return "execution", nil
		case "budget", "skill_cutover_budget":
			return "budget", nil
		}
	}
	return "", fmt.Errorf("manual skill optimize could not infer follow-up gate")
}

func findSkillRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Clean(wd)
	for {
		info, statErr := os.Stat(filepath.Join(dir, "assets", "skills"))
		if statErr == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not locate repository root containing assets/skills")
}

func normalizeSkillSourcePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func skillRevisionSHA256(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func firstNonEmptySkillRevisionValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func querySkillRevisionRows(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([]skillRevisionRow, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]skillRevisionRow, 0)
	for rows.Next() {
		var row skillRevisionRow
		if err := rows.Scan(
			&row.ID,
			&row.SkillID,
			&row.Status,
			&row.SourcePath,
			&row.CandidateID,
			&row.ParentRevisionID,
			&row.BackupOfRevisionID,
			&row.EvalRunID,
			&row.OptimizationRunID,
			&row.Content,
			&row.ContentSHA256,
			&row.CreatedAt,
			&row.PromotedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
