package selfreflect

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type ProposalStatus string

const (
	ProposalStatusPending  ProposalStatus = "pending"
	ProposalStatusApproved ProposalStatus = "approved"
	ProposalStatusRejected ProposalStatus = "rejected"
)

type ProposalMode string

const (
	ProposalModeReviewOnly ProposalMode = "review_only"
)

type ProposalCandidate struct {
	Lesson      string   `json:"lesson"`
	WhenToApply string   `json:"when_to_apply,omitempty"`
	Evidence    string   `json:"evidence"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
	TargetFile  string   `json:"target_file,omitempty"`
}

type Proposal struct {
	ID                 string                 `json:"id"`
	OwnerUserID        string                 `json:"owner_user_id,omitempty"`
	SourceKind         string                 `json:"source_kind,omitempty"`
	SourceID           string                 `json:"source_id,omitempty"`
	ProposalMode       ProposalMode           `json:"proposal_mode,omitempty"`
	TargetFile         string                 `json:"target_file"`
	TargetSection      string                 `json:"target_section,omitempty"`
	Status             ProposalStatus         `json:"status"`
	DedupKey           string                 `json:"dedup_key,omitempty"`
	Lesson             string                 `json:"lesson"`
	WhenToApply        string                 `json:"when_to_apply,omitempty"`
	Evidence           string                 `json:"evidence"`
	EvidenceIDs        []string               `json:"evidence_ids,omitempty"`
	EvaluationSummary  map[string]interface{} `json:"evaluation_summary,omitempty"`
	CalibrationSummary map[string]interface{} `json:"calibration_summary,omitempty"`
	PatchPreview       string                 `json:"patch_preview,omitempty"`
	ReviewNote         string                 `json:"review_note,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	ReviewedAt         *time.Time             `json:"reviewed_at,omitempty"`
}

type ProposalFilter struct {
	OwnerUserID string
	SourceKind  string
	SourceID    string
	Statuses    []ProposalStatus
	Limit       int
}

type ProposalStore interface {
	CreateProposal(ctx context.Context, proposal *Proposal) error
	UpdateProposal(ctx context.Context, proposal *Proposal) error
	GetProposal(ctx context.Context, id string) (*Proposal, error)
	ListProposals(ctx context.Context, filter ProposalFilter) ([]Proposal, error)
	FindProposalByDedup(ctx context.Context, dedupKey string, targetFile string) (*Proposal, error)
}

type SQLiteProposalStore struct {
	db *sql.DB
}

const proposalSchemaSQL = `
CREATE TABLE IF NOT EXISTS self_reflect_proposals (
	id TEXT PRIMARY KEY,
	owner_user_id TEXT DEFAULT '',
	source_kind TEXT DEFAULT '',
	source_id TEXT DEFAULT '',
	proposal_mode TEXT DEFAULT '',
	target_file TEXT NOT NULL,
	target_section TEXT DEFAULT '',
	status TEXT NOT NULL,
	dedup_key TEXT DEFAULT '',
	lesson TEXT NOT NULL,
	when_to_apply TEXT DEFAULT '',
	evidence TEXT NOT NULL,
	evidence_ids_json TEXT NOT NULL DEFAULT '[]',
	evaluation_summary_json TEXT NOT NULL DEFAULT '{}',
	calibration_summary_json TEXT NOT NULL DEFAULT '{}',
	patch_preview TEXT DEFAULT '',
	review_note TEXT DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	reviewed_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_self_reflect_proposals_dedup ON self_reflect_proposals(dedup_key, target_file);
CREATE INDEX IF NOT EXISTS idx_self_reflect_proposals_owner_status ON self_reflect_proposals(owner_user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_self_reflect_proposals_source ON self_reflect_proposals(source_kind, source_id, created_at DESC);
`

func NewSQLiteProposalStore(db *sql.DB) (*SQLiteProposalStore, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	if _, err := db.Exec(proposalSchemaSQL); err != nil {
		return nil, err
	}
	return &SQLiteProposalStore{db: db}, nil
}

