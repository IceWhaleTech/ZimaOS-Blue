package deepresearch

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
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
	planner   Planner
	searcher  Searcher
	summary   SummarySynthesizer
	v2Enabled bool
	events    EventPublisher

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
	entityThreshold float64
	httpClient      *http.Client
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
	deepResearchSearchRetryAttempts  = 3
	deepResearchSearchRetryBaseDelay = 200 * time.Millisecond
	deepResearchExtractTopK          = 2
	deepResearchExtractTimeout       = 8 * time.Second
)

var (
	ErrJobNotFound        = errors.New("job not found")
	ErrJobForbidden       = errors.New("job forbidden")
	ErrReportNotReady     = errors.New("report not ready")
	ErrJobAlreadyTerminal = errors.New("job already terminal")
	ErrInvalidRouteMode   = errors.New("invalid route_mode")
	ErrRateLimited        = errors.New("deep research rate limit exceeded")
	ErrTooManyActiveJobs  = errors.New("too many deep research jobs in progress")
	eventCounter          uint64
)

type SummaryInput struct {
	Query               string
	Lang                string
	Evidence            []Evidence
	AllEvidence         []Evidence
	Draft               string
	ResearchTrace       []ResearchTraceEntry
	VerificationSummary *VerificationSummary
}

type SummarySynthesizer interface {
	Summarize(ctx context.Context, input SummaryInput) (string, error)
}

type EventPublisher interface {
	Publish(userID string, eventType string, data any)
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
		v2Enabled:             false,
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
		entityThreshold:       defaultEntityThreshold,
		httpClient: &http.Client{
			Timeout: deepResearchExtractTimeout,
		},
	}
}

func (s *Service) SetRoutePolicy(policy RoutePolicy) {
	_ = policy
}

func (s *Service) SetEventPublisher(publisher EventPublisher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = publisher
}

func (s *Service) SetSummarySynthesizer(synth SummarySynthesizer) {
	s.summary = synth
}

func (s *Service) SetV2Enabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.v2Enabled = enabled
}

