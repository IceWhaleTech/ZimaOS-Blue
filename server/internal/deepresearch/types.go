package deepresearch

import "time"

type Mode string

const (
	ModeFast     Mode = "fast"
	ModeStandard Mode = "standard"
	ModeDeep     Mode = "deep"
)

type JobStatus string

const (
	JobStatusPending      JobStatus = "pending"
	JobStatusRunning      JobStatus = "running"
	JobStatusSynthesizing JobStatus = "synthesizing"
	JobStatusCompleted    JobStatus = "completed"
	JobStatusFailed       JobStatus = "failed"
	JobStatusCancelled    JobStatus = "cancelled"
)

type Budget struct {
	MaxSteps   int `json:"max_steps"`
	MaxSources int `json:"max_sources"`
	MaxSeconds int `json:"max_seconds"`
}

type Job struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id,omitempty"`
	TenantID    string     `json:"tenant_id,omitempty"`
	Query       string     `json:"query"`
	Lang        string     `json:"lang,omitempty"`
	Mode        Mode       `json:"mode"`
	StrictEntity bool      `json:"strict_entity,omitempty"`
	TimeWindows []string   `json:"time_windows,omitempty"`
	ReportStyle string     `json:"report_style,omitempty"`
	Status      JobStatus  `json:"status"`
	Budget      Budget     `json:"budget"`
	Progress    int        `json:"progress"`
	Stage       string     `json:"stage,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Tasks       []Task     `json:"tasks,omitempty"`
	Evidence    []Evidence `json:"evidence,omitempty"`
	Report      *Report    `json:"report,omitempty"`
}

type Task struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Priority    int      `json:"priority"`
	Depth       int      `json:"depth"`
	Status      string   `json:"status"`
	Category    string   `json:"category,omitempty"`
	TimeWindow  string   `json:"time_window,omitempty"`
	NegKeywords []string `json:"neg_keywords,omitempty"`
}

type Evidence struct {
	ID               string    `json:"id"`
	TaskID           string    `json:"task_id"`
	Query            string    `json:"query"`
	Title            string    `json:"title"`
	URL              string    `json:"url"`
	Snippet          string    `json:"snippet,omitempty"`
	Source           string    `json:"source,omitempty"`
	Domain           string    `json:"domain,omitempty"`
	FetchedAt        time.Time `json:"fetched_at"`
	PublishedAt      *time.Time `json:"published_at,omitempty"`
	Author           string    `json:"author,omitempty"`
	Quote            string    `json:"quote,omitempty"`
	RelevanceScore   float64   `json:"relevance_score"`
	CredibilityScore float64   `json:"credibility_score"`
	NoveltyScore     float64   `json:"novelty_score"`
	EntityScore      float64   `json:"entity_score,omitempty"`
	ClaimKey         string    `json:"claim_key,omitempty"`
	TimeLabel        string    `json:"time_label,omitempty"`
}

type Citation struct {
	EvidenceID string `json:"evidence_id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

type Report struct {
	Answer        string     `json:"answer"`
	Confidence    float64    `json:"confidence"`
	Citations     []Citation `json:"citations"`
	OpenQuestions []string   `json:"open_questions,omitempty"`
	SupportCount  int        `json:"support_count,omitempty"`
	ConflictCount int        `json:"conflict_count,omitempty"`
	HasConflict   bool       `json:"has_conflict,omitempty"`
	CitationCoverage    float64              `json:"citation_coverage,omitempty"`
	EntityDisambiguation *EntityDisambiguation `json:"entity_disambiguation,omitempty"`
	StageErrors         []string             `json:"stage_errors,omitempty"`
	TimelineSections    []TimelineSection    `json:"timeline_sections,omitempty"`
}

type EntityDisambiguation struct {
	Enabled        bool    `json:"enabled"`
	Threshold      float64 `json:"threshold"`
	FilteredCount  int     `json:"filtered_count,omitempty"`
	AmbiguousCount int     `json:"ambiguous_count,omitempty"`
}

type TimelineSection struct {
	Label      string   `json:"label"`
	Highlights []string `json:"highlights,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type Event struct {
	ID        string      `json:"id,omitempty"`
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"`
}

type CreateJobRequest struct {
	UserID   string  `json:"user_id,omitempty"`
	TenantID string  `json:"tenant_id,omitempty"`
	Query    string  `json:"query"`
	Mode     Mode    `json:"mode,omitempty"`
	Lang     string  `json:"lang,omitempty"`
	Budget   *Budget `json:"budget,omitempty"`
	StrictEntity *bool    `json:"strict_entity,omitempty"`
	TimeWindows  []string `json:"time_windows,omitempty"`
	ReportStyle  string   `json:"report_style,omitempty"`
}
