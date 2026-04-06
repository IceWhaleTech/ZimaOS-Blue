package harness

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type SkillEvolutionMode string

const (
	SkillEvolutionModeFix     SkillEvolutionMode = "fix"
	SkillEvolutionModeCapture SkillEvolutionMode = "capture"
)

type SkillEvolutionReason string

const (
	SkillEvolutionReasonRuntimeFailure      SkillEvolutionReason = "runtime_failure"
	SkillEvolutionReasonRuntimeCapture      SkillEvolutionReason = "runtime_capture"
	SkillEvolutionReasonSelectorGateFailed  SkillEvolutionReason = "selector_gate_failed"
	SkillEvolutionReasonExecutionGateFailed SkillEvolutionReason = "execution_gate_failed"
	SkillEvolutionReasonBudgetGateFailed    SkillEvolutionReason = "budget_gate_failed"
	SkillEvolutionReasonManual              SkillEvolutionReason = "manual"
)

type SkillEvolutionCaseStatus string

const (
	SkillEvolutionCaseStatusOpen             SkillEvolutionCaseStatus = "open"
	SkillEvolutionCaseStatusCandidateCreated SkillEvolutionCaseStatus = "candidate_created"
	SkillEvolutionCaseStatusAccepted         SkillEvolutionCaseStatus = "accepted"
	SkillEvolutionCaseStatusRejected         SkillEvolutionCaseStatus = "rejected"
	SkillEvolutionCaseStatusPromoted         SkillEvolutionCaseStatus = "promoted"
	SkillEvolutionCaseStatusSkipped          SkillEvolutionCaseStatus = "skipped"
)