func (s *Service) IsV2Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v2Enabled
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
	requestedRouteMode, effectiveRouteMode, routeReason, err := resolveRouteMode(query, req.RouteMode, RoutePolicy{})
	if err != nil {
		return nil, err
	}
	lang := strings.TrimSpace(req.Lang)
	strictEntity := false
	if req.StrictEntity != nil {
		strictEntity = *req.StrictEntity
	} else if looksLikePersonTimelineResearch(query, lang) {
		strictEntity = true
	}
	timeWindows := normalizeTimeWindows(req.TimeWindows)
	reportStyle := normalizeReportStyle(req.ReportStyle)
	if reportStyle == "" {
		if looksLikePersonTimelineResearch(query, lang) {
			reportStyle = "timeline"
		} else if queryWantsKnowledgeBaseStyle(query) {
			reportStyle = "knowledge_base"
		} else {
			reportStyle = "summary"
		}
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
	jobID := strings.TrimSpace(req.RequestedID)
	if jobID == "" {
		jobID = uuid.NewString()
	}
	job := &Job{
		ID:                 jobID,
		ConversationID:     strings.TrimSpace(req.ConversationID),
		UserID:             userID,
		TenantID:           tenantID,
		Query:              query,
		Lang:               lang,
		Mode:               mode,
		RequestedRouteMode: requestedRouteMode,
		EffectiveRouteMode: effectiveRouteMode,
		RouteReason:        routeReason,
		StrictEntity:       strictEntity,
		TimeWindows:        append([]string(nil), timeWindows...),
		ReportStyle:        reportStyle,
		Status:             JobStatusPending,
		Budget:             budget,
		CreatedAt:          now,
		UpdatedAt:          now,
		Stage:              "intake",
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
	s.publishJobEvent("deep_research.job_created", cloneJob(job))

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

func (s *Service) ListJobsForUser(userID, tenantID string, activeOnly bool) ([]JobSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]JobSummary, 0, len(s.jobs))
	for _, job := range s.jobs {
		if !isAllowedActor(job, userID, tenantID) {
			continue
		}
		if activeOnly && isTerminalStatus(job.Status) {
			continue
		}
		jobs = append(jobs, jobSummaryFromJob(job))
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].UpdatedAt.After(jobs[j].UpdatedAt)
	})
	return jobs, nil
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
	return cloneReport(job.Report), nil
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
		j.LatestAction = "augment_query"
	}) {
		return
	}
	s.broadcast(jobID, "job_started", map[string]interface{}{"job_id": jobID})

	job, ok := s.getJobPtr(jobID)
	if !ok {
		return
	}
	useV2 := s.IsV2Enabled()
	brief := buildResearchBrief(job.Query, job.Lang, job.TimeWindows, job.ReportStyle)
	s.broadcast(jobID, "brief_augmented", map[string]interface{}{
		"goal":               brief.Goal,
		"entity":             brief.Entity,
		"time_windows":       brief.TimeWindows,
		"must_verify_claims": brief.MustVerifyClaims,
	})

	tasks := annotateTasksWithBrief(s.planner.Plan(job.Query, job.Mode, job.Lang), brief)
	tasks = dedupeTaskQueries(tasks)
	retrieveStepBudget := maxInt(1, job.Budget.MaxSteps-2) // reserve steps for plan + synthesize
	if len(tasks) > retrieveStepBudget {
		tasks = tasks[:retrieveStepBudget]
	}
	if len(tasks) == 0 {
		s.failJob(jobID, "no tasks generated")
		return
	}

	taskByID := make(map[string]Task, len(tasks))
	for _, task := range tasks {
		taskByID[task.ID] = task
	}
	s.updateJob(jobID, func(j *Job) {
		j.Tasks = tasks
		j.Stage = "retrieve"
		j.Progress = 20
		j.Iteration = 1
		j.LatestAction = "initial_retrieve"
	})
	s.broadcast(jobID, "task_planned", map[string]interface{}{
		"count":     len(tasks),
		"iteration": 1,
		"tasks": func() []map[string]interface{} {
			out := make([]map[string]interface{}, 0, len(tasks))
			for _, task := range tasks {
				out = append(out, map[string]interface{}{
					"id":          task.ID,
					"question":    task.Question,
					"axis":        task.Axis,
					"category":    task.Category,
					"time_window": task.TimeWindow,
					"priority":    task.Priority,
					"depth":       task.Depth,
				})
			}
			return out
		}(),
	})

	seenURLs := make(map[string]struct{})
	reportedFiltered := make(map[string]struct{})
	evidence := make([]Evidence, 0)
	stageErrors := make([]string, 0)
	entityStats := EntityDisambiguation{
		Enabled:   false,
		Threshold: s.entityThreshold,
	}
	allFailedQueries := make([]string, 0)
	trace := make([]ResearchTraceEntry, 0)
	var verificationSummary *VerificationSummary
	stopReason := ""
	iteration := 0
	followUpRounds := 0
	remainingSteps := maxInt(0, retrieveStepBudget-len(tasks))
	nextTaskIndex := len(tasks) + 1
	roundTasks := tasks

	for len(roundTasks) > 0 {
		iteration++
		s.updateJob(jobID, func(j *Job) {
			j.Stage = "retrieve"
			j.Progress = minInt(85, 18+iteration*14)
			j.Iteration = iteration
			if iteration > 1 {
				j.LatestAction = "followup_retrieve"
			}
		})

		round := s.collectEvidenceRound(ctx, jobID, roundTasks, job.Mode, job.Lang, useV2, seenURLs, len(evidence), job.Budget.MaxSources, iteration)
		if round.Aborted {
			if errors.Is(round.AbortErr, context.DeadlineExceeded) {
				s.failJob(jobID, "job timed out")
			} else {
				s.cancelJobInternal(jobID)
			}
			return
		}
		if len(evidence) == 0 && len(round.Evidence) == 0 && round.TaskErrors >= len(roundTasks) {
			s.failJob(jobID, "all search tasks failed")
			return
		}
		evidence = append(evidence, round.Evidence...)
		stageErrors = append(stageErrors, round.StageErrors...)
		allFailedQueries = append(allFailedQueries, round.FailedQueries...)

		s.updateJob(jobID, func(j *Job) {
			j.Stage = "verify"
			j.Progress = minInt(88, 24+iteration*15)
			j.Iteration = iteration
			j.LatestAction = "verification"
			j.Evidence = append([]Evidence(nil), evidence...)
		})

		if useV2 {
			original := append([]Evidence(nil), evidence...)
			filteredEvidence, roundEntityStats := applyEntityDisambiguation(job.Query, job.StrictEntity, s.entityThreshold, evidence, taskByID)
			evidence = filteredEvidence
			entityStats.Enabled = roundEntityStats.Enabled
			entityStats.Threshold = roundEntityStats.Threshold
			if roundEntityStats.AmbiguousCount > entityStats.AmbiguousCount {
				entityStats.AmbiguousCount = roundEntityStats.AmbiguousCount
			}
			kept := make(map[string]struct{}, len(evidence))
			for _, ev := range evidence {
				kept[ev.ID] = struct{}{}
			}
			for _, ev := range original {
				if _, ok := kept[ev.ID]; ok {
					continue
				}
				if _, seen := reportedFiltered[ev.ID]; seen {
					continue
				}
				reportedFiltered[ev.ID] = struct{}{}
				s.broadcast(jobID, "entity_filtered", map[string]interface{}{
					"iteration":    iteration,
					"evidence_id":  ev.ID,
					"title":        ev.Title,
					"url":          ev.URL,
					"entity_score": ev.EntityScore,
					"threshold":    entityStats.Threshold,
				})
			}
			entityStats.FilteredCount = len(reportedFiltered)
			if job.StrictEntity && len(evidence) == 0 {
				stageErrors = append(stageErrors, "all evidence filtered by strict entity disambiguation")
				s.broadcast(jobID, "stage_warning", map[string]interface{}{
					"stage":     "verify",
					"message":   "all evidence filtered by strict entity disambiguation",
					"iteration": iteration,
				})
			}
		}

		var gaps []researchGap
		verificationSummary, gaps = buildVerificationSummary(job.Query, job.Lang, brief, evidence, tasks)
		unresolved := unresolvedResearchGaps(gaps)
		primaryGap := primaryResearchGap(unresolved)
		verificationPayload := map[string]interface{}{
			"iteration":          iteration,
			"resolved_count":     0,
			"conflicted_count":   0,
			"insufficient_count": 0,
			"latest_gap":         primaryGap.Gap,
			"latest_action":      "verification_completed",
		}
		if verificationSummary != nil {
			verificationPayload["resolved_count"] = verificationSummary.ResolvedCount
			verificationPayload["conflicted_count"] = verificationSummary.ConflictedCount
			verificationPayload["insufficient_count"] = verificationSummary.InsufficientCount
		}
		s.updateJob(jobID, func(j *Job) {
			j.Stage = "verify"
			j.Iteration = iteration
			j.LatestGap = primaryGap.Gap
			j.LatestAction = "verification_completed"
			j.Evidence = append([]Evidence(nil), evidence...)
		})
		s.broadcast(jobID, "verification_completed", verificationPayload)

		draft := synthesizeReportWithOptions(job.Query, job.Lang, append([]Evidence(nil), evidence...), reportBuildOptions{
			StrictEntity:         job.StrictEntity,
			TimeWindows:          append([]string(nil), job.TimeWindows...),
			ReportStyle:          job.ReportStyle,
			StageErrors:          dedupeStrings(stageErrors),
			EntityDisambiguation: &entityStats,
			UseTimelineStyle:     useV2,
			Iterations:           iteration,
			ResearchTrace:        trace,
			VerificationSummary:  verificationSummary,
			Tasks:                append([]Task(nil), tasks...),
		})

		roundOutcome := verificationOutcome(verificationSummary)
		roundEvidenceAdded := len(round.Evidence)
		if roundEvidenceAdded == 0 {
			stopReason = "no_new_canonical_evidence"
		} else if draft.CitationCoverage >= 0.85 && !hasHighPriorityGap(unresolved) {
			stopReason = "coverage_sufficient"
		} else if round.MaxSourcesReached || remainingSteps < 2 || followUpRounds >= maxFollowUpRounds(job.Mode) {
			stopReason = "budget_exhausted"
		}

		if stopReason != "" {
			trace = append(trace, ResearchTraceEntry{
				Iteration:           iteration,
				Focus:               firstNonEmpty(primaryGap.Focus, localizedAxisLabel(job.Lang, "research", "Research")),
				Gap:                 firstNonEmpty(primaryGap.Gap, stopReason),
				EvidenceAdded:       roundEvidenceAdded,
				VerificationOutcome: roundOutcome,
			})
			s.updateJob(jobID, func(j *Job) {
				j.Iteration = iteration
				j.LatestGap = primaryGap.Gap
				j.LatestAction = "loop_stopped"
			})
			s.broadcast(jobID, "loop_stopped", map[string]interface{}{
				"iteration":     iteration,
				"stop_reason":   stopReason,
				"latest_gap":    primaryGap.Gap,
				"latest_action": "loop_stopped",
			})
			break
		}

		followUpTasks := buildFollowUpTasks(brief, unresolved, job.Lang, iteration+1, remainingSteps, nextTaskIndex)
		if len(followUpTasks) == 0 {
			stopReason = "budget_exhausted"
			trace = append(trace, ResearchTraceEntry{
				Iteration:           iteration,
				Focus:               firstNonEmpty(primaryGap.Focus, localizedAxisLabel(job.Lang, "research", "Research")),
				Gap:                 firstNonEmpty(primaryGap.Gap, stopReason),
				EvidenceAdded:       roundEvidenceAdded,
				VerificationOutcome: roundOutcome,
			})
			s.broadcast(jobID, "loop_stopped", map[string]interface{}{
				"iteration":     iteration,
				"stop_reason":   stopReason,
				"latest_gap":    primaryGap.Gap,
				"latest_action": "loop_stopped",
			})
			break
		}

		gapByID := make(map[string]researchGap, len(unresolved))
		for _, gap := range unresolved {
			gapByID[gap.ID] = gap
			s.broadcast(jobID, "gap_detected", map[string]interface{}{
				"iteration": iteration,
				"focus":     gap.Focus,
				"gap":       gap.Gap,
				"status":    gap.Status,
				"priority":  gap.Priority,
			})
		}
		for _, task := range followUpTasks {
			tasks = append(tasks, task)
			taskByID[task.ID] = task
			nextTaskIndex++
			gap := gapByID[task.FollowUpOf]
			trace = append(trace, ResearchTraceEntry{
				Iteration:           iteration,
				Focus:               firstNonEmpty(gap.Focus, task.Axis),
				Gap:                 gap.Gap,
				FollowUpQuery:       task.Question,
				EvidenceAdded:       roundEvidenceAdded,
				VerificationOutcome: roundOutcome,
			})
			s.broadcast(jobID, "followup_planned", map[string]interface{}{
				"iteration":       iteration + 1,
				"focus":           firstNonEmpty(gap.Focus, task.Axis),
				"gap":             gap.Gap,
				"follow_up_query": task.Question,
				"axis":            task.Axis,
			})
		}
		remainingSteps -= len(followUpTasks)
		followUpRounds++
		roundTasks = followUpTasks
		s.updateJob(jobID, func(j *Job) {
			j.Tasks = append([]Task(nil), tasks...)
			j.Stage = "planning"
			j.Progress = minInt(89, 28+iteration*15)
			j.Iteration = iteration + 1
			j.LatestGap = primaryGap.Gap
			j.LatestAction = "followup_planned"
			j.Evidence = append([]Evidence(nil), evidence...)
		})
	}

	if len(allFailedQueries) > 0 {
		stageErrors = append(stageErrors, fmt.Sprintf("failed_queries=%d", len(dedupeStrings(allFailedQueries))))
	}
	stageErrors = dedupeStrings(stageErrors)
	if stopReason == "" {
		stopReason = "coverage_sufficient"
	}

	s.updateJob(jobID, func(j *Job) {
		j.Status = JobStatusSynthesizing
		j.Stage = "synthesize"
		j.Progress = 90
		j.Iteration = iteration
		j.LatestAction = "synthesizing"
		j.Evidence = evidence
	})

	report := s.synthesizeReport(ctx, job.Query, job.Lang, evidence, reportBuildOptions{
		StrictEntity:         job.StrictEntity,
		TimeWindows:          append([]string(nil), job.TimeWindows...),
		ReportStyle:          job.ReportStyle,
		StageErrors:          stageErrors,
		EntityDisambiguation: &entityStats,
		UseTimelineStyle:     useV2,
		Iterations:           iteration,
		StopReason:           stopReason,
		ResearchTrace:        trace,
		VerificationSummary:  verificationSummary,
		Tasks:                append([]Task(nil), tasks...),
	})
	s.broadcast(jobID, "citation_coverage_updated", map[string]interface{}{
		"citation_coverage": report.CitationCoverage,
		"evidence_count":    len(evidence),
		"iteration":         iteration,
	})
	if report.CitationCoverage < 0.8 {
		s.broadcast(jobID, "stage_warning", map[string]interface{}{
			"stage":             "synthesize",
			"message":           "citation coverage below threshold",
			"citation_coverage": report.CitationCoverage,
		})
	}

	s.updateJob(jobID, func(j *Job) {
		j.Status = JobStatusCompleted
		j.Stage = "completed"
		j.Progress = 100
		j.Iteration = iteration
		j.LatestAction = "completed"
		j.Report = &report
		now := timeutil.NowTime()
		j.CompletedAt = &now
	})
	s.broadcast(jobID, "job_completed", map[string]interface{}{
		"iterations":        report.Iterations,
		"stop_reason":       report.StopReason,
		"evidence_count":    len(evidence),
		"confidence":        report.Confidence,
		"citation_coverage": report.CitationCoverage,
	})
	s.publishJobEventByID("deep_research.job_completed", jobID)
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
	s.publishJobEventByID("deep_research.job_failed", jobID)
}

