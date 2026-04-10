package knowledge

import (
	"context"
	"sort"
	"strings"
)

func (s *Service) RepairConflicts(ctx context.Context, req RepairConflictsRequest) (*KnowledgeConflictRepairReport, error) {
	notifyProgress := func(progress int, stage string, detail string) {
		if req.onProgress == nil {
			return
		}
		req.onProgress(clampKnowledgeJobProgress(progress), strings.TrimSpace(stage), strings.TrimSpace(detail))
	}

	ctx = applyKnowledgeProviderRouting(ctx, s.resolveLintProviderID(req.ProviderID))
	if err := s.ensureKnowledgeDirs(); err != nil {
		return nil, err
	}
	pages, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, err
	}

	groups := s.conflictRepairGroups(pages, req.TargetSlugs)
	notifyProgress(15, "scan_conflicts", "")
	if len(groups) == 0 {
		notifyProgress(92, "rerun_lint", "")
		lintReport, err := s.Lint(ctx, LintRequest{ProviderID: req.ProviderID})
		if err != nil {
			return nil, err
		}
		return &KnowledgeConflictRepairReport{
			GeneratedAt: s.now(),
			FixedPaths:  append([]string(nil), lintReport.FixedPaths...),
			Lint:        cloneLintReport(lintReport),
		}, nil
	}

	now := s.now()
	bySlug := make(map[string]*pageDocument, len(pages))
	for idx := range pages {
		bySlug[pages[idx].Summary.Slug] = &pages[idx]
	}

	canonicalSlugs := make([]string, 0)
	supersededSlugs := make([]string, 0)
	fixedPaths := make([]string, 0, len(pages)+1)

	for idx, group := range groups {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		notifyProgress(20+(idx*45)/len(groups), "resolve_group", strings.Join(uniqueStrings(group), ", "))
		canonicalSlug := pickCanonicalConflictPage(group, bySlug)
		canonical := bySlug[canonicalSlug]
		if canonical == nil {
			continue
		}
		canonicalSlugs = append(canonicalSlugs, canonicalSlug)

		mergedSourceRefs := append([]string(nil), canonical.Summary.SourceRefs...)
		mergedKeywords := append([]string(nil), canonical.Summary.Keywords...)
		for _, slug := range group {
			page := bySlug[slug]
			if page == nil {
				continue
			}
			if slug != canonicalSlug {
				page.Summary.Status = KnowledgeStatusSuperseded
				page.Summary.ConflictsWith = nil
				page.Summary.SupersededBy = uniqueStrings(append(page.Summary.SupersededBy, canonicalSlug))
				page.Summary.UpdatedAt = now
				supersededSlugs = append(supersededSlugs, slug)
				mergedSourceRefs = append(mergedSourceRefs, page.Summary.SourceRefs...)
				mergedKeywords = append(mergedKeywords, page.Summary.Keywords...)
			}
		}

		canonical.Summary.Status = KnowledgeStatusActive
		canonical.Summary.ConflictsWith = nil
		canonical.Summary.SupersededBy = nil
		canonical.Summary.SourceRefs = uniqueStrings(mergedSourceRefs)
		canonical.Summary.Keywords = uniqueStrings(mergedKeywords)
		canonical.Summary.UpdatedAt = now
	}

	notifyProgress(78, "write_pages", "")
	backlinks := computeBacklinks(pages)
	for idx := range pages {
		pages[idx].Summary.Backlinks = append([]string(nil), backlinks[pages[idx].Summary.Slug]...)
		if err := s.writeSinglePage(pages[idx]); err != nil {
			return nil, err
		}
		fixedPaths = append(fixedPaths, s.pagePath(pages[idx].Summary.Slug))
	}

	indexPath, err := s.writePagesIndex(pages)
	if err != nil {
		return nil, err
	}
	fixedPaths = append(fixedPaths, indexPath)

	notifyProgress(92, "rerun_lint", "")
	lintReport, err := s.Lint(ctx, LintRequest{ProviderID: req.ProviderID})
	if err != nil {
		return nil, err
	}
	fixedPaths = append(fixedPaths, lintReport.FixedPaths...)

	reloaded, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, err
	}
	canonicalPages := pageSummariesBySlugSet(reloaded, canonicalSlugs)
	supersededPages := pageSummariesBySlugSet(reloaded, supersededSlugs)
	report := &KnowledgeConflictRepairReport{
		GeneratedAt:     s.now(),
		CanonicalPages:  canonicalPages,
		SupersededPages: supersededPages,
		FixedPaths:      uniqueStrings(fixedPaths),
		Lint:            cloneLintReport(lintReport),
	}
	if err := s.appendLogEntry(KnowledgeLogEntry{
		Timestamp:    s.now(),
		Operation:    "repair_conflicts",
		Title:        "Knowledge conflict repair",
		UpdatedPages: uniqueStrings(append(pageSlugs(canonicalPages), pageSlugs(supersededPages)...)),
		Conflicts:    uniqueConflictPages(lintReport.Issues),
		Gaps:         uniqueGapMessages(lintReport.Issues),
		Reason:       "knowledge conflict repair completed",
	}); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *Service) conflictRepairGroups(pages []pageDocument, targetSlugs []string) [][]string {
	targetSet := make(map[string]struct{}, len(targetSlugs))
	for _, slug := range targetSlugs {
		slug = strings.TrimSpace(slug)
		if slug != "" {
			targetSet[slug] = struct{}{}
		}
	}

	adjacency := make(map[string]map[string]struct{})
	titleGroups := make(map[string][]string)
	for _, page := range pages {
		slug := strings.TrimSpace(page.Summary.Slug)
		if slug == "" || page.Summary.Status == KnowledgeStatusSuperseded {
			continue
		}
		for _, conflict := range page.Summary.ConflictsWith {
			addConflictRepairEdge(adjacency, slug, conflict)
		}
		if titleKey := normalizeTopicKey(page.Summary.Title); titleKey != "" {
			titleGroups[titleKey] = append(titleGroups[titleKey], slug)
		}
	}
	for _, group := range titleGroups {
		group = uniqueStrings(group)
		if len(group) < 2 {
			continue
		}
		for idx := range group {
			for j := idx + 1; j < len(group); j++ {
				addConflictRepairEdge(adjacency, group[idx], group[j])
			}
		}
	}

	visited := make(map[string]struct{}, len(adjacency))
	groups := make([][]string, 0)
	for slug := range adjacency {
		if _, ok := visited[slug]; ok {
			continue
		}
		stack := []string{slug}
		component := make([]string, 0)
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if _, ok := visited[current]; ok {
				continue
			}
			visited[current] = struct{}{}
			component = append(component, current)
			for next := range adjacency[current] {
				if _, ok := visited[next]; !ok {
					stack = append(stack, next)
				}
			}
		}
		component = uniqueStrings(component)
		if len(component) < 2 {
			continue
		}
		if len(targetSet) > 0 && !conflictRepairGroupIntersects(component, targetSet) {
			continue
		}
		groups = append(groups, component)
	}

	sort.SliceStable(groups, func(i, j int) bool {
		return strings.Join(groups[i], ",") < strings.Join(groups[j], ",")
	})
	return groups
}

