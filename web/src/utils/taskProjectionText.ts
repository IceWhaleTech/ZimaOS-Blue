import type { UserTaskKind } from '@/api/tasks'
import { localizeDeepResearchSegment, splitDeepResearchSegments } from '@/utils/deepResearchText'

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
  research_task: ['chat.taskDefaultResearchTitle', 'Deep Research task'],
  agent_task: ['chat.taskDefaultAgentTitle', 'Agent task'],
  workflow: ['chat.taskDefaultWorkflowTitle', 'Workflow task'],
}

const TASK_MESSAGE_PREFIX_KEYS: Record<string, [key: string, fallback: string]> = {
  task_failed: ['chat.taskFailed', 'Task failed'],
  task_failed_during_clarification: [
    'chat.taskFailedDuringClarification',
    'Task failed during clarification',
  ],
  task_failed_during_planning: ['chat.taskFailedDuringPlanning', 'Task failed during planning'],
  task_failed_during_confirmation: [
    'chat.taskFailedDuringConfirmation',
    'Task failed during confirmation',
  ],
  task_failed_while_revising_the_plan: [
    'chat.taskFailedWhileRevisingPlan',
    'Task failed while revising the plan',
  ],
  verification_failed_and_recovery_did_not_succeed: [
    'chat.verificationFailedRecoveryFailed',
    'Verification failed and recovery did not succeed',
  ],
  task_failed_while_updating_runtime_state: [
    'chat.taskFailedWhileUpdatingRuntimeState',
    'Task failed while updating runtime state',
  ],
}

const TASK_FAILURE_REASON_KEYS: Record<string, [key: string, fallback: string]> = {
  grounded_verification_failed: [
    'chat.taskFailureGroundedVerificationFailed',
    'grounded verification failed',
  ],
}

function normalizeToken(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, '_')
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

function localizeTaskMessagePrefix(prefix: string, translate: Translate): string {
  const token = normalizeToken(prefix)
  return translateKnownToken(token, TASK_MESSAGE_PREFIX_KEYS, translate) || prefix
}

function localizeTaskFailureReason(reason: string, translate: Translate): string {
  const trimmed = String(reason || '').trim()
  if (!trimmed) return trimmed
  const token = normalizeToken(trimmed)
  const direct = translateKnownToken(token, TASK_FAILURE_REASON_KEYS, translate)
  if (direct) return direct

  // Replace known reasons inside longer free-form messages.
  return trimmed.replace(
    /\bgrounded verification failed\b/gi,
    translate('chat.taskFailureGroundedVerificationFailed', 'grounded verification failed')
  )
}

export function localizeTaskProjectionTitle(
  title: string | null | undefined,
  kind: UserTaskKind,
  translate: Translate
): string {
  const trimmed = String(title || '').trim()
  if (!trimmed) {
    switch (kind) {
      case 'research':
        return translate('chat.taskDefaultResearchTitle', 'Deep Research task')
      case 'workflow':
        return translate('chat.taskDefaultWorkflowTitle', 'Workflow task')
      default:
        return translate('chat.taskDefaultAgentTitle', 'Agent task')
    }
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

export function localizeTaskProjectionPreviewText(
  preview: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(preview || '').trim()
  if (!trimmed) return ''

  // Common agent/runtime failure message: "Task failed: <reason>"
  const colonIndex = trimmed.indexOf(':')
  if (colonIndex >= 0) {
    const rawPrefix = trimmed.slice(0, colonIndex).trim()
    const rawSuffix = trimmed.slice(colonIndex + 1).trim()
    const prefix = localizeTaskMessagePrefix(rawPrefix, translate)
    const suffix = localizeTaskFailureReason(rawSuffix, translate)
    if (!suffix) return prefix
    return `${prefix}: ${suffix}`
  }

  // If the preview itself is a known failure reason, localize it.
  return localizeTaskFailureReason(trimmed, translate)
}