func (s *Service) cancelJobInternal(jobID string) {
	s.updateJob(jobID, func(j *Job) {
		j.Status = JobStatusCancelled
		j.Stage = "cancelled"
		now := timeutil.NowTime()
		j.CompletedAt = &now
	})
	s.broadcast(jobID, "job_cancelled", map[string]interface{}{"job_id": jobID})
	s.publishJobEventByID("deep_research.job_cancelled", jobID)
}

func (s *Service) getJobPtr(jobID string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[jobID]
	return j, ok
}

func (s *Service) updateJob(jobID string, fn func(*Job)) bool {
	s.mu.Lock()
	j, ok := s.jobs[jobID]
	if !ok {
		s.mu.Unlock()
		return false
	}
	fn(j)
	j.UpdatedAt = timeutil.NowTime()
	snapshot := cloneJob(j)
	s.mu.Unlock()
	if snapshot != nil && !isTerminalStatus(snapshot.Status) {
		s.publishJobEvent("deep_research.job_updated", snapshot)
	}
	return true
}

func (s *Service) publishJobEventByID(eventType, jobID string) {
	if strings.TrimSpace(jobID) == "" {
		return
	}
	if snapshot, err := s.GetJob(jobID); err == nil {
		s.publishJobEvent(eventType, snapshot)
	}
}

