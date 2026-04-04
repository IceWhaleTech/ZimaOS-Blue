package skillmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/crawler"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const githubCodeSearchMaxPages = 10

type discoverStepStats struct {
	Inserted int
	Updated  int
	Failed   int
}

type discoverJob interface {
	Source() Source
	Run() *CrawlRun
	Result() *DiscoverSourceResult
	Step(ctx context.Context) (discoverStepStats, error)
	Done() bool
}

type baseDiscoverJob struct {
	svc    *Service
	source Source
	run    *CrawlRun
	result DiscoverSourceResult
	done   bool
}

func newBaseDiscoverJob(svc *Service, source Source, run *CrawlRun) baseDiscoverJob {
	return baseDiscoverJob{
		svc:    svc,
		source: source,
		run:    run,
		result: DiscoverSourceResult{SourceID: source.ID, Status: "pending"},
	}
}

func (j *baseDiscoverJob) Source() Source                { return j.source }
func (j *baseDiscoverJob) Run() *CrawlRun                { return j.run }
func (j *baseDiscoverJob) Result() *DiscoverSourceResult { return &j.result }
func (j *baseDiscoverJob) Done() bool                    { return j.done }

func (j *baseDiscoverJob) beginStep() {
	if !j.done && j.result.Status == "pending" {
		j.result.Status = "running"
	}
}

func (j *baseDiscoverJob) addWarning(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	for _, existing := range j.result.Warnings {
		if existing == message {
			return
		}
	}
	j.result.Warnings = append(j.result.Warnings, message)
}

func (j *baseDiscoverJob) finishSuccess() {
	if j.result.Partial {
		j.result.Status = "partial"
	} else {
		j.result.Status = "success"
	}
	j.done = true
}

func (j *baseDiscoverJob) finishFailure(err error) {
	if err != nil {
		j.addWarning(err.Error())
	}
	j.result.Status = "failed"
	j.done = true
}

func (j *baseDiscoverJob) applyFlush(inserted, updated, failed int) discoverStepStats {
	j.result.Discovered += inserted
	j.result.Updated += updated
	j.result.Failed += failed
	return discoverStepStats{Inserted: inserted, Updated: updated, Failed: failed}
}

type gitHubDiscoverJob struct {
	baseDiscoverJob
	page    int
	perPage int
}

type gitHubCodeSearchPayload struct {
	TotalCount        int  `json:"total_count"`
	IncompleteResults bool `json:"incomplete_results"`
	Items             []struct {
		Name       string `json:"name"`
		Path       string `json:"path"`
		URL        string `json:"url"`
		HTMLURL    string `json:"html_url"`
		SHA        string `json:"sha"`
		Repository struct {
			FullName        string `json:"full_name"`
			HTMLURL         string `json:"html_url"`
			StargazersCount int    `json:"stargazers_count"`
			UpdatedAt       string `json:"updated_at"`
		} `json:"repository"`
	} `json:"items"`
}

func newGitHubDiscoverJob(svc *Service, source Source, run *CrawlRun) discoverJob {
	perPage := svc.cfg.GitHubSearchPageSize
	if perPage <= 0 {
		perPage = 25
	}
	job := &gitHubDiscoverJob{
		baseDiscoverJob: newBaseDiscoverJob(svc, source, run),
		perPage:         perPage,
	}
	return job
}