func (s *SQLiteProposalStore) CreateProposal(ctx context.Context, proposal *Proposal) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("proposal store is not configured")
	}
	if proposal == nil {
		return fmt.Errorf("proposal is required")
	}
	now := timeutil.NowTime()
	if strings.TrimSpace(proposal.ID) == "" {
		proposal.ID = uuid.NewString()
	}
	if proposal.CreatedAt.IsZero() {
		proposal.CreatedAt = now
	}
	if proposal.UpdatedAt.IsZero() {
		proposal.UpdatedAt = proposal.CreatedAt
	}
	if proposal.Status == "" {
		proposal.Status = ProposalStatusPending
	}
	if proposal.ProposalMode == "" {
		proposal.ProposalMode = ProposalModeReviewOnly
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO self_reflect_proposals (
		id, owner_user_id, source_kind, source_id, proposal_mode, target_file, target_section, status, dedup_key,
		lesson, when_to_apply, evidence, evidence_ids_json, evaluation_summary_json, calibration_summary_json,
		patch_preview, review_note, created_at, updated_at, reviewed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proposal.ID,
		strings.TrimSpace(proposal.OwnerUserID),
		strings.TrimSpace(proposal.SourceKind),
		strings.TrimSpace(proposal.SourceID),
		string(proposal.ProposalMode),
		strings.TrimSpace(proposal.TargetFile),
		strings.TrimSpace(proposal.TargetSection),
		string(proposal.Status),
		strings.TrimSpace(proposal.DedupKey),
		strings.TrimSpace(proposal.Lesson),
		strings.TrimSpace(proposal.WhenToApply),
		strings.TrimSpace(proposal.Evidence),
		marshalProposalJSON(proposal.EvidenceIDs, "[]"),
		marshalProposalJSON(proposal.EvaluationSummary, "{}"),
		marshalProposalJSON(proposal.CalibrationSummary, "{}"),
		proposal.PatchPreview,
		strings.TrimSpace(proposal.ReviewNote),
		proposal.CreatedAt,
		proposal.UpdatedAt,
		nullableTime(proposal.ReviewedAt),
	)
	return err
}

func (s *SQLiteProposalStore) UpdateProposal(ctx context.Context, proposal *Proposal) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("proposal store is not configured")
	}
	if proposal == nil {
		return fmt.Errorf("proposal is required")
	}
	proposal.UpdatedAt = timeutil.NowTime()
	_, err := s.db.ExecContext(ctx, `UPDATE self_reflect_proposals SET
		owner_user_id=?, source_kind=?, source_id=?, proposal_mode=?, target_file=?, target_section=?, status=?, dedup_key=?,
		lesson=?, when_to_apply=?, evidence=?, evidence_ids_json=?, evaluation_summary_json=?, calibration_summary_json=?,
		patch_preview=?, review_note=?, updated_at=?, reviewed_at=?
		WHERE id=?`,
		strings.TrimSpace(proposal.OwnerUserID),
		strings.TrimSpace(proposal.SourceKind),
		strings.TrimSpace(proposal.SourceID),
		string(proposal.ProposalMode),
		strings.TrimSpace(proposal.TargetFile),
		strings.TrimSpace(proposal.TargetSection),
		string(proposal.Status),
		strings.TrimSpace(proposal.DedupKey),
		strings.TrimSpace(proposal.Lesson),
		strings.TrimSpace(proposal.WhenToApply),
		strings.TrimSpace(proposal.Evidence),
		marshalProposalJSON(proposal.EvidenceIDs, "[]"),
		marshalProposalJSON(proposal.EvaluationSummary, "{}"),
		marshalProposalJSON(proposal.CalibrationSummary, "{}"),
		proposal.PatchPreview,
		strings.TrimSpace(proposal.ReviewNote),
		proposal.UpdatedAt,
		nullableTime(proposal.ReviewedAt),
		proposal.ID,
	)
	return err
}

func (s *SQLiteProposalStore) GetProposal(ctx context.Context, id string) (*Proposal, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	row := s.db.QueryRowContext(ctx, `SELECT
		id, owner_user_id, source_kind, source_id, proposal_mode, target_file, target_section, status, dedup_key,
		lesson, when_to_apply, evidence, evidence_ids_json, evaluation_summary_json, calibration_summary_json,
		patch_preview, review_note, created_at, updated_at, reviewed_at
		FROM self_reflect_proposals
		WHERE id = ?`, strings.TrimSpace(id))
	return scanProposal(row)
}