func (s *Service) publishJobEvent(eventType string, job *Job) {
	if job == nil {
		return
	}
	s.mu.RLock()
	publisher := s.events
	s.mu.RUnlock()
	if publisher == nil || strings.TrimSpace(job.UserID) == "" {
		return
	}
	publisher.Publish(job.UserID, eventType, map[string]interface{}{
		"id":              job.ID,
		"job_id":          job.ID,
		"query":           job.Query,
		"status":          job.Status,
		"stage":           job.Stage,
		"progress":        job.Progress,
		"iteration":       job.Iteration,
		"latest_action":   job.LatestAction,
		"latest_gap":      job.LatestGap,
		"conversation_id": job.ConversationID,
		"updated_at":      job.UpdatedAt,
	})
}

func jobSummaryFromJob(job *Job) JobSummary {
	if job == nil {
		return JobSummary{}
	}
	return JobSummary{
		ID:             job.ID,
		JobID:          job.ID,
		Query:          job.Query,
		Status:         job.Status,
		Stage:          job.Stage,
		Progress:       job.Progress,
		Iteration:      job.Iteration,
		LatestAction:   job.LatestAction,
		LatestGap:      job.LatestGap,
		ConversationID: job.ConversationID,
		UpdatedAt:      job.UpdatedAt,
	}
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

type reportBuildOptions struct {
	StrictEntity         bool
	TimeWindows          []string
	ReportStyle          string
	StageErrors          []string
	EntityDisambiguation *EntityDisambiguation
	UseTimelineStyle     bool
	Iterations           int
	StopReason           string
	ResearchTrace        []ResearchTraceEntry
	VerificationSummary  *VerificationSummary
	Tasks                []Task
}

func (s *Service) synthesizeReport(ctx context.Context, query, lang string, evidence []Evidence, opts ...reportBuildOptions) Report {
	buildOpts := reportBuildOptions{}
	if len(opts) > 0 {
		buildOpts = opts[0]
	}
	report := synthesizeReportWithOptions(query, lang, evidence, buildOpts)
	if s.summary == nil {
		return report
	}
	activeEvidence := topEvidenceWorkset(evidence, deepResearchActiveEvidenceLimit)
	if len(activeEvidence) == 0 {
		activeEvidence = append([]Evidence(nil), evidence...)
	}

	answer, err := s.summary.Summarize(ctx, SummaryInput{
		Query:               query,
		Lang:                lang,
		Evidence:            append([]Evidence(nil), activeEvidence...),
		AllEvidence:         append([]Evidence(nil), evidence...),
		Draft:               report.Answer,
		ResearchTrace:       append([]ResearchTraceEntry(nil), buildOpts.ResearchTrace...),
		VerificationSummary: cloneVerificationSummary(buildOpts.VerificationSummary),
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

func (s *Service) searchWithRetry(ctx context.Context, jobID, query string, maxResults int, lang string) ([]SearchHit, error) {
	var lastErr error
	for attempt := 1; attempt <= deepResearchSearchRetryAttempts; attempt++ {
		hits, err := s.searchWithCache(ctx, query, maxResults, lang)
		if err == nil {
			return hits, nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if attempt >= deepResearchSearchRetryAttempts {
			break
		}
		backoff := deepResearchSearchRetryBaseDelay * time.Duration(1<<(attempt-1))
		s.broadcast(jobID, "search_retry", map[string]interface{}{
			"query":   query,
			"attempt": attempt + 1,
			"delayMs": backoff.Milliseconds(),
			"error":   err.Error(),
		})
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
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
	return synthesizeReportWithOptions(query, lang, evidence, reportBuildOptions{})
}

func synthesizeReportWithOptions(query, lang string, evidence []Evidence, opts reportBuildOptions) Report {
	if len(evidence) == 0 {
		return Report{
			Answer:      localizedNoEvidence(lang, query),
			Confidence:  0.0,
			StageErrors: append([]string(nil), opts.StageErrors...),
		}
	}

	supportCount, conflictCount, hasConflict := analyzeEvidenceConsistencyByClaim(evidence)

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
	indexByEvidenceID := make(map[string]int, len(evidence))
	for i, ev := range evidence {
		citations = append(citations, Citation{
			EvidenceID: ev.ID,
			Title:      ev.Title,
			URL:        ev.URL,
		})
		indexByEvidenceID[ev.ID] = i + 1
		label := ev.Title
		if label == "" {
			label = ev.URL
		}
		lines = append(lines, fmt.Sprintf("%d. %s (%s) [%s#%d]", i+1, label, ev.Domain, localizedSourceLabel(lang, query), i+1))
		if ev.Snippet != "" {
			lines = append(lines, fmt.Sprintf("   - %s", ev.Snippet))
		}
		if ev.Quote != "" {
			lines = append(lines, fmt.Sprintf("   - %s: %s", localizedQuoteLabel(lang, query), ev.Quote))
		}
	}

	timelineSections := buildTimelineSections(evidence, opts.TimeWindows, lang, indexByEvidenceID)
	if opts.UseTimelineStyle || strings.EqualFold(strings.TrimSpace(opts.ReportStyle), "timeline") {
		lines = append(lines, "")
		lines = append(lines, localizedTimelineHeading(lang, query))
		for _, section := range timelineSections {
			if len(section.Highlights) == 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf("### %s", section.Label))
			for _, hl := range section.Highlights {
				lines = append(lines, "- "+hl)
			}
		}
	}

	conf := confidenceScore(evidence)
	openQuestions := []string{
		localizedOpenQuestion(lang, query),
	}
	if hasConflict {
		openQuestions = append(openQuestions, localizedConflictOpenQuestion(lang, query))
	}
	if len(opts.StageErrors) > 0 {
		openQuestions = append(openQuestions, localizedStageErrorHint(lang, query))
	}
	if opts.VerificationSummary != nil {
		for _, item := range opts.VerificationSummary.Items {
			if item.Status == verificationStatusResolved || strings.TrimSpace(item.Gap) == "" {
				continue
			}
			openQuestions = append(openQuestions, item.Gap)
		}
		openQuestions = dedupeStrings(openQuestions)
	}
	if opts.VerificationSummary != nil && len(opts.VerificationSummary.Items) > 0 {
		lines = append(lines, "")
		lines = append(lines, localizedVerificationHeading(lang, query))
		for _, item := range opts.VerificationSummary.Items {
			line := fmt.Sprintf("- %s: %s", localizedVerificationStatusLabel(lang, item.Status), strings.TrimSpace(item.Summary))
			if refs := formatCitationRefs(item.EvidenceIDs, indexByEvidenceID, lang, query); refs != "" {
				line += " " + refs
			}
			lines = append(lines, line)
		}
	}
	answer := strings.Join(lines, "\n")
	if isKnowledgeBaseReportStyle(opts.ReportStyle) {
		answer = buildKnowledgeBaseAnswer(query, lang, evidence, citations, timelineSections, openQuestions, opts.VerificationSummary, indexByEvidenceID)
	}
	coverage := computeCitationCoverage(answer, len(citations))
	calibration := buildCalibration(
		evidence,
		conf,
		coverage,
		supportCount,
		conflictCount,
		hasConflict,
		opts.VerificationSummary,
		opts.StageErrors,
	)
	if calibration != nil && calibration.RecommendedAction != calibrationActionPublish {
		answer = localizedCautiousConclusionPrefix(lang, query) + "\n\n" + answer
	}

	var entityInfo *EntityDisambiguation
	if opts.EntityDisambiguation != nil {
		cp := *opts.EntityDisambiguation
		entityInfo = &cp
	}

	return Report{
		Answer:               answer,
		Confidence:           conf,
		Citations:            citations,
		OpenQuestions:        openQuestions,
		SupportCount:         supportCount,
		ConflictCount:        conflictCount,
		HasConflict:          hasConflict,
		Iterations:           opts.Iterations,
		StopReason:           strings.TrimSpace(opts.StopReason),
		CitationCoverage:     coverage,
		EntityDisambiguation: entityInfo,
		StageErrors:          append([]string(nil), opts.StageErrors...),
		TimelineSections:     timelineSections,
		ResearchTrace:        cloneResearchTrace(opts.ResearchTrace),
		VerificationSummary:  cloneVerificationSummary(opts.VerificationSummary),
		Calibration:          cloneCalibration(calibration),
	}
}

func analyzeEvidenceConsistency(evidence []Evidence) (supportCount, conflictCount int, hasConflict bool) {
	return analyzeEvidenceConsistencyByClaim(evidence)
}

func buildTimelineSections(evidence []Evidence, timeWindows []string, lang string, citationIdx map[string]int) []TimelineSection {
	labels := normalizedTimelineLabels(timeWindows, lang)
	sections := make([]TimelineSection, 0, len(labels))
	for _, label := range labels {
		sections = append(sections, TimelineSection{Label: label})
	}
	if len(sections) == 0 {
		return nil
	}

	for i, ev := range evidence {
		idx := timelineIndexForEvidence(ev, labels)
		if idx < 0 || idx >= len(sections) {
			idx = i % len(sections)
		}
		name := strings.TrimSpace(ev.Title)
		if name == "" {
			name = strings.TrimSpace(ev.URL)
		}
		if name == "" {
			continue
		}
		refIdx := citationIdx[ev.ID]
		highlight := name
		if refIdx > 0 {
			highlight = fmt.Sprintf("%s [来源#%d]", highlight, refIdx)
		}
		sections[idx].Highlights = append(sections[idx].Highlights, highlight)
		sections[idx].EvidenceIDs = append(sections[idx].EvidenceIDs, ev.ID)
	}

	out := make([]TimelineSection, 0, len(sections))
	for _, sec := range sections {
		if len(sec.Highlights) == 0 {
			continue
		}
		if len(sec.Highlights) > 6 {
			sec.Highlights = sec.Highlights[:6]
		}
		out = append(out, sec)
	}
	return out
}

func timelineIndexForEvidence(ev Evidence, labels []string) int {
	year := extractEvidenceYear(ev)
	if year == 0 || len(labels) == 0 {
		return -1
	}
	// Default bucket: <=2018 / 2019-2022 / >=2023.
	switch {
	case year <= 2018:
		return 0
	case year <= 2022:
		if len(labels) >= 2 {
			return 1
		}
		return 0
	default:
		if len(labels) >= 3 {
			return 2
		}
		return len(labels) - 1
	}
}

func normalizedTimelineLabels(timeWindows []string, lang string) []string {
	if len(timeWindows) > 0 {
		out := make([]string, 0, len(timeWindows))
		for _, tw := range timeWindows {
			tw = strings.TrimSpace(tw)
			if tw != "" {
				out = append(out, tw)
			}
		}
		if len(out) > 0 {
			if len(out) > 6 {
				return out[:6]
			}
			return out
		}
	}
	return localizedTimelineDefaultLabels(lang)
}

func computeCitationCoverage(answer string, citationCount int) float64 {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return 0
	}
	lines := strings.Split(trimmed, "\n")
	statementCount := 0
	citedCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !(strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") || strings.HasPrefix(line, "3.") || strings.HasPrefix(line, "4.") || strings.HasPrefix(line, "5.") || strings.HasPrefix(line, "6.") || strings.HasPrefix(line, "7.") || strings.HasPrefix(line, "8.") || strings.HasPrefix(line, "9.")) {
			continue
		}
		statementCount++
		if strings.Contains(line, "来源#") || strings.Contains(strings.ToLower(line), "source#") || strings.Contains(line, "http://") || strings.Contains(line, "https://") {
			citedCount++
		}
	}
	if statementCount == 0 {
		if citationCount > 0 {
			return 1
		}
		return 0
	}
	return float64(citedCount) / float64(statementCount)
}

func localizedVerificationHeading(lang, query string) string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return "核验摘要"
	}
	return "Verification summary"
}

func localizedVerificationStatusLabel(lang, status string) string {
	if normalizeResearchLang(lang, status) == researchLangZH {
		switch status {
		case verificationStatusResolved:
			return "已核实"
		case verificationStatusConflicted:
			return "有冲突"
		default:
			return "待补证"
		}
	}
	switch status {
	case verificationStatusResolved:
		return "Resolved"
	case verificationStatusConflicted:
		return "Conflicted"
	default:
		return "Insufficient"
	}
}

func formatCitationRefs(evidenceIDs []string, citationIdx map[string]int, lang, query string) string {
	if len(evidenceIDs) == 0 || len(citationIdx) == 0 {
		return ""
	}
	indices := make([]int, 0, len(evidenceIDs))
	seen := make(map[int]struct{}, len(evidenceIDs))
	for _, id := range evidenceIDs {
		idx := citationIdx[id]
		if idx <= 0 {
			continue
		}
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		indices = append(indices, idx)
	}
	if len(indices) == 0 {
		return ""
	}
	sort.Ints(indices)
	parts := make([]string, len(indices))
	for i, idx := range indices {
		parts[i] = strconv.Itoa(idx)
	}
	return fmt.Sprintf("[%s#%s]", localizedSourceLabel(lang, query), strings.Join(parts, ","))
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

var (
	reMetaTagPattern = regexp.MustCompile(`(?is)<meta[^>]+>`)
	reMetaAttrKV     = regexp.MustCompile(`(?is)([a-zA-Z0-9:_-]+)\s*=\s*["']([^"']+)["']`)
	reParagraph      = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	reTagStrip       = regexp.MustCompile(`(?is)<[^>]+>`)
	reWhitespace     = regexp.MustCompile(`\s+`)
)

func (s *Service) enrichEvidenceMetadata(ctx context.Context, ev *Evidence) {
	if ev == nil || strings.TrimSpace(ev.URL) == "" {
		return
	}
	reqCtx, cancel := context.WithTimeout(ctx, deepResearchExtractTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ev.URL, nil)
	if err != nil {
		ev.CredibilityScore = maxFloat(0.0, ev.CredibilityScore-0.05)
		return
	}
	req.Header.Set("User-Agent", "ZimaOS-Blue/1.0 (+deepresearch)")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		ev.CredibilityScore = maxFloat(0.0, ev.CredibilityScore-0.05)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		ev.CredibilityScore = maxFloat(0.0, ev.CredibilityScore-0.05)
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		ev.CredibilityScore = maxFloat(0.0, ev.CredibilityScore-0.05)
		return
	}
	htmlText := string(body)

	if author := firstNonEmpty(
		extractMetaContent(htmlText, "author"),
		extractMetaContent(htmlText, "article:author"),
		extractMetaContent(htmlText, "og:article:author"),
	); author != "" {
		ev.Author = author
	}

	if publishedRaw := firstNonEmpty(
		extractMetaContent(htmlText, "article:published_time"),
		extractMetaContent(htmlText, "og:published_time"),
		extractMetaContent(htmlText, "publishdate"),
		extractMetaContent(htmlText, "pubdate"),
		extractMetaContent(htmlText, "date"),
	); publishedRaw != "" {
		if ts := parsePublishedTime(publishedRaw); ts != nil {
			ev.PublishedAt = ts
			ev.CredibilityScore = minFloat(0.99, ev.CredibilityScore+0.03)
		}
	}

	if quote := extractBestQuote(htmlText); quote != "" {
		ev.Quote = quote
	}
	if ev.ClaimKey == "" {
		ev.ClaimKey = buildClaimKey(ev.Title, firstNonEmpty(ev.Quote, ev.Snippet))
	}
	if year := extractEvidenceYear(*ev); year > 0 {
		ev.TimeLabel = strconv.Itoa(year)
	}
}

func extractMetaContent(rawHTML, targetKey string) string {
	target := strings.ToLower(strings.TrimSpace(targetKey))
	if target == "" {
		return ""
	}
	tags := reMetaTagPattern.FindAllString(rawHTML, -1)
	for _, tag := range tags {
		attrs := map[string]string{}
		matches := reMetaAttrKV.FindAllStringSubmatch(tag, -1)
		for _, m := range matches {
			if len(m) != 3 {
				continue
			}
			attrs[strings.ToLower(strings.TrimSpace(m[1]))] = strings.TrimSpace(html.UnescapeString(m[2]))
		}
		key := strings.ToLower(firstNonEmpty(attrs["name"], attrs["property"], attrs["itemprop"]))
		if key == target {
			return strings.TrimSpace(attrs["content"])
		}
	}
	return ""
}

func parsePublishedTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02",
		"2006/01/02 15:04:05",
	}
	for _, layout := range layouts {
		ts, err := time.Parse(layout, raw)
		if err == nil {
			return &ts
		}
	}
	return nil
}

