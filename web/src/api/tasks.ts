import api from './client'

export type UserTaskKind = 'agent_task' | 'research' | 'workflow'
export type UserTaskScope = 'current' | 'background' | 'all'
export type UserTaskStatus = 'running' | 'waiting_user' | 'completed' | 'failed' | 'cancelled'
export type UserTaskStage =
  | 'planning'
  | 'working'
  | 'verifying'
  | 'waiting_user'
  | 'completed'
  | 'partial'
  | 'failed'
  | 'cancelled'

export type UserTaskActionID = 'cancel' | 'resume' | 'send_update' | string
export type UserTaskActionVariant = 'default' | 'primary' | 'danger' | string

export interface UserTaskBlocker {
  kind: 'approval' | 'question' | string
  label: string
  pending_count: number
  modal_only: boolean
}

export interface UserTaskArtifact {
  kind: 'file' | 'url' | 'report' | 'snapshot' | string
  label: string
  url?: string
}

export interface UserTaskResearchSource {
  title: string
  url?: string
  domain?: string
  source_type?: string
  published_at?: string
  fetched_at?: string
  relevance_score?: number
  credibility_score?: number
}

export interface UserTaskSubagentSummary {
  total: number
  running?: number
  waiting_user?: number
  completed?: number
  failed?: number
  cancelled?: number
  latest_title?: string
  latest_status?: string
  latest_updated_at?: string
}

export interface UserTaskActionDescriptor {
  id: UserTaskActionID
  label: string
  method: string
  path: string
  variant?: UserTaskActionVariant
  requires_input?: boolean
  input?: {
    [key: string]: unknown
    fields?: Array<{
      key: string
      label: string
      kind?: 'choice' | 'text' | 'textarea' | 'json' | string
      target?: 'decision' | 'payload' | 'payload_root' | 'root' | string
      payload_key?: string
      required?: boolean
      placeholder?: string
      options?: string[]
    }>
    title?: string
    description?: string
    submit_label?: string
  }
}

export interface UserTaskActions {
  items?: UserTaskActionDescriptor[]
}

export interface UserTaskProjection {
  id: string
  kind: UserTaskKind
  conversation_id?: string
  scope: UserTaskScope
  title: string
  subtitle?: string
  status: UserTaskStatus
  stage: UserTaskStage
  progress: number
  blocker?: UserTaskBlocker
  result_preview?: string
  error_preview?: string
  artifacts?: UserTaskArtifact[]
  research_sources?: UserTaskResearchSource[]
  subagent_summary?: UserTaskSubagentSummary
  actions: UserTaskActions
  run_status?: string
  verification_status?: string
  score?: number
  evidence_count?: number
  detail_href?: string
  updated_at: string
  finished_at?: string
}

export const taskProjectionApi = {
  listTasks: (params?: { conversation_id?: string; scope?: UserTaskScope; limit?: number }) =>
    api.get<UserTaskProjection[]>('/tasks', { params }),

  getTask: (id: string, conversationId?: string) =>
    api.get<UserTaskProjection>(`/tasks/${id}`, {
      params: conversationId ? { conversation_id: conversationId } : undefined,
    }),

  performTaskAction: (id: string, action: UserTaskActionID, payload?: unknown) =>
    api.post<UserTaskProjection>(`/tasks/${id}/actions/${action}`, payload),

  performTaskActionDescriptor: (descriptor: UserTaskActionDescriptor, payload?: unknown) =>
    api.request<UserTaskProjection>({
      url: descriptor.path,
      method: descriptor.method || 'POST',
      data: payload,
    }),
}
