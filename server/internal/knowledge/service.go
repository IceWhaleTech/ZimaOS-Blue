package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Service struct {
	workspaceDir     string
	repoRoot         string
	memorySink       MemorySink
	events           EventPublisher
	author           KnowledgeAuthor
	onCompileSuccess func(context.Context, *KnowledgeCompileReport)
	now              func() time.Time

	mu           sync.RWMutex
	jobs         map[string]*KnowledgeJob
	cancelFuncs  map[string]context.CancelFunc
	subscribers  map[string]map[chan Event]struct{}
	lastTerminal map[string]Event
}

type sourceDocument struct {
	AbsPath    string
	Ref        string
	Title      string
	PageType   string
	Content    string
	Summary    string
	Keywords   []string
	SourceHash string
	Slug       string
}

type sourceManifest struct {
	GeneratedAt time.Time              `json:"generated_at"`
	Sources     []sourceManifestRecord `json:"sources"`
}

type sourceManifestRecord struct {
	Ref            string   `json:"ref"`
	Slug           string   `json:"slug"`
	Title          string   `json:"title"`
	PageType       string   `json:"page_type"`
	SourceHash     string   `json:"source_hash"`
	Keywords       []string `json:"keywords,omitempty"`
	LastIngestedAt string   `json:"last_ingested_at,omitempty"`
	AffectedPages  []string `json:"affected_pages,omitempty"`
	Status         string   `json:"status,omitempty"`
}

var (
	eventCounter           uint64
	jobIDCounter           uint64
	tokenPattern           = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]{2,}`)
	wordPattern            = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]*`)
	capitalizedTokenRegexp = regexp.MustCompile(`^(?:[A-Z]{2,}|[A-Z][a-zA-Z0-9]{2,})$`)
	stopwordSet            = map[string]struct{}{
		"about": {}, "also": {}, "and": {}, "blue": {}, "docs": {}, "file": {}, "for": {}, "from": {},
		"into": {}, "knowledge": {}, "markdown": {}, "runtime": {}, "that": {}, "the": {}, "this": {},
		"with": {}, "workspace": {},
	}
	entityTokenStopwordSet = map[string]struct{}{
		"architecture": {}, "comparison": {}, "concept": {}, "decision": {}, "docs": {}, "guide": {},
		"index": {}, "knowledge": {}, "lint": {}, "note": {}, "notes": {}, "overview": {}, "page": {},
		"pages": {}, "query": {}, "readme": {}, "runtime": {}, "schema": {}, "shared": {}, "source": {},
		"summary": {}, "synthesis": {}, "topic": {}, "wiki": {}, "workspace": {},
	}
	conceptTokenStopwordSet = map[string]struct{}{
		"about": {}, "blue": {}, "comparison": {}, "concept": {}, "decision": {}, "docs": {}, "guide": {},
		"index": {}, "note": {}, "notes": {}, "overview": {}, "page": {}, "pages": {}, "query": {},
		"readme": {}, "schema": {}, "shared": {}, "source": {}, "summary": {}, "synthesis": {},
		"topic": {}, "wiki": {}, "workspace": {},
	}
)

func NewService(options ServiceOptions) *Service {
	nowFn := options.Now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		workspaceDir:     strings.TrimSpace(options.WorkspaceDir),
		repoRoot:         strings.TrimSpace(options.RepoRoot),
		memorySink:       options.MemorySink,
		events:           options.EventPublisher,
		author:           options.KnowledgeAuthor,
		onCompileSuccess: options.OnCompileSuccess,
		now: func() time.Time {
			return nowFn().UTC()
		},
		jobs:         make(map[string]*KnowledgeJob),
		cancelFuncs:  make(map[string]context.CancelFunc),
		subscribers:  make(map[string]map[chan Event]struct{}),
		lastTerminal: make(map[string]Event),
	}
}

func (s *Service) Compile(ctx context.Context, req CompileRequest) (*KnowledgeCompileReport, error) {
	ctx = applyKnowledgeProviderRouting(ctx, req.ProviderID)
	return s.ingest(ctx, req.TargetPaths)
}

func (s *Service) ingest(ctx context.Context, targetPaths []string) (*KnowledgeIngestReport, error) {
	return s.ingestWithOptions(ctx, targetPaths, false)
}

func (s *Service) ingestWithOptions(ctx context.Context, targetPaths []string, force bool) (*KnowledgeIngestReport, error) {
	if err := s.ensureKnowledgeDirs(); err != nil {
		return nil, err
	}
	schemaPath, schemaContent, err := s.ensureSchemaDocument()
	if err != nil {
		return nil, err
	}
	logPath, err := s.ensureLogDocument()
	if err != nil {
		return nil, err
	}
	sources, err := s.discoverSources(targetPaths)
	if err != nil {
		return nil, err
	}
	existingPages, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, err
	}
	existingBySlug := make(map[string]pageDocument, len(existingPages))
	pageMap := make(map[string]pageDocument, len(existingPages))
	for _, page := range existingPages {
		existingBySlug[page.Summary.Slug] = page
		pageMap[page.Summary.Slug] = page
	}
	manifest := s.loadManifest()
	manifest.GeneratedAt = s.now()
	manifestByRef := make(map[string]sourceManifestRecord, len(manifest.Sources))
	for _, record := range manifest.Sources {
		manifestByRef[record.Ref] = record
	}

	newPages := make([]KnowledgePageSummary, 0)
	updatedPages := make([]KnowledgePageSummary, 0)
	skippedSources := make([]string, 0)
	conflicts := make([]string, 0)
	gaps := make([]string, 0)
	changedDocs := make([]pageDocument, 0)
	fallbackMode := false

	for _, source := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		previous := manifestByRef[source.Ref]
		if !force && previous.SourceHash == source.SourceHash {
			skippedSources = append(skippedSources, source.Ref)
			previous.Status = "skipped"
			manifestByRef[source.Ref] = previous
			continue
		}

		authored, authoredConflicts, authoredGaps, usedFallback, err := s.generatePagesForSource(ctx, schemaContent, source, pageSummariesFromMap(pageMap))
		if err != nil {
			return nil, err
		}
		fallbackMode = fallbackMode || usedFallback
		conflicts = append(conflicts, authoredConflicts...)
		gaps = append(gaps, authoredGaps...)

		affectedPages := make([]string, 0, len(authored))
		for _, page := range authored {
			existing, exists := pageMap[page.Slug]
			_, existedBefore := existingBySlug[page.Slug]
			if exists {
				page.GeneratedAt = existing.Summary.GeneratedAt
			}
			doc := pageDocument{
				Summary: page.KnowledgePageSummary,
				Content: page.Content,
			}
			pageMap[page.Slug] = doc
			changedDocs = append(changedDocs, doc)
			affectedPages = append(affectedPages, page.Slug)
			if existedBefore {
				updatedPages = append(updatedPages, clonePageSummary(doc.Summary))
			} else {
				newPages = append(newPages, clonePageSummary(doc.Summary))
			}
		}

		manifestByRef[source.Ref] = sourceManifestRecord{
			Ref:            source.Ref,
			Slug:           source.Slug,
			Title:          source.Title,
			PageType:       PageTypeSourceSummary,
			SourceHash:     source.SourceHash,
			Keywords:       append([]string(nil), source.Keywords...),
			LastIngestedAt: s.now().Format(time.RFC3339),
			AffectedPages:  uniqueStrings(affectedPages),
			Status:         "ingested",
		}
	}

	allPages := mapToSortedPages(pageMap)
	rebuildBacklinks(allPages)
	rewritten := make([]pageDocument, 0)
	for _, page := range allPages {
		existing, exists := existingBySlug[page.Summary.Slug]
		if exists && renderPageDocument(existing) == renderPageDocument(page) {
			continue
		}
		if err := s.writeSinglePage(page); err != nil {
			return nil, err
		}
		rewritten = append(rewritten, page)
	}

	manifest.Sources = manifestRecordsFromMap(manifestByRef)
	manifestPath, err := s.writeManifest(manifest)
	if err != nil {
		return nil, err
	}
	indexPath, err := s.writePagesIndex(allPages)
	if err != nil {
		return nil, err
	}

	conflicts = uniqueStrings(conflicts)
	gaps = uniqueStrings(gaps)
	skippedSources = uniqueStrings(skippedSources)
	report := &KnowledgeIngestReport{
		GeneratedAt:    s.now(),
		Pages:          pageSummaries(allPages),
		NewPages:       dedupePageSummaries(newPages),
		UpdatedPages:   dedupePageSummaries(updatedPages),
		SkippedSources: skippedSources,
		Conflicts:      conflicts,
		Gaps:           gaps,
		ManifestPath:   manifestPath,
		IndexPath:      indexPath,
		SchemaPath:     schemaPath,
		LogPath:        logPath,
		FallbackMode:   fallbackMode || s.author == nil,
	}
	if err := s.appendLogEntry(KnowledgeLogEntry{
		Timestamp:    s.now(),
		Operation:    "ingest",
		Title:        buildIngestTitle(sources, targetPaths),
		Sources:      sourceRefsFromSources(sources),
		NewPages:     pageSlugs(report.NewPages),
		UpdatedPages: pageSlugs(report.UpdatedPages),
		Conflicts:    conflicts,
		Gaps:         gaps,
		Reason:       buildIngestReason(skippedSources, report.NewPages, report.UpdatedPages),
	}); err != nil {
		return nil, err
	}
	if len(rewritten) > 0 {
		if err := s.rememberPages(ctx, rewritten); err != nil {
			return nil, err
		}
	}
	if s.onCompileSuccess != nil {
		s.onCompileSuccess(ctx, report)
	}
	return report, nil
}

