package harness

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
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

type SkillRevisionDecisionAction string

const (
	SkillRevisionDecisionActionPromote  SkillRevisionDecisionAction = "promote"
	SkillRevisionDecisionActionRollback SkillRevisionDecisionAction = "rollback"
)

type SkillRevision struct {
	ID                  string                      `json:"id"`
	SkillID             string                      `json:"skill_id"`
	Status              SkillRevisionStatus         `json:"status"`
	SourcePath          string                      `json:"source_path,omitempty"`
	CandidateID         string                      `json:"candidate_id,omitempty"`
	BaseContentSHA256   string                      `json:"base_content_sha256,omitempty"`
	OriginCaseID        string                      `json:"origin_case_id,omitempty"`
	ParentRevisionID    string                      `json:"parent_revision_id,omitempty"`
	BackupOfRevisionID  string                      `json:"backup_of_revision_id,omitempty"`
	EvalRunID           string                      `json:"eval_run_id,omitempty"`
	OptimizationRunID   string                      `json:"optimization_run_id,omitempty"`
	FollowupGate        string                      `json:"followup_gate,omitempty"`
	OptimizationSurface OptimizationSurface         `json:"optimization_surface,omitempty"`
	DecisionAction      SkillRevisionDecisionAction `json:"decision_action,omitempty"`
	ReviewNote          string                      `json:"review_note,omitempty"`
	ReviewedBy          string                      `json:"reviewed_by,omitempty"`
	DecisionLogJSON     string                      `json:"decision_log_json,omitempty"`
	Content             string                      `json:"content,omitempty"`
	ContentSHA256       string                      `json:"content_sha256,omitempty"`
	CreatedAt           time.Time                   `json:"created_at"`
	ReviewedAt          *time.Time                  `json:"reviewed_at,omitempty"`
	PromotedAt          *time.Time                  `json:"promoted_at,omitempty"`
}

type SkillRevisionFilter struct {
	SkillID  string                `json:"skill_id,omitempty"`
	Statuses []SkillRevisionStatus `json:"statuses,omitempty"`
	Limit    int                   `json:"limit,omitempty"`
}

type SkillDecisionHistoryEntry struct {
	RevisionID          string                      `json:"revision_id"`
	SkillID             string                      `json:"skill_id"`
	Status              SkillRevisionStatus         `json:"status"`
	SourcePath          string                      `json:"source_path,omitempty"`
	CandidateID         string                      `json:"candidate_id,omitempty"`
	BaseContentSHA256   string                      `json:"base_content_sha256,omitempty"`
	OriginCaseID        string                      `json:"origin_case_id,omitempty"`
	ParentRevisionID    string                      `json:"parent_revision_id,omitempty"`
	BackupOfRevisionID  string                      `json:"backup_of_revision_id,omitempty"`
	EvalRunID           string                      `json:"eval_run_id,omitempty"`
	OptimizationRunID   string                      `json:"optimization_run_id,omitempty"`
	FollowupGate        string                      `json:"followup_gate,omitempty"`
	OptimizationSurface OptimizationSurface         `json:"optimization_surface,omitempty"`
	DecisionAction      SkillRevisionDecisionAction `json:"decision_action,omitempty"`
	ReviewNote          string                      `json:"review_note,omitempty"`
	ReviewedBy          string                      `json:"reviewed_by,omitempty"`
	DecisionLog         map[string]interface{}      `json:"decision_log,omitempty"`
	DecisionLogJSON     string                      `json:"decision_log_json,omitempty"`
	DecisionAt          time.Time                   `json:"decision_at"`
	CreatedAt           time.Time                   `json:"created_at"`
	ReviewedAt          *time.Time                  `json:"reviewed_at,omitempty"`
	PromotedAt          *time.Time                  `json:"promoted_at,omitempty"`
}