func extractBestQuote(rawHTML string) string {
	paras := reParagraph.FindAllStringSubmatch(rawHTML, 5)
	for _, p := range paras {
		if len(p) < 2 {
			continue
		}
		clean := sanitizeHTMLText(p[1])
		if len(clean) < 40 {
			continue
		}
		if len(clean) > 320 {
			clean = clean[:320] + "..."
		}
		return clean
	}
	return ""
}

func sanitizeHTMLText(raw string) string {
	raw = reTagStrip.ReplaceAllString(raw, " ")
	raw = html.UnescapeString(raw)
	raw = reWhitespace.ReplaceAllString(raw, " ")
	return strings.TrimSpace(raw)
}

func normalizeTimeWindows(windows []string) []string {
	if len(windows) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(windows))
	out := make([]string, 0, len(windows))
	for _, item := range windows {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

func dedupeTaskQueries(tasks []Task) []Task {
	if len(tasks) <= 1 {
		return tasks
	}
	seen := make(map[string]struct{}, len(tasks))
	out := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		key := strings.ToLower(strings.TrimSpace(task.Question))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, task)
	}
	return out
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			return item
		}
	}
	return ""
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func cloneJob(j *Job) *Job {
	cp := *j
	if j.TimeWindows != nil {
		cp.TimeWindows = append([]string(nil), j.TimeWindows...)
	}
	if j.Tasks != nil {
		cp.Tasks = append([]Task(nil), j.Tasks...)
	}
	if j.Evidence != nil {
		cp.Evidence = append([]Evidence(nil), j.Evidence...)
		for i := range cp.Evidence {
			if j.Evidence[i].PublishedAt != nil {
				ts := *j.Evidence[i].PublishedAt
				cp.Evidence[i].PublishedAt = &ts
			}
		}
	}
	if j.Report != nil {
		cp.Report = cloneReport(j.Report)
	}
	return &cp
}