func (s *Service) Lint(ctx context.Context, req LintRequest) (*KnowledgeLintReport, error) {
	ctx = applyKnowledgeProviderRouting(ctx, req.ProviderID)
	if err := s.ensureKnowledgeDirs(); err != nil {
		return nil, err
	}
	pages, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, err
	}
	pages, repairedPaths, err := s.repairPagesForLint(ctx, pages, req.TargetPaths)
	if err != nil {
		return nil, err
	}
	expectedBacklinks := computeBacklinks(pages)
	currentIndex, _ := s.GetIndexMarkdown(ctx)
	issues := make([]LintIssue, 0)
	fixedPaths := append([]string(nil), repairedPaths...)
	seenTitles := make(map[string]string)
	conflictPairs := make(map[string][]string)

	for idx := range pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page := &pages[idx]
		if isLowQualityPage(*page) {
			issues = append(issues, LintIssue{
				Kind:            IssueKindLowQuality,
				PageSlug:        page.Summary.Slug,
				Message:         "page summary or body is too thin",
				Severity:        "medium",
				Category:        "review_required",
				SuggestedAction: "enrich the summary or re-ingest the source",
			})
		}
		if hasStaleHash, staleMessage := s.pageHasStaleHash(*page); hasStaleHash {
			issues = append(issues, LintIssue{
				Kind:            IssueKindStaleHash,
				PageSlug:        page.Summary.Slug,
				Message:         staleMessage,
				Severity:        "medium",
				Category:        "review_required",
				SuggestedAction: "run ingest for the stale source and review superseded claims",
			})
		}
		expected := expectedBacklinks[page.Summary.Slug]
		if !sameStringSet(page.Summary.Backlinks, expected) {
			page.Summary.Backlinks = append([]string(nil), expected...)
			issues = append(issues, LintIssue{
				Kind:            IssueKindMissingBacklinks,
				PageSlug:        page.Summary.Slug,
				Message:         "page backlinks were missing or stale",
				AutoFixed:       true,
				Severity:        "low",
				Category:        "auto_fixed",
				SuggestedAction: "backlinks were rebuilt automatically",
			})
			if err := s.writeSinglePage(*page); err != nil {
				return nil, err
			}
			fixedPaths = append(fixedPaths, s.pagePath(page.Summary.Slug))
		}
		if !strings.Contains(currentIndex, fmt.Sprintf("../pages/%s.md", page.Summary.Slug)) {
			issues = append(issues, LintIssue{
				Kind:            IssueKindMissingIndex,
				PageSlug:        page.Summary.Slug,
				Message:         "page was missing from the knowledge index",
				AutoFixed:       true,
				Severity:        "low",
				Category:        "auto_fixed",
				SuggestedAction: "index membership was rebuilt automatically",
			})
		}
		normalizedTitle := normalizeTopicKey(page.Summary.Title)
		if existingSlug, exists := seenTitles[normalizedTitle]; normalizedTitle != "" && exists && existingSlug != page.Summary.Slug {
			issues = append(issues, LintIssue{
				Kind:            IssueKindDuplicateTopic,
				PageSlug:        page.Summary.Slug,
				Message:         fmt.Sprintf("topic duplicates %s", existingSlug),
				Severity:        "high",
				Category:        "review_required",
				RelatedPages:    []string{existingSlug, page.Summary.Slug},
				SuggestedAction: "merge or distinguish overlapping pages",
			})
			conflictPairs[page.Summary.Slug] = append(conflictPairs[page.Summary.Slug], existingSlug)
			conflictPairs[existingSlug] = append(conflictPairs[existingSlug], page.Summary.Slug)
			if strings.TrimSpace(page.Summary.Summary) != "" {
				issues = append(issues, LintIssue{
					Kind:            IssueKindConflictingClaim,
					PageSlug:        page.Summary.Slug,
					Message:         fmt.Sprintf("page may conflict with %s", existingSlug),
					Severity:        "high",
					Category:        "review_required",
					RelatedPages:    []string{existingSlug, page.Summary.Slug},
					SuggestedAction: "review conflicting claims and mark the canonical answer",
				})
			}
		} else if normalizedTitle != "" {
			seenTitles[normalizedTitle] = page.Summary.Slug
		}
		if len(page.Summary.SourceRefs) == 0 && page.Summary.PageType != PageTypeSynthesis {
			issues = append(issues, LintIssue{
				Kind:            IssueKindInsufficientSrc,
				PageSlug:        page.Summary.Slug,
				Message:         "page has no source references",
				Severity:        "medium",
				Category:        "review_required",
				SuggestedAction: "attach source refs or re-ingest supporting documents",
			})
		}
	}

	hasConcept := false
	hasEntity := false
	for idx := range pages {
		switch pages[idx].Summary.PageType {
		case PageTypeConcept:
			hasConcept = true
		case PageTypeEntity:
			hasEntity = true
		}
	}
	if !hasConcept {
		issues = append(issues, LintIssue{
			Kind:            IssueKindMissingConcept,
			Message:         "knowledge space has no concept pages yet",
			Severity:        "medium",
			Category:        "research_suggestions",
			SuggestedAction: "promote recurring topics into concept pages",
		})
	}
	if !hasEntity {
		issues = append(issues, LintIssue{
			Kind:            IssueKindMissingEntity,
			Message:         "knowledge space has no entity pages yet",
			Severity:        "medium",
			Category:        "research_suggestions",
			SuggestedAction: "add entity pages for durable actors or systems",
		})
	}

	for idx := range pages {
		page := &pages[idx]
		pairs := uniqueStrings(conflictPairs[page.Summary.Slug])
		if len(pairs) == 0 {
			continue
		}
		page.Summary.Status = KnowledgeStatusConflicted
		page.Summary.ConflictsWith = pairs
		page.Summary.UpdatedAt = s.now()
		if err := s.writeSinglePage(*page); err != nil {
			return nil, err
		}
		fixedPaths = append(fixedPaths, s.pagePath(page.Summary.Slug))
	}

	indexPath, err := s.writePagesIndex(pages)
	if err != nil {
		return nil, err
	}
	fixedPaths = append(fixedPaths, indexPath)
	report := &KnowledgeLintReport{
		GeneratedAt: s.now(),
		Issues:      issues,
		FixedPaths:  uniqueStrings(fixedPaths),
	}
	if err := s.writeLatestLintReport(report); err != nil {
		return nil, err
	}
	if err := s.appendLogEntry(KnowledgeLogEntry{
		Timestamp: s.now(),
		Operation: "lint",
		Title:     "Knowledge lint",
		Conflicts: uniqueConflictPages(issues),
		Gaps:      uniqueGapMessages(issues),
		Reason:    "knowledge lint completed",
	}); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Service) Answer(ctx context.Context, req AnswerRequest) (*KnowledgeAnswerReport, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	pages, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, err
	}
	selected := selectRelevantPages(pages, query, strings.TrimSpace(req.PageSlug), normalizeQueryScope(req.QueryScope), req.SelectedRefs)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no knowledge pages available")
	}
	answerText := synthesizeAnswer(query, selected)
	report := &KnowledgeAnswerReport{
		GeneratedAt:   s.now(),
		Query:         query,
		Answer:        answerText,
		Citations:     citationsFromPages(selected),
		RelatedPages:  pageSummaries(selected),
		Confidence:    answerConfidence(selected),
		ConflictNotes: conflictNotesFromPages(selected),
		OpenQuestions: openQuestionsForSelection(selected),
	}
	if req.ArchiveAnswer {
		archivedPath, err := s.archiveAnswer(ctx, report, selected[0].Summary.Slug)
		if err != nil {
			return nil, err
		}
		report.ArchivedPath = archivedPath
	}
	if err := s.appendLogEntry(KnowledgeLogEntry{
		Timestamp: s.now(),
		Operation: "query",
		Title:     query,
		Sources:   uniqueCitationRefs(report.Citations),
		Conflicts: append([]string(nil), report.ConflictNotes...),
		Gaps:      append([]string(nil), report.OpenQuestions...),
		Reason:    "query executed against compiled knowledge",
	}); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Service) PromoteQuery(ctx context.Context, jobID string) (*KnowledgePageSummary, error) {
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok {
		s.mu.Unlock()
		return nil, ErrJobNotFound
	}
	if job.Report == nil || job.Report.Answer == nil {
		s.mu.Unlock()
		return nil, ErrReportNotReady
	}
	answerReport := cloneJobReport(job.Report).Answer
	s.mu.Unlock()

	if err := s.ensureKnowledgeDirs(); err != nil {
		return nil, err
	}
	pages, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, err
	}
	pageMap := make(map[string]pageDocument, len(pages)+1)
	for _, page := range pages {
		pageMap[page.Summary.Slug] = page
	}
	slug := slugify(fmt.Sprintf("synthesis-%s", answerReport.Query))
	if _, exists := pageMap[slug]; exists {
		slug = fmt.Sprintf("%s-%d", slug, s.now().Unix())
	}
	summary := KnowledgePageSummary{
		Title:            buildPromotedPageTitle(answerReport.Query),
		Slug:             slug,
		PageType:         PageTypeSynthesis,
		Summary:          summarizeContent(answerReport.Answer),
		SourceRefs:       uniqueCitationRefs(answerReport.Citations),
		Keywords:         extractKeywords(answerReport.Query + "\n" + answerReport.Answer),
		GeneratedAt:      s.now(),
		UpdatedAt:        s.now(),
		SourceHash:       hashText(answerReport.Answer),
		Status:           KnowledgeStatusActive,
		Confidence:       answerReport.Confidence,
		DerivedFromQuery: answerReport.Query,
	}
	body := renderPromotedSynthesisBody(answerReport)
	pageMap[slug] = pageDocument{Summary: summary, Content: body}
	allPages := mapToSortedPages(pageMap)
	rebuildBacklinks(allPages)
	for _, page := range allPages {
		if err := s.writeSinglePage(page); err != nil {
			return nil, err
		}
	}
	if _, err := s.writePagesIndex(allPages); err != nil {
		return nil, err
	}
	answerReport.PromotedPageSlug = slug
	s.mu.Lock()
	if current, ok := s.jobs[jobID]; ok && current.Report != nil && current.Report.Answer != nil {
		current.Report.Answer.PromotedPageSlug = slug
	}
	s.mu.Unlock()
	if err := s.appendLogEntry(KnowledgeLogEntry{
		Timestamp: s.now(),
		Operation: "query",
		Title:     answerReport.Query,
		NewPages:  []string{slug},
		Sources:   uniqueCitationRefs(answerReport.Citations),
		Reason:    "promoted query result into synthesis page",
	}); err != nil {
		return nil, err
	}
	if err := s.rememberPages(ctx, []pageDocument{{Summary: summary, Content: body}}); err != nil {
		return nil, err
	}
	page := pageMap[slug]
	return &page.Summary, nil
}

