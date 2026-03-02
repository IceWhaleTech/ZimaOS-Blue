package deepresearch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type Service struct {
	planner  Planner
	searcher Searcher
	summary  SummarySynthesizer

	mu           sync.RWMutex
	jobs         map[string]*Job
	cancelFuncs  map[string]context.CancelFunc
	subscribers  map[string]map[chan Event]struct{}
	lastTerminal map[string]Event

	userActiveJobs       map[string]int
	userCreateWindow     map[string][]time.Time
	maxConcurrentPerUser int
	maxCreatesPerWindow  int
	createRateWindow     time.Duration

	searchCacheMu         sync.Mutex
	searchCache           map[string]cachedSearchResult
	searchInFlight        map[string]*inflightSearchCall
	searchCacheTTL        time.Duration
	searchCacheMaxEntries int

	searchPerfMu    sync.Mutex
	searchPerf      [deepResearchSearchPerfWindow]searchPerfSample
	searchPerfIdx   int
	searchPerfCount int
}

const (
	fastSearchResultsPerTask         = 6
	standardSearchResultsPerTask     = 8
	deepResearchResultsPerTask       = 12
	deepResearchCompletedJobTTL      = 30 * time.Minute
	deepResearchMaxStoredJobs        = 256
	deepResearchSearchCacheTTL       = 3 * time.Minute
	deepResearchSearchCacheMax       = 512
	deepResearchSearchPerfWindow     = 32
	deepResearchSearchPerfMinN       = 8
	deepResearchMaxConcurrentPerUser = 3
	deepResearchMaxCreatesPerWindow  = 8
	deepResearchCreateRateWindow     = time.Minute
)

var (
	ErrJobNotFound        = errors.New("job not found")
	ErrJobForbidden       = errors.New("job forbidden")
	ErrReportNotReady     = errors.New("report not ready")
	ErrJobAlreadyTerminal = errors.New("job already terminal")
	ErrRateLimited        = errors.New("deep research rate limit exceeded")
	ErrTooManyActiveJobs  = errors.New("too many deep research jobs in progress")
	eventCounter          uint64
)

type SummaryInput struct {
	Query    string
	Lang     string
	Evidence []Evidence
	Draft    string
}

type SummarySynthesizer interface {
	Summarize(ctx context.Context, input SummaryInput) (string, error)
}

type cachedSearchResult struct {
	hits      []SearchHit
	expiresAt time.Time
	updatedAt time.Time
}

type inflightSearchCall struct {
	done chan struct{}
	hits []SearchHit
	err  error
}

type searchPerfSample struct {
	latency time.Duration
	success bool
	timeout bool
}

func NewService(planner Planner, searcher Searcher) *Service {
	if planner == nil {
		planner = NewHeuristicPlanner()
	}
	if searcher == nil {
		searcher = NewToolWebSearcher()
	}
	return &Service{
		planner:               planner,
		searcher:              searcher,
		jobs:                  make(map[string]*Job),
		cancelFuncs:           make(map[string]context.CancelFunc),
		subscribers:           make(map[string]map[chan Event]struct{}),
		lastTerminal:          make(map[string]Event),
		userActiveJobs:        make(map[string]int),
		userCreateWindow:      make(map[string][]time.Time),
		maxConcurrentPerUser:  deepResearchMaxConcurrentPerUser,
		maxCreatesPerWindow:   deepResearchMaxCreatesPerWindow,
		createRateWindow:      deepResearchCreateRateWindow,
		searchCache:           make(map[string]cachedSearchResult),
		searchInFlight:        make(map[string]*inflightSearchCall),
		searchCacheTTL:        deepResearchSearchCacheTTL,
		searchCacheMaxEntries: deepResearchSearchCacheMax,
	}
}

func (s *Service) SetSummarySynthesizer(synth SummarySynthesizer) {
	s.summary = synth
}