type SkillDecisionHistoryFilter struct {
	SkillID string                        `json:"skill_id,omitempty"`
	Actions []SkillRevisionDecisionAction `json:"actions,omitempty"`
	Limit   int                           `json:"limit,omitempty"`
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

type SkillRevisionDecisionRequest struct {
	ReviewNote string `json:"review_note,omitempty"`
	ReviewedBy string `json:"-"`
}

type CanonicalSkillSourceState struct {
	AbsolutePath   string
	NormalizedPath string
	Content        string
	ContentSHA256  string
}

type SkillRevisionPromotionRecorder interface {
	RecordSkillRevisionPromotion(ctx context.Context, promotedRevision *SkillRevision, backupRevision *SkillRevision, writtenSourcePath string) error
}

type skillRevisionRow struct {
	ID                  string         `zorm:"id"`
	SkillID             string         `zorm:"skill_id"`
	Status              string         `zorm:"status"`
	SourcePath          string         `zorm:"source_path"`
	CandidateID         string         `zorm:"candidate_id"`
	BaseContentSHA256   string         `zorm:"base_content_sha256"`
	OriginCaseID        string         `zorm:"origin_case_id"`
	ParentRevisionID    string         `zorm:"parent_revision_id"`
	BackupOfRevisionID  string         `zorm:"backup_of_revision_id"`
	EvalRunID           string         `zorm:"eval_run_id"`
	OptimizationRunID   string         `zorm:"optimization_run_id"`
	FollowupGate        string         `zorm:"followup_gate"`
	OptimizationSurface string         `zorm:"optimization_surface"`
	DecisionAction      string         `zorm:"decision_action"`
	ReviewNote          string         `zorm:"review_note"`
	ReviewedBy          string         `zorm:"reviewed_by"`
	DecisionLogJSON     string         `zorm:"decision_log_json"`
	Content             string         `zorm:"content"`
	ContentSHA256       string         `zorm:"content_sha256"`
	CreatedAt           time.Time      `zorm:"created_at"`
	ReviewedAt          sql.NullString `zorm:"reviewed_at"`
	PromotedAt          sql.NullString `zorm:"promoted_at"`
}

func skillRevisionFromRow(row skillRevisionRow) SkillRevision {
	revision := SkillRevision{
		ID:                  row.ID,
		SkillID:             row.SkillID,
		Status:              SkillRevisionStatus(row.Status),
		SourcePath:          row.SourcePath,
		CandidateID:         row.CandidateID,
		BaseContentSHA256:   row.BaseContentSHA256,
		OriginCaseID:        row.OriginCaseID,
		ParentRevisionID:    row.ParentRevisionID,
		BackupOfRevisionID:  row.BackupOfRevisionID,
		EvalRunID:           row.EvalRunID,
		OptimizationRunID:   row.OptimizationRunID,
		FollowupGate:        row.FollowupGate,
		OptimizationSurface: OptimizationSurface(row.OptimizationSurface),
		DecisionAction:      SkillRevisionDecisionAction(row.DecisionAction),
		ReviewNote:          row.ReviewNote,
		ReviewedBy:          row.ReviewedBy,
		DecisionLogJSON:     row.DecisionLogJSON,
		Content:             row.Content,
		ContentSHA256:       row.ContentSHA256,
		CreatedAt:           row.CreatedAt,
	}
	if row.ReviewedAt.Valid {
		ts, ok := parseSkillRevisionTime(row.ReviewedAt.String)
		if ok {
			revision.ReviewedAt = &ts
		}
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
		id, skill_id, status, source_path, candidate_id, base_content_sha256, origin_case_id, parent_revision_id, backup_of_revision_id,
		eval_run_id, optimization_run_id, followup_gate, optimization_surface, decision_action, review_note, reviewed_by, decision_log_json,
		content, content_sha256, created_at, reviewed_at, promoted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		revision.ID, revision.SkillID, string(revision.Status), revision.SourcePath, revision.CandidateID,
		revision.BaseContentSHA256, revision.OriginCaseID, revision.ParentRevisionID, revision.BackupOfRevisionID, revision.EvalRunID, revision.OptimizationRunID,
		revision.FollowupGate, string(revision.OptimizationSurface), string(revision.DecisionAction), revision.ReviewNote, revision.ReviewedBy,
		revision.DecisionLogJSON, revision.Content, revision.ContentSHA256, revision.CreatedAt, nullableSkillRevisionTime(revision.ReviewedAt),
		nullableSkillRevisionTime(revision.PromotedAt),
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
		skill_id=?, status=?, source_path=?, candidate_id=?, base_content_sha256=?, origin_case_id=?, parent_revision_id=?, backup_of_revision_id=?,
		eval_run_id=?, optimization_run_id=?, followup_gate=?, optimization_surface=?, decision_action=?, review_note=?, reviewed_by=?, decision_log_json=?,
		content=?, content_sha256=?, created_at=?, reviewed_at=?, promoted_at=?
		WHERE id=?`,
		revision.SkillID, string(revision.Status), revision.SourcePath, revision.CandidateID, revision.BaseContentSHA256, revision.OriginCaseID,
		revision.ParentRevisionID, revision.BackupOfRevisionID, revision.EvalRunID, revision.OptimizationRunID, revision.FollowupGate, string(revision.OptimizationSurface),
		string(revision.DecisionAction), revision.ReviewNote, revision.ReviewedBy, revision.DecisionLogJSON,
		revision.Content, revision.ContentSHA256, revision.CreatedAt, nullableSkillRevisionTime(revision.ReviewedAt), nullableSkillRevisionTime(revision.PromotedAt),
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
		id, skill_id, status, source_path, candidate_id, base_content_sha256, origin_case_id, parent_revision_id, backup_of_revision_id,
		eval_run_id, optimization_run_id, followup_gate, optimization_surface, decision_action, review_note, reviewed_by, decision_log_json,
		content, content_sha256, created_at, reviewed_at, promoted_at
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

func (s *SQLiteStore) ListSkillDecisionHistory(ctx context.Context, filter SkillDecisionHistoryFilter) ([]SkillDecisionHistoryEntry, error) {
	rows, err := s.listSkillDecisionHistoryRows(ctx, s.reader(), filter)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.listSkillDecisionHistoryRows(ctx, s.db, filter)
		if err != nil {
			return nil, err
		}
	} else if s.hasSeparateReader() {
		writerRows, writerErr := s.listSkillDecisionHistoryRows(ctx, s.db, filter)
		if writerErr == nil {
			rows = mergeUniqueRowsByKey(rows, writerRows, filter.Limit, func(row skillRevisionRow) string { return row.ID })
		}
	}
	out := make([]SkillDecisionHistoryEntry, 0, len(rows))
	for i := range rows {
		entry, buildErr := skillDecisionHistoryEntryFromRevision(skillRevisionFromRow(rows[i]))
		if buildErr != nil {
			return nil, buildErr
		}
		out = append(out, entry)
	}
	return out, nil
}

func (c *Controller) ListSkillDecisionHistory(ctx context.Context, filter SkillDecisionHistoryFilter) ([]SkillDecisionHistoryEntry, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.ListSkillDecisionHistory(ctx, filter)
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
			id, skill_id, status, source_path, candidate_id, base_content_sha256, origin_case_id, parent_revision_id, backup_of_revision_id,
			eval_run_id, optimization_run_id, followup_gate, optimization_surface, decision_action, review_note, reviewed_by, decision_log_json,
			content, content_sha256, created_at, reviewed_at, promoted_at
			FROM harness_skill_revisions
			WHERE `+strings.Join(conds, " AND ")+`
			ORDER BY created_at DESC, rowid DESC
			LIMIT ?`, args...)
	}
	return querySkillRevisionRows(ctx, db, `SELECT
		id, skill_id, status, source_path, candidate_id, base_content_sha256, origin_case_id, parent_revision_id, backup_of_revision_id,
		eval_run_id, optimization_run_id, followup_gate, optimization_surface, decision_action, review_note, reviewed_by, decision_log_json,
		content, content_sha256, created_at, reviewed_at, promoted_at
		FROM harness_skill_revisions
		ORDER BY created_at DESC, rowid DESC
		LIMIT ?`, limit)
}

