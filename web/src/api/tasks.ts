import api from './client'

export type UserTaskKind = 'agent_task' | 'research'
export type UserTaskScope = 'current' | 'background' | 'all'
export type UserTaskStatus = 'running' | 'waiting_user' | 'completed' | 'failed' | 'cancelled'
export type UserTaskStage =
  | 'planning'
  | 'working'
  | 'verifying'
  | 'waiting_user'
  | 'completed'
  | 'failed'
  | 'cancelled'

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

export interface UserTaskActions {
  can_cancel: boolean
  can_open_chat: boolean
  can_send_update: boolean
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
  actions: UserTaskActions
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

  cancelTask: (id: string) => api.post<UserTaskProjection>(`/tasks/${id}/cancel`),
}