func (s *SQLiteProposalStore) ListProposals(ctx context.Context, filter ProposalFilter) ([]Proposal, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	query := `SELECT
		id, owner_user_id, source_kind, source_id, proposal_mode, target_file, target_section, status, dedup_key,
		lesson, when_to_apply, evidence, evidence_ids_json, evaluation_summary_json, calibration_summary_json,
		patch_preview, review_note, created_at, updated_at, reviewed_at
		FROM self_reflect_proposals`
	var (
		clauses []string
		args    []interface{}
	)
	if owner := strings.TrimSpace(filter.OwnerUserID); owner != "" {
		clauses = append(clauses, "owner_user_id = ?")
		args = append(args, owner)
	}
	if sourceKind := strings.TrimSpace(filter.SourceKind); sourceKind != "" {
		clauses = append(clauses, "source_kind = ?")
		args = append(args, sourceKind)
	}
	if sourceID := strings.TrimSpace(filter.SourceID); sourceID != "" {
		clauses = append(clauses, "source_id = ?")
		args = append(args, sourceID)
	}
	if len(filter.Statuses) > 0 {
		parts := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			if status == "" {
				continue
			}
			parts = append(parts, "?")
			args = append(args, string(status))
		}
		if len(parts) > 0 {
			clauses = append(clauses, "status IN ("+strings.Join(parts, ",")+")")
		}
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Proposal, 0, limit)
	for rows.Next() {
		proposal, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *proposal)
	}
	return out, rows.Err()
}

func (s *SQLiteProposalStore) FindProposalByDedup(ctx context.Context, dedupKey string, targetFile string) (*Proposal, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	row := s.db.QueryRowContext(ctx, `SELECT
		id, owner_user_id, source_kind, source_id, proposal_mode, target_file, target_section, status, dedup_key,
		lesson, when_to_apply, evidence, evidence_ids_json, evaluation_summary_json, calibration_summary_json,
		patch_preview, review_note, created_at, updated_at, reviewed_at
		FROM self_reflect_proposals
		WHERE dedup_key = ? AND target_file = ?
		LIMIT 1`, strings.TrimSpace(dedupKey), strings.TrimSpace(targetFile))
	return scanProposal(row)
}

type proposalRowScanner interface {
	Scan(dest ...interface{}) error
}

func scanProposal(scanner proposalRowScanner) (*Proposal, error) {
	var (
		proposal               Proposal
		proposalMode           string
		status                 string
		evidenceIDsJSON        string
		evaluationSummaryJSON  string
		calibrationSummaryJSON string
		reviewedAt             sql.NullTime
	)
	if err := scanner.Scan(
		&proposal.ID,
		&proposal.OwnerUserID,
		&proposal.SourceKind,
		&proposal.SourceID,
		&proposalMode,
		&proposal.TargetFile,
		&proposal.TargetSection,
		&status,
		&proposal.DedupKey,
		&proposal.Lesson,
		&proposal.WhenToApply,
		&proposal.Evidence,
		&evidenceIDsJSON,
		&evaluationSummaryJSON,
		&calibrationSummaryJSON,
		&proposal.PatchPreview,
		&proposal.ReviewNote,
		&proposal.CreatedAt,
		&proposal.UpdatedAt,
		&reviewedAt,
	); err != nil {
		return nil, err
	}
	proposal.ProposalMode = ProposalMode(strings.TrimSpace(proposalMode))
	proposal.Status = ProposalStatus(strings.TrimSpace(status))
	proposal.EvidenceIDs = unmarshalProposalStringSlice(evidenceIDsJSON)
	proposal.EvaluationSummary = unmarshalProposalMap(evaluationSummaryJSON)
	proposal.CalibrationSummary = unmarshalProposalMap(calibrationSummaryJSON)
	if reviewedAt.Valid {
		ts := reviewedAt.Time
		proposal.ReviewedAt = &ts
	}
	return &proposal, nil
}

func marshalProposalJSON(value interface{}, fallback string) string {
	if value == nil {
		return fallback
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fallback
	}
	return string(raw)
}

func unmarshalProposalStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func unmarshalProposalMap(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func nullableTime(ts *time.Time) interface{} {
	if ts == nil || ts.IsZero() {
		return nil
	}
	return *ts
}