type SkillEvolutionCase struct {
	ID                string                   `json:"id"`
	SkillID           string                   `json:"skill_id"`
	OwnerUserID       string                   `json:"owner_user_id,omitempty"`
	Mode              SkillEvolutionMode       `json:"mode"`
	Reason            SkillEvolutionReason     `json:"reason"`
	SourceKind        string                   `json:"source_kind,omitempty"`
	SourceID          string                   `json:"source_id,omitempty"`
	CandidateID       string                   `json:"candidate_id,omitempty"`
	BaseContentSHA256 string                   `json:"base_content_sha256,omitempty"`
	FailureSignature  string                   `json:"failure_signature,omitempty"`
	DedupKey          string                   `json:"dedup_key,omitempty"`
	Summary           string                   `json:"summary,omitempty"`
	EvidenceJSON      string                   `json:"evidence_json,omitempty"`
	RevisionID        string                   `json:"revision_id,omitempty"`
	Status            SkillEvolutionCaseStatus `json:"status"`
	SkippedReason     string                   `json:"skipped_reason,omitempty"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

type SkillEvolutionCaseDetail struct {
	SkillEvolutionCase
	LinkedRevision  *SkillRevision `json:"linked_revision,omitempty"`
	SourceRunID     string         `json:"source_run_id,omitempty"`
	SourceRun       *Run           `json:"source_run,omitempty"`
	SourceEvalRunID string         `json:"source_eval_run_id,omitempty"`
	SourceEvalRun   *EvalRun       `json:"source_eval_run,omitempty"`
	LinkedEvalRunID string         `json:"linked_eval_run_id,omitempty"`
	LinkedEvalRun   *EvalRun       `json:"linked_eval_run,omitempty"`
}

type SkillEvolutionCaseFilter struct {
	SkillID     string                     `json:"skill_id,omitempty"`
	OwnerUserID string                     `json:"owner_user_id,omitempty"`
	CandidateID string                     `json:"candidate_id,omitempty"`
	RevisionID  string                     `json:"revision_id,omitempty"`
	Statuses    []SkillEvolutionCaseStatus `json:"statuses,omitempty"`
	Limit       int                        `json:"limit,omitempty"`
}

type SkillEvolutionCaseSpec struct {
	ID                string               `json:"id,omitempty"`
	SkillID           string               `json:"skill_id"`
	OwnerUserID       string               `json:"owner_user_id,omitempty"`
	Mode              SkillEvolutionMode   `json:"mode"`
	Reason            SkillEvolutionReason `json:"reason"`
	SourceKind        string               `json:"source_kind,omitempty"`
	SourceID          string               `json:"source_id,omitempty"`
	CandidateID       string               `json:"candidate_id,omitempty"`
	BaseContentSHA256 string               `json:"base_content_sha256,omitempty"`
	FailureSignature  string               `json:"failure_signature,omitempty"`
	Summary           string               `json:"summary,omitempty"`
	EvidenceJSON      string               `json:"evidence_json,omitempty"`
}

type skillEvolutionCaseRow struct {
	ID                string    `zorm:"id"`
	SkillID           string    `zorm:"skill_id"`
	OwnerUserID       string    `zorm:"owner_user_id"`
	Mode              string    `zorm:"mode"`
	Reason            string    `zorm:"reason"`
	SourceKind        string    `zorm:"source_kind"`
	SourceID          string    `zorm:"source_id"`
	CandidateID       string    `zorm:"candidate_id"`
	BaseContentSHA256 string    `zorm:"base_content_sha256"`
	FailureSignature  string    `zorm:"failure_signature"`
	DedupKey          string    `zorm:"dedup_key"`
	Summary           string    `zorm:"summary"`
	EvidenceJSON      string    `zorm:"evidence_json"`
	RevisionID        string    `zorm:"revision_id"`
	Status            string    `zorm:"status"`
	SkippedReason     string    `zorm:"skipped_reason"`
	CreatedAt         time.Time `zorm:"created_at"`
	UpdatedAt         time.Time `zorm:"updated_at"`
}

func skillEvolutionCaseFromRow(row skillEvolutionCaseRow) SkillEvolutionCase {
	return SkillEvolutionCase{
		ID:                row.ID,
		SkillID:           row.SkillID,
		OwnerUserID:       row.OwnerUserID,
		Mode:              SkillEvolutionMode(row.Mode),
		Reason:            SkillEvolutionReason(row.Reason),
		SourceKind:        row.SourceKind,
		SourceID:          row.SourceID,
		CandidateID:       row.CandidateID,
		BaseContentSHA256: row.BaseContentSHA256,
		FailureSignature:  row.FailureSignature,
		DedupKey:          row.DedupKey,
		Summary:           row.Summary,
		EvidenceJSON:      row.EvidenceJSON,
		RevisionID:        row.RevisionID,
		Status:            SkillEvolutionCaseStatus(row.Status),
		SkippedReason:     row.SkippedReason,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func (s *SQLiteStore) CreateSkillEvolutionCase(ctx context.Context, evolutionCase *SkillEvolutionCase) error {
	if err := normalizeSkillEvolutionCaseForWrite(evolutionCase); err != nil {
		return err
	}
	_, err := s.execContext(ctx, `INSERT INTO harness_skill_evolution_cases (
		id, skill_id, owner_user_id, mode, reason, source_kind, source_id, candidate_id,
		base_content_sha256, failure_signature, dedup_key, summary, evidence_json, revision_id,
		status, skipped_reason, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		evolutionCase.ID, evolutionCase.SkillID, evolutionCase.OwnerUserID, string(evolutionCase.Mode),
		string(evolutionCase.Reason), evolutionCase.SourceKind, evolutionCase.SourceID, evolutionCase.CandidateID,
		evolutionCase.BaseContentSHA256, evolutionCase.FailureSignature, evolutionCase.DedupKey, evolutionCase.Summary,
		evolutionCase.EvidenceJSON, evolutionCase.RevisionID, string(evolutionCase.Status), evolutionCase.SkippedReason,
		evolutionCase.CreatedAt, evolutionCase.UpdatedAt,
	)
	return err
}

func (c *Controller) CreateSkillEvolutionCase(ctx context.Context, evolutionCase SkillEvolutionCase) (*SkillEvolutionCase, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(evolutionCase.ID) == "" {
		evolutionCase.ID = uuid.NewString()
	}
	if err := c.store.CreateSkillEvolutionCase(ctx, &evolutionCase); err != nil {
		return nil, err
	}
	return c.store.GetSkillEvolutionCase(ctx, evolutionCase.ID)
}

func (s *SQLiteStore) UpdateSkillEvolutionCase(ctx context.Context, evolutionCase *SkillEvolutionCase) error {
	if err := normalizeSkillEvolutionCaseForWrite(evolutionCase); err != nil {
		return err
	}
	_, err := s.execContext(ctx, `UPDATE harness_skill_evolution_cases SET
		skill_id=?, owner_user_id=?, mode=?, reason=?, source_kind=?, source_id=?, candidate_id=?,
		base_content_sha256=?, failure_signature=?, dedup_key=?, summary=?, evidence_json=?, revision_id=?,
		status=?, skipped_reason=?, created_at=?, updated_at=?
		WHERE id=?`,
		evolutionCase.SkillID, evolutionCase.OwnerUserID, string(evolutionCase.Mode), string(evolutionCase.Reason),
		evolutionCase.SourceKind, evolutionCase.SourceID, evolutionCase.CandidateID, evolutionCase.BaseContentSHA256,
		evolutionCase.FailureSignature, evolutionCase.DedupKey, evolutionCase.Summary, evolutionCase.EvidenceJSON,
		evolutionCase.RevisionID, string(evolutionCase.Status), evolutionCase.SkippedReason, evolutionCase.CreatedAt,
		evolutionCase.UpdatedAt, evolutionCase.ID,
	)
	return err
}