func (s *Service) GetPage(_ context.Context, slug string) (*KnowledgePage, error) {
	path := s.pagePath(slug)
	body, err := os.ReadFile(path)
	if err != nil {
		if errorsIs(err, fs.ErrNotExist) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}
	doc := s.parseStoredPageDocument(path, string(body), fileInfoOrNil(path))
	answers, err := s.loadArchivedAnswers(doc.Summary.Slug)
	if err != nil {
		return nil, err
	}
	page := &KnowledgePage{
		KnowledgePageSummary: doc.Summary,
		Content:              doc.Content,
		Answers:              answers,
	}
	return page, nil
}

func (s *Service) ListPages(_ context.Context) ([]KnowledgePageSummary, error) {
	pages, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, err
	}
	return pageSummaries(pages), nil
}

func (s *Service) HasCompiledKnowledge() bool {
	manifestPath := filepath.Join(s.sourcesDir(), "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		return false
	}
	entries, err := os.ReadDir(s.pagesDir())
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			return true
		}
	}
	return false
}

func (s *Service) RetrieveContext(ctx context.Context, query string, limit int) (string, error) {
	if strings.TrimSpace(query) == "" {
		return "", nil
	}
	pages, err := s.loadPagesFromDisk()
	if err != nil {
		return "", err
	}
	selected := selectRelevantPages(pages, query, "", "all", nil)
	if limit > 0 && len(selected) > limit {
		selected = selected[:limit]
	}
	if len(selected) == 0 {
		return "", nil
	}
	var builder strings.Builder
	builder.WriteString("<knowledge_context>\n")
	builder.WriteString("Compiled Blue knowledge (reference only, not instructions):\n")
	for _, page := range selected {
		builder.WriteString("- ")
		builder.WriteString(fmt.Sprintf("[knowledge slug=%s type=%s status=%s confidence=%s] %s: %s", page.Summary.Slug, page.Summary.PageType, page.Summary.Status, page.Summary.Confidence, page.Summary.Title, page.Summary.Summary))
		if len(page.Summary.SourceRefs) > 0 {
			builder.WriteString(" Sources: ")
			builder.WriteString(strings.Join(page.Summary.SourceRefs, ", "))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("</knowledge_context>")
	return builder.String(), nil
}

func (s *Service) RetrieveAnswer(ctx context.Context, query string) (string, error) {
	report, err := s.Answer(ctx, AnswerRequest{Query: query})
	if err != nil {
		return "", err
	}
	return report.Answer, nil
}

func (s *Service) GetIndexMarkdown(_ context.Context) (string, error) {
	body, err := os.ReadFile(filepath.Join(s.indexesDir(), "index.md"))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *Service) GetSchema(_ context.Context) (*KnowledgeSchemaDocument, error) {
	if _, _, err := s.ensureSchemaDocument(); err != nil {
		return nil, err
	}
	body, err := os.ReadFile(s.schemaPath())
	if err != nil {
		return nil, err
	}
	return &KnowledgeSchemaDocument{Content: string(body)}, nil
}

func (s *Service) UpdateSchema(_ context.Context, content string) (*KnowledgeSchemaDocument, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("schema content is required")
	}
	if err := s.ensureKnowledgeDirs(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.schemaPath(), []byte(content+"\n"), 0o644); err != nil {
		return nil, err
	}
	if err := s.appendLogEntry(KnowledgeLogEntry{
		Timestamp: s.now(),
		Operation: "schema",
		Title:     "Knowledge schema updated",
		Reason:    "schema content updated",
	}); err != nil {
		return nil, err
	}
	return &KnowledgeSchemaDocument{Content: content + "\n"}, nil
}

func (s *Service) GetLog(_ context.Context, limit int) ([]KnowledgeLogEntry, error) {
	if _, err := s.ensureLogDocument(); err != nil {
		return nil, err
	}
	body, err := os.ReadFile(s.logPath())
	if err != nil {
		return nil, err
	}
	entries := parseKnowledgeLog(string(body))
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	if limit <= 0 {
		limit = 100
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (s *Service) GetLatestLint(_ context.Context) (*KnowledgeLintReport, error) {
	body, err := os.ReadFile(filepath.Join(s.lintDir(), "latest.json"))
	if err != nil {
		return nil, err
	}
	var report KnowledgeLintReport
	if err := json.Unmarshal(body, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func (s *Service) CreateJob(ctx context.Context, req CreateJobRequest) (*KnowledgeJob, error) {
	kind := normalizeJobKind(req.Kind)
	if kind == "" {
		kind = JobKindIngest
	}
	if kind == JobKindAnswer && strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query is required for answer jobs")
	}
	req.Kind = kind
	jobID := strings.TrimSpace(req.RequestedID)
	if jobID == "" {
		jobID = nextJobID()
	}
	now := s.now()
	job := &KnowledgeJob{
		ID:         jobID,
		UserID:     strings.TrimSpace(req.UserID),
		TenantID:   strings.TrimSpace(req.TenantID),
		ProviderID: strings.TrimSpace(req.ProviderID),
		Query:      strings.TrimSpace(req.Query),
		Kind:       kind,
		Status:     JobStatusPending,
		Stage:      "queued",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	runCtx, cancel := context.WithCancel(applyKnowledgeProviderRouting(context.Background(), req.ProviderID))
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.cancelFuncs[job.ID] = cancel
	s.mu.Unlock()
	s.publishJobEvent(job.ID, "job_created", cloneJob(job))
	go s.runJob(runCtx, job.ID, req)
	return cloneJob(job), nil
}

func (s *Service) ListJobsForUser(userID, tenantID string, activeOnly bool) ([]KnowledgeJobSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summaries := make([]KnowledgeJobSummary, 0, len(s.jobs))
	for _, job := range s.jobs {
		if !isAllowedActor(job, userID, tenantID) {
			continue
		}
		if activeOnly && isTerminalStatus(job.Status) {
			continue
		}
		summaries = append(summaries, KnowledgeJobSummary{
			ID:         job.ID,
			JobID:      job.ID,
			ProviderID: job.ProviderID,
			Query:      job.Query,
			Kind:       job.Kind,
			Status:     job.Status,
			Progress:   job.Progress,
			Stage:      job.Stage,
			UpdatedAt:  job.UpdatedAt,
		})
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})
	return summaries, nil
}

func (s *Service) GetJob(id string) (*KnowledgeJob, error) {
	return s.GetJobForUser(id, "", "")
}

func (s *Service) GetReport(id string) (*KnowledgeJobReport, error) {
	return s.GetReportForUser(id, "", "")
}

func (s *Service) GetJobForUser(id, userID, tenantID string) (*KnowledgeJob, error) {
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

func (s *Service) GetReportForUser(id, userID, tenantID string) (*KnowledgeJobReport, error) {
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
	return cloneJobReport(job.Report), nil
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
	ch := make(chan Event, 32)
	if s.subscribers[jobID] == nil {
		s.subscribers[jobID] = make(map[chan Event]struct{})
	}
	s.subscribers[jobID][ch] = struct{}{}
	if job, ok := s.jobs[jobID]; ok {
		if !isAllowedActor(job, userID, tenantID) {
			delete(s.subscribers[jobID], ch)
			if len(s.subscribers[jobID]) == 0 {
				delete(s.subscribers, jobID)
			}
			close(ch)
			return nil, nil, ErrJobForbidden
		}
		ch <- Event{
			ID:        nextEventID(),
			Type:      "job_snapshot",
			Timestamp: s.now(),
			Payload:   cloneJob(job),
		}
	}
	if terminal, ok := s.lastTerminal[jobID]; ok {
		ch <- terminal
	}
	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if subscribers, ok := s.subscribers[jobID]; ok {
			delete(subscribers, ch)
			close(ch)
			if len(subscribers) == 0 {
				delete(s.subscribers, jobID)
			}
		}
	}
	return ch, unsubscribe, nil
}

func (s *Service) runJob(ctx context.Context, jobID string, req CreateJobRequest) {
	s.updateJob(jobID, func(job *KnowledgeJob) {
		job.Status = JobStatusRunning
		job.Progress = 10
		job.Stage = "running"
	})
	s.publishJobEventByID(jobID, "job_started")

	var (
		report *KnowledgeJobReport
		err    error
	)
	switch normalizeJobKind(req.Kind) {
	case JobKindLint:
		var lintReport *KnowledgeLintReport
		lintReport, err = s.Lint(ctx, LintRequest{TargetPaths: append([]string(nil), req.TargetPaths...), ProviderID: req.ProviderID})
		if lintReport != nil {
			report = &KnowledgeJobReport{Kind: JobKindLint, Lint: lintReport}
		}
	case JobKindAnswer:
		var answerReport *KnowledgeAnswerReport
		answerReport, err = s.Answer(ctx, AnswerRequest{
			Query:         req.Query,
			PageSlug:      req.PageSlug,
			ArchiveAnswer: req.ArchiveAnswer,
			QueryScope:    req.QueryScope,
			SelectedRefs:  append([]string(nil), req.SelectedRefs...),
		})
		if answerReport != nil {
			report = &KnowledgeJobReport{Kind: JobKindAnswer, Answer: answerReport}
		}
	default:
		var ingestReport *KnowledgeIngestReport
		ingestReport, err = s.ingest(ctx, append([]string(nil), req.TargetPaths...))
		if ingestReport != nil {
			report = &KnowledgeJobReport{Kind: JobKindIngest, Ingest: ingestReport, Compile: ingestReport}
		}
	}
	if err != nil {
		status := JobStatusFailed
		if errorsIs(err, context.Canceled) {
			status = JobStatusCancelled
		}
		s.finishJob(jobID, status, report, err)
		if status == JobStatusCancelled {
			s.publishJobEventByID(jobID, "job_cancelled")
		} else {
			s.publishJobEventByID(jobID, "job_failed")
		}
		return
	}
	s.finishJob(jobID, JobStatusCompleted, report, nil)
	s.publishJobEventByID(jobID, "job_completed")
}

func (s *Service) finishJob(jobID string, status JobStatus, report *KnowledgeJobReport, jobErr error) {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return
	}
	job.Status = status
	job.Progress = 100
	job.Stage = string(status)
	job.UpdatedAt = now
	job.CompletedAt = &now
	job.Report = cloneJobReport(report)
	if report != nil {
		job.Kind = report.Kind
	}
	if jobErr != nil {
		job.Error = jobErr.Error()
	}
	delete(s.cancelFuncs, jobID)
}

func (s *Service) updateJob(jobID string, mutate func(job *KnowledgeJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return
	}
	mutate(job)
	job.UpdatedAt = s.now()
}

func (s *Service) publishJobEventByID(jobID string, eventType string) {
	s.mu.RLock()
	job := s.jobs[jobID]
	s.mu.RUnlock()
	if job == nil {
		return
	}
	s.publishJobEvent(jobID, eventType, cloneJob(job))
}

func (s *Service) publishJobEvent(jobID string, eventType string, payload interface{}) {
	event := Event{
		ID:        nextEventID(),
		Type:      eventType,
		Timestamp: s.now(),
		Payload:   payload,
	}
	s.mu.Lock()
	subscribers := s.subscribers[jobID]
	if strings.HasPrefix(eventType, "job_") && (eventType == "job_completed" || eventType == "job_failed" || eventType == "job_cancelled") {
		s.lastTerminal[jobID] = event
	}
	for ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	var userID string
	if job, ok := payload.(*KnowledgeJob); ok {
		userID = job.UserID
	}
	publisher := s.events
	s.mu.Unlock()
	if publisher != nil {
		publisher.Publish(userID, "knowledge."+eventType, payload)
	}
}

func (s *Service) ensureKnowledgeDirs() error {
	if err := os.MkdirAll(s.knowledgeRoot(), 0o755); err != nil {
		return err
	}
	for _, dir := range []string{s.pagesDir(), s.indexesDir(), s.answersDir(), s.lintDir(), s.sourcesDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureSchemaDocument() (string, string, error) {
	if err := s.ensureKnowledgeDirs(); err != nil {
		return "", "", err
	}
	path := s.schemaPath()
	if _, err := os.Stat(path); err == nil {
		body, readErr := os.ReadFile(path)
		return path, string(body), readErr
	}
	body := defaultSchemaMarkdown()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", "", err
	}
	return path, body, nil
}

func (s *Service) ensureLogDocument() (string, error) {
	if err := s.ensureKnowledgeDirs(); err != nil {
		return "", err
	}
	path := s.logPath()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.WriteFile(path, []byte("# Knowledge Log\n\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) appendLogEntry(entry KnowledgeLogEntry) error {
	if _, err := s.ensureLogDocument(); err != nil {
		return err
	}
	f, err := os.OpenFile(s.logPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	payload := renderLogEntry(entry)
	_, err = f.WriteString(payload)
	return err
}

func defaultSchemaMarkdown() string {
	return strings.Join([]string{
		"# Knowledge Space Schema",
		"",
		"- Organize durable knowledge as source summaries, entities, concepts, comparisons, syntheses, and decisions.",
		"- Ingest should update related pages incrementally instead of recompiling everything from scratch when possible.",
		"- Query results worth reusing should be promoted into synthesis pages.",
		"- Lint should flag conflicts, gaps, stale claims, and missing cross-links.",
		"",
	}, "\n")
}

func (s *Service) loadManifest() sourceManifest {
	body, err := os.ReadFile(filepath.Join(s.sourcesDir(), "manifest.json"))
	if err != nil {
		return sourceManifest{}
	}
	var manifest sourceManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return sourceManifest{}
	}
	return manifest
}

func (s *Service) discoverSources(targetPaths []string) ([]sourceDocument, error) {
	paths, err := s.expandSourcePaths(targetPaths)
	if err != nil {
		return nil, err
	}
	sources := make([]sourceDocument, 0, len(paths))
	for _, absPath := range paths {
		body, err := os.ReadFile(absPath)
		if err != nil {
			return nil, err
		}
		content := normalizeSourceContent(string(body))
		ref := s.sourceRef(absPath)
		title := extractTitle(content, absPath)
		keywords := extractKeywords(title + "\n" + content)
		sources = append(sources, sourceDocument{
			AbsPath:    absPath,
			Ref:        ref,
			Title:      title,
			PageType:   classifySourceType(ref),
			Content:    strings.TrimSpace(content),
			Summary:    summarizeContent(content),
			Keywords:   keywords,
			SourceHash: hashText(content),
			Slug:       slugify(strings.TrimSuffix(ref, filepath.Ext(ref))),
		})
	}
	sort.SliceStable(sources, func(i, j int) bool {
		return sources[i].Ref < sources[j].Ref
	})
	return sources, nil
}

func (s *Service) generatePagesForSource(ctx context.Context, schema string, source sourceDocument, existing []KnowledgePageSummary) ([]KnowledgePage, []string, []string, bool, error) {
	request := KnowledgeAuthorRequest{
		Schema: schema,
		Source: SourceSnapshot{
			Ref:        source.Ref,
			Title:      source.Title,
			PageType:   source.PageType,
			Content:    source.Content,
			Summary:    source.Summary,
			Keywords:   append([]string(nil), source.Keywords...),
			SourceHash: source.SourceHash,
			Slug:       source.Slug,
		},
		ExistingPages: existing,
		GeneratedAt:   s.now(),
		WorkspaceRoot: s.knowledgeRoot(),
	}
	if s.author != nil {
		result, err := s.author.AuthorDelta(ctx, request)
		if err == nil && result != nil && len(result.Pages) > 0 {
			return result.Pages, result.Conflicts, result.Gaps, result.FallbackMode, nil
		}
	}
	pages, conflicts, gaps := s.generateFallbackPagesForSource(source, existing)
	return pages, conflicts, gaps, true, nil
}

func (s *Service) generateFallbackPagesForSource(source sourceDocument, existing []KnowledgePageSummary) ([]KnowledgePage, []string, []string) {
	existingBySlug := make(map[string]KnowledgePageSummary, len(existing))
	for _, page := range existing {
		existingBySlug[page.Slug] = clonePageSummary(page)
	}

	pages := make([]KnowledgePage, 0, 4)
	conflicts := make([]string, 0)
	gaps := make([]string, 0, 2)

	sourceKeywords := mergeOrderedStrings(sourceTitleKeywords(source.Title), source.Keywords)
	if existingSummary, ok := existingBySlug[source.Slug]; ok && existingSummary.PageType != "" && existingSummary.PageType != PageTypeSourceSummary {
		conflicts = append(conflicts, fmt.Sprintf("%s already exists as %s", source.Slug, existingSummary.PageType))
	}
	pages = append(pages, s.buildFallbackPage(
		PageTypeSourceSummary,
		source.Title,
		source.Slug,
		source.Summary,
		source.Ref,
		source.SourceHash,
		sourceKeywords,
		renderFallbackSourceSummaryBody(source, sourceKeywords),
		existingBySlug[source.Slug],
	))

	entityTitles := extractEntityCandidates(source)
	if len(entityTitles) == 0 {
		gaps = append(gaps, fmt.Sprintf("No durable entity identified for %s", source.Ref))
	}
	for _, title := range entityTitles {
		slug := "entity-" + slugify(title)
		existingSummary := existingBySlug[slug]
		if existingSummary.PageType != "" && existingSummary.PageType != PageTypeEntity {
			conflicts = append(conflicts, fmt.Sprintf("%s already exists as %s", slug, existingSummary.PageType))
		}
		summary := buildDerivedPageSummary(PageTypeEntity, title, source, mergeSourceRefs(source.Ref, existingSummary.SourceRefs))
		pages = append(pages, s.buildFallbackPage(
			PageTypeEntity,
			title,
			slug,
			summary,
			source.Ref,
			source.SourceHash,
			mergeOrderedStrings([]string{strings.ToLower(title)}, sourceKeywords),
			renderFallbackDerivedBody(PageTypeEntity, title, summary, mergeSourceRefs(source.Ref, existingSummary.SourceRefs), source),
			existingSummary,
		))
	}

	conceptTitles := extractConceptCandidates(source, entityTitles)
	if len(conceptTitles) == 0 {
		gaps = append(gaps, fmt.Sprintf("No durable concept identified for %s", source.Ref))
	}
	for _, title := range conceptTitles {
		slug := "concept-" + slugify(title)
		existingSummary := existingBySlug[slug]
		if existingSummary.PageType != "" && existingSummary.PageType != PageTypeConcept {
			conflicts = append(conflicts, fmt.Sprintf("%s already exists as %s", slug, existingSummary.PageType))
		}
		summary := buildDerivedPageSummary(PageTypeConcept, title, source, mergeSourceRefs(source.Ref, existingSummary.SourceRefs))
		pages = append(pages, s.buildFallbackPage(
			PageTypeConcept,
			title,
			slug,
			summary,
			source.Ref,
			source.SourceHash,
			mergeOrderedStrings([]string{strings.ToLower(title)}, sourceKeywords),
			renderFallbackDerivedBody(PageTypeConcept, title, summary, mergeSourceRefs(source.Ref, existingSummary.SourceRefs), source),
			existingSummary,
		))
	}

	return dedupeKnowledgePages(pages), uniqueStrings(conflicts), uniqueStrings(gaps)
}

func (s *Service) buildFallbackPage(pageType string, title string, slug string, summary string, sourceRef string, sourceHash string, keywords []string, body string, existing KnowledgePageSummary) KnowledgePage {
	now := s.now()
	mergedSummary := mergeSummarySnippets(existing.Summary, summary)
	sourceRefs := mergeSourceRefs(sourceRef, existing.SourceRefs)
	pageKeywords := mergeOrderedStrings(keywords, existing.Keywords)
	generatedAt := now
	if !existing.GeneratedAt.IsZero() {
		generatedAt = existing.GeneratedAt.UTC()
	}
	status := KnowledgeStatusActive
	if existing.Status != "" {
		status = existing.Status
	}
	return KnowledgePage{
		KnowledgePageSummary: KnowledgePageSummary{
			Title:            title,
			Slug:             slug,
			PageType:         pageType,
			Summary:          mergedSummary,
			SourceRefs:       sourceRefs,
			Keywords:         pageKeywords,
			GeneratedAt:      generatedAt,
			UpdatedAt:        now,
			SourceHash:       sourceHash,
			Status:           status,
			Confidence:       KnowledgeConfidenceLow,
			ConflictsWith:    append([]string(nil), existing.ConflictsWith...),
			SupersededBy:     append([]string(nil), existing.SupersededBy...),
			DerivedFromQuery: existing.DerivedFromQuery,
		},
		Content: body,
	}
}

func renderFallbackSourceSummaryBody(source sourceDocument, keywords []string) string {
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(source.Title)
	builder.WriteString("\n\n## Summary\n\n")
	builder.WriteString(nonEmptyText(source.Summary, "No summary was extracted for this source yet."))
	builder.WriteString("\n\n## Source Metadata\n\n")
	builder.WriteString("- Ref: `")
	builder.WriteString(source.Ref)
	builder.WriteString("`\n")
	builder.WriteString("- Type: `")
	builder.WriteString(source.PageType)
	builder.WriteString("`\n")
	if len(keywords) > 0 {
		builder.WriteString("- Keywords: ")
		builder.WriteString(strings.Join(keywords, ", "))
		builder.WriteString("\n")
	}
	builder.WriteString("\n## Excerpt\n\n")
	builder.WriteString(strings.TrimSpace(source.Content))
	builder.WriteString("\n")
	return builder.String()
}

func renderFallbackDerivedBody(pageType string, title string, summary string, refs []string, source sourceDocument) string {
	sectionTitle := "Concept"
	sectionLabel := "recurring concept"
	if pageType == PageTypeEntity {
		sectionTitle = "Entity"
		sectionLabel = "durable entity"
	}
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(title)
	builder.WriteString("\n\n## ")
	builder.WriteString(sectionTitle)
	builder.WriteString(" Snapshot\n\n")
	builder.WriteString(summary)
	builder.WriteString("\n\n## Latest Evidence\n\n")
	builder.WriteString("- Derived from: `")
	builder.WriteString(source.Ref)
	builder.WriteString("`\n")
	builder.WriteString("- Source title: ")
	builder.WriteString(source.Title)
	builder.WriteString("\n- Interpretation: ")
	builder.WriteString(title)
	builder.WriteString(" is treated as a ")
	builder.WriteString(sectionLabel)
	builder.WriteString(" in this knowledge space.\n")
	builder.WriteString("\n## Supporting Sources\n\n")
	for _, ref := range refs {
		builder.WriteString("- `")
		builder.WriteString(ref)
		builder.WriteString("`\n")
	}
	return builder.String()
}

func buildDerivedPageSummary(pageType string, title string, source sourceDocument, refs []string) string {
	role := "recurring concept"
	if pageType == PageTypeEntity {
		role = "durable entity"
	}
	return strings.TrimSpace(fmt.Sprintf(
		"%s is a %s across %s. Latest ingest: %s",
		title,
		role,
		strings.Join(refs, ", "),
		firstSentence(nonEmptyText(source.Summary, source.Title)),
	))
}

func mergeSourceRefs(primary string, existing []string) []string {
	values := make([]string, 0, len(existing)+1)
	if strings.TrimSpace(primary) != "" {
		values = append(values, primary)
	}
	values = append(values, existing...)
	return mergeOrderedStrings(values)
}

func mergeOrderedStrings(groups ...[]string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, group := range groups {
		for _, value := range group {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

func mergeSummarySnippets(existing string, incoming string) string {
	parts := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	for _, value := range []string{existing, incoming} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, value)
	}
	merged := strings.TrimSpace(strings.Join(parts, " "))
	if len(merged) > 220 {
		return strings.TrimSpace(merged[:220]) + "..."
	}
	return merged
}

func dedupeKnowledgePages(values []KnowledgePage) []KnowledgePage {
	bySlug := make(map[string]KnowledgePage, len(values))
	order := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := bySlug[value.Slug]; !exists {
			order = append(order, value.Slug)
		}
		bySlug[value.Slug] = value
	}
	out := make([]KnowledgePage, 0, len(order))
	for _, slug := range order {
		out = append(out, bySlug[slug])
	}
	return out
}

func extractEntityCandidates(source sourceDocument) []string {
	counts := make(map[string]int)
	display := make(map[string]string)
	order := make([]string, 0)
	for _, token := range wordPattern.FindAllString(source.Content, -1) {
		if !capitalizedTokenRegexp.MatchString(token) {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(token))
		if _, skip := entityTokenStopwordSet[normalized]; skip {
			continue
		}
		if _, exists := counts[normalized]; !exists {
			order = append(order, normalized)
			display[normalized] = token
		}
		counts[normalized]++
	}
	sort.SliceStable(order, func(i, j int) bool {
		if counts[order[i]] == counts[order[j]] {
			return order[i] < order[j]
		}
		return counts[order[i]] > counts[order[j]]
	})
	out := make([]string, 0, 2)
	for _, normalized := range order {
		if counts[normalized] < 2 {
			continue
		}
		out = append(out, display[normalized])
		if len(out) == 2 {
			break
		}
	}
	return out
}

func extractConceptCandidates(source sourceDocument, entityTitles []string) []string {
	entitySet := make(map[string]struct{}, len(entityTitles))
	for _, title := range entityTitles {
		entitySet[strings.ToLower(strings.TrimSpace(title))] = struct{}{}
	}
	out := make([]string, 0, 2)
	seen := make(map[string]struct{})
	if len(entitySet) == 0 {
		return out
	}
	for _, token := range wordPattern.FindAllString(source.Title, -1) {
		normalized := strings.ToLower(strings.TrimSpace(token))
		if normalized == "" {
			continue
		}
		if _, isEntity := entitySet[normalized]; isEntity {
			continue
		}
		if _, skip := conceptTokenStopwordSet[normalized]; skip {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, toDisplayTitle(token))
		if len(out) == 2 {
			break
		}
	}
	return out
}

func sourceTitleKeywords(title string) []string {
	keywords := make([]string, 0, 3)
	for _, token := range wordPattern.FindAllString(title, -1) {
		normalized := strings.ToLower(strings.TrimSpace(token))
		if normalized == "" {
			continue
		}
		keywords = append(keywords, normalized)
	}
	return mergeOrderedStrings(keywords)
}

func toDisplayTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) == 0 {
		parts = []string{value}
	}
	for idx := range parts {
		part := strings.TrimSpace(parts[idx])
		if part == "" {
			continue
		}
		if part == strings.ToUpper(part) {
			parts[idx] = part
			continue
		}
		lower := strings.ToLower(part)
		parts[idx] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}

func nonEmptyText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) expandSourcePaths(targetPaths []string) ([]string, error) {
	if len(targetPaths) > 0 {
		return s.resolveExplicitTargets(targetPaths)
	}
	candidates := make([]string, 0)
	added := make(map[string]struct{})
	add := func(path string) {
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if _, exists := added[path]; exists {
			return
		}
		added[path] = struct{}{}
		candidates = append(candidates, path)
	}

	rootEntries, err := os.ReadDir(s.repoRoot)
	if err == nil {
		for _, entry := range rootEntries {
			if entry.IsDir() {
				continue
			}
			if isSupportedSourceFile(entry.Name()) {
				add(filepath.Join(s.repoRoot, entry.Name()))
			}
		}
	}
	for _, dir := range []string{
		filepath.Join(s.repoRoot, "docs-site"),
		filepath.Join(s.repoRoot, "docs"),
		filepath.Join(s.repoRoot, ".research", "web-query"),
	} {
		for _, path := range walkMarkdownFiles(dir, func(path string) bool { return false }) {
			add(path)
		}
	}
	if s.repoRoot != "" {
		entries, err := os.ReadDir(s.repoRoot)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				name := strings.ToLower(entry.Name())
				if strings.Contains(name, "context-pack") || strings.Contains(name, "context_pack") {
					for _, path := range walkMarkdownFiles(filepath.Join(s.repoRoot, entry.Name()), func(path string) bool { return false }) {
						add(path)
					}
				}
			}
		}
	}
	if s.workspaceDir != "" {
		knowledgeRoot := s.knowledgeRoot()
		for _, path := range walkMarkdownFiles(s.workspaceDir, func(path string) bool {
			clean := filepath.Clean(path)
			return clean == knowledgeRoot || strings.HasPrefix(clean, knowledgeRoot+string(os.PathSeparator))
		}) {
			add(path)
		}
	}
	sort.Strings(candidates)
	return candidates, nil
}

func (s *Service) resolveExplicitTargets(targetPaths []string) ([]string, error) {
	paths := make([]string, 0, len(targetPaths))
	seen := make(map[string]struct{})
	for _, target := range targetPaths {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		candidates := []string{target}
		if !filepath.IsAbs(target) {
			if s.repoRoot != "" {
				candidates = append(candidates, filepath.Join(s.repoRoot, target))
			}
			if s.workspaceDir != "" {
				candidates = append(candidates, filepath.Join(s.workspaceDir, target))
			}
		}
		var resolved string
		for _, candidate := range candidates {
			info, err := os.Stat(candidate)
			if err != nil {
				continue
			}
			if info.IsDir() {
				for _, path := range walkMarkdownFiles(candidate, func(path string) bool { return false }) {
					if _, exists := seen[path]; !exists {
						seen[path] = struct{}{}
						paths = append(paths, path)
					}
				}
				resolved = candidate
				break
			}
			if isSupportedSourceFile(candidate) {
				resolved = candidate
				break
			}
		}
		if resolved == "" {
			return nil, fmt.Errorf("knowledge source target not found: %s", target)
		}
		if info, err := os.Stat(resolved); err == nil && !info.IsDir() {
			resolved = filepath.Clean(resolved)
			if _, exists := seen[resolved]; !exists {
				seen[resolved] = struct{}{}
				paths = append(paths, resolved)
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func walkMarkdownFiles(root string, shouldSkip func(string) bool) []string {
	if root == "" {
		return nil
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}
	files := make([]string, 0)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if shouldSkip != nil && shouldSkip(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if isSupportedSourceFile(path) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func isSupportedSourceFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx") || strings.HasSuffix(lower, ".txt")
}

func normalizeSourceContent(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.TrimSpace(normalized)
}

func extractTitle(content string, absPath string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	base := filepath.Base(absPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func summarizeContent(content string) string {
	lines := strings.Split(content, "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts = append(parts, line)
		if len(strings.Join(parts, " ")) >= 180 {
			break
		}
	}
	summary := strings.TrimSpace(strings.Join(parts, " "))
	if len(summary) > 220 {
		summary = strings.TrimSpace(summary[:220]) + "..."
	}
	return summary
}

func extractKeywords(content string) []string {
	matches := tokenPattern.FindAllString(strings.ToLower(content), -1)
	counts := make(map[string]int)
	order := make([]string, 0)
	for _, match := range matches {
		if _, stopword := stopwordSet[match]; stopword {
			continue
		}
		if _, exists := counts[match]; !exists {
			order = append(order, match)
		}
		counts[match]++
	}
	sort.SliceStable(order, func(i, j int) bool {
		if counts[order[i]] == counts[order[j]] {
			return order[i] < order[j]
		}
		return counts[order[i]] > counts[order[j]]
	})
	if len(order) > 8 {
		order = order[:8]
	}
	return order
}

func classifySourceType(ref string) string {
	lower := strings.ToLower(ref)
	switch {
	case strings.HasPrefix(lower, ".research/"):
		return "research_artifact"
	case strings.Contains(lower, "context-pack") || strings.Contains(lower, "context_pack"):
		return "context_pack"
	case strings.Contains(lower, "architecture"):
		return "architecture"
	case strings.HasPrefix(lower, "docs-site/") || filepath.Base(lower) == "readme.md":
		return "product_doc"
	case strings.HasPrefix(lower, "workspace/"):
		return "workspace_note"
	default:
		return "note"
	}
}

func hashText(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:16])
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("\\", "-", "/", "-", "_", "-", ".", "-")
	value = replacer.Replace(value)
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "knowledge-page"
	}
	return result
}

func rebuildBacklinks(pages []pageDocument) {
	backlinks := computeBacklinks(pages)
	for idx := range pages {
		pages[idx].Summary.Backlinks = append([]string(nil), backlinks[pages[idx].Summary.Slug]...)
	}
}

func computeBacklinks(pages []pageDocument) map[string][]string {
	type keywordSet map[string]struct{}
	sets := make(map[string]keywordSet, len(pages))
	for _, page := range pages {
		set := make(keywordSet)
		for _, keyword := range page.Summary.Keywords {
			set[keyword] = struct{}{}
		}
		sets[page.Summary.Slug] = set
	}
	backlinks := make(map[string][]string, len(pages))
	for _, page := range pages {
		backlinks[page.Summary.Slug] = nil
	}
	for i := 0; i < len(pages); i++ {
		for j := i + 1; j < len(pages); j++ {
			left := pages[i]
			right := pages[j]
			if pagesOverlap(sets[left.Summary.Slug], sets[right.Summary.Slug]) {
				backlinks[left.Summary.Slug] = append(backlinks[left.Summary.Slug], right.Summary.Slug)
				backlinks[right.Summary.Slug] = append(backlinks[right.Summary.Slug], left.Summary.Slug)
			}
		}
	}
	for slug, refs := range backlinks {
		sort.Strings(refs)
		backlinks[slug] = uniqueStrings(refs)
	}
	return backlinks
}

func pagesOverlap(left, right map[string]struct{}) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	for keyword := range left {
		if _, ok := right[keyword]; ok {
			return true
		}
	}
	return false
}

func (s *Service) writeSinglePage(page pageDocument) error {
	return os.WriteFile(s.pagePath(page.Summary.Slug), []byte(renderPageDocument(page)), 0o644)
}

func (s *Service) writeManifest(manifest sourceManifest) (string, error) {
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(s.sourcesDir(), "manifest.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) writePagesIndex(pages []pageDocument) (string, error) {
	sort.SliceStable(pages, func(i, j int) bool {
		if pages[i].Summary.PageType == pages[j].Summary.PageType {
			return pages[i].Summary.Title < pages[j].Summary.Title
		}
		return pages[i].Summary.PageType < pages[j].Summary.PageType
	})
	grouped := map[string][]pageDocument{
		PageTypeSourceSummary: nil,
		PageTypeEntity:        nil,
		PageTypeConcept:       nil,
		PageTypeComparison:    nil,
		PageTypeSynthesis:     nil,
		PageTypeDecision:      nil,
	}
	for _, page := range pages {
		group := page.Summary.PageType
		if _, ok := grouped[group]; !ok {
			group = PageTypeSourceSummary
		}
		grouped[group] = append(grouped[group], page)
	}
	var builder strings.Builder
	builder.WriteString("# Knowledge Index\n\n")
	for _, section := range []struct {
		key   string
		label string
	}{
		{PageTypeSourceSummary, "Sources"},
		{PageTypeEntity, "Entities"},
		{PageTypeConcept, "Concepts"},
		{PageTypeComparison, "Comparisons"},
		{PageTypeSynthesis, "Syntheses"},
		{PageTypeDecision, "Decisions"},
	} {
		builder.WriteString("## ")
		builder.WriteString(section.label)
		builder.WriteString("\n\n")
		if len(grouped[section.key]) == 0 {
			builder.WriteString("- None yet.\n\n")
			continue
		}
		for _, page := range grouped[section.key] {
			builder.WriteString(fmt.Sprintf("- [%s](../pages/%s.md): %s\n", page.Summary.Title, page.Summary.Slug, page.Summary.Summary))
		}
		builder.WriteString("\n")
	}
	path := filepath.Join(s.indexesDir(), "index.md")
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) rememberPages(ctx context.Context, pages []pageDocument) error {
	if s.memorySink == nil {
		return nil
	}
	for _, page := range pages {
		content := fmt.Sprintf("%s: %s", page.Summary.Title, page.Summary.Summary)
		tags := []string{"knowledge", "knowledge-page", page.Summary.Slug}
		if err := s.memorySink.Remember(ctx, content, tags); err != nil {
			return err
		}
	}
	return nil
}

func pageSummaries(pages []pageDocument) []KnowledgePageSummary {
	summaries := make([]KnowledgePageSummary, 0, len(pages))
	for _, page := range pages {
		summaries = append(summaries, clonePageSummary(page.Summary))
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].PageType == summaries[j].PageType {
			return summaries[i].Title < summaries[j].Title
		}
		return summaries[i].PageType < summaries[j].PageType
	})
	return summaries
}

func pageSummariesFromMap(pages map[string]pageDocument) []KnowledgePageSummary {
	return pageSummaries(mapToSortedPages(pages))
}

func clonePageSummary(summary KnowledgePageSummary) KnowledgePageSummary {
	summary.SourceRefs = append([]string(nil), summary.SourceRefs...)
	summary.Keywords = append([]string(nil), summary.Keywords...)
	summary.Backlinks = append([]string(nil), summary.Backlinks...)
	summary.ConflictsWith = append([]string(nil), summary.ConflictsWith...)
	summary.SupersededBy = append([]string(nil), summary.SupersededBy...)
	return summary
}

func (s *Service) pageHasStaleHash(page pageDocument) (bool, string) {
	if len(page.Summary.SourceRefs) == 0 {
		return false, ""
	}
	path := s.resolveSourceRef(page.Summary.SourceRefs[0])
	if path == "" {
		return false, ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return false, ""
	}
	current := hashText(normalizeSourceContent(string(body)))
	if current != page.Summary.SourceHash {
		return true, "compiled page hash is stale relative to the current source"
	}
	return false, ""
}

func isLowQualityPage(page pageDocument) bool {
	bodyLength := len(strings.TrimSpace(page.Content))
	return strings.TrimSpace(page.Summary.Summary) == "" || bodyLength < 24
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	left = uniqueStrings(left)
	right = uniqueStrings(right)
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeTopicKey(title string) string {
	return strings.TrimSpace(strings.ToLower(title))
}

func (s *Service) loadPagesFromDisk() ([]pageDocument, error) {
	entries, err := os.ReadDir(s.pagesDir())
	if err != nil {
		if errorsIs(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	pages := make([]pageDocument, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(s.pagesDir(), entry.Name()))
		if err != nil {
			return nil, err
		}
		path := filepath.Join(s.pagesDir(), entry.Name())
		doc := s.parseStoredPageDocument(path, string(body), dirEntryInfoOrNil(entry))
		pages = append(pages, doc)
	}
	sort.SliceStable(pages, func(i, j int) bool {
		return pages[i].Summary.Slug < pages[j].Summary.Slug
	})
	return pages, nil
}

func selectRelevantPages(pages []pageDocument, query string, pageSlug string, scope string, selectedRefs []string) []pageDocument {
	scope = normalizeQueryScope(scope)
	if pageSlug != "" && scope != "selected_sources" {
		for _, page := range pages {
			if page.Summary.Slug == pageSlug {
				return append([]pageDocument{page}, topRelatedPages(pages, query, pageSlug, 2)...)
			}
		}
	}
	switch scope {
	case "current_page":
		if pageSlug != "" {
			for _, page := range pages {
				if page.Summary.Slug == pageSlug {
					return append([]pageDocument{page}, topRelatedPages(pages, query, pageSlug, 2)...)
				}
			}
		}
	case "selected_sources":
		filtered := filterPagesByRefs(pages, selectedRefs)
		if len(filtered) > 0 {
			pages = filtered
		}
	}
	type scoredPage struct {
		page  pageDocument
		score int
	}
	queryTerms := extractKeywords(query)
	scored := make([]scoredPage, 0, len(pages))
	for _, page := range pages {
		score := 1
		for _, term := range queryTerms {
			if strings.Contains(strings.ToLower(page.Summary.Title), term) || strings.Contains(strings.ToLower(page.Summary.Summary), term) {
				score += 3
			}
			for _, keyword := range page.Summary.Keywords {
				if keyword == term {
					score += 2
				}
			}
		}
		if page.Summary.Status == KnowledgeStatusConflicted {
			score--
		}
		scored = append(scored, scoredPage{page: page, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].page.Summary.Title < scored[j].page.Summary.Title
		}
		return scored[i].score > scored[j].score
	})
	limit := 3
	if len(scored) < limit {
		limit = len(scored)
	}
	selected := make([]pageDocument, 0, limit)
	for idx := 0; idx < limit; idx++ {
		selected = append(selected, scored[idx].page)
	}
	return selected
}

func filterPagesByRefs(pages []pageDocument, refs []string) []pageDocument {
	if len(refs) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			set[ref] = struct{}{}
		}
	}
	out := make([]pageDocument, 0)
	for _, page := range pages {
		for _, ref := range page.Summary.SourceRefs {
			if _, ok := set[ref]; ok {
				out = append(out, page)
				break
			}
		}
	}
	return out
}

func topRelatedPages(pages []pageDocument, query string, excludeSlug string, limit int) []pageDocument {
	selected := selectRelevantPages(filterPages(pages, excludeSlug), query, "", "all", nil)
	if len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func filterPages(pages []pageDocument, excludeSlug string) []pageDocument {
	out := make([]pageDocument, 0, len(pages))
	for _, page := range pages {
		if page.Summary.Slug != excludeSlug {
			out = append(out, page)
		}
	}
	return out
}

func synthesizeAnswer(query string, pages []pageDocument) string {
	if len(pages) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("Blue knowledge space suggests that ")
	builder.WriteString(firstSentence(pages[0].Summary.Summary))
	if len(pages) > 1 {
		builder.WriteString(" Related compiled pages add that ")
		extras := make([]string, 0, len(pages)-1)
		for _, page := range pages[1:] {
			if sentence := firstSentence(page.Summary.Summary); sentence != "" {
				extras = append(extras, sentence)
			}
		}
		builder.WriteString(strings.Join(extras, "; "))
	}
	builder.WriteString(". Question: ")
	builder.WriteString(strings.TrimSpace(query))
	builder.WriteString(".")
	return strings.TrimSpace(builder.String())
}

func firstSentence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "the compiled knowledge page does not yet have a distilled summary"
	}
	for _, sep := range []string{". ", "。", "\n"} {
		if idx := strings.Index(value, sep); idx > 0 {
			return strings.TrimSpace(value[:idx+1])
		}
	}
	if !strings.HasSuffix(value, ".") {
		return value + "."
	}
	return value
}

func citationsFromPages(pages []pageDocument) []KnowledgeCitation {
	citations := make([]KnowledgeCitation, 0, len(pages))
	for _, page := range pages {
		citations = append(citations, KnowledgeCitation{
			PageSlug:   page.Summary.Slug,
			Title:      page.Summary.Title,
			SourceRefs: append([]string(nil), page.Summary.SourceRefs...),
		})
	}
	return citations
}

func (s *Service) archiveAnswer(ctx context.Context, report *KnowledgeAnswerReport, pageSlug string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	slug := fmt.Sprintf("%s-%d", slugify(pageSlug), s.now().Unix())
	path := filepath.Join(s.answersDir(), slug+".md")
	answerDoc := archivedAnswerDocument{
		Answer: KnowledgeArchivedAnswer{
			Title:       "Knowledge Answer",
			Path:        path,
			PageSlug:    pageSlug,
			Query:       report.Query,
			Summary:     summarizeContent(report.Answer),
			GeneratedAt: report.GeneratedAt,
		},
		Body: report.Answer,
	}
	if err := os.WriteFile(path, []byte(renderArchivedAnswerDocument(answerDoc)), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) loadArchivedAnswers(pageSlug string) ([]KnowledgeArchivedAnswer, error) {
	entries, err := os.ReadDir(s.answersDir())
	if err != nil {
		if errorsIs(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	answers := make([]KnowledgeArchivedAnswer, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(s.answersDir(), entry.Name()))
		if err != nil {
			return nil, err
		}
		doc, err := parseArchivedAnswerDocument(string(body))
		if err != nil {
			continue
		}
		if doc.Answer.PageSlug != pageSlug {
			continue
		}
		if doc.Answer.Path == "" {
			doc.Answer.Path = filepath.Join(s.answersDir(), entry.Name())
		}
		answers = append(answers, doc.Answer)
	}
	sort.SliceStable(answers, func(i, j int) bool {
		return answers[i].GeneratedAt.After(answers[j].GeneratedAt)
	})
	return answers, nil
}

func (s *Service) writeLatestLintReport(report *KnowledgeLintReport) error {
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.lintDir(), "latest.json"), body, 0o644)
}

func (s *Service) resolveSourceRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if filepath.IsAbs(ref) {
		return ref
	}
	if strings.HasPrefix(ref, "workspace/") {
		if s.workspaceDir == "" {
			return ""
		}
		return filepath.Join(s.workspaceDir, strings.TrimPrefix(ref, "workspace/"))
	}
	if s.repoRoot == "" {
		return ""
	}
	return filepath.Join(s.repoRoot, ref)
}

func (s *Service) sourceRef(absPath string) string {
	absPath = filepath.Clean(absPath)
	if s.repoRoot != "" {
		if rel, err := filepath.Rel(s.repoRoot, absPath); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	if s.workspaceDir != "" {
		if rel, err := filepath.Rel(s.workspaceDir, absPath); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(filepath.Join("workspace", rel))
		}
	}
	return filepath.ToSlash(absPath)
}

func cloneJob(job *KnowledgeJob) *KnowledgeJob {
	if job == nil {
		return nil
	}
	cloned := *job
	if job.CompletedAt != nil {
		completedAt := *job.CompletedAt
		cloned.CompletedAt = &completedAt
	}
	cloned.Report = cloneJobReport(job.Report)
	return &cloned
}

func cloneJobReport(report *KnowledgeJobReport) *KnowledgeJobReport {
	if report == nil {
		return nil
	}
	cloned := *report
	if report.Ingest != nil {
		ingest := *report.Ingest
		ingest.Pages = append([]KnowledgePageSummary(nil), report.Ingest.Pages...)
		ingest.NewPages = append([]KnowledgePageSummary(nil), report.Ingest.NewPages...)
		ingest.UpdatedPages = append([]KnowledgePageSummary(nil), report.Ingest.UpdatedPages...)
		ingest.SkippedSources = append([]string(nil), report.Ingest.SkippedSources...)
		ingest.Conflicts = append([]string(nil), report.Ingest.Conflicts...)
		ingest.Gaps = append([]string(nil), report.Ingest.Gaps...)
		cloned.Ingest = &ingest
	}
	if report.Compile != nil {
		compile := *report.Compile
		compile.Pages = append([]KnowledgePageSummary(nil), report.Compile.Pages...)
		compile.NewPages = append([]KnowledgePageSummary(nil), report.Compile.NewPages...)
		compile.UpdatedPages = append([]KnowledgePageSummary(nil), report.Compile.UpdatedPages...)
		compile.SkippedSources = append([]string(nil), report.Compile.SkippedSources...)
		compile.Conflicts = append([]string(nil), report.Compile.Conflicts...)
		compile.Gaps = append([]string(nil), report.Compile.Gaps...)
		cloned.Compile = &compile
	}
	if report.Lint != nil {
		lint := *report.Lint
		lint.Issues = append([]LintIssue(nil), report.Lint.Issues...)
		lint.FixedPaths = append([]string(nil), report.Lint.FixedPaths...)
		cloned.Lint = &lint
	}
	if report.Answer != nil {
		answer := *report.Answer
		answer.Citations = append([]KnowledgeCitation(nil), report.Answer.Citations...)
		answer.RelatedPages = append([]KnowledgePageSummary(nil), report.Answer.RelatedPages...)
		answer.ConflictNotes = append([]string(nil), report.Answer.ConflictNotes...)
		answer.OpenQuestions = append([]string(nil), report.Answer.OpenQuestions...)
		cloned.Answer = &answer
	}
	return &cloned
}

func isAllowedActor(job *KnowledgeJob, userID, tenantID string) bool {
	if strings.TrimSpace(userID) != "" && job.UserID != "" && job.UserID != strings.TrimSpace(userID) {
		return false
	}
	if strings.TrimSpace(tenantID) != "" && job.TenantID != "" && job.TenantID != strings.TrimSpace(tenantID) {
		return false
	}
	return true
}

func isTerminalStatus(status JobStatus) bool {
	switch status {
	case JobStatusCompleted, JobStatusFailed, JobStatusCancelled:
		return true
	default:
		return false
	}
}

func nextEventID() string {
	return fmt.Sprintf("knowledge-event-%d", atomic.AddUint64(&eventCounter, 1))
}

func nextJobID() string {
	return fmt.Sprintf("knowledge-job-%d", atomic.AddUint64(&jobIDCounter, 1))
}

func errorsIs(err error, target error) bool {
	return err != nil && target != nil && (err == target || strings.Contains(err.Error(), target.Error()))
}

func normalizeJobKind(kind JobKind) JobKind {
	switch kind {
	case "", JobKindCompile, JobKindIngest:
		return JobKindIngest
	default:
		return kind
	}
}

func normalizeQueryScope(scope string) string {
	switch strings.TrimSpace(strings.ToLower(scope)) {
	case "", "all":
		return "all"
	case "current_page":
		return "current_page"
	case "selected_sources":
		return "selected_sources"
	default:
		return "all"
	}
}

func answerConfidence(pages []pageDocument) KnowledgeConfidence {
	switch {
	case len(pages) >= 3:
		return KnowledgeConfidenceHigh
	case len(pages) == 2:
		return KnowledgeConfidenceMedium
	default:
		return KnowledgeConfidenceLow
	}
}

func conflictNotesFromPages(pages []pageDocument) []string {
	notes := make([]string, 0)
	for _, page := range pages {
		if page.Summary.Status == KnowledgeStatusConflicted {
			notes = append(notes, fmt.Sprintf("%s has unresolved conflicts", page.Summary.Title))
		}
	}
	return uniqueStrings(notes)
}

func openQuestionsForSelection(pages []pageDocument) []string {
	if len(pages) >= 3 {
		return []string{}
	}
	return []string{"Consider ingesting more supporting sources or promoting a synthesis page"}
}

func buildPromotedPageTitle(query string) string {
	query = strings.TrimSpace(query)
	if len(query) > 72 {
		query = strings.TrimSpace(query[:72]) + "..."
	}
	return "Query Synthesis: " + query
}

func renderPromotedSynthesisBody(report *KnowledgeAnswerReport) string {
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(buildPromotedPageTitle(report.Query))
	builder.WriteString("\n\n")
	builder.WriteString(report.Answer)
	builder.WriteString("\n\n## Citations\n\n")
	for _, citation := range report.Citations {
		builder.WriteString("- ")
		builder.WriteString(citation.Title)
		if len(citation.SourceRefs) > 0 {
			builder.WriteString(" (")
			builder.WriteString(strings.Join(citation.SourceRefs, ", "))
			builder.WriteString(")")
		}
		builder.WriteString("\n")
	}
	if len(report.OpenQuestions) > 0 {
		builder.WriteString("\n## Open Questions\n\n")
		for _, question := range report.OpenQuestions {
			builder.WriteString("- ")
			builder.WriteString(question)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func buildIngestTitle(sources []sourceDocument, targetPaths []string) string {
	if len(targetPaths) == 1 {
		return fmt.Sprintf("Ingest | %s", strings.TrimSpace(targetPaths[0]))
	}
	if len(sources) == 1 {
		return fmt.Sprintf("Ingest | %s", sources[0].Title)
	}
	return "Knowledge ingest"
}

func buildIngestReason(skipped []string, newPages, updatedPages []KnowledgePageSummary) string {
	if len(newPages) == 0 && len(updatedPages) == 0 && len(skipped) > 0 {
		return "skipped unchanged source"
	}
	return "ingest updated the knowledge space"
}

func dedupePageSummaries(values []KnowledgePageSummary) []KnowledgePageSummary {
	bySlug := make(map[string]KnowledgePageSummary, len(values))
	for _, value := range values {
		bySlug[value.Slug] = clonePageSummary(value)
	}
	out := make([]KnowledgePageSummary, 0, len(bySlug))
	for _, value := range bySlug {
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

func pageSlugs(values []KnowledgePageSummary) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.Slug)
	}
	return uniqueStrings(out)
}

func sourceRefsFromSources(values []sourceDocument) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.Ref)
	}
	return uniqueStrings(out)
}