func cloneReport(r *Report) *Report {
	if r == nil {
		return nil
	}
	cp := *r
	if r.Citations != nil {
		cp.Citations = append([]Citation(nil), r.Citations...)
	}
	if r.OpenQuestions != nil {
		cp.OpenQuestions = append([]string(nil), r.OpenQuestions...)
	}
	if r.StageErrors != nil {
		cp.StageErrors = append([]string(nil), r.StageErrors...)
	}
	if r.TimelineSections != nil {
		cp.TimelineSections = append([]TimelineSection(nil), r.TimelineSections...)
		for i := range cp.TimelineSections {
			cp.TimelineSections[i].Highlights = append([]string(nil), r.TimelineSections[i].Highlights...)
			cp.TimelineSections[i].EvidenceIDs = append([]string(nil), r.TimelineSections[i].EvidenceIDs...)
		}
	}
	if r.EntityDisambiguation != nil {
		ed := *r.EntityDisambiguation
		cp.EntityDisambiguation = &ed
	}
	cp.ResearchTrace = cloneResearchTrace(r.ResearchTrace)
	cp.VerificationSummary = cloneVerificationSummary(r.VerificationSummary)
	cp.Calibration = cloneCalibration(r.Calibration)
	return &cp
}

func cloneCalibration(calibration *Calibration) *Calibration {
	if calibration == nil {
		return nil
	}
	cp := *calibration
	if calibration.TakeawayCandidates != nil {
		cp.TakeawayCandidates = append([]TakeawayCandidate(nil), calibration.TakeawayCandidates...)
		for i := range cp.TakeawayCandidates {
			cp.TakeawayCandidates[i].EvidenceIDs = append([]string(nil), calibration.TakeawayCandidates[i].EvidenceIDs...)
		}
	}
	return &cp
}

func cloneInterfaceMap(values map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		return nil
	}
	cp := make(map[string]interface{}, len(values))
	for k, v := range values {
		cp[k] = v
	}
	return cp
}

func cloneResearchTrace(items []ResearchTraceEntry) []ResearchTraceEntry {
	if len(items) == 0 {
		return nil
	}
	return append([]ResearchTraceEntry(nil), items...)
}

func cloneVerificationSummary(summary *VerificationSummary) *VerificationSummary {
	if summary == nil {
		return nil
	}
	cp := *summary
	if summary.Items != nil {
		cp.Items = append([]VerificationItem(nil), summary.Items...)
		for i := range cp.Items {
			cp.Items[i].EvidenceIDs = append([]string(nil), summary.Items[i].EvidenceIDs...)
		}
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
