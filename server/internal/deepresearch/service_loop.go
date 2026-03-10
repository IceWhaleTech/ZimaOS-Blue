package deepresearch

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type retrieveRoundResult struct {
	Evidence          []Evidence
	StageErrors       []string
	FailedQueries     []string
	TaskErrors        int
	MaxSourcesReached bool
	Aborted           bool
	AbortErr          error
}

func (s *Service) collectEvidenceRound(
	ctx context.Context,
	jobID string,
	tasks []Task,
	mode Mode,
	lang string,
	useV2 bool,
	seenURLs map[string]struct{},
	currentEvidenceCount int,
	maxSources int,
	iteration int,
) retrieveRoundResult {
	if len(tasks) == 0 {
		return retrieveRoundResult{}
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
	workerCount := maxInt(1, minInt(len(tasks), s.searchParallelismForMode(mode)))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasksCh {
				hits, err := s.searchWithRetry(searchCtx, jobID, task.Question, searchResultsPerTask(mode), lang)
				results <- taskEvidence{taskID: task.ID, query: task.Question, hits: hits, err: err}
			}
		}()
	}

	go func() {
		defer close(tasksCh)
		for _, task := range tasks {
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

	result := retrieveRoundResult{}
	maxSourcesReached := false
	for {
		select {
		case <-ctx.Done():
			result.Aborted = true
			result.AbortErr = ctx.Err()
			return result
		case r, ok := <-results:
			if !ok {
				result.StageErrors = dedupeStrings(result.StageErrors)
				result.FailedQueries = dedupeStrings(result.FailedQueries)
				result.MaxSourcesReached = maxSourcesReached
				return result
			}
			if r.err != nil {
				if errors.Is(r.err, context.Canceled) || errors.Is(r.err, context.DeadlineExceeded) {
					continue
				}
				result.TaskErrors++
				result.FailedQueries = append(result.FailedQueries, r.query)
				result.StageErrors = append(result.StageErrors, fmt.Sprintf("search failed: %s (%v)", r.query, r.err))
				s.broadcast(jobID, "stage_warning", map[string]interface{}{
					"stage":     "retrieve",
					"query":     r.query,
					"error":     r.err.Error(),
					"iteration": iteration,
				})
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
				if currentEvidenceCount+len(result.Evidence) >= maxSources {
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
				ev.ClaimKey = buildClaimKey(ev.Title, ev.Snippet)
				if useV2 && idx < deepResearchExtractTopK {
					s.enrichEvidenceMetadata(searchCtx, &ev)
				}
				result.Evidence = append(result.Evidence, ev)
				s.broadcast(jobID, "evidence_added", map[string]interface{}{
					"iteration": iteration,
					"task_id":   ev.TaskID,
					"title":     ev.Title,
					"url":       ev.URL,
					"domain":    ev.Domain,
				})
				if currentEvidenceCount+len(result.Evidence) >= maxSources {
					maxSourcesReached = true
					searchCancel()
					break
				}
			}
		}
	}
}