func (s *SQLiteStore) listSkillDecisionHistoryRows(ctx context.Context, db *sql.DB, filter SkillDecisionHistoryFilter) ([]skillRevisionRow, error) {
	var (
		conds = []string{"(decision_action <> '' OR review_note <> '' OR reviewed_by <> '' OR reviewed_at IS NOT NULL OR decision_log_json <> '')"}
		args  []interface{}
	)
	if v := strings.TrimSpace(filter.SkillID); v != "" {
		conds = append(conds, "skill_id = ?")
		args = append(args, v)
	}
	if len(filter.Actions) > 0 {
		actions := make([]string, 0, len(filter.Actions))
		for _, action := range filter.Actions {
			if action == "" {
				continue
			}
			actions = append(actions, string(action))
		}
		if len(actions) > 0 {
			placeholders := make([]string, 0, len(actions))
			for _, action := range actions {
				placeholders = append(placeholders, "?")
				args = append(args, action)
			}
			conds = append(conds, "decision_action IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit)
	return querySkillRevisionRows(ctx, db, `SELECT
		id, skill_id, status, source_path, candidate_id, base_content_sha256, origin_case_id, parent_revision_id, backup_of_revision_id,
		eval_run_id, optimization_run_id, followup_gate, optimization_surface, decision_action, review_note, reviewed_by, decision_log_json,
		content, content_sha256, created_at, reviewed_at, promoted_at
		FROM harness_skill_revisions
		WHERE `+strings.Join(conds, " AND ")+`
		ORDER BY COALESCE(reviewed_at, promoted_at, created_at) DESC, rowid DESC
		LIMIT ?`, args...)
}

func skillDecisionHistoryEntryFromRevision(revision SkillRevision) (SkillDecisionHistoryEntry, error) {
	var decisionLog map[string]interface{}
	if raw := strings.TrimSpace(revision.DecisionLogJSON); raw != "" {
		if err := json.Unmarshal([]byte(raw), &decisionLog); err != nil {
			return SkillDecisionHistoryEntry{}, fmt.Errorf("unmarshal skill decision history decision_log_json for revision %s: %w", revision.ID, err)
		}
	}
	return SkillDecisionHistoryEntry{
		RevisionID:          revision.ID,
		SkillID:             revision.SkillID,
		Status:              revision.Status,
		SourcePath:          revision.SourcePath,
		CandidateID:         revision.CandidateID,
		BaseContentSHA256:   revision.BaseContentSHA256,
		OriginCaseID:        revision.OriginCaseID,
		ParentRevisionID:    revision.ParentRevisionID,
		BackupOfRevisionID:  revision.BackupOfRevisionID,
		EvalRunID:           revision.EvalRunID,
		OptimizationRunID:   revision.OptimizationRunID,
		FollowupGate:        revision.FollowupGate,
		OptimizationSurface: revision.OptimizationSurface,
		DecisionAction:      revision.DecisionAction,
		ReviewNote:          revision.ReviewNote,
		ReviewedBy:          revision.ReviewedBy,
		DecisionLog:         decisionLog,
		DecisionLogJSON:     revision.DecisionLogJSON,
		DecisionAt:          skillRevisionDecisionTimestamp(revision),
		CreatedAt:           revision.CreatedAt,
		ReviewedAt:          revision.ReviewedAt,
		PromotedAt:          revision.PromotedAt,
	}, nil
}

func skillRevisionDecisionTimestamp(revision SkillRevision) time.Time {
	if revision.ReviewedAt != nil && !revision.ReviewedAt.IsZero() {
		return revision.ReviewedAt.UTC()
	}
	if revision.PromotedAt != nil && !revision.PromotedAt.IsZero() {
		return revision.PromotedAt.UTC()
	}
	return revision.CreatedAt.UTC()
}

func normalizeSkillRevisionForWrite(revision *SkillRevision) error {
	if revision == nil {
		return fmt.Errorf("skill revision is required")
	}
	revision.ID = strings.TrimSpace(revision.ID)
	revision.SkillID = strings.TrimSpace(revision.SkillID)
	revision.SourcePath = normalizeSkillSourcePath(revision.SourcePath)
	revision.CandidateID = strings.TrimSpace(revision.CandidateID)
	revision.BaseContentSHA256 = strings.TrimSpace(revision.BaseContentSHA256)
	revision.OriginCaseID = strings.TrimSpace(revision.OriginCaseID)
	revision.ParentRevisionID = strings.TrimSpace(revision.ParentRevisionID)
	revision.BackupOfRevisionID = strings.TrimSpace(revision.BackupOfRevisionID)
	revision.EvalRunID = strings.TrimSpace(revision.EvalRunID)
	revision.OptimizationRunID = strings.TrimSpace(revision.OptimizationRunID)
	revision.FollowupGate = strings.TrimSpace(revision.FollowupGate)
	revision.OptimizationSurface = OptimizationSurface(strings.TrimSpace(string(revision.OptimizationSurface)))
	revision.DecisionAction = SkillRevisionDecisionAction(strings.TrimSpace(string(revision.DecisionAction)))
	revision.ReviewNote = strings.TrimSpace(revision.ReviewNote)
	revision.ReviewedBy = strings.TrimSpace(revision.ReviewedBy)
	decisionLogJSON, err := normalizeSkillRevisionDecisionLogJSON(revision.DecisionLogJSON)
	if err != nil {
		return err
	}
	revision.DecisionLogJSON = decisionLogJSON
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

func normalizeSkillRevisionDecisionLogJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return "", fmt.Errorf("skill revision decision_log_json must be valid JSON: %w", err)
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return "", fmt.Errorf("marshal skill revision decision_log_json: %w", err)
	}
	return string(normalized), nil
}

func nullableSkillRevisionTime(ts *time.Time) interface{} {
	if ts == nil || ts.IsZero() {
		return nil
	}
	return *ts
}

func normalizeSkillRevisionDecisionRequest(req SkillRevisionDecisionRequest) SkillRevisionDecisionRequest {
	req.ReviewNote = strings.TrimSpace(req.ReviewNote)
	req.ReviewedBy = strings.TrimSpace(req.ReviewedBy)
	return req
}

func applySkillRevisionDecision(
	revision *SkillRevision,
	action SkillRevisionDecisionAction,
	decision SkillRevisionDecisionRequest,
	reviewedAt time.Time,
	decisionLogJSON string,
) {
	if revision == nil {
		return
	}
	revision.DecisionAction = action
	revision.ReviewNote = strings.TrimSpace(decision.ReviewNote)
	revision.ReviewedBy = strings.TrimSpace(decision.ReviewedBy)
	revision.ReviewedAt = &reviewedAt
	revision.DecisionLogJSON = decisionLogJSON
}

func skillRevisionDecisionLogJSON(
	action SkillRevisionDecisionAction,
	decision SkillRevisionDecisionRequest,
	reviewedAt time.Time,
	fields map[string]string,
) string {
	payload := make(map[string]string, len(fields)+4)
	payload["action"] = strings.TrimSpace(string(action))
	payload["reviewed_at"] = reviewedAt.UTC().Format(time.RFC3339Nano)
	if note := strings.TrimSpace(decision.ReviewNote); note != "" {
		payload["review_note"] = note
	}
	if reviewedBy := strings.TrimSpace(decision.ReviewedBy); reviewedBy != "" {
		payload["reviewed_by"] = reviewedBy
	}
	for key, value := range fields {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		payload[key] = value
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (c *Controller) PromoteSkillRevision(ctx context.Context, revisionID string, decision SkillRevisionDecisionRequest) (*SkillPromoteResult, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	decision = normalizeSkillRevisionDecisionRequest(decision)
	revision, err := c.store.GetSkillRevision(ctx, strings.TrimSpace(revisionID))
	if err != nil {
		return nil, err
	}
	if revision.Status != SkillRevisionStatusAccepted {
		return nil, fmt.Errorf("skill revision must be accepted before promote")
	}
	sourceState, err := ResolveCanonicalSkillSourceState(revision.SkillID, revision.SourcePath)
	if err != nil {
		return nil, err
	}
	if revision.BaseContentSHA256 != "" && revision.BaseContentSHA256 != sourceState.ContentSHA256 {
		return nil, fmt.Errorf("skill revision is stale: canonical skill content changed since candidate creation")
	}
	now := timeutil.NowTime()
	backup := &SkillRevision{
		ID:                  uuid.NewString(),
		SkillID:             revision.SkillID,
		Status:              SkillRevisionStatusBackup,
		SourcePath:          sourceState.NormalizedPath,
		CandidateID:         revision.CandidateID,
		BaseContentSHA256:   sourceState.ContentSHA256,
		OriginCaseID:        revision.OriginCaseID,
		BackupOfRevisionID:  revision.ID,
		EvalRunID:           revision.EvalRunID,
		OptimizationRunID:   revision.OptimizationRunID,
		FollowupGate:        revision.FollowupGate,
		OptimizationSurface: revision.OptimizationSurface,
		Content:             sourceState.Content,
		CreatedAt:           now,
	}
	if err := c.store.CreateSkillRevision(ctx, backup); err != nil {
		return nil, err
	}
	if err := os.WriteFile(sourceState.AbsolutePath, []byte(revision.Content), 0o644); err != nil {
		return nil, err
	}
	revision.Status = SkillRevisionStatusPromoted
	revision.SourcePath = sourceState.NormalizedPath
	applySkillRevisionDecision(revision, SkillRevisionDecisionActionPromote, decision, now, skillRevisionDecisionLogJSON(
		SkillRevisionDecisionActionPromote,
		decision,
		now,
		map[string]string{
			"selected_revision_id": revision.ID,
			"target_revision_id":   revision.ID,
			"backup_revision_id":   backup.ID,
			"written_source_path":  sourceState.AbsolutePath,
			"source_path":          sourceState.NormalizedPath,
			"base_content_sha256":  sourceState.ContentSHA256,
			"candidate_id":         revision.CandidateID,
			"origin_case_id":       revision.OriginCaseID,
			"eval_run_id":          revision.EvalRunID,
			"optimization_surface": strings.TrimSpace(string(revision.OptimizationSurface)),
		},
	))
	revision.PromotedAt = &now
	if err := c.store.UpdateSkillRevision(ctx, revision); err != nil {
		return nil, err
	}
	if strings.TrimSpace(revision.OriginCaseID) != "" {
		evolutionCase, getErr := c.store.GetSkillEvolutionCase(ctx, revision.OriginCaseID)
		if getErr != nil {
			if !errors.Is(getErr, sql.ErrNoRows) {
				return nil, getErr
			}
		} else {
			evolutionCase.Status = SkillEvolutionCaseStatusPromoted
			evolutionCase.RevisionID = revision.ID
			evolutionCase.UpdatedAt = now
			if _, updateErr := c.UpdateSkillEvolutionCase(ctx, *evolutionCase); updateErr != nil {
				return nil, updateErr
			}
		}
	}
	c.mu.RLock()
	triggerer := c.optimization
	c.mu.RUnlock()
	if recorder, ok := triggerer.(SkillRevisionPromotionRecorder); ok {
		if err := recorder.RecordSkillRevisionPromotion(ctx, revision, backup, sourceState.AbsolutePath); err != nil {
			return nil, err
		}
	}
	return &SkillPromoteResult{
		PromotedRevisionID: revision.ID,
		BackupRevisionID:   backup.ID,
		WrittenSourcePath:  sourceState.AbsolutePath,
	}, nil
}

func (c *Controller) RollbackSkillRevision(ctx context.Context, revisionID string, decision SkillRevisionDecisionRequest) (*SkillPromoteResult, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	decision = normalizeSkillRevisionDecisionRequest(decision)
	backupRevision, err := c.store.GetSkillRevision(ctx, strings.TrimSpace(revisionID))
	if err != nil {
		return nil, err
	}
	if backupRevision.Status != SkillRevisionStatusBackup {
		return nil, fmt.Errorf("skill revision must be a backup before rollback")
	}
	if strings.TrimSpace(backupRevision.BackupOfRevisionID) == "" {
		return nil, fmt.Errorf("skill backup revision does not reference the promoted revision it can restore")
	}
	currentCanonicalRevision, err := c.store.GetSkillRevision(ctx, backupRevision.BackupOfRevisionID)
	if err != nil {
		return nil, err
	}
	sourceState, err := ResolveCanonicalSkillSourceState(backupRevision.SkillID, backupRevision.SourcePath)
	if err != nil {
		return nil, err
	}
	if currentCanonicalRevision.ContentSHA256 != "" && sourceState.ContentSHA256 != currentCanonicalRevision.ContentSHA256 {
		return nil, fmt.Errorf("skill rollback is stale: canonical skill content changed since this backup was created")
	}
	now := timeutil.NowTime()
	newBackupID := uuid.NewString()
	rollbackRevision := &SkillRevision{
		ID:                  uuid.NewString(),
		SkillID:             backupRevision.SkillID,
		Status:              SkillRevisionStatusPromoted,
		SourcePath:          sourceState.NormalizedPath,
		CandidateID:         backupRevision.CandidateID,
		BaseContentSHA256:   sourceState.ContentSHA256,
		OriginCaseID:        backupRevision.OriginCaseID,
		ParentRevisionID:    backupRevision.ID,
		EvalRunID:           backupRevision.EvalRunID,
		OptimizationRunID:   backupRevision.OptimizationRunID,
		FollowupGate:        backupRevision.FollowupGate,
		OptimizationSurface: backupRevision.OptimizationSurface,
		Content:             backupRevision.Content,
		CreatedAt:           now,
		PromotedAt:          &now,
	}
	applySkillRevisionDecision(rollbackRevision, SkillRevisionDecisionActionRollback, decision, now, skillRevisionDecisionLogJSON(
		SkillRevisionDecisionActionRollback,
		decision,
		now,
		map[string]string{
			"selected_revision_id":     backupRevision.ID,
			"source_revision_id":       backupRevision.ID,
			"target_revision_id":       rollbackRevision.ID,
			"current_live_revision_id": currentCanonicalRevision.ID,
			"backup_revision_id":       newBackupID,
			"written_source_path":      sourceState.AbsolutePath,
			"source_path":              sourceState.NormalizedPath,
			"base_content_sha256":      sourceState.ContentSHA256,
			"candidate_id":             rollbackRevision.CandidateID,
			"origin_case_id":           rollbackRevision.OriginCaseID,
			"eval_run_id":              rollbackRevision.EvalRunID,
			"optimization_surface":     strings.TrimSpace(string(rollbackRevision.OptimizationSurface)),
		},
	))
	newBackup := &SkillRevision{
		ID:                  newBackupID,
		SkillID:             backupRevision.SkillID,
		Status:              SkillRevisionStatusBackup,
		SourcePath:          sourceState.NormalizedPath,
		CandidateID:         rollbackRevision.CandidateID,
		BaseContentSHA256:   sourceState.ContentSHA256,
		OriginCaseID:        rollbackRevision.OriginCaseID,
		BackupOfRevisionID:  rollbackRevision.ID,
		EvalRunID:           rollbackRevision.EvalRunID,
		OptimizationRunID:   rollbackRevision.OptimizationRunID,
		FollowupGate:        rollbackRevision.FollowupGate,
		OptimizationSurface: rollbackRevision.OptimizationSurface,
		Content:             sourceState.Content,
		CreatedAt:           now,
	}
	if err := c.store.CreateSkillRevision(ctx, newBackup); err != nil {
		return nil, err
	}
	if err := os.WriteFile(sourceState.AbsolutePath, []byte(rollbackRevision.Content), 0o644); err != nil {
		return nil, err
	}
	if err := c.store.CreateSkillRevision(ctx, rollbackRevision); err != nil {
		return nil, err
	}
	c.mu.RLock()
	triggerer := c.optimization
	c.mu.RUnlock()
	if recorder, ok := triggerer.(SkillRevisionPromotionRecorder); ok {
		if err := recorder.RecordSkillRevisionPromotion(ctx, rollbackRevision, newBackup, sourceState.AbsolutePath); err != nil {
			return nil, err
		}
	}
	return &SkillPromoteResult{
		PromotedRevisionID: rollbackRevision.ID,
		BackupRevisionID:   newBackup.ID,
		WrittenSourcePath:  sourceState.AbsolutePath,
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
	sourceState, err := ResolveCanonicalSkillSourceState(skillID, sourcePath)
	if err != nil {
		return nil, err
	}
	sourcePath = sourceState.NormalizedPath
	if existingSkillID := strings.TrimSpace(metadataString(sourceCandidate, "skill_id")); existingSkillID != "" && existingSkillID != skillID {
		return nil, fmt.Errorf("skill candidate skill_id %q does not match requested skill %q", existingSkillID, skillID)
	}

	candidateID := firstNonEmptySkillRevisionValue(
		strings.TrimSpace(req.CandidateID),
		evalRunCandidateID(evalRun),
		metadataString(sourceCandidate, "candidate_id"),
	)
	if candidateID == "" {
		candidateID = fmt.Sprintf("%s-%s", skillID, evalRun.ID)
	}
	candidateContent := ""
	if rawContent, ok := sourceCandidate["content"].(string); ok && strings.TrimSpace(rawContent) != "" {
		candidateContent = rawContent
	}
	if candidateContent == "" {
		candidateContent = sourceState.Content
	}
	revision, err := c.CreateSkillRevision(ctx, SkillRevision{
		ID:                  uuid.NewString(),
		SkillID:             skillID,
		Status:              SkillRevisionStatusCandidate,
		SourcePath:          sourcePath,
		CandidateID:         candidateID,
		BaseContentSHA256:   sourceState.ContentSHA256,
		EvalRunID:           evalRun.ID,
		FollowupGate:        followupGate,
		OptimizationSurface: OptimizationSurfaceSkillDefinition,
		Content:             candidateContent,
		CreatedAt:           timeutil.NowTime(),
	})
	if err != nil {
		return nil, err
	}
	metadata := evalRunOptimizationMetadata(evalRun)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["followup_gate"] = followupGate
	metadata["optimization_surface"] = string(OptimizationSurfaceSkillDefinition)
	metadata["candidate_revision_id"] = revision.ID
	if candidateID != "" {
		metadata["candidate_id"] = candidateID
	}
	skillCandidate := cloneMetadataMap(sourceCandidate)
	if skillCandidate == nil {
		skillCandidate = map[string]interface{}{}
	}
	skillCandidate["skill_id"] = skillID
	skillCandidate["source_path"] = sourcePath
	skillCandidate["content"] = revision.Content
	skillCandidate["revision_id"] = revision.ID
	skillCandidate["base_content_sha256"] = sourceState.ContentSHA256
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

func ResolveCanonicalSkillSourceState(skillID string, sourcePath string) (*CanonicalSkillSourceState, error) {
	absPath, normalizedPath, err := resolveCanonicalSkillSourcePath(skillID, sourcePath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	return &CanonicalSkillSourceState{
		AbsolutePath:   absPath,
		NormalizedPath: normalizedPath,
		Content:        string(content),
		ContentSHA256:  skillRevisionSHA256(string(content)),
	}, nil
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
			&row.BaseContentSHA256,
			&row.OriginCaseID,
			&row.ParentRevisionID,
			&row.BackupOfRevisionID,
			&row.EvalRunID,
			&row.OptimizationRunID,
			&row.FollowupGate,
			&row.OptimizationSurface,
			&row.DecisionAction,
			&row.ReviewNote,
			&row.ReviewedBy,
			&row.DecisionLogJSON,
			&row.Content,
			&row.ContentSHA256,
			&row.CreatedAt,
			&row.ReviewedAt,
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
