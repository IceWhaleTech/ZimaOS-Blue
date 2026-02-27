package deepsearch

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
	Query       string     `json:"query"`
	Lang        string     `json:"lang,omitempty"`
	Mode        Mode       `json:"mode"`
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
	ID       string `json:"id"`
	Question string `json:"question"`
	Priority int    `json:"priority"`
	Depth    int    `json:"depth"`
	Status   string `json:"status"`
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
	RelevanceScore   float64   `json:"relevance_score"`
	CredibilityScore float64   `json:"credibility_score"`
	NoveltyScore     float64   `json:"novelty_score"`
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
}

type Event struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"`
}

type CreateJobRequest struct {
	Query  string  `json:"query"`
	Mode   Mode    `json:"mode,omitempty"`
	Lang   string  `json:"lang,omitempty"`
	Budget *Budget `json:"budget,omitempty"`
}
