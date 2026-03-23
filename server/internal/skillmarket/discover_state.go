package skillmarket

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

func cloneSourceResults(items []DiscoverSourceResult) []DiscoverSourceResult {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]DiscoverSourceResult, len(items))
	for i := range items {
		cloned[i] = items[i]
		if len(items[i].Warnings) > 0 {
			cloned[i].Warnings = append([]string(nil), items[i].Warnings...)
		}
	}
	return cloned
}

func aggregateDiscoverResult(jobs []discoverJob) *DiscoverResult {
	result := &DiscoverResult{
		SourceResults: make([]DiscoverSourceResult, 0, len(jobs)),
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		item := *job.Result()
		result.SourceResults = append(result.SourceResults, item)
		result.Discovered += item.Discovered
		result.Updated += item.Updated
		result.Failed += item.Failed
		if job.Done() {
			result.SourcesProcessed++
		}
	}
	return result
}

func countCompletedJobs(jobs []discoverJob) int {
	completed := 0
	for _, job := range jobs {
		if job != nil && job.Done() {
			completed++
		}
	}
	return completed
}

func buildDiscoverJob(s *Service, source Source, run *CrawlRun) (discoverJob, error) {
	switch source.Type {
	case "lightmake_api":
		return newLightmakeDiscoverJob(s, source, run), nil
	case "github_code_search":
		return newGitHubDiscoverJob(s, source, run), nil
	case "clawhub":
		return newClawHubDiscoverJob(s, source, run), nil
	case "html_catalog":
		return newHTMLCatalogDiscoverJob(s, source, run), nil
	case "seed_page":
		return newSeedPageDiscoverJob(s, source, run), nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.Type)
	}
}

func (s *Service) runDiscoverJobToCompletion(ctx context.Context, job discoverJob) error {
	if job == nil {
		return nil
	}
	for !job.Done() {
		if _, err := job.Step(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) completeDiscoverJob(ctx context.Context, job discoverJob, stepErr error) {
	if job == nil {
		return
	}
	run := job.Run()
	result := job.Result()
	if run == nil || result == nil {
		return
	}
	switch result.Status {
	case "failed":
		run.Status = "failed"
		if stepErr != nil {
			run.ErrorText = stepErr.Error()
		}
	case "partial":
		run.Status = "success"
		run.ErrorText = ""
	default:
		run.Status = "success"
		run.ErrorText = ""
	}
	if stepErr != nil && result.Status == "failed" {
		run.ErrorText = stepErr.Error()
	}
	if err := s.store.CompleteCrawlRun(ctx, run); err != nil && s.logger != nil {
		s.logger.Warn("complete crawl run failed", zap.Error(err))
	}
}
