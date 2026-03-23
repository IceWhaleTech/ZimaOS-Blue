import type { UserTaskKind } from '@/api/tasks'
import {
  localizeDeepResearchSegment,
  splitDeepResearchSegments,
} from '@/utils/deepResearchText'

type Translate = (key: string, fallback: string) => string

const AGENT_RUNTIME_KEYS: Record<string, [key: string, fallback: string]> = {
  intake: ['chat.taskRuntimeIntake', 'Intake'],
  clarify: ['chat.taskRuntimeClarify', 'Clarifying'],
  plan: ['chat.taskRuntimePlan', 'Planning'],
  confirm_gate: ['chat.taskRuntimeConfirmGate', 'Waiting for confirmation'],
  execute: ['chat.taskRuntimeExecute', 'Executing'],
  verify: ['chat.taskRuntimeVerify', 'Verifying'],
  reflect: ['chat.taskRuntimeReflect', 'Reflecting'],
  report: ['chat.taskRuntimeReport', 'Preparing report'],
  recover: ['chat.taskRuntimeRecover', 'Recovering'],
  done: ['chat.taskRuntimeDone', 'Completed'],
  aborted: ['chat.taskRuntimeAborted', 'Aborted'],
  pending: ['chat.taskRuntimePending', 'Pending'],
  planning: ['chat.taskStagePlanning', 'Planning'],
  executing: ['chat.taskStageWorking', 'Working'],
  waiting_input: ['chat.taskStageWaiting', 'Waiting'],
  completed: ['chat.taskStageCompleted', 'Completed'],
  failed: ['chat.taskStageFailed', 'Failed'],
  cancelled: ['chat.taskStageCancelled', 'Cancelled'],
}

const DEFAULT_TITLE_KEYS: Record<string, [key: string, fallback: string]> = {
  research_task: ['chat.taskDefaultResearchTitle', 'Research task'],
  agent_task: ['chat.taskDefaultAgentTitle', 'Agent task'],
}

function normalizeToken(value: string): string {
  return value.trim().toLowerCase().replace(/[\s-]+/g, '_')
}

function translateKnownToken(
  token: string,
  table: Record<string, [key: string, fallback: string]>,
  translate: Translate
): string {
  const entry = table[token]
  return entry ? translate(entry[0], entry[1]) : ''
}

function localizeAgentSegment(segment: string, translate: Translate): string {
  const token = normalizeToken(segment)
  return translateKnownToken(token, AGENT_RUNTIME_KEYS, translate) || segment
}

export function localizeTaskProjectionTitle(
  title: string | null | undefined,
  kind: UserTaskKind,
  translate: Translate
): string {
  const trimmed = String(title || '').trim()
  if (!trimmed) {
    return kind === 'research'
      ? translate('chat.taskDefaultResearchTitle', 'Research task')
      : translate('chat.taskDefaultAgentTitle', 'Agent task')
  }
  const normalized = normalizeToken(trimmed)
  return translateKnownToken(normalized, DEFAULT_TITLE_KEYS, translate) || trimmed
}

export function localizeTaskProjectionSubtitle(
  subtitle: string | null | undefined,
  kind: UserTaskKind,
  translate: Translate
): string {
  const trimmed = String(subtitle || '').trim()
  if (!trimmed) return ''
  const parts = splitDeepResearchSegments(trimmed)

  if (parts.length <= 1) {
    return kind === 'research'
      ? localizeDeepResearchSegment(trimmed, translate)
      : localizeAgentSegment(trimmed, translate)
  }

  return parts
    .map((part) =>
      kind === 'research'
        ? localizeDeepResearchSegment(part, translate)
        : localizeAgentSegment(part, translate)
    )
    .join(' · ')
}
