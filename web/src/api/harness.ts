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

export interface HarnessRunGroupItemSpec {
  run_kind: HarnessRunKind
  profile?: string
  input?: Record<string, unknown> | null
  expected?: Record<string, unknown> | null
  metadata?: Record<string, unknown> | null
}

export interface HarnessRunGroupSpec {
  kind?: HarnessRunGroupKind
  title?: string
  subject?: string
  metadata?: Record<string, unknown> | null
  scheduler?: {
    max_concurrency?: number
    max_attempts?: number
    lease_ttl?: number
    retry_backoff?: number
  }
  scoring?: {
    mode?: HarnessScoringMode
    rule_profile?: string
    judge_model?: string
    pass_threshold?: number
  }
  items: HarnessRunGroupItemSpec[]
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

export interface HarnessContract {
  deliverables?: string[]
  success_criteria?: string[]
  expected_artifacts?: Array<{
    path?: string
    label?: string
    must_exist?: boolean
  }>
  required_tool_calls?: string[]
  forbidden_tool_calls?: string[]
  required_checks?: string[]
  required_observations?: string[]
  forbidden_observations?: string[]
  browser_checks?: Array<{
    name?: string
    target?: string
    expectation?: string
    required_observation?: string
    required_artifact?: string
    failure_label?: string
    require_screenshot?: boolean
  }>
  api_checks?: Array<{
    name?: string
    target?: string
    expectation?: string
    required_check?: string
    failure_label?: string
  }>
  fallback_order?: string[]
  stop_conditions?: string[]
  evaluator_hints?: string[]
  risk_level?: string
}

export interface HarnessRuntimeEvidenceEntry {
  id: string
  run_id: string
  step_index?: number
  planner_round?: number
  event_type: string
  summary?: string
  payload_json?: string
  created_at: string
}

export interface HarnessCheckpointArtifact {
  run_id: string
  group_item_id?: string
  artifact: HarnessArtifactRef
  payload?: Record<string, unknown> | null
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
  runtime_evidence?: Record<string, HarnessRuntimeEvidenceEntry[]>
  item_contracts?: Record<string, HarnessContract>
  checkpoints?: HarnessCheckpointArtifact[]
}

export interface HarnessDataset {
  id: string
  name: string
  description?: string
  owner_user_id?: string
  subject?: string
  default_run_kind?: HarnessRunKind
  default_profile?: string
  active_version_id?: string
  metadata?: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export interface HarnessDatasetVersion {
  id: string
  dataset_id: string
  version: string
  manifest_sha256?: string
  item_count: number
  source_type?: string
  source_ref?: string
  manifest?: Record<string, unknown> | null
  metadata?: Record<string, unknown> | null
  created_by?: string
  created_at: string
}

export interface HarnessDatasetSpec {
  name: string
  description?: string
  subject?: string
  default_run_kind?: HarnessRunKind
  default_profile?: string
  metadata?: Record<string, unknown> | null
}

export interface HarnessDatasetVersionSpec {
  version?: string
  source_type?: string
  source_ref?: string
  manifest: Record<string, unknown>
  metadata?: Record<string, unknown> | null
}

export interface HarnessPromoteGroupSpec {
  dataset_name: string
  description?: string
  subject?: string
  eval_name: string
}

export interface HarnessGroupPromotionResult {
  dataset?: HarnessDataset | null
  dataset_version?: HarnessDatasetVersion | null
  eval_spec?: HarnessEvalSpec | null
}

export interface HarnessEvalSpec {
  id: string
  name: string
  owner_user_id?: string
  subject?: string
  run_kind: HarnessRunKind
  profile?: string
  dataset_id: string
  dataset_version_id?: string
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
  runtime_policy?: Record<string, unknown> | null
  metadata?: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export interface HarnessEvalSpecSpec {
  name: string
  subject?: string
  run_kind?: HarnessRunKind
  profile?: string
  dataset_id: string
  dataset_version_id?: string
  scheduler?: {
    max_concurrency?: number
    max_attempts?: number
    lease_ttl?: number
    retry_backoff?: number
  }
  scoring?: {
    mode?: HarnessScoringMode
    rule_profile?: string
    judge_model?: string
    pass_threshold?: number
  }
  runtime_policy?: Record<string, unknown> | null
  metadata?: Record<string, unknown> | null
}

export interface HarnessEvalRun {
  id: string
  eval_spec_id: string
  group_id: string
  dataset_version_id?: string
  baseline_eval_run_id?: string
  title?: string
  owner_user_id?: string
  status: HarnessRunGroupStatus
  trigger_kind?: string
  trigger_ref?: string
  metadata?: Record<string, unknown> | null
  summary?: Record<string, unknown> | null
  created_at: string
  updated_at: string
  started_at?: string | null
  finished_at?: string | null
}

export interface HarnessEvalRunSpec {
  eval_spec_id: string
  baseline_eval_run_id?: string
  title?: string
  trigger_kind?: string
  trigger_ref?: string
  metadata?: Record<string, unknown> | null
}

export interface HarnessEvalRunReport {
  eval_run: HarnessEvalRun
  eval_spec?: HarnessEvalSpec | null
  dataset?: HarnessDataset | null
  dataset_version?: HarnessDatasetVersion | null
  group_report?: HarnessRunGroupReport | null
}

export interface HarnessBaseline {
  id: string
  name: string
  subject?: string
  owner_user_id?: string
  eval_spec_id: string
  eval_run_id: string
  is_default: boolean
  metadata?: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export interface HarnessBaselineSpec {
  name: string
  subject?: string
  eval_spec_id?: string
  eval_run_id: string
  is_default?: boolean
  metadata?: Record<string, unknown> | null
}

export interface HarnessComparisonCaseDelta {
  key: string
  label?: string
  item_index: number
  profile?: string
  base_verdict?: string
  target_verdict?: string
  base_status?: string
  target_status?: string
  base_score?: number
  target_score?: number
  delta_score?: number
  base_run_id?: string
  target_run_id?: string
  base_reason?: string
  target_reason?: string
  base_failure_label?: string
  target_failure_label?: string
  base_verification?: string
  target_verification?: string
  base_evidence_score?: number
  target_evidence_score?: number
}

export interface HarnessComparisonReport {
  id: string
  owner_user_id?: string
  baseline_id?: string
  eval_spec_id: string
  base_eval_run_id: string
  target_eval_run_id: string
  summary?: Record<string, unknown> | null
  regressions?: HarnessComparisonCaseDelta[]
  improvements?: HarnessComparisonCaseDelta[]
  scorer_delta?: Record<string, unknown> | null
  created_at: string
}

export interface HarnessRunGroupListParams {
  limit?: number
  kind?: HarnessRunGroupKind | HarnessRunGroupKind[]
  status?: HarnessRunGroupStatus | HarnessRunGroupStatus[]
}

export interface HarnessDatasetListParams {
  limit?: number
}

export interface HarnessDatasetVersionListParams {
  limit?: number
}

export interface HarnessEvalSpecListParams {
  limit?: number
  datasetID?: string
}

export interface HarnessEvalRunListParams {
  limit?: number
  evalSpecID?: string
  status?: HarnessRunGroupStatus | HarnessRunGroupStatus[]
}

export interface HarnessBaselineListParams {
  limit?: number
  evalSpecID?: string
}

export interface HarnessCompareEvalRunRequest {
  baseline_id?: string
  base_eval_run_id?: string
}

function normalizeQueryArray(value?: string | string[]) {
  if (!value) return undefined
  return Array.isArray(value) ? value.join(',') : value
}

export const harnessApi = {
  createGroup: (payload: HarnessRunGroupSpec) =>
    api.post<HarnessRunGroup>('/harness/groups', payload),

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

  promoteGroup: (id: string, payload: HarnessPromoteGroupSpec) =>
    api.post<HarnessGroupPromotionResult>(`/harness/groups/${id}/promote`, payload),

  listDatasets: (params: HarnessDatasetListParams = {}) =>
    api.get<HarnessDataset[]>('/harness/datasets', {
      params: {
        limit: params.limit,
      },
    }),

  getDataset: (id: string) => api.get<HarnessDataset>(`/harness/datasets/${id}`),

  createDataset: (payload: HarnessDatasetSpec) =>
    api.post<HarnessDataset>('/harness/datasets', payload),

  listDatasetVersions: (datasetID: string, params: HarnessDatasetVersionListParams = {}) =>
    api.get<HarnessDatasetVersion[]>(`/harness/datasets/${datasetID}/versions`, {
      params: {
        limit: params.limit,
      },
    }),

  getDatasetVersion: (id: string) =>
    api.get<HarnessDatasetVersion>(`/harness/dataset-versions/${id}`),

  createDatasetVersion: (datasetID: string, payload: HarnessDatasetVersionSpec) =>
    api.post<HarnessDatasetVersion>(`/harness/datasets/${datasetID}/versions`, payload),

  listEvalSpecs: (params: HarnessEvalSpecListParams = {}) =>
    api.get<HarnessEvalSpec[]>('/harness/eval-specs', {
      params: {
        limit: params.limit,
        dataset_id: params.datasetID,
      },
    }),

  getEvalSpec: (id: string) => api.get<HarnessEvalSpec>(`/harness/eval-specs/${id}`),

  createEvalSpec: (payload: HarnessEvalSpecSpec) =>
    api.post<HarnessEvalSpec>('/harness/eval-specs', payload),

  listEvalRuns: (params: HarnessEvalRunListParams = {}) =>
    api.get<HarnessEvalRun[]>('/harness/eval-runs', {
      params: {
        limit: params.limit,
        eval_spec_id: params.evalSpecID,
        statuses: normalizeQueryArray(params.status),
      },
    }),

  getEvalRun: (id: string) => api.get<HarnessEvalRun>(`/harness/eval-runs/${id}`),

  createEvalRun: (payload: HarnessEvalRunSpec) =>
    api.post<HarnessEvalRun>('/harness/eval-runs', payload),

  getEvalRunReport: (id: string) =>
    api.get<HarnessEvalRunReport>(`/harness/eval-runs/${id}/report`),

  cancelEvalRun: (id: string) => api.post<{ status: string }>(`/harness/eval-runs/${id}/cancel`),

  compareEvalRun: (id: string, payload: HarnessCompareEvalRunRequest = {}) =>
    api.post<HarnessComparisonReport>(`/harness/eval-runs/${id}/compare`, payload),

  listBaselines: (params: HarnessBaselineListParams = {}) =>
    api.get<HarnessBaseline[]>('/harness/baselines', {
      params: {
        limit: params.limit,
        eval_spec_id: params.evalSpecID,
      },
    }),

  createBaseline: (payload: HarnessBaselineSpec) =>
    api.post<HarnessBaseline>('/harness/baselines', payload),
}