func (j *gitHubDiscoverJob) Step(ctx context.Context) (discoverStepStats, error) {
	if j.done {
		return discoverStepStats{}, nil
	}
	j.beginStep()
	j.page++
	j.result.Pages++
	j.result.Requests++

	apiURL := fmt.Sprintf("%s/search/code?q=%s&per_page=%d&page=%d", strings.TrimRight(j.svc.cfg.GitHubAPIBaseURL, "/"), urlQueryEscape(j.source.BaseURL), j.perPage, j.page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(j.svc.cfg.GitHubToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := j.svc.httpClient.Do(req)
	if err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnprocessableEntity {
		j.result.Partial = true
		j.addWarning(fmt.Sprintf("GitHub code search returned %d and may be rate limited or capped", resp.StatusCode))
		j.finishSuccess()
		return discoverStepStats{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		err = fmt.Errorf("github code search: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		j.finishFailure(err)
		return discoverStepStats{}, err
	}

	var payload gitHubCodeSearchPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	if payload.IncompleteResults {
		j.result.Partial = true
		j.addWarning("GitHub code search reported incomplete_results")
	}
	if payload.TotalCount > j.perPage*githubCodeSearchMaxPages {
		j.result.Partial = true
		j.addWarning("GitHub code search exceeded the 1000-result search cap")
	}
	if len(payload.Items) == 0 {
		j.finishSuccess()
		return discoverStepStats{}, nil
	}

	batch := make([]*SkillUpsertRecord, 0, len(payload.Items))
	for _, item := range payload.Items {
		raw, requestCount, fetchErr := j.svc.fetchGitHubBlobWithFallback(ctx, item.URL, item.HTMLURL)
		j.result.Requests += requestCount
		if fetchErr != nil {
			j.run.Failed++
			j.result.Failed++
			continue
		}
		lastUpdated := parseTime(strings.TrimSpace(item.Repository.UpdatedAt))
		record, recordErr := j.svc.prepareIngestRecord(ctx, ingestRequest{
			SourceID:       j.source.ID,
			SourceName:     defaultString(j.source.DisplayName, j.source.ID),
			SourceGroup:    defaultString(j.source.SourceGroup, j.source.ID),
			SourceType:     j.source.Type,
			RepoURL:        item.Repository.HTMLURL,
			Homepage:       item.Repository.HTMLURL,
			DownloadURL:    item.HTMLURL,
			SourceURL:      item.HTMLURL,
			SkillPath:      item.Path,
			SkillContent:   raw,
			Stars:          item.Repository.StargazersCount,
			LastUpdated:    lastUpdated,
			CommitHash:     item.SHA,
			DefaultSkillID: pathSkillID(item.Repository.HTMLURL, item.Path, item.Name),
			Installable:    true,
			InstallType:    InstallTypeGitRepo,
			ArtifactKind:   ArtifactKindOpenSource,
		})
		if recordErr != nil {
			j.run.Failed++
			j.result.Failed++
			continue
		}
		batch = append(batch, record)
	}
	inserted, updated, failed, err := j.svc.flushPreparedBatch(ctx, j.run, batch)
	stats := j.applyFlush(inserted, updated, failed)
	if err != nil {
		j.finishFailure(err)
		return stats, err
	}
	if len(payload.Items) < j.perPage || j.page >= githubCodeSearchMaxPages {
		j.finishSuccess()
	}
	return stats, nil
}

type clawHubDiscoverJob struct {
	baseDiscoverJob
	page      int
	seenSlugs map[string]struct{}
}

func newClawHubDiscoverJob(svc *Service, source Source, run *CrawlRun) discoverJob {
	return &clawHubDiscoverJob{
		baseDiscoverJob: newBaseDiscoverJob(svc, source, run),
		page:            1,
		seenSlugs:       make(map[string]struct{}),
	}
}

func (j *clawHubDiscoverJob) Step(ctx context.Context) (discoverStepStats, error) {
	if j.done {
		return discoverStepStats{}, nil
	}
	j.beginStep()
	j.result.Pages++
	j.result.Requests++

	baseURL := strings.TrimRight(j.source.BaseURL, "/") + "/api/v1/skills"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?page=%d", baseURL, j.page), nil)
	if err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	resp, err := j.svc.httpClient.Do(req)
	if err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("clawhub list status: %d", resp.StatusCode)
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	var payload clawHubListResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	if j.page == 1 && len(payload.Items) == 0 {
		var wrapped struct {
			Data string `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapped); err == nil && strings.Contains(strings.ToLower(wrapped.Data), "<!doctype html>") {
			err = fmt.Errorf("clawhub list returned wrapped html instead of skill data")
			j.finishFailure(err)
			return discoverStepStats{}, err
		}
	}
	if len(payload.Items) == 0 {
		j.finishSuccess()
		return discoverStepStats{}, nil
	}

	batch := make([]*SkillUpsertRecord, 0, len(payload.Items))
	newItems := 0
	for _, item := range payload.Items {
		if _, exists := j.seenSlugs[item.Slug]; exists {
			continue
		}
		j.seenSlugs[item.Slug] = struct{}{}
		newItems++

		var enrichment *clawHubSkillEnrichment
		enrichment, err = j.svc.fetchClawHubSkillEnrichment(ctx, j.source, item.Slug)
		j.result.Requests++
		if err != nil {
			enrichment = nil
		}
		explicitAuthor := ""
		descriptionHint := ""
		categoryHint := ""
		var additionalTags []string
		var securitySignals *SourceSecuritySignals
		if enrichment != nil {
			explicitAuthor = enrichment.Author
			descriptionHint = enrichment.Description
			categoryHint = enrichment.CategoryHint
			additionalTags = enrichment.AdditionalTags
			securitySignals = enrichment.SecuritySignals
		}

		skillURL := fmt.Sprintf("%s/api/v1/skills/%s/skill-md", strings.TrimRight(j.source.BaseURL, "/"), item.Slug)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, skillURL, nil)
		if err != nil {
			j.run.Failed++
			j.result.Failed++
			continue
		}
		j.result.Requests++
		resp, err := j.svc.httpClient.Do(req)
		if err != nil {
			j.run.Failed++
			j.result.Failed++
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			j.run.Failed++
			j.result.Failed++
			continue
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			j.run.Failed++
			j.result.Failed++
			continue
		}
		record, err := j.svc.prepareIngestRecord(ctx, ingestRequest{
			SourceID:        j.source.ID,
			SourceName:      defaultString(j.source.DisplayName, j.source.ID),
			SourceGroup:     defaultString(j.source.SourceGroup, j.source.ID),
			SourceType:      j.source.Type,
			RepoURL:         strings.TrimRight(j.source.BaseURL, "/") + "/skills/" + item.Slug,
			Homepage:        strings.TrimRight(j.source.BaseURL, "/") + "/skills/" + item.Slug,
			DownloadURL:     skillURL,
			SourceURL:       skillURL,
			SkillPath:       "SKILL.md",
			SkillContent:    string(raw),
			Stars:           item.Stats.Stars,
			Downloads:       item.Stats.Downloads,
			LastUpdated:     unixMilliTime(item.UpdatedAt),
			ExplicitName:    item.DisplayName,
			ExplicitID:      normalizeSkillID(item.Slug),
			ExplicitVersion: defaultString(item.LatestVersion.Version, "0.1.0"),
			ExplicitAuthor:  explicitAuthor,
			DescriptionHint: descriptionHint,
			CategoryHint:    categoryHint,
			AdditionalTags:  additionalTags,
			SecuritySignals: securitySignals,
			Installable:     true,
			InstallType:     InstallTypeRawSkill,
			ArtifactKind:    ArtifactKindOpenSource,
		})
		if err != nil {
			j.run.Failed++
			j.result.Failed++
			continue
		}
		batch = append(batch, record)
	}
	inserted, updated, failed, err := j.svc.flushPreparedBatch(ctx, j.run, batch)
	stats := j.applyFlush(inserted, updated, failed)
	if err != nil {
		j.finishFailure(err)
		return stats, err
	}
	if newItems == 0 {
		j.finishSuccess()
		return stats, nil
	}
	j.page++
	return stats, nil
}

type lightmakeDiscoverJob struct {
	baseDiscoverJob
	page       int
	pageSize   int
	totalPages int
	maxPages   int
}

func newLightmakeDiscoverJob(svc *Service, source Source, run *CrawlRun) discoverJob {
	pageSize := svc.cfg.LightmakePageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	return &lightmakeDiscoverJob{
		baseDiscoverJob: newBaseDiscoverJob(svc, source, run),
		page:            1,
		pageSize:        pageSize,
		totalPages:      1,
		maxPages:        500,
	}
}

func (j *lightmakeDiscoverJob) Step(ctx context.Context) (discoverStepStats, error) {
	if j.done {
		return discoverStepStats{}, nil
	}
	j.beginStep()
	j.result.Pages++
	j.result.Requests++

	queryURL := fmt.Sprintf("%s/api/skills?page=%d&pageSize=%d&sortBy=score&order=desc", strings.TrimRight(j.source.BaseURL, "/"), j.page, j.pageSize)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := j.svc.httpClient.Do(req)
	if err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	resp.Body.Close()
	if err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("lightmake list status: %d", resp.StatusCode)
		j.finishFailure(err)
		return discoverStepStats{}, err
	}

	var payload lightmakeListResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	if payload.Code != 0 {
		err = fmt.Errorf("lightmake list error: %s", strings.TrimSpace(defaultString(payload.Message, string(body))))
		j.finishFailure(err)
		return discoverStepStats{}, err
	}
	if payload.Data.Total > 0 {
		j.totalPages = (payload.Data.Total + j.pageSize - 1) / j.pageSize
	}
	if len(payload.Data.Skills) == 0 {
		j.finishSuccess()
		return discoverStepStats{}, nil
	}

	pageBatch := make([]*SkillUpsertRecord, 0, len(payload.Data.Skills))
	for _, item := range payload.Data.Skills {
		record, err := j.svc.prepareIngestRecord(ctx, ingestRequest{
			SourceID:        j.source.ID,
			SourceName:      defaultString(j.source.DisplayName, j.source.ID),
			SourceGroup:     defaultString(j.source.SourceGroup, j.source.ID),
			SourceType:      j.source.Type,
			RepoURL:         item.Homepage,
			Homepage:        item.Homepage,
			DownloadURL:     strings.TrimRight(j.source.BaseURL, "/") + "/api/v1/download?slug=" + url.QueryEscape(strings.TrimSpace(item.Slug)),
			SourceURL:       item.Homepage,
			SkillPath:       "SKILL.md",
			SkillContent:    lightmakeSkillMarkdown(item),
			Stars:           item.Stars,
			Downloads:       item.Downloads,
			LastUpdated:     unixMilliTime(item.UpdatedAt),
			ExplicitID:      normalizeSkillID(item.Slug),
			ExplicitName:    strings.TrimSpace(defaultString(item.Name, item.Slug)),
			ExplicitVersion: defaultString(strings.TrimSpace(item.Version), "catalog"),
			ExplicitAuthor:  strings.TrimSpace(item.OwnerName),
			DescriptionHint: lightmakeDescription(item),
			CategoryHint:    strings.TrimSpace(item.Category),
			AdditionalTags:  item.Tags,
			Installable:     true,
			InstallType:     InstallTypeSourceArchive,
			ArtifactKind:    ArtifactKindUnknown,
		})
		if err != nil {
			j.run.Failed++
			j.result.Failed++
			continue
		}
		pageBatch = append(pageBatch, record)
	}
	inserted, updated, failed, err := j.svc.flushPreparedBatch(ctx, j.run, pageBatch)
	stats := j.applyFlush(inserted, updated, failed)
	if err != nil {
		j.finishFailure(err)
		return stats, err
	}
	if j.page >= j.totalPages || j.page >= j.maxPages {
		j.finishSuccess()
		return stats, nil
	}
	j.page++
	return stats, nil
}

type htmlCatalogDiscoverJob struct {
	baseDiscoverJob
	queue          []string
	seenPages      map[string]struct{}
	seenSeeds      map[string]struct{}
	processedPages int
}

func newHTMLCatalogDiscoverJob(svc *Service, source Source, run *CrawlRun) discoverJob {
	initialURL := strings.TrimSpace(source.BaseURL)
	seenPages := make(map[string]struct{}, 1)
	if initialURL != "" {
		seenPages[initialURL] = struct{}{}
	}
	return &htmlCatalogDiscoverJob{
		baseDiscoverJob: newBaseDiscoverJob(svc, source, run),
		queue:           []string{initialURL},
		seenPages:       seenPages,
		seenSeeds:       make(map[string]struct{}),
	}
}

func (j *htmlCatalogDiscoverJob) Step(ctx context.Context) (discoverStepStats, error) {
	if j.done {
		return discoverStepStats{}, nil
	}
	j.beginStep()
	batchLimit := j.svc.cfg.HTMLCatalogCrawlBatchPages
	if batchLimit <= 0 {
		batchLimit = 4
	}
	maxPages := j.svc.cfg.HTMLCatalogCrawlMaxPages
	if maxPages <= 0 {
		maxPages = 200
	}

	batch := make([]*SkillUpsertRecord, 0, nonPagedIngestBatchSize)
	for processed := 0; processed < batchLimit && len(j.queue) > 0 && j.processedPages < maxPages; processed++ {
		currentURL := j.queue[0]
		j.queue = j.queue[1:]
		j.result.Pages++
		j.result.Requests++
		j.processedPages++

		page, err := j.svc.fetchCatalogPage(ctx, j.source, currentURL)
		if err != nil {
			j.run.Failed++
			j.result.Failed++
			continue
		}
		for _, link := range page.Links {
			resolved, ok := resolveCatalogLink(page.URL, link)
			if !ok || !looksLikeCatalogPage(resolved, j.source.BaseURL) {
				continue
			}
			if _, exists := j.seenPages[resolved]; exists {
				continue
			}
			j.seenPages[resolved] = struct{}{}
			j.queue = append(j.queue, resolved)
		}

		records, failures, err := j.preparePageRecords(ctx, page)
		if failures > 0 {
			j.run.Failed += failures
			j.result.Failed += failures
		}
		if err != nil {
			j.addWarning(err.Error())
			continue
		}
		batch = append(batch, records...)
	}

	inserted, updated, failed, err := j.svc.flushPreparedBatch(ctx, j.run, batch)
	stats := j.applyFlush(inserted, updated, failed)
	if err != nil {
		j.finishFailure(err)
		return stats, err
	}
	if len(j.queue) == 0 {
		j.finishSuccess()
		return stats, nil
	}
	if j.processedPages >= maxPages {
		j.result.Partial = true
		j.addWarning(fmt.Sprintf("html catalog crawl reached max page limit (%d)", maxPages))
		j.finishSuccess()
		return stats, nil
	}
	return stats, nil
}

func (j *htmlCatalogDiscoverJob) preparePageRecords(ctx context.Context, page *catalogPage) ([]*SkillUpsertRecord, int, error) {
	if page == nil {
		return nil, 0, nil
	}
	if page.Embedded != nil && strings.TrimSpace(page.Embedded.RawSkill) != "" {
		record, err := j.svc.prepareEmbeddedCatalogSkillRecord(ctx, j.source, page)
		if err != nil {
			return nil, 1, nil
		}
		return []*SkillUpsertRecord{record}, 0, nil
	}

	records := make([]*SkillUpsertRecord, 0, 4)
	failures := 0
	hadInstallable := false
	for _, link := range page.Links {
		resolved, ok := resolveCatalogLink(page.URL, link)
		if !ok {
			continue
		}
		if _, exists := j.seenSeeds[resolved]; exists {
			continue
		}
		switch {
		case skillbundle.IsGitHubRepoURL(resolved):
			j.seenSeeds[resolved] = struct{}{}
			record, err := j.svc.prepareGitHubRepoSeedRecord(ctx, repoOwner(resolved), repoName(resolved), &j.source)
			if err != nil {
				failures++
				continue
			}
			hadInstallable = true
			records = append(records, record)
		case looksLikeSkillURL(resolved):
			j.seenSeeds[resolved] = struct{}{}
			record, err := j.svc.prepareSkillURLSeedRecord(ctx, resolved, &j.source)
			if err != nil {
				failures++
				continue
			}
			hadInstallable = true
			records = append(records, record)
		}
	}
	if hadInstallable {
		return records, failures, nil
	}
	record, err := j.svc.prepareCatalogOnlySkillRecord(ctx, j.source, page)
	if err != nil {
		return nil, failures + 1, nil
	}
	return []*SkillUpsertRecord{record}, failures, nil
}

type seedPageDiscoverJob struct {
	baseDiscoverJob
}

func newSeedPageDiscoverJob(svc *Service, source Source, run *CrawlRun) discoverJob {
	return &seedPageDiscoverJob{baseDiscoverJob: newBaseDiscoverJob(svc, source, run)}
}

func (j *seedPageDiscoverJob) Step(ctx context.Context) (discoverStepStats, error) {
	if j.done {
		return discoverStepStats{}, nil
	}
	j.beginStep()
	j.result.Pages++
	j.result.Requests++
	pageCrawler := crawler.New(crawler.Config{
		MaxDepth:       0,
		MaxConcurrency: maxInt(1, j.svc.cfg.SeedPageMaxConcurrency),
		RequestTimeout: 20 * time.Second,
		UserAgent:      "ZimaOS-SkillMarket/1.0",
	})
	results := pageCrawler.CrawlSync(ctx, []string{j.source.BaseURL})
	batch := make([]*SkillUpsertRecord, 0, nonPagedIngestBatchSize)
	seenSeeds := make(map[string]struct{})
	for _, page := range results {
		for _, link := range page.Links {
			resolved, ok := resolveCatalogLink(page.URL, link)
			if !ok {
				continue
			}
			if _, exists := seenSeeds[resolved]; exists {
				continue
			}
			switch {
			case skillbundle.IsGitHubRepoURL(resolved):
				record, err := j.svc.prepareGitHubRepoSeedRecord(ctx, repoOwner(resolved), repoName(resolved), &j.source)
				if err != nil {
					j.run.Failed++
					j.result.Failed++
					continue
				}
				seenSeeds[resolved] = struct{}{}
				batch = append(batch, record)
			case looksLikeSkillURL(resolved):
				record, err := j.svc.prepareSkillURLSeedRecord(ctx, resolved, &j.source)
				if err != nil {
					j.run.Failed++
					j.result.Failed++
					continue
				}
				seenSeeds[resolved] = struct{}{}
				batch = append(batch, record)
			}
		}
	}
	inserted, updated, failed, err := j.svc.flushPreparedBatch(ctx, j.run, batch)
	stats := j.applyFlush(inserted, updated, failed)
	if err != nil {
		j.finishFailure(err)
		return stats, err
	}
	j.finishSuccess()
	return stats, nil
}

func unixMilliTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}

func nowTime() time.Time {
	return timeutil.NowTime()
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