func (c *Controller) UpdateSkillEvolutionCase(ctx context.Context, evolutionCase SkillEvolutionCase) (*SkillEvolutionCase, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if err := c.store.UpdateSkillEvolutionCase(ctx, &evolutionCase); err != nil {
		return nil, err
	}
	return c.store.GetSkillEvolutionCase(ctx, evolutionCase.ID)
}

func (s *SQLiteStore) GetSkillEvolutionCase(ctx context.Context, id string) (*SkillEvolutionCase, error) {
	rows, err := s.selectSkillEvolutionCaseRows(ctx, s.reader(), strings.TrimSpace(id))
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.selectSkillEvolutionCaseRows(ctx, s.db, strings.TrimSpace(id))
		if err != nil {
			return nil, err
		}
	} else if len(rows) == 0 && s.hasSeparateReader() {
		rows, err = s.selectSkillEvolutionCaseRows(ctx, s.db, strings.TrimSpace(id))
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	evolutionCase := skillEvolutionCaseFromRow(rows[0])
	return &evolutionCase, nil
}

func (c *Controller) GetSkillEvolutionCase(ctx context.Context, id string) (*SkillEvolutionCase, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.GetSkillEvolutionCase(ctx, strings.TrimSpace(id))
}

func (c *Controller) GetSkillEvolutionCaseDetail(ctx context.Context, id string) (*SkillEvolutionCaseDetail, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	evolutionCase, err := c.GetSkillEvolutionCase(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.BuildSkillEvolutionCaseDetail(ctx, evolutionCase)
}

func (s *SQLiteStore) selectSkillEvolutionCaseRows(ctx context.Context, db *sql.DB, id string) ([]skillEvolutionCaseRow, error) {
	return querySkillEvolutionCaseRows(ctx, db, `SELECT
		id, skill_id, owner_user_id, mode, reason, source_kind, source_id, candidate_id,
		base_content_sha256, failure_signature, dedup_key, summary, evidence_json, revision_id,
		status, skipped_reason, created_at, updated_at
		FROM harness_skill_evolution_cases
		WHERE id = ?
		LIMIT 1`, strings.TrimSpace(id))
}

func (s *SQLiteStore) ListSkillEvolutionCases(ctx context.Context, filter SkillEvolutionCaseFilter) ([]SkillEvolutionCase, error) {
	rows, err := s.listSkillEvolutionCaseRows(ctx, s.reader(), filter)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = s.listSkillEvolutionCaseRows(ctx, s.db, filter)
		if err != nil {
			return nil, err
		}
	} else if s.hasSeparateReader() {
		writerRows, writerErr := s.listSkillEvolutionCaseRows(ctx, s.db, filter)
		if writerErr == nil {
			rows = mergeUniqueRowsByKey(rows, writerRows, filter.Limit, func(row skillEvolutionCaseRow) string { return row.ID })
		}
	}
	out := make([]SkillEvolutionCase, 0, len(rows))
	for i := range rows {
		out = append(out, skillEvolutionCaseFromRow(rows[i]))
	}
	return out, nil
}

func (c *Controller) ListSkillEvolutionCases(ctx context.Context, filter SkillEvolutionCaseFilter) ([]SkillEvolutionCase, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.ListSkillEvolutionCases(ctx, filter)
}

