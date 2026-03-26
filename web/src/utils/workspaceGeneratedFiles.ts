import type {
  TypelessCard,
  TypelessCardConvertTask,
  TypelessCardFile,
  TypelessCardResult,
} from '@/types/typeless'
import { isLocalAbsolutePath } from '@/utils/localPath'

const pathLikeKeys = [
  'path',
  'absolute_path',
  'file',
  'file_path',
  'output_path',
  'download_url',
  'local_path',
  'artifact_path',
  'reveal_path',
  'url',
]

function hasLikelyUrlScheme(value: string): boolean {
  return /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(value)
}

export function resolveWorkspaceGeneratedPathCandidate(
  value: string,
  workspaceRootPath: string
): string | null {
  const trimmed = value.trim().replace(/^['"`]+|['"`]+$/g, '')
  if (!trimmed) return null
  if (isLocalAbsolutePath(trimmed)) return trimmed
  if (!workspaceRootPath || !isLocalAbsolutePath(workspaceRootPath)) return null
  if (trimmed.startsWith('/api/') || hasLikelyUrlScheme(trimmed)) return null

  const relative = trimmed.replace(/^[.][\\/]+/, '').replace(/^[\\/]+/, '')
  if (!relative) return null
  if (relative.split(/[\\/]+/).some((segment) => segment === '..')) return null

  const root = workspaceRootPath.replace(/[\\/]+$/, '')
  const separator = root.includes('\\') && !root.includes('/') ? '\\' : '/'
  return `${root}${separator}${relative}`
}

function collectPathCandidates(value: unknown, workspaceRootPath: string, into: string[]): void {
  if (typeof value === 'string') {
    const path = resolveWorkspaceGeneratedPathCandidate(value, workspaceRootPath)
    if (path) into.push(path)
    return
  }

  if (Array.isArray(value)) {
    for (const item of value) collectPathCandidates(item, workspaceRootPath, into)
    return
  }

  if (!value || typeof value !== 'object') return

  const record = value as Record<string, unknown>
  for (const key of pathLikeKeys) {
    const candidate = record[key]
    if (typeof candidate !== 'string') continue
    const path = resolveWorkspaceGeneratedPathCandidate(candidate, workspaceRootPath)
    if (path) into.push(path)
  }
}

export function extractLocalPathCandidatesFromCard(
  card: TypelessCard,
  workspaceRootPath: string
): string[] {
  const paths: string[] = []

  if (card.type === 'file') {
    const fileCard = card as TypelessCardFile
    collectPathCandidates(fileCard.downloadUrl, workspaceRootPath, paths)
    collectPathCandidates(fileCard.previewUrl, workspaceRootPath, paths)
  }

  if (card.type === 'result') {
    const resultCard = card as TypelessCardResult
    for (const detail of resultCard.details || []) {
      const detailRecord = detail as unknown as Record<string, unknown>
      const hasExplicitLocation =
        typeof detailRecord.reveal_path === 'string' ||
        typeof detailRecord.local_path === 'string' ||
        typeof detailRecord.absolute_path === 'string'
      if (!hasExplicitLocation) {
        collectPathCandidates(detail.value, workspaceRootPath, paths)
      }
      collectPathCandidates(detail as unknown, workspaceRootPath, paths)
    }
  }

  if (card.type === 'convert-task') {
    const convertCard = card as TypelessCardConvertTask
    for (const output of convertCard.outputs || []) {
      collectPathCandidates(output.path, workspaceRootPath, paths)
      collectPathCandidates(output.download_url, workspaceRootPath, paths)
    }
  }

  const genericCard = card as unknown as Record<string, unknown>
  collectPathCandidates(genericCard.artifacts, workspaceRootPath, paths)
  collectPathCandidates(genericCard.download_url, workspaceRootPath, paths)

  return Array.from(new Set(paths))
}
