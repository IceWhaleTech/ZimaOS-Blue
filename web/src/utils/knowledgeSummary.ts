import type { KnowledgeLintIssue, KnowledgePageSummary } from '@/api/knowledge'

const KNOWLEDGE_CONFLICT_ISSUE_KINDS = new Set(['duplicate_topic', 'conflicting_claim'])

export function isKnowledgeConflictIssue(issue: KnowledgeLintIssue | null | undefined): boolean {
  if (!issue) return false
  return (
    issue.category === 'review_required' && KNOWLEDGE_CONFLICT_ISSUE_KINDS.has(String(issue.kind || ''))
  )
}

export function countKnowledgeConflicts(
  pages: KnowledgePageSummary[],
  issues?: KnowledgeLintIssue[] | null
): number {
  const conflictedPages = pages.filter((page) => page.status === 'conflicted').length
  if (pages.length > 0) return conflictedPages

  const relatedPages = new Set<string>()
  for (const issue of issues || []) {
    if (!isKnowledgeConflictIssue(issue)) continue
    if (issue.page_slug) relatedPages.add(issue.page_slug)
    for (const relatedPage of issue.related_pages || []) {
      if (relatedPage) relatedPages.add(relatedPage)
    }
  }
  return relatedPages.size
}

export function countKnowledgeGaps(issues?: KnowledgeLintIssue[] | null): number {
  return (issues || []).filter((issue) => issue.category === 'research_suggestions').length
}