func uniqueCitationRefs(values []KnowledgeCitation) []string {
	out := make([]string, 0)
	for _, value := range values {
		out = append(out, value.SourceRefs...)
	}
	return uniqueStrings(out)
}

func uniqueConflictPages(issues []LintIssue) []string {
	out := make([]string, 0)
	for _, issue := range issues {
		if issue.Category == "review_required" && (issue.Kind == IssueKindDuplicateTopic || issue.Kind == IssueKindConflictingClaim) {
			if issue.PageSlug != "" {
				out = append(out, issue.PageSlug)
			}
			out = append(out, issue.RelatedPages...)
		}
	}
	return uniqueStrings(out)
}

func uniqueGapMessages(issues []LintIssue) []string {
	out := make([]string, 0)
	for _, issue := range issues {
		if issue.Category == "research_suggestions" {
			out = append(out, issue.Message)
		}
	}
	return uniqueStrings(out)
}

func hasPageType(pages []KnowledgePageSummary, want string) bool {
	for _, page := range pages {
		if page.PageType == want {
			return true
		}
	}
	return false
}

func manifestRecordsFromMap(records map[string]sourceManifestRecord) []sourceManifestRecord {
	out := make([]sourceManifestRecord, 0, len(records))
	for _, record := range records {
		out = append(out, record)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Ref < out[j].Ref
	})
	return out
}

