import type { WorkspaceStats } from '@/api/workspace'

export function getWorkspaceVisibleTokenCount(
  stats: Pick<WorkspaceStats, 'files' | 'total_tokens'> | null | undefined,
  visibleFileNames: Iterable<string>
): number {
  if (!stats) return 0

  const allowed = new Set(
    Array.from(visibleFileNames)
      .map((name) => String(name || '').trim())
      .filter(Boolean)
  )

  const files = Array.isArray(stats.files) ? stats.files : []
  if (files.length === 0) {
    return typeof stats.total_tokens === 'number' ? stats.total_tokens : 0
  }

  return files.reduce((sum, file) => {
    const name = String(file?.name || '').trim()
    if (!allowed.has(name)) return sum
    const tokens = Number(file?.tokens)
    return sum + (Number.isFinite(tokens) ? tokens : 0)
  }, 0)
}
