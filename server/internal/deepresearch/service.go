package deepresearch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type Service struct {
	planner  Planner
	searcher Searcher

	mu          sync.RWMutex
	jobs        map[string]*Job
	cancelFuncs map[string]context.CancelFunc
	subscribers map[string]map[chan Event]struct{}
}

const (
	fastSearchResultsPerTask     = 6
	standardSearchResultsPerTask = 8
	deepResearchResultsPerTask   = 12
)

func NewService(planner Planner, searcher Searcher) *Service {
	if planner == nil {
		planner = NewHeuristicPlanner()
	}
	if searcher == nil {
		searcher = NewToolWebSearcher()
	}
	return &Service{
		planner:     planner,
		searcher:    searcher,
		jobs:        make(map[string]*Job),
		cancelFuncs: make(map[string]context.CancelFunc),
		subscribers: make(map[string]map[chan Event]struct{}),
	}
}

func (s *Service) CreateJob(ctx context.Context, req CreateJobRequest) (*Job, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	mode := req.Mode
	if mode == "" {
		mode = ModeStandard
	}
	budget := defaultBudget(mode)
	if req.Budget != nil {
		if req.Budget.MaxSteps > 0 {
			budget.MaxSteps = req.Budget.MaxSteps
		}
		if req.Budget.MaxSources > 0 {
			budget.MaxSources = req.Budget.MaxSources
		}
		if req.Budget.MaxSeconds > 0 {
			budget.MaxSeconds = req.Budget.MaxSeconds
		}
	}

	now := timeutil.NowTime()
	job := &Job{
		ID:        uuid.NewString(),
		Query:     query,
		Lang:      strings.TrimSpace(req.Lang),
		Mode:      mode,
		Status:    JobStatusPending,
		Budget:    budget,
		CreatedAt: now,
		UpdatedAt: now,
		Stage:     "intake",
	}

	timeout := time.Duration(maxInt(1, budget.MaxSeconds)) * time.Second
	runCtx, cancel := context.WithTimeout(context.Background(), timeout)

	s.mu.Lock()
	s.jobs[job.ID] = job
	s.cancelFuncs[job.ID] = cancel
	s.mu.Unlock()

	go s.runJob(runCtx, job.ID)

	return cloneJob(job), nil
}

func (s *Service) GetJob(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found")
	}
	return cloneJob(job), nil
}

func (s *Service) GetReport(id string) (*Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found")
	}
	if job.Report == nil {
		return nil, fmt.Errorf("report not ready")
	}
	rep := *job.Report
	return &rep, nil
}

func (s *Service) CancelJob(id string) error {
	s.mu.RLock()
	cancel := s.cancelFuncs[id]
	s.mu.RUnlock()
	if cancel == nil {
		return fmt.Errorf("job not found")
	}
	cancel()
	return nil
}

func (s *Service) Subscribe(jobID string) (<-chan Event, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[jobID]; !ok {
		return nil, nil, fmt.Errorf("job not found")
	}

	ch := make(chan Event, 32)
	if s.subscribers[jobID] == nil {
		s.subscribers[jobID] = make(map[chan Event]struct{})
	}
	s.subscribers[jobID][ch] = struct{}{}

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if subs, ok := s.subscribers[jobID]; ok {
			delete(subs, ch)
			close(ch)
			if len(subs) == 0 {
				delete(s.subscribers, jobID)
			}
		}
	}
	return ch, cancel, nil
}