func mapToSortedPages(pageMap map[string]pageDocument) []pageDocument {
	out := make([]pageDocument, 0, len(pageMap))
	for _, page := range pageMap {
		out = append(out, page)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Summary.Slug < out[j].Summary.Slug
	})
	return out
}

func renderLogEntry(entry KnowledgeLogEntry) string {
	var builder strings.Builder
	builder.WriteString("## [")
	builder.WriteString(entry.Timestamp.UTC().Format(time.RFC3339))
	builder.WriteString("] ")
	builder.WriteString(strings.TrimSpace(entry.Operation))
	builder.WriteString(" | ")
	builder.WriteString(strings.TrimSpace(entry.Title))
	builder.WriteString("\n")
	builder.WriteString("sources: ")
	builder.WriteString(strings.Join(uniqueStrings(entry.Sources), ", "))
	builder.WriteString("\n")
	builder.WriteString("new_pages: ")
	builder.WriteString(strings.Join(uniqueStrings(entry.NewPages), ", "))
	builder.WriteString("\n")
	builder.WriteString("updated_pages: ")
	builder.WriteString(strings.Join(uniqueStrings(entry.UpdatedPages), ", "))
	builder.WriteString("\n")
	builder.WriteString("conflicts: ")
	builder.WriteString(strings.Join(uniqueStrings(entry.Conflicts), ", "))
	builder.WriteString("\n")
	builder.WriteString("gaps: ")
	builder.WriteString(strings.Join(uniqueStrings(entry.Gaps), ", "))
	builder.WriteString("\n")
	builder.WriteString("reason: ")
	builder.WriteString(strings.TrimSpace(entry.Reason))
	builder.WriteString("\n\n")
	return builder.String()
}

