import type { UserTaskProjection } from '@/api/tasks'

type Translate = (key: string, fallback: string) => string

export interface ProjectedHarnessQuickLink {
  id: 'detail' | 'rerun'
  label: string
  href: string
}

function normalizeToken(value: unknown): string {
  return String(value ?? '')
    .trim()
    .toLowerCase()
}

function formatHarnessStatusToken(value: unknown, translate: Translate): string {
  switch (normalizeToken(value)) {
    case 'planning':
    case 'planned':
    case 'pending':
    case 'queued':
      return translate('chat.taskStagePlanning', 'Planning')
    case 'running':
    case 'working':
    case 'executing':
      return translate('chat.taskStageWorking', 'Working')
    case 'verifying':
    case 'scoring':
      return translate('chat.taskStageVerifying', 'Verifying')
    case 'completed':
      return translate('chat.taskStageCompleted', 'Completed')
    case 'passed':
      return translate('chat.taskHarnessVerificationPassed', 'Passed')
    case 'partial':
      return translate('chat.taskStagePartial', 'Partially passed')
    case 'failed':
      return translate('chat.taskStageFailed', 'Failed')
    case 'cancelled':
      return translate('chat.taskStageCancelled', 'Cancelled')
    default:
      return String(value ?? '').trim()
  }
}

function detailHref(task: Pick<UserTaskProjection, 'detail_href'>): string {
  return String(task.detail_href || '').trim()
}

export function hasHarnessProjectionDetail(task: Pick<UserTaskProjection, 'detail_href'>): boolean {
  return detailHref(task).length > 0
}

export function projectHarnessSummary(
  task: Pick<
    UserTaskProjection,
    'run_status' | 'verification_status' | 'score' | 'evidence_count' | 'detail_href'
  >,
  translate: Translate
): string[] {
  if (!hasHarnessProjectionDetail(task)) return []

  const parts: string[] = []
  if (task.run_status) {
    parts.push(
      `${translate('chat.taskHarnessRunStatus', 'Run')}: ${formatHarnessStatusToken(task.run_status, translate)}`
    )
  }
  if (task.verification_status) {
    parts.push(
      `${translate('chat.taskHarnessVerificationStatus', 'Verification')}: ${formatHarnessStatusToken(task.verification_status, translate)}`
    )
  }
  if (typeof task.score === 'number' && Number.isFinite(task.score)) {
    parts.push(`${translate('harness.groups.score', 'Score')}: ${task.score.toFixed(2)}`)
  }
  if (typeof task.evidence_count === 'number' && Number.isFinite(task.evidence_count)) {
    parts.push(`${translate('chat.taskHarnessEvidenceCount', 'Evidence')}: ${task.evidence_count}`)
  }
  parts.push(translate('chat.taskHarnessRecorded', 'Recorded'))
  return parts
}

export function projectHarnessQuickLinks(
  task: Pick<UserTaskProjection, 'detail_href' | 'status'>,
  translate: Translate
): ProjectedHarnessQuickLink[] {
  const href = detailHref(task)
  if (!href) return []

  const links: ProjectedHarnessQuickLink[] = [
    {
      id: 'detail',
      label: translate('chat.taskHarnessViewDetails', 'View run details'),
      href,
    },
  ]

  if (task.status !== 'running' && task.status !== 'waiting_user') {
    links.push({
      id: 'rerun',
      label: translate('chat.taskHarnessRerun', 'Rerun'),
      href: `${href}#retry`,
    })
  }

  return links
}