func addConflictRepairEdge(adjacency map[string]map[string]struct{}, left string, right string) {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" || left == right {
		return
	}
	if adjacency[left] == nil {
		adjacency[left] = make(map[string]struct{})
	}
	if adjacency[right] == nil {
		adjacency[right] = make(map[string]struct{})
	}
	adjacency[left][right] = struct{}{}
	adjacency[right][left] = struct{}{}
}

func conflictRepairGroupIntersects(group []string, targetSet map[string]struct{}) bool {
	if len(targetSet) == 0 {
		return true
	}
	for _, slug := range group {
		if _, ok := targetSet[slug]; ok {
			return true
		}
	}
	return false
}

func pickCanonicalConflictPage(group []string, bySlug map[string]*pageDocument) string {
	candidates := uniqueStrings(group)
	sort.SliceStable(candidates, func(i, j int) bool {
		left := bySlug[candidates[i]]
		right := bySlug[candidates[j]]
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		leftConfidence := conflictRepairConfidenceRank(left.Summary.Confidence)
		rightConfidence := conflictRepairConfidenceRank(right.Summary.Confidence)
		if leftConfidence != rightConfidence {
			return leftConfidence > rightConfidence
		}
		if len(left.Summary.SourceRefs) != len(right.Summary.SourceRefs) {
			return len(left.Summary.SourceRefs) > len(right.Summary.SourceRefs)
		}
		if len(strings.TrimSpace(left.Content)) != len(strings.TrimSpace(right.Content)) {
			return len(strings.TrimSpace(left.Content)) > len(strings.TrimSpace(right.Content))
		}
		if len(strings.TrimSpace(left.Summary.Summary)) != len(strings.TrimSpace(right.Summary.Summary)) {
			return len(strings.TrimSpace(left.Summary.Summary)) > len(strings.TrimSpace(right.Summary.Summary))
		}
		if !left.Summary.UpdatedAt.Equal(right.Summary.UpdatedAt) {
			return left.Summary.UpdatedAt.After(right.Summary.UpdatedAt)
		}
		return left.Summary.Slug < right.Summary.Slug
	})
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func conflictRepairConfidenceRank(value KnowledgeConfidence) int {
	switch value {
	case KnowledgeConfidenceHigh:
		return 3
	case KnowledgeConfidenceMedium:
		return 2
	case KnowledgeConfidenceLow:
		return 1
	default:
		return 0
	}
}

func pageSummariesBySlugSet(pages []pageDocument, slugs []string) []KnowledgePageSummary {
	targets := make(map[string]struct{}, len(slugs))
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if slug != "" {
			targets[slug] = struct{}{}
		}
	}
	out := make([]KnowledgePageSummary, 0, len(targets))
	for _, page := range pages {
		if _, ok := targets[page.Summary.Slug]; ok {
			out = append(out, clonePageSummary(page.Summary))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}