func (s *Service) CreateJob(ctx context.Context, req CreateJobRequest) (*Job, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	userID := normalizeActorID(req.UserID)
	tenantID := strings.TrimSpace(req.TenantID)

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
	budget = clampBudget(mode, budget)

	now := timeutil.NowTime()
	job := &Job{
		ID:        uuid.NewString(),
		UserID:    userID,
		TenantID:  tenantID,
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
	if err := s.reserveUserCreateSlotLocked(userID, now); err != nil {
		s.mu.Unlock()
		cancel()
		return nil, err
	}
	s.jobs[job.ID] = job
	s.cancelFuncs[job.ID] = cancel
	s.pruneJobsLocked(now)
	s.mu.Unlock()

	go s.runJob(runCtx, job.ID)

	return cloneJob(job), nil
}

func (s *Service) GetJob(id string) (*Job, error) {
	return s.GetJobForUser(id, "", "")
}

func (s *Service) GetReport(id string) (*Report, error) {
	return s.GetReportForUser(id, "", "")
}

func (s *Service) GetJobForUser(id, userID, tenantID string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	if !isAllowedActor(job, userID, tenantID) {
		return nil, ErrJobForbidden
	}
	return cloneJob(job), nil
}

func (s *Service) GetReportForUser(id, userID, tenantID string) (*Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	if !isAllowedActor(job, userID, tenantID) {
		return nil, ErrJobForbidden
	}
	if job.Report == nil {
		return nil, ErrReportNotReady
	}
	rep := *job.Report
	return &rep, nil
}

func (s *Service) CancelJob(id string) error {
	return s.CancelJobForUser(id, "", "")
}

func (s *Service) CancelJobForUser(id, userID, tenantID string) error {
	s.mu.RLock()
	job, ok := s.jobs[id]
	if !ok {
		s.mu.RUnlock()
		return ErrJobNotFound
	}
	if !isAllowedActor(job, userID, tenantID) {
		s.mu.RUnlock()
		return ErrJobForbidden
	}
	if isTerminalStatus(job.Status) {
		s.mu.RUnlock()
		return ErrJobAlreadyTerminal
	}
	cancel := s.cancelFuncs[id]
	s.mu.RUnlock()
	if cancel == nil {
		return ErrJobNotFound
	}
	cancel()
	return nil
}

func (s *Service) Subscribe(jobID string) (<-chan Event, func(), error) {
	return s.SubscribeForUser(jobID, "", "")
}

func (s *Service) SubscribeForUser(jobID, userID, tenantID string) (<-chan Event, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return nil, nil, ErrJobNotFound
	}
	if !isAllowedActor(job, userID, tenantID) {
		return nil, nil, ErrJobForbidden
	}

	ch := make(chan Event, 32)
	if s.subscribers[jobID] == nil {
		s.subscribers[jobID] = make(map[chan Event]struct{})
	}
	s.subscribers[jobID][ch] = struct{}{}
	ch <- Event{
		ID:        nextEventID(),
		Type:      "job_snapshot",
		Timestamp: timeutil.NowTime(),
		Payload:   cloneJob(job),
	}
	if termEv, ok := s.lastTerminal[jobID]; ok {
		ch <- termEv
	}

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
		if job, ok := s.jobs[jobID]; ok {
			s.releaseUserSlotLocked(normalizeActorID(job.UserID))
		}
		delete(s.cancelFuncs, jobID)
		s.pruneJobsLocked(timeutil.NowTime())
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

	tasks := s.planner.Plan(job.Query, job.Mode, job.Lang)
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
	searchCtx, searchCancel := context.WithCancel(ctx)
	defer searchCancel()
	tasksCh := make(chan Task)
	workerCount := maxInt(1, minInt(len(tasks), s.searchParallelismForMode(job.Mode)))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasksCh {
				hits, err := s.searchWithCache(searchCtx, task.Question, searchResultsPerTask(job.Mode), job.Lang)
				results <- taskEvidence{taskID: task.ID, query: task.Question, hits: hits, err: err}
			}
		}()
	}

	go func() {
		defer close(tasksCh)
		for _, t := range tasks {
			task := t
			select {
			case <-searchCtx.Done():
				return
			case tasksCh <- task:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	seenURLs := make(map[string]struct{})
	evidence := make([]Evidence, 0)
	taskErrors := 0
	maxSourcesReached := false

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
				if errors.Is(r.err, context.Canceled) || errors.Is(r.err, context.DeadlineExceeded) {
					continue
				}
				taskErrors++
				continue
			}
			if maxSourcesReached {
				continue
			}
			for idx, hit := range r.hits {
				canonicalURL := canonicalizeURL(hit.URL)
				if canonicalURL == "" {
					continue
				}
				if _, ok := seenURLs[canonicalURL]; ok {
					continue
				}
				if len(evidence) >= job.Budget.MaxSources {
					maxSourcesReached = true
					searchCancel()
					break
				}
				seenURLs[canonicalURL] = struct{}{}
				domain := extractDomain(canonicalURL)
				ev := Evidence{
					ID:               uuid.NewString(),
					TaskID:           r.taskID,
					Query:            r.query,
					Title:            hit.Title,
					URL:              canonicalURL,
					Snippet:          hit.Description,
					Source:           hit.Source,
					Domain:           domain,
					FetchedAt:        timeutil.NowTime(),
					RelevanceScore:   scoreByRank(idx),
					CredibilityScore: scoreByDomain(domain),
					NoveltyScore:     0.5,
				}
				evidence = append(evidence, ev)
				s.broadcast(jobID, "evidence_added", ev)
				if len(evidence) >= job.Budget.MaxSources {
					maxSourcesReached = true
					searchCancel()
					break
				}
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

	report := s.synthesizeReport(ctx, job.Query, job.Lang, evidence)
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
		ID:        nextEventID(),
		Type:      eventType,
		Timestamp: timeutil.NowTime(),
		Payload:   payload,
	}
	terminal := isTerminalEventType(eventType)

	s.mu.Lock()
	defer s.mu.Unlock()
	if terminal {
		// Store terminal event for late subscribers.
		s.lastTerminal[jobID] = ev
	}
	for ch := range s.subscribers[jobID] {
		if !terminal {
			select {
			case ch <- ev:
			default:
			}
			continue
		}
		// Terminal event is force-enqueued by dropping stale buffered events first.
		for {
			select {
			case ch <- ev:
				goto nextSub
			default:
				select {
				case <-ch:
				default:
				}
			}
		}
	nextSub:
	}
}

func (s *Service) synthesizeReport(ctx context.Context, query, lang string, evidence []Evidence) Report {
	report := synthesizeReport(query, lang, evidence)
	if s.summary == nil {
		return report
	}

	answer, err := s.summary.Summarize(ctx, SummaryInput{
		Query:    query,
		Lang:     lang,
		Evidence: append([]Evidence(nil), evidence...),
		Draft:    report.Answer,
	})
	if err != nil {
		return report
	}
	answer = strings.TrimSpace(answer)
	if answer != "" {
		report.Answer = answer
	}
	return report
}

func (s *Service) searchWithCache(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
	key := searchCacheKey(query, maxResults, lang)
	now := timeutil.NowTime()

	s.searchCacheMu.Lock()
	s.pruneSearchCacheLocked(now)
	if cached, ok := s.searchCache[key]; ok && now.Before(cached.expiresAt) {
		hits := cloneSearchHits(cached.hits)
		s.searchCacheMu.Unlock()
		return hits, nil
	}
	if call, ok := s.searchInFlight[key]; ok {
		wait := call.done
		s.searchCacheMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-wait:
		}
		if call.err != nil {
			return nil, call.err
		}
		return cloneSearchHits(call.hits), nil
	}

	call := &inflightSearchCall{done: make(chan struct{})}
	s.searchInFlight[key] = call
	s.searchCacheMu.Unlock()

	hits, err := s.searcher.Search(ctx, query, maxResults, lang)
	s.recordSearchOutcome(timeutil.SinceTime(now), err)
	completedAt := timeutil.NowTime()

	s.searchCacheMu.Lock()
	delete(s.searchInFlight, key)
	if err == nil {
		copied := cloneSearchHits(hits)
		call.hits = copied
		call.err = nil
		ttl := s.searchCacheTTL
		if ttl <= 0 {
			ttl = deepResearchSearchCacheTTL
		}
		s.searchCache[key] = cachedSearchResult{
			hits:      cloneSearchHits(copied),
			expiresAt: completedAt.Add(ttl),
			updatedAt: completedAt,
		}
		s.pruneSearchCacheLocked(completedAt)
	} else {
		call.err = err
	}
	close(call.done)
	s.searchCacheMu.Unlock()

	if err != nil {
		return nil, err
	}
	return cloneSearchHits(hits), nil
}

