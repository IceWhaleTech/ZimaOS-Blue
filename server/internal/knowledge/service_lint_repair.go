package knowledge

import (
	"context"
	"strings"
)

func (s *Service) repairPagesForLint(ctx context.Context, pages []pageDocument, targetPaths []string) ([]pageDocument, []string, error) {
	if s == nil || s.author == nil {
		return pages, nil, nil
	}
	targets := s.lintRepairTargets(pages, targetPaths)
	if len(targets) == 0 {
		return pages, nil, nil
	}
	report, err := s.ingestWithOptions(ctx, targets, true)
	if err != nil {
		return nil, nil, err
	}
	reloaded, err := s.loadPagesFromDisk()
	if err != nil {
		return nil, nil, err
	}
	fixedPaths := make([]string, 0, len(report.NewPages)+len(report.UpdatedPages)+1)
	for _, page := range report.NewPages {
		fixedPaths = append(fixedPaths, s.pagePath(page.Slug))
	}
	for _, page := range report.UpdatedPages {
		fixedPaths = append(fixedPaths, s.pagePath(page.Slug))
	}
	if strings.TrimSpace(report.IndexPath) != "" {
		fixedPaths = append(fixedPaths, report.IndexPath)
	}
	return reloaded, uniqueStrings(fixedPaths), nil
}

func (s *Service) lintRepairTargets(pages []pageDocument, targetPaths []string) []string {
	if s == nil || s.author == nil {
		return nil
	}
	if len(targetPaths) > 0 {
		return append([]string(nil), targetPaths...)
	}
	targets := make([]string, 0)
	for _, page := range pages {
		stale, _ := s.pageHasStaleHash(page)
		if !isLowQualityPage(page) && !stale {
			continue
		}
		for _, ref := range page.Summary.SourceRefs {
			if resolved := strings.TrimSpace(s.resolveSourceRef(ref)); resolved != "" {
				targets = append(targets, resolved)
			}
		}
	}
	return uniqueStrings(targets)
}