func (c *Controller) BuildSkillEvolutionCaseDetail(ctx context.Context, evolutionCase *SkillEvolutionCase) (*SkillEvolutionCaseDetail, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if evolutionCase == nil {
		return nil, fmt.Errorf("skill evolution case is required")
	}

	detail := &SkillEvolutionCaseDetail{
		SkillEvolutionCase: *evolutionCase,
	}

	if revisionID := strings.TrimSpace(evolutionCase.RevisionID); revisionID != "" {
		revision, err := c.GetSkillRevision(ctx, revisionID)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if err == nil && revision != nil {
			detail.LinkedRevision = revision
			if evalRunID := strings.TrimSpace(revision.EvalRunID); evalRunID != "" {
				detail.LinkedEvalRunID = evalRunID
				evalRun, evalErr := c.GetEvalRun(ctx, evalRunID)
				if evalErr != nil && evalErr != sql.ErrNoRows {
					return nil, evalErr
				}
				if evalErr == nil {
					detail.LinkedEvalRun = evalRun
				}
			}
		}
	}

	if strings.EqualFold(strings.TrimSpace(evolutionCase.SourceKind), "eval_run") {
		if sourceEvalRunID := strings.TrimSpace(evolutionCase.SourceID); sourceEvalRunID != "" {
			detail.SourceEvalRunID = sourceEvalRunID
			sourceEvalRun, err := c.GetEvalRun(ctx, sourceEvalRunID)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if err == nil {
				detail.SourceEvalRun = sourceEvalRun
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(evolutionCase.SourceKind), "runtime_run") {
		if sourceRunID := strings.TrimSpace(evolutionCase.SourceID); sourceRunID != "" {
			detail.SourceRunID = sourceRunID
			sourceRun, err := c.Get(ctx, sourceRunID)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if err == nil {
				detail.SourceRun = sourceRun
			}
		}
	}

	return detail, nil
}

func (s *SQLiteStore) listSkillEvolutionCaseRows(ctx context.Context, db *sql.DB, filter SkillEvolutionCaseFilter) ([]skillEvolutionCaseRow, error) {
	var (
		conds []string
		args  []interface{}
	)
	if v := strings.TrimSpace(filter.SkillID); v != "" {
		conds = append(conds, "skill_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.OwnerUserID); v != "" {
		conds = append(conds, "owner_user_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.CandidateID); v != "" {
		conds = append(conds, "candidate_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.RevisionID); v != "" {
		conds = append(conds, "revision_id = ?")
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
	query := `SELECT
		id, skill_id, owner_user_id, mode, reason, source_kind, source_id, candidate_id,
		base_content_sha256, failure_signature, dedup_key, summary, evidence_json, revision_id,
		status, skipped_reason, created_at, updated_at
		FROM harness_skill_evolution_cases`
	if len(conds) > 0 {
		query += ` WHERE ` + strings.Join(conds, " AND ")
	}
	query += ` ORDER BY updated_at DESC, created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	return querySkillEvolutionCaseRows(ctx, db, query, args...)
}

func (c *Controller) EnsureSkillEvolutionCase(ctx context.Context, spec SkillEvolutionCaseSpec) (*SkillEvolutionCase, bool, error) {
	if c == nil || c.store == nil {
		return nil, false, fmt.Errorf("harness controller is not configured")
	}
	dedupKey := skillEvolutionDedupKey(spec.SkillID, spec.BaseContentSHA256, spec.FailureSignature)
	if dedupKey != "" {
		existing, err := c.store.FindLatestSkillEvolutionCaseByDedupKey(ctx, strings.TrimSpace(spec.SkillID), strings.TrimSpace(spec.OwnerUserID), dedupKey)
		if err == nil && existing != nil {
			return existing, false, nil
		}
		if err != nil && err != sql.ErrNoRows {
			return nil, false, err
		}
	}
	now := timeutil.NowTime()
	evolutionCase := SkillEvolutionCase{
		ID:                strings.TrimSpace(spec.ID),
		SkillID:           strings.TrimSpace(spec.SkillID),
		OwnerUserID:       strings.TrimSpace(spec.OwnerUserID),
		Mode:              spec.Mode,
		Reason:            spec.Reason,
		SourceKind:        strings.TrimSpace(spec.SourceKind),
		SourceID:          strings.TrimSpace(spec.SourceID),
		CandidateID:       strings.TrimSpace(spec.CandidateID),
		BaseContentSHA256: strings.TrimSpace(spec.BaseContentSHA256),
		FailureSignature:  strings.TrimSpace(spec.FailureSignature),
		DedupKey:          dedupKey,
		Summary:           strings.TrimSpace(spec.Summary),
		EvidenceJSON:      strings.TrimSpace(spec.EvidenceJSON),
		Status:            SkillEvolutionCaseStatusOpen,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if evolutionCase.ID == "" {
		evolutionCase.ID = uuid.NewString()
	}
	created, err := c.CreateSkillEvolutionCase(ctx, evolutionCase)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

func (s *SQLiteStore) FindLatestSkillEvolutionCaseByDedupKey(ctx context.Context, skillID string, ownerUserID string, dedupKey string) (*SkillEvolutionCase, error) {
	skillID = strings.TrimSpace(skillID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	dedupKey = strings.TrimSpace(dedupKey)
	if skillID == "" || dedupKey == "" {
		return nil, sql.ErrNoRows
	}
	query := `SELECT
		id, skill_id, owner_user_id, mode, reason, source_kind, source_id, candidate_id,
		base_content_sha256, failure_signature, dedup_key, summary, evidence_json, revision_id,
		status, skipped_reason, created_at, updated_at
		FROM harness_skill_evolution_cases
		WHERE skill_id = ? AND dedup_key = ?`
	args := []interface{}{skillID, dedupKey}
	if ownerUserID != "" {
		query += ` AND owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	query += ` ORDER BY updated_at DESC, created_at DESC, id DESC LIMIT 1`
	rows, err := querySkillEvolutionCaseRows(ctx, s.reader(), query, args...)
	if err != nil {
		if !s.shouldFallbackToWriter(err, false) {
			return nil, err
		}
		rows, err = querySkillEvolutionCaseRows(ctx, s.db, query, args...)
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	evolutionCase := skillEvolutionCaseFromRow(rows[0])
	return &evolutionCase, nil
}

func normalizeSkillEvolutionCaseForWrite(evolutionCase *SkillEvolutionCase) error {
	if evolutionCase == nil {
		return fmt.Errorf("skill evolution case is required")
	}
	evolutionCase.ID = strings.TrimSpace(evolutionCase.ID)
	evolutionCase.SkillID = strings.TrimSpace(evolutionCase.SkillID)
	evolutionCase.OwnerUserID = strings.TrimSpace(evolutionCase.OwnerUserID)
	evolutionCase.SourceKind = strings.TrimSpace(evolutionCase.SourceKind)
	evolutionCase.SourceID = strings.TrimSpace(evolutionCase.SourceID)
	evolutionCase.CandidateID = strings.TrimSpace(evolutionCase.CandidateID)
	evolutionCase.BaseContentSHA256 = strings.TrimSpace(evolutionCase.BaseContentSHA256)
	evolutionCase.FailureSignature = strings.TrimSpace(evolutionCase.FailureSignature)
	evolutionCase.DedupKey = strings.TrimSpace(evolutionCase.DedupKey)
	evolutionCase.Summary = strings.TrimSpace(evolutionCase.Summary)
	evolutionCase.RevisionID = strings.TrimSpace(evolutionCase.RevisionID)
	evolutionCase.SkippedReason = strings.TrimSpace(evolutionCase.SkippedReason)
	if evolutionCase.ID == "" {
		return fmt.Errorf("skill evolution case id is required")
	}
	if evolutionCase.SkillID == "" {
		return fmt.Errorf("skill evolution case skill_id is required")
	}
	if evolutionCase.Mode == "" {
		return fmt.Errorf("skill evolution case mode is required")
	}
	if evolutionCase.Reason == "" {
		return fmt.Errorf("skill evolution case reason is required")
	}
	if evolutionCase.Status == "" {
		evolutionCase.Status = SkillEvolutionCaseStatusOpen
	}
	if evolutionCase.EvidenceJSON == "" {
		evolutionCase.EvidenceJSON = "{}"
	}
	if evolutionCase.CreatedAt.IsZero() {
		evolutionCase.CreatedAt = timeutil.NowTime()
	}
	if evolutionCase.UpdatedAt.IsZero() {
		evolutionCase.UpdatedAt = evolutionCase.CreatedAt
	}
	return nil
}

func skillEvolutionDedupKey(skillID string, baseContentSHA256 string, failureSignature string) string {
	skillID = strings.ToLower(strings.TrimSpace(skillID))
	baseContentSHA256 = strings.ToLower(strings.TrimSpace(baseContentSHA256))
	failureSignature = strings.ToLower(strings.TrimSpace(failureSignature))
	if skillID == "" || baseContentSHA256 == "" || failureSignature == "" {
		return ""
	}
	return skillID + "|" + baseContentSHA256 + "|" + failureSignature
}

func querySkillEvolutionCaseRows(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([]skillEvolutionCaseRow, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]skillEvolutionCaseRow, 0)
	for rows.Next() {
		var row skillEvolutionCaseRow
		if err := rows.Scan(
			&row.ID,
			&row.SkillID,
			&row.OwnerUserID,
			&row.Mode,
			&row.Reason,
			&row.SourceKind,
			&row.SourceID,
			&row.CandidateID,
			&row.BaseContentSHA256,
			&row.FailureSignature,
			&row.DedupKey,
			&row.Summary,
			&row.EvidenceJSON,
			&row.RevisionID,
			&row.Status,
			&row.SkippedReason,
			&row.CreatedAt,
			&row.UpdatedAt,
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