func searchCacheKey(query string, maxResults int, lang string) string {
	return fmt.Sprintf("%s|%d|%s",
		strings.TrimSpace(query),
		maxInt(1, maxResults),
		strings.ToLower(strings.TrimSpace(lang)),
	)
}

func cloneSearchHits(hits []SearchHit) []SearchHit {
	if len(hits) == 0 {
		return nil
	}
	cp := make([]SearchHit, len(hits))
	copy(cp, hits)
	return cp
}

func (s *Service) pruneSearchCacheLocked(now time.Time) {
	for key, entry := range s.searchCache {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(s.searchCache, key)
		}
	}

	limit := s.searchCacheMaxEntries
	if limit <= 0 {
		limit = deepResearchSearchCacheMax
	}
	if len(s.searchCache) <= limit {
		return
	}

	type cacheEvictCandidate struct {
		key string
		ts  time.Time
	}
	candidates := make([]cacheEvictCandidate, 0, len(s.searchCache))
	for key, entry := range s.searchCache {
		candidates = append(candidates, cacheEvictCandidate{key: key, ts: entry.updatedAt})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ts.Before(candidates[j].ts)
	})

	excess := len(s.searchCache) - limit
	for i := 0; i < excess && i < len(candidates); i++ {
		delete(s.searchCache, candidates[i].key)
	}
}