func (s *Service) runJob(ctx context.Context, jobID string) {
	defer func() {
		s.mu.Lock()
		delete(s.cancelFuncs, jobID)
		s.mu.Unlock()
	}()

	if !s.updateJob(jobID, func(j *Job) {
		j.Status = JobStatusRunning
		j.Stage = "planning"
		j.Progress = 10
	}) {
		return
	}
	s.broadcast(jobID, "job_started", map[string]interface{}{"job_id": jobID})

	job, ok := s.getJobPtr(jobID)
	if !ok {
		return
	}

	tasks := s.planner.Plan(job.Query, job.Mode)
	retrieveStepBudget := maxInt(1, job.Budget.MaxSteps-2) // reserve steps for plan + synthesize
	if len(tasks) > retrieveStepBudget {
		tasks = tasks[:retrieveStepBudget]
	}
	s.updateJob(jobID, func(j *Job) {
		j.Tasks = tasks
		j.Stage = "retrieve"
		j.Progress = 20
	})
	s.broadcast(jobID, "task_planned", map[string]interface{}{"count": len(tasks)})

	if len(tasks) == 0 {
		s.failJob(jobID, "no tasks generated")
		return
	}

	type taskEvidence struct {
		taskID string
		query  string
		hits   []SearchHit
		err    error
	}
	results := make(chan taskEvidence, len(tasks))

	var wg sync.WaitGroup
	for _, t := range tasks {
		task := t
		wg.Add(1)
		go func() {
			defer wg.Done()
			hits, err := s.searcher.Search(ctx, task.Question, searchResultsPerTask(job.Mode))
			results <- taskEvidence{taskID: task.ID, query: task.Question, hits: hits, err: err}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	seenURLs := make(map[string]struct{})
	evidence := make([]Evidence, 0)
	taskErrors := 0

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				s.failJob(jobID, "job timed out")
			} else {
				s.cancelJobInternal(jobID)
			}
			return
		case r, ok := <-results:
			if !ok {
				goto done
			}
			if r.err != nil {
				taskErrors++
				continue
			}
			for idx, hit := range r.hits {
				if hit.URL == "" {
					continue
				}
				if _, ok := seenURLs[hit.URL]; ok {
					continue
				}
				if len(evidence) >= job.Budget.MaxSources {
					break
				}
				seenURLs[hit.URL] = struct{}{}
				ev := Evidence{
					ID:               uuid.NewString(),
					TaskID:           r.taskID,
					Query:            r.query,
					Title:            hit.Title,
					URL:              hit.URL,
					Snippet:          hit.Description,
					Source:           hit.Source,
					Domain:           extractDomain(hit.URL),
					FetchedAt:        timeutil.NowTime(),
					RelevanceScore:   scoreByRank(idx),
					CredibilityScore: scoreByDomain(extractDomain(hit.URL)),
					NoveltyScore:     0.5,
				}
				evidence = append(evidence, ev)
				s.broadcast(jobID, "evidence_added", ev)
			}
			s.updateJob(jobID, func(j *Job) {
				j.Progress = minInt(80, j.Progress+10)
			})
		}
	}

done:
	if len(evidence) == 0 && taskErrors > 0 {
		s.failJob(jobID, "all search tasks failed")
		return
	}

	s.updateJob(jobID, func(j *Job) {
		j.Status = JobStatusSynthesizing
		j.Stage = "synthesize"
		j.Progress = 90
		j.Evidence = evidence
	})

	report := synthesizeReport(job.Query, evidence)
	s.updateJob(jobID, func(j *Job) {
		j.Status = JobStatusCompleted
		j.Stage = "completed"
		j.Progress = 100
		j.Report = &report
		now := timeutil.NowTime()
		j.CompletedAt = &now
	})
	s.broadcast(jobID, "job_completed", map[string]interface{}{"evidence_count": len(evidence), "confidence": report.Confidence})
}

func (s *Service) failJob(jobID, msg string) {
	s.updateJob(jobID, func(j *Job) {
		j.Status = JobStatusFailed
		j.Stage = "failed"
		j.Error = msg
		now := timeutil.NowTime()
		j.CompletedAt = &now
	})
	s.broadcast(jobID, "job_failed", map[string]interface{}{"error": msg})
}

func (s *Service) cancelJobInternal(jobID string) {
	s.updateJob(jobID, func(j *Job) {
		j.Status = JobStatusCancelled
		j.Stage = "cancelled"
		now := timeutil.NowTime()
		j.CompletedAt = &now
	})
	s.broadcast(jobID, "job_cancelled", map[string]interface{}{"job_id": jobID})
}

func (s *Service) getJobPtr(jobID string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[jobID]
	return j, ok
}

func (s *Service) updateJob(jobID string, fn func(*Job)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[jobID]
	if !ok {
		return false
	}
	fn(j)
	j.UpdatedAt = timeutil.NowTime()
	return true
}

