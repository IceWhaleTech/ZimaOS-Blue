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
	z "github.com/IceWhaleTech/zorm"
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
	db     *sql.DB
	readDB *sql.DB
}

type proposalRow struct {
	ID                     string  `json:"id" zorm:"id"`
	OwnerUserID            string  `json:"owner_user_id" zorm:"owner_user_id"`
	SourceKind             string  `json:"source_kind" zorm:"source_kind"`
	SourceID               string  `json:"source_id" zorm:"source_id"`
	ProposalMode           string  `json:"proposal_mode" zorm:"proposal_mode"`
	TargetFile             string  `json:"target_file" zorm:"target_file"`
	TargetSection          string  `json:"target_section" zorm:"target_section"`
	Status                 string  `json:"status" zorm:"status"`
	DedupKey               string  `json:"dedup_key" zorm:"dedup_key"`
	Lesson                 string  `json:"lesson" zorm:"lesson"`
	WhenToApply            string  `json:"when_to_apply" zorm:"when_to_apply"`
	Evidence               string  `json:"evidence" zorm:"evidence"`
	EvidenceIDsJSON        string  `json:"evidence_ids_json" zorm:"evidence_ids_json"`
	EvaluationSummaryJSON  string  `json:"evaluation_summary_json" zorm:"evaluation_summary_json"`
	CalibrationSummaryJSON string  `json:"calibration_summary_json" zorm:"calibration_summary_json"`
	PatchPreview           string  `json:"patch_preview" zorm:"patch_preview"`
	ReviewNote             string  `json:"review_note" zorm:"review_note"`
	CreatedAt              string  `json:"created_at" zorm:"created_at"`
	UpdatedAt              string  `json:"updated_at" zorm:"updated_at"`
	ReviewedAt             *string `json:"reviewed_at" zorm:"reviewed_at"`
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
	return NewSQLiteProposalStoreWithReadDB(db, db)
}

// NewSQLiteProposalStoreWithReadDB creates a proposal store with separate
// write and read database handles.
func NewSQLiteProposalStoreWithReadDB(writeDB, readDB *sql.DB) (*SQLiteProposalStore, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("db is required")
	}
	if readDB == nil {
		readDB = writeDB
	}
	if _, err := writeDB.Exec(proposalSchemaSQL); err != nil {
		return nil, err
	}
	return &SQLiteProposalStore{db: writeDB, readDB: readDB}, nil
}

func (s *SQLiteProposalStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *SQLiteProposalStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "self_reflect_proposals")
}

func (s *SQLiteProposalStore) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "self_reflect_proposals")
}

func proposalToValues(proposal *Proposal) z.V {
	return z.V{
		"id":                       proposal.ID,
		"owner_user_id":            strings.TrimSpace(proposal.OwnerUserID),
		"source_kind":              strings.TrimSpace(proposal.SourceKind),
		"source_id":                strings.TrimSpace(proposal.SourceID),
		"proposal_mode":            string(proposal.ProposalMode),
		"target_file":              strings.TrimSpace(proposal.TargetFile),
		"target_section":           strings.TrimSpace(proposal.TargetSection),
		"status":                   string(proposal.Status),
		"dedup_key":                strings.TrimSpace(proposal.DedupKey),
		"lesson":                   strings.TrimSpace(proposal.Lesson),
		"when_to_apply":            strings.TrimSpace(proposal.WhenToApply),
		"evidence":                 strings.TrimSpace(proposal.Evidence),
		"evidence_ids_json":        marshalProposalJSON(proposal.EvidenceIDs, "[]"),
		"evaluation_summary_json":  marshalProposalJSON(proposal.EvaluationSummary, "{}"),
		"calibration_summary_json": marshalProposalJSON(proposal.CalibrationSummary, "{}"),
		"patch_preview":            proposal.PatchPreview,
		"review_note":              strings.TrimSpace(proposal.ReviewNote),
		"created_at":               proposal.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":               proposal.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"reviewed_at":              nullableTimeString(proposal.ReviewedAt),
	}
}