func (s *Service) recordSearchOutcome(latency time.Duration, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	if latency < 0 {
		latency = 0
	}

	sample := searchPerfSample{
		latency: latency,
		success: err == nil,
		timeout: errors.Is(err, context.DeadlineExceeded),
	}

	s.searchPerfMu.Lock()
	s.searchPerf[s.searchPerfIdx] = sample
	s.searchPerfIdx = (s.searchPerfIdx + 1) % deepResearchSearchPerfWindow
	if s.searchPerfCount < deepResearchSearchPerfWindow {
		s.searchPerfCount++
	}
	s.searchPerfMu.Unlock()
}

func (s *Service) searchPerfSnapshot() (timeoutRate float64, avgSuccessLatency time.Duration, sampleCount int) {
	s.searchPerfMu.Lock()
	defer s.searchPerfMu.Unlock()

	if s.searchPerfCount == 0 {
		return 0, 0, 0
	}

	total := s.searchPerfCount
	timeouts := 0
	successes := 0
	var successLatencySum time.Duration
	for i := 0; i < total; i++ {
		sm := s.searchPerf[i]
		if sm.timeout {
			timeouts++
		}
		if sm.success {
			successes++
			successLatencySum += sm.latency
		}
	}

	timeoutRate = float64(timeouts) / float64(total)
	if successes > 0 {
		avgSuccessLatency = successLatencySum / time.Duration(successes)
	}
	return timeoutRate, avgSuccessLatency, total
}

func (s *Service) searchParallelismForMode(mode Mode) int {
	base := searchParallelism(mode)
	timeoutRate, avgSuccessLatency, n := s.searchPerfSnapshot()
	if n < deepResearchSearchPerfMinN {
		return base
	}

	switch {
	case timeoutRate >= 0.50:
		return maxInt(1, base-2)
	case timeoutRate >= 0.30:
		return maxInt(1, base-1)
	case timeoutRate == 0 && avgSuccessLatency > 0 && avgSuccessLatency <= 250*time.Millisecond:
		return minInt(searchParallelismCeiling(mode), base+1)
	default:
		return base
	}
}

