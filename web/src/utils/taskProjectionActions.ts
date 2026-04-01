import type { UserTaskActionDescriptor, UserTaskActionID, UserTaskProjection } from '@/api/tasks'

type Translate = (key: string, fallback: string) => string
type UserTaskActionInput = NonNullable<UserTaskActionDescriptor['input']>
type UserTaskActionInputField = NonNullable<UserTaskActionInput['fields']>[number]

export type UserTaskActionVariant = 'default' | 'primary' | 'danger' | string

export interface ProjectedUserTaskAction extends UserTaskActionDescriptor {
  variant?: UserTaskActionVariant
}

function normalizeToken(value: unknown): string {
  return String(value ?? '')
    .trim()
    .toLowerCase()
}

function normalizeActionInputFields(
  fields: UserTaskActionInput['fields'] | null | undefined
): UserTaskActionInputField[] | undefined {
  if (!Array.isArray(fields)) return undefined
  const normalized: UserTaskActionInputField[] = []
  for (const field of fields) {
    const key = String(field?.key || '').trim()
    if (!key) continue
    normalized.push({
      key,
      label: String(field?.label || '').trim() || key,
      kind: String(field?.kind || '').trim() || undefined,
      target: String(field?.target || '').trim() || undefined,
      payload_key: String(field?.payload_key || '').trim() || undefined,
      required: Boolean(field?.required),
      placeholder: String(field?.placeholder || '').trim() || undefined,
      options: Array.isArray(field?.options) ? field.options.filter(Boolean) : undefined,
    })
  }
  return normalized
}

function defaultActionLabel(actionID: UserTaskActionID, translate: Translate): string {
  switch (normalizeToken(actionID)) {
    case 'resume':
      return translate('chat.taskResume', 'Resume')
    case 'cancel':
      return translate('chat.taskCancel', 'Cancel')
    case 'send_update':
      return translate('chat.taskSendUpdate', 'Send update')
    default:
      return String(actionID || '').trim()
  }
}

function defaultActionVariant(actionID: UserTaskActionID): UserTaskActionVariant {
  switch (normalizeToken(actionID)) {
    case 'resume':
      return 'primary'
    case 'cancel':
      return 'danger'
    default:
      return 'default'
  }
}

function actionDisplayRank(action: Pick<ProjectedUserTaskAction, 'variant' | 'id'>): number {
  switch (normalizeToken(action.variant)) {
    case 'primary':
      return 0
    case 'default':
      return 1
    case 'danger':
      return 2
    default:
      switch (normalizeToken(action.id)) {
        case 'resume':
          return 0
        case 'cancel':
          return 2
        default:
          return 1
      }
  }
}

function projectTaskActions(
  task: Pick<UserTaskProjection, 'id' | 'actions'>,
  translate: Translate
): ProjectedUserTaskAction[] {
  const source = Array.isArray(task.actions?.items) ? task.actions.items : []

  const deduped = new Map<string, ProjectedUserTaskAction>()
  for (const item of source) {
    const id = String(item?.id || '').trim()
    if (!id || deduped.has(id)) continue
    deduped.set(id, {
      ...item,
      id,
      label: String(item?.label || '').trim() || defaultActionLabel(id, translate),
      method: String(item?.method || '').trim() || 'POST',
      path: String(item?.path || '').trim(),
      variant: String(item?.variant || '').trim() || defaultActionVariant(id),
      input: item?.input
        ? {
            ...item.input,
            fields: normalizeActionInputFields(item.input.fields),
            title: String(item.input.title || '').trim() || undefined,
            description: String(item.input.description || '').trim() || undefined,
            submit_label: String(item.input.submit_label || '').trim() || undefined,
          }
        : undefined,
    })
  }

  return [...deduped.values()].sort((left, right) => {
    return actionDisplayRank(left) - actionDisplayRank(right)
  })
}

export function canOpenTaskConversation(
  task: Pick<UserTaskProjection, 'conversation_id' | 'scope'> | null | undefined
): boolean {
  return (
    Boolean(String(task?.conversation_id || '').trim()) &&
    String(task?.scope || '') === 'background'
  )
}

export function projectTaskControlActions(
  task: Pick<UserTaskProjection, 'id' | 'actions'>,
  translate: Translate
): ProjectedUserTaskAction[] {
  return projectTaskActions(task, translate)
}

export function findProjectedTaskAction(
  task: Pick<UserTaskProjection, 'id' | 'actions'>,
  actionID: UserTaskActionID,
  translate: Translate
): ProjectedUserTaskAction | null {
  const normalizedID = normalizeToken(actionID)
  if (!normalizedID) return null
  return (
    projectTaskActions(task, translate).find(
      (action) => normalizeToken(action.id) === normalizedID
    ) || null
  )
}