func parseKnowledgeLog(raw string) []KnowledgeLogEntry {
	blocks := strings.Split(raw, "\n## [")
	entries := make([]KnowledgeLogEntry, 0)
	for idx, block := range blocks {
		if idx == 0 {
			if !strings.HasPrefix(strings.TrimSpace(block), "## [") {
				continue
			}
		} else {
			block = "## [" + block
		}
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) == 0 || !strings.HasPrefix(lines[0], "## [") {
			continue
		}
		entry := KnowledgeLogEntry{}
		header := strings.TrimPrefix(lines[0], "## [")
		parts := strings.SplitN(header, "] ", 2)
		if len(parts) != 2 {
			continue
		}
		if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[0])); err == nil {
			entry.Timestamp = ts.UTC()
		}
		opParts := strings.SplitN(parts[1], " | ", 2)
		entry.Operation = strings.TrimSpace(opParts[0])
		if len(opParts) == 2 {
			entry.Title = strings.TrimSpace(opParts[1])
		}
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			keyValue := strings.SplitN(line, ":", 2)
			if len(keyValue) != 2 {
				continue
			}
			key := strings.TrimSpace(keyValue[0])
			value := splitLogList(strings.TrimSpace(keyValue[1]))
			switch key {
			case "sources":
				entry.Sources = value
			case "new_pages":
				entry.NewPages = value
			case "updated_pages":
				entry.UpdatedPages = value
			case "conflicts":
				entry.Conflicts = value
			case "gaps":
				entry.Gaps = value
			case "reason":
				entry.Reason = strings.TrimSpace(keyValue[1])
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

func splitLogList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func (s *Service) parseStoredPageDocument(path string, raw string, info fs.FileInfo) pageDocument {
	doc, err := parsePageDocument(raw)
	if err == nil {
		if doc.Summary.Slug == "" {
			doc.Summary.Slug = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		return doc
	}
	return s.recoverMalformedPageDocument(path, raw, info)
}

func (s *Service) recoverMalformedPageDocument(path string, raw string, info fs.FileInfo) pageDocument {
	fields, content := parseLooseFrontmatterDocument(raw)
	slug := strings.TrimSpace(parseFrontmatterString(fields["slug"]))
	if slug == "" {
		slug = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	title := strings.TrimSpace(parseFrontmatterString(fields["title"]))
	if title == "" {
		title = extractTitle(content, path)
	}
	pageType := strings.TrimSpace(parseFrontmatterString(fields["page_type"]))
	if pageType == "" {
		pageType = PageTypeSourceSummary
	}
	summary := strings.TrimSpace(parseFrontmatterString(fields["summary"]))
	if summary == "" {
		summary = summarizeContent(content)
	}
	if summary == "" {
		summary = "Compiled page metadata is malformed. Re-run knowledge compile to repair this page."
	}
	sourceRefs := parseFrontmatterArray(fields["source_refs"])
	keywords := parseFrontmatterArray(fields["keywords"])
	if len(keywords) == 0 {
		keywords = extractKeywords(content)
	}
	sourceHash := strings.TrimSpace(parseFrontmatterString(fields["source_hash"]))
	if sourceHash == "" {
		sourceHash = hashText(normalizeSourceContent(content))
	}
	return pageDocument{
		Summary: KnowledgePageSummary{
			Title:            title,
			Slug:             slug,
			PageType:         pageType,
			Summary:          summary,
			SourceRefs:       sourceRefs,
			Keywords:         keywords,
			Backlinks:        parseFrontmatterArray(fields["backlinks"]),
			GeneratedAt:      parseFrontmatterTimeWithFallback(fields["generated_at"], info, s.now),
			UpdatedAt:        parseFrontmatterTimeWithFallback(fields["updated_at"], info, s.now),
			SourceHash:       sourceHash,
			Status:           KnowledgeStatus(parseFrontmatterString(fields["status"])),
			Confidence:       KnowledgeConfidence(parseFrontmatterString(fields["confidence"])),
			ConflictsWith:    parseFrontmatterArray(fields["conflicts_with"]),
			SupersededBy:     parseFrontmatterArray(fields["superseded_by"]),
			DerivedFromQuery: parseFrontmatterString(fields["derived_from_query"]),
		},
		Content: content,
	}
}

func parseFrontmatterTimeWithFallback(raw string, info fs.FileInfo, fallback func() time.Time) time.Time {
	if value := strings.TrimSpace(parseFrontmatterString(raw)); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed.UTC()
		}
	}
	if info != nil {
		return info.ModTime().UTC()
	}
	if fallback != nil {
		return fallback().UTC()
	}
	return time.Time{}
}

func dirEntryInfoOrNil(entry fs.DirEntry) fs.FileInfo {
	if entry == nil {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return nil
	}
	return info
}

func fileInfoOrNil(path string) fs.FileInfo {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	return info
}

func (s *Service) knowledgeRoot() string {
	return filepath.Join(s.workspaceDir, "knowledge")
}

func (s *Service) pagesDir() string {
	return filepath.Join(s.knowledgeRoot(), "pages")
}

func (s *Service) indexesDir() string {
	return filepath.Join(s.knowledgeRoot(), "indexes")
}

func (s *Service) answersDir() string {
	return filepath.Join(s.knowledgeRoot(), "answers")
}

func (s *Service) lintDir() string {
	return filepath.Join(s.knowledgeRoot(), "lint")
}

func (s *Service) sourcesDir() string {
	return filepath.Join(s.knowledgeRoot(), "sources")
}

func (s *Service) schemaPath() string {
	return filepath.Join(s.knowledgeRoot(), "SCHEMA.md")
}

func (s *Service) logPath() string {
	return filepath.Join(s.knowledgeRoot(), "log.md")
}

func (s *Service) pagePath(slug string) string {
	return filepath.Join(s.pagesDir(), slug+".md")
}