func searchParallelismCeiling(mode Mode) int {
	switch mode {
	case ModeFast:
		return 3
	case ModeDeep:
		return 6
	default:
		return 4
	}
}

func isTerminalStatus(status JobStatus) bool {
	return status == JobStatusCompleted || status == JobStatusFailed || status == JobStatusCancelled
}

func terminalJobSortTime(j *Job) time.Time {
	if j == nil {
		return time.Time{}
	}
	if j.CompletedAt != nil {
		return *j.CompletedAt
	}
	if !j.UpdatedAt.IsZero() {
		return j.UpdatedAt
	}
	return j.CreatedAt
}

func (s *Service) removeJobLocked(jobID string) {
	if subs := s.subscribers[jobID]; len(subs) > 0 {
		for ch := range subs {
			close(ch)
		}
		delete(s.subscribers, jobID)
	}
	delete(s.lastTerminal, jobID)
	delete(s.cancelFuncs, jobID)
	delete(s.jobs, jobID)
}

func (s *Service) pruneJobsLocked(now time.Time) {
	// Time-based GC for terminal jobs.
	for id, job := range s.jobs {
		if !isTerminalStatus(job.Status) {
			continue
		}
		ts := terminalJobSortTime(job)
		if ts.IsZero() || now.Sub(ts) <= deepResearchCompletedJobTTL {
			continue
		}
		s.removeJobLocked(id)
	}

	if len(s.jobs) <= deepResearchMaxStoredJobs {
		return
	}

	// Size-based eviction: remove oldest terminal jobs first.
	type evictCandidate struct {
		id string
		ts time.Time
	}
	candidates := make([]evictCandidate, 0, len(s.jobs))
	for id, job := range s.jobs {
		if !isTerminalStatus(job.Status) {
			continue
		}
		candidates = append(candidates, evictCandidate{
			id: id,
			ts: terminalJobSortTime(job),
		})
	}
	if len(candidates) == 0 {
		return
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ts.Before(candidates[j].ts)
	})

	excess := len(s.jobs) - deepResearchMaxStoredJobs
	if excess <= 0 {
		return
	}
	if excess > len(candidates) {
		excess = len(candidates)
	}
	for i := 0; i < excess; i++ {
		s.removeJobLocked(candidates[i].id)
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

func clampBudget(mode Mode, budget Budget) Budget {
	def := defaultBudget(mode)

	if budget.MaxSteps <= 0 {
		budget.MaxSteps = def.MaxSteps
	}
	if budget.MaxSources <= 0 {
		budget.MaxSources = def.MaxSources
	}
	if budget.MaxSeconds <= 0 {
		budget.MaxSeconds = def.MaxSeconds
	}

	maxSteps := def.MaxSteps * 2
	maxSources := def.MaxSources * 2
	maxSeconds := def.MaxSeconds * 2

	budget.MaxSteps = minInt(maxSteps, maxInt(2, budget.MaxSteps))
	budget.MaxSources = minInt(maxSources, maxInt(1, budget.MaxSources))
	budget.MaxSeconds = minInt(maxSeconds, maxInt(5, budget.MaxSeconds))
	return budget
}

func normalizeActorID(raw string) string {
	id := strings.ToLower(strings.TrimSpace(raw))
	if id == "" {
		return "anonymous"
	}
	return id
}

func (s *Service) reserveUserCreateSlotLocked(userID string, now time.Time) error {
	userID = normalizeActorID(userID)

	window := s.createRateWindow
	if window <= 0 {
		window = deepResearchCreateRateWindow
	}
	maxCreates := s.maxCreatesPerWindow
	if maxCreates <= 0 {
		maxCreates = deepResearchMaxCreatesPerWindow
	}
	maxConcurrent := s.maxConcurrentPerUser
	if maxConcurrent <= 0 {
		maxConcurrent = deepResearchMaxConcurrentPerUser
	}

	cutoff := now.Add(-window)
	events := s.userCreateWindow[userID]
	keep := events[:0]
	for _, ts := range events {
		if ts.After(cutoff) {
			keep = append(keep, ts)
		}
	}
	if len(keep) >= maxCreates {
		s.userCreateWindow[userID] = keep
		return ErrRateLimited
	}
	if s.userActiveJobs[userID] >= maxConcurrent {
		s.userCreateWindow[userID] = keep
		return ErrTooManyActiveJobs
	}

	keep = append(keep, now)
	s.userCreateWindow[userID] = keep
	s.userActiveJobs[userID] = s.userActiveJobs[userID] + 1
	return nil
}

func (s *Service) releaseUserSlotLocked(userID string) {
	userID = normalizeActorID(userID)
	n := s.userActiveJobs[userID]
	if n <= 1 {
		delete(s.userActiveJobs, userID)
		return
	}
	s.userActiveJobs[userID] = n - 1
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

func searchParallelism(mode Mode) int {
	switch mode {
	case ModeFast:
		return 2
	case ModeDeep:
		return 4
	default:
		return 3
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

func synthesizeReport(query, lang string, evidence []Evidence) Report {
	if len(evidence) == 0 {
		return Report{
			Answer:     localizedNoEvidence(lang, query),
			Confidence: 0.0,
		}
	}

	supportCount, conflictCount, hasConflict := analyzeEvidenceConsistency(evidence)

	sort.SliceStable(evidence, func(i, j int) bool {
		if evidence[i].RelevanceScore == evidence[j].RelevanceScore {
			return evidence[i].CredibilityScore > evidence[j].CredibilityScore
		}
		return evidence[i].RelevanceScore > evidence[j].RelevanceScore
	})

	citations := make([]Citation, 0, len(evidence))
	lines := make([]string, 0, len(evidence)+2)
	lines = append(lines, localizedSummaryTitle(query, lang))
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
	openQuestions := []string{
		localizedOpenQuestion(lang, query),
	}
	if hasConflict {
		openQuestions = append(openQuestions, localizedConflictOpenQuestion(lang, query))
	}
	return Report{
		Answer:        strings.Join(lines, "\n"),
		Confidence:    conf,
		Citations:     citations,
		OpenQuestions: openQuestions,
		SupportCount:  supportCount,
		ConflictCount: conflictCount,
		HasConflict:   hasConflict,
	}
}

func analyzeEvidenceConsistency(evidence []Evidence) (supportCount, conflictCount int, hasConflict bool) {
	conflictMarkers := []string{
		"not", "no", "deny", "denied", "dispute", "conflict", "uncertain", "unconfirmed", "rumor",
		"并非", "不是", "否认", "争议", "矛盾", "未证实", "传闻",
	}

	for _, ev := range evidence {
		text := strings.ToLower(strings.TrimSpace(ev.Title + " " + ev.Snippet))
		if text == "" {
			continue
		}
		isConflict := false
		for _, marker := range conflictMarkers {
			if strings.Contains(text, marker) {
				isConflict = true
				break
			}
		}
		if isConflict {
			conflictCount++
			continue
		}

		supportCount++
	}
	hasConflict = supportCount > 0 && conflictCount > 0
	return supportCount, conflictCount, hasConflict
}

func isAllowedActor(job *Job, userID, tenantID string) bool {
	if job == nil {
		return false
	}
	userID = strings.TrimSpace(strings.ToLower(userID))
	tenantID = strings.TrimSpace(tenantID)

	// Internal calls can use service-level wrappers without actor scope.
	if userID == "" && tenantID == "" {
		return true
	}

	jobUser := strings.TrimSpace(strings.ToLower(job.UserID))
	jobTenant := strings.TrimSpace(job.TenantID)

	if jobUser != "" && userID != jobUser {
		return false
	}
	if jobTenant != "" && tenantID != jobTenant {
		return false
	}
	return true
}

func isTerminalEventType(eventType string) bool {
	switch eventType {
	case "job_completed", "job_failed", "job_cancelled":
		return true
	default:
		return false
	}
}

func nextEventID() string {
	n := atomic.AddUint64(&eventCounter, 1)
	return strconv.FormatUint(n, 10)
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