func (s *Service) broadcast(jobID, eventType string, payload interface{}) {
	ev := Event{
		Type:      eventType,
		Timestamp: timeutil.NowTime(),
		Payload:   payload,
	}

	s.mu.RLock()
	subs := s.subscribers[jobID]
	targets := make([]chan Event, 0, len(subs))
	for ch := range subs {
		targets = append(targets, ch)
	}
	s.mu.RUnlock()

	for _, ch := range targets {
		select {
		case ch <- ev:
		default:
		}
	}
}

func defaultBudget(mode Mode) Budget {
	switch mode {
	case ModeFast:
		return Budget{MaxSteps: 6, MaxSources: 8, MaxSeconds: 20}
	case ModeDeep:
		return Budget{MaxSteps: 24, MaxSources: 60, MaxSeconds: 180}
	default:
		return Budget{MaxSteps: 12, MaxSources: 16, MaxSeconds: 60}
	}
}

func searchResultsPerTask(mode Mode) int {
	switch mode {
	case ModeFast:
		return fastSearchResultsPerTask
	case ModeDeep:
		return deepResearchResultsPerTask
	default:
		return standardSearchResultsPerTask
	}
}

func scoreByRank(rank int) float64 {
	switch {
	case rank <= 1:
		return 0.92
	case rank <= 3:
		return 0.82
	default:
		return 0.7
	}
}

func scoreByDomain(domain string) float64 {
	switch {
	case strings.HasSuffix(domain, ".gov"), strings.HasSuffix(domain, ".edu"):
		return 0.95
	case strings.Contains(domain, "wikipedia.org"), strings.Contains(domain, "github.com"):
		return 0.85
	case domain == "":
		return 0.5
	default:
		return 0.7
	}
}

func synthesizeReport(query string, evidence []Evidence) Report {
	if len(evidence) == 0 {
		return Report{
			Answer:     "No sufficient evidence was collected.",
			Confidence: 0.0,
		}
	}

	sort.SliceStable(evidence, func(i, j int) bool {
		if evidence[i].RelevanceScore == evidence[j].RelevanceScore {
			return evidence[i].CredibilityScore > evidence[j].CredibilityScore
		}
		return evidence[i].RelevanceScore > evidence[j].RelevanceScore
	})

	citations := make([]Citation, 0, len(evidence))
	lines := make([]string, 0, len(evidence)+2)
	lines = append(lines, fmt.Sprintf("Research summary for: %s", query))
	lines = append(lines, "")
	for i, ev := range evidence {
		citations = append(citations, Citation{
			EvidenceID: ev.ID,
			Title:      ev.Title,
			URL:        ev.URL,
		})
		label := ev.Title
		if label == "" {
			label = ev.URL
		}
		lines = append(lines, fmt.Sprintf("%d. %s (%s)", i+1, label, ev.Domain))
		if ev.Snippet != "" {
			lines = append(lines, fmt.Sprintf("   - %s", ev.Snippet))
		}
	}

	conf := confidenceScore(evidence)
	return Report{
		Answer:     strings.Join(lines, "\n"),
		Confidence: conf,
		Citations:  citations,
		OpenQuestions: []string{
			"Check primary sources for final verification.",
		},
	}
}

func confidenceScore(evidence []Evidence) float64 {
	if len(evidence) == 0 {
		return 0
	}
	domains := make(map[string]struct{})
	total := 0.0
	for _, ev := range evidence {
		total += ev.RelevanceScore*0.6 + ev.CredibilityScore*0.4
		if ev.Domain != "" {
			domains[ev.Domain] = struct{}{}
		}
	}
	base := total / float64(len(evidence))
	diversityBoost := float64(len(domains)) / float64(maxInt(1, len(evidence)))
	score := base*0.85 + diversityBoost*0.15
	if score > 0.99 {
		score = 0.99
	}
	return score
}

func cloneJob(j *Job) *Job {
	cp := *j
	if j.Tasks != nil {
		cp.Tasks = append([]Task(nil), j.Tasks...)
	}
	if j.Evidence != nil {
		cp.Evidence = append([]Evidence(nil), j.Evidence...)
	}
	if j.Report != nil {
		r := *j.Report
		if j.Report.Citations != nil {
			r.Citations = append([]Citation(nil), j.Report.Citations...)
		}
		if j.Report.OpenQuestions != nil {
			r.OpenQuestions = append([]string(nil), j.Report.OpenQuestions...)
		}
		cp.Report = &r
	}
	return &cp
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
