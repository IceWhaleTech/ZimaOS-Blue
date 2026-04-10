package knowledge

import (
	"context"
	"errors"
	"time"
)

var (
	ErrJobNotFound        = errors.New("knowledge job not found")
	ErrJobForbidden       = errors.New("knowledge job forbidden")
	ErrReportNotReady     = errors.New("knowledge report not ready")
	ErrJobAlreadyTerminal = errors.New("knowledge job already terminal")
	ErrPageNotFound       = errors.New("knowledge page not found")
)

type MemorySink interface {
	Remember(ctx context.Context, content string, tags []string) error
}

type EventPublisher interface {
	Publish(userID string, eventType string, data any)
}

type KnowledgeAuthor interface {
	AuthorDelta(ctx context.Context, req KnowledgeAuthorRequest) (*KnowledgeAuthorResult, error)
}

type KnowledgeAuthorRequest struct {
	Schema        string
	Source        SourceSnapshot
	ExistingPages []KnowledgePageSummary
	GeneratedAt   time.Time
	WorkspaceRoot string
}

type KnowledgeAuthorResult struct {
	Pages        []KnowledgePage
	Conflicts    []string
	Gaps         []string
	FallbackMode bool
}

type SourceSnapshot struct {
	Ref        string
	Title      string
	PageType   string
	Content    string
	Summary    string
	Keywords   []string
	SourceHash string
	Slug       string
}

type ServiceOptions struct {
	WorkspaceDir          string
	RepoRoot              string
	MemorySink            MemorySink
	EventPublisher        EventPublisher
	KnowledgeAuthor       KnowledgeAuthor
	DefaultLintProviderID func() string
	OnCompileSuccess      func(context.Context, *KnowledgeCompileReport)
	Now                   func() time.Time
}

type JobKind string

