import api from './client'

export type HarnessRunKind = 'agent_task' | 'research' | 'subagent'
export type HarnessRunStatus =
  | 'pending'
  | 'planning'
  | 'waiting_input'
  | 'executing'
  | 'verifying'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'aborted'

export type HarnessRunGroupKind = 'eval' | 'experiment' | 'batch'
export type HarnessRunGroupStatus =
  | 'pending'
  | 'queued'
  | 'running'
  | 'scoring'
  | 'completed'
  | 'partial'
  | 'failed'
  | 'cancelled'

export type HarnessRunGroupItemStatus =
  | 'pending'
  | 'queued'
  | 'running'
  | 'scoring'
  | 'passed'
  | 'failed'
  | 'error'
  | 'cancelled'

export type HarnessScoringMode = 'rule' | 'judge' | 'hybrid'
export type HarnessScoreVerdict = 'pass' | 'fail' | 'partial' | 'error'

export interface HarnessRunGroup {
  id: string
  kind: HarnessRunGroupKind
  title?: string
  status: HarnessRunGroupStatus
  owner_user_id?: string
  subject?: string
  scheduler_config?: {
    max_concurrency?: number
    max_attempts?: number
    lease_ttl?: number
    retry_backoff?: number
  }
  scoring_config?: {
    mode?: HarnessScoringMode
    rule_profile?: string
    judge_model?: string
    pass_threshold?: number
  }
  metadata?: Record<string, unknown> | null
  summary?: Record<string, unknown> | null
  created_at: string
  updated_at: string
  started_at?: string | null
  finished_at?: string | null
}

export interface HarnessRunGroupItem {
  id: string
  group_id: string
  index: number
  run_kind: HarnessRunKind
  profile?: string
  input?: Record<string, unknown> | null
  expected?: Record<string, unknown> | null
  metadata?: Record<string, unknown> | null
  status: HarnessRunGroupItemStatus
  latest_run_id?: string
  attempt_count: number
  max_attempts: number
  lease_owner?: string
  lease_expires_at?: string | null
  created_at: string
  updated_at: string
}

export interface HarnessScorecard {
  id: string
  group_id: string
  group_item_id: string
  run_id?: string
  mode: HarnessScoringMode
  verdict: HarnessScoreVerdict
  score: number
  breakdown_json?: string
  evidence_json?: string
  judge_trace_json?: string
  created_at: string
}

export interface HarnessRunSummary {
  id: string
  root_run_id: string
  parent_run_id?: string
  group_id?: string
  group_item_id?: string
  attempt_index?: number
  kind: HarnessRunKind
  status: HarnessRunStatus
  goal: string
  result?: string
  error?: string
  agent_id?: string
  model?: string
  created_at: string
  updated_at: string
  started_at?: string | null
  finished_at?: string | null
}

export interface HarnessArtifactRef {
  id: string
  run_id: string
  kind: 'file' | 'dir' | 'url' | 'report' | 'log' | 'snapshot' | string
  label?: string
  path_or_url?: string
  mime_type?: string
  size_bytes?: number
  metadata_json?: string
}

export interface HarnessRunGroupReport {
  group: HarnessRunGroup
  items?: HarnessRunGroupItem[]
  verdict_counts?: Record<string, number>
  overall_score?: number
  pass_rate?: number
  breakdown?: Record<string, unknown> | null
  failed_items?: Array<Record<string, unknown>>
  linked_runs?: HarnessRunSummary[]
  artifacts?: HarnessArtifactRef[]
  scorecards?: HarnessScorecard[]
}

export interface HarnessRunGroupListParams {
  limit?: number
  kind?: HarnessRunGroupKind | HarnessRunGroupKind[]
  status?: HarnessRunGroupStatus | HarnessRunGroupStatus[]
}

function normalizeQueryArray(value?: string | string[]) {
  if (!value) return undefined
  return Array.isArray(value) ? value.join(',') : value
}

export const harnessApi = {
  listGroups: (params: HarnessRunGroupListParams = {}) =>
    api.get<HarnessRunGroup[]>('/harness/groups', {
      params: {
        limit: params.limit,
        kinds: normalizeQueryArray(params.kind),
        statuses: normalizeQueryArray(params.status),
      },
    }),

  getGroup: (id: string) => api.get<HarnessRunGroup>(`/harness/groups/${id}`),

  getGroupItems: (id: string) => api.get<HarnessRunGroupItem[]>(`/harness/groups/${id}/items`),

  getGroupReport: (id: string) => api.get<HarnessRunGroupReport>(`/harness/groups/${id}/report`),

  cancelGroup: (id: string) => api.post<{ status: string }>(`/harness/groups/${id}/cancel`),

  retryFailedGroup: (id: string) =>
    api.post<{ retried: number }>(`/harness/groups/${id}/retry_failed`),
}