func proposalFromRow(row proposalRow) *Proposal {
	proposal := &Proposal{
		ID:                 row.ID,
		OwnerUserID:        row.OwnerUserID,
		SourceKind:         row.SourceKind,
		SourceID:           row.SourceID,
		ProposalMode:       ProposalMode(strings.TrimSpace(row.ProposalMode)),
		TargetFile:         row.TargetFile,
		TargetSection:      row.TargetSection,
		Status:             ProposalStatus(strings.TrimSpace(row.Status)),
		DedupKey:           row.DedupKey,
		Lesson:             row.Lesson,
		WhenToApply:        row.WhenToApply,
		Evidence:           row.Evidence,
		EvidenceIDs:        unmarshalProposalStringSlice(row.EvidenceIDsJSON),
		EvaluationSummary:  unmarshalProposalMap(row.EvaluationSummaryJSON),
		CalibrationSummary: unmarshalProposalMap(row.CalibrationSummaryJSON),
		PatchPreview:       row.PatchPreview,
		ReviewNote:         row.ReviewNote,
	}
	proposal.CreatedAt = parseProposalTime(row.CreatedAt)
	proposal.UpdatedAt = parseProposalTime(row.UpdatedAt)
	if row.ReviewedAt != nil && strings.TrimSpace(*row.ReviewedAt) != "" {
		ts := parseProposalTime(*row.ReviewedAt)
		if !ts.IsZero() {
			proposal.ReviewedAt = &ts
		}
	}
	return proposal
}

func parseProposalTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
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
	_, err := s.table(ctx).Insert(proposalToValues(proposal))
	return err
}

func (s *SQLiteProposalStore) UpdateProposal(ctx context.Context, proposal *Proposal) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("proposal store is not configured")
	}
	if proposal == nil {
		return fmt.Errorf("proposal is required")
	}
	proposal.UpdatedAt = timeutil.NowTime().UTC()
	_, err := s.table(ctx).Update(
		proposalToValues(proposal),
		z.Fields(
			"owner_user_id",
			"source_kind",
			"source_id",
			"proposal_mode",
			"target_file",
			"target_section",
			"status",
			"dedup_key",
			"lesson",
			"when_to_apply",
			"evidence",
			"evidence_ids_json",
			"evaluation_summary_json",
			"calibration_summary_json",
			"patch_preview",
			"review_note",
			"updated_at",
			"reviewed_at",
		),
		z.Where(z.Eq("id", proposal.ID)),
	)
	return err
}

func (s *SQLiteProposalStore) GetProposal(ctx context.Context, id string) (*Proposal, error) {
	if s == nil || s.reader() == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	var rows []proposalRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("id", strings.TrimSpace(id))),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return proposalFromRow(rows[0]), nil
}

func (s *SQLiteProposalStore) ListProposals(ctx context.Context, filter ProposalFilter) ([]Proposal, error) {
	if s == nil || s.reader() == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	var conds []interface{}
	if owner := strings.TrimSpace(filter.OwnerUserID); owner != "" {
		conds = append(conds, z.Eq("owner_user_id", owner))
	}
	if sourceKind := strings.TrimSpace(filter.SourceKind); sourceKind != "" {
		conds = append(conds, z.Eq("source_kind", sourceKind))
	}
	if sourceID := strings.TrimSpace(filter.SourceID); sourceID != "" {
		conds = append(conds, z.Eq("source_id", sourceID))
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
			statusVals := make([]interface{}, 0, len(statuses))
			for _, status := range statuses {
				statusVals = append(statusVals, status)
			}
			conds = append(conds, z.In("status", statusVals...))
		}
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []proposalRow
	if _, err := s.readTable(ctx).Select(&rows, opts...); err != nil {
		return nil, err
	}
	out := make([]Proposal, 0, len(rows))
	for i := range rows {
		out = append(out, *proposalFromRow(rows[i]))
	}
	return out, nil
}

func (s *SQLiteProposalStore) FindProposalByDedup(ctx context.Context, dedupKey string, targetFile string) (*Proposal, error) {
	if s == nil || s.reader() == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	var rows []proposalRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(
			z.Eq("dedup_key", strings.TrimSpace(dedupKey)),
			z.Eq("target_file", strings.TrimSpace(targetFile)),
		),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return proposalFromRow(rows[0]), nil
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

func nullableTimeString(ts *time.Time) interface{} {
	if ts == nil || ts.IsZero() {
		return nil
	}
	return ts.UTC().Format(time.RFC3339Nano)
}