const (
	JobKindIngest          JobKind = "ingest"
	JobKindCompile         JobKind = "compile"
	JobKindLint            JobKind = "lint"
	JobKindAnswer          JobKind = "answer"
	JobKindRepairConflicts JobKind = "repair_conflicts"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

type IssueKind string

const (
	IssueKindStaleHash        IssueKind = "stale_hash"
	IssueKindOrphanPage       IssueKind = "orphan_page"
	IssueKindDuplicateTopic   IssueKind = "duplicate_topic"
	IssueKindMissingBacklinks IssueKind = "missing_backlinks"
	IssueKindLowQuality       IssueKind = "low_quality"
	IssueKindMissingIndex     IssueKind = "missing_index"
	IssueKindConflictingClaim IssueKind = "conflicting_claim"
	IssueKindInsufficientSrc  IssueKind = "insufficient_sources"
	IssueKindMissingConcept   IssueKind = "missing_concept_page"
	IssueKindMissingEntity    IssueKind = "missing_entity_page"
)

type KnowledgeStatus string

const (
	KnowledgeStatusActive     KnowledgeStatus = "active"
	KnowledgeStatusSuperseded KnowledgeStatus = "superseded"
	KnowledgeStatusConflicted KnowledgeStatus = "conflicted"
)

type KnowledgeConfidence string

const (
	KnowledgeConfidenceLow    KnowledgeConfidence = "low"
	KnowledgeConfidenceMedium KnowledgeConfidence = "medium"
	KnowledgeConfidenceHigh   KnowledgeConfidence = "high"
)

const (
	PageTypeSourceSummary = "source_summary"
	PageTypeEntity        = "entity"
	PageTypeConcept       = "concept"
	PageTypeComparison    = "comparison"
	PageTypeSynthesis     = "synthesis"
	PageTypeDecision      = "decision"
)

type CreateJobRequest struct {
	RequestedID   string   `json:"requested_id,omitempty"`
	UserID        string   `json:"user_id,omitempty"`
	TenantID      string   `json:"tenant_id,omitempty"`
	ProviderID    string   `json:"provider_id,omitempty"`
	Query         string   `json:"query,omitempty"`
	Kind          JobKind  `json:"kind"`
	TargetPaths   []string `json:"target_paths,omitempty"`
	TargetSlugs   []string `json:"target_slugs,omitempty"`
	PageSlug      string   `json:"page_slug,omitempty"`
	ArchiveAnswer bool     `json:"archive_answer,omitempty"`
	QueryScope    string   `json:"query_scope,omitempty"`
	SelectedRefs  []string `json:"selected_refs,omitempty"`
}

type CompileRequest struct {
	TargetPaths []string `json:"target_paths,omitempty"`
	ProviderID  string   `json:"provider_id,omitempty"`
}

type LintRequest struct {
	TargetPaths []string `json:"target_paths,omitempty"`
	ProviderID  string   `json:"provider_id,omitempty"`
}

type RepairConflictsRequest struct {
	TargetSlugs []string `json:"target_slugs,omitempty"`
	ProviderID  string   `json:"provider_id,omitempty"`
}

type AnswerRequest struct {
	Query         string   `json:"query"`
	PageSlug      string   `json:"page_slug,omitempty"`
	ArchiveAnswer bool     `json:"archive_answer,omitempty"`
	QueryScope    string   `json:"query_scope,omitempty"`
	SelectedRefs  []string `json:"selected_refs,omitempty"`
}

type Event struct {
	ID        string      `json:"id,omitempty"`
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"`
}

type KnowledgePageSummary struct {
	Title            string              `json:"title"`
	Slug             string              `json:"slug"`
	PageType         string              `json:"page_type"`
	Summary          string              `json:"summary"`
	SourceRefs       []string            `json:"source_refs"`
	Keywords         []string            `json:"keywords"`
	Backlinks        []string            `json:"backlinks"`
	GeneratedAt      time.Time           `json:"generated_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	SourceHash       string              `json:"source_hash"`
	Status           KnowledgeStatus     `json:"status"`
	Confidence       KnowledgeConfidence `json:"confidence"`
	ConflictsWith    []string            `json:"conflicts_with,omitempty"`
	SupersededBy     []string            `json:"superseded_by,omitempty"`
	DerivedFromQuery string              `json:"derived_from_query,omitempty"`
}

type KnowledgeArchivedAnswer struct {
	Title       string    `json:"title"`
	Path        string    `json:"path"`
	PageSlug    string    `json:"page_slug"`
	Query       string    `json:"query"`
	Summary     string    `json:"summary"`
	GeneratedAt time.Time `json:"generated_at"`
}

type KnowledgePage struct {
	KnowledgePageSummary
	Content string                    `json:"content"`
	Answers []KnowledgeArchivedAnswer `json:"answers,omitempty"`
}

type KnowledgeIngestReport struct {
	GeneratedAt    time.Time              `json:"generated_at"`
	Pages          []KnowledgePageSummary `json:"pages"`
	NewPages       []KnowledgePageSummary `json:"new_pages,omitempty"`
	UpdatedPages   []KnowledgePageSummary `json:"updated_pages,omitempty"`
	SkippedSources []string               `json:"skipped_sources,omitempty"`
	Conflicts      []string               `json:"conflicts,omitempty"`
	Gaps           []string               `json:"gaps,omitempty"`
	ManifestPath   string                 `json:"manifest_path"`
	IndexPath      string                 `json:"index_path"`
	SchemaPath     string                 `json:"schema_path"`
	LogPath        string                 `json:"log_path"`
	FallbackMode   bool                   `json:"fallback_mode,omitempty"`
}

type KnowledgeCompileReport = KnowledgeIngestReport

type LintIssue struct {
	Kind            IssueKind `json:"kind"`
	PageSlug        string    `json:"page_slug,omitempty"`
	Message         string    `json:"message"`
	AutoFixed       bool      `json:"auto_fixed,omitempty"`
	Severity        string    `json:"severity,omitempty"`
	Category        string    `json:"category,omitempty"`
	RelatedPages    []string  `json:"related_pages,omitempty"`
	SuggestedAction string    `json:"suggested_action,omitempty"`
}

type KnowledgeLintReport struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Issues      []LintIssue `json:"issues"`
	FixedPaths  []string    `json:"fixed_paths,omitempty"`
}

type KnowledgeConflictRepairReport struct {
	GeneratedAt     time.Time              `json:"generated_at"`
	CanonicalPages  []KnowledgePageSummary `json:"canonical_pages,omitempty"`
	SupersededPages []KnowledgePageSummary `json:"superseded_pages,omitempty"`
	FixedPaths      []string               `json:"fixed_paths,omitempty"`
	Lint            *KnowledgeLintReport   `json:"lint,omitempty"`
}

type KnowledgeCitation struct {
	PageSlug   string   `json:"page_slug"`
	Title      string   `json:"title"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type KnowledgeAnswerReport struct {
	GeneratedAt      time.Time              `json:"generated_at"`
	Query            string                 `json:"query"`
	Answer           string                 `json:"answer"`
	Citations        []KnowledgeCitation    `json:"citations"`
	RelatedPages     []KnowledgePageSummary `json:"related_pages,omitempty"`
	ArchivedPath     string                 `json:"archived_path,omitempty"`
	Confidence       KnowledgeConfidence    `json:"confidence,omitempty"`
	ConflictNotes    []string               `json:"conflict_notes,omitempty"`
	OpenQuestions    []string               `json:"open_questions,omitempty"`
	PromotedPageSlug string                 `json:"promoted_page_slug,omitempty"`
}

type KnowledgeJobReport struct {
	Kind    JobKind                        `json:"kind"`
	Ingest  *KnowledgeIngestReport         `json:"ingest,omitempty"`
	Compile *KnowledgeCompileReport        `json:"compile,omitempty"`
	Lint    *KnowledgeLintReport           `json:"lint,omitempty"`
	Repair  *KnowledgeConflictRepairReport `json:"repair,omitempty"`
	Answer  *KnowledgeAnswerReport         `json:"answer,omitempty"`
}

type KnowledgeJob struct {
	ID          string              `json:"id"`
	UserID      string              `json:"user_id,omitempty"`
	TenantID    string              `json:"tenant_id,omitempty"`
	ProviderID  string              `json:"provider_id,omitempty"`
	Query       string              `json:"query,omitempty"`
	Kind        JobKind             `json:"kind"`
	Status      JobStatus           `json:"status"`
	Progress    int                 `json:"progress"`
	Stage       string              `json:"stage,omitempty"`
	Error       string              `json:"error,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
	Report      *KnowledgeJobReport `json:"report,omitempty"`
}

type KnowledgeJobSummary struct {
	ID         string    `json:"id"`
	JobID      string    `json:"job_id"`
	ProviderID string    `json:"provider_id,omitempty"`
	Query      string    `json:"query,omitempty"`
	Kind       JobKind   `json:"kind"`
	Status     JobStatus `json:"status"`
	Progress   int       `json:"progress"`
	Stage      string    `json:"stage,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type KnowledgeSchemaDocument struct {
	Content string `json:"content"`
}

type KnowledgeLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Operation    string    `json:"operation"`
	Title        string    `json:"title"`
	Sources      []string  `json:"sources,omitempty"`
	NewPages     []string  `json:"new_pages,omitempty"`
	UpdatedPages []string  `json:"updated_pages,omitempty"`
	Conflicts    []string  `json:"conflicts,omitempty"`
	Gaps         []string  `json:"gaps,omitempty"`
	Reason       string    `json:"reason,omitempty"`
}

type PromoteQueryResult struct {
	Page KnowledgePageSummary `json:"page"`
}
